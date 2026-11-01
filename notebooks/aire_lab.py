"""Laboratorio en Python de aire-distribuido. NO es parte del sistema.

El sistema (carga, limpieza, estandarización y K-Means) es Go. Este módulo reúne lo que los
notebooks de Colab comparten para explorar, calibrar, auditar y dibujar:

- configuración del bronze (`configs/limpieza.json` o el diccionario por defecto),
- lectura del CSV largo de OEFA y las reglas de limpieza 1-8 de la spec de la PC3 (§4.2),
  como **prototipo independiente** de lo que hace `cmd/cargar`,
- el contrato de silver y gold (mismo formato que escribe Go),
- las features de la receta del K-Means (`configs/kmeans.json`),
- mediciones de la criba (huecos, ECA, ciclos, distancias) y métricas de la calibración,
- el estilo de las figuras del informe.

Depende solo de numpy, pandas y matplotlib; scikit-learn y scipy se importan donde hacen falta.
"""

from __future__ import annotations

import copy
import csv
import hashlib
import io
import json
import math
import re
import unicodedata
from pathlib import Path

import numpy as np
import pandas as pd

# ---------------------------------------------------------------------------------------------
# Constantes del contrato compartido con Go
# ---------------------------------------------------------------------------------------------

CANONICOS = [
    "pm2_5", "pm10", "no2", "so2", "co", "o3",
    "temperatura", "humedad", "viento_velocidad", "viento_direccion",
    "presion", "precipitacion", "radiacion",
]
CONTAMINANTES = ["pm2_5", "pm10", "no2", "so2", "co", "o3"]
GASES = ["no2", "so2", "co", "o3"]
METEO = ["temperatura", "humedad", "viento_velocidad", "viento_direccion",
         "presion", "precipitacion", "radiacion"]
DERIVADAS = ["fraccion_fina"]  # pm2_5 / pm10, recortada a [0, 1]

# ECA de aire, DS 003-2017-MINAM, valores de 24 h (µg/m³).
ECA = {"pm2_5": 50.0, "pm10": 100.0}

DESFASE_LIMA = "-05:00"  # hora local de Lima, sin horario de verano

UNIDAD_TXT = {
    "pm2_5": "µg/m³", "pm10": "µg/m³", "no2": "µg/m³", "so2": "µg/m³", "co": "µg/m³",
    "o3": "µg/m³", "temperatura": "°C", "humedad": "%", "viento_velocidad": "m/s",
    "viento_direccion": "°", "presion": "hPa", "precipitacion": "mm", "radiacion": "W/m²",
    "fraccion_fina": "PM2.5/PM10",
}
NOMBRE_TXT = {
    "pm2_5": "PM2.5", "pm10": "PM10", "no2": "NO₂", "so2": "SO₂", "co": "CO", "o3": "O₃",
    "temperatura": "Temperatura", "humedad": "Humedad", "viento_velocidad": "Viento",
    "viento_direccion": "Dir. viento", "presion": "Presión", "precipitacion": "Precipitación",
    "radiacion": "Radiación", "fraccion_fina": "Fracción fina",
}

# Las 25 regiones del Perú (24 departamentos y la Provincia Constitucional del Callao).
REGIONES_PERU = [
    "AMAZONAS", "ANCASH", "APURIMAC", "AREQUIPA", "AYACUCHO", "CAJAMARCA", "CALLAO", "CUSCO",
    "HUANCAVELICA", "HUANUCO", "ICA", "JUNIN", "LA LIBERTAD", "LAMBAYEQUE", "LIMA", "LORETO",
    "MADRE DE DIOS", "MOQUEGUA", "PASCO", "PIURA", "PUNO", "SAN MARTIN", "TACNA", "TUMBES",
    "UCAYALI",
]

NOMBRE_REGION = {r: r.title() for r in REGIONES_PERU}
NOMBRE_REGION.update({"ANCASH": "Áncash", "APURIMAC": "Apurímac", "HUANUCO": "Huánuco",
                      "JUNIN": "Junín", "SAN MARTIN": "San Martín", "LA LIBERTAD": "La Libertad",
                      "MADRE DE DIOS": "Madre de Dios"})


def region(texto) -> str | None:
    """Nombre de departamento -> clave de REGIONES_PERU (mayúsculas, sin tildes)."""
    if texto is None or (isinstance(texto, float) and math.isnan(texto)):
        return None
    t = unicodedata.normalize("NFKD", str(texto).strip().upper())
    t = "".join(c for c in t if not unicodedata.combining(c))
    t = re.sub(r"\s+", " ", re.sub(r"[^A-Z ]", " ", t)).strip()
    if t.startswith("PROV") and "CALLAO" in t:
        return "CALLAO"
    return t


def pct(x, dec=0) -> str:
    """0.753 -> '75 %' (tipografía en español)."""
    return "—" if x is None or (isinstance(x, float) and math.isnan(x)) else f"{100 * x:.{dec}f} %"


def boxplot_horizontal(ax, datos, **kw):
    """boxplot horizontal compatible con matplotlib < 3.10 (vert) y >= 3.10 (orientation)."""
    try:
        return ax.boxplot(datos, orientation="horizontal", **kw)
    except TypeError:
        return ax.boxplot(datos, vert=False, **kw)


# ---------------------------------------------------------------------------------------------
# Configuración del bronze
# ---------------------------------------------------------------------------------------------

# Diccionario por defecto. Si existe configs/limpieza.json (lo escribe el cargador de Go), sus
# claves pisan a estas. Los nombres reales de columnas y parámetros se confirman el lunes con el
# diccionario de datos de OEFA; mientras tanto se resuelven también por alias (ALIAS_COLUMNAS).
CONFIG_DEFECTO = {
    "columnas": {
        "estacion": "NOMBRE_PUNTO",
        "fecha": "FECHA",
        "hora": "HORA",            # opcional: si la fecha ya trae la hora, se deja en None
        "parametro": "PARAMETRO",
        "valor": "VALOR",
        "periodo": "PERIODO",
        "unidad": "UNIDAD",
        "departamento": "DEPARTAMENTO",
        "provincia": "PROVINCIA",
        "distrito": "DISTRITO",
        "ubigeo": "UBIGEO",
        "este": "ESTE",
        "norte": "NORTE",
        "zona": "ZONA",
        "datum": "DATUM",
    },
    "delimitador": ";",
    "codificacion": "utf-8-sig",     # si falla, se reintenta con latin-1
    "decimal": "auto",               # "auto": acepta 12.5 y 12,5
    "formato_fecha": None,           # p. ej. "%d/%m/%Y"; None = se infiere
    "dia_primero": True,             # 01/06/2023 es 1 de junio
    "convencion_hora": "inicio",     # "inicio": 01:00 cubre 01-02; "fin": 01:00 cubre 00-01
    "fecha_minima": "2000-01-01 00:00",
    "fecha_maxima": "2026-10-31 23:59",
    # Valores de PERIODO que cuentan como lectura horaria (se comparan normalizados).
    "periodos_horarios": ["Horario", "Hora", "1 hora", "1 h", "1h", "60 min", "Horaria",
                          "Promedio horario", "Promedio 1 hora"],
    # Nombre real del parámetro -> nombre canónico (se comparan normalizados: sin tildes,
    # minúsculas, sin espacios ni signos; "PM 2,5" y "PM2.5" son la misma clave).
    "parametros": {
        "PM2.5": "pm2_5", "PM 2.5": "pm2_5", "PM2,5": "pm2_5", "Material particulado PM2.5": "pm2_5",
        "PM10": "pm10", "PM 10": "pm10", "Material particulado PM10": "pm10",
        "NO2": "no2", "Dióxido de nitrógeno": "no2",
        "SO2": "so2", "Dióxido de azufre": "so2",
        "CO": "co", "Monóxido de carbono": "co",
        "O3": "o3", "Ozono": "o3",
        "Temperatura": "temperatura", "Temperatura del aire": "temperatura",
        "Humedad relativa": "humedad", "Humedad": "humedad",
        "Velocidad del viento": "viento_velocidad", "Velocidad de viento": "viento_velocidad",
        "Dirección del viento": "viento_direccion", "Dirección de viento": "viento_direccion",
        "Presión atmosférica": "presion", "Presión barométrica": "presion", "Presión": "presion",
        "Precipitación": "precipitacion",
        "Radiación solar": "radiacion", "Radiación": "radiacion",
    },
    # Centinelas del equipo: se descartan (regla 3).
    "centinelas": [-999, -9999, -99.9, -999.9, 9999, 99999, -888],
    # Parámetros que admiten negativos (regla 3).
    "negativos_permitidos": ["temperatura"],
    # Rango físico por parámetro en su unidad canónica (regla 4): [mínimo, máximo] inclusive.
    "rangos": {
        "pm2_5": [0, 1000], "pm10": [0, 2000], "no2": [0, 2000], "so2": [0, 10000],
        "co": [0, 60000], "o3": [0, 1000], "temperatura": [-30, 50], "humedad": [0, 100],
        "viento_velocidad": [0, 60], "viento_direccion": [0, 360], "presion": [400, 1100],
        "precipitacion": [0, 200], "radiacion": [0, 1500],
    },
    # Unidades (regla 6): unidad canónica y factor que lleva cada unidad aceptada a la canónica.
    # Una unidad que no figure aquí descarta la fila. Un parámetro sin entrada acepta cualquiera.
    "unidades": {
        "pm2_5": {"canonica": "ug/m3", "factores": {"ug/m3": 1}},
        "pm10": {"canonica": "ug/m3", "factores": {"ug/m3": 1}},
        "no2": {"canonica": "ug/m3", "factores": {"ug/m3": 1}},
        "so2": {"canonica": "ug/m3", "factores": {"ug/m3": 1}},
        "co": {"canonica": "ug/m3", "factores": {"ug/m3": 1, "mg/m3": 1000}},
        "o3": {"canonica": "ug/m3", "factores": {"ug/m3": 1}},
    },
    "unidad_vacia_es_canonica": True,
}

# Alias de columnas (normalizados) por si el nombre real no coincide con el de la config.
ALIAS_COLUMNAS = {
    "estacion": ["nombre_punto", "estacion", "nombre_estacion", "punto", "punto_monitoreo",
                 "nombre_del_punto", "punto_de_monitoreo", "codigo_estacion", "cod_estacion"],
    "fecha": ["fecha", "fecha_hora", "fecha_medicion", "fecha_muestreo", "fechahora",
              "fecha_monitoreo", "fecha_y_hora"],
    "hora": ["hora", "hora_medicion", "hora_muestreo"],
    "parametro": ["parametro", "nombre_parametro", "variable", "contaminante", "parametros"],
    "valor": ["valor", "resultado", "concentracion", "medicion", "valor_medido", "valor_parametro"],
    "periodo": ["periodo", "periodo_medicion", "tiempo_promedio", "periodo_de_medicion"],
    "unidad": ["unidad", "unidad_medida", "unidades", "unidad_de_medida"],
    "departamento": ["departamento", "region"],
    "provincia": ["provincia"],
    "distrito": ["distrito"],
    "ubigeo": ["ubigeo", "cod_ubigeo"],
    "este": ["este", "coordenada_este", "utm_este", "x"],
    "norte": ["norte", "coordenada_norte", "utm_norte", "y"],
    "zona": ["zona", "zona_utm"],
    "datum": ["datum"],
}
REQUERIDAS = ["estacion", "fecha", "parametro", "valor"]


def normalizar(texto) -> str:
    """Clave de comparación: minúsculas, sin tildes, µ->u, ³->3, %->pct, °->deg, sin signos."""
    if texto is None or (isinstance(texto, float) and math.isnan(texto)):
        return ""
    t = str(texto).strip().lower()
    t = (t.replace("µ", "u").replace("μ", "u").replace("³", "3").replace("²", "2")
          .replace("%", "pct").replace("°", "deg").replace("º", "deg"))
    t = unicodedata.normalize("NFKD", t)
    t = "".join(c for c in t if not unicodedata.combining(c))
    return re.sub(r"[^a-z0-9]", "", t)


def normalizar_columna(texto) -> str:
    t = unicodedata.normalize("NFKD", str(texto).strip().lower())
    t = "".join(c for c in t if not unicodedata.combining(c))
    return re.sub(r"[^a-z0-9]+", "_", t).strip("_")


def _fusionar(base: dict, encima: dict) -> dict:
    out = copy.deepcopy(base)
    for k, v in encima.items():
        if isinstance(v, dict) and isinstance(out.get(k), dict) and k not in ("parametros",):
            out[k] = _fusionar(out[k], v)
        else:
            out[k] = copy.deepcopy(v)
    return out


def fusionar(base: dict, encima: dict) -> dict:
    """Fusión profunda de diccionarios (encima pisa a base)."""
    return _fusionar(base, encima)


def cargar_config(candidatas) -> tuple[dict, str]:
    """Devuelve (config, origen). Toma el primer `limpieza.json` que exista entre `candidatas`
    y lo fusiona sobre CONFIG_DEFECTO; si no hay ninguno, usa el diccionario por defecto.

    `parametros` no se fusiona: si el JSON lo trae, reemplaza al mapeo por defecto (el mapeo real
    es el del JSON). Las claves que el JSON traiga y este módulo no conozca se conservan."""
    for ruta in candidatas:
        ruta = Path(ruta)
        if ruta.exists():
            with open(ruta, encoding="utf-8") as fh:
                propio = json.load(fh)
            return _fusionar(CONFIG_DEFECTO, propio), str(ruta)
    return copy.deepcopy(CONFIG_DEFECTO), "diccionario por defecto (no hay configs/limpieza.json)"


def mapa_parametros(config) -> dict:
    """Clave normalizada del nombre real -> canónico."""
    out = {}
    for real, canon in config["parametros"].items():
        out[normalizar(real)] = canon
    for canon in CANONICOS:  # el canónico mismo también vale ("pm2_5", "humedad", ...)
        out.setdefault(normalizar(canon), canon)
    return out


def factores_unidad(config) -> dict:
    """canónico -> {unidad normalizada: factor}."""
    out = {}
    for canon, spec in (config.get("unidades") or {}).items():
        fac = {normalizar(u): float(f) for u, f in spec.get("factores", {}).items()}
        fac.setdefault(normalizar(spec.get("canonica", "")), 1.0)
        out[canon] = fac
    return out


# ---------------------------------------------------------------------------------------------
# Bronze: inspección, hash y lectura
# ---------------------------------------------------------------------------------------------

def sha256(ruta, bloque=1 << 24) -> str:
    h = hashlib.sha256()
    with open(ruta, "rb") as fh:
        while True:
            b = fh.read(bloque)
            if not b:
                break
            h.update(b)
    return h.hexdigest()


def inspeccionar_primer_mb(ruta, n_bytes=1 << 20) -> dict:
    """Lo primero que se mira del CSV (spec PC3 §4.1): BOM, codificación, delimitador, comillas
    y si algún campo entrecomillado trae saltos de línea (rompería la partición por bloques)."""
    with open(ruta, "rb") as fh:
        crudo = fh.read(n_bytes)
    info = {"bytes_leidos": len(crudo), "bom_utf8": crudo.startswith(b"\xef\xbb\xbf")}
    for cod in ("utf-8", "latin-1"):
        try:
            texto = crudo.decode(cod)
            # el corte del MB puede partir un carácter multibyte al final
            info["codificacion_probable"] = cod
            break
        except UnicodeDecodeError as e:
            if cod == "utf-8" and e.start >= len(crudo) - 4:
                texto = crudo[: e.start].decode("utf-8")
                info["codificacion_probable"] = "utf-8"
                break
    texto = texto.lstrip("﻿")
    lineas = texto.splitlines()
    if lineas and not texto.endswith(("\n", "\r")):
        lineas = lineas[:-1]  # la última línea puede estar cortada
    cabecera = lineas[0] if lineas else ""
    conteo = {d: cabecera.count(d) for d in [";", ",", "\t", "|"]}
    delim = max(conteo, key=conteo.get) if cabecera else ";"
    info["delimitador_probable"] = delim
    info["conteo_delimitadores_cabecera"] = conteo
    info["cabecera"] = cabecera
    info["primeras_lineas"] = lineas[1:6]
    info["fin_de_linea"] = "CRLF" if "\r\n" in texto[:10000] else "LF"
    info["lineas_en_el_mb"] = len(lineas)
    info["hay_comillas"] = '"' in texto
    # Un campo con salto de línea dentro deja comillas impares en una línea física.
    impares = sum(1 for l in lineas if l.count('"') % 2 == 1)
    info["lineas_con_comillas_impares"] = impares
    try:
        filas = list(csv.reader(io.StringIO("\n".join(lineas)), delimiter=delim))
        ncols = pd.Series([len(f) for f in filas]).value_counts().to_dict()
    except csv.Error as e:  # pragma: no cover
        ncols = {"error": str(e)}
    info["columnas_por_fila"] = ncols
    return info


def resolver_columnas(cabecera, config) -> tuple[dict, list]:
    """Mapea cada clave lógica a una columna real. Devuelve (mapa, faltantes_requeridas)."""
    reales = list(cabecera)
    por_norm = {normalizar_columna(c): c for c in reales}
    mapa = {}
    cols_cfg = config.get("columnas") or {}
    for clave in CONFIG_DEFECTO["columnas"]:
        pedido = cols_cfg.get(clave, CONFIG_DEFECTO["columnas"][clave])
        anulada = clave in cols_cfg and cols_cfg[clave] is None  # None explícito: no existe
        elegido = None
        if pedido is not None:
            if pedido in reales:
                elegido = pedido
            elif normalizar_columna(pedido) in por_norm:
                elegido = por_norm[normalizar_columna(pedido)]
        if elegido is None and not anulada:
            for alias in ALIAS_COLUMNAS.get(clave, []):
                if alias in por_norm and por_norm[alias] not in mapa.values():
                    elegido = por_norm[alias]
                    break
        if elegido is not None and elegido in mapa.values():
            elegido = None  # una columna real no puede servir a dos claves
        mapa[clave] = elegido
    faltan = [c for c in REQUERIDAS if mapa.get(c) is None]
    return mapa, faltan


def leer_bronze(ruta, config, columnas_extra=()) -> tuple[pd.DataFrame, dict]:
    """Lee el CSV largo con las columnas resueltas, todas como categoría (los valores se repiten
    millones de veces: la categoría ahorra memoria y deja parsear una vez por valor distinto).
    Devuelve el DataFrame con nombres lógicos, en orden de archivo, y un dict con lo resuelto."""
    cod = config.get("codificacion") or "utf-8-sig"
    sep = config.get("delimitador") or ";"
    try:
        cab = pd.read_csv(ruta, sep=sep, encoding=cod, nrows=0).columns
    except UnicodeDecodeError:
        cod = "latin-1"
        cab = pd.read_csv(ruta, sep=sep, encoding=cod, nrows=0).columns
    mapa, faltan = resolver_columnas(cab, config)
    if faltan:
        raise ValueError(
            f"No encuentro las columnas {faltan} en el bronze. Cabecera real: {list(cab)}. "
            "Ajusta config['columnas'] (o configs/limpieza.json).")
    usar = {real: logica for logica, real in mapa.items() if real is not None}
    for extra in columnas_extra:
        if extra in cab and extra not in usar:
            usar[extra] = extra
    try:
        df = pd.read_csv(ruta, sep=sep, encoding=cod, usecols=list(usar), dtype="category",
                         keep_default_na=False, na_values=[], low_memory=False)
    except UnicodeDecodeError:
        cod = "latin-1"
        df = pd.read_csv(ruta, sep=sep, encoding=cod, usecols=list(usar), dtype="category",
                         keep_default_na=False, na_values=[], low_memory=False)
    df = df.rename(columns=usar)
    info = {"columnas_reales": list(cab), "mapa_columnas": mapa, "codificacion_usada": cod,
            "delimitador": sep, "filas_largas": len(df)}
    return df, info


# ---------------------------------------------------------------------------------------------
# Parseo por categorías
# ---------------------------------------------------------------------------------------------

def _por_categoria(serie: pd.Series, funcion) -> np.ndarray:
    """Aplica `funcion` a las categorías (valores distintos) y expande con los códigos."""
    cats = pd.Index(serie.cat.categories.astype(str))
    vals = np.asarray(funcion(cats))
    codes = serie.cat.codes.to_numpy()
    if vals.dtype.kind == "M":
        fuera = np.array(["NaT"], dtype=vals.dtype)
    elif vals.dtype.kind == "f":
        fuera = np.array([np.nan])
    else:
        fuera = np.array([None], dtype=object)
        vals = vals.astype(object)
    return np.concatenate([vals, fuera])[codes]  # código -1 (faltante) -> último


def parsear_numero(cats, decimal="auto") -> np.ndarray:
    s = pd.Series(cats, dtype=object).astype(str).str.strip()
    if decimal == ",":
        s = s.str.replace(".", "", regex=False).str.replace(",", ".", regex=False)
    elif decimal == "auto":
        sin_punto = ~s.str.contains(".", regex=False)
        s = s.where(~sin_punto, s.str.replace(",", ".", regex=False))
    return pd.to_numeric(s, errors="coerce").to_numpy(dtype=float)


_RE_HORA = re.compile(r"^\s*(\d{1,2})(?::(\d{1,2}))?(?::(\d{1,2}))?(?:\.\d+)?\s*$")


def parsear_hora(cats) -> np.ndarray:
    """'01:00', '1', '01:00:00', '24:00' -> segundos desde la medianoche; inválida -> NaN."""
    out = np.full(len(cats), np.nan)
    for i, c in enumerate(cats):
        m = _RE_HORA.match(str(c))
        if not m:
            continue
        h, mi, se = int(m.group(1)), int(m.group(2) or 0), int(m.group(3) or 0)
        if h > 24 or mi > 59 or se > 59 or (h == 24 and (mi or se)):
            continue
        out[i] = h * 3600 + mi * 60 + se
    return out


_RE_TIENE_HORA = re.compile(r"\d{1,2}:\d{2}")
_RE_24 = re.compile(r"^(.*\S)[ T]24:00(?::00)?$")


def parsear_fecha(cats, formato=None, dia_primero=True) -> np.ndarray:
    """Fechas (o fecha-hora) distintas -> datetime64. '... 24:00' se lee como las 00:00 del día
    siguiente (convención de fin de hora); el resto se infiere o usa `formato`."""
    cats = pd.Index(cats).astype(str).str.strip()
    m24 = np.array([bool(_RE_24.match(c)) for c in cats], dtype=bool)
    if m24.any():
        base = parsear_fecha(pd.Index([_RE_24.match(c).group(1) for c in cats[m24]]),
                             formato.split(" ")[0] if formato else None, dia_primero)
        resto = parsear_fecha(cats[~m24], formato, dia_primero) if (~m24).any() else np.array([], dtype="datetime64[ns]")
        out = np.empty(len(cats), dtype="datetime64[ns]")
        out[m24] = base + np.timedelta64(1, "D")
        out[~m24] = resto
        return out
    if formato:
        r = pd.to_datetime(cats, format=formato, errors="coerce")
    else:
        r = pd.to_datetime(cats, dayfirst=dia_primero, errors="coerce")
        validos = (cats != "")
        if validos.sum() and (pd.isna(r) & validos).mean() > 0.01:
            r = pd.to_datetime(cats, format="mixed", dayfirst=dia_primero, errors="coerce")
    return np.asarray(r, dtype="datetime64[ns]")


# ---------------------------------------------------------------------------------------------
# Reglas de limpieza 1-6 (spec PC3 §4.2) y pivot (regla 7)
# ---------------------------------------------------------------------------------------------

ORDEN_REGLAS = [
    ("r1_periodo", "1. Periodo no horario"),
    ("r1_parametro", "1. Parámetro fuera del mapeo canónico"),
    ("r2_fecha", "2. Fecha u hora no parseable o fuera de rango"),
    ("r3_valor", "3. Valor no numérico, centinela o negativo"),
    ("r6_unidad", "6. Unidad no reconocida para el parámetro"),
    ("r4_rango", "4. Fuera del rango físico del parámetro"),
    ("r5_duplicado", "5. Duplicado (estación, parámetro, fecha-hora): se conserva el primero"),
]


def limpiar(largo: pd.DataFrame, config) -> tuple[pd.DataFrame, dict]:
    """Aplica las reglas 1-6 en este orden: 1 (periodo, parámetro) -> 2 (fecha) -> 3 (valor) ->
    6 (unidad: normaliza o descarta) -> 4 (rango, ya en unidad canónica) -> 5 (duplicados).
    Cada fila descartada cuenta en la primera regla que incumple. Devuelve el largo válido
    (estacion, parametro canónico, fecha_hora, valor) en orden de archivo y el reporte."""
    n = len(largo)
    vivo = np.ones(n, dtype=bool)
    conteos, detalle, marcas = {}, {}, {}

    def descartar(clave, mascara):
        m = mascara & vivo
        conteos[clave] = int(m.sum())
        vivo[m] = False
        return m

    # Regla 1: periodo horario.
    if "periodo" in largo and config.get("periodos_horarios"):
        horarios = {normalizar(p) for p in config["periodos_horarios"]}
        ok = _por_categoria(largo["periodo"], lambda c: np.array([normalizar(x) in horarios for x in c]))
        ok = ok.astype(bool)
        m = descartar("r1_periodo", ~ok)
        detalle["r1_periodo"] = (largo.loc[m, "periodo"].astype(str).value_counts().head(20).to_dict())
    else:
        conteos["r1_periodo"] = 0
        detalle["r1_periodo"] = "sin columna periodo: regla no aplicada"

    # Regla 1: parámetro con nombre canónico.
    mp = mapa_parametros(config)
    canon = _por_categoria(largo["parametro"], lambda c: np.array([mp.get(normalizar(x)) for x in c], dtype=object))
    m = descartar("r1_parametro", pd.isna(canon))
    detalle["r1_parametro"] = largo.loc[m, "parametro"].astype(str).value_counts().to_dict()

    # Regla 2: fecha y hora.
    fecha = _por_categoria(largo["fecha"], lambda c: parsear_fecha(c, config.get("formato_fecha"),
                                                                   config.get("dia_primero", True)))
    if "hora" not in largo:  # fecha y hora juntas: una lectura horaria sin hora no es válida
        con_hora = _por_categoria(largo["fecha"], lambda c: np.array([bool(_RE_TIENE_HORA.search(x)) for x in c]))
        fecha = np.where(con_hora.astype(bool), fecha, np.datetime64("NaT"))
    fh = pd.to_datetime(fecha)
    if "hora" in largo:
        seg = _por_categoria(largo["hora"], parsear_hora)
        fh = fh.normalize() + pd.to_timedelta(seg, unit="s")
    fh = pd.DatetimeIndex(fh)
    if config.get("convencion_hora") == "fin":
        fh = fh - pd.Timedelta(hours=1)
    con_minutos = ~np.array(fh.isna(), dtype=bool) & np.asarray((fh.minute != 0) | (fh.second != 0))
    marcas["r2_minutos_truncados"] = int((con_minutos & vivo).sum())
    fh = fh.floor("h")
    fmin = pd.Timestamp(config["fecha_minima"]) if config.get("fecha_minima") else None
    fmax = pd.Timestamp(config["fecha_maxima"]) if config.get("fecha_maxima") else None
    nat = np.array(fh.isna(), dtype=bool)  # copia: el índice cachea su máscara de NaT
    malo = nat.copy()
    if fmin is not None:
        malo |= np.asarray(fh < fmin)
    if fmax is not None:
        malo |= np.asarray(fh > fmax)
    m = descartar("r2_fecha", malo)
    detalle["r2_fecha"] = {"no_parseable": int((nat & m).sum()), "fuera_de_rango": int((~nat & m).sum())}

    # Regla 3: valor numérico, centinelas, negativos.
    valor = _por_categoria(largo["valor"], lambda c: parsear_numero(c, config.get("decimal", "auto")))
    no_num = np.isnan(valor)
    cent = np.isin(valor, np.asarray(config.get("centinelas", []), dtype=float))
    neg_ok = pd.Series(canon, dtype=object).isin(list(config.get("negativos_permitidos", []))).to_numpy()
    neg = (valor < 0) & ~neg_ok & ~cent
    m3 = descartar("r3_valor", no_num | cent | neg)
    detalle["r3_valor"] = {"no_numerico": int((no_num & m3).sum()), "centinela": int((cent & m3).sum()),
                           "negativo": int((neg & m3 & ~no_num).sum())}

    # Regla 6: unidad -> canónica.
    fac = np.ones(n)
    if "unidad" in largo and config.get("unidades"):
        tabla = factores_unidad(config)
        vacia_ok = config.get("unidad_vacia_es_canonica", True)
        un = _por_categoria(largo["unidad"], lambda c: np.array([normalizar(x) for x in c], dtype=object))
        un = np.where(pd.isna(un), "", un)
        cp = pd.Categorical(pd.Series(canon, dtype=object))
        cu = pd.Categorical(pd.Series(un, dtype=object))
        combo = cp.codes.astype(np.int64) * (len(cu.categories) + 1) + cu.codes.astype(np.int64)
        unicos, inversa = np.unique(combo, return_inverse=True)
        f_unico = np.empty(len(unicos))
        for i, cod in enumerate(unicos):
            ip, iu = divmod(int(cod), len(cu.categories) + 1)
            p = cp.categories[ip] if ip >= 0 and cod >= 0 and ip < len(cp.categories) else None
            u = cu.categories[iu] if iu < len(cu.categories) else ""
            if p is None or p not in tabla:
                f_unico[i] = 1.0
            elif u == "" and vacia_ok:
                f_unico[i] = 1.0
            else:
                f_unico[i] = tabla[p].get(u, np.nan)
        fac = f_unico[inversa.ravel()] if n else fac
        m6 = descartar("r6_unidad", np.isnan(fac))
        detalle["r6_unidad"] = (pd.Series([f"{p} [{u}]" for p, u in zip(canon[m6], un[m6])])
                                .value_counts().head(20).to_dict()) if m6.any() else {}
        marcas["r6_convertidas"] = int((vivo & (fac != 1.0)).sum())
        fac = np.where(np.isnan(fac), 1.0, fac)
    else:
        conteos["r6_unidad"] = 0
        detalle["r6_unidad"] = "sin columna unidad: regla no aplicada"
        marcas["r6_convertidas"] = 0
    valor = valor * fac

    # Regla 4: rango físico, en unidad canónica.
    lo = np.full(n, -np.inf)
    hi = np.full(n, np.inf)
    for p, (a, b) in (config.get("rangos") or {}).items():
        sel = canon == p
        lo[sel], hi[sel] = a, b
    fuera = (valor < lo) | (valor > hi)
    m4 = descartar("r4_rango", fuera)
    detalle["r4_rango"] = pd.Series(canon[m4]).value_counts().to_dict() if m4.any() else {}

    # Regla 5: duplicados (estación, parámetro, fecha-hora); se conserva el primero del archivo.
    est = _por_categoria(largo["estacion"], lambda c: np.array([str(x).strip() for x in c], dtype=object))
    validos = pd.DataFrame({
        "estacion": est[vivo], "parametro": canon[vivo],
        "fecha_hora": np.asarray(fh)[vivo], "valor": valor[vivo],
    }, index=np.flatnonzero(vivo))
    dup = validos.duplicated(["estacion", "parametro", "fecha_hora"], keep="first")
    primero = validos.groupby(["estacion", "parametro", "fecha_hora"], sort=False)["valor"].transform("first")
    distinto = dup & (validos["valor"] != primero)
    conteos["r5_duplicado"] = int(dup.sum())
    marcas["r5_valor_distinto"] = int(distinto.sum())
    vivo[validos.index[dup.to_numpy()]] = False
    validos = validos[~dup.to_numpy()]

    descartadas = int(sum(conteos.values()))
    reporte = {
        "filas_largas": {"leidas": int(n), "procesadas": int(len(validos)), "descartadas": descartadas},
        "reglas": {k: int(conteos.get(k, 0)) for k, _ in ORDEN_REGLAS},
        "marcas": marcas,
        "detalle": detalle,
        "orden_de_aplicacion": [k for k, _ in ORDEN_REGLAS],
    }
    assert reporte["filas_largas"]["leidas"] == reporte["filas_largas"]["procesadas"] + descartadas
    validos["estacion"] = validos["estacion"].astype("category")
    validos["parametro"] = validos["parametro"].astype("category")
    return validos.reset_index(names="fila"), reporte


def pivotar(validos: pd.DataFrame) -> pd.DataFrame:
    """Regla 7: una fila ancha por (estación, fecha-hora); parámetro faltante -> NaN. Columnas de
    parámetros en orden alfabético; filas ordenadas por estación y fecha-hora."""
    v = validos.copy()
    v["estacion"] = v["estacion"].astype(str)
    v["parametro"] = v["parametro"].astype(str)
    ancho = v.pivot(index=["estacion", "fecha_hora"], columns="parametro", values="valor")
    ancho = ancho[sorted(ancho.columns)]
    ancho.columns.name = None
    ancho = ancho.reset_index().sort_values(["estacion", "fecha_hora"], kind="mergesort")
    return ancho.reset_index(drop=True)


def completas(ancho: pd.DataFrame, features) -> pd.Series:
    """Regla 8: la fila entra al modelo solo si tiene todas las features (máscara booleana)."""
    base = set(features) - set(DERIVADAS)
    if "fraccion_fina" in features:
        base |= {"pm2_5", "pm10"}
    faltan = [c for c in base if c not in ancho]
    if faltan:
        return pd.Series(False, index=ancho.index)
    m = ancho[list(base)].notna().all(axis=1)
    if "fraccion_fina" in features:
        m &= ancho["pm10"] > 0
    return m


# ---------------------------------------------------------------------------------------------
# Contrato silver y gold (mismo formato que Go)
# ---------------------------------------------------------------------------------------------

def formato_fecha_hora(serie: pd.Series) -> pd.Series:
    return serie.dt.strftime("%Y-%m-%dT%H:%M:%S") + DESFASE_LIMA


def escribir_csv_contrato(df: pd.DataFrame, ruta) -> None:
    """estacion,fecha_hora,<columnas>; coma; vacío = faltante; fecha RFC3339 con -05:00; floats
    con la representación más corta que vuelve al mismo float64 (como strconv con -1 en Go)."""
    out = df.copy()
    out["fecha_hora"] = formato_fecha_hora(out["fecha_hora"])
    Path(ruta).parent.mkdir(parents=True, exist_ok=True)
    with open(ruta, "w", encoding="utf-8", newline="") as fh:
        w = csv.writer(fh, lineterminator="\n")
        w.writerow(list(out.columns))
        cols = [out[c].to_numpy() for c in out.columns]
        for fila in zip(*cols):
            w.writerow(["" if (isinstance(x, float) and math.isnan(x)) else
                        (repr(float(x)) if isinstance(x, (float, np.floating)) else x) for x in fila])


def leer_csv_contrato(ruta) -> pd.DataFrame:
    """Lee silver, gold o asignaciones: fecha_hora RFC3339 -> hora local de Lima sin zona."""
    df = pd.read_csv(ruta, dtype={"estacion": str}, keep_default_na=True)
    fh = pd.to_datetime(df["fecha_hora"], utc=True, format="ISO8601")
    df["fecha_hora"] = fh.dt.tz_convert("Etc/GMT+5").dt.tz_localize(None)  # GMT+5 = UTC-5
    return df


def fraccion_fina(ancho: pd.DataFrame) -> pd.Series:
    """PM2.5 / PM10 recortada a [0, 1]; faltante si falta alguna o si PM10 <= 0."""
    if "pm2_5" not in ancho or "pm10" not in ancho:
        return pd.Series(np.nan, index=ancho.index)
    pm10 = ancho["pm10"].where(ancho["pm10"] > 0)
    return (ancho["pm2_5"] / pm10).clip(0, 1)


def matriz_features(ancho: pd.DataFrame, features, log1p=()) -> pd.DataFrame:
    """Columnas de la receta en su orden, con log1p donde corresponda (sin estandarizar)."""
    out = pd.DataFrame(index=ancho.index)
    for f in features:
        out[f] = fraccion_fina(ancho) if f == "fraccion_fina" else ancho[f] if f in ancho else np.nan
    for f in log1p:
        out[f] = np.log1p(out[f])
    return out


def construir_gold(ancho: pd.DataFrame, receta: dict) -> tuple[pd.DataFrame, dict]:
    """Gold con la receta: features en orden, log1p, solo filas completas (regla 8), z-score con
    desviación poblacional (ddof=0, como Welford en Go), global ('absoluto') o por estación."""
    feats = list(receta["features"])
    logs = list(receta.get("log1p") or [])
    nivel = receta.get("nivel", "absoluto")
    m = completas(ancho, feats)
    base = ancho.loc[m, ["estacion", "fecha_hora"]].reset_index(drop=True)
    X = matriz_features(ancho.loc[m], feats, logs).reset_index(drop=True)
    if nivel == "absoluto":
        media = X.mean(axis=0).to_numpy()
        desv = X.std(axis=0, ddof=0).to_numpy()
        desv_seg = np.where(desv > 0, desv, 1.0)
        Z = (X.to_numpy() - media) / desv_seg
        stats = {"features": feats, "log1p": logs, "nivel": nivel, "n": int(len(X)),
                 "media": media.tolist(), "desv": desv.tolist()}
    elif nivel == "estacion":
        Z = np.empty(X.shape)
        media, desv = {}, {}
        for est, idx in base.groupby("estacion", sort=True).groups.items():
            sub = X.loc[idx]
            mu = sub.mean(axis=0).to_numpy()
            sd = sub.std(axis=0, ddof=0).to_numpy()
            Z[np.asarray(idx)] = (sub.to_numpy() - mu) / np.where(sd > 0, sd, 1.0)
            media[est], desv[est] = mu.tolist(), sd.tolist()
        stats = {"features": feats, "log1p": logs, "nivel": nivel, "n": int(len(X)),
                 "media": media, "desv": desv}
    else:
        raise ValueError(f"nivel desconocido: {nivel}")
    gold = pd.concat([base, pd.DataFrame(Z, columns=feats)], axis=1)
    return gold, stats


def desestandarizar(centroides_z: np.ndarray, stats: dict) -> np.ndarray | None:
    """Centroides a unidades originales (solo nivel absoluto): z*desv + media y expm1 en log1p."""
    if stats.get("nivel") != "absoluto":
        return None
    C = np.asarray(centroides_z) * np.asarray(stats["desv"]) + np.asarray(stats["media"])
    for j, f in enumerate(stats["features"]):
        if f in (stats.get("log1p") or []):
            C[:, j] = np.expm1(C[:, j])
    return C


def leer_receta(candidatas) -> tuple[dict, str]:
    for ruta in candidatas:
        if Path(ruta).exists():
            with open(ruta, encoding="utf-8") as fh:
                return json.load(fh), str(ruta)
    raise FileNotFoundError(f"No encuentro configs/kmeans.json en {list(map(str, candidatas))}")


# ---------------------------------------------------------------------------------------------
# ECA y promedios móviles
# ---------------------------------------------------------------------------------------------

def excede_horario(ancho: pd.DataFrame) -> pd.Series:
    """Etiqueta horaria: PM2.5 > 50 o PM10 > 100 µg/m³ (faltante cuenta como no)."""
    e = pd.Series(False, index=ancho.index)
    for p, u in ECA.items():
        if p in ancho:
            e |= ancho[p] > u
    return e


def regularizar(ancho: pd.DataFrame, columnas) -> pd.DataFrame:
    """Rejilla horaria completa por estación (entre su primera y última fila), con NaN en las
    horas sin lectura. Útil para huecos y promedios móviles."""
    partes = []
    for est, g in ancho.groupby("estacion", sort=True):
        g = g.set_index("fecha_hora")[[c for c in columnas if c in g]]
        idx = pd.date_range(g.index.min(), g.index.max(), freq="h")
        g = g.reindex(idx)
        g.index.name = "fecha_hora"
        g = g.reset_index()
        g.insert(0, "estacion", est)
        partes.append(g)
    return pd.concat(partes, ignore_index=True)


def movil_24h(rejilla: pd.DataFrame, min_horas=18) -> pd.DataFrame:
    """Promedio móvil de 24 h (ventana que termina en la hora) de PM2.5 y PM10 por estación."""
    out = rejilla[["estacion", "fecha_hora"]].copy()
    for p in ECA:
        if p in rejilla:
            out[p + "_24h"] = (rejilla.groupby("estacion", sort=False)[p]
                               .transform(lambda s: s.rolling(24, min_periods=min_horas).mean()))
    e = pd.Series(False, index=out.index)
    hay = pd.Series(False, index=out.index)
    for p, u in ECA.items():
        c = p + "_24h"
        if c in out:
            e |= out[c] > u
            hay |= out[c].notna()
    out["excede_24h"] = e
    out["hay_24h"] = hay
    return out


# ---------------------------------------------------------------------------------------------
# Huecos
# ---------------------------------------------------------------------------------------------

def huecos(ancho: pd.DataFrame, columnas=("pm2_5", "pm10")) -> pd.DataFrame:
    """Tramos sin ninguna de `columnas` entre la primera y la última hora con dato de cada
    estación: estacion, inicio, horas."""
    cols = [c for c in columnas if c in ancho]
    filas = []
    for est, g in ancho.groupby("estacion", sort=True):
        t = g.loc[g[cols].notna().any(axis=1), "fecha_hora"].sort_values()
        if len(t) < 2:
            continue
        h = ((t - t.iloc[0]) / pd.Timedelta(hours=1)).to_numpy().astype(np.int64)
        d = np.diff(h)
        for i in np.flatnonzero(d > 1):
            filas.append((est, t.iloc[0] + pd.Timedelta(hours=int(h[i] + 1)), int(d[i] - 1)))
    return pd.DataFrame(filas, columns=["estacion", "inicio", "horas"])


# ---------------------------------------------------------------------------------------------
# Coordenadas: UTM -> lat/lon (WGS84) y distancias
# ---------------------------------------------------------------------------------------------

def zona_utm(texto, sur_por_defecto=True) -> tuple[float, bool]:
    """'18', '18S', '18 L', 'Zona 18 Sur' -> (18, True). Perú está en el hemisferio sur (bandas
    K, L, M); 'S' se lee como Sur. Solo 'N'/'Norte' explícitos pasan al norte."""
    t = str(texto).strip().upper()
    m = re.search(r"(\d{1,2})", t)
    if not m:
        return np.nan, sur_por_defecto
    resto = t[m.end():].strip()
    sur = sur_por_defecto
    if resto in ("N", "NORTE"):
        sur = False
    return float(m.group(1)), sur


def utm_a_latlon(este, norte, zona, sur=True):
    """Inversa de la proyección UTM sobre WGS84 (Snyder 1987, serie con error < 1 m). Un datum
    PSAD56 desplaza unos 300-400 m: irrelevante para agrupar estaciones a 15 km."""
    este = np.asarray(este, dtype=float)
    norte = np.asarray(norte, dtype=float)
    zona = np.asarray(zona, dtype=float)
    sur = np.asarray(sur, dtype=bool)
    a, f, k0 = 6378137.0, 1 / 298.257223563, 0.9996
    e2 = f * (2 - f)
    ep2 = e2 / (1 - e2)
    x = este - 500000.0
    y = np.where(sur, norte - 10000000.0, norte)
    lon0 = np.radians((zona - 1) * 6 - 180 + 3)
    M = y / k0
    mu = M / (a * (1 - e2 / 4 - 3 * e2**2 / 64 - 5 * e2**3 / 256))
    e1 = (1 - np.sqrt(1 - e2)) / (1 + np.sqrt(1 - e2))
    phi1 = (mu + (3 * e1 / 2 - 27 * e1**3 / 32) * np.sin(2 * mu)
            + (21 * e1**2 / 16 - 55 * e1**4 / 32) * np.sin(4 * mu)
            + (151 * e1**3 / 96) * np.sin(6 * mu) + (1097 * e1**4 / 512) * np.sin(8 * mu))
    C1 = ep2 * np.cos(phi1) ** 2
    T1 = np.tan(phi1) ** 2
    N1 = a / np.sqrt(1 - e2 * np.sin(phi1) ** 2)
    R1 = a * (1 - e2) / (1 - e2 * np.sin(phi1) ** 2) ** 1.5
    D = x / (N1 * k0)
    lat = phi1 - (N1 * np.tan(phi1) / R1) * (
        D**2 / 2 - (5 + 3 * T1 + 10 * C1 - 4 * C1**2 - 9 * ep2) * D**4 / 24
        + (61 + 90 * T1 + 298 * C1 + 45 * T1**2 - 252 * ep2 - 3 * C1**2) * D**6 / 720)
    lon = lon0 + (D - (1 + 2 * T1 + C1) * D**3 / 6
                  + (5 - 2 * C1 + 28 * T1 - 3 * C1**2 + 8 * ep2 + 24 * T1**2) * D**5 / 120) / np.cos(phi1)
    return np.degrees(lat), np.degrees(lon)


def haversine_km(lat1, lon1, lat2, lon2):
    r = 6371.0088
    p1, p2 = np.radians(lat1), np.radians(lat2)
    dp, dl = p2 - p1, np.radians(np.asarray(lon2) - np.asarray(lon1))
    h = np.sin(dp / 2) ** 2 + np.cos(p1) * np.cos(p2) * np.sin(dl / 2) ** 2
    return 2 * r * np.arcsin(np.sqrt(h))


def grupos_cercanos(dist: pd.DataFrame, umbral_km: float) -> list[list[str]]:
    """Componentes conexas del grafo 'a menos de umbral_km' (solo grupos de 2 o más)."""
    nombres = list(dist.index)
    padre = {n: n for n in nombres}

    def raiz(n):
        while padre[n] != n:
            padre[n] = padre[padre[n]]
            n = padre[n]
        return n

    for i, a in enumerate(nombres):
        for b in nombres[i + 1:]:
            if dist.loc[a, b] < umbral_km:
                padre[raiz(a)] = raiz(b)
    comp = {}
    for n in nombres:
        comp.setdefault(raiz(n), []).append(n)
    return sorted([sorted(g) for g in comp.values() if len(g) > 1], key=len, reverse=True)


# ---------------------------------------------------------------------------------------------
# Estadística pequeña
# ---------------------------------------------------------------------------------------------

def eta2(valores: pd.Series, grupos: pd.Series) -> float:
    """Fracción de la varianza explicada por las medias de grupo (p. ej. la hora del día)."""
    d = pd.DataFrame({"v": valores, "g": grupos}).dropna()
    if len(d) < 3 or d["v"].var(ddof=0) == 0:
        return float("nan")
    medias = d.groupby("g")["v"].transform("mean")
    return float(((medias - d["v"].mean()) ** 2).sum() / ((d["v"] - d["v"].mean()) ** 2).sum())


def ic_bootstrap_cociente(a: np.ndarray, b: np.ndarray, n=2000, semilla=0, nivel=0.95):
    """IC por bootstrap del cociente de medias mean(a)/mean(b), remuestreando cada grupo."""
    a, b = np.asarray(a, float), np.asarray(b, float)
    a, b = a[~np.isnan(a)], b[~np.isnan(b)]
    if len(a) < 2 or len(b) < 2 or b.mean() == 0:
        return float("nan"), float("nan"), float("nan")
    rng = np.random.default_rng(semilla)
    # a: una media por grupo (p. ej. por hueco), se remuestrea por grupos; b: el resto, enorme,
    # aproximado por la normal de su media (su varianza de muestreo es despreciable a su lado).
    ra = np.array([a[rng.integers(0, len(a), len(a))].mean() for _ in range(n)])
    rb = rng.normal(b.mean(), b.std(ddof=1) / np.sqrt(len(b)), n)
    r = ra / rb
    q = (1 - nivel) / 2
    return float(a.mean() / b.mean()), float(np.quantile(r, q)), float(np.quantile(r, 1 - q))


def codo(ks, inercias) -> int:
    """k del codo por la distancia máxima a la cuerda (kneedle) sobre la curva normalizada."""
    ks = np.asarray(ks, float)
    y = np.asarray(inercias, float)
    if len(ks) < 3 or np.ptp(y) == 0:
        return int(ks[0])
    xn = (ks - ks.min()) / np.ptp(ks)
    yn = (y - y.min()) / np.ptp(y)
    # curva decreciente y convexa: la cuerda va de (0,1) a (1,0); distancia = 1 - xn - yn
    return int(ks[np.argmax(1 - xn - yn)])


def emparejar(A: np.ndarray, B: np.ndarray) -> np.ndarray:
    """Permutación p que minimiza sum ||A[i] - B[p[i]]||² (asignación húngara)."""
    A, B = np.asarray(A, float), np.asarray(B, float)
    costo = ((A[:, None, :] - B[None, :, :]) ** 2).sum(axis=2)
    try:
        from scipy.optimize import linear_sum_assignment
        fila, col = linear_sum_assignment(costo)
        p = np.empty(len(A), dtype=int)
        p[fila] = col
        return p
    except ImportError:  # pragma: no cover - k chico: fuerza bruta
        import itertools
        mejor, pm = None, None
        for perm in itertools.permutations(range(len(B))):
            c = costo[np.arange(len(A)), perm].sum()
            if mejor is None or c < mejor:
                mejor, pm = c, perm
        return np.array(pm)


def asignar(X: np.ndarray, C: np.ndarray, bloque=200_000) -> tuple[np.ndarray, np.ndarray]:
    """Centroide más cercano (distancia euclídea al cuadrado; empate -> menor índice, como Go)."""
    X, C = np.asarray(X, float), np.asarray(C, float)
    lab = np.empty(len(X), dtype=np.int64)
    d2 = np.empty(len(X))
    for i in range(0, len(X), bloque):
        D = ((X[i:i + bloque, None, :] - C[None, :, :]) ** 2).sum(axis=2)
        lab[i:i + bloque] = D.argmin(axis=1)
        d2[i:i + bloque] = D[np.arange(len(D)), lab[i:i + bloque]]
    return lab, d2


# ---------------------------------------------------------------------------------------------
# Tablas en Markdown (sin depender de tabulate)
# ---------------------------------------------------------------------------------------------

def tabla_md(df: pd.DataFrame, decimales=2, indice=False) -> str:
    d = df.reset_index() if indice else df
    def fmt(x):
        if isinstance(x, (float, np.floating)):
            if math.isnan(x):
                return "—"
            return f"{x:,.{decimales}f}".replace(",", " ")
        if isinstance(x, (bool, np.bool_)):
            return "sí" if x else "no"
        if isinstance(x, (int, np.integer)):
            return f"{int(x):,}".replace(",", " ")
        return str(x).replace("|", "\\|")
    cab = "| " + " | ".join(str(c) for c in d.columns) + " |"
    sep = "|" + "|".join("---" for _ in d.columns) + "|"
    filas = ["| " + " | ".join(fmt(x) for x in fila) + " |" for fila in d.itertuples(index=False)]
    return "\n".join([cab, sep] + filas)


# ---------------------------------------------------------------------------------------------
# Estilo de las figuras (paleta validada: categórica en orden fijo, secuencial azul,
# divergente azul-rojo con gris al medio; ver el skill de dataviz)
# ---------------------------------------------------------------------------------------------

PALETA = ["#2a78d6", "#eb6834", "#1baf7a", "#eda100", "#e87ba4", "#008300", "#6250d6", "#e34948"]
SUPERFICIE = "#fcfcfb"
TINTA = "#0b0b0b"
TINTA_2 = "#52514e"
REJILLA = "#e4e3df"
CRITICO = "#d03b3b"
SECUENCIAL = ["#fcfcfb", "#cde2fb", "#9ec5f4", "#6da7ec", "#3987e5", "#2a78d6", "#256abf",
              "#1c5cab", "#184f95", "#104281", "#0d366b"]
DIVERGENTE = ["#104281", "#2a78d6", "#86b6ef", "#f0efec", "#f0a3a2", "#e34948", "#a32626"]


def estilo_figuras():
    import matplotlib as mpl
    from matplotlib.colors import LinearSegmentedColormap
    mpl.rcParams.update({
        "figure.dpi": 110, "savefig.dpi": 150, "savefig.bbox": "tight",
        "figure.facecolor": SUPERFICIE, "axes.facecolor": SUPERFICIE, "savefig.facecolor": SUPERFICIE,
        "axes.edgecolor": REJILLA, "axes.labelcolor": TINTA_2, "axes.titlecolor": TINTA,
        "axes.titlesize": 12, "axes.titleweight": "bold", "axes.labelsize": 10,
        "axes.grid": True, "grid.color": REJILLA, "grid.linewidth": 0.6, "grid.linestyle": "-",
        "axes.spines.top": False, "axes.spines.right": False, "axes.axisbelow": True,
        "xtick.color": TINTA_2, "ytick.color": TINTA_2, "xtick.labelsize": 9, "ytick.labelsize": 9,
        "legend.frameon": False, "legend.fontsize": 9, "text.color": TINTA,
        "lines.linewidth": 1.6, "axes.prop_cycle": mpl.cycler(color=PALETA),
        "font.family": "DejaVu Sans",
    })
    cmaps = {
        "aire_seq": LinearSegmentedColormap.from_list("aire_seq", SECUENCIAL),
        "aire_div": LinearSegmentedColormap.from_list("aire_div", DIVERGENTE),
    }
    for nombre, cm in cmaps.items():
        try:
            mpl.colormaps.register(cm, name=nombre, force=True)
        except Exception:  # pragma: no cover
            pass
    return cmaps


def color_regimen(i: int) -> str:
    """Color fijo por id de régimen (no por rango): el régimen 3 es siempre el mismo color."""
    if i < len(PALETA):
        return PALETA[i]
    grises = ["#8c8b86", "#b5b4ae", "#5f5e5a"]
    return grises[(i - len(PALETA)) % len(grises)]


def marca_sintetico(fig, activo: bool):
    """Marca de agua para que una figura de datos sintéticos nunca pase por real."""
    if activo:
        fig.text(0.5, 0.5, "DATOS SINTÉTICOS", fontsize=42, color="#d03b3b", alpha=0.13,
                 ha="center", va="center", rotation=22, weight="bold", zorder=100)
        fig.text(0.99, 1.0, "Datos sintéticos de prueba: no son mediciones de OEFA",
                 fontsize=7, color=CRITICO, ha="right", va="bottom")


def nota_al_pie(fig, texto: str, tamano=7.5):
    """Nota debajo de todos los ejes (con sus etiquetas), sin pisar nada."""
    fig.canvas.draw()
    r = fig.canvas.get_renderer()
    inv = fig.transFigure.inverted()
    ymin = min(ax.get_tightbbox(r).transformed(inv).y0 for ax in fig.axes if ax.get_visible())
    fig.text(0.01, ymin - 0.015, texto, fontsize=tamano, color=TINTA_2, ha="left", va="top", wrap=False)


def miles(n) -> str:
    """6260 -> '6 260' (separador de miles en español)."""
    return f"{int(round(n)):,}".replace(",", " ")
