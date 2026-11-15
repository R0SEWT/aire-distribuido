"""Las fuentes de Copernicus y cómo se leen sus netCDF crudos.

Los nombres de variable de los pedidos son los de los formularios de los catálogos, verificados el
14/11/2026 contra ``/api/catalogue/v1/collections/<id>`` (form y constraints) de CDS y ADS. Los nombres
dentro del netCDF son los cortos de ECMWF (t2m, tp, pm2p5…): ``validar`` falla con la lista de lo que
sí trae el archivo si alguno no coincide.

Archivos: ``crudo/<fuente>/AAAA-MM.nc``, un mes calendario por archivo con solo los días pedidos, y
``crudo/era5land/invariantes.nc``.
"""

import math
from contextlib import contextmanager
from dataclasses import dataclass
from datetime import date, datetime, timedelta
from pathlib import Path

import numpy as np
import pandas as pd
import xarray as xr

from grilla import Grilla


@dataclass(frozen=True)
class Fuente:
    nombre: str  # carpeta en crudo/
    servicio: str  # "cds" o "ads"
    dataset: str
    variables: tuple[str, ...]  # nombres del formulario del catálogo
    nombres_netcdf: tuple[str, ...]  # los mismos, como vienen en el netCDF
    paso_horas: int
    resolucion: float  # grados de la grilla nativa
    margen: float  # grados de más alrededor de la grilla común, para no extrapolar
    dias_antes: int  # días que se piden antes del periodo
    dias_despues: int  # y después


ERA5LAND = Fuente(
    nombre="era5land",
    servicio="cds",
    dataset="reanalysis-era5-land",
    variables=(
        "2m_temperature",
        "2m_dewpoint_temperature",
        "10m_u_component_of_wind",
        "10m_v_component_of_wind",
        "surface_pressure",
        "total_precipitation",
        "surface_solar_radiation_downwards",
    ),
    nombres_netcdf=("t2m", "d2m", "u10", "v10", "sp", "tp", "ssrd"),
    paso_horas=1,
    resolucion=0.1,
    margen=0.0,
    # La precipitación y la radiación de la 00 UTC se desacumulan con la 23 UTC del día anterior.
    dias_antes=1,
    dias_despues=0,
)

ERA5 = Fuente(
    nombre="era5",
    servicio="cds",
    dataset="reanalysis-era5-single-levels",
    variables=("boundary_layer_height", "total_cloud_cover"),
    nombres_netcdf=("blh", "tcc"),
    paso_horas=1,
    resolucion=0.25,
    margen=0.25,
    dias_antes=0,
    dias_despues=0,
)

CAMS = Fuente(
    nombre="cams",
    servicio="ads",
    dataset="cams-global-reanalysis-eac4",
    variables=(
        "particulate_matter_2.5um",
        "particulate_matter_10um",
        "total_aerosol_optical_depth_550nm",
        "dust_aerosol_optical_depth_550nm",
    ),
    nombres_netcdf=("pm2p5", "pm10", "aod550", "duaod550"),
    paso_horas=3,
    resolucion=0.75,
    margen=0.75,
    dias_antes=0,
    # Las 22 y 23 UTC del último día se interpolan con la 00 UTC del día siguiente.
    dias_despues=1,
)

FUENTES = {f.nombre: f for f in (ERA5LAND, ERA5, CAMS)}

# Invariantes de ERA5-Land (grupo "Invariant" del formulario; sin fecha, solo en netCDF o GRIB).
INVARIANTES_VARIABLES = ("geopotential", "land_sea_mask")
INVARIANTES_NETCDF = ("z", "lsm")
INVARIANTES_ARCHIVO = "invariantes.nc"


def area(fuente: Fuente, grilla: Grilla) -> list[float]:
    """[N, O, S, E] del pedido: la grilla más el margen, llevada hacia afuera a la grilla nativa.

    Alineada a la grilla nativa, el servidor recorta puntos nativos sin interpolar.
    """
    lats, lons = grilla.lats(), grilla.lons()
    r, m, eps = fuente.resolucion, fuente.margen, 1e-9

    def afuera(x, hacia_arriba):
        k = math.ceil(x / r - eps) if hacia_arriba else math.floor(x / r + eps)
        return round(k * r, 6)

    return [
        afuera(lats.max() + m, True),
        afuera(lons.min() - m, False),
        afuera(lats.min() - m, False),
        afuera(lons.max() + m, True),
    ]


def ruta_mes(dir_crudo: Path, fuente: Fuente, anio: int, mes: int) -> Path:
    return dir_crudo / fuente.nombre / f"{anio:04d}-{mes:02d}.nc"


def horas_esperadas(fuente: Fuente, dias: list[date]) -> list[datetime]:
    return [
        datetime(d.year, d.month, d.day) + timedelta(hours=h)
        for d in dias
        for h in range(0, 24, fuente.paso_horas)
    ]


def normalizar(ds: xr.Dataset) -> xr.Dataset:
    """Lleva un netCDF de CDS o ADS a dimensiones (time, latitude, longitude), longitudes en [−180, 180).

    El netCDF nuevo de CDS usa valid_time; los campos de pronóstico traen (forecast_reference_time,
    forecast_period) y se apilan a su valid_time. Las dimensiones de tamaño 1 (number, surface) y las
    coordenadas sobrantes (expver) se quitan.
    """
    if "forecast_period" in ds.dims and "forecast_reference_time" in ds.dims:
        valido = ds["valid_time"].transpose("forecast_reference_time", "forecast_period").values.ravel()
        ds = ds.drop_vars("valid_time").stack(_t=("forecast_reference_time", "forecast_period"))
        ds = ds.drop_vars(["_t", "forecast_reference_time", "forecast_period"]).rename(_t="time")
        ds = ds.assign_coords(time=valido)
    elif "valid_time" in ds.dims:
        ds = ds.rename(valid_time="time")
    ds = ds.rename({k: v for k, v in {"lat": "latitude", "lon": "longitude"}.items() if k in ds.dims})
    for d in list(ds.dims):
        if d not in ("time", "latitude", "longitude") and ds.sizes[d] == 1:
            ds = ds.isel({d: 0}, drop=True)
    ds = ds.drop_vars([c for c in ds.coords if c not in ("time", "latitude", "longitude")])
    if (ds["longitude"] >= 180).any():
        ds = ds.assign_coords(longitude=((ds["longitude"] + 180) % 360) - 180).sortby("longitude")
    if "time" in ds.dims:
        ds = ds.transpose("time", "latitude", "longitude", ...).sortby("time")
        _, unicos = np.unique(ds["time"].values, return_index=True)
        ds = ds.isel(time=np.sort(unicos))
    return ds


@contextmanager
def abrir(ruta: Path):
    with xr.open_dataset(ruta, engine="netcdf4") as ds:
        yield normalizar(ds)


def validar(ruta: Path, nombres: tuple[str, ...], horas: list[datetime] | None, grilla: Grilla) -> str | None:
    """None si el archivo abre, trae las variables y horas pedidas y cubre la grilla; si no, el motivo."""
    try:
        with abrir(ruta) as ds:
            faltan = [n for n in nombres if n not in ds.data_vars]
            if faltan:
                return f"faltan variables {faltan}; trae {sorted(ds.data_vars)}"
            if horas is not None:
                tiene = set(pd.DatetimeIndex(ds["time"].values).to_pydatetime())
                sin = [h for h in horas if h not in tiene]
                if sin:
                    return f"faltan {len(sin)} de {len(horas)} horas (la primera, {sin[0]:%Y-%m-%d %H}h)"
            lat, lon = ds["latitude"].values, ds["longitude"].values
            tol = 1e-4
            if lat.max() < grilla.lats().max() - tol or lat.min() > grilla.lats().min() + tol:
                return f"latitudes {lat.min()}…{lat.max()} no cubren la grilla"
            if lon.min() > grilla.lons().min() + tol or lon.max() < grilla.lons().max() - tol:
                return f"longitudes {lon.min()}…{lon.max()} no cubren la grilla"
    except Exception as e:  # noqa: BLE001 — cualquier archivo que no abre es inválido
        return f"no abre: {e}"
    return None


def leer(dir_crudo: Path, fuente: Fuente, inicio: datetime, fin: datetime) -> xr.Dataset:
    """Las horas de inicio a fin (incluidas) de una fuente, juntando los archivos mensuales necesarios."""
    meses = sorted({(d.year, d.month) for d in pd.date_range(inicio.date(), fin.date(), freq="D")})
    partes = []
    for anio, mes in meses:
        ruta = ruta_mes(dir_crudo, fuente, anio, mes)
        if not ruta.exists():
            raise FileNotFoundError(f"falta {ruta}: corre descargar_copernicus.py")
        with abrir(ruta) as ds:
            partes.append(ds[list(fuente.nombres_netcdf)].sel(time=slice(inicio, fin)).load())
    ds = xr.concat(partes, dim="time") if len(partes) > 1 else partes[0]
    return normalizar(ds)
