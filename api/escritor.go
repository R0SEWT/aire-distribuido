package api

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/R0SEWT/aire-distribuido/almacen"
)

const (
	// plazoEscritura es lo que puede tardar un lote en el almacén antes de
	// darlo por fallido.
	plazoEscritura = 2 * time.Second
	// plazoVaciado es lo que el escritor sigue escribiendo después de que se
	// le pide parar. Con los 5 s que cmd/api les da a los pedidos en curso,
	// el apagado completo queda por debajo de los 10 s que Docker espera
	// antes de matar el proceso.
	plazoVaciado = 3 * time.Second
	// maxLote es el tope de predicciones por ida y vuelta al almacén.
	maxLote = 256
)

// escritor saca la persistencia del camino del pedido (spec de la PC4, §6.2):
// el handler encola la predicción y responde sin esperar. Hay una cola con
// búfer y una goroutine por almacén, una para MongoDB (la colección
// predicciones) y otra para Redis (el último régimen de cada estación), así
// que un almacén lento o caído no frena al otro.
//
// Cada goroutine escribe por lotes: toma todo lo que haya en su cola, hasta
// maxLote, y lo manda en una sola ida y vuelta. Con el almacén al día el lote
// es de una predicción y no se agrega espera; cuando se atrasa, los lotes
// crecen solos y el costo por predicción baja. Si aun así una cola se llena,
// la predicción no se escribe en ese almacén: se cuenta el descarte y el
// pedido responde igual.
type escritor struct {
	mongo, redis *cola
	plazoVaciado time.Duration
}

func nuevoEscritor(tam int, p almacen.Predicciones, c almacen.Cache, log *slog.Logger) *escritor {
	return &escritor{
		mongo: nuevaCola("mongo", tam, p.GuardarLote, log),
		redis: nuevaCola("redis", tam, func(ctx context.Context, lote []almacen.Prediccion) error {
			return c.GuardarRegimenes(ctx, regimenes(lote))
		}, log),
		plazoVaciado: plazoVaciado,
	}
}

// regimenes reduce un lote a lo último de cada estación: régimen, media de
// 24 h, alerta e INCA de su lectura de fecha_hora más reciente (entre dos de la
// misma hora, la que llegó después). La caché aplica la misma regla contra lo
// que ya tiene guardado (almacen.Cache).
func regimenes(lote []almacen.Prediccion) map[string]almacen.RegimenEstacion {
	rs := make(map[string]almacen.RegimenEstacion)
	for _, p := range lote {
		if r, ok := rs[p.Estacion]; ok && r.FechaHora.After(p.FechaHora) {
			continue
		}
		rs[p.Estacion] = almacen.RegimenEstacion{
			FechaHora: p.FechaHora, Regimen: p.Regimen, ExcedeECA: p.ExcedeECA,
			Media24h: p.Media24h, Alerta: p.Alerta, INCA: p.INCA, TS: p.TS,
		}
	}
	return rs
}

// encolar nunca bloquea. Devuelve false si alguna de las dos colas estaba
// llena: la predicción entra igual en la otra.
func (e *escritor) encolar(p almacen.Prediccion) bool {
	enMongo, enRedis := e.mongo.encolar(p), e.redis.encolar(p)
	return enMongo && enRedis
}

// correr escribe lo que llega a las colas hasta que ctx se cancela; entonces
// sigue hasta vaciarlas o hasta que pase plazoVaciado, lo que ocurra primero,
// y vuelve. Lo que no alcanzó a escribirse se cuenta como descarte.
func (e *escritor) correr(ctx context.Context) {
	// duro corta las escrituras en curso y el vaciado: se cancela plazoVaciado
	// después que ctx.
	duro, cortar := context.WithCancel(context.Background())
	defer cortar()
	dejarDeEsperar := context.AfterFunc(ctx, func() {
		t := time.AfterFunc(e.plazoVaciado, cortar)
		context.AfterFunc(duro, func() { t.Stop() })
	})
	defer dejarDeEsperar()

	var wg sync.WaitGroup
	for _, c := range []*cola{e.mongo, e.redis} {
		wg.Go(func() { c.correr(ctx, duro) })
	}
	wg.Wait()
}

// estadoEscritura es lo que /api/metricas publica de cada almacén.
type estadoEscritura struct {
	Escritas    uint64 `json:"escritas"`
	Descartadas uint64 `json:"descartadas"`
	Fallidas    uint64 `json:"fallidas"`
	Lotes       uint64 `json:"lotes"`
}

func (e *escritor) estado() map[string]estadoEscritura {
	return map[string]estadoEscritura{e.mongo.nombre: e.mongo.estado(), e.redis.nombre: e.redis.estado()}
}

// descartadas y fallidas suman las de los dos almacenes: cada una es una
// escritura que no se hizo.
func (e *escritor) descartadas() uint64 {
	return e.mongo.descartadas.Load() + e.redis.descartadas.Load()
}

func (e *escritor) fallidas() uint64 { return e.mongo.fallidas.Load() + e.redis.fallidas.Load() }

// cola es la cola de escritura de un almacén y sus contadores.
type cola struct {
	nombre   string
	ch       chan almacen.Prediccion
	escribir func(context.Context, []almacen.Prediccion) error
	log      *slog.Logger

	escritas    atomic.Uint64 // predicciones en lotes que el almacén aceptó
	descartadas atomic.Uint64 // cola llena, o sin escribir al apagar
	fallidas    atomic.Uint64 // predicciones en lotes que el almacén rechazó
	lotes       atomic.Uint64 // idas y vueltas al almacén
}

func nuevaCola(nombre string, tam int, escribir func(context.Context, []almacen.Prediccion) error, log *slog.Logger) *cola {
	return &cola{nombre: nombre, ch: make(chan almacen.Prediccion, tam), escribir: escribir, log: log}
}

func (c *cola) encolar(p almacen.Prediccion) bool {
	select {
	case c.ch <- p:
		return true
	default:
		c.descartadas.Add(1)
		return false
	}
}

func (c *cola) estado() estadoEscritura {
	return estadoEscritura{c.escritas.Load(), c.descartadas.Load(), c.fallidas.Load(), c.lotes.Load()}
}

// correr es la goroutine del almacén. El canal no se cierra nunca: un handler
// que todavía esté respondiendo no puede mandar a un canal cerrado.
func (c *cola) correr(ctx, duro context.Context) {
	lote := make([]almacen.Prediccion, 0, maxLote)
	for {
		select {
		case p := <-c.ch:
			lote = c.completar(append(lote[:0], p))
			c.escribirLote(duro, lote)
		case <-ctx.Done():
			c.vaciar(duro, lote)
			return
		}
	}
}

// completar agrega al lote lo que ya esté en la cola, sin esperar, hasta maxLote.
func (c *cola) completar(lote []almacen.Prediccion) []almacen.Prediccion {
	for len(lote) < maxLote {
		select {
		case p := <-c.ch:
			lote = append(lote, p)
		default:
			return lote
		}
	}
	return lote
}

// vaciar escribe lo que quedó en la cola. Si duro se cancela antes de
// terminar, cuenta el resto como descarte y lo deja en el log.
func (c *cola) vaciar(duro context.Context, lote []almacen.Prediccion) {
	for {
		lote = c.completar(lote[:0])
		if len(lote) == 0 {
			return
		}
		if duro.Err() != nil {
			sinEscribir := len(lote) + len(c.ch)
			c.descartadas.Add(uint64(sinEscribir))
			c.log.Error("apagado: quedaron predicciones sin escribir", "almacen", c.nombre, "predicciones", sinEscribir)
			return
		}
		c.escribirLote(duro, lote)
	}
}

func (c *cola) escribirLote(duro context.Context, lote []almacen.Prediccion) {
	ctx, cancel := context.WithTimeout(duro, plazoEscritura)
	defer cancel()
	c.lotes.Add(1)
	if err := c.escribir(ctx, lote); err != nil {
		c.fallidas.Add(uint64(len(lote)))
		c.log.Error("no se escribió el lote de predicciones", "almacen", c.nombre, "predicciones", len(lote), "err", err)
		return
	}
	c.escritas.Add(uint64(len(lote)))
}
