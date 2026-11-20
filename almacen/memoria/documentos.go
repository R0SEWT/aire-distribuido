package memoria

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/R0SEWT/aire-distribuido/almacen"
)

// Modelos es la colección modelos en memoria.
type Modelos struct {
	mu    sync.RWMutex
	porID map[string]almacen.Modelo
}

var _ almacen.Modelos = (*Modelos)(nil)

func NuevosModelos() *Modelos { return &Modelos{porID: map[string]almacen.Modelo{}} }

func (m *Modelos) Guardar(_ context.Context, mod almacen.Modelo) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if mod.Activo {
		for id, otro := range m.porID {
			otro.Activo = false
			m.porID[id] = otro
		}
	}
	m.porID[mod.ID] = mod
	return nil
}

func (m *Modelos) Activo(context.Context) (almacen.Modelo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, mod := range m.porID {
		if mod.Activo {
			return mod, nil
		}
	}
	return almacen.Modelo{}, almacen.ErrNoEncontrado
}

// Entrenamientos es la colección entrenamientos en memoria.
type Entrenamientos struct {
	mu    sync.RWMutex
	porID map[string]almacen.Entrenamiento
}

var _ almacen.Entrenamientos = (*Entrenamientos)(nil)

func NuevosEntrenamientos() *Entrenamientos {
	return &Entrenamientos{porID: map[string]almacen.Entrenamiento{}}
}

func (e *Entrenamientos) Crear(_ context.Context, en almacen.Entrenamiento) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.porID[en.ID]; ok {
		return fmt.Errorf("memoria: ya existe el entrenamiento %s", en.ID)
	}
	en.Rondas = slices.Clone(en.Rondas)
	e.porID[en.ID] = en
	return nil
}

func (e *Entrenamientos) AgregarRonda(_ context.Context, id string, r almacen.Ronda) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	en, ok := e.porID[id]
	if !ok {
		return almacen.ErrNoEncontrado
	}
	// append sobre una copia: Obtener pudo haber entregado el slice anterior.
	en.Rondas = append(slices.Clone(en.Rondas), r)
	e.porID[id] = en
	return nil
}

func (e *Entrenamientos) Terminar(_ context.Context, id string, c almacen.Cierre) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	en, ok := e.porID[id]
	if !ok {
		return almacen.ErrNoEncontrado
	}
	fin := c.Fin
	en.Fin, en.Estado, en.Iteraciones = &fin, c.Estado, c.Iteraciones
	en.Paro, en.DifSecuencial, en.MsTotal, en.Error = c.Paro, c.DifSecuencial, c.MsTotal, c.Error
	e.porID[id] = en
	return nil
}

func (e *Entrenamientos) Obtener(_ context.Context, id string) (almacen.Entrenamiento, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	en, ok := e.porID[id]
	if !ok {
		return almacen.Entrenamiento{}, almacen.ErrNoEncontrado
	}
	return en, nil
}

func (e *Entrenamientos) Listar(context.Context) ([]almacen.Entrenamiento, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	lista := make([]almacen.Entrenamiento, 0, len(e.porID))
	for _, en := range e.porID {
		en.Rondas = nil
		lista = append(lista, en)
	}
	slices.SortFunc(lista, func(a, b almacen.Entrenamiento) int {
		if c := b.Inicio.Compare(a.Inicio); c != 0 {
			return c
		}
		return strings.Compare(b.ID, a.ID)
	})
	return lista, nil
}

// Eventos es la colección eventos_cluster en memoria.
type Eventos struct {
	mu    sync.RWMutex
	todos []almacen.EventoCluster
}

var _ almacen.Eventos = (*Eventos)(nil)

func NuevosEventos() *Eventos { return &Eventos{} }

func (e *Eventos) Registrar(_ context.Context, ev almacen.EventoCluster) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.todos = append(e.todos, ev)
	return nil
}

func (e *Eventos) Recientes(_ context.Context, n int) ([]almacen.EventoCluster, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	lista := slices.Clone(e.todos)
	// Orden estable: entre dos eventos del mismo instante, primero el último
	// que se registró.
	slices.Reverse(lista)
	slices.SortStableFunc(lista, func(a, b almacen.EventoCluster) int { return b.TS.Compare(a.TS) })
	return lista[:max(min(n, len(lista)), 0)], nil
}
