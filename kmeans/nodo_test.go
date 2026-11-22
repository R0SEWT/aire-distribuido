package kmeans

import (
	"math"
	"testing"
)

// Lo que un nodo de la PC4 calcula en cada ronda tiene que ser exactamente lo
// que una iteración de Concurrente fusiona: con un solo nodo, el K-Means
// distribuido es entonces el de cmd/entrenar bit a bit.

func TestParcialesPorChunksEsLaFusionEnOrdenDeChunk(t *testing.T) {
	d := nubes(150)
	cent := centroidesDe(d, 4, 5, t)
	const chunk = 37

	var ps []Parcial
	for ini := 0; ini < d.N; ini += chunk {
		ps = append(ps, Parciales(d, cent, ini, min(ini+chunk, d.N)))
	}
	want := Fusionar(ps, 4, d.D)

	for _, w := range []int{1, 2, 3, 8, 64} {
		got := ParcialesPorChunks(d, cent, w, chunk)
		mismoParcial(t, got, want, "workers=%d", w)
	}
}

func TestParcialesPorChunksEsLaPrimeraIteracionDeConcurrente(t *testing.T) {
	d := nubes(150)
	cent := centroidesDe(d, 4, 9, t)
	op := opcionesConcDe(4, 3, 50)
	op.MaxIter = 1

	r, err := Concurrente(d, cent, op)
	if err != nil {
		t.Fatalf("Concurrente: %v", err)
	}
	p := ParcialesPorChunks(d, cent, 5, 50)
	if p.Inercia != r.Inercias[0] {
		t.Fatalf("inercia de la iteración 1: %.17g por chunks, %.17g en Concurrente", p.Inercia, r.Inercias[0])
	}
	// Una iteración más de Concurrente es CerrarIteracion sobre ese parcial.
	propio := append([]float64(nil), cent...)
	var res Resultado
	CerrarIteracion(&res, propio, p, 1, op.Opciones)
	for i := range propio {
		if propio[i] != r.Centroides[i] {
			t.Fatalf("centroide %d tras una iteración: %.17g, Concurrente da %.17g", i, propio[i], r.Centroides[i])
		}
	}
}

func TestParcialesPorChunksConUnChunkEsParciales(t *testing.T) {
	d := nubes(20)
	cent := centroidesDe(d, 4, 1, t)
	mismoParcial(t, ParcialesPorChunks(d, cent, 4, d.N), Parciales(d, cent, 0, d.N), "un chunk")
	mismoParcial(t, ParcialesPorChunks(d, cent, 4, 10*d.N), Parciales(d, cent, 0, d.N), "chunk mayor que N")
}

func TestParcialesPorChunksConservaLasFilas(t *testing.T) {
	d := nubes(33)
	cent := centroidesDe(d, 4, 2, t)
	p := ParcialesPorChunks(d, cent, 3, 7)
	total := 0
	for _, c := range p.Conteos {
		total += c
	}
	if total != d.N {
		t.Fatalf("los conteos suman %d y hay %d filas", total, d.N)
	}
}

func TestCerrarIteracionAplicaElCriterioDeParadaDelTP(t *testing.T) {
	// Dos clusters de un punto cada uno: tras la primera iteración los
	// centroides caen sobre los puntos y la segunda no los mueve.
	d := datosDe(p6(0, 0), p6(4, 4))
	cent := append(append([]float64{}, p6(1, 1)...), p6(3, 3)...)
	op := opcionesDe(2)
	var r Resultado
	if CerrarIteracion(&r, cent, Parciales(d, cent, 0, d.N), 1, op) {
		t.Fatal("la primera iteración mueve los centroides una unidad: no puede converger")
	}
	if !CerrarIteracion(&r, cent, Parciales(d, cent, 0, d.N), 2, op) {
		t.Fatal("la segunda iteración no mueve nada: tenía que converger")
	}
	if r.Iteraciones != 2 || r.Paro != ParoTolerancia || len(r.Inercias) != 2 || r.Inercias[1] != 0 {
		t.Fatalf("resultado inesperado: %+v", r)
	}
}

func mismoParcial(t *testing.T, got, want Parcial, formato string, args ...any) {
	t.Helper()
	if math.Float64bits(got.Inercia) != math.Float64bits(want.Inercia) {
		t.Errorf(formato+": inercia %.17g, se esperaba %.17g", append(args, got.Inercia, want.Inercia)...)
	}
	for i := range want.Sumas {
		if math.Float64bits(got.Sumas[i]) != math.Float64bits(want.Sumas[i]) {
			t.Fatalf(formato+": suma %d = %.17g, se esperaba %.17g", append(args, i, got.Sumas[i], want.Sumas[i])...)
		}
	}
	for j := range want.Conteos {
		if got.Conteos[j] != want.Conteos[j] {
			t.Fatalf(formato+": conteo %d = %d, se esperaba %d", append(args, j, got.Conteos[j], want.Conteos[j])...)
		}
	}
}
