<!-- Generado por tablas.py desde generado/benchmark_kmeans.json. No editar a mano. -->

::: {#tbl-bench-kmeans tbl-colwidths="[14,7,5,8,11,12,10,9,15,9]"}
| Experimento | Modo | W | Chunk | Mediana (ms) | Media recortada (ms) | ms/iter. | Speedup | IC 95 % | Eficiencia |
|:---|:---|---:|---:|---:|---:|---:|---:|---:|---:|
| hasta converger | sec. | — | — | 91,5 | 99,9 | 2,03 | 1 (ref.) | — | — |
| hasta converger | conc. | 1 | 1 000 | 92,3 | 98,1 | 2,05 | 1,02 | [0,85; 1,22] | 102 % |
| hasta converger | conc. | 2 | 1 000 | 70,6 | 69,5 | 1,57 | 1,44 | [1,22; 1,71] | 72 % |
| hasta converger | conc. | 4 | 1 000 | 40,1 | 39,0 | 0,892 | 2,57 | [2,21; 3,05] | 64 % |
| hasta converger | conc. | 8 | 1 000 | 32,5 | 31,3 | 0,722 | 3,19 | [2,75; 3,80] | 40 % |
| hasta converger | conc. | 12 | 1 000 | 24,8 | 25,9 | 0,551 | 3,86 | [3,28; 4,60] | 32 % |
| hasta converger | conc. | 1 | 10 000 | 105,8 | 108,0 | 2,35 | 0,93 | [0,75; 1,13] | 93 % |
| hasta converger | conc. | 2 | 10 000 | 68,5 | 67,6 | 1,52 | 1,48 | [1,26; 1,77] | 74 % |
| hasta converger | conc. | 4 | 10 000 | 45,7 | 47,6 | 1,02 | 2,10 | [1,75; 2,54] | 52 % |
| hasta converger | conc. | 8 | 10 000 | 41,9 | 41,2 | 0,931 | 2,43 | [2,11; 2,87] | 30 % |
| hasta converger | conc. | 12 | 10 000 | 36,4 | 44,4 | 0,809 | 2,25 | [1,67; 3,01] | 19 % |
| 10 iteraciones | sec. | — | — | 37,9 | 34,9 | 3,45 | 1 (ref.) | — | — |
| 10 iteraciones | conc. | 1 | 1 000 | 31,2 | 30,7 | 2,83 | 1,14 | [0,87; 1,49] | 114 % |
| 10 iteraciones | conc. | 2 | 1 000 | 18,6 | 18,5 | 1,69 | 1,89 | [1,46; 2,38] | 94 % |
| 10 iteraciones | conc. | 4 | 1 000 | 10,6 | 10,3 | 0,966 | 3,39 | [2,66; 4,16] | 85 % |
| 10 iteraciones | conc. | 8 | 1 000 | 8,01 | 8,25 | 0,728 | 4,23 | [3,36; 5,05] | 53 % |
| 10 iteraciones | conc. | 12 | 1 000 | 7,84 | 7,61 | 0,713 | 4,58 | [3,64; 5,53] | 38 % |
| 10 iteraciones | conc. | 1 | 10 000 | 21,0 | 24,4 | 1,91 | 1,43 | [1,04; 1,88] | 143 % |
| 10 iteraciones | conc. | 2 | 10 000 | 20,1 | 20,0 | 1,83 | 1,74 | [1,40; 2,07] | 87 % |
| 10 iteraciones | conc. | 4 | 10 000 | 13,8 | 13,6 | 1,26 | 2,57 | [2,02; 3,18] | 64 % |
| 10 iteraciones | conc. | 8 | 10 000 | 12,1 | 12,1 | 1,10 | 2,89 | [2,32; 3,43] | 36 % |
| 10 iteraciones | conc. | 12 | 10 000 | 11,8 | 12,2 | 1,08 | 2,86 | [2,28; 3,43] | 24 % |

Tiempo y speedup del K-Means paralelo frente al secuencial
:::

*Nota.* Fuente: `generado/benchmark_kmeans.json`, escrito por `cmd/benchmark` (commit `6fb3b00`, 2026-11-01, laptop de desarrollo). 5 repeticiones medidas tras 1 de calentamiento, orden aleatorizado por bloques; se guarda el orden de cada ronda; se mide solo el clustering; carga y k-means++ quedan fuera. Speedup: media recortada secuencial / media recortada concurrente, IC bootstrap 95 % (2 000 réplicas); recorte de 10 % por extremo. n = 65 873 filas, k = 3. 14 CPU lógicas, go1.27.3. El resultado fue idéntico bit a bit para todo W en los dos experimentos.
