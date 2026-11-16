"""Baja ERA5-Land, ERA5 y CAMS EAC4 por meses para el mapa nacional (spec §3.2).

    uv run python descargar_copernicus.py --simular     # lista los pedidos y su estado; no baja nada
    uv run python descargar_copernicus.py               # baja lo que falte (la cola de Copernicus tarda horas)
    uv run python descargar_copernicus.py --fuentes cams --desde 2018-01-01 --hasta 2018-03-31 --hilos 2

Un pedido por fuente y mes calendario a ``data/mapa/crudo/{era5land,era5,cams}/AAAA-MM.nc``, más las
invariantes de ERA5-Land (geopotencial y máscara de tierra) en ``crudo/era5land/invariantes.nc``.
Periodo por defecto: 2017-12-31 → 2022-12-31; ERA5-Land pide además el día anterior (para desacumular
la 00 UTC del primer día) y CAMS el siguiente (para interpolar las 22 y 23 UTC del último).

Es idempotente: salta los archivos que ya están y validan (abren, traen las variables y todas las
horas pedidas, cubren la grilla). Cada descarga se escribe a un temporal y se renombra al validar.

Credenciales: ``~/.cdsapirc`` para CDS y ``~/.adsapirc`` para ADS (el archivo que usarías con
``CDSAPI_RC=~/.adsapirc``), cada uno con las líneas ``url:`` y ``key:``. Se leen acá y se pasan al
cliente para que los dos servicios convivan en el mismo proceso; la clave nunca se imprime. Antes de
bajar hay que aceptar la licencia de cada dataset en su página.
"""

import argparse
import json
import os
import shutil
import sys
import tempfile
import zipfile
from collections import defaultdict
from concurrent.futures import ThreadPoolExecutor, as_completed
from dataclasses import dataclass
from datetime import date, datetime, timedelta
from pathlib import Path

import cdsapi
import cdsapi.api
import xarray as xr

from comun import DESDE, DIR_MAPA, HASTA, dias, fecha
from crudo import (
    ERA5LAND,
    FUENTES,
    INVARIANTES_ARCHIVO,
    INVARIANTES_NETCDF,
    INVARIANTES_VARIABLES,
    Fuente,
    area,
    horas_esperadas,
    normalizar,
    ruta_mes,
    validar,
)
from grilla import GRILLA_NACIONAL, Grilla

SERVICIOS = {
    "cds": {
        "nombre": "CDS",
        "url": "https://cds.climate.copernicus.eu/api",
        "rc": "~/.cdsapirc",
        "perfil": "https://cds.climate.copernicus.eu/profile",
    },
    "ads": {
        "nombre": "ADS",
        "url": "https://ads.atmosphere.copernicus.eu/api",
        "rc": "~/.adsapirc",
        "perfil": "https://ads.atmosphere.copernicus.eu/profile",
    },
}


class SinCredenciales(Exception):
    pass


@dataclass
class Pedido:
    fuente: str
    servicio: str
    dataset: str
    solicitud: dict
    destino: Path
    nombres: tuple[str, ...]  # variables que tiene que traer el netCDF
    horas: list[datetime] | None  # None para las invariantes

    def describir(self) -> str:
        if self.horas is None:
            return f"{self.dataset} invariantes"
        return f"{self.dataset} {self.destino.stem} ({len(self.horas)} horas)"


def solicitud_mes(fuente: Fuente, dias_mes: list[date], grilla: Grilla) -> dict:
    horas = [f"{h:02d}:00" for h in range(0, 24, fuente.paso_horas)]
    anio, mes = dias_mes[0].year, dias_mes[0].month
    if fuente.servicio == "ads":
        return {
            "variable": list(fuente.variables),
            "date": [f"{dias_mes[0]}/{dias_mes[-1]}"],
            "time": horas,
            "data_format": "netcdf_zip",
            "area": area(fuente, grilla),
        }
    solicitud = {
        "variable": list(fuente.variables),
        "year": f"{anio}",
        "month": f"{mes:02d}",
        "day": [f"{d.day:02d}" for d in dias_mes],
        "time": horas,
        "data_format": "netcdf",
        "download_format": "unarchived",
        "area": area(fuente, grilla),
    }
    if fuente.dataset == "reanalysis-era5-single-levels":
        # En este formulario año y mes son listas y hay que elegir el producto.
        solicitud = {"product_type": ["reanalysis"], **solicitud, "year": [f"{anio}"], "month": [f"{mes:02d}"]}
    return solicitud


def armar_pedidos(desde: date, hasta: date, dir_crudo: Path, grilla: Grilla, fuentes: list[str]) -> list[Pedido]:
    pedidos = []
    for nombre in fuentes:
        fuente = FUENTES[nombre]
        todos = dias(desde - timedelta(days=fuente.dias_antes), hasta + timedelta(days=fuente.dias_despues))
        por_mes = defaultdict(list)
        for d in todos:
            por_mes[(d.year, d.month)].append(d)
        for (anio, mes), dias_mes in sorted(por_mes.items()):
            pedidos.append(Pedido(
                fuente=nombre,
                servicio=fuente.servicio,
                dataset=fuente.dataset,
                solicitud=solicitud_mes(fuente, dias_mes, grilla),
                destino=ruta_mes(dir_crudo, fuente, anio, mes),
                nombres=fuente.nombres_netcdf,
                horas=horas_esperadas(fuente, dias_mes),
            ))
        if fuente is ERA5LAND:
            pedidos.append(Pedido(
                fuente=nombre,
                servicio=fuente.servicio,
                dataset=fuente.dataset,
                solicitud={
                    "variable": list(INVARIANTES_VARIABLES),
                    "data_format": "netcdf",
                    "download_format": "unarchived",
                    "area": area(fuente, grilla),
                },
                destino=dir_crudo / fuente.nombre / INVARIANTES_ARCHIVO,
                nombres=INVARIANTES_NETCDF,
                horas=None,
            ))
    return pedidos


def ya_bajado(pedido: Pedido, grilla: Grilla) -> bool:
    return pedido.destino.exists() and validar(pedido.destino, pedido.nombres, pedido.horas, grilla) is None


def credenciales(servicio: str, ruta: Path) -> tuple[str, str]:
    """url y clave del archivo rc; SinCredenciales con lo que hay que crear si falta o no sirve."""
    s = SERVICIOS[servicio]
    instrucciones = (
        f"Faltan las credenciales de {s['nombre']}: crea {ruta} con estas dos líneas\n"
        f"    url: {s['url']}\n"
        f"    key: <tu token personal, en {s['perfil']}>\n"
        f"  y acepta la licencia de cada dataset en su página antes de bajar."
    )
    if not ruta.exists():
        raise SinCredenciales(instrucciones)
    config = cdsapi.api.read_config(ruta)
    url, clave = config.get("url", ""), config.get("key", "")
    if not url or not clave:
        raise SinCredenciales(f"{ruta} no tiene las líneas url: y key:.\n  {instrucciones}")
    if url.rstrip("/") != s["url"]:
        raise SinCredenciales(f"{ruta} apunta a {url}, no a {s['nombre']} ({s['url']}).")
    return url, clave


def combinar_zip(zip_ruta: Path, salida: Path) -> None:
    """Junta los netCDF de un zip (CDS separa instantáneas y acumuladas; ADS entrega netcdf_zip)."""
    with tempfile.TemporaryDirectory(dir=salida.parent, prefix=".zip-") as tmp:
        with zipfile.ZipFile(zip_ruta) as z:
            nombres = [n for n in z.namelist() if n.endswith(".nc")]
            if not nombres:
                raise RuntimeError(f"el zip no trae netCDF: {z.namelist()}")
            for n in nombres:
                # Se extrae cada miembro a un nombre propio, sin usar la ruta que trae el zip.
                with z.open(n) as origen, open(Path(tmp) / f"{len(list(Path(tmp).iterdir()))}.nc", "wb") as destino:
                    shutil.copyfileobj(origen, destino)
        partes = []
        for ruta in sorted(Path(tmp).iterdir()):
            with xr.open_dataset(ruta, engine="netcdf4") as ds:
                partes.append(normalizar(ds).load())
        combinado = xr.merge(partes, compat="no_conflicts", join="outer", combine_attrs="drop_conflicts")
        codificacion = {v: {"zlib": True, "complevel": 1} for v in combinado.data_vars}
        combinado.to_netcdf(salida, engine="netcdf4", encoding=codificacion)


def bajar(pedido: Pedido, url: str, clave: str, grilla: Grilla) -> str:
    pedido.destino.parent.mkdir(parents=True, exist_ok=True)
    cliente = cdsapi.Client(url=url, key=clave, quiet=True, progress=False)
    with tempfile.TemporaryDirectory(dir=pedido.destino.parent, prefix=".bajando-") as tmp:
        descarga = Path(tmp) / "descarga"
        cliente.retrieve(pedido.dataset, pedido.solicitud, str(descarga))
        listo = Path(tmp) / "listo.nc"
        if zipfile.is_zipfile(descarga):
            combinar_zip(descarga, listo)
        else:
            descarga.rename(listo)
        motivo = validar(listo, pedido.nombres, pedido.horas, grilla)
        if motivo:
            raise RuntimeError(f"lo bajado no valida: {motivo}")
        tamanio = listo.stat().st_size
        os.replace(listo, pedido.destino)
    return f"{tamanio / 1e6:.1f} MB"


def tamanio_estimado(pedido: Pedido, grilla: Grilla) -> float:
    """MB sin comprimir en float32: variables × horas × puntos del área."""
    fuente = FUENTES[pedido.fuente]
    n, o, s, e = area(fuente, grilla)
    puntos = (round((n - s) / fuente.resolucion) + 1) * (round((e - o) / fuente.resolucion) + 1)
    horas = len(pedido.horas) if pedido.horas else 1
    return len(pedido.nombres) * horas * puntos * 4 / 1e6


def main(argv=None) -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--desde", type=fecha, default=DESDE, help=f"primer día del periodo (por defecto {DESDE})")
    ap.add_argument("--hasta", type=fecha, default=HASTA, help=f"último día del periodo (por defecto {HASTA})")
    ap.add_argument("--fuentes", default="era5land,era5,cams", help="subconjunto de era5land,era5,cams")
    ap.add_argument("--crudo", type=Path, default=DIR_MAPA / "crudo")
    ap.add_argument("--hilos", type=int, default=3, help="pedidos a la vez por servicio (por defecto 3)")
    ap.add_argument("--simular", action="store_true", help="lista los pedidos sin bajar nada")
    ap.add_argument("--rc-cds", type=Path, default=Path(SERVICIOS["cds"]["rc"]).expanduser())
    ap.add_argument("--rc-ads", type=Path, default=Path(SERVICIOS["ads"]["rc"]).expanduser())
    args = ap.parse_args(argv)

    fuentes = [f.strip() for f in args.fuentes.split(",") if f.strip()]
    desconocidas = [f for f in fuentes if f not in FUENTES]
    if desconocidas:
        ap.error(f"fuentes desconocidas: {desconocidas}")
    grilla = GRILLA_NACIONAL
    pedidos = armar_pedidos(args.desde, args.hasta, args.crudo, grilla, fuentes)
    rc = {"cds": args.rc_cds, "ads": args.rc_ads}

    if args.simular:
        total = defaultdict(float)
        for p in pedidos:
            estado = "ya está" if ya_bajado(p, grilla) else "falta"
            total[p.fuente] += tamanio_estimado(p, grilla)
            print(f"[{estado:7}] {p.describir()} → {p.destino}")
            print(f"          {json.dumps(p.solicitud)}")
        print(f"\n{len(pedidos)} pedidos. Tamaño sin comprimir estimado (float32):")
        for f, mb in total.items():
            print(f"  {f:9} {mb / 1e3:7.1f} GB")
        for servicio in sorted({p.servicio for p in pedidos}):
            hay = rc[servicio].exists()
            print(f"Credenciales de {SERVICIOS[servicio]['nombre']} ({rc[servicio]}): {'están' if hay else 'faltan'}")
        return 0

    servicios = sorted({p.servicio for p in pedidos})
    claves, errores = {}, []
    for s in servicios:
        try:
            claves[s] = credenciales(s, rc[s])
        except SinCredenciales as e:
            errores.append(str(e))
    if errores:
        print("\n\n".join(errores), file=sys.stderr)
        return 2

    pendientes = [p for p in pedidos if not ya_bajado(p, grilla)]
    print(f"{len(pedidos)} pedidos, {len(pedidos) - len(pendientes)} ya bajados, {len(pendientes)} por bajar")
    fallidos = []
    ejecutores = {s: ThreadPoolExecutor(max_workers=args.hilos) for s in servicios}
    try:
        futuros = {
            ejecutores[p.servicio].submit(bajar, p, *claves[p.servicio], grilla): p for p in pendientes
        }
        for futuro in as_completed(futuros):
            p = futuros[futuro]
            try:
                print(f"listo   {p.describir()}: {futuro.result()}", flush=True)
            except Exception as e:  # noqa: BLE001 — se reporta y se sigue con los demás
                fallidos.append(p)
                print(f"FALLÓ   {p.describir()}: {e}", file=sys.stderr, flush=True)
    finally:
        for ex in ejecutores.values():
            ex.shutdown(wait=True)
    if fallidos:
        print(f"{len(fallidos)} pedidos fallaron; vuelve a correr el script para reintentarlos.", file=sys.stderr)
        return 1
    print("todo bajado")
    return 0


if __name__ == "__main__":
    sys.exit(main())
