// Package protocolo define los mensajes entre el coordinador y los nodos de
// cómputo del K-Means distribuido, su codificación y la barrera de una
// iteración. El diseño completo está en docs/diseno.md, sección 4. La versión
// 2, para el entrenamiento del mapa nacional con local SGD, está al final.
//
// # Formato en el cable
//
// Cada mensaje es un objeto JSON en una sola línea terminada en '\n' (JSON
// delimitado por saltos de línea), con un campo "tipo" que dice cuál es:
//
//	{"tipo":"HOLA","version":1,"nodo":"gorgo","workers":11}
//	{"tipo":"CARGAR","rango":{"desde":0,"hasta":250000},"huella":"sha256:…"}
//	{"tipo":"LISTO","huella":"sha256:…","n":250000}
//	{"tipo":"ITERAR","iter":7,"centroides":[0.12,-1.3,…]}
//	{"tipo":"PARCIAL","iter":7,"sumas":[…],"conteos":[…],"inercia":1234.5}
//	{"tipo":"FIN"}
//
// Se eligió texto y no binario porque el volumen por ronda es chico (unos
// kilobytes: el costo de la barrera es la latencia, no el ancho de banda), se
// puede leer con tcpdump o nc durante una demo, y encoding/json escribe cada
// float64 con la representación más corta que se relee al mismo valor: la ida y
// vuelta por el cable no cambia ningún bit, así que el texto no rompe el
// determinismo de la fusión.
//
// # Sesión
//
// El nodo abre la conexión y se presenta (HOLA): Colab y los teléfonos no
// aceptan conexiones entrantes. El coordinador le asigna un rango de filas
// (CARGAR) y el nodo responde LISTO cuando lo tiene en memoria. Después, cada
// iteración es una ronda ITERAR → PARCIAL con todos los nodos (Ronda), que
// termina cuando todos respondieron o vence el plazo. FIN cierra la
// corrida y el nodo vuelve a esperar un CARGAR. PING/PONG mide la latencia y
// comprueba que el nodo sigue vivo entre corridas. ERROR, en cualquier sentido,
// aborta lo que se estaba haciendo con un texto legible.
//
// # La huella y el rango de CARGAR
//
// La huella identifica el gold: "sha256:" seguido del sha256 del archivo en 64
// dígitos hexadecimales en minúscula, el mismo valor que reports/kmeans.json
// guarda en gold.sha256. Un nodo calcula el sha256 de su copia y, si no
// coincide, responde ERROR.
//
// El rango es de filas de la tabla ancha en el orden del gold, contando desde
// 0 y sin la cabecera. Decisión de la PC4 (docs/diseno.md, §4.3), réplicas
// virtuales: la tabla puede ser más larga que el gold. Con N filas en el gold,
// la fila i de la tabla es la fila i mod N, así que un CARGAR con Hasta > N
// pide repetir el gold y el nodo arma su rango tomando cada fila con módulo.
// Sirve para medir el clúster con r × N filas sin escribir un CSV r veces más
// grande, y no cambia el formato de ningún mensaje: un rango dentro de [0, N)
// es exactamente el de siempre. La huella sigue siendo la del gold real, y
// LISTO responde n = Hasta − Desde, las filas virtuales que el nodo tiene en
// memoria. Todo nodo, en Go o en otro lenguaje, tiene que aceptar Hasta > N.
//
// # Lo que no viaja
//
// El chunk con que el nodo parte su rango no está en el protocolo: es un flag
// del nodo. Solo cambia los últimos bits de sus sumas (el orden en que suma),
// no el modelo; con el chunk de configs/kmeans.json y un solo nodo, el K-Means
// distribuido es el de cmd/entrenar bit a bit.
//
// # Dónde está el clúster
//
// Este paquete no abre sockets ni calcula nada. El listener, el nodo, el
// reparto de rangos y el bucle de Lloyd están en cluster/, sobre estas piezas;
// los comandos son cmd/nodo, cmd/distribuido y cmd/smoke.
//
// # Versión 2: el mapa nacional (local SGD)
//
// El entrenamiento distribuido del mapa de PM2.5 y PM10
// (docs/specs/2026-11-14-mapa-nacional-design.md, §5.3) extiende la v1 sin
// romperla. Los mensajes de control siguen siendo JSON en una línea; un
// mensaje con carga binaria lleva en la línea el campo "bytes": N y le siguen
// EXACTAMENTE N bytes crudos después del '\n', sin separador al final. Un
// mensaje sin "bytes" es uno de la v1 y se lee como siempre. Solo PESOS y LOCAL
// llevan carga, y es un vector de n float32 en little-endian (bytes = 4·n):
//
//	{"tipo":"HOLA","version":2,"nodo":"pc-07","workers":1,"gpu":"NVIDIA T1000 8GB","vram_gb":8}
//	{"tipo":"TAREA","huella":"sha256:…","tarea":"mapa-1","manifiesto":"http://…/manifiesto.json","dias":["2018-01-08",…],"validacion":["CA-SB-01"],"hiper":{…}}
//	{"tipo":"LISTO","tarea":"mapa-1","muestras":1234}
//	{"tipo":"PESOS","n":8000000,"ronda":1,"pasos":200,"bytes":32000000}  + 32 000 000 bytes
//	{"tipo":"LOCAL","n":8000000,"ronda":1,"bytes":32000000,"muestras":1600,"perdida":0.21,"perdida_val":0.25,"ms_computo":81234.5}  + 32 000 000 bytes
//
// MaxLinea sigue rigiendo la línea JSON y la carga tiene su propio tope,
// MaxCarga (256 MB). Una cabecera que anuncia más, o una carga que se corta
// antes de los N bytes, deja el Decodificador inservible con un error claro:
// el flujo ya no se puede resincronizar. Un plazo que vence a mitad de la carga
// no pierde nada, igual que a mitad de una línea. Una carga en un tipo que no
// la lleva se consume y se rechaza, y el flujo sigue sano.
//
// La sesión: el nodo llama y se presenta con un HOLA de la versión 2, que
// agrega la GPU que va a usar ("" si entrena en CPU) y su memoria en GB. El
// coordinador le manda su TAREA: id, el manifiesto (la URL o la ruta tal como
// se le pasó al coordinador), su huella ("sha256:" y el sha256 del archivo en
// hexadecimal en minúscula, que el nodo compara con el del manifiesto que
// baja), sus días (AAAA-MM-DD, en orden), las estaciones de validación (que no
// entran en su pérdida) y los hiperparámetros, un objeto JSON que el
// coordinador no interpreta. El nodo baja sus días, los verifica y responde
// LISTO con la tarea y las horas con etiqueta que tiene. Después, cada ronda
// es PESOS → LOCAL con todos los nodos: PESOS trae la ronda, los pasos H y los
// pesos globales; el nodo entrena H pasos desde ahí (su optimizador conserva su
// estado entre rondas) y responde LOCAL con la misma ronda, sus pesos, las
// muestras que vio en la ronda (su peso en el promedio), sus pérdidas de
// entrenamiento y de validación, y los ms que tardó. FIN, PING, PONG y ERROR
// son los de la v1. Como ITERAR, la ronda crece en toda la vida de la conexión
// y un LOCAL de una ronda ya abortada se descarta.
//
// Los campos que valen cero no se escriben (por ejemplo, "validacion" sin
// estaciones o "hiper" sin hiperparámetros): quien lee tiene que darlos por
// vacíos. El decodificador rechaza los campos desconocidos y los valores no
// finitos, así que un nodo no puede mandar campos de más ni NaN.
//
// Qué versión acepta cada coordinador. Cada uno habla una sola, porque los
// nodos de una y otra no entienden los mismos mensajes: el del K-Means
// (cmd/distribuido, cmd/smoke; RecibirHola) solo acepta HOLA de la versión 1, y
// el del mapa (cmd/mapa-coord; RecibirHolaVersion con Version2) solo de la 2. A
// un nodo de la otra versión se le responde ERROR con las dos versiones en el
// texto y no entra al clúster. Version sigue siendo 1: los nodos del K-Means
// (cmd/nodo, lab/gpu/nodo_gpu.py) no cambian.
//
// Como en la v1, el fan-out y el plazo común están en cluster/
// (cluster.RondaLocal sobre EnviarPesos y RecibirLocal), y el promedio
// también (cluster.Promediar): este paquete solo define los mensajes, su
// codificación y lo que corresponde a un nodo.
package protocolo
