"""M0 (CAMS crudo en la celda) y M1 (XGBoost por celda-hora), con las mismas particiones y métricas que M2.

    uv run python base.py --datos data/mapa --salida reports/mapa_base.json

M0 es el PM2.5 y el PM10 de CAMS en la celda de la estación a la hora t (los canales "cams_pm2_5" y
"cams_pm10" llevados a µg/m³ con la media y la desviación del manifiesto). No se entrena: es la línea
base del modelo físico.

M1 es un XGBoost por contaminante sobre log1p, una fila por estación-hora, con los C canales en t, sus
medias de las últimas 6 y 24 horas, los S estáticos de la celda y los 4 de calendario. Para cada una de
las 5 particiones por estación (datos.particiones, las de entrenar.py) se entrena con las demás
estaciones y se predicen las de la partición: cada estación queda predicha por un modelo que no la vio.
Usa la GPU (device="cuda") si torch la ve, si no la CPU.

Solo entran las estación-horas con ventana de 24 h completa (las mismas que puede predecir M2).
"""

from __future__ import annotations

import argparse
import json
import time
from pathlib import Path

import numpy as np

import datos
import metricas
from datos import HORAS

PARAMETROS_XGB = {"max_depth": 6, "eta": 0.05, "subsample": 0.8, "colsample_bytree": 0.8, "min_child_weight": 5,
                  "tree_method": "hist", "objective": "reg:squarederror"}
ARBOLES = 400


def series_en_celdas(origen: datos.Origen, celdas: list[tuple[int, int]]) -> tuple[list[str], np.ndarray, np.ndarray]:
    """Los canales de cada celda en una línea de tiempo horaria continua.

    Devuelve (días del calendario, X [T, C, n_celdas] float32 con NaN en los días que faltan, hay [T]).
    """
    man = origen.manifiesto
    calendario_dias = datos.rango_dias(man.dias[0], man.dias[-1])
    presentes = set(man.dias)
    ii = np.array([c[0] for c in celdas])
    jj = np.array([c[1] for c in celdas])
    X = np.full((len(calendario_dias) * HORAS, man.n_canales, len(celdas)), np.nan, np.float32)
    hay = np.zeros(len(calendario_dias) * HORAS, bool)
    for k, d in enumerate(calendario_dias):
        if d not in presentes:
            continue
        arr = np.load(origen.ruta(datos.ruta_dia(d)), mmap_mode="r")
        X[k * HORAS:(k + 1) * HORAS] = np.asarray(arr[:, :, ii, jj], np.float32)
        hay[k * HORAS:(k + 1) * HORAS] = True
    return calendario_dias, X, hay


def media_movil(X: np.ndarray, ventana: int) -> np.ndarray:
    """Media de las últimas `ventana` horas (incluida t) a lo largo del eje 0; NaN al principio."""
    cs = np.concatenate([np.zeros_like(X[:1], np.float64), np.cumsum(np.nan_to_num(X), 0, dtype=np.float64)])
    m = np.full(X.shape, np.nan, np.float32)
    m[ventana - 1:] = (cs[ventana:] - cs[:-ventana]) / ventana
    return m


def tabla(origen: datos.Origen, etiquetas: datos.Etiquetas, estaticos: np.ndarray) -> dict:
    """Una fila por estación-hora con etiqueta y ventana completa: features de M1, CAMS (M0) y observado."""
    man = origen.manifiesto
    celdas = sorted(set(zip(etiquetas.i.tolist(), etiquetas.j.tolist())))
    col = {c: k for k, c in enumerate(celdas)}
    cal_dias, X, hay = series_en_celdas(origen, celdas)
    m6, m24 = media_movil(X, 6), media_movil(X, HORAS)
    completa = np.zeros_like(hay)
    completa[HORAS - 1:] = np.lib.stride_tricks.sliding_window_view(hay, HORAS).all(axis=1)
    pos = {d: k for k, d in enumerate(cal_dias)}

    t = np.array([pos.get(d, -1) * HORAS + h if d in pos else -1 for d, h in zip(etiquetas.dia, etiquetas.hora)])
    ok = (t >= 0) & ~np.isnan(etiquetas.valores).all(axis=1)
    ok[ok] &= completa[t[ok]]
    e = etiquetas.filtrar(ok)
    t = t[ok]
    k = np.array([col[(i, j)] for i, j in zip(e.i.tolist(), e.j.tolist())], np.int64)
    cal = np.stack([datos.calendario(d, int(h), 1, 1)[:, 0, 0] for d, h in zip(e.dia, e.hora)]) if len(e) \
        else np.empty((0, datos.N_CALENDARIO), np.float32)
    features = np.concatenate([X[t, :, k], m6[t, :, k], m24[t, :, k], estaticos[:, e.i, e.j].T, cal], axis=1)
    nombres = ([f"{c}_t" for c in man.canales] + [f"{c}_m6" for c in man.canales]
               + [f"{c}_m24" for c in man.canales] + list(man.estaticos) + ["hora_sen", "hora_cos", "doy_sen", "doy_cos"])
    m0 = np.stack([man.desnormalizar(X[t, man.indice("cams_pm2_5"), k], "cams_pm2_5"),
                   man.desnormalizar(X[t, man.indice("cams_pm10"), k], "cams_pm10")], axis=1)
    return {"etiquetas": e, "X": features.astype(np.float32), "nombres": nombres, "m0": np.clip(m0, 0, None)}


def dispositivo_xgb() -> str:
    try:
        import torch
        return "cuda" if torch.cuda.is_available() else "cpu"
    except ImportError:
        return "cpu"


def m1_por_particion(t: dict, pliegues: list[list[str]], parametros: dict, arboles: int, dispositivo: str,
                     semilla: int = 0) -> np.ndarray:
    """Predicción de M1 (µg/m³, [N, 2]) para cada fila con el modelo de la partición que no vio su estación."""
    import xgboost as xgb

    e = t["etiquetas"]
    pred = np.full(e.valores.shape, np.nan, np.float32)
    for pliegue in pliegues:
        prueba = np.isin(e.estacion, pliegue)
        for c in range(2):
            ent = ~prueba & ~np.isnan(e.valores[:, c])
            if not ent.any() or not prueba.any():
                continue
            dm = xgb.DMatrix(t["X"][ent], label=np.log1p(np.clip(e.valores[ent, c], 0, None)))
            m = xgb.train({**parametros, "device": dispositivo, "seed": semilla}, dm, num_boost_round=arboles)
            pred[prueba, c] = np.expm1(m.predict(xgb.DMatrix(t["X"][prueba])))
    return np.clip(pred, 0, None)


def main() -> None:
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("--datos", required=True, help="carpeta (o URL) con manifiesto.json")
    p.add_argument("--particiones", type=int, default=datos.N_PARTICIONES)
    p.add_argument("--semilla-particiones", type=int, default=0)
    p.add_argument("--semilla", type=int, default=0)
    p.add_argument("--arboles", type=int, default=ARBOLES)
    p.add_argument("--cpu", action="store_true")
    p.add_argument("--salida", required=True)
    a = p.parse_args()

    origen = datos.Origen(a.datos)
    etiquetas = datos.leer_etiquetas(origen.ruta("etiquetas.csv"))
    estaticos = datos.cargar_estaticos(origen.ruta("estaticos.npy"))
    pliegues = datos.particiones(etiquetas.estaciones(), a.particiones, a.semilla_particiones)
    t0 = time.perf_counter()
    t = tabla(origen, etiquetas, estaticos)
    e = t["etiquetas"]
    print(f"{len(e):,} estación-horas con ventana completa, {t['X'].shape[1]} features "
          f"({time.perf_counter() - t0:.1f} s)", flush=True)
    dev = "cpu" if a.cpu else dispositivo_xgb()
    t1 = time.perf_counter()
    m1 = m1_por_particion(t, pliegues, PARAMETROS_XGB, a.arboles, dev, a.semilla)
    seg_m1 = time.perf_counter() - t1

    def bloque(pred: np.ndarray) -> dict:
        return metricas.por_estacion(e.estacion, e.dia, e.valores, pred)

    salida = {
        "particiones": pliegues, "semilla_particiones": a.semilla_particiones, "filas": len(e),
        "M0": {"metricas": bloque(t["m0"])},
        "M1": {"xgboost": {**PARAMETROS_XGB, "arboles": a.arboles}, "dispositivo": dev, "features": t["nombres"], "segundos": round(seg_m1, 1),
               "metricas": bloque(m1)},
        "por_particion": {},
    }
    for f, pliegue in enumerate(pliegues):
        sel = np.isin(e.estacion, pliegue)
        salida["por_particion"][str(f)] = {
            "validacion": pliegue,
            "M0": metricas.por_estacion(e.estacion[sel], e.dia[sel], e.valores[sel], t["m0"][sel])["global"],
            "M1": metricas.por_estacion(e.estacion[sel], e.dia[sel], e.valores[sel], m1[sel])["global"],
        }
    Path(a.salida).parent.mkdir(parents=True, exist_ok=True)
    Path(a.salida).write_text(json.dumps(salida, indent=1, ensure_ascii=False), encoding="utf-8")
    for nombre in ("M0", "M1"):
        g = salida[nombre]["metricas"]["global"]
        print(f"{nombre}: RMSE PM2.5 {g['pm2_5']['ug']['rmse']}, PM10 {g['pm10']['ug']['rmse']} µg/m³ "
              f"(log1p {g['pm2_5']['log1p']['rmse']}, {g['pm10']['log1p']['rmse']})")
    print(f"→ {a.salida}")


if __name__ == "__main__":
    main()
