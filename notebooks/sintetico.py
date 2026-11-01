#!/usr/bin/env python3
"""Datos sintéticos para probar los notebooks antes de tener el CSV real de OEFA.

Genera, con semilla fija, un bronze largo verosímil y, a partir de él, las salidas que escribe
el sistema en Go («como si» las hubiera escrito Go):

    <raiz>/SINTETICO.txt                       marca: las figuras salen con marca de agua
    <raiz>/configs/kmeans.json                 receta provisional (F2, M1, T2, N1, k = 5)
    <raiz>/data/bronze/oefa_sintetico.csv      CSV largo, `;`, con suciedad de cada regla
    <raiz>/data/silver/oefa_ancho.csv          tabla ancha limpia (contrato de Go)
    <raiz>/data/gold/oefa_features.csv         features de la receta, estandarizadas
    <raiz>/data/gold/asignaciones.csv          estacion,fecha_hora,regimen
    <raiz>/reports/limpieza.json               conteos por regla, filas largas y anchas
    <raiz>/reports/estandarizacion.json        features, log1p, nivel, n, media, desv
    <raiz>/reports/kmeans.json                 centroides, tamaños, inercia
    <raiz>/reports/regimenes.json              perfil, etiqueta ECA, distribución por hora y estación

No importa `aire_lab`: el silver, los conteos y el K-Means de aquí son una implementación
independiente, de modo que la auditoría del notebook 03 compara dos implementaciones de verdad.

Seis estaciones en tres departamentos (dos a menos de 15 km en cada uno), 60 días de horas,
ocho parámetros, cinco regímenes plantados con persistencia (cadena de Markov que depende de la
hora), huecos (algunos justo después de los episodios en una estación), duplicados, centinelas,
fechas rotas, unidades distintas para CO, PM2.5 > PM10 en algunas horas y decimales con coma en
una estación.

Uso:  python notebooks/sintetico.py /ruta/a/la/raiz [--semilla 20261031] [--dias 60]
"""

from __future__ import annotations

import argparse
import csv
import json
import math
from pathlib import Path

import numpy as np

INICIO = np.datetime64("2020-01-01T00:00")  # con --dias hasta ~2400 no pasa de fecha_maxima

ESTACIONES = [
    # nombre, departamento, provincia, distrito, ubigeo, lat, lon, zona UTM (texto), altitud,
    # preferencia de régimen, parámetros medidos, decimal, perfil de SO2
    dict(nombre="SIN-CALLAO-NORTE", dep="CALLAO", prov="CALLAO", dist="VENTANILLA", ubigeo="070106",
         lat=-11.93, lon=-77.13, zona="18S", alt=20, pref=[0.40, 0.30, 0.22, 0.03, 0.05],
         params=["pm2_5", "pm10", "so2", "co", "temperatura", "humedad", "viento_velocidad", "presion"],
         dec=".", so2=6, tbase=17, tamp=3),
    dict(nombre="SIN-CALLAO-SUR", dep="CALLAO", prov="CALLAO", dist="CALLAO", ubigeo="070101",
         lat=-12.04, lon=-77.12, zona="18S", alt=15, pref=[0.38, 0.28, 0.20, 0.08, 0.06],
         params=["pm2_5", "pm10", "so2", "co", "temperatura", "humedad", "viento_velocidad"],
         dec=".", so2=9, tbase=17.5, tamp=3),
    dict(nombre="SIN-LA-OROYA", dep="JUNÍN", prov="YAULI", dist="LA OROYA", ubigeo="120801",
         lat=-11.52, lon=-75.90, zona="18 L", alt=3745, pref=[0.30, 0.25, 0.15, 0.08, 0.22],
         params=["pm2_5", "pm10", "so2", "co", "temperatura", "humedad", "viento_velocidad", "presion"],
         dec=".", so2=60, tbase=8, tamp=9),
    dict(nombre="SIN-HUAYHUAY", dep="JUNÍN", prov="YAULI", dist="HUAY-HUAY", ubigeo="120806",
         lat=-11.60, lon=-75.95, zona="18 L", alt=3900, pref=[0.35, 0.30, 0.10, 0.15, 0.10],
         params=["pm10", "so2", "temperatura", "humedad", "viento_velocidad", "presion"],
         dec=".", so2=25, tbase=6, tamp=9),
    dict(nombre="SIN-ILO-PUERTO", dep="MOQUEGUA", prov="ILO", dist="ILO", ubigeo="180301",
         lat=-17.64, lon=-71.34, zona="19", alt=30, pref=[0.40, 0.25, 0.05, 0.25, 0.05],
         params=["pm2_5", "pm10", "so2", "temperatura", "humedad", "viento_velocidad", "presion"],
         dec=".", so2=12, tbase=19, tamp=4),
    dict(nombre="SIN-ILO-PAMPA", dep="MOQUEGUA", prov="ILO", dist="EL ALGARROBAL", ubigeo="180302",
         lat=-17.60, lon=-71.28, zona="19", alt=120, pref=[0.45, 0.25, 0.05, 0.22, 0.03],
         params=["pm2_5", "pm10", "co", "temperatura", "humedad", "viento_velocidad", "presion"],
         dec=",", so2=5, tbase=19.5, tamp=5),
]

# Nombre real (como vendría en el CSV) y unidad por parámetro canónico.
NOMBRE_REAL = {"pm2_5": "PM2.5", "pm10": "PM10", "so2": "SO2", "co": "CO", "temperatura": "Temperatura",
               "humedad": "Humedad relativa", "viento_velocidad": "Velocidad del viento",
               "presion": "Presión atmosférica"}
UNIDAD = {"pm2_5": "ug/m3", "pm10": "ug/m3", "so2": "ug/m3", "co": "ug/m3", "temperatura": "°C",
          "humedad": "%", "viento_velocidad": "m/s", "presion": "hPa"}
DECIMALES = {"pm2_5": 1, "pm10": 1, "so2": 1, "co": 1, "temperatura": 1, "humedad": 1,
             "viento_velocidad": 2, "presion": 1}

# Regímenes plantados: limpio, moderado, inversión/combustión, polvo con viento, episodio.
PM10_MED = np.array([20, 40, 55, 120, 220.0])
FF_MU = np.array([0.45, 0.50, 0.72, 0.22, 0.55])
VIENTO_MU = np.array([3.8, 2.5, 0.7, 7.0, 0.9])
HUM_MU = np.array([70, 62, 92, 40, 82.0])
CO_FACTOR = np.array([1.0, 1.2, 2.5, 0.9, 2.0])

RECETA = {"features": ["pm10", "fraccion_fina", "viento_velocidad", "humedad"], "log1p": ["pm10"],
          "nivel": "absoluto", "k": 5, "semilla": 42, "tolerancia": 1e-9, "max_iter": 100,
          "workers": 8, "chunk": 10000}

COLUMNAS = ["ID_INFORME", "PROCEDENCIA", "NOMBRE_PUNTO", "DEPARTAMENTO", "PROVINCIA", "DISTRITO",
            "UBIGEO", "ESTE", "NORTE", "ALTITUD", "ZONA", "DATUM", "PARAMETRO", "FECHA", "HORA",
            "VALOR", "UNIDAD", "PERIODO"]


def latlon_a_utm(lat, lon, zona):
    """Proyección UTM directa (WGS84, Snyder 1987). Independiente de la inversa de aire_lab."""
    a, f, k0 = 6378137.0, 1 / 298.257223563, 0.9996
    e2 = f * (2 - f)
    ep2 = e2 / (1 - e2)
    phi, lam = math.radians(lat), math.radians(lon)
    lam0 = math.radians((zona - 1) * 6 - 180 + 3)
    N = a / math.sqrt(1 - e2 * math.sin(phi) ** 2)
    T, C, A = math.tan(phi) ** 2, ep2 * math.cos(phi) ** 2, math.cos(phi) * (lam - lam0)
    M = a * ((1 - e2 / 4 - 3 * e2**2 / 64 - 5 * e2**3 / 256) * phi
             - (3 * e2 / 8 + 3 * e2**2 / 32 + 45 * e2**3 / 1024) * math.sin(2 * phi)
             + (15 * e2**2 / 256 + 45 * e2**3 / 1024) * math.sin(4 * phi)
             - (35 * e2**3 / 3072) * math.sin(6 * phi))
    x = k0 * N * (A + (1 - T + C) * A**3 / 6 + (5 - 18 * T + T**2 + 72 * C - 58 * ep2) * A**5 / 120) + 500000
    y = k0 * (M + N * math.tan(phi) * (A**2 / 2 + (5 - T + 9 * C + 4 * C**2) * A**4 / 24
                                       + (61 - 58 * T + T**2 + 600 * C - 330 * ep2) * A**6 / 720))
    if lat < 0:
        y += 10000000.0
    return round(x), round(y)


def fmt_go(v: float) -> str:
    """Como strconv.FormatFloat(v, 'f', -1, 64): el decimal más corto que vuelve al float."""
    return np.format_float_positional(float(v), unique=True, trim="-")


def fmt_valor(v: float, p: str, dec: str, mg=False) -> str:
    s = f"{v:.{3 if mg else DECIMALES[p]}f}"
    return s.replace(".", ",") if dec == "," else s


def parse_valor(s: str) -> float:
    return float(s.replace(",", ".")) if "." not in s else float(s)


def regimenes(rng, est, horas):
    pref = np.array(est["pref"])
    r = np.empty(len(horas), dtype=int)
    r[0] = rng.choice(5, p=pref)
    for i in range(1, len(horas)):
        h = horas[i]
        if rng.random() < 0.92:
            r[i] = r[i - 1]
            continue
        w = pref.copy()
        if h < 7 or h >= 20:
            w[2] *= 2.0
            w[4] *= 1.8
        if 12 <= h <= 17:
            w[3] *= 2.5
            w[2] *= 0.3
            w[4] *= 0.5
        r[i] = rng.choice(5, p=w / w.sum())
    return r


def generar(raiz: Path, semilla: int, dias: int) -> dict:
    rng = np.random.default_rng(semilla)
    n_h = dias * 24
    tiempos = INICIO + np.arange(n_h).astype("timedelta64[h]")
    horas = (np.arange(n_h) % 24)
    dow = ((tiempos.astype("datetime64[D]").view("int64") - 4) % 7)  # 0 = lunes

    verdad = {}      # (estacion, i_hora, param) -> float
    presente = {}    # misma clave -> bool
    regimen_verdad = {}
    for est in ESTACIONES:
        reg = regimenes(rng, est, horas)
        regimen_verdad[est["nombre"]] = reg
        pm10 = PM10_MED[reg] * np.exp(rng.normal(0, 0.22, n_h))
        ff = np.clip(FF_MU[reg] + rng.normal(0, 0.05, n_h), 0.05, 0.97)
        pm25 = pm10 * ff
        doble = rng.random(n_h) < 0.015  # PM2.5 > PM10: error de medición plantado
        pm25[doble] = pm10[doble] * rng.uniform(1.02, 1.25, doble.sum())
        viento = rng.gamma(6.0, VIENTO_MU[reg] / 6.0)
        hum = np.clip(HUM_MU[reg] + rng.normal(0, 5, n_h), 5, 100)
        temp = (est["tbase"] + est["tamp"] * np.sin(2 * np.pi * (horas - 9) / 24)
                + rng.normal(0, 1.0, n_h) - 1.0 * np.isin(reg, [2, 4]))
        pres = (1013.25 * math.exp(-est["alt"] / 8434) + 1.2 * np.cos(2 * np.pi * 2 * horas / 24)
                + rng.normal(0, 0.6, n_h))
        turno = np.isin(horas, [7, 8, 15, 16, 23, 0]).astype(float)
        semana = np.where(dow == 6, 0.6, np.where(dow == 5, 0.9, 1.15))
        so2 = (est["so2"] * (1 + 1.2 * turno * (est["so2"] > 20)) * semana
               * np.where(reg == 4, 2.0, 1.0) * np.exp(rng.normal(0, 0.35, n_h)))
        co = 450 * CO_FACTOR[reg] * np.exp(rng.normal(0, 0.3, n_h))
        serie = {"pm2_5": pm25, "pm10": pm10, "so2": so2, "co": co, "temperatura": temp,
                 "humedad": hum, "viento_velocidad": viento, "presion": pres}

        # Presencia: cortes de la estación (todas las variables) y apagones por parámetro.
        corte = np.zeros(n_h, dtype=bool)
        for _ in range(5):
            ini = rng.integers(0, n_h)
            corte[ini: ini + min(96, 1 + rng.geometric(1 / 10))] = True
        if est["nombre"] == "SIN-LA-OROYA":  # Q9: la estación se cae justo después de episodios
            for i in np.flatnonzero(reg == 4):
                if rng.random() < 0.04:
                    ini = i + rng.integers(1, 4)
                    corte[ini: ini + rng.integers(6, 31)] = True
        apagon = {p: np.zeros(n_h, dtype=bool) for p in est["params"]}
        if est["nombre"] == "SIN-ILO-PAMPA":
            apagon["humedad"][20 * 24: 30 * 24] = True
        if est["nombre"] == "SIN-CALLAO-NORTE":
            apagon["pm2_5"][40 * 24: 45 * 24] = True
        for p in est["params"]:
            pres_p = ~corte & ~apagon[p]
            for i in range(n_h):
                verdad[(est["nombre"], i, p)] = float(serie[p][i])
                presente[(est["nombre"], i, p)] = bool(pres_p[i])

    # ------------------------------------------------------------------ filas largas del bronze
    info_est = {}
    for est in ESTACIONES:
        num = int("".join(c for c in est["zona"] if c.isdigit()))
        e, n = latlon_a_utm(est["lat"], est["lon"], num)
        info_est[est["nombre"]] = dict(est, este=e, norte=n)

    def fila(nombre, i, p, valor_txt, unidad=None, periodo="Horario", fecha=None, hora=None, param_real=None):
        st = info_est[nombre]
        t = tiempos[i].astype(object)
        return {
            "ID_INFORME": f"INF-SIN-{st['ubigeo']}-{t.strftime('%Y%m')}",
            "PROCEDENCIA": "Estación automática (sintética)", "NOMBRE_PUNTO": nombre,
            "DEPARTAMENTO": st["dep"], "PROVINCIA": st["prov"], "DISTRITO": st["dist"],
            "UBIGEO": st["ubigeo"], "ESTE": str(st["este"]), "NORTE": str(st["norte"]),
            "ALTITUD": str(st["alt"]), "ZONA": st["zona"], "DATUM": "WGS84",
            "PARAMETRO": param_real or ("PM 2,5" if (p == "pm2_5" and st["dec"] == ",") else NOMBRE_REAL[p]),
            "FECHA": fecha if fecha is not None else t.strftime("%d/%m/%Y"),
            "HORA": hora if hora is not None else t.strftime("%H:%M"),
            "VALOR": valor_txt, "UNIDAD": unidad or unidad_de(nombre, p), "PERIODO": periodo,
        }

    def unidad_de(nombre, p):
        if p == "co" and nombre == "SIN-LA-OROYA":
            return "mg/m3"
        if p in ("pm2_5", "pm10") and nombre.startswith("SIN-CALLAO"):
            return "µg/m³"
        return UNIDAD[p]

    def texto(nombre, i, p):
        v = verdad[(nombre, i, p)]
        mg = p == "co" and nombre == "SIN-LA-OROYA"
        return fmt_valor(v / 1000 if mg else v, p, info_est[nombre]["dec"], mg)

    celdas = [k for k, v in presente.items() if v]
    filas = {k: fila(k[0], k[1], k[2], texto(*k)) for k in celdas}
    perdida = set()            # celdas que no llegan al silver
    cuenta = {"r1_periodo": 0, "r1_parametro": 0, "r2_fecha": 0, "r3_valor": 0, "r6_unidad": 0,
              "r4_rango": 0, "r5_duplicado": 0}
    marcas = {"r2_minutos_truncados": 0, "r5_valor_distinto": 0, "r6_convertidas": 0}

    orden = rng.permutation(len(celdas))
    cursor = 0

    def tomar(n, condicion=lambda k: True):
        nonlocal cursor
        out = []
        while len(out) < n and cursor < len(orden):
            k = celdas[orden[cursor]]
            cursor += 1
            if condicion(k):
                out.append(k)
        return out

    for k in tomar(30):  # regla 2: fecha rota
        filas[k]["FECHA"] = ["31/02/2023", "2023-13-45", "sin fecha"][rng.integers(0, 3)]
        perdida.add(k); cuenta["r2_fecha"] += 1
    for k in tomar(10):  # regla 2: hora vacía
        filas[k]["HORA"] = ""
        perdida.add(k); cuenta["r2_fecha"] += 1
    for k in tomar(50):  # regla 3: centinelas
        filas[k]["VALOR"] = "-999" if rng.random() < 0.8 else "-9999"
        perdida.add(k); cuenta["r3_valor"] += 1
    for k in tomar(35):  # regla 3: no numérico
        filas[k]["VALOR"] = "ND" if rng.random() < 0.6 else ""
        perdida.add(k); cuenta["r3_valor"] += 1
    for k in tomar(20, lambda k: k[2] != "temperatura"):  # regla 3: negativo
        filas[k]["VALOR"] = "-3,5" if info_est[k[0]]["dec"] == "," else "-3.5"
        perdida.add(k); cuenta["r3_valor"] += 1
    fuera = {"pm10": 5000.0, "humedad": 135.0, "viento_velocidad": 99.0}
    for k in tomar(25, lambda k: k[2] in fuera):  # regla 4: fuera de rango
        filas[k]["VALOR"] = fmt_valor(fuera[k[2]], k[2], info_est[k[0]]["dec"])
        perdida.add(k); cuenta["r4_rango"] += 1
    for k in tomar(15, lambda k: k[2] in ("so2", "co")):  # regla 6: unidad desconocida
        filas[k]["UNIDAD"] = "ppm"
        perdida.add(k); cuenta["r6_unidad"] += 1

    # Regla 5: duplicados (al final del archivo, así el original es el primero).
    sanas = [k for k in celdas if k not in perdida]
    elegidas = rng.choice(len(sanas), 200, replace=False)
    dups = []
    for j, idx in enumerate(elegidas):
        k = sanas[idx]
        d = dict(filas[k])
        if j < 50 and k[2] in ("pm10", "pm2_5", "so2"):
            v = parse_valor(d["VALOR"]) + 7.3
            d["VALOR"] = fmt_valor(v, k[2], info_est[k[0]]["dec"])
            marcas["r5_valor_distinto"] += 1
        dups.append(d)
        cuenta["r5_duplicado"] += 1

    # Filas extra que la limpieza descarta enteras.
    extras = []
    for est in ESTACIONES:  # regla 1: promedios diarios
        for d in range(dias):
            v = float(np.mean([verdad[(est["nombre"], d * 24 + h, "pm10")] for h in range(24)]))
            extras.append(fila(est["nombre"], d * 24, "pm10", fmt_valor(v, "pm10", est["dec"]),
                               periodo="Diario", hora="00:00"))
            cuenta["r1_periodo"] += 1
    for i in rng.choice(n_h, 120, replace=False):  # regla 1: parámetro fuera del mapeo
        extras.append(fila("SIN-LA-OROYA", int(i), "so2", f"{rng.gamma(3, 4):.1f}",
                           unidad="ug/m3", param_real="H2S"))
        cuenta["r1_parametro"] += 1
    for j in range(12):  # regla 2: fuera del rango del dataset
        est = ESTACIONES[j % len(ESTACIONES)]
        extras.append(fila(est["nombre"], 5, "pm10", "40.0", fecha="01/01/1970" if j % 2 else "15/03/2099"))
        cuenta["r2_fecha"] += 1

    # Orden de archivo: por parámetro, estación y hora; extras intercalados; duplicados al final.
    base = [filas[k] for k in sorted(celdas, key=lambda k: (k[2], k[0], k[1]))]
    huecos_ins = np.sort(rng.integers(0, len(base) + 1, len(extras)))
    principales, previo = [], 0
    for pos, e in zip(huecos_ins, extras):
        principales.extend(base[previo:pos])
        principales.append(e)
        previo = pos
    principales.extend(base[previo:])
    dups = [dups[i] for i in rng.permutation(len(dups))]
    todas = principales + dups

    # Unidades convertidas (mg/m3 -> µg/m3) que sobreviven a las reglas 1-4 (antes de deduplicar).
    for f in todas:
        if f["UNIDAD"] == "mg/m3" and f["PERIODO"] == "Horario":
            try:
                v = parse_valor(f["VALOR"])
            except ValueError:
                continue
            fecha_ok = f["FECHA"] not in ("31/02/2023", "2023-13-45", "sin fecha") and f["HORA"] != ""
            if fecha_ok and v >= 0 and v not in (-999, -9999):
                marcas["r6_convertidas"] += 1

    rutas = {k: raiz / v for k, v in {
        "bronze": "data/bronze/oefa_sintetico.csv", "silver": "data/silver/oefa_ancho.csv",
        "gold": "data/gold/oefa_features.csv", "asig": "data/gold/asignaciones.csv",
        "limpieza": "reports/limpieza.json", "estand": "reports/estandarizacion.json",
        "kmeans": "reports/kmeans.json", "regimenes": "reports/regimenes.json",
        "receta": "configs/kmeans.json"}.items()}
    for r in rutas.values():
        r.parent.mkdir(parents=True, exist_ok=True)
    with open(rutas["bronze"], "w", encoding="utf-8", newline="") as fh:
        w = csv.DictWriter(fh, fieldnames=COLUMNAS, delimiter=";", lineterminator="\r\n")
        w.writeheader()
        w.writerows(todas)

    # ------------------------------------------------------------------ silver «de Go»
    ancho = {}
    for k in celdas:
        if k in perdida:
            continue
        v = parse_valor(filas[k]["VALOR"])
        if filas[k]["UNIDAD"] == "mg/m3":
            v = v * 1000.0
        ancho.setdefault((k[0], k[1]), {})[k[2]] = v
    params = sorted({p for e in ESTACIONES for p in e["params"]})
    claves = sorted(ancho)  # (estación, i_hora): orden de estación y fecha-hora
    with open(rutas["silver"], "w", encoding="utf-8", newline="") as fh:
        w = csv.writer(fh, lineterminator="\n")
        w.writerow(["estacion", "fecha_hora"] + params)
        for est, i in claves:
            t = tiempos[i].astype(object).strftime("%Y-%m-%dT%H:%M:%S") + "-05:00"
            w.writerow([est, t] + [fmt_go(ancho[(est, i)][p]) if p in ancho[(est, i)] else "" for p in params])

    procesadas = sum(len(v) for v in ancho.values())
    leidas = len(todas)

    # ------------------------------------------------------------------ gold «de Go»
    feats, logs = RECETA["features"], RECETA["log1p"]
    filas_gold, X = [], []
    for est, i in claves:
        f = ancho[(est, i)]
        if all(p in f for p in ("pm10", "pm2_5", "viento_velocidad", "humedad")) and f["pm10"] > 0:
            ffina = min(max(f["pm2_5"] / f["pm10"], 0.0), 1.0)
            X.append([math.log1p(f["pm10"]), ffina, f["viento_velocidad"], f["humedad"]])
            filas_gold.append((est, i))
    X = np.array(X)
    # Welford, como el cargador de Go (una pasada, estable).
    media = np.zeros(X.shape[1]); m2 = np.zeros(X.shape[1])
    for j, x in enumerate(X, start=1):
        d = x - media
        media += d / j
        m2 += d * (x - media)
    desv = np.sqrt(m2 / len(X))
    Z = (X - media) / desv
    with open(rutas["gold"], "w", encoding="utf-8", newline="") as fh:
        w = csv.writer(fh, lineterminator="\n")
        w.writerow(["estacion", "fecha_hora"] + feats)
        for (est, i), z in zip(filas_gold, Z):
            t = tiempos[i].astype(object).strftime("%Y-%m-%dT%H:%M:%S") + "-05:00"
            w.writerow([est, t] + [fmt_go(v) for v in z])
    with open(rutas["estand"], "w", encoding="utf-8") as fh:
        json.dump({"features": feats, "log1p": logs, "nivel": "absoluto", "n": len(X),
                   "media": media.tolist(), "desv": desv.tolist(), "origen": "sintetico"}, fh, indent=2)

    # ------------------------------------------------------------------ K-Means «de Go» (Lloyd)
    k, rng_k = RECETA["k"], np.random.default_rng(RECETA["semilla"])
    C = [Z[rng_k.integers(len(Z))]]
    d2 = ((Z - C[0]) ** 2).sum(1)
    for _ in range(1, k):
        i = rng_k.choice(len(Z), p=d2 / d2.sum())
        C.append(Z[i])
        d2 = np.minimum(d2, ((Z - Z[i]) ** 2).sum(1))
    C = np.array(C)
    for it in range(1, RECETA["max_iter"] + 1):
        lab = ((Z[:, None, :] - C[None]) ** 2).sum(2).argmin(1)
        nuevo = C.copy()
        for j in range(k):
            if (lab == j).any():
                nuevo[j] = Z[lab == j].mean(0)
        mov = np.abs(nuevo - C).max()
        C = nuevo
        if mov <= RECETA["tolerancia"]:
            break
    D = ((Z[:, None, :] - C[None]) ** 2).sum(2)
    lab = D.argmin(1)
    inercia = float(D[np.arange(len(Z)), lab].sum())
    tam = np.bincount(lab, minlength=k)
    Corig = C * desv + media
    Corig[:, 0] = np.expm1(Corig[:, 0])
    with open(rutas["kmeans"], "w", encoding="utf-8") as fh:
        json.dump({"k": k, "semilla": RECETA["semilla"], "features": feats, "iteraciones": it,
                   "convergio": bool(mov <= RECETA["tolerancia"]), "inercia": inercia,
                   "tamanos": tam.tolist(), "centroides_estandarizados": C.tolist(),
                   "centroides_desestandarizados": Corig.tolist(), "origen": "sintetico"}, fh, indent=2)
    with open(rutas["asig"], "w", encoding="utf-8", newline="") as fh:
        w = csv.writer(fh, lineterminator="\n")
        w.writerow(["estacion", "fecha_hora", "regimen"])
        for (est, i), l in zip(filas_gold, lab):
            w.writerow([est, tiempos[i].astype(object).strftime("%Y-%m-%dT%H:%M:%S") + "-05:00", int(l)])

    regs = []
    for j in range(k):
        miembros = [filas_gold[x] for x in np.flatnonzero(lab == j)]
        perfil = {}
        for p in params:
            vals = [ancho[m][p] for m in miembros if p in ancho[m]]
            perfil[p] = float(np.mean(vals)) if vals else None
        excede = (perfil.get("pm2_5") or 0) > 50 or (perfil.get("pm10") or 0) > 100
        por_hora = np.bincount([int(horas[i]) for _, i in miembros], minlength=24).tolist()
        por_est = {}
        for est, _ in miembros:
            por_est[est] = por_est.get(est, 0) + 1
        regs.append({"regimen": j, "n": len(miembros), "perfil": perfil, "excede_eca": bool(excede),
                     "etiqueta": "excede el ECA" if excede else "bajo el ECA",
                     "por_hora": por_hora, "por_estacion": dict(sorted(por_est.items()))})
    with open(rutas["regimenes"], "w", encoding="utf-8") as fh:
        json.dump({"eca": {"pm2_5": 50, "pm10": 100}, "regimenes": regs, "origen": "sintetico"},
                  fh, indent=2, ensure_ascii=False)

    por_est_ancho = {}
    for est, _ in claves:
        por_est_ancho[est] = por_est_ancho.get(est, 0) + 1
    por_est_gold = {}
    for est, _ in filas_gold:
        por_est_gold[est] = por_est_gold.get(est, 0) + 1
    with open(rutas["limpieza"], "w", encoding="utf-8") as fh:
        json.dump({"origen": "sintetico",
                   "filas_largas": {"leidas": leidas, "procesadas": procesadas,
                                    "descartadas": sum(cuenta.values())},
                   "reglas": cuenta, "marcas": marcas,
                   "filas_anchas": {"total": len(claves), "completas": len(filas_gold),
                                    "por_estacion": por_est_ancho, "completas_por_estacion": por_est_gold}},
                  fh, indent=2, ensure_ascii=False)
    assert leidas == procesadas + sum(cuenta.values()), (leidas, procesadas, cuenta)

    with open(rutas["receta"], "w", encoding="utf-8") as fh:
        json.dump(RECETA, fh, indent=2)
    (raiz / "SINTETICO.txt").write_text(
        "Datos sintéticos generados por notebooks/sintetico.py: no son mediciones de OEFA.\n"
        f"semilla={semilla} dias={dias}\n", encoding="utf-8")
    # Regímenes plantados (verdad) para quien quiera mirar cuánto recupera el K-Means.
    with open(raiz / "reports" / "sintetico_regimenes_plantados.csv", "w", encoding="utf-8", newline="") as fh:
        w = csv.writer(fh, lineterminator="\n")
        w.writerow(["estacion", "fecha_hora", "regimen_plantado"])
        for est in ESTACIONES:
            for i, r in enumerate(regimen_verdad[est["nombre"]]):
                w.writerow([est["nombre"], tiempos[i].astype(object).strftime("%Y-%m-%dT%H:%M:%S") + "-05:00", int(r)])
    return {"filas_largas": leidas, "procesadas": procesadas, "filas_anchas": len(claves),
            "filas_gold": len(filas_gold), "reglas": cuenta, "iteraciones_kmeans": it}


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("raiz", type=Path, help="directorio raíz de las capas (se crea si no existe)")
    ap.add_argument("--semilla", type=int, default=20261031)
    ap.add_argument("--dias", type=int, default=60)
    a = ap.parse_args()
    a.raiz.mkdir(parents=True, exist_ok=True)
    print(json.dumps(generar(a.raiz, a.semilla, a.dias), indent=2, ensure_ascii=False))


if __name__ == "__main__":
    main()
