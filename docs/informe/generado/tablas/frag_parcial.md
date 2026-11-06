<!-- Generado por tablas.py desde protocolo/mensaje.go. No editar a mano. -->

::: {#lst-parcial}
```go
// Los tipos de mensaje. La flecha dice quién lo manda: C es el coordinador y N
// un nodo.
const (
    Hola    Tipo = "HOLA"    // N → C: el nodo se presenta (versión, nombre, workers).
    Cargar  Tipo = "CARGAR"  // C → N: carga las filas [desde, hasta) del gold con esta huella.
    Listo   Tipo = "LISTO"   // N → C: partición en memoria; n filas y la huella que verificó.
    Iterar  Tipo = "ITERAR"  // C → N: asigna tu partición a estos centroides (ronda iter).
    Parcial Tipo = "PARCIAL" // N → C: sumas k×D, conteos k e inercia de la ronda iter.
    Fin     Tipo = "FIN"     // C → N: terminó la corrida; libera la partición.
    Ping    Tipo = "PING"    // C → N: ¿sigues ahí?
    Pong    Tipo = "PONG"    // N → C: respuesta a PING.
    Error   Tipo = "ERROR"   // ambos sentidos: aborta lo que se estaba haciendo.
)

// ITERAR y PARCIAL. Iter es el número de ronda: crece estrictamente en toda
// la vida de la conexión, también entre corridas, y es lo que permite
// descartar un PARCIAL rezagado de una ronda abortada.
Iter       int       `json:"iter,omitempty"`
Centroides []float64 `json:"centroides,omitempty"` // k*D, por filas
Sumas      []float64 `json:"sumas,omitempty"`      // k*D, por filas
Conteos    []int     `json:"conteos,omitempty"`    // k
Inercia    float64   `json:"inercia,omitempty"`
```

Los tipos de mensaje del protocolo y los campos de ITERAR y PARCIAL (`protocolo/mensaje.go`, líneas 17–29 y 60–67)
:::
