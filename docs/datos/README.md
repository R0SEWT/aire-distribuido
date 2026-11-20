# Bases de datos de PC4

MongoDB guarda lo persistente y Redis es la caché (spec de la PC4, §7). La API las usa a través de las
interfaces de `almacen/`; las implementaciones están en `almacen/mongo` y `almacen/redis`, los dos únicos
paquetes que importan un driver. `docs/datos/ejemplos/` tiene un documento de ejemplo por colección y uno
por clave.

`docker-compose.yml` es un entorno de desarrollo: las bases no tienen autenticación y solo publican su
puerto en `localhost`. No debe exponerse fuera de la máquina ni presentarse como una configuración apta
para producción.

## Levantar y verificar

Desde la raíz del repositorio:

```bash
cp .env.example .env        # y completar JWT_SECRETO y API_USUARIOS
docker compose up -d --build
docker compose ps -a
```

`api`, `mongo` y `redis` deben aparecer como `healthy`, y `seed` como `Exited (0)`: es un servicio de una
sola pasada. Con `--scale nodo=3` entran tres nodos al pool de la API (`GET /api/cluster/nodos`) y un
admin puede lanzar un entrenamiento (`POST /api/entrenamientos`); la API y los nodos leen el gold de
`data/gold`, o de la carpeta que diga `GOLD_DIR`. Para trabajar solo con las bases (las pruebas de integración, por ejemplo):

```bash
docker compose up -d mongo redis seed
```

## El seed

`docs/datos/seed.js` crea las cinco colecciones y los índices de la spec, cada uno con nombre propio
(`activo`, `creado`, `inicio`, `estacion_ts`, `ts`), y carga las estaciones de
`reports/estaciones_ficha.csv`. Lo ejecuta el servicio `seed` en **cada** `docker compose up`, contra el
MongoDB ya sano; la API espera a que termine bien. Es idempotente y se puede repetir a mano:

```bash
docker compose run --rm seed
```

- **Valida antes de escribir**, con las reglas de `almacen/ficha.go`: toda estación tiene `lat` y `lon`,
  numéricas y en rango, y no se repite; una marca de orden de bytes (BOM) al inicio se ignora. Una ficha
  que la API no podría leer no entra: el seed sale con error y MongoDB queda como estaba.
- **Si falla**, `docker compose up` lo dice y la API no arranca. Se corrige la ficha y se vuelve a
  levantar: no hace falta borrar el volumen.
- **La ficha es el catálogo completo.** Las estaciones presentes se insertan o se reemplazan, y las que ya
  no están en la ficha se borran de MongoDB (`estaciones_retiradas` en la salida del seed). Los campos
  `NA`, `-` o vacíos se guardan como `null`; no se inventan metadatos ni se completan ubicaciones.
- `almacen/mongo` no crea índices: usa los del seed. Crear el mismo índice sin nombre fallaría con
  `IndexOptionsConflict`.

## Qué se guarda

| Dónde | Qué | Quién lo escribe |
|---|---|---|
| MongoDB `estaciones` | El catálogo: `_id` (código OEFA) y la ficha | El seed |
| MongoDB `predicciones` | Un documento por lectura predicha | La API, por lotes |
| MongoDB `modelos` | Los modelos; a lo sumo uno con `activo: true` | La API al arrancar y al activar un modelo |
| MongoDB `entrenamientos` | Cada corrida distribuida con sus rondas | El coordinador de la API |
| MongoDB `eventos_cluster` | Altas, bajas y errores de los nodos | El coordinador de la API |
| Redis `estacion:{id}:regimen` | El estado de la estación en su lectura más reciente | La API, por lotes |
| Redis `modelo:activo` | El modelo activo, para arrancar con él | La API al arrancar y al activar un modelo |
| Redis `entrenamiento:{id}:punto` | Los centroides de la última ronda completa | El coordinador de la API |

Decisiones de diseño:

- **Fechas.** En MongoDB todas son BSON Date, nunca texto: un campo tiene un solo tipo y el índice
  `{estacion, ts}` ordena por tiempo. Un BSON Date es un instante en UTC al milisegundo, así que se pierden
  el desfase `-05:00` con que llegó la fecha (la API la vuelve a expresar en la hora de Lima al
  responder) y lo que haya por debajo del milisegundo. Por eso la duración de un entrenamiento va en
  `ms_total` y no se calcula con `fin - inicio`. En Redis las fechas son texto RFC 3339, siempre en la
  hora de Lima y al segundo.
- **Lo que todavía no tiene valor es `null`**, no cero ni texto vacío: el `fin` de un entrenamiento en
  curso, la media de 24 h con menos de 18 lecturas, el `error` de una corrida que terminó bien.
- **El régimen de una estación no retrocede.** Una lectura más antigua que la guardada no la reemplaza
  (una de la misma hora, sí). La comparación y la escritura van juntas en un script de Lua, que Redis
  ejecuta de forma atómica.
- **El modelo activo.** Cuando un entrenamiento activa su modelo (`entrenado-{id}`), la API lo guarda en
  `modelos` con `activo: true` y en `modelo:activo`, con su procedencia: `origen` (el id del
  entrenamiento), `creado` y `gold_sha256`. Al arrancar lee el modelo de `reports/` y los dos guardados,
  y se queda con el entrenado más nuevo por `creado`; si las dos bases no coinciden (activar son dos
  escrituras y una puede fallar), gana el más nuevo y, con el mismo `creado`, el de MongoDB. La otra
  base queda al día. El modelo sembrado siempre se vuelve a leer de `reports/`, para que un
  entrenamiento nuevo con `cmd/entrenar` no quede tapado por una copia vieja.
- **El punto de control.** `entrenamiento:{id}:punto` guarda la iteración y los centroides
  estandarizados de la última ronda completa de una corrida que terminó. Se escribe una vez, al
  terminar: el gancho por ronda del clúster informa la inercia y los tiempos de cada ronda, pero no sus
  centroides. Es un registro; no se reanuda desde él. Una corrida abortada no lo deja.
- **La predicción no espera a las bases.** Se encola y una goroutine por almacén la escribe por lotes;
  si una base se cae, la API sigue prediciendo, `/api/salud` lo informa y `/api/metricas` cuenta lo que
  no se pudo escribir.
- **El tamaño de las colas.** Cada base tiene su cola, de `-cola` predicciones (1024 por omisión). Si
  llegan más rápido de lo que la base las escribe y la cola se llena, las que no entran se descartan y
  quedan contadas en `escrituras_descartadas`. El Compose pasa `-cola 32768`: con 1024, una ráfaga de
  20 000 predicciones llegó a perder un tercio en MongoDB; con 32768, ninguna.

## Consultar

```bash
docker compose exec -T mongo mongosh --quiet aire_distribuido --eval \
  'printjson({estaciones: db.estaciones.countDocuments({}), ejemplo: db.estaciones.findOne({_id: "CA-HUAR-01"})})'

docker compose exec -T mongo mongosh --quiet aire_distribuido --eval \
  'for (const c of ["modelos","entrenamientos","predicciones","eventos_cluster","estaciones"]) printjson({coleccion: c, indices: db.getCollection(c).getIndexes()})'

# Los entrenamientos, del más reciente al más antiguo, sin las rondas
docker compose exec -T mongo mongosh --quiet aire_distribuido --eval \
  'printjson(db.entrenamientos.find({}, {rondas: 0}).sort({inicio: -1}).toArray())'

# Los modelos y cuál está activo
docker compose exec -T mongo mongosh --quiet aire_distribuido --eval \
  'printjson(db.modelos.find({}, {origen: 1, creado: 1, activo: 1, iteraciones: 1, inercia: 1}).toArray())'

# Las últimas predicciones de una estación (usa el índice estacion_ts)
docker compose exec -T mongo mongosh --quiet aire_distribuido --eval \
  'printjson(db.predicciones.find({estacion: "CA-AYAS-01"}).sort({ts: -1}).limit(3).toArray())'

docker compose exec -T redis redis-cli GET estacion:CA-AYAS-01:regimen
docker compose exec -T redis redis-cli --scan --pattern 'estacion:*'
docker compose exec -T redis redis-cli --scan --pattern 'entrenamiento:*'
docker compose exec -T redis redis-cli CONFIG GET appendonly
```

## Pruebas de integración

El CI corre con el doble en memoria. Las pruebas contra MongoDB y Redis reales llevan la etiqueta de
compilación `integracion` y se corren con las bases del Compose levantadas:

```bash
docker compose up -d mongo redis seed
go test -race -tags integracion ./almacen/... ./cmd/api/
```

Leen la base sembrada (`aire_distribuido`) y escriben en bases aparte que borran al terminar: una base
temporal de MongoDB y las bases 14 y 15 de Redis. `MONGO_URI`, `MONGO_DB` y `REDIS_ADDR` dicen dónde
están si no es en `localhost`.

## Más de un entorno local

Para levantar dos entornos simultáneos se necesitan nombres de proyecto y puertos de host distintos:

```bash
docker compose -p pc4-a up -d
API_PORT=8081 MONGO_PORT=27018 REDIS_PORT=6380 docker compose -p pc4-b up -d
```

## Detener

```bash
docker compose down
```

Los volúmenes se conservan. `docker compose down -v` también los elimina y borra los datos locales.
