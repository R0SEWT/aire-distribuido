# Muestras del CSV de OEFA

Dos muestras pequeñas, versionadas, para los tests y el CI. Las dos tienen el **esquema real** del
CSV de OEFA (spec §4.1): 36 columnas, `;`, textos entre comillas, UTF-8 sin BOM, fin de línea CRLF,
una columna por parámetro con un solo valor por fila, `-` y `NA` como vacíos.

## `oefa_real_muestra.csv`: recorte del CSV real

**Datos reales**, sin modificar: 4,491 registros copiados byte a byte del archivo completo
(sha256 `1f09a806…f26d92`), en su orden original. Fuente: OEFA, *Vigilancia y Seguimiento ambiental
en la calidad del aire*, Portal de Datos Abiertos del Perú (datosabiertos.gob.pe), licencia ODC-By:
el uso exige citar a OEFA como fuente.

| Estación | Días | Por qué |
|---|---|---|
| CA-TA-01 | 2018-03-27 a 2018-04-02 | gases en ppb y µg/m³, presión en mmHg, meteorología con periodo `-`, móviles, 24 horas; presión, CO y meteorología en 0 (días sin medir); horas sueltas de SO2, O3 y H2S en 0 (`cero_como_faltante`) |
| CA-HUA-01 | 2021-12-09 y 10 | hora explícita (`1899-12-31 HH:00:00`, medianoche como `1899-12-31`) |
| CA-MAC-01 | 2021-12-20 | hora explícita |
| CA-JUPR-01 | 2022-06-07, 08 y 11 | filas sin valor intercaladas (plantilla de 2022: el día del parámetro tiene menos de 24 filas, pero el bloque físico las tiene; ningún día queda en la regla `hora`), presión en hPa y mmHg |
| CA-VMP-1 | 2019-01-25 y 26 | dos registros con saltos de línea dentro de un campo |
| CA-VMP-2 | todos (202 filas) | la misma estación con tres combinaciones de coordenadas y altitud |

`TestMuestraReal` la carga y verifica conservación, hora explícita e inferida (también por bloque
físico), unidades duplicadas, filas vacías (`sin_valor`), ceros sueltos, registros multilínea,
ubicaciones distintas y determinismo con W = 8 y bloques de 3 KB.

## `oefa_muestra.csv`: sintética

**No son datos reales**: la genera `datos/muestra_test.go` con semilla fija, con estaciones
`SINT-A` a `SINT-D` para que nadie las confunda con la red. El generador devuelve además la
verdad (la hora que quiso darle a cada fila, su valor canónico y qué regla la descarta), contra la
que se prueban las reglas, el silver y el determinismo.

```bash
go test ./datos -run TestMuestraVersionadaAlDia -actualizar   # regenera
go test ./datos -run TestMuestraVersionadaAlDia               # falla si difiere
```

3,321 registros, cinco días (4–8 de marzo de 2024), en bloques de 24 filas por día:

| Estación | Contenido |
|---|---|
| SINT-A | PM10, PM2.5, SO2 en ppb y µg/m³, SO2 en «3 horas móviles», meteorología con periodo `-`, presión en mmHg, dirección del viento con unidad `°c`; una hora con PM2.5 = PM10 = 0 y una de SO2 en µg/m³ en 0 (queda el de ppb) dentro de días con datos |
| SINT-B | PM10 y PM2.5 con hora explícita (medianoche como `1899-12-31`), CO solo en ppb, O3 en las dos unidades, presión en hPa; un día de presión que empieza con 10 filas vacías (plantilla de 2022) |
| SINT-C | **zona UTM 19** (error sembrado, como CA-HUAR-01: los tests la corrigen a 18 con `correcciones_ubicacion`), H2S en las dos unidades (un día en 0 en µg/m³: queda el de ppb), temperaturas bajo cero, un día de filas sin valor, un día de PM10 con 3 filas vacías intercaladas, un día de PM10 con 23 filas y uno de temperatura `-` con 23 |
| SINT-D | PM10 y PM2.5, «Media móvil de 8 horas», «24 horas» y «24 Horas» (este, multilínea), un día de PM2.5 partido en dos tramos (no contiguo), un día con otras coordenadas, una fila con el nombre en minúsculas |

Casos sembrados de cada regla: fila con columnas de menos, estación vacía, filas sin valor (regla
sin_valor; las intercaladas en un día ocupan su hora) y una con dos valores (regla parametro),
periodos no horarios, fecha `NA` e imposible, fecha fuera de rango, hora no en punto, días
incompletos o no contiguos, `<LD` y `ND`, centinela, negativo, fuera de rango, unidad desconocida,
día en 0, horas sueltas de contaminantes en 0 (cero_como_faltante; incluye un PM2.5 que sale 0 en
la hora en que falta el PM10 de SINT-C), gases duplicados en dos unidades, duplicados exactos y con
otro valor (con hora explícita), horas con PM2.5 > PM10 y una zona UTM mal cargada.
