import json
from datetime import date, datetime, timedelta

import numpy as np
import pytest

import preparar as pr
from comun import sha256_archivo
from sinteticos import (
    GRILLA_PRUEBA,
    LON_MAR,
    PIXELES_POR_CELDA,
    acumulado_era5land,
    cams,
    escribir_adm0,
    escribir_crudo,
    escribir_worldpop,
    instantaneo,
    orografia,
    tasa,
)


def horas_del_dia(d: date) -> list[datetime]:
    return [datetime(d.year, d.month, d.day) + timedelta(hours=h) for h in range(24)]


# --- Interpolación espacial ---------------------------------------------------------------------


def test_bilineal_reproduce_un_campo_lineal():
    lat_o = np.array([1.0, 0.75, 0.5, 0.25, 0.0])  # descendente, como ERA5
    lon_o = np.array([-82.0, -81.75, -81.5, -81.25])
    campo = 3.0 + 2.0 * lat_o[:, None] - 0.5 * lon_o[None, :]
    lat_d = np.array([0.6, 0.5, 0.1])
    lon_d = np.array([-81.6, -81.5, -81.3])
    pesos = pr.pesos_bilineales(lat_o, lon_o, lat_d, lon_d)
    obtenido = pr.bilineal(campo[None], pesos)[0]
    esperado = 3.0 + 2.0 * lat_d[:, None] - 0.5 * lon_d[None, :]
    np.testing.assert_allclose(obtenido, esperado, rtol=0, atol=1e-12)


def test_bilineal_en_puntos_coincidentes_no_mezcla_el_mar():
    # Con la grilla de ERA5-Land los puntos coinciden: se copia el valor y el NaN vecino no contamina.
    lat = np.array([0.6, 0.5, 0.4])
    lon = np.array([-81.6, -81.5, -81.4])
    campo = np.array([[np.nan, 1.0, 2.0], [np.nan, 3.0, 4.0], [5.0, 6.0, 7.0]])
    pesos = pr.pesos_bilineales(lat, lon, lat + 1e-9, lon - 1e-9)
    np.testing.assert_array_equal(pr.bilineal(campo, pesos), campo)


def test_bilineal_no_extrapola():
    with pytest.raises(ValueError):
        pr.pesos_bilineales(np.array([1.0, 0.0]), np.array([-82.0, -81.0]), np.array([1.1]), np.array([-81.5]))


# --- Tiempo ------------------------------------------------------------------------------------


def test_desacumular_era5land_con_la_00_utc_del_dia_anterior():
    d = date(2018, 1, 2)
    horas = horas_del_dia(d)
    tiempos = [horas[0] - timedelta(hours=1)] + horas  # de la 23 UTC del día anterior a la 23 UTC
    acumulado = np.array([acumulado_era5land(0, t, 0.0, 0.0) for t in tiempos])[:, None, None]
    obtenido = pr.desacumular(acumulado, np.array(tiempos, dtype="datetime64[ns]"), horas)[:, 0, 0]
    esperado = np.array([tasa(0, t, 0.0, 0.0) for t in horas])
    np.testing.assert_allclose(obtenido, esperado, rtol=1e-12)
    # La 00 UTC no es el acumulado del día (24 h): es solo la última hora del día anterior.
    assert acumulado[1, 0, 0] > 20 * obtenido[0]


def test_desacumular_sin_la_hora_anterior_falla():
    horas = horas_del_dia(date(2018, 1, 2))
    acumulado = np.zeros((24, 1, 1))
    with pytest.raises(KeyError):
        pr.desacumular(acumulado, np.array(horas, dtype="datetime64[ns]"), horas)


def test_cams_de_3_horas_a_horario():
    d = date(2018, 1, 2)
    horas = horas_del_dia(d)
    tiempos = [horas[0] + timedelta(hours=3 * k) for k in range(9)]  # 00…21 UTC y 00 UTC del día siguiente
    valores = np.array([cams(0, t, -11.0, -77.0) for t in tiempos])[:, None, None]
    obtenido = pr.interpolar_tiempo(valores, np.array(tiempos, dtype="datetime64[ns]"), horas)[:, 0, 0]
    esperado = np.array([cams(0, t, -11.0, -77.0) for t in horas])  # lineal en el tiempo: exacto
    np.testing.assert_allclose(obtenido, esperado, rtol=1e-12)
    # A la 01 UTC pesa 2/3 la de las 00 y 1/3 la de las 03.
    assert obtenido[1] == pytest.approx(2 / 3 * valores[0, 0, 0] + 1 / 3 * valores[1, 0, 0])
    with pytest.raises(ValueError):
        pr.interpolar_tiempo(valores[:8], np.array(tiempos[:8], dtype="datetime64[ns]"), horas)


def test_estadisticas_por_dia_se_fusionan_como_una_sola_pasada():
    rng = np.random.default_rng(0)
    dias = [rng.normal(size=(24, 2, 3, 4)) for _ in range(3)]
    dias[1][:, 0, 0, 0] = np.nan
    total = pr.Estadisticas.vacia(2)
    for x in dias:
        total = total.fusionar(pr.Estadisticas.de(x))
    todo = np.concatenate([x.transpose(1, 0, 2, 3).reshape(2, -1) for x in dias], axis=1)
    np.testing.assert_allclose(total.media(), np.nanmean(todo, axis=1), rtol=1e-12)
    np.testing.assert_allclose(total.desviacion(), np.nanstd(todo, axis=1), rtol=1e-12)


# --- De punta a punta, con grilla y periodo reducidos ----------------------------------------------


@pytest.fixture(scope="module")
def corrida(tmp_path_factory):
    raiz = tmp_path_factory.mktemp("mapa")
    crudo = raiz / "crudo"
    desde, hasta = date(2018, 1, 31), date(2018, 2, 2)  # cruza un cambio de mes
    escribir_crudo(crudo, desde, hasta)
    escribir_worldpop(crudo / "estaticos" / "worldpop.tif")
    escribir_adm0(crudo / "estaticos" / "adm0.geojson")
    grilla = GRILLA_PRUEBA
    argumentos = [
        "--crudo", str(crudo), "--desde", str(desde), "--hasta", str(hasta),
        "--estadisticas-desde", "2018-02-01", "--estadisticas-hasta", "2018-02-02",
        f"--grilla={grilla.lat_norte},{grilla.lon_oeste},{grilla.paso},{grilla.filas},{grilla.columnas}",
        "--worldpop", str(crudo / "estaticos" / "worldpop.tif"), "--adm0", str(crudo / "estaticos" / "adm0.geojson"),
    ]
    assert pr.main(argumentos + ["--salida", str(raiz / "dos"), "--procesos", "2"]) == 0
    assert pr.main(argumentos + ["--salida", str(raiz / "uno"), "--procesos", "1"]) == 0
    return raiz / "dos", raiz / "uno", desde, hasta


def test_forma_y_dtype_de_los_dias(corrida):
    salida, _, desde, hasta = corrida
    archivos = sorted((salida / "dias").glob("*.npy"))
    assert [a.name for a in archivos] == ["2018-01-31.npy", "2018-02-01.npy", "2018-02-02.npy"]
    for a in archivos:
        x = np.load(a)
        assert x.dtype == np.float16 and x.shape == (24, len(pr.CANALES), 8, 6)
        assert np.isfinite(x).all()


def test_manifiesto_con_sha256_de_cada_archivo(corrida):
    salida, _, _, _ = corrida
    m = json.loads((salida / "manifiesto.json").read_text())
    assert [c["nombre"] for c in m["canales"]] == [c.nombre for c in pr.CANALES]
    assert m["grilla"]["filas"] == 8 and m["grilla"]["columnas"] == 6
    assert m["periodo"]["dias"] == 3 and m["dias"]["forma"] == [24, len(pr.CANALES), 8, 6]
    assert [e["nombre"] for e in m["estaticos"]] == [
        "orografia", "mascara_tierra", "mascara_peru", "latitud", "longitud", "log1p_poblacion"]
    assert set(m["archivos"]) == {"dias/2018-01-31.npy", "dias/2018-02-01.npy", "dias/2018-02-02.npy", "estaticos.npy"}
    for relativa, info in m["archivos"].items():
        assert sha256_archivo(salida / relativa) == info["sha256"]
        assert (salida / relativa).stat().st_size == info["bytes"]
    assert {f["licencia"] for f in m["fuentes"] if f["nombre"].startswith(("ERA5", "CAMS"))} == {"CC-BY-4.0"}


def test_paralelo_igual_que_secuencial(corrida):
    dos, uno, _, _ = corrida
    m2 = json.loads((dos / "manifiesto.json").read_text())
    m1 = json.loads((uno / "manifiesto.json").read_text())
    assert m2["archivos"] == m1["archivos"]
    assert m2["canales"] == m1["canales"]


def fisico(salida, d: date, nombre: str) -> np.ndarray:
    """El canal de un día, desnormalizado con el manifiesto y sin log1p."""
    m = json.loads((salida / "manifiesto.json").read_text())
    k = [c["nombre"] for c in m["canales"]].index(nombre)
    c = m["canales"][k]
    x = np.load(salida / "dias" / f"{d}.npy")[:, k].astype(np.float64) * c["desviacion"] + c["media"]
    return np.expm1(x) if c["transformacion"] == "log1p" else x


def malla():
    return np.meshgrid(GRILLA_PRUEBA.lats(), GRILLA_PRUEBA.lons(), indexing="ij")


def test_valores_fisicos_de_cada_fuente(corrida):
    salida, _, _, _ = corrida
    d = date(2018, 2, 1)
    lat, lon = malla()
    tierra = lon >= LON_MAR
    horas = horas_del_dia(d)

    t2m = fisico(salida, d, "temperatura_2m")
    esperado = np.stack([instantaneo(0, t, lat, lon) for t in horas]) - 273.15
    np.testing.assert_allclose(t2m[:, tierra], esperado[:, tierra], atol=0.02)

    # El mar de ERA5-Land queda en 0 normalizado: la media del canal.
    m = json.loads((salida / "manifiesto.json").read_text())
    media_t2m = m["canales"][0]["media"]
    np.testing.assert_allclose(t2m[:, ~tierra], media_t2m, atol=1e-9)

    lluvia = fisico(salida, d, "precipitacion")  # mm en la hora que termina en h
    esperado = np.stack([tasa(5, t, lat, lon) for t in horas]) * 1000
    np.testing.assert_allclose(lluvia[:, tierra], esperado[:, tierra], rtol=0.01)

    blh = fisico(salida, d, "altura_capa_limite")
    esperado = np.stack([instantaneo(7, t, lat, lon) for t in horas])
    np.testing.assert_allclose(blh, esperado, atol=0.01)

    pm = fisico(salida, d, "cams_pm2_5")  # µg/m³, interpolado en el tiempo y el espacio
    esperado = np.stack([cams(0, t, lat, lon) for t in horas]) * 1e9
    np.testing.assert_allclose(pm, esperado, rtol=0.005)


def test_ultimo_dia_usa_la_00_utc_del_siguiente_en_cams(corrida):
    salida, _, _, hasta = corrida
    lat, lon = malla()
    pm = fisico(salida, hasta, "cams_pm10")
    esperado = np.stack([cams(1, t, lat, lon) for t in horas_del_dia(hasta)]) * 1e9
    np.testing.assert_allclose(pm[22:], esperado[22:], rtol=0.005)


def test_normalizacion_con_el_periodo_de_estadisticas(corrida):
    salida, _, _, _ = corrida
    m = json.loads((salida / "manifiesto.json").read_text())
    assert m["estadisticas"] == {"desde": "2018-02-01", "hasta": "2018-02-02", "dias": 2}
    k = [c["nombre"] for c in m["canales"]].index("altura_capa_limite")
    x = np.concatenate([np.load(salida / "dias" / f"2018-02-0{n}.npy")[:, k].astype(np.float64) for n in (1, 2)])
    assert abs(x.mean()) < 1e-3 and abs(x.std() - 1) < 1e-3


def test_estaticos(corrida):
    salida, _, _, _ = corrida
    m = json.loads((salida / "manifiesto.json").read_text())
    e = np.load(salida / "estaticos.npy")
    assert e.dtype == np.float32 and e.shape == (6, 8, 6)
    nombres = [c["nombre"] for c in m["estaticos"]]

    def crudo(nombre):
        c = m["estaticos"][nombres.index(nombre)]
        return e[nombres.index(nombre)].astype(np.float64) * c["desviacion"] + c["media"]

    lat, lon = malla()
    np.testing.assert_allclose(crudo("orografia"), orografia(lat, lon), atol=1e-3)
    np.testing.assert_array_equal(crudo("mascara_tierra"), np.where(lon < LON_MAR, 0.0, 1.0))
    peru = np.zeros((8, 6))
    peru[3:, 2:] = 1  # latitud ≤ −11.3 y longitud ≥ −77.3
    np.testing.assert_array_equal(crudo("mascara_peru"), peru)
    np.testing.assert_allclose(crudo("latitud"), lat, atol=1e-6)
    assert e[nombres.index("latitud")].max() == pytest.approx(1) and e[nombres.index("latitud")].min() == pytest.approx(-1)
    poblacion = np.expm1(crudo("log1p_poblacion"))
    esperada = np.full((8, 6), PIXELES_POR_CELDA**2, dtype=float)
    esperada[0, 0] = 0  # sin dato
    np.testing.assert_allclose(poblacion, esperada, rtol=1e-5, atol=1e-4)


def test_mascara_peru_es_la_fraccion_de_la_celda(tmp_path):
    # Borde oeste en −77.3, el punto de la columna 2: esa columna queda mitad dentro.
    poligono = [[-77.3, -10.0], [-70.0, -10.0], [-70.0, -13.0], [-77.3, -13.0], [-77.3, -10.0]]
    ruta = tmp_path / "pais.geojson"
    ruta.write_text(json.dumps({"type": "FeatureCollection", "features": [
        {"type": "Feature", "properties": {}, "geometry": {"type": "Polygon", "coordinates": [poligono]}}]}))
    m = pr.mascara_pais(ruta, GRILLA_PRUEBA)
    np.testing.assert_allclose(m[:, :2], 0.0)
    np.testing.assert_allclose(m[:, 2], 0.5)
    np.testing.assert_allclose(m[:, 3:], 1.0)
