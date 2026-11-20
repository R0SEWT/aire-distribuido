package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	secreto        = "0123456789abcdef0123456789abcdef"
	usuariosPrueba = "ana:una-clave-larga:admin"
)

// Las rutas por omisión son relativas a la raíz del repo; las pruebas corren
// en cmd/api.
var argsDelRepo = []string{
	"-kmeans", "../../reports/kmeans.json", "-regimenes", "../../reports/regimenes.json",
	"-estandarizacion", "../../reports/estandarizacion.json", "-estaciones", "../../reports/estaciones_ficha.csv",
	"-openapi", "../../docs/api/openapi.yaml",
	// Sin clúster ni gold: estas pruebas son las del arranque. El coordinador
	// se prueba de punta a punta en coordinador_test.go.
	"-nodos-addr", "", "-gold", "",
}

func entornoCon(vars map[string]string) func(string) string {
	return func(k string) string { return vars[k] }
}

func TestArrancaContestaYSeApagaEnOrden(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var log bytes.Buffer
	dir := make(chan string, 1)
	fin := make(chan error, 1)
	go func() {
		fin <- correr(ctx, append([]string{"-addr", "127.0.0.1:0", "-demo"}, argsDelRepo...),
			entornoCon(map[string]string{"JWT_SECRETO": secreto}), &bufferSeguro{b: &log}, func(d, _ string) { dir <- d })
	}()
	var base string
	select {
	case d := <-dir:
		base = "http://" + d
	case err := <-fin:
		t.Fatalf("correr terminó antes de escuchar: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("la API no empezó a escuchar")
	}

	salud := pedir(t, "GET", base+"/api/salud", "", "")
	if salud["api"] != "ok" || salud["modelo_activo"] != "sembrado-kmeans-json" {
		t.Fatalf("salud = %v", salud)
	}
	tok := pedir(t, "POST", base+"/api/auth/token", "", `{"usuario": "demo", "clave": "demo"}`)
	pred := pedir(t, "POST", base+"/api/predicciones", tok["token"].(string),
		`{"estacion": "CA-AYAS-01", "fecha_hora": "2026-11-14T15:00:00-05:00", "pm2_5": 70.2, "pm10": 190.5}`)
	if pred["regimen"] != 1.0 || pred["excede_eca"] != true {
		t.Fatalf("predicción = %v", pred)
	}
	// La verificación del healthcheck de Docker (-salud) contra la API viva.
	if err := consultarSalud(base + "/api/salud"); err != nil {
		t.Fatalf("consultarSalud: %v", err)
	}

	cancel()
	select {
	case err := <-fin:
		if err != nil {
			t.Fatalf("correr devolvió %v al apagarse", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("la API no se apagó")
	}
	if !strings.Contains(log.String(), "level=WARN") || !strings.Contains(log.String(), "usuarios de demostración") {
		t.Errorf("con -demo tiene que avisar que usa los usuarios de demostración:\n%s", log.String())
	}
}

// Sin API_USUARIOS la API no arranca, salvo con -demo; con API_USUARIOS, los
// usuarios de demostración no existen.
func TestUsuariosObligatorios(t *testing.T) {
	op, err := leerOpciones(argsDelRepo)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = armar(op, entornoCon(map[string]string{"JWT_SECRETO": secreto}), logMudo(), nil)
	if err == nil || !strings.Contains(err.Error(), "API_USUARIOS") || !strings.Contains(err.Error(), "-demo") {
		t.Fatalf("sin API_USUARIOS ni -demo: %v; tiene que negarse a arrancar y nombrar las dos salidas", err)
	}
	srv, _, err := armar(op, entornoCon(map[string]string{"JWT_SECRETO": secreto, "API_USUARIOS": "ana:una-clave-larga:admin"}), logMudo(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for usuario, clave := range map[string]string{"admin": "admin", "demo": "demo"} {
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, httptest.NewRequest("POST", "/api/auth/token", strings.NewReader(`{"usuario": "`+usuario+`", "clave": "`+clave+`"}`)))
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("%s/%s con API_USUARIOS definida: %d, se esperaba 401", usuario, clave, w.Code)
		}
	}
}

func TestArmarFallaConUnaConfiguracionMala(t *testing.T) {
	op, err := leerOpciones(argsDelRepo)
	if err != nil {
		t.Fatal(err)
	}
	casos := map[string]struct {
		vars    map[string]string
		cambiar func(*opciones)
		texto   string
	}{
		"sin secreto":      {map[string]string{}, nil, "JWT_SECRETO"},
		"secreto corto":    {map[string]string{"JWT_SECRETO": "corto"}, nil, "32"},
		"usuarios malos":   {map[string]string{"JWT_SECRETO": secreto, "API_USUARIOS": "demo:demo"}, nil, "usuario:clave:rol"},
		"sin reports":      {map[string]string{"JWT_SECRETO": secreto, "API_USUARIOS": usuariosPrueba}, func(o *opciones) { o.kmeans = "no-existe.json" }, "no-existe.json"},
		"sin ficha":        {map[string]string{"JWT_SECRETO": secreto, "API_USUARIOS": usuariosPrueba}, func(o *opciones) { o.estaciones = "no-existe.csv" }, "no-existe.csv"},
		"sin contrato":     {map[string]string{"JWT_SECRETO": secreto, "API_USUARIOS": usuariosPrueba}, func(o *opciones) { o.openapi = "no-existe.yaml" }, "no-existe.yaml"},
		"cola sin búfer":   {map[string]string{"JWT_SECRETO": secreto, "API_USUARIOS": usuariosPrueba}, func(o *opciones) { o.cola = 0 }, "cola"},
		"duración del JWT": {map[string]string{"JWT_SECRETO": secreto}, func(o *opciones) { o.duracionJWT = 0 }, "duración"},
	}
	for nombre, c := range casos {
		t.Run(nombre, func(t *testing.T) {
			o := op
			if c.cambiar != nil {
				c.cambiar(&o)
			}
			_, _, err := armar(o, entornoCon(c.vars), logMudo(), nil)
			if err == nil || !strings.Contains(err.Error(), c.texto) {
				t.Fatalf("armar = %v, se esperaba un error que mencione %q", err, c.texto)
			}
		})
	}
	if _, err := leerOpciones([]string{"-no-existe"}); err == nil {
		t.Fatal("leerOpciones aceptó una opción desconocida")
	}
}

func TestConsultarSaludSinAPI(t *testing.T) {
	if err := consultarSalud("http://127.0.0.1:1/api/salud"); err == nil {
		t.Fatal("consultarSalud no falló sin una API escuchando")
	}
}

func TestDireccionDeSalud(t *testing.T) {
	for addr, want := range map[string]string{
		":8080": "http://127.0.0.1:8080/api/salud", "0.0.0.0:9000": "http://127.0.0.1:9000/api/salud",
		"localhost:8081": "http://localhost:8081/api/salud", "[::]:8080": "http://127.0.0.1:8080/api/salud",
	} {
		if got := urlSalud(addr); got != want {
			t.Errorf("urlSalud(%q) = %q, se esperaba %q", addr, got, want)
		}
	}
}

func pedir(t *testing.T, metodo, url, token, cuerpo string) map[string]any {
	t.Helper()
	var rd io.Reader
	if cuerpo != "" {
		rd = strings.NewReader(cuerpo)
	}
	r, _ := http.NewRequest(metodo, url, rd)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var obj map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&obj); err != nil || resp.StatusCode != 200 {
		t.Fatalf("%s %s: %d, %v", metodo, url, resp.StatusCode, err)
	}
	return obj
}

// bufferSeguro deja que el log de la API (que escribe desde varias goroutines)
// y la prueba compartan el buffer.
type bufferSeguro struct {
	mu sync.Mutex
	b  *bytes.Buffer
}

func (s *bufferSeguro) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func logMudo() *slog.Logger { return slogDe(io.Discard) }

func slogDe(w io.Writer) *slog.Logger { return slog.New(slog.NewTextHandler(w, nil)) }
