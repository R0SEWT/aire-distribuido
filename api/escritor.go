package api

import (
	"context"
	"errors"
	"log/slog"
	"maps"
	"slices"
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
//
// La goroutine de Redis, además, avisa (alGuardar) de los regímenes que
// quedaron guardados, que es de donde salen los eventos estacion.regimen. Si
// un lote termina con error no se sabe qué guardó Redis: cuando vuelve a
// responder se le pregunta (releer) y se avisa de eso.
type escritor struct {
	mongo, redis *cola
	plazoVaciado time.Duration
	// alGuardar, si no es nil, recibe después de cada lote de Redis los
	// regímenes que quedaron guardados: los que pasaron a ser el estado de su
	// estación. Es de donde salen los eventos estacion.regimen. Lo llama la
	// goroutine de Redis, así que no puede bloquear; se pone antes de correr.
	alGuardar func(map[string]almacen.RegimenEstacion)

	cache almacen.Cache
	log   *slog.Logger
	// La relectura (releer). Todo esto es de la goroutine de Redis, salvo los
	// contadores.
	cadaRelectura      time.Duration       // cuánto se espera antes de releer, y entre intentos
	porReleer          map[string]struct{} // estaciones de lotes con error, de las que no se sabe qué quedó
	releerDesde        time.Time           // no antes de este momento
	relecturas         atomic.Uint64       // relecturas que la caché respondió
	relecturasFallidas atomic.Uint64       // intentos en que la caché seguía sin responder
}

// cadaRelecturaPorOmision es lo que se espera, después de un lote de Redis con
// error, antes de preguntarle a Redis qué quedó guardado, y entre un intento
// fallido y el siguiente.
const cadaRelecturaPorOmision = time.Second

func nuevoEscritor(tam int, p almacen.Predicciones, c almacen.Cache, log *slog.Logger) *escritor {
	e := &escritor{plazoVaciado: plazoVaciado, cache: c, log: log,
		cadaRelectura: cadaRelecturaPorOmision, porReleer: map[string]struct{}{}}
	e.mongo = nuevaCola("mongo", tam, p.GuardarLote, log)
	e.redis = nuevaCola("redis", tam, e.guardarRegimenes, log)
	e.redis.repaso = e.releer
	return e
}

// guardarRegimenes escribe un lote en Redis y avisa de lo que quedó guardado.
func (e *escritor) guardarRegimenes(ctx context.Context, lote []almacen.Prediccion) error {
	rs := regimenes(lote)
	guardados, err := e.cache.GuardarRegimenes(ctx, rs)
	// Aunque el lote haya fallado, lo que la caché sabe que guardó ya es el
	// estado de esas estaciones.
	e.avisar(guardados)
	if err != nil {
		// De las demás no se sabe: puede que Redis haya ejecutado el script y
		// que solo la respuesta no haya llegado (un plazo vencido, la conexión
		// cortada). Ese estado ya lo devuelve GET /api/regimenes, y si no se
		// avisara el WebSocket lo contradiría hasta el siguiente cambio de la
		// estación. Se anotan para preguntarle a Redis cuando vuelva (releer).
		for estacion := range rs {
			if _, ok := guardados[estacion]; !ok {
				e.porReleer[estacion] = struct{}{}
			}
		}
		e.releerDesde = time.Now().Add(e.cadaRelectura)
	}
	return err
}

func (e *escritor) avisar(guardados map[string]almacen.RegimenEstacion) {
	if e.alGuardar != nil && len(guardados) > 0 {
		e.alGuardar(guardados)
	}
}

// releer le pregunta a Redis por el estado de las estaciones de los lotes que
// terminaron con error y avisa de lo que haya guardado, sea de ese lote o de
// antes: un evento repetido no hace daño; uno perdido deja al panel con un
// estado que el GET ya no tiene. Es el repaso de la cola de Redis: corre en su
// goroutine después de cada lote y cuando vence la espera, y devuelve cuánto
// falta para el próximo intento, o 0 si no queda nada por releer.
//
// No se intenta enseguida del error: si Redis no responde, otra lectura
// costaría otro plazo entero con la cola detenida. Se espera cadaRelectura
// desde el último lote fallido o desde el último intento.
func (e *escritor) releer(ctx context.Context) time.Duration {
	if len(e.porReleer) == 0 {
		return 0
	}
	if falta := time.Until(e.releerDesde); falta > 0 {
		return falta
	}
	estaciones := slices.Sorted(maps.Keys(e.porReleer))
	ctx, cancel := context.WithTimeout(ctx, plazoEscritura)
	defer cancel()
	rs, err := e.cache.Regimenes(ctx, estaciones)
	var ilegibles *almacen.RegimenesIlegibles
	if err != nil && !errors.As(err, &ilegibles) {
		e.relecturasFallidas.Add(1)
		e.releerDesde = time.Now().Add(e.cadaRelectura)
		return e.cadaRelectura
	}
	e.relecturas.Add(1)
	clear(e.porReleer)
	e.log.Info("régimen releído después de un lote con error", "almacen", e.redis.nombre, "estaciones", len(estaciones), "con_estado", len(rs))
	e.avisar(rs)
	return 0
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

	// repaso, si no es nil, corre en la goroutine de la cola después de cada
	// lote y cuando vence la espera que él mismo pidió: devuelve cuánto falta
	// para volver a llamarlo, o 0 si no tiene nada pendiente. No corre durante
	// el vaciado del apagado.
	repaso func(context.Context) time.Duration

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

// correr es la goroutine del almacén: escribe lo que llega hasta que ctx se
// cancela y entonces vacía la cola. Desde que ctx se cancela no empieza ninguna
// escritura fuera del vaciado, así que qué se escribe y qué se descarta al
// apagar no depende del azar. El canal no se cierra nunca: un handler que
// todavía esté respondiendo no puede mandar a un canal cerrado.
func (c *cola) correr(ctx, duro context.Context) {
	lote := make([]almacen.Prediccion, 0, maxLote)
	var despertar <-chan time.Time // nil mientras el repaso no tenga nada pendiente
	for {
		// Se mira antes del select: con ctx cancelado y la cola con datos, el
		// select elegiría al azar entre los dos casos y el vaciado empezaría
		// una, dos o ninguna escritura después.
		if ctx.Err() != nil {
			c.vaciar(duro, lote)
			return
		}
		select {
		case p := <-c.ch:
			lote = c.completar(append(lote[:0], p))
			c.escribirLote(duro, lote)
		case <-despertar:
		case <-ctx.Done():
			c.vaciar(duro, lote)
			return
		}
		despertar = nil
		if c.repaso != nil {
			if espera := c.repaso(duro); espera > 0 {
				despertar = time.After(espera)
			}
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
