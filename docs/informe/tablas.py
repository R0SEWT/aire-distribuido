# /// script
# requires-python = ">=3.10"
# dependencies = ["pandas>=2"]
# ///
"""Convierte los JSON de generado/ en tablas Markdown y cifras para el informe.

    python3 tablas.py                 # lee generado/*.json, escribe generado/tablas/*.md y generado/cifras.yml
    python3 tablas.py --estricto      # además falla si queda un PLACEHOLDER o no cuadra un invariante
    uv run tablas.py                  # si no hay pandas en el Python del sistema

Ninguna cifra del informe se escribe a mano: los comandos de Go escriben los JSON (contrato en
README.md), este script los vuelve tablas (`{{< include generado/tablas/x.md >}}` en el .qmd) y
cifras para la prosa (`{{< meta cifras.x.y >}}`, desde generado/cifras.yml). Si un JSON trae
`"placeholder": true`, su tabla sale con un aviso PLACEHOLDER y sus cifras con la marca
[PLACEHOLDER], para que nadie las confunda con resultados.

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

# Nombres para mostrar. Lo que no esté aquí sale con su clave tal cual.
NOMBRES = {
    "pm2_5": "PM2,5 (µg/m³)",
    "pm10": "PM10 (µg/m³)",
    "fraccion_fina": "Fracción fina PM2,5/PM10",
    "viento": "Viento (m/s)",
    "humedad": "Humedad relativa (%)",
    "temperatura": "Temperatura (°C)",
    "presion": "Presión (hPa)",
    "no2": "NO2 (µg/m³)",
    "so2": "SO2 (µg/m³)",
    "co": "CO (µg/m³)",
    "o3": "O3 (µg/m³)",
}
# Para tablas anchas (perfiles y centroides): la unidad va en la nota de la tabla.
CORTOS = {
    "pm2_5": "PM2,5", "pm10": "PM10", "fraccion_fina": "Fracción fina", "viento": "Viento",
    "humedad": "Humedad", "temperatura": "Temp.", "presion": "Presión",
    "no2": "NO2", "so2": "SO2", "co": "CO", "o3": "O3",
}
FRANJAS = [("Madrugada (0–5 h)", range(0, 6)), ("Mañana (6–11 h)", range(6, 12)),
           ("Tarde (12–17 h)", range(12, 18)), ("Noche (18–23 h)", range(18, 24))]


# --------------------------------------------------------------------------- formato (es-PE)
def entero(x) -> str:
    """3512448 → «3 512 448» con espacio duro, como pide la RAE para los millares."""
    if x is None or (isinstance(x, float) and math.isnan(x)):
        return "—"
    return f"{int(round(x)):,}".replace(",", "\u00a0")


def decimal(x, d=2) -> str:
    if x is None or (isinstance(x, float) and math.isnan(x)):
        return "—"
    s = f"{x:,.{d}f}"
    return s.replace(",", "\u00a0").replace(".", ",")


def pct(x, d=1) -> str:
    return "—" if x is None else f"{decimal(100 * x, d)}\u00a0%"


def nombre(clave: str) -> str:
    return NOMBRES.get(clave, clave)


def corto(clave: str) -> str:
    return CORTOS.get(clave, clave)


def unidades(claves) -> str:
    """«PM10 en µg/m³, viento en m/s…» para la nota de una tabla con nombres cortos."""
    partes = []
    for c in claves:
        largo = NOMBRES.get(c, "")
        if "(" in largo:
            partes.append(f"{corto(c)} en {largo[largo.index('(') + 1:largo.rindex(')')]}")
    return ("Unidades: " + "; ".join(partes) + ".") if partes else ""


def celda(v) -> str:
    return str(v).replace("|", "\\|").replace("\n", " ")


def markdown(df: pd.DataFrame, alineacion: list[str] | None = None) -> str:
    """Tabla pipe de Pandoc. alineacion: 'l' o 'r' por columna (por defecto, texto a la izquierda)."""
    cols = list(df.columns)
    alineacion = alineacion or ["l"] * len(cols)
    sep = {"l": ":---", "r": "---:", "c": ":---:"}
    filas = ["| " + " | ".join(celda(c) for c in cols) + " |",
             "|" + "|".join(sep[a] for a in alineacion) + "|"]
    for _, fila in df.iterrows():
        filas.append("| " + " | ".join(celda(v) for v in fila.tolist()) + " |")
    return "\n".join(filas)


# --------------------------------------------------------------------------- salida
class Informe:
    def __init__(self, entrada: Path, salida: Path, estricto: bool):
        self.entrada, self.salida, self.estricto = entrada, salida, estricto
        self.cifras: dict[str, dict] = {}
        self.placeholders: list[str] = []
        self.faltantes: list[str] = []
        self.problemas: list[str] = []

    def leer(self, nombre: str) -> dict | None:
        ruta = self.entrada / nombre
        if not ruta.exists():
            self.faltantes.append(nombre)
            return None
        d = json.loads(ruta.read_text(encoding="utf-8"))
        if d.get("placeholder"):
            self.placeholders.append(nombre)
        return d

    def cifra(self, d: dict, valor: str) -> str:
        return f"{valor} {MARCA}" if d.get("placeholder") else valor

    def problema(self, origen: str, texto: str) -> None:
        self.problemas.append(f"{origen}: {texto}")
        print(f"aviso: {origen}: {texto}", file=sys.stderr)

    def tabla(self, archivo: str, origen: str, d: dict | None, ident: str, titulo: str,
              cuerpo: str, nota: str = "", anchos: list[int] | None = None) -> None:
        """Escribe generado/tablas/<archivo>: aviso si es PLACEHOLDER, tabla con id y título, nota.

        anchos: porcentaje de cada columna (tbl-colwidths); sin él, Pandoc reparte por igual.
        """
        attr = f' tbl-colwidths="[{",".join(str(a) for a in anchos)}]"' if anchos else ""
        partes = [f"<!-- Generado por tablas.py desde generado/{origen}. No editar a mano. -->", ""]
        if d is None:
            partes += ["::: {.callout-warning title=\"PENDIENTE\"}",
                       f"Falta `generado/{origen}`: esta tabla sale de ahí. Ver el contrato en "
                       "`docs/informe/README.md`.", ":::", ""]
            partes += [f"::: {{#{ident}}}", "| (sin datos) |", "|:---|", "| — |", "", titulo, ":::", ""]
        else:
            if d.get("placeholder"):
                partes += ["::: {.callout-warning title=\"PLACEHOLDER\"}",
                           f"Cifras de ejemplo: `generado/{origen}` todavía es el archivo de muestra. "
                           f"Se reemplaza cuando `{d.get('generado_por', '¿?')}` escriba el real.",
                           ":::", ""]
            partes += [f"::: {{#{ident}{attr}}}", cuerpo, "", titulo, ":::", ""]
            commit = d.get("commit") or d.get("metadatos", {}).get("commit", "¿?")
            momento = (d.get("momento") or d.get("metadatos", {}).get("momento", ""))[:10]
            fuente = (f"*Nota.* Fuente: `generado/{origen}`, escrito por `{d.get('generado_por', '¿?')}` "
                      f"(commit `{commit}`, {momento}).")
            partes += [f"{fuente} {nota}".strip(), ""]
        (self.salida / archivo).write_text("\n".join(partes), encoding="utf-8")

    # ----------------------------------------------------------------------- limpieza
    def limpieza(self) -> None:
        o = "limpieza.json"
        d = self.leer(o)
        if d is None:
            for a, i, t in (("limpieza_reglas.md", "tbl-limpieza-reglas", "Filas largas descartadas y marcadas por regla de limpieza"),
                            ("limpieza_resumen.md", "tbl-limpieza-resumen", "Conservación de filas en la carga"),
                            ("limpieza_estaciones.md", "tbl-limpieza-estaciones", "Filas anchas y completas por estación"),
                            ("cobertura.md", "tbl-cobertura", "Cobertura de la red por departamento")):
                self.tabla(a, o, None, i, t, "")
            return
        fl, fa = d["filas_largas"], d["filas_anchas"]
        reglas = pd.DataFrame(d["reglas"])
        suma = int(reglas["descartadas"].sum())
        if fl["leidas"] != fl["procesadas"] + fl["descartadas"]:
            self.problema(o, "leídas ≠ procesadas + descartadas")
        if suma != fl["descartadas"]:
            self.problema(o, f"la suma por regla ({suma}) ≠ descartadas ({fl['descartadas']})")
        est = pd.DataFrame(d["estaciones"])
        if int(est["filas_anchas"].sum()) != fa["total"] or int(est["completas"].sum()) != fa["completas"]:
            self.problema(o, "la suma por estación no coincide con filas_anchas")

        t = pd.DataFrame({
            "N.º": reglas["regla"],
            "Regla": reglas["descripcion"],
            "Descartadas": reglas["descartadas"].map(entero),
            "% de leídas": (reglas["descartadas"] / fl["leidas"]).map(lambda x: pct(x, 2)),
            "Marcadas": reglas["marcadas"].map(entero),
        })
        t.loc[len(t)] = ["", "**Total**", f"**{entero(suma)}**", f"**{pct(suma / fl['leidas'], 2)}**",
                         f"**{entero(reglas['marcadas'].sum())}**"]
        self.tabla("limpieza_reglas.md", o, d, "tbl-limpieza-reglas",
                   "Filas largas descartadas y marcadas por regla de limpieza", markdown(t, list("llrrr")),
                   anchos=[6, 52, 14, 14, 14], nota=
                   "Las marcadas se conservan (duplicados con valor distinto, unidades convertidas). "
                   "Las reglas 7 y 8 (pivot y completitud) actúan sobre filas anchas: ver la tabla de conservación.")

        r = pd.DataFrame([
            ("Filas largas leídas", entero(fl["leidas"]), "100\u00a0%"),
            ("Filas largas procesadas", entero(fl["procesadas"]), pct(fl["procesadas"] / fl["leidas"])),
            ("Filas largas descartadas", entero(fl["descartadas"]), pct(fl["descartadas"] / fl["leidas"])),
            ("Filas anchas (estación × fecha-hora), regla 7", entero(fa["total"]), "100\u00a0%"),
            ("Filas anchas completas para el modelo, regla 8", entero(fa["completas"]), pct(fa["completas"] / fa["total"])),
            ("Lecturas con PM2,5 > PM10 (marcadas)", entero(d["violaciones_pm2_5_mayor_pm10"]),
             pct(d["violaciones_pm2_5_mayor_pm10"] / fa["total"], 2)),
        ], columns=["Concepto", "Filas", "Proporción"])
        p = d["parametros"]
        self.tabla("limpieza_resumen.md", o, d, "tbl-limpieza-resumen", "Conservación de filas en la carga",
                   markdown(r, list("lrr")), anchos=[60, 22, 18], nota=
                   f"Leídas = procesadas + descartadas, en filas largas. Corrida con {p['workers']} workers y "
                   f"bloques de {entero(p['bloque_bytes'] / 2**20)} MiB en {decimal(d['duracion_s'], 1)} s.")

        e = pd.DataFrame({
            "Estación": est["estacion"],
            "Departamento": est["departamento"],
            "Desde": est["desde"].str[:10],
            "Hasta": est["hasta"].str[:10],
            "Filas anchas": est["filas_anchas"].map(entero),
            "Completas": est["completas"].map(entero),
            "% completas": (est["completas"] / est["filas_anchas"]).map(pct),
        })
        self.tabla("limpieza_estaciones.md", o, d, "tbl-limpieza-estaciones",
                   "Filas anchas, completas para el modelo y rango de fechas por estación",
                   markdown(e, list("llllrrr")))

        dep = (est.groupby("departamento")
                  .agg(estaciones=("estacion", "count"), completas=("completas", "sum"))
                  .reset_index().sort_values(["estaciones", "completas"], ascending=False))
        c = pd.DataFrame({"Departamento": dep["departamento"], "Estaciones": dep["estaciones"].map(entero),
                          "Horas completas": dep["completas"].map(entero)})
        self.tabla("cobertura.md", o, d, "tbl-cobertura", "Cobertura de la red de OEFA por departamento",
                   markdown(c, list("lrr")),
                   "Un departamento que no aparece no tiene estación de OEFA en el dataset: no hay dato, "
                   "no aire limpio.")

        self.cifras["limpieza"] = {k: self.cifra(d, v) for k, v in {
            "filas_largas_leidas": entero(fl["leidas"]),
            "filas_largas_procesadas": entero(fl["procesadas"]),
            "filas_largas_descartadas": entero(fl["descartadas"]),
            "pct_descartadas": pct(fl["descartadas"] / fl["leidas"]),
            "filas_anchas": entero(fa["total"]),
            "filas_completas": entero(fa["completas"]),
            "pct_completas": pct(fa["completas"] / fa["total"]),
            "violaciones_pm": entero(d["violaciones_pm2_5_mayor_pm10"]),
            "n_estaciones": entero(len(est)),
            "n_departamentos": entero(est["departamento"].nunique()),
            "fecha_min": str(est["desde"].min())[:10],
            "fecha_max": str(est["hasta"].max())[:10],
            "duracion_s": decimal(d["duracion_s"], 1),
            "workers": entero(p["workers"]),
        }.items()}

    # ----------------------------------------------------------------------- estandarización
    def estandarizacion(self) -> None:
        o = "estandarizacion.json"
        d = self.leer(o)
        titulo = "Media y desviación usadas en el z-score de cada feature"
        if d is None:
            self.tabla("estandarizacion.md", o, None, "tbl-estandarizacion", titulo, "")
            return
        f = pd.DataFrame(d["features"])
        if d.get("nivel") == "por_estacion":
            filas = [(nombre(x["feature"]), "sí" if x["log1p"] else "no", e["estacion"], entero(e["n"]),
                      decimal(e["media"], 3), decimal(e["desv"], 3))
                     for x in d["features"] for e in x["por_estacion"]]
            t = pd.DataFrame(filas, columns=["Feature", "log1p", "Estación", "n", "Media", "Desv."])
            al = list("lllrrr")
        else:
            t = pd.DataFrame({"Feature": f["feature"].map(nombre), "log1p": f["log1p"].map({True: "sí", False: "no"}),
                              "Media": f["media"].map(lambda x: decimal(x, 3)),
                              "Desv.": f["desv"].map(lambda x: decimal(x, 3))})
            al = list("llrr")
        self.tabla("estandarizacion.md", o, d, "tbl-estandarizacion", titulo, markdown(t, al),
                   f"Nivel {d.get('nivel', '¿?').replace('_', ' ')}, n = {entero(d['n'])} filas anchas completas. "
                   "Con log1p, la media y la desviación son de log(1 + x). Calculadas con Welford por chunk "
                   "y fusión de Chan en orden de chunk.")
        self.cifras["estandarizacion"] = {k: self.cifra(d, v) for k, v in {
            "n": entero(d["n"]), "n_features": entero(len(f)),
            "nivel": d.get("nivel", "").replace("_", " "),
            "features": ", ".join(nombre(x) for x in f["feature"]),
        }.items()}

    # ----------------------------------------------------------------------- benchmarks
    def benchmark(self, o: str, archivo: str, ident: str, titulo: str, clave: str) -> None:
        d = self.leer(o)
        if d is None:
            self.tabla(archivo, o, None, ident, titulo, "")
            return
        r = pd.DataFrame(d["resultados"])
        pr = d["protocolo"]
        unidad = pr.get("unidad_chunk", "filas")

        def chunk(x):
            if x is None or (isinstance(x, float) and math.isnan(x)):
                return "—"
            return f"{entero(x / 2**20)} MiB" if unidad == "bytes" else entero(x)

        t = pd.DataFrame({
            "Modo": r["modo"],
            "Workers": r["workers"].map(entero),
            "Bloque" if unidad == "bytes" else "Chunk (filas)": r["chunk"].map(chunk),
            "Mediana (s)": r["mediana_s"].map(lambda x: decimal(x, 3)),
        })
        al = list("lrrr")
        if "ms_por_iteracion" in r:
            t["ms/iteración"] = r["ms_por_iteracion"].map(lambda x: decimal(x, 1))
            al.append("r")
        sp = r["speedup"].apply(pd.Series)
        t["Speedup"] = sp["punto"].map(lambda x: decimal(x, 2))
        t[f"IC {int(100 * pr.get('nivel_ic', 0.95))} %"] = [
            f"[{decimal(a, 2)}; {decimal(b, 2)}]" for a, b in zip(sp["ic_inferior"], sp["ic_superior"])]
        t["Eficiencia"] = r["eficiencia"].map(lambda x: pct(x, 0))
        al += list("rrr")
        m = d.get("metadatos", {})
        nota = (f"Mediana de {pr['repeticiones']} repeticiones tras {pr.get('calentamiento', 0)} de calentamiento, "
                f"en {m.get('maquina', '¿?')} ({m.get('cpus_logicas', '¿?')} CPU lógicas, {m.get('version_go', '')}). "
                f"Speedup contra {pr.get('referencia', '¿?')}; intervalo bootstrap del cociente de medianas "
                f"con {entero(pr.get('replicas_bootstrap', 0))} réplicas. Eficiencia = speedup / workers.")
        anchos = [14, 9, 11, 11, 11, 10, 22, 12] if "ms/iteración" in t else [16, 10, 12, 14, 12, 24, 12]
        t["Modo"] = t["Modo"].map(lambda m: {"concurrente": "conc.", "secuencial": "sec."}.get(m, m))
        nota += " Modo: sec. = secuencial, conc. = concurrente."
        self.tabla(archivo, o, d, ident, titulo, markdown(t, al), nota, anchos=anchos)

        def lista(xs, fmt):
            xs = sorted({x for x in xs if x is not None and not (isinstance(x, float) and math.isnan(x))})
            textos = [fmt(x) for x in xs] or ["—"]
            return textos[0] if len(textos) == 1 else ", ".join(textos[:-1]) + " y " + textos[-1]

        conc = r[r["modo"] == "concurrente"]
        mejor = conc.loc[sp.loc[conc.index, "punto"].idxmax()]
        base = r.iloc[0]
        self.cifras[clave] = {k: self.cifra(d, v) for k, v in {
            "speedup_max": decimal(mejor["speedup"]["punto"], 2),
            "speedup_max_ic": f"[{decimal(mejor['speedup']['ic_inferior'], 2)}; {decimal(mejor['speedup']['ic_superior'], 2)}]",
            "workers_max": entero(mejor["workers"]),
            "chunk_max": chunk(mejor["chunk"]),
            "mediana_referencia_s": decimal(base["mediana_s"], 2),
            "mediana_mejor_s": decimal(mejor["mediana_s"], 2),
            "eficiencia_max": pct(mejor["eficiencia"], 0),
            "maquina": str(m.get("maquina", "")),
            "workers": lista(conc["workers"].tolist(), entero),
            "chunks": lista(conc["chunk"].tolist(), chunk),
        }.items()}

    # ----------------------------------------------------------------------- K-Means
    def kmeans(self) -> None:
        o = "kmeans.json"
        d = self.leer(o)
        if d is None:
            self.tabla("kmeans_centroides.md", o, None, "tbl-centroides", "Centroides des-estandarizados", "")
            self.tabla("kmeans_centroides_z.md", o, None, "tbl-centroides-z", "Centroides estandarizados", "")
            return
        cl = pd.DataFrame(d["clusters"])
        feats = d["receta"]["features"]
        if int(cl["tamano"].sum()) != d["n"]:
            self.problema(o, "la suma de tamaños no coincide con n")
        base = pd.DataFrame({"Cluster": cl["cluster"], "Filas": cl["tamano"].map(entero),
                             "%": (cl["tamano"] / d["n"]).map(pct)})
        des = base.copy()
        for f in feats:
            des[corto(f)] = cl["centroide"].map(lambda c, f=f: decimal(c[f], 2))
        z = base.copy()
        for i, f in enumerate(feats):
            z[corto(f)] = cl["centroide_z"].map(lambda c, i=i: decimal(c[i], 3))
        rec = d["receta"]
        nota = (f"k = {rec['k']}, semilla {rec['semilla']}, {d['iteraciones']} iteraciones "
                f"({'convergió' if d.get('convergio') else 'llegó al máximo'}), inercia {decimal(d['inercia'], 1)}, "
                f"n = {entero(d['n'])}. Receta en `{rec.get('config', 'configs/kmeans.json')}`.")
        self.tabla("kmeans_centroides.md", o, d, "tbl-centroides",
                   "Centroides des-estandarizados: media de cada feature en su unidad original",
                   markdown(des, list("rrr") + ["r"] * len(feats)), f"{nota} {unidades(feats)}")
        self.tabla("kmeans_centroides_z.md", o, d, "tbl-centroides-z",
                   "Centroides en el espacio estandarizado (z-score) en que corre el K-Means",
                   markdown(z, list("rrr") + ["r"] * len(feats)), nota)
        self.cifras["kmeans"] = {k: self.cifra(d, v) for k, v in {
            "k": entero(rec["k"]), "semilla": str(rec["semilla"]), "iteraciones": entero(d["iteraciones"]),
            "paro": (f"convergió en {entero(d['iteraciones'])} iteraciones" if d.get("convergio")
                     else f"llegó al máximo de {entero(d['iteraciones'])} iteraciones sin converger"),
            "inercia": decimal(d["inercia"], 1), "n": entero(d["n"]),
            "features": ", ".join(nombre(f) for f in feats),
        }.items()}

    def regimenes(self) -> None:
        o = "regimenes.json"
        d = self.leer(o)
        archivos = (("regimenes.md", "tbl-regimenes", "Perfil de cada régimen y su etiqueta frente al ECA"),
                    ("regimenes_hora.md", "tbl-regimenes-hora", "Distribución de cada régimen por franja horaria"),
                    ("regimenes_estacion.md", "tbl-regimenes-estacion", "Estaciones donde más aparece cada régimen"))
        if d is None:
            for a, i, t in archivos:
                self.tabla(a, o, None, i, t, "")
            return
        rg = d["regimenes"]
        claves = list(rg[0]["perfil"].keys())
        filas = []
        for x in rg:
            f = {"Régimen": f"{x['cluster']}. {x['nombre']}", "Filas": entero(x["tamano"]), "%": pct(x["proporcion"])}
            for c in claves:
                f[corto(c)] = decimal(x["perfil"][c], 2 if c == "fraccion_fina" else 1)
            f["ECA"] = x["etiqueta_eca"]
            f["INCA"] = x.get("banda_inca", "—")
            filas.append(f)
        t = pd.DataFrame(filas)
        e = d["eca"]
        resto = 100 - 16 - 11 - 9 - 11 - 10
        anchos = [16, 11, 9] + [resto // len(claves)] * len(claves) + [11, 10]
        anchos[0] += 100 - sum(anchos)
        self.tabla(archivos[0][0], o, d, archivos[0][1], archivos[0][2],
                   markdown(t, list("lrr") + ["r"] * len(claves) + list("ll")), anchos=anchos, nota=
                   f"{unidades(claves)} Perfil = media de las lecturas del régimen. Un régimen «excede» si su PM2,5 medio supera "
                   f"{e['pm2_5_24h']} µg/m³ o su PM10 medio supera {e['pm10_24h']} µg/m³ ({e['norma']}). El ECA "
                   "es un promedio de 24 horas: una lectura horaria alta no es por sí sola un incumplimiento.")

        h = []
        for x in rg:
            f = {"Régimen": f"{x['cluster']}. {x['nombre']}"}
            for etiqueta, horas in FRANJAS:
                f[etiqueta] = pct(sum(x["por_hora"][i] for i in horas), 0)
            pico = max(range(24), key=lambda i: x["por_hora"][i])
            f["Hora pico"] = f"{pico:02d}:00"
            h.append(f)
            if abs(sum(x["por_hora"]) - 1) > 0.01:
                self.problema(o, f"por_hora del régimen {x['cluster']} no suma 1")
        self.tabla(archivos[1][0], o, d, archivos[1][1], archivos[1][2],
                   markdown(pd.DataFrame(h), list("lrrrrr")),
                   "Proporción de las filas de cada régimen en cada franja (hora local, UTC−5). "
                   "La distribución hora a hora está en la figura del heatmap régimen × hora.")

        s = []
        for x in rg:
            top = sorted(x["por_estacion"], key=lambda e: -e["filas"])[:3]
            s.append({"Régimen": f"{x['cluster']}. {x['nombre']}",
                      "Estaciones principales (proporción del régimen)":
                          "; ".join(f"{e['estacion']} ({pct(e['proporcion'], 0)})" for e in top),
                      "Estaciones donde aparece": entero(sum(1 for e in x["por_estacion"] if e["filas"] > 0))})
        self.tabla(archivos[2][0], o, d, archivos[2][1], archivos[2][2],
                   markdown(pd.DataFrame(s), list("llr")), anchos=[30, 50, 20], nota=
                   "Las tres estaciones con más filas de cada régimen. Que un régimen se concentre en una "
                   "estación no dice que esa estación tenga peor aire que una zona sin estación.")

        excede = [x for x in rg if x["etiqueta_eca"] != "no excede"]
        self.cifras["regimenes"] = {k: self.cifra(d, v) for k, v in {
            "n": entero(len(rg)),
            "n_excede": entero(len(excede)),
            "excede": "; ".join(f"{x['cluster']}. {x['nombre']}" for x in excede) or "ninguno",
            "pct_filas_excede": pct(sum(x["proporcion"] for x in excede)),
        }.items()}

    def codo(self) -> None:
        o = "codo.json"
        d = self.leer(o)
        titulo = "Inercia por número de clusters (método del codo)"
        if d is None:
            self.tabla("codo.md", o, None, "tbl-codo", titulo, "")
            return
        p = pd.DataFrame(d["puntos"]).sort_values("k")
        caida = p["inercia"].pct_change().mul(-1)
        t = pd.DataFrame({"k": p["k"].map(entero), "Inercia": p["inercia"].map(lambda x: decimal(x, 1)),
                          "Caída frente a k − 1": caida.map(lambda x: "—" if pd.isna(x) else pct(x))})
        sm = d.get("submuestra", {})
        self.tabla("codo.md", o, d, "tbl-codo", titulo, markdown(t, list("rrr")),
                   f"Submuestra {sm.get('metodo', '')} de {entero(sm.get('n'))} filas anchas, semilla {d.get('semilla')}. "
                   f"k elegido: {d.get('k_elegido')}.")
        self.cifras["codo"] = {"k_elegido": self.cifra(d, entero(d.get("k_elegido")))}

    # ----------------------------------------------------------------------- estado y cifras
    def cerrar(self) -> int:
        estado = ["<!-- Generado por tablas.py. No editar a mano. -->", ""]
        if self.placeholders or self.faltantes or self.problemas:
            estado += ["::: {.callout-warning title=\"PENDIENTE: cifras de ejemplo\"}"]
            if self.placeholders:
                estado.append("Estos archivos de `generado/` todavía son PLACEHOLDER y sus cifras no son "
                              "resultados: " + ", ".join(f"`{p}`" for p in self.placeholders) + ".")
            if self.faltantes:
                estado.append("\nFaltan: " + ", ".join(f"`{p}`" for p in self.faltantes) + ".")
            if self.problemas:
                estado.append("\nInvariantes que no cuadran: " + "; ".join(self.problemas) + ".")
            estado += [":::", ""]
        (self.salida / "_estado.md").write_text("\n".join(estado), encoding="utf-8")

        lineas = ["# Generado por tablas.py desde generado/*.json. No editar a mano.",
                  "# Uso en el .qmd: {{< meta cifras.limpieza.filas_largas_leidas >}}", "cifras:"]
        lineas.append("  estado:")
        lineas.append(f"    placeholders: {json.dumps(', '.join(self.placeholders) or 'ninguno', ensure_ascii=False)}")
        for grupo, valores in self.cifras.items():
            lineas.append(f"  {grupo}:")
            for k, v in valores.items():
                lineas.append(f"    {k}: {json.dumps(v, ensure_ascii=False)}")
        (AQUI / "generado" / "cifras.yml").write_text("\n".join(lineas) + "\n", encoding="utf-8")

        print(f"tablas en {self.salida.relative_to(AQUI)}/, cifras en generado/cifras.yml")
        if self.placeholders:
            print("PLACEHOLDER: " + ", ".join(self.placeholders), file=sys.stderr)
        if self.estricto and (self.placeholders or self.faltantes or self.problemas):
            print("error (--estricto): quedan placeholders, faltantes o invariantes sin cuadrar", file=sys.stderr)
            return 1
        return 0


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("--entrada", type=Path, default=AQUI / "generado", help="carpeta con los JSON")
    ap.add_argument("--estricto", action="store_true", help="falla si queda algún PLACEHOLDER")
    a = ap.parse_args()
    entrada = a.entrada.resolve()
    salida = AQUI / "generado" / "tablas"
    salida.mkdir(parents=True, exist_ok=True)
    inf = Informe(entrada, salida, a.estricto)
    inf.limpieza()
    inf.estandarizacion()
    inf.benchmark("benchmark_cargador.json", "benchmark_cargador.md", "tbl-bench-cargador",
                  "Tiempo y speedup del cargador concurrente sobre el CSV completo", "benchmark_cargador")
    inf.benchmark("benchmark_kmeans.json", "benchmark_kmeans.md", "tbl-bench-kmeans",
                  "Tiempo y speedup del K-Means paralelo por número de workers y tamaño de chunk",
                  "benchmark_kmeans")
    inf.kmeans()
    inf.regimenes()
    inf.codo()
    return inf.cerrar()


if __name__ == "__main__":
    sys.exit(main())
