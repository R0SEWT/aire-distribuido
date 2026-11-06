#!/usr/bin/env python3
"""Datos sintéticos para probar los notebooks a escala sin el CSV real de OEFA.

Genera, con semilla fija, un bronze largo verosímil **en el formato real de OEFA** que lee Go con
configs/limpieza.json (el de data/sample/: 36 columnas, `;`, textos entre comillas, CRLF, una columna
por parámetro con un solo valor por fila, `-` y `NA` como vacíos; los días completos van en bloques de
24 filas sin hora, que Go infiere por posición, y los días con huecos con la hora de inicio explícita,
`1899-12-31 HH:00:00`) y, con `--go`, corre el sistema de verdad sobre él: `cmd/cargar` (silver, gold,
limpieza.json, estandarizacion.json) y `cmd/entrenar` (kmeans.json, asignaciones, regimenes.json,
codo.json). Go es la fuente de verdad: este script no imita sus salidas. Todas las salidas quedan en la
raíz que se le pasa (también los centroides iniciales de cmd/entrenar); las configs son las del repo.

    <raiz>/SINTETICO.txt                       marca: las figuras salen con marca de agua
    <raiz>/data/bronze/oefa_sintetico.csv      CSV largo con suciedad de cada regla
    <raiz>/reports/sintetico_regimenes_plantados.csv   los regímenes plantados (la «verdad», fechados
                                               con la convención de Go: fin del intervalo horario)
    con --go: <raiz>/data/{silver,gold}/… y <raiz>/reports/*.json, escritos por Go

Seis estaciones en tres departamentos (dos a menos de 15 km en cada uno), días de horas desde
2020-01-01, ocho parámetros, cinco regímenes plantados con persistencia (cadena de Markov que
depende de la hora), huecos (algunos justo después de los episodios en una estación), y suciedad de
cada regla de la cascada sin romper los bloques de 24: centinelas, textos, negativos y fuera de rango
dentro de los días; un día entero en 0; un día de SO2 en ppm; SO2 duplicado en ppb; CO en mg/m³ y
presión en mmHg (se convierten); meteorología con periodo `-`; promedios de 24 horas; fechas rotas y
fuera de rango, horas ilegibles, filas sin parámetro o con dos, sin estación o con columnas de menos;
duplicados exactos y con otro valor al final; PM2.5 > PM10 en algunas horas. Estaciones con prefijo SIN-.

Uso:  python notebooks/sintetico.py /ruta/a/la/raiz [--semilla 20261031] [--dias 60] [--go]
"""

from __future__ import annotations

import argparse
import csv
import json
import math
import os
import shutil
import subprocess
from collections import Counter
from pathlib import Path

import numpy as np

INICIO = np.datetime64("2020-01-01T00:00")  # con --dias hasta ~2400 no pasa de fecha_maxima

ESTACIONES = [
    # nombre, departamento, provincia, distrito, ubigeo, lat, lon, zona UTM (texto), altitud,
    # preferencia de régimen, parámetros medidos, perfil de SO2
    dict(nombre="SIN-CALLAO-NORTE", dep="CALLAO", prov="CALLAO", dist="VENTANILLA", ubigeo="070106",
         lat=-11.93, lon=-77.13, zona="18S", alt=20, pref=[0.40, 0.30, 0.22, 0.03, 0.05],
         params=["pm2_5", "pm10", "so2", "co", "temperatura", "humedad", "viento_velocidad", "presion"],
         so2=6, tbase=17, tamp=3),
    dict(nombre="SIN-CALLAO-SUR", dep="CALLAO", prov="CALLAO", dist="CALLAO", ubigeo="070101",
         lat=-12.04, lon=-77.12, zona="18S", alt=15, pref=[0.38, 0.28, 0.20, 0.08, 0.06],
         params=["pm2_5", "pm10", "so2", "co", "temperatura", "humedad", "viento_velocidad"],
         so2=9, tbase=17.5, tamp=3),
    dict(nombre="SIN-LA-OROYA", dep="JUNÍN", prov="YAULI", dist="LA OROYA", ubigeo="120801",
         lat=-11.52, lon=-75.90, zona="18 L", alt=3745, pref=[0.30, 0.25, 0.15, 0.08, 0.22],
         params=["pm2_5", "pm10", "so2", "co", "temperatura", "humedad", "viento_velocidad", "presion"],
         so2=60, tbase=8, tamp=9),
    dict(nombre="SIN-HUAYHUAY", dep="JUNÍN", prov="YAULI", dist="HUAY-HUAY", ubigeo="120806",
         lat=-11.60, lon=-75.95, zona="18 L", alt=3900, pref=[0.35, 0.30, 0.10, 0.15, 0.10],
         params=["pm10", "so2", "temperatura", "humedad", "viento_velocidad", "presion"],
         so2=25, tbase=6, tamp=9),
    dict(nombre="SIN-ILO-PUERTO", dep="MOQUEGUA", prov="ILO", dist="ILO", ubigeo="180301",
         lat=-17.64, lon=-71.34, zona="19", alt=30, pref=[0.40, 0.25, 0.05, 0.25, 0.05],
         params=["pm2_5", "pm10", "so2", "temperatura", "humedad", "viento_velocidad", "presion"],
         so2=12, tbase=19, tamp=4),
    dict(nombre="SIN-ILO-PAMPA", dep="MOQUEGUA", prov="ILO", dist="EL ALGARROBAL", ubigeo="180302",
         lat=-17.60, lon=-71.28, zona="19", alt=120, pref=[0.45, 0.25, 0.05, 0.22, 0.03],
         params=["pm2_5", "pm10", "co", "temperatura", "humedad", "viento_velocidad", "presion"],
         so2=5, tbase=19.5, tamp=5),
]

# Esquema real del CSV de OEFA (spec PC3 §4.1; configs/limpieza.json, parametros_en_columnas).
COLUMNAS = ["NOMBRE_EVALUACION", "COMPONENTE_AMBIENTAL", "PROCEDENCIA_MUESTRA", "NOMBRE_PUNTO", "ESTE", "NORTE",
            "ALTITUD", "ZONA", "DATUM", "DEPARTAMENTO", "PROVINCIA", "DISTRITO", "UBIGEO", "DESCRIPCION_UBICACION",
            "TIPO_MUESTRA", "TIPO_ANALISIS", "PERIODO", "FECHA_INICIO", "HORA_INICIO", "FECHA_FIN", "HORA_FIN",
            "UNIDAD_MEDIDA", "DIOXIDO_AZUFRE", "DIRECCION_VIENTO", "HUMEDAD_RELATIVA", "PM10", "PM2.5",
            "MONOXIDO_CARBONO", "OZONO", "PRECIPITACION", "PRESION_BAROMETRICA", "RADIACION_SOLAR",
            "SULFURO_HIDROGENO", "TEMPERATURA", "VELOCIDAD_VIENTO", "FECHA_CORTE"]
COLUMNAS_PARAMETRO = COLUMNAS[22:35]
SIN_COMILLAS = {"ESTE", "NORTE", "ALTITUD", "ZONA", "UBIGEO", "FECHA_INICIO", "HORA_INICIO", "FECHA_FIN", "HORA_FIN",
                "FECHA_CORTE"}
COLUMNA_PARAM = {"pm2_5": "PM2.5", "pm10": "PM10", "so2": "DIOXIDO_AZUFRE", "co": "MONOXIDO_CARBONO",
                 "temperatura": "TEMPERATURA", "humedad": "HUMEDAD_RELATIVA", "viento_velocidad": "VELOCIDAD_VIENTO",
                 "presion": "PRESION_BAROMETRICA"}
TIPO_ANALISIS = {"pm10": "Concentración de partículas en PM10", "pm2_5": "Concentración de partículas en PM2,5",
                 "so2": "-", "co": "-"}
UNIDAD = {"pm2_5": "ug/m3", "pm10": "ug/m3", "so2": "ug/m3", "co": "ug/m3", "temperatura": "°c",
          "humedad": "%", "viento_velocidad": "m/s", "presion": "hPa"}
DECIMALES = {"pm2_5": 1, "pm10": 1, "so2": 1, "co": 1, "temperatura": 1, "humedad": 1,
             "viento_velocidad": 2, "presion": 1}
FUERA = {"pm10": "5000.0", "humedad": "135.0", "viento_velocidad": "99.0"}  # fuera de los rangos de limpieza.json
METEO_SIN = {"temperatura", "humedad", "viento_velocidad", "presion"}

# Regímenes plantados: limpio, moderado, inversión/combustión, polvo con viento, episodio.
PM10_MED = np.array([20, 40, 55, 120, 220.0])
FF_MU = np.array([0.45, 0.50, 0.72, 0.22, 0.55])
VIENTO_MU = np.array([3.8, 2.5, 0.7, 7.0, 0.9])
HUM_MU = np.array([70, 62, 92, 40, 82.0])
CO_FACTOR = np.array([1.0, 1.2, 2.5, 0.9, 2.0])


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

    verdad = {}      # (estacion, param) -> serie horaria (np.ndarray)
    presente = {}    # misma clave -> si hay lectura en cada hora
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
            verdad[(est["nombre"], p)] = np.asarray(serie[p], dtype=float)
            presente[(est["nombre"], p)] = ~corte & ~apagon[p]

    bronze = raiz / "data" / "bronze" / "oefa_sintetico.csv"
    esperado = escribir_bronze(bronze, rng, dias, verdad, presente)
    (raiz / "SINTETICO.txt").write_text(
        "Datos sintéticos generados por notebooks/sintetico.py: no son mediciones de OEFA.\n"
        f"semilla={semilla} dias={dias}\n", encoding="utf-8")
    (raiz / "reports").mkdir(parents=True, exist_ok=True)
    # La verdad, con la convención de Go (fin): la lectura de la hora i cubre [i, i+1) y se fecha en i+1.
    with open(raiz / "reports" / "sintetico_regimenes_plantados.csv", "w", encoding="utf-8", newline="") as fh:
        w = csv.writer(fh, lineterminator="\n")
        w.writerow(["estacion", "fecha_hora", "regimen_plantado"])
        for est in ESTACIONES:
            for i, r in enumerate(regimen_verdad[est["nombre"]]):
                w.writerow([est["nombre"], (tiempos[i] + np.timedelta64(1, "h")).astype(object).strftime("%Y-%m-%dT%H:%M:%S")
                            + "-05:00", int(r)])
    with open(bronze, "rb") as fh:
        filas = sum(1 for _ in fh) - 1
    return {"bronze": str(bronze), "filas_largas": filas, "sembrado_por_regla": esperado}


# ---------------------------------------------------------------------------------------------
# Bronze en el formato real de OEFA (el que lee configs/limpieza.json; ver data/sample/README.md)
# ---------------------------------------------------------------------------------------------

def _campo(columna: str, valor: str) -> str:
    """Como el CSV real: textos entre comillas; coordenadas, ubigeo, fechas y horas sin ellas."""
    if columna in SIN_COMILLAS:
        return valor
    return '"' + valor.replace('"', '""') + '"'


def _hora(h) -> str:
    """HORA_INICIO del CSV real: NA sin hora; «1899-12-31» es la medianoche; si no, «1899-12-31 HH:00:00»."""
    if h is None:
        return "NA"
    return "1899-12-31" if h == 0 else f"1899-12-31 {h:02d}:00:00"


def escribir_bronze(ruta: Path, rng, dias: int, verdad: dict, presente: dict) -> dict:
    """Escribe el bronze largo con el esquema real: 36 columnas, «;», CRLF, una columna por parámetro y un
    solo valor por fila. Un día (estación, parámetro, fecha, unidad, periodo) con sus 24 horas va en 24
    filas contiguas sin hora (NA: Go la infiere por posición); un día con huecos, con la hora explícita
    de inicio de cada lectura. La suciedad que se siembra no rompe los bloques de 24: las reglas de valor
    van dentro (la fila conserva su posición) y el resto, en filas sueltas con hora explícita."""
    info = {}
    for est in ESTACIONES:
        num = int("".join(c for c in est["zona"] if c.isdigit()))
        e, n = latlon_a_utm(est["lat"], est["lon"], num)
        info[est["nombre"]] = dict(est, este=e, norte=n, zona_num=num)

    esperado = {r: 0 for r in ["columnas", "estacion", "periodo", "parametro", "fecha", "fecha_fuera_de_rango", "hora",
                               "valor_no_numerico", "centinela", "unidad", "negativo", "fuera_de_rango", "bloque_en_cero",
                               "duplicado_unidad", "duplicado_exacto", "duplicado_valor_distinto"]}

    def unidad_de(nombre, p):
        if p == "co" and nombre == "SIN-LA-OROYA":
            return "mg/m3"           # factor 1000 en configs/limpieza.json
        if p == "presion" and nombre == "SIN-ILO-PUERTO":
            return "mmHg"            # Go la convierte a hPa (marca unidad_convertida)
        return UNIDAD.get(p, "ug/m3")

    def valor_de(nombre, p, x):
        u = unidad_de(nombre, p)
        if u == "mg/m3":
            return f"{x / 1000:.3f}"
        if u == "mmHg":
            return f"{x / 1.33322:.1f}"
        return f"{x:.{DECIMALES[p]}f}"

    def periodo_de(nombre, p):
        # Meteorología con periodo «-» en una estación: horaria solo con hora o con su bloque completo.
        return "-" if (nombre == "SIN-CALLAO-NORTE" and p in METEO_SIN) else "1 hora"

    def fila(nombre, p, dia, h, valor, unidad=None, periodo=None, fecha=None, valores=None):
        st = info[nombre] if nombre in info else info[ESTACIONES[0]["nombre"]]
        c = {
            "NOMBRE_EVALUACION": f"Vigilancia sintética {st['dist'].title()}", "COMPONENTE_AMBIENTAL": "Aire",
            "PROCEDENCIA_MUESTRA": "Aire ambiental", "NOMBRE_PUNTO": nombre, "ESTE": str(st["este"]),
            "NORTE": str(st["norte"]), "ALTITUD": str(st["alt"]), "ZONA": str(st["zona_num"]), "DATUM": "WGS84",
            "DEPARTAMENTO": st["dep"], "PROVINCIA": st["prov"], "DISTRITO": st["dist"], "UBIGEO": st["ubigeo"],
            "DESCRIPCION_UBICACION": f"Punto sintético {nombre}: no es una estación de OEFA.", "TIPO_MUESTRA": "Simple",
            "TIPO_ANALISIS": TIPO_ANALISIS.get(p, "Datos Meteorológicos"),
            "PERIODO": periodo if periodo is not None else periodo_de(nombre, p),
            "FECHA_INICIO": fecha if fecha is not None else dia, "HORA_INICIO": _hora(h), "FECHA_FIN": "NA",
            "HORA_FIN": "NA", "UNIDAD_MEDIDA": unidad or unidad_de(nombre, p), "FECHA_CORTE": "20240429",
        }
        for col in COLUMNAS_PARAMETRO:
            c[col] = "-"
        if valores is not None:  # más de un parámetro con valor (regla parametro)
            c.update(valores)
        elif p is not None:
            c[COLUMNA_PARAM[p]] = valor
        return ";".join(_campo(k, c[k]) for k in COLUMNAS)

    dias_txt = [str(np.datetime64(INICIO, "D") + np.timedelta64(d, "D")) for d in range(dias)]
    # Un día fuera de los bloques (después del último): las filas sueltas que caen en él no completan ni
    # rompen ningún día de 24 filas.
    dia_libre = str(np.datetime64(INICIO, "D") + np.timedelta64(dias + 2, "D"))
    # El día entero en 0: el primer día completo de PM10 en SIN-HUAYHUAY desde el cuarto.
    hay_h = presente.get(("SIN-HUAYHUAY", "pm10"), np.zeros(0, dtype=bool))
    dia_cero = next((d for d in range(3, dias) if hay_h[24 * d: 24 * d + 24].all()), None)
    bloques = []        # cada bloque: lista de líneas contiguas
    explicitas = []     # (estacion, parametro, dia, hora, valor): candidatas a duplicado
    for est in ESTACIONES:
        nombre = est["nombre"]
        for p in est["params"]:
            v, pres = verdad[(nombre, p)], presente[(nombre, p)]
            for d in range(dias):
                sl = slice(24 * d, 24 * d + 24)
                vd, hay = v[sl], pres[sl]
                if not hay.any():
                    continue
                txt = [valor_de(nombre, p, x) for x in vd]
                completo = bool(hay.all())
                en_cero = nombre == "SIN-HUAYHUAY" and p == "pm10" and d == dia_cero
                if en_cero:
                    txt = ["0"] * 24  # un día entero en 0 exacto: equipo sin medir
                    esperado["bloque_en_cero"] += 24
                # Suciedad de valor dentro del día (la fila conserva su posición): pocas por bloque.
                for h in np.flatnonzero(hay & (rng.random(24) < 0.0015) & (not en_cero)):
                    regla = rng.choice(["centinela", "valor_no_numerico", "negativo", "fuera_de_rango"])
                    if regla == "negativo" and p == "temperatura":
                        continue
                    if regla == "fuera_de_rango" and p not in FUERA:
                        continue
                    txt[h] = {"centinela": rng.choice(["-999", "-9999"]), "valor_no_numerico": rng.choice(["ND", "<LD"]),
                              "negativo": "-3.5", "fuera_de_rango": FUERA.get(p, "")}[regla]
                    esperado[regla] += 1
                if completo and rng.random() > 0.1:  # el 90 % de los días completos, sin hora (como el CSV real)
                    bloques.append([fila(nombre, p, dias_txt[d], None, t) for t in txt])
                else:
                    filas_d = []
                    for h in np.flatnonzero(hay):
                        filas_d.append(fila(nombre, p, dias_txt[d], int(h), txt[h]))
                        explicitas.append((nombre, p, dias_txt[d], int(h), txt[h]))
                    bloques.append(filas_d)
                # La misma lectura en ppb cuando está en µg/m³ (gases): Go se queda con la de µg/m³.
                if p == "so2" and nombre == "SIN-CALLAO-SUR" and completo and d % 7 == 0:
                    bloques.append([fila(nombre, p, dias_txt[d], None, f"{x / 2.62:.2f}", unidad="ppb") for x in vd])
                    esperado["duplicado_unidad"] += 24
            # Un día de SO2 en una unidad que no figura para el parámetro (regla unidad), entero.
            if p == "so2" and nombre == "SIN-LA-OROYA":
                d = dias // 2
                bloques.append([fila(nombre, p, dias_txt[d], h, f"{verdad[(nombre, p)][24 * d + h] / 2620:.4f}", unidad="ppm")
                                for h in range(24)])
                esperado["unidad"] += 24

    # Filas sueltas que la limpieza descarta enteras (con hora explícita: no tocan ningún bloque).
    sueltas = []
    for est in ESTACIONES:  # periodo: promedios de 24 horas
        nombre = est["nombre"]
        for d in range(0, dias, 3):
            x = float(np.mean(verdad[(nombre, "pm10")][24 * d: 24 * d + 24]))
            sueltas.append(fila(nombre, "pm10", dias_txt[d], None, f"{x:.1f}", periodo="24 horas"))
            esperado["periodo"] += 1
    n0 = ESTACIONES[0]["nombre"]
    for j, f_mala in enumerate(["2023-02-31", "31/02/2023", "sin fecha", "NA"] * 5):  # fecha
        sueltas.append(fila(n0, "pm10", None, j % 24, "40.0", fecha=f_mala))
        esperado["fecha"] += 1
    for j, f_fuera in enumerate(["1999-12-31", "2026-08-15"] * 6):  # fecha_fuera_de_rango
        sueltas.append(fila(ESTACIONES[j % len(ESTACIONES)]["nombre"], "pm10", None, 5, "40.0", fecha=f_fuera))
        esperado["fecha_fuera_de_rango"] += 1
    for j in range(15):  # parametro: ninguna columna con valor, o dos
        dos = j % 3 == 0
        sueltas.append(fila(n0, None, dia_libre, j % 24, "", valores={"PM10": "35.0", "PM2.5": "20.0"} if dos else None))
        esperado["parametro"] += 1
    for j in range(5):  # estacion vacía
        sueltas.append(fila("", "pm10", dia_libre, j, "30.0"))
        esperado["estacion"] += 1
    for j in range(5):  # hora ilegible (en el día libre: en un día de 24 filas sin hora lo dejaría incompleto)
        sueltas.append(fila(n0, "pm10", dia_libre, None, "30.0").replace(";NA;NA;NA;", ";1899-12-31 xx:00:00;NA;NA;", 1))
        esperado["hora"] += 1
    for j in range(3):  # columnas: la fila tiene menos campos que la cabecera
        sueltas.append(";".join(fila(n0, "pm10", dia_libre, 8 + j, "30.0").split(";")[:20]))
        esperado["columnas"] += 1

    # Orden de archivo: bloques contiguos; las sueltas, entre bloques; duplicados al final (solo filas con
    # hora explícita: una fila de más en un día sin hora lo dejaría incompleto).
    lineas = []
    cuantas = Counter(rng.integers(0, len(bloques) + 1, size=len(sueltas)).tolist())
    cola = iter(sueltas)
    for i in range(len(bloques) + 1):
        lineas.extend(next(cola) for _ in range(cuantas.get(i, 0)))
        if i < len(bloques):
            lineas.extend(bloques[i])
    validas = [e for e in explicitas if _es_numero(e[4]) and e[4] not in FUERA.values()]
    for j, idx in enumerate(rng.choice(len(validas), size=min(200, len(validas)), replace=False)):
        nombre, p, dia, h, t = validas[idx]
        if j < 50 and p in ("pm10", "pm2_5", "so2"):
            t = f"{float(t) + 7.3:.{DECIMALES[p]}f}"
            esperado["duplicado_valor_distinto"] += 1
        else:
            esperado["duplicado_exacto"] += 1
        lineas.append(fila(nombre, p, dia, h, t))

    ruta.parent.mkdir(parents=True, exist_ok=True)
    with open(ruta, "w", encoding="utf-8", newline="") as fh:
        fh.write(";".join('"' + c + '"' for c in COLUMNAS) + "\r\n")
        for l in lineas:
            fh.write(l + "\r\n")
    return esperado


def _es_numero(s: str) -> bool:
    try:
        float(s)
        return not s.startswith("-")
    except ValueError:
        return False


def correr_go(raiz: Path, bronze: str, workers: int = 0) -> None:
    """Corre el sistema de verdad: cmd/cargar y cmd/entrenar (con el codo) sobre el bronze, con las configs
    del repo y TODAS las salidas dentro de <raiz> (también los centroides iniciales de cmd/entrenar)."""
    repo = Path(__file__).resolve().parent.parent
    go = os.environ.get("GO") or shutil.which("go") or "/snap/go/current/bin/go"
    if not Path(go).exists() and not shutil.which(go):
        raise SystemExit("No encuentro el binario de Go (variable GO o go en el PATH).")
    r = lambda *a: str(raiz.joinpath(*a))
    centroides = raiz / "reports" / "centroides_iniciales.json"
    centroides.unlink(missing_ok=True)  # son del gold anterior: cmd/entrenar los regenera
    pasos = [
        [go, "run", "./cmd/cargar", "-entrada", str(bronze), "-silver", r("data", "silver", "oefa_ancho.csv"),
         "-gold", r("data", "gold", "oefa_features.csv"), "-reports", r("reports"), "-workers", str(workers)],
        [go, "run", "./cmd/entrenar", "-gold", r("data", "gold", "oefa_features.csv"),
         "-estandarizacion", r("reports", "estandarizacion.json"), "-salida", r("reports", "kmeans.json"),
         "-asignaciones", r("data", "gold", "asignaciones.csv"), "-regimenes", r("reports", "regimenes.json"),
         "-centroides", str(centroides), "-codo", "2:10", "-codo-salida", r("reports", "codo.json")],
    ]
    for p in pasos:
        print("$", " ".join(p[1:3]), "…")
        subprocess.run(p, cwd=repo, check=True, stdout=subprocess.DEVNULL)


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("raiz", type=Path, help="directorio raíz de las capas (se crea si no existe)")
    ap.add_argument("--semilla", type=int, default=20261031)
    ap.add_argument("--dias", type=int, default=60)
    ap.add_argument("--go", action="store_true", help="correr cmd/cargar y cmd/entrenar sobre el bronze")
    a = ap.parse_args()
    a.raiz.mkdir(parents=True, exist_ok=True)
    res = generar(a.raiz, a.semilla, a.dias)
    print(json.dumps(res, indent=2, ensure_ascii=False))
    if a.go:
        correr_go(a.raiz, res["bronze"])
        lim = json.loads((a.raiz / "reports" / "limpieza.json").read_text(encoding="utf-8"))
        print("Descartadas por regla (Go):", lim["filas_largas"]["descartadas_por_regla"])


if __name__ == "__main__":
    main()
