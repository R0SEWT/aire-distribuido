<!-- Generado por tablas.py desde generado/limpieza.json. No editar a mano. -->

::: {#tbl-limpieza-reglas tbl-colwidths="[8,28,42,12,10]"}
| Spec | Regla | Qué descarta | Descartadas | % de leídas |
|:---|:---|:---|---:|---:|
| — | `columnas` | La fila no tiene tantos campos como la cabecera | 0 | 0,00 % |
| — | `estacion` | Estación vacía | 0 | 0,00 % |
| 1 | `periodo` | Periodo no horario (24 horas, medias móviles); «-» sin hora y con el día incompleto | 218 998 | 7,59 % |
| — | `sin_valor` | Ninguna columna de parámetro con valor: el hueco de la plantilla de 2022 | 262 743 | 9,11 % |
| — | `parametro` | Más de una columna de parámetro con valor | 0 | 0,00 % |
| 2 | `fecha` | Fecha no parseable | 3 | 0,00 % |
| 2 | `fecha_fuera_de_rango` | Fecha fuera del rango configurado | 0 | 0,00 % |
| 2 | `hora_no_en_punto` | Hora con minutos o segundos | 0 | 0,00 % |
| 2′ | `hora` | Hora ilegible, o sin hora y con el día incompleto (no se puede inferir por posición) | 10 691 | 0,37 % |
| 3 | `valor_no_numerico` | Texto (<LD, ND, ---), NaN o infinito; no se imputa | 0 | 0,00 % |
| 3 | `centinela` | Código del equipo (−999, 9999…), sobre el valor crudo | 0 | 0,00 % |
| 6 | `unidad` | Unidad que no figura para ese parámetro | 0 | 0,00 % |
| 3 | `negativo` | Negativo en un parámetro que no los admite | 2 | 0,00 % |
| 4 | `fuera_de_rango` | Fuera del rango físico, ya en la unidad canónica | 51 219 | 1,78 % |
| 3′ | `bloque_en_cero` | Un día entero en 0 exacto: equipo sin medir, no aire limpio | 762 744 | 26,45 % |
| 3′ | `cero_como_faltante` | Un 0 exacto de un contaminante en un día con datos: hora sin dato | 8 694 | 0,30 % |
| 5/6 | `duplicado_unidad` | La misma lectura en otra unidad (ppb, mmHg): se conserva la canónica | 134 285 | 4,66 % |
| 5 | `duplicado_exacto` | Misma (estación, parámetro, fecha-hora) y mismo valor | 0 | 0,00 % |
| 5 | `duplicado_valor_distinto` | Misma clave con otro valor: se conserva el primero | 0 | 0,00 % |
|  |  | **Total** | **1 449 379** | **50,26 %** |

Filas largas descartadas por regla, en el orden de la cascada
:::

*Nota.* Fuente: `generado/limpieza.json`, escrito por `cmd/cargar` (commit `6fb3b00`, 2026-11-01, máquina A019803). Cada fila descartada se cuenta en la primera regla que falla, en el orden de la cascada de Go (`datos/doc.go`), así que la suma no tiene solapes. «Spec» es el número de regla de la @tbl-reglas; «—» son controles de estructura. Lo que se conserva y se marca está en la @tbl-limpieza-marcas. Fuera de rango por parámetro: O3 744; Precip. 2; Presión 50 472; Temp. 1.
