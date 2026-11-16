"""Lectura del formato en disco de lab/mapa/datos/ y armado de las muestras.

Corre en los nodos del laboratorio: solo importa numpy, torch y la biblioteca estándar.

El contrato es la §3.4 de docs/specs/2026-11-14-mapa-nacional-design.md:

- dias/AAAA-MM-DD.npy: float16 [24, C, filas, columnas], horas 00–23 UTC, canales normalizados.
- estaticos.npy: float32 [S, filas, columnas].
- etiquetas.csv: hora_utc (ISO, hora de fin), estacion, fuente, i, j, pm2_5, pm10 (vacío si falta).
- manifiesto.json. Lo que se lee de él (y lo que escribe sintetico.py):

    {
      "version": 1,
      "grilla": {"lat_norte": 0.6, "lon_oeste": -81.6, "paso": 0.1, "filas": 192, "columnas": 136},
      "periodo": {"desde": "2018-01-01", "hasta": "2022-12-31"},
      "canales": [{"nombre": "temperatura_2m", "media": 17.1, "desviacion": 6.2, "transformacion": "ninguna"}, ...],
      "estaticos": [{"nombre": "orografia", ...}, ...],
      "archivos": {"dias/2018-01-01.npy": {"sha256": "…", "bytes": 17547392}, "estaticos.npy": {...}, ...},
      "fuentes": [...]
    }

  Es lo que escribe lab/mapa/datos/preparar.py. Se toleran variantes: "archivos" como {ruta: "sha256"}
  o una clave "sha256" con ese mapa; los estáticos como lista de nombres o de objetos con "nombre";
  "desviacion", "desv" o "std". Un canal con "transformacion": "log1p" se normalizó como log1p(x).
  M0 necesita los canales "cams_pm2_5" y "cams_pm10".

Una muestra para la hora t (día d, hora h UTC) es la ventana de las 24 horas que terminan en t
(si h < 23 empieza en el día anterior), los estáticos y 4 canales de calendario. La etiqueta con
hora_utc = t (hora de fin) va con la entrada de la hora t.
"""

from __future__ import annotations

import csv
import hashlib
import json
import math
import tempfile
import urllib.parse
import urllib.request
from dataclasses import dataclass, field
from datetime import date, datetime, timedelta, timezone
from pathlib import Path

import numpy as np
import torch

HORAS = 24
CONTAMINANTES = ("pm2_5", "pm10")
ECA_24H = {"pm2_5": 50.0, "pm10": 100.0}  # µg/m³, estándar de 24 h
N_CALENDARIO = 4
TAM_BLOQUE = 7
N_PARTICIONES = 5


# ---------------------------------------------------------------- manifiesto


@dataclass
class Manifiesto:
    canales: list[str]
    medias: np.ndarray
    desviaciones: np.ndarray
    estaticos: list[str]
    filas: int
    columnas: int
    sha256: dict[str, str]  # ruta relativa -> sha256 en hexadecimal
    crudo: dict = field(repr=False)
    transformaciones: list[str] = field(default_factory=list)  # "ninguna" o "log1p", por canal

    @classmethod
    def desde_json(cls, m: dict) -> "Manifiesto":
        canales, medias, desv, transf = [], [], [], []
        for c in m["canales"]:
            if isinstance(c, str):
                raise ValueError(f"el canal {c!r} no trae media ni desviación en el manifiesto")
            canales.append(c["nombre"])
            medias.append(float(c["media"]))
            desv.append(float(c.get("desviacion", c.get("desv", c.get("std")))))
            transf.append(c.get("transformacion", "ninguna"))
        estaticos = [e if isinstance(e, str) else e["nombre"] for e in m["estaticos"]]
        g = m.get("grilla", {})
        filas = int(g.get("filas", g.get("forma", [0, 0])[0] if "forma" in g else 0))
        columnas = int(g.get("columnas", g.get("forma", [0, 0])[1] if "forma" in g else 0))
        sha: dict[str, str] = {}
        for ruta, v in (m.get("archivos") or m.get("sha256") or {}).items():
            sha[ruta] = v if isinstance(v, str) else v["sha256"]
        return cls(canales, np.array(medias, np.float64), np.array(desv, np.float64), estaticos,
                   filas, columnas, sha, m, transf)

    @property
    def dias(self) -> list[str]:
        """Los días que tiene el manifiesto, en orden."""
        return sorted(Path(r).stem for r in self.sha256 if r.startswith("dias/") and r.endswith(".npy"))

    @property
    def n_canales(self) -> int:
        return len(self.canales)

    @property
    def n_entrada(self) -> int:
        """Canales de entrada de la red: 24·C + S + 4."""
        return HORAS * self.n_canales + len(self.estaticos) + N_CALENDARIO

    def indice(self, canal: str) -> int:
        return self.canales.index(canal)

    def desnormalizar(self, x: np.ndarray, canal: str) -> np.ndarray:
        """Del z-score a la unidad del canal (deshaciendo log1p si el canal lo declara)."""
        k = self.indice(canal)
        v = np.asarray(x, np.float64) * self.desviaciones[k] + self.medias[k]
        return np.expm1(v) if self.transformaciones and self.transformaciones[k] == "log1p" else v


def leer_manifiesto(ruta: str | Path) -> Manifiesto:
    return Manifiesto.desde_json(json.loads(Path(ruta).read_text(encoding="utf-8")))


def ruta_dia(dia: str) -> str:
    return f"dias/{dia}.npy"


# ---------------------------------------------------------------- fechas


def dia_anterior(dia: str) -> str:
    return (date.fromisoformat(dia) - timedelta(days=1)).isoformat()


def rango_dias(desde: str, hasta: str) -> list[str]:
    d, h = date.fromisoformat(desde), date.fromisoformat(hasta)
    return [(d + timedelta(days=k)).isoformat() for k in range((h - d).days + 1)]


def calendario(dia: str, hora: int, filas: int, columnas: int) -> np.ndarray:
    """Seno y coseno de la hora UTC y del día del año, como planos constantes [4, filas, columnas]."""
    doy = date.fromisoformat(dia).timetuple().tm_yday
    ah = 2 * math.pi * hora / HORAS
    ad = 2 * math.pi * (doy - 1) / 365.25
    v = np.array([math.sin(ah), math.cos(ah), math.sin(ad), math.cos(ad)], np.float32)
    return np.broadcast_to(v[:, None, None], (N_CALENDARIO, filas, columnas)).copy()


# ---------------------------------------------------------------- archivos


def sha256_archivo(ruta: str | Path) -> str:
    h = hashlib.sha256()
    with open(ruta, "rb") as f:
        for bloque in iter(lambda: f.read(1 << 20), b""):
            h.update(bloque)
    return h.hexdigest()


class Origen:
    """De dónde salen los archivos: una carpeta local o la URL del manifiesto (el mismo origen).

    Con una URL, cada archivo se baja a `cache` (con el sha256 en el nombre) y se verifica contra
    el manifiesto; si ya estaba en la cache y su sha256 coincide, no se vuelve a bajar.
    """

    def __init__(self, base: str, cache: str | Path | None = None, verificar: bool = True):
        self.es_url = base.startswith(("http://", "https://"))
        self.verificar = verificar
        self._verificados: dict[str, Path] = {}
        if self.es_url:
            self.url_manifiesto = base if base.endswith(".json") else base.rstrip("/") + "/manifiesto.json"
            self.cache = Path(cache or Path(tempfile.gettempdir()) / "aire-mapa-cache")
            self.cache.mkdir(parents=True, exist_ok=True)
            crudo = self._bajar(self.url_manifiesto)
            self.huella = hashlib.sha256(crudo).hexdigest()
            self.manifiesto = Manifiesto.desde_json(json.loads(crudo))
        else:
            p = Path(base)
            self.raiz = p.parent if p.suffix == ".json" else p
            ruta_m = p if p.suffix == ".json" else p / "manifiesto.json"
            self.huella = sha256_archivo(ruta_m)
            self.manifiesto = leer_manifiesto(ruta_m)

    @staticmethod
    def _bajar(url: str) -> bytes:
        with urllib.request.urlopen(url, timeout=120) as r:
            return r.read()

    def ruta(self, relativa: str) -> Path:
        """Ruta local del archivo, verificado contra el sha256 del manifiesto (una vez por Origen)."""
        if relativa not in self._verificados:
            self._verificados[relativa] = self._ruta(relativa)
        return self._verificados[relativa]

    def _ruta(self, relativa: str) -> Path:
        esperado = self.manifiesto.sha256.get(relativa)
        if self.es_url:
            if esperado is None:
                raise FileNotFoundError(f"{relativa} no está en el manifiesto")
            destino = self.cache / f"{esperado[:16]}_{relativa.replace('/', '_')}"
            if destino.exists() and sha256_archivo(destino) == esperado:
                return destino
            url = urllib.parse.urljoin(self.url_manifiesto, relativa)
            h = hashlib.sha256()
            tmp = destino.with_suffix(destino.suffix + ".parcial")
            with urllib.request.urlopen(url, timeout=120) as r, open(tmp, "wb") as f:
                for bloque in iter(lambda: r.read(1 << 20), b""):
                    h.update(bloque)
                    f.write(bloque)
            if h.hexdigest() != esperado:
                tmp.unlink(missing_ok=True)
                raise ValueError(f"{relativa}: sha256 {h.hexdigest()[:12]}… no es el del manifiesto {esperado[:12]}…")
            tmp.replace(destino)
            return destino
        local = self.raiz / relativa
        if self.verificar and esperado is not None and sha256_archivo(local) != esperado:
            raise ValueError(f"{relativa}: el sha256 no es el del manifiesto")
        return local


def cargar_estaticos(ruta: str | Path) -> np.ndarray:
    """Los estáticos [S, filas, columnas] en float32, cada canal llevado a media 0 y desviación 1 en la
    grilla (la orografía viene en metros; si datos/ ya los normalizó, esto no los cambia)."""
    e = np.load(ruta).astype(np.float32)
    media = e.mean(axis=(1, 2), keepdims=True)
    desv = e.std(axis=(1, 2), keepdims=True)
    return (e - media) / np.where(desv > 0, desv, 1.0)


# ---------------------------------------------------------------- etiquetas


@dataclass
class Etiquetas:
    """Las filas de etiquetas.csv, como columnas. valores es [N, 2] (pm2_5, pm10) con NaN si falta."""

    dia: np.ndarray  # str
    hora: np.ndarray  # int
    estacion: np.ndarray  # str
    i: np.ndarray
    j: np.ndarray
    valores: np.ndarray

    def __len__(self) -> int:
        return len(self.hora)

    def estaciones(self) -> list[str]:
        return sorted(set(self.estacion.tolist()))

    def filtrar(self, mascara: np.ndarray) -> "Etiquetas":
        return Etiquetas(self.dia[mascara], self.hora[mascara], self.estacion[mascara], self.i[mascara],
                         self.j[mascara], self.valores[mascara])


def _num(s: str) -> float:
    s = s.strip()
    return float(s) if s else math.nan


def leer_etiquetas(ruta: str | Path) -> Etiquetas:
    dias, horas, est, ii, jj, vals = [], [], [], [], [], []
    with open(ruta, newline="", encoding="utf-8") as f:
        for fila in csv.DictReader(f):
            t = datetime.fromisoformat(fila["hora_utc"].strip())
            if t.tzinfo is not None:
                t = t.astimezone(timezone.utc)
            dias.append(t.date().isoformat())
            horas.append(t.hour)
            est.append(fila["estacion"])
            ii.append(int(fila["i"]))
            jj.append(int(fila["j"]))
            vals.append((_num(fila["pm2_5"]), _num(fila["pm10"])))
    return Etiquetas(np.array(dias, dtype=object), np.array(horas, np.int64), np.array(est, dtype=object),
                     np.array(ii, np.int64), np.array(jj, np.int64),
                     np.array(vals, np.float32).reshape(-1, 2))


# ---------------------------------------------------------------- particiones y reparto


def particiones(estaciones: list[str], k: int = N_PARTICIONES, semilla: int = 0) -> list[list[str]]:
    """k grupos de estaciones, disjuntos y deterministas: se ordenan, se barajan con la semilla y se
    reparten en ronda. La partición f es la lista de estaciones de validación del pliegue f."""
    orden = sorted(set(estaciones))
    perm = np.random.default_rng(semilla).permutation(len(orden))
    return [sorted(orden[p] for p in perm[f::k]) for f in range(k)]


def repartir_bloques(dias: list[str], n_nodos: int, semilla: int = 0, tam: int = TAM_BLOQUE) -> list[list[str]]:
    """Bloques de `tam` días contiguos, barajados con la semilla y repartidos en ronda entre los nodos
    (§5.2). Devuelve los días de cada nodo en orden. (El reparto del entrenamiento distribuido lo hace
    cmd/mapa-coord; esta versión es la de las corridas locales y de la inferencia por partes.)"""
    dias = sorted(dias)
    bloques = [dias[k:k + tam] for k in range(0, len(dias), tam)]
    orden = np.random.default_rng(semilla).permutation(len(bloques))
    return [sorted(d for b in orden[n::n_nodos] for d in bloques[b]) for n in range(n_nodos)]


def dias_a_bajar(asignados: list[str], disponibles: list[str]) -> list[str]:
    """Los días asignados más el día anterior a cada bloque (la ventana de 24 h), si existe."""
    hay = set(disponibles)
    necesarios = set(asignados)
    for d in asignados:
        a = dia_anterior(d)
        if a in hay:
            necesarios.add(a)
    return sorted(necesarios)


# ---------------------------------------------------------------- serie y muestras


class Serie:
    """Los días cargados ([24, C, filas, columnas] cada uno) para armar ventanas de 24 horas.

    Cada día puede ser un tensor de torch (en RAM o en la GPU) o un arreglo de numpy (por ejemplo un
    memmap, para entrenar en un solo equipo con más días de los que caben en memoria).
    """

    def __init__(self, dias: dict[str, torch.Tensor | np.ndarray]):
        self.dias = dict(sorted(dias.items()))
        primero = next(iter(self.dias.values()))
        _, self.n_canales, self.filas, self.columnas = primero.shape

    @classmethod
    def cargar(cls, origen: Origen, dias: list[str], dispositivo: torch.device | str | None = None,
               mmap: bool = False) -> "Serie":
        datos: dict[str, torch.Tensor | np.ndarray] = {}
        for d in dias:
            ruta = origen.ruta(ruta_dia(d))
            if mmap:
                datos[d] = np.load(ruta, mmap_mode="r")
            else:
                datos[d] = torch.from_numpy(np.load(ruta)).to(dispositivo or "cpu")
        return cls(datos)

    def nbytes(self) -> int:
        return sum(int(np.prod(x.shape)) * 2 for x in self.dias.values())

    def tiene_ventana(self, dia: str, hora: int) -> bool:
        return dia in self.dias and (hora == HORAS - 1 or dia_anterior(dia) in self.dias)

    def ventana(self, dia: str, hora: int, rellenar: bool = False) -> torch.Tensor:
        """Las 24 horas que terminan en (dia, hora), de la más vieja a la más nueva, como
        [24·C, filas, columnas] en float16. Con `rellenar`, si falta el día anterior se repite la hora
        00 del día (solo para la inferencia del primer día del periodo)."""
        hoy = self.dias[dia]
        if hora == HORAS - 1:
            partes = [hoy[:HORAS]]
        else:
            anterior = self.dias.get(dia_anterior(dia))
            if anterior is None:
                if not rellenar:
                    raise KeyError(f"falta el día anterior a {dia} para la ventana de la hora {hora}")
                faltan = HORAS - 1 - hora
                partes = [hoy[:1]] * faltan + [hoy[:hora + 1]]
            else:
                partes = [anterior[hora + 1:], hoy[:hora + 1]]
        if isinstance(hoy, np.ndarray):
            v = torch.from_numpy(np.concatenate([np.asarray(p) for p in partes]))
        else:
            v = torch.cat(partes)
        return v.reshape(HORAS * self.n_canales, self.filas, self.columnas)


def armar_entrada(serie: Serie, estaticos: torch.Tensor, horas: list[tuple[str, int]],
                  rellenar: bool = False) -> torch.Tensor:
    """La entrada de la red para estas horas: [B, 24·C + S + 4, filas, columnas] en float32, en el
    dispositivo de `estaticos`. El orden de los canales es ventana (hora más vieja primero, C canales por
    hora), estáticos, calendario."""
    dev = estaticos.device
    xs = []
    for d, h in horas:
        v = serie.ventana(d, h, rellenar).to(dev, non_blocking=True).float()
        cal = torch.from_numpy(calendario(d, h, serie.filas, serie.columnas)).to(dev)
        xs.append(torch.cat([v, estaticos, cal]))
    return torch.stack(xs)


def _log1p_seguro(v: np.ndarray) -> np.ndarray:
    return np.log1p(np.clip(v, 0, None))


class Conjunto:
    """Las horas con etiqueta de un grupo de días, separadas en entrenamiento y validación por estación.

    Para cada hora (dia, hora) con ventana completa guarda las estaciones con valor: las de
    `validacion` van a la pérdida de validación y las demás a la de entrenamiento.
    """

    def __init__(self, serie: Serie, estaticos: np.ndarray, etiquetas: Etiquetas, dias: list[str],
                 validacion: set[str] | list[str], dispositivo: torch.device | str = "cpu"):
        self.serie = serie
        self.dispositivo = torch.device(dispositivo)
        self.estaticos = torch.as_tensor(estaticos, dtype=torch.float32, device=self.dispositivo)
        self.validacion = set(validacion)
        mios = set(dias)
        sel = np.array([d in mios for d in etiquetas.dia], bool) & ~np.isnan(etiquetas.valores).all(axis=1)
        e = etiquetas.filtrar(sel)
        grupos: dict[tuple[str, int], list[int]] = {}
        for k, (d, h) in enumerate(zip(e.dia, e.hora)):
            grupos.setdefault((d, int(h)), []).append(k)
        self.etiquetas = e
        self.filas_ent: dict[tuple[str, int], np.ndarray] = {}
        self.filas_val: dict[tuple[str, int], np.ndarray] = {}
        es_val = np.array([s in self.validacion for s in e.estacion], bool)
        for clave in sorted(grupos):
            if not serie.tiene_ventana(*clave):
                continue
            filas = np.array(grupos[clave])
            ent, val = filas[~es_val[filas]], filas[es_val[filas]]
            if len(ent):
                self.filas_ent[clave] = ent
            if len(val):
                self.filas_val[clave] = val
        self.horas_ent = list(self.filas_ent)
        self.horas_val = list(self.filas_val)

    def entrada(self, horas: list[tuple[str, int]]) -> torch.Tensor:
        """[B, 24·C + S + 4, filas, columnas] en float32 en el dispositivo."""
        return armar_entrada(self.serie, self.estaticos, horas)

    def objetivos(self, horas: list[tuple[str, int]], val: bool = False):
        """Etiquetas del lote como índices dispersos: (b, i, j, y [L, 2] en log1p, m [L, 2] 0/1)."""
        tabla = self.filas_val if val else self.filas_ent
        bs, filas = [], []
        for b, clave in enumerate(horas):
            f = tabla.get(clave, np.empty(0, np.int64))
            bs.append(np.full(len(f), b))
            filas.append(f)
        f = np.concatenate(filas).astype(np.int64)
        b = np.concatenate(bs).astype(np.int64)
        v = self.etiquetas.valores[f]
        m = ~np.isnan(v)
        y = np.where(m, _log1p_seguro(np.nan_to_num(v)), 0.0)
        a = lambda x, t: torch.as_tensor(x, dtype=t, device=self.dispositivo)  # noqa: E731
        return (a(b, torch.long), a(self.etiquetas.i[f], torch.long), a(self.etiquetas.j[f], torch.long),
                a(y, torch.float32), a(m, torch.float32))
