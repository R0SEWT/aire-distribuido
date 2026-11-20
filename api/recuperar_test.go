package api

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/R0SEWT/aire-distribuido/almacen"
	"github.com/R0SEWT/aire-distribuido/almacen/memoria"
)

// historial deja en el almacén lo que una API anterior habría dejado: un
// entrenamiento terminado, uno abortado y uno que estaba en curso cuando murió.
func historial(t *testing.T) *memoria.Entrenamientos {
	t.Helper()
	ctx := context.Background()
	ent := memoria.NuevosEntrenamientos()
	t0 := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	for _, e := range []almacen.Entrenamiento{
		{ID: "e-20261120-0500", Inicio: t0},
		{ID: "e-20261120-0510", Inicio: t0.Add(10 * time.Minute)},
		{ID: "e-20261120-0520", Inicio: t0.Add(20 * time.Minute), Parametros: almacen.ParametrosEntrenamiento{Nodos: 3, Replicas: 1, Semilla: 42}},
	} {
		e.Estado = almacen.EstadoEnCurso
		if err := ent.Crear(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	ent.Cerrar(ctx, "e-20261120-0500", almacen.CierreEntrenamiento{Fin: t0.Add(time.Second), Estado: almacen.EstadoTerminado, Iteraciones: 44, Paro: "tolerancia", MsTotal: 31.2})
	ent.Cerrar(ctx, "e-20261120-0510", almacen.CierreEntrenamiento{Fin: t0.Add(11 * time.Minute), Estado: almacen.EstadoAbortado, Error: "el nodo \"a\" no respondió"})
	for i := 1; i <= 2; i++ { // el que estaba en curso alcanzó a guardar dos rondas
		ent.AgregarRonda(ctx, "e-20261120-0520", almacen.Ronda{Iter: i, Inercia: 100 / float64(i)})
	}
	return ent
}

// Al arrancar, la API recupera del almacén lo que el coordinador tenía solo en
// memoria: cuál fue el último entrenamiento, que /api/metricas informa; y
// cierra los que quedaron en curso cuando la API anterior murió, que si no
// quedarían en_curso para siempre.
func TestAlArrancarSeRecuperaElUltimoEntrenamientoYSeCierranLosHuerfanos(t *testing.T) {
	ctx := context.Background()
	ent := historial(t)
	e := nuevoEntorno(t, func(c *Config) { c.Entrenamientos = ent })
	admin := e.token(t, "admin", "clave-admin")
	if mt := objeto(t, e.pedir(t, "GET", "/api/metricas", admin, ""), 200); mt["ultimo_entrenamiento"] != nil {
		t.Fatalf("antes de recuperar, ultimo_entrenamiento = %v", mt["ultimo_entrenamiento"])
	}
	ahora := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	e.srv.ahora = func() time.Time { return ahora }

	e.srv.RecuperarEntrenamientos(time.Second)

	// El último por inicio, aunque no haya terminado bien.
	if mt := objeto(t, e.pedir(t, "GET", "/api/metricas", admin, ""), 200); mt["ultimo_entrenamiento"] != "e-20261120-0520" {
		t.Fatalf("ultimo_entrenamiento = %v, se esperaba e-20261120-0520", mt["ultimo_entrenamiento"])
	}
	// El huérfano quedó abortado, con el motivo y sus rondas.
	det := objeto(t, e.pedir(t, "GET", "/api/entrenamientos/e-20261120-0520", admin, ""), 200, "estado", "error", "rondas")
	errTexto, _ := det["error"].(string)
	if det["estado"] != "abortado" || !strings.Contains(errTexto, "se reinició") || len(det["rondas"].([]any)) != 2 {
		t.Fatalf("el entrenamiento que estaba en curso = %v", det)
	}
	doc, _ := ent.Obtener(ctx, "e-20261120-0520")
	if !doc.Fin.Equal(ahora) || doc.Parametros.Semilla != 42 {
		t.Fatalf("documento del huérfano = %+v; el fin es el momento en que se lo cerró", doc)
	}
	// Los que ya habían terminado no se tocan.
	if a, _ := ent.Obtener(ctx, "e-20261120-0500"); a.Estado != almacen.EstadoTerminado || a.Iteraciones != 44 {
		t.Fatalf("el terminado cambió: %+v", a)
	}
	if b, _ := ent.Obtener(ctx, "e-20261120-0510"); b.Estado != almacen.EstadoAbortado || !strings.Contains(b.Error, "no respondió") {
		t.Fatalf("el abortado cambió: %+v", b)
	}
	// Recuperar otra vez no cambia nada.
	e.srv.ahora = func() time.Time { return ahora.Add(time.Hour) }
	e.srv.RecuperarEntrenamientos(time.Second)
	if doc, _ := ent.Obtener(ctx, "e-20261120-0520"); !doc.Fin.Equal(ahora) {
		t.Fatalf("recuperar dos veces volvió a cerrar el huérfano: fin = %v", doc.Fin)
	}
}

// Un entrenamiento lanzado después de arrancar pasa a ser el último.
func TestUnEntrenamientoNuevoReemplazaAlRecuperado(t *testing.T) {
	ent := historial(t)
	ec := nuevoEntornoCluster(t, func(c *Config) { c.Entrenamientos = ent })
	ec.srv.RecuperarEntrenamientos(time.Second)
	ec.lanzarNodos(t, 2)
	id := ec.entrenar(t, `{"nodos": 2, "replicas": 1, "semilla": 42}`)
	if det := ec.esperarFin(t, id); det["estado"] != "terminado" {
		t.Fatalf("detalle = %v", det)
	}
	if mt := objeto(t, ec.pedir(t, "GET", "/api/metricas", ec.admin, ""), 200); mt["ultimo_entrenamiento"] != id {
		t.Fatalf("ultimo_entrenamiento = %v, se esperaba %s", mt["ultimo_entrenamiento"], id)
	}
}

// Si la base no responde, la API arranca igual: no se recupera nada, queda el
// aviso en el log y la espera no pasa del plazo.
func TestSiElAlmacenNoRespondeLaAPIArrancaIgual(t *testing.T) {
	var log strings.Builder
	e := nuevoEntorno(t, func(c *Config) {
		c.Entrenamientos = entrenamientosColgados{}
		c.Log = slog.New(slog.NewTextHandler(&escritorSeguro{b: &log}, nil))
	})
	inicio := time.Now()
	e.srv.RecuperarEntrenamientos(50 * time.Millisecond)
	if d := time.Since(inicio); d < 50*time.Millisecond || d > time.Second {
		t.Fatalf("RecuperarEntrenamientos tardó %v con un plazo de 50 ms", d)
	}
	if mt := objeto(t, e.pedir(t, "GET", "/api/metricas", e.token(t, "admin", "clave-admin"), ""), 200); mt["ultimo_entrenamiento"] != nil {
		t.Fatalf("ultimo_entrenamiento = %v sin almacén", mt["ultimo_entrenamiento"])
	}
	if !strings.Contains(log.String(), "level=WARN") || !strings.Contains(log.String(), "entrenamientos") {
		t.Fatalf("el log tiene que avisar que no se pudo leer el historial:\n%s", log.String())
	}
}

// entrenamientosColgados no responde hasta que vence el plazo de quien llama.
type entrenamientosColgados struct{ almacen.Entrenamientos }

func (entrenamientosColgados) Listar(ctx context.Context) ([]almacen.Entrenamiento, error) {
	<-ctx.Done()
	return nil, errors.Join(errors.New("MongoDB no responde"), ctx.Err())
}
