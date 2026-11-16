"""Arma el mapa con los resultados de inferir.py y cuenta personas-día sobre el ECA por distrito (§6, §7).

Solo laptop (usa shapely; rasterio para el GeoTIFF de WorldPop: uv sync --extra geo).

    uv run --extra geo python agregar.py --datos data/mapa --resultados data/mapa/resultados \
        --poblacion data/mapa/crudo/estaticos/per_ppp_2020_1km_Aggregated.tif \
        --limites data/mapa/crudo/estaticos/geoBoundaries-PER-ADM3.geojson \
        --mapa data/mapa/mapa --salida reports/mapa_distritos.json

Escribe en --mapa: pm25_diario.npy, pm10_diario.npy y sus _desv (float16 [D, filas, columnas]),
dias_sobre_eca.npy (int16 [años, 2, filas, columnas]), distancia_estacion_km.npy y mapa.json (días, años,
grilla). En --salida, por distrito: población y, por año y contaminante, personas-día sobre el ECA (con
la media; y con media ± desviación entre particiones como banda) y días sobre el ECA ponderados por
población (personas-día / población).

Las personas salen de cada píxel de WorldPop (1 km), no de la celda de 0.1°: un píxel cae en un
distrito (punto en polígono, por su centro) y toma el PM de la celda de 0.1° que lo contiene. Así un
distrito más chico que una celda (en Lima hay muchos) igual recibe su población.
"""

from __future__ import annotations

import argparse
import json
import math
from pathlib import Path

import numpy as np

import datos
from datos import CONTAMINANTES, ECA_24H


# ---------------------------------------------------------------- mapa diario


def armar_diarios(carpeta: str | Path) -> tuple[list[str], np.ndarray]:
    """Los mapa_AAAA-MM-DD.npy en orden de día: (días, arreglo float16 [D, 2, 2, filas, columnas])."""
    rutas = sorted(Path(carpeta).glob("mapa_*.npy"))
    if not rutas:
        raise FileNotFoundError(f"no hay mapa_*.npy en {carpeta}")
    dias = [r.stem.removeprefix("mapa_") for r in rutas]
    return dias, np.stack([np.load(r) for r in rutas])


def dias_sobre_eca(dias: list[str], media: np.ndarray) -> tuple[list[int], np.ndarray]:
    """Días sobre el ECA de 24 h por celda y año: (años, int16 [años, 2, filas, columnas]). media es [D, 2, f, c]."""
    anios = sorted({int(d[:4]) for d in dias})
    año_de = np.array([int(d[:4]) for d in dias])
    umbral = np.array([ECA_24H[c] for c in CONTAMINANTES])[:, None, None]
    sobre = media.astype(np.float32) > umbral
    return anios, np.stack([sobre[año_de == a].sum(0) for a in anios]).astype(np.int16)


def distancia_estacion_km(lat_celda: np.ndarray, lon_celda: np.ndarray, lat_est: np.ndarray,
                          lon_est: np.ndarray) -> np.ndarray:
    """Distancia (haversine, km) de cada celda a la estación más cercana."""
    la1, lo1 = np.radians(lat_celda)[..., None], np.radians(lon_celda)[..., None]
    la2, lo2 = np.radians(lat_est), np.radians(lon_est)
    h = np.sin((la2 - la1) / 2) ** 2 + np.cos(la1) * np.cos(la2) * np.sin((lo2 - lo1) / 2) ** 2
    return (2 * 6371.0 * np.arcsin(np.sqrt(h))).min(-1)


# ---------------------------------------------------------------- población y distritos


def grilla(man: datos.Manifiesto) -> dict:
    """Origen y paso de la grilla del manifiesto: {lat_norte, lon_oeste, paso} (preparar.py) o
    {lat0, dlat, lon0, dlon}. La fila i está en lat0 + dlat·i y la columna j en lon0 + dlon·j."""
    g = man.crudo["grilla"]
    if "lat_norte" in g:
        return {"filas": man.filas, "columnas": man.columnas, "lat0": g["lat_norte"], "dlat": -g["paso"],
                "lon0": g["lon_oeste"], "dlon": g["paso"]}
    return {"filas": man.filas, "columnas": man.columnas, "lat0": g["lat0"], "dlat": g["dlat"],
            "lon0": g["lon0"], "dlon": g["dlon"]}


def celdas_de_puntos(lon: np.ndarray, lat: np.ndarray, g: dict) -> tuple[np.ndarray, np.ndarray, np.ndarray]:
    """(i, j, dentro): la celda de 0.1° cuyo centro está más cerca de cada punto."""
    i = np.rint((lat - g["lat0"]) / g["dlat"]).astype(np.int64)
    j = np.rint((lon - g["lon0"]) / g["dlon"]).astype(np.int64)
    dentro = (i >= 0) & (i < g["filas"]) & (j >= 0) & (j < g["columnas"])
    return np.clip(i, 0, g["filas"] - 1), np.clip(j, 0, g["columnas"] - 1), dentro


def leer_poblacion(ruta: str | Path) -> tuple[np.ndarray, np.ndarray, np.ndarray]:
    """(lon, lat, personas) de cada píxel con población. GeoTIFF (WorldPop) o .npz con lon, lat, pob."""
    ruta = Path(ruta)
    if ruta.suffix == ".npz":
        z = np.load(ruta)
        return z["lon"].astype(np.float64), z["lat"].astype(np.float64), z["pob"].astype(np.float64)
    import rasterio

    with rasterio.open(ruta) as r:
        banda = r.read(1, masked=True)
        filas, cols = np.nonzero(~np.ma.getmaskarray(banda) & (banda.filled(0) > 0))
        xs, ys = rasterio.transform.xy(r.transform, filas, cols, offset="center")
        return np.asarray(xs, np.float64), np.asarray(ys, np.float64), banda.data[filas, cols].astype(np.float64)


def leer_limites(ruta: str | Path) -> tuple[list[dict], list]:
    """(propiedades, geometrías shapely) de un GeoJSON de distritos (geoBoundaries ADM3: shapeID, shapeName)."""
    from shapely.geometry import shape

    fc = json.loads(Path(ruta).read_text(encoding="utf-8"))
    props, geoms = [], []
    for k, f in enumerate(fc["features"]):
        p = f.get("properties") or {}
        props.append({"id": str(p.get("shapeID", p.get("id", k))), "nombre": p.get("shapeName", p.get("nombre", ""))})
        geoms.append(shape(f["geometry"]))
    return props, geoms


def asignar_distritos(lon: np.ndarray, lat: np.ndarray, geoms: list) -> np.ndarray:
    """Índice del distrito que contiene cada punto (−1 si ninguno; si cae en dos, el de menor índice)."""
    import shapely

    arbol = shapely.STRtree(geoms)
    puntos = shapely.points(lon, lat)
    idx_p, idx_g = arbol.query(puntos, predicate="intersects")
    distrito = np.full(len(lon), -1, np.int64)
    orden = np.lexsort((idx_g, idx_p))  # por punto y, dentro del punto, por distrito
    idx_p, idx_g = idx_p[orden], idx_g[orden]
    _, primero = np.unique(idx_p, return_index=True)
    distrito[idx_p[primero]] = idx_g[primero]
    return distrito


def personas_dia(diario: np.ndarray, umbral: float, i: np.ndarray, j: np.ndarray, pob: np.ndarray,
                 distrito: np.ndarray, n_distritos: int) -> np.ndarray:
    """Suma, por distrito, de personas × días con el promedio diario de su celda sobre el umbral.

    diario es [D, filas, columnas]; i, j, pob y distrito son de cada píxel de población."""
    dias_sobre = (diario[:, i, j].astype(np.float32) > umbral).sum(0)
    ok = distrito >= 0
    return np.bincount(distrito[ok], weights=(pob * dias_sobre)[ok], minlength=n_distritos)


def por_distrito(dias: list[str], media: np.ndarray, desv: np.ndarray, i: np.ndarray, j: np.ndarray,
                 pob: np.ndarray, distrito: np.ndarray, props: list[dict]) -> list[dict]:
    """Población y personas-día sobre el ECA por distrito, año y contaminante. media y desv: [D, 2, f, c]."""
    n = len(props)
    ok = distrito >= 0
    poblacion = np.bincount(distrito[ok], weights=pob[ok], minlength=n)
    año_de = np.array([int(d[:4]) for d in dias])
    salida = [{**p, "poblacion": round(float(poblacion[k]), 1), "anios": {}} for k, p in enumerate(props)]
    for a in sorted(set(año_de.tolist())):
        sel = año_de == a
        for c, nombre in enumerate(CONTAMINANTES):
            u = ECA_24H[nombre]
            m = media[sel, c].astype(np.float32)
            s = desv[sel, c].astype(np.float32)
            centro = personas_dia(m, u, i, j, pob, distrito, n)
            bajo = personas_dia(m - s, u, i, j, pob, distrito, n)
            alto = personas_dia(m + s, u, i, j, pob, distrito, n)
            for k in range(n):
                salida[k]["anios"].setdefault(str(a), {"dias": int(sel.sum())})[nombre] = {
                    "personas_dia": round(float(centro[k]), 1),
                    "personas_dia_bajo": round(float(bajo[k]), 1),
                    "personas_dia_alto": round(float(alto[k]), 1),
                    "dias_ponderados": round(float(centro[k] / poblacion[k]), 3) if poblacion[k] > 0 else None,
                }
    return salida


# ---------------------------------------------------------------- CLI


def main() -> None:
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("--datos", required=True, help="carpeta con manifiesto.json y etiquetas.csv")
    p.add_argument("--resultados", required=True, help="carpeta con mapa_AAAA-MM-DD.npy")
    p.add_argument("--poblacion", required=True, help="GeoTIFF de WorldPop o .npz (lon, lat, pob)")
    p.add_argument("--limites", required=True, help="GeoJSON de distritos (geoBoundaries ADM3)")
    p.add_argument("--mapa", required=True, help="carpeta para los .npy del mapa")
    p.add_argument("--salida", required=True, help="JSON por distrito (reports/mapa_distritos.json)")
    a = p.parse_args()

    origen = datos.Origen(a.datos, verificar=False)
    man = origen.manifiesto
    g = grilla(man)
    dias, arr = armar_diarios(a.resultados)
    media, desv = arr[:, :, 0], arr[:, :, 1]  # [D, 2, f, c]
    mapa = Path(a.mapa)
    mapa.mkdir(parents=True, exist_ok=True)
    for c, corto in enumerate(("pm25", "pm10")):
        np.save(mapa / f"{corto}_diario.npy", media[:, c])
        np.save(mapa / f"{corto}_diario_desv.npy", desv[:, c])
    anios, sobre = dias_sobre_eca(dias, media)
    np.save(mapa / "dias_sobre_eca.npy", sobre)
    lat = g["lat0"] + g["dlat"] * np.arange(g["filas"])
    lon = g["lon0"] + g["dlon"] * np.arange(g["columnas"])
    lat_c, lon_c = np.meshgrid(lat, lon, indexing="ij")
    e = datos.leer_etiquetas(origen.ruta("etiquetas.csv"))
    celdas_est = sorted(set(zip(e.i.tolist(), e.j.tolist())))
    dist = distancia_estacion_km(lat_c, lon_c, np.array([lat[i] for i, _ in celdas_est]),
                                 np.array([lon[j] for _, j in celdas_est]))
    np.save(mapa / "distancia_estacion_km.npy", dist.astype(np.float32))
    (mapa / "mapa.json").write_text(json.dumps({"dias": dias, "anios": anios, "grilla": g,
                                                "contaminantes": list(CONTAMINANTES), "eca_24h": ECA_24H},
                                               ensure_ascii=False), encoding="utf-8")

    plon, plat, pob = leer_poblacion(a.poblacion)
    i, j, dentro = celdas_de_puntos(plon, plat, g)
    props, geoms = leer_limites(a.limites)
    distrito = asignar_distritos(plon, plat, geoms)
    distrito[~dentro] = -1
    filas = por_distrito(dias, media, desv, i, j, pob, distrito, props)
    sin_distrito = float(pob[distrito < 0].sum())
    salida = {"dias": [dias[0], dias[-1]], "n_dias": len(dias), "eca_24h": ECA_24H,
              "poblacion_total": round(float(pob.sum()), 1), "poblacion_sin_distrito": round(sin_distrito, 1),
              "distritos": filas}
    Path(a.salida).parent.mkdir(parents=True, exist_ok=True)
    Path(a.salida).write_text(json.dumps(salida, indent=1, ensure_ascii=False), encoding="utf-8")
    total = sum(f["anios"][str(x)]["pm2_5"]["personas_dia"] for f in filas for x in anios)
    print(f"{len(dias)} días, {len(filas)} distritos, {math.fsum(pob):,.0f} personas; "
          f"personas-día sobre el ECA de PM2.5: {total:,.0f} → {a.salida}")


if __name__ == "__main__":
    main()
