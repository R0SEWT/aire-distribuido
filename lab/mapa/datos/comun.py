"""Rutas, fechas, sha256 y JSON que comparten los scripts de datos del mapa."""

import hashlib
import json
import os
from datetime import date, timedelta
from pathlib import Path

RAIZ_REPO = Path(__file__).resolve().parents[3]
DIR_MAPA = RAIZ_REPO / "data" / "mapa"

# Periodo de la spec (§1, §3.4): 2018–2022 más el día previo para la ventana de 24 h.
DESDE = date(2017, 12, 31)
HASTA = date(2022, 12, 31)
# Años con los que se normaliza (z-score) cada canal.
ESTADISTICAS_DESDE = date(2018, 1, 1)
ESTADISTICAS_HASTA = date(2021, 12, 31)

MANIFIESTO = "manifiesto.json"


def dias(desde: date, hasta: date) -> list[date]:
    """Los días de desde a hasta, ambos incluidos."""
    return [desde + timedelta(days=k) for k in range((hasta - desde).days + 1)]


def fecha(texto: str) -> date:
    return date.fromisoformat(texto)


def sha256_bytes(datos: bytes) -> str:
    return hashlib.sha256(datos).hexdigest()


def sha256_archivo(ruta: Path) -> str:
    h = hashlib.sha256()
    with open(ruta, "rb") as f:
        for bloque in iter(lambda: f.read(1 << 20), b""):
            h.update(bloque)
    return h.hexdigest()


def escribir_atomico(ruta: Path, datos: bytes) -> None:
    """Escribe a un temporal y lo renombra: un archivo a medias nunca queda con el nombre final."""
    ruta.parent.mkdir(parents=True, exist_ok=True)
    temporal = ruta.with_name(ruta.name + ".parcial")
    temporal.write_bytes(datos)
    os.replace(temporal, ruta)


def escribir_json(ruta: Path, obj) -> None:
    escribir_atomico(ruta, (json.dumps(obj, ensure_ascii=False, indent=2) + "\n").encode())


def registrar_archivo(dir_mapa: Path, relativa: str) -> bool:
    """Actualiza el sha256 y el tamaño de un archivo en el manifiesto, si el manifiesto ya existe."""
    ruta_manifiesto = dir_mapa / MANIFIESTO
    if not ruta_manifiesto.exists():
        return False
    manifiesto = json.loads(ruta_manifiesto.read_text())
    ruta = dir_mapa / relativa
    manifiesto.setdefault("archivos", {})[relativa] = {
        "sha256": sha256_archivo(ruta),
        "bytes": ruta.stat().st_size,
    }
    escribir_json(ruta_manifiesto, manifiesto)
    return True
