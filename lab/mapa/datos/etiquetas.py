"""Arma data/mapa/etiquetas.csv: la verdad de las estaciones en la grilla del mapa (spec §3.1 y §3.4).

    uv run python etiquetas.py --silver /ruta/al/checkout/data/silver/oefa_ancho.csv

Una fila por estación-hora con al menos un valor: ``hora_utc`` (ISO, la hora de fin del intervalo),
``estacion``, ``fuente``, ``i``, ``j`` (la celda más cercana), ``pm2_5`` y ``pm10`` en µg/m³ (vacío si
falta), ordenadas por estación y hora. Solo las horas del periodo de ``dias/`` (2017-12-31 00 UTC a
2022-12-31 23 UTC).

OEFA: el silver trae ``fecha_hora`` en RFC3339 −05:00 con la convención de ``reports/limpieza.json →
convencion_hora`` ("fin": la lectura de 00:00 a 01:00 se fecha a la 01:00); se pasa a UTC y, si la
convención fuera "inicio", se le suma una hora. Coordenadas de ``reports/estaciones_ficha.csv``.

SENAMHI (gancho, decisión abierta 1 de la spec): si hay CSV en ``data/raw/senamhi/`` del mismo checkout
que el silver, se suman con ``fuente=senamhi``. El lector supone las columnas ESTACION, FECHA, HORA,
PM10, PM2_5, LATITUD, LONGITUD (las de la propuesta SENAMHI del equipo) en hora local −05:00, y exige
``--senamhi-convencion`` porque esa convención no está vista.

Deja además ``etiquetas_resumen.json`` al lado (filas, estaciones, horas, celdas, choques) y, si ya hay
manifiesto, actualiza ahí el sha256 de etiquetas.csv.
"""

import argparse
import json
import sys
from datetime import date, timedelta, timezone
from pathlib import Path

import pandas as pd

from comun import DESDE, DIR_MAPA, HASTA, RAIZ_REPO, escribir_atomico, escribir_json, fecha, registrar_archivo
from grilla import GRILLA_NACIONAL, Grilla

COLUMNAS = ["hora_utc", "estacion", "fuente", "i", "j", "pm2_5", "pm10"]
FORMATO_HORA = "%Y-%m-%dT%H:%M:%SZ"
COLUMNAS_SENAMHI = ["ESTACION", "FECHA", "HORA", "PM10", "PM2_5", "LATITUD", "LONGITUD"]
LOCAL = timezone(timedelta(hours=-5))


def a_utc_fin(horas: pd.Series, convencion: str) -> pd.Series:
    """Horas con zona (texto RFC3339 o Timestamp) → instante UTC del fin del intervalo."""
    if convencion not in ("fin", "inicio"):
        raise ValueError(f"convención horaria desconocida: {convencion!r} (fin o inicio)")
    if pd.api.types.is_datetime64_any_dtype(horas):
        utc = horas.dt.tz_convert("UTC")
    else:
        utc = pd.to_datetime(horas, utc=True, format="ISO8601")
    return utc + pd.Timedelta(hours=1) if convencion == "inicio" else utc


def leer_oefa(silver: Path, ficha: Path, convencion: str) -> pd.DataFrame:
    df = pd.read_csv(silver, usecols=["estacion", "fecha_hora", "pm2_5", "pm10"], dtype={"estacion": str, "fecha_hora": str})
    coordenadas = pd.read_csv(ficha, usecols=["estacion", "lat", "lon"], dtype={"estacion": str})
    df = df.merge(coordenadas, on="estacion", how="left")
    df["hora_utc"] = a_utc_fin(df["fecha_hora"], convencion)
    df["fuente"] = "oefa"
    return df.drop(columns="fecha_hora")


def leer_senamhi(directorio: Path, convencion: str | None) -> pd.DataFrame | None:
    """Los CSV de SENAMHI Lima, o None si no hay. Columnas supuestas: COLUMNAS_SENAMHI."""
    archivos = sorted(directorio.glob("*.csv")) if directorio.is_dir() else []
    if not archivos:
        return None
    if convencion not in ("fin", "inicio"):
        raise ValueError(
            f"hay CSV de SENAMHI en {directorio}: di su convención horaria con --senamhi-convencion "
            "inicio|fin (mírala en la documentación del dataset; no se supone)"
        )
    partes = []
    for archivo in archivos:
        df = pd.read_csv(archivo, dtype=str, encoding="utf-8-sig")
        faltan = [c for c in COLUMNAS_SENAMHI if c not in df.columns]
        if faltan:
            raise ValueError(f"{archivo}: faltan las columnas {faltan}; trae {list(df.columns)}. Ajusta leer_senamhi.")
        local = pd.to_datetime(df["FECHA"].str.strip() + " " + df["HORA"].str.strip(), format="mixed", dayfirst=True)
        partes.append(pd.DataFrame({
            "estacion": df["ESTACION"].str.strip(),
            "fuente": "senamhi",
            "lat": pd.to_numeric(df["LATITUD"], errors="coerce"),
            "lon": pd.to_numeric(df["LONGITUD"], errors="coerce"),
            "hora_utc": a_utc_fin(local.dt.tz_localize(LOCAL), convencion),
            "pm2_5": pd.to_numeric(df["PM2_5"], errors="coerce"),
            "pm10": pd.to_numeric(df["PM10"], errors="coerce"),
        }))
    return pd.concat(partes, ignore_index=True)


def armar(crudas: list[pd.DataFrame], grilla: Grilla, desde: date, hasta: date) -> tuple[pd.DataFrame, dict]:
    """Etiquetas en la grilla y el resumen: filas, estaciones, fuera de grilla o periodo, choques de celda."""
    df = pd.concat(crudas, ignore_index=True).dropna(subset=["pm2_5", "pm10"], how="all")
    sin_coordenadas = sorted(df.loc[df["lat"].isna() | df["lon"].isna(), "estacion"].unique())
    df = df.dropna(subset=["lat", "lon"])

    estaciones = df.groupby(["estacion", "fuente"], as_index=False)[["lat", "lon"]].first()
    estaciones["celda"] = [grilla.celda(la, lo) for la, lo in zip(estaciones["lat"], estaciones["lon"])]
    fuera_de_grilla = sorted(estaciones.loc[estaciones["celda"].isna(), "estacion"])
    estaciones = estaciones.dropna(subset=["celda"])
    estaciones["i"] = [c[0] for c in estaciones["celda"]]
    estaciones["j"] = [c[1] for c in estaciones["celda"]]
    df = df.merge(estaciones[["estacion", "fuente", "i", "j"]], on=["estacion", "fuente"], how="inner")

    inicio = pd.Timestamp(desde, tz="UTC")
    fin = pd.Timestamp(hasta, tz="UTC") + pd.Timedelta(hours=23)
    en_periodo = (df["hora_utc"] >= inicio) & (df["hora_utc"] <= fin)
    fuera_de_periodo = int((~en_periodo).sum())
    df = df[en_periodo].sort_values(["estacion", "hora_utc"], kind="stable")
    duplicadas = int(df.duplicated(["estacion", "fuente", "hora_utc"]).sum())
    if duplicadas:
        raise ValueError(f"{duplicadas} filas repiten estación, fuente y hora")

    por_estacion = []
    for (est, fuente), g in df.groupby(["estacion", "fuente"], sort=True):
        e = estaciones[(estaciones["estacion"] == est) & (estaciones["fuente"] == fuente)].iloc[0]
        por_estacion.append({
            "estacion": est, "fuente": fuente, "lat": float(e["lat"]), "lon": float(e["lon"]),
            "i": int(e["i"]), "j": int(e["j"]), "filas": len(g),
            "con_pm2_5": int(g["pm2_5"].notna().sum()), "con_pm10": int(g["pm10"].notna().sum()),
            "desde": g["hora_utc"].min().strftime(FORMATO_HORA), "hasta": g["hora_utc"].max().strftime(FORMATO_HORA),
        })
    por_celda = {}
    for e in por_estacion:
        por_celda.setdefault((e["i"], e["j"]), []).append(e["estacion"])
    choques = []
    for (i, j), ests in sorted(por_celda.items()):
        if len(ests) > 1:
            # Horas en que más de una estación de la celda tiene valor: ahí la celda tiene dos verdades.
            simultaneas = df[(df["i"] == i) & (df["j"] == j)].groupby("hora_utc")["estacion"].nunique()
            choques.append({"i": i, "j": j, "estaciones": sorted(ests), "horas_simultaneas": int((simultaneas > 1).sum())})

    df = df.assign(hora_utc=df["hora_utc"].dt.strftime(FORMATO_HORA))[COLUMNAS].reset_index(drop=True)
    df[["i", "j"]] = df[["i", "j"]].astype(int)
    resumen = {
        "filas": len(df),
        "estaciones": len(por_estacion),
        "horas": int(df["hora_utc"].nunique()),
        "desde": df["hora_utc"].min() if len(df) else None,
        "hasta": df["hora_utc"].max() if len(df) else None,
        "con_pm2_5": int(df["pm2_5"].notna().sum()),
        "con_pm10": int(df["pm10"].notna().sum()),
        "por_fuente": {k: int(v) for k, v in df["fuente"].value_counts().sort_index().items()},
        "fuera_de_grilla": fuera_de_grilla,
        "sin_coordenadas": sin_coordenadas,
        "fuera_de_periodo": fuera_de_periodo,
        "choques": choques,
        "por_estacion": por_estacion,
    }
    return df, resumen


def escribir(df: pd.DataFrame, ruta: Path) -> None:
    escribir_atomico(ruta, df.to_csv(index=False).encode())


def main(argv=None) -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--silver", type=Path, default=RAIZ_REPO / "data" / "silver" / "oefa_ancho.csv")
    ap.add_argument("--ficha", type=Path, default=RAIZ_REPO / "reports" / "estaciones_ficha.csv")
    ap.add_argument("--limpieza", type=Path, default=RAIZ_REPO / "reports" / "limpieza.json",
                    help="de acá sale convencion_hora del silver")
    ap.add_argument("--senamhi", type=Path, help="carpeta con los CSV de SENAMHI (por defecto data/raw/senamhi del checkout del silver)")
    ap.add_argument("--senamhi-convencion", choices=["inicio", "fin"])
    ap.add_argument("--desde", type=fecha, default=DESDE)
    ap.add_argument("--hasta", type=fecha, default=HASTA)
    ap.add_argument("--salida", type=Path, default=DIR_MAPA / "etiquetas.csv")
    args = ap.parse_args(argv)

    if not args.silver.exists():
        print(f"no está el silver en {args.silver}: pásalo con --silver (vive en el checkout principal)", file=sys.stderr)
        return 2
    convencion = json.loads(args.limpieza.read_text())["convencion_hora"]
    crudas = [leer_oefa(args.silver, args.ficha, convencion)]
    dir_senamhi = args.senamhi or args.silver.resolve().parents[1] / "raw" / "senamhi"
    senamhi = leer_senamhi(dir_senamhi, args.senamhi_convencion)
    if senamhi is None:
        print(f"aviso: no hay CSV de SENAMHI en {dir_senamhi}; las etiquetas son solo de OEFA")
    else:
        crudas.append(senamhi)

    tabla, resumen = armar(crudas, GRILLA_NACIONAL, args.desde, args.hasta)
    resumen = {"convencion_hora_oefa": convencion, "senamhi": str(dir_senamhi) if senamhi is not None else None, **resumen}
    escribir(tabla, args.salida)
    escribir_json(args.salida.with_name("etiquetas_resumen.json"), resumen)
    if registrar_archivo(args.salida.parent, args.salida.name):
        print("sha256 de etiquetas.csv actualizado en el manifiesto")

    print(f"{resumen['filas']} filas, {resumen['estaciones']} estaciones, {resumen['horas']} horas "
          f"({resumen['desde']} → {resumen['hasta']}); con PM2.5 {resumen['con_pm2_5']}, con PM10 {resumen['con_pm10']}")
    print(f"por fuente: {resumen['por_fuente']}")
    print(f"fuera de la grilla: {resumen['fuera_de_grilla'] or 'ninguna'}; sin coordenadas: {resumen['sin_coordenadas'] or 'ninguna'}; "
          f"filas fuera del periodo: {resumen['fuera_de_periodo']}")
    if resumen["choques"]:
        for c in resumen["choques"]:
            print(f"choque de celda ({c['i']}, {c['j']}): {', '.join(c['estaciones'])}; "
                  f"{c['horas_simultaneas']} horas con más de una a la vez")
    else:
        print("choques de celda: ninguno")
    print(f"→ {args.salida}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
