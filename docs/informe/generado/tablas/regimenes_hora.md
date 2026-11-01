<!-- Generado por tablas.py desde generado/regimenes.json. No editar a mano. -->

::: {.callout-warning title="PLACEHOLDER"}
Cifras de ejemplo: `generado/regimenes.json` todavía es el archivo de muestra. Se reemplaza cuando `cmd/entrenar` escriba el real.
:::

::: {#tbl-regimenes-hora}
| Régimen | Madrugada (0–5 h) | Mañana (6–11 h) | Tarde (12–17 h) | Noche (18–23 h) | Hora pico |
|:---|---:|---:|---:|---:|---:|
| 0. Fondo con viento | 10 % | 25 % | 45 % | 20 % | 14:00 |
| 1. Humedad nocturna | 45 % | 25 % | 10 % | 20 % | 03:00 |
| 2. Polvo diurno | 7 % | 38 % | 46 % | 9 % | 12:00 |
| 3. Combustión en calma | 21 % | 6 % | 15 % | 57 % | 21:00 |
| 4. Episodio de polvo | 4 % | 27 % | 62 % | 7 % | 13:00 |

Distribución de cada régimen por franja horaria
:::

*Nota.* Fuente: `generado/regimenes.json`, escrito por `cmd/entrenar` (commit `PLACEHOLDER`, 2026-11-04). Proporción de las filas de cada régimen en cada franja (hora local, UTC−5). La distribución hora a hora está en la figura del heatmap régimen × hora.
