import json

import numpy as np

import agregar

G = {"filas": 24, "columnas": 16, "lat0": 0.6, "dlat": -0.1, "lon0": -81.6, "dlon": 0.1}


def _cuadro(lon0, lon1, lat0, lat1):
    return {"type": "Polygon", "coordinates": [[[lon0, lat0], [lon1, lat0], [lon1, lat1], [lon0, lat1], [lon0, lat0]]]}


def _limites(ruta):
    fc = {"type": "FeatureCollection", "features": [
        {"type": "Feature", "properties": {"shapeID": "A", "shapeName": "Distrito A"},
         "geometry": _cuadro(-81.65, -80.85, -1.85, 0.65)},
        {"type": "Feature", "properties": {"shapeID": "B", "shapeName": "Distrito B"},
         "geometry": _cuadro(-80.85, -80.05, -1.85, 0.65)},
    ]}
    ruta.write_text(json.dumps(fc))
    return ruta


def test_celdas_de_puntos():
    i, j, dentro = agregar.celdas_de_puntos(np.array([-81.6 + 0.3 - 0.03, -90.0]), np.array([0.6 - 0.5 + 0.02, 0.0]), G)
    assert (i[0], j[0], dentro[0]) == (5, 3, True) and not dentro[1]


def test_personas_dia_por_distrito(tmp_path):
    props, geoms = agregar.leer_limites(_limites(tmp_path / "limites.geojson"))
    assert [p["id"] for p in props] == ["A", "B"]
    # Dos píxeles en la celda (2, 1) del distrito A, uno en la (3, 10) del B y uno fuera de todo distrito.
    lat = np.array([0.6 - 0.2, 0.6 - 0.2 + 0.03, 0.6 - 0.3, -2.0])
    lon = np.array([-81.6 + 0.1, -81.6 + 0.1 - 0.02, -81.6 + 1.0, -81.0])
    pob = np.array([100.0, 50.0, 200.0, 1000.0])
    i, j, dentro = agregar.celdas_de_puntos(lon, lat, G)
    distrito = agregar.asignar_distritos(lon, lat, geoms)
    assert distrito.tolist() == [0, 0, 1, -1] and dentro[:3].all()
    dias = ["2020-12-30", "2020-12-31", "2021-01-01", "2021-01-02"]
    media = np.zeros((4, 2, 24, 16), np.float16)
    desv = np.zeros_like(media)
    media[:3, 0, 2, 1] = 60  # PM2.5 sobre el ECA (50) en A: 2 días de 2020 y 1 de 2021
    desv[:, 0, 2, 1] = 10
    media[3, 0, 3, 10] = 55  # en B, 1 día de 2021
    media[:, 1, 2, 1] = 99  # PM10 justo bajo el ECA (100)
    filas = agregar.por_distrito(dias, media, desv, i, j, pob, distrito, props)
    a, b = filas
    assert a["poblacion"] == 150 and b["poblacion"] == 200
    assert a["anios"]["2020"]["pm2_5"] == {"personas_dia": 300.0, "personas_dia_bajo": 0.0,
                                           "personas_dia_alto": 300.0, "dias_ponderados": 2.0}
    assert a["anios"]["2021"]["pm2_5"]["personas_dia"] == 150 and b["anios"]["2021"]["pm2_5"]["personas_dia"] == 200
    assert b["anios"]["2020"]["pm2_5"]["personas_dia"] == 0
    assert a["anios"]["2020"]["pm10"]["personas_dia"] == 0 and a["anios"]["2020"]["pm10"]["personas_dia_alto"] == 0
    assert a["anios"]["2020"]["dias"] == 2 and a["anios"]["2021"]["dias"] == 2
    anios, sobre = agregar.dias_sobre_eca(dias, media)
    assert anios == [2020, 2021] and sobre.shape == (2, 2, 24, 16)
    assert sobre[0, 0, 2, 1] == 2 and sobre[1, 0, 2, 1] == 1 and sobre[1, 0, 3, 10] == 1 and sobre.sum() == 4


def test_distrito_superpuesto_toma_el_primero():
    from shapely.geometry import shape
    geoms = [shape(_cuadro(0, 2, 0, 2)), shape(_cuadro(1, 3, 1, 3))]
    d = agregar.asignar_distritos(np.array([1.5, 2.5, 0.5, 9.0]), np.array([1.5, 2.5, 0.5, 9.0]), geoms)
    assert d.tolist() == [0, 1, 0, -1]


def test_distancia_a_la_estacion():
    d = agregar.distancia_estacion_km(np.array([[0.0, 1.0]]), np.array([[-75.0, -75.0]]), np.array([0.0]),
                                      np.array([-75.0]))
    assert d[0, 0] == 0 and abs(d[0, 1] - 111.19) < 0.1
