<!-- Generado por tablas.py desde generado/limpieza.json. No editar a mano. -->

::: {.callout-warning title="PLACEHOLDER"}
Cifras de prueba: `generado/limpieza.json` sale de correr los comandos de Go sobre la muestra sintética data/sample/oefa_muestra.csv (estaciones SINT-*, una semana), no sobre el CSV de OEFA. Se reemplaza cuando `cmd/cargar` corra sobre el CSV completo.
:::

::: {#tbl-limpieza-estaciones tbl-colwidths="[17,12,10,10,8,8,9,13,13]"}
| Estación | Departamento | Largas leídas | Procesadas | Anchas | Gold | % gold | Desde | Hasta |
|:---|:---|---:|---:|---:|---:|---:|:---|:---|
| SINT-A Lima Norte | Lima | 1 364 | 1 339 | 168 | 165 | 98,2 % | 2024-03-04 | 2024-03-10 |
| SINT-B Arequipa | Arequipa | 1 178 | 1 173 | 168 | 166 | 98,8 % | 2024-03-04 | 2024-03-10 |
| SINT-C Junín | Junín | 853 | 850 | 144 | 131 | 91,0 % | 2024-03-04 | 2024-03-10 |
| SINT-D Ucayali | Ucayali | 838 | 826 | 168 | 155 | 92,3 % | 2024-03-04 | 2024-03-10 |

Filas por estación: largas leídas y procesadas, anchas, completas para el modelo y rango de fechas
:::

*Nota.* Fuente: `generado/limpieza.json`, escrito por `cmd/cargar` (commit `e1cef10`, 2026-11-01, máquina A019803). Gold: filas anchas con todas las features de la receta (regla 8). Desde y hasta: primera y última hora con fila ancha, en hora de Lima. Una estación con lecturas y ninguna fila procesada aparece igual, con ceros.
