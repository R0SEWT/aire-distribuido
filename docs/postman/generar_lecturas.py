#!/usr/bin/env python3
"""Genera docs/postman/lecturas_prueba.csv: los datos de prueba de la API (spec de la PC4, §9).

Lo esperado de cada fila se calcula aquí, sin preguntarle a la API:

- regimen_esperado sale de data/gold/asignaciones.csv, la asignación que dejó cmd/entrenar al
  entrenar el modelo sembrado (reports/kmeans.json).
- pm2_5 y pm10 son las concentraciones crudas, en µg/m³, de data/silver/oefa_ancho.csv, tal cual
  están escritas ahí.
- lecturas_esperadas, las medias, alerta_esperada e inca_esperada se calculan con la regla del
  contrato (docs/api/openapi.yaml, esquemas Media24h, Alerta y BandaINCA): ventana (t − 24 h, t]
  de la estación, media desde 18 lecturas, ECA de 24 h (PM2.5 > 50 o PM10 > 100 µg/m³) e INCA
  (la peor banda entre PM10 ≤ 75, ≤ 150, ≤ 250 y PM2.5 ≤ 12.5, ≤ 25, ≤ 125).

Qué filas trae:

- regimen_0, regimen_1, regimen_2: diez lecturas reales por régimen, repartidas entre las
  estaciones del gold y separadas más de 48 h de cualquier otra fila de su estación. Cada una
  queda sola en su ventana: 1 lectura, sin media, sin alerta y sin INCA.
- serie_alerta: 24 horas seguidas de CA-HU-04 cuya media de 24 h supera el ECA.
- serie_sin_alerta: 24 horas seguidas de CA-ROM-01 con INCA «mala» por PM2.5 y sin alerta: el
  INCA y el ECA se informan aparte.

Las dos estaciones de las series no aparecen en las filas de régimen. Repetir el archivo deja el
mismo estado en la API (la misma hora reemplaza a la anterior), así que el Runner y cmd/carga
pueden correrlo las veces que haga falta y en cualquier orden.

Uso, desde la raíz del repositorio y con el silver y el gold en data/ (no se versionan):

    python3 docs/postman/generar_lecturas.py [--datos data] [--salida docs/postman/lecturas_prueba.csv]

Solo usa la biblioteca estándar. La selección es determinista (semilla fija).
"""

import argparse
import collections
import csv
import datetime as dt
import random
import sys

POR_REGIMEN = 10
SEPARACION = dt.timedelta(hours=48)
MINIMO_LECTURAS = 18
SEMILLA = 42

# (caso, estación, última hora de la serie, alerta esperada al final, INCA esperado al final)
SERIES = [
    ("serie_alerta", "CA-HU-04", "2021-12-11T00:00:00-05:00", True, "mala"),
    ("serie_sin_alerta", "CA-ROM-01", "2022-06-05T00:00:00-05:00", False, "mala"),
]

BANDAS = ["buena", "moderada", "mala", "umbral_de_cuidado"]
CORTES_PM10 = [75, 150, 250]
CORTES_PM25 = [12.5, 25, 125]
ECA_PM25, ECA_PM10 = 50, 100

COLUMNAS = [
    "caso", "estacion", "fecha_hora", "pm2_5", "pm10", "regimen_esperado", "lecturas_esperadas",
    "media_pm2_5_esperada", "media_pm10_esperada", "alerta_esperada", "inca_esperada",
]


def banda(x, cortes):
    for i, c in enumerate(cortes):
        if x <= c:
            return i
    return len(cortes)


def esperado(lecturas):
    """Media, alerta e INCA de una ventana: lecturas es la lista de (pm2_5, pm10) en orden de hora."""
    n = len(lecturas)
    if n < MINIMO_LECTURAS:
        return n, "", "", False, ""
    s25 = s10 = 0.0
    for a, b in lecturas:  # el mismo orden de suma que ventana.calcular
        s25 += a
        s10 += b
    m25, m10 = s25 / n, s10 / n
    alerta = m25 > ECA_PM25 or m10 > ECA_PM10
    inca = BANDAS[max(banda(m10, CORTES_PM10), banda(m25, CORTES_PM25))]
    return n, repr(m25), repr(m10), alerta, inca


def main():
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("--datos", default="data", help="carpeta con silver/oefa_ancho.csv y gold/asignaciones.csv")
    ap.add_argument("--salida", default="docs/postman/lecturas_prueba.csv")
    args = ap.parse_args()

    # Régimen de cada fila del gold.
    regimen = {}
    with open(f"{args.datos}/gold/asignaciones.csv", newline="") as f:
        for r in csv.DictReader(f):
            regimen[(r["estacion"], r["fecha_hora"])] = int(r["regimen"])

    # PM crudo de esas mismas filas, con el texto original del silver.
    crudo = {}
    with open(f"{args.datos}/silver/oefa_ancho.csv", newline="") as f:
        for r in csv.DictReader(f):
            clave = (r["estacion"], r["fecha_hora"])
            if clave in regimen:
                crudo[clave] = (r["pm2_5"], r["pm10"])
    faltan = len(regimen) - len(crudo)
    if faltan:
        sys.exit(f"{faltan} filas del gold no están en el silver: no son de la misma corrida")

    filas = []
    usadas = collections.defaultdict(list)  # estación → horas ya elegidas
    en_series = {s[1] for s in SERIES}

    # Diez lecturas por régimen, una estación por vuelta.
    for g in sorted(set(regimen.values())):
        candidatas = collections.defaultdict(list)
        for (est, fh), reg in sorted(regimen.items()):
            if reg == g and est not in en_series:
                candidatas[est].append(fh)
        azar = random.Random(SEMILLA + g)
        for est in candidatas:
            azar.shuffle(candidatas[est])
        elegidas = []
        while len(elegidas) < POR_REGIMEN:
            antes = len(elegidas)
            for est in sorted(candidatas):
                while candidatas[est]:
                    fh = candidatas[est].pop()
                    t = dt.datetime.fromisoformat(fh)
                    if all(abs(t - u) > SEPARACION for u in usadas[est]):
                        usadas[est].append(t)
                        elegidas.append((est, fh))
                        break
                if len(elegidas) == POR_REGIMEN:
                    break
            if len(elegidas) == antes:
                sys.exit(f"no hay {POR_REGIMEN} lecturas separadas del régimen {g}")
        for est, fh in sorted(elegidas):
            pm25, pm10 = crudo[(est, fh)]
            n, m25, m10, alerta, inca = esperado([(float(pm25), float(pm10))])
            filas.append([f"regimen_{g}", est, fh, pm25, pm10, g, n, m25, m10, str(alerta).lower(), inca])

    # Las dos series de 24 horas seguidas.
    for caso, est, fin, alerta_final, inca_final in SERIES:
        t_fin = dt.datetime.fromisoformat(fin)
        horas = [(t_fin - dt.timedelta(hours=h)).isoformat() for h in range(23, -1, -1)]
        ventana = []
        for fh in horas:
            if (est, fh) not in crudo:
                sys.exit(f"{caso}: falta la lectura {est} {fh}")
            pm25, pm10 = crudo[(est, fh)]
            ventana.append((float(pm25), float(pm10)))
            n, m25, m10, alerta, inca = esperado(ventana)
            filas.append([caso, est, fh, pm25, pm10, regimen[(est, fh)], n, m25, m10, str(alerta).lower(), inca])
        if (alerta, inca) != (alerta_final, inca_final):
            sys.exit(f"{caso}: termina con alerta={alerta} e inca={inca}; se esperaba {alerta_final} y {inca_final}")

    with open(args.salida, "w", newline="") as f:
        w = csv.writer(f, lineterminator="\n")
        w.writerow(COLUMNAS)
        w.writerows(filas)
    print(f"{args.salida}: {len(filas)} filas")


if __name__ == "__main__":
    main()
