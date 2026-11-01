// Package datos es la parte A del Trabajo Final: el cargador concurrente del CSV
// de calidad del aire de OEFA. Lee el CSV largo (una fila por estación,
// parámetro y fecha-hora) por bloques con goroutines, lo limpia, lo pivota a una
// tabla ancha (una fila por estación y fecha-hora) y arma el gold estandarizado
// que consume el K-Means. Solo usa la biblioteca estándar.
//
// # Pipeline
//
//		lector ──índices de bloque──▶ canal ──▶ W workers ──parciales──▶ recolector ──▶ tabla ancha ──▶ gold
//
//	  - Lector: la cabecera se lee una vez (quitando el BOM). El resto del archivo
//	    se parte en bloques de B bytes (64 MB por defecto) alineados al siguiente
//	    salto de línea; los índices de bloque van por un canal.
//	  - Workers: cada goroutine lee su bloque con ReadAt sobre el mismo *os.File
//	    (seguro para uso concurrente), parsea las filas, aplica las reglas 1 a 6 y
//	    acumula en su propio mapa (estación, fecha-hora) → vector de parámetros.
//	    Sin estado compartido y sin Mutex: cada worker tiene su búfer y sus cachés.
//	  - Recolector: recibe los parciales en cualquier orden y los fusiona en orden
//	    de índice de bloque (reglas 5 y 7). Por eso «el primero» de un duplicado es
//	    el primero del archivo y el resultado es el mismo para cualquier W.
//	  - Tabla ancha (silver): se vuelca ordenada por (estación, fecha-hora), nunca
//	    iterando el mapa, cuyo orden en Go es aleatorio.
//	  - Gold: features derivadas y log1p, filas completas (regla 8), media y
//	    desviación con Welford por chunk y fusión de Chan en orden de chunk, y
//	    z-score; las tres fases, por chunks en paralelo.
//
// # Determinismo
//
// Con el mismo archivo, la misma config y el mismo chunk, el silver, el gold y
// los conteos son idénticos byte a byte para cualquier W y cualquier B. W no
// cambia nada porque todas las reducciones se hacen en orden de bloque o de
// chunk, nunca en orden de llegada. B no cambia nada porque los duplicados se
// clasifican contra el valor que se conserva, no contra el primero del bloque.
// Cambiar el chunk sí puede mover el último bit de la media y la desviación
// (otro árbol de suma en punto flotante).
//
// # Reglas (spec §4.2)
//
// Cada fila larga descartada se atribuye a la primera regla que falla, en este
// orden, así que leídas = procesadas + Σ descartadas por regla. Los nombres son
// las claves de descartadas_por_regla en reports/limpieza.json.
//
//	columnas                 la fila no tiene tantos campos como la cabecera
//	estacion                 estación vacía
//	periodo                  regla 1: el periodo no es horario (diario, mensual...)
//	parametro                el parámetro no está en el mapeo a canónicos
//	fecha                    regla 2: fecha y hora no parseables con ningún formato
//	fecha_fuera_de_rango     regla 2: fuera de [fecha_desde, fecha_hasta)
//	valor_no_numerico        regla 3: vacío, texto, NaN o infinito
//	centinela                regla 3: un código del equipo (-999, 9999, ...)
//	unidad                   regla 6: unidad que no figura para ese parámetro
//	negativo                 regla 3: negativo en un parámetro que no los admite
//	fuera_de_rango           regla 4: fuera del rango físico (se cuenta también por parámetro)
//	duplicado_exacto         regla 5: misma (estación, parámetro, fecha-hora) y mismo valor
//	duplicado_valor_distinto regla 5: misma clave, otro valor; se conserva el primero
//
// La regla 6 normaliza antes del rango: el valor se multiplica por el factor de
// su unidad (y por el del mapeo del parámetro, si lo tiene) y el rango físico se
// compara ya en la unidad canónica. Los centinelas se comparan sobre el valor
// crudo. Además se marcan, sin descartar: las celdas cuyo valor conservado tuvo
// un duplicado con otro valor, y las filas anchas con PM2.5 > PM10.
//
// # configs/limpieza.json
//
// Todo lo que depende del formato real del CSV y de los umbrales vive aquí, no
// en el código. Un campo desconocido es un error (un umbral mal escrito no puede
// quedar en cero en silencio). Los nombres de columnas, parámetros, periodos y
// unidades se comparan normalizados: minúsculas, sin tildes, µ y μ como u,
// superíndices como dígitos y espacios colapsados («µg/m³» = «ug/m3»).
//
//	_supuestos          lista de texto libre: qué se supuso y falta verificar
//	delimitador         ";", ",", "\t", "|" u otro carácter; "auto" (o vacío) lo
//	                    detecta en la cabecera
//	codificacion        "utf-8" (por defecto; el BOM se quita), "latin1" o "windows-1252"
//	separador_decimal   "." o ","
//	zona_horaria        nombre IANA; "America/Lima" (UTC-5, sin horario de verano)
//	columnas            columna lógica → nombre real en la cabecera. Obligatorias:
//	                    estacion, fecha, parametro, valor. Opcionales: hora (si la
//	                    hora viene aparte, se une a la fecha con un espacio),
//	                    periodo (sin ella no se aplica la regla 1), unidad (sin
//	                    ella la regla 6 solo usa el factor del mapeo), y las de
//	                    ubicación (departamento, provincia, distrito, ubigeo,
//	                    este, norte, altitud, zona, datum), que se guardan una vez
//	                    por estación para la regla 9 (TB2); si faltan, solo se avisa
//	parametros          nombre real → canónico, como texto ("pm10") u objeto
//	                    ({"canonico": "co", "factor": 1000}) cuando la unidad
//	                    viene en el nombre. Canónicos: pm2_5, pm10, no2, so2, co,
//	                    o3, temperatura, humedad, viento_velocidad,
//	                    viento_direccion, presion, precipitacion, radiacion
//	periodos_horarios   valores de la columna de periodo que cuentan como horario
//	formatos_fecha      layouts de Go que se prueban en orden
//	fecha_desde         primer instante válido (aaaa-mm-dd o RFC3339), inclusive
//	fecha_hasta         primer instante no válido, exclusive
//	centinelas          valores crudos que son códigos del equipo
//	admiten_negativos   canónicos que pueden ser negativos (temperatura)
//	rangos              canónico → [mínimo, máximo], inclusivos, en la unidad canónica
//	unidades            canónico → {unidad real → factor a la canónica}; un
//	                    parámetro sin entrada no se chequea
//
// ConfigPorDefecto devuelve los supuestos del 31/10, que son exactamente el
// formato de la muestra sintética de data/sample/. configs/limpieza.json
// arranca igual y es lo que se ajusta con el archivo real.
//
// # configs/kmeans.json (solo lectura)
//
// Lo define el paquete del K-Means. El cargador usa features (canónicos o
// fraccion_fina = pm2_5/pm10, NaN si pm10 ≤ 0 o falta, recortada a [0, 1]),
// log1p (subconjunto de features, se aplica antes de estandarizar), nivel
// ("absoluto" o "estacion") y chunk. Si el archivo no existe se usa
// RecetaProvisional.
//
// # Salidas
//
//   - data/silver/oefa_ancho.csv: estacion,fecha_hora, y los canónicos presentes
//     en orden alfabético; vacío = faltante; fecha_hora en RFC3339 con -05:00;
//     filas ordenadas por estación y fecha_hora; coma como delimitador; valores
//     con strconv.FormatFloat(x, 'g', -1, 64).
//   - data/gold/oefa_features.csv: estacion,fecha_hora, y las features de la
//     receta en su orden, transformadas y estandarizadas; solo filas completas.
//   - reports/limpieza.json (ReporteLimpieza) y reports/estandarizacion.json
//     (ReporteEstandarizacion; desviación poblacional, ddof = 0, como el TP).
//   - Dataset en memoria: X por filas, N, D, más Estacion y Hora para agregar
//     después del clustering, con el mismo contrato que kmeans.Datos del TP.
package datos
