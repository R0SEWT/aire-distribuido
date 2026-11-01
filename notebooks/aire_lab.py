"""Laboratorio en Python de aire-distribuido. NO es parte del sistema.

El sistema (carga, limpieza, estandarización y K-Means) es Go. Este módulo reúne lo que los
notebooks de Colab comparten para explorar, calibrar, auditar y dibujar. **Go es la fuente de
verdad**: el prototipo de limpieza replica la cascada de `datos/bloque.go`, la regla 5 de
`datos/recolector.go`, el reporte de `datos/reporte.go`, el gold de `datos/estandarizar.go` y el
perfil de `kmeans/regimenes.go`, con los mismos nombres y el mismo esquema JSON, para que la
auditoría compare dos implementaciones de lo mismo.

- configuración del bronze: `configs/limpieza.json` con el esquema de `datos.ConfigLimpieza`,
- lectura del CSV largo y reglas de limpieza (cascada de Go), silver byte a byte como el de Go,
- gold y estandarización, perfil de regímenes y etiqueta ECA como `cmd/entrenar`,
- mediciones de la criba (huecos, ECA, ciclos, distancias), estadística chica y figuras.

Depende solo de numpy, pandas y matplotlib; scikit-learn y scipy se importan donde hacen falta.
"""

from __future__ import annotations

import copy
import csv
import datetime as dt
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
# Configuración del bronze: configs/limpieza.json, el MISMO esquema que lee Go (datos/config.go)
# ---------------------------------------------------------------------------------------------
#
# Go es la fuente de verdad: este prototipo replica la cascada de reglas de datos/bloque.go, la
# deduplicación de datos/recolector.go y el reporte de datos/reporte.go, con los mismos nombres.
# Si algo de aquí discrepa de Go, el que manda es Go y la auditoría del notebook 03 lo señala.

# = datos.ConfigPorDefecto(): los supuestos del 31/10, que describen la muestra data/sample/.
CONFIG_DEFECTO = {
    "delimitador": ";",
    "codificacion": "utf-8",
    "separador_decimal": ".",
    "zona_horaria": "America/Lima",
    "columnas": {
        "estacion": "NOMBRE_PUNTO", "fecha": "FECHA", "hora": "", "parametro": "PARAMETRO",
        "valor": "VALOR", "periodo": "PERIODO", "unidad": "UNIDAD", "departamento": "DEPARTAMENTO",
        "provincia": "PROVINCIA", "distrito": "DISTRITO", "ubigeo": "UBIGEO", "este": "ESTE",
        "norte": "NORTE", "altitud": "ALTITUD", "zona": "ZONA", "datum": "DATUM",
    },
    "parametros": {
        "PM2.5": "pm2_5", "PM 2.5": "pm2_5", "PM2,5": "pm2_5", "PM10": "pm10", "PM 10": "pm10",
        "NO2": "no2", "SO2": "so2", "CO": "co", "O3": "o3", "Temperatura": "temperatura",
        "Temperatura del aire": "temperatura", "Humedad Relativa": "humedad", "Humedad": "humedad",
        "Velocidad del Viento": "viento_velocidad", "Dirección del Viento": "viento_direccion",
        "Presión Atmosférica": "presion", "Presión": "presion", "Precipitación": "precipitacion",
        "Radiación Solar": "radiacion", "Radiación Solar Global": "radiacion",
        "Velocidad de Viento": "viento_velocidad", "Direccion de Viento": "viento_direccion",
        "Presion Barometrica": "presion", "Temperatura Ambiente": "temperatura",
        "Humedad Relativa del Aire": "humedad",
    },
    "periodos_horarios": ["Horario", "Hora", "1 h", "1h", "1 hora", "60 min"],
    "formatos_fecha": ["02/01/2006 15:04", "02/01/2006 15:04:05", "2/1/2006 15:04",
                       "2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02T15:04:05"],
    "fecha_desde": "2000-01-01",
    "fecha_hasta": "2026-07-01",
    "centinelas": [-999, -9999, 9999, 99999],
    "admiten_negativos": ["temperatura"],
    "rangos": {
        "pm2_5": [0, 1000], "pm10": [0, 2000], "no2": [0, 2000], "so2": [0, 10000],
        "co": [0, 50000], "o3": [0, 1000], "temperatura": [-30, 50], "humedad": [0, 100],
        "viento_velocidad": [0, 60], "viento_direccion": [0, 360], "presion": [400, 1100],
        "precipitacion": [0, 200], "radiacion": [0, 1500],
    },
    "unidades": {
        "pm2_5": {"ug/m3": 1}, "pm10": {"ug/m3": 1}, "no2": {"ug/m3": 1}, "so2": {"ug/m3": 1},
        "o3": {"ug/m3": 1}, "co": {"ug/m3": 1, "mg/m3": 1000}, "temperatura": {"°c": 1, "c": 1},
        "humedad": {"%": 1}, "viento_velocidad": {"m/s": 1}, "viento_direccion": {"°": 1, "grados": 1},
        "presion": {"hpa": 1, "mbar": 1, "mb": 1}, "precipitacion": {"mm": 1}, "radiacion": {"w/m2": 1},
    },
}
CLAVES_CONFIG = ["_supuestos", "delimitador", "codificacion", "separador_decimal", "zona_horaria",
                 "columnas", "parametros", "periodos_horarios", "formatos_fecha", "fecha_desde",
                 "fecha_hasta", "centinelas", "admiten_negativos", "rangos", "unidades"]
COLUMNAS_LOGICAS = ["estacion", "fecha", "hora", "parametro", "valor", "periodo", "unidad",
                    "departamento", "provincia", "distrito", "ubigeo", "este", "norte", "altitud",
                    "zona", "datum"]
COLUMNAS_UBICACION = ["departamento", "provincia", "distrito", "ubigeo", "este", "norte", "altitud",
                      "zona", "datum"]
REQUERIDAS = ["estacion", "fecha", "parametro", "valor"]

# Reglas de descarte en el orden de la cascada de Go (datos/bloque.go). Son las claves de
# filas_largas.descartadas_por_regla en reports/limpieza.json.
REGLAS = [
    ("columnas", "La fila no tiene tantos campos como la cabecera"),
    ("estacion", "Estación vacía"),
    ("periodo", "Regla 1: el periodo no es horario"),
    ("parametro", "El parámetro no está en el mapeo a canónicos"),
    ("fecha", "Regla 2: fecha y hora no parseables con ningún formato"),
    ("fecha_fuera_de_rango", "Regla 2: fuera de [fecha_desde, fecha_hasta)"),
    ("valor_no_numerico", "Regla 3: vacío, texto, NaN o infinito"),
    ("centinela", "Regla 3: código del equipo (sobre el valor crudo)"),
    ("unidad", "Regla 6: unidad que no figura para ese parámetro"),
    ("negativo", "Regla 3: negativo en un parámetro que no los admite"),
    ("fuera_de_rango", "Regla 4: fuera del rango físico, ya en la unidad canónica"),
    ("duplicado_exacto", "Regla 5: misma (estación, parámetro, fecha-hora) y mismo valor"),
    ("duplicado_valor_distinto", "Regla 5: misma clave y otro valor; se conserva el primero del archivo"),
]
NOMBRES_REGLAS = [r for r, _ in REGLAS]

# Alias de columnas: solo para SUGERIR un nombre cuando el de la config no está en la cabecera.
# El prototipo, como Go, usa únicamente lo que dice la config.
ALIAS_COLUMNAS = {
    "estacion": ["nombre_punto", "estacion", "nombre_estacion", "punto", "punto_monitoreo"],
    "fecha": ["fecha", "fecha_hora", "fecha_medicion", "fecha_muestreo", "fechahora"],
    "hora": ["hora", "hora_medicion"],
    "parametro": ["parametro", "nombre_parametro", "variable", "contaminante"],
    "valor": ["valor", "resultado", "concentracion", "medicion"],
    "periodo": ["periodo", "periodo_medicion", "tiempo_promedio"],
    "unidad": ["unidad", "unidad_medida", "unidades"],
}

_ACENTOS = str.maketrans({"á": "a", "à": "a", "ä": "a", "â": "a", "é": "e", "è": "e", "ë": "e",
                          "ê": "e", "í": "i", "ì": "i", "ï": "i", "î": "i", "ó": "o", "ò": "o",
                          "ö": "o", "ô": "o", "ú": "u", "ù": "u", "ü": "u", "û": "u", "µ": "u",
                          "μ": "u", "ñ": "n", "³": "3", "²": "2"})


def normalizar(s) -> str:
    """= datos.normalizar: minúsculas, sin tildes, µ y μ como u, superíndices como dígitos y
    espacios colapsados (y recortados). Conserva la puntuación: «µg/m³» -> «ug/m3»."""
    if s is None or (isinstance(s, float) and math.isnan(s)):
        return ""
    return " ".join(str(s).lower().translate(_ACENTOS).split())


def normalizar_columna(texto) -> str:
    t = unicodedata.normalize("NFKD", str(texto).strip().lower())
    t = "".join(c for c in t if not unicodedata.combining(c))
    return re.sub(r"[^a-z0-9]+", "_", t).strip("_")


def _fusionar(base: dict, encima: dict) -> dict:
    out = copy.deepcopy(base)
    for k, v in encima.items():
        if isinstance(v, dict) and isinstance(out.get(k), dict):
            out[k] = _fusionar(out[k], v)
        else:
            out[k] = copy.deepcopy(v)
    return out


def fusionar(base: dict, encima: dict) -> dict:
    """Fusión profunda de diccionarios (encima pisa a base). Para probar ajustes en el notebook."""
    return _fusionar(base, encima)


def cargar_config(candidatas) -> tuple[dict, str]:
    """Lee el primer configs/limpieza.json que exista, como Go: el JSON es la config completa (no
    se fusiona con los valores por defecto; una clave que falta vale vacío) y un campo desconocido
    es un error. Sin JSON, usa CONFIG_DEFECTO (= datos.ConfigPorDefecto)."""
    for ruta in candidatas:
        ruta = Path(ruta)
        if ruta.exists():
            with open(ruta, encoding="utf-8") as fh:
                propio = json.load(fh)
            desconocidas = sorted(set(propio) - set(CLAVES_CONFIG))
            if desconocidas:
                raise ValueError(f"{ruta}: campos desconocidos {desconocidas} (Go los rechaza)")
            vacio = {"columnas": {}, "parametros": {}, "periodos_horarios": [], "formatos_fecha": [],
                     "centinelas": [], "admiten_negativos": [], "rangos": {}, "unidades": {}}
            cfg = {k: copy.deepcopy(vacio.get(k, "")) for k in CLAVES_CONFIG if k != "_supuestos"}
            cfg.update(copy.deepcopy(propio))
            return cfg, str(ruta)
    return copy.deepcopy(CONFIG_DEFECTO), "datos.ConfigPorDefecto() (no hay configs/limpieza.json)"


# --- Formatos de fecha de Go (time.Parse), en estricto ----------------------------------------

_TOKENS_GO = [  # (token del layout, regex, campo); el orden importa: los largos primero
    ("2006", r"(?P<Y>\d{4})", "Y"), ("01", r"(?P<m>\d{2})", "m"), ("02", r"(?P<d>\d{2})", "d"),
    ("15", r"(?P<H>\d\d?)", "H"), ("04", r"(?P<M>\d{2})", "M"), ("05", r"(?P<S>\d{2})(?:[.,]\d+)?", "S"),
    ("1", r"(?P<m>\d\d?)", "m"), ("2", r"(?P<d>\d\d?)", "d"), ("4", r"(?P<M>\d\d?)", "M"),
    ("5", r"(?P<S>\d\d?)(?:[.,]\d+)?", "S"),
]
_NO_SOPORTADOS = ("January", "Jan", "Monday", "Mon", "MST", "PM", "pm", "Z07", "-07", "03", "06",
                  "_2", "002", "3")  # tokens de Go que el prototipo no implementa


def compilar_formato_go(layout: str) -> re.Pattern:
    """Layout de Go (p. ej. «02/01/2006 15:04») -> regex con la misma rigidez que time.Parse:
    «02», «01», «04», «05» exigen dos dígitos; «2», «1», «15» aceptan uno o dos; «2006» cuatro;
    tras los segundos se admite una fracción. Lo demás es literal y no puede sobrar texto."""
    partes, i = [], 0
    while i < len(layout):
        for tok, rx, _ in _TOKENS_GO:
            if layout.startswith(tok, i):
                partes.append(rx)
                i += len(tok)
                break
        else:
            malo = next((t for t in _NO_SOPORTADOS if layout.startswith(t, i)), None)
            if malo:
                raise ValueError(f"formato de fecha {layout!r}: el prototipo no soporta {malo!r}")
            partes.append(re.escape(layout[i]))
            i += 1
    return re.compile("".join(partes))


def parsear_fecha_go(s: str, formatos) -> "dt.datetime | None":
    """Prueba los formatos en orden, como datos.trabajador.fecha; None si ninguno sirve."""
    for rx in formatos:
        m = rx.fullmatch(s)
        if not m:
            continue
        g = m.groupdict()
        try:
            h = int(g.get("H") or 0)
            if h > 23:
                continue
            return dt.datetime(int(g["Y"]), int(g["m"]), int(g["d"]), h, int(g.get("M") or 0),
                               int(g.get("S") or 0))
        except (ValueError, KeyError, TypeError):
            continue
    return None


def _limite_fecha(s: str) -> pd.Timestamp:
    """datos.parsearLimite: aaaa-mm-dd (medianoche en Lima) o RFC3339, como hora local de Lima."""
    if re.fullmatch(r"\d{4}-\d{2}-\d{2}", s.strip()):
        return pd.Timestamp(s.strip())
    t = pd.Timestamp(s)
    if t.tzinfo is not None:
        t = t.tz_convert("Etc/GMT+5").tz_localize(None)
    return t


_RE_FLOAT_GO = re.compile(r"[+-]?(?:\d+(?:\.\d*)?|\.\d+)(?:[eE][+-]?\d+)?")


def parsear_valor_go(s: str, decimal: str = ".") -> float:
    """= datos.trabajador.valor: vacío, texto, NaN o infinito -> NaN (no numérico). Sintaxis de
    strconv.ParseFloat en base 10 (los hexadecimales de Go no se admiten aquí)."""
    s = s.strip()
    if not s:
        return math.nan
    if decimal == "," and "," in s:
        s = s.replace(",", ".")
    if not _RE_FLOAT_GO.fullmatch(s):
        return math.nan
    v = float(s)
    return v if math.isfinite(v) else math.nan


def compilar(cfg: dict) -> dict:
    """= ConfigLimpieza.compilar: valida y deja la config lista para las reglas."""
    errores = []
    d = cfg.get("delimitador") or "auto"
    delim = None if d == "auto" else ("\t" if d == "\\t" else d)
    if delim is not None and (len(delim) != 1 or delim in '"\r\n'):
        errores.append(f"delimitador {d!r}")
    cod = (cfg.get("codificacion") or "utf-8").lower().replace("_", "-")
    codificacion = {"utf-8": "utf-8", "utf8": "utf-8", "latin1": "latin-1", "latin-1": "latin-1",
                    "iso-8859-1": "latin-1", "windows-1252": "cp1252", "cp1252": "cp1252"}.get(cod)
    if codificacion is None:
        errores.append(f"codificación {cfg.get('codificacion')!r}")
    decimal = cfg.get("separador_decimal") or "."
    if decimal not in (".", ","):
        errores.append(f"separador decimal {decimal!r}")
    if (cfg.get("zona_horaria") or "America/Lima") != "America/Lima":
        errores.append("el prototipo solo soporta zona_horaria America/Lima (UTC-5 fijo)")
    cols = cfg.get("columnas") or {}
    for c in REQUERIDAS:
        if not str(cols.get(c, "")).strip():
            errores.append(f"columnas.{c} es obligatoria")
    parametros = {}
    for real, m in (cfg.get("parametros") or {}).items():
        canon, factor = (m, 1.0) if isinstance(m, str) else (m.get("canonico"), float(m.get("factor", 1)))
        if canon not in CANONICOS:
            errores.append(f"parametros[{real!r}] = {canon!r} no es canónico")
            continue
        parametros[normalizar(real)] = (canon, factor)
    if not parametros:
        errores.append("parametros está vacío")
    formatos = [compilar_formato_go(f) for f in (cfg.get("formatos_fecha") or [])]
    if not formatos:
        errores.append("formatos_fecha está vacío")
    desde, hasta = _limite_fecha(cfg["fecha_desde"]), _limite_fecha(cfg["fecha_hasta"])
    if not desde < hasta:
        errores.append("fecha_desde tiene que ser anterior a fecha_hasta")
    unidades = {p: {normalizar(u): float(f) for u, f in us.items()} for p, us in (cfg.get("unidades") or {}).items()}
    if errores:
        raise ValueError("config de limpieza inválida: " + "; ".join(errores))
    return {
        "delimitador": delim, "codificacion": codificacion, "decimal": decimal, "columnas": cols,
        "parametros": parametros, "periodos": {normalizar(p) for p in cfg.get("periodos_horarios") or []},
        "formatos": formatos, "desde": desde, "hasta": hasta,
        "centinelas": [float(c) for c in cfg.get("centinelas") or []],
        "negativos": set(cfg.get("admiten_negativos") or []),
        "rangos": {p: (float(a), float(b)) for p, (a, b) in (cfg.get("rangos") or {}).items()},
        "unidades": unidades,
    }


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
    try:
        texto = crudo.decode("utf-8")
        info["codificacion_probable"] = "utf-8"
    except UnicodeDecodeError as e:
        if e.start >= len(crudo) - 4:  # el corte del MB partió un carácter multibyte
            texto = crudo[: e.start].decode("utf-8")
            info["codificacion_probable"] = "utf-8"
        else:
            texto = crudo.decode("latin-1")
            info["codificacion_probable"] = "latin1 o windows-1252 (no es UTF-8 válido)"
    texto = texto.lstrip("﻿")
    lineas = texto.splitlines()
    if lineas and not texto.endswith(("\n", "\r")):
        lineas = lineas[:-1]
    cabecera = lineas[0] if lineas else ""
    conteo = {d: cabecera.count(d) for d in [";", ",", "\t", "|"]}
    info["delimitador_probable"] = _detectar_delimitador(cabecera)
    info["conteo_delimitadores_cabecera"] = conteo
    info["cabecera"] = cabecera
    info["primeras_lineas"] = lineas[1:6]
    info["fin_de_linea"] = "CRLF" if "\r\n" in texto[:10000] else "LF"
    info["lineas_en_el_mb"] = len(lineas)
    info["hay_comillas"] = '"' in texto
    info["lineas_con_comillas_impares"] = sum(1 for l in lineas if l.count('"') % 2 == 1)
    d = info["delimitador_probable"]
    info["columnas_por_fila"] = pd.Series([len(f) for f in csv.reader(io.StringIO("\n".join(lineas)), delimiter=d)]).value_counts().to_dict()
    return info


def _detectar_delimitador(linea) -> str:
    """= datos.detectarDelimitador: el más frecuente entre ; , tab y |; ante empate, el primero."""
    mejor, cuenta = ";", 0
    for c in [";", ",", "\t", "|"]:
        n = linea.count(c if isinstance(linea, str) else c.encode())
        if n > cuenta:
            mejor, cuenta = c, n
    return mejor


def _contar_campos(linea: bytes, delim: bytes, codificacion: str) -> int:
    if b'"' not in linea:
        return linea.count(delim) + 1
    return len(next(csv.reader([linea.decode(codificacion, errors="replace")], delimiter=delim.decode())))


def escanear_lineas(ruta, delim: str, codificacion: str) -> tuple[np.ndarray, int]:
    """Campos por línea de datos, como el lector de Go: se parte por '\\n', se quita un '\\r'
    final y las líneas vacías no cuentan. Devuelve (campos de cada línea que pandas va a leer,
    líneas solo con espacios: Go las lee y las descarta por columnas, pandas las salta)."""
    campos, blancas = [], 0
    d = delim.encode()
    with open(ruta, "rb") as fh:
        fh.readline()  # cabecera
        for crudo in fh:
            if crudo.endswith(b"\n"):
                crudo = crudo[:-1]
            if crudo.endswith(b"\r"):
                crudo = crudo[:-1]
            if not crudo:
                continue
            if not crudo.strip():
                blancas += 1
                continue
            campos.append(_contar_campos(crudo, d, codificacion))
    return np.asarray(campos, dtype=np.int32), blancas


def resolver_columnas(nombres, cfg) -> tuple[dict, list, list]:
    """= datos.resolverColumnas: cada columna configurada se busca por nombre normalizado.
    Devuelve (lógica -> índice o None, errores, avisos). Sugiere alias si falta alguna."""
    pos = {}
    for i, n in enumerate(nombres):
        pos.setdefault(normalizar(n), i)
    por_alias = {normalizar_columna(n): n for n in nombres}
    cols = cfg.get("columnas") or {}
    mapa, errores, avisos = {}, [], []
    for logica in COLUMNAS_LOGICAS:
        nombre = str(cols.get(logica, "") or "")
        if not nombre.strip():
            mapa[logica] = None
            if logica in REQUERIDAS:
                errores.append(f"columnas.{logica} no está configurada")
            continue
        i = pos.get(normalizar(nombre))
        if i is None:
            sug = [por_alias[a] for a in ALIAS_COLUMNAS.get(logica, []) if a in por_alias]
            texto = f"la cabecera no tiene la columna {nombre!r} (columnas.{logica})" + (
                f"; ¿será {sug[0]!r}?" if sug else "")
            (avisos if logica in COLUMNAS_UBICACION else errores).append(texto)
        mapa[logica] = i
    return mapa, errores, avisos


def leer_bronze(ruta, cfg) -> tuple[pd.DataFrame, dict]:
    """Lee el CSV largo como Go: cabecera (sin BOM), delimitador de la config (o detectado),
    columnas resueltas por nombre normalizado. Devuelve una fila por línea de datos no vacía (en
    orden de archivo) con las columnas lógicas como categorías y `_campos` = campos de la línea,
    para la regla «columnas». Las líneas solo con espacios no vienen (pandas las salta) pero se
    cuentan en info["lineas_blancas"]."""
    reg = compilar(cfg)
    cod = reg["codificacion"]
    with open(ruta, "rb") as fh:
        primera = fh.readline()
    linea = primera[:-1] if primera.endswith(b"\n") else primera
    linea = linea[:-1] if linea.endswith(b"\r") else linea
    bom = linea.startswith(b"\xef\xbb\xbf")
    if bom:
        linea = linea[3:]
    texto = linea.decode(cod, errors="replace")
    delim = reg["delimitador"] or _detectar_delimitador(texto)
    nombres = [c.strip() for c in next(csv.reader([texto], delimiter=delim))]
    mapa, errores, avisos = resolver_columnas(nombres, cfg)
    if errores:
        raise ValueError("; ".join(errores) + f". Cabecera real ({len(nombres)} columnas): {nombres}")
    campos, blancas = escanear_lineas(ruta, delim, cod)
    ancho = max(len(nombres), int(campos.max()) if len(campos) else 0)
    usar = sorted({i for i in mapa.values() if i is not None})
    # Se lee como TEXTO por trozos y cada trozo se pasa a categoría: con dtype="category", pandas 3
    # infiere categorías numéricas y «070106» (un ubigeo) se volvería 70106.
    from pandas.api.types import union_categoricals
    partes, n_filas = {i: [] for i in usar}, 0
    for trozo in pd.read_csv(ruta, sep=delim, header=None, skiprows=1, names=list(range(ancho)), dtype=str,
                             keep_default_na=False, na_values=[], encoding=cod, encoding_errors="replace",
                             skip_blank_lines=True, chunksize=500_000):
        n_filas += len(trozo)
        for i in usar:
            partes[i].append(trozo[i].astype(object).astype("category"))
    if n_filas != len(campos):
        raise ValueError(f"pandas leyó {n_filas:,} filas y el escaneo de líneas {len(campos):,}: ¿campos "
                         "entrecomillados con saltos de línea? Go tampoco los admite (partición por bloques).")
    columnas_cat = {i: pd.Series(union_categoricals(partes[i]) if partes[i] else pd.Categorical([])) for i in usar}
    largo = pd.DataFrame({logica: columnas_cat[i] for logica, i in mapa.items() if i is not None})
    largo["_campos"] = campos
    info = {"cabecera": nombres, "bom": bom, "delimitador": delim, "codificacion": cod,
            "columnas": {k: (nombres[i] if i is not None else None) for k, i in mapa.items()},
            "indices": mapa, "avisos": avisos, "lineas_blancas": blancas,
            "leidas": int(len(campos) + blancas), "total_columnas": len(nombres)}
    return largo, info


# ---------------------------------------------------------------------------------------------
# Reglas de limpieza: la cascada de datos/bloque.go y la regla 5 de datos/recolector.go
# ---------------------------------------------------------------------------------------------

def _por_categoria(serie: pd.Series, funcion, relleno=None) -> np.ndarray:
    """Aplica `funcion` a las categorías (valores distintos) y expande con los códigos."""
    cats = [str(c) for c in serie.cat.categories]
    vals = list(funcion(cats)) + [relleno]
    return np.asarray(vals, dtype=object)[serie.cat.codes.to_numpy()]


def limpiar(largo: pd.DataFrame, cfg: dict, info: dict) -> tuple[pd.DataFrame, dict]:
    """Cascada de Go, en este orden; cada fila descartada cuenta en la primera regla que falla:
    columnas -> estacion -> periodo -> parametro -> fecha -> fecha_fuera_de_rango ->
    valor_no_numerico -> centinela (sobre el valor crudo) -> unidad -> negativo -> (factor de la
    unidad y del mapeo) -> fuera_de_rango (en la unidad canónica) -> duplicados. Devuelve las
    lecturas conservadas (estacion, parametro, fecha_hora, valor, en orden de archivo) y el
    reporte con el esquema de reports/limpieza.json."""
    reg = compilar(cfg)
    n = len(largo)
    desc = {r: 0 for r in NOMBRES_REGLAS}
    detalle = {}

    ok = largo["_campos"].to_numpy() == info["total_columnas"]
    desc["columnas"] = int((~ok).sum()) + int(info.get("lineas_blancas", 0))
    vivo = ok.copy()

    est = _por_categoria(largo["estacion"], lambda c: [x.strip() for x in c], "")
    vacia = vivo & (est == "")
    desc["estacion"] = int(vacia.sum())
    vivo &= ~vacia
    registrada = vivo.copy()  # cuentan como leídas de su estación (y entran a «vistos»)

    def strip_col(nombre):
        return _por_categoria(largo[nombre], lambda c: [x.strip() for x in c], "") if nombre in largo else None

    periodo = strip_col("periodo")
    horario = (np.ones(n, dtype=bool) if periodo is None else
               _por_categoria(largo["periodo"], lambda c: [normalizar(x.strip()) in reg["periodos"] for x in c], False).astype(bool))
    param = strip_col("parametro")
    mapeo = _por_categoria(largo["parametro"], lambda c: [reg["parametros"].get(normalizar(x.strip())) for x in c])
    canon = np.array([m[0] if m else None for m in mapeo], dtype=object)
    f_param = np.array([m[1] if m else 1.0 for m in mapeo], dtype=float)

    unidad = strip_col("unidad")
    f_unidad = np.ones(n)
    unidad_ok = np.ones(n, dtype=bool)
    if unidad is not None:
        pc, pu = pd.factorize(pd.Series(canon, dtype=object))  # None -> -1
        uc, uu = pd.factorize(pd.Series(unidad, dtype=object))
        base = len(uu) + 1
        unicos, inversa = np.unique(pc.astype(np.int64) * base + uc, return_inverse=True)
        res_u = []
        for c in unicos:
            ip, iu = divmod(int(c), base)
            p = pu[ip] if ip >= 0 else None
            u = uu[iu] if 0 <= iu < len(uu) else ""
            if p is None or p not in reg["unidades"]:
                res_u.append((True, 1.0))
            else:
                f = reg["unidades"][p].get(normalizar(u))
                res_u.append((f is not None, f if f is not None else 1.0))
        res = [res_u[i] for i in inversa.ravel()]
        unidad_ok = np.array([r[0] for r in res], dtype=bool)
        f_unidad = np.array([r[1] for r in res], dtype=float)

    # «Vistos», como Go: sobre todas las filas que llegan a la cascada (las registradas).
    vistos = {
        "parametros": pd.Series(param[registrada]).value_counts().to_dict(),
        "parametros_no_mapeados": pd.Series(param[registrada & pd.isna(canon)]).value_counts().to_dict(),
        "periodos": pd.Series(periodo[registrada]).value_counts().to_dict() if periodo is not None else {},
        "unidades": {},
    }
    if unidad is not None:
        m = registrada & ~pd.isna(canon)
        vc = pd.DataFrame({"p": canon[m], "u": unidad[m]}).value_counts()
        for (p, u), c in vc.items():
            vistos["unidades"].setdefault(p, {})[u] = int(c)
    vistos = {k: ({kk: (int(vv) if not isinstance(vv, dict) else vv) for kk, vv in v.items()}) for k, v in vistos.items()}

    def descartar(regla, mascara):
        m = vivo & mascara
        desc[regla] += int(m.sum())
        vivo[m] = False
        return m

    descartar("periodo", ~horario)
    descartar("parametro", pd.isna(canon))

    # Fecha: fecha (+ " " + hora), probando los formatos en orden; caché por texto distinto.
    if "hora" in largo:
        ff = largo["fecha"].cat.codes.to_numpy().astype(np.int64)
        hh = largo["hora"].cat.codes.to_numpy().astype(np.int64)
        fc = [str(c).strip() for c in largo["fecha"].cat.categories]
        hc = [str(c).strip() for c in largo["hora"].cat.categories]
        combo = ff * (len(hc) + 1) + hh
        unicos, inversa = np.unique(combo, return_inverse=True)
        textos = [f"{fc[c // (len(hc) + 1)]} {hc[c % (len(hc) + 1)]}" if c >= 0 else "" for c in unicos]
        fechas_u = [parsear_fecha_go(t, reg["formatos"]) for t in textos]
        fechas = np.asarray(fechas_u + [None], dtype=object)[inversa.ravel()]
    else:
        fechas = _por_categoria(largo["fecha"], lambda c: [parsear_fecha_go(x.strip(), reg["formatos"]) for x in c])
    fh = pd.to_datetime(pd.Series(fechas, dtype=object), errors="coerce").to_numpy(dtype="datetime64[ns]")
    nat = np.isnat(fh)
    m = descartar("fecha", nat)
    detalle["fecha"] = (pd.Series(_por_categoria(largo["fecha"], lambda c: c, "")[m]).value_counts().head(10).to_dict()
                        if m.any() else {})
    desde, hasta = np.datetime64(reg["desde"]), np.datetime64(reg["hasta"])
    descartar("fecha_fuera_de_rango", ~nat & ((fh < desde) | (fh >= hasta)))

    valor = _por_categoria(largo["valor"], lambda c: [parsear_valor_go(x, reg["decimal"]) for x in c], math.nan).astype(float)
    m = descartar("valor_no_numerico", np.isnan(valor))
    detalle["valor_no_numerico"] = (pd.Series(_por_categoria(largo["valor"], lambda c: c, "")[m]).value_counts().head(10).to_dict()
                                    if m.any() else {})
    descartar("centinela", np.isin(valor, np.asarray(reg["centinelas"], dtype=float)))
    m = descartar("unidad", ~unidad_ok)
    detalle["unidad"] = (pd.Series([f"{p} [{u}]" for p, u in zip(canon[m], unidad[m])]).value_counts().to_dict()
                         if m.any() else {})
    neg_ok = pd.Series(canon, dtype=object).isin(list(reg["negativos"])).to_numpy()
    descartar("negativo", (valor < 0) & ~neg_ok)

    factor = f_param * f_unidad
    valor = np.where(factor != 1, valor * factor, valor)
    valor = np.where(valor == 0, 0.0, valor)  # -0 -> 0, como Go
    lo = np.full(n, -np.inf)
    hi = np.full(n, np.inf)
    for p, (a, b) in reg["rangos"].items():
        sel = canon == p
        lo[sel], hi[sel] = a, b
    m = descartar("fuera_de_rango", (valor < lo) | (valor > hi))
    fuera_param = pd.Series(canon[m]).value_counts().to_dict() if m.any() else {}

    # Regla 5: en orden de archivo, la primera lectura de cada celda se conserva; cada lectura
    # siguiente se compara contra ese valor (exacto o con otro valor, que además marca la celda).
    validos = pd.DataFrame({"estacion": est[vivo], "parametro": canon[vivo], "fecha_hora": fh[vivo],
                            "valor": valor[vivo]}, index=np.flatnonzero(vivo))
    claves = ["estacion", "parametro", "fecha_hora"]
    dup = validos.duplicated(claves, keep="first").to_numpy()
    primero = validos.groupby(claves, sort=False)["valor"].transform("first").to_numpy()
    exacto = dup & (validos["valor"].to_numpy() == primero)
    distinto = dup & ~exacto
    desc["duplicado_exacto"] = int(exacto.sum())
    desc["duplicado_valor_distinto"] = int(distinto.sum())
    marcadas = int(validos.loc[distinto, claves].drop_duplicates().shape[0]) if distinto.any() else 0
    conservados = validos[~dup].copy()

    leidas = int(info["leidas"])
    procesadas = int(len(conservados))
    descartadas = int(sum(desc.values()))
    if leidas != procesadas + descartadas:
        raise AssertionError(f"leídas {leidas} != procesadas {procesadas} + descartadas {descartadas}")

    # Estaciones: leídas (filas que llegan a la cascada), procesadas, ubicación de su primera fila.
    estaciones = {}
    reg_est = pd.Series(est[registrada])
    for e, c in reg_est.value_counts().items():
        estaciones[e] = {"largas_leidas": int(c), "largas_procesadas": 0, "anchas": 0, "completas_gold": 0}
    for e, c in conservados["estacion"].value_counts().items():
        estaciones[e]["largas_procesadas"] = int(c)
    primera = pd.Series(np.flatnonzero(registrada), index=reg_est.to_numpy()).groupby(level=0).min()
    for logica in COLUMNAS_UBICACION:
        if logica in largo:
            vals = _por_categoria(largo[logica], lambda c: [x.strip() for x in c], "")
            for e, fila in primera.items():
                if vals[fila]:
                    estaciones[e].setdefault("ubicacion", {})[logica] = vals[fila]

    conservados["estacion"] = conservados["estacion"].astype("category")
    conservados["parametro"] = conservados["parametro"].astype("category")
    reporte = {
        "cabecera": info["cabecera"], "bom": info["bom"], "delimitador": info["delimitador"],
        "filas_largas": {
            "leidas": leidas, "procesadas": procesadas, "descartadas": descartadas,
            "descartadas_por_regla": desc,
            "marcadas_por_regla": {"duplicado_valor_distinto": marcadas},
            "fuera_de_rango_por_parametro": {k: int(v) for k, v in fuera_param.items()},
        },
        "filas_anchas": {"total": None, "completas_gold": None, "pm2_5_mayor_pm10": None},
        "parametros_presentes": sorted(conservados["parametro"].astype(str).unique()),
        "estaciones": dict(sorted(estaciones.items())),
        "vistos": vistos,
        "avisos": info.get("avisos", []),
        "_python": {"ejemplos_por_regla": detalle},  # extra del prototipo; Go no lo escribe
    }
    return conservados.reset_index(names="fila"), reporte


def pivotar(validos: pd.DataFrame, reporte: dict | None = None) -> pd.DataFrame:
    """Regla 7: una fila ancha por (estación, fecha-hora); parámetro faltante -> NaN. Columnas en
    el orden de bytes de los canónicos; filas ordenadas por estación y fecha-hora. Si se pasa el
    reporte, completa filas_anchas y las fechas de cada estación."""
    v = validos.copy()
    v["estacion"] = v["estacion"].astype(str)
    v["parametro"] = v["parametro"].astype(str)
    ancho = v.pivot(index=["estacion", "fecha_hora"], columns="parametro", values="valor")
    ancho = ancho[sorted(ancho.columns)]
    ancho.columns.name = None
    ancho = ancho.reset_index().sort_values(["estacion", "fecha_hora"], kind="mergesort").reset_index(drop=True)
    if reporte is not None:
        fa = reporte["filas_anchas"]
        fa["total"] = int(len(ancho))
        fa["pm2_5_mayor_pm10"] = int((ancho["pm2_5"] > ancho["pm10"]).sum()) if {"pm2_5", "pm10"} <= set(ancho) else 0
        for e, g in ancho.groupby("estacion", sort=True):
            r = reporte["estaciones"].setdefault(e, {"largas_leidas": 0, "largas_procesadas": 0, "anchas": 0, "completas_gold": 0})
            r["anchas"] = int(len(g))
            r["desde"] = g["fecha_hora"].iloc[0].strftime("%Y-%m-%dT%H:%M:%S") + DESFASE_LIMA
            r["hasta"] = g["fecha_hora"].iloc[-1].strftime("%Y-%m-%dT%H:%M:%S") + DESFASE_LIMA
    return ancho


def completar_reporte(reporte: dict, gold: pd.DataFrame, receta: dict) -> dict:
    """completas_gold (total y por estación), features y nivel, como armarReporte de Go."""
    reporte["filas_anchas"]["completas_gold"] = int(len(gold))
    for e, c in gold["estacion"].value_counts().items():
        reporte["estaciones"][e]["completas_gold"] = int(c)
    reporte["features"] = list(receta["features"])
    reporte["nivel"] = receta.get("nivel", "absoluto")
    return reporte


def completas(ancho: pd.DataFrame, features, log1p=()) -> pd.Series:
    """Regla 8: la fila entra al modelo si todas sus features (ya con log1p) son finitas."""
    X = matriz_features(ancho, features, log1p)
    return pd.Series(np.isfinite(X.to_numpy(dtype=float)).all(axis=1), index=ancho.index)


# ---------------------------------------------------------------------------------------------
# Contrato silver y gold (mismo formato que Go: datos/salida.go)
# ---------------------------------------------------------------------------------------------

def formato_g(x: float) -> str:
    """= strconv.FormatFloat(x, 'g', -1, 64): los dígitos más cortos que vuelven al mismo float64,
    en notación científica si el exponente decimal es < -4 o >= 6 (con la precisión «más corta»
    Go decide con 6). Ej.: 873 -> «873», 1e6 -> «1e+06», 1.5e-05 -> «1.5e-05»."""
    if math.isnan(x):
        return "NaN"
    if math.isinf(x):
        return "+Inf" if x > 0 else "-Inf"
    if x == 0:
        return "-0" if math.copysign(1.0, x) < 0 else "0"
    r = np.format_float_scientific(abs(x), unique=True, trim="-")
    mant, e = r.split("e")
    digitos = mant.replace(".", "")
    exp = int(e)
    signo = "-" if x < 0 else ""
    if exp < -4 or exp >= 6:
        cuerpo = digitos[0] + ("." + digitos[1:] if len(digitos) > 1 else "")
        return f"{signo}{cuerpo}e{'-' if exp < 0 else '+'}{abs(exp):02d}"
    dp = exp + 1  # posición del punto decimal en los dígitos
    if dp <= 0:
        return f"{signo}0." + "0" * (-dp) + digitos
    if dp >= len(digitos):
        return signo + digitos + "0" * (dp - len(digitos))
    return f"{signo}{digitos[:dp]}.{digitos[dp:]}"


def formato_fecha_hora(serie: pd.Series) -> pd.Series:
    return serie.dt.strftime("%Y-%m-%dT%H:%M:%S") + DESFASE_LIMA


def _campo_csv(s: str) -> str:
    """= datos.appendCampo: entrecomilla solo si hace falta (RFC 4180)."""
    if any(c in s for c in ',"\r\n'):
        return '"' + s.replace('"', '""') + '"'
    return s


def escribir_csv_contrato(df: pd.DataFrame, ruta) -> None:
    """estacion,fecha_hora,<columnas>; coma; vacío = faltante; fecha RFC3339 con -05:00; valores
    con formato_g (= FormatFloat 'g' -1 de Go): sale byte a byte igual al silver de Go."""
    Path(ruta).parent.mkdir(parents=True, exist_ok=True)
    fechas = formato_fecha_hora(df["fecha_hora"]).to_numpy()
    estaciones = [_campo_csv(str(e)) for e in df["estacion"].to_numpy()]
    cols = [c for c in df.columns if c not in ("estacion", "fecha_hora")]
    M = df[cols].to_numpy(dtype=float)
    with open(ruta, "w", encoding="utf-8", newline="") as fh:
        fh.write(",".join(["estacion", "fecha_hora"] + cols) + "\n")
        for e, t, fila in zip(estaciones, fechas, M):
            fh.write(e + "," + t + "".join("," + ("" if v != v else formato_g(v)) for v in fila) + "\n")


def leer_csv_contrato(ruta) -> pd.DataFrame:
    """Lee silver, gold o asignaciones: fecha_hora RFC3339 -> hora local de Lima sin zona."""
    df = pd.read_csv(ruta, dtype={"estacion": str}, keep_default_na=True)
    fh = pd.to_datetime(df["fecha_hora"], utc=True, format="ISO8601")
    df["fecha_hora"] = fh.dt.tz_convert("Etc/GMT+5").dt.tz_localize(None)  # GMT+5 = UTC-5
    return df


def fraccion_fina(ancho: pd.DataFrame) -> pd.Series:
    """PM2.5 / PM10 recortada a [0, 1]; faltante si falta alguna o si PM10 <= 0 (como Go)."""
    if "pm2_5" not in ancho or "pm10" not in ancho:
        return pd.Series(np.nan, index=ancho.index)
    pm10 = ancho["pm10"].where(ancho["pm10"] > 0)
    return (ancho["pm2_5"] / pm10).clip(0, 1)


def matriz_features(ancho: pd.DataFrame, features, log1p=()) -> pd.DataFrame:
    """Columnas de la receta en su orden, con log1p donde corresponda (sin estandarizar)."""
    out = pd.DataFrame(index=ancho.index)
    for f in features:
        out[f] = fraccion_fina(ancho) if f == "fraccion_fina" else (ancho[f] if f in ancho else np.nan)
    with np.errstate(invalid="ignore", divide="ignore"):
        for f in log1p:
            out[f] = np.log1p(out[f])
    return out


def construir_gold(ancho: pd.DataFrame, receta: dict) -> tuple[pd.DataFrame, dict]:
    """= datos.construirGold: features (fracción fina recortada, log1p), filas completas (regla 8:
    todo finito después del log1p), z-score con desviación poblacional (ddof = 0), global
    ('absoluto') o por estación; una feature con desviación 0 queda en 0. Las estadísticas tienen
    el esquema de reports/estandarizacion.json."""
    feats = list(receta["features"])
    logs = list(receta.get("log1p") or [])
    nivel = receta.get("nivel", "absoluto")
    if nivel not in ("absoluto", "estacion"):
        raise ValueError(f"nivel desconocido: {nivel}")
    X = matriz_features(ancho, feats, logs)
    m = np.isfinite(X.to_numpy(dtype=float)).all(axis=1)
    base = ancho.loc[m, ["estacion", "fecha_hora"]].reset_index(drop=True)
    Xc = X.loc[m].to_numpy(dtype=float)
    media, desv = Xc.mean(axis=0), Xc.std(axis=0, ddof=0)

    def z(x, mu, sd):
        with np.errstate(invalid="ignore", divide="ignore"):
            return np.where(sd > 0, (x - mu) / np.where(sd > 0, sd, 1.0), 0.0)

    stats = {"features": feats, "log1p": logs, "nivel": nivel, "n": int(len(Xc)),
             "media": media.tolist(), "desv": desv.tolist(), "ddof": 0, "chunk": int(receta.get("chunk") or 10000)}
    if nivel == "absoluto":
        Z = z(Xc, media, desv)
    else:
        Z = np.empty_like(Xc)
        por = {}
        for est, idx in base.groupby("estacion", sort=True).groups.items():
            idx = np.asarray(idx)
            mu, sd = Xc[idx].mean(axis=0), Xc[idx].std(axis=0, ddof=0)
            Z[idx] = z(Xc[idx], mu, sd)
            por[est] = {"n": int(len(idx)), "media": mu.tolist(), "desv": sd.tolist()}
        stats["por_estacion"] = por
    gold = pd.concat([base, pd.DataFrame(Z, columns=feats)], axis=1)
    return gold, stats


def _media_desv(stats: dict, estacion=None):
    if stats.get("nivel", "absoluto") == "absoluto" or estacion is None:
        return np.asarray(stats["media"], float), np.asarray(stats["desv"], float)
    s = stats["por_estacion"][estacion]
    return np.asarray(s["media"], float), np.asarray(s["desv"], float)


def desestandarizar(centroides_z: np.ndarray, stats: dict) -> np.ndarray | None:
    """Centroides a unidades originales (solo nivel absoluto): z*desv + media y expm1 en log1p."""
    if stats.get("nivel", "absoluto") != "absoluto":
        return None
    mu, sd = _media_desv(stats)
    C = np.asarray(centroides_z, float) * sd + mu
    for j, f in enumerate(stats["features"]):
        if f in (stats.get("log1p") or []):
            C[:, j] = np.expm1(C[:, j])
    return C


def invertir_gold(gold: pd.DataFrame, stats: dict) -> pd.DataFrame:
    """= kmeans.Estandarizacion.Invertir fila por fila: el gold en unidades originales."""
    feats = stats["features"]
    Z = gold[feats].to_numpy(dtype=float)
    X = np.empty_like(Z)
    if stats.get("nivel", "absoluto") == "absoluto":
        mu, sd = _media_desv(stats)
        X[:] = Z * sd + mu
    else:
        for est, idx in gold.groupby("estacion", sort=False).groups.items():
            mu, sd = _media_desv(stats, est)
            idx = np.asarray(idx)
            X[idx] = Z[idx] * sd + mu
    for j, f in enumerate(feats):
        if f in (stats.get("log1p") or []):
            X[:, j] = np.expm1(X[:, j])
    out = gold[["estacion", "fecha_hora"]].copy()
    for j, f in enumerate(feats):
        out[f] = X[:, j]
    return out


def etiqueta_eca(pm25, pm10) -> tuple[str, list]:
    """= kmeans.EtiquetaECA: excede si PM2.5 medio > 50 o PM10 medio > 100; no_excede si están
    los dos y ninguno supera; no_evaluable si falta el que podría superar."""
    por = []
    if pm25 is not None and pm25 > ECA["pm2_5"]:
        por.append("pm2_5")
    if pm10 is not None and pm10 > ECA["pm10"]:
        por.append("pm10")
    if por:
        return "excede", por
    if pm25 is not None and pm10 is not None:
        return "no_excede", []
    return "no_evaluable", []


def perfilar(gold: pd.DataFrame, regimen: np.ndarray, k: int, stats: dict) -> list[dict]:
    """= kmeans.Perfilar: por régimen, tamaño, fracción, perfil (media ARITMÉTICA de cada feature
    en unidades originales, invirtiendo cada fila), PM2.5 medio (la feature, o fraccion_fina ×
    pm10 fila por fila), PM10 medio, etiqueta ECA, distribución por hora y por estación."""
    X = invertir_gold(gold, stats)
    feats = stats["features"]
    regimen = np.asarray(regimen)
    if "pm2_5" in feats:
        pm25, origen = X["pm2_5"].to_numpy(), "pm2_5"
    elif "fraccion_fina" in feats and "pm10" in feats:
        pm25, origen = X["fraccion_fina"].to_numpy() * X["pm10"].to_numpy(), "fraccion_fina*pm10"
    else:
        pm25, origen = None, ""
    horas = gold["fecha_hora"].dt.hour.to_numpy()
    regs = []
    for j in range(k):
        m = regimen == j
        r = {"regimen": j, "tamano": int(m.sum()), "fraccion": float(m.sum() / len(regimen)) if len(regimen) else 0.0}
        if not m.any():
            r.update({"perfil": None, "pm2_5_medio": None, "pm10_medio": None, "eca": "no_evaluable",
                      "por_hora": [0] * 24, "por_estacion": {}})
            regs.append(r)
            continue
        r["perfil"] = {f: float(X.loc[m, f].mean()) for f in feats}
        r["pm2_5_medio"] = float(pm25[m].mean()) if pm25 is not None else None
        if origen:
            r["pm2_5_origen"] = origen
        r["pm10_medio"] = r["perfil"]["pm10"] if "pm10" in feats else None
        r["eca"], por = etiqueta_eca(r["pm2_5_medio"], r["pm10_medio"])
        if por:
            r["eca_por"] = por
        r["por_hora"] = np.bincount(horas[m], minlength=24).astype(int).tolist()
        r["por_estacion"] = {str(e): int(c) for e, c in gold.loc[m, "estacion"].value_counts().sort_index().items()}
        regs.append(r)
    return regs


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


# ---------------------------------------------------------------------------------------------
# Origen de los datos y de las figuras
# ---------------------------------------------------------------------------------------------

PREFIJOS_SINTETICOS = ("SINT-", "SIN-")  # data/sample (Go) y notebooks/sintetico.py


def es_sintetico(raiz=None, bronze=None, estaciones=()) -> bool:
    """True si los datos son sintéticos: SINTETICO.txt en la raíz, un bronze dentro de
    data/sample/, o todas las estaciones con prefijo SINT- o SIN-."""
    if raiz is not None and (Path(raiz) / "SINTETICO.txt").exists():
        return True
    if bronze is not None and Path(bronze).parent.name == "sample" and Path(bronze).parent.parent.name == "data":
        return True
    est = [str(e) for e in estaciones]
    return bool(est) and all(e.startswith(PREFIJOS_SINTETICOS) for e in est)


def registrar_figura(carpeta, nombre: str, sintetico: bool, fuente: str, notebook: str) -> None:
    """Anota en <carpeta>/origen.json de dónde salió cada figura. Así el empaquetado del informe
    puede negarse a entregar una figura hecha con datos sintéticos."""
    ruta = Path(carpeta) / "origen.json"
    try:
        datos = json.loads(ruta.read_text(encoding="utf-8")) if ruta.exists() else {}
    except (OSError, json.JSONDecodeError):
        datos = {}
    datos[nombre] = {"sintetico": bool(sintetico), "fuente": str(fuente), "notebook": notebook,
                     "fecha": dt.datetime.now().isoformat(timespec="seconds")}
    ruta.write_text(json.dumps(dict(sorted(datos.items())), indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
