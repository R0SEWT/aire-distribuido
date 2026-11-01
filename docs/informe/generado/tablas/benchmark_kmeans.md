<!-- Generado por tablas.py desde generado/benchmark_kmeans.json. No editar a mano. -->

::: {#tbl-bench-kmeans tbl-colwidths="[14,7,5,8,11,12,10,9,15,9]"}
| Experimento | Modo | W | Chunk | Mediana (ms) | Media recortada (ms) | ms/iter. | Speedup | IC 95 % | Eficiencia |
|:---|:---|---:|---:|---:|---:|---:|---:|---:|---:|
| hasta converger | sec. | — | — | 148,0 | 150,3 | 2,55 | 1 (ref.) | — | — |
| hasta converger | conc. | 1 | 1 000 | 158,6 | 157,2 | 2,73 | 0,96 | [0,92; 1,00] | 96 % |
| hasta converger | conc. | 2 | 1 000 | 82,9 | 83,3 | 1,43 | 1,80 | [1,71; 1,91] | 90 % |
| hasta converger | conc. | 4 | 1 000 | 57,4 | 57,6 | 0,989 | 2,61 | [2,50; 2,73] | 65 % |
| hasta converger | conc. | 8 | 1 000 | 44,7 | 45,3 | 0,770 | 3,32 | [3,20; 3,45] | 41 % |
| hasta converger | conc. | 12 | 1 000 | 41,5 | 42,8 | 0,716 | 3,51 | [3,17; 3,90] | 29 % |
| hasta converger | conc. | 1 | 10 000 | 149,4 | 151,3 | 2,58 | 0,99 | [0,96; 1,03] | 99 % |
| hasta converger | conc. | 2 | 10 000 | 89,0 | 90,9 | 1,53 | 1,65 | [1,47; 1,82] | 83 % |
| hasta converger | conc. | 4 | 10 000 | 67,4 | 66,1 | 1,16 | 2,27 | [2,14; 2,43] | 57 % |
| hasta converger | conc. | 8 | 10 000 | 69,5 | 70,7 | 1,20 | 2,12 | [2,01; 2,24] | 27 % |
| hasta converger | conc. | 12 | 10 000 | 67,4 | 68,6 | 1,16 | 2,19 | [2,07; 2,33] | 18 % |
| 10 iteraciones | sec. | — | — | 32,8 | 33,8 | 2,98 | 1 (ref.) | — | — |
| 10 iteraciones | conc. | 1 | 1 000 | 31,7 | 33,0 | 2,89 | 1,03 | [0,88; 1,20] | 103 % |
| 10 iteraciones | conc. | 2 | 1 000 | 18,9 | 19,1 | 1,72 | 1,77 | [1,50; 2,10] | 88 % |
| 10 iteraciones | conc. | 4 | 1 000 | 11,7 | 12,5 | 1,07 | 2,70 | [2,20; 3,24] | 68 % |
| 10 iteraciones | conc. | 8 | 1 000 | 10,4 | 10,6 | 0,944 | 3,20 | [2,68; 3,83] | 40 % |
| 10 iteraciones | conc. | 12 | 1 000 | 8,80 | 8,68 | 0,800 | 3,90 | [3,39; 4,52] | 32 % |
| 10 iteraciones | conc. | 1 | 10 000 | 30,6 | 30,7 | 2,78 | 1,10 | [0,95; 1,27] | 110 % |
| 10 iteraciones | conc. | 2 | 10 000 | 23,7 | 22,9 | 2,15 | 1,48 | [1,30; 1,69] | 74 % |
| 10 iteraciones | conc. | 4 | 10 000 | 13,7 | 13,4 | 1,25 | 2,53 | [2,24; 2,87] | 63 % |
| 10 iteraciones | conc. | 8 | 10 000 | 14,5 | 14,5 | 1,31 | 2,34 | [2,07; 2,65] | 29 % |
| 10 iteraciones | conc. | 12 | 10 000 | 14,5 | 14,4 | 1,32 | 2,35 | [2,08; 2,69] | 20 % |

Tiempo y speedup del K-Means paralelo frente al secuencial
:::

*Nota.* Fuente: `generado/benchmark_kmeans.json`, escrito por `cmd/benchmark` (commit `53858a0`, 2026-11-01, máquina A019803). 5 repeticiones medidas tras 1 de calentamiento, orden aleatorizado por bloques; se guarda el orden de cada ronda; se mide solo el clustering; carga y k-means++ quedan fuera. Speedup: media recortada secuencial / media recortada concurrente, IC bootstrap 95 % (2 000 réplicas); recorte de 10 % por extremo. n = 41 991 filas, k = 5. 14 CPU lógicas, go1.27.3. Idéntico bit a bit para todo W: hasta converger sí; hasta converger sí; 10 iteraciones sí; 10 iteraciones sí.
