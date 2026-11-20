package cluster

import (
	"fmt"

	"github.com/R0SEWT/aire-distribuido/protocolo"
)

// Repartir parte n filas en k rangos contiguos [desde, hasta) que cubren
// [0, n) en orden, sin huecos ni solapes: el rango i es del nodo i, y ese es
// también el orden de la fusión. Los tamaños difieren a lo sumo en una fila
// (los primeros n mod k rangos llevan una más).
//
// Rangos iguales es la línea base de docs/diseno.md, §4.3: el costo por fila es
// el mismo en cualquier fila, así que la carga depende solo de cuántas filas
// tiene cada nodo. Repartir en proporción a los workers declarados no se hace:
// un núcleo de un equipo no vale lo mismo que el de otro, y la extensión de la
// spec es repartir según el rendimiento medido, no el declarado.
func Repartir(n, k int) ([]protocolo.Rango, error) {
	if n <= 0 || k <= 0 {
		return nil, fmt.Errorf("no se pueden repartir %d filas entre %d nodos", n, k)
	}
	if k > n {
		return nil, fmt.Errorf("hay %d nodos y solo %d filas: algún nodo se quedaría sin rango", k, n)
	}
	base, resto := n/k, n%k
	rangos := make([]protocolo.Rango, k)
	desde := 0
	for i := range rangos {
		tam := base
		if i < resto {
			tam++
		}
		rangos[i] = protocolo.Rango{Desde: desde, Hasta: desde + tam}
		desde += tam
	}
	return rangos, nil
}
