// Package datos es la parte A del Trabajo Final: el cargador concurrente del CSV
// de calidad del aire de OEFA. Lee el CSV por bloques con goroutines, lo limpia,
// le da hora a cada lectura, lo pivota a una tabla ancha (una fila por estación
// y fecha-hora) y arma el gold estandarizado que consume el K-Means. Solo usa la
// biblioteca estándar.
//
// # El CSV real (spec §4.1)
//
// 2,883,877 registros, 36 columnas, «;», textos entre comillas, UTF-8 sin BOM,
// fin de línea CRLF. Una columna por parámetro (PM10, PM2.5, DIOXIDO_AZUFRE, ...),
// pero cada fila trae un solo valor y las demás «-»; «-» y «NA» son vacíos. La
// hora (HORA_INICIO) es el inicio del promedio horario, como «1899-12-31 HH:00:00»
// (artefacto de Excel; «1899-12-31» solo es la medianoche) y falta en el 94 % de
// las filas: entonces se reconstruye por la posición de la fila en su día. Cien
// registros tienen saltos de línea dentro de un campo entrecomillado.
//
// # Pipeline
//
//	productor ──índices──▶ W workers ──parciales──▶ recolector ──▶ horas ──▶ pivot por estación ──▶ gold
//
// Cada pieza:
//
//   - Lector: la cabecera se lee una vez (quitando el BOM, si hay). El resto se
//     parte en bloques de B bytes (64 MB por defecto) alineados al siguiente salto
//     de línea; los índices van por un canal, con un semáforo que deja a lo sumo
//     2W bloques leídos sin fusionar (cota de memoria).
//   - Workers (bloque.go): cada goroutine lee su bloque con ReadAt sobre el mismo
//     *os.File (seguro para uso concurrente), une las líneas de un registro
//     multilínea (cantidad impar de comillas), y valida cada fila (validarFila).
//     Sin estado compartido y sin Mutex: búfer, cachés y tablas son del worker.
//   - Recolector (pipeline.go, recolector.go): recibe los parciales en cualquier
//     orden y los fusiona en orden de índice de bloque. Si un registro quedó
//     abierto al final de un bloque, rehace el siguiente desde su inicio
//     (cosedora). Guarda los registros en orden de archivo.
//   - Horas (resolverHoras): con todos los días ya completos, da la hora por
//     posición, decide periodo «-», hora, reglas de valor y bloque_en_cero.
//   - Pivot por estación (pivotar): las celdas de dos estaciones nunca chocan,
//     así que cada estación es una tarea del pool; dentro de ella los registros
//     se aplican en orden de archivo. Las filas se ordenan por fecha_hora y las
//     estaciones por nombre: nunca se itera un mapa para escribir.
//   - Gold (estandarizar.go): features derivadas y log1p, filas completas
//     (regla 8), media y desviación con Welford por chunk y fusión de Chan en
//     orden de chunk, y z-score; las tres fases, por chunks en paralelo.
//
// fases_s del reporte separa lectura_parseo (paralela; contiene a fusion, serial
// y solapada), horas y tabla (seriales), pivot (paralela por estación) y
// gold_estandarizacion, para el análisis de Amdahl.
//
// # Determinismo
//
// Con el mismo archivo, la misma config y el mismo chunk, el silver, el gold y
// los conteos son idénticos byte a byte para cualquier W y cualquier B: todas
// las reducciones van en orden de bloque, de archivo, de estación o de chunk,
// nunca en orden de llegada. Los tests lo comparan contra un oráculo secuencial
// (procesar y fusionar sin goroutines). Cambiar el chunk sí puede mover el último
// bit de la media y la desviación (otro árbol de suma). Los productos de Welford
// van envueltos en float64(...) para que arm64 no los funda en un FMA.
//
// # Hora y convención
//
// fecha_hora es el fin del intervalo horario (convencion_hora = "fin", la de
// OEFA): la lectura de 00:00 a 01:00 se fecha a las 01:00 y la de 23:00 a 24:00,
// a las 00:00 del día siguiente. Con hora explícita, fecha_hora = HORA_INICIO +
// 1 h. Sin hora, la fila toma su posición (0 a 23) dentro de su día —(estación,
// parámetro, fecha, unidad, periodo)— si el día tiene exactamente filas_por_dia
// filas contiguas en el archivo; si no, regla hora. En los 4,691 días del CSV con
// hora explícita completa, hora = posición en el 100 % de las filas. Una fila
// descartada por una regla de valor conserva su posición. La columna
// hora_inferida del silver vale 1 si alguna celda de la fila tiene hora
// reconstruida.
//
// # Reglas
//
// Cada fila larga descartada se atribuye a la primera regla que falla, en este
// orden, así que leídas = procesadas + Σ descartadas por regla. Los nombres son
// las claves de descartadas_por_regla en reports/limpieza.json.
//
//	columnas                 la fila no tiene tantos campos como la cabecera
//	estacion                 estación vacía
//	sin_valor                ninguna columna de parámetro con valor
//	parametro                más de una columna de parámetro con valor
//	periodo                  regla 1: periodo no horario (24 horas, móviles); «-» sin
//	                         hora explícita y con el día incompleto
//	fecha                    regla 2: fecha no parseable
//	fecha_fuera_de_rango     regla 2: fuera de [fecha_desde, fecha_hasta)
//	hora_no_en_punto         regla 2: hora con minutos o segundos (política descartar)
//	hora                     regla 2: hora ilegible, o sin hora y el día incompleto
//	valor_no_numerico        regla 3: texto (<LD, ND, ---), NaN o infinito; no se imputa
//	centinela                regla 3: un código del equipo (-999, 9999, ...), sobre el crudo
//	unidad                   regla 6: unidad que no figura para ese parámetro
//	negativo                 regla 3: negativo en un parámetro que no los admite
//	fuera_de_rango           regla 4: fuera del rango físico, ya en la unidad canónica
//	bloque_en_cero           un día entero en 0 exacto: equipo sin medir
//	duplicado_unidad         regla 5/6: la misma lectura en otra unidad (ppb, mmHg)
//	duplicado_exacto         regla 5: misma (estación, parámetro, fecha-hora) y valor
//	duplicado_valor_distinto regla 5: misma clave, otro valor; se conserva el primero
//
// Se marcan, sin descartar (marcadas_por_regla): las celdas cuyo valor
// conservado tuvo un duplicado con otro valor, las convertidas desde una unidad
// no canónica y las horas truncadas (política truncar). Las filas anchas con
// PM2.5 > PM10 se cuentan aparte. vistos inventaría periodos, unidades por
// parámetro, filas por columna de parámetro y los textos no numéricos.
//
// La estación es NOMBRE_PUNTO normalizado (espacios colapsados, mayúsculas). Si
// un mismo nombre aparece con varias combinaciones de columnas de ubicación, se
// carga como una estación, se listan en ubicaciones_distintas y se avisa.
//
// # configs/limpieza.json
//
// Todo lo que depende del formato del CSV y los umbrales vive aquí. Un campo
// desconocido es un error. Nombres de columnas, periodos y unidades se comparan
// normalizados: minúsculas, sin tildes, µ y μ como u, º como °, superíndices
// como dígitos y espacios colapsados («µg/m³» = «ug/m3»).
//
//	_supuestos                    texto libre: qué se supuso o verificó
//	delimitador                   «;», «,», «\t», «|»...; "auto" lo detecta en la cabecera
//	codificacion                  "utf-8" (el BOM se quita), "latin1" o "windows-1252"
//	separador_decimal             "." o ","
//	zona_horaria                  nombre IANA; "America/Lima" (UTC-5, sin horario de verano)
//	valores_vacios                textos que son «sin dato» ("-", "NA"; "" siempre lo es)
//	columnas                      columna lógica → nombre real. Obligatorias: estacion y
//	                              fecha. Opcionales: hora, periodo, unidad y las de
//	                              ubicación (departamento, provincia, distrito, ubigeo,
//	                              este, norte, altitud, zona, datum)
//	parametros_en_columnas        columna real → canónico (pm2_5, pm10, no2, so2, co, o3,
//	                              h2s, temperatura, humedad, viento_velocidad,
//	                              viento_direccion, presion, precipitacion, radiacion)
//	periodos_horarios             periodos que son horarios ("1 hora")
//	periodos_horarios_por_grupo   horarios solo con hora explícita o con el día completo ("-")
//	formatos_fecha, formatos_hora layouts de Go, en orden
//	hora_prefijo_ignorar          prefijo de la hora que se quita ("1899-12-31")
//	hora_solo_prefijo             "medianoche" (default) o "sin_hora": qué es el prefijo solo
//	hora_explicita                "inicio" (default) o "fin": qué marca HORA_INICIO
//	hora_no_en_punto              "descartar" (default) o "truncar"
//	hora_por_posicion             {"activar", "filas_por_dia", "convencion": "fin"|"inicio"};
//	                              la convención vale también para las horas explícitas
//	fecha_desde, fecha_hasta      [desde, hasta), aaaa-mm-dd o RFC3339
//	centinelas                    valores crudos que son códigos del equipo
//	admiten_negativos             canónicos que pueden ser negativos (temperatura)
//	rangos                        canónico → [mínimo, máximo] en la unidad canónica
//	unidades                      canónico → {unidad → factor a la canónica}. La de factor 1
//	                              es la canónica y se prefiere; un parámetro sin entrada
//	                              no se chequea (viento_direccion, que trae «°c» por error)
//	bloques_en_cero_como_faltante canónicos cuyo día entero en 0 se descarta
//
// ConfigPorDefecto es el formato real, verificado sobre el CSV completo, y el de
// las dos muestras de data/sample/; configs/limpieza.json arranca igual. El modo
// del primer borrador (una columna de parámetro y una de valor) se retiró: el
// CSV real no es así.
//
// # configs/kmeans.json (solo lectura)
//
// Lo define el paquete del K-Means. El cargador usa features (canónicos o
// fraccion_fina = pm2_5/pm10, NaN si pm10 ≤ 0 o falta, recortada a [0, 1]),
// log1p, nivel ("absoluto" o "estacion") y chunk. Si no existe, RecetaProvisional.
//
// # Salidas
//
//   - data/silver/oefa_ancho.csv: estacion,fecha_hora, los canónicos presentes en
//     orden alfabético y hora_inferida (0/1); vacío = faltante; fecha_hora en
//     RFC3339 con -05:00; filas ordenadas por estación y fecha_hora; coma; valores
//     con strconv.FormatFloat(x, 'g', -1, 64). Un valor convertido es el producto
//     exacto en float64 (10 ppb × 2.62 = 26.200000000000003).
//   - data/gold/oefa_features.csv: estacion,fecha_hora y las features de la
//     receta, transformadas y estandarizadas; solo filas completas.
//   - reports/limpieza.json (ReporteLimpieza) y reports/estandarizacion.json
//     (ReporteEstandarizacion; desviación poblacional, ddof = 0, como el TP).
//   - Dataset en memoria: X por filas, N, D, más Estacion y Hora para agregar
//     después del clustering, con el mismo contrato que kmeans.Datos del TP.
package datos
