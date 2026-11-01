<!-- Generado por tablas.py desde generado/regimenes.json. No editar a mano. -->

::: {.callout-warning title="PLACEHOLDER"}
Cifras de prueba: `generado/regimenes.json` sale de correr los comandos de Go sobre la muestra sintética data/sample/oefa_muestra.csv (estaciones SINT-*, una semana), no sobre el CSV de OEFA. Se reemplaza cuando `cmd/entrenar` corra sobre el CSV completo.
:::

::: {#tbl-regimenes-hora tbl-colwidths="[12,18,18,18,18,16]"}
| Régimen | Madrugada (0–5 h) | Mañana (6–11 h) | Tarde (12–17 h) | Noche (18–23 h) | Hora pico |
|---:|---:|---:|---:|---:|---:|
| 0 | 36 % | 15 % | 2 % | 48 % | 22:00 |
| 1 | 45 % | 34 % | 4 % | 18 % | 05:00 |
| 2 | 7 % | 19 % | 46 % | 28 % | 17:00 |
| 3 | 0 % | 15 % | 62 % | 23 % | 14:00 |
| 4 | 43 % | 44 % | 1 % | 13 % | 04:00 |

Distribución de cada régimen por franja horaria
:::

*Nota.* Fuente: `generado/regimenes.json`, escrito por `cmd/entrenar` (commit `abf3e7f`, 2026-11-01, máquina A019803). Proporción de las filas de cada régimen en cada franja (hora de Lima, UTC−5). La distribución hora a hora está en la figura del heatmap régimen × hora.
