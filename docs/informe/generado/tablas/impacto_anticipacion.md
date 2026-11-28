<!-- Generado por tablas.py desde reports/impacto.json. No editar a mano. -->

::: {#tbl-impacto-anticipacion tbl-colwidths="[24,12,14,11,9,18,12]"}
| Señal | Episodios en que se encendió antes | Horas antes: mediana (Q1 a Q3) | Mínimo a máximo | En el tope de 23 h | Horas seguidas justo antes: mediana (Q1 a Q3) | Máximo seguido |
|:---|---:|---:|---:|---:|---:|---:|
| Una lectura horaria sobre el umbral del ECA | 30 de 30 | 20,0 (16,2 a 22,0) | 4 a 23 | 6 | 1,0 (0,0 a 3,8) | 16 |
| Una lectura en el régimen que excede el ECA (régimen 1) | 30 de 30 | 18,0 (9,5 a 20,0) | 3 a 23 | 3 | 0,5 (0,0 a 3,0) | 15 |

Con cuánta anticipación se encienden las dos señales antes de que la media de 24 horas cruce el ECA, sobre los episodios del gold
:::

*Nota.* Fuente: `reports/impacto.json` (`anticipacion`), escrito por `scripts/impacto.py`. Un episodio empieza en la hora t en que la media de 24 horas de una estación pasa a superar el ECA sin haberlo superado en las 24 horas anteriores (30 episodios, de 44 cruces hacia arriba). «Horas antes» cuenta desde la primera vez que la señal se enciende dentro de la ventana de esa media y tiene un tope de 23 horas por construcción; «horas seguidas» es la cota baja: cuántas horas seguidas, justo antes de t, estuvo encendida. Con solo los 28 episodios cuya hora anterior ya tenía media, las medianas de «horas antes» son 20,5 y 19,0. Es descriptivo, sobre las mismas lecturas del entrenamiento: no es un pronóstico.
