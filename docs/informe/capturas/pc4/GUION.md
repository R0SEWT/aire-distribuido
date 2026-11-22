# Guion de capturas de la PC4

El docente pide capturas de pantalla reales. Los `.txt` de esta carpeta son la salida en texto de
una corrida real (2026-11-20, Compose con 3 nodos): sirven de referencia de lo que tiene que verse,
no reemplazan a las capturas. Las capturas se toman a mano con este guion y van en esta carpeta
(`docs/informe/capturas/pc4/`) con el nombre de archivo indicado, en PNG.

| Bloque | Capturas | Tiempo estimado |
|---|---|---|
| Preparación (levantar, importar la colección, correrla una vez) | 0 | 10 min |
| A. Docker Compose y clúster | 2 | 4 min |
| B. Postman, una por ruta | 18 | 20 min |
| C. Swagger UI | 2 | 4 min |
| D. MongoDB y Redis | 8 | 12 min |
| E. Entrenamiento, carga y métricas | 3 | 8 min |
| **Total** | **33** | **cerca de 1 hora** |

Si el tiempo no alcanza, el mínimo que cubre la rúbrica son las 19 marcadas con ★ (unos 35 minutos
con la preparación).

Reglas para todas:

- **El comando o el pedido a la vista**, completo, y debajo su resultado. Sin recortar la línea del
  comando.
- En Postman, que se vean el **código HTTP y el tiempo en ms** (arriba a la derecha de la respuesta)
  y la pestaña **Test Results** abierta, con todas las pruebas en verde. Una de ellas dice
  «responde en menos de 100 ms (tiempo HTTP completo: N ms)».
- **Ninguna clave ni token en la imagen.** En la terminal, las claves se cargan antes con `read -rs`
  y se limpia la pantalla (está en cada bloque). En Postman, las claves van en el entorno, que no
  sale en la captura; en `postman-token.png`, recortar o tapar el valor de `token`.
- Los comandos son de bash (Linux, macOS, WSL o Git Bash) y usan `jq` para recortar el JSON a lo que
  importa; sin `jq`, se quita el `| jq …` y se captura el principio de la respuesta.

## Preparación

```bash
cp .env.example .env                       # completar JWT_SECRETO y API_USUARIOS
docker compose up -d --build --scale nodo=3
```

En Postman: *Import* → los dos `.json` de `docs/postman/`, elegir el entorno «aire-distribuido -
local» y completar `usuario_usuario`, `usuario_clave`, `admin_usuario` y `admin_clave` con los de
`API_USUARIOS` (detalle en `docs/postman/README.md`).

Antes de capturar, correr una vez, en este orden, para que las bases tengan registros:

1. *Run folder* sobre «1. Base HTTP», «2. Entrenamiento» y «3. Conflicto controlado».
2. *Run folder* sobre «4. Datos de prueba», con `docs/postman/lecturas_prueba.csv` como archivo de
   datos (78 iteraciones).

## A. Docker Compose y clúster

| # | Archivo | Comando o pedido | Qué tiene que verse | Referencia |
|---|---|---|---|---|
| 1 ★ | `compose-ps.png` | `docker compose ps -a` | `api`, `mongo` y `redis` en `healthy`; tres contenedores `nodo` en `Up`; `seed` en `Exited (0)`; el puerto `127.0.0.1:8080->8080` | `compose-ps.txt` |
| 2 | `nodos.png` | Postman, «1. Base HTTP» → «Nodos admin» (`GET /api/cluster/nodos`) | 200; tres nodos con `nombre`, `direccion`, `workers`, `latencia_ms` y `libre: true`; el tiempo en ms | `entrenamiento.txt` |

## B. Postman, una por ruta

Los pedidos son de la colección «aire-distribuido - PC4»; la carpeta está entre paréntesis.

| # | Archivo | Pedido (carpeta) y ruta | Qué tiene que verse |
|---|---|---|---|
| 3 ★ | `postman-salud.png` | Salud pública (1) · `GET /api/salud` | 200; `mongo: "ok"`, `redis: "ok"`, `nodos_en_pool: 3` y `modelo_activo` |
| 4 ★ | `postman-token.png` | Token usuario (1) · `POST /api/auth/token` | 200; `rol: "usuario"` y `expira`; el `token`, tapado |
| 5 ★ | `postman-prediccion.png` | Predicción válida (1) · `POST /api/predicciones` | 200; el cuerpo enviado y la respuesta con `regimen`, `excede_eca`, `media_24h`, `alerta`, `inca`, `modelo_id` y `ms` |
| 6 ★ | `postman-prediccion-400.png` | Fecha futura (1) · `POST /api/predicciones` | 400 y el error «fecha_hora no puede estar más de 1 hora adelante de la hora del servidor» |
| 7 | `postman-prediccion-401.png` | Predicción sin token (1) · `POST /api/predicciones` | 401 y «token ausente, inválido o expirado» |
| 8 ★ | `postman-estaciones.png` | Listar estaciones (1) · `GET /api/estaciones` | 200 y las primeras estaciones con su ficha y coordenadas |
| 9 ★ | `postman-regimen-estacion.png` | Régimen de estación (1) · `GET /api/estaciones/CA-AYAS-01/regimen` | 200 con `fecha_hora`, `regimen`, `media_24h`, `alerta`, `inca` y `ts` |
| 10 ★ | `postman-modelo.png` | Modelo activo (1) · `GET /api/modelos/activo` | 200; `id`, `k: 3`, `iteraciones`, `inercia` y los tres `regimenes` |
| 11 | `postman-403.png` | Métricas denegado a usuario (1) · `GET /api/metricas` | 403 y «se requiere el rol admin» |
| 12 ★ | `postman-metricas.png` | Métricas admin (1) · `GET /api/metricas` | 200; `POST /api/predicciones` con `p50_ms`, `p95_ms` y `p99_ms`; `ultimo_entrenamiento`; `escritura` con `mongo` y `redis` |
| 13 ★ | `postman-entrenamiento-202.png` | Aceptar entrenamiento (2) · `POST /api/entrenamientos` | 202; `id` (`e-AAAAMMDD-HHMM`) y `estado: "en_curso"`; el tiempo en ms. Si responde 422, esperar 3 s a que los nodos vuelvan al pool y reenviar |
| 14 | `postman-entrenamiento-422.png` | Nodos insuficientes (2) · `POST /api/entrenamientos` | 422 y «hay 3 nodos libres en el pool y se pidieron 99» |
| 15 | `postman-entrenamiento-409.png` | Rechazar entrenamiento simultáneo (3) · `POST /api/entrenamientos` | 409 y «ya hay un entrenamiento en curso: e-…». Se toma del Runner: *Run folder* sobre la carpeta 3 y, en los resultados, abrir ese pedido (enviado suelto responde 202, porque no hay nada en curso) |
| 16 ★ | `postman-entrenamiento-rondas.png` | Consultar hasta terminar (2) · `GET /api/entrenamientos/{id}` | 200; `estado: "terminado"`, `nodos` (3), `iteraciones: 44`, `paro`, `dif_secuencial`, `ms_total`, `modelo_id` y las primeras `rondas` con `ms_por_nodo` |
| 17 | `postman-historial.png` | Historial con el entrenamiento (2) · `GET /api/entrenamientos` | 200; la lista del más reciente al más antiguo, sin `rondas` |
| 18 | `postman-runner-coleccion.png` | Runner sobre las carpetas 1, 2 y 3 | El resumen de la corrida: todas las pruebas pasadas, 0 fallidas. El total de pedidos varía entre corridas (62 o 63) por los sondeos; ver `docs/postman/README.md` |
| 19 ★ | `postman-runner-csv.png` | Runner sobre la carpeta 4 con `lecturas_prueba.csv` | 78 iteraciones, 0 pruebas fallidas |
| 20 | `postman-alerta.png` | Runner de la carpeta 4, iteración 54 → «Predicción del CSV» | La última lectura de `serie_alerta` (`CA-HU-04`, `2021-12-11T00:00:00-05:00`): `media_24h` con `lecturas: 24`, `pm2_5: 80.36…`, `pm10: 117.24…`, `alerta: true`, `inca: "mala"` |

Referencia: `newman-coleccion.txt` (62 pedidos, 310 aserciones, 0 fallos) y `newman-runner-csv.txt`
(78 iteraciones, 714 aserciones, 0 fallos).

## C. Swagger UI

| # | Archivo | Dónde | Qué tiene que verse |
|---|---|---|---|
| 21 ★ | `swagger-ui.png` | Navegador, `http://localhost:8080/docs` | La barra de direcciones, el título «API coordinadora de aire-distribuido», el botón *Authorize* y las rutas agrupadas por etiqueta |
| 22 | `swagger-prediccion.png` | En `/docs`: *Authorize* con un token, `POST /api/predicciones` → *Try it out* → *Execute* | El cuerpo enviado, el código 200, la respuesta y la duración del pedido. Cerrar el diálogo *Authorize* antes de capturar |

## D. MongoDB y Redis, con registros escritos por la API

Cada captura es un comando y su salida. Referencia: `mongo.txt` y `redis.txt`.

```bash
# 23 ★ mongo-conteos.png: las cinco colecciones con registros (33 estaciones)
docker compose exec -T mongo mongosh --quiet aire_distribuido --eval \
  'for (const c of ["estaciones","predicciones","modelos","entrenamientos","eventos_cluster"]) print(c.padEnd(16), db.getCollection(c).countDocuments({}))'

# 24 ★ mongo-prediccion.png: un documento con media_24h, alerta: true, inca: "mala" y modelo_id
docker compose exec -T mongo mongosh --quiet aire_distribuido --eval \
  'printjson(db.predicciones.find({estacion: "CA-HU-04"}).sort({fecha_hora: -1, ts: -1}).limit(1).toArray()[0])'

# 25 mongo-modelo.png: el sembrado y los entrenados; uno solo con activo: true
docker compose exec -T mongo mongosh --quiet aire_distribuido --eval \
  'printjson(db.modelos.find({}, {origen: 1, creado: 1, activo: 1, iteraciones: 1, inercia: 1}).sort({creado: 1}).toArray())'

# 26 ★ mongo-entrenamiento.png: los entrenamientos con estado, parámetros, nodos y su rango de filas
docker compose exec -T mongo mongosh --quiet aire_distribuido --eval \
  'printjson(db.entrenamientos.find({}, {rondas: 0}).sort({inicio: -1}).limit(1).toArray())'

# 27 mongo-eventos.png: altas y bajas de los nodos, con su detalle
docker compose exec -T mongo mongosh --quiet aire_distribuido --eval \
  'printjson(db.eventos_cluster.find().sort({ts: 1}).limit(4).toArray())'

# 28 redis-claves.png: modelo:activo, estacion:*:regimen y entrenamiento:*:punto
docker compose exec -T redis redis-cli --scan --pattern '*' | sort

# 29 ★ redis-estacion.png: el estado de la estación de la serie con alerta (lecturas: 24, alerta: true)
docker compose exec -T redis redis-cli GET estacion:CA-HU-04:regimen

# 30 redis-punto.png: iter 44 y los centroides de la última ronda; el id sale de la captura 28
docker compose exec -T redis redis-cli GET entrenamiento:<id>:punto
```

## E. Entrenamiento, carga y métricas

Las claves se cargan sin que se vean y se limpia la pantalla antes de capturar:

```bash
read -rs CARGA_CLAVE && export CARGA_CLAVE                 # escribir la clave de demo y Enter
read -rs CARGA_CLAVE_ADMIN && export CARGA_CLAVE_ADMIN     # escribir la clave de admin y Enter
TOKEN=$(curl -s -X POST localhost:8080/api/auth/token \
  -d "{\"usuario\":\"admin\",\"clave\":\"$CARGA_CLAVE_ADMIN\"}" | jq -r .token)
clear
```

```bash
# 31 ★ entrenamiento.png: el 202 con el id y, debajo, el entrenamiento terminado con sus primeras rondas
curl -s -X POST localhost:8080/api/entrenamientos -H "Authorization: Bearer $TOKEN" \
  -d '{"nodos": 3, "replicas": 1, "semilla": 42}'
curl -s -H "Authorization: Bearer $TOKEN" localhost:8080/api/entrenamientos/<id> \
  | jq '{id, estado, nodos, iteraciones, paro, dif_secuencial, ms_total, modelo_id, rondas: .rondas[:2]}'

# 32 ★ carga.png: la tabla de los cuatro niveles (p50, p95, p99, máximo, pedidos/s, 0 errores)
#      y las líneas de escrituras de mongo y redis, sin ningún AVISO
go run ./cmd/carga -pedidos 20000

# 33 metricas.png: los percentiles del servidor y lo escrito en cada almacén
curl -s -H "Authorization: Bearer $TOKEN" localhost:8080/api/metricas \
  | jq '{prediccion: .endpoints["POST /api/predicciones"], nodos_en_pool, ultimo_entrenamiento, escrituras_descartadas, escrituras_fallidas, escritura}'
```

En la 31, el segundo comando va un segundo después del primero, con el `id` que devolvió. Si el
`POST` responde 422, los nodos todavía están volviendo al pool: repetirlo a los 3 segundos.
Referencia: `entrenamiento.txt`, `carga.txt` y `metricas.txt`.

## Al terminar

```bash
docker compose down        # con -v, además borra los datos
```

Revisar que estén los 33 archivos (o los 19 ★) con estos nombres exactos: el informe los referencia
por nombre.
