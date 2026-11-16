from pathlib import Path

import pytest

import sintetico


@pytest.fixture(scope="session")
def sintetico_dir(tmp_path_factory) -> Path:
    """Un conjunto sintético de 24×16 celdas y 30 días, compartido por todas las pruebas."""
    return sintetico.generar(tmp_path_factory.mktemp("mapa") / "datos", filas=24, columnas=16, n_dias=30,
                             n_estaciones=5, semilla=0)
