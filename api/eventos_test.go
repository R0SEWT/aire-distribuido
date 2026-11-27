package api

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/R0SEWT/aire-distribuido/almacen"
	"github.com/R0SEWT/aire-distribuido/almacen/memoria"
	"github.com/R0SEWT/aire-distribuido/cluster"
	"github.com/R0SEWT/aire-distribuido/kmeans"
)

// Los eventos del clúster son de admin: sin token 401, con rol usuario 403.
// Sin eventos, una lista vacía.
func TestEventosDelClusterPermisosYVacio(t *testing.T) {
	e := nuevoEntorno(t)
	mensajeDeError(t, e.pedir(t, "GET", "/api/cluster/eventos", "", ""), 401, msgNoAutenticado)
	mensajeDeError(t, e.pedir(t, "GET", "/api/cluster/eventos", e.token(t, "demo", "demo"), ""), 403, "se requiere el rol admin")
	w := e.pedir(t, "GET", "/api/cluster/eventos", e.token(t, "admin", "clave-admin"), "")
	if l := lista(t, w, 200); len(l) != 0 || w.Body.String() != "[]\n" {
		t.Fatalf("sin eventos la lista tiene que ser []: %s", w.Body)
	}
}

// Del más reciente al más antiguo, con un límite por omisión de 50 y un
// máximo de 200; un limite que no es un entero en ese rango es un 400.
func TestEventosDelClusterOrdenYLimite(t *testing.T) {
	e := nuevoEntorno(t)
	admin := e.token(t, "admin", "clave-admin")
	t0 := time.Date(2026, 11, 19, 20, 0, 0, 0, time.UTC)
	const guardados = 230
	for i := range guardados {
		tipo := []string{cluster.EventoAlta, cluster.EventoBaja, cluster.EventoError}[i%3]
		if err := e.ev.Guardar(context.Background(), almacen.EventoCluster{
			TS: t0.Add(time.Duration(i) * time.Second), Tipo: tipo, Nodo: fmt.Sprintf("nodo-%d", i), Detalle: "10.0.0.1:5000: detalle",
		}); err != nil {
			t.Fatal(err)
		}
	}
	for consulta, n := range map[string]int{"": 50, "?limite=1": 1, "?limite=3": 3, "?limite=200": 200, "?limite=50&otro=1": 50} {
		w := e.pedir(t, "GET", "/api/cluster/eventos"+consulta, admin, "")
		l := lista(t, w, 200)
		if len(l) != n {
			t.Fatalf("%q trajo %d eventos, se esperaban %d", consulta, len(l), n)
		}
		for i, ev := range l {
			if ev["nodo"] != fmt.Sprintf("nodo-%d", guardados-1-i) {
				t.Fatalf("%q: el evento %d es de %v; tienen que venir del más reciente al más antiguo", consulta, i, ev["nodo"])
			}
			if len(ev) != 4 || ev["detalle"] != "10.0.0.1:5000: detalle" || !strings.Contains("alta baja error", ev["tipo"].(string)) {
				t.Fatalf("evento con otros campos que ts, tipo, nodo y detalle: %v", ev)
			}
		}
		// El más reciente: 20:03:49 UTC, que son las 15:03:49 en Lima.
		if l[0]["ts"] != "2026-11-19T15:03:49-05:00" {
			t.Fatalf("ts = %v; tiene que estar en la hora de Lima", l[0]["ts"])
		}
	}
	for _, mala := range []string{"0", "-1", "201", "abc", "1.5", "", "1e2", "999999999999999999999"} {
		mensajeDeError(t, e.pedir(t, "GET", "/api/cluster/eventos?limite="+mala, admin, ""), 400, "limite tiene que ser un entero entre 1 y 200")
	}
}

// Lo que el pool avisa (una alta, una baja) llega a la ruta pasando por el
// almacén, con la dirección del nodo delante del detalle.
func TestEventosDelClusterSonLosQueAvisaElPool(t *testing.T) {
	e := nuevoEntorno(t)
	admin := e.token(t, "admin", "clave-admin")
	ts := time.Date(2026, 11, 19, 15, 30, 4, 512_700_000, kmeans.ZonaPeru)
	e.srv.registrarEvento(cluster.EventoPool{TS: ts, Tipo: cluster.EventoAlta, Nodo: "H45-022", Direccion: "10.11.38.69:56455", Detalle: "24 workers, latencia 0.49 ms"})
	e.srv.registrarEvento(cluster.EventoPool{TS: ts.Add(time.Second), Tipo: cluster.EventoBaja, Nodo: "H45-022", Detalle: "dejó de responder el latido"})
	esperarHasta(t, "que los eventos lleguen al almacén", func() bool { return len(e.ev.Todos()) == 2 })
	l := lista(t, e.pedir(t, "GET", "/api/cluster/eventos", admin, ""), 200)
	if len(l) != 2 || l[0]["tipo"] != "baja" || l[0]["detalle"] != "dejó de responder el latido" ||
		l[1]["tipo"] != "alta" || l[1]["nodo"] != "H45-022" || l[1]["detalle"] != "10.11.38.69:56455: 24 workers, latencia 0.49 ms" ||
		l[1]["ts"] != "2026-11-19T15:30:04.512-05:00" {
		t.Fatalf("eventos = %v", l)
	}
}

// eventosRotos es un almacén de eventos que no responde.
type eventosRotos struct{ *memoria.Eventos }

func (eventosRotos) Recientes(context.Context, int) ([]almacen.EventoCluster, error) {
	return nil, errors.New("sin conexión")
}

func TestEventosDelClusterConMongoDBCaidoEs503(t *testing.T) {
	e := nuevoEntorno(t, func(c *Config) { c.Eventos = eventosRotos{memoria.NuevosEventos()} })
	mensajeDeError(t, e.pedir(t, "GET", "/api/cluster/eventos", e.token(t, "admin", "clave-admin"), ""), 503, "MongoDB no disponible")
}
