"""Métricas por estación, iguales para M0, M1 y M2 (§4): RMSE, sesgo y R² en µg/m³ y en log1p, y la
detección de días sobre el ECA de 24 h (sensibilidad y precisión). Solo numpy.

Un "registro" es una estación-hora: estacion, dia (UTC), obs [N, 2] y pred [N, 2] en µg/m³ (pm2_5,
pm10; NaN si falta). El promedio diario de una estación es el de las horas 00–23 UTC del día con
observación (el mismo día que el promedio diario del mapa); el día cuenta para el ECA si tiene al
menos MIN_HORAS horas, y el predicho se promedia sobre esas mismas horas.
"""

from __future__ import annotations

import math

import numpy as np

from datos import CONTAMINANTES, ECA_24H

MIN_HORAS = 18  # 75 % de las 24 horas


def _f(x: float) -> float | None:
    return None if x is None or not math.isfinite(x) else round(float(x), 6)


def resumen(obs: np.ndarray, pred: np.ndarray) -> dict:
    """RMSE, sesgo (pred − obs) y R² de dos vectores ya filtrados."""
    if len(obs) == 0:
        return {"rmse": None, "sesgo": None, "r2": None}
    err = pred - obs
    ss_tot = float(((obs - obs.mean()) ** 2).sum())
    r2 = 1 - float((err ** 2).sum()) / ss_tot if ss_tot > 0 else math.nan
    return {"rmse": _f(math.sqrt(float((err ** 2).mean()))), "sesgo": _f(float(err.mean())), "r2": _f(r2)}


def deteccion_eca(dia: np.ndarray, obs: np.ndarray, pred: np.ndarray, umbral: float,
                  min_horas: int = MIN_HORAS) -> dict:
    vp = fp = fn = vn = 0
    for d in np.unique(dia):
        sel = dia == d
        if sel.sum() < min_horas:
            continue
        o, p = bool(obs[sel].mean() > umbral), bool(pred[sel].mean() > umbral)
        vp += int(o and p)
        fp += int(p and not o)
        fn += int(o and not p)
        vn += int(not o and not p)
    return {"umbral": umbral, "dias": vp + fp + fn + vn, "dias_sobre_obs": vp + fn, "vp": vp, "fp": fp, "fn": fn,
            "vn": vn, "sensibilidad": _f(vp / (vp + fn)) if vp + fn else None,
            "precision": _f(vp / (vp + fp)) if vp + fp else None}


def _bloque(dia: np.ndarray, obs: np.ndarray, pred: np.ndarray, contaminante: str) -> dict:
    ok = np.isfinite(obs) & np.isfinite(pred)
    o, p, d = obs[ok], pred[ok], dia[ok]
    lo, lp = np.log1p(np.clip(o, 0, None)), np.log1p(np.clip(p, 0, None))
    return {"n": int(ok.sum()), "ug": resumen(o, p), "log1p": resumen(lo, lp),
            "eca": deteccion_eca(d, o, p, ECA_24H[contaminante])}


def por_estacion(estacion: np.ndarray, dia: np.ndarray, obs: np.ndarray, pred: np.ndarray) -> dict:
    """{"estaciones": {nombre: {pm2_5: …, pm10: …}}, "global": {…}} con todas las horas juntas."""
    estacion, dia = np.asarray(estacion), np.asarray(dia)
    obs, pred = np.asarray(obs, np.float64), np.asarray(pred, np.float64)
    salida = {"estaciones": {}, "global": {}}
    for s in sorted(set(estacion.tolist())):
        sel = estacion == s
        salida["estaciones"][s] = {c: _bloque(dia[sel], obs[sel, k], pred[sel, k], c)
                                   for k, c in enumerate(CONTAMINANTES)}
    # En el global el día se cuenta por estación (estación-día), no se mezclan estaciones.
    clave = np.array([f"{s}|{d}" for s, d in zip(estacion, dia)], dtype=object)
    salida["global"] = {c: _bloque(clave, obs[:, k], pred[:, k], c) for k, c in enumerate(CONTAMINANTES)}
    return salida
