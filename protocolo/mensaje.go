package protocolo

import (
	"errors"
	"fmt"
	"math"
)

// Version es la versión del protocolo. El nodo la manda en su HOLA y el
// coordinador rechaza otra: un nodo de Colab que corre el binario de ayer no
// entra al clúster de hoy.
const Version = 1

// Tipo dice qué mensaje es. En el cable es el campo "tipo".
type Tipo string

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

// Rango es un intervalo semiabierto [Desde, Hasta) de filas de la tabla ancha,
// en el orden del gold (estación, fecha-hora).
type Rango struct {
	Desde int `json:"desde"`
	Hasta int `json:"hasta"`
}

// Len es la cantidad de filas del rango.
func (r Rango) Len() int { return r.Hasta - r.Desde }

// Mensaje es lo que viaja en una línea. Es un sobre único con el Tipo como
// discriminante y los campos que aplican a cada tipo; los que no aplican quedan
// en su valor cero y no se escriben. Un solo struct, y no uno por tipo, porque
// así el lector decodifica cada línea una sola vez y el modelo en Promela de la
// PC4 es un mensaje con una etiqueta, igual que aquí.
type Mensaje struct {
	Tipo Tipo `json:"tipo"`

	// HOLA
	Version int    `json:"version,omitempty"`
	Nodo    string `json:"nodo,omitempty"`
	Workers int    `json:"workers,omitempty"`

	// CARGAR y LISTO. Huella es el sha256 del gold: todos los nodos tienen que
	// tener la misma tabla en el mismo orden, o los rangos no significan nada.
	Rango  *Rango `json:"rango,omitempty"`
	Huella string `json:"huella,omitempty"`
	N      int    `json:"n,omitempty"`

	// ITERAR y PARCIAL. Iter es el número de ronda: crece estrictamente en toda
	// la vida de la conexión, también entre corridas, y es lo que permite
	// descartar un PARCIAL rezagado de una ronda abortada.
	Iter       int       `json:"iter,omitempty"`
	Centroides []float64 `json:"centroides,omitempty"` // k*D, por filas
	Sumas      []float64 `json:"sumas,omitempty"`      // k*D, por filas
	Conteos    []int     `json:"conteos,omitempty"`    // k
	Inercia    float64   `json:"inercia,omitempty"`

	// ERROR
	Mensaje string `json:"mensaje,omitempty"`
}

// ErrMensajeInvalido envuelve todo rechazo de Validar.
var ErrMensajeInvalido = errors.New("mensaje inválido")

// Validar comprueba que el mensaje tenga los campos que su tipo exige. El
// Codificador no manda un mensaje inválido y el Decodificador no entrega uno:
// un error de forma se detecta en el borde, con el tipo y el campo en el texto,
// y no tres funciones más adentro como un índice fuera de rango.
func (m Mensaje) Validar() error {
	mal := func(formato string, args ...any) error {
		return fmt.Errorf("%w: %s: %s", ErrMensajeInvalido, m.Tipo, fmt.Sprintf(formato, args...))
	}
	switch m.Tipo {
	case Hola:
		if m.Version <= 0 {
			return mal("falta la versión del protocolo")
		}
		if m.Nodo == "" {
			return mal("falta el nombre del nodo")
		}
		if m.Workers < 0 {
			return mal("workers negativo (%d)", m.Workers)
		}
	case Cargar:
		if m.Rango == nil {
			return mal("falta el rango")
		}
		if m.Rango.Desde < 0 || m.Rango.Desde >= m.Rango.Hasta {
			return mal("rango vacío o negativo [%d, %d)", m.Rango.Desde, m.Rango.Hasta)
		}
		if m.Huella == "" {
			return mal("falta la huella del gold")
		}
	case Listo:
		if m.N <= 0 {
			return mal("n debe ser positivo, es %d", m.N)
		}
		if m.Huella == "" {
			return mal("falta la huella del gold")
		}
	case Iterar:
		if m.Iter <= 0 {
			return mal("iter debe ser positivo, es %d", m.Iter)
		}
		if len(m.Centroides) == 0 {
			return mal("no trae centroides")
		}
		if i, ok := noFinito(m.Centroides); ok {
			return mal("centroide no finito en la posición %d (%v)", i, m.Centroides[i])
		}
	case Parcial:
		if m.Iter <= 0 {
			return mal("iter debe ser positivo, es %d", m.Iter)
		}
		k := len(m.Conteos)
		if k == 0 {
			return mal("no trae conteos")
		}
		if len(m.Sumas) == 0 || len(m.Sumas)%k != 0 {
			return mal("%d sumas no es múltiplo de k=%d conteos", len(m.Sumas), k)
		}
		for j, c := range m.Conteos {
			if c < 0 {
				return mal("conteo negativo en el cluster %d (%d)", j, c)
			}
		}
		if i, ok := noFinito(m.Sumas); ok {
			return mal("suma no finita en la posición %d (%v)", i, m.Sumas[i])
		}
		if math.IsNaN(m.Inercia) || math.IsInf(m.Inercia, 0) || m.Inercia < 0 {
			return mal("inercia inválida (%v)", m.Inercia)
		}
	case Fin, Ping, Pong:
	case Error:
		if m.Mensaje == "" {
			return mal("un ERROR sin texto no le sirve a nadie")
		}
	case "":
		return fmt.Errorf("%w: falta el tipo", ErrMensajeInvalido)
	default:
		return fmt.Errorf("%w: tipo desconocido %q", ErrMensajeInvalido, string(m.Tipo))
	}
	return nil
}

// noFinito devuelve la posición del primer NaN o ±Inf. JSON no puede
// representarlos, así que es mejor decirlo antes de intentar escribirlos.
func noFinito(xs []float64) (int, bool) {
	for i, x := range xs {
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return i, true
		}
	}
	return 0, false
}
