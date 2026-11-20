package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"slices"
	"sync"
	"time"

	"github.com/R0SEWT/aire-distribuido/cluster"
	"github.com/R0SEWT/aire-distribuido/protocolo"
)

// miembro es un nodo que se presentó (cluster.Miembro) y lo que se midió con él.
type miembro struct {
	*cluster.Miembro
	latidos []time.Duration
	err     error // primer fallo; el nodo sale de las rondas siguientes
}

// esperarNodos es cluster.EsperarNodos, con lugar para lo que mide el smoke
// test. Devuelve los nodos ordenados por nombre.
func esperarNodos(ctx context.Context, ln net.Listener, n int, plazoHola time.Duration, lg *log.Logger) []*miembro {
	var out []*miembro
	for _, m := range cluster.EsperarNodos(ctx, ln, n, plazoHola, lg) {
		out = append(out, &miembro{Miembro: m})
	}
	return out
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
				d, err := protocolo.Latido(m.Enlace, limite)
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
			if err := m.Enlace.Cod.EscribirAntesDe(protocolo.Mensaje{Tipo: protocolo.Fin}, limite); err != nil && m.err == nil {
				m.err = fmt.Errorf("enviar FIN: %w", err)
			}
		}
		m.Enlace.Cerrar()
	}
}
