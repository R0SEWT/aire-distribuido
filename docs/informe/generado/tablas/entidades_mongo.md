<!-- Generado por tablas.py desde docs/datos/ejemplos/. No editar a mano. -->

::: {#tbl-entidades-mongo tbl-colwidths="[14,20,36,15,15]"}
| Colección | Qué guarda | Campos (además de `_id`) | Índices | Quién la escribe |
|:---|:---|:---|:---|:---|
| `estaciones` | El catálogo de estaciones: el código OEFA y su ficha | `objeto_fiscalizado`, `tipo_actividad`, `departamento`, `provincia`, `distrito`, `lat`, `lon` | — | El seed, desde la ficha |
| `predicciones` | Una por lectura predicha: la lectura, su régimen, la media de 24 h, la alerta y el INCA | `ts`, `estacion`, `fecha_hora`, `pm2_5`, `pm10`, `regimen`, `excede_eca`, `media_24h` {pm2_5, pm10, lecturas}, `alerta`, `inca`, `modelo_id`, `ms` | `estacion_ts` (estacion, ts) | La API, por lotes |
| `modelos` | Cada modelo, con lo necesario para volver a predecir con él; a lo sumo uno activo | `creado`, `origen`, `receta` {features, log1p, nivel, k, semilla, semillas, tolerancia, max_iter, workers, chunk}, `gold_sha256`, `estandarizacion` {features, log1p, nivel, n, media, desv}, `centroides_std`, `centroides`, `regimenes` [{regimen, excede_eca, pm2_5_medio, pm10_medio, tamano}], `inercia`, `iteraciones`, `activo` | `activo` (activo), `creado` (creado) | La API al arrancar y al activar un modelo |
| `entrenamientos` | Cada corrida distribuida: parámetros, nodos con su rango, rondas y resultado | `inicio`, `fin`, `estado`, `parametros` {nodos, replicas, semilla}, `nodos` [{nombre, direccion, workers, rango}], `rondas` [{iter, inercia, ms_ronda, ms_fusion, ms_por_nodo}], `iteraciones`, `paro`, `dif_secuencial`, `ms_total`, `modelo_id`, `aviso`, `error` | `inicio` (inicio) | El coordinador de la API |
| `eventos_cluster` | Altas, bajas y errores de los nodos | `ts`, `tipo`, `nodo`, `detalle` | `ts` (ts) | El coordinador de la API |

Entidades de MongoDB: las cinco colecciones de la base `aire_distribuido`
:::

*Nota.* Fuente: los documentos de ejemplo de `docs/datos/ejemplos/` (los campos) y `docs/datos/seed.js` (los índices, con su nombre y, entre paréntesis, sus campos). Entre llaves, los campos de un subdocumento; con corchetes, los de cada elemento de un arreglo. Las pruebas `TestLosDocumentosTienenLosCamposDeSusEjemplos` (`almacen` y `almacen/mongo`) fallan si un documento que escribe la API deja de tener exactamente los campos de su ejemplo.
