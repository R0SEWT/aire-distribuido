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
// Lo que falta viene de dos formas (docs/eda-procedencia.md §3.2-3.4): hasta 2021,
// como 0 (días enteros en 0, y horas sueltas en 0 dentro de días con datos);
// desde enero de 2022, como una fila vacía —todos los parámetros en «-»— que
// ocupa su hora en el bloque de 24 filas del día (274,866 filas, 99.94 % de 2022).
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
//     También anota el bloque físico de cada fila (ver Hora y convención).
//     Sin estado compartido y sin Mutex: búfer, cachés y tablas son del worker.
//   - Recolector (pipeline.go, recolector.go): recibe los parciales en cualquier
//     orden y los fusiona en orden de índice de bloque. Si un registro quedó
//     abierto al final de un bloque, rehace el siguiente desde su inicio
//     (cosedora); si un bloque físico cruza el borde, une sus dos partes (el
//     primero del bloque sigue al último del anterior si las claves coinciden).
//     Guarda los registros en orden de archivo.
//   - Horas (resolverHoras): con todos los días ya completos, da la hora por
//     posición, decide periodo «-», hora, reglas de valor, bloque_en_cero y
//     cero_como_faltante.
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
// filas contiguas en el archivo. En los 4,691 días del CSV con hora explícita
// completa, hora = posición en el 100 % de las filas. Una fila descartada por
// una regla de valor conserva su posición.
//
// Si el día del parámetro no está completo, se mira su bloque físico: las filas
// contiguas del archivo con la misma (estación, fecha, unidad, periodo, tipo de
// análisis), con valor o vacías. Si tiene exactamente filas_por_dia filas y sus
// filas con valor son de un solo parámetro, la hora es la posición en el bloque
// físico: la fila vacía de la plantilla de 2022 ocupa su hora (y después se
// descarta por sin_valor). Es lo que validó el notebook 04: contada así, la
// posición coincide con la hora explícita en el 100 % de las 165,912 filas que
// la traen, incluidas 53,328 de días con huecos; y recupera 19,710 lecturas que
// antes caían en la regla hora (docs/eda-procedencia.md §3.4). El tipo de
// análisis entra en la clave porque separa PM10 de PM2.5 en una fila vacía: sin
// él, uno de los bloques de 24 filas del CSV mezcla dos parámetros (la condición
// de un solo parámetro lo dejaría fuera igual). Si no, regla hora. La columna hora_inferida del
// silver vale 1 si alguna celda de la fila tiene hora reconstruida; el reporte
// cuenta aparte las filas con hora del bloque físico (hora_por_bloque_fisico).
//
// # Reglas
//
// Cada fila larga descartada se atribuye a la primera regla que falla, en este
// orden, así que leídas = procesadas + Σ descartadas por regla. Los nombres son
// las claves de descartadas_por_regla en reports/limpieza.json.
//
//	columnas                 la fila no tiene tantos campos como la cabecera
//	estacion                 estación vacía
//	periodo                  regla 1: periodo no horario (24 horas, móviles); «-» sin
//	                         hora explícita y con el día incompleto (o sin parámetro)
//	sin_valor                ninguna columna de parámetro con valor: el hueco de la
//	                         plantilla de 2022 (ocupa antes su posición en el bloque físico)
//	parametro                más de una columna de parámetro con valor
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
//	cero_como_faltante       un 0 exacto de un parámetro de cero_como_faltante en un
//	                         día con datos: hora sin dato
//	duplicado_unidad         regla 5/6: la misma lectura en otra unidad (ppb, mmHg)
//	duplicado_exacto         regla 5: misma (estación, parámetro, fecha-hora) y valor
//	duplicado_valor_distinto regla 5: misma clave, otro valor; se conserva el primero
//
// El 0 exacto de un contaminante es «sin dato», no una medición. Un cero
// verdadero de PM en una hora es físicamente improbable con un equipo automático:
// siempre hay partículas en el aire, y cerca del límite de detección el equipo da
// valores pequeños, no un 0 exacto. Y OEFA usa el 0 como faltante: su promedio
// de 24 h vale 0 si el día tiene menos de 18 horas no nulas (90-98 % de los días)
// y, si tiene 18 o más, es la media de las horas no nulas, sin contar las horas
// en 0 (docs/eda-procedencia.md §1 y §3.5). Por eso el día entero en 0 se
// descarta (bloque_en_cero, 762,744 lecturas) y, en los días con datos, cada
// hora en 0 de un contaminante también (cero_como_faltante: quedaban 3,551
// lecturas de PM en 0 y 413 horas del gold con PM2.5 = PM10 = 0, §3.2). La
// meteorología no: un viento de 0 m/s es calma (salvo el día entero en 0, que
// es bloque_en_cero), y la precipitación nunca, porque una hora seca es 0. En los
// gases un 0 también podría ser «bajo el límite de detección»: se trata como
// faltante, igual que OEFA, en vez de imputarlo.
//
// Sin sin_valor (antes contado en parametro), la hora por bloque físico y
// cero_como_faltante, la cascada es la de notebooks/aire_lab.py (más
// hora_no_en_punto, que en el CSV real da 0): con ella, sobre el CSV completo, los
// conteos por regla y por estación y el silver coincidían byte a byte con el
// prototipo del EDA (reports/eda_limpieza_prototipo.json). Esas tres piezas son
// posteriores: ni el prototipo ni el notebook 04 las aplican todavía, así que
// hoy difieren en parametro y sin_valor, hora, cero_como_faltante,
// duplicado_unidad y procesadas.
//
// Se marcan, sin descartar (marcadas_por_regla): las celdas cuyo valor
// conservado tuvo un duplicado con otro valor, las convertidas desde una unidad
// no canónica, las de hora inferida y las horas truncadas (política truncar); y
// las filas largas cuya ubicación se corrigió (ubicacion_corregida). Las filas anchas con
// PM2.5 > PM10 se cuentan aparte. vistos inventaría periodos, unidades por
// parámetro, filas por columna de parámetro y los textos no numéricos.
//
// La estación es NOMBRE_PUNTO normalizado (espacios colapsados, mayúsculas). Si
// un mismo nombre aparece con varias combinaciones de columnas de ubicación, se
// carga como una estación, se listan en ubicaciones_distintas y se avisa; la
// ubicación del reporte es la más frecuente. UBIGEO es texto de 6 dígitos (el
// CSV lo trae como número y pierde el cero inicial: 21801 es 021801).
//
// correcciones_ubicacion corrige un campo de ubicación mal cargado al registrar
// cada combinación: CA-HUAR-01 trae zona UTM 19, que la pondría en la Amazonía
// (-10.79, -70.29), pero su descripción la sitúa en Huaraucaca (Pasco), zona 18
// (docs/eda-procedencia.md §4). El valor del CSV queda en ubicacion_original
// (y en original de ubicaciones_distintas), las filas se cuentan en
// ubicacion_corregida y, si la estación no está en el archivo, se avisa. La
// ubicación del reporte trae además lat y lon en WGS84 (6 decimales), calculadas
// desde este, norte y zona con la inversa de UTM de Snyder (la de aire_lab.py),
// en el hemisferio sur: todas las estaciones están en el Perú. Si el datum no es
// WGS84 o falta una coordenada, no se calculan.
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
//	                              fecha. Opcionales: hora, periodo, unidad, tipo_analisis
//	                              (clave del bloque físico) y las de ubicación
//	                              (departamento, provincia, distrito, ubigeo, este, norte,
//	                              altitud, zona, datum)
//	parametros_en_columnas        columna real → canónico (pm2_5, pm10, no2, so2, co, o3,
//	                              h2s, temperatura, humedad, viento_velocidad,
//	                              viento_direccion, presion, precipitacion, radiacion)
//	periodos_horarios             periodos que son horarios ("1 hora")
//	periodos_horarios_por_grupo   horarios solo con hora explícita o con el día completo ("-")
//	formatos_fecha, formatos_hora layouts de Go, en orden (hora: "15:04:05" y "15:04" por defecto)
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
//	cero_como_faltante            canónicos cuyo 0 exacto es «sin dato» hora a hora: los
//	                              contaminantes (pm2_5, pm10, so2, co, o3, h2s); nunca la
//	                              precipitación ni la meteorología
//	correcciones_ubicacion        estación → {columna de ubicación → valor}; el valor puede
//	                              ir como número o como texto ({"CA-HUAR-01": {"zona": 18}})
//
// ConfigPorDefecto es el formato real, verificado sobre el CSV completo, y el de
// las dos muestras de data/sample/; configs/limpieza.json arranca igual. El modo
// del primer borrador (una columna de parámetro y una de valor) se retiró: el
// CSV real no es así. Sus claves (parametros, columnas.parametro y
// columnas.valor) se aceptan vacías, porque el laboratorio en Python las escribe.
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
