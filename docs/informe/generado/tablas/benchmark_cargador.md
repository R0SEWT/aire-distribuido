<!-- Generado por tablas.py desde generado/benchmark_cargador.json. No editar a mano. -->

::: {#tbl-bench-cargador tbl-colwidths="[9,11,12,11,9,15,11,22]"}
| Workers | Mediana (s) | Media recortada (s) | Speedup (medianas) | Speedup | IC 95 % | Eficiencia | sha256 del gold |
|---:|---:|---:|---:|---:|---:|---:|:---|
| 1 | 6,68 | 6,73 | 1,00 | 1,00 | [0,95; 1,05] | 100 % | `2fdace5ad859` |
| 2 | 3,68 | 3,75 | 1,82 | 1,80 | [1,70; 1,91] | 90 % | `2fdace5ad859` |
| 4 | 2,36 | 2,38 | 2,84 | 2,83 | [2,74; 2,98] | 71 % | `2fdace5ad859` |
| 8 | 1,59 | 1,61 | 4,22 | 4,17 | [3,99; 4,40] | 52 % | `2fdace5ad859` |
| 12 | 1,35 | 1,36 | 4,95 | 4,94 | [4,66; 5,21] | 41 % | `2fdace5ad859` |

Tiempo y speedup del cargador concurrente
:::

*Nota.* Fuente: `generado/benchmark_cargador.json`, escrito por `cmd/cargar -bench` (commit `6fb3b00`, 2026-11-01, laptop de desarrollo). Carga completa (lectura, limpieza, pivot y gold, sin escribir los CSV) de `1a_Vigilancia y Seguimiento ambiental en la calidad del aire.csv` (2 883 877 filas largas, 20 bloques de 64 MiB). 10 repeticiones por W, orden intercalado: en cada repetición se corre cada W una vez, en el orden de la lista. Speedup: media recortada con la base / media recortada con W, IC bootstrap 95 % (kmeans.ICBootstrapCociente), 2 000 réplicas, recorte de 10 % por extremo; al lado, el cociente de medianas. Base: 1 worker. Eficiencia = speedup / workers. 14 CPU, go1.27.3, linux/amd64. El silver y el gold salieron idénticos para todo W.
