package api

import (
	"net/http"
	"net/url"
	"slices"
	"strings"
)

// cors decide qué orígenes pueden llamar a la API desde un navegador. La SPA
// del TB2 se sirve desde otro origen (otro puerto, o Vite en desarrollo), así
// que el navegador le exige a la API los encabezados de CORS y, antes de
// cualquier pedido con Authorization o con cuerpo JSON, le manda un preflight
// OPTIONS.
//
// Los orígenes permitidos se configuran en el servidor (cmd/api los lee de la
// variable de entorno CORS_ORIGENES). Sin ninguno, la API no manda ningún
// encabezado de CORS y un navegador solo puede usarla desde su mismo origen
// (Swagger UI en /docs). La API autentica con un JWT en Authorization, no con
// cookies: por eso no hace falta Access-Control-Allow-Credentials, y permitir
// un origen no le presta a nadie la sesión de otro.
type cors struct {
	todos    bool // «*»: cualquier origen
	origenes []string
}

// LeerOrigenesCORS lee la lista de orígenes separados por comas, como llega en
// CORS_ORIGENES: «http://localhost:5173,https://aire.example.com», o «*» para
// cualquiera. Cada origen es esquema://host[:puerto], sin ruta ni barra final,
// que es como el navegador lo manda en Origin.
func LeerOrigenesCORS(s string) ([]string, error) {
	var out []string
	for _, o := range strings.Split(s, ",") {
		o = strings.TrimSpace(o)
		if o == "" {
			continue
		}
		if o != "*" {
			u, err := url.Parse(o)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
				return nil, &errorOrigen{o}
			}
			o = strings.ToLower(o)
		}
		if !slices.Contains(out, o) {
			out = append(out, o)
		}
	}
	return out, nil
}

type errorOrigen struct{ origen string }

func (e *errorOrigen) Error() string {
	return "CORS: «" + e.origen + "» no es un origen: tiene que ser esquema://host[:puerto], sin ruta (o «*»)"
}

func nuevoCORS(origenes []string) *cors {
	return &cors{todos: slices.Contains(origenes, "*"), origenes: origenes}
}

func (c *cors) permite(origen string) bool {
	return c.todos || slices.Contains(c.origenes, strings.ToLower(origen))
}

// Lo que un pedido de otro origen puede usar. Authorization y Content-Type son
// los dos encabezados que la API lee de un cliente.
const (
	corsMetodos     = "GET, POST, OPTIONS"
	corsEncabezados = "Authorization, Content-Type"
	corsMaxAge      = "600" // segundos que el navegador recuerda el preflight
)

// atender pone los encabezados de CORS que correspondan y dice si el pedido ya
// quedó respondido (era un preflight). Un pedido sin Origin no es de otro
// origen y pasa sin cambios.
func (c *cors) atender(w http.ResponseWriter, r *http.Request) bool {
	origen := r.Header.Get("Origin")
	preflight := r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != ""
	if origen == "" {
		return false
	}
	// La respuesta depende de Origin: una caché no puede servirle a un origen
	// la respuesta de otro.
	w.Header().Add("Vary", "Origin")
	if !c.permite(origen) {
		if preflight {
			escribirError(w, http.StatusForbidden, "CORS: el origen "+recortar(origen, 128)+" no está permitido")
			return true
		}
		return false // sin encabezados: el servidor responde y el navegador no deja leer
	}
	if c.todos {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	} else {
		w.Header().Set("Access-Control-Allow-Origin", origen)
	}
	if !preflight {
		return false
	}
	w.Header().Add("Vary", "Access-Control-Request-Method")
	w.Header().Add("Vary", "Access-Control-Request-Headers")
	w.Header().Set("Access-Control-Allow-Methods", corsMetodos)
	w.Header().Set("Access-Control-Allow-Headers", corsEncabezados)
	w.Header().Set("Access-Control-Max-Age", corsMaxAge)
	w.WriteHeader(http.StatusNoContent)
	return true
}

// permiteWebSocket decide si se acepta el apretón de manos de un WebSocket.
// CORS no se aplica a un WebSocket: el navegador lo abre desde cualquier
// página y solo informa el Origin, así que es el servidor el que tiene que
// mirarlo. Se acepta sin Origin (no es un navegador), desde el mismo host que
// la API, o desde un origen permitido.
func (c *cors) permiteWebSocket(r *http.Request) bool {
	origen := r.Header.Get("Origin")
	if origen == "" || c.permite(origen) {
		return true
	}
	u, err := url.Parse(origen)
	return err == nil && strings.EqualFold(u.Host, r.Host)
}
