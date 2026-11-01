<!-- Generado por tablas.py desde generado/benchmark_cargador.json. No editar a mano. -->

::: {#tbl-bench-cargador tbl-colwidths="[9,11,12,11,9,15,11,22]"}
| Workers | Mediana (s) | Media recortada (s) | Speedup (medianas) | Speedup | IC 95 % | Eficiencia | sha256 del gold |
|---:|---:|---:|---:|---:|---:|---:|:---|
| 1 | 6,25 | 6,23 | 1,00 | 1,00 | [0,99; 1,01] | 100 % | `ce1fd2a036e1` |
| 2 | 3,33 | 3,33 | 1,87 | 1,87 | [1,80; 1,92] | 93 % | `ce1fd2a036e1` |
| 4 | 2,11 | 2,11 | 2,96 | 2,96 | [2,91; 3,00] | 74 % | `ce1fd2a036e1` |
| 8 | 1,41 | 1,42 | 4,42 | 4,38 | [4,29; 4,49] | 55 % | `ce1fd2a036e1` |
| 12 | 1,20 | 1,20 | 5,22 | 5,17 | [5,06; 5,26] | 43 % | `ce1fd2a036e1` |

Tiempo y speedup del cargador concurrente
:::

*Nota.* Fuente: `generado/benchmark_cargador.json`, escrito por `cmd/cargar -bench` (commit `5306012`, 2026-11-01, máquina A019803). Carga completa (lectura, limpieza, pivot y gold, sin escribir los CSV) de `1a_Vigilancia y Seguimiento ambiental en la calidad del aire.csv` (2 883 877 filas largas, 20 bloque(s) de 64 MiB). 10 repeticiones por W, orden intercalado: en cada repetición se corre cada W una vez, en el orden de la lista. Speedup: media recortada con la base / media recortada con W, IC bootstrap 95 % (kmeans.ICBootstrapCociente), 2 000 réplicas, recorte de 10 % por extremo; al lado, el cociente de medianas. Base: 1 worker(s). Eficiencia = speedup / workers. 14 CPU, go1.27.3, linux/amd64. Silver y gold idénticos para todo W: sí.
