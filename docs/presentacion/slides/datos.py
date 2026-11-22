"""Datos de los gráficos del deck: lee reports/*.json y escribe datos.js.

    python3 datos.py        # desde docs/presentacion/slides/ (solo biblioteca estándar)

Las cifras del texto no pasan por aquí: index.qmd las toma de docs/informe/generado/cifras.yml con
el shortcode meta, igual que el informe. Este script solo prepara las series que dibuja D3
(js/graficos.js), sin redondear ni escribir ningún número a mano, para que el deck no pueda
contradecir al informe. datos.js se versiona: el deck se renderiza sin correr Go ni Python.
"""

from __future__ import annotations

import json
import re
from pathlib import Path

AQUI = Path(__file__).resolve().parent
RAIZ = AQUI.parents[2]  # docs/presentacion/slides → raíz del repo
REPORTS = RAIZ / "reports"

# Motivos de descarte del cargador, en palabras de la exposición. Los que no están aquí van a «otras».
MOTIVOS = {
    "bloque_en_cero": "día entero en 0 (no medido)",
    "sin_valor": "fila sin valor",
    "periodo": "promedio diario o móvil",
    "duplicado_unidad": "gas repetido en otra unidad",
    "fuera_de_rango": "fuera del rango físico",
}


def leer(nombre: str) -> dict:
    return json.loads((REPORTS / nombre).read_text(encoding="utf-8"))


def embudo(limpieza: dict) -> dict:
    fl = limpieza["filas_largas"]
    por_regla = fl["descartadas_por_regla"]
    motivos = [{"motivo": MOTIVOS[k], "filas": por_regla.get(k, 0)} for k in MOTIVOS]
    otras = sum(v for k, v in por_regla.items() if k not in MOTIVOS)
    motivos.sort(key=lambda m: -m["filas"])
    motivos.append({"motivo": "otras reglas", "filas": otras})
    assert sum(m["filas"] for m in motivos) == fl["descartadas"], "los motivos no suman las descartadas"
    assert fl["procesadas"] + fl["descartadas"] == fl["leidas"], "procesadas + descartadas ≠ leídas"
    estaciones = limpieza["estaciones"]
    return {
        "leidas": fl["leidas"],
        "procesadas": fl["procesadas"],
        "descartadas": fl["descartadas"],
        "motivos": motivos,
        "hora_inferida": fl["hora_inferida"],
        "anchas": limpieza["filas_anchas"]["total"] if isinstance(limpieza["filas_anchas"], dict)
        else limpieza["filas_anchas"],
        "completas": sum(e.get("completas_gold", 0) for e in estaciones.values()),
        "estaciones": len(estaciones),
        "estaciones_gold": sum(1 for e in estaciones.values() if e.get("completas_gold", 0) > 0),
    }


def speedup_cargador(bc: dict) -> list[dict]:
    return [{"w": m["workers"], "punto": m["speedup_ic"]["punto"], "inf": m["speedup_ic"]["inferior"],
             "sup": m["speedup_ic"]["superior"]} for m in sorted(bc["mediciones"], key=lambda m: m["workers"])]


def speedup_kmeans(bk: dict, chunk: int, experimento: str = "convergencia") -> list[dict]:
    filas = [r for r in bk["resumen"] if r.get("experimento") == experimento and r.get("modo") == "conc"
             and r.get("chunk") == chunk]
    return [{"w": r["workers"], "punto": r["speedup"]["punto"], "inf": r["speedup"]["inferior"],
             "sup": r["speedup"]["superior"]} for r in sorted(filas, key=lambda r: r["workers"])]


def niveles(reg: dict, limpieza: dict) -> dict:
    # El nombre del nivel sale del PM2,5 medio de cada régimen, como en el informe (tablas.py).
    orden = sorted(reg["regimenes"], key=lambda r: r["pm2_5_medio"])
    nombres = dict(zip((r["regimen"] for r in orden), ("bajo", "medio", "alto")))
    estaciones = limpieza["estaciones"]

    def ubicacion(e: str) -> dict:
        u = estaciones.get(e, {}).get("ubicacion", {})
        return {"departamento": (u.get("departamento") or "").title(), "altitud": u.get("altitud")}

    filas = []
    for r in orden:
        total = r["tamano"]
        top = sorted(r["por_estacion"].items(), key=lambda t: -t[1])[:5]
        filas.append({
            "nivel": nombres[r["regimen"]],
            "regimen": r["regimen"],
            "horas": total,
            "fraccion": r["fraccion"],
            "pm2_5": r["pm2_5_medio"],
            "pm10": r["pm10_medio"],
            "eca": r["eca"],
            "por_hora": r["por_hora"],
            "estaciones": [{"estacion": e, "horas": n, "fraccion": n / total, **ubicacion(e)} for e, n in top],
        })
    return {"niveles": filas, "eca": {"pm2_5": reg["eca"]["pm2_5_ugm3"], "pm10": reg["eca"]["pm10_ugm3"]},
            "convencion_hora": reg.get("convencion_hora")}


def pruebas() -> int:
    """Funciones Test* de los archivos versionados: lo que corre go test -race ./... (sin las copias
    de las carpetas ocultas, como los worktrees de .claude/)."""
    patron = re.compile(r"^func Test\w*\(t \*testing\.T\)", re.M)
    return sum(len(patron.findall(p.read_text(encoding="utf-8"))) for p in RAIZ.rglob("*_test.go")
               if not any(parte.startswith(".") for parte in p.relative_to(RAIZ).parts))


def portada(limpieza: dict) -> str:
    """Fondo de la portada: las estaciones en su posición real (lon escalada por cos(lat)), con
    colores fijos porque una imagen de fondo no hereda el CSS. Llenas, las que llegan al gold."""
    import math

    puntos = []
    for e, x in limpieza["estaciones"].items():
        u = x.get("ubicacion") or {}
        try:
            lat, lon = float(u["lat"]), float(u["lon"])
        except (KeyError, TypeError, ValueError):
            continue
        puntos.append((lat, lon, x.get("anchas", 0), x.get("completas_gold", 0) > 0))
    # Marco fijo del Perú, para que la forma se reconozca aunque falten estaciones en el oriente
    lat_n, lat_s, lon_o, lon_e = 0.2, -18.6, -81.5, -68.5
    k = math.cos(math.radians((lat_n + lat_s) / 2))
    ancho, alto = 420.0, 600.0
    esc = min(ancho / ((lon_e - lon_o) * k), alto / (lat_n - lat_s))
    max_anchas = max(p[2] for p in puntos) or 1
    circulos = []
    for lat, lon, anchas, gold in sorted(puntos, key=lambda p: -p[2]):
        cx, cy = (lon - lon_o) * k * esc, (lat_n - lat) * esc
        r = 5 + 22 * math.sqrt(anchas / max_anchas)
        estilo = ('fill="#86B6EF" fill-opacity="0.85" stroke="#FFFFFF" stroke-opacity="0.5" stroke-width="1.5"'
                  if gold else 'fill="none" stroke="#7F93AA" stroke-width="2" stroke-dasharray="4 3"')
        circulos.append(f'<circle cx="{cx:.1f}" cy="{cy:.1f}" r="{r:.1f}" {estilo}/>')
    return (f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="-30 -30 {ancho + 60:.0f} {alto + 60:.0f}">'
            + "".join(circulos) + "</svg>\n")


def main() -> None:
    limpieza = leer("limpieza.json")
    bk = leer("benchmark_kmeans.json")
    datos = {
        "embudo": embudo(limpieza),
        "speedup": {
            "cargador": speedup_cargador(leer("benchmark_cargador.json")),
            "kmeans_1000": speedup_kmeans(bk, 1000),
            "kmeans_10000": speedup_kmeans(bk, 10000),
            "chunks_1000": -(-bk["datos"]["n"] // 1000),
            "chunks_10000": -(-bk["datos"]["n"] // 10000),
        },
        **niveles(leer("regimenes.json"), limpieza),
        "pruebas": pruebas(),
    }
    js = ("// Generado por datos.py desde reports/*.json. No editar a mano.\n"
          "window.DATOS = " + json.dumps(datos, ensure_ascii=False, indent=1) + ";\n")
    (AQUI / "datos.js").write_text(js, encoding="utf-8")
    # Lo que el texto del deck cita y no está en cifras.yml: se lee con {{< meta deck.x >}}
    (AQUI / "_deck.yml").write_text(
        "# Generado por datos.py. No editar a mano.\n"
        f'deck:\n  pruebas: "{datos["pruebas"]}"\n', encoding="utf-8")
    (AQUI / "img" / "estaciones-portada.svg").write_text(portada(limpieza), encoding="utf-8")
    print(f"datos.js, _deck.yml e img/estaciones-portada.svg: {len(datos['niveles'])} niveles, "
          f"{datos['pruebas']} pruebas")


if __name__ == "__main__":
    main()
