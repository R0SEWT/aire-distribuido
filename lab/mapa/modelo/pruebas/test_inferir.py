import functools
import http.server
import io
import subprocess
import sys
import threading
from pathlib import Path

import numpy as np
import pytest
import torch

import datos
import inferir
import red

MODELO = Path(__file__).resolve().parents[1]


def _serie(sintetico_dir, dias):
    o = datos.Origen(str(sintetico_dir))
    return o, datos.Serie.cargar(o, dias), torch.as_tensor(datos.cargar_estaticos(o.ruta("estaticos.npy")))


def test_promedio_diario_es_la_media_de_las_24_horas(sintetico_dir):
    o, serie, est = _serie(sintetico_dir, ["2020-01-09", "2020-01-10"])
    modelo = red.construir(red.arquitectura(o.manifiesto.n_entrada, 8), semilla=3).eval()
    media, desv = inferir.promedio_diario([modelo], serie, est, "2020-01-10", lote=5)
    assert media.shape == desv.shape == (2, 24, 16) and (desv == 0).all()
    with torch.no_grad():
        x = datos.armar_entrada(serie, est, [("2020-01-10", h) for h in range(24)])
        esperado = torch.expm1(modelo(x).clamp_min(0)).mean(0).numpy()
    assert np.allclose(media, esperado, rtol=1e-5)


def test_desviacion_entre_modelos(sintetico_dir):
    o, serie, est = _serie(sintetico_dir, ["2020-01-01"])  # sin día anterior: rellena
    arq = red.arquitectura(o.manifiesto.n_entrada, 8)
    a, b = red.construir(arq, semilla=1).eval(), red.construir(arq, semilla=2).eval()
    ma, _ = inferir.promedio_diario([a], serie, est, "2020-01-01")
    mb, _ = inferir.promedio_diario([b], serie, est, "2020-01-01")
    m, s = inferir.promedio_diario([a, b], serie, est, "2020-01-01")
    assert np.allclose(m, (ma + mb) / 2, rtol=1e-5) and np.allclose(s, np.abs(ma - mb) / 2, rtol=1e-4, atol=1e-5)
    _, s0 = inferir.promedio_diario([a, a], serie, est, "2020-01-01")
    assert (s0 == 0).all()
    arr = np.load(io.BytesIO(inferir.empaquetar(m, s)))
    assert arr.dtype == np.float16 and arr.shape == (2, 2, 24, 16)
    assert np.allclose(arr[:, 0], m, rtol=1e-3) and np.allclose(arr[:, 1], s, rtol=1e-3, atol=1e-3)


class Servidor(http.server.SimpleHTTPRequestHandler):
    recibidos: dict = {}

    def do_PUT(self):
        datos_ = self.rfile.read(int(self.headers["Content-Length"]))
        Servidor.recibidos[self.path] = datos_
        self.send_response(201)
        self.send_header("Content-Length", "0")
        self.end_headers()

    def log_message(self, *args):
        pass


@pytest.fixture
def servidor(sintetico_dir, tmp_path):
    raiz = tmp_path / "raiz"
    raiz.mkdir()
    (raiz / "mapa").symlink_to(sintetico_dir, target_is_directory=True)
    m = datos.leer_manifiesto(sintetico_dir / "manifiesto.json")
    arq = red.arquitectura(m.n_entrada, 8)
    red.exportar(red.construir(arq, semilla=1), arq, raiz / "pesos", nombre="pesos_p0.bin")
    red.exportar(red.construir(arq, semilla=2), arq, raiz / "pesos", nombre="pesos_p1.bin")
    Servidor.recibidos = {}
    srv = http.server.ThreadingHTTPServer(("127.0.0.1", 0), functools.partial(Servidor, directory=str(raiz)))
    threading.Thread(target=srv.serve_forever, daemon=True).start()
    yield f"http://127.0.0.1:{srv.server_address[1]}"
    srv.shutdown()


def test_modo_nodo_sube_cada_dia_con_put(sintetico_dir, servidor, tmp_path):
    r = subprocess.run(
        [sys.executable, str(MODELO / "inferir.py"), "--datos", f"{servidor}/mapa/manifiesto.json",
         "--pesos-json", f"{servidor}/pesos/pesos.json", "--pesos", f"{servidor}/pesos/pesos_p0.bin",
         f"{servidor}/pesos/pesos_p1.bin", "--parte", "1/3", "--subir", servidor + "/", "--cpu",
         "--cache", str(tmp_path / "cache")], cwd=MODELO, capture_output=True, text=True, timeout=300)
    assert r.returncode == 0, r.stdout + r.stderr
    m = datos.leer_manifiesto(sintetico_dir / "manifiesto.json")
    mios = datos.repartir_bloques(m.dias, 3)[1]
    assert sorted(Servidor.recibidos) == [f"/resultados/mapa_{d}.npy" for d in mios]
    arr = np.load(io.BytesIO(Servidor.recibidos[f"/resultados/mapa_{mios[0]}.npy"]))
    assert arr.shape == (2, 2, 24, 16) and arr.dtype == np.float16
    assert (arr[:, 0] > 0).all() and (arr[:, 1] > 0).any()  # dos modelos distintos: hay desviación
