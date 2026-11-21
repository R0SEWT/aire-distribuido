// Package memoria es el doble en memoria de las interfaces de almacen: lo que
// usan las pruebas, el CI (que no levanta MongoDB ni Redis) y la API cuando
// arranca sin MONGO_URI o sin REDIS_ADDR. Cada tipo es seguro para usarse desde
// varias goroutines. Modelos está en documentos.go; Entrenamientos y Eventos, en
// entrenamientos.go.
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

func (p *Predicciones) Guardar(ctx context.Context, pr almacen.Prediccion) error {
	return p.GuardarLote(ctx, []almacen.Prediccion{pr})
}

func (p *Predicciones) GuardarLote(_ context.Context, ps []almacen.Prediccion) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.todas = append(p.todas, ps...)
	return nil
}

// Todas devuelve una copia de las predicciones guardadas, en orden de llegada.
func (p *Predicciones) Todas() []almacen.Prediccion {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.todas)
}

func (p *Predicciones) Ping(context.Context) error { return nil }

// Cache es la caché de Redis en memoria: el régimen por estación, el modelo
// activo y los puntos de control.
type Cache struct {
	mu        sync.RWMutex
	regimenes map[string]almacen.RegimenEstacion
	modelo    *almacen.Modelo
	puntos    map[string]almacen.PuntoControl
}

var (
	_ almacen.Cache         = (*Cache)(nil)
	_ almacen.CacheModelo   = (*Cache)(nil)
	_ almacen.PuntosControl = (*Cache)(nil)
)

func NuevaCache() *Cache {
	return &Cache{regimenes: map[string]almacen.RegimenEstacion{}, puntos: map[string]almacen.PuntoControl{}}
}

// GuardarRegimen guarda el régimen de una sola estación, con la regla de
// GuardarRegimenes.
func (c *Cache) GuardarRegimen(ctx context.Context, estacion string, r almacen.RegimenEstacion) error {
	return c.GuardarRegimenes(ctx, map[string]almacen.RegimenEstacion{estacion: r})
}

// GuardarRegimenes no reemplaza un régimen de una fecha_hora posterior:
// compara y escribe con el mismo lock.
func (c *Cache) GuardarRegimenes(_ context.Context, rs map[string]almacen.RegimenEstacion) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for estacion, r := range rs {
		if guardado, ok := c.regimenes[estacion]; ok && guardado.FechaHora.After(r.FechaHora) {
			continue
		}
		c.regimenes[estacion] = r
	}
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

func (c *Cache) Regimenes(_ context.Context, estaciones []string) (map[string]almacen.RegimenEstacion, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	rs := make(map[string]almacen.RegimenEstacion, len(estaciones))
	for _, estacion := range estaciones {
		if r, ok := c.regimenes[estacion]; ok {
			rs[estacion] = r
		}
	}
	return rs, nil
}

func (c *Cache) Ping(context.Context) error { return nil }

func (c *Cache) GuardarModeloActivo(_ context.Context, m almacen.Modelo) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.modelo = &m
	return nil
}

func (c *Cache) ModeloActivo(context.Context) (almacen.Modelo, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.modelo == nil {
		return almacen.Modelo{}, almacen.ErrNoEncontrado
	}
	return *c.modelo, nil
}

func (c *Cache) GuardarPunto(_ context.Context, entrenamiento string, p almacen.PuntoControl) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.puntos[entrenamiento] = p
	return nil
}

func (c *Cache) Punto(_ context.Context, entrenamiento string) (almacen.PuntoControl, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	p, ok := c.puntos[entrenamiento]
	if !ok {
		return almacen.PuntoControl{}, almacen.ErrNoEncontrado
	}
	return p, nil
}
