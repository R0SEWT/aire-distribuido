<!-- Generado por tablas.py desde docs/datos/ejemplos/. No editar a mano. -->

::: {#tbl-claves-redis tbl-colwidths="[25,37,38]"}
| Clave | Campos del valor | Para qué |
|:---|:---|:---|
| `estacion:{id}:regimen` | `fecha_hora`, `regimen`, `excede_eca`, `media_24h` {pm2_5, pm10, lecturas}, `alerta`, `inca`, `ts` | El estado de la estación en su lectura más reciente; lo lee `GET /api/estaciones/{id}/regimen` |
| `modelo:activo` | El documento de `modelos`, completo | El modelo activo, para arrancar con él sin esperar a MongoDB |
| `entrenamiento:{id}:punto` | `iter`, `centroides` | Punto de control: la iteración y los centroides de la última ronda completa |

Entidades de Redis: las tres familias de claves, cada una con un valor JSON
:::

*Nota.* Fuente: los valores de ejemplo de `docs/datos/ejemplos/redis-*.json`; las claves las arma `almacen/redis/redis.go`. Ninguna clave vence: son pocas (una por estación y una por entrenamiento) y el régimen de una estación es su única copia.
