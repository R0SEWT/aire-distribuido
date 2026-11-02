package kmeans_test

// Pruebas del contrato con el cargador usando el código del cargador mismo
// (datos.Dataset.EscribirGold y datos.ReporteEstandarizacion), no una
// imitación: lo que el cargador escribe, el K-Means tiene que leerlo.
// Es un paquete de test externo porque datos importa kmeans.

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/R0SEWT/aire-distribuido/datos"
	"github.com/R0SEWT/aire-distribuido/kmeans"
)

func TestElGoldDelCargadorHaceIdaYVueltaConEstacionesRaras(t *testing.T) {
	lima := time.FixedZone("", -5*3600)
	t0 := time.Date(2024, 1, 1, 1, 0, 0, 0, lima).Unix()
	ds := &datos.Dataset{
		Features:   []string{"pm10", "humedad"},
		N:          4,
		D:          2,
		X:          []float64{0.5, -1.25, 1e-17, 3, -0.1, 0, 2.75, 1.5},
		Estaciones: []string{"A,B", `dijo "hola"`, "CA-LI-01"},
		Estacion:   []uint16{0, 0, 1, 2},
		Hora:       []int64{t0, t0 + 3600, t0, t0},
		Zona:       lima,
	}
	var buf bytes.Buffer
	if err := ds.EscribirGold(&buf); err != nil {
		t.Fatal(err)
	}
	g, err := kmeans.LeerGoldDe(&buf, ds.Features)
	if err != nil {
		t.Fatalf("el gold del cargador no se lee:\n%s\nerror: %v", buf.String(), err)
	}
	if want := []string{"A,B", "A,B", `dijo "hola"`, "CA-LI-01"}; !slices.Equal(g.Estacion, want) {
		t.Errorf("estaciones %q, se esperaban %q", g.Estacion, want)
	}
	for i, v := range ds.X {
		if math.Float64bits(g.Datos.X[i]) != math.Float64bits(v) {
			t.Errorf("X[%d] = %v, se esperaba %v", i, g.Datos.X[i], v)
		}
	}
	for i, h := range ds.Hora {
		if g.FechaHora[i].Unix() != h {
			t.Errorf("fila %d: %v, se esperaba %v", i, g.FechaHora[i], time.Unix(h, 0).In(lima))
		}
	}
}

func TestLaEstandarizacionDelCargadorConDesviacionCeroSeLee(t *testing.T) {
	// El cargador escribe desviación 0 y z = 0 para una feature constante, en
	// todo el gold o dentro de una estación (datos.TestDesviacionCeroDaZCero).
	casos := map[string]datos.ReporteEstandarizacion{
		"absoluto": {Features: []string{"humedad", "pm10"}, Log1p: []string{"pm10"}, Nivel: datos.NivelAbsoluto,
			N: 3, Media: []float64{50, 2.5}, Desv: []float64{0, 0.4}},
		"estacion": {Features: []string{"humedad", "pm10"}, Log1p: []string{"pm10"}, Nivel: datos.NivelEstacion,
			N: 3, Media: []float64{50, 2.5}, Desv: []float64{1, 0.4},
			PorEstacion: map[string]datos.EstadisticaEstacion{
				"CA-LI-01": {N: 3, Media: []float64{50, 2.5}, Desv: []float64{0, 0.4}},
			}},
	}
	for nombre, rep := range casos {
		t.Run(nombre, func(t *testing.T) {
			b, err := json.Marshal(rep)
			if err != nil {
				t.Fatal(err)
			}
			ruta := filepath.Join(t.TempDir(), "estandarizacion.json")
			if err := os.WriteFile(ruta, b, 0o600); err != nil {
				t.Fatal(err)
			}
			e, err := kmeans.LeerEstandarizacion(ruta)
			if err != nil {
				t.Fatalf("una feature constante no puede impedir leer la estandarización: %v", err)
			}
			x := make([]float64, 2)
			if err := e.Invertir([]float64{0, 0}, "CA-LI-01", x); err != nil {
				t.Fatal(err)
			}
			if x[0] != 50 {
				t.Errorf("la humedad constante vuelve a %v; con desviación 0 el valor original es la media, 50", x[0])
			}
		})
	}
}
