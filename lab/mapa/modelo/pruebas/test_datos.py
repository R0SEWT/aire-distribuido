import json

import numpy as np
import pytest
import torch

import datos
import sintetico
from datos import HORAS


def test_formato_en_disco(sintetico_dir):
    m = datos.leer_manifiesto(sintetico_dir / "manifiesto.json")
    assert len(m.dias) == 30 and m.dias[0] == "2020-01-01" and m.dias[-1] == "2020-01-30"
    C = len(sintetico.CANALES)
    dia = np.load(sintetico_dir / "dias" / "2020-01-05.npy")
    assert dia.dtype == np.float16 and dia.shape == (HORAS, C, 24, 16)
    est = np.load(sintetico_dir / "estaticos.npy")
    assert est.dtype == np.float32 and est.shape == (len(sintetico.ESTATICOS), 24, 16)
    assert m.n_entrada == HORAS * C + len(sintetico.ESTATICOS) + 4
    # Los canales normalizados tienen media ~0 y desviación ~1 en el periodo.
    todo = np.stack([np.load(sintetico_dir / "dias" / f"{d}.npy").astype(np.float32) for d in m.dias])
    assert np.allclose(todo.mean(axis=(0, 1, 3, 4)), 0, atol=0.02)
    assert np.allclose(todo.std(axis=(0, 1, 3, 4)), 1, atol=0.02)


def test_cams_se_recupera_en_ug(sintetico_dir):
    m = datos.leer_manifiesto(sintetico_dir / "manifiesto.json")
    dia = np.load(sintetico_dir / "dias" / "2020-01-05.npy")
    pm25 = m.desnormalizar(dia[:, m.indice("cams_pm2_5")], "cams_pm2_5")
    pm10 = m.desnormalizar(dia[:, m.indice("cams_pm10")], "cams_pm10")
    assert 2 < np.median(pm25) < 200 and np.median(pm10) > np.median(pm25)


def test_origen_verifica_sha256(sintetico_dir, tmp_path):
    import shutil
    copia = tmp_path / "copia"
    shutil.copytree(sintetico_dir, copia)
    assert datos.Origen(str(copia)).ruta("dias/2020-01-01.npy").exists()
    arr = np.load(copia / "dias" / "2020-01-02.npy")
    arr[0, 0, 0, 0] += 1
    np.save(copia / "dias" / "2020-01-02.npy", arr)
    with pytest.raises(ValueError, match="sha256"):
        datos.Origen(str(copia)).ruta("dias/2020-01-02.npy")


def test_manifiesto_tolera_variantes():
    m = datos.Manifiesto.desde_json({
        "canales": [{"nombre": "a", "media": 1, "std": 2}],
        "estaticos": [{"nombre": "orografia"}],
        "grilla": {"forma": [192, 136]},
        "sha256": {"dias/2018-01-02.npy": "ab", "dias/2018-01-01.npy": "cd", "estaticos.npy": "ef"},
    })
    assert m.dias == ["2018-01-01", "2018-01-02"] and m.filas == 192 and m.columnas == 136
    assert m.desviaciones[0] == 2 and m.estaticos == ["orografia"]
    assert m.desnormalizar(np.array([0.5]), "a")[0] == 2.0
    m2 = datos.Manifiesto.desde_json({
        "canales": [{"nombre": "cams_pm10", "media": 3.0, "desviacion": 0.5, "transformacion": "log1p"}],
        "estaticos": [], "grilla": {"filas": 8, "columnas": 8}, "archivos": {}})
    assert np.isclose(m2.desnormalizar(np.array([2.0]), "cams_pm10")[0], np.expm1(4.0))


def test_etiquetas(sintetico_dir):
    e = datos.leer_etiquetas(sintetico_dir / "etiquetas.csv")
    assert e.estaciones() == [f"SIN-0{k}" for k in range(1, 6)]
    assert e.valores.shape == (len(e), 2)
    # La última estación no mide PM2.5; ninguna fila viene vacía del todo.
    assert np.isnan(e.valores[e.estacion == "SIN-05", 0]).all()
    assert not np.isnan(e.valores).all(axis=1).any()
    assert e.dia[0] == "2020-01-01" and e.hora[0] == 0


def test_hora_con_zona_se_pasa_a_utc(tmp_path):
    p = tmp_path / "e.csv"
    p.write_text("hora_utc,estacion,fuente,i,j,pm2_5,pm10\n2020-01-01T20:00:00-05:00,X,OEFA,1,2,,30\n")
    e = datos.leer_etiquetas(p)
    assert e.dia[0] == "2020-01-02" and e.hora[0] == 1 and np.isnan(e.valores[0, 0]) and e.valores[0, 1] == 30


def test_ventana_cruza_al_dia_anterior(sintetico_dir):
    o = datos.Origen(str(sintetico_dir))
    s = datos.Serie.cargar(o, ["2020-01-04", "2020-01-05"])
    ayer = np.load(sintetico_dir / "dias" / "2020-01-04.npy")
    hoy = np.load(sintetico_dir / "dias" / "2020-01-05.npy")
    C = ayer.shape[1]
    v = s.ventana("2020-01-05", 5).numpy()
    assert v.shape == (HORAS * C, 24, 16) and v.dtype == np.float16
    esperado = np.concatenate([ayer[6:], hoy[:6]]).reshape(HORAS * C, 24, 16)
    assert np.array_equal(v, esperado)
    # La última hora del bloque de 24 es la hora t.
    assert np.array_equal(v[-C:], hoy[5])
    assert np.array_equal(s.ventana("2020-01-05", 23).numpy(), hoy.reshape(-1, 24, 16))
    # Sin el día anterior no hay ventana, salvo para la hora 23 o rellenando.
    assert not s.tiene_ventana("2020-01-04", 5) and s.tiene_ventana("2020-01-04", 23)
    with pytest.raises(KeyError):
        s.ventana("2020-01-04", 5)
    r = s.ventana("2020-01-04", 5, rellenar=True).numpy().reshape(HORAS, C, 24, 16)
    assert np.array_equal(r[:18], np.repeat(ayer[:1], 18, axis=0)) and np.array_equal(r[18:], ayer[:6])
    # Con memmap da lo mismo.
    sm = datos.Serie.cargar(o, ["2020-01-04", "2020-01-05"], mmap=True)
    assert np.array_equal(sm.ventana("2020-01-05", 5).numpy(), esperado)


def test_calendario():
    c = datos.calendario("2020-01-01", 6, 3, 2)
    assert c.shape == (4, 3, 2)
    assert np.allclose(c[:, 0, 0], [1, 0, 0, 1], atol=1e-6)


def test_conjunto_muestras_y_objetivos(sintetico_dir):
    o = datos.Origen(str(sintetico_dir))
    m = o.manifiesto
    e = datos.leer_etiquetas(o.ruta("etiquetas.csv"))
    dias = m.dias[:7]
    serie = datos.Serie.cargar(o, datos.dias_a_bajar(dias, m.dias))
    est = datos.cargar_estaticos(o.ruta("estaticos.npy"))
    c = datos.Conjunto(serie, est, e, dias, {"SIN-02"})
    # El primer día no tiene día anterior: solo su hora 23 tiene ventana.
    assert ("2020-01-01", 5) not in c.filas_ent and ("2020-01-01", 23) in c.filas_ent
    assert all(d in dias for d, _ in c.horas_ent)
    assert all(e.estacion[c.filas_ent[k]].tolist().count("SIN-02") == 0 for k in c.horas_ent)
    assert all(set(c.etiquetas.estacion[c.filas_val[k]]) == {"SIN-02"} for k in c.horas_val)
    horas = c.horas_ent[:3]
    x = c.entrada(horas)
    assert x.shape == (3, m.n_entrada, 24, 16) and x.dtype == torch.float32
    b, i, j, y, msk = c.objetivos(horas)
    assert y.shape == msk.shape == (len(b), 2) and set(b.tolist()) == {0, 1, 2}
    assert torch.isfinite(y).all() and (y[msk == 0] == 0).all()
    assert msk.sum() > 0


def test_particiones_sin_estaciones_compartidas():
    est = [f"E{k:02d}" for k in range(22)]
    p = datos.particiones(est, 5, semilla=7)
    assert len(p) == 5 and sorted(sum(p, [])) == est
    assert all(set(a).isdisjoint(b) for x, a in enumerate(p) for b in p[x + 1:])
    assert sorted(len(x) for x in p) == [4, 4, 4, 5, 5]
    assert p == datos.particiones(list(reversed(est)), 5, semilla=7)
    assert p != datos.particiones(est, 5, semilla=8)


def test_reparto_en_bloques_de_7():
    dias = datos.rango_dias("2018-01-01", "2018-03-31")  # 90 días: 12 bloques de 7 y uno de 6
    partes = datos.repartir_bloques(dias, 4, semilla=1)
    assert sorted(sum(partes, [])) == dias
    assert [len(p) for p in partes] and max(len(p) for p in partes) - min(len(p) for p in partes) <= 7
    bloques = [dias[k:k + 7] for k in range(0, 90, 7)]
    for p in partes:
        for b in bloques:  # un bloque va entero a un solo nodo
            assert set(b) <= set(p) or set(b).isdisjoint(p)
    assert partes == datos.repartir_bloques(dias, 4, semilla=1)


def test_dias_a_bajar_suma_el_dia_previo_de_cada_bloque():
    dispo = datos.rango_dias("2018-01-01", "2018-01-31")
    asig = datos.rango_dias("2018-01-08", "2018-01-14") + datos.rango_dias("2018-01-22", "2018-01-28")
    assert datos.dias_a_bajar(asig, dispo) == ["2018-01-07"] + asig[:7] + ["2018-01-21"] + asig[7:]
    assert datos.dias_a_bajar(["2018-01-01"], dispo) == ["2018-01-01"]


def test_lo_del_nodo_no_importa_mas_que_numpy_torch_y_la_biblioteca_estandar():
    import subprocess
    import sys
    from pathlib import Path
    codigo = ("import sys\n"
              "for m in ('pandas', 'xgboost', 'shapely', 'rasterio', 'scipy', 'sklearn', 'metricas', 'base', "
              "'agregar', 'entrenar', 'sintetico'):\n"
              "    sys.modules[m] = None\n"
              "import datos, red, inferir, nodo_mapa\n")
    r = subprocess.run([sys.executable, "-c", codigo], cwd=Path(__file__).resolve().parents[1], capture_output=True,
                       text=True)
    assert r.returncode == 0, r.stderr
