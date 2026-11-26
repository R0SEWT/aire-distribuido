package api

import (
	"context"
	"sync/atomic"

	"github.com/R0SEWT/aire-distribuido/almacen"
)

// El punto de control de un entrenamiento (spec §5.2 y §7.2) es la clave
// entrenamiento:{id}:punto de Redis: la iteración y los centroides
// estandarizados de la última ronda completa. Se escribe en cada ronda, como
// registro; no se reanuda desde él.
//
// Es el tercer consumidor de cada ronda, con su propio canal, y el único al
// que solo le importa lo último: la clave guarda un solo punto. Por eso su
// canal es un buzón de un lugar. Si la goroutine que escribe todavía no tomó
// el punto anterior cuando llega el siguiente, el nuevo lo reemplaza y el
// reemplazado se cuenta como omitido. Así dejar un punto nunca espera a Redis:
// si responde al ritmo de las rondas se escriben todas; si está lento o caído,
// las rondas siguen, se saltean los puntos que ya quedaron viejos y, al
// terminar, a lo sumo quedan por hacer una escritura en curso y la del último
// punto.

// cuentaPuntos es lo que el coordinador hizo con los puntos de control desde
// que arrancó la API.
type cuentaPuntos struct {
	escritos atomic.Uint64 // puntos que Redis guardó
	fallidos atomic.Uint64 // escrituras que Redis rechazó o no respondió a tiempo
	omitidos atomic.Uint64 // puntos reemplazados por el de una ronda posterior antes de escribirse
}

// estadoPuntos es lo que /api/metricas publica de los puntos de control.
type estadoPuntos struct {
	Escritos uint64 `json:"escritos"`
	Fallidos uint64 `json:"fallidos"`
	Omitidos uint64 `json:"omitidos"`
}

func (c *cuentaPuntos) estado() estadoPuntos {
	return estadoPuntos{c.escritos.Load(), c.fallidos.Load(), c.omitidos.Load()}
}

// buzonPuntos lleva los puntos de control de un entrenamiento desde la
// goroutine de la corrida hasta la que los escribe. Un buzón nil no hace nada:
// es el de una API sin almacén de puntos.
type buzonPuntos struct {
	ch     chan almacen.PuntoControl // un lugar: el último punto sin escribir
	listo  chan struct{}             // se cierra cuando la goroutine que escribe terminó
	cuenta *cuentaPuntos
}

// abrirPuntos arranca la goroutine que escribe los puntos de control del
// entrenamiento id.
func (c *coordinador) abrirPuntos(id string) *buzonPuntos {
	puntos := c.s.puntos
	if puntos == nil {
		return nil
	}
	b := &buzonPuntos{ch: make(chan almacen.PuntoControl, 1), listo: make(chan struct{}), cuenta: &c.s.cuentaPuntos}
	go func() {
		defer close(b.listo)
		for p := range b.ch {
			ctx, cancel := context.WithTimeout(context.Background(), plazoEscritura)
			err := puntos.GuardarPunto(ctx, id, p)
			cancel()
			if err != nil {
				b.cuenta.fallidos.Add(1)
				c.log.Error("no se guardó el punto de control", "id", id, "iter", p.Iter, "err", err)
				continue
			}
			b.cuenta.escritos.Add(1)
		}
	}()
	return b
}

// dejar entrega el punto de una ronda sin esperar. Lo llama una sola
// goroutine, la de la corrida: por eso, después de sacar el punto viejo (o de
// que la otra goroutine se lo haya llevado), el lugar está libre.
func (b *buzonPuntos) dejar(p almacen.PuntoControl) {
	if b == nil {
		return
	}
	select {
	case b.ch <- p:
		return
	default:
	}
	select {
	case <-b.ch:
		b.cuenta.omitidos.Add(1)
	default:
	}
	b.ch <- p
}

// cerrar avisa que no hay más rondas y espera a que se escriba lo pendiente:
// a lo sumo la escritura en curso y la del último punto.
func (b *buzonPuntos) cerrar() {
	if b == nil {
		return
	}
	close(b.ch)
	<-b.listo
}

// puntoDe arma el punto de control de una ronda: sus centroides, por filas.
func puntoDe(iter, k int, centroides []float64) almacen.PuntoControl {
	p := almacen.PuntoControl{Iter: iter, Centroides: make([][]float64, k)}
	d := len(centroides) / k
	for j := range p.Centroides {
		p.Centroides[j] = centroides[j*d : (j+1)*d]
	}
	return p
}
