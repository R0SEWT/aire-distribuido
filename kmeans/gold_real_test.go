package kmeans

import (
	"math"
	"os"
	"testing"
	"time"
)

const (
	rutaGoldReal            = "../data/gold/oefa_features.csv"
	rutaEstandarizacionReal = "../reports/estandarizacion.json"
	rutaReceta              = "../configs/kmeans.json"
)

// goldReal carga el gold de verdad con la receta del repo. Se salta con -short
// y si el archivo no está: el CI no lo tiene y el cargador puede no haberlo
// producido todavía.
func goldReal(t *testing.T) (*Gold, Receta) {
	t.Helper()
	if testing.Short() {
		t.Skip("-short: no se lee el gold completo")
	}
	if _, err := os.Stat(rutaGoldReal); err != nil {
		t.Skipf("no está el gold en %s: %v", rutaGoldReal, err)
	}
	r, err := LeerReceta(rutaReceta)
	if err != nil {
		t.Fatalf("LeerReceta: %v", err)
	}
	inicio := time.Now()
	g, err := LeerGold(rutaGoldReal, r.Features)
	if err != nil {
		t.Fatalf("LeerGold: %v", err)
	}
	t.Logf("gold leído en %s: %d filas × %d features, %d estaciones, %.1f MiB en float64",
		time.Since(inicio).Round(time.Millisecond), g.Datos.N, g.Datos.D, len(g.Estaciones()),
		float64(len(g.Datos.X)*8)/(1<<20))
	return g, r
}

func TestGoldRealCumpleElContrato(t *testing.T) {
	g, r := goldReal(t)
	if len(g.Datos.X) != g.Datos.N*g.Datos.D || len(g.Estacion) != g.Datos.N || len(g.FechaHora) != g.Datos.N {
		t.Fatalf("tamaños inconsistentes: N=%d, len(X)=%d, estaciones %d, fechas %d",
			g.Datos.N, len(g.Datos.X), len(g.Estacion), len(g.FechaHora))
	}
	if _, err := os.Stat(rutaEstandarizacionReal); err != nil {
		t.Skipf("no está %s: %v", rutaEstandarizacionReal, err)
	}
	e, err := LeerEstandarizacion(rutaEstandarizacionReal)
	if err != nil {
		t.Fatalf("LeerEstandarizacion: %v", err)
	}
	if err := e.Coincide(r); err != nil {
		t.Fatalf("la estandarización no es de la receta: %v", err)
	}
	if e.N != g.Datos.N {
		t.Errorf("la estandarización dice n = %d y el gold tiene %d filas", e.N, g.Datos.N)
	}
}

// TestPilotoSobreElGoldReal es un piloto, NO el benchmark: una sola corrida
// por configuración. Comprueba sobre los datos de verdad lo que los tests de
// juguete comprueban en chico (W = 1 y W = 8 idénticos bit a bit, secuencial
// equivalente con tolerancia) y dice cuánto cuesta una iteración.
func TestPilotoSobreElGoldReal(t *testing.T) {
	g, r := goldReal(t)
	d := g.Datos
	cent, kEf, err := KMeansPP(d, r.K, r.Semilla)
	if err != nil {
		t.Fatalf("KMeansPP: %v", err)
	}
	op := r.OpcionesConc()
	op.K = kEf

	inicio := time.Now()
	sec, err := Secuencial(d, cent, op.Opciones)
	if err != nil {
		t.Fatalf("Secuencial: %v", err)
	}
	tSec := time.Since(inicio)
	t.Logf("secuencial: %s, %d iteraciones (%s), inercia %.6g",
		tSec.Round(time.Millisecond), sec.Iteraciones, sec.Paro, sec.Inercia)

	var uno *Resultado
	for _, w := range []int{1, 8} {
		op.Workers = w
		inicio := time.Now()
		con, err := Concurrente(d, cent, op)
		if err != nil {
			t.Fatalf("Concurrente W=%d: %v", w, err)
		}
		tCon := time.Since(inicio)
		t.Logf("W=%d: %s · speedup %.2fx", w, tCon.Round(time.Millisecond), tSec.Seconds()/tCon.Seconds())
		if rel := math.Abs(con.Inercia-sec.Inercia) / math.Abs(sec.Inercia); rel > 1e-9 {
			t.Errorf("W=%d: la inercia difiere del secuencial en %.3g relativo", w, rel)
		}
		if uno == nil {
			uno = con
			continue
		}
		mismoResultado(t, "W=8 contra W=1", con, uno)
	}
}
