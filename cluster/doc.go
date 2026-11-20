// Package cluster es el K-Means distribuido de la PC4 sobre las piezas de
// protocolo/ y kmeans/: el nodo de cómputo, la espera de nodos del coordinador,
// el reparto en rangos contiguos y Lloyd con una barrera de red por iteración.
// El diseño está en docs/diseno.md, sección 4.
//
// # Lado del nodo
//
// Nodo llama al coordinador, se presenta con HOLA y atiende la sesión: CARGAR
// lee su rango del gold local después de comprobar que el sha256 del archivo es
// la huella pedida y responde LISTO; cada ITERAR calcula el parcial de su rango
// con su propio pool de workers (kmeans.ParcialesPorChunks) y responde PARCIAL;
// FIN cierra la corrida y libera la partición; PING se responde siempre. Si algo
// falla (no está el gold, otra huella, un rango de más filas que su tope,
// Nodo.MaxFilas) responde ERROR con un texto legible y sigue atendiendo: decide
// el coordinador.
//
// # Lado del coordinador
//
// EsperarNodos junta los nodos que se presentan, Repartir les asigna rangos
// contiguos iguales en orden de nodo, Cargar se los manda a todos a la vez y
// Lloyd corre las iteraciones: cada una es una protocolo.Ronda (fan-out del
// ITERAR, fan-in de los PARCIAL con un plazo común), kmeans.Fusionar de los
// parciales en orden de nodo y kmeans.CerrarIteracion, el mismo código que
// cierra cada iteración en una sola máquina. Si un nodo cae o no responde a
// tiempo, la ronda se aborta con un error que lo nombra y no hay modelo:
// Abortar les avisa a los demás y Terminar cierra una corrida que salió bien.
//
// cmd/nodo y cmd/distribuido son los comandos; cmd/smoke usa la misma espera
// de nodos. La API de la parte C puede usar este paquete tal cual.
package cluster
