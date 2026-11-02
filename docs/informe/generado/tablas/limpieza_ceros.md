<!-- Generado por tablas.py desde generado/limpieza.json. No editar a mano. -->

::: {#tbl-limpieza-ceros tbl-colwidths="[34,33,33]"}
| Parámetro | Días enteros en 0 (lecturas) | Horas en 0 en días con datos |
|:---|---:|---:|
| SO2 | 168 912 | 3 083 |
| H2S | 137 088 | 1 237 |
| CO | 107 448 | 256 |
| O3 | 100 944 | 567 |
| PM10 | 62 400 | 3 136 |
| Humedad | 42 918 | 0 |
| Dir. viento | 40 182 | 0 |
| Viento | 40 182 | 0 |
| Temp. | 39 822 | 0 |
| PM2,5 | 22 368 | 415 |
| Radiación | 480 | 0 |

Ceros descartados por parámetro: días enteros y horas sueltas
:::

*Nota.* Fuente: `generado/limpieza.json`, escrito por `cmd/cargar` (commit `6fb3b00`, 2026-11-01, máquina A019803). Un día completo de un parámetro con todas sus lecturas en 0 exacto es un equipo que no midió (regla `bloque_en_cero`); en los días con datos, un 0 exacto de un contaminante es una hora sin dato (regla `cero_como_faltante`), como lo trata OEFA. La meteorología no: un viento de 0 m/s es calma, y una hora seca tiene precipitación 0.
