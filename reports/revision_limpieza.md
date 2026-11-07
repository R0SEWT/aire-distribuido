# Revisión de limpieza y completitud

Generado con `python3 docs/verificar-limpieza.py`. Compara salidas guardadas; no vuelve a ejecutar el bronze.

Fuentes: `reports/limpieza.json`, `reports/eda_limpieza_prototipo.json`, `reports/eda_criba.md`,
`reports/estandarizacion.json`, `configs/kmeans.json` y la copia de limpieza del informe.

SHA-256 del bronze declarado por Go: `1f09a8064ee0aed0b3a9a0a90b96f63ce226b45a969dace49f73e45134f26d92`. No se recalculó sobre el CSV.

| Control | Resultado |
|:---|:---|
| Copia del informe idéntica al reporte de Go | OK |
| Conteos de filas largas Go = prototipo Python | OK |
| Conteos de filas anchas Go = prototipo Python | OK |
| Mismo conjunto de estaciones Go/Python | OK |
| Conservación: leídas = procesadas + descartadas | OK |
| Suma de descartes por regla | OK |
| Hora explícita + inferida = procesadas | OK |
| Suma por estación: largas_leidas | OK |
| Suma por estación: largas_procesadas | OK |
| Suma por estación: anchas | OK |
| Suma por estación: completas_gold | OK |
| Gold = n de estandarización | OK |
| Features de limpieza y estandarización = receta | OK |
| EDA: total F1-M0 = gold | OK |
| EDA: mismas estaciones con silver | OK |
| CA-01: conteos y fechas Go/Python | OK |
| CA-01: gold ≤ silver | OK |
| CA-03: conteos y fechas Go/Python | OK |
| CA-03: gold ≤ silver | OK |
| CA-03: F1-M0 del EDA = gold | OK |
| CA-AYAS-01: conteos y fechas Go/Python | OK |
| CA-AYAS-01: gold ≤ silver | OK |
| CA-AYAS-01: F1-M0 del EDA = gold | OK |
| CA-CC-01: conteos y fechas Go/Python | OK |
| CA-CC-01: gold ≤ silver | OK |
| CA-CC-01: F1-M0 del EDA = gold | OK |
| CA-CG-01: conteos y fechas Go/Python | OK |
| CA-CG-01: gold ≤ silver | OK |
| CA-CG-01: F1-M0 del EDA = gold | OK |
| CA-CG-02: conteos y fechas Go/Python | OK |
| CA-CG-02: gold ≤ silver | OK |
| CA-CG-02: F1-M0 del EDA = gold | OK |
| CA-CH-03: conteos y fechas Go/Python | OK |
| CA-CH-03: gold ≤ silver | OK |
| CA-CH-03: F1-M0 del EDA = gold | OK |
| CA-COLQ-01: conteos y fechas Go/Python | OK |
| CA-COLQ-01: gold ≤ silver | OK |
| CA-COLQ-01: F1-M0 del EDA = gold | OK |
| CA-CP-01: conteos y fechas Go/Python | OK |
| CA-CP-01: gold ≤ silver | OK |
| CA-CP-01: F1-M0 del EDA = gold | OK |
| CA-ESP-01: conteos y fechas Go/Python | OK |
| CA-ESP-01: gold ≤ silver | OK |
| CA-ESP-01: F1-M0 del EDA = gold | OK |
| CA-HU-04: conteos y fechas Go/Python | OK |
| CA-HU-04: gold ≤ silver | OK |
| CA-HU-04: F1-M0 del EDA = gold | OK |
| CA-HU-09: conteos y fechas Go/Python | OK |
| CA-HU-09: gold ≤ silver | OK |
| CA-HU-09: F1-M0 del EDA = gold | OK |
| CA-HUA-01: conteos y fechas Go/Python | OK |
| CA-HUA-01: gold ≤ silver | OK |
| CA-HUA-01: F1-M0 del EDA = gold | OK |
| CA-HUAR-01: conteos y fechas Go/Python | OK |
| CA-HUAR-01: gold ≤ silver | OK |
| CA-HUAR-01: F1-M0 del EDA = gold | OK |
| CA-JUPR-01: conteos y fechas Go/Python | OK |
| CA-JUPR-01: gold ≤ silver | OK |
| CA-JUPR-01: F1-M0 del EDA = gold | OK |
| CA-MAC-01: conteos y fechas Go/Python | OK |
| CA-MAC-01: gold ≤ silver | OK |
| CA-MAC-01: F1-M0 del EDA = gold | OK |
| CA-ROM-01: conteos y fechas Go/Python | OK |
| CA-ROM-01: gold ≤ silver | OK |
| CA-ROM-01: F1-M0 del EDA = gold | OK |
| CA-RZC-01: conteos y fechas Go/Python | OK |
| CA-RZC-01: gold ≤ silver | OK |
| CA-RZC-01: F1-M0 del EDA = gold | OK |
| CA-RZC-02: conteos y fechas Go/Python | OK |
| CA-RZC-02: gold ≤ silver | OK |
| CA-RZC-02: F1-M0 del EDA = gold | OK |
| CA-SB-01: conteos y fechas Go/Python | OK |
| CA-SB-01: gold ≤ silver | OK |
| CA-SB-01: F1-M0 del EDA = gold | OK |
| CA-SOC-01: conteos y fechas Go/Python | OK |
| CA-SOC-01: gold ≤ silver | OK |
| CA-SOC-01: F1-M0 del EDA = gold | OK |
| CA-TA-01: conteos y fechas Go/Python | OK |
| CA-TA-01: gold ≤ silver | OK |
| CA-TA-01: F1-M0 del EDA = gold | OK |
| CA-TAL-01: conteos y fechas Go/Python | OK |
| CA-TAL-01: gold ≤ silver | OK |
| CA-TAL-01: F1-M0 del EDA = gold | OK |
| CA-TO-01: conteos y fechas Go/Python | OK |
| CA-TO-01: gold ≤ silver | OK |
| CA-TO-01: F1-M0 del EDA = gold | OK |
| CA-VE-01: conteos y fechas Go/Python | OK |
| CA-VE-01: gold ≤ silver | OK |
| CA-VE-01: F1-M0 del EDA = gold | OK |
| CA-VMP-1: conteos y fechas Go/Python | OK |
| CA-VMP-1: gold ≤ silver | OK |
| CA-VMP-10: conteos y fechas Go/Python | OK |
| CA-VMP-10: gold ≤ silver | OK |
| CA-VMP-2: conteos y fechas Go/Python | OK |
| CA-VMP-2: gold ≤ silver | OK |
| CA-VMP-2A: conteos y fechas Go/Python | OK |
| CA-VMP-2A: gold ≤ silver | OK |
| CA-VMP-2A: F1-M0 del EDA = gold | OK |
| CA-VMP-6: conteos y fechas Go/Python | OK |
| CA-VMP-6: gold ≤ silver | OK |
| CA-VMP-7: conteos y fechas Go/Python | OK |
| CA-VMP-7: gold ≤ silver | OK |
| CA-VMP-8: conteos y fechas Go/Python | OK |
| CA-VMP-8: gold ≤ silver | OK |
| CA-VMP-9: conteos y fechas Go/Python | OK |
| CA-VMP-9: gold ≤ silver | OK |

## Completitud por estación

Rango: horas entre la primera y última fila silver, ambos extremos incluidos. No equivale al calendario de monitoreo activo.

| Estación | Silver | Gold | Horas del rango | Gold/silver (%) | Gold/rango (%) |
|:---|---:|---:|---:|---:|---:|
| CA-01 | 0 | 0 | — | — | — |
| CA-03 | 6048 | 0 | 6048 | 0.0 | 0.0 |
| CA-AYAS-01 | 7412 | 7382 | 9283 | 99.6 | 79.5 |
| CA-CC-01 | 35064 | 0 | 35064 | 0.0 | 0.0 |
| CA-CG-01 | 1752 | 1061 | 1752 | 60.6 | 60.6 |
| CA-CG-02 | 6611 | 6159 | 7704 | 93.2 | 79.9 |
| CA-CH-03 | 15816 | 1201 | 17544 | 7.6 | 6.8 |
| CA-COLQ-01 | 696 | 558 | 696 | 80.2 | 80.2 |
| CA-CP-01 | 17016 | 16689 | 17016 | 98.1 | 98.1 |
| CA-ESP-01 | 1128 | 1087 | 1128 | 96.4 | 96.4 |
| CA-HU-04 | 528 | 254 | 528 | 48.1 | 48.1 |
| CA-HU-09 | 432 | 431 | 432 | 99.8 | 99.8 |
| CA-HUA-01 | 8212 | 7448 | 9318 | 90.7 | 79.9 |
| CA-HUAR-01 | 624 | 282 | 648 | 45.2 | 43.5 |
| CA-JUPR-01 | 5859 | 5797 | 5880 | 98.9 | 98.6 |
| CA-MAC-01 | 8247 | 8233 | 9290 | 99.8 | 88.6 |
| CA-ROM-01 | 8069 | 8049 | 9456 | 99.8 | 85.1 |
| CA-RZC-01 | 3931 | 0 | 4023 | 0.0 | 0.0 |
| CA-RZC-02 | 3676 | 245 | 4068 | 6.7 | 6.0 |
| CA-SB-01 | 35064 | 0 | 35064 | 0.0 | 0.0 |
| CA-SOC-01 | 696 | 696 | 696 | 100.0 | 100.0 |
| CA-TA-01 | 33024 | 0 | 33024 | 0.0 | 0.0 |
| CA-TAL-01 | 600 | 301 | 600 | 50.2 | 50.2 |
| CA-TO-01 | 25512 | 0 | 25512 | 0.0 | 0.0 |
| CA-VE-01 | 9744 | 0 | 9744 | 0.0 | 0.0 |
| CA-VMP-1 | 0 | 0 | — | — | — |
| CA-VMP-10 | 0 | 0 | — | — | — |
| CA-VMP-2 | 0 | 0 | — | — | — |
| CA-VMP-2A | 240 | 0 | 240 | 0.0 | 0.0 |
| CA-VMP-6 | 0 | 0 | — | — | — |
| CA-VMP-7 | 0 | 0 | — | — | — |
| CA-VMP-8 | 0 | 0 | — | — | — |
| CA-VMP-9 | 0 | 0 | — | — | — |

Las estaciones sin silver no tienen denominador temporal calculable en este reporte.
Una coincidencia entre reportes guardados demuestra consistencia, no una nueva auditoría independiente del CSV.
