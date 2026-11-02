<!-- Generado por tablas.py desde generado/codo.json. No editar a mano. -->

::: {#tbl-codo tbl-colwidths="[9,11,17,11,18,15,19]"}
| k | k efectivo | Inercia | Semilla | Caída frente a k − 1 | Iteraciones | Paro |
|---:|---:|---:|---:|---:|---:|:---|
| 2 | 2 | 74 444,0 | 42 | — | 35 | tolerancia |
| 3 | 3 | 50 950,4 | 42 | 31,6 % | 44 | tolerancia |
| 4 | 4 | 41 194,7 | 43 | 19,1 % | 58 | tolerancia |
| 5 | 5 | 33 126,0 | 43 | 19,6 % | 90 | tolerancia |
| 6 | 6 | 28 104,4 | 48 | 15,2 % | 46 | tolerancia |
| 7 | 7 | 23 183,7 | 51 | 17,5 % | 43 | tolerancia |
| 8 | 8 | 19 793,8 | 42 | 14,6 % | 87 | tolerancia |
| 9 | 9 | 17 043,4 | 45 | 13,9 % | 90 | tolerancia |
| 10 | 10 | 15 128,8 | 47 | 11,2 % | 88 | tolerancia |

Inercia por número de clusters (método del codo)
:::

*Nota.* Fuente: `generado/codo.json`, escrito por `cmd/entrenar -codo` (commit `6fb3b00`, 2026-11-01, máquina A019803). n = 65 873 filas. Inercia con los centroides finales sobre una submuestra sistemática del gold; para cada k, la mejor de 10 semillas desde la de la receta, igual que el modelo final. La silueta va en la PC4. k de la receta: 3.
