# Bases de datos de PC4

Este directorio contiene el seed de MongoDB de D2. `docker-compose.yml` levanta únicamente MongoDB y
Redis, con almacenamiento persistente y puertos publicados solo en `localhost`. La configuración es para
desarrollo local y no habilita autenticación. Debe utilizarse únicamente en un equipo con usuarios y
procesos locales confiables; no debe exponerse fuera de la máquina ni presentarse como una configuración
apta para producción. La especificación de PC4 no exige autenticación para las bases de D2.

## Levantar y verificar

Desde la raíz del repositorio:

```bash
docker compose up -d
docker compose ps
```

Ambos servicios deben aparecer como `healthy`. MongoDB ejecuta `docs/datos/seed.js` automáticamente solo
cuando inicializa un volumen vacío. El script crea las cinco colecciones y los índices de la especificación
y carga las 33 filas versionadas de `reports/estaciones_ficha.csv`. El healthcheck de MongoDB exige tanto
un `ping` correcto como la señal `.pc4-seed-completo`, escrita en el volumen únicamente después de crear
los índices y verificar la carga completa.

Para comprobar la idempotencia, se puede ejecutar el seed dos veces sobre el mismo volumen:

```bash
docker compose exec -T mongo mongosh --quiet aire_distribuido /docker-entrypoint-initdb.d/10-seed.js
docker compose exec -T mongo mongosh --quiet aire_distribuido /docker-entrypoint-initdb.d/10-seed.js
```

En ambas ejecuciones, `estaciones_en_mongo` debe seguir siendo `33`. Los campos `NA`, `-` o vacíos de la
ficha se guardan como `null`; el seed no aplica las propuestas Bronze → Silver pendientes de aprobación.
El seed se ejecuta de forma controlada para desarrollo local, al inicializar el volumen o como operación
manual de mantenimiento, y no debe lanzarse concurrentemente con otro seed. La especificación no exige
versionado de seeds ni bloqueo distribuido, por lo que D2 no los incorpora.

El documento de estación contiene exactamente los campos obligatorios de §7.1: `_id`,
`objeto_fiscalizado`, `tipo_actividad`, `departamento`, `provincia`, `distrito`, `lat` y `lon`. No se
inventan metadatos ni se completan ubicaciones ausentes.

## Catálogo y estaciones retiradas

La especificación no define si `reports/estaciones_ficha.csv` es un snapshot autoritativo o una fuente
histórica acumulativa. Por ello el seed actual hace `upsert` de las estaciones presentes, pero no elimina
documentos cuyo `_id` haya desaparecido de una versión posterior de la ficha. Antes de implementar bajas,
Rody debe confirmar la semántica del catálogo y, si corresponde, cómo conservar su trazabilidad.

## Consultar documentos e índices

```bash
docker compose exec -T mongo mongosh --quiet aire_distribuido --eval \
  'printjson({estaciones: db.estaciones.countDocuments({}), ejemplo: db.estaciones.findOne({_id: "CA-HUAR-01"})})'

docker compose exec -T mongo mongosh --quiet aire_distribuido --eval \
  'for (const c of ["modelos","entrenamientos","predicciones","eventos_cluster","estaciones"]) printjson({coleccion: c, indices: db.getCollection(c).getIndexes()})'

docker compose exec -T redis redis-cli ping
docker compose exec -T redis redis-cli CONFIG GET appendonly
```

Redis debe responder `PONG` y mostrar `appendonly` con valor `yes`.

## Más de un entorno local

Compose ya no fija el nombre del proyecto. Para levantar dos entornos simultáneos se necesitan nombres de
proyecto y puertos de host distintos, por ejemplo:

```bash
docker compose -p pc4-a up -d
MONGO_PORT=27018 REDIS_PORT=6380 docker compose -p pc4-b up -d
```

Los nombres separan redes, contenedores y volúmenes; las variables evitan conflictos en los puertos de
`localhost`.

## Detener

```bash
docker compose down
```

Los volúmenes se conservan. `docker compose down -v` también los elimina y, por tanto, borra los datos
locales; debe usarse solo cuando se quiera reinicializar el entorno.
