<!-- Generado por tablas.py desde generado/limpieza.json. No editar a mano. -->

::: {.callout-warning title="PLACEHOLDER"}
Cifras de ejemplo: `generado/limpieza.json` todavía es el archivo de muestra. Se reemplaza cuando `cmd/cargar` escriba el real.
:::

::: {#tbl-limpieza-reglas tbl-colwidths="[6,52,14,14,14]"}
| N.º | Regla | Descartadas | % de leídas | Marcadas |
|:---|:---|---:|---:|---:|
| 1 | Solo lecturas con Periodo horario | 240 000 | 6,67 % | 0 |
| 2 | Fecha y hora parseables y dentro del rango del dataset | 1 200 | 0,03 % | 0 |
| 3 | Valor numérico; sin negativos (salvo temperatura) ni centinelas | 18 500 | 0,51 % | 0 |
| 4 | Dentro del rango físico del parámetro (configs/limpieza.json) | 3 100 | 0,09 % | 0 |
| 5 | Duplicados por (estación, parámetro, fecha-hora): exactos se descartan, con valor distinto se marcan y se conserva el primero | 9 800 | 0,27 % | 450 |
| 6 | Una sola unidad por parámetro: las filas convertidas se cuentan como marcadas | 0 | 0,00 % | 120 000 |
|  | **Total** | **272 600** | **7,57 %** | **120 450** |

Filas largas descartadas y marcadas por regla de limpieza
:::

*Nota.* Fuente: `generado/limpieza.json`, escrito por `cmd/cargar` (commit `PLACEHOLDER`, 2026-11-04). Las marcadas se conservan (duplicados con valor distinto, unidades convertidas). Las reglas 7 y 8 (pivot y completitud) actúan sobre filas anchas: ver la tabla de conservación.
