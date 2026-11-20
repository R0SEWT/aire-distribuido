package memoria

import (
	"errors"
	"testing"
	"time"

	"github.com/R0SEWT/aire-distribuido/almacen"
)

func TestModelosALoSumoUnoActivo(t *testing.T) {
	m := NuevosModelos()
	if _, err := m.Activo(ctx); !errors.Is(err, almacen.ErrNoEncontrado) {
		t.Fatalf("Activo sin modelos = %v, se esperaba ErrNoEncontrado", err)
	}
	for _, mod := range []almacen.Modelo{
		{ID: "sembrado-kmeans-json", Origen: almacen.OrigenSembrado, Activo: true},
		{ID: "m-e-1", Origen: "e-1", Activo: true},
		{ID: "m-e-2", Origen: "e-2"}, // una medición con réplicas no se activa
	} {
		if err := m.Guardar(ctx, mod); err != nil {
			t.Fatal(err)
		}
	}
	if a, err := m.Activo(ctx); err != nil || a.ID != "m-e-1" {
		t.Fatalf("Activo = %+v, %v; se esperaba m-e-1", a, err)
	}
}

func TestCacheModeloYPunto(t *testing.T) {
	c := NuevaCache()
	if _, err := c.ModeloActivo(ctx); !errors.Is(err, almacen.ErrNoEncontrado) {
		t.Fatalf("ModeloActivo sin datos = %v", err)
	}
	if _, err := c.Punto(ctx, "e-1"); !errors.Is(err, almacen.ErrNoEncontrado) {
		t.Fatalf("Punto sin datos = %v", err)
	}
	if err := c.GuardarModeloActivo(ctx, almacen.Modelo{ID: "m-1"}); err != nil {
		t.Fatal(err)
	}
	if m, err := c.ModeloActivo(ctx); err != nil || m.ID != "m-1" {
		t.Fatalf("ModeloActivo = %+v, %v", m, err)
	}
	if err := c.GuardarPunto(ctx, "e-1", almacen.PuntoControl{Iter: 7, Centroides: [][]float64{{1, 2}}}); err != nil {
		t.Fatal(err)
	}
	if p, err := c.Punto(ctx, "e-1"); err != nil || p.Iter != 7 {
		t.Fatalf("Punto = %+v, %v", p, err)
	}
}

// Un lote con varias estaciones: cada una sigue la regla por separado.
func TestCacheGuardaRegimenesPorLote(t *testing.T) {
	c := NuevaCache()
	t0 := time.Date(2026, 11, 14, 15, 0, 0, 0, time.UTC)
	if err := c.GuardarRegimenes(ctx, map[string]almacen.RegimenEstacion{
		"CA-A": {FechaHora: t0, Regimen: 1}, "CA-B": {FechaHora: t0, Regimen: 2},
	}); err != nil {
		t.Fatal(err)
	}
	if err := c.GuardarRegimenes(ctx, map[string]almacen.RegimenEstacion{
		"CA-A": {FechaHora: t0.Add(-time.Hour), Regimen: 0}, "CA-B": {FechaHora: t0.Add(time.Hour), Regimen: 0},
	}); err != nil {
		t.Fatal(err)
	}
	a, _ := c.Regimen(ctx, "CA-A")
	b, _ := c.Regimen(ctx, "CA-B")
	if a.Regimen != 1 || b.Regimen != 0 {
		t.Fatalf("CA-A = %d (se esperaba 1: la lectura más antigua no reemplaza), CA-B = %d (se esperaba 0)", a.Regimen, b.Regimen)
	}
}
