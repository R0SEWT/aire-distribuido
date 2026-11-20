package api

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

func TestLeerOrigenesCORS(t *testing.T) {
	got, err := LeerOrigenesCORS(" http://localhost:5173 , https://Aire.Example.com,http://localhost:5173,, ")
	if err != nil || !slices.Equal(got, []string{"http://localhost:5173", "https://aire.example.com"}) {
		t.Fatalf("orígenes = %v, %v", got, err)
	}
	if got, err := LeerOrigenesCORS(""); err != nil || len(got) != 0 {
		t.Fatalf("sin orígenes = %v, %v", got, err)
	}
	if got, err := LeerOrigenesCORS("*"); err != nil || !slices.Equal(got, []string{"*"}) {
		t.Fatalf("comodín = %v, %v", got, err)
	}
	for _, malo := range []string{"localhost:5173", "http://localhost:5173/", "http://localhost/app", "ftp://x", "http://u:c@x", "http://x?a=1", "http://"} {
		if _, err := LeerOrigenesCORS(malo); err == nil || !strings.Contains(err.Error(), "origen") {
			t.Errorf("%q se aceptó como origen (%v)", malo, err)
		}
	}
}

// pedirDesde hace un pedido como lo manda un navegador desde otro origen.
func pedirDesde(e *entorno, metodo, ruta, origen string, cab map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(metodo, ruta, nil)
	if origen != "" {
		r.Header.Set("Origin", origen)
	}
	for k, v := range cab {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	e.srv.ServeHTTP(w, r)
	return w
}

func TestCORSYPreflight(t *testing.T) {
	const spa = "http://localhost:5173"
	e := nuevoEntorno(t, func(c *Config) { c.OrigenesCORS = []string{spa} })
	preflight := map[string]string{"Access-Control-Request-Method": "POST", "Access-Control-Request-Headers": "authorization, content-type"}

	// El preflight de la SPA: 204, sin cuerpo y sin pasar por el JWT de la ruta.
	for _, ruta := range []string{"/api/predicciones", "/api/entrenamientos", "/api/cluster/nodos", "/api/no-existe"} {
		w := pedirDesde(e, "OPTIONS", ruta, spa, preflight)
		h := w.Header()
		if w.Code != http.StatusNoContent || w.Body.Len() != 0 || h.Get("Access-Control-Allow-Origin") != spa ||
			!strings.Contains(h.Get("Access-Control-Allow-Methods"), "POST") || !strings.Contains(h.Get("Access-Control-Allow-Headers"), "Authorization") ||
			!strings.Contains(h.Get("Access-Control-Allow-Headers"), "Content-Type") || h.Get("Access-Control-Max-Age") == "" ||
			!slices.Contains(h.Values("Vary"), "Origin") {
			t.Fatalf("preflight de %s: %d %v", ruta, w.Code, h)
		}
		if h.Get("Access-Control-Allow-Credentials") != "" {
			t.Fatal("la API no usa cookies: no manda Allow-Credentials")
		}
	}
	// El pedido de verdad lleva el encabezado, también cuando la respuesta es un error.
	w := pedirDesde(e, "GET", "/api/salud", spa, nil)
	if w.Code != 200 || w.Header().Get("Access-Control-Allow-Origin") != spa {
		t.Fatalf("GET desde la SPA: %d %v", w.Code, w.Header())
	}
	if w := pedirDesde(e, "GET", "/api/estaciones", spa, nil); w.Code != 401 || w.Header().Get("Access-Control-Allow-Origin") != spa {
		t.Fatalf("un 401 también tiene que poder leerse desde la SPA: %d %v", w.Code, w.Header())
	}
	if w := pedirDesde(e, "GET", "/api/no-existe", spa, nil); w.Code != 404 || w.Header().Get("Access-Control-Allow-Origin") != spa {
		t.Fatalf("un 404: %d %v", w.Code, w.Header())
	}

	// Otro origen: el preflight se rechaza y el pedido sale sin encabezados.
	mensajeDeError(t, pedirDesde(e, "OPTIONS", "/api/predicciones", "http://otro.example", preflight), http.StatusForbidden,
		"CORS: el origen http://otro.example no está permitido")
	if w := pedirDesde(e, "GET", "/api/salud", "http://otro.example", nil); w.Code != 200 || w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("GET desde otro origen: %d %v", w.Code, w.Header())
	}
	// Sin Origin no es un pedido de otro origen: nada cambia.
	if w := pedirDesde(e, "GET", "/api/salud", "", nil); w.Code != 200 || w.Header().Get("Access-Control-Allow-Origin") != "" || w.Header().Get("Vary") != "" {
		t.Fatalf("GET sin Origin: %v", w.Header())
	}
	// Un OPTIONS que no es un preflight sigue al mux: 405 con Allow.
	if w := pedirDesde(e, "OPTIONS", "/api/salud", spa, nil); w.Code != http.StatusMethodNotAllowed || w.Header().Get("Allow") == "" {
		t.Fatalf("OPTIONS sin Access-Control-Request-Method: %d %v", w.Code, w.Header())
	}
}

func TestCORSSinConfigurarYConComodin(t *testing.T) {
	preflight := map[string]string{"Access-Control-Request-Method": "GET"}
	sin := nuevoEntorno(t)
	if w := pedirDesde(sin, "OPTIONS", "/api/salud", "http://localhost:5173", preflight); w.Code != http.StatusForbidden {
		t.Fatalf("sin CORS_ORIGENES ningún origen pasa el preflight: %d", w.Code)
	}
	if w := pedirDesde(sin, "GET", "/api/salud", "http://localhost:5173", nil); w.Code != 200 || w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("sin CORS_ORIGENES: %v", w.Header())
	}
	todos := nuevoEntorno(t, func(c *Config) { c.OrigenesCORS = []string{"*"} })
	if w := pedirDesde(todos, "OPTIONS", "/api/salud", "http://cualquiera.example", preflight); w.Code != http.StatusNoContent || w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("con comodín: %d %v", w.Code, w.Header())
	}
}
