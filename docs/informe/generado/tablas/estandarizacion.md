<!-- Generado por tablas.py desde generado/estandarizacion.json. No editar a mano. -->

::: {.callout-warning title="PLACEHOLDER"}
Cifras de ejemplo: `generado/estandarizacion.json` todavía es el archivo de muestra. Se reemplaza cuando `cmd/cargar` escriba el real.
:::

::: {#tbl-estandarizacion}
| Feature | log1p | Media | Desv. |
|:---|:---|---:|---:|
| PM10 (µg/m³) | sí | 3,620 | 0,710 |
| Fracción fina PM2,5/PM10 | no | 0,470 | 0,160 |
| Viento (m/s) | no | 2,350 | 1,420 |
| Humedad relativa (%) | no | 74,800 | 14,900 |

Media y desviación usadas en el z-score de cada feature
:::

*Nota.* Fuente: `generado/estandarizacion.json`, escrito por `cmd/cargar` (commit `PLACEHOLDER`, 2026-11-04). Nivel absoluto, n = 268 400 filas anchas completas. Con log1p, la media y la desviación son de log(1 + x). Calculadas con Welford por chunk y fusión de Chan en orden de chunk.
