<!-- Generado por tablas.py desde generado/csv_oefa.json. No editar a mano. -->

::: {.callout-warning title="PENDIENTE"}
Cifras transcritas de docs/specs/2026-10-31-pc3-design.md §4.1, «Lo que el CSV es de verdad» (inspección del 01/11). Se reemplazan por la salida del notebook 01 (`eda_bronze.json`) o de `cmd/cargar` cuando corran sobre el bronze.
:::

::: {#tbl-csv-oefa tbl-colwidths="[50,50]"}
| Característica | Valor |
|:---|:---|
| Tamaño y sha256 | 1 317 949 839 bytes, `1f09a806…f26d92` |
| Formato | 36 columnas, delimitador «;», UTF-8 sin BOM, punto decimal, textos entre comillas |
| Filas de datos | 2 883 877 |
| Filas con un solo parámetro (su columna; el resto en «-») | 2 609 011 (90,5 %) |
| Filas sin ningún parámetro (se descartan) | 274 866 (9,5 %) |
| Estaciones · evaluaciones · departamentos | 33 · 19 · 10 |
| Años | 2018–2022 (fecha de corte 2024-04-29) |
| Hora: falta (NA) | 2 715 954 (94,2 %) |
| Hora: «1899-12-31 HH:MM» (artefacto de Excel) | 159 454 |
| Hora: solo fecha | 8 469 |
| Grupos sin hora en bloques contiguos de 24 | 93 935 de 95 932 (97,9 %) |
| Control: hora = posición en el día + 1 | 98,5 % de 112 383 filas con hora |
| Gases en dos unidades (ppb y µg/m³) | SO2, CO, O3, H2S: 11 290 claves con hora |
| PM10 · PM2,5 horarios | 193 353 · 90 492 filas |

El CSV de OEFA tal como viene
:::

*Nota.* Inspección del archivo descargado el 2026-11-01. Las filas de datos se reparten en las tres formas de la hora sin resto, y en filas con y sin parámetro también.
