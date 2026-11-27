#!/usr/bin/env python3
"""Arma web/public/datos/peru.geojson: los departamentos del Perú para el mapa del panel (spec del
frontend, §3 y §15: solo departamentos, simplificado, con su licencia anotada).

La fuente es geoBoundaries (gbOpen, PER, ADM1), que se baja a una carpeta fuera del repo y no se
versiona; aquí entra solo el resultado. Se parte de la geometría ya simplificada que publica la
fuente y se hace lo mínimo para que pese menos de 300 KB:

- coordenadas redondeadas a 4 decimales (unos 11 m), sin puntos repetidos seguidos;
- tres propiedades por polígono: `departamento` (la clave con que lo nombra
  reports/estaciones_ficha.csv: mayúsculas y sin tildes), `nombre` (el de la fuente, tal cual) e
  `iso` (ISO 3166-2; la fuente escribe «PE-LMA*» y aquí va sin el asterisco);
- anillos exteriores en sentido horario, que es lo que espera d3-geo. La fuente sigue el RFC 7946
  (antihorario) y con eso `d3.geoPath` sobre una proyección esférica pinta el mundo entero menos
  el departamento. Leaflet, QGIS y geojson.io aceptan los dos sentidos.

La fuente trae 26 polígonos y no 25: Lima Metropolitana va aparte del departamento de Lima, y los dos
llevan `departamento` = LIMA. El Callao es una provincia constitucional; la ficha lo trata como
departamento (CALLAO) y aquí también.

Uso, desde la raíz del repositorio:

    python3 -I scripts/mapa_peru.py RUTA/geoBoundaries-PER-ADM1_simplified.geojson
    python3 scripts/mapa_peru.py --verificar     # solo comprueba el archivo versionado

Las dos formas comprueban que las 33 estaciones de la ficha caigan dentro del departamento que dice
su fila, o a menos de 5 km de su borde (el trazo es grueso): un punto en el mar o en otro país es
señal de coordenadas invertidas. Solo usa la biblioteca estándar.
"""

import argparse
import csv
import json
import math
import sys
import unicodedata
from pathlib import Path

RAIZ = Path(__file__).resolve().parents[1]
SALIDA = RAIZ / "web/public/datos/peru.geojson"
FICHA = RAIZ / "reports/estaciones_ficha.csv"

DECIMALES = 4
MAX_BYTES = 300_000
TOLERANCIA_KM = 5.0
# Los dos polígonos de la fuente cuyo nombre no es el del departamento de la ficha.
CLAVES = {"El Callao": "CALLAO", "Municipalidad Metropolitana de Lima": "LIMA"}


def clave(nombre):
    """El nombre de un departamento como lo escribe la ficha: mayúsculas y sin tildes."""
    if nombre in CLAVES:
        return CLAVES[nombre]
    sin_tildes = "".join(c for c in unicodedata.normalize("NFD", nombre) if unicodedata.category(c) != "Mn")
    return sin_tildes.upper()


def poligonos(geometria):
    if geometria["type"] == "Polygon":
        return [geometria["coordinates"]]
    if geometria["type"] == "MultiPolygon":
        return geometria["coordinates"]
    raise ValueError(f"geometría que no se esperaba: {geometria['type']}")


def area_con_signo(anillo):
    """Positiva si el anillo gira en sentido horario (con el norte arriba)."""
    return sum((x2 - x1) * (y2 + y1) for (x1, y1), (x2, y2) in zip(anillo, anillo[1:])) / 2


def redondear_anillo(anillo, horario):
    puntos = []
    for x, y, *_ in anillo:
        p = [round(x, DECIMALES), round(y, DECIMALES)]
        if not puntos or puntos[-1] != p:
            puntos.append(p)
    if puntos[0] != puntos[-1]:
        puntos.append(puntos[0])
    if len(puntos) < 4 or area_con_signo(puntos) == 0:
        raise ValueError("un anillo quedó sin área al redondear")
    if (area_con_signo(puntos) > 0) != horario:
        puntos.reverse()
    return puntos


def simplificar(fuente):
    """El FeatureCollection que se versiona, a partir del de geoBoundaries."""
    rasgos = []
    for f in fuente["features"]:
        nombre = f["properties"]["shapeName"]
        # El exterior en sentido horario y los huecos al revés.
        polis = [[redondear_anillo(a, horario=(i == 0)) for i, a in enumerate(p)] for p in poligonos(f["geometry"])]
        geometria = ({"type": "Polygon", "coordinates": polis[0]} if len(polis) == 1
                     else {"type": "MultiPolygon", "coordinates": polis})
        rasgos.append({
            "type": "Feature",
            "properties": {"departamento": clave(nombre), "nombre": nombre, "iso": f["properties"]["shapeISO"].rstrip("*")},
            "geometry": geometria,
        })
    rasgos.sort(key=lambda r: (r["properties"]["departamento"], r["properties"]["nombre"]))
    return {"type": "FeatureCollection", "features": rasgos}


def escribir(geo, ruta):
    """Un rasgo por línea: el archivo pesa lo mismo y un cambio se puede leer en un diff."""
    lineas = [json.dumps(r, ensure_ascii=False, separators=(",", ":")) for r in geo["features"]]
    texto = '{"type":"FeatureCollection","features":[\n' + ",\n".join(lineas) + "\n]}\n"
    ruta.parent.mkdir(parents=True, exist_ok=True)
    ruta.write_text(texto, encoding="utf-8")
    return len(texto.encode("utf-8"))


def _en_anillo(x, y, anillo):
    dentro = False
    for (x1, y1), (x2, y2) in zip(anillo, anillo[1:]):
        if (y1 > y) != (y2 > y) and x < (x2 - x1) * (y - y1) / (y2 - y1) + x1:
            dentro = not dentro
    return dentro


def contiene(geometria, lon, lat):
    for exterior, *huecos in poligonos(geometria):
        if _en_anillo(lon, lat, exterior) and not any(_en_anillo(lon, lat, h) for h in huecos):
            return True
    return False


def distancia_km(geometria, lon, lat):
    """Distancia del punto al borde más cercano, con una proyección plana alrededor del punto."""
    kx, ky = 111.32 * math.cos(math.radians(lat)), 110.57
    mejor = math.inf
    for poli in poligonos(geometria):
        for anillo in poli:
            for (x1, y1), (x2, y2) in zip(anillo, anillo[1:]):
                ax, ay = (x1 - lon) * kx, (y1 - lat) * ky
                dx, dy = (x2 - x1) * kx, (y2 - y1) * ky
                largo = dx * dx + dy * dy
                t = 0.0 if largo == 0 else max(0.0, min(1.0, -(ax * dx + ay * dy) / largo))
                mejor = min(mejor, math.hypot(ax + t * dx, ay + t * dy))
    return mejor


def leer_ficha(ruta=FICHA):
    with open(ruta, newline="", encoding="utf-8") as f:
        return list(csv.DictReader(f))


def ubicar_estaciones(geo, ficha):
    """Por estación: el departamento que declara su fila, los polígonos que la contienen y, si ninguno
    es de ese departamento, la distancia en km al más cercano que sí lo es."""
    filas = []
    for e in ficha:
        lon, lat = float(e["lon"]), float(e["lat"])
        dentro = sorted({r["properties"]["departamento"] for r in geo["features"] if contiene(r["geometry"], lon, lat)})
        declarado = e["departamento"]
        km = None
        if declarado != "NA" and declarado not in dentro:
            propios = [r for r in geo["features"] if r["properties"]["departamento"] == declarado]
            km = min((distancia_km(r["geometry"], lon, lat) for r in propios), default=math.inf)
        filas.append({"estacion": e["estacion"], "declarado": declarado, "dentro_de": dentro, "km_al_declarado": km})
    return filas


def problemas(geo, ficha):
    """Lo que está mal en el mapa o en las coordenadas; una lista vacía es que todo cuadra."""
    malos = []
    claves = [r["properties"]["departamento"] for r in geo["features"]]
    if len(geo["features"]) != 26 or len(set(claves)) != 25:
        malos.append(f"se esperaban 26 polígonos de 25 departamentos; hay {len(claves)} de {len(set(claves))}")
    for r in geo["features"]:
        if set(r["properties"]) != {"departamento", "nombre", "iso"} or not all(r["properties"].values()):
            malos.append(f"propiedades incompletas: {r['properties']}")
    for r in geo["features"]:
        for exterior, *huecos in poligonos(r["geometry"]):
            if area_con_signo(exterior) <= 0 or any(area_con_signo(h) >= 0 for h in huecos):
                malos.append(f"{r['properties']['nombre']}: un anillo no gira como espera d3-geo")
    for u in ubicar_estaciones(geo, ficha):
        if not u["dentro_de"]:
            malos.append(f"{u['estacion']}: no cae en ningún departamento (¿coordenadas invertidas?)")
        elif u["declarado"] == "NA":
            continue  # la fuente no le pone departamento: basta con que caiga en el país
        elif u["km_al_declarado"] is not None and u["km_al_declarado"] > TOLERANCIA_KM:
            malos.append(f"{u['estacion']}: su fila dice {u['declarado']} y cae en {u['dentro_de']}, "
                         f"a {u['km_al_declarado']:.1f} km de {u['declarado']}")
    return malos


def main():
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("fuente", nargs="?", help="geoBoundaries-PER-ADM1_simplified.geojson, ya descargado")
    ap.add_argument("--salida", default=str(SALIDA))
    ap.add_argument("--verificar", action="store_true", help="no escribe: comprueba el archivo versionado")
    args = ap.parse_args()
    salida = Path(args.salida)
    if args.verificar:
        geo = json.loads(salida.read_text(encoding="utf-8"))
        peso = salida.stat().st_size
    elif args.fuente:
        geo = simplificar(json.loads(Path(args.fuente).read_text(encoding="utf-8")))
        peso = escribir(geo, salida)
    else:
        ap.error("falta la ruta del GeoJSON de geoBoundaries (o --verificar)")
    ficha = leer_ficha()
    malos = problemas(geo, ficha)
    if peso >= MAX_BYTES:
        malos.append(f"{salida.name} pesa {peso} bytes; el tope es {MAX_BYTES}")
    for u in ubicar_estaciones(geo, ficha):
        if u["declarado"] == "NA" or u["km_al_declarado"] is not None:
            print(f"  {u['estacion']}: fila {u['declarado']}, cae en {', '.join(u['dentro_de']) or 'ninguno'}"
                  + (f" (a {u['km_al_declarado']:.1f} km de {u['declarado']})" if u["km_al_declarado"] is not None else ""))
    vertices = sum(len(a) for r in geo["features"] for p in poligonos(r["geometry"]) for a in p)
    print(f"{salida}: {len(geo['features'])} polígonos, {vertices} vértices, {peso} bytes; "
          f"{len(ficha)} estaciones comprobadas, {len(malos)} problemas")
    for m in malos:
        print("PROBLEMA:", m, file=sys.stderr)
    return 1 if malos else 0


if __name__ == "__main__":
    raise SystemExit(main())
