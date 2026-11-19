package kmeans

import (
	"math/rand"
	"testing"
)

// MasCercano tiene que dar, punto por punto, lo mismo que Asignar: es el
// contrato con el que se entrenó el modelo y con el que la API predice.
func TestMasCercanoCoincideConAsignar(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	const n, k, dim = 2000, 5, 3
	d := &Datos{X: make([]float64, n*dim), N: n, D: dim}
	for i := range d.X {
		d.X[i] = rng.NormFloat64()
	}
	cent := make([]float64, k*dim)
	for i := range cent {
		cent[i] = rng.NormFloat64()
	}
	asign := make([]int, n)
	Asignar(d, cent, 0, n, asign)
	for i := 0; i < n; i++ {
		j, dist := MasCercano(d.Punto(i), cent)
		if j != asign[i] {
			t.Fatalf("fila %d: MasCercano da %d y Asignar %d", i, j, asign[i])
		}
		if want := distancia2(d.Punto(i), cent[j*dim:(j+1)*dim]); dist != want {
			t.Fatalf("fila %d: distancia %v, se esperaba %v", i, dist, want)
		}
	}
}

func TestMasCercanoDesempataPorElMenorIndice(t *testing.T) {
	// El punto (0, 0) está a la misma distancia de los centroides 1, 2 y 3.
	cent := []float64{
		5, 5,
		1, 0,
		0, 1,
		-1, 0,
	}
	if j, dist := MasCercano([]float64{0, 0}, cent); j != 1 || dist != 1 {
		t.Fatalf("MasCercano = (%d, %v), se esperaba (1, 1)", j, dist)
	}
	// Centroides repetidos: gana el primero.
	if j, _ := MasCercano([]float64{2, 2}, []float64{2, 2, 2, 2}); j != 0 {
		t.Fatalf("con centroides idénticos ganó el %d", j)
	}
}

func TestMasCercanoEntraEnPanicoConFormasInvalidas(t *testing.T) {
	casos := map[string]struct{ x, cent []float64 }{
		"punto vacío":         {nil, []float64{1, 2}},
		"sin centroides":      {[]float64{1, 2}, nil},
		"no es múltiplo de D": {[]float64{1, 2}, []float64{1, 2, 3}},
	}
	for nombre, c := range casos {
		t.Run(nombre, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("no entró en pánico")
				}
			}()
			MasCercano(c.x, c.cent)
		})
	}
}

func distancia2(a, b []float64) float64 {
	s := 0.0
	for f := range a {
		delta := a[f] - b[f]
		s += float64(delta * delta)
	}
	return s
}
