<!-- Generado por tablas.py desde generado/benchmark_cargador.json. No editar a mano. -->

::: {.callout-warning title="PLACEHOLDER"}
Cifras de ejemplo: `generado/benchmark_cargador.json` todavía es el archivo de muestra. Se reemplaza cuando `cmd/benchmark -parte cargador` escriba el real.
:::

::: {#tbl-bench-cargador tbl-colwidths="[16,10,12,14,12,24,12]"}
| Modo | Workers | Bloque | Mediana (s) | Speedup | IC 95 % | Eficiencia |
|:---|---:|---:|---:|---:|---:|---:|
| conc. | 1 | 64 MiB | 182,000 | 1,00 | [0,98; 1,02] | 100 % |
| conc. | 2 | 64 MiB | 95,100 | 1,91 | [1,88; 1,95] | 96 % |
| conc. | 4 | 64 MiB | 51,300 | 3,55 | [3,48; 3,61] | 89 % |
| conc. | 8 | 64 MiB | 30,200 | 6,03 | [5,92; 6,13] | 75 % |
| conc. | 11 | 64 MiB | 26,400 | 6,89 | [6,77; 7,01] | 63 % |

Tiempo y speedup del cargador concurrente sobre el CSV completo
:::

*Nota.* Fuente: `generado/benchmark_cargador.json`, escrito por `cmd/benchmark -parte cargador` (commit `PLACEHOLDER`, 2026-11-04). Mediana de 5 repeticiones tras 1 de calentamiento, en PLACEHOLDER (11 CPU lógicas, go1.26.x). Speedup contra workers=1; intervalo bootstrap del cociente de medianas con 2 000 réplicas. Eficiencia = speedup / workers. Modo: sec. = secuencial, conc. = concurrente.
