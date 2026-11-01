<!-- Generado por tablas.py desde generado/estandarizacion.json. No editar a mano. -->

::: {.callout-warning title="PLACEHOLDER"}
Cifras de prueba: `generado/estandarizacion.json` sale de correr los comandos de Go sobre la muestra sintética data/sample/oefa_muestra.csv (estaciones SINT-*, una semana), no sobre el CSV de OEFA. Se reemplaza cuando `cmd/cargar` corra sobre el CSV completo.
:::

::: {#tbl-estandarizacion tbl-colwidths="[45,15,20,20]"}
| Feature | log1p | Media | Desv. |
|:---|:---|---:|---:|
| PM10 (µg/m³) | sí | 3,9104 | 0,4624 |
| Fracción fina PM2,5/PM10 | no | 0,5249 | 0,1062 |
| Velocidad del viento (m/s) | no | 2,5065 | 1,1518 |
| Humedad relativa (%) | no | 69,9890 | 17,8738 |

Media y desviación usadas en el z-score de cada feature
:::

*Nota.* Fuente: `generado/estandarizacion.json`, escrito por `cmd/cargar` (commit `e1cef10`, 2026-11-01, máquina A019803). Nivel absoluto, n = 617 filas anchas completas, ddof = 0, chunk de 10 000 filas. Con log1p, la media y la desviación son de log(1 + x).
