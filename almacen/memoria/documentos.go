package memoria

import (
	"context"
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
