<!-- Generado por tablas.py desde generado/regimenes.json. No editar a mano. -->

::: {#tbl-regimenes tbl-colwidths="[9,8,8,13,13,13,13,10,13]"}
| Régimen | Filas | % | PM10 | Fracción fina | Viento | Humedad | PM2,5 medio | ECA |
|---:|---:|---:|---:|---:|---:|---:|---:|:---|
| 0 | 6 227 | 14,8 % | 80,1 | 0,24 | 1,8 | 35,8 | 17,8 | no excede |
| 1 | 7 004 | 16,7 % | 37,5 | 0,42 | 4,2 | 55,2 | 14,4 | no excede |
| 2 | 7 845 | 18,7 % | 23,9 | 0,45 | 1,5 | 52,3 | 10,8 | no excede |
| 3 | 7 647 | 18,2 % | 6,3 | 0,80 | 1,4 | 74,1 | 5,1 | no excede |
| 4 | 13 268 | 31,6 % | 28,8 | 0,73 | 1,6 | 83,2 | 20,6 | no excede |

Perfil de cada régimen y su etiqueta frente al ECA
:::

*Nota.* Fuente: `generado/regimenes.json`, escrito por `cmd/entrenar` (commit `53858a0`, 2026-11-01, máquina A019803). Unidades: PM10 en µg/m³; Viento en m/s; Humedad en %. Perfil: media aritmética de cada feature en unidades originales sobre las lecturas del régimen (no es el centroide des-estandarizado). PM2,5 medio derivado como `fraccion_fina*pm10` lectura por lectura; PM2,5 y PM10 en µg/m³. Etiqueta ECA (DS 003-2017-MINAM, valores de 24 horas): excede si el PM2,5 medio supera 50 µg/m³ o el PM10 medio supera 100 µg/m³; no evaluable si falta el contaminante que podría superarlo. El ECA es un promedio de 24 horas: una media horaria alta no es por sí sola un incumplimiento.
