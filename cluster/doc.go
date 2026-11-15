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
// falla (no está el gold, otra huella, un rango que se sale) responde ERROR con
// un texto legible y sigue atendiendo: decide el coordinador.
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
//
// # Local SGD del mapa nacional (protocolo v2)
//
// El coordinador del entrenamiento del mapa
// (docs/specs/2026-11-14-mapa-nacional-design.md, §5; cmd/mapa-coord) usa las
// mismas piezas con la versión 2 del protocolo. EsperarNodosVersion junta los
// nodos que se presentan con un HOLA v2. LeerManifiesto lee manifiesto.json
// (por HTTP o de una ruta) para sacar los días (los dias/AAAA-MM-DD.npy que
// lista con su sha256) y su huella. RepartirDias agrupa los días en bloques de 7, los baraja con la
// semilla y los reparte en ronda, y CargarTarea le manda a cada nodo su TAREA
// a la vez. SGDLocal corre las rondas: cada una es una RondaLocal (fan-out del
// PESOS, fan-in de los LOCAL con un plazo común) y Promediar, el promedio de
// los pesos ponderado por las muestras de cada nodo, acumulado en float64 en
// orden de nodo y bajado a float32: determinista, como la fusión del K-Means.
// Los pesos son un vector opaco: el coordinador no sabe de capas. Si un nodo
// cae, no responde a tiempo o devuelve pesos no finitos, la ronda se aborta
// con un error que lo nombra; el punto de control es la última ronda completa.
package cluster
