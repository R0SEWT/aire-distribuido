<!-- Generado por tablas.py desde generado/benchmark_cargador.json. No editar a mano. -->

::: {.callout-warning title="PLACEHOLDER"}
Cifras de prueba: `generado/benchmark_cargador.json` sale de correr los comandos de Go sobre la muestra sintética data/sample/oefa_muestra.csv (estaciones SINT-*, una semana), no sobre el CSV de OEFA. Se reemplaza cuando `cmd/cargar -bench` corra sobre el CSV completo.
:::

::: {#tbl-bench-cargador tbl-colwidths="[9,11,12,11,9,15,11,22]"}
| Workers | Mediana (s) | Media recortada (s) | Speedup (medianas) | Speedup | IC 95 % | Eficiencia | sha256 del gold |
|---:|---:|---:|---:|---:|---:|---:|:---|
| 1 | 0,00853 | 0,00878 | 1,00 | 1,00 | [0,78; 1,28] | 100 % | `ddf7ae68f071` |
| 2 | 0,00643 | 0,00683 | 1,33 | 1,29 | [1,03; 1,60] | 64 % | `ddf7ae68f071` |
| 4 | 0,00694 | 0,00703 | 1,23 | 1,25 | [0,97; 1,60] | 31 % | `ddf7ae68f071` |

Tiempo y speedup del cargador concurrente
:::

*Nota.* Fuente: `generado/benchmark_cargador.json`, escrito por `cmd/cargar -bench` (commit `e1cef10`, 2026-11-01, máquina A019803). Carga completa (lectura, limpieza, pivot y gold, sin escribir los CSV) de `data/sample/oefa_muestra.csv` (4 236 filas largas, 1 bloque(s) de 64 MiB). 3 repeticiones por W, orden intercalado: en cada repetición se corre cada W una vez, en el orden de la lista. Speedup: media recortada con la base / media recortada con W, IC bootstrap 95 % (kmeans.ICBootstrapCociente), 2 000 réplicas, recorte de 10 % por extremo; al lado, el cociente de medianas. Base: 1 worker(s). Eficiencia = speedup / workers. 14 CPU, go1.27.3, linux/amd64. Silver y gold idénticos para todo W: sí.
