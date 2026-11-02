<!-- Generado por tablas.py desde reports/descriptores_regimen.csv. No editar a mano. -->

::: {#tbl-meteorologia tbl-colwidths="[10,11,14,15,15,17,18]"}
| Nivel | Horas | Fracción fina media | Viento (m/s) | Humedad (%) | Temperatura (°C) | Estaciones con meteorología |
|:---|---:|---:|---:|---:|---:|---:|
| bajo | 48 161 | 0,60 | 2,00 (−0,07) | 65,4 (+2,1) | 14,4 (−0,3) | 10 de 15 |
| medio | 16 176 | 0,49 | 2,09 (+0,18) | 62,6 (−5,5) | 16,3 (+0,8) | 11 de 17 |
| alto | 1 536 | 0,43 | 1,68 (−0,03) | 56,9 (−5,3) | 14,6 (+0,4) | 9 de 15 |

Meteorología y fracción fina de cada nivel (no son features: se describen)
:::

*Nota.* Fuente: `reports/descriptores_regimen.csv` (notebooks, sobre las asignaciones de Go y el silver). Entre paréntesis, la anomalía media dentro de la estación: cuánto se aparta la hora del promedio de su propia estación, para que la comparación no sea entre lugares. La fracción fina es la media por hora de PM2,5/PM10. La meteorología solo existe en las estaciones que la miden; las demás no entran en esas columnas. «Estaciones con meteorología»: cuántas estaciones del nivel tienen lecturas de cada variable en sus horas; un número si las tres coinciden y, si no, viento · humedad · temperatura.
