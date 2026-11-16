"""El nodo (como subproceso) contra un coordinador falso que habla el protocolo v2 byte a byte."""

import functools
import hashlib
import http.server
import json
import socket
import subprocess
import sys
import threading
from pathlib import Path

import numpy as np
import pytest

import datos
import red

MODELO = Path(__file__).resolve().parents[1]


class Silencioso(http.server.SimpleHTTPRequestHandler):
    def log_message(self, *args):
        pass


@pytest.fixture
def url_manifiesto(sintetico_dir):
    srv = http.server.ThreadingHTTPServer(("127.0.0.1", 0),
                                          functools.partial(Silencioso, directory=str(sintetico_dir)))
    threading.Thread(target=srv.serve_forever, daemon=True).start()
    yield f"http://127.0.0.1:{srv.server_address[1]}/manifiesto.json"
    srv.shutdown()


class CoordinadorFalso:
    """El framing de la v2 escrito a mano: línea JSON y, si trae "bytes", exactamente esos bytes."""

    def __init__(self, sock):
        self.sock = sock
        self.lector = sock.makefile("rb")

    def escribir(self, m, carga=None):
        if carga is not None:
            m = {**m, "bytes": len(carga)}
        self.sock.sendall(json.dumps(m).encode() + b"\n" + (carga or b""))

    def leer(self):
        linea = self.lector.readline()
        assert linea.endswith(b"\n") and b"\n" not in linea[:-1]
        m = json.loads(linea)
        carga = self.lector.read(m["bytes"]) if "bytes" in m else None
        return m, carga


def test_nodo_contra_coordinador_falso(sintetico_dir, url_manifiesto, tmp_path):
    escucha = socket.create_server(("127.0.0.1", 0))
    escucha.settimeout(60)
    log = open(tmp_path / "nodo.log", "w")
    proc = subprocess.Popen(
        [sys.executable, str(MODELO / "nodo_mapa.py"), "--coordinador", f"127.0.0.1:{escucha.getsockname()[1]}",
         "--cpu", "--cache", str(tmp_path / "cache"), "--nombre", "prueba", "--una-vez", "--espera", "0.2"],
        cwd=MODELO, stdout=log, stderr=subprocess.STDOUT)
    try:
        sock, _ = escucha.accept()
        sock.settimeout(120)
        c = CoordinadorFalso(sock)

        hola, carga = c.leer()
        assert hola == {"tipo": "HOLA", "version": 2, "nodo": "prueba", "workers": 1, "gpu": "", "vram_gb": 0}
        assert carga is None
        c.escribir({"tipo": "PING"})
        assert c.leer() == ({"tipo": "PONG"}, None)

        # PESOS antes de TAREA: ERROR, y la carga se consumió (el flujo sigue sano).
        c.escribir({"tipo": "PESOS", "ronda": 1, "pasos": 1, "n": 3}, b"\0" * 12)
        m, _ = c.leer()
        assert m["tipo"] == "ERROR" and "TAREA" in m["mensaje"]

        man = datos.leer_manifiesto(sintetico_dir / "manifiesto.json")
        huella = hashlib.sha256((sintetico_dir / "manifiesto.json").read_bytes()).hexdigest()
        dias = man.dias[7:14] + man.dias[21:28]  # dos bloques de 7 días
        hiper = {"lote": 2, "tasa": 3e-3, "semilla": 1, "ancho": 8, "muestras_val": 8}
        tarea = {"tipo": "TAREA", "tarea": "t1", "manifiesto": url_manifiesto, "dias": dias,
                 "validacion": ["SIN-02"], "hiper": hiper}

        c.escribir({**tarea, "huella": "sha256:" + "0" * 64})
        m, _ = c.leer()
        assert m["tipo"] == "ERROR" and "sha256" in m["mensaje"]

        c.escribir({**tarea, "huella": "sha256:" + huella})
        listo, carga = c.leer()
        assert listo["tipo"] == "LISTO" and listo["tarea"] == "t1" and listo["muestras"] > 0 and carga is None
        assert set(listo) == {"tipo", "tarea", "muestras"}
        # Bajó los 14 días, el día previo de cada bloque, estaticos.npy y etiquetas.csv.
        bajados = sorted(p.name.split("_", 1)[1] for p in (tmp_path / "cache").iterdir())
        esperados = sorted(f"dias_{d}.npy" for d in dias + [man.dias[6], man.dias[20]]) + ["estaticos.npy",
                                                                                           "etiquetas.csv"]
        assert bajados == sorted(esperados)

        arq = red.arquitectura(man.n_entrada, 8)
        w = red.aplanar(red.construir(arq, semilla=0))
        n = w.size

        # n que no es el de la red: ERROR y se sigue.
        c.escribir({"tipo": "PESOS", "ronda": 1, "pasos": 2, "n": n - 1}, red.a_bytes(w[:-1]))
        m, _ = c.leer()
        assert m["tipo"] == "ERROR" and str(n) in m["mensaje"]

        perdidas = []
        for ronda in (1, 2, 3):
            c.escribir({"tipo": "PESOS", "ronda": ronda, "pasos": 5, "n": n}, red.a_bytes(w))
            local, carga = c.leer()
            assert set(local) == {"tipo", "ronda", "n", "bytes", "muestras", "perdida", "perdida_val", "ms_computo"}
            assert local["tipo"] == "LOCAL" and local["ronda"] == ronda and local["n"] == n
            assert local["bytes"] == 4 * n == len(carga)
            assert local["muestras"] == 5 * hiper["lote"]
            assert isinstance(local["ms_computo"], int) and local["ms_computo"] > 0
            assert local["perdida"] > 0 and local["perdida_val"] > 0
            nuevo = np.frombuffer(carga, dtype="<f4")
            assert np.isfinite(nuevo).all() and not np.array_equal(nuevo, w)
            perdidas.append(local["perdida"])
            w = nuevo.copy()  # con un solo nodo, el promedio es su propio vector

        # Después de la carga binaria viene el siguiente mensaje, sin bytes de más.
        c.escribir({"tipo": "PING"})
        assert c.leer() == ({"tipo": "PONG"}, None)
        c.escribir({"tipo": "FIN"})
        sock.shutdown(socket.SHUT_RDWR)
        c.lector.close()
        sock.close()
        assert proc.wait(timeout=30) == 0
    finally:
        if proc.poll() is None:
            proc.kill()
        escucha.close()
        log.close()
        print((tmp_path / "nodo.log").read_text())
