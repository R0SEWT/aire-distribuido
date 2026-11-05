<!-- Generado por tablas.py desde generado/limpieza.json. No editar a mano. -->

::: {#tbl-limpieza-hora tbl-colwidths="[62,20,18]"}
| Concepto | Lecturas | Proporción |
|:---|---:|---:|
| Lecturas procesadas con hora explícita (HORA_INICIO) | 100 566 | 7,0 % |
| Lecturas procesadas con hora inferida por posición | 1 333 932 | 93,0 % |
| … de ellas, con la hora por su posición en el bloque físico (días de 2022 con filas vacías) | 19 710 | 1,4 % |
| Descartadas por la regla `hora` (día incompleto u hora ilegible) | 10 691 | 0,4 % |
| Filas anchas con al menos una celda de hora inferida | 219 542 | 93,0 % |

La hora de cada lectura: explícita o reconstruida por posición
:::

*Nota.* Fuente: `generado/limpieza.json`, escrito por `cmd/cargar` (commit `6fb3b00`, 2026-11-01, laptop de desarrollo). Convención de hora: «fin» (la lectura de 00:00 a 01:00 se fecha a las 01:00). Las proporciones de las tres primeras filas son sobre las procesadas; la de la regla, sobre las leídas; la última, sobre las filas anchas. La hora por bloque físico se cuenta al asignarla: alguna de esas lecturas pudo caer después en otra regla.
