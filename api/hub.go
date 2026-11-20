package api

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

// opcionesHub son los tiempos y tamaños del hub. Las pruebas los achican.
type opcionesHub struct {
	// buferCliente es cuántos eventos puede tener pendientes un cliente antes
	// de que el hub lo dé por lento. Un entrenamiento emite a lo sumo
	// max_iter + 1 eventos (100 rondas y el fin) en unas decenas de ms: con 256
	// un cliente que lee con normalidad nunca se llena.
	buferCliente   int
	buferDifusion  int
	cadaPing       time.Duration // cada cuánto el servidor manda un ping
	plazoPong      time.Duration // lo que el cliente tiene para dar señales de vida después
	plazoEscritura time.Duration // lo que puede tardar una escritura en la conexión
	graciaCierre   time.Duration // lo que se espera la respuesta del cliente a un cierre
}

func opcionesHubPorOmision() opcionesHub {
	return opcionesHub{buferCliente: 256, buferDifusion: 256, cadaPing: 30 * time.Second, plazoPong: 10 * time.Second,
		plazoEscritura: 5 * time.Second, graciaCierre: time.Second}
}

// Hub reparte los eventos de la API a los clientes del WebSocket (spec de la
// PC4, §6.4). Una sola goroutine (Correr) es dueña del conjunto de clientes y
// lo cambia solo por lo que le llega por tres canales:
//
//   - altas: un cliente que acaba de conectarse.
//   - bajas: un cliente cuya conexión terminó (cerró, se cortó o no se le pudo
//     escribir).
//   - difusion: un evento para todos los clientes a los que les corresponde.
//
// Como el mapa de clientes lo toca una sola goroutine, no hay candado.
//
// Cada cliente tiene su propio canal con búfer y su propia goroutine
// escritora, que es la única que escribe en su conexión. El hub nunca espera a
// un cliente: si al difundir el búfer de uno está lleno, ese cliente es lento
// y el hub lo da de baja (su escritora le manda un cierre 1008 y corta); los
// demás reciben el evento a tiempo. Un cliente lento no frena al hub ni al
// entrenamiento.
type Hub struct {
	op  opcionesHub
	log *slog.Logger

	altas    chan *clienteWS
	bajas    chan *clienteWS
	difusion chan difusion
	fin      chan struct{} // cerrado cuando el hub deja de atender sus canales

	mu       sync.Mutex
	cerrado  bool           // ya no se admiten clientes
	clientes sync.WaitGroup // las goroutines de los clientes (dos por cliente)

	conectados  atomic.Int64  // clientes anotados ahora
	lentos      atomic.Uint64 // clientes dados de baja por lentos
	descartados atomic.Uint64 // eventos que no entraron al canal de difusión
}

// difusion es un evento ya codificado en su trama, que se comparte entre los
// clientes (nadie la modifica), y los roles que lo reciben (nil: todos).
type difusion struct {
	trama []byte
	roles []string
}

// motivoBaja es por qué el hub dio de baja a un cliente: el cierre que su
// escritora le manda.
type motivoBaja struct {
	codigo int
	texto  string
}

// finLectura es lo que la goroutine lectora le avisa a la escritora cuando
// deja de leer: el cliente mandó un cierre (delCliente, con su código), rompió
// el protocolo (el código con que hay que cerrarle) o la conexión se cortó (0).
type finLectura struct {
	codigo     int
	delCliente bool
}

type clienteWS struct {
	conn   net.Conn
	lector *bufio.Reader
	rol    string
	expira time.Time // cuándo vence su token; cero: nunca

	envios    chan []byte                // tramas pendientes; lo cierra solo el hub
	pongs     chan []byte                // de la lectora a la escritora: responder este ping
	cierre    chan finLectura            // de la lectora a la escritora
	lectorFin chan struct{}              // cerrado cuando la lectora terminó
	baja      atomic.Pointer[motivoBaja] // lo pone el hub antes de cerrar envios
}

func nuevoHub(op opcionesHub, log *slog.Logger) *Hub {
	return &Hub{
		op: op, log: log,
		altas: make(chan *clienteWS), bajas: make(chan *clienteWS),
		difusion: make(chan difusion, op.buferDifusion), fin: make(chan struct{}),
	}
}

// Correr atiende el hub hasta que se cancela ctx. Entonces deja de admitir
// clientes, les manda a los conectados un cierre 1001 (el servidor se va) y
// vuelve cuando no queda ninguna goroutine de ningún cliente.
func (h *Hub) Correr(ctx context.Context) {
	clientes := map[*clienteWS]struct{}{}
	soltar := func(c *clienteWS, m motivoBaja) {
		delete(clientes, c)
		c.baja.Store(&m)
		close(c.envios)
		h.conectados.Store(int64(len(clientes)))
	}
	for {
		select {
		case c := <-h.altas:
			clientes[c] = struct{}{}
			h.conectados.Store(int64(len(clientes)))
		case c := <-h.bajas:
			// Puede que ya no esté: el hub lo soltó por lento mientras su
			// escritora avisaba que la conexión se cortó.
			if _, ok := clientes[c]; ok {
				delete(clientes, c)
				close(c.envios)
				h.conectados.Store(int64(len(clientes)))
			}
		case d := <-h.difusion:
			for c := range clientes {
				if d.roles != nil && !slices.Contains(d.roles, c.rol) {
					continue
				}
				select {
				case c.envios <- d.trama:
				default:
					h.lentos.Add(1)
					h.log.Warn("websocket: cliente lento, se desconecta", "remoto", c.conn.RemoteAddr().String(), "pendientes", len(c.envios))
					soltar(c, motivoBaja{cierrePolitica, "cliente lento: no lee al ritmo de los eventos"})
				}
			}
		case <-ctx.Done():
			h.mu.Lock()
			h.cerrado = true
			h.mu.Unlock()
			close(h.fin)
			for c := range clientes {
				soltar(c, motivoBaja{cierreSeVa, "la API se está apagando"})
			}
			h.clientes.Wait()
			return
		}
	}
}

var errHubCerrado = errors.New("websocket: el hub ya no atiende")

// Difundir manda un evento a los clientes conectados: una trama de texto con
// {"evento": evento, "datos": datos} (esquema EventoWS del contrato). Si se
// pasan roles, solo lo reciben los clientes con alguno de ellos.
//
// Nunca bloquea a quien llama: el evento entra al canal de difusión, que tiene
// búfer, y el hub lo reparte. El hub no espera a ningún cliente, así que ese
// búfer solo se llena si el hub no está corriendo; en ese caso el evento se
// descarta y se cuenta, como hace la cola de escritura de las predicciones.
// Los eventos que manda una misma goroutine llegan a cada cliente en ese orden.
func (h *Hub) Difundir(evento string, datos any, roles ...string) error {
	cuerpo, err := codificar(struct {
		Evento string `json:"evento"`
		Datos  any    `json:"datos"`
	}{evento, datos})
	if err != nil {
		return err
	}
	d := difusion{trama: tramaWS(opTexto, bytes.TrimRight(cuerpo, "\n")), roles: roles}
	select {
	case <-h.fin:
		return errHubCerrado
	default:
	}
	select {
	case h.difusion <- d:
		return nil
	default:
		h.descartados.Add(1)
		return errors.New("websocket: el canal de difusión está lleno; el evento se descartó")
	}
}

// admite dice si el hub todavía recibe clientes.
func (h *Hub) admite() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return !h.cerrado
}

// Atender se queda con una conexión que ya es un WebSocket: la anota en el hub
// y lanza sus dos goroutines. saludo, si no es nil, es lo primero que se
// escribe en la conexión (la respuesta 101): el cliente queda anotado ANTES de
// enterarse de que está conectado, así que no se pierde ningún evento que se
// difunda después de que vio el 101. Devuelve false, con la conexión cerrada,
// si el hub ya no admite clientes.
func (h *Hub) Atender(conn net.Conn, lector *bufio.Reader, rol string, expira time.Time, saludo []byte) bool {
	c := &clienteWS{
		conn: conn, lector: lector, rol: rol, expira: expira,
		envios: make(chan []byte, h.op.buferCliente), pongs: make(chan []byte, 1),
		cierre: make(chan finLectura, 1), lectorFin: make(chan struct{}),
	}
	h.mu.Lock()
	if h.cerrado {
		h.mu.Unlock()
		conn.Close()
		return false
	}
	h.clientes.Add(2) // con el candado: nunca después de que Correr empezó a esperar
	h.mu.Unlock()
	select {
	case h.altas <- c:
	case <-h.fin:
		h.clientes.Add(-2)
		conn.Close()
		return false
	}
	go h.leer(c)
	go h.escribir(c, saludo)
	return true
}

// leer es la goroutine lectora de un cliente. El cliente no manda datos que
// importen, pero hay que leer igual: es la única forma de ver sus pings, sus
// pongs y su cierre, y de notar que la conexión se cortó. Cada trama que llega
// renueva el plazo de lectura; si pasan un ping y su plazo sin ninguna, el
// cliente se da por muerto.
func (h *Hub) leer(c *clienteWS) {
	defer h.clientes.Done()
	defer close(c.lectorFin)
	enMensaje := false // hay un mensaje fragmentado a medias
	for {
		c.conn.SetReadDeadline(time.Now().Add(h.op.cadaPing + h.op.plazoPong))
		t, err := leerTrama(c.lector, maxCargaCliente)
		if err == nil {
			switch t.op {
			case opPing:
				select {
				case c.pongs <- t.carga:
				default: // ya hay un pong pendiente: con uno alcanza
				}
			case opPong:
			case opCierre:
				codigo, errC := codigoDeCierre(t.carga)
				if errC != nil {
					c.cierre <- finLectura{codigo: cierreProtocolo}
				} else {
					c.cierre <- finLectura{codigo: codigo, delCliente: true}
				}
				return
			case opTexto, opBinario:
				if enMensaje {
					err = errProtocoloWS // empezó otro mensaje sin terminar el anterior
				}
				enMensaje = !t.fin
			case opContinuacion:
				if !enMensaje {
					err = errProtocoloWS // una continuación sin mensaje que continuar
				}
				enMensaje = !t.fin
			}
		}
		switch {
		case err == nil:
		case errors.Is(err, errProtocoloWS):
			c.cierre <- finLectura{codigo: cierreProtocolo}
			return
		case errors.Is(err, errTramaGrande):
			c.cierre <- finLectura{codigo: cierreGrande}
			return
		default: // EOF, conexión cerrada o plazo vencido
			c.cierre <- finLectura{}
			return
		}
	}
}

// escribir es la goroutine escritora de un cliente: la única que escribe en su
// conexión y la que la cierra. Manda las tramas que el hub le deja en envios,
// los pongs que pide la lectora y un ping cada cadaPing; cada escritura tiene
// plazo, así que una conexión trabada no la retiene para siempre.
func (h *Hub) escribir(c *clienteWS, saludo []byte) {
	defer h.clientes.Done()
	mandar := func(trama []byte) error {
		c.conn.SetWriteDeadline(time.Now().Add(h.op.plazoEscritura))
		_, err := c.conn.Write(trama)
		return err
	}
	// cortar cierra la conexión y espera a la lectora, que sale al fallar su
	// lectura. Con eco, antes le da al cliente graciaCierre para responder el
	// cierre que se le acaba de mandar (RFC 6455, §7.1.1).
	cortar := func(eco bool) {
		if eco {
			select {
			case <-c.lectorFin:
			case <-time.After(h.op.graciaCierre):
			}
		}
		c.conn.Close()
		<-c.lectorFin
	}
	// salir es el final cuando la conexión falló o el cliente cerró: se le
	// avisa al hub, que puede haberlo soltado ya o estar apagándose.
	salir := func() {
		select {
		case h.bajas <- c:
		case <-h.fin:
		}
		cortar(false)
	}
	if saludo != nil {
		if err := mandar(saludo); err != nil {
			salir()
			return
		}
	}
	ping := time.NewTicker(h.op.cadaPing)
	defer ping.Stop()
	var vence <-chan time.Time
	if !c.expira.IsZero() {
		t := time.NewTimer(time.Until(c.expira))
		defer t.Stop()
		vence = t.C
	}
	for {
		// El hub lo dio de baja (lento, o la API se apaga): lo pendiente ya no
		// se manda, solo el cierre con el motivo.
		if m := c.baja.Load(); m != nil {
			mandar(tramaDeCierre(m.codigo, m.texto))
			cortar(true)
			return
		}
		var err error
		select {
		case trama, ok := <-c.envios:
			if !ok {
				if c.baja.Load() == nil { // no pasa: quien cierra envios sin motivo es por pedido de esta goroutine
					cortar(false)
					return
				}
				continue // el motivo está en baja
			}
			err = mandar(trama)
		case carga := <-c.pongs:
			err = mandar(tramaWS(opPong, carga))
		case <-ping.C:
			err = mandar(tramaWS(opPing, nil))
		case <-vence:
			mandar(tramaDeCierre(cierrePolitica, "el token expiró: pide otro y vuelve a conectar"))
			select {
			case h.bajas <- c:
			case <-h.fin:
			}
			cortar(true)
			return
		case f := <-c.cierre:
			if f.codigo != 0 {
				// Responde el cierre del cliente con su mismo código, o le
				// dice por qué se le cierra.
				mandar(tramaDeCierre(f.codigo, ""))
			}
			salir()
			return
		}
		if err != nil {
			salir()
			return
		}
	}
}
