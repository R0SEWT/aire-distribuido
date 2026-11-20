# /// script
# requires-python = ">=3.10"
# dependencies = ["pandas>=2"]
# ///
"""Convierte los JSON de generado/ en tablas Markdown y cifras para el informe.

    python3 tablas.py                 # lee generado/*.json, escribe generado/tablas/*.md y generado/cifras.yml
    python3 tablas.py --estricto      # además falla si falta un JSON o no cuadra un invariante
    python3 tablas.py --final         # --estricto y, además, falla si queda un JSON PLACEHOLDER, una
                                      # entrada de data/sample/, una estación SINT-/SIN- o una figura
                                      # sintética (generado/figuras/origen.json)
    uv run tablas.py                  # si no hay pandas en el Python del sistema

Ninguna cifra del informe se escribe a mano: los comandos de Go escriben los JSON, este script los
vuelve tablas (shortcode include en el .qmd) y cifras para la prosa (shortcode meta, desde
generado/cifras.yml). Los esquemas son los de Go, que son la fuente de verdad:

    limpieza.json            cmd/cargar             datos/reporte.go       (ReporteLimpieza)
    estandarizacion.json     cmd/cargar             datos/estandarizar.go  (Estandarizacion)
    benchmark_cargador.json  cmd/cargar -bench      datos/medicion.go      (Benchmark)
    kmeans.json              cmd/entrenar           cmd/entrenar/main.go
    regimenes.json           cmd/entrenar           kmeans/regimenes.go    (Regimen)
    codo.json                cmd/entrenar -codo     kmeans/codo.go
    benchmark_kmeans.json    cmd/benchmark          cmd/benchmark/main.go
    eda.json                 (transcrito)           lo que solo mide el EDA, de reports/eda_criba.md
    auditoria.json           notebook 03            Go contra pandas y scikit-learn
    ../../reports/calibracion_resumen.csv y kmeans_propuesto.json   notebook 02 (calibración)

Si un JSON trae `"placeholder": true` (lo agrega el equipo, no Go), su tabla sale con un aviso
PLACEHOLDER y sus cifras con la marca [PLACEHOLDER], para que nadie las confunda con resultados.

Python aquí es herramienta de reporte, no parte del sistema: el sistema es íntegramente Go.
"""

from __future__ import annotations

import argparse
import json
import math
from datetime import datetime, timedelta
import sys
from pathlib import Path

try:
    import pandas as pd
except ImportError:  # el python3 del sistema puede no tenerlo
    sys.exit("tablas.py necesita pandas: `uv run tablas.py` lo instala al vuelo (o pip install pandas)")

AQUI = Path(__file__).resolve().parent
# Raíz del repo: configs/ y el código Go se leen de aquí, no del padre de --reports (que puede ser otra carpeta).
RAIZ = AQUI.parent.parent
MARCA = "[PLACEHOLDER]"

COMANDOS = {
    "limpieza.json": "cmd/cargar",
    "estandarizacion.json": "cmd/cargar",
    "benchmark_cargador.json": "cmd/cargar -bench",
    "kmeans.json": "cmd/entrenar",
    "regimenes.json": "cmd/entrenar",
    "codo.json": "cmd/entrenar -codo",
    "benchmark_kmeans.json": "cmd/benchmark",
    # PC4: se leen de reports/ y del repositorio, sin copia en generado/.
    "reports/smoke_lab_*.json": "cmd/smoke",
    "reports/kmeans_distribuido_*.json": "cmd/distribuido",
    "reports/kmeans_gpu/*.json": "lab/gpu/kmeans_gpu.py",
    "reports/mapa/lab_d56_*.json": "cmd/mapa-coord (PR #52)",
    "reports/alerta_24h.json": "go test ./ventana -run LaVentanaSobreElGold -actualizar",
    "reports/api_latencia.json": "cmd/carga",
    "docs/postman/lecturas_prueba.csv": "docs/postman/generar_lecturas.py",
}

# Reglas de descarte en el orden de la cascada de Go (datos/doc.go, «Reglas»): cada fila descartada
# se atribuye a la primera que falla. Clave del JSON → (regla de la spec §4.2, descripción).
REGLAS = {
    "columnas": ("—", "La fila no tiene tantos campos como la cabecera"),
    "estacion": ("—", "Estación vacía"),
    "periodo": ("1", "Periodo no horario (24 horas, medias móviles); «-» sin hora y con el día incompleto"),
    "sin_valor": ("—", "Ninguna columna de parámetro con valor: el hueco de la plantilla de 2022"),
    "parametro": ("—", "Más de una columna de parámetro con valor"),
    "fecha": ("2", "Fecha no parseable"),
    "fecha_fuera_de_rango": ("2", "Fecha fuera del rango configurado"),
    "hora_no_en_punto": ("2", "Hora con minutos o segundos"),
    "hora": ("2′", "Hora ilegible, o sin hora y con el día incompleto (no se puede inferir por posición)"),
    "valor_no_numerico": ("3", "Texto (<LD, ND, ---), NaN o infinito; no se imputa"),
    "centinela": ("3", "Código del equipo (−999, 9999…), sobre el valor crudo"),
    "unidad": ("6", "Unidad que no figura para ese parámetro"),
    "negativo": ("3", "Negativo en un parámetro que no los admite"),
    "fuera_de_rango": ("4", "Fuera del rango físico, ya en la unidad canónica"),
    "bloque_en_cero": ("3′", "Un día entero en 0 exacto: equipo sin medir, no aire limpio"),
    "cero_como_faltante": ("3′", "Un 0 exacto de un contaminante en un día con datos: hora sin dato"),
    "duplicado_unidad": ("5/6", "La misma lectura en otra unidad (ppb, mmHg): se conserva la canónica"),
    "duplicado_exacto": ("5", "Misma (estación, parámetro, fecha-hora) y mismo valor"),
    "duplicado_valor_distinto": ("5", "Misma clave con otro valor: se conserva el primero"),
}
# Marcas: se conservan y se cuentan (marcadas_por_regla).
MARCAS = {
    "hora_inferida": "Hora reconstruida por la posición de la fila en su día",
    "unidad_convertida": "Convertida a la unidad canónica (mmHg → hPa, ppb → µg/m³)",
    "duplicado_valor_distinto": "Celda cuyo valor conservado tuvo un duplicado con otro valor",
    "hora_no_en_punto": "Hora truncada a la hora en punto",
    "ubicacion_corregida": "Fila leída de una estación con la ubicación corregida (sobre las leídas)",
}
# Fases de la carga (datos/cargador.go): cuál es paralela y cuál serial, para leer Amdahl.
FASES = [
    ("lectura_parseo", "Lectura, parseo y reglas por fila", "paralela (W workers); incluye la fusión"),
    ("fusion", "Fusión de parciales en orden de bloque", "serial, solapada con la lectura"),
    ("horas", "Hora por posición y reglas por día", "serial"),
    ("pivot", "Pivot a la tabla ancha", "paralela por estación"),
    ("tabla", "Orden de la tabla ancha", "serial"),
    ("gold_estandarizacion", "Gold: completitud y z-score", "paralela por chunks"),
]

# Nombres para mostrar (unidad canónica de configs/limpieza.json). Lo que no esté sale con su clave.
NOMBRES = {
    "pm2_5": "PM2,5 (µg/m³)",
    "pm10": "PM10 (µg/m³)",
    "fraccion_fina": "Fracción fina PM2,5/PM10",
    "viento_velocidad": "Velocidad del viento (m/s)",
    "h2s": "H2S (µg/m³)",
    "viento_direccion": "Dirección del viento (°)",
    "humedad": "Humedad relativa (%)",
    "temperatura": "Temperatura (°C)",
    "presion": "Presión (hPa)",
    "precipitacion": "Precipitación (mm)",
    "radiacion": "Radiación solar (W/m²)",
    "no2": "NO2 (µg/m³)",
    "so2": "SO2 (µg/m³)",
    "co": "CO (µg/m³)",
    "o3": "O3 (µg/m³)",
}
# Para tablas anchas (perfiles y centroides): la unidad va en la nota de la tabla.
CORTOS = {
    "pm2_5": "PM2,5", "pm10": "PM10", "fraccion_fina": "Fracción fina", "viento_velocidad": "Viento",
    "viento_direccion": "Dir. viento", "humedad": "Humedad", "temperatura": "Temp.",
    "presion": "Presión", "precipitacion": "Precip.", "radiacion": "Radiación",
    "no2": "NO2", "so2": "SO2", "co": "CO", "o3": "O3", "h2s": "H2S",
}
ECA = {"excede": "excede", "no_excede": "no excede", "no_evaluable": "no evaluable"}
# Unidades tal como las normaliza el cargador (minúsculas, sin tildes ni superíndices) → para mostrar.
UNIDADES = {"ug/m3": "µg/m³", "mg/m3": "mg/m³", "c": "°C", "°c": "°C", "hpa": "hPa", "mb": "mb", "mbar": "mbar",
            "mmhg": "mmHg", "w/m2": "W/m²", "grados": "°", "°": "°", "m/s": "m/s", "%": "%", "mm": "mm", "ppb": "ppb"}
FRANJAS = [("Madrugada (0–5 h)", range(0, 6)), ("Mañana (6–11 h)", range(6, 12)),
           ("Tarde (12–17 h)", range(12, 18)), ("Noche (18–23 h)", range(18, 24))]


# --------------------------------------------------------------------------- formato (es-PE)
def vacio(x) -> bool:
    return x is None or (isinstance(x, float) and math.isnan(x))


def entero(x) -> str:
    """3512448 → «3 512 448» con espacio duro, como pide la RAE para los millares."""
    return "—" if vacio(x) else f"{int(round(x)):,}".replace(",", "\u00a0")


def decimal(x, d=2) -> str:
    if vacio(x):
        return "—"
    return f"{x:,.{d}f}".replace(",", "\u00a0").replace(".", ",")


def pct(x, d=1) -> str:
    return "—" if vacio(x) else f"{decimal(100 * x, d)}\u00a0%"


def tiempo(x) -> str:
    """Tres cifras significativas sin notación científica: 182,0 · 2,45 · 0,00764."""
    if vacio(x):
        return "—"
    if x == 0:
        return "0"
    d = max(1, 2 - math.floor(math.log10(abs(x))))
    return decimal(x, d)


SUPERINDICES = str.maketrans("-0123456789", "⁻⁰¹²³⁴⁵⁶⁷⁸⁹")


def cientifica(x, d=1) -> str:
    """1.33e-15 → «1,3 × 10⁻¹⁵»; con d = 0, «10⁻⁹» si la mantisa es 1. El cero exacto es «0»."""
    if vacio(x):
        return "—"
    x = float(x)
    if x == 0:
        return "0"
    e = math.floor(math.log10(abs(x)))
    m = x / 10 ** e
    if round(m, d) >= 10:
        m, e = m / 10, e + 1
    potencia = "10" + str(e).translate(SUPERINDICES)
    return potencia if d == 0 and round(m) == 1 else f"{decimal(m, d)} × {potencia}"


def karp_flatt(speedup, p) -> float | None:
    """Fracción serial experimental de Karp y Flatt: e = (1/S − 1/p) / (1 − 1/p)."""
    if not speedup or p is None or p <= 1:
        return None
    return (1 / speedup - 1 / p) / (1 - 1 / p)


def techo_amdahl(e) -> str:
    """Amdahl con la fracción serial e: el speedup no pasa de 1/e por más workers que se agreguen."""
    return "—" if vacio(e) or not e or e <= 0 else decimal(1 / e, 1)


def con_signo(x, d=1) -> str:
    """+2,1 · −5,4 (signo menos tipográfico)."""
    return "—" if vacio(x) else ("+" if x >= 0 else "\u2212") + decimal(abs(x), d)


def lista(xs, fmt=entero) -> str:
    xs = sorted({x for x in xs if not vacio(x)})
    textos = [fmt(x) for x in xs] or ["—"]
    return textos[0] if len(textos) == 1 else ", ".join(textos[:-1]) + " y " + textos[-1]


def nombre(clave: str) -> str:
    return NOMBRES.get(clave, clave)


def corto(clave: str) -> str:
    return CORTOS.get(clave, clave)


def unidades(claves) -> str:
    """«PM10 en µg/m³; Viento en m/s…» para la nota de una tabla con nombres cortos."""
    partes = []
    for c in claves:
        largo = NOMBRES.get(c, "")
        if "(" in largo:
            partes.append(f"{corto(c)} en {largo[largo.index('(') + 1:largo.rindex(')')]}")
    return ("Unidades: " + "; ".join(partes) + ".") if partes else ""


def fecha(iso: str | None) -> str:
    return iso[:10] if iso else "—"


def titulo_lugar(s: str | None) -> str:
    return s.title() if s else "(sin dato)"


def celda(v) -> str:
    return str(v).replace("|", "\\|").replace("\n", " ")


# Lo que Pandoc o Quarto interpretarían en una celda: énfasis, código, enlaces, HTML, entidades, citas (@),
# matemáticas ($), atributos y shortcodes ({ }). La barra | la escapa celda().
ESPECIALES = set("\\`*_[]<>{}#$@~^&!")


def escapar(x) -> str:
    """Texto importado (nombres de OEFA, valores del CSV) → Markdown que se lee tal cual, sin formato."""
    if not isinstance(x, str):
        return "—" if vacio(x) else str(x)
    return "".join("\\" + c if c in ESPECIALES else c for c in x)


def markdown(df: pd.DataFrame, alineacion: str | None = None) -> str:
    """Tabla pipe de Pandoc. alineacion: una letra por columna, 'l', 'r' o 'c'."""
    cols = list(df.columns)
    alineacion = alineacion or "l" * len(cols)
    sep = {"l": ":---", "r": "---:", "c": ":---:"}
    filas = ["| " + " | ".join(celda(c) for c in cols) + " |",
             "|" + "|".join(sep[a] for a in alineacion) + "|"]
    for _, fila in df.iterrows():
        filas.append("| " + " | ".join(celda(v) for v in fila.tolist()) + " |")
    return "\n".join(filas)


# --------------------------------------------------------------------------- salida
LISTAS = ("placeholders", "sinteticos", "desactualizadas", "faltantes", "problemas")


class Informe:
    def __init__(self, entrada: Path, salida: Path, reports: Path):
        self.entrada, self.salida, self.reports = entrada, salida, reports
        self.datos: dict[str, dict] = {}
        self.cifras: dict[str, dict] = {}
        # Lo que falta o no cuadra se anota por entregable: lo de la PC4 (ámbito "pc4") no cambia el
        # estado ni el código de salida del informe de la PC3. main() cambia el ámbito antes de los
        # métodos de la PC4; las cinco listas de abajo son las del ámbito en curso.
        self.ambito = "pc3"
        self._listas = {a: {n: [] for n in LISTAS} for a in ("pc3", "pc4")}

    placeholders = property(lambda self: self._listas[self.ambito]["placeholders"])
    sinteticos = property(lambda self: self._listas[self.ambito]["sinteticos"])
    desactualizadas = property(lambda self: self._listas[self.ambito]["desactualizadas"])
    faltantes = property(lambda self: self._listas[self.ambito]["faltantes"])
    problemas = property(lambda self: self._listas[self.ambito]["problemas"])

    def leer(self, nombre_json: str) -> dict | None:
        ruta = self.entrada / nombre_json
        if not ruta.exists():
            self.faltantes.append(nombre_json)
            return None
        d = json.loads(ruta.read_text(encoding="utf-8"))
        if d.get("placeholder"):
            self.placeholders.append(nombre_json)
        self.datos[nombre_json] = d
        return d

    def cifra(self, d: dict, valor) -> str:
        valor = str(valor)
        return f"{valor} {MARCA}" if d.get("placeholder") else valor

    def guardar_cifras(self, grupo: str, d: dict, valores: dict) -> None:
        self.cifras[grupo] = {k: self.cifra(d, v) for k, v in valores.items()}

    def sintetico(self, origen: str, texto: str) -> None:
        """Señal de datos sintéticos, que Go no marca: la entrega final no puede llevarlos."""
        self.sinteticos.append(f"{origen}: {texto}")

    def revisar_estaciones(self, origen: str, nombres) -> None:
        sint = sorted(e for e in nombres if str(e).upper().startswith(("SINT-", "SIN-")))
        if sint:
            self.sintetico(origen, "estaciones sintéticas " + ", ".join(sint))

    def revisar_entrada(self, origen: str, ruta) -> None:
        if ruta and "data/sample/" in str(ruta):
            self.sintetico(origen, f"entrada {ruta}")

    def figuras(self) -> None:
        """generado/figuras/origen.json: el notebook marca cada figura con sintetico: true/false."""
        ruta = AQUI / "generado" / "figuras" / "origen.json"
        if not ruta.exists():
            return
        origen = json.loads(ruta.read_text(encoding="utf-8"))
        sint = [f for f, meta in sorted(origen.items()) if isinstance(meta, dict) and meta.get("sintetico")]
        if sint:
            self.sintetico("figuras/origen.json", "figuras sintéticas " + ", ".join(sint))
        # Una figura que sale del modelo y es anterior a él muestra otro modelo: la entrega no la lleva.
        momento = ((self.datos.get("kmeans.json") or {}).get("metadatos") or {}).get("momento")
        if momento:
            modelo = datetime.fromisoformat(momento).replace(tzinfo=None)
            claves = ("kmeans", "regimenes", "codo", "asignaciones", "benchmark")
            viejas = [f for f, meta in sorted(origen.items())
                      if isinstance(meta, dict) and meta.get("fecha")
                      and any(c in str(meta.get("fuente", "")) for c in claves)
                      and datetime.fromisoformat(meta["fecha"]).replace(tzinfo=None) < modelo]
            if viejas:
                self.desactualizadas.append("figuras anteriores al modelo de kmeans.json: " + ", ".join(viejas))

    def problema(self, origen: str, texto: str) -> None:
        self.problemas.append(f"{origen}: {texto}")
        print(f"aviso: {origen}: {texto}", file=sys.stderr)

    def comprobar(self, origen: str, condicion: bool, texto: str) -> None:
        if not condicion:
            self.problema(origen, texto)

    @staticmethod
    def procedencia(d: dict) -> str:
        """Commit, fecha y máquina, si el JSON los registra (metadatos de kmeans o campos sueltos)."""
        m = d.get("metadatos") or {}
        commit = m.get("commit")
        momento = m.get("momento") or d.get("fecha")
        maquina = m.get("maquina") or d.get("host")
        partes = []
        if commit:
            partes.append(f"commit `{commit[:7]}`")
        if momento:
            partes.append(fecha(momento))
        if maquina:
            partes.append("laptop de desarrollo")
        return " (" + ", ".join(partes) + ")" if partes else " (el JSON no registra commit ni fecha)"

    def tabla(self, archivo: str, origen: str, d: dict | None, ident: str, titulo: str,
              cuerpo: str = "", nota: str = "", anchos: list[int] | None = None, fuente: str | None = None) -> None:
        """Escribe generado/tablas/<archivo>: aviso si es PLACEHOLDER, tabla con id y título, nota.

        anchos: porcentaje de cada columna (tbl-colwidths); sin él, Pandoc reparte por igual.
        """
        attr = f' tbl-colwidths="[{",".join(str(a) for a in anchos)}]"' if anchos else ""
        # Las notas que Go escribe en los JSON nombran campos; en el informe van en prosa.
        for campo, prosa in (("ms_kmeanspp y ms_lloyd", "Los tiempos de k-means++ y de Lloyd"), ("ms_kmeanspp es 0", "El de k-means++ es 0"),
                             ("ms_kmeanspp", "el tiempo de k-means++"), ("ms_lloyd", "el tiempo de Lloyd")):
            nota = nota.replace(campo, prosa)
        ruta_origen = origen if "/" in origen else f"generado/{origen}"
        partes = [f"<!-- Generado por tablas.py desde {ruta_origen}. No editar a mano. -->", ""]
        if d is None:
            partes += ["::: {.callout-warning title=\"PENDIENTE\"}",
                       f"Falta `{ruta_origen}`: esta tabla sale de ahí. Lo escribe "
                       f"`{COMANDOS.get(origen, '¿?')}` (ver `docs/informe/README.md`).", ":::", ""]
            partes += [f"::: {{#{ident}}}", "| (sin datos) |", "|:---|", "| — |", "", titulo, ":::", ""]
        else:
            if d.get("placeholder"):
                motivo = d.get("_nota") or "todavía no es el resultado sobre el dataset de OEFA."
                partes += ["::: {.callout-warning title=\"PLACEHOLDER\"}",
                           f"Cifras de prueba: `generado/{origen}` {motivo} Se reemplaza cuando "
                           f"`{COMANDOS.get(origen, '¿?')}` corra sobre el CSV completo.", ":::", ""]
            partes += [f"::: {{#{ident}{attr}}}", cuerpo, "", titulo, ":::", ""]
            fuente = "*Nota.* " + (fuente or (f"Fuente: `generado/{origen}`, escrito por "
                                              f"`{COMANDOS.get(origen, '¿?')}`{self.procedencia(d)}."))
            partes += [f"{fuente} {nota}".strip(), ""]
        (self.salida / archivo).write_text("\n".join(partes), encoding="utf-8")

    # ----------------------------------------------------------------------- limpieza
    def limpieza(self) -> None:
        o = "limpieza.json"
        d = self.leer(o)
        tablas = (("limpieza_resumen.md", "tbl-limpieza-resumen", "Conservación de filas en la carga"),
                  ("limpieza_reglas.md", "tbl-limpieza-reglas", "Filas largas descartadas por regla, en el orden de la cascada"),
                  ("limpieza_estaciones.md", "tbl-limpieza-estaciones", "Filas por estación: largas leídas y procesadas, anchas, completas para el modelo y rango de fechas"),
                  ("limpieza_parametros.md", "tbl-limpieza-parametros", "Parámetros vistos en el archivo y si tienen mapeo a un parámetro canónico"),
                  ("limpieza_unidades.md", "tbl-limpieza-unidades", "Unidades vistas por parámetro canónico"),
                  ("cobertura.md", "tbl-cobertura", "Cobertura de la red por departamento"),
                  ("limpieza_marcas.md", "tbl-limpieza-marcas", "Lo que se conserva pero se marca"),
                  ("limpieza_hora.md", "tbl-limpieza-hora", "La hora de cada lectura: explícita o reconstruida por posición"),
                  ("limpieza_ceros.md", "tbl-limpieza-ceros", "Ceros descartados por parámetro: días enteros y horas sueltas"),
                  ("limpieza_fases.md", "tbl-limpieza-fases", "Tiempo de cada fase de la carga"))
        if d is None:
            for a, i, t in tablas:
                self.tabla(a, o, None, i, t)
            return
        fl, fa = d["filas_largas"], d["filas_anchas"]
        self.revisar_entrada(o, d.get("entrada"))
        self.revisar_estaciones(o, d.get("estaciones", {}))
        desc = fl.get("descartadas_por_regla", {})
        marc = fl.get("marcadas_por_regla", {})
        est = d.get("estaciones", {})
        vistos = d.get("vistos", {})

        # Invariantes (datos/reporte.go): leídas = procesadas + descartadas = procesadas + Σ por regla.
        self.comprobar(o, fl["leidas"] == fl["procesadas"] + fl["descartadas"],
                       "leídas ≠ procesadas + descartadas")
        self.comprobar(o, sum(desc.values()) == fl["descartadas"],
                       f"la suma por regla ({sum(desc.values())}) ≠ descartadas ({fl['descartadas']})")
        # Por estación: las filas sin estación (reglas columnas y estacion) no se atribuyen a ninguna.
        sin_estacion = desc.get("columnas", 0) + desc.get("estacion", 0)
        self.comprobar(o, sum(e["largas_leidas"] for e in est.values()) + sin_estacion == fl["leidas"],
                       "Σ leídas por estación + descartes sin estación ≠ leídas")
        self.comprobar(o, sum(e["largas_procesadas"] for e in est.values()) == fl["procesadas"],
                       "Σ procesadas por estación ≠ procesadas")
        self.comprobar(o, sum(e["anchas"] for e in est.values()) == fa["total"],
                       "Σ anchas por estación ≠ filas anchas")
        self.comprobar(o, sum(e["completas_gold"] for e in est.values()) == fa["completas_gold"],
                       "Σ completas por estación ≠ completas del gold")
        for clave in desc:
            self.comprobar(o, clave in REGLAS, f"regla desconocida «{clave}»: agregarla a REGLAS en tablas.py")
        if "hora_inferida" in fl and "hora_explicita" in fl:
            self.comprobar(o, fl["hora_inferida"] + fl["hora_explicita"] == fl["procesadas"],
                           "hora inferida + hora explícita ≠ procesadas")
        if fl.get("bloque_en_cero_por_parametro"):
            self.comprobar(o, sum(fl["bloque_en_cero_por_parametro"].values()) == desc.get("bloque_en_cero", 0),
                           "Σ bloque_en_cero por parámetro ≠ descartadas por bloque_en_cero")

        leidas = fl["leidas"]
        r = pd.DataFrame([
            ("Filas largas leídas", entero(leidas), "100\u00a0%"),
            ("Filas largas procesadas (llegan al silver)", entero(fl["procesadas"]), pct(fl["procesadas"] / leidas)),
            ("Filas largas descartadas", entero(fl["descartadas"]), pct(fl["descartadas"] / leidas)),
            ("Filas anchas (estación × fecha-hora), regla 7", entero(fa["total"]), "100\u00a0%"),
            ("Filas anchas completas para el modelo (gold), regla 8", entero(fa["completas_gold"]),
             pct(fa["completas_gold"] / fa["total"]) if fa["total"] else "—"),
            ("Filas anchas con PM2,5 > PM10 (se marcan, no se descartan)", entero(fa.get("pm2_5_mayor_pm10")),
             pct(fa.get("pm2_5_mayor_pm10", 0) / fa["total"], 2) if fa["total"] else "—"),
        ], columns=["Concepto", "Filas", "Proporción"])
        self.tabla(tablas[0][0], o, d, tablas[0][1], tablas[0][2], markdown(r, "lrr"), anchos=[62, 20, 18], nota=
                   f"Leídas = procesadas + descartadas, en filas largas. Archivo `{Path(str(d.get('entrada', '¿?'))).name}` "
                   f"({entero(d.get('bytes'))} bytes, sha256 `{(d.get('sha256') or '¿?')[:12]}`, "
                   f"delimitador «{d.get('delimitador', '?')}», "
                   f"{'con' if d.get('bom') else 'sin'} BOM), {d.get('workers')} workers, "
                   f"{entero(d.get('bloques'))} bloques de {entero(d.get('bloque_bytes', 0) / 2**20)} MiB, "
                   f"{tiempo(d.get('duracion_s'))} s.")

        filas = []
        orden = list(REGLAS) + [k for k in desc if k not in REGLAS]
        for clave in orden:
            spec, texto = REGLAS.get(clave, ("?", "(sin descripción en tablas.py)"))
            filas.append((spec, f"`{clave}`", texto, entero(desc.get(clave, 0)),
                          pct(desc.get(clave, 0) / leidas, 2) if leidas else "—",
                          entero(marc.get(clave, 0)) if clave in marc else "—"))
        filas.append(("", "", "**Total**", f"**{entero(fl['descartadas'])}**",
                      f"**{pct(fl['descartadas'] / leidas, 2)}**" if leidas else "—", ""))
        t = pd.DataFrame(filas, columns=["Spec", "Regla", "Qué descarta", "Descartadas", "% de leídas", "Marcadas"])
        t = t.drop(columns="Marcadas")
        fdr = fl.get("fuera_de_rango_por_parametro", {})
        nota_fdr = ("Fuera de rango por parámetro: " + "; ".join(f"{corto(k)} {entero(v)}" for k, v in sorted(fdr.items()))
                    + ".") if fdr else ""
        self.tabla(tablas[1][0], o, d, tablas[1][1], tablas[1][2], markdown(t, "lllrr"),
                   anchos=[8, 28, 42, 12, 10], nota=
                   "Cada fila descartada se cuenta en la primera regla que falla, en el orden de la cascada de Go "
                   "(`datos/doc.go`), así que la suma no tiene solapes. «Spec» es el número de regla de la "
                   "@tbl-reglas; «—» son controles de estructura. Lo que se conserva y se marca está en la "
                   f"@tbl-limpieza-marcas. {nota_fdr}")

        e = pd.DataFrame([{
            "Estación": nom,
            "Departamento": titulo_lugar((x.get("ubicacion") or {}).get("departamento")),
            "Largas leídas": entero(x["largas_leidas"]),
            "Procesadas": entero(x["largas_procesadas"]),
            "Anchas": entero(x["anchas"]),
            "Gold": entero(x["completas_gold"]),
            "% gold": pct(x["completas_gold"] / x["anchas"]) if x["anchas"] else "—",
            "Desde": fecha(x.get("desde")),
            "Hasta": fecha(x.get("hasta")),
        } for nom, x in sorted(est.items())])
        self.tabla(tablas[2][0], o, d, tablas[2][1], tablas[2][2], markdown(e, "llrrrrrll"),
                   anchos=[17, 12, 10, 10, 8, 8, 9, 13, 13], nota=
                   "Gold: filas anchas con todas las features de la receta (regla 8). Desde y hasta: primera "
                   "y última hora con fila ancha, en hora de Lima y con la convención de hora final. Una "
                   "estación con lecturas y ninguna fila procesada aparece igual, con ceros."
                   + (" Con más de una ubicación en el archivo (se cargan como una sola estación): "
                      + ", ".join(f"{n} ({len(x['ubicaciones_distintas'])})" for n, x in sorted(est.items())
                                  if x.get("ubicaciones_distintas")) + "."
                      if any(x.get("ubicaciones_distintas") for x in est.values()) else "")
                   + ("".join(f" Ubicación corregida: {n} ("
                              + ", ".join(f"{c} {v} → {(x.get('ubicacion') or {}).get(c, '?')}"
                                          for c, v in sorted(x["ubicacion_original"].items())) + ")."
                              for n, x in sorted(est.items()) if x.get("ubicacion_original"))))

        par = vistos.get("parametros", {})
        nomap = vistos.get("parametros_no_mapeados", {})
        p = pd.DataFrame([(escapar(k), entero(v), "sin mapeo: se descarta (regla `parametro`)" if k in nomap else "mapeado")
                          for k, v in sorted(par.items(), key=lambda kv: -kv[1])],
                         columns=["Parámetro en el archivo", "Lecturas", "Estado"])
        periodos = vistos.get("periodos", {})
        self.tabla(tablas[3][0], o, d, tablas[3][1], tablas[3][2], markdown(p, "lrl"), anchos=[35, 15, 50], nota=
                   "Nombres tal como vienen en el CSV, con su frecuencia. Periodos vistos: "
                   + ("; ".join(f"{escapar(k)} {entero(v)}" for k, v in sorted(periodos.items(), key=lambda kv: -kv[1])) or "—")
                   + ". Parámetros canónicos presentes en la tabla ancha: "
                   + ", ".join(corto(x) for x in d.get("parametros_presentes", [])) + ".")

        u = pd.DataFrame([(corto(q), "; ".join(f"{escapar(un)} ({entero(n)})" for un, n in sorted(us.items(), key=lambda kv: -kv[1])))
                          for q, us in sorted(vistos.get("unidades", {}).items())],
                         columns=["Parámetro", "Unidades vistas (lecturas)"])
        self.tabla(tablas[4][0], o, d, tablas[4][1], tablas[4][2], markdown(u, "ll"), anchos=[25, 75], nota=
                   "Las unidades con factor en `configs/limpieza.json` se convierten a la canónica (regla 6); "
                   "las que no figuran se descartan (regla `unidad`).")

        deps: dict[str, list] = {}
        for x in est.values():
            dep = titulo_lugar((x.get("ubicacion") or {}).get("departamento"))
            deps.setdefault(dep, []).append(x)
        c = pd.DataFrame([(dep, entero(len(xs)), entero(sum(1 for x in xs if x["largas_procesadas"] > 0)),
                           entero(sum(1 for x in xs if x["completas_gold"] > 0)),
                           entero(sum(x["completas_gold"] for x in xs)))
                          for dep, xs in sorted(deps.items(), key=lambda kv: (-len(kv[1]), kv[0]))],
                         columns=["Departamento", "Estaciones", "Con datos", "En el gold", "Horas completas (gold)"])
        self.tabla(tablas[5][0], o, d, tablas[5][1], tablas[5][2], markdown(c, "lrrrr"), anchos=[34, 15, 15, 15, 21], nota=
                   "Un departamento que no aparece no tiene estación en el dataset: no hay dato, no aire limpio.")

        base_marca = {"ubicacion_corregida": leidas}
        filas_m = [(f"`{k}`", MARCAS.get(k, "(sin descripción en tablas.py)"), entero(v),
                    pct(v / base_marca.get(k, fl["procesadas"]), 1) if base_marca.get(k, fl["procesadas"]) else "—")
                   for k, v in sorted(marc.items(), key=lambda kv: -kv[1])]
        filas_m.append(("filas anchas con PM2,5 > PM10", "Se conservan; en el gold la fracción fina se recorta a 1",
                        entero(fa.get("pm2_5_mayor_pm10")), "—"))
        conv = fl.get("unidad_convertida_por_parametro") or {}
        nota_conv = ("Conversiones de unidad por parámetro: "
                     + "; ".join(f"{corto(k)} {entero(v)}" for k, v in sorted(conv.items(), key=lambda kv: -kv[1])) + ".") if conv else ""
        self.tabla(tablas[6][0], o, d, tablas[6][1], tablas[6][2],
                   markdown(pd.DataFrame(filas_m, columns=["Marca", "Qué es", "Lecturas", "% (de procesadas)"]), "llrr"),
                   anchos=[24, 46, 15, 15], nota=nota_conv)

        hi, he = fl.get("hora_inferida"), fl.get("hora_explicita")
        if hi is not None and he is not None:
            filas_h = [
                ("Lecturas procesadas con hora explícita (HORA_INICIO)", entero(he), pct(he / fl["procesadas"])),
                ("Lecturas procesadas con hora inferida por posición", entero(hi), pct(hi / fl["procesadas"])),
                ("… de ellas, con la hora por su posición en el bloque físico (días de 2022 con filas vacías)",
                 entero(fl.get("hora_por_bloque_fisico")), pct(fl.get("hora_por_bloque_fisico", 0) / fl["procesadas"])
                 if fl.get("hora_por_bloque_fisico") is not None else "—"),
                ("Descartadas por la regla `hora` (día incompleto u hora ilegible)", entero(desc.get("hora", 0)),
                 pct(desc.get("hora", 0) / leidas)),
                ("Filas anchas con al menos una celda de hora inferida", entero(fa.get("hora_inferida")),
                 pct(fa.get("hora_inferida", 0) / fa["total"]) if fa["total"] else "—"),
            ]
            self.tabla(tablas[7][0], o, d, tablas[7][1], tablas[7][2],
                       markdown(pd.DataFrame(filas_h, columns=["Concepto", "Lecturas", "Proporción"]), "lrr"),
                       anchos=[62, 20, 18], nota=
                       f"Convención de hora: «{d.get('convencion_hora', '?')}» (la lectura de 00:00 a 01:00 se fecha a "
                       "las 01:00). Las proporciones de las tres primeras filas son sobre las procesadas; la de la "
                       "regla, sobre las leídas; la última, sobre las filas anchas. La hora por bloque físico se "
                       "cuenta al asignarla: alguna de esas lecturas pudo caer después en otra regla.")
        else:
            self.tabla(tablas[7][0], o, None, tablas[7][1], tablas[7][2])

        ceros = fl.get("bloque_en_cero_por_parametro") or {}
        ceros_h = fl.get("cero_como_faltante_por_parametro") or {}
        z = pd.DataFrame([(corto(k), entero(ceros.get(k, 0)), entero(ceros_h.get(k, 0)) if ceros_h else "—")
                          for k in sorted(set(ceros) | set(ceros_h), key=lambda q: (-(ceros.get(q, 0) + ceros_h.get(q, 0)), q))],
                         columns=["Parámetro", "Días enteros en 0 (lecturas)", "Horas en 0 en días con datos"])
        if ceros:
            self.tabla(tablas[8][0], o, d, tablas[8][1], tablas[8][2], markdown(z, "lrr"), anchos=[34, 33, 33], nota=
                       "Un día completo de un parámetro con todas sus lecturas en 0 exacto es un equipo que no midió "
                       "(regla `bloque_en_cero`); en los días con datos, un 0 exacto de un contaminante es una hora sin "
                       "dato (regla `cero_como_faltante`), como lo trata OEFA. La meteorología no: un viento de 0 m/s "
                       "es calma, y una hora seca tiene precipitación 0.")
        else:
            self.tabla(tablas[8][0], o, None, tablas[8][1], tablas[8][2])

        fs = d.get("fases_s") or {}
        tot = d.get("duracion_s") or 0
        filas_f = [(texto, f"`{k}`", tiempo(fs[k]), pct(fs[k] / tot) if tot else "—", tipo)
                   for k, texto, tipo in FASES if k in fs]
        serial = sum(fs.get(k, 0) for k in ("horas", "tabla"))
        self.tabla(tablas[9][0], o, d, tablas[9][1], tablas[9][2],
                   markdown(pd.DataFrame(filas_f, columns=["Fase", "Clave", "Tiempo (s)", "% del total", "Tipo"]), "llrrl"),
                   anchos=[30, 20, 12, 12, 26], nota=
                   f"Carga de {tiempo(tot)} s con {d.get('workers')} workers y {entero(d.get('bloques'))} bloques de "
                   f"{entero(d.get('bloque_bytes', 0) / 2**20)} MiB. Las fases sin la fusión suman el total; la fusión "
                   "está dentro de la lectura. Fuera de la medición, la escritura del silver y el gold "
                   f"(`salida`): {tiempo(fs.get('salida'))} s.")

        desdes = [x["desde"] for x in est.values() if x.get("desde")]
        hastas = [x["hasta"] for x in est.values() if x.get("hasta")]
        self.guardar_cifras("limpieza", d, {
            "n_estaciones_con_datos": entero(sum(1 for x in est.values() if x["largas_procesadas"] > 0)),
            "n_estaciones_gold": entero(sum(1 for x in est.values() if x["completas_gold"] > 0)),
            "hora_inferida": entero(hi) if hi is not None else "—",
            "hora_explicita": entero(he) if he is not None else "—",
            "pct_hora_inferida": pct(hi / fl["procesadas"], 0) if hi is not None and fl["procesadas"] else "—",
            "pct_anchas_hora_inferida": pct(fa.get("hora_inferida", 0) / fa["total"], 0) if fa["total"] else "—",
            "descartadas_hora": entero(desc.get("hora", 0)),
            "descartadas_periodo": entero(desc.get("periodo", 0)),
            "descartadas_parametro": entero(desc.get("parametro", 0)),
            "bloque_en_cero": entero(desc.get("bloque_en_cero", 0)),
            "pct_bloque_en_cero": pct(desc.get("bloque_en_cero", 0) / leidas) if leidas else "—",
            "bloque_en_cero_pm10": entero(ceros.get("pm10", 0)),
            "fuera_de_rango": entero(desc.get("fuera_de_rango", 0)),
            "n_estaciones_sin_datos": entero(sum(1 for x in est.values() if x["largas_procesadas"] == 0)),
            "n_ubicaciones_distintas": entero(sum(1 for x in est.values() if x.get("ubicaciones_distintas"))),
            "estacion_gold_max": max(est.items(), key=lambda kv: kv[1]["completas_gold"])[0] if est else "—",
            "departamento_gold_max": titulo_lugar((max(est.values(), key=lambda x: x["completas_gold"]).get("ubicacion") or {}).get("departamento")) if est else "—",
            "pct_estacion_gold_max": pct(max(x["completas_gold"] for x in est.values()) / fa["completas_gold"], 0)
                                     if est and fa["completas_gold"] else "—",
            "estaciones_ubicaciones_distintas": ", ".join(sorted(n for n, x in est.items() if x.get("ubicaciones_distintas"))) or "ninguna",
            "fuera_de_rango_presion": entero((fl.get("fuera_de_rango_por_parametro") or {}).get("presion", 0)),
            "pct_descartadas_parametro": pct(desc.get("parametro", 0) / leidas) if leidas else "—",
            "sin_valor": entero(desc.get("sin_valor", 0)),
            "pct_sin_valor": pct(desc.get("sin_valor", 0) / leidas) if leidas else "—",
            "cero_como_faltante": entero(desc.get("cero_como_faltante", 0)),
            "cero_como_faltante_pm": entero(sum(v for q, v in (fl.get("cero_como_faltante_por_parametro") or {}).items()
                                                if q in ("pm10", "pm2_5"))),
            "hora_por_bloque_fisico": entero(fl.get("hora_por_bloque_fisico")),
            "ubicacion_corregida": entero(marc.get("ubicacion_corregida", 0)),
            "n_estaciones_inicio_fin_2021": entero(sum(1 for x in est.values() if (x.get("desde") or "")[:7] in ("2021-11", "2021-12"))),
            **self.grupo_vmp(est),
            "pct_descartadas_periodo": pct(desc.get("periodo", 0) / leidas) if leidas else "—",
            "pct_duplicado_unidad": pct(desc.get("duplicado_unidad", 0) / leidas) if leidas else "—",
            "duplicado_unidad": entero(desc.get("duplicado_unidad", 0)),
            "duplicado_exacto": entero(desc.get("duplicado_exacto", 0) + desc.get("duplicado_valor_distinto", 0)),
            "unidad_convertida": entero(marc.get("unidad_convertida", 0)),
            "convencion_hora": d.get("convencion_hora", "?"),
            "columnas": entero(len(d.get("cabecera", []))),
            "anios": (f"{min(desdes)[:4]}–{(datetime.fromisoformat(max(hastas)) - timedelta(hours=1)).year}"
                      if desdes and hastas else "—"),
            "fraccion_serial": pct(serial / tot) if tot else "—",
            "fusion_s": tiempo(fs.get("fusion")),
            "pm10_lecturas": entero((vistos.get("unidades", {}).get("pm10") or {}).get("ug/m3")),
            "pm2_5_lecturas": entero((vistos.get("unidades", {}).get("pm2_5") or {}).get("ug/m3")),
            "filas_largas_leidas": entero(leidas),
            "filas_largas_procesadas": entero(fl["procesadas"]),
            "filas_largas_descartadas": entero(fl["descartadas"]),
            "pct_descartadas": pct(fl["descartadas"] / leidas) if leidas else "—",
            "filas_anchas": entero(fa["total"]),
            "filas_completas": entero(fa["completas_gold"]),
            "pct_completas": pct(fa["completas_gold"] / fa["total"]) if fa["total"] else "—",
            "violaciones_pm": entero(fa.get("pm2_5_mayor_pm10")),
            "n_estaciones": entero(len(est)),
            "n_departamentos": entero(len([k for k in deps if k != "(sin dato)"])),
            "fecha_min": fecha(min(desdes)) if desdes else "—",
            "fecha_max": fecha(max(hastas)) if hastas else "—",
            "duracion_s": tiempo(d.get("duracion_s")),
            "workers": entero(d.get("workers")),
            "n_parametros": entero(len(d.get("parametros_presentes", []))),
            "no_mapeados": ", ".join(sorted(nomap)) or "ninguno",
            "delimitador": d.get("delimitador", "?"),
            "entrada": d.get("entrada", "?"),
            "bytes": entero(d.get("bytes")),
            "sha256": d.get("sha256") or "—",
            "sha256_config_limpieza": ((d.get("configs") or {}).get("limpieza") or {}).get("sha256") or "—",
            "sha256_config_kmeans": ((d.get("configs") or {}).get("kmeans") or {}).get("sha256") or "—",
        })

    @staticmethod
    def grupo_vmp(est: dict) -> dict:
        """El grupo de Ventanilla y Mi Perú (CA-VMP-*): cuántas estaciones tienen lecturas horarias."""
        vmp = {n: x for n, x in est.items() if n.startswith("CA-VMP")}
        con = {n: x for n, x in vmp.items() if x["largas_procesadas"] > 0}
        return {
            "vmp_estaciones": entero(len(vmp)),
            "vmp_con_horarias": entero(len(con)),
            "vmp_horarias_cuales": ", ".join(sorted(con)) or "ninguna",
            "vmp_lecturas": entero(sum(x["largas_procesadas"] for x in con.values())),
            "vmp_horas": entero(sum(x["anchas"] for x in con.values())),
            "vmp_desde": min((x.get("desde") or "")[:10] for x in con.values()) if con else "—",
            "vmp_hasta": max((x.get("hasta") or "")[:10] for x in con.values()) if con else "—",
        }

    # ----------------------------------------------------------------------- estandarización
    def estandarizacion(self) -> None:
        o = "estandarizacion.json"
        d = self.leer(o)
        titulo = "Media y desviación usadas en el z-score de cada feature"
        if d is None:
            self.tabla("estandarizacion.md", o, None, "tbl-estandarizacion", titulo)
            return
        feats, log = d["features"], set(d.get("log1p") or [])
        self.comprobar(o, len(d["media"]) == len(feats) == len(d["desv"]), "media, desv y features no tienen el mismo largo")
        lim = self.datos.get("limpieza.json")
        if lim:
            self.comprobar(o, d["n"] == lim["filas_anchas"]["completas_gold"],
                           "n no coincide con las filas completas del gold de limpieza.json")
        filas = [(nombre(f), "sí" if f in log else "no", decimal(m, 4), decimal(s, 4))
                 for f, m, s in zip(feats, d["media"], d["desv"])]
        t = pd.DataFrame(filas, columns=["Feature", "log1p", "Media", "Desv."])
        nota = (f"Nivel {d.get('nivel', '¿?')}, n = {entero(d['n'])} filas anchas completas, ddof = {d.get('ddof')}, "
                f"chunk de {entero(d.get('chunk'))} filas. Con log1p, la media y la desviación son de log(1 + x).")
        por = d.get("por_estacion") or {}
        if por:
            nota += f" Además hay media y desviación por estación ({len(por)} estaciones) en el JSON."
        self.tabla("estandarizacion.md", o, d, "tbl-estandarizacion", titulo, markdown(t, "llrr"),
                   anchos=[45, 15, 20, 20], nota=nota)
        self.guardar_cifras("estandarizacion", d, {
            "n": entero(d["n"]), "n_features": entero(len(feats)), "nivel": d.get("nivel", ""),
            "features": ", ".join(nombre(f) for f in feats), "ddof": str(d.get("ddof")),
        })

    # ----------------------------------------------------------------------- benchmarks
    def benchmark_cargador(self) -> None:
        o = "benchmark_cargador.json"
        d = self.leer(o)
        titulo = "Tiempo y speedup del cargador concurrente"
        if d is None:
            self.tabla("benchmark_cargador.md", o, None, "tbl-bench-cargador", titulo)
            return
        med = d["mediciones"]
        base = med[0]
        pr = d.get("protocolo", {})
        self.revisar_entrada(o, d.get("entrada"))
        for m in med:
            self.comprobar(o, abs(m["speedup"] - base["mediana_s"] / m["mediana_s"]) < 1e-9,
                           f"el speedup de W = {m['workers']} no es mediana(base) / mediana(W)")
            if m.get("speedup_ic") and m.get("media_recortada_s"):
                self.comprobar(o, abs(m["speedup_ic"]["punto"] - base["media_recortada_s"] / m["media_recortada_s"]) < 1e-9,
                               f"el speedup_ic de W = {m['workers']} no es media recortada(base) / media recortada(W)")
        self.comprobar(o, bool(d.get("identicos")), "silver y gold NO son idénticos entre W: se perdió el determinismo")

        def punto(m):
            """Speedup principal: cociente de medias recortadas (con IC), como en el K-Means."""
            return (m.get("speedup_ic") or {}).get("punto", m["speedup"])

        def ic(m):
            x = m.get("speedup_ic")
            return f"[{decimal(x['inferior'], 2)}; {decimal(x['superior'], 2)}]" if x else "—"

        t = pd.DataFrame([(
            entero(m["workers"]),
            tiempo(m["mediana_s"]),
            tiempo(m.get("media_recortada_s")),
            decimal(m["speedup"], 2),
            decimal(punto(m), 2),
            ic(m),
            pct(punto(m) / m["workers"], 0),
            f"`{m.get('sha256_gold', '')[:12]}`",
        ) for m in med], columns=["Workers", "Mediana (s)", "Media recortada (s)", "Speedup (medianas)",
                                  "Speedup", "IC 95 %", "Eficiencia", "sha256 del gold"])
        self.tabla("benchmark_cargador.md", o, d, "tbl-bench-cargador", titulo, markdown(t, "rrrrrrrl"),
                   anchos=[9, 11, 12, 11, 9, 15, 11, 22], nota=
                   f"Carga completa (lectura, limpieza, pivot y gold, sin escribir los CSV) de `{Path(str(d.get('entrada'))).name}` "
                   f"({entero(d.get('filas_largas'))} filas largas, {entero(d.get('bloques'))} bloques de "
                   f"{entero(d.get('bloque_bytes', 0) / 2**20)} MiB). {d.get('repeticiones')} repeticiones por W, orden "
                   f"{d.get('orden', '¿?')}. Speedup: {pr.get('speedup_ic', 'media recortada con la base / media recortada con W')}, "
                   f"{entero(pr.get('replicas_bootstrap'))} réplicas, recorte de {pct(pr.get('recorte_por_extremo'), 0)} "
                   f"por extremo; al lado, el cociente de medianas. Base: {base['workers']} worker. "
                   f"Eficiencia = speedup / workers. {d.get('cpus')} CPU, {d.get('go', '')}, {d.get('plataforma', '')}. "
                   + (f"{pr['nota_recorte']}. " if pr.get("nota_recorte") else "") +
                   ("El silver y el gold salieron idénticos para todo W." if d.get("identicos") else "El silver y el gold NO salieron idénticos para todo W."))
        # El resumen reporta un W fijado de antemano (el máximo medido), no el mejor de la tabla: elegir
        # el máximo entre configuraciones ruidosas sesga el speedup hacia arriba.
        fijo = max(med, key=lambda m: m["workers"])
        self.guardar_cifras("benchmark_cargador", d, {
            "w": entero(fijo["workers"]),
            "speedup": decimal(punto(fijo), 2),
            "speedup_ic": ic(fijo),
            "speedup_medianas": decimal(fijo["speedup"], 2),
            "mediana_base_s": tiempo(base["mediana_s"]),
            "mediana_w_s": tiempo(fijo["mediana_s"]),
            "eficiencia": pct(punto(fijo) / fijo["workers"], 0),
            "karp_flatt": pct(karp_flatt(punto(fijo), fijo["workers"]), 0),
            "techo_amdahl": techo_amdahl(karp_flatt(punto(fijo), fijo["workers"])),
            "workers": lista([m["workers"] for m in med]),
            "bloque": f"{entero(d.get('bloque_bytes', 0) / 2**20)} MiB",
            "repeticiones": entero(d.get("repeticiones")),
            "identicos": "sí" if d.get("identicos") else "no",
            "maquina": str((d.get("metadatos") or {}).get("maquina") or d.get("host", "")),
        })

    def benchmark_kmeans(self) -> None:
        o = "benchmark_kmeans.json"
        d = self.leer(o)
        titulo = "Tiempo y speedup del K-Means paralelo frente al secuencial"
        if d is None:
            self.tabla("benchmark_kmeans.md", o, None, "tbl-bench-kmeans", titulo)
            return
        pr = d.get("protocolo", {})
        res = d["resumen"]
        det = d.get("determinismo", [])
        if d.get("datos", {}).get("sintetico"):
            self.sintetico(o, "gold sintético (datos.sintetico)")
        self.revisar_entrada(o, d.get("datos", {}).get("ruta"))
        for x in det:
            self.comprobar(o, bool(x.get("identico_bit_a_bit_para_todo_w")),
                           f"experimento {x.get('experimento')}: el resultado NO es idéntico bit a bit para todo W")
        exp = {"fijas": f"{pr.get('iteraciones_fijas', '¿?')} iteraciones", "convergencia": "hasta converger"}
        filas = []
        for r in res:
            sp = r.get("speedup") or {}
            conc = r["modo"] == "conc"
            filas.append((
                exp.get(r["experimento"], r["experimento"]),
                {"seq": "sec.", "conc": "conc."}.get(r["modo"], r["modo"]),
                entero(r.get("workers")) if conc else "—",
                entero(r.get("chunk")) if conc else "—",
                tiempo(r["mediana_ms"]),
                tiempo(r["media_recortada_ms"]),
                tiempo(r.get("mediana_ms_por_iteracion")),
                decimal(sp["punto"], 2) if sp else "1 (ref.)",
                f"[{decimal(sp['inferior'], 2)}; {decimal(sp['superior'], 2)}]" if sp else "—",
                pct(r["eficiencia_por_worker"], 0) if r.get("eficiencia_por_worker") else "—",
            ))
        t = pd.DataFrame(filas, columns=["Experimento", "Modo", "W", "Chunk", "Mediana (ms)", "Media recortada (ms)",
                                         "ms/iter.", "Speedup", "IC 95 %", "Eficiencia"])
        m = d.get("metadatos", {})
        nota = (f"{pr.get('repeticiones_medidas', '¿?')} repeticiones medidas tras {pr.get('rondas_calentamiento', 0)} "
                f"de calentamiento, orden {pr.get('orden', '¿?')}; se mide {pr.get('se_mide', 'el clustering')}. "
                f"Speedup: {pr.get('speedup', '¿?')} ({entero(pr.get('replicas_bootstrap'))} réplicas); "
                f"recorte de {pct(pr.get('recorte_por_extremo'), 0)} por extremo. "
                f"n = {entero(d.get('datos', {}).get('n'))} filas, k = {d.get('receta', {}).get('k')}"
                f"{', gold sintético' if d.get('datos', {}).get('sintetico') else ''}. "
                f"{m.get('cpus_logicas', '¿?')} CPU lógicas, {m.get('version_go', '')}. "
                + (("El resultado fue idéntico bit a bit para todo W en los dos experimentos." if all(x.get("identico_bit_a_bit_para_todo_w") for x in det)
                    else "El resultado NO fue idéntico bit a bit para todo W en: " + ", ".join(exp.get(x["experimento"], x["experimento"]) for x in det if not x.get("identico_bit_a_bit_para_todo_w")) + ".")
                   if det else ""))
        self.tabla("benchmark_kmeans.md", o, d, "tbl-bench-kmeans", titulo, markdown(t, "llrrrrrrrr"),
                   anchos=[14, 7, 5, 8, 11, 12, 10, 9, 15, 9], nota=nota)

        # W fijo de antemano: el experimento hasta converger (mismo resultado), el máximo de workers
        # medido y el chunk de la receta si se midió (si no, el primero del protocolo). No el mejor de la
        # tabla, que sesga el speedup hacia arriba.
        concs = [r for r in res if r["modo"] == "conc" and r.get("speedup")]
        exper = "convergencia" if any(r["experimento"] == "convergencia" for r in concs) else (concs[0]["experimento"] if concs else None)
        candidatos = [r for r in concs if r["experimento"] == exper]
        if candidatos:
            w_max = max(r["workers"] for r in candidatos)
            chunks = [r["chunk"] for r in candidatos if r["workers"] == w_max]
            chunk_receta = d.get("receta", {}).get("chunk")
            orden = [c for c in (pr.get("chunks") or []) if c in chunks] or chunks
            chunk = chunk_receta if chunk_receta in chunks else orden[0]
            fijo = next(r for r in candidatos if r["workers"] == w_max and r["chunk"] == chunk)

            def otro_chunk():
                """El otro chunk medido con el mismo W: muestra cuánto pesa el tamaño de chunk."""
                otros = [r for r in candidatos if r["workers"] == w_max and r["chunk"] != chunk]
                if not otros:
                    return {}
                r = min(otros, key=lambda x: x["chunk"])
                return {"otro_chunk": entero(r["chunk"]),
                        "speedup_otro_chunk": decimal(r["speedup"]["punto"], 2),
                        "speedup_otro_chunk_ic": f"[{decimal(r['speedup']['inferior'], 2)}; {decimal(r['speedup']['superior'], 2)}]",
                        "n_chunks_otro": entero(math.ceil(d.get("datos", {}).get("n", 0) / r["chunk"]))}
            sec = next((r for r in res if r["modo"] == "seq" and r["experimento"] == exper), None)
            self.guardar_cifras("benchmark_kmeans", d, {
                "experimento": exp.get(exper, exper),
                "w": entero(fijo["workers"]),
                "chunk": entero(fijo["chunk"]),
                "speedup": decimal(fijo["speedup"]["punto"], 2),
                "speedup_ic": f"[{decimal(fijo['speedup']['inferior'], 2)}; {decimal(fijo['speedup']['superior'], 2)}]",
                "mediana_seq_ms": tiempo(sec["mediana_ms"]) if sec else "—",
                "mediana_w_ms": tiempo(fijo["mediana_ms"]),
                # Lo que cuesta una iteración: la vara contra la que se mide la barrera de red de la PC4.
                "ms_iter_seq": tiempo(sec.get("mediana_ms_por_iteracion")) if sec else "—",
                "ms_iter_w": tiempo(fijo.get("mediana_ms_por_iteracion")),
                "eficiencia": pct(fijo["eficiencia_por_worker"], 0),
                "karp_flatt": pct(karp_flatt(fijo["speedup"]["punto"], fijo["workers"]), 0),
                "techo_amdahl": techo_amdahl(karp_flatt(fijo["speedup"]["punto"], fijo["workers"])),
                "n_chunks": entero(math.ceil(d.get("datos", {}).get("n", 0) / fijo["chunk"])) if fijo["chunk"] else "—",
                **otro_chunk(),
                "workers": lista(pr.get("workers") or [r.get("workers") for r in concs]),
                "chunks": lista(pr.get("chunks") or [r.get("chunk") for r in concs]),
                "identico": "sí" if det and all(x.get("identico_bit_a_bit_para_todo_w") for x in det) else "no",
                "maquina": str(m.get("maquina", "")),
            })

    # ----------------------------------------------------------------------- K-Means
    def kmeans(self) -> None:
        o = "kmeans.json"
        d = self.leer(o)
        if d is None:
            self.tabla("kmeans_centroides_z.md", o, None, "tbl-centroides-z", "Centroides estandarizados")
            self.tabla("kmeans_semillas.md", o, None, "tbl-semillas", "Arranques del K-Means por semilla")
            return
        feats = d["features"]
        tam = d["tamanos"]
        n = d.get("gold", {}).get("n", sum(tam))
        self.comprobar(o, sum(tam) == n, f"la suma de tamaños ({sum(tam)}) ≠ n del gold ({n})")
        self.comprobar(o, len(d["centroides_estandarizados"]) == len(tam) == d.get("k_efectivo", len(tam)),
                       "centroides, tamaños y k_efectivo no coinciden")
        z = pd.DataFrame({"Cluster": [entero(i) for i in range(len(tam))], "Filas": [entero(x) for x in tam],
                          "%": [pct(x / n) for x in tam]})
        for i, f in enumerate(feats):
            z[corto(f)] = [decimal(c[i], 3) for c in d["centroides_estandarizados"]]
        paro = ("convergió por tolerancia" if d.get("paro") == "tolerancia" else
                "llegó al máximo de iteraciones" if d.get("paro") == "max_iter" else f"paro «{d.get('paro')}»")
        nota = (f"k = {d['k']} (efectivo {d.get('k_efectivo')}), semilla elegida {d.get('semilla')}, {d['iteraciones']} "
                f"iteraciones ({paro}), inercia {decimal(d['inercia'], 1)}, n = {entero(n)}, chunk "
                f"{entero(d.get('chunk'))}, clusters vacíos: {d.get('clusters_vacios', 0)}. El perfil de cada cluster "
                "en unidades originales está en la tabla de regímenes: es la media aritmética de sus lecturas, no "
                "este centroide invertido.")
        al = "rrr" + "r" * len(feats)
        anch = [10, 12, 10] + [68 // len(feats)] * len(feats)
        anch[0] += 100 - sum(anch)
        self.tabla("kmeans_centroides_z.md", o, d, "tbl-centroides-z",
                   "Centroides en el espacio estandarizado (z-score) en que corre el K-Means", markdown(z, al),
                   anchos=anch, nota=nota)

        arr = d.get("arranques") or []
        elegido = d.get("arranque_elegido")
        if arr:
            mejor = min(range(len(arr)), key=lambda i: (arr[i]["inercia"], arr[i]["semilla"]))
            self.comprobar(o, elegido is None or elegido == mejor, "arranque_elegido no es el de menor inercia")
            self.comprobar(o, arr[mejor]["semilla"] == d.get("semilla"), "la semilla elegida no es la del arranque de menor inercia")
        t = pd.DataFrame([(
            str(a["semilla"]), decimal(a["inercia"], 1), entero(a["iteraciones"]), a.get("paro", ""),
            entero(a.get("workers")), tiempo(a.get("ms_kmeanspp")), tiempo(a.get("ms_lloyd")),
            "**sí**" if i == elegido else "",
        ) for i, a in enumerate(arr)], columns=["Semilla", "Inercia", "Iteraciones", "Paro", "Workers",
                                                "k-means++ (ms)", "Lloyd (ms)", "Elegida"])
        ms = d.get("ms", {})
        suma = sum((a.get("ms_kmeanspp") or 0) + (a.get("ms_lloyd") or 0) for a in arr)
        pared = ms.get("arranques")
        wpc = d.get("workers_por_corrida") or []
        ci = d.get("centroides_iniciales") or {}
        self.tabla("kmeans_semillas.md", o, d, "tbl-semillas", "Arranques del K-Means: una corrida completa por semilla",
                   markdown(t, "rrrlrrrc") if arr else "| (sin arranques) |\n|:---|\n| — |",
                   anchos=[11, 15, 13, 14, 11, 14, 12, 10], nota=
                   f"{d.get('semillas', len(arr))} semillas desde {d.get('semilla_base')}; criterio: "
                   f"{d.get('criterio_semillas', 'menor inercia final')}. Paralelismo por semillas: "
                   f"{d.get('corridas_a_la_vez', '¿?')} corridas a la vez, con {', '.join(str(x) for x in wpc) or '¿?'} "
                   f"workers cada una ({d.get('workers')} en total). Los {len(arr)} arranques tardaron "
                   f"{tiempo(pared)} ms de pared frente a {tiempo(suma)} ms de la suma de sus corridas"
                   + (f" ({decimal(suma / pared, 1)} corridas a la vez en promedio)." if pared else ".")
                   + (f" {d['nota_tiempos_arranques']}" if d.get("nota_tiempos_arranques") else "")
                   + (f" Centroides iniciales de k-means++: `{ci.get('archivo')}` ({ci.get('origen')}, sha256 "
                      f"`{str(ci.get('sha256', ''))[:12]}`), para que otra corrida parta de los mismos." if ci else ""))
        self.guardar_cifras("kmeans", d, {
            "k": entero(d["k"]), "k_efectivo": entero(d.get("k_efectivo")), "semilla": str(d.get("semilla")),
            "semillas": entero(d.get("semillas", len(arr))), "semilla_base": str(d.get("semilla_base")),
            "corridas_a_la_vez": entero(d.get("corridas_a_la_vez")),
            "inercia_peor_semilla": decimal(max(a["inercia"] for a in arr), 1) if arr else "—",
            "ms_arranques_pared": tiempo(pared), "ms_arranques_suma": tiempo(suma),
            "solapamiento_semillas": decimal(suma / pared, 1) if pared else "—",
            "semillas_cerca_del_minimo": entero(sum(1 for a in arr if a["inercia"] <= min(x["inercia"] for x in arr) * (1 + 1e-5))) if arr else "—",
            "inercia_relativa_peor": pct(max(a["inercia"] for a in arr) / min(a["inercia"] for a in arr) - 1, 2) if arr else "—",
            "iteraciones": entero(d["iteraciones"]),
            "paro": (f"convergió por tolerancia en {entero(d['iteraciones'])} iteraciones" if d.get("paro") == "tolerancia"
                     else f"llegó al máximo de {entero(d['iteraciones'])} iteraciones sin converger"),
            "inercia": decimal(d["inercia"], 1), "n": entero(n),
            "features": ", ".join(nombre(f) for f in feats),
            "workers": entero(d.get("workers")), "chunk": entero(d.get("chunk")),
            "clusters_vacios": entero(d.get("clusters_vacios", 0)),
            "nivel": d.get("receta", {}).get("nivel", ""),
        })

    def regimenes(self) -> None:
        o = "regimenes.json"
        d = self.leer(o)
        archivos = (("regimenes.md", "tbl-regimenes", "Perfil de cada régimen y su etiqueta frente al ECA"),
                    ("regimenes_hora.md", "tbl-regimenes-hora", "Distribución de cada régimen por franja horaria"),
                    ("regimenes_estacion.md", "tbl-regimenes-estacion", "Estaciones donde más aparece cada régimen"))
        if d is None:
            for a, i, t in archivos:
                self.tabla(a, o, None, i, t)
            return
        rg = d["regimenes"]
        self.revisar_estaciones(o, {e for x in rg for e in x.get("por_estacion", {})})
        n = d.get("gold", {}).get("n", sum(x["tamano"] for x in rg))
        self.comprobar(o, sum(x["tamano"] for x in rg) == n, "la suma de tamaños ≠ n del gold")
        for x in rg:
            self.comprobar(o, sum(x["por_hora"]) == x["tamano"], f"por_hora del régimen {x['regimen']} no suma su tamaño")
            self.comprobar(o, sum(x["por_estacion"].values()) == x["tamano"],
                           f"por_estacion del régimen {x['regimen']} no suma su tamaño")
        # kmeans.json y regimenes.json los escribe la misma corrida de cmd/entrenar: mismo gold y mismos tamaños.
        km = self.datos.get("kmeans.json")
        if km and ((km.get("gold") or {}).get("sha256") != (d.get("gold") or {}).get("sha256")
                   or km.get("tamanos") != [x["tamano"] for x in sorted(rg, key=lambda x: x["regimen"])]):
            self.desactualizadas.append("regimenes.json y kmeans.json no son de la misma corrida de cmd/entrenar "
                                        "(otro gold u otros tamaños)")
        feats = d.get("features") or sorted(rg[0]["perfil"])
        eca = d.get("eca", {})

        def etiqueta(x):
            e = ECA.get(x["eca"], x["eca"])
            por = x.get("eca_por") or []
            return f"{e} ({', '.join(corto(p) for p in por)})" if por else e

        # Con k = 3 y solo PM como features, los regímenes son niveles: se nombran por su PM10 medio.
        niveles = {}
        if len(rg) == 3:
            orden = sorted(rg, key=lambda x: x.get("pm10_medio") or 0)
            niveles = {x["regimen"]: n for x, n in zip(orden, ("bajo", "medio", "alto"))}
        filas = []
        for x in rg:
            f = {"Régimen": entero(x["regimen"])}
            if niveles:
                f["Nivel"] = niveles[x["regimen"]]
            f.update({"Filas": entero(x["tamano"]), "%": pct(x["fraccion"])})
            for c in feats:
                f[corto(c)] = decimal(x["perfil"].get(c), 2 if c == "fraccion_fina" else 1)
            # PM2,5 y PM10 medios van aparte solo si no son features (si lo son, ya están en el perfil).
            if "pm2_5" not in feats:
                f["PM2,5 medio"] = decimal(x.get("pm2_5_medio"), 1)
            if "pm10" not in feats:
                f["PM10 medio"] = decimal(x.get("pm10_medio"), 1)
            if "fraccion_fina" not in feats and x.get("pm2_5_medio") and x.get("pm10_medio"):
                f["PM2,5/PM10"] = decimal(x["pm2_5_medio"] / x["pm10_medio"], 2)
            f["ECA"] = etiqueta(x)
            filas.append(f)
        t = pd.DataFrame(filas)
        fijas = 4 if niveles else 3                          # Régimen, (Nivel,) Filas, %
        extra = len(t.columns) - fijas - len(feats) - 1      # columnas agregadas (PM medio, cociente)
        anch = [9] + ([9] if niveles else []) + [10, 9] + [(58 - 11 * extra - (9 if niveles else 0)) // len(feats)] * len(feats) + [11] * extra + [16]
        anch[-1] += 100 - sum(anch)
        origenes = {x.get("pm2_5_origen") for x in rg if x.get("pm2_5_origen")}
        nota = (f"{unidades(feats)} Perfil: media aritmética de cada feature en unidades originales sobre las lecturas "
                "del régimen (no es el centroide des-estandarizado). PM2,5 medio"
                + (f" derivado como `{', '.join(sorted(origenes))}` lectura por lectura" if origenes else "")
                + f"; PM2,5 y PM10 en µg/m³. Etiqueta ECA ({eca.get('norma', 'DS 003-2017-MINAM')}): excede si el "
                f"PM2,5 medio supera {eca.get('pm2_5_ugm3', 50)} µg/m³ o el PM10 medio supera "
                f"{eca.get('pm10_ugm3', 100)} µg/m³; no evaluable si falta el contaminante que podría superarlo. "
                "El ECA es un promedio de 24 horas: una media horaria alta no es por sí sola un incumplimiento.")
        self.tabla(archivos[0][0], o, d, archivos[0][1], archivos[0][2],
                   markdown(t, ("rlrr" if niveles else "rrr") + "r" * (len(feats) + extra) + "l"), anchos=anch, nota=nota)

        h = []
        for x in rg:
            f = {"Régimen": entero(x["regimen"]) + (f" ({niveles[x['regimen']]})" if niveles else "")}
            for etiq, horas in FRANJAS:
                f[etiq] = pct(sum(x["por_hora"][i] for i in horas) / x["tamano"], 0) if x["tamano"] else "—"
            pico = max(range(24), key=lambda i: x["por_hora"][i])
            f["Hora pico"] = f"{pico:02d}:00"
            h.append(f)
        self.tabla(archivos[1][0], o, d, archivos[1][1], archivos[1][2], markdown(pd.DataFrame(h), "rrrrrr"),
                   anchos=[12, 18, 18, 18, 18, 16], nota=
                   "Proporción de las filas de cada régimen en cada franja (hora de Lima, UTC−5). La "
                   "distribución hora a hora está en la @fig-regimen-hora.")

        s = []
        for x in rg:
            top = sorted(x["por_estacion"].items(), key=lambda kv: (-kv[1], kv[0]))[:3]
            s.append({"Régimen": entero(x["regimen"]) + (f" ({niveles[x['regimen']]})" if niveles else ""),
                      "Estaciones principales (proporción del régimen)":
                          "; ".join(f"{e} ({pct(c / x['tamano'], 0)})" for e, c in top if c > 0),
                      "Estaciones donde aparece": entero(sum(1 for c in x["por_estacion"].values() if c > 0))})
        self.tabla(archivos[2][0], o, d, archivos[2][1], archivos[2][2], markdown(pd.DataFrame(s), "rlr"),
                   anchos=[12, 66, 22], nota=
                   "Las tres estaciones con más filas de cada régimen. Que un régimen se concentre en una "
                   "estación no dice que esa estación tenga peor aire que una zona sin estación.")

        lim = (self.datos.get("limpieza.json") or {}).get("estaciones", {})
        for x in rg:
            r = x["regimen"]
            top = sorted(x["por_estacion"].items(), key=lambda kv: (-kv[1], kv[0]))
            est0, n0 = top[0] if top else ("—", 0)
            ub = (lim.get(est0) or {}).get("ubicacion") or {}
            pico = max(range(24), key=lambda i: x["por_hora"][i])
            franja = max(FRANJAS, key=lambda fr: sum(x["por_hora"][i] for i in fr[1]))
            self.guardar_cifras(niveles.get(r, f"r{r}"), d, {
                "regimen": entero(r),
                "pct": pct(x["fraccion"], 1),
                "filas": entero(x["tamano"]),
                **{c: decimal(x["perfil"].get(c), 2 if c == "fraccion_fina" else 1) for c in feats},
                "pm2_5": decimal(x.get("pm2_5_medio"), 1),
                "pm10": decimal(x.get("pm10_medio"), 1),
                "eca": ECA.get(x["eca"], x["eca"]),
                "hora_pico": f"{pico:02d}:00",
                "franja": franja[0].split(" (")[0].lower(),
                "pct_franja": pct(sum(x["por_hora"][i] for i in franja[1]) / x["tamano"], 0) if x["tamano"] else "—",
                "estacion": est0,
                "pct_estacion": pct(n0 / x["tamano"], 0) if x["tamano"] else "—",
                "departamento": titulo_lugar(ub.get("departamento")),
                "altitud": entero(float(ub["altitud"])) if ub.get("altitud") else "—",
                "principales": ", ".join(
                    f"{e} ({titulo_lugar(((lim.get(e) or {}).get('ubicacion') or {}).get('departamento'))}, "
                    f"{pct(c / x['tamano'], 0)})" for e, c in top[:3] if c > 0) if x["tamano"] else "—",
                "n_estaciones": entero(sum(1 for c in x["por_estacion"].values() if c > 0)),
                "cociente": decimal(x["pm2_5_medio"] / x["pm10_medio"], 2) if x.get("pm2_5_medio") and x.get("pm10_medio") else "—",
            })
        if niveles:
            alto = next(x for x in rg if niveles[x["regimen"]] == "alto")
            lima = {n for n, e in lim.items() if ((e.get("ubicacion") or {}).get("departamento") or "").upper() == "LIMA"}
            de_lima = {e: c for e, c in alto["por_estacion"].items() if e in lima and c > 0}
            self.cifras["alto"].update({k: self.cifra(d, v) for k, v in {
                "horas_lima": entero(sum(de_lima.values())),
                "pct_lima": pct(sum(de_lima.values()) / alto["tamano"], 0) if alto["tamano"] else "—",
                "estaciones_lima": ", ".join(f"{e} ({entero(c)})" for e, c in sorted(de_lima.items(), key=lambda kv: -kv[1])) or "ninguna",
                "lima_desde_hasta": "; ".join(f"{e}: {fecha((lim.get(e) or {}).get('desde'))} a {fecha((lim.get(e) or {}).get('hasta'))}"
                                             for e in sorted(de_lima, key=lambda e: -de_lima[e])),
            }.items()})
        excede = [x for x in rg if x["eca"] == "excede"]
        self.guardar_cifras("regimenes", d, {
            "pm10_max": decimal(max((x.get("pm10_medio") or 0) for x in rg), 1),
            "pm2_5_max": decimal(max((x.get("pm2_5_medio") or 0) for x in rg), 1),
            "n": entero(len(rg)),
            "n_excede": entero(len(excede)),
            "n_no_evaluable": entero(sum(1 for x in rg if x["eca"] == "no_evaluable")),
            "excede": "; ".join(f"régimen {x['regimen']} ({', '.join(corto(p) for p in x.get('eca_por') or [])})"
                                if x.get("eca_por") else f"régimen {x['regimen']}" for x in excede) or "ninguno",
            "pct_filas_excede": pct(sum(x["fraccion"] for x in excede)),
        })

    def codo(self) -> None:
        o = "codo.json"
        d = self.leer(o)
        titulo = "Inercia por número de clusters (método del codo)"
        if d is None:
            self.tabla("codo.md", o, None, "tbl-codo", titulo)
            return
        p = pd.DataFrame(d["puntos"]).sort_values("k")
        caida = p["inercia"].pct_change().mul(-1)
        t = pd.DataFrame({"k": p["k"].map(entero), "k efectivo": p["k_efectivo"].map(entero),
                          "Inercia": p["inercia"].map(lambda x: decimal(x, 1)),
                          "Caída frente a k − 1": caida.map(lambda x: "—" if pd.isna(x) else pct(x)),
                          "Iteraciones": p["iteraciones"].map(entero), "Paro": p["paro"]})
        if "semilla" in p:
            t.insert(3, "Semilla", p["semilla"].map(str))
        k = d.get("receta", {}).get("k")
        self.tabla("codo.md", o, d, "tbl-codo", titulo, markdown(t, "rrr" + ("r" if "semilla" in p else "") + "rrl"),
                   anchos=([9, 11, 17, 11, 18, 15, 19] if "semilla" in p else [10, 14, 20, 22, 16, 18]),
                   nota=f"n = {lista(p['n'].tolist())} filas. {d.get('nota', '')} k de la receta: {k}.")
        self.guardar_cifras("codo", d, {"k_elegido": entero(k), "k_min": entero(p["k"].min()),
                                        "k_max": entero(p["k"].max())})

    # ----------------------------------------------------------------------- el CSV real
    def csv_oefa(self) -> None:
        """El CSV de OEFA tal como viene: casi todo de limpieza.json; lo que solo mide el EDA (el
        control de la hora por posición, los registros multilínea, los episodios) de generado/eda.json,
        transcrito de reports/eda_criba.md con su fuente."""
        o = "limpieza.json"
        d = self.datos.get(o)
        titulo = "El CSV de OEFA tal como viene"
        if d is None:
            self.tabla("csv_oefa.md", o, None, "tbl-csv-oefa", titulo)
            return
        ruta_eda = AQUI / "generado" / "eda.json"
        eda = json.loads(ruta_eda.read_text(encoding="utf-8")) if ruta_eda.exists() else {}
        if not eda:
            self.faltantes.append("eda.json")
        fl, est = d["filas_largas"], d.get("estaciones", {})
        desc = fl.get("descartadas_por_regla", {})
        per = d.get("vistos", {}).get("periodos", {})
        horarios = sum(v for k, v in per.items() if k.strip().lower() in ("1 hora", "-"))
        ch = eda.get("control_hora", {})
        if ch:
            self.comprobar("eda.json", ch["filas_posicion"] <= ch["filas_con_hora_inicio"], "control de hora inconsistente")
        filas = [
            ("Tamaño y sha256", f"{entero(d['bytes'])} bytes, `{(d.get('sha256') or '¿?')[:12]}…`"),
            ("Formato", f"{entero(len(d.get('cabecera', [])))} columnas, delimitador «{d.get('delimitador')}», "
                        f"{'con' if d.get('bom') else 'sin'} BOM; una columna por parámetro y un valor por fila"),
            ("Registros con un salto de línea dentro de un campo", entero(eda.get("multilinea")) if eda else "—"),
            ("Filas leídas", entero(fl["leidas"])),
            ("Estaciones: en el archivo · con lecturas válidas · en el gold",
             f"{entero(len(est))} · {entero(sum(1 for x in est.values() if x['largas_procesadas'] > 0))} · "
             f"{entero(sum(1 for x in est.values() if x['completas_gold'] > 0))}"),
            ("Periodos horarios («1 hora» y «-») · otros (24 h, móviles)",
             f"{entero(horarios)} · {entero(sum(per.values()) - horarios)}"),
            ("Filas sin ningún valor (regla `sin_valor`: la plantilla de 2022)", entero(desc.get("sin_valor", 0))),
            ("Lecturas procesadas con hora inferida por posición", f"{entero(fl.get('hora_inferida'))} "
             f"({pct(fl.get('hora_inferida', 0) / fl['procesadas'])})" if fl.get("procesadas") else "—"),
            ("Control: hora de inicio = posición en el bloque físico, donde hay hora explícita",
             f"{pct(ch['filas_bloque_fisico_posicion'] / ch['filas_bloque_fisico'])} de "
             f"{entero(ch['filas_bloque_fisico'])} filas" if ch.get("filas_bloque_fisico") else "—"),
            ("Días enteros en 0 exacto (regla `bloque_en_cero`)", entero(desc.get("bloque_en_cero", 0))),
            ("Horas en 0 exacto de un contaminante en días con datos (`cero_como_faltante`)", entero(desc.get("cero_como_faltante", 0))),
            ("Lecturas gemelas en otra unidad (regla `duplicado_unidad`)", entero(desc.get("duplicado_unidad", 0))),
        ]
        t = pd.DataFrame(filas, columns=["Característica", "Valor"])
        partes = [f"<!-- Generado por tablas.py desde generado/limpieza.json y generado/eda.json. No editar a mano. -->", ""]
        if d.get("placeholder"):
            partes += ["::: {.callout-warning title=\"PLACEHOLDER\"}", "Cifras de prueba: `generado/limpieza.json` no es la "
                       "corrida sobre el CSV de OEFA.", ":::", ""]
        partes += ['::: {#tbl-csv-oefa tbl-colwidths="[50,50]"}', markdown(t, "ll"), "", titulo, ":::", ""]
        partes += [f"*Nota.* Fuente: `generado/limpieza.json`, escrito por `cmd/cargar`{self.procedencia(d)}; el "
                   "control de la hora y los registros multilínea, de `reports/eda_criba.md` (notebook 01 sobre el "
                   f"mismo archivo), transcritos en `generado/eda.json`.", ""]
        (self.salida / "csv_oefa.md").write_text("\n".join(partes), encoding="utf-8")
        if eda:
            ep = eda.get("episodios", {})
            self.cifras["eda"] = {k: str(v) for k, v in {
                "multilinea": entero(eda.get("multilinea")),
                "control_hora": pct(ch["filas_en_bloques_completos_posicion"] / ch["filas_en_bloques_completos"]) if ch else "—",
                "control_filas": entero(ch.get("filas_en_bloques_completos")),
                "control_dias": entero(ch.get("dias_en_bloques_completos")),
                "control_filas_con_hora": entero(ch.get("filas_con_hora_inicio")),
                "control_pct_todas": pct(ch["filas_posicion"] / ch["filas_con_hora_inicio"]) if ch else "—",
                "estaciones_episodios": entero(ep.get("estaciones_con_episodios_24h")),
                "episodios_24h": entero(ep.get("episodios_24h")),
                "control_fisico": pct(ch["filas_bloque_fisico_posicion"] / ch["filas_bloque_fisico"]) if ch.get("filas_bloque_fisico") else "—",
                "control_fisico_filas": entero(ch.get("filas_bloque_fisico")),
                "control_fisico_huecos": entero(ch.get("filas_bloque_fisico_con_huecos")),
                **{k: (entero(v) if isinstance(v, int) else (decimal(v, 2 if v < 99.9 else 2).rstrip("0").rstrip(",")
                                                               if isinstance(v, float) else str(v)))
                   for k, v in (eda.get("procedencia") or {}).items()},
            }.items()}

    # ----------------------------------------------------------------------- tipología de estaciones
    def tipologia(self) -> None:
        """Cuatro tipos de serie (notebook 04, reports/estaciones_ficha.csv), con las lecturas válidas y las
        filas del gold de la corrida actual (limpieza.json): la clasificación no depende de la corrida."""
        o = "estaciones_ficha.csv"
        ruta = self.reports / o
        titulo = "Tipos de serie: cuánto pesa cada uno en lo que ve el modelo"
        lim = self.datos.get("limpieza.json")
        if not ruta.exists() or lim is None:
            self.faltantes.append(o)
            self.tabla("tipologia.md", o, None, "tbl-tipologia", titulo)
            return
        f = pd.read_csv(ruta, dtype=str)
        est = lim["estaciones"]
        sin_ficha = sorted(set(est) - set(f["estacion"]))
        if sin_ficha:
            self.problema(o, "estaciones de limpieza.json sin ficha: " + ", ".join(sin_ficha))
        gold_total = sum(x["completas_gold"] for x in est.values())
        orden = ["serie de años", "serie de meses", "campaña de semanas", "muestreo manual de 24 h"]
        filas, cif = [], {}
        for tipo in sorted(f["tipo_serie"].dropna().unique(), key=lambda t: orden.index(t) if t in orden else 99):
            ests = sorted(f.loc[f["tipo_serie"] == tipo, "estacion"])
            proc = sum(est.get(e, {}).get("largas_procesadas", 0) for e in ests)
            gold = sum(est.get(e, {}).get("completas_gold", 0) for e in ests)
            filas.append((escapar(tipo), entero(len(ests)), ", ".join(escapar(e) for e in ests), entero(proc), entero(gold),
                          pct(gold / gold_total) if gold_total else "—"))
            clave = {"serie de años": "anios", "serie de meses": "meses", "campaña de semanas": "semanas",
                     "muestreo manual de 24 h": "manual"}.get(tipo, tipo)
            cif[clave] = entero(len(ests))
            cif[f"{clave}_pct_gold"] = pct(gold / gold_total) if gold_total else "—"
        t = pd.DataFrame(filas, columns=["Tipo", "Estaciones", "Cuáles", "Lecturas válidas", "Filas del gold", "% del gold"])
        self.tabla("tipologia.md", o, {"generado_por": "notebook 04"}, "tbl-tipologia", titulo,
                   markdown(t, "lrlrrr"), anchos=[16, 10, 38, 12, 12, 12],
                   fuente="Fuente: la clasificación, de `reports/estaciones_ficha.csv` (notebook 04, procedencia del "
                          "dato); las lecturas y el gold, de `generado/limpieza.json` (`cmd/cargar`).",
                   nota="Las series de años cubren varios años de una estación; las de meses, casi todas de diciembre "
                        "de 2021 a diciembre de 2022; las campañas, unas semanas; los puntos manuales solo traen "
                        "promedios de 24 h, que el K-Means (horario) no usa.")
        self.cifras["tipologia"] = {k: str(v) for k, v in cif.items()}

    # ----------------------------------------------------------------------- umbrales de la limpieza
    def umbrales(self) -> None:
        """Los umbrales de las reglas 3, 4 y 6, tal como los lee el cargador de configs/limpieza.json."""
        ruta = RAIZ / "configs" / "limpieza.json"
        titulo = "Umbrales de la limpieza por parámetro (configs/limpieza.json)"
        if not ruta.exists():
            self.faltantes.append("configs/limpieza.json")
            self.tabla("umbrales.md", "configs/limpieza.json", None, "tbl-umbrales", titulo)
            return
        # La tabla tiene que mostrar la config con la que corrió el cargador: limpieza.json guarda su sha256.
        import hashlib
        lim = self.datos.get("limpieza.json")
        usada = (((lim or {}).get("configs") or {}).get("limpieza") or {}).get("sha256")
        actual = hashlib.sha256(ruta.read_bytes()).hexdigest()
        if lim is not None and not usada:     # sin limpieza.json ya figura como faltante
            self.desactualizadas.append("limpieza.json no registra el sha256 de configs/limpieza.json: no se puede "
                                        "comprobar que la tabla de umbrales sea la de la corrida")
        elif usada and usada != actual:
            self.desactualizadas.append("configs/limpieza.json cambió después de la corrida de limpieza.json "
                                        "(otro sha256): la tabla de umbrales no es la de la corrida")
        c = json.loads(ruta.read_text(encoding="utf-8"))
        centinelas = ", ".join(decimal(x, 0).replace("-", "\u2212") for x in c.get("centinelas", []))
        rangos, unidades = c.get("rangos", {}), c.get("unidades", {})
        neg = set(c.get("admiten_negativos", []))
        cero_h = set(c.get("cero_como_faltante", []))
        cero_d = set(c.get("bloques_en_cero_como_faltante", []))
        filas = []
        for q in sorted(set(rangos) | set(unidades), key=lambda q: list(NOMBRES).index(q) if q in NOMBRES else 99):
            r = rangos.get(q)
            us = unidades.get(q, {})
            canon = next((UNIDADES.get(u, u) for u, f in us.items() if f == 1), "—")
            otras = "; ".join(f"{UNIDADES.get(u, u)} × {decimal(f, 5).rstrip('0').rstrip(',')}" for u, f in us.items() if f != 1)
            rango = (f"[{decimal(r[0], 0)}; {decimal(r[1], 0)}]".replace("-", "−") if r else "—")
            filas.append((corto(q), rango, canon, otras or "—",
                          "sí" if q in neg else "no", "sí" if q in cero_d else "no", "sí" if q in cero_h else "no"))
        t = pd.DataFrame(filas, columns=["Parámetro", "Rango físico", "Unidad canónica", "Otras unidades (factor)",
                                         "Negativos", "Día en 0 = sin dato", "Hora en 0 = sin dato"])
        hp = c.get("hora_por_posicion", {})
        self.tabla("umbrales.md", "configs/limpieza.json", {"generado_por": "el equipo"}, "tbl-umbrales", titulo,
                   markdown(t, "llllccc"), anchos=[13, 13, 12, 26, 11, 12, 13],
                   fuente="Fuente: `configs/limpieza.json`, que `cmd/cargar` lee al arrancar (los cambios no tocan el código).",
                   nota=f"Centinelas del equipo: {centinelas}. "
                        f"Periodos horarios: {', '.join('«' + x + '»' for x in c.get('periodos_horarios', []))}; también "
                        f"«{', '.join(c.get('periodos_horarios_por_grupo', []))}» si el día tiene sus "
                        f"{hp.get('filas_por_dia', 24)} filas. Valores vacíos: "
                        f"{', '.join('«' + x + '»' for x in c.get('valores_vacios', []) if x)} y la celda vacía. Fechas "
                        f"válidas: desde el {c.get('fecha_desde')} hasta antes del {c.get('fecha_hasta')}. Hora por "
                        f"posición: {'sí' if hp.get('activar') else 'no'}, convención «{hp.get('convencion')}». "
                        f"Hora con minutos: «{c.get('hora_no_en_punto')}». El rango se compara en la unidad canónica, "
                        "después de convertir.")
        self.cifras["umbrales"] = {"centinelas": centinelas, "n_parametros": entero(len(filas)),
                                   "config_de_la_corrida": "sí" if usada == actual else "no"}

    # ----------------------------------------------------------------------- meteorología por régimen
    def meteorologia(self) -> None:
        """reports/descriptores_regimen.csv (notebook 02/03): la meteorología y la fracción fina de cada
        régimen, que no son features. Si el archivo es de otra corrida (sus horas no suman el gold), se avisa."""
        o = "descriptores_regimen.csv"
        ruta = self.reports / o
        titulo = "Meteorología y fracción fina de cada nivel (no son features: se describen)"
        km = self.datos.get("kmeans.json")
        if not ruta.exists():
            self.faltantes.append(o)
            self.tabla("meteorologia.md", f"reports/{o}", None, "tbl-meteorologia", titulo)
            return
        r = pd.read_csv(ruta)
        # Los descriptores salen de las asignaciones de un modelo: tienen que ser las del de kmeans.json. El total
        # de horas no basta (otro k u otra receta sobre el mismo gold suman lo mismo): cada régimen tiene que tener
        # el tamaño, las estaciones y los PM medios que registra regimenes.json, de la misma corrida de Go.
        rg = {x["regimen"]: x for x in (self.datos.get("regimenes.json") or {}).get("regimenes") or []}

        def igual(a, b):
            return (vacio(a) and vacio(b)) or (not vacio(a) and not vacio(b) and math.isclose(a, b, rel_tol=1e-6))

        def del_modelo(x):
            g = rg.get(int(x["regimen"]))
            return (g is not None and int(x["horas"]) == g["tamano"]
                    and int(x["estaciones"]) == sum(1 for c in (g.get("por_estacion") or {}).values() if c > 0)
                    and igual(x["pm2_5_medio"], g.get("pm2_5_medio")) and igual(x["pm10_medio"], g.get("pm10_medio")))

        if rg:
            al_dia = len(r) == len(rg) and all(del_modelo(x) for _, x in r.iterrows())
            motivo = "sus regímenes no son los de regimenes.json: tamaño, estaciones o PM medios"
        else:   # sin regimenes.json (ya figura como faltante), al menos el total de horas
            n_gold = (km or {}).get("gold", {}).get("n")
            al_dia = n_gold is None or int(r["horas"].sum()) == n_gold
            motivo = f"sus horas suman {entero(r['horas'].sum())}, el gold tiene {entero(n_gold)}"
        if not al_dia:
            self.desactualizadas.append(f"reports/descriptores_regimen.csv no es del modelo de kmeans.json ({motivo}); "
                                        f"se rehace con la §8 del notebook 02")

        def con_anomalia(fila, var, d=1):
            m, a = fila.get(f"{var}_media"), fila.get(f"{var}_anomalia_media")
            if vacio(m):
                return "—"
            return f"{decimal(m, d)} ({'+' if a >= 0 else '−'}{decimal(abs(a), d)})" if not vacio(a) else decimal(m, d)

        # Cobertura por variable: las estaciones con viento, con humedad y con temperatura pueden no ser las mismas.
        # La tabla da un número si coinciden y los tres si no; la prosa, el menor, que no sobreestima ninguna.
        meteo = ("viento_velocidad", "humedad", "temperatura")

        def n_meteo(x):
            return [int(x[f"{v}_estaciones"]) for v in meteo]

        def cobertura(x):
            ns = n_meteo(x)
            return f"{entero(ns[0]) if len(set(ns)) == 1 else ' · '.join(entero(n) for n in ns)} de {entero(x['estaciones'])}"

        filas = []
        for _, x in r.sort_values("pm10_medio").iterrows():
            filas.append((escapar(x["nombre"]), entero(x["horas"]), decimal(x["fraccion_fina_media"], 2),
                          con_anomalia(x, "viento_velocidad", 2), con_anomalia(x, "humedad"), con_anomalia(x, "temperatura"),
                          cobertura(x)))
        t = pd.DataFrame(filas, columns=["Nivel", "Horas", "Fracción fina media", "Viento (m/s)", "Humedad (%)",
                                         "Temperatura (°C)", "Estaciones con meteorología"])
        estado = "" if al_dia else (" **Ojo: el archivo no es del modelo de `kmeans.json`; "
                                    "las cifras se rehacen con la §8 del notebook 02.**")
        self.tabla("meteorologia.md", f"reports/{o}", {"generado_por": "notebook 02"}, "tbl-meteorologia", titulo,
                   markdown(t, "lrrrrrr"), anchos=[10, 11, 14, 15, 15, 17, 18],
                   fuente="Fuente: `reports/descriptores_regimen.csv` (notebooks, sobre las asignaciones de Go y el silver).",
                   nota="Entre paréntesis, la anomalía media dentro de la estación: cuánto se aparta la hora del promedio "
                        "de su propia estación, para que la comparación no sea entre lugares. La fracción fina es la media "
                        "por hora de PM2,5/PM10. La meteorología solo existe en las estaciones que la miden; las demás no "
                        "entran en esas columnas. «Estaciones con meteorología»: cuántas estaciones del nivel tienen "
                        "lecturas de cada variable en sus horas; un número si las tres coinciden y, si no, viento · "
                        "humedad · temperatura." + estado)
        por_nivel = r.sort_values("pm10_medio").reset_index(drop=True)
        cif = {"estaciones_con_meteo": entero(max(min(n_meteo(x)) for _, x in r.iterrows())),
               "estaciones": entero(r["estaciones"].max()),
               "al_dia": "sí" if al_dia else "no"}
        for i, x in por_nivel.iterrows():
            nivel = ("bajo", "medio", "alto")[i] if len(por_nivel) == 3 else str(x["regimen"])
            cif.update({f"{nivel}_fraccion_fina": decimal(x["fraccion_fina_media"], 2),
                        f"{nivel}_humedad_anomalia": con_signo(x["humedad_anomalia_media"], 1),
                        f"{nivel}_viento_anomalia": con_signo(x["viento_velocidad_anomalia_media"], 2),
                        f"{nivel}_temperatura_anomalia": con_signo(x["temperatura_anomalia_media"], 1),
                        f"{nivel}_estaciones_con_meteo": entero(min(n_meteo(x))),
                        f"{nivel}_estaciones": entero(x["estaciones"])})
        self.cifras["meteo"] = {k: str(v) for k, v in cif.items()}

    # ----------------------------------------------------------------------- estaciones y su actividad
    def estaciones_actividad(self) -> None:
        o = "estaciones_ficha.csv"
        ruta = self.reports / o
        titulo = "Las estaciones y la actividad fiscalizada junto a la que miden"
        if not ruta.exists():
            self.faltantes.append(o)
            self.tabla("estaciones_actividad.md", f"reports/{o}", None, "tbl-estaciones-actividad", titulo)
            return
        f = pd.read_csv(ruta, dtype=str)
        lim = (self.datos.get("limpieza.json") or {}).get("estaciones", {})
        # Horas de cada estación en el gold y en el nivel alto, de regimenes.json (lo que escribió Go).
        rg = (self.datos.get("regimenes.json") or {}).get("regimenes") or []
        gold_est: dict[str, int] = {}
        for x in rg:
            for e, n in (x.get("por_estacion") or {}).items():
                gold_est[e] = gold_est.get(e, 0) + n
        alto_est = (max(rg, key=lambda x: x.get("pm10_medio") or 0).get("por_estacion") or {}) if rg else {}
        # Las horas del gold por estación vienen de dos comandos (cmd/cargar y cmd/entrenar): tienen que coincidir.
        lim_gold = {e: x["completas_gold"] for e, x in lim.items() if x.get("completas_gold")}
        if rg and lim and {e: n for e, n in gold_est.items() if n} != lim_gold:
            self.desactualizadas.append("regimenes.json y limpieza.json no son del mismo gold (las horas por estación "
                                        "no coinciden): la tabla de estaciones mezclaría corridas")

        def actividades(x):
            """Todas las actividades de la estación: una por evaluación de OEFA, separadas por « | » en la ficha."""
            if not isinstance(x, str):
                return "—"
            return "; ".join(escapar(a) for a in dict.fromkeys(a.strip() for a in x.split(" | ") if a.strip())) or "—"

        filas = []
        orden = {"serie de años": 0, "serie de meses": 1, "campaña de semanas": 2, "muestreo manual de 24 h": 3}
        f = f.assign(_o=f["tipo_serie"].map(lambda t: orden.get(t, 9))).sort_values(["_o", "estacion"])
        for _, x in f.iterrows():
            e = x["estacion"]
            horas = lim.get(e, {}).get("anchas")
            gold = gold_est.get(e, 0)
            filas.append((escapar(e), actividades(x["objeto_fiscalizado"]), escapar(x.get("tipo_actividad")),
                          escapar(titulo_lugar(x.get("departamento") if isinstance(x.get("departamento"), str) else None)),
                          escapar(x.get("tipo_serie")), entero(horas) if horas is not None else "—",
                          entero(gold) if gold else "—", pct(alto_est.get(e, 0) / gold) if gold else "—"))
        t = pd.DataFrame(filas, columns=["Estación", "Actividad fiscalizada", "Rubro", "Departamento", "Serie",
                                         "Horas válidas", "Horas en el gold", "% en nivel alto"])
        self.tabla("estaciones_actividad.md", f"reports/{o}", {"generado_por": "notebook 04"}, "tbl-estaciones-actividad",
                   titulo, markdown(t, "lllllrrr"), anchos=[11, 26, 13, 10, 12, 9, 9, 10],
                   fuente="Fuente: la actividad, el rubro y la serie, de `reports/estaciones_ficha.csv` (notebook 04); "
                          "las horas válidas (filas anchas), de `generado/limpieza.json`; las del gold y las del nivel "
                          "alto, de `generado/regimenes.json` (`cmd/entrenar`).",
                   nota="La actividad sale del nombre de la evaluación de OEFA (todas, separadas por punto y coma, si la "
                        "estación tuvo más de una); el rubro, del mismo nombre, sin adivinar "
                        "cuando no lo dice. «% en nivel alto»: proporción de las horas de la estación en el gold que el "
                        "K-Means asignó al nivel alto («—» si la estación no entra al gold). El departamento va como lo "
                        "escribe el archivo, sin tildes; CA-HUAR-01 lo trae vacío.")

    # ----------------------------------------------------------------------- fragmentos de código
    # Cada fragmento: (archivo de salida, id del listado, ruta en el repo, tramos, título). Un tramo es
    # ("bloque", patrón): desde la línea del patrón hasta la llave que la cierra; ("rango", desde, hasta):
    # de la línea de un patrón a la siguiente de otro; ("firma", patrón): el comentario de documentación y
    # la línea de la firma. Se copian tal cual del código: si el código cambia, el fragmento cambia solo.
    FRAGMENTOS = [
        ("frag_cargador_pool", "lst-cargador-pool", "datos/pipeline.go",
         [("bloque", r"^\s*for range workers \{")],
         "El worker pool del cargador: cada goroutine procesa los bloques que le llegan por el canal"),
        ("frag_cargador_orden", "lst-cargador-orden", "datos/pipeline.go",
         [("bloque", r"^\s*for p := range resultados \{")],
         "El recolector: los parciales llegan en cualquier orden y se fusionan en orden de bloque"),
        ("frag_kmeans_pool", "lst-kmeans-pool", "kmeans/concurrente.go",
         [("bloque", r"^\s*for w := 0; w < op\.Workers")],
         "El worker pool del K-Means: cada worker escribe solo el parcial de su chunk"),
        ("frag_kmeans_iter", "lst-kmeans-iter", "kmeans/concurrente.go",
         [("bloque", r"^\s*for it := 1; it <= op\.MaxIter"), ("bloque", r"^// repartir manda")],
         "Una iteración: repartir los chunks, esperar la barrera y fusionar en orden de chunk"),
        ("frag_fusionar", "lst-fusionar", "kmeans/parciales.go",
         [("bloque", r"^func Fusionar\(")],
         "Fusionar: la reducción exacta, en el orden dado"),
        ("frag_firmas", "lst-firmas", "kmeans/parciales.go",
         [("firma", r"^func Parciales\("), ("firma", r"^func Actualizar\(")],
         "Las dos funciones que la versión distribuida reutiliza tal cual: Parciales y Actualizar"),
        ("frag_parcial", "lst-parcial", "protocolo/mensaje.go",
         [("rango", r"^// Los tipos de mensaje", r"^\)"), ("rango", r"// ITERAR y PARCIAL", r"Inercia\s+float64")],
         "Los tipos de mensaje del protocolo y los campos de ITERAR y PARCIAL"),
    ]

    @staticmethod
    def bloque_go(lineas: list[str], i: int) -> int:
        """Índice de la línea que cierra la llave abierta en lineas[i] (sin contar llaves en cadenas ni comentarios)."""
        prof = 0
        for j in range(i, len(lineas)):
            linea, dentro, k = lineas[j], None, 0
            while k < len(linea):
                ch = linea[k]
                if dentro:
                    if ch == "\\" and dentro != "`":
                        k += 2
                        continue
                    if ch == dentro:
                        dentro = None
                elif linea.startswith("//", k):
                    break
                elif ch in "\"'`":
                    dentro = ch
                elif ch == "{":
                    prof += 1
                elif ch == "}":
                    prof -= 1
                    if prof == 0:
                        return j
                k += 1
        raise ValueError("llave sin cerrar")

    def fragmento_pendiente(self, archivo: str, ident: str, ruta: str, titulo: str, motivo: str) -> None:
        """Si la extracción falla, el fragmento sale como PENDIENTE: no queda en el informe el de una corrida vieja."""
        texto = [f"<!-- Generado por tablas.py desde {ruta}. No editar a mano. -->", "",
                 "::: {.callout-warning title=\"PENDIENTE\"}",
                 f"{motivo}: falta este fragmento de código (se corrige `FRAGMENTOS` en `tablas.py`).",
                 ":::", "", f"::: {{#{ident}}}", "```go", "// (fragmento pendiente)", "```", "",
                 f"{titulo} (`{ruta}`)", ":::", ""]
        (self.salida / f"{archivo}.md").write_text("\n".join(texto), encoding="utf-8")

    @staticmethod
    def sin_commitear(rutas: list[str]) -> list[str]:
        """Las rutas con cambios sin commitear o sin versionar. Sin git (p. ej., desde el zip), ninguna."""
        import subprocess
        try:
            r = subprocess.run(["git", "-C", str(RAIZ), "status", "--porcelain", "--", *rutas],
                               capture_output=True, text=True, check=True)
        except (OSError, subprocess.CalledProcessError):
            return []
        return sorted({l[3:] for l in r.stdout.splitlines() if l.strip()})

    def fragmentos(self, lista: list | None = None) -> None:
        import re
        lista = self.FRAGMENTOS if lista is None else lista
        # Los fragmentos se leen del árbol de trabajo y el zip de la entrega es un commit (git archive): un
        # archivo con cambios sin commitear daría un fragmento que no está en el código entregado.
        sucios = self.sin_commitear(sorted({f[2] for f in lista}))
        if sucios:
            self.desactualizadas.append("código con cambios sin commitear (" + ", ".join(sucios) + "): sus "
                                        "fragmentos no serían los del commit que se entrega")
        for archivo, ident, ruta, tramos_def, titulo in lista:
            fuente = RAIZ / ruta
            if not fuente.exists():
                self.faltantes.append(ruta)
                self.fragmento_pendiente(archivo, ident, ruta, titulo, f"No está `{ruta}`")
                continue
            lineas = fuente.read_text(encoding="utf-8").splitlines()

            def buscar(patron, desde=0):
                return next(n for n, l in enumerate(lineas) if n >= desde and re.search(patron, l))

            tramos = []
            try:
                for t in tramos_def:
                    if t[0] == "bloque":
                        a = buscar(t[1])
                        tramos.append((a, self.bloque_go(lineas, a)))
                    elif t[0] == "rango":
                        a = buscar(t[1])
                        tramos.append((a, buscar(t[2], a)))
                    elif t[0] == "funcion":   # el comentario de documentación y el cuerpo entero
                        f = buscar(t[1])
                        c = f
                        while c > 0 and lineas[c - 1].startswith("//"):
                            c -= 1
                        tramos.append((c, self.bloque_go(lineas, f)))
                    else:   # firma: el comentario de documentación y la línea de la firma
                        f = buscar(t[1])
                        c = f
                        while c > 0 and lineas[c - 1].startswith("//"):
                            c -= 1
                        tramos.append((c, f))
            except (StopIteration, ValueError):
                self.problema(ruta, f"no se encontró el fragmento «{ident}»: ¿cambió el código?")
                self.fragmento_pendiente(archivo, ident, ruta, titulo, f"El patrón ya no aparece en `{ruta}`")
                continue
            codigo, rangos = [], []
            for a, b in tramos:
                bloque = lineas[a:b + 1]
                sangria = min((len(l) - len(l.lstrip("\t")) for l in bloque if l.strip()), default=0)
                codigo += [l[sangria:].replace("\t", "    ") for l in bloque] + [""]
                rangos.append(f"{a + 1}–{b + 1}")
            n = sum(b - a + 1 for a, b in tramos)
            if n > 30:
                self.problema(ruta, f"el fragmento «{ident}» tiene {n} líneas: ¿cambió el código?")
            lenguaje = "lua" if ruta.endswith("redis.go") and "guion" in archivo else "go"
            cuerpo = f"```{lenguaje}\n" + "\n".join(codigo).rstrip() + "\n```"
            texto = [f"<!-- Generado por tablas.py desde {ruta}. No editar a mano. -->", "",
                     f"::: {{#{ident}}}", cuerpo, "",
                     f"{titulo} (`{ruta}`, líneas {' y '.join(rangos)})", ":::", ""]
            (self.salida / f"{archivo}.md").write_text("\n".join(texto), encoding="utf-8")

    # ----------------------------------------------------------------------- calibración y auditoría
    @staticmethod
    def variante(rec: dict) -> str:
        """Código de la grilla de calibración (spec de calibración §3) para una receta."""
        f = set(rec.get("features", []))
        F = "F2" if "fraccion_fina" in f else "F1"
        met = f & {"viento_velocidad", "humedad", "temperatura", "presion"}
        M = "M0" if not met else ("M2" if met & {"temperatura", "presion"} else "M1")
        T = "T2" if rec.get("log1p") else "T1"
        N = "N2" if rec.get("nivel") in ("estacion", "por_estacion") else "N1"
        return f"{F}-{M}-{T}-{N}"

    def calibracion(self) -> None:
        """La decisión de la receta: la final frente a su versión con log1p, la que prefería la regla
        automática del notebook 02 y la provisional, con las métricas de reports/calibracion_resumen.csv."""
        o = "calibracion_resumen.csv"
        ruta = self.reports / o
        titulo = "Calibración: la receta final frente a las alternativas"
        km = self.datos.get("kmeans.json")
        if not ruta.exists() or km is None:
            self.faltantes.append(o)
            self.tabla("calibracion.md", o, None, "tbl-calibracion", titulo)
            return
        r = pd.read_csv(ruta)
        rec = km["receta"]
        final = (self.variante(rec), rec["k"])
        con_log = (final[0].replace("-T1-", "-T2-"), rec["k"])
        # La elección de la regla automática del notebook 02 está en calibracion_decision.md («Variante
        # elegida: `X` con k = N»); kmeans_propuesto.json puede ya traer la decisión del equipo.
        auto = None
        dec = self.reports / "calibracion_decision.md"
        if dec.exists():
            import re
            m = re.search(r"Variante elegida: `([^`]+)` con k = (\d+)", dec.read_text(encoding="utf-8"))
            if m:
                auto = (m.group(1), int(m.group(2)))
        prop = self.reports / "kmeans_propuesto.json"
        if auto is None and prop.exists():
            pr = json.loads(prop.read_text(encoding="utf-8"))
            auto = (self.variante(pr), pr["k"])
        provisional = ("F2-M1-T2-N1", 5)     # la receta provisional de la spec de la PC3 (§4.3)
        cols = [("Final (elegida)", final), ("Final con log1p", con_log)]
        if auto and auto != final:
            cols.append(("Regla automática", auto))
        cols.append(("Provisional", provisional))
        filas_r = {}
        for nombre_col, (v, k) in cols:
            x = r[(r["variante"] == v) & (r["k"] == k)]
            if x.empty:
                self.problema(o, f"no está la variante {v} con k = {k}")
                continue
            filas_r[nombre_col] = x.iloc[0]
        if "Final (elegida)" not in filas_r:
            self.tabla("calibracion.md", o, None, "tbl-calibracion", titulo)
            return

        def f3(x):
            return decimal(x, 3)

        filas = [
            ("Variante", lambda x: f"`{x['variante']}`"),
            ("k", lambda x: entero(x["k"])),
            ("Estaciones · filas", lambda x: f"{entero(x['n_estaciones'])} · {entero(x['n_filas'])}"),
            ("ARI entre semillas (> 0,8)", lambda x: f3(x["ari_semillas"])),
            ("ARI por bloques de días (> 0,7)", lambda x: f3(x["ari_dias"])),
            ("Pureza del régimen «excede» (> 0,6)", lambda x: f3(x["pureza"])),
            ("Recall de las horas sobre el ECA (> 0,5)", lambda x: f3(x["recall"])),
            ("NMI con la estación · línea base", lambda x: f"{f3(x['nmi_estacion'])} · {f3(x['nmi_estacion_base'])}"),
            ("NMI con la hora", lambda x: f3(x["nmi_hora"])),
            ("Cluster más chico", lambda x: pct(x["min_cluster"])),
            ("Silueta", lambda x: f3(x["silueta"])),
        ]
        t = pd.DataFrame([[n] + [fn(filas_r[c]) for c, _ in cols if c in filas_r] for n, fn in filas],
                         columns=["Criterio (umbral)"] + [c for c, _ in cols if c in filas_r])
        n_corridas = None
        crudo = self.reports / "calibracion.csv"
        if crudo.exists():
            n_corridas = sum(1 for _ in crudo.open(encoding="utf-8")) - 1
        self.tabla("calibracion.md", o, {"generado_por": "notebook 02"}, "tbl-calibracion", titulo,
                   markdown(t, "l" + "r" * (len(t.columns) - 1)),
                   anchos=[34] + [66 // (len(t.columns) - 1)] * (len(t.columns) - 1),
                   fuente=f"Fuente: `reports/calibracion_resumen.csv` (notebook 02: {entero(r['variante'].nunique())} "
                          f"variantes × k de 2 a 10 × 5 semillas{', ' + entero(n_corridas) + ' corridas' if n_corridas else ''}).",
                   nota=
                   "Umbrales entre paréntesis, los de la especificación de la calibración. La línea base de NMI es la de "
                   "partir solo por el nivel de PM10: un régimen por debajo de ella no aprendió geografía. "
                   "Pureza y recall usan la etiqueta del promedio móvil de 24 h sobre el ECA.")
        x = filas_r["Final (elegida)"]
        mismos_k = r[r["k"] >= final[1]]
        rango = int((mismos_k["silueta"] > x["silueta"]).sum()) + 1
        cif = {
            "variante": final[0], "k": entero(final[1]),
            "silueta": decimal(x["silueta"], 2), "ari_semillas": decimal(x["ari_semillas"], 2),
            "ari_dias": decimal(x["ari_dias"], 2), "pureza": decimal(x["pureza"], 2),
            "recall": pct(x["recall"], 0), "nmi_estacion": decimal(x["nmi_estacion"], 3),
            "nmi_base": decimal(x["nmi_estacion_base"], 3), "estaciones": entero(x["n_estaciones"]),
            "silueta_frase": (f"la silueta más alta de la grilla con k ≥ {final[1]}" if rango == 1
                              else f"la {rango}.ª silueta más alta de la grilla con k ≥ {final[1]}"),
            "n_variantes": entero(r["variante"].nunique()),
            "n_corridas": entero(n_corridas) if n_corridas else "—",
        }
        if "Final con log1p" in filas_r:
            y = filas_r["Final con log1p"]
            cif.update({"log_pureza": decimal(y["pureza"], 2), "log_recall": pct(y["recall"], 0)})
        if "Provisional" in filas_r:
            y = filas_r["Provisional"]
            cif.update({"prov_ari_semillas": decimal(y["ari_semillas"], 2), "prov_nmi_estacion": decimal(y["nmi_estacion"], 3),
                        "prov_nmi_base": decimal(y["nmi_estacion_base"], 3), "prov_estaciones": entero(y["n_estaciones"])})
        if "Regla automática" in filas_r:
            y = filas_r["Regla automática"]
            cif.update({"auto_variante": y["variante"], "auto_k": entero(y["k"]),
                        "auto_silueta": decimal(y["silueta"], 2), "auto_recall": pct(y["recall"], 0)})
        self.cifras["calibracion"] = {k: str(v) for k, v in cif.items()}

    def auditoria(self) -> None:
        """Go contra pandas y scikit-learn (notebook 03). Si la auditoría corrió con otra receta, lo dice."""
        o = "auditoria.json"
        d = self.leer(o)
        titulo = "Auditoría de las salidas de Go contra una segunda implementación"
        if d is None:
            self.tabla("auditoria.md", o, None, "tbl-auditoria", titulo)
            return
        ch = d.get("chequeos", [])
        cuenta = {e: sum(1 for c in ch if c.get("estado") == e) for e in ("OK", "DIFERENCIA", "AVISO", "OMITIDO")}
        if cuenta["DIFERENCIA"]:
            self.problema(o, f"{cuenta['DIFERENCIA']} chequeos con DIFERENCIA (bug en Go o en Python)")
        rec_km = (self.datos.get("kmeans.json") or {}).get("receta") or {}
        claves = ("features", "log1p", "nivel", "k")
        misma = all(d.get("receta", {}).get(c) == rec_km.get(c) for c in claves) if rec_km else False
        if not misma:
            self.desactualizadas.append("la auditoría (auditoria.json) corrió con otra receta que la de kmeans.json")
        # Misma receta no basta: el gold tiene que ser el mismo (una limpieza nueva cambia el gold).
        sha = ((self.datos.get("kmeans.json") or {}).get("gold") or {}).get("sha256", "")[:12]
        det = " ".join(c.get("detalle", "") for c in ch if "gold" in c.get("chequeo", "") and "entren" in c.get("chequeo", ""))
        mismo_gold = bool(sha) and sha in det
        if misma and not mismo_gold:
            self.desactualizadas.append("la auditoría (auditoria.json) corrió sobre otro gold que el de kmeans.json")
        misma = misma and mismo_gold
        t = pd.DataFrame([(c["chequeo"], c["estado"], c.get("detalle", "")) for c in ch],
                         columns=["Chequeo", "Estado", "Detalle"])
        self.tabla("auditoria.md", o, {**d, "generado_por": "notebook 03"}, "tbl-auditoria", titulo,
                   markdown(t, "lll"), anchos=[30, 12, 58],
                   fuente=f"Fuente: `generado/auditoria.json` (notebook 03, {d.get('fecha', '¿?')}).", nota=
                   f"OK: {cuenta['OK']}; "
                   f"DIFERENCIA (bug): {cuenta['DIFERENCIA']}; AVISO: {cuenta['AVISO']}; OMITIDO: {cuenta['OMITIDO']}. "
                   + ("" if misma else "La auditoría no corrió sobre la misma receta y el mismo gold que el modelo "
                      "de `kmeans.json`: valida el código de la limpieza, del gold y del K-Means sobre una corrida "
                      "anterior, no las cifras de esta."))
        self.cifras["auditoria"] = {k: str(v) for k, v in {
            "ok": entero(cuenta["OK"]), "diferencias": entero(cuenta["DIFERENCIA"]),
            "avisos": entero(cuenta["AVISO"]), "chequeos": entero(len(ch)),
            "misma_receta": "sí" if misma else "no",
        }.items()}

    # ----------------------------------------------------------------------- estado y cifras
    # ======================================================================= PC4
    # Lo del Entregable 2 se lee directo de reports/ y del repositorio (el contrato, los ejemplos de
    # datos, el seed, el código): no se copia a generado/. Todo lo que falte o no cuadre queda en el
    # ámbito "pc4" (ver __init__) y sale en generado/tablas/_estado_pc4.md.
    PENDIENTE = "[PENDIENTE]"

    def leer_reports(self, relativa: str) -> dict | None:
        ruta = self.reports / relativa
        if not ruta.exists():
            self.faltantes.append(f"reports/{relativa}")
            return None
        return json.loads(ruta.read_text(encoding="utf-8"))

    def pendientes(self, grupo: str, claves: list[str]) -> None:
        """Cifras de un archivo que todavía no existe: la prosa las muestra como [PENDIENTE], a la vista."""
        self.cifras[grupo] = {k: self.PENDIENTE for k in claves}

    def gold_del_modelo(self) -> str | None:
        return ((self.datos.get("kmeans.json") or {}).get("gold") or {}).get("sha256")

    # ----------------------------------------------------------------------- distribución
    def smoke(self) -> None:
        """reports/smoke_lab_N.json (cmd/smoke): la barrera de una ronda con N nodos del laboratorio."""
        o = "reports/smoke_lab_*.json"
        titulo = "La barrera de una ronda medida en el laboratorio: rondas de latido con todos los nodos a la vez"
        claves = ["nodos", "nodos_lista", "rondas", "workers", "fecha", "p50", "p95", "max", "latido_p50_min",
                  "latido_p50_max", "equipos", "p50_menor", "nodos_menor"]
        rutas = sorted(self.reports.glob("smoke_lab_*.json"))
        if not rutas:
            self.faltantes.append(o)
            self.tabla("smoke_lab.md", o, None, "tbl-smoke", titulo)
            self.pendientes("smoke", claves)
            return
        corridas = []
        for ruta in rutas:
            d = json.loads(ruta.read_text(encoding="utf-8"))
            nombre_json = f"reports/{ruta.name}"
            nodos, b = d.get("nodos", []), d.get("barrera", {})
            self.comprobar(nombre_json, bool(d.get("ok")), "el smoke test no terminó bien (ok = false)")
            self.comprobar(nombre_json, d.get("nodos_conectados") == d.get("nodos_esperados") == len(nodos),
                           "los nodos conectados no son los esperados")
            self.comprobar(nombre_json, b.get("n") == d.get("rondas") and all(n["latido"]["n"] == d.get("rondas") for n in nodos),
                           "no todos los nodos respondieron todas las rondas")
            # La barrera de una ronda espera al último PONG: ningún percentil suyo puede ser menor que
            # el del latido de un nodo.
            self.comprobar(nombre_json, all(b.get(q, 0) >= n["latido"][q] - 1e-9 for n in nodos for q in ("p50_ms", "p95_ms", "max_ms")),
                           "la barrera es menor que el latido de algún nodo")
            self.comprobar(nombre_json, len({n["nodo"] for n in nodos}) == len(nodos), "hay nombres de nodo repetidos")
            corridas.append((len(nodos), d))
        corridas.sort(key=lambda c: c[0])
        self.comprobar(o, len({n for n, _ in corridas}) == len(corridas), "hay dos corridas con el mismo número de nodos")

        def latidos(d):
            ps = [n["latido"]["p50_ms"] for n in d["nodos"]]
            return f"{tiempo(min(ps))}–{tiempo(max(ps))}"

        t = pd.DataFrame([(
            entero(n), entero(d["rondas"]), tiempo(d["barrera"]["p50_ms"]), tiempo(d["barrera"]["p95_ms"]),
            tiempo(d["barrera"]["max_ms"]), latidos(d), lista([x["workers"] for x in d["nodos"]]), fecha(d.get("fecha")),
        ) for n, d in corridas], columns=["Nodos", "Rondas", "Barrera p50 (ms)", "Barrera p95 (ms)", "Barrera máx. (ms)",
                                           "Latido p50 por nodo (ms)", "Workers por nodo", "Fecha"])
        n_max, mayor = corridas[-1]
        n_min, menor = corridas[0]
        equipos = sorted({x["nodo"] for _, d in corridas for x in d["nodos"]})
        redes = sorted({".".join(x["direccion"].split(".")[:3]) for _, d in corridas for x in d["nodos"]})
        self.tabla("smoke_lab.md", o, mayor, "tbl-smoke", titulo, markdown(t, "rrrrrrrl"), anchos=[10, 10, 12, 12, 12, 16, 12, 16],
                   fuente="Fuente: " + ", ".join(f"`reports/{r.name}`" for r in rutas) + ", escritos por `cmd/smoke`.",
                   nota="Cada ronda manda un `PING` a todos los nodos a la vez (`protocolo.Latido`) y espera el último `PONG`: es la "
                        "barrera de una iteración sin el cómputo del nodo. «Latido p50 por nodo» es el menor y el mayor entre los "
                        f"nodos de la corrida. Nodos: los equipos {', '.join(f'`{e}`' for e in equipos)} del laboratorio "
                        f"(red {', '.join(r + '.0/24' for r in redes)}), con el coordinador en la laptop de desarrollo.")
        ps = [x["latido"]["p50_ms"] for x in mayor["nodos"]]
        self.cifras["smoke"] = {
            "nodos": entero(n_max), "nodos_lista": lista([n for n, _ in corridas]), "rondas": entero(mayor["rondas"]),
            "workers": lista([x["workers"] for x in mayor["nodos"]]), "fecha": fecha(mayor.get("fecha")),
            "p50": tiempo(mayor["barrera"]["p50_ms"]), "p95": tiempo(mayor["barrera"]["p95_ms"]),
            "max": tiempo(mayor["barrera"]["max_ms"]), "latido_p50_min": tiempo(min(ps)), "latido_p50_max": tiempo(max(ps)),
            "equipos": entero(len(equipos)), "p50_menor": tiempo(menor["barrera"]["p50_ms"]), "nodos_menor": entero(n_min),
        }

    @staticmethod
    def es_loopback(d: dict) -> bool:
        return all(str(n.get("direccion", "")).startswith(("127.", "[::1]", "localhost")) for n in d.get("nodos", []))

    def distribuido(self) -> None:
        """reports/kmeans_distribuido_*.json (cmd/distribuido): el K-Means sobre el clúster frente a cmd/entrenar."""
        o = "reports/kmeans_distribuido_*.json"
        titulo = "K-Means distribuido frente al de una sola máquina: mismo gold, misma semilla y mismo arranque"
        claves = ["corridas", "nodos_lista", "nodos_max", "iteraciones", "dif_max", "bit_a_bit_con", "tolerancia", "fecha",
                  "ronda_p50_1", "ronda_p50_max", "fusion_p50_max", "replicas_max", "filas_max", "ronda_p50_replicas",
                  "nodos_replicas", "workers", "lab"]
        rutas = sorted(self.reports.glob("kmeans_distribuido_*.json"))
        if not rutas:
            self.faltantes.append(o)
            self.tabla("kmeans_distribuido.md", o, None, "tbl-distribuido", titulo)
            self.pendientes("distribuido", claves)
            return
        filas = []
        for ruta in rutas:
            d = json.loads(ruta.read_text(encoding="utf-8"))
            nj = f"reports/{ruta.name}"
            nodos, g, ref = d.get("nodos", []), d.get("gold", {}), d.get("referencia") or {}
            self.revisar_entrada(nj, g.get("ruta"))
            self.comprobar(nj, len(nodos) == d.get("nodos_esperados"), "corrió con menos nodos de los esperados")
            self.comprobar(nj, d.get("huella") == "sha256:" + str(g.get("sha256")), "la huella no es el sha256 del gold")
            if self.gold_del_modelo():
                self.comprobar(nj, g.get("sha256") == self.gold_del_modelo(), "no es el gold de kmeans.json")
            # Los rangos cubren la tabla en orden, sin huecos ni solapes (cluster.Repartir).
            desde = 0
            for n in nodos:
                self.comprobar(nj, n["rango"]["desde"] == desde and n["filas"] == n["rango"]["hasta"] - n["rango"]["desde"],
                               f"el rango de {n['nodo']} no continúa al anterior")
                desde = n["rango"]["hasta"]
            self.comprobar(nj, desde == g.get("filas_tabla") == g.get("replicas", 1) * g.get("n", 0),
                           "los rangos no cubren la tabla (réplicas × filas del gold)")
            tams = [n["filas"] for n in nodos] or [0]
            self.comprobar(nj, max(tams) - min(tams) <= 1, "los rangos difieren en más de una fila")
            self.comprobar(nj, len(d.get("ms_ronda_por_iteracion", [])) == d.get("iteraciones"),
                           "no hay un tiempo de ronda por iteración")
            self.comprobar(nj, sum(d.get("tamanos", [])) == g.get("filas_tabla"), "los tamaños de los clusters no suman la tabla")
            self.comprobar(nj, bool(ref.get("comparable")), "no se pudo comparar con reports/kmeans.json")
            self.comprobar(nj, bool(ref.get("mismas_iteraciones")) and bool(ref.get("dentro_de_tolerancia")),
                           "el modelo distribuido NO coincide con el de cmd/entrenar dentro de la tolerancia")
            if len(nodos) == 1 and g.get("replicas", 1) == 1:
                self.comprobar(nj, bool(ref.get("bit_a_bit")), "con un nodo el modelo no es el de cmd/entrenar bit a bit")
            filas.append((d, ruta.name))
        filas.sort(key=lambda f: (not self.es_loopback(f[0]), f[0]["gold"].get("replicas", 1), len(f[0]["nodos"])))

        def frente(ref):
            if ref.get("bit_a_bit"):
                return "bit a bit"
            return f"dentro de {cientifica(ref.get('tolerancia'), 0)}" if ref.get("dentro_de_tolerancia") else "FUERA DE TOLERANCIA"

        t = pd.DataFrame([(
            "loopback" if self.es_loopback(d) else "laboratorio", entero(len(d["nodos"])), entero(d["gold"].get("replicas", 1)),
            entero(d["gold"]["filas_tabla"]), entero(d["iteraciones"]), frente(d.get("referencia") or {}),
            cientifica((d.get("referencia") or {}).get("dif_max_centroides")), tiempo(d["ms_ronda"]["p50"]),
            tiempo(d["ms_fusion"]["p50"]), tiempo(d["ms"]["lloyd"]),
        ) for d, _ in filas], columns=["Dónde", "Nodos", "Réplicas", "Filas", "Iter.", "Frente a `cmd/entrenar`",
                                       "Dif. máx. de centroides", "Ronda p50 (ms)", "Fusión p50 (ms)", "Lloyd (ms)"])
        locales = [d for d, _ in filas if self.es_loopback(d)]
        lab = [d for d, _ in filas if not self.es_loopback(d)]
        nota = ("«Iter.» son las iteraciones hasta converger. «Frente a `cmd/entrenar`» compara los centroides con `reports/kmeans.json`: bit a bit, o la mayor diferencia "
                "absoluta en el espacio estandarizado. «Ronda» es la barrera de red de una iteración (del primer `ITERAR` al último "
                "`PARCIAL`) y «Fusión», lo que el coordinador hace después. ")
        if locales:
            m = locales[0].get("metadatos") or {}
            nota += (f"Las corridas «loopback» ({fecha(m.get('momento'))}, commit `{str(m.get('commit'))[:7]}`) tienen el coordinador "
                     f"y todos los nodos en la laptop de desarrollo ({m.get('cpus_logicas')} CPU lógicas), con "
                     f"{lista([n['workers'] for d in locales for n in d['nodos']])} workers por nodo: comprueban el protocolo de punta "
                     "a punta y el determinismo, no el rendimiento de un clúster. ")
        if not lab:
            nota += ("En `reports/` no hay corridas del K-Means distribuido sobre los equipos del laboratorio: allí se midió la "
                     "barrera con el smoke test (@tbl-smoke).")
        self.tabla("kmeans_distribuido.md", o, filas[0][0], "tbl-distribuido", titulo, markdown(t, "lrrrrlrrrr"),
                   anchos=[11, 9, 11, 10, 7, 14, 12, 9, 9, 8],
                   fuente="Fuente: " + ", ".join(f"`reports/{n}`" for _, n in filas) + ", escritos por `cmd/distribuido`.", nota=nota)
        base = [d for d in (locales or lab) if d["gold"].get("replicas", 1) == 1]
        con_r = [d for d in (locales or lab) if d["gold"].get("replicas", 1) > 1]
        uno = next((d for d in base if len(d["nodos"]) == 1), None)
        mas = max(base, key=lambda d: len(d["nodos"]), default=None)
        rep = max(con_r, key=lambda d: d["gold"]["replicas"], default=None)
        todas = [d for d, _ in filas]
        self.cifras["distribuido"] = {
            "corridas": entero(len(filas)),
            "nodos_lista": lista([len(d["nodos"]) for d in todas]),
            "nodos_max": entero(max(len(d["nodos"]) for d in todas)),
            "iteraciones": lista([d["iteraciones"] for d in todas]),
            "dif_max": cientifica(max((d.get("referencia") or {}).get("dif_max_centroides", 0) for d in todas)),
            "bit_a_bit_con": lista([len(d["nodos"]) for d in todas if (d.get("referencia") or {}).get("bit_a_bit")]),
            "tolerancia": cientifica((todas[0].get("referencia") or {}).get("tolerancia"), 0),
            "fecha": fecha((todas[0].get("metadatos") or {}).get("momento")),
            "ronda_p50_1": tiempo(uno["ms_ronda"]["p50"]) if uno else "—",
            "ronda_p50_max": tiempo(mas["ms_ronda"]["p50"]) if mas else "—",
            "fusion_p50_max": tiempo(mas["ms_fusion"]["p50"]) if mas else "—",
            "replicas_max": entero(rep["gold"]["replicas"]) if rep else "—",
            "filas_max": entero(rep["gold"]["filas_tabla"]) if rep else "—",
            "ronda_p50_replicas": tiempo(rep["ms_ronda"]["p50"]) if rep else "—",
            "nodos_replicas": entero(len(rep["nodos"])) if rep else "—",
            "workers": lista([n["workers"] for d in (locales or lab) for n in d["nodos"]]),
            "lab": "sí" if lab else "no",
        }

    def gpu(self) -> None:
        """reports/kmeans_gpu/*.json (lab/gpu/kmeans_gpu.py): el mismo K-Means en la GPU de los PC del laboratorio."""
        o = "reports/kmeans_gpu/*.json"
        titulo = "El mismo K-Means en la GPU de los equipos del laboratorio (experimento aparte, no es parte del sistema)"
        claves = ["equipos", "dispositivo", "dif_fp64", "dif_fp32", "ms_iter_1", "ms_iter_max", "replicas_max", "filas_max"]
        rutas = sorted((self.reports / "kmeans_gpu").glob("*.json"))
        if not rutas:
            self.faltantes.append(o)
            self.tabla("kmeans_gpu.md", o, None, "tbl-gpu", titulo)
            self.pendientes("gpu", claves)
            return
        ds = [json.loads(r.read_text(encoding="utf-8")) for r in rutas]
        modelo = self.datos.get("kmeans.json") or {}
        for d in ds:
            nj = f"reports/kmeans_gpu/kmeans_gpu_{d.get('equipo')}.json"
            self.comprobar(nj, bool(d.get("mismo_gold_que_go")), "no corrió con el gold de Go")
            if modelo:
                self.comprobar(nj, d.get("go", {}).get("iteraciones") == modelo.get("iteraciones")
                               and d.get("go", {}).get("inercia") == modelo.get("inercia"), "su referencia no es reports/kmeans.json")
        replicas = sorted({v["replicas"] for d in ds for v in d["velocidad"]})
        r_max = replicas[-1]

        def rango(xs, fmt=tiempo):
            xs = list(xs)
            return fmt(min(xs)) if fmt(min(xs)) == fmt(max(xs)) else f"{fmt(min(xs))}–{fmt(max(xs))}"

        def velocidad(prec, suma, r):
            return [v["ms_por_iteracion"] for d in ds for v in d["velocidad"]
                    if (v["precision"], v["suma"], v["replicas"]) == (prec, suma, r)]

        filas = []
        configs = sorted({(c["precision"], c["suma"]) for d in ds for c in d["convergencia"]}, key=lambda c: (c[0] != "float64", c[1] != "matmul"))
        for prec, suma in configs:
            cs = [c for d in ds for c in d["convergencia"] if (c["precision"], c["suma"]) == (prec, suma)]
            filas.append((prec, "producto de matrices" if suma == "matmul" else "suma atómica", lista([c["iteraciones"] for c in cs]),
                          cientifica(max(c["dif_max_centroides_go"] for c in cs)),
                          "sí" if all(c["repetible_bit_a_bit"] for c in cs) else "no",
                          rango(velocidad(prec, suma, 1)), rango(velocidad(prec, suma, r_max))))
        t = pd.DataFrame(filas, columns=["Precisión", "Suma por cluster", "Iteraciones", "Dif. máx. de centroides frente a Go",
                                         "Repetible bit a bit", "ms por iteración, ×1", f"ms por iteración, ×{entero(r_max)}"])
        equipos = sorted(d["equipo"] for d in ds)
        self.tabla("kmeans_gpu.md", o, ds[0], "tbl-gpu", titulo, markdown(t, "llrrlrr"), anchos=[13, 16, 13, 18, 12, 14, 14],
                   fuente="Fuente: " + ", ".join(f"`reports/kmeans_gpu/{r.name}`" for r in rutas) + ", escritos por `lab/gpu/kmeans_gpu.py`.",
                   nota=f"Equipos {', '.join(f'`{e}`' for e in equipos)}, cada uno con una {ds[0].get('dispositivo')} "
                        f"(PyTorch {ds[0].get('torch')}). Lloyd desde el mismo arranque que `cmd/entrenar` (semilla "
                        f"{modelo.get('semilla', '¿?')}), comparado con `reports/kmeans.json`. Los tiempos son el menor y el mayor entre "
                        f"los equipos; ×{entero(r_max)} es el gold repetido {entero(r_max)} veces "
                        f"({entero(max(v['filas'] for d in ds for v in d['velocidad']))} filas).")

        def dif(prec):
            return cientifica(max(c["dif_max_centroides_go"] for d in ds for c in d["convergencia"] if c["precision"] == prec))

        self.cifras["gpu"] = {
            "equipos": entero(len(ds)), "dispositivo": str(ds[0].get("dispositivo")), "dif_fp64": dif("float64"), "dif_fp32": dif("float32"),
            "ms_iter_1": rango(velocidad("float64", "matmul", 1)), "ms_iter_max": rango(velocidad("float64", "matmul", r_max)),
            "replicas_max": entero(r_max), "filas_max": entero(max(v["filas"] for d in ds for v in d["velocidad"])),
        }

    def mapa_gpu(self) -> None:
        """reports/mapa/lab_d56_*.json (cmd/mapa-coord, PR #52): local SGD en las GPU del laboratorio D56.

        Las corridas usan DATOS SINTÉTICOS a propósito (miden el clúster, no el aire): no se anotan como
        «sintéticos» del informe, que es la guarda contra entregar la muestra como si fueran los datos;
        la tabla y la prosa lo dicen a la vista.
        """
        from statistics import median
        o = "reports/mapa/lab_d56_*.json"
        titulo = ("Experimento de laboratorio, fuera del sistema y con DATOS SINTÉTICOS: una red convolucional entrenada con local SGD "
                  "en las GPU del laboratorio, con el coordinador en Go")
        claves = ["corridas", "nodos", "gpu", "vram_gb", "parametros", "parametros_millones", "mb_por_ronda", "mb_por_nodo", "fecha",
                  "dias", "lote", "promedio_ms", "h_menor", "h_mayor", "rondas_menor", "rondas_mayor", "barrera_s_menor",
                  "barrera_s_mayor", "computo_s_menor", "computo_s_mayor", "red_s_menor", "red_s_mayor", "pct_red_menor",
                  "pct_red_mayor", "muestras_s_menor", "muestras_s_mayor", "veces_menor", "veces_mayor", "dispersion_computo",
                  "total_min_menor", "total_min_mayor"]
        rutas = sorted((self.reports / "mapa").glob("lab_d56_*.json"))
        if not rutas:
            self.faltantes.append(o)
            self.tabla("mapa_gpu.md", o, None, "tbl-mapa-gpu", titulo)
            self.pendientes("mapa", claves)
            return
        corridas = []
        for ruta in rutas:
            d = json.loads(ruta.read_text(encoding="utf-8"))
            nj = f"reports/mapa/{ruta.name}"
            nodos, rondas, n_par, lote = d["nodos"], d["rondas"], d["parametros"], d["hiper"]["lote"]
            self.comprobar(nj, d.get("estado") == "completa" and len(rondas) == d.get("rondas_pedidas"), "la corrida no terminó todas sus rondas")
            self.comprobar(nj, all(len(r["nodos"]) == len(nodos) for r in rondas), "alguna ronda no tiene a todos los nodos")
            # Por ronda cruzan la red un PESOS y un LOCAL por nodo, de n parámetros float32 cada uno.
            self.comprobar(nj, all(r["bytes"] == 2 * len(nodos) * n_par * 4 for r in rondas), "los bytes por ronda no son 2 × nodos × parámetros × 4")
            self.comprobar(nj, all(r["muestras"] == len(nodos) * d["pasos"] * lote for r in rondas), "las muestras por ronda no son nodos × pasos × lote")
            self.comprobar(nj, all(x.get("rezagados", 0) == 0 for x in nodos), "hubo mensajes rezagados")
            self.comprobar(nj, "sint" in str(d["manifiesto"].get("fuente", "")) or "sintetico" in ruta.name,
                           "no está marcada como corrida con datos sintéticos: revisar cómo se presenta en el informe")
            # Medianas sin la primera ronda (carga los datos en la GPU y calienta) cuando hay más de una.
            rs = rondas[1:] if len(rondas) > 1 else rondas
            barrera = median(r["ms_barrera"] for r in rs) / 1000
            computo = median(max(x["ms_computo"] for x in r["nodos"]) for r in rs) / 1000
            red = median(r["ms_barrera"] - max(x["ms_computo"] for x in r["nodos"]) for r in rs) / 1000
            ritmo = median(r["muestras"] / ((r["ms_barrera"] + r["ms_promedio"]) / 1000) for r in rs)
            # Lo que rinde un nodo mientras calcula: sus muestras de la ronda sobre su propio cómputo.
            ritmo_nodo = median(x["muestras"] / (x["ms_computo"] / 1000) for r in rs for x in r["nodos"])
            cs = [median(x["ms_computo"] for r in rs for x in r["nodos"] if x["nodo"] == n["nodo"]) for n in nodos]
            corridas.append({"d": d, "h": d["pasos"], "rondas": len(rondas), "nodos": len(nodos), "barrera": barrera, "computo": computo,
                             "red": red, "ritmo": ritmo, "veces": ritmo / ritmo_nodo, "promedio": median(r["ms_promedio"] for r in rs),
                             "dispersion": (max(cs) - min(cs)) / min(cs), "perdida": (rondas[0]["perdida"], rondas[-1]["perdida"])})
        corridas.sort(key=lambda c: c["h"])
        d0 = corridas[0]["d"]
        self.comprobar(o, len({(c["nodos"], c["d"]["parametros"], c["d"]["manifiesto"]["huella"]) for c in corridas}) == 1,
                       "las corridas no son comparables (otros nodos, otra red u otros datos)")
        t = pd.DataFrame([(
            entero(c["nodos"]), entero(c["h"]), entero(c["rondas"]), entero(c["d"]["rondas"][0]["muestras"]), decimal(c["barrera"], 1),
            decimal(c["computo"], 1), f"{decimal(c['red'], 1)} ({pct(c['red'] / c['barrera'], 1)})", entero(c["ritmo"]),
            f"{decimal(c['veces'], 1)}×", decimal(c["d"]["ms"]["total"] / 60000, 1),
        ) for c in corridas], columns=["Nodos", "H (pasos por ronda)", "Rondas", "Muestras por ronda", "Barrera (s)",
                                       "Cómputo (s)", "Red y espera (s)", "Muestras por s",
                                       "Frente a un nodo", "Total (min)"])
        gpus = sorted({x["gpu"] for c in corridas for x in c["d"]["nodos"]})
        self.tabla("mapa_gpu.md", o, d0, "tbl-mapa-gpu", titulo, markdown(t, "rrrrrrrrrr"), anchos=[9, 10, 10, 11, 10, 10, 12, 11, 9, 8],
                   fuente="Fuente: " + ", ".join(f"`reports/mapa/{r.name}`" for r in rutas) + f", escritos por `cmd/mapa-coord` "
                          f"({fecha(d0['metadatos'].get('momento'))}); el código está en la PR #52, sin fusionar.",
                   nota=f"**Datos sintéticos** de tamaño nacional ({entero(d0['manifiesto']['dias'])} días): las corridas miden el clúster, no "
                        f"el aire. Nodos: {entero(corridas[0]['nodos'])} equipos del laboratorio D56, cada uno con una {', '.join(gpus)}, "
                        f"que entrenan en Python con PyTorch una red de {entero(d0['parametros'])} parámetros (lote {d0['hiper']['lote']}). "
                        "Por ronda, cada nodo recibe los pesos (`PESOS`), entrena H pasos con sus datos y los devuelve (`LOCAL`); el coordinador "
                        "espera a todos (la barrera) y promedia en orden de nodo. Barrera, cómputo y red son medianas por ronda, sin la primera. "
                        "«Cómputo» es el del nodo más lento de la ronda; «Red y espera», la barrera menos ese cómputo, con su porcentaje "
                        "sobre la barrera. «Frente a un nodo» divide las muestras por segundo del clúster entre las de un nodo mientras calcula (sus muestras sobre su propio "
                        "cómputo).")
        a, b = corridas[0], corridas[-1]
        self.cifras["mapa"] = {
            "corridas": entero(len(corridas)), "nodos": entero(a["nodos"]), "gpu": ", ".join(gpus),
            "vram_gb": lista([x["vram_gb"] for x in d0["nodos"]]), "parametros": entero(d0["parametros"]),
            "parametros_millones": decimal(d0["parametros"] / 1e6, 1), "mb_por_ronda": decimal(d0["rondas"][0]["bytes"] / 1e6, 0),
            "mb_por_nodo": decimal(d0["parametros"] * 4 / 1e6, 0), "fecha": fecha(d0["metadatos"].get("momento")),
            "dias": entero(d0["manifiesto"]["dias"]), "lote": entero(d0["hiper"]["lote"]), "promedio_ms": decimal(a["promedio"], 0),
            "h_menor": entero(a["h"]), "h_mayor": entero(b["h"]), "rondas_menor": entero(a["rondas"]), "rondas_mayor": entero(b["rondas"]),
            "barrera_s_menor": decimal(a["barrera"], 1), "barrera_s_mayor": decimal(b["barrera"], 1),
            "computo_s_menor": decimal(a["computo"], 1), "computo_s_mayor": decimal(b["computo"], 1),
            "red_s_menor": decimal(a["red"], 1), "red_s_mayor": decimal(b["red"], 1),
            "pct_red_menor": pct(a["red"] / a["barrera"], 0), "pct_red_mayor": pct(b["red"] / b["barrera"], 1),
            "muestras_s_menor": entero(a["ritmo"]), "muestras_s_mayor": entero(b["ritmo"]),
            "veces_menor": decimal(a["veces"], 1), "veces_mayor": decimal(b["veces"], 1),
            "dispersion_computo": pct(a["dispersion"], 1),
            "total_min_menor": decimal(a["d"]["ms"]["total"] / 60000, 1), "total_min_mayor": decimal(b["d"]["ms"]["total"] / 60000, 1),
        }

    # Salidas en texto de corridas reales (docs/informe/capturas/pc4/*.txt, de la sesión de evidencias):
    # (archivo .md, id, .txt, tramos, título). Un tramo es ("orden", patrón, n): el bloque que empieza en la
    # línea «$ …» que cumple el patrón, hasta n líneas; o ("cola", n): las últimas n líneas. Con "tabla" al
    # final, la salida es una tabla de columnas alineadas más ancha que la página: se le quita un espacio
    # a cada separación de columnas, en todas las filas por igual, hasta que quepa.
    SALIDAS = [
        ("salida_compose", "lst-salida-compose", "compose-ps.txt", [("orden", r"compose .* ps", 10, "tabla"), ("orden", r"api/salud", 8)],
         "`docker compose up --scale nodo=3`: los servicios sanos, los tres nodos y el seed terminado"),
        ("salida_nodos", "lst-salida-nodos", "entrenamiento.txt", [("orden", r"api/cluster/nodos", 9)],
         "`GET /api/cluster/nodos`: los nodos del pool tal como los ve el coordinador (primeras líneas)"),
        ("salida_entrenamiento", "lst-salida-entrenamiento", "entrenamiento.txt",
         [("orden", r"-X POST localhost:8080/api/entrenamientos", 4), ("orden", r"api/entrenamientos/e-", 28)],
         "Un entrenamiento distribuido por la API: el 202 con el id y el detalle al terminar (primeras líneas)"),
        ("salida_carga", "lst-salida-carga", "carga.txt", [("cola", 14, "tabla")],
         "`cmd/carga`: latencia de la predicción con 1, 4, 16 y 64 clientes a la vez"),
        ("salida_newman", "lst-salida-newman", "newman-coleccion.txt", [("cola", 20)],
         "`newman` sobre las carpetas 1 a 3 de la colección de Postman: resumen de la corrida"),
        ("salida_newman_csv", "lst-salida-newman-csv", "newman-runner-csv.txt", [("cola", 20)],
         "`newman` con los datos de prueba (`lecturas_prueba.csv`): resumen de la corrida"),
        ("salida_mongo", "lst-salida-mongo", "mongo.txt",
         [("orden", r"countDocuments", 7), ("orden", r"db\.predicciones\.find", 21)],
         "MongoDB después de las pruebas: documentos por colección y la última predicción de una estación"),
        ("salida_mongo_modelos", "lst-salida-mongo-modelos", "mongo.txt", [("orden", r"db\.modelos\.find\(\{\}", 28)],
         "MongoDB: los modelos guardados y cuál está activo"),
        ("salida_redis", "lst-salida-redis", "redis.txt",
         [("orden", r"--scan --pattern modelo", 3), ("orden", r"--scan --pattern entrenamiento", 5),
          ("orden", r"GET estacion:CA-HU-04", 3), ("orden", r"GET entrenamiento:", 3)],
         "Redis después de las pruebas: las claves, el estado de una estación y un punto de control"),
        ("salida_metricas", "lst-salida-metricas", "metricas.txt", [("orden", r"api/metricas", 26)],
         "`GET /api/metricas`: percentiles por ruta medidos dentro del servidor (primeras líneas)"),
    ]

    def salidas(self) -> None:
        import re
        carpeta = AQUI / "capturas" / "pc4"
        ancho = 86

        def juntar(bloque):
            """Estrecha una tabla de texto: cada tramo de dos o más espacios pierde uno, en todas las filas."""
            for _ in range(8):
                # Solo cuentan las filas de la tabla (las que tienen columnas separadas por espacios).
                if max((len(l) for l in bloque if not l.startswith("$ ") and re.search(r"\S {2,}\S", l)), default=0) <= ancho:
                    break
                nuevo = [l if l.startswith("$ ") else re.sub(r" {2,}", lambda m: m.group(0)[1:], l) for l in bloque]
                if nuevo == bloque:
                    break
                bloque = nuevo
            return bloque

        for archivo, ident, txt, tramos, titulo in self.SALIDAS:
            ruta, destino = carpeta / txt, self.salida / f"{archivo}.md"
            origen = f"docs/informe/capturas/pc4/{txt}"
            if not ruta.exists():
                self.faltantes.append(origen)
                destino.write_text("\n".join([
                    f"<!-- Generado por tablas.py desde {origen}. No editar a mano. -->", "",
                    "::: {.callout-warning title=\"PENDIENTE\"}",
                    f"Falta `{origen}`: la salida en texto de la corrida real, que llega con las evidencias de la PC4.", ":::", "",
                    f"::: {{#{ident}}}", "```text", "(salida pendiente)", "```", "", titulo, ":::", ""]), encoding="utf-8")
                continue
            lineas = ruta.read_text(encoding="utf-8").splitlines()
            cuerpo = []
            for t in tramos:
                if t[0] == "cola":
                    bloque = [l for l in lineas if l.strip()][-t[1]:]
                else:
                    ini = next((i for i, l in enumerate(lineas) if l.startswith("$ ") and re.search(t[1], l)), None)
                    if ini is None:
                        self.problema(origen, f"no se encontró la orden «{t[1]}»: ¿cambió la salida?")
                        continue
                    fin = next((i for i in range(ini + 1, len(lineas)) if lineas[i].startswith("$ ")), len(lineas))
                    bloque = [l for l in lineas[ini:fin] if l.strip()]
                    if len(bloque) > t[2]:
                        bloque = bloque[:t[2]] + ["…"]
                if t[-1] == "tabla":
                    bloque = juntar(bloque)
                # Una línea más ancha que la página se parte con «\» al final, como en la terminal.
                for l in bloque:
                    while len(l) > ancho:
                        corte = l.rfind(" ", 0, ancho)
                        corte = corte if corte > ancho // 2 else ancho
                        cuerpo.append(l[:corte] + " \\")
                        l = "    " + l[corte:].lstrip()
                    cuerpo.append(l)
                cuerpo.append("")
            destino.write_text("\n".join([
                f"<!-- Generado por tablas.py desde {origen}. No editar a mano. -->", "",
                f"::: {{#{ident}}}", "```text", "\n".join(cuerpo).rstrip(), "```", "",
                f"{titulo} (salida real, `{origen}`)", ":::", ""]), encoding="utf-8")

    # ----------------------------------------------------------------------- API
    def alerta(self) -> None:
        """reports/alerta_24h.json (go test ./ventana -actualizar): por qué la alerta sale de la media de 24 h."""
        o = "reports/alerta_24h.json"
        titulo = "Qué acertaría cada señal como alerta, sobre las horas del gold con media de 24 horas calculable"
        claves = ["horas_calculables", "horas_sobre_eca", "pct_sobre_eca", "regimen_horas", "regimen_precision", "regimen_recall",
                  "horaria_horas", "horaria_precision", "horaria_recall", "minimo_lecturas", "ventana_horas", "eca_pm2_5", "eca_pm10",
                  "inca_buena", "inca_moderada", "inca_mala", "inca_umbral"]
        d = self.leer_reports("alerta_24h.json")
        if d is None:
            self.tabla("alerta_24h.md", o, None, "tbl-alerta", titulo)
            self.pendientes("alerta", claves)
            return
        calc, sobre = d["horas_calculables"], d["horas_sobre_eca"]
        alto, hor = d.get("regimen_alto") or {}, d["lectura_horaria"]
        if self.gold_del_modelo():
            self.comprobar(o, d["gold"]["sha256"] == self.gold_del_modelo(), "no se calculó sobre el gold de kmeans.json")
        self.comprobar(o, sum(d["inca"].values()) == calc, "las bandas del INCA no suman las horas calculables")
        self.comprobar(o, 0 < sobre <= calc <= d["gold"]["filas"], "horas sobre el ECA, calculables y del gold fuera de orden")
        for nombre_s, s_ in (("régimen alto", alto), ("lectura horaria", hor)):
            self.comprobar(o, bool(s_) and 0 < s_["aciertos"] <= min(s_["horas"], sobre), f"los aciertos de «{nombre_s}» no cuadran")
        regs = self.datos.get("regimenes.json") or {}
        exceden = [r.get("cluster", r.get("regimen")) for r in regs.get("regimenes", []) if r.get("eca") == "excede"]
        if exceden and alto:
            self.comprobar(o, exceden == [alto.get("regimen")], "el régimen alto no es el que excede el ECA en regimenes.json")

        def fila(nombre_s, s_):
            return (nombre_s, entero(s_["horas"]), entero(s_["aciertos"]), pct(s_["aciertos"] / s_["horas"], 0), pct(s_["aciertos"] / sobre, 0))

        t = pd.DataFrame([
            fila("La lectura cae en el régimen alto del K-Means", alto) if alto else ("El régimen alto", "—", "—", "—", "—"),
            fila("La lectura horaria supera el umbral del ECA", hor),
            ("La media de 24 h de la estación supera el ECA (la regla de la API)", entero(sobre), entero(sobre), "100 %", "100 %"),
        ], columns=["Señal", "Horas con la señal", "De esas, con la media de 24 h sobre el ECA", "Precisión", "Recall"])
        self.tabla("alerta_24h.md", o, d, "tbl-alerta", titulo, markdown(t, "lrrrr"), anchos=[40, 14, 22, 12, 12],
                   fuente="Fuente: `reports/alerta_24h.json`, escrito por `go test ./ventana -run LaVentanaSobreElGold -actualizar`.",
                   nota=f"De las {entero(d['gold']['filas'])} horas del gold, {entero(calc)} tienen al menos {d['minimo_lecturas']} "
                        f"lecturas de su estación en las {d['ventana_horas']} horas que terminan en ella; en {entero(sobre)} "
                        f"({pct(sobre / calc)}) esa media supera el ECA (PM2,5 > {decimal(d['eca_pm2_5'], 0)} o PM10 > "
                        f"{decimal(d['eca_pm10'], 0)} µg/m³). Precisión: de las horas con la señal, cuántas tienen la media sobre el "
                        "ECA. Recall: de las horas con la media sobre el ECA, cuántas tienen la señal. La última fila es la regla "
                        "misma, no una medición.")
        self.cifras["alerta"] = {
            "horas_calculables": entero(calc), "horas_sobre_eca": entero(sobre), "pct_sobre_eca": pct(sobre / calc),
            "regimen_horas": entero(alto.get("horas")), "regimen_precision": pct(alto["aciertos"] / alto["horas"], 0) if alto else "—",
            "regimen_recall": pct(alto["aciertos"] / sobre, 0) if alto else "—",
            "horaria_horas": entero(hor["horas"]), "horaria_precision": pct(hor["aciertos"] / hor["horas"], 0),
            "horaria_recall": pct(hor["aciertos"] / sobre, 0),
            "minimo_lecturas": entero(d["minimo_lecturas"]), "ventana_horas": entero(d["ventana_horas"]),
            "eca_pm2_5": decimal(d["eca_pm2_5"], 0), "eca_pm10": decimal(d["eca_pm10"], 0),
            "inca_buena": entero(d["inca"].get("buena", 0)), "inca_moderada": entero(d["inca"].get("moderada", 0)),
            "inca_mala": entero(d["inca"].get("mala", 0)), "inca_umbral": entero(d["inca"].get("umbral_de_cuidado", 0)),
        }

    def api_latencia(self) -> None:
        """reports/api_latencia.json (cmd/carga): latencia de POST /api/predicciones por nivel de concurrencia."""
        o = "reports/api_latencia.json"
        titulo = "Latencia de `POST /api/predicciones` medida del lado del cliente, por nivel de concurrencia"
        claves = ["clientes_lista", "clientes_max", "pedidos_por_nivel", "pedidos_total", "limite_ms", "p50_1", "p99_1", "p50_max",
                  "p95_max", "p99_max", "p99_peor", "max_peor", "clientes_peor", "errores", "sobre_el_limite", "pedidos_por_s_max",
                  "lecturas", "cumple", "fecha"]
        d = self.leer_reports("api_latencia.json")
        if d is None:
            self.tabla("api_latencia.md", o, None, "tbl-api-latencia", titulo)
            self.pendientes("api_latencia", claves)
            return
        par, niveles = d.get("parametros", {}), sorted(d.get("niveles", []), key=lambda n: n["clientes"])
        limite = par.get("limite_ms", 100)
        self.comprobar(o, bool(niveles), "no trae ningún nivel de concurrencia")
        if not niveles:
            self.tabla("api_latencia.md", o, None, "tbl-api-latencia", titulo)
            self.pendientes("api_latencia", claves)
            return
        for n in niveles:
            c = n["clientes"]
            self.comprobar(o, n["p50_ms"] <= n["p95_ms"] <= n["p99_ms"] <= n["max_ms"], f"los percentiles con {c} clientes no están en orden")
            self.comprobar(o, n.get("errores", 0) == 0, f"hubo {n.get('errores')} pedidos con error con {c} clientes")
            self.comprobar(o, n.get("respuestas_distintas", 0) == 0, f"{n.get('respuestas_distintas')} respuestas con otro régimen con {c} clientes")
            self.comprobar(o, n["p99_ms"] < limite, f"el p99 con {c} clientes ({tiempo(n['p99_ms'])} ms) no baja de {limite} ms")
        self.comprobar(o, bool(d.get("cumple")) == all(n.get("cumple") for n in niveles), "«cumple» no es el de todos los niveles")
        if par.get("ruta"):
            self.comprobar(o, "/api/predicciones" in str(par["ruta"]), "no midió POST /api/predicciones")
        t = pd.DataFrame([(
            entero(n["clientes"]), entero(n["pedidos"]), entero(n.get("errores", 0)), tiempo(n["p50_ms"]), tiempo(n["p95_ms"]),
            tiempo(n["p99_ms"]), tiempo(n["max_ms"]), entero(n.get("pedidos_por_segundo")), entero(n.get("sobre_el_limite", 0)),
        ) for n in niveles], columns=["Clientes a la vez", "Pedidos", "Errores", "p50 (ms)", "p95 (ms)", "p99 (ms)", "Máx. (ms)",
                                       "Pedidos por segundo", f"Pedidos de {limite} ms o más"])
        m = d.get("metadatos") or {}
        self.tabla("api_latencia.md", o, d, "tbl-api-latencia", titulo, markdown(t, "rrrrrrrrr"), anchos=[11, 11, 11, 10, 10, 10, 10, 13, 14],
                   fuente=f"Fuente: `reports/api_latencia.json`, escrito por `cmd/carga` ({fecha(m.get('momento'))}, commit "
                          f"`{str(m.get('commit'))[:7]}`).",
                   nota=f"Contra `{par.get('url', '¿?')}`, con las {entero(par.get('filas'))} lecturas de `{par.get('lecturas', '¿?')}` "
                        f"en ronda y {entero(par.get('calentamiento'))} pedidos de calentamiento por nivel, que no se miden. "
                        "La latencia de un pedido va desde antes de enviarlo hasta tener el cuerpo de la respuesta: incluye la red, el "
                        f"JWT y el JSON. {str(d.get('medicion') or '').strip()}".strip())
        primero, ultimo = niveles[0], niveles[-1]
        peor = max(niveles, key=lambda n: n["p99_ms"])
        self.cifras["api_latencia"] = {
            "clientes_lista": lista([n["clientes"] for n in niveles]), "clientes_max": entero(ultimo["clientes"]),
            "pedidos_por_nivel": entero(par.get("pedidos_por_nivel", primero["pedidos"])),
            "pedidos_total": entero(sum(n["pedidos"] for n in niveles)), "limite_ms": entero(limite),
            "p50_1": tiempo(primero["p50_ms"]), "p99_1": tiempo(primero["p99_ms"]), "p50_max": tiempo(ultimo["p50_ms"]),
            "p95_max": tiempo(ultimo["p95_ms"]), "p99_max": tiempo(ultimo["p99_ms"]), "p99_peor": tiempo(peor["p99_ms"]),
            "max_peor": tiempo(max(n["max_ms"] for n in niveles)), "clientes_peor": entero(peor["clientes"]),
            "errores": entero(sum(n.get("errores", 0) for n in niveles)),
            "sobre_el_limite": entero(sum(n.get("sobre_el_limite", 0) for n in niveles)),
            "pedidos_por_s_max": entero(max(n.get("pedidos_por_segundo", 0) for n in niveles)),
            "lecturas": entero(par.get("filas")), "cumple": "sí" if d.get("cumple") else "no", "fecha": fecha(m.get("momento")),
        }

    def api_rutas(self) -> None:
        """docs/api/openapi.yaml contra las rutas que registra api/servidor.go: el contrato es el del código."""
        import re
        o = "docs/api/openapi.yaml"
        titulo = "Rutas de la API: el contrato OpenAPI, que la API sirve en `/openapi.yaml` y Swagger UI muestra en `/docs`"
        ruta_c, ruta_s = RAIZ / o, RAIZ / "api" / "servidor.go"
        if not ruta_c.exists() or not ruta_s.exists():
            self.faltantes.append(o if not ruta_c.exists() else "api/servidor.go")
            self.tabla("api_rutas.md", o, None, "tbl-api-rutas", titulo)
            self.pendientes("api", ["rutas", "libres", "admin", "usuario"])
            return
        # El contrato está escrito a mano con dos espacios de sangría: ruta, método y sus campos.
        ops, camino, op = [], None, None
        for linea in ruta_c.read_text(encoding="utf-8").splitlines():
            if linea.startswith("components:"):
                break
            if m := re.match(r"^  (/\S*):\s*$", linea):
                camino = m.group(1)
            elif (m := re.match(r"^    (get|post|put|patch|delete):\s*$", linea)) and camino:
                op = {"metodo": m.group(1).upper(), "ruta": camino, "roles": None, "libre": False, "resumen": ""}
                ops.append(op)
            elif op is not None:
                if m := re.match(r"^      summary:\s*(.+)$", linea):
                    op["resumen"] = m.group(1).strip().strip("'\"")
                elif m := re.match(r"^      x-roles:\s*\[(.*)\]", linea):
                    op["roles"] = [r.strip() for r in m.group(1).split(",")]
                elif re.match(r"^      security:\s*\[\]", linea):
                    op["libre"] = True
        codigo = {}
        for m in re.finditer(r's\.ruta\("(\w+) (\S+)", (nil|cualquiera|\[\]string\{RolAdmin\})', ruta_s.read_text(encoding="utf-8")):
            codigo[(m.group(1), m.group(2))] = {"nil": None, "cualquiera": ["usuario", "admin"]}.get(m.group(3), ["admin"])
        self.comprobar(o, {(x["metodo"], x["ruta"]) for x in ops} == set(codigo),
                       "las rutas del contrato no son las que registra api/servidor.go")
        for x in ops:
            esperado = codigo.get((x["metodo"], x["ruta"]), "?")
            if x["ruta"] == "/ws":     # se autentica dentro del handler (el navegador no manda Authorization)
                continue
            self.comprobar(o, (x["roles"] or None) == esperado and x["libre"] == (esperado is None),
                           f"el acceso de {x['metodo']} {x['ruta']} en el contrato no es el del código")

        def acceso(x):
            return "libre" if not x["roles"] else " o ".join(x["roles"])

        t = pd.DataFrame([(f"`{x['metodo']} {x['ruta']}`", x["resumen"], acceso(x)) for x in ops],
                         columns=["Método y ruta", "Qué hace", "Acceso"])
        self.tabla("api_rutas.md", o, {}, "tbl-api-rutas", titulo, markdown(t, "lll"), anchos=[34, 48, 18],
                   fuente="Fuente: `docs/api/openapi.yaml` (el texto de «Qué hace» es el `summary` de cada operación; el acceso, su "
                          "`x-roles`), comprobado contra las rutas que registra `api/servidor.go`.",
                   nota="«libre»: sin token. En las demás hace falta un JWT con alguno de esos roles; sin token la respuesta es "
                        "401 y con otro rol, 403.")
        self.cifras["api"] = {
            "rutas": entero(len(ops)), "libres": entero(sum(1 for x in ops if not x["roles"])),
            "admin": entero(sum(1 for x in ops if x["roles"] == ["admin"])),
            "usuario": entero(sum(1 for x in ops if x["roles"] and "usuario" in x["roles"])),
        }

    # ----------------------------------------------------------------------- bases de datos
    COLECCIONES = [   # colección, archivo de ejemplo, qué guarda, quién la escribe (docs/datos/README.md)
        ("estaciones", "estaciones.json", "El catálogo de estaciones: el código OEFA y su ficha", "El seed, desde la ficha"),
        ("predicciones", "predicciones.json", "Una por lectura predicha: la lectura, su régimen, la media de 24 h, la alerta y el INCA",
         "La API, por lotes"),
        ("modelos", "modelos.json", "Cada modelo, con lo necesario para volver a predecir con él; a lo sumo uno activo",
         "La API al arrancar y al activar un modelo"),
        ("entrenamientos", "entrenamientos.json", "Cada corrida distribuida: parámetros, nodos con su rango, rondas y resultado",
         "El coordinador de la API"),
        ("eventos_cluster", "eventos_cluster.json", "Altas, bajas y errores de los nodos", "El coordinador de la API"),
    ]
    CLAVES = [        # clave, archivo de ejemplo, para qué
        ("estacion:{id}:regimen", "redis-estacion-regimen.json",
         "El estado de la estación en su lectura más reciente; lo lee `GET /api/estaciones/{id}/regimen`"),
        ("modelo:activo", "redis-modelo-activo.json", "El modelo activo, para arrancar con él sin esperar a MongoDB"),
        ("entrenamiento:{id}:punto", "redis-entrenamiento-punto.json",
         "Punto de control: la iteración y los centroides de la última ronda completa"),
    ]

    @staticmethod
    def campos(doc: dict, omitir=("_id",)) -> str:
        """Los campos de un documento de ejemplo; los de un subdocumento (o del primero de un arreglo), entre llaves."""
        partes = []
        for k, v in doc.items():
            if k in omitir:
                continue
            hijo = v[0] if isinstance(v, list) and v and isinstance(v[0], dict) else v
            if isinstance(hijo, dict) and hijo and k != "ms_por_nodo":
                dentro = ", ".join(hijo)
                partes.append(f"`{k}` [{{{dentro}}}]" if isinstance(v, list) else f"`{k}` {{{dentro}}}")
            else:
                partes.append(f"`{k}`")
        return ", ".join(partes)

    def entidades(self) -> None:
        """docs/datos/ejemplos/ y docs/datos/seed.js: las colecciones de MongoDB, sus índices y las claves de Redis."""
        import csv
        import re
        o = "docs/datos/ejemplos/"
        t_mongo = "Entidades de MongoDB: las cinco colecciones de la base `aire_distribuido`"
        t_redis = "Entidades de Redis: las tres familias de claves, cada una con un valor JSON"
        dir_ej, seed = RAIZ / "docs" / "datos" / "ejemplos", RAIZ / "docs" / "datos" / "seed.js"
        ejemplos = {}
        for _, archivo, *_ in self.COLECCIONES + self.CLAVES:
            if (dir_ej / archivo).exists():
                ejemplos[archivo] = json.loads((dir_ej / archivo).read_text(encoding="utf-8"))
            else:
                self.faltantes.append(f"{o}{archivo}")
        if len(ejemplos) < len(self.COLECCIONES) + len(self.CLAVES) or not seed.exists():
            if not seed.exists():
                self.faltantes.append("docs/datos/seed.js")
            self.tabla("entidades_mongo.md", o, None, "tbl-entidades-mongo", t_mongo)
            self.tabla("claves_redis.md", o, None, "tbl-claves-redis", t_redis)
            self.pendientes("datos", ["colecciones", "claves", "indices", "estaciones", "estaciones_sin_gold"])
            return
        js = seed.read_text(encoding="utf-8")
        indices = {}
        for m in re.finditer(r"database\.(\w+)\.createIndex\(\s*\{([^}]*)\},\s*\{\s*name:\s*\"(\w+)\"\s*\},?\s*\)", js, re.S):
            campos_i = ", ".join(c.split(":")[0].strip() for c in m.group(2).split(",") if c.strip())
            indices.setdefault(m.group(1), []).append(f"`{m.group(3)}` ({campos_i})")
        del_seed = re.search(r"const colecciones = \[(.*?)\];", js, re.S)
        del_seed = set(re.findall(r'"(\w+)"', del_seed.group(1))) if del_seed else set()
        self.comprobar("docs/datos/seed.js", del_seed == {c[0] for c in self.COLECCIONES},
                       "las colecciones que crea el seed no son las del informe")
        self.comprobar("docs/datos/seed.js", set(indices) <= del_seed, "hay un índice sobre una colección que el seed no crea")
        redis_go = (RAIZ / "almacen" / "redis" / "redis.go").read_text(encoding="utf-8") if (RAIZ / "almacen" / "redis" / "redis.go").exists() else ""
        for trozo in ('"modelo:activo"', '"estacion:"', '":regimen"', '"entrenamiento:"', '":punto"'):
            self.comprobar("almacen/redis/redis.go", trozo in redis_go, f"ya no arma la clave con {trozo}")
        # Un régimen de la caché es el estado de una predicción: sus campos tienen que estar en ella.
        self.comprobar(o, set(ejemplos["redis-estacion-regimen.json"]) <= set(ejemplos["predicciones.json"]),
                       "el régimen por estación trae campos que la predicción no tiene")
        self.comprobar(o, ejemplos["redis-modelo-activo.json"] == ejemplos["modelos.json"],
                       "el ejemplo de modelo:activo no es el documento de modelos")
        t = pd.DataFrame([(f"`{col}`", que, self.campos(ejemplos[archivo]), ", ".join(indices.get(col, ["—"])), quien)
                          for col, archivo, que, quien in self.COLECCIONES],
                         columns=["Colección", "Qué guarda", "Campos (además de `_id`)", "Índices", "Quién la escribe"])
        self.tabla("entidades_mongo.md", o, {}, "tbl-entidades-mongo", t_mongo, markdown(t, "lllll"), anchos=[21, 16, 33, 15, 15],
                   fuente="Fuente: los documentos de ejemplo de `docs/datos/ejemplos/` (los campos) y `docs/datos/seed.js` (los índices, con "
                          "su nombre y, entre paréntesis, sus campos).",
                   nota="Entre llaves, los campos de un subdocumento; con corchetes, los de cada elemento de un arreglo. Las pruebas "
                        "`TestLosDocumentosTienenLosCamposDeSusEjemplos` (`almacen` y `almacen/mongo`) fallan si un documento que escribe "
                        "la API deja de tener exactamente los campos de su ejemplo.")
        t = pd.DataFrame([(f"`{clave}`", self.campos(ejemplos[archivo]) if clave != "modelo:activo"
                           else "El documento de `modelos`, completo", para) for clave, archivo, para in self.CLAVES],
                         columns=["Clave", "Campos del valor", "Para qué"])
        self.tabla("claves_redis.md", o, {}, "tbl-claves-redis", t_redis, markdown(t, "lll"), anchos=[25, 37, 38],
                   fuente="Fuente: los valores de ejemplo de `docs/datos/ejemplos/redis-*.json`; las claves las arma `almacen/redis/redis.go`.",
                   nota="Ninguna clave vence: son pocas (una por estación y una por entrenamiento) y el régimen de una estación es su "
                        "única copia.")
        ficha = self.reports / "estaciones_ficha.csv"
        n_est = n_gold = None
        if ficha.exists():
            with ficha.open(encoding="utf-8-sig", newline="") as f:
                filas_f = list(csv.DictReader(f))
            n_est = len(filas_f)
            n_gold = sum(1 for r in filas_f if str(r.get("filas_gold", "0")).strip() not in ("", "0", "0.0"))
            lim = self.datos.get("limpieza.json") or {}
            if lim.get("estaciones"):
                self.comprobar("reports/estaciones_ficha.csv", n_est == len(lim["estaciones"]),
                               "la ficha no tiene las estaciones de limpieza.json")
        else:
            self.faltantes.append("reports/estaciones_ficha.csv")
        self.cifras["datos"] = {
            "colecciones": entero(len(self.COLECCIONES)), "claves": entero(len(self.CLAVES)),
            "indices": entero(sum(len(v) for v in indices.values())),
            "estaciones": entero(n_est) if n_est is not None else self.PENDIENTE,
            "estaciones_gold": entero(n_gold) if n_gold is not None else self.PENDIENTE,
        }

    # ----------------------------------------------------------------------- evidencias
    def datos_prueba(self) -> None:
        """docs/postman/lecturas_prueba.csv: los datos de prueba de Postman y de cmd/carga."""
        o = "docs/postman/lecturas_prueba.csv"
        titulo = "Datos de prueba: lecturas reales con la respuesta que la API tiene que dar"
        claves = ["lecturas", "casos", "estaciones", "regimenes"]
        ruta = RAIZ / o
        if not ruta.exists():
            self.faltantes.append(o)
            self.tabla("datos_prueba.md", o, None, "tbl-datos-prueba", titulo)
            self.pendientes("datos_prueba", claves)
            return
        df = pd.read_csv(ruta)
        necesarias = {"caso", "estacion", "fecha_hora", "pm2_5", "pm10", "regimen_esperado"}
        if not necesarias <= set(df.columns):
            self.problema(o, "no tiene las columnas " + ", ".join(sorted(necesarias - set(df.columns))))
            self.tabla("datos_prueba.md", o, None, "tbl-datos-prueba", titulo)
            self.pendientes("datos_prueba", claves)
            return
        self.revisar_estaciones(o, df["estacion"].unique())
        regs = {r.get("cluster", r.get("regimen")) for r in (self.datos.get("regimenes.json") or {}).get("regimenes", [])}
        if regs:
            self.comprobar(o, set(df["regimen_esperado"].unique()) <= regs, "espera un régimen que el modelo no tiene")
        con_alerta = "alerta_esperada" in df.columns

        def rango(xs):
            return f"{decimal(xs.min(), 1)}–{decimal(xs.max(), 1)}"

        filas = []
        for caso, g in df.groupby("caso", sort=False):
            filas.append((escapar(str(caso)), entero(len(g)), entero(g["estacion"].nunique()), lista(g["regimen_esperado"].unique()),
                          rango(g["pm2_5"]), rango(g["pm10"]),
                          entero((g["alerta_esperada"].astype(str).str.lower() == "true").sum()) if con_alerta else "—"))
        t = pd.DataFrame(filas, columns=["Caso", "Lecturas", "Estaciones", "Régimen esperado", "PM2,5 (µg/m³)", "PM10 (µg/m³)",
                                         "Con alerta esperada"])
        self.tabla("datos_prueba.md", o, {}, "tbl-datos-prueba", titulo, markdown(t, "lrrrrrr"), anchos=[19, 12, 13, 13, 15, 15, 13],
                   fuente=f"Fuente: `{o}`.",
                   nota="Cada fila del archivo es una lectura real (estación, fecha y hora, PM2,5 y PM10) con el régimen que el modelo "
                        "tiene que asignarle" + (" y, donde aplica, la media de 24 h, la alerta y la banda INCA esperadas" if con_alerta else "")
                        + ". Lo usan el Runner de Postman y `cmd/carga`.")
        self.cifras["datos_prueba"] = {
            "lecturas": entero(len(df)), "casos": entero(df["caso"].nunique()), "estaciones": entero(df["estacion"].nunique()),
            "regimenes": lista(df["regimen_esperado"].unique()),
        }

    PAQUETES_PC4 = [   # paquete, qué comprueban sus pruebas
        ("protocolo", "Mensajes y codificación, la barrera de una ronda con su plazo, la fusión en orden de nodo y el parcial rezagado"),
        ("cluster", "El nodo, la espera de nodos, el reparto de rangos, Lloyd sobre el clúster (bit a bit con un nodo, con tolerancia "
                    "con varios, un nodo caído) y el pool"),
        ("cmd/distribuido", "El coordinador de línea de comandos de punta a punta, con nodos reales por TCP"),
        ("cmd/nodo", "El comando del nodo"),
        ("cmd/smoke", "El smoke test del clúster"),
        ("modelo", "Sembrar el modelo desde `reports/` y predecir: casos fijos del gold y el ejemplo del contrato"),
        ("ventana", "La media de 24 horas: ventana, lecturas desordenadas, mínimo de lecturas, ECA e INCA, concurrencia"),
        ("api", "Rutas y errores, JWT, métricas, el escritor por colas, el coordinador de entrenamientos, el hub y el WebSocket, CORS"),
        ("almacen", "La ficha de estaciones y los documentos frente a sus ejemplos"),
        ("almacen/memoria", "El doble en memoria que usan las pruebas y el CI"),
        ("almacen/mongo", "MongoDB real: colecciones, índices del seed, un solo modelo activo, el ciclo de un entrenamiento"),
        ("almacen/redis", "Redis real: el régimen que no retrocede, escritores concurrentes, modelo activo y punto de control"),
        ("cmd/api", "El arranque, el apagado en orden, los almacenes reales y el coordinador con nodos reales"),
    ]

    def pruebas_pc4(self) -> None:
        """Las pruebas de los paquetes de la PC4, contadas del código (func Test…)."""
        import re
        o = "los archivos _test.go de cluster/, protocolo/, api/, almacen/, modelo/, ventana/ y cmd/"
        titulo = "Pruebas automáticas de los paquetes del Entregable 2"
        filas, total, integ_total = [], 0, 0
        for paquete, que in self.PAQUETES_PC4:
            carpeta = RAIZ / paquete
            archivos = sorted(carpeta.glob("*_test.go")) if carpeta.is_dir() else []
            if not archivos:
                self.problema(paquete, "no tiene pruebas (¿cambió la estructura del repositorio?)")
                continue
            n = integ = 0
            for a in archivos:
                texto = a.read_text(encoding="utf-8")
                k = len(re.findall(r"^func Test\w+\(t \*testing\.T\)", texto, re.M))
                if re.search(r"^//go:build .*\bintegracion\b", texto, re.M):
                    integ += k
                else:
                    n += k
            total, integ_total = total + n, integ_total + integ
            filas.append((f"`{paquete}`", que, entero(n) if n else "—", entero(integ) if integ else "—"))
        todas = sum(len(re.findall(r"^func Test\w+\(t \*testing\.T\)", a.read_text(encoding="utf-8"), re.M))
                    for a in RAIZ.rglob("*_test.go") if ".claude" not in a.parts and "node_modules" not in a.parts)
        t = pd.DataFrame(filas, columns=["Paquete", "Qué comprueban", "Pruebas", "De integración"])
        self.tabla("pruebas_pc4.md", o, {}, "tbl-pruebas-pc4", titulo, markdown(t, "llrr"), anchos=[18, 58, 11, 13],
                   fuente="Fuente: las funciones `Test…` de los archivos `_test.go` de cada paquete, contadas del código en cada render.",
                   nota="«De integración» son las que llevan la etiqueta de compilación `integracion`: corren contra el MongoDB y el "
                        "Redis del Compose (`go test -race -tags integracion ./almacen/... ./cmd/api/`), no en la integración continua, que "
                        "usa el doble en memoria.")
        self.cifras["pruebas"] = {"pc4": entero(total), "pc4_integracion": entero(integ_total), "paquetes_pc4": entero(len(filas)),
                                  "repo": entero(todas)}

    # Constantes del código que la prosa cita: se leen del código en cada render, para que el informe no
    # diga un valor que el código ya cambió. (clave, archivo, patrón con un grupo, cómo se escribe).
    CONSTANTES = [
        ("ventana_horas", "ventana/ventana.go", r"Duracion = (\d+) \* time\.Hour", entero),
        ("minimo_lecturas", "ventana/ventana.go", r"MinimoLecturas = (\d+)", entero),
        ("lecturas_guardadas", "ventana/ventana.go", r"Guardadas = (\d+)", entero),
        ("max_lote", "api/escritor.go", r"maxLote = (\d+)", entero),
        ("plazo_escritura_s", "api/escritor.go", r"plazoEscritura = (\d+) \* time\.Second", entero),
        ("plazo_vaciado_s", "api/escritor.go", r"plazoVaciado = (\d+) \* time\.Second", entero),
        ("max_pm", "api/rutas.go", r"maxPM\s+= (\d+)", entero),
        ("max_estacion", "api/rutas.go", r"maxEstacion = (\d+)", entero),
        ("limite_cuerpo_kib", "api/servidor.go", r"limiteCuerpo = (\d+) << 10", entero),
        ("jwt_secreto_bytes", "api/jwt.go", r"largoMinimoSecreto = (\d+)", entero),
        ("cubetas_por_octava", "api/metricas.go", r"cubetasPorOctava = (\d+)", entero),
        ("ws_bufer_cliente", "api/hub.go", r"buferCliente: (\d+)", entero),
        ("ws_ping_s", "api/hub.go", r"cadaPing: (\d+) \* time\.Second", entero),
        ("ws_pong_s", "api/hub.go", r"plazoPong: (\d+) \* time\.Second", entero),
        ("tolerancia", "cluster/lloyd.go", r"const Tolerancia = (\S+)", lambda x: cientifica(x, 0)),
        ("chunk", "cluster/nodo.go", r"ChunkPorOmision = (\d+)", entero),
        ("max_filas_nodo", "cluster/nodo.go", r"MaxFilasPorOmision = ([\d_]+)", entero),
        ("version_protocolo", "protocolo/mensaje.go", r"const Version = (\d+)", entero),
        ("max_linea_mib", "protocolo/codec.go", r"const MaxLinea = (\d+) << 20", entero),
        ("puerto_http", "cmd/api/main.go", r'"addr", ":(\d+)"', str),
        ("puerto_nodos", "cmd/api/main.go", r'"nodos-addr", ":(\d+)"', str),
        ("cola_por_omision", "cmd/api/main.go", r'"cola", (\d+),', entero),
        ("latido_s", "cmd/api/main.go", r'"latido", (\d+)\*time\.Second', entero),
        ("plazo_hola_s", "cmd/api/main.go", r'"plazo-hola", (\d+)\*time\.Second', entero),
        ("plazo_carga_s", "cmd/api/main.go", r'"plazo-carga", (\d+)\*time\.Second', entero),
        ("plazo_ronda_s", "cmd/api/main.go", r'"plazo-ronda", (\d+)\*time\.Second', entero),
        ("plazo_latido_s", "cluster/pool.go", r"op\.PlazoLatido = (\d+) \* time\.Second", entero),
        ("espera_nodo_s", "cmd/nodo/main.go", r'"espera", (\d+)\*time\.Second', entero),
        ("cola_compose", "docker-compose.yml", r'"-cola", "(\d+)"', entero),
        ("gracia_compose_s", "docker-compose.yml", r"stop_grace_period: (\d+)s", entero),
        ("mongo_imagen", "docker-compose.yml", r"image: (mongo:[\w.\-]+)", str),
        ("redis_imagen", "docker-compose.yml", r"image: (redis:[\w.\-]+)", str),
        ("go", "go.mod", r"^go (\S+)", str),
        ("driver_mongo", "go.mod", r"(go\.mongodb\.org/mongo-driver/v2 v\S+)", str),
        ("driver_redis", "go.mod", r"(github\.com/redis/go-redis/v9 v\S+)", str),
        ("swagger_ui", "api/rutas.go", r"swagger-ui-dist@([\d.]+)/swagger-ui-bundle", str),
    ]

    def constantes_pc4(self) -> None:
        import re
        valores = {}
        for clave, archivo, patron, fmt in self.CONSTANTES:
            ruta = RAIZ / archivo
            m = re.search(patron, ruta.read_text(encoding="utf-8"), re.M) if ruta.exists() else None
            if m is None:
                self.problema(archivo, f"no se encontró la constante «{clave}»: ¿cambió el código?")
                valores[clave] = self.PENDIENTE
                continue
            crudo = m.group(1).replace("_", "") if fmt is not str else m.group(1)
            valores[clave] = fmt(float(crudo)) if fmt is not str else crudo
        # El modelo sembrado y el tamaño de la tabla, de los JSON de la PC3 que la API lee al arrancar.
        modelo = self.datos.get("kmeans.json") or {}
        if modelo:
            n, k, dim = modelo["gold"]["n"], modelo["k"], modelo["gold"]["d"]
            valores |= {"filas_gold": entero(n), "k": entero(k), "features": entero(dim),
                        "inercia": decimal(modelo["inercia"], 2), "iteraciones": entero(modelo["iteraciones"]),
                        # Lo que viaja por ronda y por nodo en binario: k·D centroides de ida, k·D sumas, k conteos y la inercia de vuelta.
                        "numeros_por_ronda": entero(2 * k * dim + k + 1)}
        self.cifras["pc4"] = valores

    FRAGMENTOS_PC4 = [
        ("frag_repartir", "lst-repartir", "cluster/reparto.go",
         [("rango", r"^\tbase, resto := n/k, n%k", r"^\treturn rangos, nil")],
         "Repartir: rangos contiguos que difieren a lo sumo en una fila, en orden de nodo"),
        ("frag_ronda", "lst-ronda", "protocolo/coordinador.go",
         [("rango", r"^\tparciales := make\(\[\]Mensaje", r"^\treturn parciales, nil")],
         "La barrera de una ronda: una goroutine por nodo, un WaitGroup y los parciales en orden de nodo"),
        ("frag_esperar", "lst-esperar", "protocolo/coordinador.go",
         [("rango", r"^\t\tswitch \{", r"ErrProtocolo, m\.Iter, e\.ultimaIter\)")],
         "Mientras se espera el PARCIAL de una ronda: se descartan los PONG sueltos y los parciales rezagados"),
        ("frag_lloyd", "lst-lloyd", "cluster/lloyd.go",
         [("bloque", r"^\tfor it := 1; it <= op\.MaxIter; it\+\+ \{")],
         "Lloyd sobre el clúster: ronda, fusión en orden de nodo, cierre de la iteración y aviso a quien la sigue"),
        ("frag_nodo_iterar", "lst-nodo-iterar", "cluster/nodo.go",
         [("funcion", r"^func \(n \*Nodo\) iterar\(")],
         "El nodo ante un ITERAR: el parcial de su rango, con su propio pool de workers"),
        ("frag_pool_tomar", "lst-pool-tomar", "cluster/pool.go",
         [("rango", r"^\tp\.latir\(\)$", r"^\treturn ms, nil")],
         "Tomar: un entrenamiento se lleva n nodos libres del pool, ordenados por nombre"),
        ("frag_semaforo", "lst-semaforo", "api/entrenamientos.go",
         [("rango", r"^\t// El semáforo: un entrenamiento a la vez", r"^\t\treturn\n?$")],
         "Un entrenamiento a la vez: un canal de capacidad 1 como semáforo, sin esperar"),
        ("frag_al_ronda", "lst-al-ronda", "api/entrenamientos.go",
         [("bloque", r"^\talRonda := func\(r cluster\.Ronda\) \{")],
         "Cada ronda va a sus tres consumidores, cada uno por su canal"),
        ("frag_predecir", "lst-predecir", "api/rutas.go",
         [("rango", r"^\tmedia := s\.ventanas\.Agregar", r"^\t\}\)\n?$")],
         "El final del camino de una predicción: la ventana en memoria y la escritura encolada"),
        ("frag_cola", "lst-cola", "api/escritor.go",
         [("bloque", r"^func \(c \*cola\) encolar\("), ("bloque", r"^func \(c \*cola\) completar\(")],
         "La cola de escritura de un almacén: encolar nunca bloquea y el lote toma lo que ya esté esperando"),
        ("frag_ventana", "lst-ventana", "ventana/ventana.go",
         [("bloque", r"^func \(w \*ventana\) agregar\(")],
         "La ventana de 24 horas de una estación: una lectura por hora, 72 guardadas y la media de las de la ventana"),
        ("frag_activar", "lst-activar", "api/servidor.go",
         [("funcion", r"^func \(s \*Servidor\) ActivarModelo\(")],
         "El modelo activo se reemplaza entero, sin candado en el camino de la predicción"),
        ("frag_jwt", "lst-jwt", "api/jwt.go",
         [("bloque", r"^func \(f \*Firmador\) Verificar\(")],
         "Verificación de un JWT HS256 con la biblioteca estándar"),
        ("frag_hub", "lst-hub", "api/hub.go",
         [("bloque", r"^\trepartir := func\(d difusion\) \{")],
         "El hub reparte un evento sin esperar a nadie: el cliente con el búfer lleno se desconecta"),
        ("frag_guion_regimen", "lst-guion-regimen", "almacen/redis/redis.go",
         [("rango", r"^local guardado = redis\.call", r"^return 1")],
         "El script de Lua con que Redis compara y escribe el régimen de una estación en un solo paso"),
        ("frag_buzon", "lst-buzon", "api/puntos.go",
         [("funcion", r"^func \(b \*buzonPuntos\) dejar\(")],
         "El punto de control de cada ronda: un buzón de un lugar donde el punto nuevo reemplaza al que no se escribió"),
    ]

    def estado(self, ambito: str, archivo: str) -> None:
        """El aviso de lo que falta o no cuadra en un entregable, que su informe incluye al inicio."""
        l = self._listas[ambito]
        estado = ["<!-- Generado por tablas.py. No editar a mano. -->", ""]
        if any(l.values()):
            estado += ["::: {.callout-warning title=\"PENDIENTE: salidas por actualizar\"}"]
            if l["placeholders"]:
                estado.append("Estos archivos de `generado/` todavía son PLACEHOLDER y sus cifras no son "
                              "resultados sobre el dataset de OEFA: " + ", ".join(f"`{p}`" for p in l["placeholders"]) + ".")
            if l["sinteticos"]:
                estado.append("\nDatos sintéticos: " + "; ".join(l["sinteticos"]) + ".")
            if l["desactualizadas"]:
                estado.append("\nDesactualizado (se regenera con los notebooks): " + "; ".join(l["desactualizadas"]) + ".")
            if l["faltantes"]:
                estado.append("\nFaltan: " + ", ".join(f"`{p}`" for p in dict.fromkeys(l["faltantes"])) + ".")
            if l["problemas"]:
                estado.append("\nInvariantes que no cuadran: " + "; ".join(l["problemas"]) + ".")
            estado += [":::", ""]
        (self.salida / archivo).write_text("\n".join(estado), encoding="utf-8")

    def cerrar(self, estricto: bool, final: bool, entrega: str = "pc3") -> int:
        self.estado("pc3", "_estado.md")
        self.estado("pc4", "_estado_pc4.md")
        pc3 = self._listas["pc3"]

        lineas = ["# Generado por tablas.py desde generado/*.json. No editar a mano.",
                  "# Uso en el .qmd: shortcode meta con la clave cifras.grupo.clave", "cifras:", "  estado:",
                  f"    placeholders: {json.dumps(', '.join(pc3['placeholders']) or 'ninguno', ensure_ascii=False)}"]
        for grupo, valores in self.cifras.items():
            lineas.append(f"  {grupo}:")
            for k, v in valores.items():
                lineas.append(f"    {k}: {json.dumps(v, ensure_ascii=False)}")
        (AQUI / "generado" / "cifras.yml").write_text("\n".join(lineas) + "\n", encoding="utf-8")

        print(f"tablas en {self.salida.relative_to(AQUI)}/, cifras en generado/cifras.yml")
        if pc3["placeholders"]:
            print("PLACEHOLDER: " + ", ".join(pc3["placeholders"]), file=sys.stderr)
        # El informe de la PC4 contiene el de la PC3: tiene que estar en regla lo de los dos.
        ambitos = ("pc3", "pc4") if entrega == "pc4" else ("pc3",)
        junto = {n: [x for a in ambitos for x in self._listas[a][n]] for n in LISTAS}
        if (estricto or final) and (junto["faltantes"] or junto["problemas"]):
            for x in dict.fromkeys(junto["faltantes"]):
                print(f"falta: {x}", file=sys.stderr)
            print("error: faltan JSON o no cuadran invariantes", file=sys.stderr)
            return 1
        if final and (junto["placeholders"] or junto["sinteticos"] or junto["desactualizadas"]):
            for x in junto["desactualizadas"]:
                print(f"desactualizado: {x}", file=sys.stderr)
            for x in junto["sinteticos"]:
                print(f"sintético: {x}", file=sys.stderr)
            print("error (--final): quedan JSON PLACEHOLDER, datos sintéticos o salidas desactualizadas", file=sys.stderr)
            return 1
        return 0


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("--entrada", type=Path, default=AQUI / "generado", help="carpeta con los JSON (p. ej. ../../reports)")
    ap.add_argument("--reports", type=Path, default=RAIZ / "reports",
                    help="reports/ del repo: calibracion_resumen.csv y kmeans_propuesto.json del notebook 02")
    ap.add_argument("--estricto", action="store_true", help="falla si falta un JSON o no cuadra un invariante")
    ap.add_argument("--final", action="store_true", help="como --estricto y, además, falla si queda un PLACEHOLDER")
    ap.add_argument("--entrega", choices=("pc3", "pc4"), default="pc3",
                    help="qué informe decide el código de salida de --estricto y --final: el de la PC4 exige "
                         "además lo de la PC4 (las tablas y cifras de los dos se generan siempre)")
    a = ap.parse_args()
    salida = AQUI / "generado" / "tablas"
    salida.mkdir(parents=True, exist_ok=True)
    inf = Informe(a.entrada.resolve(), salida, a.reports.resolve())
    inf.limpieza()          # primero: estandarizacion lo usa para cruzar n
    inf.estandarizacion()
    inf.benchmark_cargador()
    inf.benchmark_kmeans()
    inf.kmeans()
    inf.regimenes()
    inf.codo()
    inf.csv_oefa()
    inf.tipologia()
    inf.umbrales()
    inf.meteorologia()
    inf.estaciones_actividad()
    inf.fragmentos()
    inf.calibracion()
    inf.auditoria()
    inf.figuras()
    inf.ambito = "pc4"      # lo que sigue es del Entregable 2
    inf.smoke()
    inf.distribuido()
    inf.gpu()
    inf.mapa_gpu()
    inf.alerta()
    inf.api_latencia()
    inf.api_rutas()
    inf.entidades()
    inf.datos_prueba()
    inf.salidas()
    inf.pruebas_pc4()
    inf.constantes_pc4()
    inf.fragmentos(inf.FRAGMENTOS_PC4)
    return inf.cerrar(a.estricto, a.final, a.entrega)


if __name__ == "__main__":
    sys.exit(main())
