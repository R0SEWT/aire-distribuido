package modelo

import (
	"encoding/csv"
	"errors"
	"io"
	"os"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/R0SEWT/aire-distribuido/kmeans"
)

const (
	rutaGold         = "../data/gold/oefa_features.csv"
	rutaAsignaciones = "../data/gold/asignaciones.csv"
)

// La predicción de la API, sobre cada fila del gold, tiene que dar el régimen
// que le da kmeans con los centroides del modelo, y el que dejó el
// entrenamiento en data/gold/asignaciones.csv. Cada fila se lleva a unidades
// originales (µg/m³, como llega una lectura a la API) y Predecir la vuelve a
// estandarizar: la prueba cubre la receta, la estandarización y el contrato
// del centroide más cercano. Se salta con -short o sin el gold (no se versiona).
func TestPrediccionCoincideConKmeansSobreElGold(t *testing.T) {
	if testing.Short() {
		t.Skip("-short: no se lee el gold completo")
	}
	if _, err := os.Stat(rutaGold); err != nil {
		t.Skipf("no está el gold en %s: %v", rutaGold, err)
	}
	m := sembrarDelRepo(t)
	if !slices.Equal(m.Features(), []string{"pm2_5", "pm10"}) {
		t.Skipf("la prueba arma la lectura con pm2_5 y pm10; el modelo usa %v", m.Features())
	}
	est, err := kmeans.LeerEstandarizacion(reportsDelRepo.Estandarizacion)
	if err != nil {
		t.Fatal(err)
	}
	g, err := kmeans.LeerGold(rutaGold, m.Features())
	if err != nil {
		t.Fatal(err)
	}
	n := g.Datos.N
	if n != est.N {
		t.Fatalf("el gold tiene %d filas y la estandarización dice %d: no es el gold del modelo", n, est.N)
	}

	// El régimen de cada fila según kmeans, con los centroides del modelo.
	conKmeans := make([]int, n)
	kmeans.Asignar(g.Datos, m.CentroidesEstandarizados(), 0, n, conKmeans)

	delEntrenamiento := leerAsignaciones(t, g)

	x := make([]float64, g.Datos.D)
	distintas := 0
	inicio := time.Now()
	for i := 0; i < n; i++ {
		if err := est.Invertir(g.Datos.Punto(i), g.Estacion[i], x); err != nil {
			t.Fatal(err)
		}
		r, err := m.Predecir(Lectura{Estacion: g.Estacion[i], PM25: x[0], PM10: x[1]})
		if err != nil {
			t.Fatalf("fila %d (%s %s): %v", i, g.Estacion[i], g.FechaHora[i].Format(time.RFC3339), err)
		}
		entrenamiento := conKmeans[i]
		if delEntrenamiento != nil {
			entrenamiento = delEntrenamiento[i]
		}
		if r.Regimen != conKmeans[i] || r.Regimen != entrenamiento {
			distintas++
			if distintas <= 5 {
				t.Errorf("fila %d (%s %s, pm2_5 = %v, pm10 = %v): Predecir da %d, kmeans %d, el entrenamiento %d",
					i, g.Estacion[i], g.FechaHora[i].Format(time.RFC3339), x[0], x[1], r.Regimen, conKmeans[i], entrenamiento)
			}
		}
	}
	if distintas > 0 {
		t.Fatalf("%d de %d filas con otro régimen", distintas, n)
	}
	t.Logf("%d filas del gold: Predecir coincide con kmeans.Asignar y con asignaciones.csv (%s en total, %.2f µs por lectura)",
		n, time.Since(inicio).Round(time.Millisecond), float64(time.Since(inicio).Microseconds())/float64(n))
}

// leerAsignaciones lee data/gold/asignaciones.csv (estacion,fecha_hora,regimen,
// en el orden del gold). Si no está, devuelve nil y la prueba compara solo con
// kmeans.
func leerAsignaciones(t *testing.T, g *kmeans.Gold) []int {
	t.Helper()
	f, err := os.Open(rutaAsignaciones)
	if err != nil {
		t.Logf("sin %s: se compara solo con kmeans.Asignar", rutaAsignaciones)
		return nil
	}
	defer f.Close()
	cr := csv.NewReader(f)
	if cab, err := cr.Read(); err != nil || !slices.Equal(cab, []string{"estacion", "fecha_hora", "regimen"}) {
		t.Fatalf("%s: cabecera %v, %v", rutaAsignaciones, cab, err)
	}
	asign := make([]int, 0, g.Datos.N)
	for i := 0; ; i++ {
		fila, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if i >= g.Datos.N {
			t.Fatalf("%s tiene más filas que el gold", rutaAsignaciones)
		}
		fh, err := time.Parse(time.RFC3339, fila[1])
		if err != nil || fila[0] != g.Estacion[i] || !fh.Equal(g.FechaHora[i]) {
			t.Fatalf("%s, fila %d: %s %s no es la fila del gold (%s %s)", rutaAsignaciones, i+1, fila[0], fila[1],
				g.Estacion[i], g.FechaHora[i].Format(time.RFC3339))
		}
		j, err := strconv.Atoi(fila[2])
		if err != nil {
			t.Fatal(err)
		}
		asign = append(asign, j)
	}
	if len(asign) != g.Datos.N {
		t.Fatalf("%s tiene %d filas y el gold %d", rutaAsignaciones, len(asign), g.Datos.N)
	}
	return asign
}
