"""Nodo del entrenamiento distribuido del mapa (local SGD), protocolo v2 (§5.3 de la spec).

    python nodo_mapa.py --coordinador 10.11.37.73:7071
    python nodo_mapa.py --coordinador 127.0.0.1:7071 --cpu        # para probar sin GPU

Solo importa numpy, torch y la biblioteca estándar (más datos.py y red.py, que tampoco importan otra
cosa): corre con el Python portable de lab/gpu/armar-paquete.sh. El nodo llama al coordinador
(cmd/mapa-coord), como lab/gpu/nodo_gpu.py llama al de la PC4, y vuelve a llamar si se corta.

# Cable

Cada mensaje es un objeto JSON en una línea terminada en '\\n' (como la v1). Si el JSON trae
"bytes": N, le siguen exactamente N bytes crudos después del '\\n': un vector float32 little-endian.
La línea no pasa de MAX_LINEA (1 MB) y la carga de MAX_CARGA (256 MB).

    nodo → coord  {"tipo":"HOLA","version":2,"nodo":"H45-022-gpu","workers":1,"gpu":"NVIDIA T1000 8GB","vram_gb":8}
    coord → nodo  {"tipo":"TAREA","tarea":…,"manifiesto":"http://…/manifiesto.json","huella":"sha256:…",
                   "dias":["2018-01-08",…],"validacion":["CA-SB-01",…],"hiper":{"lote":4,"tasa":0.001,…}}
    nodo → coord  {"tipo":"LISTO","tarea":…,"muestras":812}
    coord → nodo  {"tipo":"PESOS","ronda":1,"pasos":200,"n":2040322,"bytes":8161288}  + 8161288 bytes
    nodo → coord  {"tipo":"LOCAL","ronda":1,"n":2040322,"bytes":8161288,"muestras":800,"perdida":0.41,
                   "perdida_val":0.52,"ms_computo":61234}  + 8161288 bytes
    (en CPU, "gpu":"" y "vram_gb":0)
    PING → PONG; FIN libera la tarea y el nodo espera otra TAREA; ERROR, en cualquier sentido, con "mensaje".

# Qué hace

- TAREA: baja el manifiesto, comprueba que su sha256 es la huella (con o sin "sha256:"), baja del mismo
  origen los días asignados y el día anterior a cada bloque (la ventana de 24 h), estaticos.npy y
  etiquetas.csv, verifica cada uno contra el manifiesto (con una cache en disco por sha256), los deja en
  la GPU si caben (si no, en RAM), arma la red con hiper["ancho"] y un AdamW nuevo. LISTO dice cuántas
  horas con etiqueta de entrenamiento tiene ("muestras").
- PESOS: copia el vector en la red, da `pasos` pasos de AdamW (el optimizador conserva su estado entre
  rondas: solo viajan los pesos) sobre sus horas, mide la pérdida de validación en las estaciones
  excluidas (hasta hiper["muestras_val"] horas fijas) y devuelve LOCAL con los pesos nuevos.
  "muestras" es pasos × lote (lo que pondera el promedio); ms_computo va desde que terminó de leer la
  carga hasta antes de escribir la respuesta.

hiper (lo pasa el coordinador tal cual, ver hiper.json): lote, tasa, decaimiento, semilla, ancho,
muestras_val, amp. El vector tiene que tener los n parámetros de la red que arma el nodo (pesos.json).
"""

from __future__ import annotations

import argparse
import json
import math
import platform
import socket
import sys
import time
import urllib.error
import zlib
from pathlib import Path

import numpy as np
import torch

import datos
import red

VERSION = 2
MAX_LINEA = 1 << 20
MAX_CARGA = 256 << 20
HIPER = {"lote": 4, "tasa": 1e-3, "decaimiento": 1e-4, "semilla": 0, "ancho": 32, "muestras_val": 32, "amp": True}


def decir(nodo: str, texto: str) -> None:
    print(f"[{nodo}] {time.strftime('%H:%M:%S')} {texto}", flush=True)


class Conexion:
    """Mensajes JSON de una línea, con carga binaria opcional, sobre un socket."""

    def __init__(self, sock: socket.socket):
        self.sock = sock
        self.lector = sock.makefile("rb")

    def cerrar(self) -> None:
        self.lector.close()
        self.sock.close()

    def escribir(self, m: dict, carga: bytes | None = None) -> None:
        if carga is not None:
            m = {**m, "bytes": len(carga)}
        linea = json.dumps(m, separators=(",", ":"), allow_nan=False).encode() + b"\n"
        if len(linea) > MAX_LINEA:
            raise ValueError(f"{m.get('tipo')}: la línea ocupa {len(linea)} bytes, el máximo es {MAX_LINEA}")
        self.sock.sendall(linea)
        if carga:
            self.sock.sendall(memoryview(carga))

    def leer(self) -> tuple[dict, bytes | None]:
        linea = self.lector.readline(MAX_LINEA + 1)
        if not linea:
            raise ConnectionError("el coordinador cerró la conexión")
        if len(linea) > MAX_LINEA or not linea.endswith(b"\n"):
            raise ValueError("línea demasiado larga o cortada")
        m = json.loads(linea)
        if not isinstance(m, dict):
            raise ValueError("el mensaje no es un objeto JSON")
        carga = None
        if "bytes" in m:
            n = m["bytes"]
            if not isinstance(n, int) or isinstance(n, bool) or not 0 <= n <= MAX_CARGA:
                raise ValueError(f"carga de {n!r} bytes fuera de [0, {MAX_CARGA}]: el flujo ya no es confiable")
            carga = self.lector.read(n)
            if len(carga) != n:
                raise ConnectionError(f"el coordinador cerró a mitad de la carga ({len(carga)} de {n} bytes)")
        return m, carga


class ErrorTarea(Exception):
    """Un problema de la tarea (no de la conexión): se contesta ERROR y se sigue atendiendo."""


def describir_dispositivo(dev: torch.device) -> tuple[str, int]:
    if dev.type != "cuda":
        return "", 0  # protocolo/: gpu vacío si entrena en CPU
    p = torch.cuda.get_device_properties(dev)
    return p.name, int(round(p.total_memory / 2 ** 30))


class Tarea:
    def __init__(self, m: dict, nombre: str, dev: torch.device, cache: Path, memoria: str):
        self.id = m.get("tarea")
        self.hiper = {**HIPER, **(m.get("hiper") or {})}
        if "lr" in self.hiper and "tasa" not in (m.get("hiper") or {}):
            self.hiper["tasa"] = self.hiper["lr"]
        h = self.hiper
        origen = datos.Origen(m["manifiesto"], cache)
        huella = str(m.get("huella", "")).removeprefix("sha256:")
        if huella != origen.huella:
            raise ErrorTarea(f"{nombre}: el manifiesto tiene sha256:{origen.huella[:12]}… y la tarea pide "
                             f"sha256:{huella[:12]}…")
        man = origen.manifiesto
        dias = sorted(m.get("dias") or [])
        faltan = [d for d in dias if d not in set(man.dias)]
        if not dias or faltan:
            raise ErrorTarea(f"{nombre}: la tarea no trae días o pide días que no están en el manifiesto ({faltan[:3]})")
        bajar = datos.dias_a_bajar(dias, man.dias)
        t0 = time.perf_counter()
        rutas = [origen.ruta(datos.ruta_dia(d)) for d in bajar]  # baja y verifica antes de decidir dónde
        nbytes = sum(r.stat().st_size for r in rutas)
        en_gpu = dev.type == "cuda" and memoria != "ram" and (
            memoria == "gpu" or nbytes < 0.5 * torch.cuda.mem_get_info(dev)[0])
        serie = datos.Serie.cargar(origen, bajar, dispositivo=dev if en_gpu else "cpu")
        estaticos = datos.cargar_estaticos(origen.ruta("estaticos.npy"))
        etiquetas = datos.leer_etiquetas(origen.ruta("etiquetas.csv"))
        self.conjunto = datos.Conjunto(serie, estaticos, etiquetas, dias, set(m.get("validacion") or []), dev)
        self.arq = red.arquitectura(man.n_entrada, int(h["ancho"]))
        self.modelo = red.construir(self.arq).to(dev)
        self.n = red.n_parametros(self.modelo)
        semilla = [int(h["semilla"]), zlib.crc32(",".join(dias).encode())]
        self.entrenador = red.Entrenador(self.modelo, self.conjunto, int(h["lote"]), float(h["tasa"]),
                                         float(h["decaimiento"]), semilla=semilla, amp=bool(h["amp"]))
        rng = np.random.default_rng(semilla)
        hv = self.conjunto.horas_val
        k = min(int(h["muestras_val"]), len(hv))
        self.horas_val = [hv[x] for x in sorted(rng.choice(len(hv), k, replace=False))] if k else []
        decir(nombre, f"TAREA {self.id}: {len(dias)} días (+{len(bajar) - len(dias)} previos, {nbytes / 1e6:.0f} MB "
                      f"en {'la GPU' if en_gpu else 'RAM'}), {len(self.conjunto.horas_ent)} horas de entrenamiento, "
                      f"{len(hv)} de validación; red de {self.n:,} parámetros; "
                      f"{time.perf_counter() - t0:.1f} s")

    @property
    def muestras(self) -> int:
        return len(self.conjunto.horas_ent)

    def ronda(self, m: dict, carga: bytes | None) -> tuple[dict, bytes]:
        ronda, pasos, n = m.get("ronda"), m.get("pasos"), m.get("n")
        if not isinstance(pasos, int) or pasos <= 0:
            raise ErrorTarea(f"PESOS {ronda}: pasos inválido ({pasos!r})")
        if n != self.n or carga is None or len(carga) != 4 * self.n:
            raise ErrorTarea(f"PESOS {ronda}: la red tiene {self.n} parámetros ({4 * self.n} bytes) y llegaron "
                             f"n={n}, {0 if carga is None else len(carga)} bytes")
        t0 = time.perf_counter()
        vector = red.de_bytes(carga)
        if not np.isfinite(vector).all():
            raise ErrorTarea(f"PESOS {ronda}: el vector trae valores no finitos")
        red.cargar_plano(self.modelo, vector)
        vistas, perdida = self.entrenador.pasos(pasos)
        perdida_val, _ = self.entrenador.perdida(self.horas_val, val=True)
        salida = red.aplanar(self.modelo)
        if self.modelo.salida.weight.is_cuda:
            torch.cuda.synchronize()
        ms = (time.perf_counter() - t0) * 1000
        if not (np.isfinite(salida).all() and math.isfinite(perdida) and math.isfinite(perdida_val)):
            raise ErrorTarea(f"LOCAL {ronda}: el entrenamiento divergió (pérdida {perdida})")
        resp = {"tipo": "LOCAL", "ronda": ronda, "n": self.n, "muestras": vistas, "perdida": float(perdida),
                "perdida_val": float(perdida_val), "ms_computo": int(round(ms))}
        return resp, red.a_bytes(salida)


def atender(con: Conexion, nombre: str, dev: torch.device, cache: Path, memoria: str) -> None:
    """Una sesión: HOLA y luego responder hasta que se corte la conexión."""
    gpu, vram = describir_dispositivo(dev)
    con.escribir({"tipo": "HOLA", "version": VERSION, "nodo": nombre, "workers": 1, "gpu": gpu, "vram_gb": vram})
    tarea: Tarea | None = None
    while True:
        m, carga = con.leer()
        tipo = m.get("tipo")
        try:
            if tipo == "PING":
                con.escribir({"tipo": "PONG"})
            elif tipo == "TAREA":
                tarea = None
                if dev.type == "cuda":
                    torch.cuda.empty_cache()
                tarea = Tarea(m, nombre, dev, cache, memoria)
                con.escribir({"tipo": "LISTO", "tarea": tarea.id, "muestras": tarea.muestras})
            elif tipo == "PESOS":
                if tarea is None:
                    raise ErrorTarea(f"{nombre}: PESOS sin TAREA")
                resp, salida = tarea.ronda(m, carga)
                con.escribir(resp, salida)
                r = resp["ronda"]
                if isinstance(r, int) and (r <= 3 or r % 10 == 0):
                    decir(nombre, f"ronda {r}: {m['pasos']} pasos, pérdida {resp['perdida']:.4f}, "
                                  f"validación {resp['perdida_val']:.4f}, {resp['ms_computo']} ms")
            elif tipo == "FIN":
                decir(nombre, "FIN: tarea terminada")
                tarea = None
                if dev.type == "cuda":
                    torch.cuda.empty_cache()
            elif tipo == "ERROR":
                raise ConnectionError(f"el coordinador informó: {m.get('mensaje')}")
            else:
                raise ErrorTarea(f"{nombre}: un nodo no espera {tipo}")
        except ErrorTarea as e:
            _avisar(con, nombre, str(e))
        except ConnectionError:
            raise  # el socket se cortó o el coordinador mandó ERROR: se vuelve a llamar
        except OSError as e:
            if not isinstance(e, (urllib.error.URLError, FileNotFoundError, TimeoutError)):
                raise  # un error del socket del coordinador
            _avisar(con, nombre, f"{nombre}: {tipo}: no se pudo bajar: {e}")
        except Exception as e:  # un campo mal formado, sha256 que no coincide, memoria de la GPU…
            _avisar(con, nombre, f"{nombre}: {tipo}: {type(e).__name__}: {e}")


def _avisar(con: Conexion, nombre: str, texto: str) -> None:
    decir(nombre, f"ERROR: {texto}")
    con.escribir({"tipo": "ERROR", "mensaje": texto[:2000]})


def main() -> int:
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("--coordinador", required=True, help="IP:puerto del coordinador")
    p.add_argument("--nombre", default=None, help="por defecto, <equipo>-gpu")
    p.add_argument("--cache", type=Path, default=Path.home() / "aire-mapa" / "cache",
                   help="dónde guardar los días bajados (por sha256)")
    p.add_argument("--memoria", choices=["auto", "gpu", "ram"], default="auto",
                   help="dónde dejar los días: auto = en la GPU si ocupan menos de la mitad de la VRAM libre")
    p.add_argument("--cpu", action="store_true", help="calcular en CPU (para probar sin GPU)")
    p.add_argument("--espera", type=float, default=2.0, help="segundos entre intentos de conexión")
    p.add_argument("--una-vez", action="store_true", help="salir al terminar la primera sesión (pruebas)")
    a = p.parse_args()

    nombre = a.nombre or f"{platform.node()}-gpu"
    dev = torch.device("cuda" if torch.cuda.is_available() and not a.cpu else "cpu")
    gpu, vram = describir_dispositivo(dev)
    host, puerto = a.coordinador.rsplit(":", 1)
    decir(nombre, f"{gpu or 'CPU'} ({vram} GB) | torch {torch.__version__} | cache {a.cache} | llamando a "
                  f"{a.coordinador}; Ctrl+C para salir")

    avisado = 0.0
    while True:
        try:
            sock = socket.create_connection((host, int(puerto)), timeout=3)
        except OSError as e:
            if time.time() - avisado > 30:
                decir(nombre, f"el coordinador no responde ({e}); sigo intentando")
                avisado = time.time()
            time.sleep(a.espera)
            continue
        sock.settimeout(None)
        sock.setsockopt(socket.IPPROTO_TCP, socket.TCP_NODELAY, 1)
        sock.setsockopt(socket.SOL_SOCKET, socket.SO_KEEPALIVE, 1)
        decir(nombre, f"conectado desde {sock.getsockname()[0]} a {a.coordinador}")
        con = Conexion(sock)
        try:
            atender(con, nombre, dev, a.cache, a.memoria)
        except (ConnectionError, OSError, ValueError) as e:
            decir(nombre, f"{e}; vuelvo a llamar")
        finally:
            con.cerrar()
        if a.una_vez:
            return 0
        time.sleep(a.espera)


if __name__ == "__main__":
    sys.stdout.reconfigure(errors="replace")  # en Windows, con la salida redirigida, sin romper por una tilde
    try:
        sys.exit(main())
    except KeyboardInterrupt:
        print("detenido con Ctrl+C", flush=True)
