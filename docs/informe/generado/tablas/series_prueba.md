<!-- Generado por tablas.py desde web/public/datos/series.json. No editar a mano. -->

::: {#tbl-series tbl-colwidths="[21,14,18,9,25,13]"}
| Caso | Estación | Tramo | Lecturas | Al final: media de PM2,5 / PM10 (µg/m³), alerta e INCA | Primera alerta |
|:---|:---|:---|---:|:---|:---|
| Escenario: muchas estaciones a la vez | 14 estaciones | 2021-12-14 a 2021-12-16 (48 h) | 667 | 2 estaciones en alerta al final | — |
| Día limpio | CA-HUA-01 | 2021-12-19 a 2021-12-21 | 48 | 10,4 / 18,2 (24 lect.); alerta no; INCA buena | no se enciende |
| Día limpio | CA-JUPR-01 | 2022-05-02 a 2022-05-04 | 48 | 4,2 / 9,7 (24 lect.); alerta no; INCA buena | no se enciende |
| INCA mala sin alerta | CA-SOC-01 | 2021-12-18 a 2021-12-20 | 48 | 25,1 / 74,5 (24 lect.); alerta no; INCA mala | no se enciende |
| Cruza el ECA | CA-ESP-01 | 2021-12-20 a 2021-12-22 | 48 | 22,6 / 131,9 (24 lect.); alerta sí; INCA moderada | 2021-12-21 07:00 |
| Cruza el ECA | CA-HU-09 | 2021-12-20 a 2021-12-22 | 48 | 61,7 / 89,9 (24 lect.); alerta sí; INCA mala | 2021-12-21 06:00 |
| En alerta todo el tramo | CA-HU-04 | 2021-12-18 a 2021-12-20 | 48 | 84,7 / 114,6 (24 lect.); alerta sí; INCA mala | 2021-12-18 18:00 |
| Umbral de cuidado | CA-RZC-02 | 2021-12-20 a 2021-12-22 | 47 | 131,6 / 205,0 (24 lect.); alerta sí; INCA umbral de cuidado | 2021-12-20 19:00 |
| Con huecos y con media | CA-CG-02 | 2022-06-01 a 2022-06-03 | 43 | 22,8 / 38,9 (19 lect.); alerta no; INCA moderada | no se enciende |
| Con huecos y sin media al final | CA-MAC-01 | 2022-05-25 a 2022-05-27 | 41 | sin media (17 lecturas); alerta no; INCA sin banda | no se enciende |

Datos de prueba del TB2: el escenario y los días sueltos, con lo que la API tiene que responder al final de cada tramo
:::

*Nota.* Fuente: `web/public/datos/series.json`, escrito por `scripts/impacto.py` el 2026-11-21 desde el silver y las asignaciones del gold. Las lecturas son reales (OEFA). Lo esperado se calculó con la regla de `ventana/ventana.go` sin consultar a la API; el régimen esperado es el del modelo sembrado. El escenario se carga primero, sobre un sistema limpio; los días de una estación son posteriores a su tramo del escenario y están a más de 48 horas entre sí.
