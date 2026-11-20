package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/R0SEWT/aire-distribuido/almacen"
	"github.com/R0SEWT/aire-distribuido/almacen/memoria"
)

var logMudo = slog.New(slog.NewTextHandler(io.Discard, nil))

// esperarEscritas espera a que los dos almacenes hayan escrito n predicciones.
// Cada uno tiene su goroutine: que MongoDB ya tenga una predicción no dice que
// Redis ya tenga su régimen, ni al revés. Una prueba que lee de un almacén
// después de predecir espera con esto, sobre lo que de verdad va a leer.
func (e *entorno) esperarEscritas(t testing.TB, n uint64) {
	t.Helper()
	limite := time.Now().Add(5 * time.Second)
	for {
		est := e.srv.escritor.estado()
		if est["mongo"].Escritas >= n && est["redis"].Escritas >= n {
			return
		}
		if time.Now().After(limite) {
			t.Fatalf("el escritor no llegó a %d predicciones escritas en los dos almacenes: %+v", n, est)
		}
		time.Sleep(time.Millisecond)
	}
}

// arrancar corre el escritor y devuelve cómo pararlo y esperar a que termine.
func arrancar(e *escritor) (parar func()) {
	ctx, cancel := context.WithCancel(context.Background())
	listo := make(chan struct{})
	go func() { e.correr(ctx); close(listo) }()
	return func() { cancel(); <-listo }
}

func TestEscritorGuardaYActualizaElRegimen(t *testing.T) {
	pred, cache := memoria.NuevasPredicciones(), memoria.NuevaCache()
	e := nuevoEscritor(8, pred, cache, logMudo)
	parar := arrancar(e)

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
	parar() // correr escribe lo que quedó en las colas antes de volver
	if n := len(pred.Todas()); n != 3 {
		t.Fatalf("%d predicciones guardadas, se esperaban 3", n)
	}
	r, err := cache.Regimen(context.Background(), "CA-A")
	if err != nil || r.Regimen != 1 || !r.ExcedeECA || !r.Alerta || !r.TS.Equal(ts.Add(time.Second)) {
		t.Fatalf("régimen de CA-A = %+v, %v; se esperaba el de la última predicción", r, err)
	}
	for nombre, est := range e.estado() {
		if est.Escritas != 3 || est.Descartadas != 0 || est.Fallidas != 0 || est.Lotes == 0 {
			t.Errorf("%s = %+v; se esperaban 3 escritas y ningún descarte ni fallo", nombre, est)
		}
	}
}

// Al salir, el escritor vacía las colas: con 100 predicciones encoladas y el
// contexto ya cancelado, se guardan las 100. Si correr volviera apenas se
// cancela el contexto, el select elegiría al azar entre la cola y ctx.Done() y
// esta prueba fallaría.
func TestEscritorVaciaLaColaAlSalir(t *testing.T) {
	pred, cache := memoria.NuevasPredicciones(), memoria.NuevaCache()
	e := nuevoEscritor(100, pred, cache, logMudo)
	for i := range 100 {
		if !e.encolar(almacen.Prediccion{Estacion: fmt.Sprint("CA-", i), Regimen: i}) {
			t.Fatal("la cola se llenó antes de tiempo")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	e.correr(ctx)
	if n := len(pred.Todas()); n != 100 {
		t.Fatalf("se guardaron %d de 100 predicciones al salir", n)
	}
	if _, err := cache.Regimen(context.Background(), "CA-99"); err != nil {
		t.Fatalf("el régimen de la última estación no llegó a la caché: %v", err)
	}
}

// Con la cola llena, encolar no espera: descarta y lo cuenta.
func TestEscritorDescartaSinBloquearConLaColaLlena(t *testing.T) {
	bloqueo := nuevasPrediccionesBloqueadas()
	e := nuevoEscritor(1, bloqueo, memoria.NuevaCache(), logMudo)
	parar := arrancar(e)

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
	// La cola trabada es la de MongoDB; la de Redis, también de 1, puede haber
	// descartado alguna de la ráfaga, y el total las suma.
	if n := e.estado()["mongo"].Descartadas; n != 3 || e.descartadas() < 3 {
		t.Fatalf("%d descartes en MongoDB (%d en total), se esperaban 3", n, e.descartadas())
	}
	close(bloqueo.soltar)
	parar()
	if n := bloqueo.guardadas(); n != 2 {
		t.Fatalf("%d predicciones guardadas, se esperaban 2 (CA-1 y CA-2)", n)
	}
}

// Mientras el almacén atiende un lote, lo que llega se junta en la cola y sale
// en una sola ida y vuelta: 100 predicciones, dos escrituras.
func TestEscritorEscribePorLotes(t *testing.T) {
	bloqueo := nuevasPrediccionesBloqueadas()
	e := nuevoEscritor(128, bloqueo, memoria.NuevaCache(), logMudo)
	parar := arrancar(e)

	e.encolar(almacen.Prediccion{Estacion: "CA-0"})
	<-bloqueo.entro // el escritor está en la primera escritura, de una predicción
	for i := 1; i < 100; i++ {
		if !e.encolar(almacen.Prediccion{Estacion: "CA-A", Regimen: i}) {
			t.Fatal("la cola se llenó antes de tiempo")
		}
	}
	close(bloqueo.soltar)
	parar()
	if lotes := bloqueo.tamanos(); len(lotes) != 2 || lotes[0] != 1 || lotes[1] != 99 {
		t.Fatalf("lotes de %v predicciones; se esperaban dos, de 1 y de 99", lotes)
	}
	if est := e.estado()["mongo"]; est.Escritas != 100 || est.Lotes != 2 {
		t.Fatalf("mongo = %+v; se esperaban 100 escritas en 2 lotes", est)
	}
}

// Un lote no pasa de maxLote predicciones.
func TestEscritorLimitaElLote(t *testing.T) {
	bloqueo := nuevasPrediccionesBloqueadas()
	close(bloqueo.soltar)
	e := nuevoEscritor(3*maxLote, bloqueo, memoria.NuevaCache(), logMudo)
	for range 2*maxLote + 10 {
		e.encolar(almacen.Prediccion{Estacion: "CA-A"})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	e.correr(ctx)
	if lotes := bloqueo.tamanos(); len(lotes) != 3 || lotes[0] != maxLote || lotes[1] != maxLote || lotes[2] != 10 {
		t.Fatalf("lotes de %v predicciones; se esperaban %d, %d y 10", lotes, maxLote, maxLote)
	}
}

// Cada almacén tiene su cola y su goroutine: con MongoDB trabado en una
// escritura, Redis sigue recibiendo el régimen de las estaciones.
func TestUnAlmacenTrabadoNoFrenaAlOtro(t *testing.T) {
	bloqueo, cache := nuevasPrediccionesBloqueadas(), memoria.NuevaCache()
	e := nuevoEscritor(64, bloqueo, cache, logMudo)
	parar := arrancar(e)
	defer func() { close(bloqueo.soltar); parar() }()

	for i := range 50 {
		e.encolar(almacen.Prediccion{Estacion: "CA-A", Regimen: i, FechaHora: time.Unix(int64(i)*3600, 0)})
	}
	<-bloqueo.entro // MongoDB no devuelve nada hasta el final de la prueba
	limite := time.Now().Add(2 * time.Second)
	for {
		if r, err := cache.Regimen(context.Background(), "CA-A"); err == nil && r.Regimen == 49 {
			break
		}
		if time.Now().After(limite) {
			t.Fatalf("con MongoDB trabado, el régimen no llegó a Redis: %+v", e.estado())
		}
		time.Sleep(time.Millisecond)
	}
	if n := bloqueo.guardadas(); n != 0 {
		t.Fatalf("MongoDB guardó %d predicciones mientras estaba trabado", n)
	}
}

// El caso que motivó los lotes: un almacén que tarda 1 ms por ida y vuelta.
// Escribiendo de a una caben menos de 1000 predicciones por segundo, y lo que
// llegue más rápido que eso termina descartado cuando se llena la cola. Aquí
// llegan varias veces más rápido y, por lotes, no se descarta ninguna.
func TestEscritorNoDescartaConUnAlmacenDe1msPorEscritura(t *testing.T) {
	lento := &prediccionesLentas{demora: time.Millisecond}
	e := nuevoEscritor(1024, lento, memoria.NuevaCache(), logMudo)
	parar := arrancar(e)

	const productores, porProductor = 4, 1000
	var wg sync.WaitGroup
	for g := range productores {
		wg.Go(func() {
			for i := range porProductor {
				e.encolar(almacen.Prediccion{Estacion: fmt.Sprint("CA-", g), Regimen: i})
				time.Sleep(400 * time.Microsecond)
			}
		})
	}
	wg.Wait()
	parar()
	const total = productores * porProductor
	mongo := e.estado()["mongo"]
	if mongo.Descartadas != 0 || mongo.Escritas != total || lento.guardadas.Load() != total {
		t.Fatalf("mongo = %+v y el almacén recibió %d; se esperaban %d escritas y ningún descarte", mongo, lento.guardadas.Load(), total)
	}
	if mongo.Lotes > total/2 {
		t.Errorf("%d predicciones en %d lotes: no se están agrupando", total, mongo.Lotes)
	}
	t.Logf("%d predicciones en %d lotes (%.1f por lote)", total, mongo.Lotes, float64(total)/float64(mongo.Lotes))
}

func TestEscritorCuentaLosFallosYSigue(t *testing.T) {
	roto := &prediccionesRotas{fallan: 2}
	e := nuevoEscritor(4, roto, memoria.NuevaCache(), logMudo)
	parar := arrancar(e)
	// De a una y esperando: cada predicción es un lote.
	for i := range 3 {
		e.encolar(almacen.Prediccion{Estacion: fmt.Sprint("CA-", i)})
		for limite := time.Now().Add(2 * time.Second); roto.llamadas.Load() <= int64(i) && time.Now().Before(limite); {
			time.Sleep(time.Millisecond)
		}
	}
	parar()
	if mongo := e.estado()["mongo"]; mongo.Fallidas != 2 || mongo.Escritas != 1 {
		t.Fatalf("mongo = %+v; se esperaban 2 fallidas y, después, 1 escrita", mongo)
	}
	// El fallo de MongoDB no toca a Redis.
	if redis := e.estado()["redis"]; redis.Fallidas != 0 || redis.Escritas != 3 || e.fallidas() != 2 {
		t.Fatalf("redis = %+v, fallidas en total = %d", redis, e.fallidas())
	}
}

// Docker espera 10 s antes de matar el proceso. Con un almacén que no responde
// y la cola llena, el escritor no se queda escribiendo: vuelve al pasar
// plazoVaciado y lo que no escribió queda contado, no perdido en silencio.
func TestEscritorApagadoAcotado(t *testing.T) {
	colgado := prediccionesColgadas{}
	e := nuevoEscritor(600, colgado, memoria.NuevaCache(), logMudo)
	e.plazoVaciado = 100 * time.Millisecond
	const total = 600
	for range total {
		e.encolar(almacen.Prediccion{Estacion: "CA-A"})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	inicio := time.Now()
	e.correr(ctx)
	if d := time.Since(inicio); d < e.plazoVaciado || d > e.plazoVaciado+time.Second {
		t.Fatalf("el escritor volvió a los %v; se esperaba poco más que el plazo de %v", d, e.plazoVaciado)
	}
	// El reparto es siempre el mismo: el primer lote del vaciado espera al
	// almacén hasta que vence el plazo y falla; el resto ya no se intenta.
	mongo := e.estado()["mongo"]
	if mongo.Escritas != 0 || mongo.Fallidas != maxLote || mongo.Descartadas != total-maxLote || mongo.Lotes != 1 {
		t.Fatalf("mongo = %+v; se esperaba un lote fallido de %d y %d descartadas", mongo, maxLote, total-maxLote)
	}
	// Redis respondía: lo suyo se escribió todo.
	if redis := e.estado()["redis"]; redis.Escritas != total {
		t.Fatalf("redis = %+v; se esperaban %d escritas", redis, total)
	}
}

// De un lote, a la caché va lo último de cada estación.
func TestRegimenesDelLote(t *testing.T) {
	h := func(n int) time.Time { return time.Date(2026, 11, 14, n, 0, 0, 0, time.UTC) }
	rs := regimenes([]almacen.Prediccion{
		{Estacion: "CA-A", FechaHora: h(10), Regimen: 0},
		{Estacion: "CA-A", FechaHora: h(12), Regimen: 1, Alerta: true},
		{Estacion: "CA-A", FechaHora: h(11), Regimen: 2}, // llegó tarde: no reemplaza a la de las 12
		{Estacion: "CA-B", FechaHora: h(9), Regimen: 0},
		{Estacion: "CA-B", FechaHora: h(9), Regimen: 2}, // la misma hora, corregida: reemplaza
	})
	if len(rs) != 2 || rs["CA-A"].Regimen != 1 || !rs["CA-A"].Alerta || !rs["CA-A"].FechaHora.Equal(h(12)) || rs["CA-B"].Regimen != 2 {
		t.Fatalf("regimenes = %+v", rs)
	}
}

// Los descartes y los fallos de escritura se ven en /api/metricas, en total y
// por almacén.
func TestMetricasPublicanDescartesYFallosDeEscritura(t *testing.T) {
	roto := &prediccionesRotas{fallan: 1 << 30}
	e := nuevoEntorno(t, func(c *Config) { c.Predicciones = roto })
	tok := e.token(t, "demo", "demo")
	objeto(t, e.pedir(t, "POST", "/api/predicciones", tok, lecturaEjemplo), 200)
	for limite := time.Now().Add(2 * time.Second); roto.llamadas.Load() == 0 && time.Now().Before(limite); {
		time.Sleep(time.Millisecond)
	}
	var m struct {
		Descartadas *uint64                    `json:"escrituras_descartadas"`
		Fallidas    *uint64                    `json:"escrituras_fallidas"`
		Escritura   map[string]estadoEscritura `json:"escritura"`
	}
	w := e.pedir(t, "GET", "/api/metricas", e.token(t, "admin", "clave-admin"), "")
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil || w.Code != 200 {
		t.Fatalf("métricas: %d, %v", w.Code, err)
	}
	if m.Descartadas == nil || *m.Descartadas != 0 || m.Fallidas == nil || *m.Fallidas != 1 {
		t.Fatalf("métricas = %s; se esperaban 0 descartadas y 1 fallida", w.Body)
	}
	if mongo, ok := m.Escritura["mongo"]; !ok || mongo.Fallidas != 1 || mongo.Lotes != 1 {
		t.Fatalf("escritura.mongo = %+v en %s", mongo, w.Body)
	}
	if _, ok := m.Escritura["redis"]; !ok || len(m.Escritura) != 2 {
		t.Fatalf("escritura = %+v; se esperaban mongo y redis", m.Escritura)
	}
}

// prediccionesBloqueadas avisa por entro cuando recibe un lote y no lo guarda
// hasta que se cierra soltar.
type prediccionesBloqueadas struct {
	entro, soltar chan struct{}
	mu            sync.Mutex
	lotes         []int
}

func nuevasPrediccionesBloqueadas() *prediccionesBloqueadas {
	return &prediccionesBloqueadas{entro: make(chan struct{}, 1), soltar: make(chan struct{})}
}

func (p *prediccionesBloqueadas) GuardarLote(_ context.Context, ps []almacen.Prediccion) error {
	select {
	case p.entro <- struct{}{}:
	default:
	}
	<-p.soltar
	p.mu.Lock()
	p.lotes = append(p.lotes, len(ps))
	p.mu.Unlock()
	return nil
}

func (p *prediccionesBloqueadas) tamanos() []int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]int{}, p.lotes...)
}

func (p *prediccionesBloqueadas) guardadas() int {
	n := 0
	for _, t := range p.tamanos() {
		n += t
	}
	return n
}

// prediccionesRotas falla los primeros fallan lotes y después guarda.
type prediccionesRotas struct {
	fallan   int64
	llamadas atomic.Int64
}

func (p *prediccionesRotas) GuardarLote(context.Context, []almacen.Prediccion) error {
	if p.llamadas.Add(1) <= p.fallan {
		return errors.New("MongoDB no responde")
	}
	return nil
}

// prediccionesLentas tarda demora por ida y vuelta, sea cual sea el lote.
type prediccionesLentas struct {
	demora    time.Duration
	guardadas atomic.Int64
}

func (p *prediccionesLentas) GuardarLote(_ context.Context, ps []almacen.Prediccion) error {
	time.Sleep(p.demora)
	p.guardadas.Add(int64(len(ps)))
	return nil
}

// prediccionesColgadas no responde hasta que vence el plazo de la escritura.
type prediccionesColgadas struct{}

func (prediccionesColgadas) GuardarLote(ctx context.Context, _ []almacen.Prediccion) error {
	<-ctx.Done()
	return ctx.Err()
}
