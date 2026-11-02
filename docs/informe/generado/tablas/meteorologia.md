<!-- Generado por tablas.py desde reports/descriptores_regimen.csv. No editar a mano. -->

::: {#tbl-meteorologia tbl-colwidths="[10,11,14,15,15,17,18]"}
| Nivel | Horas | Fracción fina media | Viento (m/s) | Humedad (%) | Temperatura (°C) | Estaciones con meteorología |
|:---|---:|---:|---:|---:|---:|---:|
| bajo | 47 070 | 0,60 | 1,99 (−0,07) | 64,9 (+2,1) | 14,4 (−0,3) | 11 de 17 |
| medio | 15 651 | 0,48 | 2,09 (+0,18) | 62,1 (−5,4) | 16,2 (+0,8) | 11 de 17 |
| alto | 1 515 | 0,43 | 1,68 (−0,03) | 56,8 (−5,1) | 14,6 (+0,5) | 8 de 15 |

Meteorología y fracción fina de cada nivel (no son features: se describen)
:::

*Nota.* Fuente: `reports/descriptores_regimen.csv` (notebooks, sobre las asignaciones de Go y el silver). Entre paréntesis, la anomalía media dentro de la estación: cuánto se aparta la hora del promedio de su propia estación, para que la comparación no sea entre lugares. La fracción fina es la media por hora de PM2,5/PM10. La meteorología solo existe en las estaciones que la miden; las demás no entran en esas columnas. **Ojo: el archivo es de una corrida anterior a la de `kmeans.json`; las cifras se rehacen con la §8 del notebook 02.**
