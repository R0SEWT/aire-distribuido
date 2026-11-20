package cluster

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"net"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/R0SEWT/aire-distribuido/protocolo"
)

// Miembro es un nodo que se presentó: su enlace y de dónde llamó.
type Miembro struct {
	Enlace    *protocolo.Enlace
	Direccion string        // IP:puerto del nodo, tal como lo ve el coordinador
	Llegada   time.Duration // desde que el coordinador empezó a esperar
}

// Enlaces devuelve los enlaces de los miembros, en el mismo orden.
func Enlaces(ms []*Miembro) []*protocolo.Enlace {
	out := make([]*protocolo.Enlace, len(ms))
	for i, m := range ms {
		out[i] = m.Enlace
	}
	return out
}

// EsperarNodos acepta conexiones en ln hasta juntar n nodos que se presentaron
// con un HOLA válido, o hasta que se cancela ctx (plazo vencido o Ctrl+C). Cada
// conexión tiene plazoHola para presentarse. Si un nodo con el mismo nombre
// vuelve a llamar, la conexión nueva reemplaza a la vieja.
//
// No cierra ln, y al volver ya no acepta ni queda ninguna goroutine suya: se
// puede volver a llamar sobre el mismo listener (un pool de nodos) y la espera
// siguiente recibe todas sus conexiones. Las que estaban a medio presentarse
// tienen graciaSaludo para terminar (a un nodo con otra versión le llega su
// ERROR) y se cierran; esos nodos vuelven a llamar. Para despertar al
// aceptador sin cerrar ln le pone un plazo ya vencido y después lo quita, así
// que ln tiene que tener SetDeadline, como *net.TCPListener, y no conservar un
// plazo propio. Con otro net.Listener la espera funciona igual, pero el
// aceptador no se puede despertar: sigue hasta que se cierra ln, y si antes
// llega una conexión, la cierra.
//
// Devuelve los nodos ordenados por nombre: es el orden de los rangos y el de la
// fusión.
func EsperarNodos(ctx context.Context, ln net.Listener, n int, plazoHola time.Duration, lg *log.Logger) []*Miembro {
	r := &recepcion{
		ln: ln, plazoHola: plazoHola, lg: lg, inicio: time.Now(),
		llegan: make(chan *Miembro), hecho: make(chan struct{}), aceptador: make(chan struct{}),
		saludando: make(map[*protocolo.Enlace]struct{}),
	}
	go r.aceptar()
	defer r.cerrar()

	porNombre := make(map[string]*Miembro, n)
	for len(porNombre) < n {
		select {
		case m := <-r.llegan:
			if viejo, ok := porNombre[m.Enlace.Nodo]; ok {
				lg.Printf("%s volvió a llamar desde %s; reemplazo la conexión de %s", m.Enlace.Nodo, m.Direccion, viejo.Direccion)
				viejo.Enlace.Cerrar()
			}
			porNombre[m.Enlace.Nodo] = m
			lg.Printf("[%d/%d] %s desde %s (%d workers)", len(porNombre), n, m.Enlace.Nodo, m.Direccion, m.Enlace.Workers)
		case <-ctx.Done():
			lg.Printf("dejo de esperar con %d de %d nodos: %v", len(porNombre), n, context.Cause(ctx))
			n = len(porNombre)
		}
	}

	miembros := make([]*Miembro, 0, len(porNombre))
	for _, m := range porNombre {
		miembros = append(miembros, m)
	}
	slices.SortFunc(miembros, func(a, b *Miembro) int { return cmp.Compare(a.Enlace.Nodo, b.Enlace.Nodo) })
	return miembros
}

// graciaSaludo es lo que cerrar espera a las conexiones que todavía no mandaron
// su HOLA cuando termina la espera, si a su plazoHola le queda más que eso. Un
// HOLA que ya venía en camino se atiende: el nodo con otra versión se entera
// de por qué no entra, en vez de leer un EOF. El que no dijo nada se corta.
const graciaSaludo = time.Second

// recepcion es lo que una espera tiene corriendo: el aceptador y un saludo por
// cada conexión que todavía no mandó su HOLA. cerrar los termina a todos.
type recepcion struct {
	ln        net.Listener
	plazoHola time.Duration
	lg        *log.Logger
	inicio    time.Time

	llegan    chan *Miembro // los que se presentaron bien
	hecho     chan struct{} // cerrado: ya nadie recibe de llegan
	aceptador chan struct{} // cerrado cuando el aceptador terminó

	mu        sync.Mutex
	cerrada   bool                           // no se admiten más saludos
	saludando map[*protocolo.Enlace]struct{} // conexiones que todavía no mandaron su HOLA
	saludos   sync.WaitGroup
}

// conPlazo es un listener cuyo Accept se puede despertar sin cerrarlo:
// *net.TCPListener y *net.UnixListener.
type conPlazo interface {
	SetDeadline(time.Time) error
}

// aceptar lanza un saludo por conexión hasta que Accept falla: se cerró ln o
// cerrar le puso un plazo vencido.
func (r *recepcion) aceptar() {
	defer close(r.aceptador)
	for {
		conn, err := r.ln.Accept()
		if err != nil {
			return
		}
		e := protocolo.NuevoEnlace(conn)
		// Con el mismo Mutex que cerrar: o el saludo queda anotado antes de que
		// cerrar los recorra, o no llega a nacer.
		r.mu.Lock()
		if r.cerrada {
			r.mu.Unlock()
			e.Cerrar()
			return
		}
		r.saludando[e] = struct{}{}
		r.saludos.Add(1)
		r.mu.Unlock()
		go r.saludar(e, conn.RemoteAddr().String())
	}
}

// saludar espera el HOLA de una conexión y entrega el miembro.
func (r *recepcion) saludar(e *protocolo.Enlace, direccion string) {
	defer r.saludos.Done()
	err := protocolo.RecibirHola(e, time.Now().Add(r.plazoHola))
	r.mu.Lock()
	delete(r.saludando, e)
	r.mu.Unlock()
	if err != nil {
		if !errors.Is(err, net.ErrClosed) { // la cortó cerrar: no es un rechazo
			r.lg.Printf("rechazado %s: %v", direccion, err)
		}
		e.Cerrar()
		return
	}
	m := &Miembro{Enlace: e, Direccion: direccion, Llegada: time.Since(r.inicio)}
	select {
	case r.llegan <- m:
	case <-r.hecho:
		e.Cerrar()
	}
}

// cerrar termina la recepción y vuelve cuando no queda ninguna goroutine suya
// (con un listener sin plazos, salvo el aceptador: ver EsperarNodos).
func (r *recepcion) cerrar() {
	close(r.hecho)
	r.mu.Lock()
	r.cerrada = true // desde aquí el aceptador no lanza más saludos
	pendientes := len(r.saludando)
	r.mu.Unlock()
	// Un plazo vencido hace fallar el Accept en curso, o el siguiente, sin
	// cerrar el listener; las conexiones que lleguen después quedan en la cola
	// del sistema para quien acepte luego.
	if ln, ok := r.ln.(conPlazo); ok && ln.SetDeadline(time.Now()) == nil {
		<-r.aceptador
		ln.SetDeadline(time.Time{})
	}
	if pendientes > 0 {
		gracia := time.AfterFunc(graciaSaludo, r.cortarSaludos)
		defer gracia.Stop()
	}
	r.saludos.Wait()
}

// cortarSaludos cierra las conexiones que siguen sin mandar su HOLA: destraba
// la lectura de cada saludo.
func (r *recepcion) cortarSaludos() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for e := range r.saludando {
		e.Cerrar()
	}
}

// Terminar manda FIN a cada nodo (la corrida salió bien: el nodo libera su
// partición) y cierra todas las conexiones. Los nodos vuelven a llamar solos,
// listos para la corrida siguiente. Devuelve los FIN que no se pudieron enviar.
func Terminar(enlaces []*protocolo.Enlace, plazo time.Duration) error {
	return despedir(enlaces, protocolo.Mensaje{Tipo: protocolo.Fin}, plazo)
}

// Abortar le manda a cada nodo un ERROR con el motivo (para que lo vea en su
// ventana) y cierra todas las conexiones. Es el final de una corrida que no
// produjo modelo; los nodos vuelven a llamar solos.
func Abortar(enlaces []*protocolo.Enlace, motivo string, plazo time.Duration) {
	const maximo = 4 << 10 // un error con muchos nodos puede ser largo; al nodo le basta el comienzo
	if len(motivo) > maximo {
		motivo = strings.ToValidUTF8(motivo[:maximo], "") + "…" // sin media runa al final
	}
	despedir(enlaces, protocolo.Mensaje{Tipo: protocolo.Error, Mensaje: "corrida abortada por el coordinador: " + motivo}, plazo)
}

func despedir(enlaces []*protocolo.Enlace, m protocolo.Mensaje, plazo time.Duration) error {
	limite := time.Now().Add(plazo)
	var errs []error
	for _, e := range enlaces {
		if e.Cod.Roto() == nil {
			if err := e.Cod.EscribirAntesDe(m, limite); err != nil {
				errs = append(errs, fmt.Errorf("nodo %q: enviar %s: %w", e.Nodo, m.Tipo, err))
			}
		}
		e.Cerrar()
	}
	return errors.Join(errs...)
}

// Percentil devuelve el percentil p (0 a 100) por rango más cercano. Con xs
// vacío devuelve 0.
func Percentil(xs []time.Duration, p float64) time.Duration {
	if len(xs) == 0 {
		return 0
	}
	s := slices.Clone(xs)
	slices.Sort(s)
	i := int(math.Ceil(float64(len(s))*p/100)) - 1
	return s[max(0, min(i, len(s)-1))]
}

// DireccionesLocales dice a qué IP:puerto pueden llamar los nodos, para el
// coordinador.txt del USB: la de addr si es una IP concreta y, si escucha en
// todas, cada IPv4 no loopback de esta máquina.
func DireccionesLocales(addr net.Addr) []string {
	host, puerto, err := net.SplitHostPort(addr.String())
	if err != nil {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil && !ip.IsUnspecified() {
		return []string{addr.String()}
	}
	ifaces, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	var out []string
	for _, a := range ifaces {
		if ip, ok := a.(*net.IPNet); ok && ip.IP.To4() != nil && !ip.IP.IsLoopback() {
			out = append(out, net.JoinHostPort(ip.IP.String(), puerto))
		}
	}
	return out
}
