<!-- Generado por tablas.py desde generado/limpieza.json y generado/eda.json. No editar a mano. -->

::: {#tbl-csv-oefa tbl-colwidths="[50,50]"}
| Característica | Valor |
|:---|:---|
| Tamaño y sha256 | 1 317 949 839 bytes, `1f09a8064ee0…` |
| Formato | 36 columnas, delimitador «;», sin BOM; una columna por parámetro y un valor por fila |
| Registros con un salto de línea dentro de un campo | 100 |
| Filas leídas | 2 883 877 |
| Estaciones: en el archivo · con lecturas válidas · en el gold | 33 · 25 · 17 |
| Periodos horarios («1 hora» y «-») · otros (24 h, móviles) | 2 675 919 · 207 958 |
| Filas sin ningún valor (regla `sin_valor`: la plantilla de 2022) | 262 743 |
| Lecturas procesadas con hora inferida por posición | 1 333 932 (93,0 %) |
| Control: hora de inicio = posición en el bloque físico, donde hay hora explícita | 100,0 % de 165 912 filas |
| Días enteros en 0 exacto (regla `bloque_en_cero`) | 762 744 |
| Horas en 0 exacto de un contaminante en días con datos (`cero_como_faltante`) | 8 694 |
| Lecturas gemelas en otra unidad (regla `duplicado_unidad`) | 134 285 |

El CSV de OEFA tal como viene
:::

*Nota.* Fuente: `generado/limpieza.json`, escrito por `cmd/cargar` (commit `6fb3b00`, 2026-11-01, laptop de desarrollo); el control de la hora y los registros multilínea, de `reports/eda_criba.md` (notebook 01 sobre el mismo archivo), transcritos en `generado/eda.json`.
