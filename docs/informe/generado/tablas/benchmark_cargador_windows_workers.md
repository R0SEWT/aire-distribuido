<!-- Generado por tablas.py desde generado/benchmark_cargador_windows.json. No editar a mano. -->

::: {#tbl-bench-win-workers tbl-colwidths="[9,11,12,11,9,15,11,22]"}
| Workers | Mediana (s) | Media recortada (s) | Speedup (medianas) | Speedup | IC 95 % | Eficiencia | sha256 del gold |
|---:|---:|---:|---:|---:|---:|---:|:---|
| 1 | 4,095 | 4,121 | 1,00 | 1,00 | [0,87; 1,15] | 100 % | `2fdace5ad859` |
| 2 | 2,067 | 2,061 | 1,98 | 2,00 | [1,76; 2,10] | 100 % | `2fdace5ad859` |
| 4 | 1,174 | 1,173 | 3,49 | 3,51 | [3,09; 3,68] | 88 % | `2fdace5ad859` |
| 8 | 0,810 | 0,808 | 5,05 | 5,10 | [4,48; 5,35] | 64 % | `2fdace5ad859` |
| 12 | 0,817 | 0,817 | 5,01 | 5,04 | [4,44; 5,30] | 42 % | `2fdace5ad859` |

Tiempo y speedup del cargador en Windows con bloques de 64 MiB
:::

*Nota.* Fuente: `generado/benchmark_cargador_windows.json`, registros de `cmd/cargar -bench` en Windows. El speedup principal es el cociente de medias recortadas, con recorte de 10 % por extremo e IC bootstrap 95 % de 2 000 réplicas. La base es un worker. Eficiencia = speedup / workers. También se muestra el cociente de medianas. La huella del gold se abrevia a 12 caracteres.
