#!/usr/bin/env python3
"""Manda web/public/datos/series.json a una API en marcha y compara lo que responde con lo esperado.

Es la verificación cruzada de las series (spec del frontend, §7.3 y §12): lo esperado se calculó en
scripts/impacto.py sin consultar a la API; aquí se le pregunta a la API real.

- Manda el escenario completo, en su orden, y después cada día suelto.
- En cada lectura compara media_24h, alerta e inca con la regla de ventana/ventana.go aplicada a
  lo ya enviado; en la última de cada estación, además, con el `esperado` del archivo (régimen,
  excede_eca, media_24h, alerta e inca) y con primera_media y primera_alerta.
- Al terminar el escenario pide GET /api/estaciones/{id}/regimen de cada estación: el último estado
  que vería el panel.

Hace falta una API limpia (sin lecturas previas de esas estaciones) con el modelo sembrado. Por
ejemplo, sin MongoDB ni Redis, desde la raíz del repositorio:

    JWT_SECRETO=$(openssl rand -hex 32) go run ./cmd/api -demo -addr 127.0.0.1:18080 -nodos-addr ""
    python3 scripts/verificar_series_api.py --api http://127.0.0.1:18080

El usuario y la clave se toman de API_USUARIO y API_CLAVE; sin ellas, los de demostración de
`-demo`. Sale con 1 si algo no coincide. Solo usa la biblioteca estándar.
"""

import argparse
import json
import os
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import impacto  # noqa: E402

CAMPOS = ("regimen", "excede_eca", "media_24h", "alerta", "inca")


class API:
    def __init__(self, base, usuario, clave):
        self.base = base.rstrip("/")
        self.token = None
        self.token = self.pedir("POST", "/api/auth/token", {"usuario": usuario, "clave": clave})["token"]

    def pedir(self, metodo, ruta, cuerpo=None):
        datos = None if cuerpo is None else json.dumps(cuerpo).encode("utf-8")
        pedido = urllib.request.Request(self.base + ruta, data=datos, method=metodo)
        pedido.add_header("Content-Type", "application/json")
        if self.token:
            pedido.add_header("Authorization", "Bearer " + self.token)
        try:
            with urllib.request.urlopen(pedido, timeout=10) as r:
                return json.loads(r.read())
        except urllib.error.HTTPError as e:
            raise RuntimeError(f"{metodo} {ruta}: HTTP {e.code} {e.read().decode('utf-8', 'replace')[:200]}") from None


def mandar(api, ventanas, lecturas, diferencias):
    """Manda las lecturas en orden. Devuelve, por estación, la última respuesta y las fecha_hora de
    la primera media y de la primera alerta que devolvió la API."""
    ultima, primera_media, primera_alerta = {}, {}, {}
    for l in lecturas:
        r = api.pedir("POST", "/api/predicciones", l)
        e, fh = l["estacion"], l["fecha_hora"]
        media = ventanas.agregar(e, impacto.hora_de(fh), l["pm2_5"], l["pm10"])
        local = {"media_24h": media, "alerta": impacto.alerta(media), "inca": impacto.inca(media)}
        for campo, valor in local.items():
            if r[campo] != valor:
                diferencias.append(f"{e} {fh} {campo}: la API dice {r[campo]!r} y la regla {valor!r}")
        if r["media_24h"]["pm2_5"] is not None:
            primera_media.setdefault(e, fh)
        if r["alerta"]:
            primera_alerta.setdefault(e, fh)
        ultima[e] = r
    return ultima, primera_media, primera_alerta


def comparar(nombre, estacion, esperado, respuesta, primera_media, primera_alerta, diferencias):
    antes = len(diferencias)
    for campo in CAMPOS:
        if respuesta[campo] != esperado[campo]:
            diferencias.append(f"{nombre} {estacion} {campo}: la API dice {respuesta[campo]!r} "
                               f"y series.json {esperado[campo]!r}")
    for campo, visto in (("primera_media", primera_media.get(estacion)), ("primera_alerta", primera_alerta.get(estacion))):
        if visto != esperado[campo]:
            diferencias.append(f"{nombre} {estacion} {campo}: la API la dio en {visto} y series.json dice {esperado[campo]}")
    return len(diferencias) == antes


def estado_guardado(api, estacion, fecha_hora):
    """El último estado de la estación; la API lo escribe en segundo plano, así que se espera un poco."""
    for _ in range(40):
        try:
            estado = api.pedir("GET", f"/api/estaciones/{estacion}/regimen")
            if estado["fecha_hora"] == fecha_hora:
                return estado
        except RuntimeError:
            estado = None
        time.sleep(0.05)
    return estado


def main():
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("--api", required=True, help="la URL de la API, por ejemplo http://127.0.0.1:18080")
    ap.add_argument("--series", default=str(impacto.RAIZ / impacto.SALIDA_SERIES))
    args = ap.parse_args()
    series = impacto.leer_json(args.series)
    api = API(args.api, os.environ.get("API_USUARIO", "demo"), os.environ.get("API_CLAVE", "demo"))
    modelo = api.pedir("GET", "/api/modelos/activo")["id"]
    ventanas = impacto.Ventanas()
    diferencias = []

    escenario = series["escenario"]
    ultima, primera_media, primera_alerta = mandar(api, ventanas, escenario["lecturas"], diferencias)
    bien = []
    for estacion, esperado in escenario["esperado"].items():
        antes = len(diferencias)
        comparar("escenario", estacion, esperado, ultima[estacion], primera_media, primera_alerta, diferencias)
        guardado = estado_guardado(api, estacion, esperado["fecha_hora"])
        if guardado is not None and impacto.hora_de(guardado["fecha_hora"]) > impacto.hora_de(esperado["fecha_hora"]):
            # El último estado de una estación no retrocede (spec §5.4).
            diferencias.append(f"escenario {estacion}: la API ya tenía un estado posterior ({guardado['fecha_hora']}); "
                               "no estaba limpia y el estado guardado no se puede comparar")
            continue
        for campo in CAMPOS + ("fecha_hora",):
            if guardado is None or guardado[campo] != esperado[campo]:
                diferencias.append(f"escenario {estacion} estado guardado, {campo}: "
                                   f"{None if guardado is None else guardado[campo]!r} y series.json {esperado[campo]!r}")
        if len(diferencias) == antes:
            bien.append(estacion)
    print(f"escenario ({len(escenario['lecturas'])} lecturas): {len(bien)} de {len(escenario['esperado'])} estaciones "
          f"coinciden en la última lectura y en el estado guardado")

    dias_bien = 0
    for dia in series["dias"]:
        ultima, primera_media, primera_alerta = mandar(api, ventanas, dia["lecturas"], diferencias)
        ok = comparar(dia["id"], dia["estacion"], dia["esperado"], ultima[dia["estacion"]],
                      primera_media, primera_alerta, diferencias)
        dias_bien += ok
        print(f"  {dia['id']} ({dia['caso']}, {len(dia['lecturas'])} lecturas): {'coincide' if ok else 'DIFERENCIA'}")
    total = len(escenario["lecturas"]) + sum(len(d["lecturas"]) for d in series["dias"])
    print(f"días sueltos: {dias_bien} de {len(series['dias'])} coinciden")
    print(f"{total} lecturas enviadas al modelo {modelo}; {len(diferencias)} diferencias")
    for d in diferencias[:50]:
        print("DIFERENCIA:", d, file=sys.stderr)
    return 1 if diferencias else 0


if __name__ == "__main__":
    raise SystemExit(main())
