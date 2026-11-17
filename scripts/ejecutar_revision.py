# /// script
# requires-python = ">=3.12"
# dependencies = ["nbclient", "ipykernel", "pandas", "matplotlib", "scikit-learn", "scipy", "joblib"]
# ///
"""Ejecuta los notebooks existentes en una raíz de datos separada, sin editar sus fuentes."""
import argparse
import hashlib
import importlib.metadata
import json
import os
import platform
import subprocess
import sys
import time
from pathlib import Path

import nbformat
from jupyter_client import KernelManager
from jupyter_client.kernelspec import KernelSpec
from nbclient import NotebookClient


def sha256(ruta: Path) -> str:
    h = hashlib.sha256()
    with ruta.open("rb") as archivo:
        for bloque in iter(lambda: archivo.read(1024 * 1024), b""):
            h.update(bloque)
    return h.hexdigest()


def escribir_ejecucion(repo: Path, bronze: Path, raiz: Path) -> None:
    reports = raiz / "reports"
    silver = raiz / "data/silver/oefa_ancho.csv"
    gold = raiz / "data/gold/oefa_features.csv"
    auditoria = json.loads((reports / "auditoria.json").read_text(encoding="utf-8"))
    fuentes = [
        "configs/limpieza.json", "configs/kmeans.json", "notebooks/aire_lab.py",
        "notebooks/01_eda_criba.ipynb", "notebooks/03_auditoria_figuras.ipynb",
        "scripts/ejecutar_revision.py",
    ]
    paquetes = ("pandas", "numpy", "scikit-learn", "matplotlib", "scipy", "joblib")
    entorno = " · ".join([f"Python {platform.python_version()}"] +
                          [f"{p} {importlib.metadata.version(p)}" for p in paquetes])
    go = subprocess.run(["go", "version"], check=True, capture_output=True, text=True).stdout.strip()
    commit = subprocess.run(["git", "rev-parse", "HEAD"], cwd=repo, check=True,
                            capture_output=True, text=True).stdout.strip()
    rama = subprocess.run(["git", "branch", "--show-current"], cwd=repo, check=True,
                          capture_output=True, text=True).stdout.strip()
    estados = [c["estado"] for c in auditoria["chequeos"]]
    datos = {
        "fecha_auditoria": auditoria.get("fecha"), "rama": rama, "commit_base": commit,
        "entorno_python": entorno, "go": go,
        "bronze_sha256": sha256(bronze), "bronze_bytes": bronze.stat().st_size,
        "silver_sha256": sha256(silver), "gold_sha256": sha256(gold),
        "auditoria": {estado: estados.count(estado) for estado in sorted(set(estados))},
        "benchmarks": "Referencias históricas copiadas del repositorio; no se volvieron a medir.",
        "inicios": "Centroides iniciales publicados, reutilizados para comparar los mismos arranques.",
        "fuentes_sha256": {ruta: sha256(repo / ruta) for ruta in fuentes},
    }
    (reports / "ejecucion.json").write_text(
        json.dumps(datos, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--bronze", type=Path, required=True)
    parser.add_argument("--raiz", type=Path, required=True, help="raíz con data/ y reports/ de la revisión")
    parser.add_argument("--notebooks", nargs="+", choices=["01", "03"], default=["01", "03"])
    args = parser.parse_args()
    repo = Path(__file__).resolve().parents[1]
    bronze, raiz = args.bronze.resolve(), args.raiz.resolve()
    if not bronze.is_file():
        parser.error("No existe el CSV bronze")
    if raiz == repo:
        parser.error("Usa una raíz separada para conservar los reports/ originales")
    ejecutados = raiz / "ejecutados"
    ejecutados.mkdir(parents=True, exist_ok=True)
    os.environ.update(AIRE_RAIZ=str(raiz), AIRE_BRONZE=str(bronze),
                      AIRE_FIGURAS=str(raiz / "reports/figuras"),
                      MPLBACKEND="module://matplotlib_inline.backend_inline", TZ="America/Lima")
    if hasattr(time, "tzset"):
        time.tzset()
    nombres = {"01": "01_eda_criba", "03": "03_auditoria_figuras"}
    for numero in args.notebooks:
        nombre = nombres[numero]
        print(f"INICIO {nombre}", flush=True)
        nb = nbformat.read(repo / "notebooks" / f"{nombre}.ipynb", as_version=4)
        nbformat.validate(nb)
        km = KernelManager()
        km._kernel_spec = KernelSpec(
            argv=[sys.executable, "-m", "ipykernel_launcher", "-f", "{connection_file}"],
            display_name="Python de la revisión", language="python")
        cliente = NotebookClient(nb, km=km, timeout=900,
                                 resources={"metadata": {"path": str(repo)}})
        try:
            cliente.execute()
        finally:
            nbformat.write(nb, ejecutados / f"{nombre}.ipynb")
            with (ejecutados / f"{nombre}.log").open("w", encoding="utf-8") as salida:
                for celda in nb.cells:
                    for resultado in celda.get("outputs", []):
                        if resultado.output_type == "stream":
                            salida.write(resultado.text)
                        elif resultado.output_type == "error":
                            salida.write("\n".join(resultado.traceback))
            if km.has_kernel:
                km.shutdown_kernel(now=True)
        print(f"FIN {nombre}: {ejecutados / (nombre + '.ipynb')}", flush=True)
    escribir_ejecucion(repo, bronze, raiz)
    print(f"EVIDENCIA {raiz / 'reports/ejecucion.json'}", flush=True)


if __name__ == "__main__":
    main()
