<!-- Generado por tablas.py desde docs/postman/lecturas_prueba.csv. No editar a mano. -->

::: {#tbl-datos-prueba tbl-colwidths="[19,12,13,13,15,15,13]"}
| Caso | Lecturas | Estaciones | Régimen esperado | PM2,5 (µg/m³) | PM10 (µg/m³) | Con alerta esperada |
|:---|---:|---:|---:|---:|---:|---:|
| regimen\_0 | 10 | 10 | 0 | 6,7–16,9 | 14,2–54,5 | 0 |
| regimen\_1 | 10 | 10 | 1 | 28,7–175,2 | 81,4–351,9 | 0 |
| regimen\_2 | 10 | 10 | 2 | 11,2–32,2 | 36,0–84,5 | 0 |
| serie\_alerta | 24 | 1 | 1 y 2 | 44,3–113,0 | 64,9–170,8 | 7 |
| serie\_sin\_alerta | 24 | 1 | 0, 1 y 2 | 19,6–107,1 | 27,6–127,5 | 0 |

Datos de prueba: lecturas reales con la respuesta que la API tiene que dar
:::

*Nota.* Fuente: `docs/postman/lecturas_prueba.csv`. Cada fila del archivo es una lectura real (estación, fecha y hora, PM2,5 y PM10) con el régimen que el modelo tiene que asignarle y, donde aplica, la media de 24 h, la alerta y la banda INCA esperadas. Lo usan el Runner de Postman y `cmd/carga`.
