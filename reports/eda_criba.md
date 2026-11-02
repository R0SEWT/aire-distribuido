# Criba de preguntas candidatas — EDA del bronze de OEFA

> Generado por `notebooks/01_eda_criba.ipynb` el 2026-11-02 00:15. Bronze: `1a_Vigilancia y Seguimiento ambiental en la calidad del aire.csv` (1 317 949 839 bytes, sha256 `1f09a8064ee0aed0b3a9a0a90b96f63ce226b45a969dace49f73e45134f26d92`). Config de limpieza: configs/limpieza.json.

Criterios y umbrales en la celda `U` del notebook; ficha de las candidatas en `docs/specs/2026-10-31-preguntas-candidatas.md`.

## Resumen

| Q | veredicto | cifra |
|---|---|---|
| Q1 | dudoso | 2 de 25 estaciones con PM y viento+humedad en >= 75 % de las horas y al menos un año completo (5 sin exigir el año); 9 estaciones con >= 0.5 % de horas de 24 h sobre el ECA; la hora explica en mediana 27 % de la varianza de log PM |
| Q2 | pasa | 17 de 25 estaciones miden PM2.5 y PM10 a la vez en >= 50 % de sus horas con PM; PM2.5 > PM10 en 0.00 % de las horas con ambas |
| Q3 | pasa | 9 estaciones con >= 0.5 % de horas de 24 h sobre el ECA (máximo 100.00 %); 109 episodios de 24 h; en mediana 8.5 % de las horas caen en huecos de más de 24 h |
| Q4 | dudoso | 64 % de las estaciones con PM tienen PM en >= 75 % de sus horas; diferencia p90–p10 de completitud entre estaciones: 86 puntos |
| Q5 | pasa | 45 parejas de estaciones a menos de 15 km (3 con >= 90 días de PM en ambas); grupos: [['CA-VMP-1', 'CA-VMP-10', 'CA-VMP-2', 'CA-VMP-2A', 'CA-VMP-6', 'CA-VMP-7', 'CA-VMP-8', 'CA-VMP-9'], ['CA-HU-04', 'CA-HU-09', 'CA-RZC-01', 'CA-RZC-02'], ['CA-01', 'CA-03', 'CA-CH-03'], ['CA-CG-01', 'CA-CG-02', 'CA-ROM-01'], ['CA-COLQ-01', 'CA-HUAR-01', 'CA-SB-01'], ['CA-AYAS-01', 'CA-JUPR-01'], ['CA-HUA-01', 'CA-MAC-01']] |
| Q6 | pasa | 6 de 25 estaciones con algún gas en >= 50 % de sus horas (gases medidos: ['so2', 'co', 'o3', 'h2s']); con firma horaria o semanal: ['CA-CC-01 (so2, η² hora 0.12, semana 0.00)', 'CA-RZC-01 (so2, η² hora 0.24, semana 0.00)', 'CA-RZC-02 (so2, η² hora 0.38, semana 0.01)', 'CA-VMP-2A (h2s, η² hora 0.34, semana 0.21)'] |
| Q7 | pasa | 33 de 33 estaciones con coordenadas; cubren 10 de las 25 regiones (Áncash, Arequipa, Callao, Cusco, Junín, La Libertad, Lima, Moquegua, Pasco, Piura). La población por distrito (INEI) es externa |
| Q8 | dudoso | 12 estaciones de OEFA en Lima o Callao (CA-HU-04, CA-HU-09, CA-RZC-01, CA-RZC-02, CA-VMP-1, CA-VMP-10, CA-VMP-2, CA-VMP-2A, CA-VMP-6, CA-VMP-7, CA-VMP-8, CA-VMP-9); falta SENAMHI (va al TB2) |
| Q9 | no pasa | 281 huecos de >= 3 h; mediana de horas en huecos 10.9 %; PM de las 6 h previas / resto = 0.91 (IC 95 % 0.82–1.00); estaciones con efecto claro (IC > 1.1): ninguna; el PM previo es MENOR que el resto: los huecos vienen tras horas de PM bajo, no tras episodios |

**Lectura automática (a validar el martes con el equipo)**: base = Q3 apoyada en Q1; diferencial = Q5; Q7 y Q9 entran como advertencias. Lo que no pasó va al informe como «preguntas que este dato no puede responder».

Datos: 2 883 877 filas largas leídas, 1 434 498 válidas, 236 001 filas anchas, 25 estaciones con datos válidos, de 2018-01-01 a 2023-01-01.

## Sorpresas del dato

- **Ceros de relleno.** Días enteros en 0 exacto (equipo sin medir) en casi todos los parámetros: la regla `bloque_en_cero` descarta 762 744 lecturas (co 107 448, h2s 137 088, humedad 42 918, o3 100 944, pm10 62 400, pm2_5 22 368, radiacion 480, so2 168 912, temperatura 39 822, viento_direccion 40 182, viento_velocidad 40 182). Sin ella, PM10 tendría un 35 % de ceros y CO un 92 %. Y las horas en 0 sueltas de los contaminantes, dentro de días con datos, también son «sin dato»: `cero_como_faltante` descarta 8 694 (co 256, h2s 1 237, o3 567, pm10 3 136, pm2_5 415, so2 3 083). Las dos están en el cargador de Go.
- **Filas vacías de 2022.** Desde 2022 lo que falta llega como una fila sin ningún valor que ocupa su hora: `sin_valor` descarta 262 743 filas, y el bloque físico (filas contiguas con las vacías) le da la hora a 19 710 lecturas de días que, sin contar las vacías, no tenían 24 filas; la regla `hora` queda en 10 691.
- **La hora es casi toda inferida**: 93 % de las lecturas del silver tienen la hora reconstruida por posición; en las filas que traen hora, la hora de inicio es la posición en el bloque en 99.6 % (y en el 100.0 % de las que están en bloques completos).
- **Estaciones sin PM horario en el silver**: CA-03, CA-CC-01, CA-RZC-01 (CA-CC-01, el complejo de La Oroya, no trae PM en el dataset: solo SO2 y meteorología).**Estaciones sin ninguna lectura horaria válida** (8 de 33): CA-01, CA-VMP-1, CA-VMP-10, CA-VMP-2, CA-VMP-6, CA-VMP-7, CA-VMP-8, CA-VMP-9 (solo traen promedios de 24 horas o bloques en cero).
- **Gases en dos unidades**: 134 285 lecturas en ppb tienen su gemela en µg/m³; el cociente confirma los factores a 25 °C (CO da 1.004: OEFA usa un factor 0.4 % mayor).
- **100 registros con un salto de línea dentro de un campo** (DESCRIPCION_UBICACION de CA-VMP-1, periodo 24 horas): el divisor de Go por líneas los rechaza.
- **Ubicación inconsistente**: CA-AYAS-01, CA-VMP-1, CA-VMP-2, CA-VMP-7 tienen más de un juego de coordenadas; hay filas con departamento «NA»; algunos UBIGEO perdieron el cero inicial (021801 → 21801).
- **Ciclo diario no creíble** en CA-03, CA-TA-01: máximo de temperatura antes del mediodía o mínimo cerca de la medianoche; su meteorología horaria no sirve como feature tal como viene.

## Q1. ¿Cuándo y dónde se concentran los episodios sobre el ECA, y con qué meteorología llegan?

**Veredicto: dudoso.** 2 de 25 estaciones con PM y viento+humedad en >= 75 % de las horas y al menos un año completo (5 sin exigir el año); 9 estaciones con >= 0.5 % de horas de 24 h sobre el ECA; la hora explica en mediana 27 % de la varianza de log PM.

Criterio: pasa con >= 7 estaciones con PM, meteo y un año completo, episodios y η² hora < 0.5.

## Q2. ¿De qué está hecho cada episodio: combustión o polvo?

**Veredicto: pasa.** 17 de 25 estaciones miden PM2.5 y PM10 a la vez en >= 50 % de sus horas con PM; PM2.5 > PM10 en 0.00 % de las horas con ambas.

Criterio: pasa con >= 7 estaciones simultáneas y violaciones <= 5 %.

## Q3. ¿Con cuánta anticipación se puede avisar?

**Veredicto: pasa.** 9 estaciones con >= 0.5 % de horas de 24 h sobre el ECA (máximo 100.00 %); 109 episodios de 24 h; en mediana 8.5 % de las horas caen en huecos de más de 24 h.

Criterio: no pasa si ninguna estación llega a 0.5 %; pasa con >= 30 episodios y huecos largos <= 10 %; revisar con Q9.

## Q4. ¿Cuánto del año respira fuera del ECA la población cercana a cada estación?

**Veredicto: dudoso.** 64 % de las estaciones con PM tienen PM en >= 75 % de sus horas; diferencia p90–p10 de completitud entre estaciones: 86 puntos.

Criterio: pasa si >= 75 % de las estaciones tienen completitud buena y la dispersión p90–p10 <= 20 puntos.

## Q5. ¿Los episodios son regionales o locales?

**Veredicto: pasa.** 45 parejas de estaciones a menos de 15 km (3 con >= 90 días de PM en ambas); grupos: [['CA-VMP-1', 'CA-VMP-10', 'CA-VMP-2', 'CA-VMP-2A', 'CA-VMP-6', 'CA-VMP-7', 'CA-VMP-8', 'CA-VMP-9'], ['CA-HU-04', 'CA-HU-09', 'CA-RZC-01', 'CA-RZC-02'], ['CA-01', 'CA-03', 'CA-CH-03'], ['CA-CG-01', 'CA-CG-02', 'CA-ROM-01'], ['CA-COLQ-01', 'CA-HUAR-01', 'CA-SB-01'], ['CA-AYAS-01', 'CA-JUPR-01'], ['CA-HUA-01', 'CA-MAC-01']].

Criterio: pasa con >= 2 parejas a < 15 km y >= 90 días de solape; no pasa sin parejas.

## Q6. ¿Los gases tienen la firma horaria de una fuente industrial?

**Veredicto: pasa.** 6 de 25 estaciones con algún gas en >= 50 % de sus horas (gases medidos: ['so2', 'co', 'o3', 'h2s']); con firma horaria o semanal: ['CA-CC-01 (so2, η² hora 0.12, semana 0.00)', 'CA-RZC-01 (so2, η² hora 0.24, semana 0.00)', 'CA-RZC-02 (so2, η² hora 0.38, semana 0.01)', 'CA-VMP-2A (h2s, η² hora 0.34, semana 0.21)'].

Criterio: pasa con >= 2 estaciones con gas y η² hora >= 0.05 o semana >= 0.05; no pasa sin gases.

## Q7. ¿Qué parte del país tiene una estación cerca?

**Veredicto: pasa.** 33 de 33 estaciones con coordenadas; cubren 10 de las 25 regiones (Áncash, Arequipa, Callao, Cusco, Junín, La Libertad, Lima, Moquegua, Pasco, Piura). La población por distrito (INEI) es externa.

Criterio: siempre entra, como advertencia: una zona sin estación no es una zona con aire limpio.

## Q8. ¿Las estaciones de OEFA ven lo mismo que las de SENAMHI en Lima?

**Veredicto: dudoso.** 12 estaciones de OEFA en Lima o Callao (CA-HU-04, CA-HU-09, CA-RZC-01, CA-RZC-02, CA-VMP-1, CA-VMP-10, CA-VMP-2, CA-VMP-2A, CA-VMP-6, CA-VMP-7, CA-VMP-8, CA-VMP-9); falta SENAMHI (va al TB2).

Criterio: necesita las estaciones de SENAMHI a pocos km: se decide en el TB2.

## Q9. ¿Los huecos del dato coinciden con los episodios?

**Veredicto: no pasa.** 281 huecos de >= 3 h; mediana de horas en huecos 10.9 %; PM de las 6 h previas / resto = 0.91 (IC 95 % 0.82–1.00); estaciones con efecto claro (IC > 1.1): ninguna; el PM previo es MENOR que el resto: los huecos vienen tras horas de PM bajo, no tras episodios.

Criterio: pasa si hay >= 20 huecos y >= 1 % de horas en huecos, y el PM previo supera al resto (IC > 1.1) en conjunto o en >= 2 estaciones con >= 10 huecos; no pasa si el PM previo no se distingue del resto (huecos aleatorios); dudoso si hay efecto pero los huecos son pocos (entonces es una línea del informe, no una pregunta).

## Mediciones

### Rango de fechas y años completos

| estacion | inicio | fin | horas_rango | horas_con_pm | n_anios_completos | cobertura_por_anio |
|---|---|---|---|---|---|---|
| CA-03 | 2019-04-24 | 2020-01-01 | 6 048 | 0 | 0 |  |
| CA-AYAS-01 | 2021-12-10 | 2022-12-31 | 9 283 | 7 382 | 1 | 2021: 6 %, 2022: 79 % |
| CA-CC-01 | 2018-01-01 | 2022-01-01 | 35 064 | 0 | 0 |  |
| CA-CG-01 | 2021-12-03 | 2022-02-14 | 1 752 | 1 061 | 0 | 2021: 8 %, 2022: 4 % |
| CA-CG-02 | 2022-02-14 | 2023-01-01 | 7 704 | 6 159 | 0 | 2022: 70 %, 2023: 0 % |
| CA-CH-03 | 2020-01-01 | 2022-01-01 | 17 544 | 1 201 | 0 | 2021: 14 %, 2022: 0 % |
| CA-COLQ-01 | 2021-12-03 | 2022-01-01 | 696 | 558 | 0 | 2021: 6 %, 2022: 0 % |
| CA-CP-01 | 2020-01-23 | 2022-01-01 | 17 016 | 16 692 | 2 | 2020: 93 %, 2021: 97 %, 2022: 0 % |
| CA-ESP-01 | 2021-11-15 | 2022-01-01 | 1 128 | 1 088 | 0 | 2021: 12 % |
| CA-HU-04 | 2021-12-10 | 2022-01-01 | 528 | 254 | 0 | 2021: 3 % |
| CA-HU-09 | 2021-12-14 | 2022-01-01 | 432 | 431 | 0 | 2021: 5 %, 2022: 0 % |
| CA-HUA-01 | 2021-12-08 | 2023-01-01 | 9 318 | 7 448 | 1 | 2021: 6 %, 2022: 79 %, 2023: 0 % |
| CA-HUAR-01 | 2021-12-05 | 2022-01-01 | 648 | 282 | 0 | 2021: 3 % |
| CA-JUPR-01 | 2022-05-01 | 2023-01-01 | 5 880 | 5 798 | 0 | 2022: 66 %, 2023: 0 % |
| CA-MAC-01 | 2021-12-09 | 2023-01-01 | 9 290 | 8 233 | 1 | 2021: 6 %, 2022: 88 %, 2023: 0 % |
| CA-ROM-01 | 2021-12-03 | 2023-01-01 | 9 456 | 8 049 | 1 | 2021: 8 %, 2022: 84 %, 2023: 0 % |
| CA-RZC-01 | 2021-12-14 | 2022-05-30 | 4 023 | 0 | 0 |  |
| CA-RZC-02 | 2021-12-12 | 2022-05-30 | 4 068 | 245 | 0 | 2021: 3 % |
| CA-SB-01 | 2018-01-01 | 2022-01-01 | 35 064 | 26 029 | 3 | 2018: 22 %, 2019: 94 %, 2020: 92 %, 2021: 89 %, 2022: 0 % |
| CA-SOC-01 | 2021-12-03 | 2022-01-01 | 696 | 696 | 0 | 2021: 8 %, 2022: 0 % |
| CA-TA-01 | 2018-03-27 | 2022-01-01 | 33 024 | 3 520 | 0 | 2018: 36 %, 2019: 4 % |
| CA-TAL-01 | 2021-12-07 | 2022-01-01 | 600 | 301 | 0 | 2021: 3 % |
| CA-TO-01 | 2018-03-04 | 2021-01-30 | 25 512 | 22 853 | 2 | 2018: 71 %, 2019: 96 %, 2020: 86 %, 2021: 8 % |
| CA-VE-01 | 2019-09-22 | 2020-11-01 | 9 744 | 7 445 | 0 | 2019: 25 %, 2020: 60 % |
| CA-VMP-2A | 2018-11-20 | 2018-11-30 | 240 | 240 | 0 | 2018: 3 % |

### Completitud estación × parámetro (% de las horas del rango)

Tabla para decidir las features: se prefiere menos features con más estaciones.

| estacion | co | h2s | humedad | o3 | pm10 | pm2_5 | precipitacion | presion | radiacion | so2 | temperatura | viento_direccion | viento_velocidad | pm_alguna | pm_ambas | F1-M0 | F2-M0 | F2-M1 | F2-M2 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| CA-03 | 0.0 | 73.5 | 79.0 | 0.0 | 0.0 | 0.0 | 100.0 | 77.8 | 0.0 | 69.2 | 79.0 | 79.0 | 79.0 | 0.0 | 0.0 | 0.0 | 0.0 | 0.0 | 0.0 |
| CA-AYAS-01 | 0.0 | 0.0 | 4.7 | 0.0 | 79.5 | 79.5 | 10.4 | 4.7 | 0.0 | 0.0 | 4.7 | 4.7 | 4.7 | 79.5 | 79.5 | 79.5 | 79.5 | 4.7 | 4.7 |
| CA-CC-01 | 0.0 | 0.0 | 97.9 | 0.0 | 0.0 | 0.0 | 100.0 | 86.0 | 0.0 | 95.4 | 97.9 | 97.9 | 97.9 | 0.0 | 0.0 | 0.0 | 0.0 | 0.0 | 0.0 |
| CA-CG-01 | 0.0 | 0.0 | 60.2 | 0.0 | 60.6 | 60.6 | 99.9 | 60.1 | 0.0 | 0.0 | 60.2 | 60.2 | 60.2 | 60.6 | 60.6 | 60.6 | 60.6 | 20.8 | 20.8 |
| CA-CG-02 | 0.0 | 0.0 | 85.7 | 0.0 | 79.9 | 79.9 | 85.6 | 85.7 | 0.0 | 0.0 | 85.7 | 85.7 | 85.7 | 79.9 | 79.9 | 79.9 | 79.9 | 79.8 | 79.8 |
| CA-CH-03 | 0.0 | 52.6 | 52.8 | 0.0 | 6.8 | 6.8 | 90.2 | 51.5 | 0.0 | 7.1 | 52.8 | 52.8 | 52.8 | 6.8 | 6.8 | 6.8 | 6.8 | 0.0 | 0.0 |
| CA-COLQ-01 | 0.0 | 0.0 | 0.0 | 0.0 | 80.2 | 80.2 | 100.0 | 0.0 | 0.0 | 0.0 | 0.0 | 0.0 | 0.0 | 80.2 | 80.2 | 80.2 | 80.2 | 0.0 | 0.0 |
| CA-CP-01 | 0.0 | 0.0 | 95.1 | 0.0 | 98.1 | 98.1 | 100.0 | 94.3 | 0.0 | 0.0 | 95.1 | 95.1 | 95.1 | 98.1 | 98.1 | 98.1 | 98.1 | 93.1 | 92.8 |
| CA-ESP-01 | 0.0 | 0.0 | 0.0 | 0.0 | 96.4 | 96.5 | 100.0 | 0.0 | 0.0 | 0.0 | 0.0 | 0.0 | 0.0 | 96.5 | 96.4 | 96.4 | 96.4 | 0.0 | 0.0 |
| CA-HU-04 | 0.0 | 0.0 | 0.0 | 0.0 | 48.1 | 48.1 | 100.0 | 0.0 | 0.0 | 0.0 | 0.0 | 0.0 | 0.0 | 48.1 | 48.1 | 48.1 | 48.1 | 0.0 | 0.0 |
| CA-HU-09 | 0.0 | 0.0 | 0.0 | 0.0 | 99.8 | 99.8 | 100.0 | 0.0 | 0.0 | 0.0 | 0.0 | 0.0 | 0.0 | 99.8 | 99.8 | 99.8 | 99.8 | 0.0 | 0.0 |
| CA-HUA-01 | 0.0 | 0.0 | 67.0 | 0.0 | 79.9 | 79.9 | 72.9 | 67.0 | 0.0 | 0.0 | 67.0 | 67.0 | 67.0 | 79.9 | 79.9 | 79.9 | 79.9 | 58.8 | 58.8 |
| CA-HUAR-01 | 0.0 | 0.0 | 0.0 | 0.0 | 43.5 | 43.5 | 96.3 | 0.0 | 0.0 | 0.0 | 0.0 | 0.0 | 0.0 | 43.5 | 43.5 | 43.5 | 43.5 | 0.0 | 0.0 |
| CA-JUPR-01 | 0.0 | 0.0 | 84.3 | 0.0 | 98.6 | 98.6 | 84.3 | 84.3 | 0.0 | 0.0 | 84.3 | 70.3 | 70.3 | 98.6 | 98.6 | 98.6 | 98.6 | 69.3 | 69.3 |
| CA-MAC-01 | 0.0 | 0.0 | 54.4 | 0.0 | 88.6 | 88.6 | 61.3 | 54.4 | 0.0 | 0.0 | 54.5 | 55.7 | 55.7 | 88.6 | 88.6 | 88.6 | 88.6 | 54.3 | 54.3 |
| CA-ROM-01 | 0.0 | 0.0 | 49.8 | 0.0 | 85.1 | 85.1 | 76.8 | 69.4 | 0.0 | 0.0 | 49.8 | 67.8 | 67.8 | 85.1 | 85.1 | 85.1 | 85.1 | 49.6 | 49.6 |
| CA-RZC-01 | 0.0 | 0.0 | 97.7 | 0.0 | 0.0 | 0.0 | 97.7 | 86.7 | 97.7 | 97.3 | 97.7 | 97.7 | 97.7 | 0.0 | 0.0 | 0.0 | 0.0 | 0.0 | 0.0 |
| CA-RZC-02 | 0.0 | 0.0 | 89.7 | 0.0 | 6.0 | 6.0 | 90.2 | 87.9 | 78.4 | 89.9 | 89.7 | 89.7 | 89.7 | 6.0 | 6.0 | 6.0 | 6.0 | 6.0 | 6.0 |
| CA-SB-01 | 0.0 | 14.8 | 94.9 | 0.0 | 74.2 | 0.0 | 100.0 | 93.3 | 0.0 | 20.3 | 94.9 | 93.8 | 93.8 | 74.2 | 0.0 | 0.0 | 0.0 | 0.0 | 0.0 |
| CA-SOC-01 | 0.0 | 0.0 | 100.0 | 0.0 | 100.0 | 100.0 | 100.0 | 98.1 | 0.0 | 0.0 | 100.0 | 100.0 | 100.0 | 100.0 | 100.0 | 100.0 | 100.0 | 100.0 | 98.1 |
| CA-TA-01 | 2.8 | 11.7 | 33.0 | 10.9 | 10.7 | 0.0 | 100.0 | 29.7 | 0.0 | 43.6 | 42.4 | 42.4 | 42.4 | 10.7 | 0.0 | 0.0 | 0.0 | 0.0 | 0.0 |
| CA-TAL-01 | 0.0 | 0.0 | 96.0 | 0.0 | 50.2 | 50.2 | 99.7 | 91.0 | 0.0 | 0.0 | 96.0 | 96.0 | 96.0 | 50.2 | 50.2 | 50.2 | 50.2 | 50.2 | 49.8 |
| CA-TO-01 | 22.8 | 0.0 | 88.7 | 21.8 | 89.6 | 0.0 | 100.0 | 85.9 | 0.0 | 24.7 | 88.7 | 88.7 | 88.7 | 89.6 | 0.0 | 0.0 | 0.0 | 0.0 | 0.0 |
| CA-VE-01 | 0.0 | 0.0 | 97.5 | 0.0 | 76.4 | 0.0 | 100.0 | 96.8 | 0.0 | 0.0 | 97.5 | 97.5 | 97.5 | 76.4 | 0.0 | 0.0 | 0.0 | 0.0 | 0.0 |
| CA-VMP-2A | 100.0 | 75.4 | 0.0 | 0.0 | 100.0 | 0.0 | 0.0 | 0.0 | 0.0 | 75.4 | 0.0 | 0.0 | 0.0 | 100.0 | 0.0 | 0.0 | 0.0 | 0.0 | 0.0 |

Gases que podrían entrar al modelo (>= 50 % de horas en >= 75 % de las estaciones): ninguno.

### Filas completas por estación (regla 8) para los conjuntos de la calibración

| estacion | F1-M0 (pm2_5, pm10) | F2-M0 (pm10, fracción) | F2-M1 (+ viento, humedad) | F2-M2 (+ temperatura, presión) |
|---|---|---|---|---|
| CA-03 | 0 | 0 | 0 | 0 |
| CA-AYAS-01 | 7 382 | 7 382 | 432 | 432 |
| CA-CC-01 | 0 | 0 | 0 | 0 |
| CA-CG-01 | 1 061 | 1 061 | 365 | 364 |
| CA-CG-02 | 6 159 | 6 159 | 6 151 | 6 147 |
| CA-CH-03 | 1 201 | 1 201 | 0 | 0 |
| CA-COLQ-01 | 558 | 558 | 0 | 0 |
| CA-CP-01 | 16 689 | 16 689 | 15 849 | 15 794 |
| CA-ESP-01 | 1 087 | 1 087 | 0 | 0 |
| CA-HU-04 | 254 | 254 | 0 | 0 |
| CA-HU-09 | 431 | 431 | 0 | 0 |
| CA-HUA-01 | 7 448 | 7 448 | 5 475 | 5 475 |
| CA-HUAR-01 | 282 | 282 | 0 | 0 |
| CA-JUPR-01 | 5 797 | 5 797 | 4 072 | 4 072 |
| CA-MAC-01 | 8 233 | 8 233 | 5 047 | 5 040 |
| CA-ROM-01 | 8 049 | 8 049 | 4 692 | 4 692 |
| CA-RZC-01 | 0 | 0 | 0 | 0 |
| CA-RZC-02 | 245 | 245 | 245 | 245 |
| CA-SB-01 | 0 | 0 | 0 | 0 |
| CA-SOC-01 | 696 | 696 | 696 | 683 |
| CA-TA-01 | 0 | 0 | 0 | 0 |
| CA-TAL-01 | 301 | 301 | 301 | 299 |
| CA-TO-01 | 0 | 0 | 0 | 0 |
| CA-VE-01 | 0 | 0 | 0 | 0 |
| CA-VMP-2A | 0 | 0 | 0 | 0 |
| TOTAL | 65 873 | 65 873 | 43 325 | 43 243 |

### Huecos por estación

| estacion | n_huecos | horas_en_huecos | mediana_h | p90_h | maximo_h | frac_en_huecos | frac_en_huecos_>24h |
|---|---|---|---|---|---|---|---|
| CA-03 | 0.000 | 0.000 | 0.000 | 0.000 | 0.000 | — | — |
| CA-AYAS-01 | 44.000 | 1 901.000 | 3.000 | 17.400 | 1 668.000 | 0.205 | 0.180 |
| CA-CC-01 | 0.000 | 0.000 | 0.000 | 0.000 | 0.000 | — | — |
| CA-CG-01 | 2.000 | 683.000 | 341.500 | 613.900 | 682.000 | 0.392 | 0.391 |
| CA-CG-02 | 25.000 | 1 529.000 | 1.000 | 109.400 | 1 056.000 | 0.199 | 0.188 |
| CA-CH-03 | 0.000 | 0.000 | 0.000 | 0.000 | 0.000 | 0.000 | 0.000 |
| CA-COLQ-01 | 3.000 | 138.000 | 28.000 | 92.800 | 109.000 | 0.198 | 0.197 |
| CA-CP-01 | 42.000 | 324.000 | 3.000 | 11.900 | 150.000 | 0.019 | 0.009 |
| CA-ESP-01 | 1.000 | 2.000 | 2.000 | 2.000 | 2.000 | 0.002 | 0.000 |
| CA-HU-04 | 0.000 | 0.000 | 0.000 | 0.000 | 0.000 | 0.000 | 0.000 |
| CA-HU-09 | 1.000 | 1.000 | 1.000 | 1.000 | 1.000 | 0.002 | 0.000 |
| CA-HUA-01 | 11.000 | 1 870.000 | 3.000 | 93.000 | 1 710.000 | 0.201 | 0.199 |
| CA-HUAR-01 | 2.000 | 6.000 | 3.000 | 3.800 | 4.000 | 0.021 | 0.000 |
| CA-JUPR-01 | 22.000 | 82.000 | 2.000 | 5.800 | 24.000 | 0.014 | 0.000 |
| CA-MAC-01 | 21.000 | 1 056.000 | 2.000 | 26.000 | 960.000 | 0.114 | 0.109 |
| CA-ROM-01 | 25.000 | 1 407.000 | 2.000 | 17.600 | 1 013.000 | 0.149 | 0.139 |
| CA-RZC-01 | 0.000 | 0.000 | 0.000 | 0.000 | 0.000 | — | — |
| CA-RZC-02 | 1.000 | 1.000 | 1.000 | 1.000 | 1.000 | 0.004 | 0.000 |
| CA-SB-01 | 281.000 | 6 760.000 | 1.000 | 5.000 | 4 590.000 | 0.206 | 0.192 |
| CA-SOC-01 | 0.000 | 0.000 | 0.000 | 0.000 | 0.000 | 0.000 | 0.000 |
| CA-TA-01 | 13.000 | 6 114.000 | 17.000 | 278.200 | 5 468.000 | 0.635 | 0.628 |
| CA-TAL-01 | 2.000 | 278.000 | 139.000 | 248.600 | 276.000 | 0.480 | 0.477 |
| CA-TO-01 | 387.000 | 2 642.000 | 1.000 | 5.000 | 737.000 | 0.104 | 0.071 |
| CA-VE-01 | 323.000 | 1 444.000 | 1.000 | 3.000 | 700.000 | 0.162 | 0.099 |
| CA-VMP-2A | 0.000 | 0.000 | 0.000 | 0.000 | 0.000 | 0.000 | 0.000 |

| tramo | huecos | horas | % de horas perdidas |
|---|---|---|---|
| 1 h | 731 | 731 | 2.8 |
| 2–6 h | 345 | 974 | 3.7 |
| 7–24 h | 81 | 938 | 3.6 |
| 1–7 días | 29 | 1 912 | 7.3 |
| > 7 días | 20 | 21 683 | 82.6 |

### PM en las 6 h previas a cada hueco (>= 3 h) frente al resto

| estacion | pm | huecos_>=min | pm_previo/resto | ic95_bajo | ic95_alto | %_sobre_eca_previo | %_sobre_eca_resto |
|---|---|---|---|---|---|---|---|
| CA-AYAS-01 | pm2_5 | 23 | 0.869 | 0.627 | 1.151 | 0.725 | 0.552 |
| CA-CG-01 | pm2_5 | 1 | — | — | — | 0.000 | 1.232 |
| CA-CG-02 | pm2_5 | 11 | 1.012 | 0.782 | 1.262 | 1.515 | 2.560 |
| CA-CH-03 | pm2_5 | 0 | — | — | — | — | 0.000 |
| CA-COLQ-01 | pm2_5 | 2 | 1.498 | 0.920 | 2.108 | 0.000 | 0.000 |
| CA-CP-01 | pm10 | 22 | 0.996 | 0.487 | 1.706 | 7.879 | 7.086 |
| CA-ESP-01 | pm2_5 | 0 | — | — | — | — | 2.114 |
| CA-HU-04 | pm2_5 | 0 | — | — | — | — | 92.913 |
| CA-HU-09 | pm2_5 | 0 | — | — | — | — | 22.506 |
| CA-HUA-01 | pm2_5 | 6 | 1.312 | 1.001 | 1.797 | 2.778 | 0.081 |
| CA-HUAR-01 | pm2_5 | 1 | — | — | — | 0.000 | 0.000 |
| CA-JUPR-01 | pm2_5 | 9 | 0.654 | 0.524 | 0.783 | 0.000 | 0.383 |
| CA-MAC-01 | pm2_5 | 8 | 1.308 | 0.974 | 1.877 | 0.000 | 0.208 |
| CA-ROM-01 | pm2_5 | 9 | 1.106 | 0.808 | 1.417 | 3.704 | 2.376 |
| CA-RZC-02 | pm2_5 | 0 | — | — | — | — | 76.735 |
| CA-SB-01 | pm10 | 46 | 0.987 | 0.749 | 1.252 | 0.362 | 0.237 |
| CA-SOC-01 | pm2_5 | 0 | — | — | — | — | 11.063 |
| CA-TA-01 | pm10 | 12 | 1.528 | 0.875 | 2.300 | 54.167 | 43.097 |
| CA-TAL-01 | pm2_5 | 1 | — | — | — | 0.000 | 0.000 |
| CA-TO-01 | pm10 | 82 | 0.855 | 0.762 | 0.953 | 1.667 | 2.556 |
| CA-VE-01 | pm10 | 48 | 0.606 | 0.484 | 0.743 | 1.389 | 7.661 |
| CA-VMP-2A | pm10 | 0 | — | — | — | — | 27.083 |

### Horas sobre el ECA

| estacion | horas_con_pm | %_horas_sobre_eca | %_24h_sobre_eca | episodios_24h | %_horas_pm2_5_sobre | %_horas_pm10_sobre |
|---|---|---|---|---|---|---|
| CA-03 | 0 | — | — | 0 | — | — |
| CA-AYAS-01 | 7 382 | 2.00 | 0.79 | 3 | 0.56 | 1.73 |
| CA-CC-01 | 0 | — | — | 0 | — | — |
| CA-CG-01 | 1 061 | 6.79 | 0.00 | 0 | 1.23 | 6.22 |
| CA-CG-02 | 6 159 | 4.89 | 0.41 | 3 | 2.55 | 3.33 |
| CA-CH-03 | 1 201 | 0.58 | 0.00 | 0 | 0.00 | 0.58 |
| CA-COLQ-01 | 558 | 3.05 | 0.00 | 0 | 0.00 | 3.05 |
| CA-CP-01 | 16 692 | 7.46 | 1.96 | 22 | 1.16 | 7.09 |
| CA-ESP-01 | 1 088 | 18.20 | 17.79 | 6 | 2.11 | 18.03 |
| CA-HU-04 | 254 | 94.09 | 100.00 | 1 | 92.91 | 71.65 |
| CA-HU-09 | 431 | 22.51 | 32.05 | 6 | 22.51 | 10.67 |
| CA-HUA-01 | 7 448 | 0.85 | 0.00 | 0 | 0.09 | 0.81 |
| CA-HUAR-01 | 282 | 1.42 | 0.00 | 0 | 0.00 | 1.42 |
| CA-JUPR-01 | 5 798 | 0.72 | 0.00 | 0 | 0.38 | 0.50 |
| CA-MAC-01 | 8 233 | 0.55 | 0.00 | 0 | 0.21 | 0.41 |
| CA-ROM-01 | 8 049 | 3.30 | 0.00 | 0 | 2.39 | 1.44 |
| CA-RZC-01 | 0 | — | — | 0 | — | — |
| CA-RZC-02 | 245 | 82.04 | 94.47 | 2 | 76.73 | 62.04 |
| CA-SB-01 | 26 029 | 0.24 | 0.00 | 0 | — | 0.24 |
| CA-SOC-01 | 696 | 19.11 | 0.44 | 1 | 11.06 | 16.67 |
| CA-TA-01 | 3 520 | 43.32 | 37.08 | 45 | — | 43.32 |
| CA-TAL-01 | 301 | 0.00 | 0.00 | 0 | 0.00 | 0.00 |
| CA-TO-01 | 22 853 | 2.54 | 0.33 | 5 | — | 2.54 |
| CA-VE-01 | 7 445 | 7.47 | 2.30 | 8 | — | 7.47 |
| CA-VMP-2A | 240 | 27.08 | 23.77 | 7 | — | 27.08 |

### Fracción fina y PM2.5 > PM10

| estacion | horas_con_pm | horas_con_ambas | simultaneidad | %_pm25_mayor_pm10 | fraccion_p25 | fraccion_mediana | fraccion_p75 |
|---|---|---|---|---|---|---|---|
| CA-03 | 0 | 0 | — | — | — | — | — |
| CA-AYAS-01 | 7 382 | 7 382 | 1.000 | 0.000 | 0.410 | 0.669 | 0.808 |
| CA-CC-01 | 0 | 0 | — | — | — | — | — |
| CA-CG-01 | 1 061 | 1 061 | 1.000 | 0.000 | 0.245 | 0.597 | 0.670 |
| CA-CG-02 | 6 159 | 6 159 | 1.000 | 0.000 | 0.452 | 0.670 | 0.708 |
| CA-CH-03 | 1 201 | 1 201 | 1.000 | 0.000 | 0.494 | 0.695 | 0.753 |
| CA-COLQ-01 | 558 | 558 | 1.000 | 0.000 | 0.376 | 0.672 | 0.775 |
| CA-CP-01 | 16 692 | 16 689 | 1.000 | 0.000 | 0.267 | 0.494 | 0.748 |
| CA-ESP-01 | 1 088 | 1 087 | 0.999 | 0.000 | 0.198 | 0.500 | 0.827 |
| CA-HU-04 | 254 | 254 | 1.000 | 0.000 | 0.669 | 0.719 | 0.760 |
| CA-HU-09 | 431 | 431 | 1.000 | 0.000 | 0.574 | 0.668 | 0.695 |
| CA-HUA-01 | 7 448 | 7 448 | 1.000 | 0.000 | 0.343 | 0.508 | 0.709 |
| CA-HUAR-01 | 282 | 282 | 1.000 | 0.000 | 0.397 | 0.686 | 0.776 |
| CA-JUPR-01 | 5 798 | 5 797 | 1.000 | 0.000 | 0.406 | 0.671 | 0.828 |
| CA-MAC-01 | 8 233 | 8 233 | 1.000 | 0.000 | 0.450 | 0.612 | 0.784 |
| CA-ROM-01 | 8 049 | 8 049 | 1.000 | 0.000 | 0.503 | 0.673 | 0.716 |
| CA-RZC-01 | 0 | 0 | — | — | — | — | — |
| CA-RZC-02 | 245 | 245 | 1.000 | 0.000 | 0.577 | 0.672 | 0.730 |
| CA-SB-01 | 26 029 | 0 | 0.000 | — | — | — | — |
| CA-SOC-01 | 696 | 696 | 1.000 | 0.000 | 0.229 | 0.320 | 0.457 |
| CA-TA-01 | 3 520 | 0 | 0.000 | — | — | — | — |
| CA-TAL-01 | 301 | 301 | 1.000 | 0.000 | 0.218 | 0.286 | 0.378 |
| CA-TO-01 | 22 853 | 0 | 0.000 | — | — | — | — |
| CA-VE-01 | 7 445 | 0 | 0.000 | — | — | — | — |
| CA-VMP-2A | 240 | 0 | 0.000 | — | — | — | — |

### Ciclos (η² de la hora y del día de semana sobre log(1 + x))

| estacion | eta2_hora_so2 | eta2_semana_so2 | eta2_hora_h2s | eta2_semana_h2s | eta2_hora_pm10 | eta2_semana_pm10 | eta2_hora_pm2_5 | eta2_semana_pm2_5 | eta2_hora_co | eta2_semana_co | eta2_hora_o3 | eta2_semana_o3 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| CA-03 | 0.037 | 0.006 | 0.015 | 0.005 | — | — | — | — | — | — | — | — |
| CA-AYAS-01 | — | — | — | — | 0.085 | 0.006 | 0.133 | 0.003 | — | — | — | — |
| CA-CC-01 | 0.121 | 0.000 | — | — | — | — | — | — | — | — | — | — |
| CA-CG-01 | — | — | — | — | 0.591 | 0.012 | 0.195 | 0.028 | — | — | — | — |
| CA-CG-02 | — | — | — | — | 0.314 | 0.006 | 0.237 | 0.002 | — | — | — | — |
| CA-CH-03 | 0.196 | 0.031 | 0.002 | 0.006 | 0.511 | 0.031 | 0.120 | 0.047 | — | — | — | — |
| CA-COLQ-01 | — | — | — | — | 0.074 | 0.081 | 0.089 | 0.042 | — | — | — | — |
| CA-CP-01 | — | — | — | — | 0.211 | 0.001 | 0.150 | 0.001 | — | — | — | — |
| CA-ESP-01 | — | — | — | — | 0.227 | 0.026 | 0.189 | 0.018 | — | — | — | — |
| CA-HU-04 | — | — | — | — | 0.307 | 0.063 | 0.184 | 0.148 | — | — | — | — |
| CA-HU-09 | — | — | — | — | 0.282 | 0.063 | 0.407 | 0.046 | — | — | — | — |
| CA-HUA-01 | — | — | — | — | 0.396 | 0.004 | 0.110 | 0.011 | — | — | — | — |
| CA-HUAR-01 | — | — | — | — | 0.050 | 0.187 | 0.066 | 0.155 | — | — | — | — |
| CA-JUPR-01 | — | — | — | — | 0.024 | 0.003 | 0.030 | 0.002 | — | — | — | — |
| CA-MAC-01 | — | — | — | — | 0.309 | 0.004 | 0.090 | 0.007 | — | — | — | — |
| CA-ROM-01 | — | — | — | — | 0.366 | 0.002 | 0.274 | 0.001 | — | — | — | — |
| CA-RZC-01 | 0.238 | 0.002 | — | — | — | — | — | — | — | — | — | — |
| CA-RZC-02 | 0.379 | 0.015 | — | — | 0.335 | 0.247 | 0.574 | 0.169 | — | — | — | — |
| CA-SB-01 | 0.005 | 0.001 | 0.000 | 0.000 | 0.125 | 0.001 | — | — | — | — | — | — |
| CA-SOC-01 | — | — | — | — | 0.312 | 0.044 | 0.619 | 0.013 | — | — | — | — |
| CA-TA-01 | 0.136 | 0.000 | 0.226 | 0.002 | 0.512 | 0.024 | — | — | 0.040 | 0.023 | 0.028 | 0.002 |
| CA-TAL-01 | — | — | — | — | 0.147 | 0.181 | 0.118 | 0.091 | — | — | — | — |
| CA-TO-01 | 0.009 | 0.006 | — | — | 0.118 | 0.001 | — | — | 0.002 | 0.001 | 0.288 | 0.002 |
| CA-VE-01 | — | — | — | — | 0.208 | 0.001 | — | — | — | — | — | — |
| CA-VMP-2A | 0.104 | 0.129 | 0.340 | 0.210 | 0.091 | 0.297 | — | — | 0.240 | 0.064 | — | — |

### Meteorología en las horas con PM (%)

| estacion | viento_y_humedad | temperatura_y_presion | las_cuatro |
|---|---|---|---|
| CA-03 | — | — | — |
| CA-AYAS-01 | 5.9 | 5.9 | 5.9 |
| CA-CC-01 | — | — | — |
| CA-CG-01 | 34.4 | 34.3 | 34.3 |
| CA-CG-02 | 99.9 | 99.8 | 99.8 |
| CA-CH-03 | 0.0 | 0.0 | 0.0 |
| CA-COLQ-01 | 0.0 | 0.0 | 0.0 |
| CA-CP-01 | 95.0 | 94.6 | 94.6 |
| CA-ESP-01 | 0.0 | 0.0 | 0.0 |
| CA-HU-04 | 0.0 | 0.0 | 0.0 |
| CA-HU-09 | 0.0 | 0.0 | 0.0 |
| CA-HUA-01 | 73.5 | 73.5 | 73.5 |
| CA-HUAR-01 | 0.0 | 0.0 | 0.0 |
| CA-JUPR-01 | 70.2 | 84.4 | 70.2 |
| CA-MAC-01 | 61.3 | 61.2 | 61.2 |
| CA-ROM-01 | 58.3 | 58.3 | 58.3 |
| CA-RZC-01 | — | — | — |
| CA-RZC-02 | 100.0 | 100.0 | 100.0 |
| CA-SB-01 | 96.4 | 96.9 | 95.5 |
| CA-SOC-01 | 100.0 | 98.1 | 98.1 |
| CA-TA-01 | 10.6 | 93.3 | 10.6 |
| CA-TAL-01 | 100.0 | 99.3 | 99.3 |
| CA-TO-01 | 93.6 | 92.2 | 92.2 |
| CA-VE-01 | 96.8 | 96.3 | 96.3 |
| CA-VMP-2A | 0.0 | 0.0 | 0.0 |

### Estaciones a menos de 15 km

| a | b | km | dias_con_pm_en_ambas | correlacion_diaria_pm10 |
|---|---|---|---|---|
| CA-01 | CA-03 | 2.57 | 0 | — |
| CA-01 | CA-CH-03 | 2.57 | 0 | — |
| CA-03 | CA-CH-03 | 0.00 | 0 | — |
| CA-AYAS-01 | CA-JUPR-01 | 8.18 | 206 | 0.74 |
| CA-CG-01 | CA-CG-02 | 0.30 | 0 | — |
| CA-CG-01 | CA-ROM-01 | 4.53 | 45 | 0.69 |
| CA-CG-02 | CA-ROM-01 | 4.37 | 265 | 0.76 |
| CA-COLQ-01 | CA-HUAR-01 | 5.69 | 10 | — |
| CA-COLQ-01 | CA-SB-01 | 8.39 | 16 | 0.44 |
| CA-HU-04 | CA-HU-09 | 2.41 | 7 | — |
| CA-HU-04 | CA-RZC-01 | 4.14 | 0 | — |
| CA-HU-04 | CA-RZC-02 | 7.02 | 2 | — |
| CA-HU-09 | CA-RZC-01 | 6.45 | 0 | — |
| CA-HU-09 | CA-RZC-02 | 9.41 | 12 | 0.89 |
| CA-HUA-01 | CA-MAC-01 | 11.31 | 314 | 0.50 |
| CA-HUAR-01 | CA-SB-01 | 13.37 | 5 | — |
| CA-RZC-01 | CA-RZC-02 | 3.23 | 0 | — |
| CA-VMP-1 | CA-VMP-10 | 0.59 | 0 | — |
| CA-VMP-1 | CA-VMP-2 | 0.34 | 0 | — |
| CA-VMP-1 | CA-VMP-2A | 0.27 | 0 | — |
| CA-VMP-1 | CA-VMP-6 | 1.25 | 0 | — |
| CA-VMP-1 | CA-VMP-7 | 0.17 | 0 | — |
| CA-VMP-1 | CA-VMP-8 | 0.14 | 0 | — |
| CA-VMP-1 | CA-VMP-9 | 0.13 | 0 | — |
| CA-VMP-10 | CA-VMP-2 | 0.49 | 0 | — |
| CA-VMP-10 | CA-VMP-2A | 0.50 | 0 | — |
| CA-VMP-10 | CA-VMP-6 | 1.04 | 0 | — |
| CA-VMP-10 | CA-VMP-7 | 0.48 | 0 | — |
| CA-VMP-10 | CA-VMP-8 | 0.49 | 0 | — |
| CA-VMP-10 | CA-VMP-9 | 0.46 | 0 | — |
| CA-VMP-2 | CA-VMP-2A | 0.07 | 0 | — |
| CA-VMP-2 | CA-VMP-6 | 1.41 | 0 | — |
| CA-VMP-2 | CA-VMP-7 | 0.40 | 0 | — |
| CA-VMP-2 | CA-VMP-8 | 0.21 | 0 | — |
| CA-VMP-2 | CA-VMP-9 | 0.27 | 0 | — |
| CA-VMP-2A | CA-VMP-6 | 1.38 | 0 | — |
| CA-VMP-2A | CA-VMP-7 | 0.34 | 0 | — |
| CA-VMP-2A | CA-VMP-8 | 0.15 | 0 | — |
| CA-VMP-2A | CA-VMP-9 | 0.21 | 0 | — |
| CA-VMP-6 | CA-VMP-7 | 1.08 | 0 | — |
| CA-VMP-6 | CA-VMP-8 | 1.27 | 0 | — |
| CA-VMP-6 | CA-VMP-9 | 1.20 | 0 | — |
| CA-VMP-7 | CA-VMP-8 | 0.20 | 0 | — |
| CA-VMP-7 | CA-VMP-9 | 0.14 | 0 | — |
| CA-VMP-8 | CA-VMP-9 | 0.07 | 0 | — |

### Coordenadas y departamento

| estacion | departamento | provincia | distrito | lat | lon | juegos_de_ubicacion | con_datos_validos |
|---|---|---|---|---|---|---|---|
| CA-01 | ANCASH | SANTA | CHIMBOTE | -9.1258 | -78.5569 | 1.0000 | no |
| CA-03 | ANCASH | SANTA | CHIMBOTE | -9.1033 | -78.5619 | 1.0000 | sí |
| CA-AYAS-01 | ANCASH | HUARI | SAN MARCOS | -9.5238 | -77.0204 | 2.0000 | sí |
| CA-CC-01 | JUNIN | YAULI | LA OROYA | -11.5199 | -75.9009 | 1.0000 | sí |
| CA-CG-01 | LA LIBERTAD | ASCOPE | CASA GRANDE | -7.7424 | -79.1874 | 1.0000 | sí |
| CA-CG-02 | LA LIBERTAD | ASCOPE | CASA GRANDE | -7.7412 | -79.1850 | 1.0000 | sí |
| CA-CH-03 | ANCASH | SANTA | CHIMBOTE | -9.1033 | -78.5619 | 1.0000 | sí |
| CA-COLQ-01 | PASCO | PASCO | TINYAHUARCO | -10.7508 | -76.2636 | 1.0000 | sí |
| CA-CP-01 | CUSCO | CHUMBIVILCAS | CAPACMARCA | -14.0087 | -72.0013 | 1.0000 | sí |
| CA-ESP-01 | CUSCO | ESPINAR | ESPINAR | -14.8060 | -71.4149 | 1.0000 | sí |
| CA-HU-04 | LIMA | LIMA | LURIGANCHO | -11.9961 | -76.9225 | 1.0000 | sí |
| CA-HU-09 | LIMA | LIMA | LURIGANCHO | -12.0124 | -76.9372 | 1.0000 | sí |
| CA-HUA-01 | PIURA | PAITA | LA HUACA | -4.9147 | -80.9604 | 1.0000 | sí |
| CA-HUAR-01 | — | — | — | -10.7937 | -76.2921 | — | sí |
| CA-JUPR-01 | ANCASH | HUARI | SAN MARCOS | -9.5617 | -77.0844 | 1.0000 | sí |
| CA-MAC-01 | PIURA | PAITA | LA HUACA | -4.9170 | -80.8583 | 1.0000 | sí |
| CA-ROM-01 | LA LIBERTAD | ASCOPE | CASA GRANDE | -7.7633 | -79.1521 | 1.0000 | sí |
| CA-RZC-01 | LIMA | LIMA | LURIGANCHO | -11.9789 | -76.8887 | 1.0000 | sí |
| CA-RZC-02 | LIMA | LIMA | LURIGANCHO | -11.9529 | -76.8756 | 1.0000 | sí |
| CA-SB-01 | PASCO | PASCO | SIMON BOLIVAR | -10.6756 | -76.2692 | 1.0000 | sí |
| CA-SOC-01 | AREQUIPA | AREQUIPA | SOCABAYA | -16.4876 | -71.5191 | 1.0000 | sí |
| CA-TA-01 | PIURA | TALARA | PARIÑAS | -4.5758 | -81.2773 | 1.0000 | sí |
| CA-TAL-01 | MOQUEGUA | MARISCAL NIETO | TORATA | -17.0994 | -70.6819 | 1.0000 | sí |
| CA-TO-01 | MOQUEGUA | MARISCAL NIETO | TORATA | -17.0754 | -70.8276 | 1.0000 | sí |
| CA-VE-01 | CUSCO | CHUMBIVILCAS | VELILLE | -14.5061 | -71.8849 | 1.0000 | sí |
| CA-VMP-1 | CALLAO | CALLAO | MI PERU | -11.8619 | -77.1221 | 2.0000 | no |
| CA-VMP-10 | CALLAO | CALLAO | VENTANILLA | -11.8633 | -77.1273 | 1.0000 | no |
| CA-VMP-2 | CALLAO | CALLAO | VENTANILLA | -11.8599 | -77.1244 | 3.0000 | no |
| CA-VMP-2A | CALLAO | CALLAO | VENTANILLA | -11.8602 | -77.1239 | 1.0000 | sí |
| CA-VMP-6 | CALLAO | CALLAO | VENTANILLA | -11.8725 | -77.1257 | 1.0000 | no |
| CA-VMP-7 | CALLAO | CALLAO | MI PERU | -11.8631 | -77.1229 | 2.0000 | no |
| CA-VMP-8 | CALLAO | CALLAO | MI PERU | -11.8614 | -77.1232 | 1.0000 | no |
| CA-VMP-9 | CALLAO | CALLAO | MI PERU | -11.8620 | -77.1233 | 1.0000 | no |

## Limpieza (prototipo de la cascada de Go)

Orden de la cascada: columnas → estacion → periodo → sin_valor → parametro → fecha → fecha_fuera_de_rango → hora_no_en_punto → hora → valor_no_numerico → centinela → unidad → negativo → fuera_de_rango → bloque_en_cero → cero_como_faltante → duplicado_unidad → duplicado_exacto → duplicado_valor_distinto. Mismo orden y nombres que `datos/bloque.go`.

| regla | qué | filas descartadas | % de las leídas |
|---|---|---|---|
| columnas | La fila no tiene tantos campos como la cabecera | 0 | 0.000 |
| estacion | Estación vacía | 0 | 0.000 |
| periodo | Regla 1: el periodo no es horario (o «-» sin hora y con el día incompleto) | 218 998 | 7.594 |
| sin_valor | Ninguna columna de parámetro con valor: el hueco de la plantilla de 2022 (ocupa antes su hora en el bloque físico) | 262 743 | 9.111 |
| parametro | Más de una columna de parámetro con valor (formato real) o parámetro sin canónico (formato parámetro/valor) | 0 | 0.000 |
| fecha | Regla 2: fecha no parseable con ningún formato | 3 | 0.000 |
| fecha_fuera_de_rango | Regla 2: fuera de [fecha_desde, fecha_hasta) | 0 | 0.000 |
| hora_no_en_punto | Regla 2: hora explícita con minutos o segundos (política descartar) | 0 | 0.000 |
| hora | Regla 2: hora ilegible, o sin hora y ni su día ni su bloque físico tienen las filas_por_dia contiguas | 10 691 | 0.371 |
| valor_no_numerico | Regla 3: vacío, texto, NaN o infinito | 0 | 0.000 |
| centinela | Regla 3: código del equipo (sobre el valor crudo) | 0 | 0.000 |
| unidad | Regla 6: unidad que no figura para ese parámetro | 0 | 0.000 |
| negativo | Regla 3: negativo en un parámetro que no los admite | 2 | 0.000 |
| fuera_de_rango | Regla 4: fuera del rango físico, ya en la unidad canónica | 51 219 | 1.776 |
| bloque_en_cero | Un bloque del día entero en 0 exacto es un equipo sin medir, no una medición | 762 744 | 26.449 |
| cero_como_faltante | Un 0 exacto de un contaminante en un día con datos: hora sin dato (OEFA no promedia los 0) | 8 694 | 0.301 |
| duplicado_unidad | Regla 6: la misma lectura en otra unidad (ppb, mmHg); se conserva la canónica | 134 285 | 4.656 |
| duplicado_exacto | Regla 5: misma (estación, parámetro, fecha-hora) y mismo valor | 0 | 0.000 |
| duplicado_valor_distinto | Regla 5: misma clave y otro valor; se conserva el primero del archivo | 0 | 0.000 |

Leídas 2 883 877 = procesadas 1 434 498 + descartadas 1 449 379. Marcadas: {'duplicado_valor_distinto': 0, 'ubicacion_corregida': 5296, 'hora_inferida': 1333932, 'hora_no_en_punto': 0, 'unidad_convertida': 151524}. Fuera de rango por parámetro: {'presion': 50472, 'o3': 744, 'precipitacion': 2, 'temperatura': 1}. Filas anchas: {'total': 236001, 'completas_gold': 65873, 'pm2_5_mayor_pm10': 0, 'hora_inferida': 219542}.

## Supuestos del formato que hay que confirmar

- Config: configs/limpieza.json.
- Columnas resueltas: `{'estacion': 'NOMBRE_PUNTO', 'fecha': 'FECHA_INICIO', 'hora': 'HORA_INICIO', 'parametro': None, 'valor': None, 'periodo': 'PERIODO', 'unidad': 'UNIDAD_MEDIDA', 'tipo_analisis': 'TIPO_ANALISIS', 'departamento': 'DEPARTAMENTO', 'provincia': 'PROVINCIA', 'distrito': 'DISTRITO', 'ubigeo': 'UBIGEO', 'este': 'ESTE', 'norte': 'NORTE', 'altitud': 'ALTITUD', 'zona': 'ZONA', 'datum': 'DATUM', 'param:DIOXIDO_AZUFRE': 'DIOXIDO_AZUFRE', 'param:DIRECCION_VIENTO': 'DIRECCION_VIENTO', 'param:HUMEDAD_RELATIVA': 'HUMEDAD_RELATIVA', 'param:MONOXIDO_CARBONO': 'MONOXIDO_CARBONO', 'param:OZONO': 'OZONO', 'param:PM10': 'PM10', 'param:PM2.5': 'PM2.5', 'param:PRECIPITACION': 'PRECIPITACION', 'param:PRESION_BAROMETRICA': 'PRESION_BAROMETRICA', 'param:RADIACION_SOLAR': 'RADIACION_SOLAR', 'param:SULFURO_HIDROGENO': 'SULFURO_HIDROGENO', 'param:TEMPERATURA': 'TEMPERATURA', 'param:VELOCIDAD_VIENTO': 'VELOCIDAD_VIENTO'}`.
- Parámetros sin canónico (descartados): ninguno.
- Periodos vistos: {'1 hora': 1548507, '-': 1127412, '3 horas móviles': 77088, 'Móvil 8 horas': 66048, 'Móvil 3 horas': 33024, '24 horas': 31527, 'Media móvil de 8 horas': 257, '24 Horas': 14}.
- Unidades vistas: {'precipitacion': {'mm': 225455}, 'presion': {'mmHg': 225314, 'hPa': 37131}, 'viento_velocidad': {'m/s': 224485}, 'viento_direccion': {'°c': 224485}, 'temperatura': {'°c': 223493}, 'humedad': {'%': 223489}, 'pm10': {'ug/m3': 193353}, 'so2': {'ug/m3': 160043, 'ppb': 159857}, 'h2s': {'ug/m3': 91866, 'ppb': 91680}, 'pm2_5': {'ug/m3': 90492}, 'co': {'ug/m3': 58799, 'ppb': 58543}, 'o3': {'ppb': 58543, 'ug/m3': 58543}, 'radiacion': {'W/m2': 7602}}.
- Ejemplos de lo descartado: {'fecha': {'NA': 3}, 'hora': {'sin_hora_y_grupo_incompleto': 10691, 'hora_ilegible': 0}, 'valor_no_numerico': {}, 'unidad': {}}.
- Delimitador ';', codificación utf-8, BOM False, separador decimal '.', fechas en [2000-01-01, 2026-07-01), formatos ['2006-01-02'].
- Primer MB: comillas impares en 0 líneas; columnas por fila {36: 2009}.

## La hora por posición (supuesto fuerte)

La hora falta en la mayoría de las filas y se reconstruye por la posición de la fila en su bloque de 24 (convención `fin`: cada lectura se etiqueta con el fin de su hora). Control con las filas que sí traen `HORA_INICIO` (hora de inicio según el diccionario de OEFA):

| control | filas | % |
|---|---|---|
| Filas con HORA_INICIO | 118 596 | 100.00 |
| … cuya hora de inicio es la posición en el bloque (0 a 23) | 118 163 | 99.63 |
| … cuya hora de inicio es la posición + 1 | 369 | 0.31 |
| Filas con HORA_INICIO en bloques completos de 24 | 112 584 | 94.93 |
| … cuya hora de inicio es la posición | 112 584 | 100.00 |

Desfase hora de inicio − posición en el bloque:

| hora − posición | filas |
|---|---|
| 0 | 118 163 |
| 1 | 369 |
| 18 | 48 |
| 22 | 16 |

|  | n | % (de los bloques, o de las filas leídas) |
|---|---|---|
| Bloques (estación, parámetro, fecha, unidad, periodo) | 138 176 | — |
| Bloques sin ninguna hora | 131 888 | 95.45 |
| … con las 24 filas contiguas (hora inferible) | 101 753 | 77.15 |
| Filas con hora inferida (en el silver) | 2 274 150 | 78.86 |
| Filas con hora explícita (en el silver) | 117 292 | 4.07 |
| … de ellas, con la hora de su bloque físico (día con filas vacías) | 19 710 | 0.68 |
| Filas descartadas por la regla hora | 10 691 | 0.37 |

Ciclo diario (valor estandarizado por estación) con hora explícita frente a inferida. Solo CA-HUA-01 y CA-MAC-01 traen hora, así que se comparan esas estaciones con las de hora inferida; si la reconstrucción corriera la hora, el pico del ciclo saldría desplazado:

| parametro | estaciones con hora explícita | estaciones con hora inferida | correlacion_de_los_ciclos | hora_pico_explicita | hora_pico_inferida |
|---|---|---|---|---|---|
| temperatura | 2 | 17 | 0.985 | 15 | 14 |
| humedad | 2 | 17 | 0.988 | 7 | 6 |
| viento_velocidad | 2 | 17 | 0.835 | 17 | 15 |
| pm10 | 2 | 20 | 0.797 | 16 | 8 |

Como son estaciones distintas, el control fino es por estación: hora (etiqueta de fin) del máximo y del mínimo de temperatura y humedad. Estaciones de la misma costa y altitud deben tener la misma fase, tengan hora explícita o inferida:

| index | departamento | altitud | % hora inferida | Temperatura máx | Temperatura mín | Humedad máx | Humedad mín |
|---|---|---|---|---|---|---|---|
| CA-03 | ANCASH | 15 | 100 | 15 | 1 | 3 | 15 |
| CA-CH-03 | ANCASH | 15 | 100 | 15 | 6 | 5 | 15 |
| CA-JUPR-01 | ANCASH | 4138 | 100 | 14 | 7 | 7 | 14 |
| CA-SOC-01 | AREQUIPA | 2384 | 100 | 14 | 6 | 21 | 12 |
| CA-CP-01 | CUSCO | 3600 | 100 | 16 | 6 | 7 | 16 |
| CA-VE-01 | CUSCO | 3768 | 100 | 15 | 6 | 6 | 15 |
| CA-CC-01 | JUNIN | 3728 | 100 | 14 | 7 | 6 | 14 |
| CA-CG-02 | LA LIBERTAD | 137 | 100 | 14 | 6 | 7 | 14 |
| CA-CG-01 | LA LIBERTAD | 149 | 100 | 14 | 7 | 7 | 14 |
| CA-ROM-01 | LA LIBERTAD | 177 | 100 | 14 | 6 | 6 | 15 |
| CA-RZC-01 | LIMA | 428 | 100 | 14 | 6 | 7 | 14 |
| CA-RZC-02 | LIMA | 504 | 100 | 14 | 6 | 7 | 14 |
| CA-TO-01 | MOQUEGUA | 2516 | 100 | 13 | 5 | 19 | 11 |
| CA-TAL-01 | MOQUEGUA | 3286 | 100 | 13 | 5 | 22 | 9 |
| CA-SB-01 | PASCO | 4258 | 100 | 15 | 7 | 7 | 14 |
| CA-TA-01 | PIURA | 2 | 100 | 12 | 0 | 22 | 12 |
| CA-HUA-01 | PIURA | 32 | 0 | 14 | 7 | 7 | 14 |
| CA-MAC-01 | PIURA | 38 | 0 | 15 | 7 | 7 | 15 |

## Unidades duplicadas y ceros

Cociente µg/m³ / (ppb × factor) en las lecturas gemelas (misma estación, parámetro y hora en las dos unidades):

| index | pares | 5% | 25% | 50% | 75% | 95% | fracción a < 2 % |
|---|---|---|---|---|---|---|---|
| co | 2 643.000 | 1.004 | 1.004 | 1.004 | 1.004 | 1.004 | 1.000 |
| h2s | 18 856.000 | 0.992 | 0.999 | 1.000 | 1.001 | 1.007 | 0.998 |
| o3 | 5 669.000 | 0.998 | 1.000 | 1.000 | 1.001 | 1.003 | 1.000 |
| presion | 37 107.000 | 1.000 | 1.000 | 1.000 | 1.000 | 1.000 | 1.000 |
| so2 | 70 010.000 | 0.994 | 0.999 | 1.000 | 1.001 | 1.006 | 0.996 |

Lecturas conservadas con unidad convertida: {'presion': 135872, 'so2': 4227, 'co': 4082, 'h2s': 3864, 'o3': 3479}.

Lecturas descartadas por `bloque_en_cero` (día entero en 0 exacto):

| index | filas descartadas por bloque_en_cero |
|---|---|
| co | 107 448 |
| h2s | 137 088 |
| humedad | 42 918 |
| o3 | 100 944 |
| pm10 | 62 400 |
| pm2_5 | 22 368 |
| radiacion | 480 |
| so2 | 168 912 |
| temperatura | 39 822 |
| viento_direccion | 40 182 |
| viento_velocidad | 40 182 |

Lecturas descartadas por `cero_como_faltante` (un 0 exacto de un contaminante en un día con datos):

| index | filas descartadas por cero_como_faltante |
|---|---|
| co | 256 |
| h2s | 1 237 |
| o3 | 567 |
| pm10 | 3 136 |
| pm2_5 | 415 |
| so2 | 3 083 |

Porcentaje de ceros exactos que quedan en el silver:

| co | h2s | humedad | o3 | pm10 | pm2_5 | precipitacion | presion | radiacion | so2 | temperatura | viento_direccion | viento_velocidad |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 0.0 | 0.0 | 1.1 | 0.0 | 0.0 | 0.0 | 95.3 | 0.0 | 46.3 | 0.0 | 1.2 | 2.9 | 2.9 |

Horas con temperatura, humedad y viento en 0 a la vez (equipo apagado, casi seguro):

| estacion | horas con las tres | las tres en 0 | % |
|---|---|---|---|
| CA-TAL-01 | 576 | 30 | 5.2 |
| CA-CH-03 | 9 263 | 224 | 2.4 |
| CA-03 | 4 776 | 93 | 1.9 |
| CA-SOC-01 | 696 | 13 | 1.9 |
| CA-TA-01 | 10 896 | 202 | 1.9 |
| CA-TO-01 | 22 632 | 379 | 1.7 |
| CA-SB-01 | 32 904 | 425 | 1.3 |
| CA-CC-01 | 34 320 | 391 | 1.1 |
| CA-CP-01 | 16 176 | 133 | 0.8 |
| CA-VE-01 | 9 504 | 75 | 0.8 |
| CA-RZC-02 | 3 647 | 13 | 0.4 |
| CA-RZC-01 | 3 931 | 8 | 0.2 |
| CA-AYAS-01 | 434 | 0 | 0.0 |
| CA-ROM-01 | 4 707 | 0 | 0.0 |
| CA-MAC-01 | 5 057 | 0 | 0.0 |
| CA-CG-01 | 1 055 | 0 | 0.0 |
| CA-CG-02 | 6 601 | 0 | 0.0 |
| CA-JUPR-01 | 4 132 | 0 | 0.0 |
| CA-HUA-01 | 6 239 | 0 | 0.0 |

## Lectura y memoria

Lectura 46 s y cascada 28 s con pandas en local; RSS máximo 2,205 MB. Registros con salto de línea dentro de un campo: 100.
