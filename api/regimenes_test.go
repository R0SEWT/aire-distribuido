package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/R0SEWT/aire-distribuido/almacen"
	"github.com/R0SEWT/aire-distribuido/almacen/memoria"
)

// Los campos obligatorios del esquema EstadoEstacion del contrato.
var camposEstadoEstacion = append([]string{"estacion", "fecha_hora"}, camposRegimenEstacion...)

// lista decodifica una respuesta que es una lista de objetos JSON.
func lista(t *testing.T, w *httptest.ResponseRecorder, codigo int) []map[string]any {
	t.Helper()
	if w.Code != codigo {
		t.Fatalf("código %d, se esperaba %d: %s", w.Code, codigo, w.Body)
	}
	var l []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &l); err != nil || l == nil {
		t.Fatalf("la respuesta no es una lista JSON (%v): %s", err, w.Body)
	}
	return l
}

// Con el sistema recién levantado ninguna estación tiene estado: la lista está
// vacía y es un 200, para los dos roles. Sin token, 401.
func TestRegimenesVacioYPermisos(t *testing.T) {
	e := nuevoEntorno(t)
	mensajeDeError(t, e.pedir(t, "GET", "/api/regimenes", "", ""), 401, msgNoAutenticado)
	for _, tok := range []string{e.token(t, "demo", "demo"), e.token(t, "admin", "clave-admin")} {
		w := e.pedir(t, "GET", "/api/regimenes", tok, "")
		if l := lista(t, w, 200); len(l) != 0 || w.Body.String() != "[]\n" {
			t.Fatalf("sin predicciones la lista tiene que ser []: %s", w.Body)
		}
	}
	if w := e.pedir(t, "POST", "/api/regimenes", e.token(t, "demo", "demo"), "{}"); w.Code != 405 {
		t.Fatalf("POST /api/regimenes: %d", w.Code)
	}
}

// Cada entrada es el GET /api/estaciones/{id}/regimen de su estación más su
// código, y la lista viene ordenada por estación. Las estaciones sin
// predicciones no aparecen.
func TestRegimenesTraeElUltimoEstadoDeCadaEstacion(t *testing.T) {
	e := nuevoEntorno(t)
	tok := e.token(t, "demo", "demo")
	// Tres horas de una estación y veinticuatro de otra, que queda en alerta;
	// se mandan en el orden contrario al de la lista.
	for h := 1; h <= 24; h++ {
		objeto(t, e.pedir(t, "POST", "/api/predicciones", tok, lecturaJSON("CA-MAC-01", h, 80, 150)), 200)
	}
	for h := 1; h <= 3; h++ {
		objeto(t, e.pedir(t, "POST", "/api/predicciones", tok, lecturaJSON("CA-AYAS-01", h, 5, 9)), 200)
	}
	e.esperarEscritas(t, 27)

	w := e.pedir(t, "GET", "/api/regimenes", tok, "")
	l := lista(t, w, 200)
	if len(l) != 2 || l[0]["estacion"] != "CA-AYAS-01" || l[1]["estacion"] != "CA-MAC-01" {
		t.Fatalf("se esperaban CA-AYAS-01 y CA-MAC-01, en ese orden: %s", w.Body)
	}
	for _, estado := range l {
		for _, c := range camposEstadoEstacion {
			if _, ok := estado[c]; !ok {
				t.Fatalf("falta el campo %q en %v", c, estado)
			}
		}
		if len(estado) != len(camposEstadoEstacion) {
			t.Fatalf("el estado trae campos que el contrato no nombra: %v", estado)
		}
		// Sin el código, es lo mismo que responde la ruta de una sola estación.
		una := objeto(t, e.pedir(t, "GET", "/api/estaciones/"+estado["estacion"].(string)+"/regimen", tok, ""), 200)
		una["estacion"] = estado["estacion"]
		a, _ := json.Marshal(una)
		b, _ := json.Marshal(estado)
		if string(a) != string(b) {
			t.Fatalf("la lista y la ruta de una estación no coinciden:\n%s\n%s", b, a)
		}
	}
	if l[1]["alerta"] != true || l[1]["fecha_hora"] != "2026-11-14T00:00:00-05:00" ||
		l[0]["alerta"] != false || l[0]["inca"] != nil || l[0]["media_24h"].(map[string]any)["lecturas"] != 3.0 {
		t.Fatalf("estados: %s", w.Body)
	}
}

// Una lectura más antigua que la guardada no cambia lo que la lista dice.
func TestRegimenesNoRetrocedeConUnaLecturaMasAntigua(t *testing.T) {
	e := nuevoEntorno(t)
	tok := e.token(t, "demo", "demo")
	objeto(t, e.pedir(t, "POST", "/api/predicciones", tok, lecturaJSON("CA-MAC-01", 10, 80, 150)), 200)
	e.esperarEscritas(t, 1)
	antes := e.pedir(t, "GET", "/api/regimenes", tok, "").Body.String()
	objeto(t, e.pedir(t, "POST", "/api/predicciones", tok, lecturaJSON("CA-MAC-01", 4, 5, 5)), 200)
	e.esperarEscritas(t, 2)
	if despues := e.pedir(t, "GET", "/api/regimenes", tok, "").Body.String(); despues != antes || len(lista(t, e.pedir(t, "GET", "/api/regimenes", tok, ""), 200)) != 1 {
		t.Fatalf("la lectura de la hora 4 cambió el estado de la hora 10:\n%s\n%s", antes, despues)
	}
}

// cacheContada cuenta las idas a la caché por tipo de lectura.
type cacheContada struct {
	*memoria.Cache
	deUna, deTodas atomic.Int64
	pedidas        atomic.Int64 // estaciones del último Regimenes
	falla          atomic.Bool
}

func (c *cacheContada) Regimen(ctx context.Context, estacion string) (almacen.RegimenEstacion, error) {
	c.deUna.Add(1)
	return c.Cache.Regimen(ctx, estacion)
}

func (c *cacheContada) Regimenes(ctx context.Context, estaciones []string) (map[string]almacen.RegimenEstacion, error) {
	c.deTodas.Add(1)
	c.pedidas.Store(int64(len(estaciones)))
	if c.falla.Load() {
		return nil, errors.New("sin conexión")
	}
	if !slices.IsSorted(estaciones) {
		return nil, errors.New("las estaciones no llegaron ordenadas")
	}
	return c.Cache.Regimenes(ctx, estaciones)
}

// El panel no puede costar un pedido a Redis por estación: la lista entera es
// una sola ida a la caché, con las 33 estaciones del catálogo. Y si la caché
// no responde, 503.
func TestRegimenesEsUnaSolaIdaALaCacheY503SiNoResponde(t *testing.T) {
	cache := &cacheContada{Cache: memoria.NuevaCache()}
	e := nuevoEntorno(t, func(c *Config) { c.Cache = cache })
	tok := e.token(t, "demo", "demo")
	objeto(t, e.pedir(t, "POST", "/api/predicciones", tok, lecturaEjemplo), 200)
	e.esperarEscritas(t, 1)
	if l := lista(t, e.pedir(t, "GET", "/api/regimenes", tok, ""), 200); len(l) != 1 {
		t.Fatalf("lista = %v", l)
	}
	if cache.deTodas.Load() != 1 || cache.deUna.Load() != 0 || cache.pedidas.Load() != int64(len(e.srv.catalogo)) || len(e.srv.catalogo) != 33 {
		t.Fatalf("idas a la caché: %d por lista y %d por estación, con %d estaciones (catálogo de %d)",
			cache.deTodas.Load(), cache.deUna.Load(), cache.pedidas.Load(), len(e.srv.catalogo))
	}
	cache.falla.Store(true)
	mensajeDeError(t, e.pedir(t, "GET", "/api/regimenes", tok, ""), 503, "Redis no disponible")
}
