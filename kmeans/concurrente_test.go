package kmeans

import (
	"fmt"
	"math"
	"testing"
)

func opcionesConcDe(k, workers, chunk int) OpcionesConc {
	return OpcionesConc{Opciones: opcionesDe(k), Workers: workers, Chunk: chunk}
}

// nubes arma un dataset más grande que los de juguete, para que haya varios
// chunks de verdad y el reparto entre workers importe.
func nubes(porNube int) *Datos {
	d := &Datos{D: dimPrueba}
	centros := [][2]float64{{0, 0}, {50, 50}, {-50, 30}, {30, -40}}
	for i := 0; i < porNube; i++ {
		for _, c := range centros {
			// Desplazamiento determinista, sin aleatoriedad: los tests no deben
			// depender de una semilla para ser reproducibles.
			dx := float64(i%7) * 0.3
			dy := float64(i%5) * 0.4
			d.X = append(d.X, p6(c[0]+dx, c[1]+dy)...)
			d.N++
		}
	}
	return d
}

func centroidesDe(d *Datos, k int, semilla int64, t *testing.T) []float64 {
	t.Helper()
	cent, kEfectivo, err := KMeansPP(d, k, semilla)
	if err != nil {
		t.Fatalf("KMeansPP: %v", err)
	}
	if kEfectivo != k {
		t.Fatalf("k efectivo = %d, se esperaban %d", kEfectivo, k)
	}
	return cent
}

func TestConcurrenteConUnChunkYUnWorkerEsIdenticoAlSecuencial(t *testing.T) {
	// Con un solo chunk que cubre todo, el orden de las sumas es exactamente el
	// del secuencial, así que el resultado debe coincidir bit a bit.
	d := nubes(50)
	cent := centroidesDe(d, 4, 7, t)

	sec, err := Secuencial(d, cent, opcionesDe(4))
	if err != nil {
		t.Fatalf("Secuencial: %v", err)
	}
	con, err := Concurrente(d, cent, opcionesConcDe(4, 1, d.N))
	if err != nil {
		t.Fatalf("Concurrente: %v", err)
	}

	if con.Inercia != sec.Inercia {
		t.Errorf("inercia: concurrente %.17g, secuencial %.17g", con.Inercia, sec.Inercia)
	}
	for i := range sec.Centroides {
		if con.Centroides[i] != sec.Centroides[i] {
			t.Fatalf("centroide distinto en la posición %d: %.17g vs %.17g",
				i, con.Centroides[i], sec.Centroides[i])
		}
	}
}

func TestConcurrenteDaElMismoResultadoParaCualquierP(t *testing.T) {
	// La propiedad que justifica reducir en orden de chunk: el resultado no
	// depende de cuántos workers haya ni de qué chunk le tocó a cada uno.
	d := nubes(200)
	cent := centroidesDe(d, 4, 11, t)

	referencia, err := Concurrente(d, cent, opcionesConcDe(4, 1, 64))
	if err != nil {
		t.Fatalf("Concurrente: %v", err)
	}
	for _, p := range []int{2, 3, 4, 8, 16} {
		r, err := Concurrente(d, cent, opcionesConcDe(4, p, 64))
		if err != nil {
			t.Fatalf("Concurrente con P=%d: %v", p, err)
		}
		if r.Inercia != referencia.Inercia {
			t.Errorf("P=%d: inercia %.17g, se esperaba %.17g", p, r.Inercia, referencia.Inercia)
		}
		for i := range referencia.Centroides {
			if r.Centroides[i] != referencia.Centroides[i] {
				t.Fatalf("P=%d: centroide distinto en la posición %d (%.17g vs %.17g)",
					p, i, r.Centroides[i], referencia.Centroides[i])
			}
		}
		if r.Iteraciones != referencia.Iteraciones {
			t.Errorf("P=%d: %d iteraciones, se esperaban %d", p, r.Iteraciones, referencia.Iteraciones)
		}
	}
}

func TestConcurrenteEquivaleAlSecuencialConTolerancia(t *testing.T) {
	// Con varios chunks el orden de las sumas cambia, así que la igualdad es
	// con tolerancia. Las asignaciones, en cambio, deben ser las mismas.
	d := nubes(200)
	cent := centroidesDe(d, 4, 3, t)

	sec, err := Secuencial(d, cent, opcionesDe(4))
	if err != nil {
		t.Fatalf("Secuencial: %v", err)
	}
	con, err := Concurrente(d, cent, opcionesConcDe(4, 4, 37))
	if err != nil {
		t.Fatalf("Concurrente: %v", err)
	}

	const tol = 1e-9
	if math.Abs(con.Inercia-sec.Inercia) > tol*math.Max(1, math.Abs(sec.Inercia)) {
		t.Errorf("inercia: concurrente %v, secuencial %v", con.Inercia, sec.Inercia)
	}
	for i := range sec.Centroides {
		if math.Abs(con.Centroides[i]-sec.Centroides[i]) > tol {
			t.Errorf("centroide %d: %v vs %v", i, con.Centroides[i], sec.Centroides[i])
		}
	}
	distintas := 0
	for i := range sec.Asignaciones {
		if sec.Asignaciones[i] != con.Asignaciones[i] {
			distintas++
		}
	}
	if distintas != 0 {
		t.Errorf("%d de %d asignaciones difieren; la partición debería ser la misma", distintas, d.N)
	}
}

func TestConcurrenteConMasWorkersQuePuntos(t *testing.T) {
	d := datosDe(p6(0, 0), p6(10, 10), p6(0, 1))
	cent := append(append([]float64{}, p6(0, 0)...), p6(10, 10)...)

	r, err := Concurrente(d, cent, opcionesConcDe(2, 16, 1))
	if err != nil {
		t.Fatalf("Concurrente: %v", err)
	}
	if len(r.Asignaciones) != d.N {
		t.Errorf("len(Asignaciones) = %d, se esperaban %d", len(r.Asignaciones), d.N)
	}
}

func TestConcurrenteConChunkMayorQueN(t *testing.T) {
	d := nubes(10)
	cent := centroidesDe(d, 3, 5, t)

	r, err := Concurrente(d, cent, opcionesConcDe(3, 4, 10*d.N))
	if err != nil {
		t.Fatalf("Concurrente: %v", err)
	}
	if r.Iteraciones == 0 {
		t.Error("no corrió ninguna iteración")
	}
}

func TestConcurrenteConKUnoDaLaMedia(t *testing.T) {
	// Con un solo cluster, todos los puntos deben aportar a la misma media.
	// Chunk=1 obliga a reducir tres parciales, incluso con más workers que puntos.
	d := datosDe(p6(0, 0), p6(2, 0), p6(4, 0))
	cent := p6(10, 10)
	esperado := p6(2, 0)

	for _, workers := range []int{1, 2, 4} {
		t.Run(fmt.Sprintf("workers=%d", workers), func(t *testing.T) {
			r, err := Concurrente(d, cent, opcionesConcDe(1, workers, 1))
			if err != nil {
				t.Fatalf("Concurrente: %v", err)
			}
			for f, valor := range esperado {
				if got := r.Centroide(0)[f]; got != valor {
					t.Errorf("centroide, coordenada %d = %v; se esperaba %v", f, got, valor)
				}
			}
			if len(r.Asignaciones) != d.N {
				t.Fatalf("%d asignaciones; se esperaban %d", len(r.Asignaciones), d.N)
			}
			for i, got := range r.Asignaciones {
				if got != 0 {
					t.Errorf("asignación del punto %d = %d; se esperaba 0", i, got)
				}
			}
			if r.Inercia != 8 {
				t.Errorf("inercia = %v; se esperaba 8", r.Inercia)
			}
			if r.Vacios != 0 {
				t.Errorf("Vacios = %d; se esperaba 0", r.Vacios)
			}
		})
	}
}

func TestConcurrenteConKIgualANDaInerciaCero(t *testing.T) {
	// Cada punto distinto empieza con su propio centroide. Ninguno debe
	// cambiar ni compartir cluster al repartir los puntos entre workers.
	d := datosDe(p6(0, 0), p6(5, 5), p6(9, 1))
	cent := append(append(p6(0, 0), p6(5, 5)...), p6(9, 1)...)

	for _, workers := range []int{1, 2, 4} {
		t.Run(fmt.Sprintf("workers=%d", workers), func(t *testing.T) {
			r, err := Concurrente(d, cent, opcionesConcDe(d.N, workers, 1))
			if err != nil {
				t.Fatalf("Concurrente: %v", err)
			}
			for i, esperado := range cent {
				if got := r.Centroides[i]; got != esperado {
					t.Errorf("centroide, coordenada %d = %v; se esperaba %v", i, got, esperado)
				}
			}
			if len(r.Asignaciones) != d.N {
				t.Fatalf("%d asignaciones; se esperaban %d", len(r.Asignaciones), d.N)
			}
			for i, got := range r.Asignaciones {
				if got != i {
					t.Errorf("asignación del punto %d = %d; se esperaba %d", i, got, i)
				}
			}
			if r.Inercia != 0 {
				t.Errorf("inercia = %v; se esperaba 0", r.Inercia)
			}
			if r.Vacios != 0 {
				t.Errorf("Vacios = %d; se esperaba 0", r.Vacios)
			}
		})
	}
}

func TestConcurrenteClusterVacioConservaSuCentroide(t *testing.T) {
	// Hay cuatro puntos en dos grupos, pero el tercer centroide está tan lejos
	// que nunca recibe uno. Chunk=1 reparte los puntos en cuatro trabajos.
	d := datosDe(p6(0, 0), p6(0, 2), p6(10, 0), p6(10, 2))
	lejano := p6(1e6, 1e6)
	cent := append(append(p6(0, 0), p6(10, 0)...), lejano...)
	esperados := [][]float64{p6(0, 1), p6(10, 1), lejano}

	for _, workers := range []int{1, 2, 4} {
		t.Run(fmt.Sprintf("workers=%d", workers), func(t *testing.T) {
			op := OpcionesConc{
				Opciones: Opciones{K: 3, MaxIter: 5, TolAbs: 0, TolRel: 0},
				Workers:  workers,
				Chunk:    1,
			}
			r, err := Concurrente(d, cent, op)
			if err != nil {
				t.Fatalf("Concurrente: %v", err)
			}
			if r.Vacios != 1 {
				t.Errorf("Vacios = %d, se esperaba 1", r.Vacios)
			}
			if r.Iteraciones != 2 {
				t.Errorf("Iteraciones = %d, se esperaban 2 para comprobar el vacío también en la segunda ronda", r.Iteraciones)
			}
			for j, esperado := range esperados {
				for f, valor := range esperado {
					if got := r.Centroide(j)[f]; got != valor {
						t.Errorf("centroide %d, coordenada %d = %v; se esperaba %v", j, f, got, valor)
					}
				}
			}
			for i, esperado := range []int{0, 0, 1, 1} {
				if got := r.Asignaciones[i]; got != esperado {
					t.Errorf("asignación del punto %d = %d; se esperaba %d", i, got, esperado)
				}
			}
		})
	}
}

func TestConcurrenteDesempataPorElIndiceMenor(t *testing.T) {
	// El punto (0, 0) está a distancia 1 de ambos centroides. Si gana el índice 0,
	// la media de los puntos -2 y 0 sigue siendo -1; el empate persiste incluso
	// en la pasada final. Chunk=1 pone el punto empatado en un trabajo propio.
	d := datosDe(p6(-2, 0), p6(0, 0), p6(1, 0), p6(1, 0))
	cent := append(p6(-1, 0), p6(1, 0)...)

	for _, workers := range []int{1, 2, 4} {
		t.Run(fmt.Sprintf("workers=%d", workers), func(t *testing.T) {
			r, err := Concurrente(d, cent, OpcionesConc{
				Opciones: Opciones{K: 2, MaxIter: 3, TolAbs: 0, TolRel: 0},
				Workers:  workers,
				Chunk:    1,
			})
			if err != nil {
				t.Fatalf("Concurrente: %v", err)
			}
			for i, esperado := range []int{0, 0, 1, 1} {
				if got := r.Asignaciones[i]; got != esperado {
					t.Errorf("asignación del punto %d = %d; se esperaba %d", i, got, esperado)
				}
			}
			if r.Inercia != 2 {
				t.Errorf("inercia = %v; se esperaba 2", r.Inercia)
			}
			if r.Vacios != 0 {
				t.Errorf("Vacios = %d; se esperaba 0", r.Vacios)
			}
		})
	}
}

func TestConcurrenteAsignaTodosLosPuntosExactamenteUnaVez(t *testing.T) {
	// El equivalente en Go de la propiedad que verifica el modelo Promela:
	// la suma de los conteos no alcanza, hay que contar por punto.
	d := nubes(100)
	cent := centroidesDe(d, 4, 13, t)

	r, err := Concurrente(d, cent, opcionesConcDe(4, 4, 17))
	if err != nil {
		t.Fatalf("Concurrente: %v", err)
	}
	vistos := make([]int, d.N)
	for i, j := range r.Asignaciones {
		if j < 0 || j >= 4 {
			t.Fatalf("el punto %d quedó con el cluster %d, fuera de rango", i, j)
		}
		vistos[i]++
	}
	for i, v := range vistos {
		if v != 1 {
			t.Errorf("el punto %d se procesó %d veces", i, v)
		}
	}
}

func TestConcurrenteNoModificaEntradas(t *testing.T) {
	d := nubes(20)
	copiaX := append([]float64(nil), d.X...)
	cent := centroidesDe(d, 3, 2, t)
	copiaCent := append([]float64(nil), cent...)

	if _, err := Concurrente(d, cent, opcionesConcDe(3, 4, 8)); err != nil {
		t.Fatalf("Concurrente: %v", err)
	}
	for i := range copiaX {
		if d.X[i] != copiaX[i] {
			t.Fatalf("Concurrente modificó los datos en la posición %d", i)
		}
	}
	for i := range copiaCent {
		if cent[i] != copiaCent[i] {
			t.Fatalf("Concurrente modificó los centroides iniciales en la posición %d", i)
		}
	}
}

func TestConcurrenteRechazaParametrosInvalidos(t *testing.T) {
	d := nubes(10)
	cent := centroidesDe(d, 2, 1, t)

	casos := []struct {
		nombre  string
		workers int
		chunk   int
	}{
		{"workers cero", 0, 8},
		{"workers negativo", -2, 8},
		{"chunk cero", 4, 0},
		{"chunk negativo", 4, -5},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			_, err := Concurrente(d, cent, OpcionesConc{
				Opciones: opcionesDe(2), Workers: c.workers, Chunk: c.chunk,
			})
			if err == nil {
				t.Fatal("se esperaba error")
			}
		})
	}
}

func TestConcurrenteInformaPorQueParoIgualQueElSecuencial(t *testing.T) {
	d := nubes(50)
	cent := centroidesDe(d, 4, 9, t)

	sec, err := Secuencial(d, cent, opcionesDe(4))
	if err != nil {
		t.Fatalf("Secuencial: %v", err)
	}
	con, err := Concurrente(d, cent, opcionesConcDe(4, 4, 16))
	if err != nil {
		t.Fatalf("Concurrente: %v", err)
	}
	if con.Paro != sec.Paro {
		t.Errorf("Paro: concurrente %q, secuencial %q", con.Paro, sec.Paro)
	}
	if con.Iteraciones != sec.Iteraciones {
		t.Errorf("iteraciones: concurrente %d, secuencial %d", con.Iteraciones, sec.Iteraciones)
	}
}
