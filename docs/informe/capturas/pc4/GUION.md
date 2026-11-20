# Guion de capturas de la PC4

El docente pide capturas de pantalla reales. Los `.txt` de esta carpeta son la salida en texto de
una corrida real (2026-11-20, Compose con 3 nodos) y sirven de referencia de lo que tiene que
verse; las capturas hay que tomarlas a mano con este guion. Cada captura lleva **el comando o el
pedido a la vista** y va en esta carpeta con el nombre indicado.

## Preparación

```bash
cp .env.example .env                       # completar JWT_SECRETO y API_USUARIOS
docker compose up -d --build --scale nodo=3
```

En Postman: importar `docs/postman/*.json`, elegir el entorno «aire-distribuido - local» y completar
las cuatro credenciales (ver `docs/postman/README.md`). Correr en orden las carpetas 1, 2 y 3, y la 4
en el Runner con `lecturas_prueba.csv`, antes de capturar: así las bases ya tienen registros.

## Docker Compose

| Archivo | Comando | Qué tiene que verse | Referencia |
|---|---|---|---|
| `compose-ps.png` | `docker compose ps -a` | `api`, `mongo` y `redis` en `healthy`, tres `nodo` arriba y `seed` en `Exited (0)` | `compose-ps.txt` |
| `nodos.png` | Postman, «Nodos admin» | Los tres nodos con `workers`, `latencia_ms` y `libre: true`; el tiempo en ms | `entrenamiento.txt` |

## Postman, una por ruta (con el tiempo en ms y la pestaña *Test Results* en verde)

| Archivo | Pedido de la colección | Qué tiene que verse |
|---|---|---|
| `postman-salud.png` | Salud pública | 200, `mongo` y `redis` en `ok`, `nodos_en_pool: 3` |
| `postman-token.png` | Token usuario | 200, `rol` y `expira` (tapar el token) |
| `postman-prediccion.png` | Predicción válida | 200 con `regimen`, `media_24h`, `alerta`, `inca` y `ms` |
| `postman-prediccion-400.png` | Fecha futura (o PM2.5 mayor de 10 000) | 400 y el mensaje del error |
| `postman-prediccion-401.png` | Predicción sin token | 401 |
| `postman-estaciones.png` | Listar estaciones | 200 y el catálogo |
| `postman-regimen-estacion.png` | Régimen de estación | 200 con `fecha_hora`, `alerta`, `inca` y `ts` |
| `postman-modelo.png` | Modelo activo | 200 con los tres regímenes |
| `postman-403.png` | Métricas denegado a usuario | 403, «se requiere el rol admin» |
| `postman-metricas.png` | Métricas admin | `endpoints` con p50, p95 y p99, `ultimo_entrenamiento` y `escritura` |
| `postman-entrenamiento-202.png` | Aceptar entrenamiento | 202 con el `id` y `en_curso` |
| `postman-entrenamiento-422.png` | Nodos insuficientes | 422 |
| `postman-entrenamiento-409.png` | Rechazar entrenamiento simultáneo | 409 con el id en curso (correr la carpeta 3 y abrir el pedido en el Runner) |
| `postman-entrenamiento-rondas.png` | Consultar hasta terminar | `terminado`, `iteraciones`, `dif_secuencial`, `modelo_id` y las `rondas` con `ms_por_nodo` |
| `postman-historial.png` | Historial con el entrenamiento | La lista, del más reciente al más antiguo |
| `postman-runner-coleccion.png` | Runner, carpetas 1 a 3 | El resumen: todas las pruebas pasadas y 0 fallidas |
| `postman-runner-csv.png` | Runner, carpeta 4 con el CSV | 78 iteraciones, 0 fallidas |
| `postman-alerta.png` | Runner, iteración 54 (última de `serie_alerta`) | `media_24h` con 24 lecturas, `alerta: true`, `inca: "mala"` |

Referencia: `newman-coleccion.txt` (63 pedidos, 315 aserciones, 0 fallos) y `newman-runner-csv.txt`
(78 iteraciones, 714 aserciones, 0 fallos).

## Swagger UI

| Archivo | Dónde | Qué tiene que verse |
|---|---|---|
| `swagger-ui.png` | `http://localhost:8080/docs` | La lista de rutas agrupadas por etiqueta y el botón *Authorize* |
| `swagger-prediccion.png` | `/docs`, `POST /api/predicciones` → *Try it out* con el token | El pedido, la respuesta 200 y su duración |

## MongoDB y Redis, con registros escritos por la API

Los comandos completos están en `mongo.txt` y `redis.txt`.

| Archivo | Comando | Qué tiene que verse |
|---|---|---|
| `mongo-conteos.png` | El `countDocuments` de las cinco colecciones | 33 estaciones y las demás con registros |
| `mongo-prediccion.png` | `db.predicciones.find({estacion: "CA-HU-04"}).sort({fecha_hora: -1, ts: -1}).limit(1)` | Un documento con `media_24h`, `alerta: true` e `inca` |
| `mongo-modelo.png` | `db.modelos.find({}, {origen: 1, creado: 1, activo: 1, iteraciones: 1, inercia: 1})` | El sembrado y el entrenado, uno solo con `activo: true` |
| `mongo-entrenamiento.png` | `db.entrenamientos.find({}, {rondas: 0}).sort({inicio: -1})` | Los entrenamientos con sus nodos y rangos |
| `mongo-eventos.png` | `db.eventos_cluster.find().sort({ts: 1}).limit(4)` | Altas y bajas de los nodos |
| `redis-claves.png` | `redis-cli --scan --pattern '*'` | `modelo:activo`, `estacion:*:regimen` y `entrenamiento:*:punto` |
| `redis-estacion.png` | `redis-cli GET estacion:CA-HU-04:regimen` | El estado con `alerta: true` |
| `redis-punto.png` | `redis-cli GET entrenamiento:<id>:punto` | `iter` y los centroides |

## Entrenamiento y carga

| Archivo | Comando | Qué tiene que verse | Referencia |
|---|---|---|---|
| `entrenamiento.png` | El `POST /api/entrenamientos` y el `GET` del detalle con `curl` | 202 con el id; después `terminado`, 44 iteraciones y las primeras rondas | `entrenamiento.txt` |
| `carga.png` | `CARGA_CLAVE=… go run ./cmd/carga -pedidos 20000` (la clave, tapada o en una variable) | La tabla con p50, p95, p99, máximo y pedidos por segundo de los cuatro niveles, y 0 errores | `carga.txt` |
| `metricas.png` | `GET /api/metricas` | `POST /api/predicciones` con sus percentiles y `escrituras_descartadas: 0` | `metricas.txt` |

Al terminar: `docker compose down` (con `-v` si se quieren borrar los datos).
