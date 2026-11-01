<!-- Generado por tablas.py desde generado/benchmark_kmeans.json. No editar a mano. -->

::: {.callout-warning title="PLACEHOLDER"}
Cifras de prueba: `generado/benchmark_kmeans.json` sale de correr los comandos de Go sobre la muestra sintética data/sample/oefa_muestra.csv (estaciones SINT-*, una semana), no sobre el CSV de OEFA. Se reemplaza cuando `cmd/benchmark` corra sobre el CSV completo.
:::

::: {#tbl-bench-kmeans tbl-colwidths="[14,7,5,8,11,12,10,9,15,9]"}
| Experimento | Modo | W | Chunk | Mediana (ms) | Media recortada (ms) | ms/iter. | Speedup | IC 95 % | Eficiencia |
|:---|:---|---:|---:|---:|---:|---:|---:|---:|---:|
| hasta converger | sec. | — | — | 2,45 | 2,26 | 0,0844 | 1 (ref.) | — | — |
| hasta converger | conc. | 1 | 100 | 2,11 | 2,08 | 0,0729 | 1,08 | [0,84; 1,27] | 108 % |
| hasta converger | conc. | 2 | 100 | 1,53 | 1,54 | 0,0528 | 1,47 | [1,12; 1,69] | 73 % |
| hasta converger | conc. | 4 | 100 | 1,87 | 1,83 | 0,0644 | 1,23 | [0,98; 1,54] | 31 % |
| 10 iteraciones | sec. | — | — | 0,703 | 0,826 | 0,0640 | 1 (ref.) | — | — |
| 10 iteraciones | conc. | 1 | 100 | 1,09 | 1,18 | 0,0994 | 0,70 | [0,51; 1,03] | 70 % |
| 10 iteraciones | conc. | 2 | 100 | 0,673 | 0,666 | 0,0612 | 1,24 | [0,92; 1,64] | 62 % |
| 10 iteraciones | conc. | 4 | 100 | 0,630 | 0,683 | 0,0573 | 1,21 | [0,93; 1,56] | 30 % |

Tiempo y speedup del K-Means paralelo frente al secuencial
:::

*Nota.* Fuente: `generado/benchmark_kmeans.json`, escrito por `cmd/benchmark` (commit `51d8f65`, 2026-11-01, máquina A019803). 3 repeticiones medidas tras 1 de calentamiento, orden aleatorizado por bloques; se guarda el orden de cada ronda; se mide solo el clustering; carga y k-means++ quedan fuera. Speedup: media recortada secuencial / media recortada concurrente, IC bootstrap 95 % (200 réplicas); recorte de 10 % por extremo. n = 617 filas, k = 5. 14 CPU lógicas, go1.27.3. Idéntico bit a bit para todo W: hasta converger sí; 10 iteraciones sí.
