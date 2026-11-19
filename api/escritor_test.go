package api

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/R0SEWT/aire-distribuido/almacen"
	"github.com/R0SEWT/aire-distribuido/almacen/memoria"
)

var logMudo = slog.New(slog.NewTextHandler(io.Discard, nil))

func TestEscritorGuardaYActualizaElRegimen(t *testing.T) {
	pred, cache := memoria.NuevasPredicciones(), memoria.NuevaCache()
	e := nuevoEscritor(8, pred, cache, logMudo)
	ctx, cancel := context.WithCancel(context.Background())
	listo := make(chan struct{})
	go func() { e.correr(ctx); close(listo) }()

	ts := time.Date(2026, 11, 19, 15, 0, 0, 0, time.UTC)
	for i, p := range []almacen.Prediccion{
		{TS: ts, Estacion: "CA-A", Regimen: 0},
		{TS: ts.Add(time.Second), Estacion: "CA-A", Regimen: 1, ExcedeECA: true, Alerta: true},
		{TS: ts, Estacion: "CA-B", Regimen: 2},
	} {
		if !e.encolar(p) {
			t.Fatalf("encolar %d: la cola no debería estar llena", i)
		}
	}
	cancel() // correr escribe lo que quedó en la cola antes de volver
	<-listo
	if n := len(pred.Todas()); n != 3 {
		t.Fatalf("%d predicciones guardadas, se esperaban 3", n)
	}
	r, err := cache.Regimen(context.Background(), "CA-A")
	if err != nil || r.Regimen != 1 || !r.ExcedeECA || !r.Alerta || !r.TS.Equal(ts.Add(time.Second)) {
		t.Fatalf("régimen de CA-A = %+v, %v; se esperaba el de la última predicción", r, err)
	}
}

// Con la cola llena, encolar no espera: descarta y lo cuenta.
func TestEscritorDescartaSinBloquearConLaColaLlena(t *testing.T) {
	bloqueo := &prediccionesBloqueadas{entro: make(chan struct{}, 1), soltar: make(chan struct{})}
	e := nuevoEscritor(1, bloqueo, memoria.NuevaCache(), logMudo)
	ctx, cancel := context.WithCancel(context.Background())
	listo := make(chan struct{})
	go func() { e.correr(ctx); close(listo) }()

	e.encolar(almacen.Prediccion{Estacion: "CA-1"}) // la toma el escritor y se queda escribiendo
	<-bloqueo.entro
	e.encolar(almacen.Prediccion{Estacion: "CA-2"}) // llena la cola (búfer de 1)
	inicio := time.Now()
	for range 3 {
		if e.encolar(almacen.Prediccion{Estacion: "CA-3"}) {
			t.Fatal("encolar aceptó con la cola llena")
		}
	}
	if d := time.Since(inicio); d > 50*time.Millisecond {
		t.Fatalf("encolar con la cola llena tardó %v: no debe esperar", d)
	}
	if n := e.descartes.Load(); n != 3 {
		t.Fatalf("%d descartes, se esperaban 3", n)
	}
	close(bloqueo.soltar)
	cancel()
	<-listo
	if n := bloqueo.guardadas(); n != 2 {
		t.Fatalf("%d predicciones guardadas, se esperaban 2 (CA-1 y CA-2)", n)
	}
}

func TestEscritorCuentaLosFallosYSigue(t *testing.T) {
	e := nuevoEscritor(4, prediccionesRotas{}, memoria.NuevaCache(), logMudo)
	ctx, cancel := context.WithCancel(context.Background())
	e.encolar(almacen.Prediccion{Estacion: "CA-1"})
	e.encolar(almacen.Prediccion{Estacion: "CA-2"})
	cancel()
	e.correr(ctx) // con ctx cancelado, vacía la cola y vuelve
	if n := e.fallos.Load(); n != 2 {
		t.Fatalf("%d fallos, se esperaban 2", n)
	}
}

type prediccionesBloqueadas struct {
	entro, soltar chan struct{}
	mu            sync.Mutex
	n             int
}

func (p *prediccionesBloqueadas) Guardar(ctx context.Context, _ almacen.Prediccion) error {
	select {
	case p.entro <- struct{}{}:
	default:
	}
	<-p.soltar
	p.mu.Lock()
	p.n++
	p.mu.Unlock()
	return nil
}

func (p *prediccionesBloqueadas) guardadas() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.n
}

type prediccionesRotas struct{}

func (prediccionesRotas) Guardar(context.Context, almacen.Prediccion) error {
	return errors.New("MongoDB no responde")
}
