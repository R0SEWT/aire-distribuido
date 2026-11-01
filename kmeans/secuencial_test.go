package kmeans

import (
	"math"
	"testing"
)

// dimPrueba es la dimensión de los puntos de juguete de los tests del TP. En el
// TP era la constante Dimension del lector del gold de taxis; aquí la dimensión
// la fija la receta (configs/kmeans.json), así que los tests la declaran aparte.
const dimPrueba = 6

// datosDe arma un Datos a partir de puntos de 6 dimensiones.
func datosDe(puntos ...[]float64) *Datos {
	d := &Datos{D: dimPrueba}
	for _, p := range puntos {
		if len(p) != dimPrueba {
			panic("el punto de prueba no tiene 6 features")
		}
		d.X = append(d.X, p...)
		d.N++
	}
	return d
}

// p6 completa un punto de 6 features a partir de las dos primeras coordenadas,
// para que los casos de prueba se lean sin ruido.
func p6(a, b float64) []float64 { return []float64{a, b, 0, 0, 0, 0} }

// tresGruposSeparados: tres nubes muy separadas, 3 puntos cada una.
func tresGruposSeparados() *Datos {
	return datosDe(
		p6(0, 0), p6(0, 1), p6(1, 0),
		p6(100, 100), p6(100, 101), p6(101, 100),
		p6(-100, -100), p6(-100, -101), p6(-101, -100),
	)
}

func opcionesDe(k int) Opciones {
	return Opciones{K: k, MaxIter: 100, TolAbs: 1e-9, TolRel: 1e-9}
}

func TestSecuencialRecuperaGruposObvios(t *testing.T) {
	d := tresGruposSeparados()
	// Un centroide sembrado dentro de cada nube.
	cent := []float64{}
	cent = append(cent, p6(0, 0)...)
	cent = append(cent, p6(100, 100)...)
	cent = append(cent, p6(-100, -100)...)

	r, err := Secuencial(d, cent, opcionesDe(3))
	if err != nil {
		t.Fatalf("Secuencial: %v", err)
	}
	// Los tres puntos de cada nube deben caer en el mismo cluster.
	for _, nube := range [][]int{{0, 1, 2}, {3, 4, 5}, {6, 7, 8}} {
		j := r.Asignaciones[nube[0]]
		for _, i := range nube[1:] {
			if r.Asignaciones[i] != j {
				t.Errorf("los puntos %d y %d deberían compartir cluster", nube[0], i)
			}
		}
	}
	// El centroide de la primera nube es la media de sus tres puntos.
	c := r.Centroide(r.Asignaciones[0])
	if math.Abs(c[0]-1.0/3) > 1e-12 || math.Abs(c[1]-1.0/3) > 1e-12 {
		t.Errorf("centroide de la primera nube = (%v, %v), se esperaba (1/3, 1/3)", c[0], c[1])
	}
}

func TestSecuencialLaInerciaNoCrece(t *testing.T) {
	d := tresGruposSeparados()
	// Inicialización mala a propósito: los tres centroides en la misma zona.
	cent := []float64{}
	cent = append(cent, p6(0, 0)...)
	cent = append(cent, p6(0, 2)...)
	cent = append(cent, p6(2, 0)...)

	r, err := Secuencial(d, cent, opcionesDe(3))
	if err != nil {
		t.Fatalf("Secuencial: %v", err)
	}
	if len(r.Inercias) < 2 {
		t.Fatalf("se esperaban varias iteraciones, hubo %d", len(r.Inercias))
	}
	for i := 1; i < len(r.Inercias); i++ {
		if r.Inercias[i] > r.Inercias[i-1]+1e-9 {
			t.Errorf("la inercia creció en la iteración %d: %v → %v",
				i+1, r.Inercias[i-1], r.Inercias[i])
		}
	}
}

func TestSecuencialEsDeterminista(t *testing.T) {
	d := tresGruposSeparados()
	cent := []float64{}
	cent = append(cent, p6(0, 0)...)
	cent = append(cent, p6(1, 1)...)

	a, err := Secuencial(d, cent, opcionesDe(2))
	if err != nil {
		t.Fatalf("Secuencial: %v", err)
	}
	b, err := Secuencial(d, cent, opcionesDe(2))
	if err != nil {
		t.Fatalf("Secuencial: %v", err)
	}
	if a.Inercia != b.Inercia {
		t.Errorf("inercias distintas entre corridas: %v y %v", a.Inercia, b.Inercia)
	}
	for i := range a.Centroides {
		if a.Centroides[i] != b.Centroides[i] {
			t.Fatalf("centroides distintos entre corridas en la posición %d", i)
		}
	}
}

func TestSecuencialNoModificaLosCentroidesIniciales(t *testing.T) {
	d := tresGruposSeparados()
	cent := append(append([]float64{}, p6(0, 0)...), p6(50, 50)...)
	copia := append([]float64{}, cent...)

	if _, err := Secuencial(d, cent, opcionesDe(2)); err != nil {
		t.Fatalf("Secuencial: %v", err)
	}
	for i := range copia {
		if cent[i] != copia[i] {
			t.Fatalf("Secuencial modificó los centroides iniciales en la posición %d", i)
		}
	}
}

func TestSecuencialConKUnoDaLaMedia(t *testing.T) {
	d := datosDe(p6(0, 0), p6(2, 0), p6(4, 0))
	r, err := Secuencial(d, p6(10, 10), opcionesDe(1))
	if err != nil {
		t.Fatalf("Secuencial: %v", err)
	}
	c := r.Centroide(0)
	if math.Abs(c[0]-2) > 1e-12 || math.Abs(c[1]) > 1e-12 {
		t.Errorf("centroide = (%v, %v), se esperaba (2, 0)", c[0], c[1])
	}
}

func TestSecuencialConKIgualANDaInerciaCero(t *testing.T) {
	d := datosDe(p6(0, 0), p6(5, 5), p6(9, 1))
	cent := []float64{}
	cent = append(cent, p6(0, 0)...)
	cent = append(cent, p6(5, 5)...)
	cent = append(cent, p6(9, 1)...)

	r, err := Secuencial(d, cent, opcionesDe(3))
	if err != nil {
		t.Fatalf("Secuencial: %v", err)
	}
	if r.Inercia > 1e-18 {
		t.Errorf("inercia = %v, se esperaba 0 con un centroide por punto", r.Inercia)
	}
}

func TestSecuencialClusterVacioConservaSuCentroide(t *testing.T) {
	// El tercer centroide está tan lejos que no recibe ningún punto.
	// k = n = 3: el contrato rechaza k > n, así que el cluster vacío se
	// provoca con la posición del centroide, no con la cantidad de puntos.
	d := datosDe(p6(0, 0), p6(0, 1), p6(0, 2))
	lejano := p6(1e6, 1e6)
	cent := []float64{}
	cent = append(cent, p6(0, 0)...)
	cent = append(cent, p6(0, 1)...)
	cent = append(cent, lejano...)

	r, err := Secuencial(d, cent, opcionesDe(3))
	if err != nil {
		t.Fatalf("Secuencial: %v", err)
	}
	c := r.Centroide(2)
	for i := range lejano {
		if c[i] != lejano[i] {
			t.Fatalf("el cluster vacío debía conservar su centroide, cambió en %d: %v", i, c[i])
		}
	}
	if r.Vacios != 1 {
		t.Errorf("Vacios = %d, se esperaba 1", r.Vacios)
	}
}

func TestSecuencialDesempataPorElIndiceMenor(t *testing.T) {
	// El primer punto está exactamente a la misma distancia de los dos centroides.
	d := datosDe(p6(0, 0), p6(10, 0))
	cent := []float64{}
	cent = append(cent, p6(-1, 0)...)
	cent = append(cent, p6(1, 0)...)

	r, err := Secuencial(d, cent, Opciones{K: 2, MaxIter: 1, TolAbs: 1e-9, TolRel: 1e-9})
	if err != nil {
		t.Fatalf("Secuencial: %v", err)
	}
	if r.Asignaciones[0] != 0 {
		t.Errorf("asignación = %d, el empate debe ganarlo el centroide de índice menor", r.Asignaciones[0])
	}
}

func TestSecuencialInformaPorQueParo(t *testing.T) {
	d := tresGruposSeparados()
	cent := []float64{}
	cent = append(cent, p6(0, 0)...)
	cent = append(cent, p6(100, 100)...)
	cent = append(cent, p6(-100, -100)...)

	convergido, err := Secuencial(d, cent, opcionesDe(3))
	if err != nil {
		t.Fatalf("Secuencial: %v", err)
	}
	if convergido.Paro != ParoTolerancia {
		t.Errorf("Paro = %q, se esperaba %q", convergido.Paro, ParoTolerancia)
	}

	cortado, err := Secuencial(d, cent, Opciones{K: 3, MaxIter: 1, TolAbs: 0, TolRel: 0})
	if err != nil {
		t.Fatalf("Secuencial: %v", err)
	}
	if cortado.Paro != ParoMaxIter {
		t.Errorf("Paro = %q, se esperaba %q", cortado.Paro, ParoMaxIter)
	}
	if cortado.Iteraciones != 1 {
		t.Errorf("Iteraciones = %d, se esperaba 1", cortado.Iteraciones)
	}
}

func TestSecuencialRechazaParametrosInvalidos(t *testing.T) {
	d := tresGruposSeparados()
	bueno := append(append([]float64{}, p6(0, 0)...), p6(1, 1)...)

	casos := []struct {
		nombre string
		datos  *Datos
		cent   []float64
		op     Opciones
	}{
		{"k cero", d, bueno, Opciones{K: 0, MaxIter: 10}},
		{"k negativo", d, bueno, Opciones{K: -1, MaxIter: 10}},
		{"k mayor que n", datosDe(p6(0, 0)), bueno, Opciones{K: 2, MaxIter: 10}},
		{"max_iter cero", d, bueno, Opciones{K: 2, MaxIter: 0}},
		{"centroides de otro tamaño", d, p6(0, 0), Opciones{K: 2, MaxIter: 10}},
		{"sin datos", datosDe(), bueno, Opciones{K: 2, MaxIter: 10}},
		{"centroide con NaN", d, append(append([]float64{}, p6(math.NaN(), 0)...), p6(1, 1)...), Opciones{K: 2, MaxIter: 10}},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if _, err := Secuencial(c.datos, c.cent, c.op); err == nil {
				t.Fatal("se esperaba error")
			}
		})
	}
}

func TestSecuencialLaInerciaFinalUsaLosCentroidesFinales(t *testing.T) {
	// Con una sola iteración, la inercia de la iteración se mide contra los
	// centroides iniciales y la final contra los ya actualizados: la final
	// tiene que ser menor o igual.
	d := datosDe(p6(0, 0), p6(0, 2))
	cent := p6(0, 10)

	r, err := Secuencial(d, cent, Opciones{K: 1, MaxIter: 1, TolAbs: 0, TolRel: 0})
	if err != nil {
		t.Fatalf("Secuencial: %v", err)
	}
	if r.Inercia > r.Inercias[0] {
		t.Errorf("inercia final %v mayor que la de la iteración %v", r.Inercia, r.Inercias[0])
	}
	if math.Abs(r.Inercia-2) > 1e-12 {
		t.Errorf("inercia final = %v, se esperaba 2 (dos puntos a distancia 1 del centro)", r.Inercia)
	}
}
