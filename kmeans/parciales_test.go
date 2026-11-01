package kmeans

import (
	"fmt"
	"math"
	"math/rand"
	"testing"
)

// --- Oráculo: el Lloyd del TP tal cual, antes de reescribirlo sobre Parciales,
// Fusionar y Actualizar. Los tests de abajo exigen que la versión nueva dé lo
// mismo BIT A BIT: la reescritura es mecánica y no puede mover ni un dígito.
// El oráculo no llama a nada del paquete: masCercano, acumular, actualizar y
// converge son copias del TP (upc.edu.pe/concurrente/tp/kmeans, secuencial.go).

// masCercanoDelTP es el masCercano del TP. Único cambio: float64(delta*delta),
// para que el oráculo haga la aritmética del TP en x86-64 también en arm64 o con
// GOAMD64=v3, donde el compilador fusionaría el producto con la suma (FMA). En
// amd64 v1, que es donde corrió el TP y el CI, las dos líneas compilan igual.
func masCercanoDelTP(x, cent []float64, k, dim int) (int, float64) {
	mejor, mejorDist := 0, math.Inf(1)
	for j := 0; j < k; j++ {
		c := cent[j*dim : (j+1)*dim]
		dist := 0.0
		for f := 0; f < dim; f++ {
			delta := x[f] - c[f]
			dist += float64(delta * delta)
		}
		if dist < mejorDist {
			mejor, mejorDist = j, dist
		}
	}
	return mejor, mejorDist
}

func acumularDelTP(sumas []float64, conteos []int, x []float64, j, dim int) {
	s := sumas[j*dim : (j+1)*dim]
	for f := 0; f < dim; f++ {
		s[f] += x[f]
	}
	conteos[j]++
}

func actualizarDelTP(cent, sumas []float64, conteos []int, k, dim int) int {
	vacios := 0
	for j := 0; j < k; j++ {
		if conteos[j] == 0 {
			vacios++
			continue
		}
		c := cent[j*dim : (j+1)*dim]
		s := sumas[j*dim : (j+1)*dim]
		inv := 1 / float64(conteos[j])
		for f := 0; f < dim; f++ {
			c[f] = s[f] * inv
		}
	}
	return vacios
}

func convergeDelTP(cent, previos []float64, op Opciones) bool {
	desp, escala := 0.0, 0.0
	for i := range cent {
		if delta := math.Abs(cent[i] - previos[i]); delta > desp {
			desp = delta
		}
		if mag := math.Abs(previos[i]); mag > escala {
			escala = mag
		}
	}
	return desp <= op.TolAbs+float64(op.TolRel*escala) // float64: ver masCercanoDelTP
}

// lloydDelTP es el Lloyd del TP con la reducción por chunks hecha en serie:
// cada chunk acumula su parcial y los parciales se suman en orden de chunk. Con
// chunk >= N es exactamente el Secuencial del TP; con chunk menor es lo que el
// Concurrente del TP calculaba, sin goroutines.
func lloydDelTP(d *Datos, inicial []float64, op Opciones, chunk int) *Resultado {
	k, dim, n := op.K, d.D, d.N
	cent := append([]float64(nil), inicial...)
	previos := make([]float64, len(cent))
	asign := make([]int, n)
	r := &Resultado{K: k, D: dim, Paro: ParoMaxIter}

	pasada := func() ([]float64, []int, float64) {
		sumas := make([]float64, k*dim)
		conteos := make([]int, k)
		inercia := 0.0
		for ini := 0; ini < n; ini += chunk {
			fin := min(ini+chunk, n)
			ps := make([]float64, k*dim)
			pc := make([]int, k)
			pi := 0.0
			for i := ini; i < fin; i++ {
				x := d.X[i*dim : (i+1)*dim]
				j, dist2 := masCercanoDelTP(x, cent, k, dim)
				asign[i] = j
				pi += dist2
				acumularDelTP(ps, pc, x, j, dim)
			}
			for i := range sumas {
				sumas[i] += ps[i]
			}
			for j := range conteos {
				conteos[j] += pc[j]
			}
			inercia += pi
		}
		return sumas, conteos, inercia
	}

	for it := 1; it <= op.MaxIter; it++ {
		sumas, conteos, inercia := pasada()
		copy(previos, cent)
		r.Vacios = actualizarDelTP(cent, sumas, conteos, k, dim)
		r.Inercias = append(r.Inercias, inercia)
		r.Iteraciones = it
		if convergeDelTP(cent, previos, op) {
			r.Paro = ParoTolerancia
			break
		}
	}
	_, _, r.Inercia = pasada()
	r.Centroides, r.Asignaciones = cent, asign
	return r
}

// mismoResultado exige igualdad bit a bit entre dos corridas.
func mismoResultado(t *testing.T, que string, got, want *Resultado) {
	t.Helper()
	if got.Iteraciones != want.Iteraciones || got.Paro != want.Paro || got.Vacios != want.Vacios {
		t.Fatalf("%s: iteraciones/paro/vacíos = %d/%s/%d, se esperaba %d/%s/%d", que,
			got.Iteraciones, got.Paro, got.Vacios, want.Iteraciones, want.Paro, want.Vacios)
	}
	if math.Float64bits(got.Inercia) != math.Float64bits(want.Inercia) {
		t.Errorf("%s: inercia final %.17g, se esperaba %.17g", que, got.Inercia, want.Inercia)
	}
	for i := range want.Inercias {
		if math.Float64bits(got.Inercias[i]) != math.Float64bits(want.Inercias[i]) {
			t.Errorf("%s: inercia de la iteración %d: %.17g vs %.17g", que, i+1, got.Inercias[i], want.Inercias[i])
		}
	}
	for i := range want.Centroides {
		if math.Float64bits(got.Centroides[i]) != math.Float64bits(want.Centroides[i]) {
			t.Fatalf("%s: centroide distinto en la posición %d: %.17g vs %.17g", que, i, got.Centroides[i], want.Centroides[i])
		}
	}
	for i := range want.Asignaciones {
		if got.Asignaciones[i] != want.Asignaciones[i] {
			t.Fatalf("%s: asignación distinta en la fila %d", que, i)
		}
	}
}

func TestSecuencialEsBitABitElDelTP(t *testing.T) {
	for _, semilla := range []int64{3, 7, 11, 2024} {
		d := nubes(150)
		cent := centroidesDe(d, 4, semilla, t)
		op := opcionesDe(4)
		got, err := Secuencial(d, cent, op)
		if err != nil {
			t.Fatalf("Secuencial: %v", err)
		}
		mismoResultado(t, fmt.Sprintf("semilla %d", semilla), got, lloydDelTP(d, cent, op, d.N))
	}
}

func TestConcurrenteEsBitABitElDelTP(t *testing.T) {
	// Mismo chunk, cualquier cantidad de workers: el resultado es el de la
	// reducción en serie por chunks del TP, bit a bit.
	d := nubes(300)
	cent := centroidesDe(d, 4, 5, t)
	for _, chunk := range []int{1, 37, 64, 1000, d.N} {
		want := lloydDelTP(d, cent, opcionesDe(4), chunk)
		for _, w := range []int{1, 2, 8} {
			got, err := Concurrente(d, cent, opcionesConcDe(4, w, chunk))
			if err != nil {
				t.Fatalf("Concurrente: %v", err)
			}
			mismoResultado(t, fmt.Sprintf("chunk %d, W=%d", chunk, w), got, want)
		}
	}
}

// datosColaPesada: n puntos con colas pesadas (cociente de normales, como una
// Cauchy) y valores muy dispares, y k centroides iniciales de los que algunos
// quedan lejos de todo: hay clusters vacíos, muchas iteraciones y redondeo que
// sí depende del orden de la suma.
func datosColaPesada(rng *rand.Rand, n, dim, k int) (*Datos, []float64) {
	d := &Datos{D: dim, N: n}
	for i := 0; i < n*dim; i++ {
		v := rng.NormFloat64() / (math.Abs(rng.NormFloat64()) + 1e-3)
		if rng.Intn(10) == 0 {
			v *= 1e6
		}
		d.X = append(d.X, v)
	}
	cent := make([]float64, 0, k*dim)
	for j := 0; j < k; j++ {
		for f := 0; f < dim; f++ {
			if j%3 == 2 {
				cent = append(cent, 1e12*float64(j)) // lejos de todo: queda vacío
			} else {
				cent = append(cent, d.X[rng.Intn(n)*dim+f])
			}
		}
	}
	return d, cent
}

func TestLloydConColasPesadasVaciosYToleranciaCeroEsElDelTP(t *testing.T) {
	rng := rand.New(rand.NewSource(20261101))
	vacios, iteraciones := 0, 0
	for caso := 0; caso < 25; caso++ {
		n, dim, k := 50+rng.Intn(400), 1+rng.Intn(5), 2+rng.Intn(9)
		d, cent := datosColaPesada(rng, n, dim, k)
		op := Opciones{K: k, MaxIter: 5 + rng.Intn(40)} // tolerancia 0: solo para en un punto fijo exacto

		quiero := lloydDelTP(d, cent, op, d.N)
		sec, err := Secuencial(d, cent, op)
		if err != nil {
			t.Fatal(err)
		}
		mismoResultado(t, fmt.Sprintf("caso %d secuencial", caso), sec, quiero)

		chunk := 1 + rng.Intn(n)
		quiero = lloydDelTP(d, cent, op, chunk)
		for _, w := range []int{1, 3, 8} {
			con, err := Concurrente(d, cent, OpcionesConc{Opciones: op, Workers: w, Chunk: chunk})
			if err != nil {
				t.Fatal(err)
			}
			mismoResultado(t, fmt.Sprintf("caso %d, chunk %d, W=%d", caso, chunk, w), con, quiero)
		}
		vacios += quiero.Vacios
		iteraciones += quiero.Iteraciones
	}
	// Que el caso pruebe lo que dice: hubo vacíos y bastante más de dos iteraciones.
	if vacios == 0 || iteraciones < 25*5 {
		t.Fatalf("los casos no ejercitan lo que prometen: %d vacíos, %d iteraciones en total", vacios, iteraciones)
	}
	t.Logf("25 casos: %d clusters vacíos al final, %d iteraciones en total", vacios, iteraciones)
}

func TestParcialesSobreTodoElRangoEsUnaIteracionDelSecuencial(t *testing.T) {
	d := nubes(200)
	cent := centroidesDe(d, 4, 17, t)

	p := Parciales(d, cent, 0, d.N)
	nuevos := append([]float64(nil), cent...)
	Actualizar(nuevos, p)

	una, err := Secuencial(d, cent, Opciones{K: 4, MaxIter: 1})
	if err != nil {
		t.Fatalf("Secuencial: %v", err)
	}
	if math.Float64bits(p.Inercia) != math.Float64bits(una.Inercias[0]) {
		t.Errorf("inercia del parcial %.17g, la de la iteración 1 es %.17g", p.Inercia, una.Inercias[0])
	}
	for i := range nuevos {
		if math.Float64bits(nuevos[i]) != math.Float64bits(una.Centroides[i]) {
			t.Fatalf("centroide %d: %.17g vs %.17g", i, nuevos[i], una.Centroides[i])
		}
	}
	total := 0
	for _, c := range p.Conteos {
		total += c
	}
	if total != d.N {
		t.Errorf("los conteos suman %d, se esperaban las %d filas", total, d.N)
	}
	if len(p.Sumas) != 4*d.D || len(p.Conteos) != 4 {
		t.Errorf("tamaños del parcial: %d sumas y %d conteos, se esperaban %d y 4", len(p.Sumas), len(p.Conteos), 4*d.D)
	}
}

func TestParcialesDeUnTramoVacioEsCero(t *testing.T) {
	d := nubes(10)
	cent := centroidesDe(d, 3, 1, t)
	p := Parciales(d, cent, 5, 5)
	if p.Inercia != 0 || len(p.Sumas) != 3*d.D || len(p.Conteos) != 3 {
		t.Fatalf("parcial de un tramo vacío: %+v", p)
	}
	for _, s := range p.Sumas {
		if s != 0 {
			t.Fatal("un tramo vacío no debería sumar nada")
		}
	}
	for _, c := range p.Conteos {
		if c != 0 {
			t.Fatal("un tramo vacío no debería contar nada")
		}
	}
}

func TestParcialesRechazaTramosFueraDeRango(t *testing.T) {
	d := nubes(10)
	cent := centroidesDe(d, 2, 1, t)
	for _, c := range [][2]int{{-1, 3}, {3, 2}, {0, d.N + 1}} {
		t.Run(fmt.Sprintf("[%d,%d)", c[0], c[1]), func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("se esperaba un panic con un tramo inválido")
				}
			}()
			Parciales(d, cent, c[0], c[1])
		})
	}
}

func TestAsignarEscribeSoloSuTramoYDevuelveElMismoParcial(t *testing.T) {
	d := nubes(50)
	cent := centroidesDe(d, 4, 9, t)
	asign := make([]int, d.N)
	for i := range asign {
		asign[i] = -1
	}
	p := Asignar(d, cent, 20, 70, asign)
	q := Parciales(d, cent, 20, 70)
	if math.Float64bits(p.Inercia) != math.Float64bits(q.Inercia) {
		t.Errorf("Asignar y Parciales dan inercias distintas: %v vs %v", p.Inercia, q.Inercia)
	}
	for i, a := range asign {
		dentro := i >= 20 && i < 70
		if dentro && (a < 0 || a >= 4) {
			t.Fatalf("fila %d del tramo sin asignar: %d", i, a)
		}
		if !dentro && a != -1 {
			t.Fatalf("Asignar escribió la fila %d, fuera de su tramo", i)
		}
	}
}

func TestFusionarEnOrdenEsLaSumaEnSerieDeLosParciales(t *testing.T) {
	// Tres tramos contiguos A, B, C. Fusionar en ese orden tiene que ser,
	// bit a bit, la suma escrita a mano (0 + A) + B + C.
	d := nubes(120)
	cent := centroidesDe(d, 4, 21, t)
	a := Parciales(d, cent, 0, 100)
	b := Parciales(d, cent, 100, 333)
	c := Parciales(d, cent, 333, d.N)

	got := Fusionar([]Parcial{a, b, c}, 4, d.D)

	for i := range got.Sumas {
		want := 0.0
		want += a.Sumas[i]
		want += b.Sumas[i]
		want += c.Sumas[i]
		if math.Float64bits(got.Sumas[i]) != math.Float64bits(want) {
			t.Fatalf("suma %d: %.17g, se esperaba %.17g", i, got.Sumas[i], want)
		}
	}
	for j := range got.Conteos {
		if got.Conteos[j] != a.Conteos[j]+b.Conteos[j]+c.Conteos[j] {
			t.Fatalf("conteo %d: %d", j, got.Conteos[j])
		}
	}
	wantInercia := 0.0
	wantInercia += a.Inercia
	wantInercia += b.Inercia
	wantInercia += c.Inercia
	if math.Float64bits(got.Inercia) != math.Float64bits(wantInercia) {
		t.Errorf("inercia %.17g, se esperaba %.17g", got.Inercia, wantInercia)
	}

	// Contra una sola pasada sobre la concatenación el árbol de suma es otro:
	// la igualdad es con tolerancia y los conteos, exactos.
	todo := Parciales(d, cent, 0, d.N)
	for i := range todo.Sumas {
		if math.Abs(got.Sumas[i]-todo.Sumas[i]) > 1e-9*math.Max(1, math.Abs(todo.Sumas[i])) {
			t.Errorf("suma %d: fusionada %v, una pasada %v", i, got.Sumas[i], todo.Sumas[i])
		}
	}
	for j := range todo.Conteos {
		if got.Conteos[j] != todo.Conteos[j] {
			t.Errorf("conteo %d: fusionado %d, una pasada %d", j, got.Conteos[j], todo.Conteos[j])
		}
	}
}

func TestFusionarConDatosExactosCoincideBitABitConUnaSolaPasada(t *testing.T) {
	// Con coordenadas enteras todas las sumas parciales son exactas en float64,
	// así que fusionar A, B, C da bit a bit lo mismo que el secuencial sobre la
	// concatenación A‖B‖C: ahí no hay redondeo que dependa del orden.
	d := &Datos{D: 3}
	for i := 0; i < 90; i++ {
		d.X = append(d.X, float64(i%9), float64((i*7)%11), float64(i/30*40))
		d.N++
	}
	cent := []float64{0, 0, 0, 5, 5, 40, 8, 10, 80}

	todo := Parciales(d, cent, 0, d.N)
	got := Fusionar([]Parcial{
		Parciales(d, cent, 0, 31),
		Parciales(d, cent, 31, 64),
		Parciales(d, cent, 64, d.N),
	}, 3, d.D)
	for i := range todo.Sumas {
		if math.Float64bits(got.Sumas[i]) != math.Float64bits(todo.Sumas[i]) {
			t.Fatalf("suma %d: %.17g vs %.17g", i, got.Sumas[i], todo.Sumas[i])
		}
	}
	if math.Float64bits(got.Inercia) != math.Float64bits(todo.Inercia) {
		t.Errorf("inercia %.17g vs %.17g", got.Inercia, todo.Inercia)
	}

	sec, err := Secuencial(d, cent, Opciones{K: 3, MaxIter: 1})
	if err != nil {
		t.Fatalf("Secuencial: %v", err)
	}
	nuevos := append([]float64(nil), cent...)
	Actualizar(nuevos, got)
	for i := range nuevos {
		if math.Float64bits(nuevos[i]) != math.Float64bits(sec.Centroides[i]) {
			t.Fatalf("centroide %d: %.17g vs %.17g", i, nuevos[i], sec.Centroides[i])
		}
	}
}

func TestFusionarRespetaElOrdenDado(t *testing.T) {
	// La suma en punto flotante no es asociativa: (1e16 + 1) - 1e16 = 0, pero
	// (1e16 - 1e16) + 1 = 1. Fusionar no reordena: el orden que recibe fija el
	// resultado, y por eso la PC4 fusiona siempre en orden de nodo.
	a := Parcial{Sumas: []float64{1e16}, Conteos: []int{1}, Inercia: 1e16}
	b := Parcial{Sumas: []float64{1}, Conteos: []int{1}, Inercia: 1}
	c := Parcial{Sumas: []float64{-1e16}, Conteos: []int{1}, Inercia: -1e16}

	abc := Fusionar([]Parcial{a, b, c}, 1, 1)
	acb := Fusionar([]Parcial{a, c, b}, 1, 1)
	if abc.Sumas[0] != 0 || acb.Sumas[0] != 1 {
		t.Fatalf("A,B,C = %v y A,C,B = %v; se esperaban 0 y 1", abc.Sumas[0], acb.Sumas[0])
	}
	if abc.Inercia != 0 || acb.Inercia != 1 {
		t.Fatalf("inercias A,B,C = %v y A,C,B = %v; se esperaban 0 y 1", abc.Inercia, acb.Inercia)
	}
	if abc.Conteos[0] != 3 || acb.Conteos[0] != 3 {
		t.Fatal("los conteos son enteros: no dependen del orden")
	}
}

func TestFusionarDeNadaEsCeroYNoModificaLosParciales(t *testing.T) {
	vacio := Fusionar(nil, 2, 3)
	if len(vacio.Sumas) != 6 || len(vacio.Conteos) != 2 || vacio.Inercia != 0 {
		t.Fatalf("fusionar nada: %+v", vacio)
	}
	a := Parcial{Sumas: []float64{1, 2, 3, 4, 5, 6}, Conteos: []int{1, 2}, Inercia: 3}
	copia := append([]float64(nil), a.Sumas...)
	Fusionar([]Parcial{a, a}, 2, 3)
	for i := range copia {
		if a.Sumas[i] != copia[i] {
			t.Fatal("Fusionar modificó un parcial de entrada")
		}
	}
}

func TestParcialValidarDetectaTamanosQueNoVan(t *testing.T) {
	bueno := Parcial{Sumas: make([]float64, 6), Conteos: make([]int, 2)}
	if err := bueno.Validar(2, 3); err != nil {
		t.Fatalf("parcial bueno rechazado: %v", err)
	}
	casos := map[string]Parcial{
		"sumas de menos":    {Sumas: make([]float64, 5), Conteos: make([]int, 2)},
		"conteos de más":    {Sumas: make([]float64, 6), Conteos: make([]int, 3)},
		"conteo negativo":   {Sumas: make([]float64, 6), Conteos: []int{1, -1}},
		"suma no finita":    {Sumas: []float64{0, 0, math.NaN(), 0, 0, 0}, Conteos: make([]int, 2)},
		"inercia no finita": {Sumas: make([]float64, 6), Conteos: make([]int, 2), Inercia: math.Inf(1)},
	}
	for nombre, p := range casos {
		if err := p.Validar(2, 3); err == nil {
			t.Errorf("%s: se esperaba error", nombre)
		}
	}
	defer func() {
		if recover() == nil {
			t.Fatal("Fusionar debería entrar en pánico con un parcial de otro tamaño")
		}
	}()
	Fusionar([]Parcial{casos["sumas de menos"]}, 2, 3)
}

func TestActualizarConservaElVacioYDevuelveElMayorDesplazamiento(t *testing.T) {
	cent := []float64{0, 0, 10, 10, 5, 5}
	p := Parcial{
		Sumas:   []float64{2, 4, 0, 0, 6, 6},
		Conteos: []int{2, 0, 1},
	}
	mov := Actualizar(cent, p)
	want := []float64{1, 2, 10, 10, 6, 6}
	for i := range want {
		if cent[i] != want[i] {
			t.Fatalf("centroides = %v, se esperaba %v", cent, want)
		}
	}
	if mov != 2 {
		t.Errorf("movimiento = %v, se esperaba 2 (la coordenada que más se movió)", mov)
	}
	if p.Vacios() != 1 {
		t.Errorf("Vacios = %d, se esperaba 1", p.Vacios())
	}
	if Actualizar(cent, p) != 0 {
		t.Error("con el mismo parcial los centroides ya no se mueven")
	}
}

func TestConcurrenteConW1YW8EsIdenticoSobreElGoldSintetico(t *testing.T) {
	// Lo mismo que el test estrella del TP, sobre datos con la forma del gold de
	// OEFA (4 features estandarizadas, 3 000 filas) y el chunk fijo.
	s, err := GoldSintetico(OpcionesSintetico{Features: featuresPrueba, Log1p: []string{"pm10"},
		Nivel: NivelAbsoluto, Estaciones: 6, Horas: 500, Ruido: 0.3, Semilla: 11})
	if err != nil {
		t.Fatalf("GoldSintetico: %v", err)
	}
	d := s.Gold.Datos
	cent, _, err := KMeansPP(d, 5, 42)
	if err != nil {
		t.Fatalf("KMeansPP: %v", err)
	}
	op := OpcionesConc{Opciones: Opciones{K: 5, MaxIter: 100, TolAbs: 1e-6, TolRel: 1e-6}, Chunk: 128}
	op.Workers = 1
	uno, err := Concurrente(d, cent, op)
	if err != nil {
		t.Fatalf("Concurrente W=1: %v", err)
	}
	op.Workers = 8
	ocho, err := Concurrente(d, cent, op)
	if err != nil {
		t.Fatalf("Concurrente W=8: %v", err)
	}
	mismoResultado(t, "W=8 contra W=1", ocho, uno)
}
