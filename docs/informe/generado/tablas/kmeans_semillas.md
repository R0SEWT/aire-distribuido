<!-- Generado por tablas.py desde generado/kmeans.json. No editar a mano. -->

::: {#tbl-semillas tbl-colwidths="[11,15,13,14,11,14,12,10]"}
| Semilla | Inercia | Iteraciones | Paro | Workers | k-means++ (ms) | Lloyd (ms) | Elegida |
|---:|---:|---:|:---|---:|---:|---:|:---:|
| 42 | 50 950,4 | 44 | tolerancia | 4 | 3,83 | 48,7 | **sí** |
| 43 | 50 950,4 | 46 | tolerancia | 5 | 5,84 | 43,5 |  |
| 44 | 50 950,4 | 21 | tolerancia | 5 | 6,08 | 20,2 |  |
| 45 | 50 950,4 | 30 | tolerancia | 5 | 2,65 | 29,4 |  |
| 46 | 50 950,4 | 26 | tolerancia | 5 | 2,73 | 22,2 |  |
| 47 | 50 950,4 | 50 | tolerancia | 4 | 2,42 | 47,6 |  |
| 48 | 50 950,4 | 28 | tolerancia | 5 | 2,98 | 25,0 |  |
| 49 | 50 950,4 | 19 | tolerancia | 5 | 2,93 | 18,4 |  |
| 50 | 50 950,4 | 32 | tolerancia | 5 | 3,09 | 29,0 |  |
| 51 | 50 950,4 | 31 | tolerancia | 5 | 3,42 | 22,7 |  |

Arranques del K-Means: una corrida completa por semilla
:::

*Nota.* Fuente: `generado/kmeans.json`, escrito por `cmd/entrenar` (commit `6fb3b00`, 2026-11-01, laptop de desarrollo). 10 semillas desde 42; criterio: menor inercia final; si empatan bit a bit, la semilla menor. Paralelismo por semillas: 3 corridas a la vez, con 5, 5, 4 workers cada una (14 en total). Los 10 arranques tardaron 122,5 ms de pared frente a 342,7 ms de la suma de sus corridas (2,8 corridas a la vez en promedio). Los tiempos de k-means++ y de Lloyd de cada arranque se miden con otras semillas corriendo a la vez y con los workers de su cupo: sirven para comparar semillas entre sí, no para speedup. El speedup del Lloyd concurrente lo mide cmd/benchmark. El de k-means++ es 0 si los centroides se leyeron del archivo. Centroides iniciales de k-means++: `reports/centroides_iniciales.json` (generados, sha256 `c5db5fa5ec9c`), para que otra corrida parta de los mismos.
