<!-- Generado por tablas.py desde reports/alerta_24h.json. No editar a mano. -->

::: {#tbl-alerta tbl-colwidths="[40,14,22,12,12]"}
| Señal | Horas con la señal | De esas, con la media de 24 h sobre el ECA | Precisión | Recall |
|:---|---:|---:|---:|---:|
| La lectura cae en el régimen alto del K-Means | 1 497 | 524 | 35 % | 44 % |
| La lectura horaria supera el umbral del ECA | 3 011 | 649 | 22 % | 55 % |
| La media de 24 h de la estación supera el ECA (la regla de la API) | 1 184 | 1 184 | 100 % | 100 % |

Qué acertaría cada señal como alerta, sobre las horas del gold con media de 24 horas calculable
:::

*Nota.* Fuente: `reports/alerta_24h.json`, escrito por `go test ./ventana -run LaVentanaSobreElGold -actualizar`. De las 65 873 horas del gold, 64 748 tienen al menos 18 lecturas de su estación en las 24 horas que terminan en ella; en 1 184 (1,8 %) esa media supera el ECA (PM2,5 > 50 o PM10 > 100 µg/m³). Precisión: de las horas con la señal, cuántas tienen la media sobre el ECA. Recall: de las horas con la media sobre el ECA, cuántas tienen la señal. La última fila es la regla misma, no una medición.
