// Package protocolo define los mensajes entre el coordinador y los nodos de
// cómputo del K-Means distribuido, su codificación y la barrera de una
// iteración. El diseño completo está en docs/diseno.md, sección 4.
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
package protocolo
