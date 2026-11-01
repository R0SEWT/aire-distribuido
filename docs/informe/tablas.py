# /// script
# requires-python = ">=3.10"
# dependencies = ["pandas>=2"]
# ///
"""Convierte los JSON de generado/ en tablas Markdown y cifras para el informe.

    python3 tablas.py                 # lee generado/*.json, escribe generado/tablas/*.md y generado/cifras.yml
    python3 tablas.py --estricto      # además falla si falta un JSON o no cuadra un invariante
    python3 tablas.py --final         # --estricto y, además, falla si queda un JSON PLACEHOLDER
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

Si un JSON trae `"placeholder": true` (lo agrega el equipo, no Go), su tabla sale con un aviso
PLACEHOLDER y sus cifras con la marca [PLACEHOLDER], para que nadie las confunda con resultados.

Python aquí es herramienta de reporte, no parte del sistema: el sistema es íntegramente Go.
"""

from __future__ import annotations

import argparse
import json
import math
import sys
from pathlib import Path

import pandas as pd

AQUI = Path(__file__).resolve().parent
MARCA = "[PLACEHOLDER]"

COMANDOS = {
    "limpieza.json": "cmd/cargar",
    "estandarizacion.json": "cmd/cargar",
    "benchmark_cargador.json": "cmd/cargar -bench",
    "kmeans.json": "cmd/entrenar",
    "regimenes.json": "cmd/entrenar",
    "codo.json": "cmd/entrenar -codo",
    "benchmark_kmeans.json": "cmd/benchmark",
}

# Reglas de descarte en el orden de cascada de datos/bloque.go: cada fila descartada se atribuye a
# la primera que falla. Clave del JSON → (regla de la spec §4.2, descripción).
REGLAS = {
    "columnas": ("—", "La fila no tiene tantos campos como la cabecera"),
    "estacion": ("—", "Estación vacía"),
    "periodo": ("1", "El periodo no es horario (diario, mensual…)"),
    "parametro": ("—", "Parámetro sin mapeo a un canónico en configs/limpieza.json"),
    "fecha": ("2", "Fecha y hora no parseables con ningún formato"),
    "fecha_fuera_de_rango": ("2", "Fecha fuera del rango configurado"),
    "valor_no_numerico": ("3", "Valor vacío, texto, NaN o infinito"),
    "centinela": ("3", "Código del equipo (−999, 9999…)"),
    "unidad": ("6", "Unidad que no figura para ese parámetro"),
    "negativo": ("3", "Negativo en un parámetro que no los admite"),
    "fuera_de_rango": ("4", "Fuera del rango físico del parámetro"),
    "duplicado_exacto": ("5", "Misma (estación, parámetro, fecha-hora) y mismo valor"),
    "duplicado_valor_distinto": ("5", "Misma clave con otro valor: se descarta el repetido y se conserva el primero"),
}

# Nombres para mostrar (unidad canónica de configs/limpieza.json). Lo que no esté sale con su clave.
NOMBRES = {
    "pm2_5": "PM2,5 (µg/m³)",
    "pm10": "PM10 (µg/m³)",
    "fraccion_fina": "Fracción fina PM2,5/PM10",
    "viento_velocidad": "Velocidad del viento (m/s)",
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
    "no2": "NO2", "so2": "SO2", "co": "CO", "o3": "O3",
}
ECA = {"excede": "excede", "no_excede": "no excede", "no_evaluable": "no evaluable"}
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
class Informe:
    def __init__(self, entrada: Path, salida: Path):
        self.entrada, self.salida = entrada, salida
        self.datos: dict[str, dict] = {}
        self.cifras: dict[str, dict] = {}
        self.placeholders: list[str] = []
        self.faltantes: list[str] = []
        self.problemas: list[str] = []

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
            partes.append(f"máquina {maquina}")
        return " (" + ", ".join(partes) + ")" if partes else " (el JSON no registra commit ni fecha)"

    def tabla(self, archivo: str, origen: str, d: dict | None, ident: str, titulo: str,
              cuerpo: str = "", nota: str = "", anchos: list[int] | None = None) -> None:
        """Escribe generado/tablas/<archivo>: aviso si es PLACEHOLDER, tabla con id y título, nota.

        anchos: porcentaje de cada columna (tbl-colwidths); sin él, Pandoc reparte por igual.
        """
        attr = f' tbl-colwidths="[{",".join(str(a) for a in anchos)}]"' if anchos else ""
        partes = [f"<!-- Generado por tablas.py desde generado/{origen}. No editar a mano. -->", ""]
        if d is None:
            partes += ["::: {.callout-warning title=\"PENDIENTE\"}",
                       f"Falta `generado/{origen}`: esta tabla sale de ahí. Lo escribe "
                       f"`{COMANDOS.get(origen, '¿?')}` (ver `docs/informe/README.md`).", ":::", ""]
            partes += [f"::: {{#{ident}}}", "| (sin datos) |", "|:---|", "| — |", "", titulo, ":::", ""]
        else:
            if d.get("placeholder"):
                motivo = d.get("_nota") or "todavía no es el resultado sobre el dataset de OEFA."
                partes += ["::: {.callout-warning title=\"PLACEHOLDER\"}",
                           f"Cifras de prueba: `generado/{origen}` {motivo} Se reemplaza cuando "
                           f"`{COMANDOS.get(origen, '¿?')}` corra sobre el CSV completo.", ":::", ""]
            partes += [f"::: {{#{ident}{attr}}}", cuerpo, "", titulo, ":::", ""]
            fuente = (f"*Nota.* Fuente: `generado/{origen}`, escrito por "
                      f"`{COMANDOS.get(origen, '¿?')}`{self.procedencia(d)}.")
            partes += [f"{fuente} {nota}".strip(), ""]
        (self.salida / archivo).write_text("\n".join(partes), encoding="utf-8")

    # ----------------------------------------------------------------------- limpieza
    def limpieza(self) -> None:
        o = "limpieza.json"
        d = self.leer(o)
        tablas = (("limpieza_resumen.md", "tbl-limpieza-resumen", "Conservación de filas en la carga"),
                  ("limpieza_reglas.md", "tbl-limpieza-reglas", "Filas largas descartadas y marcadas por regla, en el orden de la cascada"),
                  ("limpieza_estaciones.md", "tbl-limpieza-estaciones", "Filas por estación: largas leídas y procesadas, anchas, completas para el modelo y rango de fechas"),
                  ("limpieza_parametros.md", "tbl-limpieza-parametros", "Parámetros vistos en el archivo y si tienen mapeo a un parámetro canónico"),
                  ("limpieza_unidades.md", "tbl-limpieza-unidades", "Unidades vistas por parámetro canónico"),
                  ("cobertura.md", "tbl-cobertura", "Cobertura de la red por departamento"))
        if d is None:
            for a, i, t in tablas:
                self.tabla(a, o, None, i, t)
            return
        fl, fa = d["filas_largas"], d["filas_anchas"]
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
                   f"Leídas = procesadas + descartadas, en filas largas. Archivo `{d.get('entrada', '¿?')}` "
                   f"({entero(d.get('bytes'))} bytes, sha256 `{(d.get('sha256') or '¿?')[:12]}`, "
                   f"delimitador «{d.get('delimitador', '?')}», "
                   f"{'con' if d.get('bom') else 'sin'} BOM), {d.get('workers')} workers, "
                   f"{entero(d.get('bloques'))} bloque(s) de {entero(d.get('bloque_bytes', 0) / 2**20)} MiB, "
                   f"{tiempo(d.get('duracion_s'))} s.")

        filas = []
        orden = list(REGLAS) + [k for k in desc if k not in REGLAS]
        for clave in orden:
            spec, texto = REGLAS.get(clave, ("?", "(sin descripción en tablas.py)"))
            filas.append((spec, f"`{clave}`", texto, entero(desc.get(clave, 0)),
                          pct(desc.get(clave, 0) / leidas, 2) if leidas else "—",
                          entero(marc.get(clave, 0)) if clave in marc else "—"))
        filas.append(("", "", "**Total**", f"**{entero(fl['descartadas'])}**",
                      f"**{pct(fl['descartadas'] / leidas, 2)}**" if leidas else "—",
                      f"**{entero(sum(marc.values()))}**"))
        t = pd.DataFrame(filas, columns=["Spec", "Regla", "Qué descarta", "Descartadas", "% de leídas", "Marcadas"])
        fdr = fl.get("fuera_de_rango_por_parametro", {})
        nota_fdr = ("Fuera de rango por parámetro: " + "; ".join(f"{corto(k)} {entero(v)}" for k, v in sorted(fdr.items()))
                    + ".") if fdr else ""
        self.tabla(tablas[1][0], o, d, tablas[1][1], tablas[1][2], markdown(t, "lllrrr"),
                   anchos=[8, 28, 33, 11, 10, 10], nota=
                   "Cada fila descartada se cuenta en la primera regla que falla (cascada de "
                   "`datos/bloque.go`), así que la suma no tiene solapes. «Spec» es el número de regla de "
                   "la @tbl-reglas; «—» son controles de estructura. Marcadas: celdas cuyo valor conservado "
                   f"tuvo un duplicado con otro valor (se conservan). {nota_fdr}")

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
                   "y última hora con fila ancha, en hora de Lima. Una estación con lecturas y ninguna fila "
                   "procesada aparece igual, con ceros.")

        par = vistos.get("parametros", {})
        nomap = vistos.get("parametros_no_mapeados", {})
        p = pd.DataFrame([(k, entero(v), "sin mapeo: se descarta (regla `parametro`)" if k in nomap else "mapeado")
                          for k, v in sorted(par.items(), key=lambda kv: -kv[1])],
                         columns=["Parámetro en el archivo", "Lecturas", "Estado"])
        periodos = vistos.get("periodos", {})
        self.tabla(tablas[3][0], o, d, tablas[3][1], tablas[3][2], markdown(p, "lrl"), anchos=[35, 15, 50], nota=
                   "Nombres tal como vienen en el CSV, con su frecuencia. Periodos vistos: "
                   + ("; ".join(f"{k} {entero(v)}" for k, v in sorted(periodos.items(), key=lambda kv: -kv[1])) or "—")
                   + ". Parámetros canónicos presentes en la tabla ancha: "
                   + ", ".join(corto(x) for x in d.get("parametros_presentes", [])) + ".")

        u = pd.DataFrame([(corto(q), "; ".join(f"{un} ({entero(n)})" for un, n in sorted(us.items(), key=lambda kv: -kv[1])))
                          for q, us in sorted(vistos.get("unidades", {}).items())],
                         columns=["Parámetro", "Unidades vistas (lecturas)"])
        self.tabla(tablas[4][0], o, d, tablas[4][1], tablas[4][2], markdown(u, "ll"), anchos=[25, 75], nota=
                   "Las unidades con factor en `configs/limpieza.json` se convierten a la canónica (regla 6); "
                   "las que no figuran se descartan (regla `unidad`).")

        deps: dict[str, list] = {}
        for x in est.values():
            dep = titulo_lugar((x.get("ubicacion") or {}).get("departamento"))
            deps.setdefault(dep, []).append(x)
        c = pd.DataFrame([(dep, entero(len(xs)), entero(sum(x["completas_gold"] for x in xs)))
                          for dep, xs in sorted(deps.items(), key=lambda kv: (-len(kv[1]), kv[0]))],
                         columns=["Departamento", "Estaciones", "Horas completas (gold)"])
        self.tabla(tablas[5][0], o, d, tablas[5][1], tablas[5][2], markdown(c, "lrr"), anchos=[50, 20, 30], nota=
                   "Un departamento que no aparece no tiene estación en el dataset: no hay dato, no aire limpio.")

        desdes = [x["desde"] for x in est.values() if x.get("desde")]
        hastas = [x["hasta"] for x in est.values() if x.get("hasta")]
        self.guardar_cifras("limpieza", d, {
            "filas_largas_leidas": entero(leidas),
            "filas_largas_procesadas": entero(fl["procesadas"]),
            "filas_largas_descartadas": entero(fl["descartadas"]),
            "pct_descartadas": pct(fl["descartadas"] / leidas) if leidas else "—",
            "filas_anchas": entero(fa["total"]),
            "filas_completas": entero(fa["completas_gold"]),
            "pct_completas": pct(fa["completas_gold"] / fa["total"]) if fa["total"] else "—",
            "violaciones_pm": entero(fa.get("pm2_5_mayor_pm10")),
            "n_estaciones": entero(len(est)),
            "n_departamentos": entero(len(deps)),
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
                   f"Carga completa (lectura, limpieza, pivot y gold, sin escribir los CSV) de `{d.get('entrada')}` "
                   f"({entero(d.get('filas_largas'))} filas largas, {entero(d.get('bloques'))} bloque(s) de "
                   f"{entero(d.get('bloque_bytes', 0) / 2**20)} MiB). {d.get('repeticiones')} repeticiones por W, orden "
                   f"{d.get('orden', '¿?')}. Speedup: {pr.get('speedup_ic', 'media recortada con la base / media recortada con W')}, "
                   f"{entero(pr.get('replicas_bootstrap'))} réplicas, recorte de {pct(pr.get('recorte_por_extremo'), 0)} "
                   f"por extremo; al lado, el cociente de medianas. Base: {base['workers']} worker(s). "
                   f"Eficiencia = speedup / workers. {d.get('cpus')} CPU, {d.get('go', '')}, {d.get('plataforma', '')}. "
                   f"Silver y gold idénticos para todo W: {'sí' if d.get('identicos') else 'NO'}.")
        mejor = max(med, key=punto)
        self.guardar_cifras("benchmark_cargador", d, {
            "speedup_max": decimal(punto(mejor), 2),
            "speedup_max_ic": ic(mejor),
            "speedup_medianas": decimal(mejor["speedup"], 2),
            "workers_max": entero(mejor["workers"]),
            "mediana_referencia_s": tiempo(base["mediana_s"]),
            "mediana_mejor_s": tiempo(mejor["mediana_s"]),
            "eficiencia_max": pct(punto(mejor) / mejor["workers"], 0),
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
                + ("Idéntico bit a bit para todo W: "
                   + "; ".join(f"{exp.get(x['experimento'], x['experimento'])} "
                               f"{'sí' if x.get('identico_bit_a_bit_para_todo_w') else 'NO'}" for x in det) + "."
                   if det else ""))
        self.tabla("benchmark_kmeans.md", o, d, "tbl-bench-kmeans", titulo, markdown(t, "llrrrrrrrr"),
                   anchos=[14, 7, 5, 8, 11, 12, 10, 9, 15, 9], nota=nota)

        concs = [r for r in res if r["modo"] == "conc" and r.get("speedup")]
        principal = [r for r in concs if r["experimento"] == "convergencia"] or concs
        if principal:
            mejor = max(principal, key=lambda r: r["speedup"]["punto"])
            sec = next((r for r in res if r["modo"] == "seq" and r["experimento"] == mejor["experimento"]), None)
            self.guardar_cifras("benchmark_kmeans", d, {
                "experimento": exp.get(mejor["experimento"], mejor["experimento"]),
                "speedup_max": decimal(mejor["speedup"]["punto"], 2),
                "speedup_max_ic": f"[{decimal(mejor['speedup']['inferior'], 2)}; {decimal(mejor['speedup']['superior'], 2)}]",
                "workers_max": entero(mejor["workers"]),
                "chunk_max": entero(mejor["chunk"]),
                "mediana_referencia_ms": tiempo(sec["mediana_ms"]) if sec else "—",
                "mediana_mejor_ms": tiempo(mejor["mediana_ms"]),
                "eficiencia_max": pct(mejor["eficiencia_por_worker"], 0),
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
            self.tabla("kmeans_centroides.md", o, None, "tbl-centroides", "Centroides des-estandarizados")
            self.tabla("kmeans_centroides_z.md", o, None, "tbl-centroides-z", "Centroides estandarizados")
            return
        feats = d["features"]
        tam = d["tamanos"]
        n = d.get("gold", {}).get("n", sum(tam))
        self.comprobar(o, sum(tam) == n, f"la suma de tamaños ({sum(tam)}) ≠ n del gold ({n})")
        self.comprobar(o, len(d["centroides_des_estandarizados"]) == len(tam) == d.get("k_efectivo", len(tam)),
                       "centroides, tamaños y k_efectivo no coinciden")
        base = pd.DataFrame({"Cluster": [entero(i) for i in range(len(tam))], "Filas": [entero(x) for x in tam],
                             "%": [pct(x / n) for x in tam]})
        des, z = base.copy(), base.copy()
        for i, f in enumerate(feats):
            des[corto(f)] = [decimal(c[i], 2) for c in d["centroides_des_estandarizados"]]
            z[corto(f)] = [decimal(c[i], 3) for c in d["centroides_estandarizados"]]
        paro = ("convergió por tolerancia" if d.get("paro") == "tolerancia" else
                "llegó al máximo de iteraciones" if d.get("paro") == "max_iter" else f"paro «{d.get('paro')}»")
        nota = (f"k = {d['k']} (efectivo {d.get('k_efectivo')}), semilla {d.get('semilla')}, {d['iteraciones']} "
                f"iteraciones ({paro}), inercia {decimal(d['inercia'], 1)}, n = {entero(n)}, "
                f"{d.get('workers')} workers, chunk {entero(d.get('chunk'))}, clusters vacíos: {d.get('clusters_vacios', 0)}.")
        al = "rrr" + "r" * len(feats)
        anch = [10, 12, 10] + [68 // len(feats)] * len(feats)
        anch[0] += 100 - sum(anch)
        self.tabla("kmeans_centroides.md", o, d, "tbl-centroides",
                   "Centroides des-estandarizados (z-score y log1p invertidos)", markdown(des, al), anchos=anch,
                   nota=f"{nota} {unidades(feats)} {d.get('nota_des_estandarizacion', '')}")
        self.tabla("kmeans_centroides_z.md", o, d, "tbl-centroides-z",
                   "Centroides en el espacio estandarizado (z-score) en que corre el K-Means", markdown(z, al),
                   anchos=anch, nota=nota)
        self.guardar_cifras("kmeans", d, {
            "k": entero(d["k"]), "k_efectivo": entero(d.get("k_efectivo")), "semilla": str(d.get("semilla")),
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
        n = d.get("gold", {}).get("n", sum(x["tamano"] for x in rg))
        self.comprobar(o, sum(x["tamano"] for x in rg) == n, "la suma de tamaños ≠ n del gold")
        for x in rg:
            self.comprobar(o, sum(x["por_hora"]) == x["tamano"], f"por_hora del régimen {x['regimen']} no suma su tamaño")
            self.comprobar(o, sum(x["por_estacion"].values()) == x["tamano"],
                           f"por_estacion del régimen {x['regimen']} no suma su tamaño")
        feats = d.get("features") or sorted(rg[0]["perfil"])
        eca = d.get("eca", {})

        def etiqueta(x):
            e = ECA.get(x["eca"], x["eca"])
            por = x.get("eca_por") or []
            return f"{e} ({', '.join(corto(p) for p in por)})" if por else e

        filas = []
        for x in rg:
            f = {"Régimen": entero(x["regimen"]), "Filas": entero(x["tamano"]), "%": pct(x["fraccion"])}
            for c in feats:
                f[corto(c)] = decimal(x["perfil"].get(c), 2 if c == "fraccion_fina" else 1)
            # PM2,5 y PM10 medios van aparte solo si no son features (si lo son, ya están en el perfil).
            if "pm2_5" not in feats:
                f["PM2,5 medio"] = decimal(x.get("pm2_5_medio"), 1)
            if "pm10" not in feats:
                f["PM10 medio"] = decimal(x.get("pm10_medio"), 1)
            f["ECA"] = etiqueta(x)
            filas.append(f)
        t = pd.DataFrame(filas)
        extra = len(t.columns) - 3 - len(feats) - 1          # columnas de PM medio agregadas
        anch = [9, 8, 8] + [(62 - 10 * extra) // len(feats)] * len(feats) + [10] * extra + [13]
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
                   markdown(t, "rrr" + "r" * (len(feats) + extra) + "l"), anchos=anch, nota=nota)

        h = []
        for x in rg:
            f = {"Régimen": entero(x["regimen"])}
            for etiq, horas in FRANJAS:
                f[etiq] = pct(sum(x["por_hora"][i] for i in horas) / x["tamano"], 0) if x["tamano"] else "—"
            pico = max(range(24), key=lambda i: x["por_hora"][i])
            f["Hora pico"] = f"{pico:02d}:00"
            h.append(f)
        self.tabla(archivos[1][0], o, d, archivos[1][1], archivos[1][2], markdown(pd.DataFrame(h), "rrrrrr"),
                   anchos=[12, 18, 18, 18, 18, 16], nota=
                   "Proporción de las filas de cada régimen en cada franja (hora de Lima, UTC−5). La "
                   "distribución hora a hora está en la figura del heatmap régimen × hora.")

        s = []
        for x in rg:
            top = sorted(x["por_estacion"].items(), key=lambda kv: (-kv[1], kv[0]))[:3]
            s.append({"Régimen": entero(x["regimen"]),
                      "Estaciones principales (proporción del régimen)":
                          "; ".join(f"{e} ({pct(c / x['tamano'], 0)})" for e, c in top if c > 0),
                      "Estaciones donde aparece": entero(sum(1 for c in x["por_estacion"].values() if c > 0))})
        self.tabla(archivos[2][0], o, d, archivos[2][1], archivos[2][2], markdown(pd.DataFrame(s), "rlr"),
                   anchos=[12, 66, 22], nota=
                   "Las tres estaciones con más filas de cada régimen. Que un régimen se concentre en una "
                   "estación no dice que esa estación tenga peor aire que una zona sin estación.")

        excede = [x for x in rg if x["eca"] == "excede"]
        self.guardar_cifras("regimenes", d, {
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
        k = d.get("receta", {}).get("k")
        self.tabla("codo.md", o, d, "tbl-codo", titulo, markdown(t, "rrrrrl"), anchos=[10, 14, 20, 22, 16, 18],
                   nota=f"n = {lista(p['n'].tolist())} filas. {d.get('nota', '')} k de la receta: {k}.")
        self.guardar_cifras("codo", d, {"k_elegido": entero(k), "k_min": entero(p["k"].min()),
                                        "k_max": entero(p["k"].max())})

    # ----------------------------------------------------------------------- estado y cifras
    def cerrar(self, estricto: bool, final: bool) -> int:
        estado = ["<!-- Generado por tablas.py. No editar a mano. -->", ""]
        if self.placeholders or self.faltantes or self.problemas:
            estado += ["::: {.callout-warning title=\"PENDIENTE: cifras de prueba\"}"]
            if self.placeholders:
                estado.append("Estos archivos de `generado/` todavía son PLACEHOLDER y sus cifras no son "
                              "resultados sobre el dataset de OEFA: " + ", ".join(f"`{p}`" for p in self.placeholders) + ".")
            if self.faltantes:
                estado.append("\nFaltan: " + ", ".join(f"`{p}`" for p in self.faltantes) + ".")
            if self.problemas:
                estado.append("\nInvariantes que no cuadran: " + "; ".join(self.problemas) + ".")
            estado += [":::", ""]
        (self.salida / "_estado.md").write_text("\n".join(estado), encoding="utf-8")

        lineas = ["# Generado por tablas.py desde generado/*.json. No editar a mano.",
                  "# Uso en el .qmd: shortcode meta con la clave cifras.grupo.clave", "cifras:", "  estado:",
                  f"    placeholders: {json.dumps(', '.join(self.placeholders) or 'ninguno', ensure_ascii=False)}"]
        for grupo, valores in self.cifras.items():
            lineas.append(f"  {grupo}:")
            for k, v in valores.items():
                lineas.append(f"    {k}: {json.dumps(v, ensure_ascii=False)}")
        (AQUI / "generado" / "cifras.yml").write_text("\n".join(lineas) + "\n", encoding="utf-8")

        print(f"tablas en {self.salida.relative_to(AQUI)}/, cifras en generado/cifras.yml")
        if self.placeholders:
            print("PLACEHOLDER: " + ", ".join(self.placeholders), file=sys.stderr)
        if (estricto or final) and (self.faltantes or self.problemas):
            print("error: faltan JSON o no cuadran invariantes", file=sys.stderr)
            return 1
        if final and self.placeholders:
            print("error (--final): quedan JSON PLACEHOLDER", file=sys.stderr)
            return 1
        return 0


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("--entrada", type=Path, default=AQUI / "generado", help="carpeta con los JSON (p. ej. ../../reports)")
    ap.add_argument("--estricto", action="store_true", help="falla si falta un JSON o no cuadra un invariante")
    ap.add_argument("--final", action="store_true", help="como --estricto y, además, falla si queda un PLACEHOLDER")
    a = ap.parse_args()
    salida = AQUI / "generado" / "tablas"
    salida.mkdir(parents=True, exist_ok=True)
    inf = Informe(a.entrada.resolve(), salida)
    inf.limpieza()          # primero: estandarizacion lo usa para cruzar n
    inf.estandarizacion()
    inf.benchmark_cargador()
    inf.benchmark_kmeans()
    inf.kmeans()
    inf.regimenes()
    inf.codo()
    return inf.cerrar(a.estricto, a.final)


if __name__ == "__main__":
    sys.exit(main())
