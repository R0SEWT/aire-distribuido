package kmeans

import (
	"math"
	"testing"
)

func casi(t *testing.T, got, want, tol float64, que string) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Errorf("%s = %v, se esperaba %v", que, got, want)
	}
}

func TestMediaRecortadaDescartaLasColas(t *testing.T) {
	// 10 valores, 10 % por extremo: se va el 1 y se va el 10, queda 2..9.
	xs := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	m, err := MediaRecortada(xs, 0.10)
	if err != nil {
		t.Fatalf("MediaRecortada: %v", err)
	}
	casi(t, m, 5.5, 1e-12, "media recortada")
}

func TestMediaRecortadaNoSeDejaLlevarPorUnOutlier(t *testing.T) {
	// El caso real: una corrida se va a las nubes porque el sistema hizo otra
	// cosa. La media se dispara; la recortada casi no se mueve.
	xs := []float64{10, 10, 10, 10, 10, 10, 10, 10, 10, 1000}

	media := 0.0
	for _, x := range xs {
		media += x
	}
	media /= float64(len(xs))
	m, err := MediaRecortada(xs, 0.10)
	if err != nil {
		t.Fatalf("MediaRecortada: %v", err)
	}
	if media < 100 {
		t.Fatalf("la media debería estar contaminada, es %v", media)
	}
	casi(t, m, 10, 1e-12, "media recortada")
}

func TestMediaRecortadaConProporcionCeroEsLaMedia(t *testing.T) {
	xs := []float64{2, 4, 6, 8}
	m, err := MediaRecortada(xs, 0)
	if err != nil {
		t.Fatalf("MediaRecortada: %v", err)
	}
	casi(t, m, 5, 1e-12, "media recortada con prop 0")
}

func TestMediaRecortadaNoModificaLaMuestra(t *testing.T) {
	xs := []float64{5, 1, 4, 2, 3}
	copia := append([]float64(nil), xs...)

	if _, err := MediaRecortada(xs, 0.2); err != nil {
		t.Fatalf("MediaRecortada: %v", err)
	}
	for i := range copia {
		if xs[i] != copia[i] {
			t.Fatalf("MediaRecortada reordenó la muestra original en %d", i)
		}
	}
}

func TestMediaRecortadaRechazaEntradasInvalidas(t *testing.T) {
	casos := []struct {
		nombre string
		xs     []float64
		prop   float64
	}{
		{"muestra vacía", nil, 0.1},
		{"proporción negativa", []float64{1, 2, 3}, -0.1},
		{"proporción de la mitad", []float64{1, 2, 3}, 0.5},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if _, err := MediaRecortada(c.xs, c.prop); err == nil {
				t.Fatal("se esperaba error")
			}
		})
	}
}

func TestMedianaParEImpar(t *testing.T) {
	impar, err := Mediana([]float64{3, 1, 2})
	if err != nil {
		t.Fatalf("Mediana: %v", err)
	}
	casi(t, impar, 2, 1e-12, "mediana impar")

	par, err := Mediana([]float64{4, 1, 3, 2})
	if err != nil {
		t.Fatalf("Mediana: %v", err)
	}
	casi(t, par, 2.5, 1e-12, "mediana par")
}

func TestDesviacionEstandarMuestral(t *testing.T) {
	// s de {2,4,4,4,5,5,7,9} con n-1 en el denominador es 2.13809...
	s, err := DesviacionEstandar([]float64{2, 4, 4, 4, 5, 5, 7, 9})
	if err != nil {
		t.Fatalf("DesviacionEstandar: %v", err)
	}
	casi(t, s, 2.13809, 1e-5, "desviación estándar")
}

func TestICBootstrapDelCocienteEsDeterministaYContieneElPunto(t *testing.T) {
	// Secuencial alrededor de 10, concurrente alrededor de 5: speedup ~2.
	sec := []float64{10.1, 9.9, 10.0, 10.2, 9.8, 10.05, 9.95, 10.1, 9.9, 10.0}
	con := []float64{5.05, 4.95, 5.0, 5.1, 4.9, 5.02, 4.98, 5.05, 4.95, 5.0}

	ic, err := ICBootstrapCociente(sec, con, 0.10, 2000, 7)
	if err != nil {
		t.Fatalf("ICBootstrapCociente: %v", err)
	}
	if ic.Punto < 1.9 || ic.Punto > 2.1 {
		t.Errorf("speedup puntual = %v, se esperaba cerca de 2", ic.Punto)
	}
	if !(ic.Inferior <= ic.Punto && ic.Punto <= ic.Superior) {
		t.Errorf("el punto %v no cae en el intervalo [%v, %v]", ic.Punto, ic.Inferior, ic.Superior)
	}
	if ic.Superior-ic.Inferior > 0.5 {
		t.Errorf("el intervalo es absurdamente ancho para datos tan limpios: [%v, %v]", ic.Inferior, ic.Superior)
	}

	otra, err := ICBootstrapCociente(sec, con, 0.10, 2000, 7)
	if err != nil {
		t.Fatalf("ICBootstrapCociente: %v", err)
	}
	if otra != ic {
		t.Errorf("con la misma semilla el intervalo debería repetirse: %+v vs %+v", otra, ic)
	}
}

func TestICBootstrapRechazaEntradasInvalidas(t *testing.T) {
	buena := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	casos := []struct {
		nombre         string
		sec, con       []float64
		prop           float64
		repes, semilla int
	}{
		{"sin muestras", nil, buena, 0.1, 100, 1},
		{"cero repeticiones", buena, buena, 0.1, 0, 1},
		{"proporción inválida", buena, buena, 0.6, 100, 1},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if _, err := ICBootstrapCociente(c.sec, c.con, c.prop, c.repes, int64(c.semilla)); err == nil {
				t.Fatal("se esperaba error")
			}
		})
	}
}

func TestMediaRecortadaConMuestraChicaNoRecortaNada(t *testing.T) {
	// floor(2 * 0.49) = 0: con dos observaciones no hay qué descartar, y el
	// resultado es la media. Es la semántica de floor, no un caso de error.
	m, err := MediaRecortada([]float64{1, 2}, 0.49)
	if err != nil {
		t.Fatalf("MediaRecortada: %v", err)
	}
	casi(t, m, 1.5, 1e-12, "media recortada de dos observaciones")
}
