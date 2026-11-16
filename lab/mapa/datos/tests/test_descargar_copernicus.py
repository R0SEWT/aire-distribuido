import zipfile
from datetime import date, datetime

import numpy as np
import pytest
import xarray as xr

import descargar_copernicus as dc
from crudo import CAMS, ERA5, ERA5LAND, abrir, area, horas_esperadas, ruta_mes, validar
from grilla import GRILLA_NACIONAL
from sinteticos import GRILLA_PRUEBA, escribir_fuente


def test_area_alineada_a_la_grilla_nativa():
    assert area(ERA5LAND, GRILLA_NACIONAL) == [0.6, -81.6, -18.5, -68.1]
    # ERA5 (0.25°) y CAMS (0.75°) con una celda nativa de margen, para interpolar sin extrapolar.
    assert area(ERA5, GRILLA_NACIONAL) == [1.0, -82.0, -18.75, -67.75]
    assert area(CAMS, GRILLA_NACIONAL) == [1.5, -82.5, -19.5, -66.75]


def test_pedidos_del_periodo_completo(tmp_path):
    pedidos = dc.armar_pedidos(date(2017, 12, 31), date(2022, 12, 31), tmp_path, GRILLA_NACIONAL, ["era5land", "era5", "cams"])
    por_fuente = {}
    for p in pedidos:
        por_fuente.setdefault(p.fuente, []).append(p)
    # 61 meses de ERA5-Land más las invariantes; 61 de ERA5; 62 de CAMS (enero de 2023 trae solo el 1).
    assert len(por_fuente["era5land"]) == 62
    assert len(por_fuente["era5"]) == 61
    assert len(por_fuente["cams"]) == 62

    primero = por_fuente["era5land"][0].solicitud
    assert primero["year"] == "2017" and primero["month"] == "12" and primero["day"] == ["30", "31"]
    assert primero["variable"] == list(ERA5LAND.variables)
    assert primero["time"][0] == "00:00" and primero["time"][-1] == "23:00" and len(primero["time"]) == 24
    assert primero["data_format"] == "netcdf" and primero["area"] == [0.6, -81.6, -18.5, -68.1]

    era5 = por_fuente["era5"][0].solicitud
    assert era5["product_type"] == ["reanalysis"] and era5["day"] == ["31"]
    assert era5["variable"] == ["boundary_layer_height", "total_cloud_cover"]

    cams = por_fuente["cams"]
    assert cams[0].solicitud["date"] == ["2017-12-31/2017-12-31"]
    assert cams[1].solicitud["date"] == ["2018-01-01/2018-01-31"]
    assert cams[-1].solicitud["date"] == ["2023-01-01/2023-01-01"]
    assert cams[1].solicitud["time"] == ["00:00", "03:00", "06:00", "09:00", "12:00", "15:00", "18:00", "21:00"]
    assert cams[1].solicitud["data_format"] == "netcdf_zip"
    assert cams[1].destino == tmp_path / "cams" / "2018-01.nc"

    inv = [p for p in por_fuente["era5land"] if p.horas is None]
    assert len(inv) == 1
    assert inv[0].solicitud["variable"] == ["geopotential", "land_sea_mask"]
    assert "year" not in inv[0].solicitud


def test_simular_sin_credenciales(tmp_path, capsys):
    codigo = dc.main([
        "--simular", "--crudo", str(tmp_path), "--desde", "2018-01-01", "--hasta", "2018-01-31",
        "--rc-cds", str(tmp_path / "no-existe-cds"), "--rc-ads", str(tmp_path / "no-existe-ads"),
    ])
    salida = capsys.readouterr().out
    assert codigo == 0
    assert "reanalysis-era5-land" in salida and "cams-global-reanalysis-eac4" in salida
    assert not any(tmp_path.iterdir())  # no baja ni crea nada


def test_sin_credenciales_dice_que_archivo_crear(tmp_path, capsys):
    codigo = dc.main([
        "--crudo", str(tmp_path), "--desde", "2018-01-01", "--hasta", "2018-01-01",
        "--rc-cds", str(tmp_path / "cdsapirc"), "--rc-ads", str(tmp_path / "adsapirc"),
    ])
    error = capsys.readouterr().err
    assert codigo == 2
    assert str(tmp_path / "cdsapirc") in error and "url: https://cds.climate.copernicus.eu/api" in error
    assert str(tmp_path / "adsapirc") in error and "url: https://ads.atmosphere.copernicus.eu/api" in error


def test_credenciales_se_leen_sin_mostrarlas(tmp_path, capsys):
    rc = tmp_path / "adsapirc"
    rc.write_text("url: https://ads.atmosphere.copernicus.eu/api\nkey: secreto-123\n")
    url, clave = dc.credenciales("ads", rc)
    assert url == "https://ads.atmosphere.copernicus.eu/api" and clave == "secreto-123"
    with pytest.raises(dc.SinCredenciales) as e:
        dc.credenciales("cds", rc)  # el archivo es del otro servicio
    assert "secreto-123" not in str(e.value)
    assert "secreto-123" not in capsys.readouterr().out


def test_validar_y_saltar_lo_bajado(tmp_path):
    escribir_fuente(tmp_path, ERA5, date(2018, 1, 1), date(2018, 1, 2))
    pedidos = dc.armar_pedidos(date(2018, 1, 1), date(2018, 1, 2), tmp_path, GRILLA_PRUEBA, ["era5"])
    assert len(pedidos) == 1 and dc.ya_bajado(pedidos[0], GRILLA_PRUEBA)
    # Si se piden más días que los que tiene el archivo, deja de ser válido y se vuelve a bajar.
    pedidos = dc.armar_pedidos(date(2018, 1, 1), date(2018, 1, 3), tmp_path, GRILLA_PRUEBA, ["era5"])
    assert not dc.ya_bajado(pedidos[0], GRILLA_PRUEBA)
    motivo = validar(ruta_mes(tmp_path, ERA5, 2018, 1), ("blh", "nada"), None, GRILLA_PRUEBA)
    assert "nada" in motivo


def test_combinar_zip_de_instantaneas_y_acumuladas(tmp_path):
    escribir_fuente(tmp_path, ERA5LAND, date(2018, 1, 2), date(2018, 1, 2))
    with xr.open_dataset(ruta_mes(tmp_path, ERA5LAND, 2018, 1)) as ds:
        ds = ds.load()
    # Así entrega CDS un pedido con variables instantáneas y acumuladas: un zip con dos netCDF.
    ds[["t2m", "d2m", "u10", "v10", "sp"]].to_netcdf(tmp_path / "data_stream-oper_stepType-instant.nc")
    ds[["tp", "ssrd"]].to_netcdf(tmp_path / "data_stream-oper_stepType-accum.nc")
    with zipfile.ZipFile(tmp_path / "descarga.zip", "w") as z:
        z.write(tmp_path / "data_stream-oper_stepType-instant.nc", "data_stream-oper_stepType-instant.nc")
        z.write(tmp_path / "data_stream-oper_stepType-accum.nc", "data_stream-oper_stepType-accum.nc")
    salida = tmp_path / "combinado.nc"
    dc.combinar_zip(tmp_path / "descarga.zip", salida)
    horas = horas_esperadas(ERA5LAND, [date(2018, 1, 1), date(2018, 1, 2)])
    assert validar(salida, ERA5LAND.nombres_netcdf, horas, GRILLA_PRUEBA) is None
    with abrir(salida) as c:
        np.testing.assert_array_equal(c["tp"].values, ds["tp"].values)
        assert c["tp"].dims == ("time", "latitude", "longitude")


def test_normalizar_pronostico_y_longitudes_0_360(tmp_path):
    ref = np.array([datetime(2018, 1, 1, 0), datetime(2018, 1, 1, 12)], dtype="datetime64[ns]")
    periodo = np.array([3, 6], dtype="timedelta64[h]").astype("timedelta64[ns]")
    valido = ref[:, None] + periodo[None, :]
    ds = xr.Dataset(
        {"pm2p5": (("forecast_reference_time", "forecast_period", "latitude", "longitude"), np.arange(8.0).reshape(2, 2, 1, 2))},
        coords={
            "forecast_reference_time": ref,
            "forecast_period": periodo,
            "valid_time": (("forecast_reference_time", "forecast_period"), valido),
            "latitude": [-11.0],
            "longitude": [282.0, 283.0],
        },
    )
    ruta = tmp_path / "f.nc"
    ds.to_netcdf(ruta)
    with abrir(ruta) as n:
        assert list(n["time"].values.astype("datetime64[h]").astype(str)) == [
            "2018-01-01T03", "2018-01-01T06", "2018-01-01T15", "2018-01-01T18"]
        np.testing.assert_allclose(n["longitude"].values, [-78.0, -77.0])
        np.testing.assert_array_equal(n["pm2p5"].values[:, 0, 0], [0, 2, 4, 6])
