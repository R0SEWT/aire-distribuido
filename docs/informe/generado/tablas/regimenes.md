<!-- Generado por tablas.py desde generado/regimenes.json. No editar a mano. -->

::: {.callout-warning title="PLACEHOLDER"}
Cifras de prueba: `generado/regimenes.json` sale de correr los comandos de Go sobre la muestra sintética data/sample/oefa_muestra.csv (estaciones SINT-*, una semana), no sobre el CSV de OEFA. Se reemplaza cuando `cmd/entrenar` corra sobre el CSV completo.
:::

::: {#tbl-regimenes tbl-colwidths="[9,8,8,13,13,13,13,10,13]"}
| Régimen | Filas | % | PM10 | Fracción fina | Viento | Humedad | PM2,5 medio | ECA |
|---:|---:|---:|---:|---:|---:|---:|---:|:---|
| 0 | 121 | 19,6 % | 29,5 | 0,53 | 1,8 | 79,5 | 15,8 | no excede |
| 1 | 140 | 22,7 % | 58,6 | 0,62 | 3,6 | 82,3 | 36,5 | no excede |
| 2 | 111 | 18,0 % | 45,9 | 0,44 | 3,4 | 58,2 | 20,2 | no excede |
| 3 | 151 | 24,5 % | 64,5 | 0,55 | 1,7 | 51,5 | 35,7 | no excede |
| 4 | 94 | 15,2 % | 73,6 | 0,43 | 2,0 | 82,9 | 31,9 | no excede |

Perfil de cada régimen y su etiqueta frente al ECA
:::

*Nota.* Fuente: `generado/regimenes.json`, escrito por `cmd/entrenar` (commit `abf3e7f`, 2026-11-01, máquina A019803). Unidades: PM10 en µg/m³; Viento en m/s; Humedad en %. Perfil: media aritmética de cada feature en unidades originales sobre las lecturas del régimen (no es el centroide des-estandarizado). PM2,5 medio derivado como `fraccion_fina*pm10` lectura por lectura; PM2,5 y PM10 en µg/m³. Etiqueta ECA (DS 003-2017-MINAM, valores de 24 horas): excede si el PM2,5 medio supera 50 µg/m³ o el PM10 medio supera 100 µg/m³; no evaluable si falta el contaminante que podría superarlo. El ECA es un promedio de 24 horas: una media horaria alta no es por sí sola un incumplimiento.
