<!-- Generado por tablas.py desde generado/limpieza.json. No editar a mano. -->

::: {.callout-warning title="PLACEHOLDER"}
Cifras de prueba: `generado/limpieza.json` sale de correr los comandos de Go sobre la muestra sintética data/sample/oefa_muestra.csv (estaciones SINT-*, una semana), no sobre el CSV de OEFA. Se reemplaza cuando `cmd/cargar` corra sobre el CSV completo.
:::

::: {#tbl-limpieza-unidades tbl-colwidths="[25,75]"}
| Parámetro | Unidades vistas (lecturas) |
|:---|:---|
| CO | ug/m3 (168); mg/m3 (167); ppm (1) |
| Humedad | % (650) |
| NO2 | ug/m3 (335); ppb (1) |
| PM10 | ug/m3 (663) |
| PM2,5 | ug/m3 (645) |
| SO2 | ug/m3 (312) |
| Temp. | °C (650) |
| Viento | m/s (636) |

Unidades vistas por parámetro canónico
:::

*Nota.* Fuente: `generado/limpieza.json`, escrito por `cmd/cargar` (commit `e1cef10`, 2026-11-01, máquina A019803). Las unidades con factor en `configs/limpieza.json` se convierten a la canónica (regla 6); las que no figuran se descartan (regla `unidad`).
