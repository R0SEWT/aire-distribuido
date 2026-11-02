<!-- Generado por tablas.py desde generado/estaciones_ficha.csv. No editar a mano. -->

::: {#tbl-tipologia tbl-colwidths="[16,10,38,12,12,12]"}
| Tipo | Estaciones | Cuáles | Lecturas válidas | Filas del gold | % del gold |
|:---|---:|:---|---:|---:|---:|
| serie de años | 6 | CA-CC-01, CA-CH-03, CA-CP-01, CA-SB-01, CA-TA-01, CA-TO-01 | 980 762 | 17 890 | 27,2 % |
| serie de meses | 10 | CA-03, CA-AYAS-01, CA-CG-02, CA-HUA-01, CA-JUPR-01, CA-MAC-01, CA-ROM-01, CA-RZC-01, CA-RZC-02, CA-VE-01 | 425 510 | 43 313 | 65,8 % |
| campaña de semanas | 9 | CA-CG-01, CA-COLQ-01, CA-ESP-01, CA-HU-04, CA-HU-09, CA-HUAR-01, CA-SOC-01, CA-TAL-01, CA-VMP-2A | 28 226 | 4 670 | 7,1 % |
| muestreo manual de 24 h | 8 | CA-01, CA-VMP-1, CA-VMP-10, CA-VMP-2, CA-VMP-6, CA-VMP-7, CA-VMP-8, CA-VMP-9 | 0 | 0 | 0,0 % |

Tipos de serie: cuánto pesa cada uno en lo que ve el modelo
:::

*Nota.* Fuente: la clasificación, de `reports/estaciones_ficha.csv` (notebook 04, procedencia del dato); las lecturas y el gold, de `generado/limpieza.json` (`cmd/cargar`). Las series de años cubren varios años de una estación; las de meses, casi todas de diciembre de 2021 a diciembre de 2022; las campañas, unas semanas; los puntos manuales solo traen promedios de 24 h, que el K-Means (horario) no usa.
