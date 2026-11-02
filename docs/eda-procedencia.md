# Procedencia del dato de OEFA: cómo se recolectó, por qué faltan valores y dónde están las estaciones

> Complemento del EDA (`reports/eda_criba.md`) pedido en la revisión del 01/11. Todas las cifras salen de
> `notebooks/04_procedencia_y_mapa.ipynb` ejecutado sobre el bronze real (sha256 `1f09a806…f26d92`) y las
> salidas de Go del 01/11 (`reports/limpieza.json`, silver, gold y `reports/regimenes.json`). Productos:
> `reports/estaciones_ficha.csv`, `reports/mapa_estaciones.html`, `reports/faltantes_*.csv` y las figuras
> `mapa.png`, `calendario.png` y `faltantes.png` en `docs/informe/generado/figuras/`.

Cada afirmación lleva su origen:

- **[dato]**: lo calculó el notebook 04 sobre el CSV.
- **[OEFA]**: lo dicen el diccionario (`1c_*.xlsx`), la ficha de metadatos (`1b_*.docx`) o un informe de OEFA citado.
- **[norma]**: lo dice una norma peruana citada.
- **[inferencia]**: es lectura nuestra, a validar.

## 1. Lo más importante, en cinco cifras

1. **El 0 quiere decir «sin dato»**, también dentro de un día. Cuando el promedio de 24 h que publica
   OEFA tiene menos de 18 horas no nulas, vale 0 (90–98 % de los días); con 18 o más, es la media de las
   horas **no nulas** (89–91 %) [dato]. Es la regla del 75 % (18 de 24 h) que OEFA declara en sus
   informes [OEFA]. Hasta 2021, el 28–44 % de los días-parámetro horarios son días enteros en cero
   (812 746 lecturas) [dato].
2. **El hueco cambió de forma en 2022.** Hasta 2021 un hueco es un día en 0; desde enero de 2022, una
   **fila vacía** (todos los parámetros en «-»). Las 274 866 filas sin valor son en un 99.94 % de 2022, de
   las 9 estaciones con datos de 2022; ese año no hay ni un día en cero [dato].
3. **La hora falta por evaluación, no por periodo ni por parámetro.** Solo la exportación de la evaluación
   de Agroaurora (CA-HUA-01 y CA-MAC-01) trae `HORA_INICIO`, en todos sus parámetros; las otras 18
   evaluaciones, incluidas las de 2022, no la traen [dato]. Donde hay hora, coincide con la posición en el
   bloque en el **100 %** de 165 912 filas, incluidas 53 328 filas de días con huecos; y la posición reproduce, hora
   por hora, un corte de luz que documenta un informe de OEFA en CA-AYAS-01 [dato + OEFA].
4. **Los «duplicados» casi nunca lo son.** El 54.7 % de las filas (1 577 173) son textualmente idénticas a
   otra, porque sin hora una fila no tiene clave; con la hora reconstruida no queda **ningún** duplicado de
   (estación, parámetro, fecha-hora) [dato]. Lo que sí se repite es la misma lectura en dos unidades:
   133 809 filas (gases en ppb y µg/m³; presión en mmHg y hPa), con cociente 1.000 tras convertir [dato].
5. **No es una red, son 19 evaluaciones de actividades fiscalizadas.** 33 estaciones en 10 de las 25
   regiones: 6 series de años (entre 2018 y 2021), 10 series de meses (casi todas 2022), 9 campañas de semanas
   (14 estaciones arrancan en noviembre–diciembre de 2021) y 8 puntos de **muestreo manual** de 24 h en
   Ventanilla–Mi Perú y Chimbote [dato + OEFA]. El gold que ve el K-Means tiene 17 estaciones y es en un
   61 % de 2022 [dato].

## 2. Cómo se recolectó el dato

### 2.1 Qué es una Evaluación Ambiental de Seguimiento

- **[OEFA]** La ficha de metadatos dice que el conjunto reúne las **Evaluaciones Ambientales de
  Seguimiento (EAS)** que la Dirección de Evaluación Ambiental (DEAM) captura, sistematiza, valida y
  visualiza, y que «actualmente» los datos provienen de **34 estaciones de monitoreo en tiempo real**.
  Entidad: OEFA; fuente: DEAM; fecha de creación y última actualización: 2024-04-29; frecuencia anual;
  cobertura declarada **«Perú, 2018-2021»**. El portal de datos abiertos repite la descripción y hoy marca
  el recurso como modificado el 2026-06-15 (publicado el 2024-04-30) [1].
- **[norma]** El Reglamento de Evaluación del OEFA (RCD N.° 00013-2020-OEFA/CD, El Peruano, 19/07/2020)
  define la EAS (art. 19) como una evaluación que se hace mediante «intervenciones periódicas o continuas»
  para observar el comportamiento de los componentes ambientales en el tiempo mediante la **vigilancia**,
  con el fin de **alertar impactos negativos**; se prioriza según el Planefa (art. 20) y, si detecta
  anomalías, las reporta a la autoridad competente sin necesidad de informe (art. 21). Define vigilancia
  (art. 5 q) como el monitoreo periódico o continuo del estado de los componentes ambientales [2]. El
  reglamento fue modificado en 2024 (RCD 00002-2024-OEFA/CD, arts. 5, 9, 13 y 22–25); no verifiqué el
  texto vigente del art. 19.
- **[OEFA]** Antes de ese reglamento los informes llamaban a lo mismo **«vigilancia ambiental»**: el
  INFORME N.° 00231-2019-OEFA/DEAM-STEC (Ventanilla–Mi Perú, agosto de 2019) declara «Tipo de evaluación:
  Vigilancia Ambiental» en el marco del Planefa 2019 [4]. Eso explica el nombre del dataset («Vigilancia y
  Seguimiento») [inferencia].
- **[OEFA]** Cada EAS de aire produce un reporte mensual con objetivo, protocolo, estaciones, equipos,
  ECA de comparación, resultados de 24 h y alertas. Ejemplo: el REPORTE N.° 00011-2021-OEFA/DEAM-STEC
  (UM Antamina, diciembre de 2021, estación CA-AYAS-01) [3].

### 2.2 «En el ámbito de influencia de» una actividad fiscalizada

- **[dato]** Cada estación pertenece a la evaluación de una actividad: `NOMBRE_EVALUACION` tiene 19
  valores, todos «Calidad del aire en el ámbito / área de influencia de …»: unidades mineras (Antamina,
  Colquijirca, Cerro de Pasco, Cuajone), metalurgia (complejo metalúrgico La Oroya, refinería de zinc
  Cajamarquilla), la Refinería Talara, agroindustria (Casa Grande), Agroaurora (el nombre no dice el
  rubro), unidades «de producción» o «fiscalizables» (Cerro Verde, Quellaveco), el corredor vial
  Apurímac–Cusco, zonas industriales (Gran Trapecio en Chimbote, Ventanilla–Mi Perú) y la zona ladrillera
  de Huachipa. La ficha (`objeto_fiscalizado`, `tipo_actividad`) lo extrae del nombre, sin adivinar el
  rubro cuando el nombre no lo dice.
- **[dato]** `DESCRIPCION_UBICACION` sitúa el punto respecto de la fuente: «a 700 m del Complejo
  Metalúrgico de La Oroya», «a 750 m del ingreso de la refinería de zinc Cajamarquilla», «a 100 m de los
  stock piles» (Paragsha), «a 200 m del borde del Tajo Norte» (Colquijirca), «a 400 m de la Planta Casa
  Grande», «estación a sotavento» / «a barlovento» (Casa Grande, Cerro Verde).
- **[OEFA]** El informe de Ventanilla–Mi Perú ubica 3 estaciones a sotavento de la zona industrial y 1 a
  barlovento [4]; el de Champamarca (Simón Bolívar, Pasco) dice que el monitoreo empezó por un pedido de la
  autoridad de supervisión tras quejas de la población por polvo [5].
- **[inferencia] Sesgo**: las estaciones están **donde hay una actividad fiscalizada y, dentro de su
  zona, a favor del viento**, y la vigilancia se activa por prioridad del Planefa o por denuncias. No es una
  muestra del aire del país ni de la exposición de su población: sobrerrepresenta sitios contaminados
  (minas, fundiciones, refinerías, puertos industriales) y los periodos en que había un motivo para medir.
  Cubre 10 de las 25 regiones; ninguna estación está en la Amazonía, y 10 de las 33 están por encima de los
  3 000 m [dato]. Una zona sin estación no es una zona con aire limpio.

### 2.3 Método: equipos automáticos y muestreo manual

- **[OEFA]** Las estaciones de la EAS son de **monitoreo continuo**. En CA-AYAS-01 el PM10 y el PM2.5 se
  miden con un monitor automático de partículas GRIMM EDM 180 (dispersión de luz), bajo el Protocolo
  Nacional de Monitoreo de la Calidad Ambiental del Aire (DS 010-2019-MINAM) [3].
- **[OEFA]** Las estaciones **CA-VMP** de Ventanilla–Mi Perú no son automáticas: en 2019 el PM10 se tomaba
  con un muestreador **de alto volumen** (Thermo Scientific G10557) y el PM2.5 con uno **de bajo volumen**
  (BGI PQ200), con pesada del filtro en laboratorio (gravimetría), y la meteorología con una estación
  Davis Vantage Pro 2; el protocolo citado era el de DIGESA (RD 1404-2005-DIGESA) [4]. Lo mismo, con
  Hi-Vol, en Champamarca en 2018–2019 (estación CA-SB-02, que no está en el dataset) [5].
- **[dato]** Eso calza con lo que traen: solo promedios de 24 h de PM10 (y algo de PM2.5), un día cada 1–5
  días, nunca en cero (sección 3.6).
- **[norma]** Los métodos de referencia del ECA son gravimetría para PM, fluorescencia UV para SO₂ y H₂S,
  infrarrojo no dispersivo para CO y fotometría UV para O₃ [6]. El equipo concreto de cada estación no
  está en el dataset; solo lo vi en el informe de CA-AYAS-01.

### 2.4 Qué dicen las columnas de contexto

| Columna | Diccionario de OEFA | Lo que trae el dato |
|---|---|---|
| `COMPONENTE_AMBIENTAL` | componente evaluado | «Aire» en todas las filas |
| `PROCEDENCIA_MUESTRA` | «ambiente donde el dato es capturado» | «Aire ambiental» con tres grafías (2 694 888 / 132 980 / 56 009): aire del ambiente, no de chimenea [inferencia] |
| `TIPO_MUESTRA` | «tipo de muestra utilizado» | «Simple» (2 883 863) y «NA» (14, todas de CA-01). El diccionario no define «Simple»; parece la muestra puntual por oposición a compuesta, y no distingue nada [inferencia] |
| `TIPO_ANALISIS` | «Meteorológicos, material particulado y gases» | «Datos Meteorológicos», «Concentración de partículas en PM10/PM2,5», «Material particulado» (PM10 de CA-SB-01) y «-» en los gases (954 324). Solo identifica el parámetro en PM |
| `PERIODO` | «intervalo en que se recopilan los datos»: 1 hora, móvil de 3 y 8 h, 24 h | «1 hora», «-» (1 127 412, casi toda la meteorología: no figura en el diccionario), «24 horas»/«24 Horas», «3 horas móviles»/«Móvil 3 horas», «Móvil 8 horas»/«Media móvil de 8 horas» |
| `FECHA_INICIO`, `HORA_INICIO` | inicio del monitoreo; formato `aaaammdd` y `HH:MM` | fecha `aaaa-mm-dd`; hora `NA` en el 94 % y, donde existe, `1899-12-31 HH:MM:SS` (artefacto de Excel) |
| `FECHA_FIN`, `HORA_FIN` | fin del monitoreo | `NA` en el 100 % |
| `ZONA`, `DATUM` | zona UTM y datum | WGS84; zonas 17, 18 y 19. CA-HUAR-01 trae 19 y es 18 (sección 4) |
| `FECHA_CORTE` | «día en que se generó el dataset», `aaaammdd` | 20240429 en todas las filas |

- **[dato]** La cobertura declarada (2018–2021) se queda corta: 644 707 filas (22.4 %) son de 2022.
- **[dato]** El dataset dice 34 estaciones y trae 33 `NOMBRE_PUNTO`; CA-03 y CA-CH-03 tienen las mismas
  coordenadas y descripción y no se solapan en el tiempo (2019 frente a 2020–2021): parecen el mismo sitio
  con dos códigos [inferencia], de modo que serían 32 sitios. No sé cuál es la estación 34.

### 2.5 Las normas que explican los periodos

- **[norma] ECA de aire, DS 003-2017-MINAM** (El Peruano, 07/06/2017) [6]: PM2.5 50 µg/m³ en 24 h
  y 25 anual; PM10 100 en 24 h y 50 anual (ambos: no exceder más de 7 veces al año); SO₂ 250 en 24 h; H₂S
  150 en 24 h; CO 10 000 en 8 h (media móvil) y 30 000 en 1 h; O₃ 100 en 8 h; NO₂ 200 en 1 h. Son
  **promedios**: una hora alta no es un incumplimiento.
- **[norma] Niveles de estado de alerta, DS 009-2003-SA** (modificado por DS 012-2005-SA) [8]: PM10 en
  promedio de 24 h, **SO₂ en promedio móvil de 3 h**, **CO en promedio móvil de 8 h** y H₂S en promedio de
  24 h. Son exactamente los periodos no horarios del CSV (3 h móviles solo en SO₂, 8 h móviles solo en CO)
  [dato]: OEFA publica, además de la serie horaria, los promedios con los que compara contra el ECA y
  contra los niveles de alerta [inferencia]. No verifiqué si este decreto sigue vigente o fue sustituido.
- **[OEFA + norma] Protocolo Nacional de Monitoreo, DS 010-2019-MINAM** (publicado el 02/12/2019) [7]. No
  pude leer el texto del protocolo (el PDF de 18 MB excede la herramienta de lectura). El criterio de
  completitud lo tomo del informe de OEFA que lo aplica: «75 % de 24 horas = 18 horas» para comparar con el
  ECA, y los días que no lo cumplen se marcan «ID» (información insuficiente) [3]. En el CSV esos días
  llevan 0 (sección 3.5).

## 3. Faltantes y duplicados: por qué

Cada fila larga se etiquetó con la primera regla de la cascada de Go que la descarta; la réplica cuadra
con `reports/limpieza.json` regla por regla (1 423 958 válidas, 762 744 `bloque_en_cero`, 262 743
`parametro`, 218 998 `periodo`, 133 809 `duplicado_unidad`, 51 219 `fuera_de_rango`, 30 401 `hora`) [dato].
Agrupadas por causa (`faltantes.png`, panel a; `reports/faltantes_por_estacion.csv`):

| Destino | Filas | % |
|---|---|---|
| válida | 1 423 958 | 49.4 |
| día en cero (incluye 50 002 de presión en 0 que Go cuenta como fuera de rango) | 812 746 | 28.2 |
| fila sin valor | 274 866 | 9.5 |
| promedio no horario (24 h, móviles) | 195 835 | 6.8 |
| duplicado por unidad | 133 809 | 4.6 |
| otra regla (día incompleto o intercalado, fuera de rango, fecha) | 42 663 | 1.5 |

### 3.1 (a) La hora `NA`: es por evaluación

- **[dato]** No es por periodo: antes de 2021-12 ninguna fila trae hora. Pero tampoco es «desde 2021-12»:
  desde ese mes, fuera de CA-HUA-01 y CA-MAC-01 solo el 0.008 % de las filas trae hora (46 de 592 392, en 19
  estaciones), aunque Antamina, Cajamarquilla y Casa Grande también se exportaron con la plantilla de 2022.
- **[dato]** Tampoco es por parámetro: en las dos estaciones de Agroaurora traen hora todos (PM, presión,
  humedad, temperatura, viento, precipitación). Es **por exportación**: la de Agroaurora conservó la hora
  y las otras 18 no la traen. De las 47 filas con «hora» de CA-AYAS-01, 46 son promedios de 24 h con
  `1899-12-31` solo y 1 es una fila de presión sin fecha.
- **[dato]** El formato es el de una celda de hora de Excel: `1899-12-31 HH:MM:SS` (159 454 filas) o
  `1899-12-31` solo (8 469), que es la medianoche: en los bloques completos está en la posición 0 el 100 %
  de las veces.
- **[dato]** La reconstrucción por posición es correcta: donde hay hora, la hora de **inicio** es la
  posición en el bloque en el 100 % de 112 584 filas de bloques completos, y en el 100 % de 165 912 filas
  si la posición se cuenta dentro del **bloque físico** (las 24 filas del día, con o sin valor).
- **[dato + OEFA] Control externo.** El reporte de Antamina dice que en CA-AYAS-01 el monitoreo empezó el
  9/12/2021 a las 14:35 y que hubo un corte de luz del 10/12 15:45 al 11/12 15:05 [3]. En el CSV (sin hora)
  el 9/12 tiene 10 filas (14:00–23:00), el 10/12 queda en 0 desde la posición 16 y el 11/12 vuelve en la
  posición 15: la hora por posición reproduce el corte al minuto de la hora. Los promedios de 24 h del 12 al
  19/12 que publica el reporte (PM2.5 6.5, 9.3, 7.1…; PM10 11.7, 18.6, 13.3…) son idénticos a los del CSV, y
  los días 9–11, «ID» en el reporte, valen 0 en el CSV.
- **[inferencia]** OEFA sistematiza cada EAS por separado (la exportación de cada evaluación tiene su
  formato: por día o por serie, con o sin hora, con o sin filas vacías) y el dataset las concatena: el
  archivo son 46 tramos contiguos de las 19 evaluaciones (CA-SB-01 alterna sus dos nombres de evaluación
  parámetro por parámetro).

### 3.2 (b) Los días enteros en cero: tramos largos de equipo sin medir

- **[dato]** 34 136 bloques horarios (estación, parámetro, día) en cero: 818 313 lecturas. Por largo, en
  días-parámetro: los tramos de más de 7 días son el **92.4 %**; los parámetros que una estación «trae» pero
  nunca midió, el 5.5 %; los días sueltos, el 0.3 % (59 días). `reports/faltantes_tramos_en_cero.csv` tiene
  los 391 tramos con inicio, fin y tipo.
- **[dato] Parámetros nunca medidos, rellenados con 0**: PM10 y PM2.5 en CA-03 y CA-RZC-01; toda la
  meteorología en CA-COLQ-01, CA-ESP-01, CA-HU-04, CA-HU-09 y CA-HUAR-01. La plantilla tiene una columna
  para cada parámetro aunque la estación no tenga el sensor.
- **[dato] Equipo apagado**: cuando cae la meteorología, caen juntas temperatura, humedad y viento el
  92.1 % de las veces (un datalogger). Hay días con la estación entera en cero: CA-TA-01 741 de 1 376 días,
  CA-CH-03 196 de 731, CA-TO-01 57, CA-SB-01 47, CA-03 41, CA-CC-01 29. En CA-TA-01 (Talara) el PM10 está
  en cero el 89 % de los días y el CO el 97 %.
- **[dato] Inicio y fin de campaña**: 12 805 de los 21 718 días en tramos (59 %) están en tramos que tocan
  el primer o el último día de la serie del parámetro: el equipo se instala antes de medir o se apaga antes
  de que termine la exportación.
- **[dato] Por año**: días-parámetro horarios en cero 27.6 % (2018), 43.6 % (2019), 41.1 % (2020), 31.3 %
  (2021) y **0 % en 2022**, cuando el 39.7 % viene como fila vacía.
- **[dato] Ceros dentro de días válidos**: la regla `bloque_en_cero` solo saca días enteros. Quedan 3 551
  lecturas válidas de PM en 0 exacto, en 1 141 corridas de horas (mediana 1 h, máximo 37 h), como el corte
  de CA-AYAS-01; en el gold hay **413 horas con PM2.5 = PM10 = 0** (0.64 %). Por la regla de las 18 horas
  (3.5), OEFA las trata como «sin dato».

### 3.3 (c) Las 274 866 filas sin ningún valor: huecos de la plantilla de 2022

- **[dato]** 274 713 (99.94 %) son de 2022 y de las 9 estaciones con datos de 2022 (AYAS 62 629,
  RZC-02 61 339, RZC-01 58 394, MAC 27 780, ROM 22 322, HUA 21 547, CG-02 10 956, JUPR 8 295, CG-01 1 451);
  las otras 153 son de CA-VMP-2A (2018) y CA-01 (2019). Traen periodo, unidad y tipo de análisis, pero todas
  las columnas de parámetro en «-».
- **[dato]** Su parámetro se infiere por posición (la serie contigua de la misma unidad y periodo) en
  274 723 casos. El 88.8 % de los días-parámetro con alguna fila vacía están vacíos entero: CA-AYAS-01 no
  tiene meteorología en el 90 % de sus días; CA-RZC-01 y CA-RZC-02 pierden meteorología y SO₂ el 62–68 % de
  sus días, desde enero de 2022; CA-MAC-01 y CA-HUA-01 tienen la presión vacía el 44 % y el 28 % de sus días
  (`reports/faltantes_sin_valor.csv`).
- **[inferencia]** Son la versión 2022 del día en cero: la plantilla genera una fila por parámetro, unidad y
  hora, con o sin medición. Para el modelo da lo mismo; para la limpieza no (3.4).

### 3.4 Lo que la regla `hora` tira por culpa de esos huecos

- **[dato]** Go cuenta la posición dentro de las filas **con valor** de un bloque y exige 24 contiguas. En la
  plantilla de 2022 un hueco es una fila vacía que ocupa su hora, así que un día con 23 lecturas y 1 vacía
  se pierde entero: **19 710 lecturas** (RZC-02 5 674, RZC-01 4 286, CG-02 2 984, ROM-01 2 930, AYAS-01
  1 920, JUPR-01 1 577…). Contando la posición en el bloque físico se recuperan, y esa posición está
  validada al 100 % con las horas de Agroaurora, incluidas 53 328 filas de días con huecos.
- **[dato]** Un tercer formato: CA-CH-03 del 21/08 al 31/10/2020 (72 días) viene **hora a hora** (todos los
  parámetros de la hora 0, luego los de la hora 1…). Cada bloque tiene sus 24 filas en orden pero no
  contiguas, y Go descarta 20 736 lecturas (10 368 por `hora` y 10 368 de meteorología «-» por `periodo`).
  Recuperarlas exige aceptar 24 filas en orden no contiguas; no hay horas explícitas para validarlo.
- El resto de `hora` (323 filas) son días que empiezan o terminan a media jornada: bien descartados.

### 3.5 (e) Promedios de 24 h y móviles: vienen además de los horarios

- **[dato]** 30 355 filas de 24 h con valor; el 97.3 % tiene la serie horaria del mismo día. Solo 825 vienen
  sin horario, todas de las 8 estaciones de la sección 3.6.
- **[dato] La regla de las 18 horas** (`faltantes.png`, panel b; `reports/faltantes_promedio_24h.csv`), en
  días con 24 filas horarias y un 24 h publicado:

  | Horas no nulas | Días | 24 h = 0 | 24 h = media de las horas no nulas |
  |---|---|---|---|
  | 0 | 13 924 | 98.1 % | — |
  | 1–17 | 432 | 90–99 % | 0–2 % |
  | 18 | 51 | 5.9 % | 90.2 % |
  | 19–23 | 987 | 1.0 % | 89.2 % |
  | 24 | 13 888 | 0.2 % | 90.8 % |

  Las horas en 0 no entran a la media: para OEFA son faltantes. El ~10 % que no coincide al 1 % no lo
  expliqué (redondeo, horas invalidadas que no vemos u otra ventana).
- **[dato]** Los móviles se calculan de la serie horaria: el SO₂ «3 horas móviles» coincide con la media
  móvil de 3 h de los horarios en el 94–99 % (CA-03, CA-CC-01, CA-CH-03, CA-RZC-01/02); el CO «Móvil 8 horas»
  de CA-TA-01 en el 94 %. El «Móvil 3 horas» de CA-TA-01 solo en el 65 % (sin explicar). No son otra medición
  y Go hace bien en descartarlos (regla `periodo`).

### 3.6 (f) Las 8 estaciones sin ninguna lectura horaria válida: muestreo manual

- **[dato]** CA-VMP-1, -2, -6, -7, -8, -9 y -10 traen solo promedios de 24 h de PM10 (y PM2.5 en algunas),
  entre 2018-03 y 2021-11, un día cada 1 a 5 días, con medianas de PM10 de 69–92 µg/m³ y **ningún cero**.
  CA-01 trae 7 días de PM10 y 5 de PM2.5 de 24 h (abril–mayo de 2019) en las instalaciones del Servicio
  Industrial de la Marina (Chimbote), con `TIPO_MUESTRA` «NA».
- **[OEFA]** Son puntos de muestreo manual (Hi-Vol y bajo volumen, gravimetría) de la vigilancia de
  Ventanilla–Mi Perú [4]. **[inferencia]** CA-01 parece una campaña manual corta del mismo tipo.
- **[dato]** CA-VMP-2A, «a 15 m del patio» del colegio donde está CA-VMP-2, es la única automática del grupo:
  12 días horarios de noviembre de 2018 (PM10, CO, SO₂, H₂S).
- No faltan horas: nunca las hubo. No entran al K-Means (que es horario), pero son las únicas mediciones de
  OEFA en Ventanilla–Mi Perú en tres años.

### 3.7 (d) Duplicados y repeticiones

| Qué se repite | Cuánto | Por qué | ¿Duplicado de lectura? |
|---|---|---|---|
| Filas idénticas en las 36 columnas | 1 577 173 (54.7 %); 1 571 065 sin hora y 984 552 en días en cero o vacíos | sin hora, 24 filas de un día con el mismo valor son el mismo texto | No: con la hora reconstruida hay **0** duplicados de (estación, parámetro, fecha-hora) |
| La misma lectura en ppb y µg/m³ | 98 687 filas en ppb (SO₂, H₂S, CO, O₃; 16 combinaciones estación-gas vienen en las dos unidades, 3 solo en µg/m³) | OEFA publica las dos unidades | Sí; cociente µg/ppb convertido 1.000 (CO 1.004). Go conserva µg/m³ |
| Presión en mmHg y hPa | 35 122 filas en mmHg | 8 de las 9 estaciones con datos de 2022 traen las dos (CA-AYAS-01 trae el hPa vacío) | Sí; cociente 1.000. Go conserva hPa |
| Varios `NOMBRE_EVALUACION` por estación | CA-CG-01, CA-ROM-01, CA-RZC-01, CA-RZC-02, CA-SB-01 | el nombre de la evaluación cambia de un año a otro | No se solapan en el tiempo |
| Dos códigos en el mismo sitio | CA-03 (2019) y CA-CH-03 (2020–2021) | mismas coordenadas y descripción | No se solapan [inferencia: renombrada] |
| Grafías | `PERIODO` (8 valores para 5 periodos), `PROCEDENCIA_MUESTRA` (3), `DESCRIPCION_UBICACION` (8 estaciones con 2 redacciones, una con saltos de línea) | carga manual | No afecta lecturas |
| «Más de una ubicación» | CA-AYAS-01 (altitud 3 728 y 3 784), CA-VMP-1, CA-VMP-2, CA-VMP-7 (altitud 0 frente a la real; CA-VMP-2 además con un segundo juego de coordenadas a unos 60 m) | altitud vacía cargada como 0 | No; el informe de 2019 da 50 m a CA-VMP-6, que en el CSV tiene 0 en todas sus filas |

## 4. Dónde están: mapa y ficha

- `reports/mapa_estaciones.html`: Leaflet sobre OpenStreetMap; color por categoría (horarias con PM 22,
  horarias sin PM 3 —CA-03, CA-CC-01, CA-RZC-01—, solo promedios de 24 h 8, sin lecturas válidas 0), tamaño
  por horas válidas; las estaciones a menos de 15 km se juntan en racimos que se abren con un clic
  (Ventanilla–Mi Perú 8, Huachipa–Cajamarquilla 4, Chimbote 3, Casa Grande 3, Pasco 3, Antamina 2,
  Agroaurora 2). Al pasar el mouse: nombre, categoría y horas; al hacer clic: la ficha (evaluación tal como
  viene, actividad, ubicación, altitud, fechas, años con datos, parámetros, % de horas en el régimen alto y
  descripción).
- `mapa.png`: el país con teselas de OpenTopoMap (datos de OpenStreetMap y relieve) y recuadros de
  Ventanilla–Mi Perú y Huachipa–Cajamarquilla. Las teselas de tile.openstreetmap.org rechazan la descarga
  por script (política de uso), por eso el estático usa OpenTopoMap.
- `reports/estaciones_ficha.csv`: 33 filas × 36 columnas (evaluación, objeto fiscalizado, tipo de
  actividad, ubicación, lat/lon, fechas, días y horas válidas, parámetros, % de horas inferidas, % de días
  en cero, % de promedios no horarios, % de filas vacías, PM2.5/PM10/meteorología, filas del gold, % en el
  régimen alto, categoría, tipo de serie, grupo de 15 km, formato de exportación).
- **Corrección declarada**: CA-HUAR-01 trae zona 19 y departamento `NA`. Con zona 19 caería en
  (-10.79, -70.29), en la Amazonía; su descripción la pone en la Comunidad Campesina Huaraucaca, a unos
  300 m de la planta de la UM Colquijirca (Pasco). Con zona 18 queda a 5.7 km de CA-COLQ-01 y a 13.4 km de
  CA-SB-01. La ficha y los
  mapas usan zona 18 y lo anotan en `nota_ubicacion` [dato + inferencia].

## 5. Estructura temporal: qué implica para el modelo y la pregunta

El calendario (`calendario.png`) muestra cuatro tipos de serie [dato]:

| Tipo | Estaciones | Horas válidas | Filas del gold | % del gold |
|---|---|---|---|---|
| serie de años (2018–2021) | CA-CC-01, CA-SB-01, CA-TA-01, CA-TO-01, CA-CH-03, CA-CP-01 | 161 496 | 18 120 | 28.2 |
| serie de meses (casi todas dic-2021 a dic-2022) | AYAS, HUA, MAC, JUPR, RZC-01, RZC-02, CG-02, ROM, CA-03, VE-01 | 65 515 | 41 340 | 64.4 |
| campaña de semanas (casi todas diciembre de 2021) | CG-01, COLQ, ESP, HU-04, HU-09, HUAR, SOC, TAL, VMP-2A | 6 696 | 4 776 | 7.4 |
| muestreo manual de 24 h | CA-01 y 7 CA-VMP | 0 | 0 | 0 |

- **[dato]** 14 estaciones arrancan en noviembre–diciembre de 2021. El gold tiene 8 255 horas de 2020,
  16 868 de 2021 y 39 108 de 2022 (61 %). Las series largas con más horas (La Oroya, Paragsha, Talara,
  Cuajone) no entran al gold porque no miden PM2.5; de las de años solo entra CA-CP-01 (16 896 horas) y algo
  de CA-CH-03.
- **[dato]** El régimen alto (1 515 horas, 2.4 % del gold) sale en un 30 % de tres estaciones de Lima que
  suman 960 horas del gold (CA-HU-04 218, CA-RZC-02 163, CA-HU-09 75): campañas de 2–3 semanas de diciembre
  de 2021. En CA-HU-04 el 82.6 % de sus horas son del régimen alto.
- **[inferencia] Para el modelo**: los regímenes mezclan sitio y estación del año. Un régimen «alto» de
  diciembre en Huachipa y uno de 2022 en Casa Grande no son comparables como «exposición». Sin las horas en
  cero de 2018–2021 y sin las filas vacías de 2022 la muestra ya está filtrada por disponibilidad de equipo,
  y los huecos no son aleatorios: arrancan y terminan con las campañas.
- **[inferencia] Para la pregunta de impacto** (episodios sobre el ECA, dónde y cuándo): el dato sostiene
  comparaciones **dentro** de una estación y entre estaciones de la **misma ventana** (diciembre de 2021 tiene
  14 a la vez), no tendencias multianuales del país ni de una región. Lo que sí permite, y es lo propio de
  OEFA, es describir el aire **junto a la fuente fiscalizada** en el periodo en que se la vigiló.

## 6. Unidad de análisis por capa

| Capa | Unidad | Filas | Estaciones |
|---|---|---|---|
| bronze (fila larga) | una lectura de un parámetro en una estación, un día y una posición (hora), o un promedio de 24 h / móvil | 2 883 877 | 33 |
| bronze válido (Go) | lectura horaria que pasa la cascada | 1 423 958 | 25 |
| silver (fila ancha) | estación × hora, con los parámetros que haya | 233 707 | 25 |
| gold | estación × hora con PM2.5 y PM10 válidos, estandarizada | 64 236 | 17 |

El promedio de 24 h (estación × día) es otra unidad: es la que compara el ECA y la única que existe en las
8 estaciones manuales.

## 7. Tres recomendaciones para el informe de la PC3

1. **Declarar la unidad y el alcance con cifras.** En la sección de datos, la tabla de la sección 6 y una
   frase: el K-Means ve 64 236 horas de 17 estaciones, el 61 % de 2022, y no ve las 8 estaciones de muestreo
   manual (Ventanilla–Mi Perú y CA-01) ni las series largas sin PM2.5 (La Oroya, Paragsha, Talara, Cuajone).
   Poner el calendario junto al mapa.
2. **Justificar las reglas de limpieza con la evidencia del dato y de OEFA.** La hora por posición,
   validada al 100 % (165 912 filas) y contra el corte de luz documentado en el REPORTE 00011-2021; el 0
   como «sin dato», con la regla de las 18 horas, que explica `bloque_en_cero`; y el cambio de codificación
   de 2022 (fila vacía). Declarar lo que la limpieza actual todavía deja pasar o tira: 413 horas del gold con
   PM2.5 = PM10 = 0, que deberían tratarse como faltantes (es lo que hace OEFA), y 19 710 lecturas de 2022
   que se recuperan contando la posición en el bloque físico (cambio de Go, fuera de este análisis).
3. **Acotar la pregunta de impacto y el mapa.** Presentar el % de régimen alto de Lima (CA-HU-04 82.6 %,
   CA-RZC-02 61.7 %) como lo que es, campañas de tres semanas de diciembre de 2021, no exposición anual;
   comparar estaciones solo en ventanas comunes (diciembre de 2021); y acompañar todo mapa de «zonas de
   riesgo» con la advertencia de cobertura: 10 de 25 regiones, estaciones puestas junto a una fuente y a
   sotavento, sin estaciones en la Amazonía.

## 8. Lo que no pude verificar

- El texto del Protocolo DS 010-2019-MINAM (no lo pude abrir): el criterio del 75 % = 18 h lo tomo del
  reporte de OEFA que lo aplica y de lo que muestra el dato.
- Qué significa `TIPO_MUESTRA` «Simple»: el diccionario no lo define.
- Cuál es la estación 34 de la descripción, y si CA-03 y CA-CH-03 son de verdad el mismo equipo.
- El rubro de Agroaurora y de las unidades de Cerro Verde y Quellaveco: el nombre de la evaluación no lo dice
  y no lo busqué.
- Por qué la exportación de Agroaurora es la única con hora, y por qué CA-CH-03 viene hora a hora en
  agosto–octubre de 2020 (hipótesis: cada EAS se sistematiza con su propia plantilla).
- El ~10 % de días cuyo 24 h no coincide al 1 % con la media de las horas no nulas, y el 65 % del «Móvil
  3 horas» de CA-TA-01.
- El equipo de cada estación automática fuera de CA-AYAS-01 (no está en el dataset; habría que leer el
  reporte de cada EAS).
- Si el art. 19 del Reglamento de Evaluación cambió con la RCD 00002-2024-OEFA/CD, y si el DS 009-2003-SA
  sigue vigente.

## Fuentes

1. Plataforma Nacional de Datos Abiertos, *Vigilancia y Seguimiento ambiental en la calidad del aire —
   OEFA* (consultado el 01/11/2026):
   https://www.datosabiertos.gob.pe/dataset/vigilancia-y-seguimiento-ambiental-en-la-calidad-del-aire-organismo-de-evaluaci%C3%B3n-y
   — y la ficha de metadatos y el diccionario que vienen con el CSV (`data/bronze/1b_*`, `1c_*`).
2. OEFA, Resolución del Consejo Directivo N.° 00013-2020-OEFA/CD, *Reglamento de Evaluación del OEFA*, El
   Peruano, 19/07/2020 (arts. 5, 19–21). Copia en FAOLEX: https://faolex.fao.org/docs/pdf/per200329.pdf
3. OEFA, REPORTE N.° 00011-2021-OEFA/DEAM-STEC, *Evaluación ambiental de seguimiento de la calidad del aire
   en el ámbito de la unidad minera Antamina … diciembre de 2021*, 29/12/2021:
   https://repositorio.oefa.gob.pe/server/api/core/bitstreams/af0b76ae-19ef-411b-b837-1e8ae2d04d58/content
4. OEFA, INFORME N.° 00231-2019-OEFA/DEAM-STEC, *Vigilancia ambiental de la calidad del aire en el ámbito
   de la Zona Industrial de Ventanilla y del distrito Mi Perú — agosto 2019*, 30/09/2019:
   https://repositorio.oefa.gob.pe/bitstreams/5d72d42f-a405-4f78-b6ae-79f04fc6d2aa/download
5. OEFA, INFORME N.° 00267-2019-OEFA/DEAM-STEC, *Vigilancia ambiental de la calidad del aire realizada en
   el centro poblado Champamarca, distrito de Simón Bolívar … 2019*, 29/10/2019:
   https://repositorio.oefa.gob.pe/bitstream/handle/20.500.12788/214/Informe_%2000267-2019-OEFA-DEAM-STEC.pdf?sequence=1
6. Decreto Supremo N.° 003-2017-MINAM, *Estándares de Calidad Ambiental (ECA) para Aire*, El Peruano,
   07/06/2017 (anexo). Copia en FAOLEX: https://faolex.fao.org/docs/pdf/per171711.pdf
7. Decreto Supremo N.° 010-2019-MINAM, *Protocolo Nacional de Monitoreo de la Calidad Ambiental del Aire*,
   02/12/2019 (ficha en SINIA; el PDF del protocolo no se leyó):
   https://sinia.minam.gob.pe/normas/decreto-supremo-que-aprueba-protocolo-nacional-monitoreo-calidad
8. Decreto Supremo N.° 009-2003-SA, *Reglamento de los Niveles de Estados de Alerta Nacionales para
   Contaminantes del Aire*, art. 3 modificado por el DS 012-2005-SA (texto en InfoAire, MINAM):
   https://infoaireperu.minam.gob.pe/wp-content/uploads/2017/12/DS_009-2003-SA_Reglamento_Niveles_de_Estados_de_Alerta-1.pdf

*Este análisis se preparó con asistencia de IA (código del notebook, búsqueda y lectura de fuentes,
redacción); las cifras salen del notebook y conviene que el equipo valide las lecturas marcadas como
inferencia.*
