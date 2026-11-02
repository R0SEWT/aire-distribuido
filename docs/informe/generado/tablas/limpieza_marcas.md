<!-- Generado por tablas.py desde generado/limpieza.json. No editar a mano. -->

::: {#tbl-limpieza-marcas tbl-colwidths="[24,46,15,15]"}
| Marca | Qué es | Lecturas | % (de procesadas) |
|:---|:---|---:|---:|
| `hora_inferida` | Hora reconstruida por la posición de la fila en su día | 1 333 932 | 93,0 % |
| `unidad_convertida` | Convertida a la unidad canónica (mmHg → hPa, ppb → µg/m³) | 151 524 | 10,6 % |
| `ubicacion_corregida` | Fila leída de una estación con la ubicación corregida (sobre las leídas) | 5 296 | 0,2 % |
| `duplicado_valor_distinto` | Celda cuyo valor conservado tuvo un duplicado con otro valor | 0 | 0,0 % |
| `hora_no_en_punto` | Hora truncada a la hora en punto | 0 | 0,0 % |
| filas anchas con PM2,5 > PM10 | Se conservan; en el gold la fracción fina se recorta a 1 | 0 | — |

Lo que se conserva pero se marca
:::

*Nota.* Fuente: `generado/limpieza.json`, escrito por `cmd/cargar` (commit `6fb3b00`, 2026-11-01, máquina A019803). Conversiones de unidad por parámetro: Presión 135 872; SO2 4 227; CO 4 082; H2S 3 864; O3 3 479.
