<!-- Generado por tablas.py desde generado/regimenes.json. No editar a mano. -->

::: {.callout-warning title="PLACEHOLDER"}
Cifras de ejemplo: `generado/regimenes.json` todavía es el archivo de muestra. Se reemplaza cuando `cmd/entrenar` escriba el real.
:::

::: {#tbl-regimenes tbl-colwidths="[17,11,9,7,7,7,7,7,7,11,10]"}
| Régimen | Filas | % | PM2,5 | PM10 | Fracción fina | Viento | Humedad | Temp. | ECA | INCA |
|:---|---:|---:|---:|---:|---:|---:|---:|---:|:---|:---|
| 0. Fondo con viento | 92 000 | 34,3 % | 9,9 | 22,0 | 0,45 | 3,6 | 66,6 | 19,8 | no excede | buena |
| 1. Humedad nocturna | 71 500 | 26,6 % | 17,2 | 32,5 | 0,53 | 1,4 | 87,9 | 14,1 | no excede | buena |
| 2. Polvo diurno | 52 300 | 19,5 % | 16,6 | 51,8 | 0,32 | 2,9 | 63,6 | 21,5 | no excede | buena |
| 3. Combustión en calma | 38 100 | 14,2 % | 48,1 | 72,9 | 0,66 | 1,1 | 79,3 | 15,2 | no excede | moderada |
| 4. Episodio de polvo | 14 500 | 5,4 % | 31,4 | 120,6 | 0,26 | 2,6 | 58,4 | 22,7 | excede PM10 | moderada |

Perfil de cada régimen y su etiqueta frente al ECA
:::

*Nota.* Fuente: `generado/regimenes.json`, escrito por `cmd/entrenar` (commit `PLACEHOLDER`, 2026-11-04). Unidades: PM2,5 en µg/m³; PM10 en µg/m³; Viento en m/s; Humedad en %; Temp. en °C. Perfil = media de las lecturas del régimen. Un régimen «excede» si su PM2,5 medio supera 50 µg/m³ o su PM10 medio supera 100 µg/m³ (DS 003-2017-MINAM). El ECA es un promedio de 24 horas: una lectura horaria alta no es por sí sola un incumplimiento.
