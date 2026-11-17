# Bases de datos de PC4

Este directorio contiene el seed de MongoDB de D2. `docker-compose.yml` levanta únicamente MongoDB y
Redis, con almacenamiento persistente y puertos publicados solo en `localhost`. La configuración es para
desarrollo local y no habilita autenticación; no debe exponerse fuera de la máquina.

## Levantar y verificar

Desde la raíz del repositorio:

```bash
docker compose up -d
docker compose ps
```

Ambos servicios deben aparecer como `healthy`. MongoDB ejecuta `docs/datos/seed.js` automáticamente solo
cuando inicializa un volumen vacío. El script crea las cinco colecciones y los índices de la especificación
y carga las 33 filas versionadas de `reports/estaciones_ficha.csv`.

Para comprobar la idempotencia, se puede ejecutar el seed dos veces sobre el mismo volumen:

```bash
docker compose exec -T mongo mongosh --quiet aire_distribuido /docker-entrypoint-initdb.d/10-seed.js
docker compose exec -T mongo mongosh --quiet aire_distribuido /docker-entrypoint-initdb.d/10-seed.js
```

En ambas ejecuciones, `estaciones_en_mongo` debe seguir siendo `33`. Los campos `NA`, `-` o vacíos de la
ficha se guardan como `null`; el seed no aplica las propuestas Bronze → Silver pendientes de aprobación.

## Consultar documentos e índices

```bash
docker compose exec -T mongo mongosh --quiet aire_distribuido --eval \
  'printjson({estaciones: db.estaciones.countDocuments({}), ejemplo: db.estaciones.findOne({_id: "CA-HUAR-01"})})'

docker compose exec -T mongo mongosh --quiet aire_distribuido --eval \
  'for (const c of ["modelos","entrenamientos","predicciones","eventos_cluster","estaciones"]) printjson({coleccion: c, indices: db.getCollection(c).getIndexes()})'

docker compose exec -T redis redis-cli ping
```

## Detener

```bash
docker compose down
```

Los volúmenes se conservan. `docker compose down -v` también los elimina y, por tanto, borra los datos
locales; debe usarse solo cuando se quiera reinicializar el entorno.
