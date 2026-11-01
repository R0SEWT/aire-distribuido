<!-- Generado por tablas.py desde generado/limpieza.json. No editar a mano. -->

::: {#tbl-limpieza-ceros tbl-colwidths="[40,30,30]"}
| Parámetro | Lecturas descartadas | % de la regla |
|:---|---:|---:|
| SO2 | 168 912 | 22,1 % |
| H2S | 137 088 | 18,0 % |
| CO | 107 448 | 14,1 % |
| O3 | 100 944 | 13,2 % |
| PM10 | 62 400 | 8,2 % |
| Humedad | 42 918 | 5,6 % |
| Dir. viento | 40 182 | 5,3 % |
| Viento | 40 182 | 5,3 % |
| Temp. | 39 822 | 5,2 % |
| PM2,5 | 22 368 | 2,9 % |
| Radiación | 480 | 0,1 % |

Días enteros en cero descartados, por parámetro
:::

*Nota.* Fuente: `generado/limpieza.json`, escrito por `cmd/cargar` (commit `53858a0`, 2026-11-01, máquina A019803). Un día completo de un parámetro con todas sus lecturas en 0 exacto es un equipo que no midió, no un día de aire limpio: se descarta entero (regla `bloque_en_cero`).
