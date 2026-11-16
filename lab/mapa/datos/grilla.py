"""La grilla común del mapa (spec §3.3).

Fila i, columna j: latitud ``lat_norte − paso·i``, longitud ``lon_oeste + paso·j``. La nacional es
de 192 × 136 celdas de 0.1°, de 0.6° a −18.5° y de −81.6° a −68.1°, y coincide con los puntos de
ERA5-Land. Las pruebas usan grillas reducidas con el mismo código.
"""

from dataclasses import asdict, dataclass

import numpy as np
from rasterio.transform import Affine

# Las coordenadas se redondean a estos decimales para que cada punto sea el double más cercano al
# decimal de la spec (0.6 − 0.1·3 da 0.29999999999999993 sin redondear).
DECIMALES = 6


@dataclass(frozen=True)
class Grilla:
    lat_norte: float
    lon_oeste: float
    paso: float
    filas: int
    columnas: int

    def lats(self) -> np.ndarray:
        return np.round(self.lat_norte - self.paso * np.arange(self.filas), DECIMALES)

    def lons(self) -> np.ndarray:
        return np.round(self.lon_oeste + self.paso * np.arange(self.columnas), DECIMALES)

    def celda(self, lat: float, lon: float) -> tuple[int, int] | None:
        """La celda cuyo punto es el más cercano a (lat, lon), o None si cae fuera de la grilla."""
        i = int(np.floor((self.lat_norte - lat) / self.paso + 0.5))
        j = int(np.floor((lon - self.lon_oeste) / self.paso + 0.5))
        if 0 <= i < self.filas and 0 <= j < self.columnas:
            return i, j
        return None

    def transformacion(self) -> Affine:
        """Transformación afín de un raster [filas, columnas] cuyo píxel (i, j) es la celda (i, j)."""
        medio = self.paso / 2
        return Affine(self.paso, 0.0, self.lon_oeste - medio, 0.0, -self.paso, self.lat_norte + medio)

    def a_dict(self) -> dict:
        return asdict(self)

    @staticmethod
    def desde_dict(d: dict) -> "Grilla":
        return Grilla(**{k: d[k] for k in ("lat_norte", "lon_oeste", "paso", "filas", "columnas")})


GRILLA_NACIONAL = Grilla(lat_norte=0.6, lon_oeste=-81.6, paso=0.1, filas=192, columnas=136)
