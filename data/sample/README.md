# Muestra sintética de OEFA (formato largo)

`oefa_muestra.csv` **no son datos reales**: la genera `datos/muestra_test.go` con semilla fija
para los tests y el CI, porque el CSV de OEFA (1.3 GB) todavía no está en disco. Las estaciones
se llaman `SINT-A` a `SINT-D` para que nadie las confunda con estaciones de la red.

Regenerarla (y comprobar que la versionada está al día):

```bash
go test ./datos -run TestMuestraVersionadaAlDia -actualizar   # regenera
go test ./datos -run TestMuestraVersionadaAlDia               # falla si difiere
```

## Qué supone sobre el formato real

Son los supuestos de `datos.ConfigPorDefecto()` y de `configs/limpieza.json` al 31/10. Se
verifican con el primer megabyte del archivo real y, si no se cumplen, se cambia el JSON; la
muestra puede seguir con este formato porque los tests usan la config por defecto, no el JSON.

- UTF-8 **con BOM**, delimitador `;`, punto decimal, fin de línea `\n`.
- Formato largo: una fila por (estación, parámetro, fecha-hora), con la ubicación de la estación
  repetida en cada fila.
- Cabecera (nombres supuestos): `ID_INFORME;PROCEDENCIA_MUESTRA;NOMBRE_PUNTO;DEPARTAMENTO;PROVINCIA;DISTRITO;UBIGEO;ESTE;NORTE;ALTITUD;ZONA;DATUM;PARAMETRO;UNIDAD;FECHA;VALOR;PERIODO`.
  La estación es `NOMBRE_PUNTO`.
- Fecha y hora en una sola columna, `dd/mm/aaaa HH:MM` (la estación D usa `dd/mm/aaaa HH:MM:SS`),
  hora local de Lima.
- Unidad en columna aparte (`ug/m3`, `mg/m3`, `°C`, `%`, `m/s`); periodo `Horario`, `Diario` o
  `Mensual`.
- Ningún campo entrecomillado contiene saltos de línea.
- El archivo no viene ordenado: las filas están mezcladas, así que una misma (estación,
  fecha-hora) aparece repartida en muchos bloques.

## Qué contiene

Cuatro estaciones, una semana de horas (4–10 de marzo de 2024), 4 236 filas largas:

| Estación | Parámetros | Particularidad |
|---|---|---|
| SINT-A Lima Norte | PM2.5, PM10, NO2, SO2, CO, temperatura, humedad, viento | parámetro no mapeado (Benceno), periodos diarios y mensual |
| SINT-B Arequipa | PM2.5, PM10, NO2, CO **en mg/m3**, temperatura, humedad, viento | CO se lleva a µg/m³ (×1000) |
| SINT-C Junín | PM2.5, PM10, SO2, temperatura, humedad, viento | altura: temperaturas bajo cero válidas; un día apagada y 12 h sin viento |
| SINT-D Ucayali | PM2.5, PM10, temperatura, humedad, viento | fecha con segundos; 11 h sin PM2.5 |

Casos sembrados de cada regla (los conteos exactos los fija el generador y los verifica
`TestReglasCuentanLoSembrado`): periodo no horario, parámetro no mapeado, estación vacía, filas
con columnas de menos o de más, fecha inválida, fecha fuera de rango, valor vacío / `ND` /
`NaN`, centinelas `-999`, `9999` y `-9999`, negativos, fuera de rango físico, unidad desconocida
(`ppb`, `ppm`), duplicados exactos y con valor distinto (junto al original y al final del
archivo), y horas con PM2.5 > PM10.
