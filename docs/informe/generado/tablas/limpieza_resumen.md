<!-- Generado por tablas.py desde generado/limpieza.json. No editar a mano. -->

::: {.callout-warning title="PLACEHOLDER"}
Cifras de ejemplo: `generado/limpieza.json` todavía es el archivo de muestra. Se reemplaza cuando `cmd/cargar` escriba el real.
:::

::: {#tbl-limpieza-resumen tbl-colwidths="[60,22,18]"}
| Concepto | Filas | Proporción |
|:---|---:|---:|
| Filas largas leídas | 3 600 000 | 100 % |
| Filas largas procesadas | 3 327 400 | 92,4 % |
| Filas largas descartadas | 272 600 | 7,6 % |
| Filas anchas (estación × fecha-hora), regla 7 | 331 200 | 100 % |
| Filas anchas completas para el modelo, regla 8 | 268 400 | 81,0 % |
| Lecturas con PM2,5 > PM10 (marcadas) | 2 150 | 0,65 % |

Conservación de filas en la carga
:::

*Nota.* Fuente: `generado/limpieza.json`, escrito por `cmd/cargar` (commit `PLACEHOLDER`, 2026-11-04). Leídas = procesadas + descartadas, en filas largas. Corrida con 8 workers y bloques de 64 MiB en 38,4 s.
