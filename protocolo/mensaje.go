package protocolo

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"
)

// Version es la versión del protocolo del K-Means distribuido. El nodo la
// manda en su HOLA y el coordinador rechaza otra: un nodo de Colab que corre el
// binario de ayer no entra al clúster de hoy.
const Version = 1

// Version2 es la del entrenamiento del mapa nacional (local SGD): agrega la
// carga binaria y los mensajes TAREA, PESOS y LOCAL. Cada coordinador habla
// una sola versión (ver el comentario del paquete).
const Version2 = 2

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

	// Versión 2 (mapa nacional). LISTO se reusa: con tarea es el de la v2.
	Tarea Tipo = "TAREA" // C → N: tus días del manifiesto, las estaciones de validación y los hiperparámetros.
	Pesos Tipo = "PESOS" // C → N: entrena pasos pasos desde estos pesos globales (carga binaria).
	Local Tipo = "LOCAL" // N → C: tus pesos tras los pasos de la ronda, muestras y pérdidas (carga binaria).
)

// Rango es un intervalo semiabierto [Desde, Hasta) de filas de la tabla ancha,
// en el orden del gold (estación, fecha-hora). La tabla puede ser virtual: la
// fila i es la fila i mod N de un gold de N filas, así que Hasta puede pasar de
// N (réplicas virtuales; ver el comentario del paquete).
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

	// HOLA. GPU y VramGB son de la v2: la GPU que el nodo va a usar ("" si
	// entrena en CPU) y su memoria en GB.
	Version int     `json:"version,omitempty"`
	Nodo    string  `json:"nodo,omitempty"`
	Workers int     `json:"workers,omitempty"`
	GPU     string  `json:"gpu,omitempty"`
	VramGB  float64 `json:"vram_gb,omitempty"`

	// CARGAR y LISTO. Huella es el sha256 del gold real, como "sha256:" y 64
	// dígitos hexadecimales en minúscula: todos los nodos tienen que tener la
	// misma tabla en el mismo orden, o los rangos no significan nada.
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

	// TAREA (v2), con Huella: el sha256 del manifiesto, como "sha256:" y 64
	// dígitos hexadecimales en minúscula. Dias son los días de este nodo
	// (AAAA-MM-DD, en orden), Validacion las estaciones que no entran en su
	// pérdida e Hiper un objeto JSON que el coordinador no interpreta.
	Tarea      string          `json:"tarea,omitempty"` // también en el LISTO de la v2
	Manifiesto string          `json:"manifiesto,omitempty"`
	Dias       []string        `json:"dias,omitempty"`
	Validacion []string        `json:"validacion,omitempty"`
	Hiper      json.RawMessage `json:"hiper,omitempty"`

	// PESOS y LOCAL (v2), con N: el número de parámetros. Ronda crece en toda
	// la vida de la conexión, como Iter. Bytes es el largo de la carga binaria
	// que sigue a la línea (4·N). Muestras, en el LISTO de la v2, son las horas
	// con etiqueta que tiene el nodo; en LOCAL, las que vio en la ronda: el
	// peso de su vector en el promedio.
	Ronda      int     `json:"ronda,omitempty"`
	Pasos      int     `json:"pasos,omitempty"`
	Bytes      int     `json:"bytes,omitempty"`
	Muestras   int     `json:"muestras,omitempty"`
	Perdida    float64 `json:"perdida,omitempty"`
	PerdidaVal float64 `json:"perdida_val,omitempty"`
	MsComputo  float64 `json:"ms_computo,omitempty"`

	// ERROR
	Mensaje string `json:"mensaje,omitempty"`

	// Carga son los Bytes crudos que viajan después del '\n' de la línea. No
	// van en el JSON. En PESOS y LOCAL es un vector de N float32 en
	// little-endian: se arma con ConVector y se lee con Vector.
	Carga []byte `json:"-"`
}

// ConVector devuelve m con el vector v como carga binaria: N = len(v), Bytes =
// 4·len(v) y Carga los float32 en little-endian. Así se arman PESOS y LOCAL.
func (m Mensaje) ConVector(v []float32) Mensaje {
	m.N, m.Bytes = len(v), 4*len(v)
	m.Carga = make([]byte, 4*len(v))
	for i, x := range v {
		binary.LittleEndian.PutUint32(m.Carga[4*i:], math.Float32bits(x))
	}
	return m
}

// Vector lee la carga de un PESOS o un LOCAL: N float32 en little-endian.
func (m Mensaje) Vector() ([]float32, error) {
	if len(m.Carga) != 4*m.N {
		return nil, fmt.Errorf("%w: %s: %d bytes de carga no son %d float32", ErrMensajeInvalido, m.Tipo, len(m.Carga), m.N)
	}
	v := make([]float32, m.N)
	for i := range v {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(m.Carga[4*i:]))
	}
	return v, nil
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
	// La carga binaria, en cualquier tipo: el tope primero, para que un
	// "bytes" enorme se rechace sin mirar nada más.
	switch {
	case m.Bytes > MaxCarga || len(m.Carga) > MaxCarga:
		return mal("carga de %d bytes, más que MaxCarga (%d)", max(m.Bytes, len(m.Carga)), MaxCarga)
	case m.Bytes < 0:
		return mal("bytes negativo (%d)", m.Bytes)
	case m.Bytes != len(m.Carga):
		return mal("dice bytes = %d y trae %d bytes de carga", m.Bytes, len(m.Carga))
	case m.Bytes > 0 && m.Tipo != Pesos && m.Tipo != Local:
		return mal("no lleva carga binaria y trae %d bytes", m.Bytes)
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
		if !finitoNoNegativo(m.VramGB) {
			return mal("vram_gb inválida (%v)", m.VramGB)
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
		if m.Tarea != "" { // el LISTO de la v2
			if m.Muestras < 0 {
				return mal("muestras negativas (%d)", m.Muestras)
			}
			break
		}
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
	case Tarea:
		switch {
		case m.Tarea == "":
			return mal("falta el id de la tarea")
		case m.Manifiesto == "":
			return mal("falta el manifiesto")
		case m.Huella == "":
			return mal("falta la huella del manifiesto")
		case len(m.Dias) == 0:
			return mal("no trae días")
		}
		for i, d := range m.Dias {
			if _, err := time.Parse(time.DateOnly, d); err != nil || len(d) != len(time.DateOnly) {
				return mal("el día %d (%q) no es AAAA-MM-DD", i, d)
			}
		}
		for i, e := range m.Validacion {
			if e == "" {
				return mal("la estación de validación %d está vacía", i)
			}
		}
		if len(m.Hiper) > 0 && !json.Valid(m.Hiper) {
			return mal("hiper no es JSON válido")
		}
	case Pesos, Local:
		switch {
		case m.Ronda <= 0:
			return mal("ronda debe ser positiva, es %d", m.Ronda)
		case m.N <= 0:
			return mal("n debe ser positivo, es %d", m.N)
		case m.Bytes != 4*m.N:
			return mal("bytes = %d no son 4·n = %d (n float32)", m.Bytes, 4*m.N)
		case m.Tipo == Pesos && m.Pasos <= 0:
			return mal("pasos debe ser positivo, es %d", m.Pasos)
		case m.Tipo == Local && m.Muestras < 0:
			return mal("muestras negativas (%d)", m.Muestras)
		case !finitoNoNegativo(m.Perdida) || !finitoNoNegativo(m.PerdidaVal):
			return mal("pérdidas inválidas (perdida %v, perdida_val %v)", m.Perdida, m.PerdidaVal)
		case !finitoNoNegativo(m.MsComputo):
			return mal("ms_computo inválido (%v)", m.MsComputo)
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

func finitoNoNegativo(x float64) bool { return x >= 0 && !math.IsInf(x, 1) }

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
