<!-- Generado por tablas.py desde generado/codo.json. No editar a mano. -->

::: {#tbl-codo tbl-colwidths="[9,11,17,11,18,15,19]"}
| k | k efectivo | Inercia | Semilla | Caída frente a k − 1 | Iteraciones | Paro |
|---:|---:|---:|---:|---:|---:|:---|
| 2 | 2 | 98 814,3 | 43 | — | 9 | tolerancia |
| 3 | 3 | 80 625,3 | 48 | 18,4 % | 22 | tolerancia |
| 4 | 4 | 67 254,8 | 42 | 16,6 % | 46 | tolerancia |
| 5 | 5 | 59 232,8 | 47 | 11,9 % | 21 | tolerancia |
| 6 | 6 | 51 343,7 | 42 | 13,3 % | 18 | tolerancia |
| 7 | 7 | 46 914,3 | 43 | 8,6 % | 31 | tolerancia |
| 8 | 8 | 43 980,4 | 49 | 6,3 % | 37 | tolerancia |
| 9 | 9 | 41 419,4 | 43 | 5,8 % | 38 | tolerancia |
| 10 | 10 | 39 182,2 | 42 | 5,4 % | 53 | tolerancia |

Inercia por número de clusters (método del codo)
:::

*Nota.* Fuente: `generado/codo.json`, escrito por `cmd/entrenar -codo` (commit `53858a0`, 2026-11-01, máquina A019803). n = 41 991 filas. Inercia con los centroides finales sobre una submuestra sistemática del gold; para cada k, la mejor de 10 semillas desde la de la receta, igual que el modelo final. La silueta va en la PC4. k de la receta: 5.
