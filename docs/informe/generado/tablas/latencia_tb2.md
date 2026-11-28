<!-- Generado por tablas.py desde capturas/tb2/pruebas/api_latencia_*.json. No editar a mano. -->

::: {#tbl-latencia-tb2}
| Clientes a la vez | p50 directo (ms) | p99 directo (ms) | p50 por Caddy (ms) | p99 por Caddy (ms) | Máximo por Caddy (ms) | Errores |
|---:|---:|---:|---:|---:|---:|---:|
| 1 | 0,161 | 0,325 | 0,353 | 0,907 | 3,75 | 0 |
| 4 | 0,144 | 0,485 | 0,376 | 1,14 | 3,29 | 0 |
| 16 | 0,410 | 2,01 | 0,909 | 2,50 | 3,89 | 0 |
| 64 | 1,19 | 4,32 | 3,39 | 12,9 | 20,1 | 0 |

La latencia de la predicción en el TB2: directo a la API y por el Caddy de la interfaz, que es el camino del navegador
:::

*Nota.* Fuente: `docs/informe/capturas/tb2/pruebas/api_latencia_directa.json` y `api_latencia_caddy.json`, escritos por `cmd/carga` el 2026-11-27 contra el Compose local con dos nodos. 5 000 predicciones medidas por nivel, con las lecturas de prueba de Postman; cada pedido se mide desde antes de enviarlo hasta leer la respuesta completa. «Por Caddy» es el camino del navegador: el pedido entra por el servicio web, que lo pasa a la API.
