<!-- Generado por tablas.py desde generado/kmeans.json. No editar a mano. -->

::: {.callout-warning title="PLACEHOLDER"}
Cifras de prueba: `generado/kmeans.json` sale de correr los comandos de Go sobre la muestra sintética data/sample/oefa_muestra.csv (estaciones SINT-*, una semana), no sobre el CSV de OEFA. Se reemplaza cuando `cmd/entrenar` corra sobre el CSV completo.
:::

::: {#tbl-semillas tbl-colwidths="[11,15,13,14,11,14,12,10]"}
| Semilla | Inercia | Iteraciones | Paro | Workers | k-means++ (ms) | Lloyd (ms) | Elegida |
|---:|---:|---:|:---|---:|---:|---:|:---:|
| 42 | 1 257,3 | 28 | tolerancia | 4 | 0,0680 | 2,44 |  |
| 43 | 1 381,7 | 30 | tolerancia | 5 | 0,157 | 3,15 |  |
| 44 | 1 254,1 | 18 | tolerancia | 5 | 0,114 | 2,76 | **sí** |
| 45 | 1 287,3 | 12 | tolerancia | 4 | 0,134 | 1,21 |  |
| 46 | 1 286,8 | 15 | tolerancia | 5 | 0,140 | 1,50 |  |
| 47 | 1 396,1 | 12 | tolerancia | 5 | 0,0927 | 0,785 |  |
| 48 | 1 369,7 | 24 | tolerancia | 4 | 0,0639 | 1,04 |  |
| 49 | 1 257,4 | 29 | tolerancia | 5 | 0,0797 | 1,78 |  |
| 50 | 1 261,8 | 26 | tolerancia | 5 | 0,136 | 2,26 |  |
| 51 | 1 284,3 | 13 | tolerancia | 4 | 0,0651 | 0,624 |  |

Arranques del K-Means: una corrida completa por semilla
:::

*Nota.* Fuente: `generado/kmeans.json`, escrito por `cmd/entrenar` (commit `abf3e7f`, 2026-11-01, máquina A019803). 10 semillas desde 42; criterio: menor inercia final; si empatan bit a bit, la semilla menor. Paralelismo por semillas: 3 corridas a la vez, con 5, 5, 4 workers cada una (14 en total). Los 10 arranques tardaron 6,96 ms de pared frente a 18,6 ms de la suma de sus corridas (2,7 veces).
