<!-- Generado por tablas.py desde reports/smoke_lab_*.json. No editar a mano. -->

::: {#tbl-smoke tbl-colwidths="[10,10,12,12,12,16,12,16]"}
| Nodos | Rondas | Barrera p50 (ms) | Barrera p95 (ms) | Barrera máx. (ms) | Latido p50 por nodo (ms) | Workers por nodo | Fecha |
|---:|---:|---:|---:|---:|---:|---:|:---|
| 3 | 200 | 0,399 | 0,804 | 1,63 | 0,378–0,386 | 24 | 2026-11-14 |
| 4 | 200 | 0,515 | 0,855 | 1,15 | 0,490–0,499 | 24 | 2026-11-14 |

La barrera de una ronda medida en el laboratorio: rondas de latido con todos los nodos a la vez
:::

*Nota.* Fuente: `reports/smoke_lab_3.json`, `reports/smoke_lab_4.json`, escritos por `cmd/smoke`. Cada ronda manda un `PING` a todos los nodos a la vez (`protocolo.Latido`) y espera el último `PONG`: es la barrera de una iteración sin el cómputo del nodo. «Latido p50 por nodo» es el menor y el mayor entre los nodos de la corrida. Nodos: los equipos `H45-022`, `H45-023`, `H45-024`, `H45-032` del laboratorio (red 10.11.38.0/24), con el coordinador en la laptop de desarrollo.
