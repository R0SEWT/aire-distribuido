package cluster

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"slices"
	"sync"
	"time"

	"github.com/R0SEWT/aire-distribuido/protocolo"
)

// Tipos de EventoPool, los de la colección eventos_cluster (spec de la PC4, §7.1).
const (
	EventoAlta  = "alta"  // un nodo se presentó y respondió su primer PING
	EventoBaja  = "baja"  // un nodo salió del pool: dejó de responder o se fue con una corrida
	EventoError = "error" // un nodo se presentó y no llegó a entrar
)

// EventoPool es lo que el pool cuenta de la vida de sus nodos.
type EventoPool struct {
	TS        time.Time
	Tipo      string
	Nodo      string
	Direccion string
	Detalle   string
}

// InfoNodo es un nodo del pool tal como lo publica GET /api/cluster/nodos.
type InfoNodo struct {
	Nombre    string
	Direccion string        // IP:puerto del nodo, como lo ve el coordinador
	Workers   int           // los que declaró en su HOLA
	Latencia  time.Duration // ida y vuelta de su último PING
	Libre     bool          // false: está en una corrida
}

// ErrorFaltanNodos es lo que devuelve Tomar cuando no hay suficientes nodos
// libres. La API lo responde como 422.
type ErrorFaltanNodos struct{ Libres, Pedidos int }

func (e *ErrorFaltanNodos) Error() string {
	if e.Libres == 1 {
		return fmt.Sprintf("hay 1 nodo libre en el pool y se pidieron %d", e.Pedidos)
	}
	return fmt.Sprintf("hay %d nodos libres en el pool y se pidieron %d", e.Libres, e.Pedidos)
}

// OpcionesPool configura un Pool. Los plazos en cero toman su valor por omisión.
type OpcionesPool struct {
	PlazoHola   time.Duration // lo que una conexión nueva tiene para mandar su HOLA (5 s)
	CadaLatido  time.Duration // cada cuánto se manda un PING a los nodos libres (5 s)
	PlazoLatido time.Duration // lo que un nodo tiene para responder el PONG (2 s)
	// AlEvento, si no es nil, recibe cada alta, baja y error. Se llama desde la
	// goroutine del pool y desde las que llaman a Tomar y a Soltar, a veces a
	// la vez: tiene que ser segura para eso, y no puede bloquear ni llamar al
	// Pool.
	AlEvento func(EventoPool)
	Log      *log.Logger
}

// Pool es el conjunto de nodos conectados al coordinador de la API (spec de la
// PC4, §5.2). Los nodos llaman, se presentan y quedan libres; un entrenamiento
// se lleva K con Tomar y, al terminar o abortar, cierra esas conexiones y los
// devuelve con Soltar: los nodos vuelven a llamar solos, como después de
// cualquier FIN, así que la baja y el alta tras cada corrida son lo normal.
//
// Un protocolo.Enlace lo usa una sola goroutine a la vez. Por eso el latido
// corre SOLO sobre los nodos libres —un PING durante una ronda competiría con
// protocolo.Ronda por el mismo decodificador— y por eso hay dos candados:
//
//   - uso: quien lo tiene es el único que habla con los nodos libres. Lo toman
//     la ronda de latidos, el alta de un nodo y Tomar, que se excluyen entre sí.
//   - mu: protege los mapas. Lo toman las consultas (Nodos, Cantidad), que así
//     nunca esperan a la red.
//
// El orden es siempre uso y después mu.
type Pool struct {
	ln net.Listener
	op OpcionesPool

	uso sync.Mutex

	mu       sync.Mutex
	libres   map[string]*entrada
	ocupados map[*Miembro]*entrada
	cerrado  bool
}

type entrada struct {
	m        *Miembro
	latencia time.Duration
}

// NuevoPool arma el pool sobre un listener que ya escucha. No acepta a nadie
// hasta que corre Correr. El listener sigue siendo de quien lo abrió.
func NuevoPool(ln net.Listener, op OpcionesPool) *Pool {
	if op.PlazoHola <= 0 {
		op.PlazoHola = 5 * time.Second
	}
	if op.CadaLatido <= 0 {
		op.CadaLatido = 5 * time.Second
	}
	if op.PlazoLatido <= 0 {
		op.PlazoLatido = 2 * time.Second
	}
	if op.Log == nil {
		op.Log = log.New(io.Discard, "", 0)
	}
	return &Pool{ln: ln, op: op, libres: map[string]*entrada{}, ocupados: map[*Miembro]*entrada{}}
}

// Direccion es donde el pool escucha a los nodos.
func (p *Pool) Direccion() net.Addr { return p.ln.Addr() }

// Correr acepta nodos y les toma el pulso hasta que se cancela ctx. Al volver
// ya no acepta, no queda ninguna goroutine suya y las conexiones de los nodos
// libres están cerradas; las de una corrida en curso son de esa corrida. No
// cierra el listener.
//
// La aceptación es la de EsperarNodos (la misma recepcion): un saludo por
// conexión, con plazo para el HOLA y rechazo de otra versión del protocolo.
// Si el aceptador se cae sin que nadie lo haya pedido (un error del sistema en
// Accept), se levanta otro: el pool vive lo que vive la API.
func (p *Pool) Correr(ctx context.Context) {
	defer p.cerrar()
	latido := time.NewTicker(p.op.CadaLatido)
	defer latido.Stop()
	for ctx.Err() == nil {
		r := &recepcion{
			ln: p.ln, plazoHola: p.op.PlazoHola, lg: p.op.Log, inicio: time.Now(),
			llegan: make(chan *Miembro), hecho: make(chan struct{}), aceptador: make(chan struct{}),
			saludando: make(map[*protocolo.Enlace]struct{}),
		}
		go r.aceptar()
		caido := false
		for !caido && ctx.Err() == nil {
			select {
			case m := <-r.llegan:
				p.alta(m)
			case <-latido.C:
				p.uso.Lock()
				p.latir()
				p.uso.Unlock()
			case <-r.aceptador:
				caido = true
			case <-ctx.Done():
			}
		}
		r.cerrar()
		if caido && ctx.Err() == nil {
			p.op.Log.Printf("el pool dejó de aceptar nodos en %s; lo reintento", p.ln.Addr())
			dormir(ctx, time.Second)
		}
	}
}

// cerrar suelta a los nodos libres: cierra sus conexiones y los saca del pool.
func (p *Pool) cerrar() {
	p.uso.Lock()
	defer p.uso.Unlock()
	p.mu.Lock()
	p.cerrado = true
	libres := p.libres
	p.libres = map[string]*entrada{}
	p.mu.Unlock()
	for _, e := range libres {
		e.m.Enlace.Cerrar()
	}
}

// alta recibe a un nodo que ya mandó su HOLA: le toma el pulso y, si
// responde, lo deja libre. Su primer PING es su primera latencia y descarta al
// que se presentó y se fue. Si ya había un nodo libre con ese nombre, la
// conexión nueva reemplaza a la vieja, como en EsperarNodos.
func (p *Pool) alta(m *Miembro) {
	p.uso.Lock()
	defer p.uso.Unlock()
	latencia, err := protocolo.Latido(m.Enlace, time.Now().Add(p.op.PlazoLatido))
	if err != nil {
		m.Enlace.Cerrar()
		p.evento(EventoError, m, fmt.Sprintf("se presentó y no respondió el PING: %v", err))
		return
	}
	p.mu.Lock()
	if p.cerrado {
		p.mu.Unlock()
		m.Enlace.Cerrar()
		return
	}
	viejo := p.libres[m.Enlace.Nodo]
	p.libres[m.Enlace.Nodo] = &entrada{m: m, latencia: latencia}
	p.mu.Unlock()
	if viejo != nil {
		viejo.m.Enlace.Cerrar()
		p.evento(EventoBaja, viejo.m, fmt.Sprintf("volvió a llamar desde %s: la conexión nueva reemplaza a esta", m.Direccion))
	}
	p.evento(EventoAlta, m, fmt.Sprintf("%d workers, latencia %.2f ms", m.Enlace.Workers, float64(latencia.Microseconds())/1000))
}

// latir manda un PING a cada nodo libre, a todos a la vez y con un mismo
// instante límite, actualiza su latencia y da de baja al que no responde.
// Quien llama tiene uso.
func (p *Pool) latir() {
	p.mu.Lock()
	libres := make([]*entrada, 0, len(p.libres))
	for _, e := range p.libres {
		libres = append(libres, e)
	}
	p.mu.Unlock()
	if len(libres) == 0 {
		return
	}
	limite := time.Now().Add(p.op.PlazoLatido)
	latencias := make([]time.Duration, len(libres))
	errs := make([]error, len(libres))
	var barrera sync.WaitGroup
	for i, e := range libres {
		barrera.Go(func() {
			// Cada goroutine escribe solo su índice y usa solo su enlace.
			latencias[i], errs[i] = protocolo.Latido(e.m.Enlace, limite)
		})
	}
	barrera.Wait()

	type caido struct {
		e   *entrada
		err error
	}
	var caidos []caido
	p.mu.Lock()
	for i, e := range libres {
		if errs[i] == nil {
			e.latencia = latencias[i]
			continue
		}
		delete(p.libres, e.m.Enlace.Nodo) // con uso tomado nadie pudo reemplazarla
		caidos = append(caidos, caido{e, errs[i]})
	}
	p.mu.Unlock()
	slices.SortFunc(caidos, func(a, b caido) int { return cmp.Compare(a.e.m.Enlace.Nodo, b.e.m.Enlace.Nodo) })
	for _, c := range caidos {
		c.e.m.Enlace.Cerrar()
		p.evento(EventoBaja, c.e.m, fmt.Sprintf("dejó de responder el latido: %v", c.err))
	}
}

// Tomar saca n nodos libres del pool para una corrida: antes les toma el
// pulso, así que no entrega un nodo que se apagó desde el último latido. Los
// devuelve ordenados por nombre, que es el orden de los rangos y el de la
// fusión; si hay más de n, se lleva los n primeros en ese orden. Desde aquí
// las conexiones son de quien llama, que al terminar las cierra (Terminar o
// Abortar) y avisa con Soltar.
//
// Si no hay n libres devuelve *ErrorFaltanNodos y no se lleva ninguno.
func (p *Pool) Tomar(n int) ([]*Miembro, error) {
	if n <= 0 {
		return nil, fmt.Errorf("hay que pedir al menos un nodo, se pidieron %d", n)
	}
	p.uso.Lock()
	defer p.uso.Unlock()
	p.latir()
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.libres) < n {
		return nil, &ErrorFaltanNodos{Libres: len(p.libres), Pedidos: n}
	}
	nombres := make([]string, 0, len(p.libres))
	for nombre := range p.libres {
		nombres = append(nombres, nombre)
	}
	slices.Sort(nombres)
	ms := make([]*Miembro, n)
	for i, nombre := range nombres[:n] {
		e := p.libres[nombre]
		delete(p.libres, nombre)
		p.ocupados[e.m] = e
		ms[i] = e.m
	}
	return ms, nil
}

// Soltar saca del pool a los nodos que se llevó una corrida, cuando terminó o
// abortó, y deja una baja por cada uno con ese detalle. No cierra las
// conexiones: eso es de la corrida. Los nodos entran otra vez cuando vuelven a
// llamar.
func (p *Pool) Soltar(ms []*Miembro, detalle string) {
	var sueltos []*Miembro
	p.mu.Lock()
	for _, m := range ms {
		if _, ok := p.ocupados[m]; ok {
			delete(p.ocupados, m)
			sueltos = append(sueltos, m)
		}
	}
	p.mu.Unlock()
	for _, m := range sueltos {
		p.evento(EventoBaja, m, detalle)
	}
}

// Nodos lista los nodos del pool, libres y en una corrida, por nombre.
func (p *Pool) Nodos() []InfoNodo {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]InfoNodo, 0, len(p.libres)+len(p.ocupados))
	info := func(e *entrada, libre bool) InfoNodo {
		return InfoNodo{Nombre: e.m.Enlace.Nodo, Direccion: e.m.Direccion, Workers: e.m.Enlace.Workers, Latencia: e.latencia, Libre: libre}
	}
	for _, e := range p.libres {
		out = append(out, info(e, true))
	}
	for _, e := range p.ocupados {
		out = append(out, info(e, false))
	}
	slices.SortFunc(out, func(a, b InfoNodo) int {
		return cmp.Or(cmp.Compare(a.Nombre, b.Nombre), cmp.Compare(a.Direccion, b.Direccion))
	})
	return out
}

// Cantidad es cuántos nodos hay en el pool, libres y en una corrida.
func (p *Pool) Cantidad() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.libres) + len(p.ocupados)
}

func (p *Pool) evento(tipo string, m *Miembro, detalle string) {
	p.op.Log.Printf("pool: %s %s (%s): %s", tipo, m.Enlace.Nodo, m.Direccion, detalle)
	if p.op.AlEvento != nil {
		p.op.AlEvento(EventoPool{TS: time.Now(), Tipo: tipo, Nodo: m.Enlace.Nodo, Direccion: m.Direccion, Detalle: detalle})
	}
}
