"""Escribe src/datos-tb2.json con las cifras de las escenas del TB2, desde reports/.

    python3 docs/video/remotion/datos.py

Ninguna cifra de las escenas se escribe a mano: Arquitectura y RondaDistribuida leen este archivo.
Las fuentes son las corridas del K-Means distribuido con 1 a 4 nodos locales
(reports/kmeans_distribuido_local_N.json), el benchmark del cargador y la latencia de la API.
"""

from __future__ import annotations

import json
from pathlib import Path

RAIZ = Path(__file__).resolve().parents[3]
REPORTS = RAIZ / "reports"
SALIDA = Path(__file__).resolve().parent / "src" / "datos-tb2.json"


def leer(nombre: str) -> dict:
    return json.loads((REPORTS / nombre).read_text(encoding="utf-8"))


def miles(n: int) -> str:
    return f"{n:,}".replace(",", " ")


def main() -> None:
    corridas = []
    dif_max = 0.0
    for n in (1, 2, 3, 4):
        d = leer(f"kmeans_distribuido_local_{n}.json")
        ref = d["referencia"]
        dif_max = max(dif_max, ref["dif_max_centroides"])
        corridas.append({
            "nodos": n,
            "iteraciones": d["iteraciones"],
            "inercia": miles(int(d["inercia"])) + "," + f"{d['inercia']:.4f}".split(".")[1],
            "ms_rondas": round(d["ms"]["rondas"], 1),
            "ronda_p50_ms": round(d["ms_ronda"]["p50"], 3),
        })
    tres = leer("kmeans_distribuido_local_3.json")
    lat = leer("api_latencia.json")
    peor = max(lat["niveles"], key=lambda x: x["p99_ms"])
    exp = f"{dif_max:.1e}"  # p. ej. 8.9e-16
    mant, _, pot = exp.partition("e")
    datos = {
        "fuente": "docs/video/remotion/datos.py sobre reports/kmeans_distribuido_local_{1..4}.json y api_latencia.json",
        "puerto_nodos": 7070,
        "puerto_http": 8080,
        "filas": tres["gold"]["n"],
        "filas_txt": miles(tres["gold"]["n"]),
        "k": tres["receta"]["k"],
        "rangos": [[r["rango"]["desde"], r["rango"]["hasta"]] for r in tres["nodos"]],
        "rangos_txt": [f"[{miles(r['rango']['desde'])}, {miles(r['rango']['hasta'])})" for r in tres["nodos"]],
        "workers_por_nodo": tres["nodos"][0]["workers"],
        "inercias": [round(x, 3) for x in tres["inercias_por_iteracion"]],
        # Los centroides estandarizados del final, tal como viajan en ITERAR (k×D, por filas).
        "centroides_txt": ",".join(f"{v:.2f}" for fila in tres["centroides_estandarizados"] for v in fila),
        "iteraciones": tres["iteraciones"],
        "corridas": corridas,
        "dif_max": {"mantisa": mant.replace(".", ","), "exponente": str(int(pot))},
        "api_p99_peor_ms": round(peor["p99_ms"], 2),
        "api_clientes_peor": peor["clientes"],
    }
    SALIDA.write_text(json.dumps(datos, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(f"escrito {SALIDA.relative_to(RAIZ)}")


if __name__ == "__main__":
    main()
