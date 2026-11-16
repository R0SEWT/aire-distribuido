"""M2: la red espacial (U-Net 2D), sus pesos como vector plano y los pasos de entrenamiento.

Corre en los nodos del laboratorio: solo importa numpy, torch y la biblioteca estándar.

    uv run python red.py exportar --manifiesto data/mapa/manifiesto.json --ancho 32 --salida pesos/
    uv run python red.py medir --canales 13 --estaticos 6 --ancho 32 --lote 2

`exportar` escribe pesos_0.bin (el vector inicial, float32 little-endian) y pesos.json (arquitectura,
n, y nombre, forma y desplazamiento de cada tensor): el coordinador en Go trata el vector como opaco y
solo necesita n. `medir` cuenta parámetros y mide ms de forward+backward por muestra a tamaño real.

La red: 3 reducciones (192×136 → 24×17), anchos a-2a-4a-8a (32-64-128-256 con ancho 32), dos
convoluciones 3×3 con GroupNorm y SiLU por bloque. GroupNorm y no BatchNorm: el promedio de pesos entre
nodos no tiene estadísticas de lote que sincronizar.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import time
from pathlib import Path

import numpy as np
import torch
from torch import nn

GRUPOS = 8
SESGO_INICIAL = 3.0  # log1p de ~20 µg/m³: arranca en un PM verosímil


# ---------------------------------------------------------------- arquitectura


class Bloque(nn.Sequential):
    def __init__(self, entrada: int, salida: int, grupos: int = GRUPOS):
        super().__init__(
            nn.Conv2d(entrada, salida, 3, padding=1), nn.GroupNorm(grupos, salida), nn.SiLU(),
            nn.Conv2d(salida, salida, 3, padding=1), nn.GroupNorm(grupos, salida), nn.SiLU(),
        )


class UNet(nn.Module):
    def __init__(self, entrada: int, salida: int = 2, ancho: int = 32, grupos: int = GRUPOS):
        super().__init__()
        a = ancho
        self.enc0 = Bloque(entrada, a, grupos)
        self.enc1 = Bloque(a, 2 * a, grupos)
        self.enc2 = Bloque(2 * a, 4 * a, grupos)
        self.fondo = Bloque(4 * a, 8 * a, grupos)
        self.dec2 = Bloque(8 * a + 4 * a, 4 * a, grupos)
        self.dec1 = Bloque(4 * a + 2 * a, 2 * a, grupos)
        self.dec0 = Bloque(2 * a + a, a, grupos)
        self.salida = nn.Conv2d(a, salida, 1)
        self.bajar = nn.MaxPool2d(2)
        self.subir = nn.Upsample(scale_factor=2, mode="nearest")
        nn.init.constant_(self.salida.bias, SESGO_INICIAL)

    def forward(self, x: torch.Tensor) -> torch.Tensor:
        if x.shape[-2] % 8 or x.shape[-1] % 8:
            raise ValueError(f"la grilla {tuple(x.shape[-2:])} tiene que ser divisible por 8")
        e0 = self.enc0(x)
        e1 = self.enc1(self.bajar(e0))
        e2 = self.enc2(self.bajar(e1))
        f = self.fondo(self.bajar(e2))
        d2 = self.dec2(torch.cat([self.subir(f), e2], 1))
        d1 = self.dec1(torch.cat([self.subir(d2), e1], 1))
        d0 = self.dec0(torch.cat([self.subir(d1), e0], 1))
        return self.salida(d0)


def arquitectura(entrada: int, ancho: int = 32, salida: int = 2) -> dict:
    return {"nombre": "unet2d", "entrada": int(entrada), "salida": int(salida), "ancho": int(ancho), "grupos": GRUPOS}


def construir(arq: dict, semilla: int | None = None) -> UNet:
    if arq.get("nombre", "unet2d") != "unet2d":
        raise ValueError(f"arquitectura desconocida {arq.get('nombre')!r}")
    if semilla is not None:
        torch.manual_seed(semilla)
    return UNet(arq["entrada"], arq.get("salida", 2), arq.get("ancho", 32), arq.get("grupos", GRUPOS))


def n_parametros(modelo: nn.Module) -> int:
    return sum(p.numel() for p in modelo.parameters())


# ---------------------------------------------------------------- pesos como vector plano


def tensores(modelo: nn.Module) -> list[dict]:
    """Nombre, forma y desplazamiento (en floats) de cada parámetro, en el orden del vector plano."""
    filas, desde = [], 0
    for nombre, p in modelo.named_parameters():
        filas.append({"nombre": nombre, "forma": list(p.shape), "desde": desde, "n": p.numel()})
        desde += p.numel()
    return filas


def aplanar(modelo: nn.Module) -> np.ndarray:
    """Todos los parámetros en un vector float32 (en el orden de named_parameters)."""
    with torch.no_grad():
        return torch.cat([p.detach().reshape(-1).float().cpu() for p in modelo.parameters()]).numpy()


def cargar_plano(modelo: nn.Module, vector: np.ndarray) -> None:
    """Copia el vector plano en los parámetros, en el mismo orden que aplanar."""
    vector = np.asarray(vector, dtype=np.float32)
    n = n_parametros(modelo)
    if vector.shape != (n,):
        raise ValueError(f"el vector tiene {vector.size} floats y el modelo {n} parámetros")
    plano = torch.from_numpy(vector)
    desde = 0
    with torch.no_grad():
        for p in modelo.parameters():
            p.copy_(plano[desde:desde + p.numel()].view_as(p))
            desde += p.numel()


def a_bytes(vector: np.ndarray) -> bytes:
    return np.ascontiguousarray(vector, dtype="<f4").tobytes()


def de_bytes(datos: bytes) -> np.ndarray:
    return np.frombuffer(datos, dtype="<f4").astype(np.float32)


def descripcion(modelo: nn.Module, arq: dict, **extra) -> dict:
    n = n_parametros(modelo)
    return {"version": 1, "formato": "float32 little-endian", "n": n, "bytes": 4 * n, "arquitectura": arq,
            **extra, "tensores": tensores(modelo)}


def exportar(modelo: nn.Module, arq: dict, carpeta: str | Path, nombre: str = "pesos_0.bin", **extra) -> dict:
    """Escribe el vector (pesos_0.bin) y pesos.json. Devuelve la descripción.

    Varios vectores de la misma red en una carpeta (uno por partición) comparten pesos.json: su
    "archivos" junta el sha256 de cada uno."""
    carpeta = Path(carpeta)
    carpeta.mkdir(parents=True, exist_ok=True)
    datos = a_bytes(aplanar(modelo))
    (carpeta / nombre).write_bytes(datos)
    d = descripcion(modelo, arq, **extra)
    ruta = carpeta / "pesos.json"
    archivos = {}
    if ruta.exists():
        previo = json.loads(ruta.read_text(encoding="utf-8"))
        if previo.get("arquitectura") == arq and previo.get("tensores") == d["tensores"]:
            archivos = previo.get("archivos", {})
    archivos[nombre] = hashlib.sha256(datos).hexdigest()
    d = {**{k: v for k, v in d.items() if k != "tensores"}, "archivos": dict(sorted(archivos.items())),
         "tensores": d["tensores"]}
    ruta.write_text(json.dumps(d, indent=1, ensure_ascii=False), encoding="utf-8")
    return d


def cargar_modelo(ruta_bin: str | Path, ruta_json: str | Path, dispositivo: torch.device | str = "cpu") -> UNet:
    d = json.loads(Path(ruta_json).read_text(encoding="utf-8"))
    modelo = construir(d["arquitectura"])
    cargar_plano(modelo, de_bytes(Path(ruta_bin).read_bytes()))
    return modelo.to(dispositivo)


# ---------------------------------------------------------------- pérdida y entrenamiento


def perdida_enmascarada(pred: torch.Tensor, b: torch.Tensor, i: torch.Tensor, j: torch.Tensor,
                        y: torch.Tensor, m: torch.Tensor) -> tuple[torch.Tensor, torch.Tensor]:
    """Error cuadrático en log1p solo en las celdas con estación y valor, por contaminante.

    pred es [B, 2, filas, columnas]; (b, i, j) dicen de qué muestra y celda es cada etiqueta, y [L, 2]
    el log1p observado y m [L, 2] si hay valor. Devuelve (media, cuántos valores entraron). Las
    celdas sin etiqueta no aportan, y un valor con m = 0 tampoco (aunque y sea NaN).
    """
    p = pred[b, :, i, j].float()  # [L, 2]
    d = torch.where(m > 0, p - y, torch.zeros_like(p))
    cuantos = m.sum()
    return (d * d).sum() / cuantos.clamp_min(1), cuantos


class Entrenador:
    """H pasos de AdamW sobre las horas de entrenamiento de un Conjunto.

    El optimizador (y el escalador de fp16) conservan su estado entre llamadas: en el local SGD solo
    viajan los pesos, y cargar_plano los copia en los mismos tensores que el optimizador conoce.
    """

    def __init__(self, modelo: nn.Module, conjunto, lote: int = 4, tasa: float = 1e-3, decaimiento: float = 1e-4,
                 semilla: int | list[int] = 0, amp: bool | None = None):
        self.modelo = modelo
        self.conjunto = conjunto
        self.lote = lote
        self.dispositivo = next(modelo.parameters()).device
        self.amp = (self.dispositivo.type == "cuda") if amp is None else (amp and self.dispositivo.type == "cuda")
        self.opt = torch.optim.AdamW(modelo.parameters(), lr=tasa, weight_decay=decaimiento)
        self.escala = torch.amp.GradScaler("cuda", enabled=self.amp)
        self.rng = np.random.default_rng(semilla)
        self.pendientes: list[int] = []

    def _lote(self) -> list[tuple[str, int]]:
        horas = self.conjunto.horas_ent
        elegidas = []
        while len(elegidas) < min(self.lote, len(horas)):
            if not self.pendientes:
                self.pendientes = self.rng.permutation(len(horas)).tolist()
            elegidas.append(horas[self.pendientes.pop()])
        return elegidas

    def pasos(self, n: int) -> tuple[int, float]:
        """n pasos; devuelve (muestras vistas, pérdida media de los pasos)."""
        if not self.conjunto.horas_ent:
            return 0, 0.0
        self.modelo.train()
        vistas, suma = 0, 0.0
        for _ in range(n):
            horas = self._lote()
            x = self.conjunto.entrada(horas)
            obj = self.conjunto.objetivos(horas)
            with torch.autocast("cuda", dtype=torch.float16, enabled=self.amp):
                pred = self.modelo(x)
            perdida, _ = perdida_enmascarada(pred, *obj)
            self.opt.zero_grad(set_to_none=True)
            self.escala.scale(perdida).backward()
            self.escala.step(self.opt)
            self.escala.update()
            vistas += len(horas)
            suma += perdida.item()
        return vistas, suma / n

    @torch.no_grad()
    def perdida(self, horas: list[tuple[str, int]], val: bool = True, lote: int | None = None) -> tuple[float, int]:
        """Pérdida agrupada (suma de errores / cuántos valores) sobre estas horas; (0, 0) si no hay."""
        self.modelo.eval()
        lote = lote or self.lote
        suma, total = 0.0, 0
        for k in range(0, len(horas), lote):
            hs = horas[k:k + lote]
            with torch.autocast("cuda", dtype=torch.float16, enabled=self.amp):
                pred = self.modelo(self.conjunto.entrada(hs))
            media, cuantos = perdida_enmascarada(pred, *self.conjunto.objetivos(hs, val=val))
            suma += media.item() * cuantos.item()
            total += int(cuantos.item())
        return (suma / total if total else 0.0), total


# ---------------------------------------------------------------- CLI


def _medir(a) -> None:
    torch.manual_seed(0)
    torch.set_num_threads(a.hilos or torch.get_num_threads())
    entrada = 24 * a.canales + a.estaticos + 4
    modelo = UNet(entrada, 2, a.ancho)
    n = n_parametros(modelo)
    x = torch.randn(a.lote, entrada, a.filas, a.columnas)
    tiempos = []
    for k in range(a.repes + 1):
        t0 = time.perf_counter()
        pred = modelo(x)
        pred.square().mean().backward()
        modelo.zero_grad(set_to_none=True)
        if k:  # la primera es de calentamiento
            tiempos.append((time.perf_counter() - t0) * 1000 / a.lote)
    print(json.dumps({
        "dispositivo": "CPU (laptop)", "hilos": torch.get_num_threads(), "torch": torch.__version__,
        "grilla": [a.filas, a.columnas], "canales_entrada": entrada, "ancho": a.ancho,
        "anchos": [a.ancho, 2 * a.ancho, 4 * a.ancho, 8 * a.ancho],
        "parametros": n, "megabytes_float32": round(4 * n / 1e6, 2), "lote": a.lote,
        "ms_por_muestra_forward_backward": [round(t, 1) for t in tiempos],
        "ms_por_muestra_mediana": round(float(np.median(tiempos)), 1),
    }, indent=1, ensure_ascii=False))


def main() -> None:
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = p.add_subparsers(dest="orden", required=True)
    e = sub.add_parser("exportar", help="pesos_0.bin y pesos.json")
    e.add_argument("--manifiesto", required=True, help="manifiesto.json (de ahí salen C y S)")
    e.add_argument("--ancho", type=int, default=32)
    e.add_argument("--semilla", type=int, default=0)
    e.add_argument("--salida", required=True)
    m = sub.add_parser("medir", help="parámetros y ms de forward+backward por muestra")
    m.add_argument("--canales", type=int, default=13)
    m.add_argument("--estaticos", type=int, default=6)
    m.add_argument("--filas", type=int, default=192)
    m.add_argument("--columnas", type=int, default=136)
    m.add_argument("--ancho", type=int, default=32)
    m.add_argument("--lote", type=int, default=2)
    m.add_argument("--repes", type=int, default=3)
    m.add_argument("--hilos", type=int, default=0)
    a = p.parse_args()
    if a.orden == "medir":
        _medir(a)
        return
    from datos import leer_manifiesto
    man = leer_manifiesto(a.manifiesto)
    arq = arquitectura(man.n_entrada, a.ancho)
    modelo = construir(arq, semilla=a.semilla)
    d = exportar(modelo, arq, a.salida, semilla=a.semilla, canales=man.canales, estaticos=man.estaticos)
    print(f"{a.salida}/pesos_0.bin: {d['n']:,} parámetros ({d['bytes'] / 1e6:.1f} MB), "
          f"sha256 {d['archivos']['pesos_0.bin'][:12]}…")


if __name__ == "__main__":
    main()
