<!-- Generado por tablas.py desde generado/benchmark_cargador.json. No editar a mano. -->

::: {#tbl-bench-cargador tbl-colwidths="[9,11,12,11,9,15,11,22]"}
| Workers | Mediana (s) | Media recortada (s) | Speedup (medianas) | Speedup | IC 95 % | Eficiencia | sha256 del gold |
|---:|---:|---:|---:|---:|---:|---:|:---|
| 1 | 6,34 | 6,32 | 1,00 | 1,00 | [0,99; 1,01] | 100 % | `1a93989d9af8` |
| 2 | 3,30 | 3,31 | 1,92 | 1,91 | [1,87; 1,94] | 95 % | `1a93989d9af8` |
| 4 | 2,14 | 2,14 | 2,96 | 2,95 | [2,90; 2,98] | 74 % | `1a93989d9af8` |
| 8 | 1,43 | 1,44 | 4,44 | 4,39 | [4,25; 4,50] | 55 % | `1a93989d9af8` |
| 12 | 1,24 | 1,24 | 5,13 | 5,11 | [4,98; 5,22] | 43 % | `1a93989d9af8` |

Tiempo y speedup del cargador concurrente
:::

*Nota.* Fuente: `generado/benchmark_cargador.json`, escrito por `cmd/cargar -bench` (commit `abf3e7f`, 2026-11-01, máquina A019803). Carga completa (lectura, limpieza, pivot y gold, sin escribir los CSV) de `1a_Vigilancia y Seguimiento ambiental en la calidad del aire.csv` (2 883 877 filas largas, 20 bloque(s) de 64 MiB). 10 repeticiones por W, orden intercalado: en cada repetición se corre cada W una vez, en el orden de la lista. Speedup: media recortada con la base / media recortada con W, IC bootstrap 95 % (kmeans.ICBootstrapCociente), 2 000 réplicas, recorte de 10 % por extremo; al lado, el cociente de medianas. Base: 1 worker(s). Eficiencia = speedup / workers. 14 CPU, go1.27.3, linux/amd64. Silver y gold idénticos para todo W: sí.
