<!-- Generado por tablas.py desde generado/regimenes.json. No editar a mano. -->

::: {.callout-warning title="PLACEHOLDER"}
Cifras de prueba: `generado/regimenes.json` sale de correr los comandos de Go sobre la muestra sintética data/sample/oefa_muestra.csv (estaciones SINT-*, una semana), no sobre el CSV de OEFA. Se reemplaza cuando `cmd/entrenar` corra sobre el CSV completo.
:::

::: {#tbl-regimenes-estacion tbl-colwidths="[12,66,22]"}
| Régimen | Estaciones principales (proporción del régimen) | Estaciones donde aparece |
|---:|:---|---:|
| 0 | SINT-D Ucayali (62 %); SINT-C Junín (19 %); SINT-B Arequipa (16 %) | 4 |
| 1 | SINT-A Lima Norte (44 %); SINT-B Arequipa (35 %); SINT-C Junín (16 %) | 4 |
| 2 | SINT-B Arequipa (30 %); SINT-A Lima Norte (27 %); SINT-C Junín (23 %) | 4 |
| 3 | SINT-A Lima Norte (43 %); SINT-B Arequipa (27 %); SINT-C Junín (24 %) | 4 |
| 4 | SINT-D Ucayali (31 %); SINT-B Arequipa (27 %); SINT-C Junín (23 %) | 4 |

Estaciones donde más aparece cada régimen
:::

*Nota.* Fuente: `generado/regimenes.json`, escrito por `cmd/entrenar` (commit `94d4037`, 2026-11-01, máquina A019803). Las tres estaciones con más filas de cada régimen. Que un régimen se concentre en una estación no dice que esa estación tenga peor aire que una zona sin estación.
