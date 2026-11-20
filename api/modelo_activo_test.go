package api

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/R0SEWT/aire-distribuido/almacen"
	"github.com/R0SEWT/aire-distribuido/almacen/memoria"
	"github.com/R0SEWT/aire-distribuido/modelo"
)

// Activar un modelo lo deja guardado como activo en MongoDB y en Redis: es lo
// que la API lee al volver a arrancar.
func TestActivarModeloLoGuardaEnLosAlmacenes(t *testing.T) {
	ctx := context.Background()
	modelos, cache := memoria.NuevosModelos(), memoria.NuevaCache()
	e := nuevoEntorno(t, func(c *Config) { c.Modelos, c.CacheModelo = modelos, cache })
	// Nuevo no guarda: con qué modelo arranca y dónde queda lo decide cmd/api.
	if _, err := cache.ModeloActivo(ctx); !errors.Is(err, almacen.ErrNoEncontrado) {
		t.Fatalf("Nuevo guardó el modelo inicial: %v", err)
	}
	doc := e.srv.modelo.Load().Documento()
	doc.ID, doc.Origen = "entrenado-e-20261119-1530", "e-20261119-1530"
	entrenado, err := modelo.DesdeDocumento(doc)
	if err != nil {
		t.Fatal(err)
	}
	e.srv.ActivarModelo(entrenado)

	enRedis, err1 := cache.ModeloActivo(ctx)
	enMongo, err2 := modelos.Activo(ctx)
	if err1 != nil || err2 != nil || enRedis.ID != doc.ID || enMongo.ID != doc.ID || !enRedis.Activo || !enMongo.Activo || enMongo.Origen != doc.Origen {
		t.Fatalf("modelo:activo = %+v (%v), modelos activo = %+v (%v)", enRedis.ID, err1, enMongo.ID, err2)
	}
	// Y se predice con él.
	obj := objeto(t, e.pedir(t, "POST", "/api/predicciones", e.token(t, "demo", "demo"), lecturaEjemplo), 200)
	if obj["modelo_id"] != doc.ID {
		t.Fatalf("la predicción usó el modelo %v", obj["modelo_id"])
	}
}

// Sin almacenes de modelos (como en las pruebas que no los configuran) o con
// uno caído, activar el modelo funciona igual.
func TestActivarModeloNoDependeDeLosAlmacenes(t *testing.T) {
	e := nuevoEntorno(t)
	m := e.srv.modelo.Load()
	e.srv.ActivarModelo(m)
	e.srv.ActivarModelo(nil) // la API se queda sin modelo y responde 503

	var log strings.Builder
	modelos := memoria.NuevosModelos()
	e = nuevoEntorno(t, func(c *Config) {
		c.Modelos, c.CacheModelo = modelos, cacheModeloRota{}
		c.Log = slog.New(slog.NewTextHandler(&escritorSeguro{b: &log}, nil))
	})
	e.srv.ActivarModelo(m)
	if e.srv.modelo.Load() != m {
		t.Fatal("el modelo no quedó activo")
	}
	if a, err := modelos.Activo(context.Background()); err != nil || a.ID != m.ID {
		t.Fatalf("con Redis caído, MongoDB igual tiene que guardar el modelo: %v, %v", a.ID, err)
	}
	if !strings.Contains(log.String(), "no se guardó el modelo activo en Redis") {
		t.Fatalf("el fallo de Redis tiene que quedar en el log:\n%s", log.String())
	}
}

type cacheModeloRota struct{}

func (cacheModeloRota) GuardarModeloActivo(context.Context, almacen.Modelo) error {
	return errors.New("Redis no responde")
}

func (cacheModeloRota) ModeloActivo(context.Context) (almacen.Modelo, error) {
	return almacen.Modelo{}, errors.New("Redis no responde")
}
