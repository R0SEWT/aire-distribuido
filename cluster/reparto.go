package cluster

import (
	"fmt"
	"math"

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

// MaxReplicas dice cuántas réplicas de un gold de n filas entran en nodos nodos
// que cargan hasta tope filas cada uno. Repartir da rangos iguales (difieren a
// lo sumo en una fila), así que entran si réplicas × n <= tope × nodos. La
// tabla entera, además, no pasa de 2³¹ filas, y ahí se satura la cuenta. Lo
// usan los dos coordinadores (cmd/distribuido y la API) para rechazar un
// pedido antes de juntar a nadie: cada nodo rechaza un rango de más filas que
// su tope, y la corrida moriría en el CARGAR.
func MaxReplicas(n, nodos, tope int) int {
	if n <= 0 || nodos <= 0 || tope <= 0 {
		return 0
	}
	total := math.MaxInt32
	if tope < math.MaxInt32 && nodos <= total/tope {
		total = tope * nodos
	}
	return total / n
}
