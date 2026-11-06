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

// trabajo es un chunk que el pool tiene que procesar. En las iteraciones basta
// el parcial; en la pasada final además se escriben las asignaciones.
type trabajo struct {
	chunk   int
	asignar bool
}

// Concurrente corre Lloyd con un worker pool persistente, escrito sobre las
// tres piezas que la PC4 distribuye: cada worker llama a Parciales sobre el
// chunk que le tocó, el coordinador fusiona los parciales con Fusionar en orden
// de chunk y cierra la iteración con Actualizar. La versión distribuida corre el
// mismo código; solo cambia quién llama a Parciales.
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
	parciales := make([]Parcial, cantChunks)
	cent := append([]float64(nil), centroidesIniciales...)
	asign := make([]int, n)

	trabajos := make(chan trabajo, cantChunks)
	var barrera sync.WaitGroup
	var pool sync.WaitGroup

	pool.Add(op.Workers)
	for w := 0; w < op.Workers; w++ {
		go func() {
			defer pool.Done()
			for t := range trabajos {
				ini := t.chunk * op.Chunk
				fin := min(ini+op.Chunk, n)
				if t.asignar {
					parciales[t.chunk] = Asignar(d, cent, ini, fin, asign)
				} else {
					parciales[t.chunk] = Parciales(d, cent, ini, fin)
				}
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
		repartir(trabajos, &barrera, cantChunks, false)
		if cerrarIteracion(r, cent, Fusionar(parciales, k, dim), it, op.Opciones) {
			break
		}
	}

	// Pasada final con los centroides finales, también en paralelo: el tiempo
	// que se mide tiene que incluir todo lo que la versión concurrente hace.
	repartir(trabajos, &barrera, cantChunks, true)
	r.Inercia = Fusionar(parciales, k, dim).Inercia

	r.Centroides = cent
	r.Asignaciones = asign
	return r, nil
}

// repartir manda los chunks al pool y espera la barrera.
func repartir(trabajos chan<- trabajo, barrera *sync.WaitGroup, cantChunks int, asignar bool) {
	barrera.Add(cantChunks)
	for c := 0; c < cantChunks; c++ {
		trabajos <- trabajo{chunk: c, asignar: asignar}
	}
	barrera.Wait()
}
