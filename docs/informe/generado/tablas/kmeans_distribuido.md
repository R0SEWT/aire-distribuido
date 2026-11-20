<!-- Generado por tablas.py desde reports/kmeans_distribuido_*.json. No editar a mano. -->

::: {#tbl-distribuido tbl-colwidths="[11,9,11,10,7,14,12,9,9,8]"}
| Dónde | Nodos | Réplicas | Filas | Iter. | Frente a `cmd/entrenar` | Dif. máx. de centroides | Ronda p50 (ms) | Fusión p50 (ms) | Lloyd (ms) |
|:---|---:|---:|---:|---:|:---|---:|---:|---:|---:|
| loopback | 1 | 1 | 65 873 | 44 | bit a bit | 0 | 0,710 | 0,00117 | 32,4 |
| loopback | 2 | 1 | 65 873 | 44 | dentro de 10⁻⁹ | 4,4 × 10⁻¹⁶ | 0,384 | 0,00113 | 18,1 |
| loopback | 3 | 1 | 65 873 | 44 | dentro de 10⁻⁹ | 8,9 × 10⁻¹⁶ | 0,293 | 0,00116 | 13,9 |
| loopback | 4 | 1 | 65 873 | 44 | dentro de 10⁻⁹ | 1,3 × 10⁻¹⁵ | 0,276 | 0,00145 | 14,3 |
| loopback | 4 | 10 | 658 730 | 44 | dentro de 10⁻⁹ | 8,9 × 10⁻¹⁶ | 2,05 | 0,00287 | 97,8 |

K-Means distribuido frente al de una sola máquina: mismo gold, misma semilla y mismo arranque
:::

*Nota.* Fuente: `reports/kmeans_distribuido_local_1.json`, `reports/kmeans_distribuido_local_2.json`, `reports/kmeans_distribuido_local_3.json`, `reports/kmeans_distribuido_local_4.json`, `reports/kmeans_distribuido_local_4_r10.json`, escritos por `cmd/distribuido`. «Iter.» son las iteraciones hasta converger. «Frente a `cmd/entrenar`» compara los centroides con `reports/kmeans.json`: bit a bit, o la mayor diferencia absoluta en el espacio estandarizado. «Ronda» es la barrera de red de una iteración (del primer `ITERAR` al último `PARCIAL`) y «Fusión», lo que el coordinador hace después. Las corridas «loopback» (2026-11-20, commit `c69dae4`) tienen el coordinador y todos los nodos en la laptop de desarrollo (14 CPU lógicas), con 2 workers por nodo: comprueban el protocolo de punta a punta y el determinismo, no el rendimiento de un clúster. En `reports/` no hay corridas del K-Means distribuido sobre los equipos del laboratorio: allí se midió la barrera con el smoke test (@tbl-smoke).
