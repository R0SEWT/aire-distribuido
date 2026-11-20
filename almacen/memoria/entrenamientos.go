package memoria

import (
	"context"
	"slices"
	"sync"

	"github.com/R0SEWT/aire-distribuido/almacen"
)

// Entrenamientos es la colección entrenamientos en memoria.
type Entrenamientos struct {
	mu    sync.RWMutex
	porID map[string]*almacen.Entrenamiento
	orden []string // en orden de creación
}

var _ almacen.Entrenamientos = (*Entrenamientos)(nil)

func NuevosEntrenamientos() *Entrenamientos {
	return &Entrenamientos{porID: map[string]*almacen.Entrenamiento{}}
}

func (s *Entrenamientos) Crear(_ context.Context, e almacen.Entrenamiento) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.porID[e.ID]; ok {
		return almacen.ErrYaExiste
	}
	e.Nodos, e.Rondas = slices.Clone(e.Nodos), slices.Clone(e.Rondas)
	s.porID[e.ID] = &e
	s.orden = append(s.orden, e.ID)
	return nil
}

func (s *Entrenamientos) AgregarRonda(_ context.Context, id string, r almacen.Ronda) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.porID[id]
	if !ok {
		return almacen.ErrNoEncontrado
	}
	e.Rondas = append(e.Rondas, r)
	return nil
}

func (s *Entrenamientos) Cerrar(_ context.Context, id string, c almacen.CierreEntrenamiento) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.porID[id]
	if !ok {
		return almacen.ErrNoEncontrado
	}
	e.Fin, e.Estado = c.Fin, c.Estado
	e.Iteraciones, e.Paro, e.DifSecuencial, e.MsTotal = c.Iteraciones, c.Paro, c.DifSecuencial, c.MsTotal
	e.ModeloID, e.Aviso, e.Error = c.ModeloID, c.Aviso, c.Error
	return nil
}

func (s *Entrenamientos) Obtener(_ context.Context, id string) (almacen.Entrenamiento, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.porID[id]
	if !ok {
		return almacen.Entrenamiento{}, almacen.ErrNoEncontrado
	}
	copia := *e
	copia.Nodos, copia.Rondas = slices.Clone(e.Nodos), slices.Clone(e.Rondas)
	return copia, nil
}

// Listar devuelve el historial sin las rondas, del más reciente al más
// antiguo; dos con el mismo Inicio, el que se creó después primero.
func (s *Entrenamientos) Listar(context.Context) ([]almacen.Entrenamiento, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]almacen.Entrenamiento, 0, len(s.orden))
	for _, id := range slices.Backward(s.orden) {
		copia := *s.porID[id]
		copia.Nodos, copia.Rondas = slices.Clone(copia.Nodos), nil
		out = append(out, copia)
	}
	slices.SortStableFunc(out, func(a, b almacen.Entrenamiento) int { return b.Inicio.Compare(a.Inicio) })
	return out, nil
}

func (s *Entrenamientos) Ping(context.Context) error { return nil }

// Eventos es la colección eventos_cluster en memoria.
type Eventos struct {
	mu    sync.Mutex
	todos []almacen.EventoCluster
}

var _ almacen.Eventos = (*Eventos)(nil)

func NuevosEventos() *Eventos { return &Eventos{} }

func (s *Eventos) Guardar(_ context.Context, e almacen.EventoCluster) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.todos = append(s.todos, e)
	return nil
}

// Todos devuelve una copia de los eventos guardados, en orden de llegada.
func (s *Eventos) Todos() []almacen.EventoCluster {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.todos)
}
