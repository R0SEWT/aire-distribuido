<!-- Generado por tablas.py desde reports/mapa/lab_d56_*.json. No editar a mano. -->

::: {#tbl-mapa-gpu tbl-colwidths="[9,10,10,11,10,10,12,11,9,8]"}
| Nodos | H (pasos por ronda) | Rondas | Muestras por ronda | Barrera (s) | Cómputo (s) | Red y espera (s) | Muestras por s | Frente a un nodo | Total (min) |
|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 20 | 200 | 10 | 16 000 | 28,0 | 17,5 | 10,5 (37,4 %) | 570 | 12,3× | 5,1 |
| 20 | 2 000 | 5 | 160 000 | 183,4 | 177,7 | 5,7 (3,1 %) | 872 | 18,6× | 15,2 |

Experimento de laboratorio, fuera del sistema y con DATOS SINTÉTICOS: una red convolucional entrenada con local SGD en las GPU del laboratorio, con el coordinador en Go
:::

*Nota.* Fuente: `reports/mapa/lab_d56_20gpus_h2000_a64_sintetico.json`, `reports/mapa/lab_d56_20gpus_h200_a64_sintetico.json`, escritos por `cmd/mapa-coord` (2026-11-14); el código está en la PR #52, sin fusionar. **Datos sintéticos** de tamaño nacional (168 días): las corridas miden el clúster, no el aire. Nodos: 20 equipos del laboratorio D56, cada uno con una NVIDIA RTX A1000, que entrenan en Python con PyTorch una red de 7 972 354 parámetros (lote 4). Por ronda, cada nodo recibe los pesos (`PESOS`), entrena H pasos con sus datos y los devuelve (`LOCAL`); el coordinador espera a todos (la barrera) y promedia en orden de nodo. Barrera, cómputo y red son medianas por ronda, sin la primera. «Cómputo» es el del nodo más lento de la ronda; «Red y espera», la barrera menos ese cómputo, con su porcentaje sobre la barrera. «Frente a un nodo» divide las muestras por segundo del clúster entre las de un nodo mientras calcula (sus muestras sobre su propio cómputo).
