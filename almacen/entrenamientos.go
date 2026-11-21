package almacen

import (
	"context"
	"errors"
	"time"
)

// ErrYaExiste es lo que devuelve Crear cuando ya hay un documento con ese id.
var ErrYaExiste = errors.New("ya existe")

// Estados de un entrenamiento (esquema EntrenamientoResumen del contrato).
const (
	EstadoEnCurso   = "en_curso"
	EstadoTerminado = "terminado"
	EstadoAbortado  = "abortado"
)

// ParametrosEntrenamiento es lo que pidió POST /api/entrenamientos.
type ParametrosEntrenamiento struct {
	Nodos    int
	Replicas int
	Semilla  int64
}

// NodoEntrenamiento es un nodo que participó, con el rango [Desde, Hasta) de
// la tabla que le tocó. El orden de los nodos es el de los rangos y el de la
// fusión.
type NodoEntrenamiento struct {
	Nombre    string
	Direccion string
	Workers   int
	Desde     int
	Hasta     int
}

// Ronda es una iteración del entrenamiento (esquema Ronda del contrato).
// MsRonda es la barrera de red, MsFusion lo que el coordinador hace después y
// MsPorNodo lo que tardó cada nodo, medido en el coordinador.
type Ronda struct {
	Iter      int                `json:"iter"`
	Inercia   float64            `json:"inercia"`
	MsRonda   float64            `json:"ms_ronda"`
	MsFusion  float64            `json:"ms_fusion"`
	MsPorNodo map[string]float64 `json:"ms_por_nodo"`
}

// Entrenamiento es un documento de la colección entrenamientos (§7.1). Las
// rondas van dentro: son a lo sumo max_iter por corrida. Mientras está en
// curso, Fin es el tiempo cero y los campos del resultado están vacíos;
// terminado, tiene Iteraciones, Paro, DifSecuencial, MsTotal y, si activó un
// modelo, ModeloID (si tenía que activarlo y no lo hizo, Aviso dice por qué);
// abortado, tiene Error con el nodo que falló.
type Entrenamiento struct {
	ID         string
	Inicio     time.Time
	Fin        time.Time
	Estado     string
	Parametros ParametrosEntrenamiento
	Nodos      []NodoEntrenamiento
	Rondas     []Ronda

	Iteraciones   int
	Paro          string
	DifSecuencial float64
	MsTotal       float64
	ModeloID      string
	Aviso         string
	Error         string
}

// CierreEntrenamiento es el estado final de un entrenamiento: lo que Cerrar
// escribe sobre el documento que Crear dejó en curso.
type CierreEntrenamiento struct {
	Fin    time.Time
	Estado string // EstadoTerminado o EstadoAbortado

	Iteraciones   int
	Paro          string
	DifSecuencial float64
	MsTotal       float64
	ModeloID      string
	Aviso         string
	Error         string
}

// Entrenamientos es la colección entrenamientos.
type Entrenamientos interface {
	// Crear guarda un entrenamiento nuevo, en curso y sin rondas. Si el id ya
	// existe devuelve ErrYaExiste y no cambia nada.
	Crear(ctx context.Context, e Entrenamiento) error
	// AgregarRonda agrega una ronda al final de las del entrenamiento, o
	// devuelve ErrNoEncontrado.
	AgregarRonda(ctx context.Context, id string, r Ronda) error
	// Cerrar escribe el estado final, o devuelve ErrNoEncontrado.
	Cerrar(ctx context.Context, id string, c CierreEntrenamiento) error
	// Obtener devuelve un entrenamiento con sus rondas, o ErrNoEncontrado.
	Obtener(ctx context.Context, id string) (Entrenamiento, error)
	// Listar devuelve el historial SIN las rondas, del más reciente al más
	// antiguo por Inicio. Una lista vacía es válida.
	Listar(ctx context.Context) ([]Entrenamiento, error)
}

// EventoCluster es un documento de la colección eventos_cluster (§7.1): un
// nodo que entró al pool (alta), que salió (baja) o que no llegó a entrar o
// hizo abortar una corrida (error). Los latidos no se guardan.
type EventoCluster struct {
	TS      time.Time
	Tipo    string
	Nodo    string
	Detalle string
}

// Eventos es la colección eventos_cluster.
type Eventos interface {
	Guardar(ctx context.Context, e EventoCluster) error
	// Recientes devuelve a lo sumo n eventos, del más reciente al más antiguo
	// por TS; entre dos del mismo TS, primero el que se guardó último. Con n
	// menor que 1, o sin eventos, devuelve una lista vacía.
	Recientes(ctx context.Context, n int) ([]EventoCluster, error)
}
