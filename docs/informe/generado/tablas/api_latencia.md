<!-- Generado por tablas.py desde reports/api_latencia.json. No editar a mano. -->

::: {#tbl-api-latencia tbl-colwidths="[11,11,11,10,10,10,10,13,14]"}
| Clientes a la vez | Pedidos | Errores | p50 (ms) | p95 (ms) | p99 (ms) | Máx. (ms) | Pedidos por segundo | Pedidos de 100 ms o más |
|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 1 | 20 000 | 0 | 0,153 | 0,204 | 0,274 | 2,92 | 6 061 | 0 |
| 4 | 20 000 | 0 | 0,139 | 0,240 | 0,432 | 1,43 | 24 406 | 0 |
| 16 | 20 000 | 0 | 0,339 | 0,903 | 1,29 | 2,91 | 36 344 | 0 |
| 64 | 20 000 | 0 | 0,997 | 2,37 | 3,14 | 4,84 | 51 967 | 0 |

Latencia de `POST /api/predicciones` medida del lado del cliente, por nivel de concurrencia
:::

*Nota.* Fuente: `reports/api_latencia.json`, escrito por `cmd/carga` (2026-11-21, commit `9216964`). Contra `http://localhost:8080`, con las 78 lecturas de `docs/postman/lecturas_prueba.csv` en ronda y 200 pedidos de calentamiento por nivel, que no se miden. La latencia de un pedido va desde antes de enviarlo hasta tener el cuerpo de la respuesta: incluye la red, el JWT y el JSON. Percentiles por rango más cercano sobre los pedidos que respondieron 200. Durante la corrida la API aceptó 80 800 predicciones (con el calentamiento): MongoDB escribió 80 800 (0 descartadas, 0 fallidas) y Redis, 80 800 (0 descartadas, 0 fallidas), según `GET /api/metricas` antes y después.
