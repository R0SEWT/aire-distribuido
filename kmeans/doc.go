// Package kmeans implementa K-Means (Lloyd) sobre las lecturas de calidad del
// aire, en versión secuencial y concurrente, usando solo la biblioteca estándar.
//
// Portado del Trabajo Parcial del mismo equipo, upc.edu.pe/concurrente/tp/kmeans.
// El contrato numérico se conserva tal cual (tp/docs/kmeans.md en el repo del
// curso): distancia euclídea al cuadrado, desempate por el centroide de menor
// índice, k-means++ con semilla fija ejecutado una vez, inercia reportada con
// los centroides finales, clusters vacíos que conservan su centroide y
// reducción de los parciales en orden de chunk.
//
// # Las tres piezas que la PC4 distribuye
//
// Una iteración de Lloyd es Parciales sobre cada tramo de filas, Fusionar de
// los parciales en un orden fijo y Actualizar de los centroides. Secuencial es
// un solo tramo; Concurrente reparte chunks entre un pool de goroutines y
// fusiona en orden de chunk; el coordinador de la PC4 repartirá particiones
// entre nodos y fusionará en orden de nodo. Es el mismo código: solo cambia
// quién llama a Parciales.
//
// # La receta: configs/kmeans.json
//
// La calibración en Colab congela la receta y Go solo la lee (Receta,
// LeerReceta). Go no decide ni las features ni k. Los campos:
//
//   - features: parámetros canónicos que entran a la distancia, en el orden de
//     las columnas del gold (pm2_5, pm10, no2, so2, co, o3, temperatura,
//     humedad, viento_velocidad, viento_direccion, presion, precipitacion,
//     radiacion, y la derivada fraccion_fina = pm2_5/pm10). La estación, la hora
//     y el mes nunca son features: se usan después, en el perfil de régimen.
//   - log1p: las features a las que el cargador aplicó log1p antes del z-score
//     (las colas largas de PM). Subconjunto de features.
//   - nivel: "absoluto" (un z-score con la media y la desviación de todo el
//     gold: un régimen es un nivel de aire, el mismo en todo el país) o
//     "estacion" (cada estación con su propia media y desviación: un régimen es
//     un episodio respecto de lo normal de ese lugar). Con "estacion" los
//     centroides no tienen unidades absolutas; los perfiles por régimen sí,
//     porque se calculan fila por fila con la estadística de su estación.
//   - k: cantidad de regímenes.
//   - semilla: semilla de k-means++ (math/rand con rand.NewSource, como en el TP).
//   - semillas: cuántos arranques se prueban (0 o ausente = 1, el comportamiento
//     de antes). Con N, MejorDeSemillas corre k-means++ y Lloyd con semilla,
//     semilla+1, …, semilla+N-1 y se queda con el de menor inercia final; si dos
//     empatan bit a bit, con la semilla menor. Lloyd converge a mínimos locales:
//     en el gold sintético, la semilla 42 sola parte un régimen plantado en dos y
//     junta otros dos, y 10 semillas lo recuperan (scikit-learn usa n_init = 10).
//     Es determinista para cualquier W y cualquier orden de ejecución. Las N
//     corridas son el paralelismo por semillas de la spec §7 y corren a la vez
//     (cmd/entrenar -paralelas; 0 = automático, ver RepartirSemillas). Medido en
//     la laptop (Core Ultra 7 255U, 14 hilos, P y E cores) con 10 semillas,
//     mediana de 5 corridas intercaladas, 510 000 filas sintéticas (102 000 entre
//     paréntesis): una semilla 0,43 s (0,17); diez en serie con todo el pool
//     3,16 s (1,01); 2 a la vez 2,60 (0,76); 3 a la vez 2,45 (0,66); 5 a la vez
//     2,71 (0,62); las 10 a la vez 3,40 (0,82). El automático elige 3.
//   - tolerancia: criterio de parada; se usa como tol_abs y tol_rel del TP:
//     max_j ||mu_j(t) - mu_j(t-1)||inf <= tol * (1 + max_j ||mu_j(t-1)||inf).
//     En el espacio estandarizado los centroides son de orden 1, así que 1e-6
//     es, en la práctica, un desplazamiento de una millonésima de desviación.
//   - max_iter: tope de iteraciones de Lloyd si no converge antes.
//   - workers: goroutines del pool concurrente; 0 = runtime.GOMAXPROCS(0), que
//     desde Go 1.25 respeta el límite de CPU del contenedor (NumCPU no).
//   - chunk: filas por chunk. Con el chunk fijo el resultado es idéntico bit a
//     bit para cualquier cantidad de workers; cambiar el chunk cambia el orden
//     de la suma y por lo tanto los últimos dígitos.
//
// # El gold y la estandarización
//
// LeerGold lee data/gold/oefa_features.csv (cabecera estacion,fecha_hora y las
// features de la receta, en orden) y devuelve un Gold: la matriz Datos más la
// estación y la fecha-hora de cada fila. LeerEstandarizacion lee
// reports/estandarizacion.json, con la media y la desviación (en escala log1p
// donde corresponde) que el cargador usó; con eso Invertir devuelve cualquier
// fila o centroide a unidades originales.
package kmeans
