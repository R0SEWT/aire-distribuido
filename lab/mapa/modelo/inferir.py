"""Inferencia nacional (§6): el promedio diario por celda de PM2.5 y PM10, con la desviación entre modelos.

Corre en los nodos del laboratorio: solo importa numpy, torch y la biblioteca estándar.

    # En la laptop, con la carpeta de datos (deja los resultados en --salida):
    uv run python inferir.py --datos data/mapa --pesos-json pesos/pesos.json \
        --pesos pesos/pesos_p0.bin pesos/pesos_p1.bin … --dias 2018-01-01:2018-01-31 --salida data/mapa/resultados

    # En un nodo: baja los días del mismo origen que el manifiesto y sube cada día con PUT a /resultados/
    python inferir.py --datos http://10.11.37.73:8090/mapa/manifiesto.json --pesos-json http://…/pesos.json \
        --pesos http://…/pesos_p0.bin … --parte 3/20 --subir http://10.11.37.73:8090/

Un día da un archivo mapa_AAAA-MM-DD.npy (formato .npy de numpy), float16 [2, 2, filas, columnas]:
[contaminante (pm2_5, pm10), (media, desviación entre modelos)], en µg/m³. La media diaria es la de las
24 horas UTC del día (expm1 de la salida en log1p, recortada en 0); con un solo modelo la desviación es 0.
El primer día del periodo no tiene día anterior: su ventana repite la hora 00 (datos.Serie.ventana).

--parte k/N toma la parte k (desde 0) del reparto en bloques de 7 días entre N (datos.repartir_bloques).
"""

from __future__ import annotations

import argparse
import io
import json
import sys
import time
import urllib.parse
import urllib.request
from pathlib import Path

import numpy as np
import torch

import datos
import red
from datos import HORAS


def obtener(ruta: str, cache: Path) -> Path:
    """Una ruta local o una URL bajada a la cache."""
    if not ruta.startswith(("http://", "https://")):
        return Path(ruta)
    cache.mkdir(parents=True, exist_ok=True)
    destino = cache / Path(urllib.parse.urlparse(ruta).path).name
    with urllib.request.urlopen(ruta, timeout=120) as r:
        destino.write_bytes(r.read())
    return destino


def cargar_modelos(rutas_bin: list[Path], ruta_json: Path, dev: torch.device) -> list[torch.nn.Module]:
    modelos = [red.cargar_modelo(r, ruta_json, dev) for r in rutas_bin]
    for m in modelos:
        m.eval()
    return modelos


@torch.no_grad()
def promedio_diario(modelos: list[torch.nn.Module], serie: datos.Serie, estaticos: torch.Tensor, dia: str,
                    lote: int = 8, amp: bool = False) -> tuple[np.ndarray, np.ndarray]:
    """(media, desviación) entre modelos del promedio diario en µg/m³, cada uno [2, filas, columnas] float32."""
    diarios = []
    for modelo in modelos:
        suma = torch.zeros(2, serie.filas, serie.columnas, dtype=torch.float64, device=estaticos.device)
        for k in range(0, HORAS, lote):
            horas = [(dia, h) for h in range(k, min(k + lote, HORAS))]
            x = datos.armar_entrada(serie, estaticos, horas, rellenar=True)
            with torch.autocast("cuda", dtype=torch.float16, enabled=amp):
                y = modelo(x).float()
            suma += torch.expm1(y.clamp_min(0)).sum(0, dtype=torch.float64)
        diarios.append((suma / HORAS).cpu().numpy())
    pila = np.stack(diarios)
    return pila.mean(0).astype(np.float32), pila.std(0).astype(np.float32)


def empaquetar(media: np.ndarray, desv: np.ndarray) -> bytes:
    """[2 contaminantes, (media, desviación), filas, columnas] en float16, como .npy."""
    arr = np.stack([media, desv], axis=1).astype(np.float16)
    b = io.BytesIO()
    np.save(b, arr)
    return b.getvalue()


def subir(base: str, nombre: str, contenido: bytes) -> None:
    url = urllib.parse.urljoin(base if base.endswith("/") else base + "/", f"resultados/{nombre}")
    req = urllib.request.Request(url, data=contenido, method="PUT",
                                 headers={"Content-Type": "application/octet-stream"})
    with urllib.request.urlopen(req, timeout=120) as r:
        if r.status not in (200, 201, 204):
            raise OSError(f"PUT {url}: {r.status}")


def elegir_dias(todos: list[str], rango: str | None, parte: str | None, semilla: int = 0) -> list[str]:
    dias = todos
    if rango:
        desde, hasta = rango.split(":")
        dias = [d for d in dias if (not desde or d >= desde) and (not hasta or d <= hasta)]
    if parte:
        k, n = (int(x) for x in parte.split("/"))
        dias = datos.repartir_bloques(dias, n, semilla)[k]
    return dias


def inferir(origen: datos.Origen, modelos: list[torch.nn.Module], dias: list[str], dev: torch.device,
            entregar, lote: int = 8, amp: bool = False, avisar=print) -> None:
    """Predice cada día en orden; tiene en memoria solo el día y el anterior. entregar(dia, bytes)."""
    man = origen.manifiesto
    estaticos = torch.as_tensor(datos.cargar_estaticos(origen.ruta("estaticos.npy")), device=dev)
    hay = set(man.dias)
    cargados: dict[str, torch.Tensor] = {}
    t0 = time.perf_counter()
    for n, dia in enumerate(dias, 1):
        necesarios = [d for d in (datos.dia_anterior(dia), dia) if d in hay]
        cargados = {d: cargados[d] if d in cargados else
                    torch.from_numpy(np.load(origen.ruta(datos.ruta_dia(d)))).to(dev) for d in necesarios}
        media, desv = promedio_diario(modelos, datos.Serie(cargados), estaticos, dia, lote, amp)
        entregar(dia, empaquetar(media, desv))
        if n <= 3 or n % 20 == 0 or n == len(dias):
            avisar(f"{n}/{len(dias)} {dia}: PM2.5 medio {media[0].mean():.1f}, PM10 {media[1].mean():.1f} µg/m³ "
                   f"({(time.perf_counter() - t0) / n:.1f} s por día)")


def main() -> int:
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("--datos", required=True, help="carpeta de datos o URL del manifiesto.json")
    p.add_argument("--pesos-json", required=True, help="pesos.json (ruta o URL)")
    p.add_argument("--pesos", nargs="+", required=True, help="uno o más vectores .bin (uno por partición)")
    p.add_argument("--dias", default=None, help="AAAA-MM-DD:AAAA-MM-DD")
    p.add_argument("--parte", default=None, help="k/N: la parte k del reparto en bloques de 7 días")
    p.add_argument("--semilla", type=int, default=0, help="semilla del reparto en bloques")
    p.add_argument("--salida", default=None, help="carpeta donde dejar mapa_AAAA-MM-DD.npy")
    p.add_argument("--subir", default=None, help="URL base del servidor: PUT a <base>/resultados/mapa_AAAA-MM-DD.npy")
    p.add_argument("--cache", type=Path, default=Path.home() / "aire-mapa" / "cache")
    p.add_argument("--lote", type=int, default=8)
    p.add_argument("--cpu", action="store_true")
    a = p.parse_args()
    if not (a.salida or a.subir):
        p.error("hace falta --salida o --subir")

    dev = torch.device("cuda" if torch.cuda.is_available() and not a.cpu else "cpu")
    origen = datos.Origen(a.datos, a.cache)
    ruta_json = obtener(a.pesos_json, a.cache)
    modelos = cargar_modelos([obtener(r, a.cache) for r in a.pesos], ruta_json, dev)
    entrada = json.loads(ruta_json.read_text(encoding="utf-8"))["arquitectura"]["entrada"]
    if entrada != origen.manifiesto.n_entrada:
        print(f"los pesos esperan {entrada} canales y el manifiesto da {origen.manifiesto.n_entrada}", file=sys.stderr)
        return 1
    dias = elegir_dias(origen.manifiesto.dias, a.dias, a.parte, a.semilla)

    def entregar(dia: str, contenido: bytes) -> None:
        nombre = f"mapa_{dia}.npy"
        if a.salida:
            Path(a.salida).mkdir(parents=True, exist_ok=True)
            (Path(a.salida) / nombre).write_bytes(contenido)
        if a.subir:
            subir(a.subir, nombre, contenido)

    print(f"{len(dias)} días con {len(modelos)} modelo(s) en {dev}", flush=True)
    inferir(origen, modelos, dias, dev, entregar, a.lote, amp=dev.type == "cuda",
            avisar=lambda t: print(t, flush=True))
    return 0


if __name__ == "__main__":
    sys.exit(main())
