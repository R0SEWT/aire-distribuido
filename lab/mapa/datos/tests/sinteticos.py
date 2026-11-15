"""netCDF, GeoTIFF y GeoJSON sintéticos con la forma de los de Copernicus, WorldPop y geoBoundaries.

Los campos son funciones conocidas para que las pruebas calculen el valor esperado: lineales en
latitud, longitud y tiempo (la interpolación bilineal y la lineal en el tiempo los reproducen
exactos) y, para precipitación y radiación, acumulados con la convención de ERA5-Land.
"""

import json
from datetime import date, datetime, timedelta
from pathlib import Path

import numpy as np
import rasterio
import xarray as xr
from rasterio.transform import from_origin

from crudo import CAMS, ERA5, ERA5LAND, INVARIANTES_ARCHIVO, Fuente, area, horas_esperadas, ruta_mes
from grilla import Grilla

EPOCA = datetime(2018, 1, 1)
LON_MAR = -77.45  # en ERA5-Land, las longitudes al oeste de esta son mar (NaN)

GRILLA_PRUEBA = Grilla(lat_norte=-11.0, lon_oeste=-77.5, paso=0.1, filas=8, columnas=6)


def horas(t: datetime) -> float:
    return (t - EPOCA).total_seconds() / 3600


def instantaneo(k: int, t: datetime, lat, lon):
    """Campo instantáneo número k: lineal en tiempo, latitud y longitud."""
    return 10.0 * k + 0.01 * horas(t) + 0.3 * lat + 0.2 * lon


def tasa(k: int, fin: datetime, lat, lon):
    """Cantidad horaria del acumulado k en el intervalo (fin − 1 h, fin] (en la unidad del archivo)."""
    return 1e-4 * (1 + horas(fin) % 7) * (1 + 0.01 * k) + 0 * lat + 0 * lon


def acumulado_era5land(k: int, t: datetime, lat, lon):
    """Acumulado desde las 00 UTC: el de la 00 UTC es el total del día anterior."""
    inicio = datetime.combine((t - timedelta(hours=1)).date(), datetime.min.time())
    total, fin = 0.0, inicio + timedelta(hours=1)
    while fin <= t:
        total = total + tasa(k, fin, lat, lon)
        fin += timedelta(hours=1)
    return total


def cams(k: int, t: datetime, lat, lon):
    """CAMS en su unidad: PM en kg/m³ (µg/m³ × 1e-9), AOD sin unidad. Lineal en tiempo y espacio."""
    valor = 20.0 + 5.0 * k + 0.05 * horas(t) + 0.4 * lat + 0.1 * lon
    return valor * 1e-9 if k < 2 else valor / 100.0


def coordenadas(fuente: Fuente, grilla: Grilla):
    n, o, s, e = area(fuente, grilla)
    r = fuente.resolucion
    lats = np.round(np.arange(round((n - s) / r) + 1) * -r + n, 6)
    lons = np.round(np.arange(round((e - o) / r) + 1) * r + o, 6)
    return lats, lons


def _dataset(nombres, funciones, tiempos, lats, lons, lon_archivo):
    lat2, lon2 = np.meshgrid(lats, lons, indexing="ij")
    datos = {}
    for nombre, f in zip(nombres, funciones):
        datos[nombre] = (
            ("valid_time", "latitude", "longitude"),
            np.stack([f(t, lat2, lon2) for t in tiempos]).astype("float32"),
        )
    ds = xr.Dataset(
        datos,
        coords={
            "valid_time": np.array(tiempos, dtype="datetime64[ns]"),
            "latitude": lats,
            "longitude": lon_archivo,
            "number": 0,
            "expver": ("valid_time", np.array(["0001"] * len(tiempos))),
        },
    )
    return ds


def escribir_fuente(dir_crudo: Path, fuente: Fuente, desde: date, hasta: date, grilla: Grilla = GRILLA_PRUEBA):
    """Escribe los AAAA-MM.nc de una fuente con los días que pediría descargar_copernicus."""
    lats, lons = coordenadas(fuente, grilla)
    todos = [desde + timedelta(days=k) for k in range(-fuente.dias_antes, (hasta - desde).days + 1 + fuente.dias_despues)]
    meses = sorted({(d.year, d.month) for d in todos})
    for anio, mes in meses:
        dias_mes = [d for d in todos if (d.year, d.month) == (anio, mes)]
        tiempos = horas_esperadas(fuente, dias_mes)
        lon_archivo = lons
        if fuente is ERA5LAND:
            def mar(v, lon):
                return np.where(lon < LON_MAR, np.nan, v)

            funciones = [
                (lambda t, la, lo, k=k: mar(instantaneo(k, t, la, lo), lo)) for k in range(5)
            ] + [(lambda t, la, lo, k=k: mar(acumulado_era5land(k, t, la, lo), lo)) for k in (5, 6)]
        elif fuente is ERA5:
            funciones = [(lambda t, la, lo, k=k: instantaneo(k, t, la, lo)) for k in (7, 8)]
        else:
            funciones = [(lambda t, la, lo, k=k: cams(k, t, la, lo)) for k in range(4)]
            lon_archivo = np.round(lons % 360, 6)  # ADS puede devolver longitudes 0–360
        ds = _dataset(fuente.nombres_netcdf, funciones, tiempos, lats, lons, lon_archivo)
        ruta = ruta_mes(dir_crudo, fuente, anio, mes)
        ruta.parent.mkdir(parents=True, exist_ok=True)
        ds.to_netcdf(ruta, engine="netcdf4")


def orografia(lat, lon):
    return 100.0 + 50.0 * (lat + 11.0) + 20.0 * (lon + 77.0)


def escribir_invariantes(dir_crudo: Path, grilla: Grilla = GRILLA_PRUEBA):
    lats, lons = coordenadas(ERA5LAND, grilla)
    lat2, lon2 = np.meshgrid(lats, lons, indexing="ij")
    z = (orografia(lat2, lon2) * 9.80665)[None].astype("float32")
    lsm = np.where(lon2 < LON_MAR, 0.0, 1.0)[None].astype("float32")
    ds = xr.Dataset(
        {"z": (("valid_time", "latitude", "longitude"), z), "lsm": (("valid_time", "latitude", "longitude"), lsm)},
        coords={"valid_time": np.array([datetime(1970, 1, 1)], dtype="datetime64[ns]"), "latitude": lats, "longitude": lons},
    )
    ruta = dir_crudo / ERA5LAND.nombre / INVARIANTES_ARCHIVO
    ruta.parent.mkdir(parents=True, exist_ok=True)
    ds.to_netcdf(ruta, engine="netcdf4")


def escribir_crudo(dir_crudo: Path, desde: date, hasta: date, grilla: Grilla = GRILLA_PRUEBA):
    for fuente in (ERA5LAND, ERA5, CAMS):
        escribir_fuente(dir_crudo, fuente, desde, hasta, grilla)
    escribir_invariantes(dir_crudo, grilla)


PIXELES_POR_CELDA = 12  # 0.1° / (1/120°)


def escribir_worldpop(ruta: Path, grilla: Grilla = GRILLA_PRUEBA):
    """Un raster de 30″ (1/120°) alineado con las celdas: cada píxel vale 1 salvo un bloque sin dato."""
    p = PIXELES_POR_CELDA
    alto, ancho = grilla.filas * p, grilla.columnas * p
    datos = np.ones((alto, ancho), dtype="float32")
    datos[:p, :p] = -99999.0  # la celda (0, 0) queda sin dato → población 0
    transformacion = from_origin(grilla.lon_oeste - grilla.paso / 2, grilla.lat_norte + grilla.paso / 2, 1 / 120, 1 / 120)
    ruta.parent.mkdir(parents=True, exist_ok=True)
    with rasterio.open(
        ruta, "w", driver="GTiff", height=alto, width=ancho, count=1, dtype="float32",
        crs="EPSG:4326", transform=transformacion, nodata=-99999.0,
    ) as r:
        r.write(datos, 1)


def escribir_adm0(ruta: Path):
    """Un 'país' que cubre las celdas con latitud ≤ −11.25 y longitud ≥ −77.35 de la grilla de prueba."""
    poligono = [[-77.35, -11.25], [-76.0, -11.25], [-76.0, -12.5], [-77.35, -12.5], [-77.35, -11.25]]
    geo = {
        "type": "FeatureCollection",
        "features": [{"type": "Feature", "properties": {"shapeName": "Prueba"}, "geometry": {"type": "Polygon", "coordinates": [poligono]}}],
    }
    ruta.parent.mkdir(parents=True, exist_ok=True)
    ruta.write_text(json.dumps(geo))
