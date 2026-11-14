package kmeans

import (
	"fmt"
	"sync"
)

// ParcialesPorChunks calcula el parcial de todas las filas de d como lo hace
// una iteración de Concurrente: parte d en chunks de chunk filas, un pool de
// workers goroutines calcula Parciales de cada chunk y el resultado es Fusionar
// de esos parciales EN ORDEN DE CHUNK. Por eso es idéntico bit a bit para
// cualquier cantidad de workers y, con los mismos centroides y el mismo chunk,
// es exactamente el parcial fusionado de una iteración de Concurrente sobre d.
//
// Es lo que un nodo del clúster calcula sobre su partición en cada ronda
// (docs/diseno.md, §4.4, paso 3): su propio pool, su barrera local y su fusión
// local en orden de chunk. Con un solo nodo, la partición es todo el gold y el
// K-Means distribuido da el mismo resultado bit a bit que cmd/entrenar.
//
// El pool vive lo que dura la llamada, no toda la corrida como en
// Concurrente: crear W goroutines por ronda cuesta microsegundos, y así el
// nodo no tiene un pool que cerrar si la conexión se corta a mitad de corrida.
// Los centroides son de solo lectura y cada worker escribe solo el parcial del
// chunk que le tocó: no hace falta Mutex.
//
// Centroides que no son múltiplo de D, workers o chunk no positivos son un
// error de programación y entran en pánico: el nodo los valida antes.
func ParcialesPorChunks(d *Datos, cent []float64, workers, chunk int) Parcial {
	if d == nil || d.D <= 0 || len(cent) == 0 || len(cent)%d.D != 0 {
		panic(fmt.Sprintf("kmeans.ParcialesPorChunks: %d centroides no son k*D", len(cent)))
	}
	if workers <= 0 || chunk <= 0 {
		panic(fmt.Sprintf("kmeans.ParcialesPorChunks: workers=%d y chunk=%d tienen que ser positivos", workers, chunk))
	}
	cantChunks := (d.N + chunk - 1) / chunk
	parciales := make([]Parcial, cantChunks)

	trabajos := make(chan int, cantChunks)
	for c := range cantChunks {
		trabajos <- c
	}
	close(trabajos)

	var barrera sync.WaitGroup
	for range min(workers, cantChunks) {
		barrera.Go(func() {
			for c := range trabajos {
				ini := c * chunk
				parciales[c] = Parciales(d, cent, ini, min(ini+chunk, d.N))
			}
		})
	}
	barrera.Wait()
	return Fusionar(parciales, len(cent)/d.D, d.D)
}
