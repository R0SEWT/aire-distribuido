import json

import numpy as np
import torch

import datos
import red


def _conjunto(sintetico_dir, dias=None, validacion=("SIN-02",)):
    o = datos.Origen(str(sintetico_dir))
    m = o.manifiesto
    dias = dias or m.dias
    serie = datos.Serie.cargar(o, datos.dias_a_bajar(dias, m.dias))
    est = datos.cargar_estaticos(o.ruta("estaticos.npy"))
    e = datos.leer_etiquetas(o.ruta("etiquetas.csv"))
    return m, datos.Conjunto(serie, est, e, dias, set(validacion))


def test_formas_y_parametros():
    modelo = red.UNet(entrada=24 * 12 + 6 + 4, salida=2, ancho=8)
    y = modelo(torch.randn(2, 298, 24, 16))
    assert y.shape == (2, 2, 24, 16)
    # A tamaño real con 13 canales: 32-64-128-256 da ~2 M de parámetros.
    grande = red.UNet(24 * 13 + 6 + 4, 2, 32)
    assert 1.9e6 < red.n_parametros(grande) < 2.2e6
    assert [t["forma"][0] for t in red.tensores(grande) if t["nombre"].endswith("0.weight")][:4] == [32, 64, 128, 256]
    assert not any(isinstance(m, torch.nn.BatchNorm2d) for m in grande.modules())


def test_aplanar_y_cargar_plano_bit_a_bit(tmp_path):
    arq = red.arquitectura(298, ancho=8)
    a = red.construir(arq, semilla=1)
    v = red.aplanar(a)
    assert v.dtype == np.float32 and v.shape == (red.n_parametros(a),)
    b = red.construir(arq, semilla=2)
    assert not np.array_equal(red.aplanar(b), v)
    red.cargar_plano(b, v)
    assert red.aplanar(b).tobytes() == v.tobytes()
    for (na, pa), (nb, pb) in zip(a.named_parameters(), b.named_parameters()):
        assert na == nb and torch.equal(pa, pb)
    # Por bytes little-endian y de vuelta, también.
    assert red.de_bytes(red.a_bytes(v)).tobytes() == v.tobytes()
    # Exportar: pesos_0.bin + pesos.json con desplazamientos que cubren el vector.
    d = red.exportar(a, arq, tmp_path)
    assert (tmp_path / "pesos_0.bin").stat().st_size == 4 * d["n"]
    j = json.loads((tmp_path / "pesos.json").read_text())
    ultimo = j["tensores"][-1]
    assert ultimo["desde"] + ultimo["n"] == j["n"] == v.size
    assert all(t["n"] == int(np.prod(t["forma"])) for t in j["tensores"])
    red.exportar(b, arq, tmp_path, nombre="pesos_p1.bin")  # otro vector de la misma red, al lado
    j2 = json.loads((tmp_path / "pesos.json").read_text())
    assert sorted(j2["archivos"]) == ["pesos_0.bin", "pesos_p1.bin"] and j2["tensores"] == j["tensores"]
    c = red.cargar_modelo(tmp_path / "pesos_0.bin", tmp_path / "pesos.json")
    assert red.aplanar(c).tobytes() == v.tobytes()
    # El tensor k sale del vector en su desplazamiento.
    t = j["tensores"][3]
    p = dict(a.named_parameters())[t["nombre"]]
    assert np.array_equal(v[t["desde"]:t["desde"] + t["n"]].reshape(t["forma"]), p.detach().numpy())


def test_perdida_ignora_celdas_sin_etiqueta():
    torch.manual_seed(0)
    pred = torch.randn(2, 2, 8, 8)
    b = torch.tensor([0, 1, 1])
    i = torch.tensor([1, 2, 5])
    j = torch.tensor([3, 4, 6])
    y = torch.tensor([[1.0, 2.0], [float("nan"), 3.0], [0.5, float("nan")]])
    m = (~torch.isnan(y)).float()
    y = torch.nan_to_num(y)
    p1, n1 = red.perdida_enmascarada(pred, b, i, j, y, m)
    assert n1 == 4
    esperado = ((pred[0, 0, 1, 3] - 1) ** 2 + (pred[0, 1, 1, 3] - 2) ** 2 + (pred[1, 1, 2, 4] - 3) ** 2
                + (pred[1, 0, 5, 6] - 0.5) ** 2) / 4
    assert torch.allclose(p1, esperado)
    # Cambiar celdas sin etiqueta (o el contaminante sin valor) no cambia la pérdida.
    otra = pred.clone()
    otra[:, :, 0, 0] += 100
    otra[1, 0, 2, 4] += 100  # PM2.5 de una celda que solo tiene PM10
    otra[1, 1, 5, 6] += 100  # PM10 de una celda que solo tiene PM2.5
    p2, _ = red.perdida_enmascarada(otra, b, i, j, y, m)
    assert torch.equal(p1, p2)
    # Un NaN en y con máscara 0 no contamina ni la pérdida ni el gradiente.
    y_nan = y.clone()
    y_nan[1, 0] = float("nan")
    q = pred.clone().requires_grad_()
    p3, _ = red.perdida_enmascarada(q, b, i, j, y_nan, m)
    p3.backward()
    assert torch.isfinite(p3) and torch.isfinite(q.grad).all() and torch.allclose(p3, p1)
    assert (q.grad != 0).sum() == 4


def test_la_red_sobreajusta_el_sintetico(sintetico_dir):
    m, c = _conjunto(sintetico_dir, validacion=())
    torch.manual_seed(0)
    modelo = red.UNet(m.n_entrada, 2, ancho=8)
    ent = red.Entrenador(modelo, c, lote=4, tasa=3e-3, decaimiento=0.0, semilla=0)
    perdidas = [ent.pasos(1)[1] for _ in range(200)]
    inicio, final = np.mean(perdidas[:10]), np.mean(perdidas[-10:])
    assert final < 0.3 * inicio and final < 0.1, (inicio, final)


def test_entrenador_conserva_el_estado_de_adamw(sintetico_dir):
    m, c = _conjunto(sintetico_dir, dias=["2020-01-02", "2020-01-03"])
    modelo = red.UNet(m.n_entrada, 2, ancho=8)
    ent = red.Entrenador(modelo, c, lote=2, tasa=1e-3)
    vistas, perdida = ent.pasos(3)
    assert vistas == 6 and perdida > 0
    estado = ent.opt.state[next(modelo.parameters())]
    assert int(estado["step"]) == 3
    red.cargar_plano(modelo, red.aplanar(modelo) * 0.5)  # lo que hace un PESOS
    ent.pasos(2)
    assert int(ent.opt.state[next(modelo.parameters())]["step"]) == 5
    val, cuantos = ent.perdida(c.horas_val)
    assert cuantos > 0 and val > 0
