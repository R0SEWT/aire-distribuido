<!-- Generado por tablas.py desde generado/regimenes.json. No editar a mano. -->

::: {.callout-warning title="PLACEHOLDER"}
Cifras de prueba: `generado/regimenes.json` sale de correr los comandos de Go sobre la muestra sintética data/sample/oefa_muestra.csv (estaciones SINT-*, una semana), no sobre el CSV de OEFA. Se reemplaza cuando `cmd/entrenar` corra sobre el CSV completo.
:::

::: {#tbl-regimenes tbl-colwidths="[9,8,8,13,13,13,13,10,13]"}
| Régimen | Filas | % | PM10 | Fracción fina | Viento | Humedad | PM2,5 medio | ECA |
|---:|---:|---:|---:|---:|---:|---:|---:|:---|
| 0 | 125 | 20,3 % | 28,3 | 0,54 | 2,2 | 81,6 | 15,4 | no excede |
| 1 | 110 | 17,8 % | 67,4 | 0,43 | 2,3 | 84,4 | 29,0 | no excede |
| 2 | 142 | 23,0 % | 58,6 | 0,54 | 1,4 | 52,4 | 31,7 | no excede |
| 3 | 123 | 19,9 % | 66,4 | 0,63 | 3,4 | 80,9 | 42,3 | no excede |
| 4 | 117 | 19,0 % | 51,9 | 0,47 | 3,4 | 53,9 | 24,2 | no excede |

Perfil de cada régimen y su etiqueta frente al ECA
:::

*Nota.* Fuente: `generado/regimenes.json`, escrito por `cmd/entrenar` (commit `94d4037`, 2026-11-01, máquina A019803). Unidades: PM10 en µg/m³; Viento en m/s; Humedad en %. Perfil: media aritmética de cada feature en unidades originales sobre las lecturas del régimen (no es el centroide des-estandarizado). PM2,5 medio derivado como `fraccion_fina*pm10` lectura por lectura; PM2,5 y PM10 en µg/m³. Etiqueta ECA (DS 003-2017-MINAM, valores de 24 horas): excede si el PM2,5 medio supera 50 µg/m³ o el PM10 medio supera 100 µg/m³; no evaluable si falta el contaminante que podría superarlo. El ECA es un promedio de 24 horas: una media horaria alta no es por sí sola un incumplimiento.
