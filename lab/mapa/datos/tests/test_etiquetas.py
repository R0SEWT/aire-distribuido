from datetime import date

import pandas as pd
import pytest

import etiquetas as et
from grilla import GRILLA_NACIONAL

SILVER = """estacion,fecha_hora,co,pm10,pm2_5,temperatura,hora_inferida
A-01,2019-04-24T01:00:00-05:00,,50.5,,21.6,1
A-01,2019-04-24T02:00:00-05:00,,,,21.2,1
A-01,2019-04-24T03:00:00-05:00,,60,30.25,21.0,0
B-01,2019-04-24T00:00:00-05:00,,,12,20.0,0
C-01,2019-04-24T19:00:00-05:00,,80,,20.0,0
LEJOS,2019-04-24T01:00:00-05:00,,10,,20.0,0
D-01,2022-12-31T10:00:00-05:00,,70,,20.0,0
D-01,2022-12-31T19:00:00-05:00,,75,,20.0,0
"""

FICHA = """estacion,lat,lon,otra
A-01,-12.0702,-77.0430,x
B-01,-12.0400,-77.0300,y
C-01,-9.1033,-78.5618,z
D-01,-9.1033,-78.5618,w
LEJOS,5.0,-77.0,v
"""


@pytest.fixture
def entradas(tmp_path):
    silver = tmp_path / "silver.csv"
    silver.write_text(SILVER)
    ficha = tmp_path / "ficha.csv"
    ficha.write_text(FICHA)
    return silver, ficha


def test_a_utc_fin_con_las_dos_convenciones():
    s = pd.Series(["2019-04-24T01:00:00-05:00", "2019-04-24T23:00:00-05:00"])
    fin = et.a_utc_fin(s, "fin")
    assert list(fin.dt.strftime("%Y-%m-%dT%H:%M:%SZ")) == ["2019-04-24T06:00:00Z", "2019-04-25T04:00:00Z"]
    # Con convención inicio, la hora rotulada es el comienzo del intervalo: el fin es una hora después.
    inicio = et.a_utc_fin(s, "inicio")
    assert list(inicio.dt.strftime("%Y-%m-%dT%H:%M:%SZ")) == ["2019-04-24T07:00:00Z", "2019-04-25T05:00:00Z"]
    with pytest.raises(ValueError):
        et.a_utc_fin(s, "medio")


def test_etiquetas_de_oefa(entradas):
    silver, ficha = entradas
    crudas = et.leer_oefa(silver, ficha, "fin")
    tabla, resumen = et.armar([crudas], GRILLA_NACIONAL, date(2018, 1, 1), date(2022, 12, 31))
    assert list(tabla.columns) == ["hora_utc", "estacion", "fuente", "i", "j", "pm2_5", "pm10"]
    # La fila sin PM se descarta; la de A-01 a la 01:00 local es la de las 06:00 UTC, con pm2_5 vacío.
    a = tabla[tabla.estacion == "A-01"]
    assert list(a.hora_utc) == ["2019-04-24T06:00:00Z", "2019-04-24T08:00:00Z"]
    assert pd.isna(a.pm2_5.iloc[0]) and a.pm10.iloc[0] == 50.5
    assert a.pm2_5.iloc[1] == 30.25
    assert (a.i.iloc[0], a.j.iloc[0]) == (127, 46) and set(tabla.fuente) == {"oefa"}
    # B-01 no cae en la celda de A-01 (−12.04 → i = 126); C-01 y D-01 sí comparten celda.
    assert resumen["choques"] == [{"i": 97, "j": 30, "estaciones": ["C-01", "D-01"], "horas_simultaneas": 0}]
    assert resumen["fuera_de_grilla"] == ["LEJOS"]
    # D-01 a las 19:00 del 31/12 es la 00:00 UTC del 1/1/2023: fuera del periodo.
    assert resumen["fuera_de_periodo"] == 1
    assert resumen["filas"] == len(tabla) == 5
    assert resumen["estaciones"] == 4 and resumen["con_pm2_5"] == 2 and resumen["con_pm10"] == 4


def test_csv_ordenado_y_vacios(entradas, tmp_path):
    silver, ficha = entradas
    tabla, _ = et.armar([et.leer_oefa(silver, ficha, "fin")], GRILLA_NACIONAL, date(2018, 1, 1), date(2022, 12, 31))
    ruta = tmp_path / "etiquetas.csv"
    et.escribir(tabla, ruta)
    lineas = ruta.read_text().splitlines()
    assert lineas[0] == "hora_utc,estacion,fuente,i,j,pm2_5,pm10"
    assert lineas[1] == "2019-04-24T06:00:00Z,A-01,oefa,127,46,,50.5"
    assert [linea.split(",")[1] for linea in lineas[1:]] == ["A-01", "A-01", "B-01", "C-01", "D-01"]


def test_gancho_senamhi(tmp_path):
    assert et.leer_senamhi(tmp_path / "no-existe", "inicio") is None
    d = tmp_path / "senamhi"
    d.mkdir()
    (d / "lima.csv").write_text(
        "ESTACION,FECHA,HORA,PM10,PM2_5,NO2,LATITUD,LONGITUD\n"
        "CAMPO DE MARTE,2019-04-24,01:00,40,20,5,-12.0702,-77.0430\n"
        "CAMPO DE MARTE,2019-04-24,02:00,,,5,-12.0702,-77.0430\n"
    )
    with pytest.raises(ValueError):
        et.leer_senamhi(d, None)  # la convención horaria de SENAMHI no se supone: hay que decirla
    crudas = et.leer_senamhi(d, "inicio")
    tabla, _ = et.armar([crudas], GRILLA_NACIONAL, date(2018, 1, 1), date(2022, 12, 31))
    assert list(tabla.hora_utc) == ["2019-04-24T07:00:00Z"] and list(tabla.fuente) == ["senamhi"]
