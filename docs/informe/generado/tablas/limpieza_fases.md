<!-- Generado por tablas.py desde generado/limpieza.json. No editar a mano. -->

::: {#tbl-limpieza-fases tbl-colwidths="[30,20,12,12,26]"}
| Fase | Clave | Tiempo (s) | % del total | Tipo |
|:---|:---|---:|---:|:---|
| Lectura, parseo y reglas por fila | `lectura_parseo` | 1,97 | 91,9 % | paralela (W workers); incluye la fusión |
| Fusión de parciales en orden de bloque | `fusion` | 0,432 | 20,1 % | serial, solapada con la lectura |
| Hora por posición y reglas por día | `horas` | 0,0373 | 1,7 % | serial |
| Pivot a la tabla ancha | `pivot` | 0,0786 | 3,7 % | paralela por estación |
| Orden de la tabla ancha | `tabla` | 0,0500 | 2,3 % | serial |
| Gold: completitud y z-score | `gold_estandarizacion` | 0,00837 | 0,4 % | paralela por chunks |

Tiempo de cada fase de la carga
:::

*Nota.* Fuente: `generado/limpieza.json`, escrito por `cmd/cargar` (commit `53858a0`, 2026-11-01, máquina A019803). Carga de 2,14 s con 4 workers y 79 bloques de 16 MiB. Las fases sin la fusión suman el total; la fusión está dentro de la lectura. Fuera de la medición, la escritura del silver y el gold (`salida`): 0,164 s.
