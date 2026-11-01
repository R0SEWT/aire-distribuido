<!-- Generado por tablas.py desde generado/calibracion_resumen.csv. No editar a mano. -->

::: {#tbl-calibracion tbl-colwidths="[34,22,22,22]"}
| Criterio (umbral) | Final (elegida) | Final con log1p | Provisional |
|:---|---:|---:|---:|
| Variante | `F1-M0-T1-N1` | `F1-M0-T2-N1` | `F2-M1-T2-N1` |
| k | 3 | 3 | 5 |
| Estaciones · filas | 17 · 64 236 | 17 · 64 236 | 11 · 41 991 |
| ARI entre semillas (> 0,8) | 0,987 | 0,954 | 0,633 |
| ARI por bloques de días (> 0,7) | 0,938 | 0,956 | 0,540 |
| Pureza del régimen «excede» (> 0,6) | 0,341 | 0,000 | 0,000 |
| Recall de las horas sobre el ECA (> 0,5) | 0,444 | 0,000 | 0,000 |
| NMI con la estación · línea base | 0,077 · 0,087 | 0,121 · 0,087 | 0,215 · 0,096 |
| NMI con la hora | 0,014 | 0,016 | 0,117 |
| Cluster más chico | 2,3 % | 17,8 % | 14,9 % |
| Silueta | 0,544 | 0,441 | 0,268 |

Calibración: la receta final frente a las alternativas
:::

*Nota.* Fuente: `reports/calibracion_resumen.csv` (notebook 02: 24 variantes × k de 2 a 10 × 5 semillas, 1 080 corridas). Umbrales entre paréntesis, los de la spec de calibración. La línea base de NMI es la de partir solo por el nivel de PM10: un régimen por debajo de ella no aprendió geografía. Pureza y recall usan la etiqueta del promedio móvil de 24 h sobre el ECA.
