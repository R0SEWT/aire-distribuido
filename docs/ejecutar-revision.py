# /// script
# requires-python = ">=3.12"
# dependencies = ["nbclient", "ipykernel", "pandas", "matplotlib", "scikit-learn", "scipy", "joblib"]
# ///
"""Ejecuta los notebooks existentes en una raíz de datos separada, sin editar sus fuentes."""
import argparse
import os
import sys
import time
from pathlib import Path

import nbformat
from jupyter_client import KernelManager
from nbclient import NotebookClient


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
    time.tzset()
    nombres = {"01": "01_eda_criba", "03": "03_auditoria_figuras"}
    for numero in args.notebooks:
        nombre = nombres[numero]
        print(f"INICIO {nombre}", flush=True)
        nb = nbformat.read(repo / "notebooks" / f"{nombre}.ipynb", as_version=4)
        nbformat.validate(nb)
        km = KernelManager(kernel_name="python3")
        km.kernel_spec.argv = [sys.executable, "-m", "ipykernel_launcher", "-f", "{connection_file}"]
        cliente = NotebookClient(nb, km=km, timeout=900,
                                 resources={"metadata": {"path": str(repo)}})
        try:
            cliente.execute()
        finally:
            nbformat.write(nb, ejecutados / f"{nombre}.ipynb")
            with (ejecutados / f"{nombre}.log").open("w") as salida:
                for celda in nb.cells:
                    for resultado in celda.get("outputs", []):
                        if resultado.output_type == "stream":
                            salida.write(resultado.text)
                        elif resultado.output_type == "error":
                            salida.write("\n".join(resultado.traceback))
            if km.has_kernel:
                km.shutdown_kernel(now=True)
        print(f"FIN {nombre}: {ejecutados / (nombre + '.ipynb')}", flush=True)


if __name__ == "__main__":
    main()
