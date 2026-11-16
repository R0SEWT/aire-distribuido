"""Del crudo de Copernicus a los días normalizados, los estáticos y el manifiesto (spec §3.4).

    uv run python preparar.py                          # periodo y grilla de la spec, todos los núcleos menos 2
    uv run python preparar.py --procesos 4
    uv run python preparar.py --grilla=-11.0,-77.5,0.1,24,16 --desde 2018-01-01 --hasta 2018-01-30 \\
        --estadisticas-desde 2018-01-01 --estadisticas-hasta 2018-01-30 --salida /tmp/mapa-chico

Salidas en ``data/mapa/``:

- ``dias/AAAA-MM-DD.npy``: float16 ``[24, C, filas, columnas]``, horas 00–23 UTC. La hora h es el
  instante h:00 UTC para las variables instantáneas y el intervalo (h−1, h] para precipitación y
  radiación: la misma convención "fin" que ``hora_utc`` de etiquetas.csv.
- ``estaticos.npy``: float32 ``[S, filas, columnas]``.
- ``manifiesto.json``: canales y estáticos en orden con su media y desviación, grilla, periodo,
  sha256 de cada archivo, fuentes y licencias.

Cada canal pasa a su unidad física (°C, hPa, mm, W/m², µg/m³) y se normaliza con la media y la
desviación de 2018–2021: z = (x − media) / desviación, como dice la spec. ``Canal.log1p`` permite
normalizar log1p(x) en vez de x (lo declara ``transformacion`` en el manifiesto); hoy ningún canal
dinámico lo usa, para que desnormalizar sea solo z · desviación + media (M0 lee así la PM de CAMS). Las celdas sin dato (el mar en ERA5-Land) valen 0 después de normalizar, es decir la media
del canal; ``mascara_tierra`` dice dónde hay dato.

Regrillado: ERA5-Land coincide con la grilla (se copia el punto); ERA5 (0.25°) y CAMS (0.75°) se
interpolan bilinealmente; CAMS además linealmente en el tiempo, de 3 h a 1 h. Precipitación y
radiación de ERA5-Land vienen acumuladas desde las 00 UTC y la de las 00 UTC trae el total del día
anterior: se desacumulan restando la hora previa (la 01 UTC queda tal cual).

Dos pasadas paralelas por días (multiprocessing): la primera suma momentos por día y los fusiona en
orden de fecha (Chan), la segunda normaliza y escribe. El resultado no depende de ``--procesos``.
"""

import argparse
import io
import json
import os
import sys
from dataclasses import dataclass
from datetime import date, datetime, timedelta, timezone
from multiprocessing import get_context
from pathlib import Path

import numpy as np
import rasterio
import rasterio.features

from comun import (
    DESDE,
    DIR_MAPA,
    ESTADISTICAS_DESDE,
    ESTADISTICAS_HASTA,
    HASTA,
    MANIFIESTO,
    dias,
    escribir_atomico,
    escribir_json,
    fecha,
    sha256_archivo,
    sha256_bytes,
)
from crudo import ERA5LAND, FUENTES, INVARIANTES_ARCHIVO, abrir, leer
from grilla import GRILLA_NACIONAL, Grilla

VERSION = "1"
GRAVEDAD = 9.80665
TOLERANCIA = 1e-6  # grados: un punto a menos de esto de uno de origen es ese punto


@dataclass(frozen=True)
class Canal:
    nombre: str
    fuente: str
    variable: str  # nombre en el netCDF
    unidad: str
    escala: float  # valor físico = crudo × escala + desplazamiento
    desplazamiento: float = 0.0
    acumulada: bool = False
    log1p: bool = False  # normalizar log1p(x) en vez de x


CANALES = (
    Canal("temperatura_2m", "era5land", "t2m", "°C", 1.0, -273.15),
    Canal("rocio_2m", "era5land", "d2m", "°C", 1.0, -273.15),
    Canal("viento_u_10m", "era5land", "u10", "m/s", 1.0),
    Canal("viento_v_10m", "era5land", "v10", "m/s", 1.0),
    Canal("presion_superficie", "era5land", "sp", "hPa", 0.01),
    Canal("precipitacion", "era5land", "tp", "mm en la hora", 1000.0, acumulada=True),
    Canal("radiacion_solar", "era5land", "ssrd", "W/m² medio de la hora", 1 / 3600, acumulada=True),
    Canal("altura_capa_limite", "era5", "blh", "m", 1.0),
    Canal("nubosidad_total", "era5", "tcc", "fracción 0–1", 1.0),
    Canal("cams_pm2_5", "cams", "pm2p5", "µg/m³", 1e9),
    Canal("cams_pm10", "cams", "pm10", "µg/m³", 1e9),
    Canal("aod_550", "cams", "aod550", "sin unidad", 1.0),
    Canal("aod_polvo_550", "cams", "duaod550", "sin unidad", 1.0),
)

FUENTES_COPERNICUS = [
    {"nombre": "ERA5-Land", "dataset": "reanalysis-era5-land", "doi": "10.24381/cds.e2161bac",
     "licencia": "CC-BY-4.0", "servicio": "CDS", "cita": "Muñoz Sabater, J. (2019): ERA5-Land hourly data from 1950 to present. "
     "Copernicus Climate Change Service (C3S) Climate Data Store (CDS). DOI: 10.24381/cds.e2161bac"},
    {"nombre": "ERA5", "dataset": "reanalysis-era5-single-levels", "doi": "10.24381/cds.adbb2d47",
     "licencia": "CC-BY-4.0", "servicio": "CDS", "cita": "Hersbach, H. et al. (2023): ERA5 hourly data on single levels from "
     "1940 to present. Copernicus Climate Change Service (C3S) Climate Data Store (CDS). DOI: 10.24381/cds.adbb2d47"},
    {"nombre": "CAMS EAC4", "dataset": "cams-global-reanalysis-eac4", "doi": "10.24381/d58bbf47",
     "licencia": "CC-BY-4.0", "servicio": "ADS", "cita": "Inness, A. et al. (2019): CAMS global reanalysis (EAC4). "
     "Copernicus Atmosphere Monitoring Service (CAMS) Atmosphere Data Store (ADS). DOI: 10.24381/d58bbf47"},
]
FUENTE_ETIQUETAS = {"nombre": "OEFA", "dataset": "Vigilancia y seguimiento ambiental de la calidad del aire (silver del repo)",
                    "licencia": "datos abiertos del Estado peruano (datosabiertos.gob.pe)", "uso": "etiquetas.csv"}


# --- Interpolación espacial ---------------------------------------------------------------------


def pesos_eje(origen: np.ndarray, destino: np.ndarray):
    """Índices (i0, i1) y fracción f para interpolar linealmente de origen a destino en un eje.

    Si el destino coincide con un punto de origen, i0 = i1 y f = 0: el valor se copia y un vecino
    sin dato (NaN) no lo contamina.
    """
    origen = np.asarray(origen, dtype=np.float64)
    destino = np.asarray(destino, dtype=np.float64)
    n = len(origen)
    if n < 2:
        raise ValueError("hacen falta al menos dos puntos de origen por eje")
    invertido = origen[0] > origen[-1]
    o = origen[::-1] if invertido else origen
    if destino.min() < o[0] - TOLERANCIA or destino.max() > o[-1] + TOLERANCIA:
        raise ValueError(f"el destino {destino.min()}…{destino.max()} se sale del origen {o[0]}…{o[-1]}: no se extrapola")
    i1 = np.clip(np.searchsorted(o, destino), 1, n - 1)
    i0 = i1 - 1
    f = np.clip((destino - o[i0]) / (o[i1] - o[i0]), 0.0, 1.0)
    en_i0 = np.abs(destino - o[i0]) < TOLERANCIA
    en_i1 = (np.abs(destino - o[i1]) < TOLERANCIA) & ~en_i0
    i1 = np.where(en_i0, i0, i1)
    i0 = np.where(en_i1, i1, i0)
    f = np.where(en_i0 | en_i1, 0.0, f)
    if invertido:
        i0, i1 = n - 1 - i0, n - 1 - i1
    return i0, i1, f


def pesos_bilineales(lat_origen, lon_origen, lat_destino, lon_destino):
    return pesos_eje(lat_origen, lat_destino), pesos_eje(lon_origen, lon_destino)


def bilineal(campo: np.ndarray, pesos) -> np.ndarray:
    """Interpola [..., lat, lon] a la grilla destino con los pesos de pesos_bilineales."""
    (i0, i1, fy), (j0, j1, fx) = pesos
    a00 = campo[..., i0[:, None], j0[None, :]]
    a01 = campo[..., i0[:, None], j1[None, :]]
    a10 = campo[..., i1[:, None], j0[None, :]]
    a11 = campo[..., i1[:, None], j1[None, :]]
    fy, fx = fy[:, None], fx[None, :]
    return (1 - fy) * ((1 - fx) * a00 + fx * a01) + fy * ((1 - fx) * a10 + fx * a11)


# --- Tiempo ------------------------------------------------------------------------------------


def _indice(tiempos: np.ndarray) -> dict:
    return {np.datetime64(t, "ns"): k for k, t in enumerate(tiempos)}


def seleccionar_horas(valores: np.ndarray, tiempos: np.ndarray, horas: list[datetime]) -> np.ndarray:
    indice = _indice(tiempos)
    return valores[[indice[np.datetime64(h, "ns")] for h in horas]]


def desacumular(acumulado: np.ndarray, tiempos: np.ndarray, horas: list[datetime]) -> np.ndarray:
    """Valores de cada hora (h−1, h] desde acumulados de ERA5-Land, que empiezan a las 00 UTC.

    01 UTC: el acumulado tal cual. Las demás, incluida la 00 UTC (que trae el total del día
    anterior): el acumulado menos el de la hora previa. Los negativos del empaquetado van a 0.
    """
    indice = _indice(tiempos)
    salida = []
    for h in horas:
        actual = acumulado[indice[np.datetime64(h, "ns")]]
        if h.hour == 1:
            salida.append(actual)
        else:
            salida.append(actual - acumulado[indice[np.datetime64(h - timedelta(hours=1), "ns")]])
    return np.maximum(np.stack(salida), 0.0)


def interpolar_tiempo(valores: np.ndarray, tiempos: np.ndarray, horas: list[datetime]) -> np.ndarray:
    """Interpolación lineal en el tiempo (CAMS: de 3 h a 1 h). No extrapola."""
    t = (tiempos.astype("datetime64[ns]") - tiempos[0]) / np.timedelta64(1, "h")
    x = (np.array(horas, dtype="datetime64[ns]") - tiempos[0]) / np.timedelta64(1, "h")
    if x.min() < t[0] or x.max() > t[-1]:
        raise ValueError(f"horas {horas[0]}…{horas[-1]} fuera de los tiempos disponibles {tiempos[0]}…{tiempos[-1]}")
    k = np.clip(np.searchsorted(t, x, side="right") - 1, 0, len(t) - 2)
    w = ((x - t[k]) / (t[k + 1] - t[k])).reshape(-1, *([1] * (valores.ndim - 1)))
    return (1 - w) * valores[k] + w * valores[k + 1]


# --- Un día ------------------------------------------------------------------------------------


def dia_fisico(d: date, dir_crudo: Path, grilla: Grilla) -> np.ndarray:
    """float32 [24, C, filas, columnas] en unidades físicas (y log1p donde toca), NaN sin dato."""
    t0 = datetime(d.year, d.month, d.day)
    horas = [t0 + timedelta(hours=h) for h in range(24)]
    ventanas = {
        "era5land": (t0 - timedelta(hours=1), horas[-1]),
        "era5": (t0, horas[-1]),
        "cams": (t0, t0 + timedelta(hours=24)),
    }
    salida = np.empty((24, len(CANALES), grilla.filas, grilla.columnas), dtype=np.float32)
    for nombre, (inicio, fin) in ventanas.items():
        ds = leer(dir_crudo, FUENTES[nombre], inicio, fin)
        pesos = pesos_bilineales(ds["latitude"].values, ds["longitude"].values, grilla.lats(), grilla.lons())
        tiempos = ds["time"].values
        for k, c in enumerate(CANALES):
            if c.fuente != nombre:
                continue
            crudo = ds[c.variable].values.astype(np.float64)
            if c.acumulada:
                horario = desacumular(crudo, tiempos, horas)
            elif nombre == "cams":
                horario = interpolar_tiempo(crudo, tiempos, horas)
            else:
                horario = seleccionar_horas(crudo, tiempos, horas)
            x = bilineal(horario, pesos) * c.escala + c.desplazamiento
            if c.log1p:
                x = np.log1p(np.maximum(x, 0.0))
            salida[:, k] = x
    return salida


@dataclass(frozen=True)
class Estadisticas:
    """Conteo, media y suma de cuadrados de desvíos por canal (float64), fusionables en orden (Chan)."""

    n: np.ndarray
    media_: np.ndarray
    m2: np.ndarray

    @staticmethod
    def vacia(canales: int) -> "Estadisticas":
        return Estadisticas(np.zeros(canales), np.zeros(canales), np.zeros(canales))

    @staticmethod
    def de(x: np.ndarray) -> "Estadisticas":
        plano = np.moveaxis(x, 1, 0).reshape(x.shape[1], -1).astype(np.float64)
        validos = ~np.isnan(plano)
        n = validos.sum(axis=1).astype(np.float64)
        media = np.where(n > 0, np.nansum(plano, axis=1) / np.maximum(n, 1), 0.0)
        m2 = np.nansum((plano - media[:, None]) ** 2, axis=1)
        return Estadisticas(n, media, m2)

    def fusionar(self, otra: "Estadisticas") -> "Estadisticas":
        n = self.n + otra.n
        delta = otra.media_ - self.media_
        seguro = np.maximum(n, 1)
        media = np.where(n > 0, self.media_ + delta * otra.n / seguro, 0.0)
        m2 = self.m2 + otra.m2 + delta**2 * self.n * otra.n / seguro
        return Estadisticas(n, media, m2)

    def media(self) -> np.ndarray:
        return self.media_

    def desviacion(self) -> np.ndarray:
        return np.sqrt(self.m2 / np.maximum(self.n, 1))


# Configuración de cada proceso (se fija una vez con el inicializador del pool).
_CONFIG: dict = {}


def _iniciar(config: dict) -> None:
    _CONFIG.update(config)


def _estadisticas_del_dia(d: date) -> Estadisticas:
    return Estadisticas.de(dia_fisico(d, _CONFIG["crudo"], _CONFIG["grilla"]))


def _escribir_dia(d: date) -> tuple[str, str, int]:
    x = dia_fisico(d, _CONFIG["crudo"], _CONFIG["grilla"])
    z = (x - _CONFIG["media"][None, :, None, None]) / _CONFIG["desviacion"][None, :, None, None]
    z = np.nan_to_num(z, nan=0.0).astype(np.float16)
    buffer = io.BytesIO()
    np.save(buffer, z)
    datos = buffer.getvalue()
    relativa = f"dias/{d.isoformat()}.npy"
    escribir_atomico(_CONFIG["salida"] / relativa, datos)
    return relativa, sha256_bytes(datos), len(datos)


def en_paralelo(funcion, dias_: list[date], config: dict, procesos: int):
    """Resultados de funcion(d) en el orden de dias_, con uno o varios procesos."""
    if procesos <= 1:
        _iniciar(config)
        return [funcion(d) for d in dias_]
    with get_context("spawn").Pool(procesos, initializer=_iniciar, initargs=(config,)) as pool:
        return list(pool.imap(funcion, dias_, chunksize=1))


# --- Estáticos ---------------------------------------------------------------------------------


def sumar_poblacion(ruta: Path, grilla: Grilla) -> tuple[np.ndarray, float]:
    """Personas por celda: cada píxel de WorldPop va a la celda de su centro. También el total del raster."""
    with rasterio.open(ruta) as r:
        pob = r.read(1).astype(np.float64)
        t = r.transform
        if r.nodata is not None:
            pob[pob == r.nodata] = 0.0
    pob[~np.isfinite(pob) | (pob < 0)] = 0.0
    filas, columnas = pob.shape
    lat = t.f + t.e * (np.arange(filas) + 0.5)
    lon = t.c + t.a * (np.arange(columnas) + 0.5)
    i = np.floor((grilla.lat_norte - lat) / grilla.paso + 0.5).astype(int)
    j = np.floor((lon - grilla.lon_oeste) / grilla.paso + 0.5).astype(int)
    ii, jj = np.meshgrid(i, j, indexing="ij")
    dentro = (ii >= 0) & (ii < grilla.filas) & (jj >= 0) & (jj < grilla.columnas)
    celdas = np.bincount((ii * grilla.columnas + jj)[dentro], weights=pob[dentro], minlength=grilla.filas * grilla.columnas)
    return celdas.reshape(grilla.filas, grilla.columnas), float(pob.sum())


def mascara_pais(ruta: Path, grilla: Grilla, sub: int = 10) -> np.ndarray:
    """Fracción de cada celda dentro del país (geoBoundaries ADM0), con sub × sub subceldas por celda.

    Fracción y no "centro dentro": con el centro, las celdas de la costa cuyo centro cae en el mar
    (Callao, Chimbote) quedan fuera y con ellas 1.7 millones de personas de WorldPop.
    """
    geo = json.loads(ruta.read_text())
    formas = [(f["geometry"], 1) for f in geo["features"]]
    t = grilla.transformacion()
    fina = rasterio.features.rasterize(
        formas, out_shape=(grilla.filas * sub, grilla.columnas * sub),
        transform=rasterio.Affine(t.a / sub, 0.0, t.c, 0.0, t.e / sub, t.f), fill=0, dtype="uint8",
    )
    return fina.reshape(grilla.filas, sub, grilla.columnas, sub).mean(axis=(1, 3))


def estaticos(dir_crudo: Path, grilla: Grilla, worldpop: Path, adm0: Path) -> tuple[np.ndarray, list[dict], dict]:
    with abrir(dir_crudo / ERA5LAND.nombre / INVARIANTES_ARCHIVO) as ds:
        pesos = pesos_bilineales(ds["latitude"].values, ds["longitude"].values, grilla.lats(), grilla.lons())
        z = bilineal(ds["z"].values.astype(np.float64).reshape(-1, *ds["z"].shape[-2:])[0], pesos)
        lsm = bilineal(ds["lsm"].values.astype(np.float64).reshape(-1, *ds["lsm"].shape[-2:])[0], pesos)
    lat, lon = np.meshgrid(grilla.lats(), grilla.lons(), indexing="ij")
    poblacion, total_raster = sumar_poblacion(worldpop, grilla)
    peru = mascara_pais(adm0, grilla)

    def zscore(x):
        return float(x.mean()), float(x.std()) or 1.0

    def centrado(x):
        return float((x.max() + x.min()) / 2), float((x.max() - x.min()) / 2) or 1.0

    capas = [
        ("orografia", np.nan_to_num(z / GRAVEDAD, nan=0.0), "m", "ninguna", zscore,
         "ERA5-Land invariantes: geopotential / 9.80665; sin dato → 0"),
        ("mascara_tierra", np.nan_to_num(lsm, nan=0.0), "fracción 0–1", "ninguna", lambda x: (0.0, 1.0),
         "ERA5-Land invariantes: land_sea_mask; sin dato → 0"),
        ("mascara_peru", peru, "fracción 0–1", "ninguna", lambda x: (0.0, 1.0),
         "geoBoundaries ADM0: fracción de la celda dentro del país (10 × 10 subceldas)"),
        ("latitud", lat, "°", "ninguna", centrado, "latitud del punto, llevada a [−1, 1]"),
        ("longitud", lon, "°", "ninguna", centrado, "longitud del punto, llevada a [−1, 1]"),
        ("log1p_poblacion", np.log1p(poblacion), "log1p(personas en la celda)", "log1p", zscore,
         "WorldPop 2020 1 km (UN adj): suma de los píxeles cuyo centro cae en la celda"),
    ]
    arreglo, descripcion = [], []
    for nombre, x, unidad, transformacion, normalizacion, origen in capas:
        media, desviacion = normalizacion(x)
        arreglo.append((x - media) / desviacion)
        descripcion.append({"nombre": nombre, "unidad": unidad, "transformacion": transformacion,
                            "media": media, "desviacion": desviacion, "origen": origen})
    control = {"poblacion_en_grilla": float(poblacion.sum()), "poblacion_del_raster": total_raster,
               "poblacion_fuera_de_peru": float(poblacion[peru == 0].sum()),
               "celdas_peru": int((peru > 0).sum()), "area_peru_en_celdas": float(peru.sum())}
    return np.stack(arreglo).astype(np.float32), descripcion, control


# --- Principal ---------------------------------------------------------------------------------


def grilla_de_texto(texto: str) -> Grilla:
    lat_norte, lon_oeste, paso, filas, columnas = texto.split(",")
    return Grilla(float(lat_norte), float(lon_oeste), float(paso), int(filas), int(columnas))


def main(argv=None) -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--crudo", type=Path, default=DIR_MAPA / "crudo")
    ap.add_argument("--salida", type=Path, default=DIR_MAPA)
    ap.add_argument("--desde", type=fecha, default=DESDE)
    ap.add_argument("--hasta", type=fecha, default=HASTA)
    ap.add_argument("--estadisticas-desde", type=fecha, default=ESTADISTICAS_DESDE)
    ap.add_argument("--estadisticas-hasta", type=fecha, default=ESTADISTICAS_HASTA)
    ap.add_argument("--grilla", type=grilla_de_texto, default=GRILLA_NACIONAL,
                    help="lat_norte,lon_oeste,paso,filas,columnas (por defecto la nacional de 192 × 136)")
    ap.add_argument("--procesos", type=int, default=max(1, (os.cpu_count() or 2) - 2))
    ap.add_argument("--worldpop", type=Path, help="por defecto crudo/estaticos/per_ppp_2020_1km_Aggregated_UNadj.tif")
    ap.add_argument("--adm0", type=Path, help="por defecto crudo/estaticos/geoBoundaries-PER-ADM0.geojson")
    args = ap.parse_args(argv)

    grilla = args.grilla
    estaticos_dir = args.crudo / "estaticos"
    worldpop = args.worldpop or estaticos_dir / "per_ppp_2020_1km_Aggregated_UNadj.tif"
    adm0 = args.adm0 or estaticos_dir / "geoBoundaries-PER-ADM0.geojson"
    todos = dias(args.desde, args.hasta)
    para_estadisticas = [d for d in todos if args.estadisticas_desde <= d <= args.estadisticas_hasta]
    if not para_estadisticas:
        print("el periodo de estadísticas no tiene días dentro del periodo a preparar", file=sys.stderr)
        return 2
    config = {"crudo": args.crudo, "grilla": grilla, "salida": args.salida}

    print(f"1/3 estadísticas con {len(para_estadisticas)} días ({para_estadisticas[0]} → {para_estadisticas[-1]}), "
          f"{args.procesos} procesos", flush=True)
    total = Estadisticas.vacia(len(CANALES))
    for parcial in en_paralelo(_estadisticas_del_dia, para_estadisticas, config, args.procesos):
        total = total.fusionar(parcial)
    media = total.media()
    desviacion = np.where(total.desviacion() > 0, total.desviacion(), 1.0)
    celdas_hora = len(para_estadisticas) * 24 * grilla.filas * grilla.columnas

    print(f"2/3 {len(todos)} días ({todos[0]} → {todos[-1]}) a {args.salida / 'dias'}", flush=True)
    config.update(media=media, desviacion=desviacion)
    archivos = {}
    for relativa, sha, tamanio in en_paralelo(_escribir_dia, todos, config, args.procesos):
        archivos[relativa] = {"sha256": sha, "bytes": tamanio}

    print("3/3 estáticos y manifiesto", flush=True)
    arreglo, descripcion_estaticos, control = estaticos(args.crudo, grilla, worldpop, adm0)
    buffer = io.BytesIO()
    np.save(buffer, arreglo)
    escribir_atomico(args.salida / "estaticos.npy", buffer.getvalue())
    archivos["estaticos.npy"] = {"sha256": sha256_bytes(buffer.getvalue()), "bytes": len(buffer.getvalue())}
    etiquetas = args.salida / "etiquetas.csv"
    if etiquetas.exists():
        archivos["etiquetas.csv"] = {"sha256": sha256_archivo(etiquetas), "bytes": etiquetas.stat().st_size}

    fuentes_estaticos = estaticos_dir / "fuentes_estaticos.json"
    fuentes = FUENTES_COPERNICUS + (json.loads(fuentes_estaticos.read_text()) if fuentes_estaticos.exists() else []) + [FUENTE_ETIQUETAS]
    manifiesto = {
        "version": VERSION,
        "creado": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "grilla": {**grilla.a_dict(), "crs": "EPSG:4326",
                   "punto": "fila i, columna j: latitud lat_norte − paso·i, longitud lon_oeste + paso·j"},
        "periodo": {"desde": args.desde.isoformat(), "hasta": args.hasta.isoformat(), "dias": len(todos),
                    "horas": "00–23 UTC; la hora h es el instante h:00 (instantáneas) y el intervalo (h−1, h] "
                             "(precipitación, radiación), como hora_utc de etiquetas.csv"},
        "estadisticas": {"desde": para_estadisticas[0].isoformat(), "hasta": para_estadisticas[-1].isoformat(),
                         "dias": len(para_estadisticas)},
        "dias": {"ruta": "dias/AAAA-MM-DD.npy", "dtype": "float16",
                 "forma": [24, len(CANALES), grilla.filas, grilla.columnas],
                 "normalizacion": "z = (x − media) / desviacion, con x en la unidad del canal y después de su transformación",
                 "sin_dato": 0.0,
                 "sin_dato_nota": "las celdas sin dato (mar en ERA5-Land) valen 0, la media del canal; ver mascara_tierra"},
        "canales": [
            {"nombre": c.nombre, "fuente": FUENTES[c.fuente].dataset, "variable": c.variable, "unidad": c.unidad,
             "transformacion": "log1p" if c.log1p else "ninguna",
             "desacumulada": c.acumulada, "media": float(m), "desviacion": float(s),
             "fraccion_sin_dato": round(1 - float(n) / celdas_hora, 6)}
            for c, m, s, n in zip(CANALES, media, desviacion, total.n)
        ],
        "estaticos": descripcion_estaticos,
        "estaticos_archivo": {"ruta": "estaticos.npy", "dtype": "float32", "forma": list(arreglo.shape),
                              "normalizacion": "valor = (x − media) / desviacion, con x después de su transformación",
                              "control": control},
        "etiquetas": {"ruta": "etiquetas.csv", "columnas": ["hora_utc", "estacion", "fuente", "i", "j", "pm2_5", "pm10"]},
        "archivos": archivos,
        "fuentes": fuentes,
        "licencias": {f.get("nombre") or f"{f['fuente']} {f.get('nivel', '')}".strip(): f["licencia"] for f in fuentes},
    }
    escribir_json(args.salida / MANIFIESTO, manifiesto)
    total_mb = sum(a["bytes"] for k, a in archivos.items() if k.startswith("dias/")) / 1e6
    print(f"listo: {len(todos)} días ({total_mb:.1f} MB), estáticos {list(arreglo.shape)}, "
          f"población en la grilla {control['poblacion_en_grilla']:,.0f} de {control['poblacion_del_raster']:,.0f}, "
          f"{control['celdas_peru']} celdas en el Perú → {args.salida / MANIFIESTO}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
