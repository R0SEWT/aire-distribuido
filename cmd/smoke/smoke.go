package main

import (
	"cmp"
	"context"
	"fmt"
	"log"
	"math"
	"net"
	"slices"
	"sync"
	"time"

	"github.com/R0SEWT/aire-distribuido/protocolo"
)

// miembro es un nodo que se presentó: su enlace, de dónde llamó y lo que se
// midió con él.
type miembro struct {
	enlace    *protocolo.Enlace
	direccion string        // IP:puerto del nodo, tal como lo ve el coordinador
	llegada   time.Duration // desde que el coordinador empezó a esperar
	latidos   []time.Duration
	err       error // primer fallo; el nodo sale de las rondas siguientes
}

// esperarNodos acepta conexiones en ln hasta juntar n nodos que se presentaron
// con un HOLA válido, o hasta que se cancela ctx (plazo vencido o Ctrl+C). Cada
// conexión tiene plazoHola para presentarse. Si un nodo con el mismo nombre
// vuelve a llamar, la conexión nueva reemplaza a la vieja.
//
// No cierra ln, pero deja de aceptar al volver: después, quien llama lo cierra.
// Devuelve los nodos ordenados por nombre.
func esperarNodos(ctx context.Context, ln net.Listener, n int, plazoHola time.Duration, lg *log.Logger) []*miembro {
	inicio := time.Now()
	// hecho avisa a las goroutines de saludo que ya nadie recibe: la que termina
	// después cierra su conexión en vez de quedarse bloqueada en llegan.
	hecho := make(chan struct{})
	defer close(hecho)
	llegan := make(chan *miembro)

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
				if err := protocolo.RecibirHola(e, time.Now().Add(plazoHola)); err != nil {
					lg.Printf("rechazado %s: %v", conn.RemoteAddr(), err)
					e.Cerrar()
					return
				}
				m := &miembro{enlace: e, direccion: conn.RemoteAddr().String(), llegada: time.Since(inicio)}
				select {
				case llegan <- m:
				case <-hecho:
					e.Cerrar()
				}
			}()
		}
	}()

	porNombre := make(map[string]*miembro, n)
	for len(porNombre) < n {
		select {
		case m := <-llegan:
			if viejo, ok := porNombre[m.enlace.Nodo]; ok {
				lg.Printf("%s volvió a llamar desde %s; reemplazo la conexión de %s", m.enlace.Nodo, m.direccion, viejo.direccion)
				viejo.enlace.Cerrar()
			}
			porNombre[m.enlace.Nodo] = m
			lg.Printf("[%d/%d] %s desde %s (%d workers)", len(porNombre), n, m.enlace.Nodo, m.direccion, m.enlace.Workers)
		case <-ctx.Done():
			lg.Printf("dejo de esperar con %d de %d nodos: %v", len(porNombre), n, context.Cause(ctx))
			n = len(porNombre)
		}
	}

	miembros := make([]*miembro, 0, len(porNombre))
	for _, m := range porNombre {
		miembros = append(miembros, m)
	}
	slices.SortFunc(miembros, func(a, b *miembro) int { return cmp.Compare(a.enlace.Nodo, b.enlace.Nodo) })
	return miembros
}

// medir hace rondas de latidos. En cada ronda manda un PING a todos los nodos
// sanos a la vez y espera todos los PONG, como la barrera de una iteración
// (protocolo.Ronda): fan-out con una goroutine por nodo y fan-in con un
// WaitGroup. Un nodo que falla queda con su error y sale de las rondas
// siguientes.
//
// Devuelve la duración de cada ronda en la que respondieron todos los nodos
// sanos: lo que la red le cobra a cada iteración del K-Means distribuido.
func medir(miembros []*miembro, rondas int, plazo time.Duration) []time.Duration {
	var barreras []time.Duration
	for range rondas {
		var sanos []*miembro
		for _, m := range miembros {
			if m.err == nil {
				sanos = append(sanos, m)
			}
		}
		if len(sanos) == 0 {
			break
		}

		limite := time.Now().Add(plazo)
		inicio := time.Now()
		var barrera sync.WaitGroup
		for _, m := range sanos {
			barrera.Go(func() {
				// Cada goroutine escribe solo en su miembro: no hace falta Mutex.
				d, err := protocolo.Latido(m.enlace, limite)
				if err != nil {
					m.err = err
					return
				}
				m.latidos = append(m.latidos, d)
			})
		}
		barrera.Wait()
		ronda := time.Since(inicio)

		if !slices.ContainsFunc(sanos, func(m *miembro) bool { return m.err != nil }) {
			barreras = append(barreras, ronda)
		}
	}
	return barreras
}

// cerrar manda FIN a los nodos sanos y cierra todas las conexiones. Los nodos
// vuelven a llamar solos, listos para la corrida siguiente.
func cerrar(miembros []*miembro, plazo time.Duration) {
	limite := time.Now().Add(plazo)
	for _, m := range miembros {
		if m.err == nil {
			if err := m.enlace.Cod.EscribirAntesDe(protocolo.Mensaje{Tipo: protocolo.Fin}, limite); err != nil && m.err == nil {
				m.err = fmt.Errorf("enviar FIN: %w", err)
			}
		}
		m.enlace.Cerrar()
	}
}

// percentil devuelve el percentil p (0 a 100) por rango más cercano. Con xs
// vacío devuelve 0.
func percentil(xs []time.Duration, p float64) time.Duration {
	if len(xs) == 0 {
		return 0
	}
	s := slices.Clone(xs)
	slices.Sort(s)
	i := int(math.Ceil(float64(len(s))*p/100)) - 1
	return s[max(0, min(i, len(s)-1))]
}
