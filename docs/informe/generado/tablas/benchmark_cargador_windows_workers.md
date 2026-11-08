<!-- Generado por tablas.py desde generado/benchmark_cargador_windows.json. No editar a mano. -->

::: {#tbl-bench-win-workers tbl-colwidths="[9,11,12,11,9,15,11,22]"}
| Workers | Mediana (s) | Media recortada (s) | Speedup (medianas) | Speedup | IC 95 % | Eficiencia | sha256 del gold |
|---:|---:|---:|---:|---:|---:|---:|:---|
| 1 | 4,259 | 3,636 | 1,00 | 1,00 | [0,76; 1,34] | 100 % | `2fdace5ad859` |
| 2 | 2,065 | 1,908 | 2,06 | 1,91 | [1,50; 2,42] | 95 % | `2fdace5ad859` |
| 4 | 1,159 | 1,058 | 3,67 | 3,44 | [2,68; 4,35] | 86 % | `2fdace5ad859` |
| 8 | 0,795 | 0,750 | 5,36 | 4,85 | [3,84; 6,02] | 61 % | `2fdace5ad859` |
| 12 | 0,794 | 0,756 | 5,36 | 4,81 | [3,81; 5,99] | 40 % | `2fdace5ad859` |

Tiempo y speedup del cargador en Windows con bloques de 64 MiB
:::

*Nota.* Fuente: `generado/benchmark_cargador_windows.json`, registros de `cmd/cargar -bench` en Windows.
