package protocolo

import (
	"fmt"
	"time"
)

// Lo de la versión 2 que corresponde a un nodo: la TAREA y las dos mitades de
// una ronda de local SGD. El fan-out a todos los nodos, el plazo común y el
// promedio están en cluster/ (cluster.RondaLocal), como cluster.Cargar está
// sobre AsignarRango.

// AsignarTarea le manda al nodo su TAREA (sus días del manifiesto, las
// estaciones de validación y los hiperparámetros) y espera su LISTO. Comprueba
// que el LISTO sea de esa tarea: un nodo que confirma otra tiene otros días.
// Si sale bien, guarda en el Enlace la tarea y las muestras que el nodo dice
// tener.
func AsignarTarea(e *Enlace, t Mensaje, limite time.Time) error {
	if t.Tipo != Tarea {
		return fmt.Errorf("AsignarTarea con un %s: se esperaba una TAREA", t.Tipo)
	}
	if err := e.enviar(t, limite); err != nil {
		return e.fallo("enviar TAREA: %w", err)
	}
	m, err := e.esperar(Listo, 0, limite)
	if err != nil {
		return e.fallo("esperando LISTO: %w", err)
	}
	if m.Tarea != t.Tarea {
		return e.fallo("%w: confirmó la tarea %q y se le mandó %q", ErrProtocolo, m.Tarea, t.Tarea)
	}
	e.Tarea, e.Muestras = t.Tarea, m.Muestras
	return nil
}

// EnviarPesos le manda al nodo un PESOS ya armado (Mensaje.ConVector). Varios
// nodos pueden recibir el mismo Mensaje a la vez: nadie escribe en su carga.
//
// La ronda tiene que ser mayor que cualquiera ya enviada por la conexión, así
// un LOCAL que llega tarde de una ronda abortada se reconoce y se descarta. Si
// el límite ya venció, o el PESOS no es válido, no envía nada y la ronda sigue
// disponible.
func EnviarPesos(e *Enlace, m Mensaje, limite time.Time) error {
	switch {
	case m.Tipo != Pesos:
		return fmt.Errorf("EnviarPesos con un %s: se esperaba un PESOS", m.Tipo)
	case m.Ronda <= e.ultimaIter:
		return fmt.Errorf("ronda %d: el número de ronda tiene que crecer y el nodo %q ya recibió la %d",
			m.Ronda, e.nombre(), e.ultimaIter)
	case !limite.After(time.Now()):
		return e.fallo("enviar PESOS de la ronda %d: %w antes de empezar; no se envió nada", m.Ronda, ErrPlazoVencido)
	}
	if err := m.Validar(); err != nil {
		return err
	}
	// Se marca antes de enviar: desde este momento, cualquier LOCAL de una
	// ronda anterior es un rezagado.
	e.ultimaIter = m.Ronda
	if err := e.enviar(m, limite); err != nil {
		return e.fallo("enviar PESOS: %w", err)
	}
	return nil
}

// RecibirLocal espera el LOCAL de la ronda con n parámetros, descartando en el
// camino los PONG sueltos y los LOCAL rezagados. Un ERROR del nodo, un LOCAL de
// una ronda que no se envió o con otro número de parámetros terminan con un
// error que nombra al nodo.
func RecibirLocal(e *Enlace, ronda, n int, limite time.Time) (Mensaje, error) {
	m, err := e.esperar(Local, ronda, limite)
	if err != nil {
		return Mensaje{}, e.fallo("esperando LOCAL de la ronda %d: %w", ronda, err)
	}
	if m.N != n {
		return Mensaje{}, e.fallo("%w: LOCAL con %d parámetros y el PESOS tenía %d", ErrProtocolo, m.N, n)
	}
	return m, nil
}
