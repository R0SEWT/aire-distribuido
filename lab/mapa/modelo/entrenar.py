"""M2 en un solo dispositivo: entrena con las estaciones fuera de la partición k y evalúa en ellas.

    uv run python entrenar.py --datos /tmp/mapa-sintetico --particion 0 --pasos 300 --ancho 8 \
        --salida reports/mapa_m2_p0.json --pesos-salida pesos/

Usa cuda si hay (autocast fp16 solo en cuda), si no la CPU. Las particiones son las de
datos.particiones (5 grupos de estaciones, deterministas con --semilla-particiones), las mismas que
base.py usa para M0 y M1. Escribe un JSON con la historia de la pérdida y las métricas por estación
(metricas.por_estacion) sobre las estaciones que el modelo no vio.

Con más días de los que caben en memoria, --memoria mmap lee cada ventana del disco.
"""

from __future__ import annotations

import argparse
import json
import time
from pathlib import Path

import numpy as np
import torch

import datos
import metricas
import red


@torch.no_grad()
def predecir_en_estaciones(modelo: torch.nn.Module, conjunto: datos.Conjunto, horas: list[tuple[str, int]],
                           lote: int = 4, amp: bool = False) -> dict:
    """Predicción en µg/m³ en las celdas de las estaciones de validación, una fila por estación-hora."""
    modelo.eval()
    e = conjunto.etiquetas
    est, dia, obs, pred = [], [], [], []
    for k in range(0, len(horas), lote):
        hs = horas[k:k + lote]
        with torch.autocast("cuda", dtype=torch.float16, enabled=amp):
            p = modelo(conjunto.entrada(hs)).float()
        p = torch.expm1(p.clamp_min(0)).cpu().numpy()
        for b, clave in enumerate(hs):
            f = conjunto.filas_val[clave]
            est.extend(e.estacion[f].tolist())
            dia.extend(e.dia[f].tolist())
            obs.append(e.valores[f])
            pred.append(p[b][:, e.i[f], e.j[f]].T)
    vacio = np.empty((0, 2), np.float32)
    return {"estacion": np.array(est, dtype=object), "dia": np.array(dia, dtype=object),
            "obs": np.concatenate(obs) if obs else vacio, "pred": np.concatenate(pred) if pred else vacio}


def filtrar_dias(todos: list[str], rango: str | None) -> list[str]:
    if not rango:
        return todos
    desde, hasta = rango.split(":")
    return [d for d in todos if (not desde or d >= desde) and (not hasta or d <= hasta)]


def main() -> None:
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("--datos", required=True, help="carpeta (o URL) con manifiesto.json")
    p.add_argument("--particion", type=int, default=0)
    p.add_argument("--particiones", type=int, default=datos.N_PARTICIONES)
    p.add_argument("--semilla-particiones", type=int, default=0)
    p.add_argument("--dias", default=None, help="AAAA-MM-DD:AAAA-MM-DD (por defecto, todos)")
    p.add_argument("--pasos", type=int, default=1000)
    p.add_argument("--lote", type=int, default=4)
    p.add_argument("--tasa", type=float, default=1e-3)
    p.add_argument("--decaimiento", type=float, default=1e-4)
    p.add_argument("--ancho", type=int, default=32)
    p.add_argument("--semilla", type=int, default=0)
    p.add_argument("--pesos-iniciales", default=None, help="pesos_0.bin (con su pesos.json al lado)")
    p.add_argument("--evaluar-cada", type=int, default=100)
    p.add_argument("--muestras-val", type=int, default=64, help="horas de validación para la curva")
    p.add_argument("--memoria", choices=["auto", "ram", "gpu", "mmap"], default="auto")
    p.add_argument("--cpu", action="store_true")
    p.add_argument("--salida", required=True)
    p.add_argument("--pesos-salida", default=None, help="carpeta para pesos_p<k>.bin y pesos.json")
    a = p.parse_args()

    dev = torch.device("cuda" if torch.cuda.is_available() and not a.cpu else "cpu")
    origen = datos.Origen(a.datos)
    man = origen.manifiesto
    etiquetas = datos.leer_etiquetas(origen.ruta("etiquetas.csv"))
    pliegues = datos.particiones(etiquetas.estaciones(), a.particiones, a.semilla_particiones)
    validacion = pliegues[a.particion]
    dias = filtrar_dias(man.dias, a.dias)
    memoria = a.memoria if a.memoria != "auto" else ("gpu" if dev.type == "cuda" else "ram")
    serie = datos.Serie.cargar(origen, datos.dias_a_bajar(dias, man.dias),
                               dispositivo=dev if memoria == "gpu" else "cpu", mmap=memoria == "mmap")
    conjunto = datos.Conjunto(serie, datos.cargar_estaticos(origen.ruta("estaticos.npy")), etiquetas, dias,
                              validacion, dev)
    if a.pesos_iniciales:
        ruta_json = Path(a.pesos_iniciales).with_name("pesos.json")
        arq = json.loads(ruta_json.read_text(encoding="utf-8"))["arquitectura"]
        modelo = red.cargar_modelo(a.pesos_iniciales, ruta_json)
    else:
        arq = red.arquitectura(man.n_entrada, a.ancho)
        modelo = red.construir(arq, semilla=a.semilla)
    if arq["entrada"] != man.n_entrada:
        raise SystemExit(f"los pesos esperan {arq['entrada']} canales de entrada y el manifiesto da {man.n_entrada}")
    modelo = modelo.to(dev)
    ent = red.Entrenador(modelo, conjunto, a.lote, a.tasa, a.decaimiento, a.semilla)
    rng = np.random.default_rng(a.semilla)
    n_val = min(a.muestras_val, len(conjunto.horas_val))
    curva_val = [conjunto.horas_val[k] for k in sorted(rng.choice(len(conjunto.horas_val), n_val, replace=False))] \
        if n_val else []
    print(f"M2 partición {a.particion}: validación {validacion}; {len(dias)} días, "
          f"{len(conjunto.horas_ent)} horas de entrenamiento y {len(conjunto.horas_val)} de validación; "
          f"{red.n_parametros(modelo):,} parámetros en {dev}", flush=True)

    historia = []
    t0 = time.perf_counter()
    hechos = 0
    while hechos < a.pasos:
        n = min(a.evaluar_cada, a.pasos - hechos)
        _, perdida = ent.pasos(n)
        hechos += n
        val, _ = ent.perdida(curva_val)
        historia.append({"paso": hechos, "perdida": perdida, "perdida_val": val,
                         "segundos": round(time.perf_counter() - t0, 2)})
        print(f"  paso {hechos}: pérdida {perdida:.4f}, validación {val:.4f}", flush=True)
    segundos = time.perf_counter() - t0

    reg = predecir_en_estaciones(modelo, conjunto, conjunto.horas_val, a.lote, ent.amp)
    salida = {
        "modelo": "M2", "particion": a.particion, "particiones": a.particiones,
        "semilla_particiones": a.semilla_particiones, "validacion": validacion,
        "dias": [dias[0], dias[-1]], "n_dias": len(dias), "horas_entrenamiento": len(conjunto.horas_ent),
        "horas_validacion": len(conjunto.horas_val), "dispositivo": str(dev),
        "hiper": {"pasos": a.pasos, "lote": a.lote, "tasa": a.tasa, "decaimiento": a.decaimiento, "ancho": a.ancho,
                  "semilla": a.semilla, "amp": ent.amp},
        "arquitectura": arq,
        "parametros": red.n_parametros(modelo), "segundos_entrenamiento": round(segundos, 2),
        "ms_por_paso": round(1000 * segundos / max(a.pasos, 1), 1), "historia": historia,
        "metricas": metricas.por_estacion(reg["estacion"], reg["dia"], reg["obs"], reg["pred"]),
    }
    Path(a.salida).parent.mkdir(parents=True, exist_ok=True)
    Path(a.salida).write_text(json.dumps(salida, indent=1, ensure_ascii=False), encoding="utf-8")
    if a.pesos_salida:
        red.exportar(modelo, arq, a.pesos_salida, nombre=f"pesos_p{a.particion}.bin", particion=a.particion,
                     validacion=validacion)
    g = salida["metricas"]["global"]
    print(f"listo en {segundos:.0f} s → {a.salida}; RMSE en estaciones no vistas: "
          f"PM2.5 {g['pm2_5']['ug']['rmse']}, PM10 {g['pm10']['ug']['rmse']} µg/m³")


if __name__ == "__main__":
    main()
