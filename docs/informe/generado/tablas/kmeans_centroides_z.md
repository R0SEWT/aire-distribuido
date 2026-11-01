<!-- Generado por tablas.py desde generado/kmeans.json. No editar a mano. -->

::: {.callout-warning title="PLACEHOLDER"}
Cifras de prueba: `generado/kmeans.json` sale de correr los comandos de Go sobre la muestra sintética data/sample/oefa_muestra.csv (estaciones SINT-*, una semana), no sobre el CSV de OEFA. Se reemplaza cuando `cmd/entrenar` corra sobre el CSV completo.
:::

::: {#tbl-centroides-z tbl-colwidths="[10,12,10,17,17,17,17]"}
| Cluster | Filas | % | PM10 | Fracción fina | Viento | Humedad |
|---:|---:|---:|---:|---:|---:|---:|
| 0 | 125 | 20,3 % | -1,239 | 0,133 | -0,270 | 0,652 |
| 1 | 110 | 17,8 % | 0,575 | -0,924 | -0,169 | 0,807 |
| 2 | 142 | 23,0 % | 0,243 | 0,141 | -0,928 | -0,986 |
| 3 | 123 | 19,9 % | 0,501 | 1,033 | 0,752 | 0,611 |
| 4 | 117 | 19,0 % | -0,038 | -0,532 | 0,783 | -0,902 |

Centroides en el espacio estandarizado (z-score) en que corre el K-Means
:::

*Nota.* Fuente: `generado/kmeans.json`, escrito por `cmd/entrenar` (commit `94d4037`, 2026-11-01, máquina A019803). k = 5 (efectivo 5), semilla 42, 28 iteraciones (convergió por tolerancia), inercia 1 257,3, n = 617, 14 workers, chunk 10 000, clusters vacíos: 0.
