<!-- Generado por tablas.py desde generado/codo.json. No editar a mano. -->

::: {#tbl-codo tbl-colwidths="[9,11,17,11,18,15,19]"}
| k | k efectivo | Inercia | Semilla | Caída frente a k − 1 | Iteraciones | Paro |
|---:|---:|---:|---:|---:|---:|:---|
| 2 | 2 | 72 440,0 | 44 | — | 23 | tolerancia |
| 3 | 3 | 49 658,4 | 42 | 31,4 % | 24 | tolerancia |
| 4 | 4 | 40 126,1 | 44 | 19,2 % | 92 | tolerancia |
| 5 | 5 | 32 224,3 | 46 | 19,7 % | 39 | tolerancia |
| 6 | 6 | 27 333,1 | 43 | 15,2 % | 73 | tolerancia |
| 7 | 7 | 22 546,3 | 45 | 17,5 % | 100 | tolerancia |
| 8 | 8 | 19 274,9 | 45 | 14,5 % | 52 | tolerancia |
| 9 | 9 | 16 583,8 | 51 | 14,0 % | 100 | max_iter |
| 10 | 10 | 14 707,6 | 45 | 11,3 % | 100 | max_iter |

Inercia por número de clusters (método del codo)
:::

*Nota.* Fuente: `generado/codo.json`, escrito por `cmd/entrenar -codo` (commit `11cef5a`, 2026-11-01, máquina A019803). n = 64 236 filas. Inercia con los centroides finales sobre una submuestra sistemática del gold; para cada k, la mejor de 10 semillas desde la de la receta, igual que el modelo final. La silueta va en la PC4. k de la receta: 3.
