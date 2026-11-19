// Package memoria es el doble en memoria de las interfaces de almacen: lo que
// usan las pruebas, el CI (que no levanta MongoDB ni Redis) y el esqueleto de
// la API mientras no existan almacen/mongo y almacen/redis. Cada tipo es seguro
// para usarse desde varias goroutines.
package memoria

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/R0SEWT/aire-distribuido/almacen"
)

// Estaciones es la colección estaciones en memoria. Es de solo lectura después
// de crearse: la siembra es la ficha completa.
type Estaciones struct {
	lista []almacen.Estacion // ordenada por id
	porID map[string]int
}

var _ almacen.Estaciones = (*Estaciones)(nil)

// NuevasEstaciones guarda una copia de las estaciones, ordenadas por id.
func NuevasEstaciones(es []almacen.Estacion) (*Estaciones, error) {
	lista := slices.Clone(es)
	slices.SortFunc(lista, func(a, b almacen.Estacion) int { return strings.Compare(a.ID, b.ID) })
	porID := make(map[string]int, len(lista))
	for i, e := range lista {
		if _, ok := porID[e.ID]; ok {
			return nil, fmt.Errorf("memoria: la estación %s está repetida", e.ID)
		}
		porID[e.ID] = i
	}
	return &Estaciones{lista: lista, porID: porID}, nil
}

func (e *Estaciones) Listar(context.Context) ([]almacen.Estacion, error) {
	return append([]almacen.Estacion{}, e.lista...), nil
}

func (e *Estaciones) Obtener(_ context.Context, id string) (almacen.Estacion, error) {
	i, ok := e.porID[id]
	if !ok {
		return almacen.Estacion{}, almacen.ErrNoEncontrado
	}
	return e.lista[i], nil
}

func (e *Estaciones) Ping(context.Context) error { return nil }

// Predicciones es la colección predicciones en memoria.
type Predicciones struct {
	mu    sync.Mutex
	todas []almacen.Prediccion
}

var _ almacen.Predicciones = (*Predicciones)(nil)

func NuevasPredicciones() *Predicciones { return &Predicciones{} }

func (p *Predicciones) Guardar(_ context.Context, pr almacen.Prediccion) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.todas = append(p.todas, pr)
	return nil
}

// Todas devuelve una copia de las predicciones guardadas, en orden de llegada.
func (p *Predicciones) Todas() []almacen.Prediccion {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.todas)
}

func (p *Predicciones) Ping(context.Context) error { return nil }

// Cache es la caché de Redis en memoria.
type Cache struct {
	mu        sync.RWMutex
	regimenes map[string]almacen.RegimenEstacion
}

var _ almacen.Cache = (*Cache)(nil)

func NuevaCache() *Cache { return &Cache{regimenes: map[string]almacen.RegimenEstacion{}} }

func (c *Cache) GuardarRegimen(_ context.Context, estacion string, r almacen.RegimenEstacion) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.regimenes[estacion] = r
	return nil
}

func (c *Cache) Regimen(_ context.Context, estacion string) (almacen.RegimenEstacion, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	r, ok := c.regimenes[estacion]
	if !ok {
		return almacen.RegimenEstacion{}, almacen.ErrNoEncontrado
	}
	return r, nil
}

func (c *Cache) Ping(context.Context) error { return nil }
