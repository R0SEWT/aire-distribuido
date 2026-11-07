# Revisión de limpieza y completitud

Complemento de EDA y evidencias de limpieza preparado en `feature/eda-limpieza`.
El EDA original y la auditoría previa son trabajo existente del equipo. Esta revisión
contrasta sus conteos, aclara la interpretación de la completitud y reproduce la limpieza,
el entrenamiento y los notebooks sobre el CSV completo descargado del portal de OEFA.
Los resultados originales se conservan; la ejecución nueva usa una raíz de datos separada.

## Cómo reproducir la revisión

Desde la raíz del repositorio, con Python 3 y sin paquetes adicionales:

```bash
python3 docs/verificar-limpieza.py
```

El comando genera [los controles y la tabla por estación](../reports/revision_limpieza.md)
y termina con error si encuentra diferencias. Comprueba conservación de filas, sumas por
estación, conteos y fechas Go/Python, la copia que consume el informe, el tamaño de la
estandarización y las filas completas F1-M0 publicadas por el notebook 01.
La tabla se calcula desde los JSON; no se debe editar a mano.

## Ejecución sobre el CSV completo

Se necesita Go, `uv` y el CSV original. Desde la raíz del repositorio, ajustar `CSV` a la
descarga local. Estas salidas no se versionan y no pisan las capas ni los reportes originales:

```bash
CSV="/ruta/al/1a_Vigilancia y Seguimiento ambiental en la calidad del aire.csv"
R="$PWD/data/bronze/revision-dayana"
mkdir -p "$R/data/silver" "$R/data/gold" "$R/reports"
cp -f reports/centroides_iniciales.json reports/benchmark_cargador.json reports/benchmark_kmeans.json "$R/reports/"
go run ./cmd/cargar -entrada "$CSV" \
  -silver "$R/data/silver/oefa_ancho.csv" -gold "$R/data/gold/oefa_features.csv" -reports "$R/reports"
go run ./cmd/entrenar -gold "$R/data/gold/oefa_features.csv" \
  -estandarizacion "$R/reports/estandarizacion.json" -salida "$R/reports/kmeans.json" \
  -asignaciones "$R/data/gold/asignaciones.csv" -regimenes "$R/reports/regimenes.json" \
  -centroides "$R/reports/centroides_iniciales.json" -codo 2:10 -codo-salida "$R/reports/codo.json"
uv run docs/ejecutar-revision.py --bronze "$CSV" --raiz "$R"
```

El ejecutor usa los notebooks 01 y 03 del checkout actual. Guarda copias ejecutadas con
sus salidas y logs en `ejecutados/` dentro de la raíz de revisión; los notebooks fuente
siguen sin salidas. Las dependencias se resuelven en el entorno de `uv`, sin LaTeX.
Los centroides iniciales se reutilizan para comparar el mismo arranque; los benchmarks
copiados son referencias históricas, no mediciones nuevas del Mac.

Las evidencias pequeñas de esta ejecución se conservan en
[`reports/revision_dayana/`](../reports/revision_dayana/), separadas de la corrida original.
La nueva ejecución encontró y corrigió la conversión del máximo RSS en el notebook 01:
macOS devuelve bytes y Linux devuelve KiB; dividir por el mismo factor daba una memoria
aparente excesiva en macOS. La corrección solo afecta esa medición de memoria.

## Qué significa la completitud

La unidad de la tabla silver es una estación y una fecha-hora. El gold contiene las filas
silver con todas las features de la receta vigente, PM2,5 y PM10. Un registro largo del
bronze es una lectura de un parámetro: no se puede comparar su cantidad directamente con
las filas silver como si fueran la misma unidad.

Se distinguen dos denominadores:

- **Gold/silver:** filas completas divididas entre las filas anchas existentes. Es el
  porcentaje de la tabla de limpieza del informe. No cuenta las horas que no dejaron fila.
- **Gold/rango:** filas completas divididas entre las horas desde la primera hasta la
  última fila silver, incluidos ambos extremos. Es la definición temporal usada en el EDA.
  Incluye los huecos entre campañas y no demuestra cuánto tiempo estuvo activo el equipo.

Por eso una estación puede tener casi todas sus filas silver completas y, al mismo tiempo,
una cobertura temporal menor. Ninguno de estos porcentajes representa por sí solo la
cobertura de un año completo ni la exposición anual de la población.

## Estaciones fuera del modelo

Las estaciones sin filas silver quedan fuera antes de seleccionar features. Los reportes
del EDA describen periodos no horarios o bloques en cero; el JSON agregado no permite
atribuir una regla de descarte particular a cada estación. Para confirmarla se necesitaría
reprocesar el bronze con un desglose de reglas por estación.

Entre las estaciones con silver y sin gold, la matriz de completitud del EDA distingue:

- CA-03, CA-CC-01 y CA-RZC-01 no tienen PM horario válido.
- CA-SB-01, CA-TA-01, CA-TO-01, CA-VE-01 y CA-VMP-2A tienen PM10, pero no PM2,5 válido.

La falta de una feature impide entrar al K-Means aunque existan otros parámetros. No se
imputan esos faltantes ni se interpretan como aire limpio. Las estaciones con pocas filas
gold tampoco deben representar zonas o periodos que no observaron.

## Fuentes y límites

- [EDA y matriz estación × parámetro](../reports/eda_criba.md).
- [Reporte de limpieza de Go](../reports/limpieza.json).
- [Conteos del prototipo Python](../reports/eda_limpieza_prototipo.json).
- [Auditoría previa del equipo](../reports/auditoria.md).
- [Sección del informe](informe/secciones/_05-limpieza.qmd).

El verificador de salidas guardadas y la auditoría nueva tienen alcances distintos:
el primero comprueba consistencia de reportes; el notebook 03 recalcula desde el bronze
y compara también silver y gold. Los benchmarks históricos no se vuelven a medir.
La hora inferida conserva el supuesto de orden por posición descrito en el informe.
La ejecución nueva no establece los periodos efectivos de operación de cada estación:
para distinguir una interrupción de una campaña finalizada hacen falta sus metadatos.
