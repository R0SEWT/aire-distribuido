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

func TestEntrenamientosCicloCompleto(t *testing.T) {
	e := NuevosEntrenamientos()
	t0 := time.Date(2026, 11, 19, 15, 30, 0, 0, time.UTC)
	if err := e.Crear(ctx, almacen.Entrenamiento{ID: "e-1", Inicio: t0, Estado: almacen.EstadoEnCurso}); err != nil {
		t.Fatal(err)
	}
	if err := e.Crear(ctx, almacen.Entrenamiento{ID: "e-1"}); err == nil {
		t.Fatal("Crear aceptó un id repetido")
	}
	if err := e.Crear(ctx, almacen.Entrenamiento{ID: "e-2", Inicio: t0.Add(time.Minute), Estado: almacen.EstadoEnCurso}); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 3; i++ {
		if err := e.AgregarRonda(ctx, "e-1", almacen.Ronda{Iter: i, Inercia: 100 / float64(i)}); err != nil {
			t.Fatal(err)
		}
	}
	paro, dif, ms := "tolerancia", 0.0, 31.2
	if err := e.Terminar(ctx, "e-1", almacen.Cierre{Fin: t0.Add(time.Second), Estado: almacen.EstadoTerminado,
		Iteraciones: 3, Paro: &paro, DifSecuencial: &dif, MsTotal: &ms}); err != nil {
		t.Fatal(err)
	}
	en, err := e.Obtener(ctx, "e-1")
	if err != nil || en.Estado != almacen.EstadoTerminado || len(en.Rondas) != 3 || en.Rondas[2].Iter != 3 ||
		en.Iteraciones != 3 || en.MsTotal == nil || *en.MsTotal != 31.2 || en.Fin == nil || en.Error != nil {
		t.Fatalf("Obtener(e-1) = %+v, %v", en, err)
	}
	lista, err := e.Listar(ctx)
	if err != nil || len(lista) != 2 || lista[0].ID != "e-2" || lista[1].ID != "e-1" || lista[1].Rondas != nil {
		t.Fatalf("Listar = %+v, %v; se esperaba e-2 y e-1, sin rondas", lista, err)
	}
	for nombre, err := range map[string]error{
		"AgregarRonda": e.AgregarRonda(ctx, "e-x", almacen.Ronda{}),
		"Terminar":     e.Terminar(ctx, "e-x", almacen.Cierre{}),
	} {
		if !errors.Is(err, almacen.ErrNoEncontrado) {
			t.Errorf("%s de un entrenamiento que no existe = %v, se esperaba ErrNoEncontrado", nombre, err)
		}
	}
	if _, err := e.Obtener(ctx, "e-x"); !errors.Is(err, almacen.ErrNoEncontrado) {
		t.Errorf("Obtener(e-x) = %v", err)
	}
}

func TestEventosRecientes(t *testing.T) {
	e := NuevosEventos()
	t0 := time.Date(2026, 11, 19, 15, 0, 0, 0, time.UTC)
	for i, tipo := range []string{almacen.EventoAlta, almacen.EventoError, almacen.EventoBaja} {
		if err := e.Registrar(ctx, almacen.EventoCluster{TS: t0.Add(time.Duration(i) * time.Second), Tipo: tipo, Nodo: "n1"}); err != nil {
			t.Fatal(err)
		}
	}
	rec, err := e.Recientes(ctx, 2)
	if err != nil || len(rec) != 2 || rec[0].Tipo != almacen.EventoBaja || rec[1].Tipo != almacen.EventoError {
		t.Fatalf("Recientes(2) = %+v, %v", rec, err)
	}
	if rec, _ := e.Recientes(ctx, 10); len(rec) != 3 {
		t.Fatalf("Recientes(10) devolvió %d", len(rec))
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
