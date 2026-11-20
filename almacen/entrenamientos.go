package almacen

import (
	"context"
	"time"
)

// Estados de un entrenamiento (esquema EntrenamientoResumen del contrato).
const (
	EstadoEnCurso   = "en_curso"
	EstadoTerminado = "terminado"
	EstadoAbortado  = "abortado"
)

// Entrenamiento es un documento de la colección entrenamientos: una corrida
// del K-Means distribuido lanzada por la API, con sus rondas (a lo sumo
// max_iter = 100, por eso van dentro del documento).
//
// Mientras está en curso, Fin, Paro, DifSecuencial, MsTotal y Error son nil.
// Iteraciones y MsTotal se guardan como campos y no se derivan: Rondas puede
// no tener todas las rondas (AgregarRonda es una escritura aparte que puede
// fallar) y las fechas se guardan con precisión de milisegundos, que no
// alcanza para una corrida de 30 ms.
type Entrenamiento struct {
	ID            string                  `json:"_id"`
	Inicio        time.Time               `json:"inicio"`
	Fin           *time.Time              `json:"fin"`
	Estado        string                  `json:"estado"`
	Parametros    ParametrosEntrenamiento `json:"parametros"`
	Nodos         []NodoEntrenamiento     `json:"nodos"`
	Rondas        []Ronda                 `json:"rondas"`
	Iteraciones   int                     `json:"iteraciones"`
	Paro          *string                 `json:"paro"`
	DifSecuencial *float64                `json:"dif_secuencial"`
	MsTotal       *float64                `json:"ms_total"`
	Error         *string                 `json:"error"`
}

// ParametrosEntrenamiento es lo que se pidió en POST /api/entrenamientos.
type ParametrosEntrenamiento struct {
	Nodos    int   `json:"nodos"`
	Replicas int   `json:"replicas"`
	Semilla  int64 `json:"semilla"`
}

// NodoEntrenamiento es un nodo que participó en la corrida y su rango de filas
// del gold, [Desde, Hasta).
type NodoEntrenamiento struct {
	Nombre    string `json:"nombre"`
	Direccion string `json:"direccion"`
	Workers   int    `json:"workers"`
	Rango     Rango  `json:"rango"`
}

// Rango es un rango de filas del gold, [Desde, Hasta).
type Rango struct {
	Desde int `json:"desde"`
	Hasta int `json:"hasta"`
}

// Ronda es una iteración de Lloyd vista desde el coordinador (esquema Ronda
// del contrato). MsPorNodo es el tiempo de cómputo de cada nodo, por nombre.
type Ronda struct {
	Iter      int                `json:"iter"`
	Inercia   float64            `json:"inercia"`
	MsRonda   float64            `json:"ms_ronda"`
	MsFusion  float64            `json:"ms_fusion"`
	MsPorNodo map[string]float64 `json:"ms_por_nodo"`
}

// Cierre es cómo terminó un entrenamiento: lo que Terminar escribe sobre el
// documento. Con Estado = EstadoAbortado, Error dice por qué y los demás
// punteros pueden ser nil.
type Cierre struct {
	Fin           time.Time
	Estado        string
	Iteraciones   int
	Paro          *string
	DifSecuencial *float64
	MsTotal       *float64
	Error         *string
}

// Entrenamientos es la colección entrenamientos.
type Entrenamientos interface {
	// Crear guarda un entrenamiento nuevo. Falla si ya existe uno con su id.
	Crear(ctx context.Context, e Entrenamiento) error
	// AgregarRonda agrega una ronda al final de las del entrenamiento, o
	// devuelve ErrNoEncontrado.
	AgregarRonda(ctx context.Context, id string, r Ronda) error
	// Terminar escribe el cierre sin tocar las rondas, o devuelve
	// ErrNoEncontrado.
	Terminar(ctx context.Context, id string, c Cierre) error
	// Obtener devuelve un entrenamiento con sus rondas o ErrNoEncontrado.
	Obtener(ctx context.Context, id string) (Entrenamiento, error)
	// Listar devuelve los entrenamientos del más reciente al más antiguo, sin
	// las rondas (Rondas es nil). Una lista vacía es válida.
	Listar(ctx context.Context) ([]Entrenamiento, error)
}

// Tipos de evento del clúster.
const (
	EventoAlta  = "alta"
	EventoBaja  = "baja"
	EventoError = "error"
)

// EventoCluster es un documento de la colección eventos_cluster: un nodo que
// entró al pool, salió o falló. Los latidos no se guardan.
type EventoCluster struct {
	TS      time.Time `json:"ts"`
	Tipo    string    `json:"tipo"`
	Nodo    string    `json:"nodo"`
	Detalle string    `json:"detalle"`
}

// Eventos es la colección eventos_cluster.
type Eventos interface {
	Registrar(ctx context.Context, e EventoCluster) error
	// Recientes devuelve a lo sumo n eventos, del más reciente al más antiguo.
	Recientes(ctx context.Context, n int) ([]EventoCluster, error)
}

// PuntoControl es el valor de la clave entrenamiento:{id}:punto de Redis: los
// centroides estandarizados de la última ronda completa. Se escribe en cada
// ronda como registro; la PC4 no reanuda desde él (spec §5.2).
type PuntoControl struct {
	Iter       int         `json:"iter"`
	Centroides [][]float64 `json:"centroides"`
}

// PuntosControl son las claves entrenamiento:{id}:punto de Redis.
type PuntosControl interface {
	GuardarPunto(ctx context.Context, entrenamiento string, p PuntoControl) error
	// Punto devuelve el último punto de control de un entrenamiento o
	// ErrNoEncontrado.
	Punto(ctx context.Context, entrenamiento string) (PuntoControl, error)
}
