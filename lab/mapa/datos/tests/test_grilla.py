import numpy as np

from grilla import GRILLA_NACIONAL, Grilla


def test_grilla_nacional_coordenadas_exactas():
    g = GRILLA_NACIONAL
    assert (g.filas, g.columnas) == (192, 136)
    lat, lon = g.lats(), g.lons()
    assert lat.shape == (192,) and lon.shape == (136,)
    assert lat[0] == 0.6 and lat[-1] == -18.5
    assert lon[0] == -81.6 and lon[-1] == -68.1
    # Cada punto es el double más cercano al decimal de la spec: 0.6 − 0.1·i y −81.6 + 0.1·j.
    for i in range(192):
        assert lat[i] == float(f"{0.6 - 0.1 * i:.1f}")
    for j in range(136):
        assert lon[j] == float(f"{-81.6 + 0.1 * j:.1f}")
    assert g.filas % 8 == 0 and g.columnas % 8 == 0


def test_celda_mas_cercana():
    g = GRILLA_NACIONAL
    assert g.celda(0.6, -81.6) == (0, 0)
    assert g.celda(-18.5, -68.1) == (191, 135)
    # Lima (Campo de Marte): −12.0702, −77.0430 → i = 127 (−12.1, el más cercano), j = 46 (−77.0)
    assert g.celda(-12.0702, -77.0430) == (127, 46)
    # Justo a 0.04 del punto sigue en la celda; fuera del borde (más de medio paso) no hay celda.
    assert g.celda(0.64, -81.64) == (0, 0)
    assert g.celda(0.66, -81.6) is None
    assert g.celda(-12.0, -68.06) == (126, 135)
    assert g.celda(-12.0, -68.04) is None


def test_grilla_reducida_y_dict():
    g = Grilla(lat_norte=-11.0, lon_oeste=-77.5, paso=0.1, filas=8, columnas=6)
    assert g.lats()[-1] == -11.7 and g.lons()[-1] == -77.0
    assert Grilla.desde_dict(g.a_dict()) == g


def test_centros_de_celda_del_raster():
    g = GRILLA_NACIONAL
    t = g.transformacion()
    # El centro del píxel (fila i, columna j) es el punto (lat_i, lon_j) de la grilla.
    x, y = t @ (0 + 0.5, 0 + 0.5)
    assert np.isclose(x, -81.6) and np.isclose(y, 0.6)
    x, y = t @ (135 + 0.5, 191 + 0.5)
    assert np.isclose(x, -68.1) and np.isclose(y, -18.5)
