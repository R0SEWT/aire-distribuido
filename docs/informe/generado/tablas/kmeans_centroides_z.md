<!-- Generado por tablas.py desde generado/kmeans.json. No editar a mano. -->

::: {.callout-warning title="PLACEHOLDER"}
Cifras de prueba: `generado/kmeans.json` sale de correr los comandos de Go sobre la muestra sintética data/sample/oefa_muestra.csv (estaciones SINT-*, una semana), no sobre el CSV de OEFA. Se reemplaza cuando `cmd/entrenar` corra sobre el CSV completo.
:::

::: {#tbl-centroides-z tbl-colwidths="[10,12,10,17,17,17,17]"}
| Cluster | Filas | % | PM10 | Fracción fina | Viento | Humedad |
|---:|---:|---:|---:|---:|---:|---:|
| 0 | 121 | 19,6 % | -1,159 | 0,063 | -0,621 | 0,534 |
| 1 | 140 | 22,7 % | 0,207 | 0,880 | 0,912 | 0,689 |
| 2 | 111 | 18,0 % | -0,289 | -0,802 | 0,805 | -0,660 |
| 3 | 151 | 24,5 % | 0,458 | 0,274 | -0,671 | -1,033 |
| 4 | 94 | 15,2 % | 0,789 | -0,887 | -0,430 | 0,725 |

Centroides en el espacio estandarizado (z-score) en que corre el K-Means
:::

*Nota.* Fuente: `generado/kmeans.json`, escrito por `cmd/entrenar` (commit `abf3e7f`, 2026-11-01, máquina A019803). k = 5 (efectivo 5), semilla elegida 44, 18 iteraciones (convergió por tolerancia), inercia 1 254,1, n = 617, chunk 10 000, clusters vacíos: 0. El perfil de cada cluster en unidades originales está en la tabla de regímenes: es la media aritmética de sus lecturas, no este centroide invertido.
