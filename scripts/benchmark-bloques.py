# /// script
# requires-python = ">=3.11"
# ///
"""Ejecuta, agrupa y resume el benchmark del cargador desde la raíz del repositorio."""

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


def tamanos_en_orden(valor):
    try:
        tamanos = [int(t) for t in valor.split(",")]
    except ValueError:
        raise argparse.ArgumentTypeError("--tamanos requiere enteros separados por comas") from None
    if not tamanos or any(t <= 0 for t in tamanos) or len(set(tamanos)) != len(tamanos):
        raise argparse.ArgumentTypeError("los tamaños deben ser positivos y no repetirse")
    return tamanos


def mediciones_por_worker(corrida):
    mediciones = {m["workers"]: m for m in corrida["mediciones"]}
    if len(mediciones) != len(corrida["mediciones"]):
        raise ValueError("hay workers duplicados")
    return mediciones


def agrupar(rutas):
    """Normaliza solo datos del equipo y rutas; conserva las mediciones originales."""
    corridas = [json.loads(r.read_text(encoding="utf-8")) for r in rutas]
    for corrida in corridas:
        corrida["entrada"] = "data/bronze/oefa.csv"
        corrida.pop("host", None)
        corrida.get("metadatos", {}).pop("maquina", None)
    tamanos, workers = corridas[:3], corridas[3]
    if {c["bloque_bytes"] for c in tamanos} != {16 * 2**20, 64 * 2**20, 128 * 2**20}:
        raise ValueError("los tres primeros JSON deben ser los bloques de 16, 64 y 128 MiB")
    if workers["bloque_bytes"] != 64 * 2**20:
        raise ValueError("el último JSON debe ser la prueba de workers con bloques de 64 MiB")
    for corrida, requeridos in [(c, {1, 4}) for c in tamanos] + [(workers, {1, 2, 4, 8, 12})]:
        if set(mediciones_por_worker(corrida)) != requeridos:
            raise ValueError("los workers de una corrida no corresponden a su experimento")
        if corrida["sha256"] != SHA256_OEFA or not corrida["identicos"]:
            raise ValueError("la corrida no conserva el CSV del informe o sus salidas")
    return {"tamanos": tamanos, "workers": workers}


def resumir(ruta, seccion):
    datos = json.loads(ruta.read_text(encoding="utf-8"))
    contexto = datos["workers"]
    meta = contexto["metadatos"]
    lineas = [
        "Resumen de los JSON de la corrida (no es una nueva medición)",
        f"Fuente: {ruta.as_posix()}",
        f"SHA-256 del JSON: {sha256(ruta)}",
        f"Compilación: {meta['commit'][:7]}; árbol modificado: {meta['arbol_sucio']}",
        f"{contexto['go']} · {contexto['plataforma']} · {contexto['cpus']} CPU lógicas",
        f"CSV: {contexto['filas_largas']:,} registros; {contexto['repeticiones']} repeticiones/configuración",
        "",
    ]
    if seccion in ("bloques", "todo"):
        lineas += ["Bloque (MiB) | Bloques | Mediana W=1 (s) | Mediana W=4 (s) | Cociente de medianas"]
        for corrida in datos["tamanos"]:
            med = mediciones_por_worker(corrida)
            lineas.append(f"{corrida['bloque_bytes'] // 2**20:12} | {corrida['bloques']:7} | "
                          f"{med[1]['mediana_s']:15.3f} | {med[4]['mediana_s']:15.3f} | {med[4]['speedup']:19.2f}")
        lineas += ["Orden de tamaños: " + ", ".join(str(c["bloque_bytes"] // 2**20) for c in datos["tamanos"]) + " MiB", ""]
    if seccion in ("workers", "todo"):
        lineas += [f"Bloques de {contexto['bloque_bytes'] // 2**20} MiB",
                   "Workers | Mediana (s) | Cociente de medianas | Cociente de medias recortadas"]
        for w, med in sorted(mediciones_por_worker(contexto).items()):
            lineas.append(f"{w:7} | {med['mediana_s']:11.3f} | {med['speedup']:19.2f} | {med['speedup_ic']['punto']:28.2f}")
        dispersion = max(max(m["tiempos_s"]) / min(m["tiempos_s"]) for m in contexto["mediciones"])
        lineas += [f"Dispersión máxima por W (máximo/mínimo): {dispersion:.1f}", ""]
    huellas = {(m["sha256_silver"], m["sha256_gold"])
               for corrida in [*datos["tamanos"], contexto] for m in corrida["mediciones"]}
    if len(huellas) != 1:
        raise ValueError("las huellas de salida no coinciden entre configuraciones")
    silver, gold = huellas.pop()
    lineas += [f"Silver SHA-256: {silver}", f"Gold SHA-256:   {gold}"]
    return "\n".join(lineas)


def ejecutar_cargador(cargador, argumentos, registro, timeout):
    """Separa el ejecutable local validado de sus argumentos, sin usar un shell."""
    # executable fija el archivo que se ejecuta; «cargar» es solo la etiqueta argv[0].
    # Los espacios, comillas y metacaracteres de las rutas siguen siendo datos.
    return subprocess.run(
        ["cargar", *argumentos], executable=str(cargador),
        stdout=registro, stderr=subprocess.STDOUT, shell=False, timeout=timeout,
    )


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("cargador", type=binario_local, nargs="?",
                        help="Binario de cmd/cargar compilado localmente")
    parser.add_argument("entrada", type=Path, nargs="?", help="CSV de entrada")
    parser.add_argument("salida", type=Path, nargs="?", help="Directorio nuevo para la evidencia")
    modo = parser.add_mutually_exclusive_group()
    modo.add_argument("--resumir", type=Path, metavar="JSON", help="Resume el JSON agrupado sin volver a medir")
    modo.add_argument("--agrupar", type=Path, nargs=4, metavar="JSON", help="Agrupa tres pruebas de bloques y una de workers")
    parser.add_argument("--destino", type=Path, help="JSON de salida para --agrupar")
    parser.add_argument("--seccion", choices=("bloques", "workers", "todo"), default="todo")
    parser.add_argument("--tamanos", type=tamanos_en_orden, default=[16, 64, 128],
                        help="Tamaños en orden de ejecución, separados por comas (16,64,128)")
    parser.add_argument("--piloto", action="store_true", help="Usa KiB para una muestra")
    parser.add_argument("--timeout", type=float, default=600,
                        help="Tiempo máximo por tamaño, en segundos (por defecto: 600)")
    args = parser.parse_args()
    if args.resumir or args.agrupar:
        if any(v is not None for v in (args.cargador, args.entrada, args.salida)):
            parser.error("--resumir y --agrupar no usan los argumentos de una nueva medición")
        try:
            if args.resumir:
                resumen = resumir(args.resumir, args.seccion)
                print(resumen)
            else:
                if not args.destino:
                    parser.error("--agrupar necesita --destino")
                datos = agrupar(args.agrupar)
                args.destino.parent.mkdir(parents=True, exist_ok=True)
                args.destino.write_text(json.dumps(datos, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
                print(f"Agrupado: {args.destino}")
        except (OSError, ValueError, KeyError) as error:
            parser.error(str(error))
        return
    if any(v is None for v in (args.cargador, args.entrada, args.salida)):
        parser.error("una nueva medición necesita cargador, entrada y salida")
    if args.destino:
        parser.error("--destino requiere --agrupar")
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
    for tamano in args.tamanos:
        corrida = args.salida / f"{tamano}{unidad}"
        corrida.mkdir()
        silver = corrida / "silver.csv"
        gold = corrida / "gold.csv"
        argumentos = [
            "-entrada", str(args.entrada),
            "-bloque", f"{tamano}{sufijo}", "-workers", "1,4",
            "-bench", str(REPETICIONES), "-silver", str(silver),
            "-gold", str(gold), "-reports", str(corrida),
        ]
        registro = corrida / "terminal.txt"
        print(f"Ejecutando {tamano} {unidad} con 1 y 4 workers", flush=True)
        with registro.open("w", encoding="utf-8") as log:
            log.write(f"Argumentos: {json.dumps([str(args.cargador), *argumentos], ensure_ascii=False)}\n\n")
            log.flush()
            try:
                resultado = ejecutar_cargador(args.cargador, argumentos, log, args.timeout)
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
        por_w = mediciones_por_worker(informe)
        base, paralelo = por_w[1], por_w[4]
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
        "Orden de tamaños: " + ", ".join(map(str, args.tamanos)) + f" {unidad}", "",
        *tabla, "", f"Silver SHA-256: `{silver_sha}`", "",
        f"Gold SHA-256: `{gold_sha}`", "",
    ])
    (args.salida / "resumen.md").write_text(resumen, encoding="utf-8")
    print(resumen)


if __name__ == "__main__":
    main()
