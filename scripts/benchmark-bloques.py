"""Ejecuta y compara el benchmark del cargador desde la raíz del repositorio."""

import argparse
import hashlib
import json
import math
import os
import subprocess
from pathlib import Path


SHA256_OEFA = "1f09a8064ee0aed0b3a9a0a90b96f63ce226b45a969dace49f73e45134f26d92"
REPETICIONES = 10


def sha256(ruta):
    with ruta.open("rb") as archivo:
        return hashlib.file_digest(archivo, "sha256").hexdigest()


def binario_local(valor):
    ruta = Path(valor).resolve()
    if not ruta.is_file():
        raise argparse.ArgumentTypeError("el cargador debe ser un archivo existente")
    if ruta.suffix.lower() in {".bat", ".cmd"}:
        raise argparse.ArgumentTypeError("usa el binario de cmd/cargar, no un script de shell")
    if os.name == "nt" and ruta.suffix.lower() != ".exe":
        raise argparse.ArgumentTypeError("el cargador en Windows debe ser un binario .exe")
    return ruta


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("cargador", type=binario_local,
                        help="Binario de cmd/cargar compilado localmente")
    parser.add_argument("entrada", type=Path, help="CSV de entrada")
    parser.add_argument("salida", type=Path, help="Directorio nuevo para la evidencia")
    parser.add_argument("--piloto", action="store_true", help="Usa KiB para una muestra")
    parser.add_argument("--timeout", type=float, default=600,
                        help="Tiempo máximo por tamaño, en segundos (por defecto: 600)")
    args = parser.parse_args()
    if not math.isfinite(args.timeout) or args.timeout <= 0:
        parser.error("--timeout tiene que ser un número positivo y finito")

    huella_entrada = sha256(args.entrada)
    if not args.piloto and huella_entrada != SHA256_OEFA:
        parser.error(f"el CSV no coincide con el del informe (SHA-256: {huella_entrada})")

    args.salida.mkdir(parents=True)
    sufijo, unidad = ("KB", "KiB") if args.piloto else ("MB", "MiB")
    tabla = [
        "| Tamaño | Bloques | Mediana con 1 worker | Mediana con 4 workers | Mejora |",
        "|---|---:|---:|---:|---:|",
    ]
    huellas = set()
    for tamano in (16, 64, 128):
        corrida = args.salida / f"{tamano}{unidad}"
        corrida.mkdir()
        silver = corrida / "silver.csv"
        gold = corrida / "gold.csv"
        comando = [
            str(args.cargador), "-entrada", str(args.entrada),
            "-bloque", f"{tamano}{sufijo}", "-workers", "1,4",
            "-bench", str(REPETICIONES), "-silver", str(silver),
            "-gold", str(gold), "-reports", str(corrida),
        ]
        registro = corrida / "terminal.txt"
        print(f"Ejecutando {tamano} {unidad} con 1 y 4 workers", flush=True)
        with registro.open("w", encoding="utf-8") as log:
            log.write(f"Argumentos: {json.dumps(comando, ensure_ascii=False)}\n\n")
            log.flush()
            try:
                # Auditoría de inyección: binario local de confianza, argv separado y sin shell.
                # Los valores del CSV y las rutas no se interpretan como comandos.
                # nosemgrep: python.lang.security.audit.dangerous-subprocess-use-audit, python.lang.security.audit.dangerous-subprocess-use-tainted-env-args
                resultado = subprocess.run(comando, stdout=log, stderr=subprocess.STDOUT,
                                           shell=False, timeout=args.timeout)
            except subprocess.TimeoutExpired:
                raise SystemExit(
                    f"La corrida de {tamano} {unidad} superó {args.timeout:g} s. Revisa {registro}"
                ) from None
        if resultado.returncode:
            raise SystemExit(f"Falló la corrida de {tamano} {unidad}. Revisa {registro}")

        informe = json.loads((corrida / "benchmark_cargador.json").read_text(encoding="utf-8"))
        if informe["sha256"] != huella_entrada or not informe["identicos"]:
            raise SystemExit(f"No se conservó la entrada o la salida en {corrida}")
        huellas.add((sha256(silver), sha256(gold)))
        base, paralelo = informe["mediciones"]
        tabla.append(
            f"| {tamano} {unidad} | {informe['bloques']} | "
            f"{base['mediana_s'] * 1000:.3f} ms | {paralelo['mediana_s'] * 1000:.3f} ms | "
            f"{paralelo['speedup']:.2f}× |"
        )

    if len(huellas) != 1:
        raise SystemExit("Silver o gold cambiaron entre tamaños de bloque. Revisa los JSON.")
    silver_sha, gold_sha = huellas.pop()
    alcance = "Piloto con muestra" if args.piloto else "Corrida con el CSV completo"
    resumen = "\n".join([
        f"# {alcance}", "", f"Entrada SHA-256: `{huella_entrada}`", "",
        *tabla, "", f"Silver SHA-256: `{silver_sha}`", "",
        f"Gold SHA-256: `{gold_sha}`", "",
    ])
    (args.salida / "resumen.md").write_text(resumen, encoding="utf-8")
    print(resumen)


if __name__ == "__main__":
    main()
