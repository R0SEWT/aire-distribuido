# Ejemplos de datos de PC4

Este directorio contiene un documento de ejemplo para cada una de las cinco colecciones de MongoDB y
un valor de ejemplo para cada una de las tres familias de claves Redis definidas en la
[especificación vigente de PC4, §7](https://github.com/R0SEWT/aire-distribuido/blob/feature/spec-pc4/docs/specs/2026-11-14-pc4-design.md#7-datos-mongodb-y-redis),
preparada en la rama `feature/spec-pc4` del PR #51. Los campos de la predicción y del régimen por estación
siguen el contrato de la API, `docs/api/openapi.yaml` (PR #55), que es posterior a esa sección.

Los archivos muestran el contrato de datos. No todos constituyen evidencia de que la infraestructura de
PC4 ya haya sido ejecutada. La clasificación usada a continuación es:

- **Real**: copiado de un artefacto producido y auditado en PC3.
- **Derivado**: calculado o relacionado de forma determinista a partir de artefactos reales.
- **Sintético**: elegido para mostrar el contrato que deberá producir PC4; no es una medición ni una
  ejecución observada.

## Archivos y procedencia

| Archivo | Destino | Clasificación | Procedencia y límites |
|---|---|---|---|
| `modelos.json` | MongoDB `modelos` | Real, salvo el estado operativo | Receta, hash, estandarización, centroides, regímenes, inercia e iteraciones de `reports/kmeans.json`, `reports/estandarizacion.json` y `reports/regimenes.json`. `_id`, `origen = sembrado` y el arranque con este modelo están definidos en la spec. `activo = true` representa el estado inicial previsto para PC4. |
| `entrenamientos.json` | MongoDB `entrenamientos` | Sintético con resultados PC3 de referencia | Identificador, timestamps, nodos, direcciones reservadas, tiempos y rangos representan una futura corrida PC4 y no fueron observados. La semilla, el tamaño del Gold, la iteración final, la inercia y el paro reproducen el resultado real de PC3 para que el ejemplo sea internamente coherente. |
| `predicciones.json` | MongoDB `predicciones` | Medición real, resultado derivado y persistencia sintética | La lectura de `CA-AYAS-01` y `fecha_hora` se verificaron en el Silver local; su vector estandarizado y el régimen 0, en el Gold y las asignaciones locales. Esos CSV se regeneran con el procedimiento indicado abajo y no están versionados. `_id` y `ts` representan una persistencia de ejemplo. `media_24h`, `alerta` e `inca` siguen la regla de «Alerta, ECA e INCA». `ms` es un valor sintético: PC3 no midió una petición HTTP. |
| `eventos_cluster.json` | MongoDB `eventos_cluster` | Sintético | Ejemplo de evento `alta` del mismo nodo sintético usado en `entrenamientos.json`. No es evidencia de un nodo real. La red `192.0.2.0/24` está reservada para documentación. |
| `estaciones.json` | MongoDB `estaciones` | Real | Fila `CA-AYAS-01` de `reports/estaciones_ficha.csv`. Las coordenadas son números JSON. |
| `redis-modelo-activo.json` | Redis `modelo:activo` | Real, salvo el estado operativo | Copia completa del modelo de `modelos.json` para representar el arranque en caliente. |
| `redis-estacion-regimen.json` | Redis `estacion:CA-AYAS-01:regimen` | Derivado con actualización sintética | Los mismos campos que la predicción real, en el esquema `RegimenEstacion` del contrato. `fecha_hora` identifica la medición y `ts` la actualización sintética de caché. |
| `redis-entrenamiento-punto.json` | Redis `entrenamiento:sintetico-e-20261116-2200:punto` | Sintético con centroides PC3 de referencia | Representa el último punto completo de la corrida sintética. Los centroides son los centroides estandarizados reales del modelo PC3, usados como valor representativo de la iteración 44. No prueba que PC4 haya escrito el checkpoint. |

## Timestamps

- `fecha_hora = 2021-12-10T01:00:00-05:00` es el fin del intervalo de medición publicado por OEFA.
- `creado = 2026-11-01T23:59:16.218568815-05:00` es el momento registrado por el entrenamiento de PC3.
- `inicio`, `fin` y los `ts` del 16 de noviembre son timestamps sintéticos de ejecución, persistencia o
  actualización de PC4. No sustituyen ni alteran la fecha de la medición.

La clave `estacion:{id}:regimen` conserva ambos instantes. El contrato lo fija así: el valor guarda la
`fecha_hora` de su medición y una lectura más antigua que la almacenada no lo reemplaza. La tarea que
implemente la caché tiene que hacer esa comparación al escribir.

## Disponibilidad y reproducción de las fuentes

Los reportes agregados de PC3 usados por estos ejemplos —`reports/kmeans.json`,
`reports/estandarizacion.json`, `reports/regimenes.json`, `reports/limpieza.json` y
`reports/estaciones_ficha.csv`— sí están versionados en Git. También están versionados el código del
pipeline, las configuraciones y las muestras pequeñas de `data/sample/`.

El Bronze completo, `data/silver/oefa_ancho.csv`, `data/gold/oefa_features.csv` y
`data/gold/asignaciones.csv` están ignorados por Git debido a su tamaño. Durante la preparación de D1 se
consultó una copia local bajo `data/bronze/revision-dayana/data/`; esa ruta no debe tratarse como una
fuente disponible después de clonar el repositorio.

Para reproducir los CSV, se descarga el CSV de OEFA indicado en `data/README.md`, se guarda localmente
como `data/bronze/oefa.csv` y se ejecuta desde la raíz:

```bash
go run ./cmd/cargar -entrada data/bronze/oefa.csv
go run ./cmd/entrenar
```

El primer comando genera `data/silver/oefa_ancho.csv`, `data/gold/oefa_features.csv` y los reportes de
limpieza y estandarización. El segundo genera `data/gold/asignaciones.csv` y los reportes del modelo. Las
huellas publicadas en los reportes permiten verificar que se usaron el Bronze, las configuraciones y el
Gold auditados; no se deben añadir estos datasets grandes al repositorio.

## Alerta, ECA e INCA

El contrato de la API separa tres cosas:

- `excede_eca` describe el **régimen** asignado: si su media supera el ECA. La observación elegida
  pertenece al régimen 0, que `reports/regimenes.json` clasifica como `no_excede`: por eso es `false`.
- `media_24h` es la media de las lecturas de la **estación** en las 24 horas que terminan en la
  `fecha_hora` de la lectura. `alerta` es que esa media supere el ECA de 24 horas (PM2.5 > 50 o
  PM10 > 100 µg/m³), sea cual sea el régimen. Con menos de 18 lecturas en la ventana, las medias son
  `null` y `alerta` es `false`.
- `inca` es la banda del INCA de esa misma media (`buena`, `moderada`, `mala`, `umbral_de_cuidado`), o
  `null` con menos de 18 lecturas.

La lectura del ejemplo es la primera de `CA-AYAS-01` en el Silver, así que su ventana tiene una sola
lectura: por eso `media_24h` trae `lecturas: 1` con medias `null`, `alerta` es `false` e `inca` es
`null`. Aquí `false` significa «todavía no se puede evaluar», no «cumple el ECA».

## Valores ausentes y limitaciones

- `predicciones.ms` es sintético (0.04): la medición existe, pero todavía no hay una petición HTTP real
  cuya latencia pueda atribuirse a ese documento. El contrato lo define como un número, no admite `null`.
- `entrenamientos.error` es `null`: en el ejemplo sintético el estado es `terminado`, por lo que no hubo
  error. Aquí `null` representa una ausencia legítima del contrato.
- Los tiempos de ronda y por nodo de `entrenamientos.json` solo ilustran tipos y estructura. Deben
  reemplazarse por resultados de laboratorio o de Compose antes de usarlos como evidencia.
- Los JSON usan cadenas ISO 8601 para poder validarse como JSON portable. Al sembrar MongoDB, los campos
  temporales deben convertirse a su tipo fecha si la implementación adopta BSON Date.
