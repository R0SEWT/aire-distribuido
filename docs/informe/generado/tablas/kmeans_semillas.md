<!-- Generado por tablas.py desde generado/kmeans.json. No editar a mano. -->

::: {#tbl-semillas tbl-colwidths="[11,15,13,14,11,14,12,10]"}
| Semilla | Inercia | Iteraciones | Paro | Workers | k-means++ (ms) | Lloyd (ms) | Elegida |
|---:|---:|---:|:---|---:|---:|---:|:---:|
| 42 | 49 658,4 | 24 | tolerancia | 4 | 1,46 | 29,2 | **sí** |
| 43 | 49 658,4 | 38 | tolerancia | 5 | 3,17 | 34,9 |  |
| 44 | 49 658,4 | 33 | tolerancia | 5 | 4,17 | 32,5 |  |
| 45 | 49 658,4 | 43 | tolerancia | 4 | 2,42 | 51,7 |  |
| 46 | 49 658,4 | 33 | tolerancia | 5 | 2,69 | 28,8 |  |
| 47 | 49 658,4 | 34 | tolerancia | 5 | 1,45 | 31,1 |  |
| 48 | 49 658,4 | 42 | tolerancia | 5 | 2,79 | 34,4 |  |
| 49 | 49 658,4 | 29 | tolerancia | 5 | 3,23 | 26,0 |  |
| 50 | 49 658,4 | 40 | tolerancia | 4 | 5,53 | 34,1 |  |
| 51 | 49 658,4 | 39 | tolerancia | 5 | 3,72 | 25,1 |  |

Arranques del K-Means: una corrida completa por semilla
:::

*Nota.* Fuente: `generado/kmeans.json`, escrito por `cmd/entrenar` (commit `11cef5a`, 2026-11-01, máquina A019803). 10 semillas desde 42; criterio: menor inercia final; si empatan bit a bit, la semilla menor. Paralelismo por semillas: 3 corridas a la vez, con 5, 5, 4 workers cada una (14 en total). Los 10 arranques tardaron 129,2 ms de pared frente a 358,2 ms de la suma de sus corridas (2,8 corridas a la vez en promedio). ms_kmeanspp y ms_lloyd de cada arranque se miden con otras semillas corriendo a la vez y con los workers de su cupo: sirven para comparar semillas entre sí, no para speedup. El speedup del Lloyd concurrente lo mide cmd/benchmark. ms_kmeanspp es 0 si los centroides se leyeron del archivo. Centroides iniciales de k-means++: `reports/centroides_iniciales.json` (generados, sha256 `69338b94cc78`), para que otra corrida parta de los mismos.
