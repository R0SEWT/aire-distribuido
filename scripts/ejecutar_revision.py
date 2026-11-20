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
from contextlib import contextmanager
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


IGNORADAS_LIMPIEZA = frozenset({"metadatos", "fases_s", "duracion_s", "entrada"})


def diferencias_limpieza(actual: dict, referencia: dict) -> list[str]:
    """Devuelve las claves de limpieza que difieren, salvo metadatos de ejecución."""
    claves = (actual.keys() | referencia.keys()) - IGNORADAS_LIMPIEZA
    return sorted(clave for clave in claves if actual.get(clave) != referencia.get(clave))


@contextmanager
def ejecucion_exclusiva(raiz: Path):
    """Impide que dos procesos mezclen artefactos en la misma raíz de revisión."""
    raiz.parent.mkdir(parents=True, exist_ok=True)
    candado = raiz.parent / f".{raiz.name}.ejecutar_revision.lock"
    try:
        descriptor = os.open(candado, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600)
    except FileExistsError as exc:
        raise RuntimeError(
            f"Ya hay una revisión usando {raiz}; si no hay otro proceso, elimina {candado}",
        ) from exc
    try:
        os.write(descriptor, f"pid={os.getpid()}\n".encode())
        os.close(descriptor)
        yield
    finally:
        try:
            os.close(descriptor)
        except OSError:
            pass
        candado.unlink(missing_ok=True)


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
    limpieza = json.loads((reports / "limpieza.json").read_text(encoding="utf-8"))
    referencia = json.loads((repo / "reports/limpieza.json").read_text(encoding="utf-8"))
    datos = {
        "fecha_auditoria": auditoria.get("fecha"), "rama": rama, "commit_base": commit,
        "entorno_python": entorno, "go": go,
        "bronze_sha256": sha256(bronze), "bronze_bytes": bronze.stat().st_size,
        "silver_sha256": sha256(silver), "gold_sha256": sha256(gold),
        "auditoria": {estado: estados.count(estado) for estado in sorted(set(estados))},
        "diferencias_con_referencia": diferencias_limpieza(limpieza, referencia),
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
    with ejecucion_exclusiva(raiz):
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
