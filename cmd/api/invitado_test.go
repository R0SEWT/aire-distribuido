package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const usuariosConInvitado = "ana:una-clave-larga:admin,visita:otra-clave-larga:usuario"

// pedirInvitado pide POST /api/auth/invitado al servidor armado y devuelve el
// código y el cuerpo.
func pedirInvitado(t *testing.T, srv http.Handler) (int, map[string]any) {
	t.Helper()
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, httptest.NewRequest("POST", "/api/auth/invitado", nil))
	var obj map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &obj); err != nil {
		t.Fatalf("POST /api/auth/invitado: %d, el cuerpo no es un objeto JSON: %s", w.Code, w.Body)
	}
	return w.Code, obj
}

// API_INVITADO nombra la cuenta con que se entra sin clave. Al arrancar queda
// en el log con qué usuario, y nunca su clave.
func TestInvitadoPorVariableDeEntorno(t *testing.T) {
	op, err := leerOpciones(argsDelRepo)
	if err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	srv, cerrar, err := armar(op, entornoCon(map[string]string{
		"JWT_SECRETO": secreto, "API_USUARIOS": usuariosConInvitado, "API_INVITADO": " visita ",
	}), slogDe(&log), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cerrar()
	codigo, obj := pedirInvitado(t, srv)
	if codigo != http.StatusOK || obj["rol"] != "usuario" || obj["token"] == "" || obj["expira"] == nil {
		t.Fatalf("POST /api/auth/invitado: %d %v; se esperaba un token con el rol usuario", codigo, obj)
	}
	// El token sirve en una ruta de usuario y no en una de admin.
	for ruta, quiere := range map[string]int{"/api/modelos/activo": http.StatusOK, "/api/metricas": http.StatusForbidden} {
		r := httptest.NewRequest("GET", ruta, nil)
		r.Header.Set("Authorization", "Bearer "+obj["token"].(string))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, r)
		if w.Code != quiere {
			t.Errorf("GET %s con el token de invitado: %d, se esperaba %d", ruta, w.Code, quiere)
		}
	}
	if !strings.Contains(log.String(), "invitado habilitado") || !strings.Contains(log.String(), "usuario=visita") {
		t.Errorf("el log tiene que decir que el invitado está habilitado y con qué usuario:\n%s", log.String())
	}
	if strings.Contains(log.String(), "clave-larga") {
		t.Errorf("el log trae una clave:\n%s", log.String())
	}
}

// Sin API_INVITADO no hay ingreso de invitado: la ruta responde 404 y el log lo dice.
func TestSinInvitado(t *testing.T) {
	op, err := leerOpciones(argsDelRepo)
	if err != nil {
		t.Fatal(err)
	}
	for nombre, vars := range map[string]map[string]string{
		"sin la variable": {"JWT_SECRETO": secreto, "API_USUARIOS": usuariosConInvitado},
		"vacía":           {"JWT_SECRETO": secreto, "API_USUARIOS": usuariosConInvitado, "API_INVITADO": ""},
		"solo espacios":   {"JWT_SECRETO": secreto, "API_USUARIOS": usuariosConInvitado, "API_INVITADO": "  "},
	} {
		t.Run(nombre, func(t *testing.T) {
			var log bytes.Buffer
			srv, cerrar, err := armar(op, entornoCon(vars), slogDe(&log), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer cerrar()
			codigo, obj := pedirInvitado(t, srv)
			if codigo != http.StatusNotFound || obj["error"] != "el ingreso de invitado no está habilitado" {
				t.Fatalf("POST /api/auth/invitado: %d %v; se esperaba 404 con su error", codigo, obj)
			}
			if !strings.Contains(log.String(), "invitado deshabilitado") {
				t.Errorf("el log tiene que decir que el invitado está deshabilitado:\n%s", log.String())
			}
		})
	}
}

// Un invitado que no existe, o que es admin, no deja arrancar: el error nombra
// la variable y al usuario, y no trae ninguna clave.
func TestInvitadoQueNoDejaArrancar(t *testing.T) {
	op, err := leerOpciones(argsDelRepo)
	if err != nil {
		t.Fatal(err)
	}
	for invitado, texto := range map[string]string{"nadie": "no está entre los usuarios", "ana": "rol admin"} {
		_, _, err := armar(op, entornoCon(map[string]string{
			"JWT_SECRETO": secreto, "API_USUARIOS": usuariosConInvitado, "API_INVITADO": invitado,
		}), logMudo(), nil)
		if err == nil || !strings.Contains(err.Error(), "API_INVITADO") || !strings.Contains(err.Error(), invitado) ||
			!strings.Contains(err.Error(), texto) {
			t.Errorf("API_INVITADO=%s: %v; se esperaba un error que nombre la variable, al usuario y diga %q", invitado, err, texto)
		}
		if err != nil && strings.Contains(err.Error(), "clave-larga") {
			t.Errorf("API_INVITADO=%s: el error trae una clave: %v", invitado, err)
		}
	}
}

// Con -demo el invitado por omisión es demo; API_INVITADO puede nombrar otro,
// con las mismas reglas.
func TestInvitadoConDemo(t *testing.T) {
	op, err := leerOpciones(append([]string{"-demo"}, argsDelRepo...))
	if err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	srv, cerrar, err := armar(op, entornoCon(map[string]string{"JWT_SECRETO": secreto}), slogDe(&log), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cerrar()
	if codigo, obj := pedirInvitado(t, srv); codigo != http.StatusOK || obj["rol"] != "usuario" {
		t.Fatalf("con -demo, POST /api/auth/invitado: %d %v; se esperaba el token de demo", codigo, obj)
	}
	if !strings.Contains(log.String(), "invitado habilitado") || !strings.Contains(log.String(), "usuario=demo") {
		t.Errorf("con -demo el log tiene que decir que el invitado es demo:\n%s", log.String())
	}
	// El admin de demostración no puede ser el invitado.
	if _, _, err := armar(op, entornoCon(map[string]string{"JWT_SECRETO": secreto, "API_INVITADO": "admin"}), logMudo(), nil); err == nil ||
		!strings.Contains(err.Error(), "API_INVITADO") {
		t.Fatalf("con -demo y API_INVITADO=admin: %v; tiene que negarse a arrancar", err)
	}
	// -demo no tiene efecto con API_USUARIOS definida: tampoco pone un invitado.
	srv, cerrar2, err := armar(op, entornoCon(map[string]string{"JWT_SECRETO": secreto, "API_USUARIOS": usuariosConInvitado}), logMudo(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cerrar2()
	if codigo, _ := pedirInvitado(t, srv); codigo != http.StatusNotFound {
		t.Fatalf("con -demo sin efecto, POST /api/auth/invitado: %d, se esperaba 404", codigo)
	}
}
