<!-- Generado por tablas.py desde generado/benchmark_cargador.json. No editar a mano. -->

::: {.callout-warning title="PLACEHOLDER"}
Cifras de prueba: `generado/benchmark_cargador.json` sale de correr los comandos de Go sobre la muestra sintética data/sample/oefa_muestra.csv (estaciones SINT-*, una semana), no sobre el CSV de OEFA. Se reemplaza cuando `cmd/cargar -bench` corra sobre el CSV completo.
:::

::: {#tbl-bench-cargador tbl-colwidths="[10,34,13,11,12,20]"}
| Workers | Tiempos (s) | Mediana (s) | Speedup | Eficiencia | sha256 del gold |
|---:|:---|---:|---:|---:|:---|
| 1 | 0,0105; 0,00853; 0,00728 | 0,00853 | 1,00 | 100 % | `ddf7ae68f071` |
| 2 | 0,00605; 0,00643; 0,00800 | 0,00643 | 1,33 | 66 % | `ddf7ae68f071` |
| 4 | 0,00838; 0,00694; 0,00579 | 0,00694 | 1,23 | 31 % | `ddf7ae68f071` |

Tiempo y speedup del cargador concurrente
:::

*Nota.* Fuente: `generado/benchmark_cargador.json`, escrito por `cmd/cargar -bench` (commit `e1cef10`, 2026-11-01, máquina A019803). Carga completa (lectura, limpieza, pivot y gold, sin escribir los CSV) de `data/sample/oefa_muestra.csv` (4 236 filas largas, 1 bloque(s) de 64 MiB). 3 repeticiones por W, orden intercalado: en cada repetición se corre cada W una vez, en el orden de la lista. Speedup = mediana con 1 worker(s) / mediana con W; el JSON no trae intervalo de confianza. 14 CPU, go1.27.3, linux/amd64. Silver y gold idénticos para todo W: sí.
