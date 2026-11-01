<!-- Generado por tablas.py desde generado/kmeans.json. No editar a mano. -->

::: {#tbl-semillas tbl-colwidths="[11,15,13,14,11,14,12,10]"}
| Semilla | Inercia | Iteraciones | Paro | Workers | k-means++ (ms) | Lloyd (ms) | Elegida |
|---:|---:|---:|:---|---:|---:|---:|:---:|
| 42 | 59 232,9 | 57 | tolerancia | 4 | 2,40 | 82,0 |  |
| 43 | 59 232,9 | 43 | tolerancia | 5 | 3,62 | 60,6 |  |
| 44 | 59 249,5 | 22 | tolerancia | 5 | 4,29 | 34,4 |  |
| 45 | 59 249,5 | 21 | tolerancia | 5 | 3,98 | 29,4 |  |
| 46 | 59 232,9 | 55 | tolerancia | 5 | 3,84 | 68,2 |  |
| 47 | 59 232,8 | 21 | tolerancia | 5 | 2,29 | 29,1 | **sí** |
| 48 | 59 232,9 | 48 | tolerancia | 4 | 3,90 | 68,2 |  |
| 49 | 59 232,9 | 60 | tolerancia | 5 | 4,04 | 80,1 |  |
| 50 | 59 232,9 | 37 | tolerancia | 5 | 2,31 | 50,8 |  |
| 51 | 59 249,5 | 28 | tolerancia | 4 | 3,65 | 36,5 |  |

Arranques del K-Means: una corrida completa por semilla
:::

*Nota.* Fuente: `generado/kmeans.json`, escrito por `cmd/entrenar` (commit `53858a0`, 2026-11-01, máquina A019803). 10 semillas desde 42; criterio: menor inercia final; si empatan bit a bit, la semilla menor. Paralelismo por semillas: 3 corridas a la vez, con 5, 5, 4 workers cada una (14 en total). Los 10 arranques tardaron 197,4 ms de pared frente a 573,8 ms de la suma de sus corridas (2,9 corridas a la vez en promedio). ms_kmeanspp y ms_lloyd de cada arranque se miden con otras semillas corriendo a la vez y con los workers de su cupo: sirven para comparar semillas entre sí, no para speedup. El speedup del Lloyd concurrente lo mide cmd/benchmark. ms_kmeanspp es 0 si los centroides se leyeron del archivo. Centroides iniciales de k-means++: `reports/centroides_iniciales.json` (generados, sha256 `3e0b1a3986ef`), para que otra corrida parta de los mismos.
