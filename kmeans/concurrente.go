package kmeans

import (
	"fmt"
	"sync"
)

// OpcionesConc agrega al contrato numérico los dos parámetros del paralelismo.
// Workers y Chunk son independientes a propósito: el reparto afecta el
// rendimiento tanto como la cantidad de goroutines, así que el benchmark los
// mueve por separado.
type OpcionesConc struct {
	Opciones
	Workers int
	Chunk   int
}

// parcial son los acumuladores de UN chunk: sumas y conteos por cluster más su
// inercia. Cada chunk tiene el suyo, y lo escribe un solo worker por iteración.
type parcial struct {
	sumas   []float64 // k*D
	conteos []int     // k
	inercia float64
}

// Concurrente corre Lloyd con un worker pool persistente.
//
// El reparto es por chunks a través de un canal: las goroutines se crean una
// vez y viven toda la corrida. Durante la asignación los centroides son de solo
// lectura y cada worker escribe únicamente el parcial del chunk que le tocó, así
// que el bucle caliente no necesita sync.Mutex. La única sincronización es la
// barrera (sync.WaitGroup) al final de cada iteración.
//
// La reducción suma los parciales EN ORDEN DE CHUNK, no en orden de worker. Por
// eso el resultado es idéntico bit a bit para cualquier cantidad de workers y no
// depende del planificador: sumar en punto flotante no es asociativo, y si cada
// worker acumulara lo que le tocó, dos corridas iguales darían números distintos.
func Concurrente(d *Datos, centroidesIniciales []float64, op OpcionesConc) (*Resultado, error) {
	if err := validar(d, centroidesIniciales, op.Opciones); err != nil {
		return nil, err
	}
	if op.Workers <= 0 {
		return nil, fmt.Errorf("workers debe ser positivo, es %d", op.Workers)
	}
	if op.Chunk <= 0 {
		return nil, fmt.Errorf("el tamaño de chunk debe ser positivo, es %d", op.Chunk)
	}
	k, dim, n := op.K, d.D, d.N

	cantChunks := (n + op.Chunk - 1) / op.Chunk
	parciales := make([]parcial, cantChunks)
	sumasBase := make([]float64, cantChunks*k*dim)
	conteosBase := make([]int, cantChunks*k)
	for c := range parciales {
		parciales[c].sumas = sumasBase[c*k*dim : (c+1)*k*dim]
		parciales[c].conteos = conteosBase[c*k : (c+1)*k]
	}

	cent := append([]float64(nil), centroidesIniciales...)
	previos := make([]float64, len(cent))
	asign := make([]int, n)
	sumas := make([]float64, k*dim)
	conteos := make([]int, k)

	trabajos := make(chan int, cantChunks)
	var barrera sync.WaitGroup
	var pool sync.WaitGroup

	pool.Add(op.Workers)
	for w := 0; w < op.Workers; w++ {
		go func() {
			defer pool.Done()
			for c := range trabajos {
				ini := c * op.Chunk
				fin := ini + op.Chunk
				if fin > n {
					fin = n
				}
				procesarChunk(d, cent, k, dim, ini, fin, &parciales[c], asign)
				barrera.Done()
			}
		}()
	}
	defer func() {
		close(trabajos)
		pool.Wait()
	}()

	r := &Resultado{K: k, D: dim, Paro: ParoMaxIter}

	for it := 1; it <= op.MaxIter; it++ {
		repartir(trabajos, &barrera, cantChunks)

		inercia := reducir(parciales, sumas, conteos, k, dim)
		copy(previos, cent)
		r.Vacios = actualizar(cent, sumas, conteos, k, dim)
		r.Inercias = append(r.Inercias, inercia)
		r.Iteraciones = it
		if op.AlIterar != nil {
			op.AlIterar(it, inercia)
		}

		if converge(cent, previos, op.Opciones) {
			r.Paro = ParoTolerancia
			break
		}
	}

	// Pasada final con los centroides finales, también en paralelo: el tiempo
	// que se mide tiene que incluir todo lo que la versión concurrente hace.
	repartir(trabajos, &barrera, cantChunks)
	r.Inercia = reducir(parciales, sumas, conteos, k, dim)

	r.Centroides = cent
	r.Asignaciones = asign
	return r, nil
}

// repartir manda los chunks al pool y espera la barrera.
func repartir(trabajos chan<- int, barrera *sync.WaitGroup, cantChunks int) {
	barrera.Add(cantChunks)
	for c := 0; c < cantChunks; c++ {
		trabajos <- c
	}
	barrera.Wait()
}

func procesarChunk(d *Datos, cent []float64, k, dim, ini, fin int, p *parcial, asign []int) {
	for i := range p.sumas {
		p.sumas[i] = 0
	}
	for j := range p.conteos {
		p.conteos[j] = 0
	}
	inercia := 0.0
	for i := ini; i < fin; i++ {
		x := d.X[i*dim : (i+1)*dim]
		j, dist2 := masCercano(x, cent, k, dim)
		asign[i] = j
		inercia += dist2
		acumular(p.sumas, p.conteos, x, j, dim)
	}
	p.inercia = inercia
}

// reducir suma los parciales en orden de chunk y devuelve la inercia total.
func reducir(parciales []parcial, sumas []float64, conteos []int, k, dim int) float64 {
	for i := range sumas {
		sumas[i] = 0
	}
	for j := range conteos {
		conteos[j] = 0
	}
	inercia := 0.0
	for c := range parciales {
		p := &parciales[c]
		for i := range sumas {
			sumas[i] += p.sumas[i]
		}
		for j := range conteos {
			conteos[j] += p.conteos[j]
		}
		inercia += p.inercia
	}
	return inercia
}
