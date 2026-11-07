"""Ejecuta y compara el benchmark del cargador desde la raíz del repositorio."""

import argparse
import hashlib
import json
import subprocess
from pathlib import Path


SHA256_OEFA = "1f09a8064ee0aed0b3a9a0a90b96f63ce226b45a969dace49f73e45134f26d92"
REPETICIONES = 10


def sha256(ruta):
    with ruta.open("rb") as archivo:
        return hashlib.file_digest(archivo, "sha256").hexdigest()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("cargador", type=Path, help="Binario de cmd/cargar")
    parser.add_argument("entrada", type=Path, help="CSV de entrada")
    parser.add_argument("salida", type=Path, help="Directorio nuevo para la evidencia")
    parser.add_argument("--piloto", action="store_true", help="Usa KiB para una muestra")
    args = parser.parse_args()

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
            str(args.cargador.resolve()), "-entrada", str(args.entrada),
            "-bloque", f"{tamano}{sufijo}", "-workers", "1,4",
            "-bench", str(REPETICIONES), "-silver", str(silver),
            "-gold", str(gold), "-reports", str(corrida),
        ]
        registro = corrida / "terminal.txt"
        print(f"Ejecutando {tamano} {unidad} con 1 y 4 workers", flush=True)
        with registro.open("w", encoding="utf-8") as log:
            log.write(f"Argumentos: {json.dumps(comando, ensure_ascii=False)}\n\n")
            log.flush()
            resultado = subprocess.run(comando, stdout=log, stderr=subprocess.STDOUT)
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
