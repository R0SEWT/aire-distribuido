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
// # Qué queda para la PC4
//
// Este paquete no abre sockets ni calcula nada: no hay listener TCP, reparto de
// rangos ni bucle de Lloyd. Eso es el clúster, y se escribe en la PC4 sobre
// estas piezas.
package protocolo
