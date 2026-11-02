<!-- Generado por tablas.py desde configs/limpieza.json. No editar a mano. -->

::: {#tbl-umbrales tbl-colwidths="[13,13,12,26,11,12,13]"}
| Parámetro | Rango físico | Unidad canónica | Otras unidades (factor) | Negativos | Día en 0 = sin dato | Hora en 0 = sin dato |
|:---|:---|:---|:---|:---:|:---:|:---:|
| PM2,5 | [0; 1 000] | µg/m³ | — | no | sí | sí |
| PM10 | [0; 2 000] | µg/m³ | — | no | sí | sí |
| Viento | [0; 60] | m/s | — | no | sí | no |
| H2S | [0; 5 000] | µg/m³ | ppb × 1,39 | no | sí | sí |
| Dir. viento | [0; 360] | ° | — | no | sí | no |
| Humedad | [0; 100] | % | — | no | sí | no |
| Temp. | [−30; 50] | °C | — | sí | sí | no |
| Presión | [400; 1 100] | hPa | mmHg × 1,33322 | no | sí | no |
| Precip. | [0; 200] | mm | — | no | no | no |
| Radiación | [0; 1 500] | W/m² | — | no | sí | no |
| NO2 | [0; 2 000] | µg/m³ | ppb × 1,88 | no | sí | no |
| SO2 | [0; 10 000] | µg/m³ | ppb × 2,62 | no | sí | sí |
| CO | [0; 50 000] | µg/m³ | mg/m³ × 1 000; ppb × 1,145 | no | sí | sí |
| O3 | [0; 1 000] | µg/m³ | ppb × 1,96 | no | sí | sí |

Umbrales de la limpieza por parámetro (configs/limpieza.json)
:::

*Nota.* Fuente: `configs/limpieza.json`, que `cmd/cargar` lee al arrancar (los cambios no tocan el código). Centinelas del equipo: −999, −9 999, 9 999, 99 999. Periodos horarios: «1 hora»; también «-» si el día tiene sus 24 filas. Valores vacíos: «-», «NA» y la celda vacía. Fechas válidas: desde el 2000-01-01 hasta antes del 2026-07-01. Hora por posición: sí, convención «fin». Hora con minutos: «descartar». El rango se compara en la unidad canónica, después de convertir.
