"""Genera docs/diagramas/entidades.drawio y despliegue.drawio (se corre una vez; el .drawio es la fuente)."""
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

    def flecha(self, origen, destino, estilo, valor="", puntos=()):
        ident = self._id("e")
        pts = ""
        if puntos:
            pts = '<Array as="points">' + "".join(f'<mxPoint x="{x}" y="{y}"/>' for x, y in puntos) + "</Array>"
        self.celdas.append(f'<mxCell id="{ident}" value="{escape(valor, quote=True)}" style="{estilo}" edge="1" parent="1" '
                           f'source="{origen}" target="{destino}"><mxGeometry relative="1" as="geometry">{pts}</mxGeometry></mxCell>')
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
ALTO_FILA, CABECERA = 17, 34


def entidad(d, x, y, w, nombre, sub, campos, color):
    """Una entidad: cabecera con el nombre y cuerpo con un campo por línea. Devuelve (id del cuerpo, alto)."""
    relleno, borde, texto = color
    alto_cuerpo = len(campos) * ALTO_FILA + 12
    d.caja(x, y, w, CABECERA, f"<b>{nombre}</b>" + (f"<br><font style=\"font-size: 10px;\">{sub}</font>" if sub else ""),
           f"rounded=0;whiteSpace=wrap;html=1;fillColor={relleno};strokeColor={borde};fontColor={texto};fontSize=12;")
    lineas = "<br>".join(f"<b>{c}</b> · {t}" if t else f"<b>{c}</b>" for c, t in campos)
    cuerpo = d.caja(x, y + CABECERA, w, alto_cuerpo, lineas,
                    f"rounded=0;whiteSpace=wrap;html=1;align=left;verticalAlign=top;spacingLeft=8;spacingTop=4;fillColor=#ffffff;"
                    f"strokeColor={borde};fontColor=#222222;fontSize=11;")
    return cuerpo, CABECERA + alto_cuerpo


def entidades():
    d = Diagrama()
    d.caja(20, 20, 1100, 790, "MongoDB · base aire_distribuido · lo persistente (cinco colecciones)", FONDO)
    d.caja(1150, 20, 390, 790, "Redis · la caché (tres familias de claves, valor JSON)", FONDO)

    est, _ = entidad(d, 40, 100, 250, "estaciones", "la escribe el seed, desde la ficha", [
        ("_id", "texto: código OEFA"), ("objeto_fiscalizado", "texto o null"), ("tipo_actividad", "texto o null"),
        ("departamento", "texto o null"), ("provincia", "texto o null"), ("distrito", "texto o null"),
        ("lat", "número"), ("lon", "número")], VERDE)
    pre, _ = entidad(d, 400, 100, 290, "predicciones", "índice estacion_ts {estacion, ts}", [
        ("_id", "ObjectId (lo genera el driver)"), ("ts", "fecha: cuándo se predijo"), ("estacion", "texto → estaciones._id"),
        ("fecha_hora", "fecha: hora de la lectura"), ("pm2_5", "número, µg/m³"), ("pm10", "número, µg/m³"),
        ("regimen", "entero"), ("excede_eca", "booleano, del régimen"), ("media_24h", "{pm2_5, pm10, lecturas}"),
        ("alerta", "booleano, de la media de 24 h"), ("inca", "texto o null: banda INCA"),
        ("modelo_id", "texto → modelos._id"), ("ms", "número: latencia en el servidor")], VERDE)
    mod, _ = entidad(d, 800, 100, 300, "modelos", "índices activo {activo} y creado {creado}", [
        ("_id", "texto: sembrado-… o entrenado-{id}"), ("creado", "fecha"), ("origen", "«sembrado» o → entrenamientos._id"),
        ("receta", "{features, k, semilla, tolerancia…}"), ("gold_sha256", "texto: huella del gold"),
        ("estandarizacion", "{features, n, media, desv…}"), ("centroides_std", "k × D números"),
        ("centroides", "k × D, unidades originales"), ("regimenes", "[{regimen, excede_eca, medias, tamano}]"),
        ("inercia", "número"), ("iteraciones", "entero"), ("activo", "booleano: a lo sumo uno true")], VERDE)
    ent, _ = entidad(d, 560, 450, 410, "entrenamientos", "índice inicio {inicio} · un documento por corrida distribuida", [
        ("_id", "texto: e-AAAAMMDD-HHMM"), ("inicio", "fecha"), ("fin", "fecha o null"),
        ("estado", "en_curso | terminado | abortado"), ("parametros", "{nodos, replicas, semilla}"),
        ("nodos", "[{nombre, direccion, workers, rango {desde, hasta}}]"),
        ("rondas", "[{iter, inercia, ms_ronda, ms_fusion, ms_por_nodo}]"), ("iteraciones", "entero"),
        ("paro", "tolerancia | max_iter | null"), ("dif_secuencial", "número o null"), ("ms_total", "número o null"),
        ("modelo_id", "texto o null → modelos._id"), ("aviso", "texto o null"), ("error", "texto o null: nombra al nodo")], VERDE)
    eve, _ = entidad(d, 40, 580, 250, "eventos_cluster", "índice ts {ts}", [
        ("_id", "ObjectId"), ("ts", "fecha"), ("tipo", "alta | baja | error"), ("nodo", "texto: nombre del nodo"),
        ("detalle", "texto")], VERDE)

    reg, _ = entidad(d, 1180, 100, 330, "estacion:{id}:regimen", "una clave por estación; no retrocede", [
        ("fecha_hora", "texto RFC 3339, hora de Lima"), ("regimen", "entero"), ("excede_eca", "booleano"),
        ("media_24h", "{pm2_5, pm10, lecturas}"), ("alerta", "booleano"), ("inca", "texto o null"),
        ("ts", "texto RFC 3339")], ROJO)
    act, _ = entidad(d, 1180, 320, 330, "modelo:activo", "una sola clave", [
        ("(valor)", "el documento de modelos que está"), ("", "")], ROJO)
    pun, _ = entidad(d, 1180, 590, 330, "entrenamiento:{id}:punto", "una clave por entrenamiento", [
        ("iter", "entero: última ronda completa"), ("centroides", "k × D números, estandarizados")], ROJO)

    solida = ETIQUETA
    punteada = ETIQUETA + "dashed=1;dashPattern=5 4;endArrow=open;"
    d.flecha(pre, est, solida + "exitX=0;exitY=0.18;entryX=1;entryY=0.3;", "N : 1")
    d.flecha(pre, mod, solida + "exitX=1;exitY=0.9;entryX=0;entryY=0.9;", "N : 1")
    d.flecha(ent, mod, solida + "exitX=0.8;exitY=0;entryX=0.3;entryY=1;startArrow=block;", "modelo_id ↔ origen<br>0..1 : 0..1")
    d.flecha(eve, ent, punteada + "exitX=1;exitY=0.6;entryX=0;entryY=0.6;", "nodo = nodos[].nombre")
    d.flecha(reg, est, punteada + "exitX=0.3;exitY=0;entryX=0.5;entryY=0;", "{id} = estaciones._id · el valor es el estado de su predicción de fecha_hora más reciente",
             puntos=[(1279, 68), (165, 68)])
    d.flecha(act, mod, punteada + "exitX=0;exitY=0.5;entryX=1;entryY=0.93;", "copia")
    d.flecha(pun, ent, punteada + "exitX=0;exitY=0.5;entryX=1;entryY=0.62;", "{id} = entrenamientos._id")

    d.caja(40, 724, 500, 76, "<b>Línea continua:</b> referencia por id entre documentos. MongoDB no impone claves foráneas: la "
           "relación la mantiene la API.<br><b>Línea punteada:</b> correspondencia por nombre, o copia en la caché.<br>"
           "Todas las fechas de MongoDB son BSON Date; en Redis, texto.", TEXTO)
    return d.xml()


def servicio(d, x, y, w, h, titulo, lineas, color, forma="rounded=0;"):
    relleno, borde, texto = color
    return d.caja(x, y, w, h, f"<b>{titulo}</b><br>" + "<br>".join(lineas),
                  f"{forma}whiteSpace=wrap;html=1;fillColor={relleno};strokeColor={borde};fontColor={texto};fontSize=11;")


def despliegue():
    d = Diagrama()
    cil = "shape=cylinder3;boundedLbl=1;backgroundOutline=1;size=12;"
    d.caja(250, 20, 1060, 640, "Docker Compose · docker compose up --scale nodo=K · una red interna", FONDO)
    cli = servicio(d, 20, 150, 180, 130, "Clientes HTTP", ["Postman y newman", "Swagger UI (/docs)", "cmd/carga", "navegador (WebSocket)"], GRIS)
    api = servicio(d, 300, 110, 300, 210, "api", ["imagen aire-distribuido · cmd/api", "", "REST con JWT, Swagger UI y WebSocket",
                                                   "coordinador del clúster: pool de nodos", "modelo activo y ventanas de 24 h en memoria",
                                                   "colas de escritura, una por base", "", "healthcheck: api -salud"], AZUL)
    for i in (2, 1, 0):
        nodo = servicio(d, 300 + 12 * i, 440 + 12 * i, 270, 110, "nodo × K" if i == 0 else "",
                        ["imagen aire-distribuido · cmd/nodo", "carga su rango del gold y responde", "cada ronda con su pool de workers",
                         "solo abre conexiones de salida"] if i == 0 else [], AZUL)
    mongo = servicio(d, 800, 80, 210, 130, "mongo", ["MongoDB", "volumen mongo_data", "healthcheck: mongosh ping"], VERDE, cil)
    seed = servicio(d, 1070, 95, 210, 100, "seed", ["mongosh seed.js, una pasada:", "colecciones, índices", "y estaciones de la ficha"], GRIS)
    redis = servicio(d, 800, 290, 210, 130, "redis", ["Redis con appendonly", "volumen redis_data", "healthcheck: redis-cli ping"], ROJO, cil)
    gold = servicio(d, 720, 500, 250, 90, "data/gold (no se versiona)", ["oefa_features.csv", "montado de solo lectura"], GRIS,
                    "shape=folder;tabWidth=70;tabHeight=14;tabPosition=left;")

    e = ETIQUETA
    d.flecha(cli, api, e + "exitX=1;exitY=0.5;entryX=0;entryY=0.5;", "HTTP<br>127.0.0.1:8080")
    d.flecha(nodo, api, e + "exitX=0.4;exitY=0;entryX=0.36;entryY=1;", "TCP api:7070<br>el nodo llama (HOLA)")
    d.flecha(api, mongo, e + "exitX=1;exitY=0.2;entryX=0;entryY=0.5;", "mongo:27017<br>lotes de predicciones,<br>entrenamientos, eventos")
    d.flecha(api, redis, e + "exitX=1;exitY=0.8;entryX=0;entryY=0.5;", "redis:6379<br>régimen por estación,<br>modelo activo, punto")
    d.flecha(seed, mongo, e + "exitX=0;exitY=0.5;entryX=1;entryY=0.5;", "")
    d.flecha(gold, api, e + "dashed=1;dashPattern=5 4;endArrow=open;exitX=0.2;exitY=0;entryX=0.9;entryY=1;", "para entrenar")
    d.flecha(gold, nodo, e + "dashed=1;dashPattern=5 4;endArrow=open;exitX=0;exitY=0.6;entryX=1;entryY=0.5;", "su rango")
    d.caja(1030, 300, 260, 140, "<b>Orden de arranque</b> (depends_on):<br>1. mongo y redis, sanos<br>2. seed termina con 0<br>"
           "3. api, sana<br>4. nodos: llaman a api:7070 y,<br>si no responde, reintentan", TEXTO)
    d.caja(1030, 480, 265, 150, "<b>Puertos</b>: al anfitrión se publican, solo en 127.0.0.1, el 8080 (api), el 27017 (mongo) y el 6379 "
           "(redis). El 7070 no se publica: lo usan solo los nodos de la red interna.<br><b>Secretos</b>: JWT_SECRETO y API_USUARIOS "
           "llegan por .env, que no se versiona.", TEXTO)
    return d.xml()


(DEST / "entidades.drawio").write_text(entidades(), encoding="utf-8")
(DEST / "despliegue.drawio").write_text(despliegue(), encoding="utf-8")
print("escritos", DEST / "entidades.drawio", DEST / "despliegue.drawio")
