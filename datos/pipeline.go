package datos

import (
	"fmt"
	"io"
	"sync"
	"time"
)

// leerBloques es el pipeline de la parte A:
//
//	productor ──índices──▶ trabajos ──▶ W workers ──parciales──▶ resultados ──▶ recolector
//	    ▲                                                                            │
//	    └──────────────────── enVuelo (semáforo de 2W bloques) ◀─────────────────────┘
//
// Un productor reparte los índices de bloque por un canal; W workers los
// procesan con su propio estado; esta goroutine recolecta. Los parciales llegan
// en cualquier orden y el recolector los fusiona en orden de índice, guardando
// los adelantados en pendientes: así «el primero» de un duplicado es el primero
// del archivo y el resultado no depende de W ni del planificador.
//
// El semáforo enVuelo acota la memoria: el productor toma un lugar antes de
// mandar un bloque y el recolector lo devuelve al fusionarlo, así que nunca hay
// más de 2W bloques leídos sin fusionar (búferes en los workers más parciales
// en pendientes). No puede trabarse: los lugares se toman en orden de índice,
// así que el bloque que el recolector espera siempre tiene el suyo.
func leerBloques(r io.ReaderAt, rangos []rango, reg *reglas, cols indicesColumnas, delim byte, workers int) (*acumulador, error) {
	workers = min(workers, max(len(rangos), 1))
	trabajos := make(chan int)
	resultados := make(chan *parcial)
	enVuelo := make(chan struct{}, 2*workers)
	fin := make(chan struct{}) // se cierra ante el primer error: corta al productor y a los workers

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := nuevoTrabajador(reg, cols, delim)
			for i := range trabajos {
				select {
				case resultados <- w.procesar(r, i, rangos[i]):
				case <-fin:
					return
				}
			}
		}()
	}
	go func() { // productor
		defer close(trabajos)
		for i := range rangos {
			select {
			case enVuelo <- struct{}{}:
			case <-fin:
				return
			}
			select {
			case trabajos <- i:
			case <-fin:
				return
			}
		}
	}()
	go func() {
		// Barrera: cuando el último worker termina, ya no llegan más
		// parciales y el recolector puede salir del range.
		wg.Wait()
		close(resultados)
	}()

	a := nuevoAcumulador()
	c := nuevaCosedora(r, rangos, reg, cols, delim)
	pendientes := map[int]*parcial{}
	siguiente := 0
	var primerErr error
	for p := range resultados {
		if primerErr != nil {
			continue // se drena el canal para que ningún worker quede colgado
		}
		pendientes[p.indice] = p
		for q, ok := pendientes[siguiente]; ok; q, ok = pendientes[siguiente] {
			delete(pendientes, siguiente)
			inicio := time.Now()
			// El error de un bloque se mira recién en su turno: si el bloque
			// anterior dejó un registro abierto, este empezó dentro de un campo
			// y su resultado (con o sin error) se descarta y se rehace.
			q, err := c.coser(q)
			if err == nil {
				a.fusionar(q)
			}
			a.tFusion += time.Since(inicio)
			siguiente++
			<-enVuelo
			if err != nil {
				primerErr = err
				close(fin)
				break
			}
		}
	}
	if primerErr == nil {
		primerErr = c.terminar()
	}
	if primerErr != nil {
		return nil, primerErr
	}
	if siguiente != len(rangos) {
		return nil, fmt.Errorf("se fusionaron %d de %d bloques", siguiente, len(rangos))
	}
	return a, nil
}

// cosedora une los registros que cruzan un borde de bloque. Los bloques se
// cortan en saltos de línea, pero un campo entrecomillado puede tener saltos
// dentro: entonces el worker del bloque k deja ese registro abierto
// (colaAbierta) y el del bloque k+1 empezó dentro del campo, así que su parcial
// no sirve. La cosedora, que recibe los bloques en orden, lo rehace en serie
// desde el inicio del registro abierto. Es determinista y casi nunca ocurre.
type cosedora struct {
	r       io.ReaderAt
	rangos  []rango
	w       *trabajador
	abierto int64 // offset de un registro abierto por el bloque anterior, o -1
}

func nuevaCosedora(r io.ReaderAt, rangos []rango, reg *reglas, cols indicesColumnas, delim byte) *cosedora {
	return &cosedora{r: r, rangos: rangos, w: nuevoTrabajador(reg, cols, delim), abierto: -1}
}

func (c *cosedora) coser(p *parcial) (*parcial, error) {
	if c.abierto >= 0 {
		p = c.w.procesar(c.r, p.indice, rango{c.abierto, c.rangos[p.indice].hasta})
	}
	if p.err != nil {
		return nil, p.err
	}
	c.abierto = p.colaAbierta
	return p, nil
}

func (c *cosedora) terminar() error {
	if c.abierto >= 0 {
		return fmt.Errorf("el registro que empieza en el byte %d: %w hasta el fin del archivo", c.abierto, errComillaAbierta)
	}
	return nil
}

// leerBloquesSecuencial hace lo mismo sin goroutines ni canales: procesa y
// fusiona cada bloque en orden. Es el oráculo de los tests de determinismo.
func leerBloquesSecuencial(r io.ReaderAt, rangos []rango, reg *reglas, cols indicesColumnas, delim byte) (*acumulador, error) {
	a := nuevoAcumulador()
	w := nuevoTrabajador(reg, cols, delim)
	c := nuevaCosedora(r, rangos, reg, cols, delim)
	for i, rg := range rangos {
		p, err := c.coser(w.procesar(r, i, rg))
		if err != nil {
			return nil, err
		}
		a.fusionar(p)
	}
	return a, c.terminar()
}
