<!-- Generado por tablas.py desde generado/benchmark_kmeans.json. No editar a mano. -->

::: {.callout-warning title="PLACEHOLDER"}
Cifras de ejemplo: `generado/benchmark_kmeans.json` todavía es el archivo de muestra. Se reemplaza cuando `cmd/benchmark -parte kmeans` escriba el real.
:::

::: {#tbl-bench-kmeans tbl-colwidths="[14,9,11,11,11,10,22,12]"}
| Modo | Workers | Chunk (filas) | Mediana (s) | ms/iteración | Speedup | IC 95 % | Eficiencia |
|:---|---:|---:|---:|---:|---:|---:|---:|
| sec. | 1 | — | 2,840 | 284,0 | 1,00 | [0,98; 1,02] | 100 % |
| conc. | 1 | 1 000 | 3,020 | 302,0 | 0,94 | [0,92; 0,96] | 94 % |
| conc. | 2 | 1 000 | 1,580 | 158,0 | 1,80 | [1,77; 1,83] | 90 % |
| conc. | 4 | 1 000 | 0,860 | 86,0 | 3,30 | [3,24; 3,36] | 83 % |
| conc. | 8 | 1 000 | 0,520 | 52,0 | 5,46 | [5,36; 5,55] | 68 % |
| conc. | 11 | 1 000 | 0,470 | 47,0 | 6,04 | [5,93; 6,15] | 55 % |
| conc. | 1 | 10 000 | 2,910 | 291,0 | 0,98 | [0,96; 0,99] | 98 % |
| conc. | 2 | 10 000 | 1,490 | 149,0 | 1,91 | [1,87; 1,94] | 95 % |
| conc. | 4 | 10 000 | 0,790 | 79,0 | 3,59 | [3,53; 3,66] | 90 % |
| conc. | 8 | 10 000 | 0,460 | 46,0 | 6,17 | [6,06; 6,28] | 77 % |
| conc. | 11 | 10 000 | 0,410 | 41,0 | 6,93 | [6,80; 7,04] | 63 % |

Tiempo y speedup del K-Means paralelo por número de workers y tamaño de chunk
:::

*Nota.* Fuente: `generado/benchmark_kmeans.json`, escrito por `cmd/benchmark -parte kmeans` (commit `PLACEHOLDER`, 2026-11-04). Mediana de 5 repeticiones tras 1 de calentamiento, en PLACEHOLDER (11 CPU lógicas, go1.26.x). Speedup contra secuencial; intervalo bootstrap del cociente de medianas con 2 000 réplicas. Eficiencia = speedup / workers. Modo: sec. = secuencial, conc. = concurrente.
