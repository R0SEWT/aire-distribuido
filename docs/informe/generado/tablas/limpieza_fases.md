<!-- Generado por tablas.py desde generado/limpieza.json. No editar a mano. -->

::: {#tbl-limpieza-fases tbl-colwidths="[30,20,12,12,26]"}
| Fase | Clave | Tiempo (s) | % del total | Tipo |
|:---|:---|---:|---:|:---|
| Lectura, parseo y reglas por fila | `lectura_parseo` | 1,00 | 85,2 % | paralela (W workers); incluye la fusión |
| Fusión de parciales en orden de bloque | `fusion` | 0,305 | 25,9 % | serial, solapada con la lectura |
| Hora por posición y reglas por día | `horas` | 0,0396 | 3,4 % | serial |
| Pivot a la tabla ancha | `pivot` | 0,0782 | 6,6 % | paralela por estación |
| Orden de la tabla ancha | `tabla` | 0,0491 | 4,2 % | serial |
| Gold: completitud y z-score | `gold_estandarizacion` | 0,00758 | 0,6 % | paralela por chunks |

Tiempo de cada fase de la carga
:::

*Nota.* Fuente: `generado/limpieza.json`, escrito por `cmd/cargar` (commit `5306012`, 2026-11-01, máquina A019803). Carga de 1,18 s con 12 workers y 20 bloques de 64 MiB. Las fases sin la fusión suman el total; la fusión está dentro de la lectura. Fuera de la medición, la escritura del silver y el gold (`salida`): 0,179 s.
