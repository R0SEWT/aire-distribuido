<!-- Generado por tablas.py desde generado/limpieza.json. No editar a mano. -->

::: {.callout-warning title="PLACEHOLDER"}
Cifras de prueba: `generado/limpieza.json` sale de correr los comandos de Go sobre la muestra sintética data/sample/oefa_muestra.csv (estaciones SINT-*, una semana), no sobre el CSV de OEFA. Se reemplaza cuando `cmd/cargar` corra sobre el CSV completo.
:::

::: {#tbl-limpieza-resumen tbl-colwidths="[62,20,18]"}
| Concepto | Filas | Proporción |
|:---|---:|---:|
| Filas largas leídas | 4 236 | 100 % |
| Filas largas procesadas (llegan al silver) | 4 188 | 98,9 % |
| Filas largas descartadas | 48 | 1,1 % |
| Filas anchas (estación × fecha-hora), regla 7 | 648 | 100 % |
| Filas anchas completas para el modelo (gold), regla 8 | 617 | 95,2 % |
| Filas anchas con PM2,5 > PM10 (se marcan, no se descartan) | 4 | 0,62 % |

Conservación de filas en la carga
:::

*Nota.* Fuente: `generado/limpieza.json`, escrito por `cmd/cargar` (commit `e1cef10`, 2026-11-01, máquina A019803). Leídas = procesadas + descartadas, en filas largas. Archivo `data/sample/oefa_muestra.csv` (722 035 bytes, sha256 `38fdecc55cb1`, delimitador «;», con BOM), 4 workers, 1 bloque(s) de 64 MiB, 0,00567 s.
