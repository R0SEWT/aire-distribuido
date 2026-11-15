"""Las herramientas de la laptop en cadena sobre el sintético, por su línea de comandos."""

import json
import subprocess
import sys
from pathlib import Path

import numpy as np

from test_agregar import _limites

MODELO = Path(__file__).resolve().parents[1]


def correr(*args):
    r = subprocess.run([sys.executable, *map(str, args)], cwd=MODELO, capture_output=True, text=True, timeout=600)
    assert r.returncode == 0, r.stdout + r.stderr
    return r.stdout


def test_de_los_datos_al_reporte_por_distrito(sintetico_dir, tmp_path):
    d = sintetico_dir
    salida = correr("red.py", "exportar", "--manifiesto", d / "manifiesto.json", "--ancho", "8", "--salida",
                    tmp_path / "pesos0")
    assert "pesos_0.bin" in salida
    pj = json.loads((tmp_path / "pesos0" / "pesos.json").read_text())
    assert (tmp_path / "pesos0" / "pesos_0.bin").stat().st_size == 4 * pj["n"] == pj["bytes"]

    correr("base.py", "--datos", d, "--arboles", "50", "--salida", tmp_path / "base.json")
    base = json.loads((tmp_path / "base.json").read_text())
    assert set(base["M0"]["metricas"]["estaciones"]) == {f"SIN-0{k}" for k in range(1, 6)}
    assert base["M1"]["metricas"]["global"]["pm10"]["ug"]["rmse"] < base["M0"]["metricas"]["global"]["pm10"]["ug"]["rmse"]

    for k in (0, 1):
        correr("entrenar.py", "--datos", d, "--particion", k, "--pasos", "20", "--evaluar-cada", "10", "--ancho", "8",
               "--pesos-iniciales", tmp_path / "pesos0" / "pesos_0.bin", "--salida", tmp_path / f"m2_p{k}.json",
               "--pesos-salida", tmp_path / "pesos", "--cpu")
        m2 = json.loads((tmp_path / f"m2_p{k}.json").read_text())
        assert m2["validacion"] == base["particiones"][k] and len(m2["historia"]) == 2
        assert set(m2["metricas"]["estaciones"]) == set(base["particiones"][k])

    correr("inferir.py", "--datos", d, "--pesos-json", tmp_path / "pesos" / "pesos.json", "--pesos",
           tmp_path / "pesos" / "pesos_p0.bin", tmp_path / "pesos" / "pesos_p1.bin", "--salida", tmp_path / "res",
           "--cpu")
    assert len(list((tmp_path / "res").glob("mapa_*.npy"))) == 30

    rng = np.random.default_rng(0)
    np.savez(tmp_path / "pob.npz", lon=rng.uniform(-81.6, -80.1, 500), lat=rng.uniform(-1.8, 0.6, 500),
             pob=rng.uniform(10, 1000, 500))
    correr("agregar.py", "--datos", d, "--resultados", tmp_path / "res", "--poblacion", tmp_path / "pob.npz",
           "--limites", _limites(tmp_path / "limites.geojson"), "--mapa", tmp_path / "mapa",
           "--salida", tmp_path / "mapa_distritos.json")
    assert np.load(tmp_path / "mapa" / "pm25_diario.npy").shape == (30, 24, 16)
    assert np.load(tmp_path / "mapa" / "dias_sobre_eca.npy").shape == (1, 2, 24, 16)
    assert np.load(tmp_path / "mapa" / "distancia_estacion_km.npy").min() == 0
    rep = json.loads((tmp_path / "mapa_distritos.json").read_text())
    assert [x["id"] for x in rep["distritos"]] == ["A", "B"]
    assert abs(sum(x["poblacion"] for x in rep["distritos"]) + rep["poblacion_sin_distrito"]
               - rep["poblacion_total"]) < 1
    assert set(rep["distritos"][0]["anios"]["2020"]) == {"dias", "pm2_5", "pm10"}
