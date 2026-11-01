package kmeans

import (
	"fmt"
	"math"
	"testing"
)

func opcionesSemillas(workers, chunk int) OpcionesConc {
	return OpcionesConc{Opciones: Opciones{MaxIter: 100, TolAbs: 1e-6, TolRel: 1e-6}, Workers: workers, Chunk: chunk}
}

// casoOptimoLocal es el gold sintético en el que una sola semilla (42) parte
// un régimen plantado en dos y junta otros dos: el caso que motivó las semillas.
func casoOptimoLocal(t *testing.T) *Sintetico {
	t.Helper()
	s, err := GoldSintetico(OpcionesSintetico{Features: featuresPrueba, Log1p: []string{"pm10"},
		Nivel: NivelAbsoluto, Estaciones: 4, Horas: 600, Ruido: 0.1, Semilla: 3})
	if err != nil {
		t.Fatalf("GoldSintetico: %v", err)
	}
	return s
}

// recuperados cuenta los regímenes plantados que tienen un régimen encontrado
// con al menos 95 % de pureza.
func recuperados(s *Sintetico, asign []int, k int) int {
	vistos := map[int]bool{}
	for j := 0; j < k; j++ {
		if p, pureza := mayoritario(s, asign, j); pureza >= 0.95 {
			vistos[p] = true
		}
	}
	return len(vistos)
}

func TestMejorDeSemillasConUnaEsLaCorridaDeSiempre(t *testing.T) {
	s := casoOptimoLocal(t)
	d := s.Gold.Datos
	op := opcionesSemillas(3, 250)

	m, err := MejorDeSemillas(d, 5, 42, 1, op, 0)
	if err != nil {
		t.Fatalf("MejorDeSemillas: %v", err)
	}
	cent, _, err := KMeansPP(d, 5, 42)
	if err != nil {
		t.Fatal(err)
	}
	op.K = 5
	directo, err := Concurrente(d, cent, op)
	if err != nil {
		t.Fatal(err)
	}
	mismoResultado(t, "una semilla contra la corrida directa", m.Mejor, directo)
	if len(m.Arranques) != 1 || m.Elegido != 0 || m.Arranques[0].Semilla != 42 {
		t.Errorf("arranques %+v, elegido %d", m.Arranques, m.Elegido)
	}
	if m.Arranques[0].CentroidesIniciales != HashCentroides(cent) {
		t.Error("el hash de los centroides iniciales no es el de k-means++ con la semilla 42")
	}
}

func TestMejorDeSemillasEligeLaMenorInercia(t *testing.T) {
	s := casoOptimoLocal(t)
	m, err := MejorDeSemillas(s.Gold.Datos, 5, 42, 10, opcionesSemillas(4, 250), 0)
	if err != nil {
		t.Fatalf("MejorDeSemillas: %v", err)
	}
	if len(m.Arranques) != 10 {
		t.Fatalf("%d arranques, se esperaban 10", len(m.Arranques))
	}
	mejor := 0
	for i, a := range m.Arranques {
		if a.Semilla != 42+int64(i) {
			t.Errorf("arranque %d con semilla %d, se esperaba %d", i, a.Semilla, 42+i)
		}
		if a.Inercia < m.Arranques[mejor].Inercia {
			mejor = i
		}
	}
	if m.Elegido != mejor {
		t.Errorf("se eligió el arranque %d y el de menor inercia es el %d", m.Elegido, mejor)
	}
	if math.Float64bits(m.Mejor.Inercia) != math.Float64bits(m.Arranques[mejor].Inercia) {
		t.Errorf("el resultado devuelto tiene inercia %v y el elegido %v", m.Mejor.Inercia, m.Arranques[mejor].Inercia)
	}
}

func TestMejorDeSemillasEsElMismoParaCualquierWYCualquierParalelismo(t *testing.T) {
	// El resultado de cada semilla no depende de W (reducción en orden de
	// chunk) y la elección es un mínimo bajo un orden total (inercia, semilla):
	// no importa cuántas corran a la vez ni en qué orden terminen.
	s := casoOptimoLocal(t)
	d := s.Gold.Datos
	ref, err := MejorDeSemillas(d, 5, 42, 10, opcionesSemillas(1, 250), 1)
	if err != nil {
		t.Fatalf("MejorDeSemillas W=1: %v", err)
	}
	for _, c := range [][2]int{{8, 0}, {8, 1}, {8, 3}, {8, 10}, {3, 10}, {1, 4}} {
		w, p := c[0], c[1]
		t.Run(fmt.Sprintf("W=%d paralelas=%d", w, p), func(t *testing.T) {
			m, err := MejorDeSemillas(d, 5, 42, 10, opcionesSemillas(w, 250), p)
			if err != nil {
				t.Fatalf("MejorDeSemillas: %v", err)
			}
			if m.Elegido != ref.Elegido {
				t.Fatalf("eligió el arranque %d, con W=1 en serie fue el %d", m.Elegido, ref.Elegido)
			}
			mismoResultado(t, "contra W=1 en serie", m.Mejor, ref.Mejor)
			for i := range ref.Arranques {
				a, b := m.Arranques[i], ref.Arranques[i]
				if math.Float64bits(a.Inercia) != math.Float64bits(b.Inercia) || a.Iteraciones != b.Iteraciones || a.Paro != b.Paro {
					t.Errorf("semilla %d: %+v vs %+v", a.Semilla, a, b)
				}
			}
		})
	}
}

func TestMejorDeSemillasEmpataPorLaSemillaMenor(t *testing.T) {
	// Tres nubes muy separadas: toda semilla llega a la misma partición, con
	// los mismos centroides y la misma inercia bit a bit. Gana la primera.
	d := tresGruposSeparados()
	m, err := MejorDeSemillas(d, 3, 100, 6, OpcionesConc{Opciones: opcionesDe(3), Workers: 4, Chunk: 2}, 0)
	if err != nil {
		t.Fatalf("MejorDeSemillas: %v", err)
	}
	for _, a := range m.Arranques {
		if math.Float64bits(a.Inercia) != math.Float64bits(m.Arranques[0].Inercia) {
			t.Fatalf("el caso no empata: inercias %+v", m.Arranques)
		}
	}
	if m.Elegido != 0 {
		t.Errorf("con todas empatadas se eligió el arranque %d; tiene que ganar la semilla menor", m.Elegido)
	}
}

func TestDiezSemillasRecuperanLosRegimenesQueUnaParte(t *testing.T) {
	s := casoOptimoLocal(t)
	d := s.Gold.Datos
	una, err := MejorDeSemillas(d, 5, 42, 1, opcionesSemillas(3, 250), 0)
	if err != nil {
		t.Fatal(err)
	}
	if n := recuperados(s, una.Mejor.Asignaciones, 5); n == 5 {
		t.Fatal("con una semilla este caso ya no cae en el óptimo local; el test dejó de probar algo")
	}
	diez, err := MejorDeSemillas(d, 5, 42, 10, opcionesSemillas(3, 250), 0)
	if err != nil {
		t.Fatal(err)
	}
	if n := recuperados(s, diez.Mejor.Asignaciones, 5); n != 5 {
		t.Errorf("con 10 semillas se recuperan %d de 5 regímenes plantados", n)
	}
	if !(diez.Mejor.Inercia < una.Mejor.Inercia) {
		t.Errorf("10 semillas: inercia %v, no mejora la de una sola (%v)", diez.Mejor.Inercia, una.Mejor.Inercia)
	}
}

func TestRepartirSemillas(t *testing.T) {
	casos := []struct {
		n, workers, paralelas int
		quiero                []int
	}{
		{10, 14, 0, []int{5, 5, 4}}, // auto: round(√min(n, W)) = 3 a la vez, los 14 repartidos
		{1, 14, 0, []int{14}},       // una semilla usa todo el pool
		{10, 4, 0, []int{2, 2}},     // W chico: √4 = 2 a la vez con 2 workers cada una
		{2, 14, 0, []int{14}},       // √2 redondea a 1: en serie con todo el pool
		{10, 14, 1, []int{14}},      // en serie, cada una con los 14
		{10, 14, 10, []int{2, 2, 2, 2, 1, 1, 1, 1, 1, 1}},
		{10, 14, 2, []int{7, 7}},
		{3, 8, 5, []int{3, 3, 2}}, // no más corridas a la vez que semillas
		{10, 2, 6, []int{1, 1, 1, 1, 1, 1}},
	}
	for _, c := range casos {
		got := RepartirSemillas(c.n, c.workers, c.paralelas)
		if fmt.Sprint(got) != fmt.Sprint(c.quiero) {
			t.Errorf("RepartirSemillas(%d, %d, %d) = %v, se esperaba %v", c.n, c.workers, c.paralelas, got, c.quiero)
		}
	}
}

func TestMejorDeSemillasRechazaParametrosInvalidos(t *testing.T) {
	d := nubes(20)
	op := opcionesSemillas(2, 10)
	if _, err := MejorDeSemillas(d, 3, 1, 0, op, 0); err == nil {
		t.Error("cero semillas: se esperaba error")
	}
	if _, err := MejorDeSemillas(d, 3, 1, 2, op, -1); err == nil {
		t.Error("paralelas negativo: se esperaba error")
	}
	op.Workers = 0
	if _, err := MejorDeSemillas(d, 3, 1, 2, op, 0); err == nil {
		t.Error("cero workers: se esperaba error")
	}
	if _, err := MejorDeSemillas(d, 0, 1, 2, opcionesSemillas(2, 10), 0); err == nil {
		t.Error("k cero: se esperaba error")
	}
}
