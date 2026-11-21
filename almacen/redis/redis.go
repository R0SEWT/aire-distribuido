// Package redis implementa con Redis las interfaces de almacen que son caché
// (spec de la PC4, §7.2). Con almacen/mongo, es uno de los dos únicos paquetes
// del repositorio que importan un driver.
//
// Tres familias de claves, todas con un valor JSON (los de docs/datos/ejemplos/):
//
//   - estacion:{id}:regimen: el estado de la estación en su lectura más
//     reciente (almacen.RegimenEstacion).
//   - modelo:activo: el modelo activo (almacen.Modelo), para que la API
//     arranque con él.
//   - entrenamiento:{id}:punto: los centroides de la última ronda completa
//     (almacen.PuntoControl).
//
// Ninguna clave vence: son pocas (una por estación, una por entrenamiento) y
// el régimen de una estación es su única copia.
//
// # El régimen no retrocede
//
// Una lectura más antigua que la guardada no la reemplaza. Leer la clave,
// comparar y escribir desde Go serían tres pasos entre los que otra escritura
// puede colarse; por eso la comparación y el SET van en un script de Lua
// (guionRegimen), que Redis ejecuta entero sin atender a nadie más. El script
// compara las fecha_hora como texto, y eso solo es comparar instantes si todas
// tienen la misma forma: este paquete las escribe siempre en la hora de Lima
// (UTC-5, sin horario de verano) y al segundo, 2006-01-02T15:04:05-05:00.
package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	driver "github.com/redis/go-redis/v9"

	"github.com/R0SEWT/aire-distribuido/almacen"
)

const claveModeloActivo = "modelo:activo"

func claveRegimen(estacion string) string    { return "estacion:" + estacion + ":regimen" }
func clavePunto(entrenamiento string) string { return "entrenamiento:" + entrenamiento + ":punto" }

// lima es la zona en que se escriben las fechas: el mismo desfase que
// kmeans.ZonaPeru, declarado aquí para no depender del núcleo.
var lima = time.FixedZone("-05", -5*60*60)

// enLima deja un instante con la forma única con que se guarda: hora de Lima,
// al segundo.
func enLima(t time.Time) time.Time { return t.In(lima).Truncate(time.Second) }

// guionRegimen guarda el régimen de una estación si el que hay no es de una
// fecha_hora posterior. KEYS[1] es la clave; ARGV[1], el valor JSON; ARGV[2],
// su fecha_hora. Devuelve 1 si escribió y 0 si conservó el guardado. Un valor
// guardado que no se puede leer se reemplaza.
var guionRegimen = driver.NewScript(`
local guardado = redis.call('GET', KEYS[1])
if guardado then
  local ok, doc = pcall(cjson.decode, guardado)
  if ok and type(doc) == 'table' and type(doc.fecha_hora) == 'string' and doc.fecha_hora > ARGV[2] then
    return 0
  end
end
redis.call('SET', KEYS[1], ARGV[1])
return 1
`)

// Opciones de la conexión. DB es el número de base de datos de Redis: la API
// usa la 0 y las pruebas de integración, otra.
type Opciones struct {
	Addr string // host:puerto
	DB   int
}

// Cache es la caché de la API en Redis.
type Cache struct{ cliente *driver.Client }

var (
	_ almacen.Cache         = (*Cache)(nil)
	_ almacen.CacheModelo   = (*Cache)(nil)
	_ almacen.PuntosControl = (*Cache)(nil)
	_ almacen.Pinger        = (*Cache)(nil)
)

// Conectar prepara el cliente. Solo falla si falta la dirección: no espera a
// que Redis responda, y el cliente se conecta y se vuelve a conectar solo. Para
// saber si responde está Ping.
func Conectar(op Opciones) (*Cache, error) {
	if op.Addr == "" {
		return nil, errors.New("redis: falta la dirección (host:puerto)")
	}
	return &Cache{cliente: driver.NewClient(&driver.Options{
		Addr:         op.Addr,
		DB:           op.DB,
		ClientName:   "aire-distribuido-api",
		DialTimeout:  2 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
		// Con Redis caído un comando falla enseguida y lo cuenta quien lo
		// pidió, en vez de reintentar dentro del driver.
		MaxRetries:            -1,
		ContextTimeoutEnabled: true,
	})}, nil
}

// Ping dice si Redis responde.
func (c *Cache) Ping(ctx context.Context) error { return c.cliente.Ping(ctx).Err() }

// Cerrar cierra las conexiones.
func (c *Cache) Cerrar() error { return c.cliente.Close() }

// GuardarRegimen guarda el régimen de una sola estación, con la regla de
// GuardarRegimenes.
func (c *Cache) GuardarRegimen(ctx context.Context, estacion string, r almacen.RegimenEstacion) error {
	return c.GuardarRegimenes(ctx, map[string]almacen.RegimenEstacion{estacion: r})
}

// GuardarRegimenes manda un guionRegimen por estación en un solo pipeline: una
// ida y vuelta para todo el lote. Cada script es atómico por su cuenta.
func (c *Cache) GuardarRegimenes(ctx context.Context, rs map[string]almacen.RegimenEstacion) error {
	if len(rs) == 0 {
		return nil
	}
	type pedido struct {
		clave        string
		valor, fecha string
	}
	pedidos := make([]pedido, 0, len(rs))
	for estacion, r := range rs {
		r.FechaHora, r.TS = enLima(r.FechaHora), enLima(r.TS)
		valor, err := json.Marshal(r)
		if err != nil {
			return fmt.Errorf("redis: régimen de %s: %w", estacion, err)
		}
		pedidos = append(pedidos, pedido{claveRegimen(estacion), string(valor), r.FechaHora.Format(time.RFC3339)})
	}
	enviar := func() error {
		_, err := c.cliente.Pipelined(ctx, func(p driver.Pipeliner) error {
			for _, pe := range pedidos {
				guionRegimen.EvalSha(ctx, p, []string{pe.clave}, pe.valor, pe.fecha)
			}
			return nil
		})
		return err
	}
	err := enviar()
	// Redis no conoce el script (es la primera vez, o se reinició): se carga y
	// se repite el lote entero. Repetir es seguro: volver a escribir el mismo
	// régimen no cambia nada.
	if driver.HasErrorPrefix(err, "NOSCRIPT") {
		if err := guionRegimen.Load(ctx, c.cliente).Err(); err != nil {
			return err
		}
		err = enviar()
	}
	return err
}

func (c *Cache) Regimen(ctx context.Context, estacion string) (almacen.RegimenEstacion, error) {
	var r almacen.RegimenEstacion
	return r, c.leer(ctx, claveRegimen(estacion), &r)
}

// Regimenes pide todas las claves con un solo MGET: una ida y vuelta para el
// catálogo entero. Una clave que no existe llega como nil y no entra al mapa.
func (c *Cache) Regimenes(ctx context.Context, estaciones []string) (map[string]almacen.RegimenEstacion, error) {
	rs := make(map[string]almacen.RegimenEstacion, len(estaciones))
	if len(estaciones) == 0 {
		return rs, nil
	}
	claves := make([]string, len(estaciones))
	for i, estacion := range estaciones {
		claves[i] = claveRegimen(estacion)
	}
	valores, err := c.cliente.MGet(ctx, claves...).Result()
	if err != nil {
		return nil, err
	}
	for i, v := range valores {
		if v == nil {
			continue
		}
		texto, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("redis: %s: el valor no es un texto", claves[i])
		}
		var r almacen.RegimenEstacion
		if err := json.Unmarshal([]byte(texto), &r); err != nil {
			return nil, fmt.Errorf("redis: %s: %w", claves[i], err)
		}
		rs[estaciones[i]] = r
	}
	return rs, nil
}

// GuardarModeloActivo escribe modelo:activo. creado se guarda en la hora de
// Lima, con sus nanosegundos.
func (c *Cache) GuardarModeloActivo(ctx context.Context, m almacen.Modelo) error {
	m.Creado = m.Creado.In(lima)
	return c.guardar(ctx, claveModeloActivo, m)
}

func (c *Cache) ModeloActivo(ctx context.Context) (almacen.Modelo, error) {
	var m almacen.Modelo
	return m, c.leer(ctx, claveModeloActivo, &m)
}

func (c *Cache) GuardarPunto(ctx context.Context, entrenamiento string, p almacen.PuntoControl) error {
	return c.guardar(ctx, clavePunto(entrenamiento), p)
}

func (c *Cache) Punto(ctx context.Context, entrenamiento string) (almacen.PuntoControl, error) {
	var p almacen.PuntoControl
	return p, c.leer(ctx, clavePunto(entrenamiento), &p)
}

func (c *Cache) guardar(ctx context.Context, clave string, v any) error {
	valor, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("redis: %s: %w", clave, err)
	}
	return c.cliente.Set(ctx, clave, valor, 0).Err()
}

// leer decodifica el valor JSON de una clave, o devuelve ErrNoEncontrado si no
// existe.
func (c *Cache) leer(ctx context.Context, clave string, v any) error {
	valor, err := c.cliente.Get(ctx, clave).Bytes()
	if errors.Is(err, driver.Nil) {
		return almacen.ErrNoEncontrado
	}
	if err != nil {
		return err
	}
	if err := json.Unmarshal(valor, v); err != nil {
		return fmt.Errorf("redis: %s: %w", clave, err)
	}
	return nil
}
