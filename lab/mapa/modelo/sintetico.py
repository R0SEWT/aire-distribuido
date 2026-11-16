"""Conjunto sintético chico con el formato exacto de la §3.4, para las pruebas y la corrida de punta a punta.

    uv run python sintetico.py --salida /tmp/mapa-sintetico --filas 24 --columnas 16 --dias 30

Escribe dias/AAAA-MM-DD.npy (float16 [24, C, filas, columnas], normalizados), estaticos.npy (float32
[S, filas, columnas]), etiquetas.csv y manifiesto.json con el sha256 de cada archivo. Los canales
"cams_pm2_5" y "cams_pm10" se recuperan en µg/m³ con la media y la desviación del manifiesto.

Las etiquetas dependen de los canales: el PM "verdadero" es el de CAMS corregido por la altura de la
capa límite, el viento de las últimas 6 horas y la orografía, más ruido; las estaciones lo miden con
otro 10 % de ruido y huecos. Una estación no mide PM2.5 (como varias de OEFA).
"""

from __future__ import annotations

import argparse
import csv
import json
from datetime import date, datetime, timedelta, timezone
from pathlib import Path

import numpy as np

from datos import HORAS, sha256_archivo

# Los mismos nombres y el mismo orden que lab/mapa/datos/preparar.py.
CANALES = ["temperatura_2m", "rocio_2m", "viento_u_10m", "viento_v_10m", "presion_superficie", "precipitacion",
           "radiacion_solar", "altura_capa_limite", "nubosidad_total", "cams_pm2_5", "cams_pm10", "aod_550",
           "aod_polvo_550"]
ESTATICOS = ["orografia", "mascara_tierra", "mascara_peru", "latitud", "longitud", "log1p_poblacion"]


def _campo_suave(rng: np.random.Generator, filas: int, columnas: int, ondas: int = 3) -> np.ndarray:
    """Un campo espacial suave de media 0 y desviación ~1 (suma de pocas ondas)."""
    y = np.linspace(0, 1, filas)[:, None]
    x = np.linspace(0, 1, columnas)[None, :]
    f = np.zeros((filas, columnas))
    for _ in range(ondas):
        ky, kx = rng.uniform(0.5, 2.5, 2)
        fy, fx = rng.uniform(0, 2 * np.pi, 2)
        f += np.sin(2 * np.pi * ky * y + fy) * np.cos(2 * np.pi * kx * x + fx)
    return (f - f.mean()) / (f.std() + 1e-9)


def generar(salida: str | Path, filas: int = 24, columnas: int = 16, n_dias: int = 30, desde: str = "2020-01-01",
            n_estaciones: int = 5, semilla: int = 0) -> Path:
    salida = Path(salida)
    (salida / "dias").mkdir(parents=True, exist_ok=True)
    rng = np.random.default_rng(semilla)
    C, T = len(CANALES), n_dias * HORAS
    t = np.arange(T)
    diurno = np.sin(2 * np.pi * (t % HORAS) / HORAS)
    lento = np.cumsum(rng.normal(0, 0.15, T))
    lento = (lento - lento.mean()) / (lento.std() + 1e-9)

    # Canales crudos [T, C, filas, columnas], con unidades verosímiles.
    crudo = np.empty((T, C, filas, columnas), np.float32)
    escala = {"temperatura_2m": (15, 5), "rocio_2m": (7, 4), "viento_u_10m": (0, 3), "viento_v_10m": (0, 3),
              "presion_superficie": (800, 90), "precipitacion": (0.2, 0.2), "radiacion_solar": (200, 150),
              "altura_capa_limite": (700, 400), "nubosidad_total": (0.5, 0.25)}
    for c, nombre in enumerate(CANALES[:9]):
        media, desv = escala[nombre]
        a, b = _campo_suave(rng, filas, columnas), _campo_suave(rng, filas, columnas)
        fase = rng.uniform(0, 2 * np.pi)
        z = (a[None] + 0.8 * b[None] * np.sin(2 * np.pi * (t % HORAS) / HORAS + fase)[:, None, None]
             + 0.5 * lento[:, None, None] + 0.3 * rng.normal(size=(T, filas, columnas)))
        crudo[:, c] = media + desv * z
    for nombre, minimo, maximo in (("precipitacion", 0, None), ("radiacion_solar", 0, None),
                                   ("nubosidad_total", 0, 1), ("altura_capa_limite", 50, None)):
        crudo[:, CANALES.index(nombre)] = np.clip(crudo[:, CANALES.index(nombre)], minimo, maximo)

    z_blh = (crudo[:, CANALES.index("altura_capa_limite")] - 700) / 400
    fuentes = _campo_suave(rng, filas, columnas)
    log_cams25 = 3.0 + 0.5 * fuentes[None] + 0.3 * diurno[:, None, None] - 0.25 * z_blh \
        + 0.1 * rng.normal(size=(T, filas, columnas))
    cams25 = np.expm1(log_cams25)
    cams10 = cams25 * 1.8 * np.exp(0.2 * _campo_suave(rng, filas, columnas))[None]
    crudo[:, CANALES.index("cams_pm2_5")] = cams25
    crudo[:, CANALES.index("cams_pm10")] = cams10
    crudo[:, CANALES.index("aod_550")] = 0.002 * cams10 + 0.02 * rng.normal(size=(T, filas, columnas))
    crudo[:, CANALES.index("aod_polvo_550")] = 0.0005 * cams10 * np.exp(0.3 * _campo_suave(rng, filas, columnas))[None]

    # Estáticos.
    lat = 0.6 - 0.1 * np.arange(filas)
    lon = -81.6 + 0.1 * np.arange(columnas)
    orografia = np.clip(2000 + 1500 * _campo_suave(rng, filas, columnas), 0, None)
    tierra = (orografia > 300).astype(np.float32)
    peru = tierra.copy()
    pob = np.exp(rng.normal(6, 2, (filas, columnas))) * tierra
    estaticos = np.stack([orografia, tierra, peru,
                          np.broadcast_to(((lat - lat.mean()) / lat.std())[:, None], (filas, columnas)),
                          np.broadcast_to(((lon - lon.mean()) / lon.std())[None, :], (filas, columnas)),
                          np.log1p(pob)]).astype(np.float32)
    np.save(salida / "estaticos.npy", estaticos)

    # Verdad: CAMS corregido por la capa límite, el viento de las últimas 6 horas y la orografía.
    u = crudo[:, CANALES.index("viento_u_10m")]
    u6 = np.stack([u[max(0, k - 5):k + 1].mean(0) for k in range(T)])
    oro = (orografia - orografia.mean()) / orografia.std()
    log25 = 0.8 * np.log1p(cams25) + 1.1 - 0.3 * z_blh + 0.15 * u6 / 3 + 0.2 * oro[None] \
        + 0.05 * rng.normal(size=(T, filas, columnas))
    log10 = 0.8 * np.log1p(cams10) + 1.5 - 0.25 * z_blh + 0.2 * u6 / 3 + 0.25 * oro[None] \
        + 0.05 * rng.normal(size=(T, filas, columnas))

    # Normalización y días.
    medias = crudo.mean(axis=(0, 2, 3), dtype=np.float64)
    desv = crudo.std(axis=(0, 2, 3), dtype=np.float64)
    dias = [(date.fromisoformat(desde) + timedelta(days=k)).isoformat() for k in range(n_dias)]
    norm = ((crudo - medias[None, :, None, None].astype(np.float32)) / desv[None, :, None, None].astype(np.float32))
    for k, d in enumerate(dias):
        np.save(salida / "dias" / f"{d}.npy", norm[k * HORAS:(k + 1) * HORAS].astype(np.float16))

    # Estaciones en celdas distintas, con ruido y huecos.
    celdas = rng.choice(filas * columnas, n_estaciones, replace=False)
    t0 = datetime.fromisoformat(desde).replace(tzinfo=timezone.utc)
    with open(salida / "etiquetas.csv", "w", newline="", encoding="utf-8") as f:
        w = csv.writer(f)
        w.writerow(["hora_utc", "estacion", "fuente", "i", "j", "pm2_5", "pm10"])
        for k in range(T):
            hora = (t0 + timedelta(hours=k)).strftime("%Y-%m-%dT%H:%M:%SZ")
            for s, celda in enumerate(celdas):
                i, j = divmod(int(celda), columnas)
                v25 = np.expm1(log25[k, i, j]) * np.exp(rng.normal(0, 0.1))
                v10 = np.expm1(log10[k, i, j]) * np.exp(rng.normal(0, 0.1))
                if s == n_estaciones - 1 or rng.random() < 0.1:
                    v25 = None  # la última estación no mide PM2.5
                if rng.random() < 0.1:
                    v10 = None
                if v25 is None and v10 is None:
                    continue
                w.writerow([hora, f"SIN-{s + 1:02d}", "SINTETICO", i, j,
                            "" if v25 is None else f"{v25:.2f}", "" if v10 is None else f"{v10:.2f}"])

    archivos = {}
    for ruta in sorted([f"dias/{d}.npy" for d in dias] + ["estaticos.npy", "etiquetas.csv"]):
        archivos[ruta] = {"sha256": sha256_archivo(salida / ruta), "bytes": (salida / ruta).stat().st_size}
    manifiesto = {
        "version": 1,
        "grilla": {"lat_norte": 0.6, "lon_oeste": -81.6, "paso": 0.1, "filas": filas, "columnas": columnas},
        "periodo": {"desde": dias[0], "hasta": dias[-1]},
        "canales": [{"nombre": n, "media": float(m), "desviacion": float(s), "transformacion": "ninguna"}
                    for n, m, s in zip(CANALES, medias, desv)],
        "estaticos": [{"nombre": n} for n in ESTATICOS],
        "archivos": archivos,
        "fuentes": ["sintético (lab/mapa/modelo/sintetico.py)"],
    }
    (salida / "manifiesto.json").write_text(json.dumps(manifiesto, indent=1, ensure_ascii=False), encoding="utf-8")
    return salida


def main() -> None:
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("--salida", required=True)
    p.add_argument("--filas", type=int, default=24)
    p.add_argument("--columnas", type=int, default=16)
    p.add_argument("--dias", type=int, default=30)
    p.add_argument("--desde", default="2020-01-01")
    p.add_argument("--estaciones", type=int, default=5)
    p.add_argument("--semilla", type=int, default=0)
    a = p.parse_args()
    ruta = generar(a.salida, a.filas, a.columnas, a.dias, a.desde, a.estaciones, a.semilla)
    print(f"sintético en {ruta}: {a.dias} días de {a.filas}×{a.columnas}, {len(CANALES)} canales, "
          f"{len(ESTATICOS)} estáticos, {a.estaciones} estaciones")


if __name__ == "__main__":
    main()
