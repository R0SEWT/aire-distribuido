package api

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/R0SEWT/aire-distribuido/almacen"
	"github.com/R0SEWT/aire-distribuido/almacen/memoria"
	"github.com/R0SEWT/aire-distribuido/modelo"
)

// Un entrenamiento que termina bien deja en los almacenes lo que hace falta
// para no perderlo al reiniciar: el modelo que activó, en la colección modelos
// y en modelo:activo, con su procedencia; y el punto de control de la corrida.
func TestUnEntrenamientoDejaSuModeloYSuPuntoDeControlEnLosAlmacenes(t *testing.T) {
	ctx := context.Background()
	modelos, cache := memoria.NuevosModelos(), memoria.NuevaCache()
	ec := nuevoEntornoCluster(t, func(c *Config) { c.Modelos, c.CacheModelo, c.Puntos = modelos, cache, cache })
	ec.lanzarNodos(t, 3)
	id := ec.entrenar(t, `{"nodos": 3, "replicas": 1, "semilla": 42}`)
	det := ec.esperarFin(t, id)
	if det["estado"] != "terminado" || det["modelo_id"] != "entrenado-"+id {
		t.Fatalf("detalle = %v", det)
	}
	activo := ec.srv.modelo.Load()

	// El modelo, en las dos bases, activo y con su procedencia.
	enMongo, err1 := modelos.Activo(ctx)
	enRedis, err2 := cache.ModeloActivo(ctx)
	if err1 != nil || err2 != nil {
		t.Fatalf("modelos activo: %v; modelo:activo: %v", err1, err2)
	}
	for donde, doc := range map[string]almacen.Modelo{"modelos": enMongo, "modelo:activo": enRedis} {
		if doc.ID != "entrenado-"+id || !doc.Activo || doc.Origen != id || doc.GoldSHA256 != ec.ent.Sha256 || doc.Creado.IsZero() {
			t.Errorf("%s = id %q, activo %v, origen %q, gold %q, creado %v; se esperaba el modelo del entrenamiento %s con su procedencia",
				donde, doc.ID, doc.Activo, doc.Origen, doc.GoldSHA256, doc.Creado, id)
		}
	}
	// Lo guardado alcanza para volver a armar el mismo modelo.
	recuperado, err := modelo.DesdeDocumento(enRedis)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(recuperado.CentroidesEstandarizados(), activo.CentroidesEstandarizados()) || !slices.Equal(recuperado.Regimenes, activo.Regimenes) {
		t.Fatal("el modelo guardado no es el que quedó activo")
	}

	// El punto de control: los centroides de la última ronda completa.
	punto, err := cache.Punto(ctx, id)
	if err != nil {
		t.Fatalf("entrenamiento:%s:punto: %v", id, err)
	}
	if punto.Iter != int(det["iteraciones"].(float64)) || len(punto.Centroides) != 3 {
		t.Fatalf("punto = iter %d con %d centroides; el entrenamiento hizo %v iteraciones con k = 3", punto.Iter, len(punto.Centroides), det["iteraciones"])
	}
	if !slices.Equal(slices.Concat(punto.Centroides...), activo.CentroidesEstandarizados()) {
		t.Fatalf("los centroides del punto de control no son los del modelo: %v", punto.Centroides)
	}

	// Una medición con réplicas no activa modelo, pero deja su punto de control.
	ec.esperarLibres(t, 3)
	id2 := ec.entrenar(t, `{"nodos": 2, "replicas": 2, "semilla": 42}`)
	if det := ec.esperarFin(t, id2); det["estado"] != "terminado" {
		t.Fatalf("medición = %v", det)
	}
	if p, err := cache.Punto(ctx, id2); err != nil || p.Iter == 0 || len(p.Centroides) != 3 {
		t.Fatalf("punto de la medición = %+v, %v", p, err)
	}
	if a, _ := modelos.Activo(ctx); a.ID != "entrenado-"+id {
		t.Fatalf("la medición cambió el modelo activo a %s", a.ID)
	}
}

// Una corrida abortada no tiene centroides que publicar: no deja punto de
// control ni toca el modelo guardado.
func TestUnEntrenamientoAbortadoNoDejaPuntoDeControl(t *testing.T) {
	ctx := context.Background()
	modelos, cache := memoria.NuevosModelos(), memoria.NuevaCache()
	ec := nuevoEntornoCluster(t, func(c *Config) { c.Modelos, c.CacheModelo, c.Puntos = modelos, cache, cache })
	ec.ent.PlazoCarga = 300 * time.Millisecond
	ec.lanzarNodos(t, 1)
	ec.nodoFalso(t, "nodo-roto", false, 0) // nunca confirma el CARGAR
	ec.esperarLibres(t, 2)
	id := ec.entrenar(t, `{"nodos": 2, "replicas": 1, "semilla": 42}`)
	if det := ec.esperarFin(t, id); det["estado"] != "abortado" {
		t.Fatalf("detalle = %v", det)
	}
	if _, err := cache.Punto(ctx, id); !errors.Is(err, almacen.ErrNoEncontrado) {
		t.Fatalf("punto de una corrida abortada: %v", err)
	}
	if _, err := modelos.Activo(ctx); !errors.Is(err, almacen.ErrNoEncontrado) {
		t.Fatalf("una corrida abortada guardó un modelo: %v", err)
	}
}
