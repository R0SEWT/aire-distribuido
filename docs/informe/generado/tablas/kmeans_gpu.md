<!-- Generado por tablas.py desde reports/kmeans_gpu/*.json. No editar a mano. -->

::: {#tbl-gpu tbl-colwidths="[11,17,11,19,12,15,15]"}
| Precisión | Suma por cluster | Iteraciones | Dif. máx. de centroides frente a Go | Repetible bit a bit | ms por iteración, ×1 | ms por iteración, ×300 |
|:---|:---|---:|---:|:---|---:|---:|
| float64 | producto de matrices | 44 | 4,4 × 10⁻¹⁶ | sí | 1,02–1,03 | 259,7–262,4 |
| float64 | suma atómica | 44 | 5,7 × 10⁻¹⁵ | no | 0,620–0,637 | 131,4–132,5 |
| float32 | producto de matrices | 44 | 9,1 × 10⁻⁷ | sí | 0,676–1,19 | 108,0–108,9 |
| float32 | suma atómica | 45 | 1,1 × 10⁻⁵ | no | 0,630–1,11 | 101,4–101,6 |

El mismo K-Means en la GPU de los equipos del laboratorio (experimento aparte, no es parte del sistema)
:::

*Nota.* Fuente: `reports/kmeans_gpu/kmeans_gpu_H45-022.json`, `reports/kmeans_gpu/kmeans_gpu_H45-023.json`, `reports/kmeans_gpu/kmeans_gpu_H45-024.json`, `reports/kmeans_gpu/kmeans_gpu_H45-025.json`, escritos por `lab/gpu/kmeans_gpu.py`. Equipos `H45-022`, `H45-023`, `H45-024`, `H45-025`, cada uno con una NVIDIA T1000 8GB (PyTorch 2.14.1+cu126). Lloyd desde el mismo arranque que `cmd/entrenar` (semilla 42), comparado con `reports/kmeans.json`. Los tiempos son el menor y el mayor entre los equipos; ×300 es el gold repetido 300 veces (19 761 900 filas).
