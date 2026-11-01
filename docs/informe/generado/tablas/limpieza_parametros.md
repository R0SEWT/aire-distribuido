<!-- Generado por tablas.py desde generado/limpieza.json. No editar a mano. -->

::: {.callout-warning title="PLACEHOLDER"}
Cifras de prueba: `generado/limpieza.json` sale de correr los comandos de Go sobre la muestra sintética data/sample/oefa_muestra.csv (estaciones SINT-*, una semana), no sobre el CSV de OEFA. Se reemplaza cuando `cmd/cargar` corra sobre el CSV completo.
:::

::: {#tbl-limpieza-parametros tbl-colwidths="[35,15,50]"}
| Parámetro en el archivo | Lecturas | Estado |
|:---|---:|:---|
| PM10 | 663 | mapeado |
| Humedad Relativa | 650 | mapeado |
| Temperatura | 650 | mapeado |
| PM2.5 | 645 | mapeado |
| Velocidad del Viento | 636 | mapeado |
| CO | 336 | mapeado |
| NO2 | 336 | mapeado |
| SO2 | 312 | mapeado |
| Benceno | 5 | sin mapeo: se descarta (regla `parametro`) |

Parámetros vistos en el archivo y si tienen mapeo a un parámetro canónico
:::

*Nota.* Fuente: `generado/limpieza.json`, escrito por `cmd/cargar` (el JSON no registra commit ni fecha). Nombres tal como vienen en el CSV, con su frecuencia. Periodos vistos: Horario 4 218; Diario 14; Mensual 1. Parámetros canónicos presentes en la tabla ancha: CO, Humedad, NO2, PM10, PM2,5, SO2, Temp., Viento.
