"""K-Means (Lloyd) del gold de OEFA en la GPU, para compararlo con el de Go.

Es un experimento del laboratorio, no parte del sistema: el sistema es Go y Python
queda en el lado del laboratorio, como los notebooks. Responde dos preguntas del
informe de la PC4:

1. ¿Da lo mismo? Corre Lloyd desde el mismo arranque que eligió cmd/entrenar
   (reports/centroides_iniciales.json) y compara centroides, iteraciones e inercia
   con reports/kmeans.json, en fp64 y en fp32.
2. ¿Desde qué tamaño conviene? Mide ms por iteración con el gold replicado r veces
   (1, 10, 100, ...), para ponerlo al lado de Go secuencial, Go concurrente y el
   clúster (reports/benchmark_kmeans.json, reports/smoke_lab_*.json).

Respeta el contrato numérico de kmeans/doc.go: distancia euclídea al cuadrado,
desempate por el centroide de menor índice, clusters vacíos que conservan su
centroide, parada max_j ||mu_j(t) - mu_j(t-1)||inf <= tol (1 + max_j ||mu_j(t-1)||inf)
e inercia con los centroides finales.

Lo que no puede respetar es el orden de la suma, y eso es parte del experimento. Hay
dos maneras de sumar las filas de cada cluster en la GPU:

- atomica (index_add_): miles de hilos suman con operaciones atómicas en el orden
  en que llegan, que cambia de corrida en corrida; los últimos bits también.
- matmul (one_hot(asignación)^T @ X): una multiplicación de matrices, con un orden
  de reducción fijo para la misma forma: se repite bit a bit.

Es la misma lección que la fusión en orden de chunk de Go, con miles de hilos.

    python kmeans_gpu.py --gold oefa_features.csv --kmeans kmeans.json \\
        --inicios centroides_iniciales.json --replicas 1,10,100,300
"""

from __future__ import annotations

import argparse
import hashlib
import json
import platform
import statistics
import sys
import time
from pathlib import Path

import numpy as np
import torch


def decir(texto: str = "") -> None:
    print(texto, flush=True)


def sha256(ruta: Path) -> str:
    h = hashlib.sha256()
    with open(ruta, "rb") as f:
        for bloque in iter(lambda: f.read(1 << 20), b""):
            h.update(bloque)
    return h.hexdigest()


def leer_gold(ruta: Path, features: list[str]) -> np.ndarray:
    """Las columnas de la receta del gold (estacion,fecha_hora,<features>), en float64."""
    with open(ruta, encoding="utf-8") as f:
        cabecera = f.readline().strip().split(",")
    columnas = [cabecera.index(c) for c in features]
    return np.loadtxt(ruta, delimiter=",", skiprows=1, usecols=columnas, dtype=np.float64, ndmin=2)


def sincronizar(dev: torch.device) -> None:
    if dev.type == "cuda":
        torch.cuda.synchronize()


def asignar(X: torch.Tensor, C: torch.Tensor) -> tuple[torch.Tensor, torch.Tensor]:
    """Cluster más cercano de cada fila y su distancia al cuadrado.

    Recorre los k centroides en orden y solo cambia de cluster con un '<' estricto:
    un empate se queda con el índice menor, como en Go. Calcula (x - c)^2 directo,
    sin la expansión x^2 - 2xc + c^2, que redondea distinto.
    """
    mejor = ((X - C[0]) ** 2).sum(1)
    etiqueta = torch.zeros(X.shape[0], dtype=torch.long, device=X.device)
    for j in range(1, C.shape[0]):
        d = ((X - C[j]) ** 2).sum(1)
        menor = d < mejor
        mejor = torch.where(menor, d, mejor)
        etiqueta = torch.where(menor, j, etiqueta)
    return etiqueta, mejor


def sumar(X: torch.Tensor, etiqueta: torch.Tensor, k: int, modo: str) -> tuple[torch.Tensor, torch.Tensor]:
    """Sumas (k, D) y conteos (k,) por cluster."""
    if modo == "atomica":
        sumas = torch.zeros(k, X.shape[1], dtype=X.dtype, device=X.device).index_add_(0, etiqueta, X)
        conteos = torch.bincount(etiqueta, minlength=k)
    else:
        uno = torch.nn.functional.one_hot(etiqueta, k).to(X.dtype)
        sumas = uno.T @ X
        conteos = torch.bincount(etiqueta, minlength=k)
    return sumas, conteos


def lloyd(X, C0, tol, max_iter, modo, iter_fijas=0):
    """Lloyd completo (o iter_fijas iteraciones sin mirar la tolerancia, para medir).

    Devuelve centroides, iteraciones, motivo de parada, inercia final y los ms de
    cada iteración (incluida la lectura del movimiento, que sincroniza como en Go).
    """
    dev = X.device
    C = C0.clone()
    tiempos = []
    paro = "max_iter"
    tope = iter_fijas or max_iter
    it = 0
    for it in range(1, tope + 1):
        sincronizar(dev)
        t0 = time.perf_counter()
        etiqueta, _ = asignar(X, C)
        sumas, conteos = sumar(X, etiqueta, C.shape[0], modo)
        vacios = conteos == 0
        nuevos = torch.where(vacios[:, None], C, sumas / conteos.clamp(min=1)[:, None].to(X.dtype))
        movimiento = (nuevos - C).abs().amax().item()
        referencia = C.abs().amax().item()
        C = nuevos
        tiempos.append((time.perf_counter() - t0) * 1000)
        if not iter_fijas and movimiento <= tol * (1 + referencia):
            paro = "tolerancia"
            break
    _, d = asignar(X, C)
    return C, it, paro, d.sum().item(), tiempos


def main() -> int:
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("--gold", type=Path, default=Path("oefa_features.csv"))
    p.add_argument("--kmeans", type=Path, default=Path("kmeans.json"), help="reports/kmeans.json de Go")
    p.add_argument("--inicios", type=Path, default=Path("centroides_iniciales.json"))
    p.add_argument("--replicas", default="1,10,100,300", help="veces que se replica el gold para medir velocidad")
    p.add_argument("--iter-fijas", type=int, default=10, help="iteraciones por medición de velocidad")
    p.add_argument("--repeticiones", type=int, default=5, help="mediciones por tamaño")
    p.add_argument("--cpu", action="store_true", help="forzar CPU (para comparar torch en CPU)")
    p.add_argument("--salida", type=Path, default=None)
    a = p.parse_args()

    dev = torch.device("cuda" if torch.cuda.is_available() and not a.cpu else "cpu")
    equipo = platform.node()
    go = json.loads(a.kmeans.read_text(encoding="utf-8"))
    receta = go["receta"]
    inicios = json.loads(a.inicios.read_text(encoding="utf-8"))
    C0 = np.array(inicios["inicios"][go["arranque_elegido"]]["centroides"], dtype=np.float64)
    C_go = np.array(go["centroides_estandarizados"], dtype=np.float64)

    decir("=" * 78)
    decir(f"K-Means del gold en {equipo} | torch {torch.__version__} | {dev.type}")
    if dev.type == "cuda":
        prop = torch.cuda.get_device_properties(0)
        decir(f"GPU: {prop.name} | sm_{prop.major}{prop.minor} | {prop.total_memory / 2**30:.1f} GB | CUDA {torch.version.cuda}")
    huella = sha256(a.gold)
    X_np = leer_gold(a.gold, receta["features"])
    decir(f"gold: {X_np.shape[0]:,} filas x {X_np.shape[1]} features | sha256 {huella[:12]}… "
          f"({'el mismo' if huella == go['gold']['sha256'] else 'OTRO gold que el de kmeans.json'})")
    decir(f"receta: k={receta['k']}, tolerancia {receta['tolerancia']}, arranque {go['arranque_elegido']} "
          f"(semilla {inicios['inicios'][go['arranque_elegido']]['semilla']}); Go: {go['iteraciones']} iteraciones, "
          f"inercia {go['inercia']:.6f}")
    decir("=" * 78)

    # 1. Convergencia: ¿da lo mismo que Go?
    convergencia = []
    decir("1) Lloyd completo desde el arranque de Go")
    for dtype in (torch.float64, torch.float32):
        X = torch.from_numpy(X_np).to(dev, dtype)
        C0_t = torch.from_numpy(C0).to(dev, dtype)
        for modo in ("matmul", "atomica"):
            corridas = [lloyd(X, C0_t, receta["tolerancia"], receta["max_iter"], modo) for _ in range(3)]
            C, it, paro, inercia, tiempos = corridas[0]
            C_np = C.double().cpu().numpy()
            repetible = all(torch.equal(C, otra[0]) for otra in corridas[1:])
            fila = {
                "precision": str(dtype).removeprefix("torch."),
                "suma": modo,
                "iteraciones": it,
                "paro": paro,
                "inercia": inercia,
                "dif_inercia_go": inercia - go["inercia"],
                "dif_max_centroides_go": float(np.abs(C_np - C_go).max()),
                "repetible_bit_a_bit": repetible,
                "ms_por_iteracion_mediana": statistics.median(tiempos),
                "centroides": C_np.tolist(),
            }
            convergencia.append(fila)
            decir(f"   {fila['precision']:<7} {modo:<7} {it:>3} iter ({paro}) | inercia {inercia:.6f} | "
                  f"máx |C - C_go| {fila['dif_max_centroides_go']:.2e} | repetible: {'sí' if repetible else 'NO'} | "
                  f"{fila['ms_por_iteracion_mediana']:.3f} ms/iter")

    # 2. Velocidad con el gold replicado
    velocidad = []
    decir()
    decir(f"2) ms por iteración ({a.iter_fijas} iteraciones fijas, mediana de {a.repeticiones}) con el gold replicado")
    decir(f"   {'réplicas':>8} {'filas':>12} {'precisión':<8} {'suma':<7} {'subir (ms)':>10} {'ms/iter':>9} {'Mfilas/s':>9}")
    for r in (int(x) for x in a.replicas.split(",")):
        for dtype in (torch.float64, torch.float32):
            try:
                sincronizar(dev)
                t0 = time.perf_counter()
                X = torch.from_numpy(np.tile(X_np, (r, 1))).to(dev, dtype)
                sincronizar(dev)
                subir = (time.perf_counter() - t0) * 1000
                C0_t = torch.from_numpy(C0).to(dev, dtype)
                for modo in ("matmul", "atomica"):
                    lloyd(X, C0_t, receta["tolerancia"], receta["max_iter"], modo, iter_fijas=2)  # calentamiento
                    medianas = [statistics.median(lloyd(X, C0_t, 0, 0, modo, iter_fijas=a.iter_fijas)[4])
                                for _ in range(a.repeticiones)]
                    ms = statistics.median(medianas)
                    fila = {"replicas": r, "filas": X.shape[0], "precision": str(dtype).removeprefix("torch."),
                            "suma": modo, "ms_subir_datos": subir, "ms_por_iteracion": ms,
                            "millones_filas_por_s": X.shape[0] / ms / 1000}
                    velocidad.append(fila)
                    decir(f"   {r:>8} {X.shape[0]:>12,} {fila['precision']:<8} {modo:<7} {subir:>10.1f} {ms:>9.3f} "
                          f"{fila['millones_filas_por_s']:>9.1f}")
                del X
                if dev.type == "cuda":
                    torch.cuda.empty_cache()
            except torch.OutOfMemoryError:
                decir(f"   {r:>8} {'':>12} {str(dtype).removeprefix('torch.'):<8} sin memoria en la GPU")
                velocidad.append({"replicas": r, "precision": str(dtype).removeprefix("torch."), "error": "sin memoria"})
                if dev.type == "cuda":
                    torch.cuda.empty_cache()

    resultado = {
        "equipo": equipo,
        "dispositivo": torch.cuda.get_device_name(0) if dev.type == "cuda" else f"cpu ({platform.processor() or platform.machine()})",
        "torch": torch.__version__,
        "cuda": torch.version.cuda if dev.type == "cuda" else None,
        "gold_sha256": huella,
        "mismo_gold_que_go": huella == go["gold"]["sha256"],
        "go": {"iteraciones": go["iteraciones"], "inercia": go["inercia"], "centroides": C_go.tolist()},
        "convergencia": convergencia,
        "velocidad": velocidad,
        "memoria_pico_gb": torch.cuda.max_memory_allocated() / 2**30 if dev.type == "cuda" else None,
    }
    salida = a.salida or Path(f"kmeans_gpu_{equipo}.json")
    salida.write_text(json.dumps(resultado, indent=2, ensure_ascii=False), encoding="utf-8")
    decir("=" * 78)
    decir(f"LISTO: resultado en {salida}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
