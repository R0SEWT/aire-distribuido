<!-- Generado por tablas.py desde generado/kmeans.json. No editar a mano. -->

::: {.callout-warning title="PLACEHOLDER"}
Cifras de prueba: `generado/kmeans.json` sale de correr los comandos de Go sobre la muestra sintética data/sample/oefa_muestra.csv (estaciones SINT-*, una semana), no sobre el CSV de OEFA. Se reemplaza cuando `cmd/entrenar` corra sobre el CSV completo.
:::

::: {#tbl-centroides tbl-colwidths="[10,12,10,17,17,17,17]"}
| Cluster | Filas | % | PM10 | Fracción fina | Viento | Humedad |
|---:|---:|---:|---:|---:|---:|---:|
| 0 | 125 | 20,3 % | 27,15 | 0,54 | 2,20 | 81,65 |
| 1 | 110 | 17,8 % | 64,11 | 0,43 | 2,31 | 84,42 |
| 2 | 142 | 23,0 % | 54,86 | 0,54 | 1,44 | 52,37 |
| 3 | 123 | 19,9 % | 61,92 | 0,63 | 3,37 | 80,91 |
| 4 | 117 | 19,0 % | 48,06 | 0,47 | 3,41 | 53,87 |

Centroides des-estandarizados (z-score y log1p invertidos)
:::

*Nota.* Fuente: `generado/kmeans.json`, escrito por `cmd/entrenar` (commit `94d4037`, 2026-11-01, máquina A019803). k = 5 (efectivo 5), semilla 42, 28 iteraciones (convergió por tolerancia), inercia 1 257,3, n = 617, 14 workers, chunk 10 000, clusters vacíos: 0. Unidades: PM10 en µg/m³; Viento en m/s; Humedad en %. Centroides llevados a unidades originales invirtiendo el z-score y luego log1p (reports/estandarizacion.json). En las features con log1p el centroide invertido es la media de los logaritmos llevada de vuelta (del orden de una media geométrica), menor que la media aritmética; la media aritmética de cada régimen está en reports/regimenes.json (perfil).
