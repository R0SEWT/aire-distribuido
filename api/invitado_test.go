package api

import (
	"strings"
	"testing"
	"time"
)

// Con una cuenta de invitado, POST /api/auth/invitado entrega un token de esa
// cuenta sin pedir nada, con la misma forma que POST /api/auth/token.
func TestInvitadoHabilitado(t *testing.T) {
	e := nuevoEntorno(t, func(c *Config) { c.Invitado = "demo" })
	w := e.pedir(t, "POST", "/api/auth/invitado", "", "")
	obj := objeto(t, w, 200, "token", "rol", "expira")
	if len(obj) != 3 {
		t.Fatalf("la respuesta tiene que traer token, rol y expira, y nada más: %s", w.Body)
	}
	if obj["rol"] != RolUsuario {
		t.Fatalf("rol = %v, el invitado tiene el rol %s", obj["rol"], RolUsuario)
	}
	expira, err := time.Parse(time.RFC3339, obj["expira"].(string))
	if err != nil || !strings.HasSuffix(obj["expira"].(string), "-05:00") {
		t.Errorf("expira = %v (%v): tiene que ser RFC3339 en hora de Lima", obj["expira"], err)
	}
	if d := time.Until(expira); d < 59*time.Minute || d > time.Hour {
		t.Errorf("el token vence en %v, se esperaba una hora", d)
	}
	// El token es de la cuenta de invitado, firmado como cualquier otro.
	rec, err := e.srv.firmador.Verificar(obj["token"].(string))
	if err != nil || rec.Usuario != "demo" || rec.Rol != RolUsuario {
		t.Fatalf("el token es de %+v (%v), se esperaba el de demo con el rol %s", rec, err, RolUsuario)
	}
	// La clave de la cuenta no viaja en la respuesta.
	if strings.Contains(w.Body.String(), `"clave"`) {
		t.Fatalf("la respuesta nombra una clave: %s", w.Body)
	}
	// El pedido no lleva cuerpo; si trae uno, no se lee.
	objeto(t, e.pedir(t, "POST", "/api/auth/invitado", "", `{"usuario": "admin", "clave": "clave-admin"}`), 200, "token")
	if obj := objeto(t, e.pedir(t, "POST", "/api/auth/invitado", "", `no es json`), 200, "rol"); obj["rol"] != RolUsuario {
		t.Fatalf("con un cuerpo cualquiera, rol = %v", obj["rol"])
	}
}

// El token de invitado sirve donde sirve el rol usuario, y en ningún otro lado.
func TestElTokenDeInvitadoTieneElRolUsuario(t *testing.T) {
	e := nuevoEntorno(t, func(c *Config) { c.Invitado = "demo" })
	tok := objeto(t, e.pedir(t, "POST", "/api/auth/invitado", "", ""), 200, "token")["token"].(string)
	objeto(t, e.pedir(t, "POST", "/api/predicciones", tok, lecturaEjemplo), 200, camposPrediccion...)
	if w := e.pedir(t, "GET", "/api/estaciones", tok, ""); w.Code != 200 {
		t.Fatalf("GET /api/estaciones con el token de invitado: %d %s", w.Code, w.Body)
	}
	objeto(t, e.pedir(t, "GET", "/api/modelos/activo", tok, ""), 200)
	for _, ruta := range [][2]string{
		{"GET", "/api/metricas"}, {"GET", "/api/cluster/nodos"}, {"GET", "/api/cluster/eventos"},
		{"GET", "/api/entrenamientos"}, {"POST", "/api/entrenamientos"},
	} {
		mensajeDeError(t, e.pedir(t, ruta[0], ruta[1], tok, ""), 403, "se requiere el rol admin")
	}
}

// Sin cuenta de invitado la ruta existe y responde 404 con el esquema Error;
// con otro método, 405 como cualquier ruta.
func TestInvitadoDeshabilitado(t *testing.T) {
	e := nuevoEntorno(t)
	mensajeDeError(t, e.pedir(t, "POST", "/api/auth/invitado", "", ""), 404, "el ingreso de invitado no está habilitado")
	// Tampoco con un token válido: no es cuestión de permisos.
	mensajeDeError(t, e.pedir(t, "POST", "/api/auth/invitado", e.token(t, "admin", "clave-admin"), ""), 404,
		"el ingreso de invitado no está habilitado")
	if w := e.pedir(t, "GET", "/api/auth/invitado", "", ""); w.Code != 405 || w.Header().Get("Allow") != "POST" {
		t.Fatalf("GET /api/auth/invitado: %d, Allow %q; se esperaba 405 con Allow: POST", w.Code, w.Header().Get("Allow"))
	}
}

// El invitado entra sin clave: tiene que ser un usuario que exista, y nunca un admin.
func TestValidarInvitado(t *testing.T) {
	usuarios, _ := LeerUsuarios("demo:demo:usuario,admin:clave-admin:admin")
	for _, bueno := range []string{"", "demo"} {
		if err := ValidarInvitado(usuarios, bueno); err != nil {
			t.Errorf("ValidarInvitado(%q) = %v", bueno, err)
		}
	}
	for malo, texto := range map[string]string{"nadie": "no está entre los usuarios", "admin": "rol admin", "Demo": "no está entre los usuarios"} {
		err := ValidarInvitado(usuarios, malo)
		if err == nil || !strings.Contains(err.Error(), texto) || !strings.Contains(err.Error(), malo) {
			t.Errorf("ValidarInvitado(%q) = %v; se esperaba un error que nombre al usuario y diga %q", malo, err, texto)
		}
		if err != nil && (strings.Contains(err.Error(), "clave-admin") || strings.Contains(err.Error(), "demo:demo")) {
			t.Errorf("el error de %q trae una clave: %v", malo, err)
		}
	}
}
