package memoria

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/R0SEWT/aire-distribuido/almacen"
)

var ctx = context.Background()

func TestEstacionesListaOrdenadaYObtiene(t *testing.T) {
	es, err := NuevasEstaciones([]almacen.Estacion{{ID: "CA-B"}, {ID: "CA-A", Lat: -9}})
	if err != nil {
		t.Fatal(err)
	}
	lista, err := es.Listar(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(lista) != 2 || lista[0].ID != "CA-A" || lista[1].ID != "CA-B" {
		t.Fatalf("Listar = %+v, se esperaba CA-A y CA-B en ese orden", lista)
	}
	// La lista es una copia: modificarla no cambia el almacén.
	lista[0].ID = "otra"
	if e, err := es.Obtener(ctx, "CA-A"); err != nil || e.Lat != -9 {
		t.Fatalf("Obtener(CA-A) = %+v, %v", e, err)
	}
	if _, err := es.Obtener(ctx, "CA-X"); !errors.Is(err, almacen.ErrNoEncontrado) {
		t.Fatalf("Obtener(CA-X) = %v, se esperaba ErrNoEncontrado", err)
	}
}

func TestEstacionesVaciasYRepetidas(t *testing.T) {
	es, err := NuevasEstaciones(nil)
	if err != nil {
		t.Fatal(err)
	}
	if lista, err := es.Listar(ctx); err != nil || lista == nil || len(lista) != 0 {
		t.Fatalf("Listar sin estaciones = %#v, %v; se esperaba una lista vacía, no nil", lista, err)
	}
	if _, err := NuevasEstaciones([]almacen.Estacion{{ID: "CA-A"}, {ID: "CA-A"}}); err == nil {
		t.Fatal("aceptó una estación repetida")
	}
}

func TestCacheGuardaElRegimenMasReciente(t *testing.T) {
	c := NuevaCache()
	if _, err := c.Regimen(ctx, "CA-A"); !errors.Is(err, almacen.ErrNoEncontrado) {
		t.Fatalf("Regimen sin datos = %v, se esperaba ErrNoEncontrado", err)
	}
	t0 := time.Date(2026, 11, 14, 15, 0, 0, 0, time.UTC)
	guardar := func(r almacen.RegimenEstacion) {
		t.Helper()
		if err := c.GuardarRegimen(ctx, "CA-A", r); err != nil {
			t.Fatal(err)
		}
	}
	leer := func() almacen.RegimenEstacion {
		t.Helper()
		r, err := c.Regimen(ctx, "CA-A")
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	guardar(almacen.RegimenEstacion{FechaHora: t0, Regimen: 0})
	guardar(almacen.RegimenEstacion{FechaHora: t0.Add(time.Hour), Regimen: 1, ExcedeECA: true, Alerta: true})
	if r := leer(); r.Regimen != 1 || !r.Alerta || !r.FechaHora.Equal(t0.Add(time.Hour)) {
		t.Fatalf("Regimen = %+v; se esperaba el de la hora más reciente", r)
	}
	// Una lectura de cinco días antes no lo reemplaza.
	guardar(almacen.RegimenEstacion{FechaHora: t0.Add(-5 * 24 * time.Hour), Regimen: 2})
	if r := leer(); r.Regimen != 1 || !r.Alerta {
		t.Fatalf("una lectura más antigua reemplazó el régimen: %+v", r)
	}
	// Una de la misma hora sí (se reenvió corregida).
	guardar(almacen.RegimenEstacion{FechaHora: t0.Add(time.Hour), Regimen: 0})
	if r := leer(); r.Regimen != 0 || r.Alerta {
		t.Fatalf("la misma hora tiene que reemplazar: %+v", r)
	}
}

// Regimenes trae de una vez las estaciones pedidas que tienen régimen; las
// que no, no están en el mapa.
func TestCacheRegimenesDeVariasEstaciones(t *testing.T) {
	c := NuevaCache()
	if rs, err := c.Regimenes(ctx, []string{"CA-A", "CA-B"}); err != nil || rs == nil || len(rs) != 0 {
		t.Fatalf("Regimenes sin datos = %v, %v; se esperaba un mapa vacío", rs, err)
	}
	t0 := time.Date(2026, 11, 14, 15, 0, 0, 0, time.UTC)
	if _, err := c.GuardarRegimenes(ctx, map[string]almacen.RegimenEstacion{
		"CA-A": {FechaHora: t0, Regimen: 1}, "CA-B": {FechaHora: t0, Regimen: 2}, "CA-C": {FechaHora: t0, Regimen: 0},
	}); err != nil {
		t.Fatal(err)
	}
	rs, err := c.Regimenes(ctx, []string{"CA-A", "CA-C", "CA-Z"})
	if err != nil || len(rs) != 2 || rs["CA-A"].Regimen != 1 || rs["CA-C"].Regimen != 0 {
		t.Fatalf("Regimenes = %+v, %v; se esperaban CA-A y CA-C", rs, err)
	}
	if rs, err := c.Regimenes(ctx, nil); err != nil || len(rs) != 0 {
		t.Fatalf("Regimenes sin estaciones = %v, %v", rs, err)
	}
}

func TestPrediccionesGuarda(t *testing.T) {
	p := NuevasPredicciones()
	for i := range 3 {
		if err := p.Guardar(ctx, almacen.Prediccion{Estacion: fmt.Sprint("CA-", i), Regimen: i}); err != nil {
			t.Fatal(err)
		}
	}
	todas := p.Todas()
	if len(todas) != 3 || todas[2].Regimen != 2 {
		t.Fatalf("Todas = %+v", todas)
	}
}

// Con -race: el doble se usa desde los handlers y desde la goroutine que
// escribe las predicciones a la vez.
func TestDobleConcurrente(t *testing.T) {
	es, _ := NuevasEstaciones([]almacen.Estacion{{ID: "CA-A"}})
	c, p := NuevaCache(), NuevasPredicciones()
	var wg sync.WaitGroup
	for g := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 200 {
				_ = c.GuardarRegimen(ctx, "CA-A", almacen.RegimenEstacion{FechaHora: time.Unix(int64(i)*3600, 0), Regimen: g + i})
				_, _ = c.Regimen(ctx, "CA-A")
				_ = p.Guardar(ctx, almacen.Prediccion{Regimen: i})
				_, _ = es.Listar(ctx)
				_, _ = es.Obtener(ctx, "CA-A")
				_ = p.Todas()
			}
		}()
	}
	wg.Wait()
	if n := len(p.Todas()); n != 8*200 {
		t.Fatalf("%d predicciones guardadas, se esperaban %d", n, 8*200)
	}
}

func TestPing(t *testing.T) {
	var _ almacen.Pinger = NuevaCache()
	var _ almacen.Pinger = NuevasPredicciones()
	es, _ := NuevasEstaciones(nil)
	if err := es.Ping(ctx); err != nil {
		t.Fatal(err)
	}
}
