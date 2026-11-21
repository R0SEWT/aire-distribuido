//go:build integracion

// Pruebas contra un Redis real: el de docker-compose.yml.
//
//	docker compose up -d mongo redis seed
//	go test -race -tags integracion ./almacen/...
//
// REDIS_ADDR (localhost:6379) dice dónde está. Usan la base de datos 15, que
// vacían al empezar y al terminar; la 0, la de la API, no se toca.
package redis

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	driver "github.com/redis/go-redis/v9"

	"github.com/R0SEWT/aire-distribuido/almacen"
)

var ctx = context.Background()

const dbDePruebas = 15

func conectar(t *testing.T) *Cache {
	t.Helper()
	c, err := Conectar(Opciones{Addr: cmp.Or(os.Getenv("REDIS_ADDR"), "localhost:6379"), DB: dbDePruebas})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Ping(ctx); err != nil {
		t.Fatalf("Redis no responde (docker compose up -d mongo redis seed): %v", err)
	}
	vaciar := func() {
		if err := c.Vaciar(ctx); err != nil {
			t.Fatal(err)
		}
	}
	vaciar()
	t.Cleanup(func() { vaciar(); c.Cerrar() })
	return c
}

var t0 = time.Date(2026, 11, 14, 15, 0, 0, 0, lima)

func regimen(t *testing.T, c *Cache, estacion string) almacen.RegimenEstacion {
	t.Helper()
	r, err := c.Regimen(ctx, estacion)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestRegimenNoRetrocede(t *testing.T) {
	c := conectar(t)
	if _, err := c.Regimen(ctx, "CA-A"); !errors.Is(err, almacen.ErrNoEncontrado) {
		t.Fatalf("Regimen sin datos = %v, se esperaba ErrNoEncontrado", err)
	}
	guardar := func(r almacen.RegimenEstacion) {
		t.Helper()
		if err := c.GuardarRegimen(ctx, "CA-A", r); err != nil {
			t.Fatal(err)
		}
	}
	pm25, pm10, inca := 58.3, 121.4, "mala"
	reciente := almacen.RegimenEstacion{
		FechaHora: t0.Add(time.Hour), Regimen: 1, ExcedeECA: true,
		Media24h: almacen.Media24h{PM25: &pm25, PM10: &pm10, Lecturas: 24}, Alerta: true, INCA: &inca,
		TS: time.Date(2026, 11, 19, 22, 5, 0, 0, lima),
	}
	guardar(almacen.RegimenEstacion{FechaHora: t0, Regimen: 0, TS: reciente.TS})
	guardar(reciente)
	leido := regimen(t, c, "CA-A")
	if !leido.FechaHora.Equal(reciente.FechaHora) || !leido.TS.Equal(reciente.TS) {
		t.Fatalf("fechas = %v y %v", leido.FechaHora, leido.TS)
	}
	leido.FechaHora, leido.TS = reciente.FechaHora, reciente.TS
	if !reflect.DeepEqual(leido, reciente) {
		t.Fatalf("se leyó %+v, se guardó %+v", leido, reciente)
	}
	// Una lectura de cinco días antes no lo reemplaza.
	guardar(almacen.RegimenEstacion{FechaHora: t0.Add(-5 * 24 * time.Hour), Regimen: 2})
	if r := regimen(t, c, "CA-A"); r.Regimen != 1 || !r.Alerta {
		t.Fatalf("una lectura más antigua reemplazó el régimen: %+v", r)
	}
	// Una de la misma hora sí (se reenvió corregida).
	guardar(almacen.RegimenEstacion{FechaHora: t0.Add(time.Hour), Regimen: 0})
	if r := regimen(t, c, "CA-A"); r.Regimen != 0 || r.Alerta {
		t.Fatalf("la misma hora tiene que reemplazar: %+v", r)
	}
}

// El valor es el JSON de docs/datos/ejemplos/redis-estacion-regimen.json, con
// las fechas en la hora de Lima.
func TestElValorDelRegimenEsElJSONDelEjemplo(t *testing.T) {
	c := conectar(t)
	var ejemplo almacen.RegimenEstacion
	b, err := os.ReadFile("../../docs/datos/ejemplos/redis-estacion-regimen.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &ejemplo); err != nil {
		t.Fatal(err)
	}
	// Llega en UTC: se guarda igual en la hora de Lima.
	ejemplo.FechaHora, ejemplo.TS = ejemplo.FechaHora.UTC(), ejemplo.TS.UTC()
	if err := c.GuardarRegimen(ctx, "CA-AYAS-01", ejemplo); err != nil {
		t.Fatal(err)
	}
	valor, err := c.cliente.Get(ctx, "estacion:CA-AYAS-01:regimen").Result()
	if err != nil {
		t.Fatal(err)
	}
	var guardado, esperado any
	json.Unmarshal([]byte(valor), &guardado)
	json.Unmarshal(b, &esperado)
	if !reflect.DeepEqual(guardado, esperado) {
		t.Fatalf("valor guardado:\n%s\nejemplo:\n%s", valor, b)
	}
	if ttl := c.cliente.TTL(ctx, "estacion:CA-AYAS-01:regimen").Val(); ttl != -1 {
		t.Errorf("la clave vence en %v; no tendría que vencer", ttl)
	}
}

// La comparación es de instantes aunque las fechas lleguen con otro desfase:
// 20:30Z es media hora después de 15:00-05:00, y 19:30Z, media hora antes.
// Comparadas como texto sin normalizar, «2026-11-14T19:30:00Z» sería mayor que
// «2026-11-14T15:00:00-05:00».
func TestRegimenComparaInstantesNoTextos(t *testing.T) {
	c := conectar(t)
	c.GuardarRegimen(ctx, "CA-A", almacen.RegimenEstacion{FechaHora: t0, Regimen: 1})
	c.GuardarRegimen(ctx, "CA-A", almacen.RegimenEstacion{FechaHora: time.Date(2026, 11, 14, 19, 30, 0, 0, time.UTC), Regimen: 2})
	if r := regimen(t, c, "CA-A"); r.Regimen != 1 {
		t.Fatalf("una lectura de media hora antes, escrita en UTC, reemplazó el régimen: %+v", r)
	}
	c.GuardarRegimen(ctx, "CA-A", almacen.RegimenEstacion{FechaHora: time.Date(2026, 11, 14, 20, 30, 0, 0, time.UTC), Regimen: 2})
	if r := regimen(t, c, "CA-A"); r.Regimen != 2 {
		t.Fatalf("una lectura de media hora después, escrita en UTC, no reemplazó el régimen: %+v", r)
	}
}

// Muchas goroutines escriben a la vez, en desorden, las mismas 200 horas de
// una estación. El régimen guardado nunca retrocede: quien acaba de escribir
// la hora h lee una hora h o posterior, y al final queda la última. Si comparar
// y escribir no fueran una sola operación, una escritura atrasada pisaría a una
// más reciente.
func TestRegimenAtomicoConEscritoresConcurrentes(t *testing.T) {
	c := conectar(t)
	const escritores, horas = 16, 200
	var wg sync.WaitGroup
	for g := range escritores {
		wg.Add(1)
		go func() {
			defer wg.Done()
			orden := rand.New(rand.NewPCG(uint64(g), 7)).Perm(horas)
			for _, h := range orden {
				r := almacen.RegimenEstacion{FechaHora: t0.Add(time.Duration(h) * time.Hour), Regimen: h}
				if err := c.GuardarRegimen(ctx, "CA-A", r); err != nil {
					t.Error(err)
					return
				}
				leido, err := c.Regimen(ctx, "CA-A")
				if err != nil {
					t.Error(err)
					return
				}
				if leido.Regimen < h {
					t.Errorf("después de escribir la hora %d quedó guardada la %d: el régimen retrocedió", h, leido.Regimen)
					return
				}
			}
		}()
	}
	wg.Wait()
	if r := regimen(t, c, "CA-A"); r.Regimen != horas-1 {
		t.Fatalf("quedó el régimen de la hora %d, se esperaba el de la %d", r.Regimen, horas-1)
	}
}

// Un lote es un pipeline; cada estación sigue la regla por separado.
func TestGuardarRegimenesPorLote(t *testing.T) {
	c := conectar(t)
	lote := map[string]almacen.RegimenEstacion{}
	for i := range 33 {
		lote[fmt.Sprintf("CA-%02d", i)] = almacen.RegimenEstacion{FechaHora: t0, Regimen: i % 3}
	}
	if err := c.GuardarRegimenes(ctx, lote); err != nil {
		t.Fatal(err)
	}
	if err := c.GuardarRegimenes(ctx, nil); err != nil {
		t.Fatalf("un lote vacío no es un error: %v", err)
	}
	if n := c.cliente.DBSize(ctx).Val(); n != 33 {
		t.Fatalf("%d claves, se esperaban 33", n)
	}
	if err := c.GuardarRegimenes(ctx, map[string]almacen.RegimenEstacion{
		"CA-00": {FechaHora: t0.Add(-time.Hour), Regimen: 9}, "CA-01": {FechaHora: t0.Add(time.Hour), Regimen: 9},
	}); err != nil {
		t.Fatal(err)
	}
	if a, b := regimen(t, c, "CA-00"), regimen(t, c, "CA-01"); a.Regimen != 0 || b.Regimen != 9 {
		t.Fatalf("CA-00 = %d (se esperaba 0: la más antigua no reemplaza), CA-01 = %d (se esperaba 9)", a.Regimen, b.Regimen)
	}
}

// cuentaComandos cuenta las idas y vueltas del cliente a Redis: un comando
// suelto es una y un pipeline entero, otra.
type cuentaComandos struct{ idas atomic.Int64 }

func (*cuentaComandos) DialHook(sig driver.DialHook) driver.DialHook { return sig }

func (h *cuentaComandos) ProcessHook(sig driver.ProcessHook) driver.ProcessHook {
	return func(ctx context.Context, cmd driver.Cmder) error {
		h.idas.Add(1)
		return sig(ctx, cmd)
	}
}

func (h *cuentaComandos) ProcessPipelineHook(sig driver.ProcessPipelineHook) driver.ProcessPipelineHook {
	return func(ctx context.Context, cmds []driver.Cmder) error {
		h.idas.Add(1)
		return sig(ctx, cmds)
	}
}

// El panel pide el estado de las 33 estaciones del catálogo: es una sola ida a
// Redis (un MGET), no una por estación. Las que no tienen régimen no vienen, y
// cada régimen es el mismo que devuelve Regimen.
func TestRegimenesDeTodoElCatalogoEnUnaSolaIda(t *testing.T) {
	c := conectar(t)
	var catalogo []string
	lote := map[string]almacen.RegimenEstacion{}
	inca := "mala"
	for i := range 33 {
		id := fmt.Sprintf("CA-%02d", i)
		catalogo = append(catalogo, id)
		if i%2 == 0 { // 17 estaciones con régimen
			lote[id] = almacen.RegimenEstacion{FechaHora: t0.Add(time.Duration(i) * time.Hour), Regimen: i % 3, Alerta: i%4 == 0,
				INCA: &inca, Media24h: almacen.Media24h{Lecturas: i % 24}, TS: t0}
		}
	}
	if vacio, err := c.Regimenes(ctx, catalogo); err != nil || vacio == nil || len(vacio) != 0 {
		t.Fatalf("Regimenes sin datos = %v, %v; se esperaba un mapa vacío", vacio, err)
	}
	if err := c.GuardarRegimenes(ctx, lote); err != nil {
		t.Fatal(err)
	}
	cuenta := &cuentaComandos{}
	c.cliente.AddHook(cuenta)
	rs, err := c.Regimenes(ctx, catalogo)
	if err != nil {
		t.Fatal(err)
	}
	if n := cuenta.idas.Load(); n != 1 {
		t.Fatalf("Regimenes hizo %d idas a Redis para %d estaciones; tiene que ser una", n, len(catalogo))
	}
	if len(rs) != 17 {
		t.Fatalf("%d estaciones con régimen, se esperaban 17: %v", len(rs), rs)
	}
	for id, r := range rs {
		if una := regimen(t, c, id); !reflect.DeepEqual(r, una) {
			t.Fatalf("%s: Regimenes = %+v y Regimen = %+v", id, r, una)
		}
	}
	if sin, err := c.Regimenes(ctx, nil); err != nil || len(sin) != 0 {
		t.Fatalf("Regimenes sin estaciones = %v, %v", sin, err)
	}
	// Una clave que alguien escribió a mano no se disimula: es un error que
	// nombra la clave, como en Regimen.
	c.cliente.Set(ctx, claveRegimen("CA-01"), "no es JSON", 0)
	if _, err := c.Regimenes(ctx, catalogo); err == nil || !strings.Contains(err.Error(), "estacion:CA-01:regimen") {
		t.Fatalf("Regimenes con un valor ilegible = %v", err)
	}
}

// Si Redis se reinicia pierde los scripts cargados: el lote se repite después
// de cargarlo otra vez.
func TestGuardarRegimenesDespuesDeQueRedisOlvidaElScript(t *testing.T) {
	c := conectar(t)
	if err := c.GuardarRegimen(ctx, "CA-A", almacen.RegimenEstacion{FechaHora: t0, Regimen: 1}); err != nil {
		t.Fatal(err)
	}
	if err := c.cliente.ScriptFlush(ctx).Err(); err != nil {
		t.Fatal(err)
	}
	if err := c.GuardarRegimen(ctx, "CA-A", almacen.RegimenEstacion{FechaHora: t0.Add(time.Hour), Regimen: 2}); err != nil {
		t.Fatal(err)
	}
	if r := regimen(t, c, "CA-A"); r.Regimen != 2 {
		t.Fatalf("régimen = %+v", r)
	}
}

// Un valor que no es JSON (alguien escribió la clave a mano) se reemplaza.
func TestUnValorIlegibleSeReemplaza(t *testing.T) {
	c := conectar(t)
	if err := c.cliente.Set(ctx, "estacion:CA-A:regimen", "esto no es JSON", 0).Err(); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Regimen(ctx, "CA-A"); err == nil || errors.Is(err, almacen.ErrNoEncontrado) {
		t.Fatalf("Regimen con un valor ilegible = %v, se esperaba un error de lectura", err)
	}
	if err := c.GuardarRegimen(ctx, "CA-A", almacen.RegimenEstacion{FechaHora: t0, Regimen: 1}); err != nil {
		t.Fatal(err)
	}
	if r := regimen(t, c, "CA-A"); r.Regimen != 1 {
		t.Fatalf("régimen = %+v", r)
	}
}

func TestModeloActivoYPuntoDeControl(t *testing.T) {
	c := conectar(t)
	if _, err := c.ModeloActivo(ctx); !errors.Is(err, almacen.ErrNoEncontrado) {
		t.Fatalf("ModeloActivo sin datos = %v", err)
	}
	if _, err := c.Punto(ctx, "e-1"); !errors.Is(err, almacen.ErrNoEncontrado) {
		t.Fatalf("Punto sin datos = %v", err)
	}
	var m almacen.Modelo
	b, err := os.ReadFile("../../docs/datos/ejemplos/redis-modelo-activo.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if err := c.GuardarModeloActivo(ctx, m); err != nil {
		t.Fatal(err)
	}
	leido, err := c.ModeloActivo(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// En Redis creado conserva los nanosegundos (en MongoDB, no).
	if !leido.Creado.Equal(m.Creado) {
		t.Errorf("creado = %v, se esperaba %v", leido.Creado, m.Creado)
	}
	leido.Creado = m.Creado
	if !reflect.DeepEqual(leido, m) {
		t.Fatalf("se leyó\n%+v\nse guardó\n%+v", leido, m)
	}
	var guardado, esperado any
	json.Unmarshal([]byte(c.cliente.Get(ctx, "modelo:activo").Val()), &guardado)
	json.Unmarshal(b, &esperado)
	if !reflect.DeepEqual(guardado, esperado) {
		t.Errorf("modelo:activo no es el JSON del ejemplo:\n%s", c.cliente.Get(ctx, "modelo:activo").Val())
	}

	punto := almacen.PuntoControl{Iter: 44, Centroides: m.CentroidesStd}
	if err := c.GuardarPunto(ctx, "e-20261119-1530", almacen.PuntoControl{Iter: 43}); err != nil {
		t.Fatal(err)
	}
	if err := c.GuardarPunto(ctx, "e-20261119-1530", punto); err != nil {
		t.Fatal(err)
	}
	if p, err := c.Punto(ctx, "e-20261119-1530"); err != nil || !reflect.DeepEqual(p, punto) {
		t.Fatalf("Punto = %+v, %v", p, err)
	}
	if n := c.cliente.Exists(ctx, "entrenamiento:e-20261119-1530:punto").Val(); n != 1 {
		t.Fatal("no existe la clave entrenamiento:{id}:punto")
	}
}

// Con Redis caído nada se cuelga ni se reintenta dentro del driver.
func TestConRedisCaidoFallaPronto(t *testing.T) {
	c, err := Conectar(Opciones{Addr: "127.0.0.1:1"})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Cerrar()
	inicio := time.Now()
	if err := c.Ping(ctx); err == nil {
		t.Fatal("Ping no falló sin Redis")
	}
	if err := c.GuardarRegimen(ctx, "CA-A", almacen.RegimenEstacion{FechaHora: t0}); err == nil {
		t.Fatal("GuardarRegimen no falló sin Redis")
	}
	if _, err := c.Regimen(ctx, "CA-A"); err == nil || errors.Is(err, almacen.ErrNoEncontrado) {
		t.Fatalf("Regimen sin Redis = %v; no es lo mismo que una clave que no existe", err)
	}
	if d := time.Since(inicio); d > 3*time.Second {
		t.Fatalf("tres operaciones sin Redis tardaron %v", d)
	}
}

func TestConectarExigeLaDireccion(t *testing.T) {
	if _, err := Conectar(Opciones{}); err == nil {
		t.Fatal("aceptó una dirección vacía")
	}
}
