<!-- Generado por tablas.py desde generado/codo.json. No editar a mano. -->

::: {.callout-warning title="PLACEHOLDER"}
Cifras de prueba: `generado/codo.json` sale de correr los comandos de Go sobre la muestra sintética data/sample/oefa_muestra.csv (estaciones SINT-*, una semana), no sobre el CSV de OEFA. Se reemplaza cuando `cmd/entrenar -codo` corra sobre el CSV completo.
:::

::: {#tbl-codo tbl-colwidths="[9,11,17,11,18,15,19]"}
| k | k efectivo | Inercia | Semilla | Caída frente a k − 1 | Iteraciones | Paro |
|---:|---:|---:|---:|---:|---:|:---|
| 2 | 2 | 1 953,7 | 49 | — | 8 | tolerancia |
| 3 | 3 | 1 652,0 | 51 | 15,4 % | 15 | tolerancia |
| 4 | 4 | 1 436,0 | 46 | 13,1 % | 34 | tolerancia |
| 5 | 5 | 1 254,1 | 44 | 12,7 % | 18 | tolerancia |
| 6 | 6 | 1 120,0 | 44 | 10,7 % | 15 | tolerancia |
| 7 | 7 | 1 024,3 | 51 | 8,5 % | 23 | tolerancia |
| 8 | 8 | 937,2 | 45 | 8,5 % | 36 | tolerancia |

Inercia por número de clusters (método del codo)
:::

*Nota.* Fuente: `generado/codo.json`, escrito por `cmd/entrenar -codo` (commit `abf3e7f`, 2026-11-01, máquina A019803). n = 617 filas. Inercia con los centroides finales sobre una submuestra sistemática del gold; para cada k, la mejor de 10 semillas desde la de la receta, igual que el modelo final. La silueta va en la PC4. k de la receta: 5.
