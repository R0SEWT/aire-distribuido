<!-- Generado por tablas.py desde generado/limpieza.json. No editar a mano. -->

::: {.callout-warning title="PLACEHOLDER"}
Cifras de prueba: `generado/limpieza.json` sale de correr los comandos de Go sobre la muestra sintética data/sample/oefa_muestra.csv (estaciones SINT-*, una semana), no sobre el CSV de OEFA. Se reemplaza cuando `cmd/cargar` corra sobre el CSV completo.
:::

::: {#tbl-limpieza-reglas tbl-colwidths="[8,28,33,11,10,10]"}
| Spec | Regla | Qué descarta | Descartadas | % de leídas | Marcadas |
|:---|:---|:---|---:|---:|---:|
| — | `columnas` | La fila no tiene tantos campos como la cabecera | 2 | 0,05 % | — |
| — | `estacion` | Estación vacía | 1 | 0,02 % | — |
| 1 | `periodo` | El periodo no es horario (diario, mensual…) | 15 | 0,35 % | — |
| — | `parametro` | Parámetro sin mapeo a un canónico en configs/limpieza.json | 5 | 0,12 % | — |
| 2 | `fecha` | Fecha y hora no parseables con ningún formato | 3 | 0,07 % | — |
| 2 | `fecha_fuera_de_rango` | Fecha fuera del rango configurado | 2 | 0,05 % | — |
| 3 | `valor_no_numerico` | Valor vacío, texto, NaN o infinito | 3 | 0,07 % | — |
| 3 | `centinela` | Código del equipo (−999, 9999…) | 3 | 0,07 % | — |
| 6 | `unidad` | Unidad que no figura para ese parámetro | 2 | 0,05 % | — |
| 3 | `negativo` | Negativo en un parámetro que no los admite | 2 | 0,05 % | — |
| 4 | `fuera_de_rango` | Fuera del rango físico del parámetro | 3 | 0,07 % | — |
| 5 | `duplicado_exacto` | Misma (estación, parámetro, fecha-hora) y mismo valor | 4 | 0,09 % | — |
| 5 | `duplicado_valor_distinto` | Misma clave con otro valor: se descarta el repetido y se conserva el primero | 3 | 0,07 % | 3 |
|  |  | **Total** | **48** | **1,13 %** | **3** |

Filas largas descartadas y marcadas por regla, en el orden de la cascada
:::

*Nota.* Fuente: `generado/limpieza.json`, escrito por `cmd/cargar` (commit `e1cef10`, 2026-11-01, máquina A019803). Cada fila descartada se cuenta en la primera regla que falla (cascada de `datos/bloque.go`), así que la suma no tiene solapes. «Spec» es el número de regla de la @tbl-reglas; «—» son controles de estructura. Marcadas: celdas cuyo valor conservado tuvo un duplicado con otro valor (se conservan). Fuera de rango por parámetro: Humedad 1; PM2,5 1; Viento 1.
