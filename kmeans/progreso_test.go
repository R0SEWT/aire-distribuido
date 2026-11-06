package kmeans

import "testing"

// El callback AlIterar existe para que la CLI muestre una barra de progreso sin
// que el paquete sepa nada de terminales: recibe la iteración recién terminada y
// su inercia, una vez por iteración, en orden, en ambas versiones.

func puntosParaProgreso() (*Datos, []float64) {
	var x []float64
	for i := 0; i < 60; i++ {
		x = append(x, float64(i%3)*10+float64(i)*0.01, float64(i%2)*10)
	}
	d := &Datos{X: x, N: 60, D: 2}
	cent := []float64{0, 0, 10, 10, 20, 0}
	return d, cent
}

func TestSecuencialAvisaCadaIteracionEnOrden(t *testing.T) {
	d, cent := puntosParaProgreso()
	var its []int
	var inercias []float64
	op := Opciones{K: 3, MaxIter: 5, AlIterar: func(it int, inercia float64) {
		its = append(its, it)
		inercias = append(inercias, inercia)
	}}
	r, err := Secuencial(d, cent, op)
	if err != nil {
		t.Fatal(err)
	}
	if len(its) != r.Iteraciones {
		t.Fatalf("se avisó %d veces y hubo %d iteraciones", len(its), r.Iteraciones)
	}
	for i, it := range its {
		if it != i+1 {
			t.Fatalf("aviso %d trajo la iteración %d", i, it)
		}
		if inercias[i] != r.Inercias[i] {
			t.Fatalf("la inercia avisada en %d (%v) no es la registrada (%v)", it, inercias[i], r.Inercias[i])
		}
	}
}

func TestConcurrenteAvisaCadaIteracionEnOrden(t *testing.T) {
	d, cent := puntosParaProgreso()
	var its []int
	op := OpcionesConc{Opciones: Opciones{K: 3, MaxIter: 5, AlIterar: func(it int, _ float64) {
		its = append(its, it)
	}}, Workers: 4, Chunk: 7}
	r, err := Concurrente(d, cent, op)
	if err != nil {
		t.Fatal(err)
	}
	if len(its) != r.Iteraciones {
		t.Fatalf("se avisó %d veces y hubo %d iteraciones", len(its), r.Iteraciones)
	}
	for i, it := range its {
		if it != i+1 {
			t.Fatalf("aviso %d trajo la iteración %d", i, it)
		}
	}
}

func TestSinCallbackNoPasaNada(t *testing.T) {
	d, cent := puntosParaProgreso()
	if _, err := Secuencial(d, cent, Opciones{K: 3, MaxIter: 3}); err != nil {
		t.Fatal(err)
	}
}
