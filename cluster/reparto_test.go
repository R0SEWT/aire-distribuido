package cluster

import (
	"testing"

	"github.com/R0SEWT/aire-distribuido/protocolo"
)

func TestRepartirDaRangosContiguosIguales(t *testing.T) {
	got, err := Repartir(10, 3)
	if err != nil {
		t.Fatal(err)
	}
	want := []protocolo.Rango{{Desde: 0, Hasta: 4}, {Desde: 4, Hasta: 7}, {Desde: 7, Hasta: 10}}
	if len(got) != len(want) {
		t.Fatalf("rangos = %v, se esperaban %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("rangos = %v, se esperaban %v", got, want)
		}
	}
}

func TestRepartirCubreTodoSinHuecosYConTamanosQueDifierenEnUnaFila(t *testing.T) {
	for _, c := range []struct{ n, k int }{{65873, 1}, {65873, 2}, {65873, 3}, {65873, 8}, {65873, 20}, {7, 7}, {100, 9}} {
		rs, err := Repartir(c.n, c.k)
		if err != nil {
			t.Fatalf("Repartir(%d, %d): %v", c.n, c.k, err)
		}
		if len(rs) != c.k {
			t.Fatalf("Repartir(%d, %d): %d rangos", c.n, c.k, len(rs))
		}
		menor, mayor := c.n, 0
		for i, r := range rs {
			if i == 0 && r.Desde != 0 || i > 0 && r.Desde != rs[i-1].Hasta {
				t.Fatalf("Repartir(%d, %d): hueco o solape antes del rango %d: %v", c.n, c.k, i, rs)
			}
			menor, mayor = min(menor, r.Len()), max(mayor, r.Len())
		}
		if rs[c.k-1].Hasta != c.n {
			t.Fatalf("Repartir(%d, %d): el último rango termina en %d", c.n, c.k, rs[c.k-1].Hasta)
		}
		if menor == 0 || mayor-menor > 1 {
			t.Fatalf("Repartir(%d, %d): tamaños entre %d y %d", c.n, c.k, menor, mayor)
		}
	}
}

func TestRepartirRechazaMasNodosQueFilas(t *testing.T) {
	for _, c := range []struct{ n, k int }{{3, 4}, {0, 1}, {10, 0}} {
		if _, err := Repartir(c.n, c.k); err == nil {
			t.Errorf("Repartir(%d, %d) tenía que fallar", c.n, c.k)
		}
	}
}

func TestMaxReplicas(t *testing.T) {
	for _, c := range []struct{ n, nodos, tope, want int }{
		{65873, 1, 25_000_000, 379},                  // el gold real en un nodo: hasta ×379
		{65873, 4, 25_000_000, 1518},                 // con 4 nodos, 4 veces más
		{1200, 2, 25_000_000, 41666},                 //
		{1000, 3, 500, 1},                            // justo una copia
		{1000, 1, 500, 0},                            // ni una copia entra
		{1000, 1000, 25_000_000, 2_147_483},          // la tabla no pasa de 2³¹ filas
		{0, 1, 10, 0}, {10, 0, 10, 0}, {10, 1, 0, 0}, // argumentos sin sentido
	} {
		if got := MaxReplicas(c.n, c.nodos, c.tope); got != c.want {
			t.Errorf("MaxReplicas(%d, %d, %d) = %d, se esperaba %d", c.n, c.nodos, c.tope, got, c.want)
		}
	}
}
