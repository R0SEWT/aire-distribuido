package memoria

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/R0SEWT/aire-distribuido/almacen"
)

func TestEntrenamientosCicloDeVida(t *testing.T) {
	ctx := context.Background()
	s := NuevosEntrenamientos()
	t0 := time.Date(2026, 11, 19, 15, 30, 0, 0, time.UTC)
	nuevo := func(id string, inicio time.Time) almacen.Entrenamiento {
		return almacen.Entrenamiento{ID: id, Inicio: inicio, Estado: almacen.EstadoEnCurso,
			Parametros: almacen.ParametrosEntrenamiento{Nodos: 2, Replicas: 1, Semilla: 42},
			Nodos:      []almacen.NodoEntrenamiento{{Nombre: "a", Hasta: 5}, {Nombre: "b", Desde: 5, Hasta: 10}}}
	}
	if err := s.Crear(ctx, nuevo("e-1", t0)); err != nil {
		t.Fatal(err)
	}
	if err := s.Crear(ctx, nuevo("e-1", t0)); !errors.Is(err, almacen.ErrYaExiste) {
		t.Fatalf("crear dos veces el mismo id = %v", err)
	}
	if err := s.Crear(ctx, nuevo("e-2", t0.Add(time.Minute))); err != nil {
		t.Fatal(err)
	}

	for i := 1; i <= 3; i++ {
		r := almacen.Ronda{Iter: i, Inercia: 10 / float64(i), MsPorNodo: map[string]float64{"a": 1, "b": 2}}
		if err := s.AgregarRonda(ctx, "e-1", r); err != nil {
			t.Fatal(err)
		}
	}
	fin := t0.Add(time.Second)
	cierre := almacen.CierreEntrenamiento{Fin: fin, Estado: almacen.EstadoTerminado, Iteraciones: 3, Paro: "tolerancia",
		DifSecuencial: 1e-16, MsTotal: 12.5, ModeloID: "entrenado-e-1"}
	if err := s.Cerrar(ctx, "e-1", cierre); err != nil {
		t.Fatal(err)
	}

	e, err := s.Obtener(ctx, "e-1")
	if err != nil {
		t.Fatal(err)
	}
	if e.Estado != almacen.EstadoTerminado || !e.Fin.Equal(fin) || e.Iteraciones != 3 || e.Paro != "tolerancia" ||
		e.DifSecuencial != 1e-16 || e.MsTotal != 12.5 || e.ModeloID != "entrenado-e-1" || e.Parametros.Semilla != 42 {
		t.Fatalf("cerrado = %+v", e)
	}
	if len(e.Rondas) != 3 || e.Rondas[2].Iter != 3 || len(e.Nodos) != 2 {
		t.Fatalf("rondas = %+v, nodos = %+v", e.Rondas, e.Nodos)
	}
	// Lo que se devuelve es una copia: cambiarla no toca el almacén.
	e.Rondas[0].Iter, e.Nodos[0].Nombre = 99, "otro"
	if otra, _ := s.Obtener(ctx, "e-1"); otra.Rondas[0].Iter != 1 || otra.Nodos[0].Nombre != "a" {
		t.Fatal("Obtener entregó la memoria del almacén")
	}

	lista, err := s.Listar(ctx)
	if err != nil || len(lista) != 2 || lista[0].ID != "e-2" || lista[1].ID != "e-1" {
		t.Fatalf("Listar = %+v, %v; se esperaba del más reciente al más antiguo", lista, err)
	}
	if lista[1].Rondas != nil || lista[1].Iteraciones != 3 || len(lista[1].Nodos) != 2 {
		t.Fatalf("Listar no trae rondas pero sí el resumen: %+v", lista[1])
	}

	for nombre, err := range map[string]error{
		"AgregarRonda": s.AgregarRonda(ctx, "no-existe", almacen.Ronda{Iter: 1}),
		"Cerrar":       s.Cerrar(ctx, "no-existe", cierre),
	} {
		if !errors.Is(err, almacen.ErrNoEncontrado) {
			t.Errorf("%s de un id que no existe = %v", nombre, err)
		}
	}
	if _, err := s.Obtener(ctx, "no-existe"); !errors.Is(err, almacen.ErrNoEncontrado) {
		t.Errorf("Obtener de un id que no existe = %v", err)
	}
	if vacia, err := NuevosEntrenamientos().Listar(ctx); err != nil || vacia == nil || len(vacia) != 0 {
		t.Errorf("la lista vacía es válida y no es nil: %v, %v", vacia, err)
	}
}

// Las rondas las escribe una goroutine mientras los handlers leen.
func TestEntrenamientosYEventosConcurrentes(t *testing.T) {
	ctx := context.Background()
	s, ev := NuevosEntrenamientos(), NuevosEventos()
	if err := s.Crear(ctx, almacen.Entrenamiento{ID: "e-1", Estado: almacen.EstadoEnCurso}); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Go(func() {
		for i := 1; i <= 200; i++ {
			s.AgregarRonda(ctx, "e-1", almacen.Ronda{Iter: i})
			ev.Guardar(ctx, almacen.EventoCluster{Tipo: "alta", Nodo: "a"})
		}
	})
	for range 4 {
		wg.Go(func() {
			for range 200 {
				e, _ := s.Obtener(ctx, "e-1")
				for i, r := range e.Rondas {
					if r.Iter != i+1 {
						t.Errorf("ronda %d en la posición %d", r.Iter, i)
						return
					}
				}
				s.Listar(ctx)
				ev.Todos()
			}
		})
	}
	wg.Wait()
	if e, _ := s.Obtener(ctx, "e-1"); len(e.Rondas) != 200 || len(ev.Todos()) != 200 {
		t.Fatalf("%d rondas y %d eventos", len(e.Rondas), len(ev.Todos()))
	}
}
