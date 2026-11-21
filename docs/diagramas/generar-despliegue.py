"""Genera despliegue.drawio en la carpeta que se le pasa.

    python3 generar-despliegue.py .
    drawio -x -f png -e -b 12 -s 2 -o despliegue.png despliegue.drawio

El .drawio se puede editar a mano en draw.io; este script solo evita calcular las posiciones.
"""
import sys
from html import escape
from pathlib import Path

DEST = Path(sys.argv[1])


class Diagrama:
    def __init__(self):
        self.celdas = []
        self.n = 0

    def _id(self, prefijo):
        self.n += 1
        return f"{prefijo}{self.n}"

    def caja(self, x, y, w, h, valor, estilo, ident=None):
        ident = ident or self._id("c")
        self.celdas.append(f'<mxCell id="{ident}" value="{escape(valor, quote=True)}" style="{estilo}" vertex="1" parent="1">'
                           f'<mxGeometry x="{x}" y="{y}" width="{w}" height="{h}" as="geometry"/></mxCell>')
        return ident

    def flecha(self, origen, destino, estilo, valor="", puntos=(), pos=0):
        ident = self._id("e")
        pts = ""
        if puntos:
            pts = '<Array as="points">' + "".join(f'<mxPoint x="{x}" y="{y}"/>' for x, y in puntos) + "</Array>"
        self.celdas.append(f'<mxCell id="{ident}" value="{escape(valor, quote=True)}" style="{estilo}" edge="1" parent="1" '
                           f'source="{origen}" target="{destino}"><mxGeometry x="{pos}" relative="1" as="geometry">{pts}</mxGeometry></mxCell>')
        return ident

    def xml(self):
        return ('<mxGraphModel adaptiveColors="auto" grid="0" page="0">\n  <root>\n    <mxCell id="0"/>\n    <mxCell id="1" parent="0"/>\n    '
                + "\n    ".join(self.celdas) + "\n  </root>\n</mxGraphModel>\n")


FONDO = "swimlane;html=1;startSize=28;fontStyle=1;fontSize=13;fillColor=#ffffff;strokeColor=#555555;strokeWidth=2;swimlaneFillColor=#ffffff;"
TEXTO = "text;html=1;align=left;verticalAlign=top;whiteSpace=wrap;fontSize=11;fontColor=#333333;"
ETIQUETA = "edgeStyle=orthogonalEdgeStyle;rounded=1;html=1;endArrow=block;fontSize=10;fontColor=#333333;labelBackgroundColor=#ffffff;"
VERDE = ("#e3f1e5", "#2e7d32", "#1b3d1f")
ROJO = ("#fbe9e7", "#c62828", "#5e1512")
AZUL = ("#e6eef9", "#1f5fa8", "#13355e")
GRIS = ("#f1f1f1", "#666666", "#222222")
def servicio(d, x, y, w, h, titulo, lineas, color, forma="rounded=0;"):
    relleno, borde, texto = color
    return d.caja(x, y, w, h, (f"<b>{titulo}</b><br>" if titulo else "") + "<br>".join(lineas),
                  f"{forma}whiteSpace=wrap;html=1;fillColor={relleno};strokeColor={borde};fontColor={texto};fontSize=13;")


def despliegue():
    """En vertical, para una página A4: unas 1 060 unidades de ancho."""
    d = Diagrama()
    cil = "shape=cylinder3;boundedLbl=1;backgroundOutline=1;size=12;"
    d.caja(20, 150, 1040, 640, "Docker Compose · docker compose up --scale nodo=K · una red interna", FONDO)
    cli = servicio(d, 60, 20, 440, 70, "Clientes HTTP", ["Postman y newman · Swagger UI (/docs) · cmd/carga · navegador (WebSocket)"], GRIS)
    api = servicio(d, 40, 210, 480, 230, "api", ["imagen aire-distribuido · cmd/api", "", "REST con JWT, Swagger UI y WebSocket",
                                                  "coordinador del clúster: pool de nodos", "modelo activo y ventanas de 24 h en memoria",
                                                  "colas de escritura, una por base", "", "healthcheck: api -salud"], AZUL)
    for i in (2, 1, 0):
        nodo = servicio(d, 40 + 12 * i, 570 + 12 * i, 330, 130, "nodo × K" if i == 0 else "",
                        ["imagen aire-distribuido · cmd/nodo", "carga su rango del gold y responde", "cada ronda con su pool de workers",
                         "solo abre conexiones de salida"] if i == 0 else [], AZUL)
    mongo = servicio(d, 640, 200, 220, 140, "mongo", ["MongoDB", "volumen mongo_data", "healthcheck: mongosh ping"], VERDE, cil)
    seed = servicio(d, 900, 215, 145, 110, "seed", ["mongosh seed.js,", "una pasada:", "colecciones, índices", "y estaciones"], GRIS)
    redis = servicio(d, 640, 400, 220, 140, "redis", ["Redis con appendonly", "volumen redis_data", "healthcheck: redis-cli ping"], ROJO, cil)
    gold = servicio(d, 420, 600, 230, 90, "data/gold (no se versiona)", ["oefa_features.csv", "montado de solo lectura"], GRIS,
                    "shape=folder;tabWidth=70;tabHeight=14;tabPosition=left;")

    e = ETIQUETA.replace("fontSize=10", "fontSize=12")
    d.flecha(cli, api, e + "exitX=0.5;exitY=1;entryX=0.5;entryY=0;", "HTTP · 127.0.0.1:8080", pos=-0.6)
    d.flecha(nodo, api, e + "exitX=0.3;exitY=0;entryX=0.2;entryY=1;", "TCP api:7070<br>el nodo llama (HOLA)")
    d.flecha(api, mongo, e + "exitX=1;exitY=0.25;entryX=0;entryY=0.5;", "mongo:27017")
    d.flecha(api, redis, e + "exitX=1;exitY=0.8;entryX=0;entryY=0.5;", "redis:6379")
    d.flecha(seed, mongo, e + "exitX=0;exitY=0.5;entryX=1;entryY=0.5;", "")
    d.flecha(gold, api, e + "dashed=1;dashPattern=5 4;endArrow=open;exitX=0.25;exitY=0;entryX=0.91;entryY=1;", "para entrenar")
    d.flecha(gold, nodo, e + "dashed=1;dashPattern=5 4;endArrow=open;exitX=0;exitY=0.6;entryX=1;entryY=0.5;", "su rango")
    d.caja(700, 580, 345, 200, "<b>Orden de arranque</b>: mongo y redis sanos → seed termina con 0 → api sana → nodos, que "
           "reintentan hasta que api:7070 responde.<br><b>Puertos</b>: al anfitrión se publican, solo en 127.0.0.1, el 8080 (api), el "
           "27017 (mongo) y el 6379 (redis). El 7070 no se publica: lo usan los nodos de la red interna.<br><b>Secretos</b>: "
           "JWT_SECRETO y API_USUARIOS llegan por .env, que no se versiona.", TEXTO.replace("fontSize=11", "fontSize=12"))
    return d.xml()


(DEST / "despliegue.drawio").write_text(despliegue(), encoding="utf-8")
print("escrito", DEST / "despliegue.drawio")
