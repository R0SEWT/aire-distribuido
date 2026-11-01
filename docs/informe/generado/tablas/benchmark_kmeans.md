<!-- Generado por tablas.py desde generado/benchmark_kmeans.json. No editar a mano. -->

::: {#tbl-bench-kmeans tbl-colwidths="[14,7,5,8,11,12,10,9,15,9]"}
| Experimento | Modo | W | Chunk | Mediana (ms) | Media recortada (ms) | ms/iter. | Speedup | IC 95 % | Eficiencia |
|:---|:---|---:|---:|---:|---:|---:|---:|---:|---:|
| hasta converger | sec. | — | — | 45,2 | 48,5 | 1,81 | 1 (ref.) | — | — |
| hasta converger | conc. | 1 | 1 000 | 45,4 | 46,1 | 1,81 | 1,05 | [0,96; 1,17] | 105 % |
| hasta converger | conc. | 2 | 1 000 | 28,5 | 31,9 | 1,14 | 1,52 | [1,24; 1,85] | 76 % |
| hasta converger | conc. | 4 | 1 000 | 19,1 | 19,9 | 0,763 | 2,44 | [2,15; 2,80] | 61 % |
| hasta converger | conc. | 8 | 1 000 | 16,6 | 16,5 | 0,662 | 2,94 | [2,61; 3,33] | 37 % |
| hasta converger | conc. | 12 | 1 000 | 16,5 | 17,3 | 0,659 | 2,80 | [2,28; 3,46] | 23 % |
| hasta converger | conc. | 1 | 10 000 | 50,4 | 51,9 | 2,02 | 0,93 | [0,85; 1,04] | 93 % |
| hasta converger | conc. | 2 | 10 000 | 33,4 | 31,5 | 1,34 | 1,54 | [1,32; 1,85] | 77 % |
| hasta converger | conc. | 4 | 10 000 | 24,7 | 25,3 | 0,987 | 1,91 | [1,70; 2,18] | 48 % |
| hasta converger | conc. | 8 | 10 000 | 22,9 | 22,9 | 0,915 | 2,12 | [1,94; 2,36] | 27 % |
| hasta converger | conc. | 12 | 10 000 | 22,9 | 23,1 | 0,915 | 2,10 | [1,91; 2,34] | 17 % |
| 10 iteraciones | sec. | — | — | 27,2 | 25,6 | 2,47 | 1 (ref.) | — | — |
| 10 iteraciones | conc. | 1 | 1 000 | 20,2 | 23,0 | 1,83 | 1,11 | [0,94; 1,32] | 111 % |
| 10 iteraciones | conc. | 2 | 1 000 | 13,5 | 13,8 | 1,23 | 1,85 | [1,61; 2,09] | 93 % |
| 10 iteraciones | conc. | 4 | 1 000 | 10,0 | 10,2 | 0,913 | 2,52 | [2,19; 2,87] | 63 % |
| 10 iteraciones | conc. | 8 | 1 000 | 7,05 | 7,03 | 0,641 | 3,64 | [3,27; 4,02] | 45 % |
| 10 iteraciones | conc. | 12 | 1 000 | 8,74 | 8,62 | 0,794 | 2,97 | [2,65; 3,27] | 25 % |
| 10 iteraciones | conc. | 1 | 10 000 | 27,6 | 26,1 | 2,51 | 0,98 | [0,87; 1,11] | 98 % |
| 10 iteraciones | conc. | 2 | 10 000 | 15,5 | 18,8 | 1,41 | 1,36 | [0,95; 1,93] | 68 % |
| 10 iteraciones | conc. | 4 | 10 000 | 13,0 | 12,3 | 1,18 | 2,07 | [1,79; 2,44] | 52 % |
| 10 iteraciones | conc. | 8 | 10 000 | 11,1 | 11,0 | 1,01 | 2,33 | [2,14; 2,52] | 29 % |
| 10 iteraciones | conc. | 12 | 10 000 | 10,8 | 10,8 | 0,978 | 2,38 | [2,17; 2,57] | 20 % |

Tiempo y speedup del K-Means paralelo frente al secuencial
:::

*Nota.* Fuente: `generado/benchmark_kmeans.json`, escrito por `cmd/benchmark` (commit `11cef5a`, 2026-11-01, máquina A019803). 5 repeticiones medidas tras 1 de calentamiento, orden aleatorizado por bloques; se guarda el orden de cada ronda; se mide solo el clustering; carga y k-means++ quedan fuera. Speedup: media recortada secuencial / media recortada concurrente, IC bootstrap 95 % (2 000 réplicas); recorte de 10 % por extremo. n = 64 236 filas, k = 3. 14 CPU lógicas, go1.27.3. Idéntico bit a bit para todo W: hasta converger sí; hasta converger sí; 10 iteraciones sí; 10 iteraciones sí.
