#!/usr/bin/env python3
"""Genera los datos de impacto y las series del frontend (spec del frontend del TB2, §5.4, §5.7 y §7).

Escribe tres archivos, los tres versionados:

- reports/impacto.json y su copia idéntica web/public/datos/impacto.json: cuántas horas superan el
  ECA y cómo se reparten en las bandas INCA, por estación, por tipo de actividad y por distrito; la
  población censada de cada distrito como contexto (sin sumarla: §7.2); qué acertaría cada señal
  si se usara como alerta; y la anticipación acotada de §7.4.
- web/public/datos/series.json: el escenario (48 horas reales seguidas de diciembre de 2021) y los
  días sueltos de la simulación, con lo que la API tiene que responder en la última lectura de cada
  estación.

De dónde sale cada cosa, sin preguntarle a la API:

- Una lectura es una fila del gold: una hora de una estación con PM2.5 y PM10. Sus concentraciones
  son las crudas del silver (data/silver/oefa_ancho.csv), tal cual están escritas ahí, y su régimen
  es el de data/gold/asignaciones.csv. Una hora sin PM2.5 o sin PM10 no está en el gold y no entra,
  igual que en docs/postman/generar_lecturas.py: la API tampoco la aceptaría.
- La media de 24 h, la alerta y la banda INCA se calculan con la regla de ventana/ventana.go, que
  aquí está portada línea por línea (clase Ventanas): ventana (t − 24 h, t], media desde 18
  lecturas, alerta si PM2.5 > 50 o PM10 > 100 µg/m³, INCA la peor banda entre PM10 ≤ 75, ≤ 150,
  ≤ 250 y PM2.5 ≤ 12,5, ≤ 25, ≤ 125.
- Los totales tienen que coincidir exactamente con reports/alerta_24h.json, que escribe la prueba
  de Go sobre el mismo gold. Si no coinciden, el script falla y no escribe nada.
- La población es la de reports/poblacion_distritos.csv (INEI, Censos Nacionales 2017), cruzada
  por ubigeo.

Uso, desde la raíz del repositorio. El silver y el gold no se versionan: van en data/ o donde diga
--datos.

    python3 scripts/impacto.py [--datos data]      # escribe los tres archivos
    python3 scripts/impacto.py --verificar         # recalcula y compara con lo versionado

Volver a correrlo sobre los mismos datos no cambia los archivos (la fecha de `metadatos.generado`
solo se mueve si cambia algo más). Solo usa la biblioteca estándar. Las pruebas están en
scripts/test_impacto.py.
"""

import argparse
import collections
import csv
import datetime as dt
import hashlib
import json
import math
import sys
from pathlib import Path

RAIZ = Path(__file__).resolve().parents[1]

# --- La regla de ventana/ventana.go y los topes de la API (api/rutas.go) -------------------------

VENTANA_HORAS = 24
MINIMO_LECTURAS = 18
GUARDADAS = 72
ECA_PM25, ECA_PM10 = 50, 100
BANDAS = ["buena", "moderada", "mala", "umbral_de_cuidado"]
CORTES_PM10 = [75, 150, 250]
CORTES_PM25 = [12.5, 25, 125]
PM_MAXIMO = 10000
LIMA = dt.timezone(dt.timedelta(hours=-5))

# --- El escenario y los días sueltos (§5.4 y §7.3) -----------------------------------------------

HORAS_TRAMO = 48
SEPARACION_HORAS = 48
# El escenario se busca entre los días 13 y 31 de diciembre de 2021 (sellos de fin de hora).
ESCENARIO_DESDE = "2021-12-13T01:00:00-05:00"
ESCENARIO_HASTA = "2022-01-01T00:00:00-05:00"

# Los días sueltos se eligieron a mano mirando el gold; el script comprueba que cada uno termine
# como dice aquí y falla si no. Cada tupla: caso, estación, último sello de sus 48 horas, alerta e
# INCA con que tiene que terminar y cuántas de las 48 horas le faltan.
DIAS = [
    ("limpio", "CA-HUA-01", "2021-12-21T00:00:00-05:00", False, "buena", 0),
    ("limpio", "CA-JUPR-01", "2022-05-04T00:00:00-05:00", False, "buena", 0),
    ("mala_sin_alerta", "CA-SOC-01", "2021-12-20T00:00:00-05:00", False, "mala", 0),
    ("cruza_eca", "CA-ESP-01", "2021-12-22T00:00:00-05:00", True, "moderada", 0),
    ("cruza_eca", "CA-HU-09", "2021-12-22T00:00:00-05:00", True, "mala", 0),
    ("en_alerta", "CA-HU-04", "2021-12-20T00:00:00-05:00", True, "mala", 0),
    ("umbral_de_cuidado", "CA-RZC-02", "2021-12-22T00:00:00-05:00", True, "umbral_de_cuidado", 1),
    ("huecos_con_media", "CA-CG-02", "2022-06-03T00:00:00-05:00", False, "moderada", 5),
    ("huecos_sin_media", "CA-MAC-01", "2022-05-27T00:00:00-05:00", False, None, 7),
]

CASOS = {
    "limpio": "Día limpio: la media de 24 h nunca supera el ECA y la banda INCA es buena de principio a fin.",
    "mala_sin_alerta": "INCA mala sin alerta: la media de PM2.5 pasa de 25 µg/m³ y no llega al ECA. "
                       "El INCA y el ECA se informan aparte.",
    "cruza_eca": "Cruza el ECA: la media de 24 h aparece bajo el ECA y la alerta se enciende durante el segundo día.",
    "en_alerta": "En alerta todo el tramo: la primera media de 24 h ya supera el ECA.",
    "umbral_de_cuidado": "Umbral de cuidado: la media de 24 h de PM2.5 pasa de 125 µg/m³. Le falta una hora.",
    "huecos_con_media": "Con huecos y con media: faltan horas, pero la ventana final junta 18 lecturas o más.",
    "huecos_sin_media": "Con huecos y sin media al final: la media aparece, un hueco de varias horas deja la "
                        "ventana con menos de 18 lecturas y el día termina sin media, sin alerta y sin INCA.",
}

RUTAS = {
    "gold": "gold/oefa_features.csv",
    "asignaciones": "gold/asignaciones.csv",
    "silver": "silver/oefa_ancho.csv",
}
FICHA = "reports/estaciones_ficha.csv"
POBLACION = "reports/poblacion_distritos.csv"
ALERTA_24H = "reports/alerta_24h.json"
REGIMENES = "reports/regimenes.json"
SALIDA_IMPACTO = "reports/impacto.json"
COPIA_IMPACTO = "web/public/datos/impacto.json"
SALIDA_SERIES = "web/public/datos/series.json"


class ErrorDatos(Exception):
    """Los datos no son lo que el script espera: no se escribe nada."""


# --- Horas y fechas ------------------------------------------------------------------------------

def hora_de(fecha_hora):
    """Horas desde la época Unix de una fecha_hora RFC 3339: la clave de una lectura en ventana.go."""
    return int(dt.datetime.fromisoformat(fecha_hora).timestamp()) // 3600


def sello(hora):
    """La fecha_hora de una hora, con la zona de Lima, como la escribe el silver."""
    return dt.datetime.fromtimestamp(hora * 3600, LIMA).isoformat()


def dia_de(hora):
    """El día de una lectura. fecha_hora es el FIN del promedio horario (convencion_hora = fin en
    reports/regimenes.json), así que el día es el de la hora anterior: 01:00 … 24:00 son un mismo
    día y el sello 00:00 pertenece al día que termina."""
    return dt.datetime.fromtimestamp((hora - 1) * 3600, LIMA).date().isoformat()


# --- La ventana de 24 h --------------------------------------------------------------------------

def banda(x, cortes):
    for i, c in enumerate(cortes):
        if x <= c:
            return i
    return len(cortes)


def calcular(puntos):
    """Media24h de ventana.calcular: suma en orden de hora y divide. Sin 18 lecturas no hay media."""
    n = len(puntos)
    if n < MINIMO_LECTURAS:
        return {"pm2_5": None, "pm10": None, "lecturas": n}
    s25 = s10 = 0.0
    for _, a, b in puntos:
        s25 += a
        s10 += b
    return {"pm2_5": s25 / n, "pm10": s10 / n, "lecturas": n}


def alerta(media):
    return media["pm2_5"] is not None and (media["pm2_5"] > ECA_PM25 or media["pm10"] > ECA_PM10)


def inca(media):
    if media["pm2_5"] is None:
        return None
    return BANDAS[max(banda(media["pm10"], CORTES_PM10), banda(media["pm2_5"], CORTES_PM25))]


class Ventanas:
    """ventana.Ventanas: por estación, las 72 lecturas horarias más cercanas a la última recibida."""

    def __init__(self):
        self.por_estacion = {}

    def agregar(self, estacion, hora, pm25, pm10):
        w = self.por_estacion.setdefault(estacion, [])
        i = 0
        while i < len(w) and w[i][0] < hora:
            i += 1
        if i < len(w) and w[i][0] == hora:
            w[i] = (hora, pm25, pm10)
        else:
            w.insert(i, (hora, pm25, pm10))
            if len(w) > GUARDADAS:
                # Sale la más lejana a la que acaba de llegar; a igual distancia, la más vieja.
                if hora - w[0][0] >= w[-1][0] - hora:
                    del w[0]
                else:
                    w.pop()
        return calcular([p for p in w if hora - VENTANA_HORAS < p[0] <= hora])


# --- Lectura de las fuentes ----------------------------------------------------------------------

def sha256(ruta):
    h = hashlib.sha256()
    with open(ruta, "rb") as f:
        for bloque in iter(lambda: f.read(1 << 20), b""):
            h.update(bloque)
    return h.hexdigest()


def leer_json(ruta):
    return json.loads(Path(ruta).read_text(encoding="utf-8"))


def leer_csv(ruta):
    with open(ruta, newline="", encoding="utf-8") as f:
        return list(csv.DictReader(f))


def leer_lecturas(datos):
    """Las lecturas del gold con su PM crudo del silver y su régimen: estación → lista ordenada por
    hora de (hora, pm2_5, pm10, régimen). Devuelve también cuántas filas tiene cada archivo."""
    datos = Path(datos)
    faltan = [str(datos / r) for r in RUTAS.values() if not (datos / r).is_file()]
    if faltan:
        raise ErrorDatos("faltan datos sin versionar: " + ", ".join(faltan))
    with open(datos / RUTAS["gold"], newline="", encoding="utf-8") as f:
        filas = csv.reader(f)
        if next(filas)[:2] != ["estacion", "fecha_hora"]:
            raise ErrorDatos("el gold no empieza con estacion,fecha_hora")
        claves = [(r[0], r[1]) for r in filas]
    regimen = {}
    with open(datos / RUTAS["asignaciones"], newline="", encoding="utf-8") as f:
        for i, r in enumerate(csv.DictReader(f)):
            if i >= len(claves) or (r["estacion"], r["fecha_hora"]) != claves[i]:
                raise ErrorDatos(f"asignaciones.csv no sigue al gold en la fila {i + 2}: no son de la misma corrida")
            regimen[claves[i]] = int(r["regimen"])
    if len(regimen) != len(claves):
        raise ErrorDatos(f"el gold tiene {len(claves)} filas, hay {len(regimen)} distintas en asignaciones.csv")
    crudo, filas_silver, completas = {}, 0, 0
    with open(datos / RUTAS["silver"], newline="", encoding="utf-8") as f:
        for r in csv.DictReader(f):
            filas_silver += 1
            clave = (r["estacion"], r["fecha_hora"])
            completas += r["pm2_5"] != "" and r["pm10"] != ""
            if clave in regimen:
                crudo[clave] = (r["pm2_5"], r["pm10"])
    if len(crudo) != len(regimen):
        raise ErrorDatos(f"{len(regimen) - len(crudo)} filas del gold no están en el silver: no son de la misma corrida")
    if completas != len(regimen):
        raise ErrorDatos(f"el silver tiene {completas} horas con PM2.5 y PM10 y el gold {len(regimen)}")
    lecturas = collections.defaultdict(list)
    for estacion, fecha_hora in claves:
        hora = hora_de(fecha_hora)
        if sello(hora) != fecha_hora:
            raise ErrorDatos(f"{estacion} {fecha_hora}: se esperaba una hora en punto con zona -05:00")
        try:
            pm25, pm10 = (float(x) for x in crudo[(estacion, fecha_hora)])
        except ValueError:
            raise ErrorDatos(f"{estacion} {fecha_hora}: PM sin valor en el silver") from None
        if not (0 <= pm25 <= PM_MAXIMO and 0 <= pm10 <= PM_MAXIMO):
            raise ErrorDatos(f"{estacion} {fecha_hora}: PM fuera de lo que acepta la API ({pm25}, {pm10})")
        lecturas[estacion].append((hora, pm25, pm10, regimen[(estacion, fecha_hora)]))
    for estacion, serie in lecturas.items():
        serie.sort()
        if any(a[0] == b[0] for a, b in zip(serie, serie[1:])):
            raise ErrorDatos(f"{estacion}: una hora repetida en el gold")
    return dict(lecturas), {"gold": len(claves), "asignaciones": len(regimen), "silver": filas_silver}


def regimen_que_excede(regimenes):
    excede = [r["regimen"] for r in regimenes["regimenes"] if r["eca"] == "excede"]
    if len(excede) != 1:
        raise ErrorDatos(f"se esperaba un solo régimen que excede el ECA en {REGIMENES}; hay {excede}")
    return excede[0]


# --- Estado de cada hora -------------------------------------------------------------------------

def estados(lecturas, alto):
    """Por estación y en orden de hora, lo que la regla dice de cada lectura: (hora, media de 24 h,
    lectura horaria sobre el umbral, lectura en el régimen que excede)."""
    ventanas = Ventanas()
    return {
        estacion: [(h, ventanas.agregar(estacion, h, a, b), a > ECA_PM25 or b > ECA_PM10, g == alto)
                   for h, a, b, g in serie]
        for estacion, serie in sorted(lecturas.items())
    }


def conteo(filas):
    """Lecturas, horas calculables, horas sobre el ECA y reparto por banda INCA de unas horas."""
    c = {"lecturas": 0, "horas_calculables": 0, "horas_sobre_eca": 0, "inca": dict.fromkeys(BANDAS, 0)}
    for _, media, _, _ in filas:
        c["lecturas"] += 1
        if media["pm2_5"] is None:
            continue
        c["horas_calculables"] += 1
        c["horas_sobre_eca"] += alerta(media)
        c["inca"][inca(media)] += 1
    return c


def sumar(conteos):
    conteos = list(conteos)
    t = {k: sum(c[k] for c in conteos) for k in ("lecturas", "horas_calculables", "horas_sobre_eca")}
    t["inca"] = {b: sum(c["inca"][b] for c in conteos) for b in BANDAS}
    return t


def banda_mas_frecuente(reparto):
    """La banda INCA con más horas; si empatan, la peor. Sin horas calculables no hay."""
    if not any(reparto.values()):
        return None
    return max(BANDAS, key=lambda b: (reparto[b], BANDAS.index(b)))


def senales(por_estacion, total):
    """En cuántas horas calculables estaba encendida cada señal y en cuántas de esas la media de
    24 h superaba el ECA: lo mismo que mide TestLaVentanaSobreElGold."""
    cuenta = {"regimen_alto": [0, 0], "lectura_horaria": [0, 0]}
    for filas in por_estacion.values():
        for _, media, horaria, en_alto in filas:
            if media["pm2_5"] is None:
                continue
            for nombre, encendida in (("regimen_alto", en_alto), ("lectura_horaria", horaria)):
                if encendida:
                    cuenta[nombre][0] += 1
                    cuenta[nombre][1] += alerta(media)
    return {
        nombre: {"horas": horas, "aciertos": aciertos,
                 "precision": aciertos / horas if horas else None,
                 "cobertura": aciertos / total["horas_sobre_eca"] if total["horas_sobre_eca"] else None}
        for nombre, (horas, aciertos) in cuenta.items()
    }


# --- Anticipación acotada (§7.4) -----------------------------------------------------------------

def cuantil(ordenados, q):
    """Cuantil con interpolación lineal entre los dos valores vecinos (el método «inclusive» de
    statistics.quantiles y el que usa numpy por omisión)."""
    pos = (len(ordenados) - 1) * q
    abajo = math.floor(pos)
    arriba = min(abajo + 1, len(ordenados) - 1)
    return ordenados[abajo] + (ordenados[arriba] - ordenados[abajo]) * (pos - abajo)


def resumen(valores):
    if not valores:
        return None
    v = sorted(valores)
    return {"n": len(v), "minimo": v[0], "q1": cuantil(v, 0.25), "mediana": cuantil(v, 0.5),
            "q3": cuantil(v, 0.75), "maximo": v[-1]}


def episodios_de(estacion, filas):
    """Los episodios de una estación y cuántas veces su media de 24 h cruza el ECA hacia arriba.

    Un episodio empieza en la hora t si la media de 24 h en t es calculable y supera el ECA y en
    las 24 horas anteriores, [t − 24 h, t), la estación no tuvo ninguna hora con media calculable
    sobre el ECA. Toda hora sobre el ECA pertenece a un episodio y solo a uno."""
    sobre = {h for h, media, _, _ in filas if alerta(media)}
    calculable = {h for h, media, _, _ in filas if media["pm2_5"] is not None}
    horaria = {h for h, _, encendida, _ in filas if encendida}
    en_alto = {h for h, _, _, encendida in filas if encendida}
    episodios, cruces, anterior = [], 0, False
    for h, media, _, _ in filas:
        if media["pm2_5"] is None:
            continue
        ahora = alerta(media)
        cruces += ahora and not anterior  # la hora calculable anterior no superaba el ECA, o no hay
        anterior = ahora
        if not ahora:
            continue
        if any(x in sobre for x in range(h - VENTANA_HORAS, h)):
            episodios[-1]["fin"] = sello(h)
            episodios[-1]["horas_sobre_eca"] += 1
            continue
        # La ventana de búsqueda es la de la media en t: (t − 24 h, t].
        ventana = range(h - VENTANA_HORAS + 1, h + 1)
        primera_horaria = next((x for x in ventana if x in horaria), None)
        primera_alto = next((x for x in ventana if x in en_alto), None)
        if primera_horaria is None:
            raise ErrorDatos(f"{estacion} {sello(h)}: la media supera el ECA sin ninguna lectura sobre el umbral")

        def seguidas(encendidas):
            k = 0
            while k < VENTANA_HORAS - 1 and (h - k - 1) in encendidas:
                k += 1
            return k
        episodios.append({
            "estacion": estacion,
            "inicio": sello(h),
            "fin": sello(h),
            "horas_sobre_eca": 1,
            "media_previa_calculable": (h - 1) in calculable,
            "horas_antes_lectura_horaria": h - primera_horaria,
            "horas_antes_regimen": None if primera_alto is None else h - primera_alto,
            "horas_seguidas_lectura_horaria": seguidas(horaria),
            "horas_seguidas_regimen": seguidas(en_alto),
        })
    return episodios, cruces


def resumen_episodios(episodios):
    def senal(nombre):
        valores = [e["horas_antes_" + nombre] for e in episodios if e["horas_antes_" + nombre] is not None]
        return {
            "con_senal": len(valores),
            "sin_senal": len(episodios) - len(valores),
            "con_senal_previa": sum(1 for v in valores if v > 0),
            "en_el_tope": sum(1 for v in valores if v == VENTANA_HORAS - 1),
            "horas_antes": resumen(valores),
            "horas_seguidas": resumen([e["horas_seguidas_" + nombre] for e in episodios]),
        }
    return {
        "episodios": len(episodios),
        "con_media_previa": sum(1 for e in episodios if e["media_previa_calculable"]),
        "sin_media_previa": sum(1 for e in episodios if not e["media_previa_calculable"]),
        "sin_senal_previa": sum(1 for e in episodios
                                if not e["horas_antes_lectura_horaria"] and not e["horas_antes_regimen"]),
        "horas_sobre_eca": sum(e["horas_sobre_eca"] for e in episodios),
        "lectura_horaria": senal("lectura_horaria"),
        "regimen": senal("regimen"),
    }


def anticipacion(por_estacion, alto, total):
    todos, cruces, filas = [], 0, []
    for estacion, horas in por_estacion.items():
        episodios, n = episodios_de(estacion, horas)
        todos += episodios
        cruces += n
        filas.append({"estacion": estacion, "cruces_hacia_arriba": n, **resumen_episodios(episodios)})
    if sum(e["horas_sobre_eca"] for e in todos) != total["horas_sobre_eca"]:
        raise ErrorDatos("las horas sobre el ECA de los episodios no suman el total")
    return {
        "definicion": {
            "episodio": "Un episodio empieza en la hora t de una estación si su media de 24 h en t es calculable "
                        "(18 lecturas o más) y supera el ECA, y en las 24 horas anteriores, [t − 24 h, t), esa "
                        "estación no tuvo ninguna hora con media calculable sobre el ECA. Las horas sobre el ECA "
                        "que siguen son del mismo episodio mientras no pasen 24 horas seguidas sin ninguna: dos "
                        "subidas separadas por menos de 24 horas comparten lecturas en su ventana y cuentan como una.",
            "ventana_de_busqueda": "Las señales se buscan en (t − 24 h, t], las mismas horas que entran en la media "
                                   "de 24 h en t. Una lectura anterior no forma parte de esa media.",
            "horas_antes_lectura_horaria": "t menos la hora de la primera lectura de esa ventana con PM2.5 > 50 o "
                                           "PM10 > 100 µg/m³. Siempre existe: una media sobre el umbral exige al "
                                           "menos una lectura sobre el umbral.",
            "horas_antes_regimen": "t menos la hora de la primera lectura de esa ventana asignada al régimen que "
                                   "excede el ECA; null si ninguna lo fue.",
            "horas_seguidas": "Cuántas horas seguidas, justo antes de t (t − 1, t − 2, …), la señal estuvo encendida "
                              "sin interrupción; una hora apagada o sin lectura corta la cuenta. 0 si en t − 1 no lo "
                              "estaba. Es la cota baja de la anticipación: las señales se encienden y se apagan, y "
                              "horas_antes cuenta desde la primera vez.",
            "tope_horas": VENTANA_HORAS - 1,
            "en_el_tope": "Episodios en que la señal ya estaba encendida en la primera hora de la ventana: la "
                          "anticipación medida es 23 horas y no puede ser mayor por construcción.",
            "media_previa_calculable": "true si la hora t − 1 tenía media calculable (el cruce se ve). false si la "
                                       "media aparece ya sobre el ECA al juntar 18 lecturas, al empezar una campaña "
                                       "o después de un hueco: ahí la anticipación es en parte un efecto del mínimo "
                                       "de lecturas.",
            "sin_senal_previa": "Episodios en que ninguna de las dos señales se encendió antes de t.",
            "con_senal_previa": "Episodios en que la señal se encendió al menos una hora antes de t.",
            "cruces_hacia_arriba": "Horas con media calculable sobre el ECA cuya hora calculable anterior no lo "
                                   "superaba (o que son la primera calculable de la estación). Son más que los "
                                   "episodios porque las subidas seguidas se cuentan como un solo episodio.",
            "cuartiles": "Interpolación lineal entre los valores vecinos; con un solo episodio, ese valor.",
        },
        "lo_que_no_es": "Es una medida descriptiva sobre las mismas lecturas con que se entrenó el modelo: no hay "
                        "partición temporal ni reentrenamiento sin fugas, que era el diseño completo anunciado en la "
                        "PC3. No es un pronóstico ni dice cuánto antes avisaría el sistema con datos nuevos. "
                        "horas_antes cuenta desde la primera vez que la señal se enciende dentro de la ventana, no "
                        "desde que queda encendida (eso es horas_seguidas, la cota baja), y tiene un tope de 23 "
                        "horas por construcción. Tampoco descuenta las veces que la señal suena sin que la media "
                        "cruce el ECA: eso lo mide la precisión, en «senales».",
        "regimen_que_excede": alto,
        "cruces_hacia_arriba": cruces,
        "total": resumen_episodios(todos),
        "solo_con_media_previa": resumen_episodios([e for e in todos if e["media_previa_calculable"]]),
        "por_estacion": filas,
        "episodios": todos,
    }


# --- impacto.json --------------------------------------------------------------------------------

def armar_impacto(lecturas, filas, ficha, poblacion, alerta_24h, regimenes, hashes):
    alto = regimen_que_excede(regimenes)
    ficha_de = {e["estacion"]: e for e in ficha}
    poblacion_de = {p["ubigeo"]: p for p in poblacion}
    sin_ficha = sorted(set(lecturas) - set(ficha_de))
    if sin_ficha:
        raise ErrorDatos(f"estaciones del gold que no están en la ficha: {sin_ficha}")
    sin_poblacion = sorted({e["ubigeo"] for e in ficha if e["ubigeo"] != "NA"} - set(poblacion_de))
    if sin_poblacion:
        raise ErrorDatos(f"ubigeos de la ficha sin fila en {POBLACION}: {sin_poblacion}")

    por_estacion = estados(lecturas, alto)
    conteos = {e: conteo(horas) for e, horas in por_estacion.items()}
    total = sumar(conteos.values())
    comparacion = senales(por_estacion, total)

    # Los totales son los de reports/alerta_24h.json, o no se escribe nada.
    esperado = {
        "gold.sha256": (hashes["gold"], alerta_24h["gold"]["sha256"]),
        "gold.sha256 de regimenes.json": (hashes["gold"], regimenes["gold"]["sha256"]),
        "gold.filas": (total["lecturas"], alerta_24h["gold"]["filas"]),
        "ventana_horas": (VENTANA_HORAS, alerta_24h["ventana_horas"]),
        "minimo_lecturas": (MINIMO_LECTURAS, alerta_24h["minimo_lecturas"]),
        "eca_pm2_5": (ECA_PM25, alerta_24h["eca_pm2_5"]),
        "eca_pm10": (ECA_PM10, alerta_24h["eca_pm10"]),
        "horas_calculables": (total["horas_calculables"], alerta_24h["horas_calculables"]),
        "horas_sobre_eca": (total["horas_sobre_eca"], alerta_24h["horas_sobre_eca"]),
        "inca": (total["inca"], {b: alerta_24h["inca"].get(b, 0) for b in BANDAS}),
        "regimen_alto.regimen": (alto, alerta_24h["regimen_alto"]["regimen"]),
        "regimen_alto.horas": (comparacion["regimen_alto"]["horas"], alerta_24h["regimen_alto"]["horas"]),
        "regimen_alto.aciertos": (comparacion["regimen_alto"]["aciertos"], alerta_24h["regimen_alto"]["aciertos"]),
        "lectura_horaria.horas": (comparacion["lectura_horaria"]["horas"], alerta_24h["lectura_horaria"]["horas"]),
        "lectura_horaria.aciertos": (comparacion["lectura_horaria"]["aciertos"],
                                     alerta_24h["lectura_horaria"]["aciertos"]),
    }
    distintos = [f"{k}: calculado {a!r}, {ALERTA_24H} dice {b!r}" for k, (a, b) in esperado.items() if a != b]
    if distintos:
        raise ErrorDatos("los totales no coinciden con el reporte de la ventana:\n  " + "\n  ".join(distintos))

    def poblacion_fila(ubigeo):
        if ubigeo == "NA":
            return None
        p = poblacion_de[ubigeo]
        return {"poblacion": int(p["poblacion"]) if p["poblacion"] else None, "anio": int(p["anio"]), "fuente": p["fuente"]}

    def texto(valor):
        return None if valor == "NA" else valor

    estaciones = []
    for estacion, horas in por_estacion.items():
        f = ficha_de[estacion]
        dias = sorted({dia_de(h) for h, _, _, _ in horas})
        if not (f["fecha_inicio"] <= dias[0] and dias[-1] <= f["fecha_fin"]):
            raise ErrorDatos(f"{estacion}: sus días {dias[0]} … {dias[-1]} se salen de las fechas de la ficha")
        if conteos[estacion]["lecturas"] != int(f["filas_gold"]):
            raise ErrorDatos(f"{estacion}: {conteos[estacion]['lecturas']} lecturas y la ficha dice {f['filas_gold']}")
        estaciones.append({
            "estacion": estacion,
            "objeto_fiscalizado": f["objeto_fiscalizado"],
            "tipo_actividad": f["tipo_actividad"],
            "departamento": texto(f["departamento"]),
            "provincia": texto(f["provincia"]),
            "distrito": texto(f["distrito"]),
            "ubigeo": texto(f["ubigeo"]),
            "poblacion_distrito": poblacion_fila(f["ubigeo"]),
            "primera_fecha": dias[0],
            "ultima_fecha": dias[-1],
            "dias_con_datos": len(dias),
            **conteos[estacion],
            "banda_mas_frecuente": banda_mas_frecuente(conteos[estacion]["inca"]),
        })

    def agregado(grupo):
        c = sumar(conteos[e["estacion"]] for e in grupo)
        return {
            "estaciones": [e["estacion"] for e in grupo],
            "primera_fecha": min(e["primera_fecha"] for e in grupo),
            "ultima_fecha": max(e["ultima_fecha"] for e in grupo),
            "dias_estacion_con_datos": sum(e["dias_con_datos"] for e in grupo),
            **c,
            "banda_mas_frecuente": banda_mas_frecuente(c["inca"]),
        }

    def distrito_de(ubigeo):
        p = poblacion_de[ubigeo]
        return {"ubigeo": ubigeo, "departamento": p["departamento"], "provincia": p["provincia"],
                "distrito": p["distrito"], **poblacion_fila(ubigeo)}

    tipos = []
    for tipo in sorted({e["tipo_actividad"] for e in estaciones}):
        grupo = [e for e in estaciones if e["tipo_actividad"] == tipo]
        tipos.append({
            "tipo_actividad": tipo,
            "objetos_fiscalizados": sorted({e["objeto_fiscalizado"] for e in grupo}),
            "distritos": [distrito_de(u) for u in sorted({e["ubigeo"] for e in grupo if e["ubigeo"]})],
            "estaciones_sin_distrito": [e["estacion"] for e in grupo if not e["ubigeo"]],
            **agregado(grupo),
        })

    distritos = []
    for ubigeo in sorted({e["ubigeo"] for e in estaciones if e["ubigeo"]}):
        grupo = [e for e in estaciones if e["ubigeo"] == ubigeo]
        distritos.append({**distrito_de(ubigeo),
                          "tipos_actividad": sorted({e["tipo_actividad"] for e in grupo}), **agregado(grupo)})
    sin_distrito = [e for e in estaciones if not e["ubigeo"]]
    if sin_distrito:
        distritos.append({"ubigeo": None, "departamento": None, "provincia": None, "distrito": None,
                          "poblacion": None, "anio": None, "fuente": None,
                          "nota": "La fuente no trae el distrito de estas estaciones.",
                          "tipos_actividad": sorted({e["tipo_actividad"] for e in sin_distrito}),
                          **agregado(sin_distrito)})

    for nombre, grupos in (("estación", [conteos[e] for e in por_estacion]), ("tipo de actividad", tipos),
                           ("distrito", distritos)):
        if sumar(grupos) != total:
            raise ErrorDatos(f"la suma por {nombre} no da los totales")

    con_pm = {e["estacion"] for e in estaciones}
    return {
        "metadatos": {
            "generado": dt.datetime.now(LIMA).date().isoformat(),
            "generador": "scripts/impacto.py",
            "fuentes": {
                "gold": {"ruta": "data/" + RUTAS["gold"], "sha256": hashes["gold"], "filas": filas["gold"]},
                "asignaciones": {"ruta": "data/" + RUTAS["asignaciones"], "sha256": hashes["asignaciones"],
                                 "filas": filas["asignaciones"]},
                "silver": {"ruta": "data/" + RUTAS["silver"], "sha256": hashes["silver"], "filas": filas["silver"]},
                "ficha": {"ruta": FICHA, "sha256": hashes["ficha"]},
                "poblacion": {"ruta": POBLACION, "sha256": hashes["poblacion"],
                              "que_es": "Población censada, Censos Nacionales 2017 (INEI, Directorio Nacional de "
                                        "Centros Poblados). La URL de cada cifra va en su fila."},
                "alerta_24h": {"ruta": ALERTA_24H, "sha256": hashes["alerta_24h"]},
                "regimenes": {"ruta": REGIMENES, "sha256": hashes["regimenes"]},
            },
            "reglas": reglas(),
            "notas": [
                "Solo entran las 17 estaciones con PM2.5 y PM10 por hora (las del gold). Las otras 16 de la ficha "
                "no están en ninguna cifra: una estación que no mide PM por hora no es una estación con aire limpio.",
                "La población es la censada del distrito de cada estación y va como contexto de su fila. No se suma "
                "entre distritos: las estaciones de OEFA vigilan una actividad fiscalizada y miden cerca de ella, y "
                "vivir en el distrito no es haber respirado ese aire.",
                "Varias estaciones son campañas de pocas semanas: sus horas sobre el ECA describen esas semanas "
                "(primera_fecha, ultima_fecha y dias_con_datos).",
                "dias_estacion_con_datos suma los días de cada estación del grupo: dos estaciones que midieron el "
                "mismo día cuentan dos.",
            ],
        },
        "alcance": {
            "estaciones_en_la_ficha": len(ficha),
            "estaciones_con_pm_horario": len(estaciones),
            "estaciones_sin_pm_horario": sorted(e["estacion"] for e in ficha if e["estacion"] not in con_pm),
            "distritos_con_pm_horario": len({e["ubigeo"] for e in estaciones if e["ubigeo"]}),
            "estaciones_sin_distrito": [e["estacion"] for e in sin_distrito],
        },
        "totales": total,
        "estaciones": estaciones,
        "tipos_actividad": tipos,
        "distritos": distritos,
        "poblacion_distritos": [
            {**distrito_de(p["ubigeo"]),
             "estaciones": sorted(e["estacion"] for e in ficha if e["ubigeo"] == p["ubigeo"]),
             "estaciones_con_pm_horario": sorted(e["estacion"] for e in ficha
                                                 if e["ubigeo"] == p["ubigeo"] and e["estacion"] in con_pm)}
            for p in sorted(poblacion, key=lambda p: p["ubigeo"])
        ],
        "senales": {
            "nota": "Qué acertaría cada señal si se usara como alerta, sobre las horas con media de 24 h "
                    "calculable. horas: en cuántas estaba encendida; aciertos: en cuántas de esas la media de 24 h "
                    "superaba el ECA; precision = aciertos / horas; cobertura = aciertos / horas_sobre_eca. La "
                    "alerta del sistema sale de la media de 24 h, que es la norma misma.",
            "horas_calculables": total["horas_calculables"],
            "horas_sobre_eca": total["horas_sobre_eca"],
            "regimen_alto": {"regimen": alto, **comparacion["regimen_alto"]},
            "lectura_horaria": comparacion["lectura_horaria"],
        },
        "anticipacion": anticipacion(por_estacion, alto, total),
    }


def reglas():
    return {
        "lectura": "Una hora de una estación con PM2.5 y PM10 (una fila del gold), con las concentraciones "
                   "crudas del silver en µg/m³.",
        "ventana": "(t − 24 h, t]",
        "ventana_horas": VENTANA_HORAS,
        "minimo_lecturas": MINIMO_LECTURAS,
        "eca_pm2_5": ECA_PM25,
        "eca_pm10": ECA_PM10,
        "alerta": "media de 24 h de PM2.5 > 50 o de PM10 > 100 µg/m³ (ECA de 24 h, DS 003-2017-MINAM)",
        "inca": {"bandas": BANDAS, "cortes_pm10": CORTES_PM10, "cortes_pm2_5": CORTES_PM25,
                 "regla": "la peor banda entre la de PM10 y la de PM2.5 (INCA, RM 181-2016-MINAM)"},
        "dia": "fecha_hora es el fin del promedio horario: el día de una lectura es el de la hora anterior "
               "(01:00 … 24:00 son un mismo día).",
        "implementacion": "ventana/ventana.go",
    }


# --- series.json ---------------------------------------------------------------------------------

def como_lectura(estacion, fila):
    return {"estacion": estacion, "fecha_hora": sello(fila[0]), "pm2_5": fila[1], "pm10": fila[2]}


def tramo(lecturas, estacion, primera, ultima):
    return [f for f in lecturas.get(estacion, []) if primera <= f[0] <= ultima]


def recorrido(estacion, filas):
    """La media de 24 h que la API devuelve en cada lectura de un tramo mandado en orden a una
    estación sin lecturas en las 24 horas anteriores."""
    ventanas = Ventanas()
    return [ventanas.agregar(estacion, h, a, b) for h, a, b, _ in filas]


def esperado_al_final(estacion, filas, excede):
    """Lo que la API responde en la última lectura de ese tramo, y cuándo aparecen la media y la
    alerta."""
    medias = recorrido(estacion, filas)
    media = medias[-1]
    primera_media = next((sello(f[0]) for f, m in zip(filas, medias) if m["pm2_5"] is not None), None)
    primera_alerta = next((sello(f[0]) for f, m in zip(filas, medias) if alerta(m)), None)
    return {
        "fecha_hora": sello(filas[-1][0]),
        "lecturas_enviadas": len(filas),
        "regimen": filas[-1][3],
        "excede_eca": filas[-1][3] == excede,
        "media_24h": media,
        "alerta": alerta(media),
        "inca": inca(media),
        "primera_media": primera_media,
        "primera_alerta": primera_alerta,
    }


def elegir_escenario(lecturas, excede):
    """Las 48 horas seguidas de diciembre de 2021 con más estaciones que terminan con media de 24 h.

    Entre las que empatan: las que cubren dos días enteros (primer sello 01:00), después las que
    tienen más lecturas y, por último, la más temprana."""
    desde, hasta = hora_de(ESCENARIO_DESDE), hora_de(ESCENARIO_HASTA)
    lecturas = {e: [f for f in serie if desde <= f[0] <= hasta] for e, serie in lecturas.items()}
    mejor = None
    candidatas = 0
    for primera in range(desde, hasta - HORAS_TRAMO + 2):
        ultima = primera + HORAS_TRAMO - 1
        tramos = {e: tramo(lecturas, e, primera, ultima) for e in sorted(lecturas)}
        tramos = {e: f for e, f in tramos.items() if f}
        finales = {e: esperado_al_final(e, f, excede) for e, f in tramos.items()}
        con_media = sum(1 for x in finales.values() if x["media_24h"]["pm2_5"] is not None)
        dias_enteros = dt.datetime.fromtimestamp(primera * 3600, LIMA).hour == 1
        puntaje = (con_media, dias_enteros, sum(len(f) for f in tramos.values()), -primera)
        candidatas += 1
        if mejor is None or puntaje > mejor[0]:
            mejor = (puntaje, primera, ultima, tramos, finales)
    _, primera, ultima, tramos, finales = mejor
    return primera, ultima, tramos, finales, candidatas


def armar_series(lecturas, filas, ficha, regimenes, hashes):
    excede = regimen_que_excede(regimenes)
    primera, ultima, tramos, finales, candidatas = elegir_escenario(lecturas, excede)
    envio = sorted((f[0], e, f) for e, serie in tramos.items() for f in serie)
    escenario = {
        "id": "escenario",
        "desde": sello(primera),
        "hasta": sello(ultima),
        "dias": [dia_de(primera), dia_de(ultima)],
        "horas": HORAS_TRAMO,
        "criterio": "Entre los tramos de 48 horas seguidas del 13 al 31 de diciembre de 2021, el que deja más "
                    "estaciones con media de 24 h en su última lectura; entre los que empatan, los que cubren dos "
                    "días enteros, después el de más lecturas y después el más temprano.",
        "tramos_comparados": candidatas,
        "estaciones": [
            {"estacion": e, "lecturas": len(f), "con_media_al_final": finales[e]["media_24h"]["pm2_5"] is not None}
            for e, f in tramos.items()
        ],
        "estaciones_sin_lecturas": sorted(set(lecturas) - set(tramos)),
        "lecturas": [como_lectura(e, f) for _, e, f in envio],
        "esperado": finales,
    }

    con_gold = {e["estacion"] for e in ficha if int(e["filas_gold"]) > 0}
    dias, usados = [], collections.defaultdict(list)
    for e, f in tramos.items():
        usados[e].append((f[0][0], f[-1][0]))
    for caso, estacion, fin, alerta_final, inca_final, huecos in DIAS:
        if estacion not in con_gold:
            raise ErrorDatos(f"{estacion} no es una estación con filas en el gold")
        h_fin = hora_de(fin)
        h_ini = h_fin - HORAS_TRAMO + 1
        f = tramo(lecturas, estacion, h_ini, h_fin)
        nombre = f"{caso} {estacion} {fin}"
        if not f:
            raise ErrorDatos(f"{nombre}: no hay lecturas en esas 48 horas")
        for u_ini, u_fin in usados[estacion]:
            if not (h_ini - u_fin > SEPARACION_HORAS or u_ini - h_fin > SEPARACION_HORAS):
                raise ErrorDatos(f"{nombre}: a 48 horas o menos de otro tramo de la estación")
        if h_ini <= ultima:
            raise ErrorDatos(f"{nombre}: no es posterior al escenario")
        usados[estacion].append((h_ini, h_fin))
        final = esperado_al_final(estacion, f, excede)
        if (final["alerta"], final["inca"], HORAS_TRAMO - len(f)) != (alerta_final, inca_final, huecos):
            raise ErrorDatos(f"{nombre}: termina con alerta={final['alerta']}, inca={final['inca']} y "
                             f"{HORAS_TRAMO - len(f)} huecos; se esperaba {alerta_final}, {inca_final} y {huecos}")
        if caso == "cruza_eca" and not (final["primera_media"] < final["primera_alerta"]
                                        and dia_de(hora_de(final["primera_alerta"])) == dia_de(h_fin)):
            raise ErrorDatos(f"{nombre}: la alerta no se enciende durante el segundo día")
        if caso == "en_alerta" and final["primera_media"] != final["primera_alerta"]:
            raise ErrorDatos(f"{nombre}: la primera media no está en alerta")
        if caso in ("limpio", "mala_sin_alerta", "huecos_con_media", "huecos_sin_media") and final["primera_alerta"]:
            raise ErrorDatos(f"{nombre}: la alerta se enciende en algún momento")
        if caso == "limpio" and {inca(m) for m in recorrido(estacion, f)} != {None, "buena"}:
            raise ErrorDatos(f"{nombre}: la banda INCA no es buena de principio a fin")
        if caso == "huecos_sin_media" and not final["primera_media"]:
            raise ErrorDatos(f"{nombre}: la media no llega a aparecer")
        dias.append({
            "id": f"{estacion}-{dia_de(h_fin)}",
            "estacion": estacion,
            "dia": dia_de(h_fin),
            "caso": caso,
            "descripcion": CASOS[caso],
            "desde": sello(h_ini),
            "hasta": sello(h_fin),
            "horas": HORAS_TRAMO,
            "huecos": HORAS_TRAMO - len(f),
            "lecturas": [como_lectura(estacion, x) for x in f],
            "esperado": final,
        })
    if len({d["estacion"] for d in dias}) != len(dias):
        raise ErrorDatos("los días sueltos tienen que ser de estaciones distintas")

    return {
        "metadatos": {
            "generado": dt.datetime.now(LIMA).date().isoformat(),
            "generador": "scripts/impacto.py",
            "fuentes": {
                "asignaciones": {"ruta": "data/" + RUTAS["asignaciones"], "sha256": hashes["asignaciones"],
                                 "filas": filas["asignaciones"]},
                "silver": {"ruta": "data/" + RUTAS["silver"], "sha256": hashes["silver"], "filas": filas["silver"]},
                "regimenes": {"ruta": REGIMENES, "sha256": hashes["regimenes"]},
            },
            "reglas": reglas(),
            "como_se_usa": {
                "envio": "Cada elemento de «lecturas» es el cuerpo de un POST /api/predicciones, en el orden en que "
                         "está: por hora y, dentro de la hora, por estación.",
                "esperado": "Lo que la API responde en la última lectura de cada estación (regimen, excede_eca, "
                            "media_24h, alerta e inca, con los nombres de la respuesta), calculado con la regla de "
                            "ventana/ventana.go sin consultarla. primera_media y primera_alerta son la fecha_hora de "
                            "la primera lectura que devuelve media y de la primera que devuelve alerta = true.",
                "regimen": "El régimen esperado es el de data/gold/asignaciones.csv: vale para el modelo sembrado "
                           "(reports/kmeans.json). Si se entrena otro modelo, los índices pueden cambiar; la media, "
                           "la alerta y el INCA no dependen del modelo.",
                "orden": "Se carga primero el escenario, sobre un sistema limpio, y después los días. Los días de "
                         "una estación son posteriores a sus lecturas del escenario y entre un tramo y otro de la "
                         "misma estación hay más de 48 horas: ninguna ventana mezcla lecturas de dos tramos y el "
                         "último estado de la estación siempre avanza.",
                "limites_api": {"pm_minimo": 0, "pm_maximo": PM_MAXIMO, "fecha_hora": "RFC 3339, se trunca a la hora"},
            },
        },
        "escenario": escenario,
        "dias": dias,
    }


# --- Escritura y comparación ---------------------------------------------------------------------

def generar(datos, raiz=RAIZ):
    """Calcula impacto.json y series.json a partir de los datos sin versionar y de reports/."""
    datos, raiz = Path(datos), Path(raiz)
    lecturas, filas = leer_lecturas(datos)
    hashes = {nombre: sha256(datos / ruta) for nombre, ruta in RUTAS.items()}
    for nombre, ruta in (("ficha", FICHA), ("poblacion", POBLACION), ("alerta_24h", ALERTA_24H), ("regimenes", REGIMENES)):
        hashes[nombre] = sha256(raiz / ruta)
    ficha, poblacion = leer_csv(raiz / FICHA), leer_csv(raiz / POBLACION)
    alerta_24h, regimenes = leer_json(raiz / ALERTA_24H), leer_json(raiz / REGIMENES)
    return (armar_impacto(lecturas, filas, ficha, poblacion, alerta_24h, regimenes, hashes),
            armar_series(lecturas, filas, ficha, regimenes, hashes))


def sin_fecha(documento):
    return {**documento, "metadatos": {k: v for k, v in documento["metadatos"].items() if k != "generado"}}


def como_texto(documento):
    return json.dumps(documento, ensure_ascii=False, indent=2) + "\n"


def texto_series(series):
    """series.json con una lectura por línea: pesa poco y un cambio se lee en un diff."""
    marca = "@@LECTURAS@@"
    bloques = []

    def sacar(lista):
        bloques.append("[\n" + ",\n".join(json.dumps(x, ensure_ascii=False, separators=(", ", ": ")) for x in lista) + "\n]")
        return f"{marca}{len(bloques) - 1}"

    copia = json.loads(json.dumps(series))
    copia["escenario"]["lecturas"] = sacar(copia["escenario"]["lecturas"])
    for d in copia["dias"]:
        d["lecturas"] = sacar(d["lecturas"])
    texto = json.dumps(copia, ensure_ascii=False, indent=2)
    for i, bloque in enumerate(bloques):
        texto = texto.replace(f'"{marca}{i}"', bloque)
    return texto + "\n"


def actual(ruta):
    try:
        return leer_json(ruta)
    except (OSError, ValueError):
        return None


def main():
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("--datos", default=str(RAIZ / "data"), help="carpeta con silver/ y gold/ (no se versionan)")
    ap.add_argument("--verificar", action="store_true", help="no escribe: recalcula y compara con lo versionado")
    args = ap.parse_args()
    try:
        impacto, series = generar(args.datos)
    except ErrorDatos as e:
        print(f"impacto.py: {e}", file=sys.stderr)
        return 1
    salidas = [(SALIDA_IMPACTO, impacto, como_texto), (COPIA_IMPACTO, impacto, como_texto),
               (SALIDA_SERIES, series, texto_series)]
    distintos = 0
    for ruta, documento, formato in salidas:
        guardado = actual(RAIZ / ruta)
        igual = guardado is not None and sin_fecha(guardado) == sin_fecha(documento)
        if args.verificar:
            print(f"{ruta}: {'coincide con lo recalculado' if igual else 'DIFERENCIA con lo recalculado'}")
            distintos += not igual
        elif igual:
            print(f"{ruta}: sin cambios")
        else:
            (RAIZ / ruta).parent.mkdir(parents=True, exist_ok=True)
            (RAIZ / ruta).write_text(formato(documento), encoding="utf-8")
            print(f"{ruta}: escrito ({(RAIZ / ruta).stat().st_size} bytes)")
    t, a, e = impacto["totales"], impacto["anticipacion"]["total"], series["escenario"]
    print(f"{t['horas_calculables']} horas calculables, {t['horas_sobre_eca']} sobre el ECA; {a['episodios']} episodios; "
          f"escenario {e['desde']} … {e['hasta']} con {len(e['estaciones'])} estaciones y {len(e['lecturas'])} lecturas; "
          f"{len(series['dias'])} días sueltos")
    if (RAIZ / SALIDA_IMPACTO).read_bytes() != (RAIZ / COPIA_IMPACTO).read_bytes():
        print(f"{COPIA_IMPACTO} no es una copia de {SALIDA_IMPACTO}", file=sys.stderr)
        distintos += 1
    return 1 if distintos else 0


if __name__ == "__main__":
    raise SystemExit(main())
