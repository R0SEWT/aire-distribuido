<!-- Generado por tablas.py desde generado/regimenes.json. No editar a mano. -->

::: {#tbl-regimenes tbl-colwidths="[9,9,10,9,19,19,11,14]"}
| Régimen | Nivel | Filas | % | PM2,5 | PM10 | PM2,5/PM10 | ECA |
|---:|:---|---:|---:|---:|---:|---:|:---|
| 0 | bajo | 47 070 | 73,3 % | 9,0 | 17,5 | 0,51 | no excede |
| 1 | medio | 15 651 | 24,4 % | 24,3 | 58,4 | 0,42 | no excede |
| 2 | alto | 1 515 | 2,4 % | 65,3 | 187,4 | 0,35 | excede (PM2,5, PM10) |

Perfil de cada régimen y su etiqueta frente al ECA
:::

*Nota.* Fuente: `generado/regimenes.json`, escrito por `cmd/entrenar` (commit `11cef5a`, 2026-11-01, máquina A019803). Unidades: PM2,5 en µg/m³; PM10 en µg/m³. Perfil: media aritmética de cada feature en unidades originales sobre las lecturas del régimen (no es el centroide des-estandarizado). PM2,5 medio derivado como `pm2_5` lectura por lectura; PM2,5 y PM10 en µg/m³. Etiqueta ECA (DS 003-2017-MINAM, valores de 24 horas): excede si el PM2,5 medio supera 50 µg/m³ o el PM10 medio supera 100 µg/m³; no evaluable si falta el contaminante que podría superarlo. El ECA es un promedio de 24 horas: una media horaria alta no es por sí sola un incumplimiento.
