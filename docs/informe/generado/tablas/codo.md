<!-- Generado por tablas.py desde generado/codo.json. No editar a mano. -->

::: {.callout-warning title="PLACEHOLDER"}
Cifras de prueba: `generado/codo.json` sale de correr los comandos de Go sobre la muestra sintética data/sample/oefa_muestra.csv (estaciones SINT-*, una semana), no sobre el CSV de OEFA. Se reemplaza cuando `cmd/entrenar -codo` corra sobre el CSV completo.
:::

::: {#tbl-codo tbl-colwidths="[10,14,20,22,16,18]"}
| k | k efectivo | Inercia | Caída frente a k − 1 | Iteraciones | Paro |
|---:|---:|---:|---:|---:|:---|
| 2 | 2 | 1 954,1 | — | 15 | tolerancia |
| 3 | 3 | 1 655,3 | 15,3 % | 30 | tolerancia |
| 4 | 4 | 1 451,1 | 12,3 % | 16 | tolerancia |
| 5 | 5 | 1 257,3 | 13,4 % | 28 | tolerancia |
| 6 | 6 | 1 143,9 | 9,0 % | 27 | tolerancia |
| 7 | 7 | 1 069,7 | 6,5 % | 10 | tolerancia |
| 8 | 8 | 975,7 | 8,8 % | 16 | tolerancia |

Inercia por número de clusters (método del codo)
:::

*Nota.* Fuente: `generado/codo.json`, escrito por `cmd/entrenar -codo` (commit `94d4037`, 2026-11-01, máquina A019803). n = 617 filas. Inercia con los centroides finales sobre una submuestra sistemática del gold, k-means++ con la semilla de la receta para cada k. La silueta va en la PC4. k de la receta: 5.
