package kmeans

import (
	"math"
	"testing"
)

func TestGoldSinteticoEsDeterministaYOrdenado(t *testing.T) {
	op := OpcionesSintetico{Features: featuresPrueba, Log1p: []string{"pm10"}, Nivel: NivelAbsoluto,
		Estaciones: 3, Horas: 100, Ruido: 0.1, Semilla: 3}
	a, err := GoldSintetico(op)
	if err != nil {
		t.Fatalf("GoldSintetico: %v", err)
	}
	b, err := GoldSintetico(op)
	if err != nil {
		t.Fatalf("GoldSintetico: %v", err)
	}
	if a.Gold.Datos.N != 300 || len(a.Plantado) != 300 || len(a.Originales) != 300*4 {
		t.Fatalf("tamaños: N=%d plantado=%d originales=%d", a.Gold.Datos.N, len(a.Plantado), len(a.Originales))
	}
	for i := range a.Gold.Datos.X {
		if a.Gold.Datos.X[i] != b.Gold.Datos.X[i] {
			t.Fatal("la misma semilla dio otro gold")
		}
	}
	for i := 1; i < a.Gold.Datos.N; i++ {
		ea, eb := a.Gold.Estacion[i-1], a.Gold.Estacion[i]
		if ea > eb || (ea == eb && !a.Gold.FechaHora[i-1].Before(a.Gold.FechaHora[i])) {
			t.Fatalf("fila %d fuera de orden", i)
		}
	}
	// Cada feature estandarizada tiene media 0 y desviación 1 sobre el total.
	d := a.Gold.Datos
	for f := 0; f < d.D; f++ {
		m, v := 0.0, 0.0
		for i := 0; i < d.N; i++ {
			m += d.X[i*d.D+f]
		}
		m /= float64(d.N)
		for i := 0; i < d.N; i++ {
			v += (d.X[i*d.D+f] - m) * (d.X[i*d.D+f] - m)
		}
		if math.Abs(m) > 1e-12 || math.Abs(v/float64(d.N)-1) > 1e-12 {
			t.Errorf("feature %d: media %v, varianza %v", f, m, v/float64(d.N))
		}
	}
	vistos := map[int]int{}
	for _, r := range a.Plantado {
		vistos[r]++
	}
	if len(vistos) != len(RegimenesDePrueba()) {
		t.Errorf("se plantaron %d regímenes de %d", len(vistos), len(RegimenesDePrueba()))
	}
}

func TestGoldSinteticoRechazaFeaturesSinRegimen(t *testing.T) {
	_, err := GoldSintetico(OpcionesSintetico{Features: []string{"pm10", "radiacion"}, Nivel: NivelAbsoluto,
		Estaciones: 2, Horas: 10, Ruido: 0.1, Semilla: 1})
	if err == nil {
		t.Fatal("radiacion no tiene valor plantado: se esperaba error")
	}
}
