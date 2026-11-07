#!/usr/bin/env python3
"""Contrasta las salidas guardadas de limpieza; no ejecuta el CSV ni requiere pandas."""
import json
from datetime import datetime
from pathlib import Path

RAIZ = Path(__file__).resolve().parents[1]


def leer(ruta):
    return json.loads((RAIZ / ruta).read_text())


def main():
    go = leer("reports/limpieza.json")
    py = leer("reports/eda_limpieza_prototipo.json")
    copia = leer("docs/informe/generado/limpieza.json")
    estandar = leer("reports/estandarizacion.json")
    receta = leer("configs/kmeans.json")
    estaciones = go["estaciones"]
    largas, anchas = go["filas_largas"], go["filas_anchas"]
    controles = []

    def comprobar(nombre, cumple):
        controles.append((nombre, bool(cumple)))

    comprobar("Copia del informe idéntica al reporte de Go", copia == go)
    comprobar("Conteos de filas largas Go = prototipo Python", largas == py["filas_largas"])
    comprobar("Conteos de filas anchas Go = prototipo Python", anchas == py["filas_anchas"])
    comprobar("Mismo conjunto de estaciones Go/Python", estaciones.keys() == py["estaciones"].keys())
    comprobar("Conservación: leídas = procesadas + descartadas",
              largas["leidas"] == largas["procesadas"] + largas["descartadas"])
    comprobar("Suma de descartes por regla", sum(largas["descartadas_por_regla"].values()) == largas["descartadas"])
    comprobar("Hora explícita + inferida = procesadas",
              largas["hora_explicita"] + largas["hora_inferida"] == largas["procesadas"])
    for campo, total in [("largas_leidas", largas["leidas"]),
                         ("largas_procesadas", largas["procesadas"]),
                         ("anchas", anchas["total"]),
                         ("completas_gold", anchas["completas_gold"])]:
        sin_estacion = sum(largas["descartadas_por_regla"].get(k, 0)
                           for k in ("columnas", "estacion")) if campo == "largas_leidas" else 0
        comprobar(f"Suma por estación: {campo}", sum(e[campo] for e in estaciones.values()) + sin_estacion == total)
    comprobar("Gold = n de estandarización", anchas["completas_gold"] == estandar["n"])
    comprobar("Features de limpieza y estandarización = receta",
              go["features"] == estandar["features"] == receta["features"])

    # El EDA publica los conteos sin redondear de F1-M0: contraste por clave, no por posición.
    texto = (RAIZ / "reports/eda_criba.md").read_text()
    bloque = texto.split("### Filas completas por estación (regla 8)", 1)[1].split("### Huecos", 1)[0]
    eda = {}
    for linea in bloque.splitlines():
        celdas = [c.strip() for c in linea.strip().strip("|").split("|")]
        if linea.startswith("| CA-") or linea.startswith("| TOTAL"):
            if celdas[0] in eda:
                raise ValueError(f"Estación repetida en el EDA: {celdas[0]}")
            eda[celdas[0]] = int(celdas[1].replace(" ", ""))
    comprobar("EDA: total F1-M0 = gold", eda.pop("TOTAL") == anchas["completas_gold"])
    comprobar("EDA: mismas estaciones con silver", set(eda) == {n for n, e in estaciones.items() if e["anchas"]})
    for nombre, e in sorted(estaciones.items()):
        p = py["estaciones"].get(nombre, {})
        comprobar(f"{nombre}: conteos y fechas Go/Python", all(e.get(k) == p.get(k) for k in
                  ("largas_leidas", "largas_procesadas", "anchas", "completas_gold", "desde", "hasta")))
        comprobar(f"{nombre}: gold ≤ silver", 0 <= e["completas_gold"] <= e["anchas"])
        if e["anchas"]:
            comprobar(f"{nombre}: F1-M0 del EDA = gold", eda.get(nombre) == e["completas_gold"])

    salida = ["# Revisión de limpieza y completitud", "",
              "Generado con `python3 docs/verificar-limpieza.py`. Compara salidas guardadas; no vuelve a ejecutar el bronze.", "",
              "Fuentes: `reports/limpieza.json`, `reports/eda_limpieza_prototipo.json`, `reports/eda_criba.md`,",
              "`reports/estandarizacion.json`, `configs/kmeans.json` y la copia de limpieza del informe.", "",
              f"SHA-256 del bronze declarado por Go: `{go['sha256']}`. No se recalculó sobre el CSV.", "",
              "| Control | Resultado |", "|:---|:---|"]
    salida += [f"| {nombre} | {'OK' if ok else 'DIFERENCIA'} |" for nombre, ok in controles]
    salida += ["", "## Completitud por estación", "",
               "Rango: horas entre la primera y última fila silver, ambos extremos incluidos. No equivale al calendario de monitoreo activo.", "",
               "| Estación | Silver | Gold | Horas del rango | Gold/silver (%) | Gold/rango (%) |",
               "|:---|---:|---:|---:|---:|---:|"]
    for nombre, e in sorted(estaciones.items()):
        if not e["anchas"]:
            salida.append(f"| {nombre} | 0 | 0 | — | — | — |")
            continue
        horas = int((datetime.fromisoformat(e["hasta"]) - datetime.fromisoformat(e["desde"])).total_seconds() / 3600) + 1
        gold, silver = e["completas_gold"], e["anchas"]
        salida.append(f"| {nombre} | {silver} | {gold} | {horas} | {100*gold/silver:.1f} | {100*gold/horas:.1f} |")
    salida += ["", "Las estaciones sin silver no tienen denominador temporal calculable en este reporte.",
               "Una coincidencia entre reportes guardados demuestra consistencia, no una nueva auditoría independiente del CSV.", ""]
    (RAIZ / "reports/revision_limpieza.md").write_text("\n".join(salida))
    errores = sum(not ok for _, ok in controles)
    print(f"{len(controles)} controles; {errores} diferencias. reports/revision_limpieza.md")
    return bool(errores)


if __name__ == "__main__":
    raise SystemExit(main())
