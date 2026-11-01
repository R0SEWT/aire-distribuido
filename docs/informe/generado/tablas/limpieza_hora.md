<!-- Generado por tablas.py desde generado/limpieza.json. No editar a mano. -->

::: {#tbl-limpieza-hora tbl-colwidths="[62,20,18]"}
| Concepto | Lecturas | Proporción |
|:---|---:|---:|
| Lecturas procesadas con hora explícita (HORA_INICIO) | 100 572 | 7,1 % |
| Lecturas procesadas con hora inferida por posición | 1 323 386 | 92,9 % |
| Descartadas por la regla `hora` (día incompleto u hora ilegible) | 30 401 | 1,1 % |
| Filas anchas con al menos una celda de hora inferida | 217 248 | 93,0 % |

La hora de cada lectura: explícita o reconstruida por posición
:::

*Nota.* Fuente: `generado/limpieza.json`, escrito por `cmd/cargar` (commit `5306012`, 2026-11-01, máquina A019803). Convención de hora: «fin» (la lectura de 00:00 a 01:00 se fecha a las 01:00). La proporción de las dos primeras filas es sobre las procesadas; la de la regla, sobre las leídas; la última, sobre las filas anchas.
