# Propuesta para valores faltantes en la transformación Bronze → Silver

## Contexto y objetivo

La PC3 transforma el CSV largo de vigilancia de calidad del aire de OEFA en el artefacto local
`data/silver/oefa_ancho.csv`: una tabla horaria ancha con una fila por estación y
`fecha_hora`, una columna por parámetro y la marca `hora_inferida`. Para la PC4 también se necesita un
catálogo de estaciones confiable, porque MongoDB expondrá la estación y su ubicación aunque una estación
no tenga observaciones aptas para entrenar.

Este documento propone cómo representar y, cuando exista evidencia suficiente, recuperar valores
faltantes de estación y ubicación. No cambia el pipeline de PC3 ni redefine Silver → Gold. En particular:

- **Bronze → Silver** valida, normaliza y pivota las lecturas; conserva como celdas vacías las mediciones
  ausentes.
- **Silver → Gold** selecciona las filas completas para las features de la receta y las estandariza. Que
  una observación no llegue a Gold no implica borrar su estación ni sus otras mediciones de Silver.

La revisión se basa en artefactos versionados: `datos/`, `cmd/cargar`, `configs/limpieza.json`, los
notebooks de PC3, `reports/limpieza.json`, `reports/estaciones_ficha.csv` y las muestras pequeñas de
Bronze en `data/sample/`. Para comprobar ejemplos fila a fila también se consultaron copias **locales no
versionadas** de Silver y Gold bajo `data/bronze/revision-dayana/data/`; esa ruta no existe necesariamente
en otro clon. El CSV Bronze completo de 1,3 GB tampoco está versionado. Los conteos globales citados son
los que dejó su corrida auditada en `reports/limpieza.json`, no una nueva ejecución realizada para esta
propuesta.

El procedimiento reproducible es descargar el CSV de OEFA enlazado en `data/README.md`, guardarlo como
`data/bronze/oefa.csv` y ejecutar:

```bash
go run ./cmd/cargar -entrada data/bronze/oefa.csv
go run ./cmd/entrenar
```

`cmd/cargar` genera `data/silver/oefa_ancho.csv`, `data/gold/oefa_features.csv` y los reportes de limpieza
y estandarización; `cmd/entrenar` genera `data/gold/asignaciones.csv` y los reportes del modelo. Todos los
CSV bajo `data/`, salvo las muestras, están ignorados por Git y no deben añadirse al PR. La propuesta es
complementaria al [diseño vigente de PC4 del PR #51](https://github.com/R0SEWT/aire-distribuido/blob/feature/spec-pc4/docs/specs/2026-11-14-pc4-design.md),
sin modificarlo desde esta rama.

## Diagnóstico del tratamiento actual

### Identidad y ubicación de la estación

La estación sale de `NOMBRE_PUNTO`. El cargador elimina espacios exteriores, colapsa espacios internos y
convierte a mayúsculas; una estación vacía o marcada como `-`, `NA` o cadena vacía se descarta. En la
corrida auditada se leyeron 33 estaciones y no hubo descartes por `estacion`.

Las columnas administrativas y UTM no condicionan la aceptación de una lectura. Los valores faltantes se
omiten del mapa de ubicación del reporte. Para cada nombre normalizado se cuentan las combinaciones de
departamento, provincia, distrito, ubigeo, este, norte, altitud, zona y datum; si hay varias, el reporte
elige la combinación con más filas y conserva todas en `ubicaciones_distintas`. Por tanto, dos puntos con
el mismo `NOMBRE_PUNTO` se fusionan actualmente aunque sus ubicaciones difieran.

El UBIGEO se conserva como texto de seis dígitos. Latitud y longitud no vienen de Silver: se calculan para
el reporte desde UTM solo cuando están presentes este, norte y zona y el datum es WGS84. El código no
calcula coordenadas parciales. `configs/limpieza.json` contiene una corrección curada: para `CA-HUAR-01`
cambia la zona UTM 19 a 18; el valor original queda en `ubicacion_original` y las 5 296 filas afectadas se
marcan como `ubicacion_corregida`.

El reporte avisa cuatro nombres con más de una ubicación: `CA-AYAS-01`, `CA-VMP-1`, `CA-VMP-2` y
`CA-VMP-7`. No decide si son correcciones, desplazamientos del equipo o reutilización del identificador.

### Fechas y horas

`FECHA_INICIO` es obligatoria y se interpreta en `America/Lima`. Una fecha no parseable se descarta; el
reporte registra 3 casos. El intervalo configurado es `[2000-01-01, 2026-07-01)` y no hubo descartes por
fecha fuera de ese rango.

`HORA_INICIO` representa el inicio del intervalo y puede traer el prefijo de Excel `1899-12-31`. Cuando la
hora está ausente, se recupera por posición únicamente si el grupo diario contiene exactamente 24 filas
contiguas, o si el bloque físico equivalente contiene 24 posiciones de un solo parámetro. Silver registra
el **fin** del intervalo en RFC 3339 y marca la fila con `hora_inferida = 1` si alguna de sus celdas usó
esta recuperación. La corrida reporta 1 333 932 lecturas con hora inferida y 10 691 descartes por no poder
determinar la hora. Esta recuperación no inventa un instante arbitrario: depende de la estructura de 24
filas verificada en los datos.

### Mediciones faltantes e inválidas

Internamente, una celda faltante de la tabla ancha es `NaN`; al escribir el CSV Silver se representa como
un campo vacío. No es el texto JSON `null` porque Silver es CSV. No se imputa una medición con la media,
con cero, con la lectura anterior ni con otra estación.

La cascada actual descarta filas largas con periodo no horario, sin valor, más de un parámetro, fecha u
hora inválida, texto no numérico, centinela, unidad desconocida, negativo no permitido o valor fuera del
rango físico. Convierte unidades conocidas antes de evaluar el rango. Los días completos en cero se
tratan como equipo sin medir para los parámetros configurados; además, un cero aislado de contaminantes
se trata como faltante. La precipitación conserva cero porque representa una hora seca, y la meteorología
solo pierde el cero cuando todo el día forma un bloque en cero.

En la corrida auditada se descartaron 762 744 lecturas por bloque diario en cero, 8 694 ceros aislados de
contaminantes y 51 219 valores fuera de rango. Silver tiene 236 001 filas anchas; sus campos vacíos son el
resultado esperado de conservar la observación temporal aunque falten algunos parámetros. Solo 65 873
filas completas para `pm2_5` y `pm10` pasaron después a Gold.

### Duplicados e inconsistencias

La clave de una celda es estación, parámetro y `fecha_hora`. Si la misma lectura aparece en más de una
unidad, se prefiere la unidad canónica frente al valor convertido. Entre lecturas de igual prioridad se
conserva la primera del archivo; un valor igual se cuenta como duplicado exacto y uno distinto se conserva
como conflicto marcado. La corrida reporta 134 285 duplicados de unidad y ningún duplicado exacto ni de
valor distinto. El orden de fusión respeta el orden del archivo para que el resultado no dependa de la
cantidad de workers.

Una inconsistencia de ubicación no descarta las mediciones: se informa y se escoge la variante modal para
el reporte. Esta decisión es útil para no perder lecturas, pero puede ocultar un traslado o la reutilización
de un código de estación; por eso no debe tomarse automáticamente como una corrección geográfica.

## Reglas propuestas por campo

La columna “actual” describe código y configuración existentes. La columna “propuesta” requiere acuerdo
del equipo antes de cambiar el pipeline o el seed de PC4.

| Campo | Tratamiento actual | Propuesta | Resultado si no se puede recuperar |
|---|---|---|---|
| Identificador de estación | Normaliza espacios y mayúsculas; descarta el identificador vacío. | Conservar esta regla. No crear códigos a partir de coordenadas, descripción ni orden de fila. Registrar el texto Bronze original y el código normalizado en la trazabilidad del catálogo. | Excluir la fila, porque no puede relacionarse de forma estable con una estación. |
| Departamento, provincia y distrito | No condicionan Silver; se conserva la variante de ubicación más frecuente. | Normalizar `-`, `NA` y vacío a ausencia. Completar solo mediante una fuente oficial verificable asociada inequívocamente al código o a sus coordenadas; guardar fuente, fecha y valor original. No inferir por cercanía a otra estación. | Mantener `null` en el catálogo; conservar las mediciones válidas. |
| UBIGEO | Rellena con cero a la izquierda hasta seis dígitos. | Conservar el relleno solo para valores numéricos de hasta seis dígitos. Validar la concordancia UBIGEO–departamento–provincia–distrito contra el catálogo oficial usado y registrar cualquier corrección como regla curada. | `null`; no fabricar un UBIGEO a partir del nombre. |
| Este, norte, zona UTM y datum | Se conservan como texto; una corrección configurada puede reemplazar un campo y guardar el original. | Exigir el conjunto completo y un datum admitido antes de derivar lat/lon. Mantener las variantes por estación y no reemplazar por la moda sin exponer que existen discrepancias. Toda corrección debe ser explícita en configuración y tener evidencia citada. | Campos faltantes en `null`; la estación sigue en el catálogo sin punto de mapa. |
| Latitud y longitud | Se derivan de UTM/WGS84 para el reporte; si falta un componente no se calculan. | Conservar la conversión determinista. Aceptar coordenadas externas solo si provienen de una ficha oficial identificable y se conserva su procedencia; no combinar latitud de una fuente con longitud de otra. Validar límites geográficos del Perú y el par completo. | Ambas en `null`. No usar `0,0`, el centroide de un distrito ni la ubicación de otra estación. |
| Fecha | Se parsea con los formatos configurados y se restringe al intervalo permitido. | Conservar. Añadir al reporte muestras acotadas de valores rechazados y su fila/origen para auditoría, sin intentar interpretar formatos ambiguos automáticamente. | Excluir la lectura; una medición sin día no es recuperable. |
| Hora | Usa hora explícita o posición en un bloque verificado de 24 filas; escribe el fin del intervalo. | Conservar la inferencia estructural y la marca `hora_inferida`. No interpolar huecos ni numerar grupos incompletos. Registrar separadamente hora explícita, inferida por día e inferida por bloque físico. | Excluir esa lectura; conservar la estación y las demás horas. |
| Mediciones | Convierte unidades conocidas, aplica centinelas y rangos, y conserva faltantes como celdas vacías. | Conservar la no imputación para Bronze → Silver. Mantener valor crudo, unidad cruda y motivo de descarte en reportes de auditoría o muestras de rechazo, sin agregarlos al CSV ancho. | Celda vacía en Silver; la fila temporal puede permanecer si otro parámetro es válido. |
| Cero | Contaminantes: faltante; día completo en cero: equipo sin medir; precipitación: cero válido. | Conservar por parámetro y mantener la regla configurable. Cualquier cambio debe apoyarse en documentación del instrumento o de OEFA y volver a medir sus efectos. | Celda vacía cuando la regla lo clasifica como faltante. |
| Duplicados | Prefiere unidad canónica; luego conserva el primero en orden de archivo y marca conflictos. | Conservar el determinismo, pero exportar una muestra de conflictos con ambos valores, unidades y posiciones Bronze. No promediar valores distintos. | Se conserva el valor ganador y se marca la celda; el alternativo permanece trazable en el reporte de conflicto. |
| Múltiples ubicaciones para un código | Fusiona por nombre y el reporte muestra la variante modal. | Conservar todas las variantes con conteo y periodo de vigencia observado. Antes de fusionarlas en el catálogo, determinar si son error, traslado o reutilización del código. | Estado `ubicacion_en_revision`; las lecturas siguen disponibles, pero el punto no debe mostrarse como una ubicación única confirmada. |

## Estaciones con ubicación incompleta

La ausencia de ubicación no invalida por sí sola una medición. Se propone separar dos capacidades:

1. **Serie medible:** requiere identificador, tiempo y al menos una medición válida. Puede participar en
   análisis no espaciales y permanecer en Silver.
2. **Estación georreferenciable:** además requiere latitud y longitud verificadas como par. Solo estas
   estaciones se muestran como puntos en el mapa.

El catálogo debe conservar la estación aunque sus campos administrativos o coordenadas sean `null`, junto
con un estado de ubicación (`confirmada`, `incompleta` o `en_revision`) y la procedencia de cada corrección
cuando el esquema de PC4 se amplíe para soportarlo. Hasta que ese cambio se apruebe, los campos faltantes
deben quedar ausentes/`null` y la limitación debe documentarse; no se deben sobrecargar los campos con
`"NA"` como si fuera una ubicación real.

Caso observado: `CA-HUAR-01` trae departamento, provincia, distrito y UBIGEO como `NA`. La zona UTM 19 fue
corregida a 18 porque la descripción original sitúa el punto en Huaraucaca y la zona 19 producía una
coordenada incompatible; `reports/limpieza.json` conserva `zona: 19` en `ubicacion_original`. Con UTM
completo y WGS84 se derivaron `lat = -10.793680` y `lon = -76.292123`. La propuesta **no** completa
automáticamente departamento, provincia, distrito ni UBIGEO con esos números: deben permanecer `null`
hasta vincular una fuente oficial que confirme esos campos.

Para estaciones con varias ubicaciones, como `CA-AYAS-01` (dos altitudes) y `CA-VMP-2` (tres variantes de
altitud y dos pares este/norte), la moda puede servir como valor operativo provisional, pero no demuestra
que las demás filas sean erróneas. Se propone conservar el conteo y el intervalo temporal de cada variante
antes de decidir una corrección.

## Observación excluida frente a estación eliminada

Excluir una observación significa que una fila larga concreta no aporta una celda a Silver, o que una fila
Silver no aporta un vector completo a Gold. La causa puede ser fecha/hora irrecuperable, valor inválido,
unidad desconocida o falta de una feature. Esto no elimina el identificador ni otras lecturas válidas.

Eliminar una estación del catálogo sería una decisión distinta y mucho más fuerte. Solo correspondería si
el identificador es inválido o si una revisión verificable demuestra que es un alias duplicado y define
cuál es el código canónico. Una estación con ubicación incompleta, sin PM o sin filas Gold sigue siendo una
estación observada. Por ejemplo, `CA-VMP-2` tiene 202 filas Bronze y ninguna fila Silver procesada por ser
promedios no horarios; aun así, `reports/estaciones_ficha.csv` conserva su ficha. Del mismo modo,
`CA-HUAR-01` aporta 282 filas Gold aunque sus campos administrativos estén ausentes.

## Recuperación permitida y trazabilidad

Un dato faltante solo se propone recuperar cuando se cumplen las cuatro condiciones siguientes:

1. La fuente es identificable y verificable: otra fila del mismo Bronze, una ficha oficial de OEFA, el
   diccionario del conjunto, o un catálogo oficial como UBIGEO de INEI con versión registrada.
2. La correspondencia con la estación es inequívoca; compartir distrito, objeto fiscalizado o cercanía no
   basta por sí solo.
3. La transformación es reproducible. Las conversiones UTM y el relleno de UBIGEO son deterministas; una
   corrección manual debe vivir en configuración, no escondida en código o en el CSV generado.
4. Se conservan valor original, valor corregido, fuente, motivo y fecha de la decisión.

No se propone recuperar mediciones desde estaciones vecinas, interpolación temporal, promedios, ceros ni
centroides. Esas operaciones crearían observaciones y cambiarían la distribución que después usa Gold.

## Validaciones recomendadas

Estas son validaciones por ejecutar cuando se implemente o cambie alguna regla; no se presentan aquí como
resultados ya obtenidos:

- Comprobar la identidad `leídas = procesadas + suma(descartadas_por_regla)` y desglosarla por estación.
- Verificar que cada corrección de ubicación conserve el valor Bronze original y cuente las filas
  afectadas.
- Listar estaciones con campos administrativos ausentes, UTM incompleto, datum no admitido, lat/lon
  parcial o más de una ubicación.
- Validar pares lat/lon contra límites plausibles del Perú y comprobar que la conversión inversa reproduce
  el UTM de origen dentro de una tolerancia acordada.
- Contrastar UBIGEO con departamento, provincia y distrito usando una versión fijada del catálogo oficial.
- Probar que variar `workers` produce Silver idéntico byte a byte y los mismos ganadores de duplicados.
- Verificar que toda hora inferida pertenece a un grupo o bloque físico de exactamente 24 posiciones y que
  `fecha_hora` usa siempre la convención de fin.
- Confirmar que una celda vacía de Silver corresponde a ausencia o descarte documentado, no a la cadena
  `"NA"`, `"null"` o a un cero agregado.
- Comparar conteos antes y después de cualquier cambio por regla, parámetro y estación; investigar cada
  diferencia antes de regenerar Gold.
- Añadir pruebas de regresión para `CA-HUAR-01`, una estación con múltiples ubicaciones y una estación sin
  filas Gold.

## Puntos por validar con el equipo

1. Datos faltantes: definir qué fuentes oficiales podemos utilizar para completar ubicaciones y datos administrativos, sin inventar información.
2. Ubicaciones inconsistentes: decidir cómo tratar estaciones que aparecen con distintas coordenadas, conservando las mediciones mientras se verifica su ubicación.
3. Trazabilidad: acordar cómo registrar las correcciones y los valores descartados para poder justificar los cambios realizados durante la limpieza.
