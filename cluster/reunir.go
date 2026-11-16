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
// con un HOLA válido de la versión 1 (la del K-Means), o hasta que se cancela
// ctx (plazo vencido o Ctrl+C). Cada conexión tiene plazoHola para
// presentarse. Si un nodo con el mismo nombre vuelve a llamar, la conexión
// nueva reemplaza a la vieja.
//
// No cierra ln, pero deja de aceptar al volver: después, quien llama lo cierra.
// Devuelve los nodos ordenados por nombre: es el orden de los rangos y el de la
// fusión.
func EsperarNodos(ctx context.Context, ln net.Listener, n int, plazoHola time.Duration, lg *log.Logger) []*Miembro {
	return EsperarNodosVersion(ctx, ln, n, protocolo.Version, plazoHola, lg)
}

// EsperarNodosVersion es EsperarNodos para un coordinador que habla la versión
// version del protocolo (cmd/mapa-coord pide protocolo.Version2): un nodo que
// se presenta con otra recibe un ERROR que lo dice y no entra.
func EsperarNodosVersion(ctx context.Context, ln net.Listener, n, version int, plazoHola time.Duration, lg *log.Logger) []*Miembro {
	inicio := time.Now()
	// hecho avisa a las goroutines de saludo que ya nadie recibe: la que termina
	// después cierra su conexión en vez de quedarse bloqueada en llegan.
	hecho := make(chan struct{})
	defer close(hecho)
	llegan := make(chan *Miembro)

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return // ln cerrado: terminó la espera
			}
			select {
			case <-hecho:
				conn.Close()
				return
			default:
			}
			go func() {
				e := protocolo.NuevoEnlace(conn)
				if err := protocolo.RecibirHolaVersion(e, version, time.Now().Add(plazoHola)); err != nil {
					lg.Printf("rechazado %s: %v", conn.RemoteAddr(), err)
					e.Cerrar()
					return
				}
				m := &Miembro{Enlace: e, Direccion: conn.RemoteAddr().String(), Llegada: time.Since(inicio)}
				select {
				case llegan <- m:
				case <-hecho:
					e.Cerrar()
				}
			}()
		}
	}()

	porNombre := make(map[string]*Miembro, n)
	for len(porNombre) < n {
		select {
		case m := <-llegan:
			if viejo, ok := porNombre[m.Enlace.Nodo]; ok {
				lg.Printf("%s volvió a llamar desde %s; reemplazo la conexión de %s", m.Enlace.Nodo, m.Direccion, viejo.Direccion)
				viejo.Enlace.Cerrar()
			}
			porNombre[m.Enlace.Nodo] = m
			gpu := ""
			if m.Enlace.GPU != "" {
				gpu = fmt.Sprintf(", GPU %s de %.0f GB", m.Enlace.GPU, m.Enlace.VramGB)
			}
			lg.Printf("[%d/%d] %s desde %s (%d workers%s)", len(porNombre), n, m.Enlace.Nodo, m.Direccion, m.Enlace.Workers, gpu)
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
