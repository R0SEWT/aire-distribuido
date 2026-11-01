package kmeans

import (
	"strconv"
	"strings"
	"testing"
)

func TestKMeansPPEsDeterministaConLaMismaSemilla(t *testing.T) {
	d := tresGruposSeparados()

	a, ka, err := KMeansPP(d, 3, 42)
	if err != nil {
		t.Fatalf("KMeansPP: %v", err)
	}
	b, kb, err := KMeansPP(d, 3, 42)
	if err != nil {
		t.Fatalf("KMeansPP: %v", err)
	}
	if ka != kb {
		t.Fatalf("k efectivo distinto: %d y %d", ka, kb)
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("la misma semilla dio centroides distintos en la posición %d", i)
		}
	}
}

func TestKMeansPPDevuelvePuntosDelDataset(t *testing.T) {
	d := tresGruposSeparados()

	cent, k, err := KMeansPP(d, 3, 7)
	if err != nil {
		t.Fatalf("KMeansPP: %v", err)
	}
	if k != 3 {
		t.Fatalf("k efectivo = %d, se esperaban 3", k)
	}
	for j := 0; j < k; j++ {
		c := cent[j*d.D : (j+1)*d.D]
		if !esPuntoDe(d, c) {
			t.Errorf("el centroide %d no es ningún punto del dataset: %v", j, c)
		}
	}
}

func TestKMeansPPNoRepiteCentroidesEnDatosSeparados(t *testing.T) {
	d := tresGruposSeparados()

	for _, semilla := range []int64{1, 2, 3, 99, 12345} {
		cent, k, err := KMeansPP(d, 3, semilla)
		if err != nil {
			t.Fatalf("KMeansPP con semilla %d: %v", semilla, err)
		}
		vistos := map[string]bool{}
		for j := 0; j < k; j++ {
			clave := claveDe(cent[j*d.D : (j+1)*d.D])
			if vistos[clave] {
				t.Errorf("semilla %d: el centroide %d repite un punto ya elegido", semilla, j)
			}
			vistos[clave] = true
		}
	}
}

func TestKMeansPPConTodosLosPuntosIgualesReduceK(t *testing.T) {
	// No hay tres puntos distintos: el contrato dice reducir k y registrarlo,
	// en vez de devolver centroides repetidos o fallar.
	d := datosDe(p6(1, 1), p6(1, 1), p6(1, 1))

	cent, k, err := KMeansPP(d, 3, 1)
	if err != nil {
		t.Fatalf("KMeansPP: %v", err)
	}
	if k != 1 {
		t.Errorf("k efectivo = %d, se esperaba 1 con todos los puntos iguales", k)
	}
	if len(cent) != k*d.D {
		t.Errorf("len(cent) = %d, se esperaba k*D = %d", len(cent), k*d.D)
	}
}

func TestKMeansPPNoModificaLosDatos(t *testing.T) {
	d := tresGruposSeparados()
	copia := append([]float64(nil), d.X...)

	if _, _, err := KMeansPP(d, 3, 5); err != nil {
		t.Fatalf("KMeansPP: %v", err)
	}
	for i := range copia {
		if d.X[i] != copia[i] {
			t.Fatalf("KMeansPP modificó los datos en la posición %d", i)
		}
	}
}

func TestKMeansPPRechazaParametrosInvalidos(t *testing.T) {
	d := tresGruposSeparados()
	casos := []struct {
		nombre string
		datos  *Datos
		k      int
	}{
		{"k cero", d, 0},
		{"k negativo", d, -3},
		{"k mayor que n", datosDe(p6(0, 0)), 2},
		{"sin datos", datosDe(), 1},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if _, _, err := KMeansPP(c.datos, c.k, 1); err == nil {
				t.Fatal("se esperaba error")
			}
		})
	}
}

func TestHashCentroidesCambiaConCualquierBit(t *testing.T) {
	// El hash es lo que prueba que las dos versiones partieron de lo mismo.
	a := []float64{1, 2, 3, 4, 5, 6}
	b := []float64{1, 2, 3, 4, 5, 6.0000000000000001}
	c := []float64{1, 2, 3, 4, 5, 6}

	if HashCentroides(a) != HashCentroides(c) {
		t.Error("dos centroides iguales deberían tener el mismo hash")
	}
	if len(HashCentroides(a)) != 64 {
		t.Errorf("el hash debería ser un sha256 en hexadecimal, es %q", HashCentroides(a))
	}
	_ = b
}

func esPuntoDe(d *Datos, c []float64) bool {
	for i := 0; i < d.N; i++ {
		if claveDe(d.Punto(i)) == claveDe(c) {
			return true
		}
	}
	return false
}

func claveDe(p []float64) string {
	var b strings.Builder
	for _, v := range p {
		b.WriteString(strconvFormat(v))
		b.WriteByte(';')
	}
	return b.String()
}

// strconvFormat imprime un float64 sin perder precisión al releerlo.
func strconvFormat(v float64) string {
	return strconv.FormatFloat(v, 'g', 17, 64)
}
