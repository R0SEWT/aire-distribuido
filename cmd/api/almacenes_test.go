package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/R0SEWT/aire-distribuido/almacen"
	"github.com/R0SEWT/aire-distribuido/almacen/memoria"
	"github.com/R0SEWT/aire-distribuido/modelo"
)

func fichaDelRepo(t *testing.T) []almacen.Estacion {
	t.Helper()
	f, err := os.Open("../../reports/estaciones_ficha.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	ficha, err := almacen.LeerFichaEstaciones(f)
	if err != nil {
		t.Fatal(err)
	}
	return ficha
}

func sembradoDelRepo(t *testing.T) *modelo.Modelo {
	t.Helper()
	m, err := modelo.Sembrar(modelo.Archivos{
		Kmeans: "../../reports/kmeans.json", Regimenes: "../../reports/regimenes.json", Estandarizacion: "../../reports/estandarizacion.json",
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func logA(b *bytes.Buffer) *slog.Logger { return slog.New(slog.NewTextHandler(b, nil)) }

// Sin MONGO_URI ni REDIS_ADDR la API usa el doble en memoria, como el CI, y lo
// avisa: lo que se guarde se pierde al apagar.
func TestSinVariablesLosAlmacenesEstanEnMemoria(t *testing.T) {
	var log bytes.Buffer
	a, err := abrirAlmacenes(entornoCon(map[string]string{"MONGO_DB": "otra"}), fichaDelRepo(t), logA(&log))
	if err != nil {
		t.Fatal(err)
	}
	defer a.cerrar()
	ctx := context.Background()
	if es, err := a.estaciones.Listar(ctx); err != nil || len(es) != 33 {
		t.Fatalf("%d estaciones en memoria (%v), se esperaban las 33 de la ficha", len(es), err)
	}
	if a.mongo.Ping(ctx) != nil || a.redis.Ping(ctx) != nil {
		t.Fatal("el doble en memoria siempre responde")
	}
	for _, aviso := range []string{"sin MONGO_URI", "sin REDIS_ADDR", "MONGO_DB no tiene efecto"} {
		if !strings.Contains(log.String(), aviso) {
			t.Errorf("falta el aviso %q en el log:\n%s", aviso, log.String())
		}
	}
	if strings.Count(log.String(), "level=WARN") != 3 {
		t.Errorf("se esperaban tres avisos:\n%s", log.String())
	}
}

func TestUnaVariableMalEscritaNoDejaArrancar(t *testing.T) {
	for nombre, c := range map[string]struct {
		vars  map[string]string
		texto string
	}{
		"URI que no es de MongoDB":  {map[string]string{"MONGO_URI": "localhost:27017"}, "MONGO_URI"},
		"REDIS_DB que no es número": {map[string]string{"REDIS_ADDR": "localhost:6379", "REDIS_DB": "cache"}, "REDIS_DB"},
		"REDIS_DB negativo":         {map[string]string{"REDIS_ADDR": "localhost:6379", "REDIS_DB": "-1"}, "REDIS_DB"},
	} {
		_, err := abrirAlmacenes(entornoCon(c.vars), fichaDelRepo(t), logMudo())
		if err == nil || !strings.Contains(err.Error(), c.texto) {
			t.Errorf("%s: %v; se esperaba un error que nombre %s", nombre, err, c.texto)
		}
	}
}

// Con las bases configuradas pero caídas, la API arranca igual: avisa en el
// log, /api/salud dirá cuál falta y el modelo es el de reports/.
func TestConLasBasesCaidasLaAPIArrancaIgual(t *testing.T) {
	if testing.Short() {
		t.Skip("espera el plazo de conexión de las dos bases")
	}
	var log bytes.Buffer
	a, err := abrirAlmacenes(entornoCon(map[string]string{
		"MONGO_URI": "mongodb://127.0.0.1:1", "REDIS_ADDR": "127.0.0.1:1",
	}), fichaDelRepo(t), logA(&log))
	if err != nil {
		t.Fatalf("una base que no responde no tiene que impedir el arranque: %v", err)
	}
	defer a.cerrar()
	if conPlazo(a.mongo.Ping) == nil || conPlazo(a.redis.Ping) == nil {
		t.Fatal("Ping respondió sin bases")
	}
	sembrado := sembradoDelRepo(t)
	if m := a.modeloActivo(sembrado, logA(&log)); m != sembrado {
		t.Fatalf("sin bases el modelo tiene que ser el de reports/, es %s", m.ID)
	}
	for _, aviso := range []string{"MongoDB no responde", "Redis no responde", "no se guardó el modelo activo en Redis", "no se guardó el modelo activo en MongoDB"} {
		if !strings.Contains(log.String(), aviso) {
			t.Errorf("falta el aviso %q en el log:\n%s", aviso, log.String())
		}
	}
}

// entrenado es el documento de un modelo que habría entrenado la API: el
// sembrado con otro id, otro origen y otra inercia.
func entrenado(t *testing.T) almacen.Modelo {
	doc := sembradoDelRepo(t).Documento()
	doc.ID, doc.Origen, doc.Inercia, doc.Activo = "m-e-20261119-1530", "e-20261119-1530", 12345.5, true
	return doc
}

func almacenesEnMemoria(t *testing.T) (*almacenes, *memoria.Cache, *memoria.Modelos) {
	t.Helper()
	cache, modelos := memoria.NuevaCache(), memoria.NuevosModelos()
	return &almacenes{cache: cache, cacheModelo: cache, modelos: modelos}, cache, modelos
}

// El modelo con que arranca la API (spec §6.2).
func TestModeloActivoAlArrancar(t *testing.T) {
	ctx := context.Background()
	sembrado := sembradoDelRepo(t)
	// Después de arrancar, las dos bases tienen guardado como activo el modelo
	// con que arrancó.
	comprobarGuardado := func(t *testing.T, cache *memoria.Cache, modelos *memoria.Modelos, id string) {
		t.Helper()
		enRedis, err1 := cache.ModeloActivo(ctx)
		enMongo, err2 := modelos.Activo(ctx)
		if err1 != nil || err2 != nil || enRedis.ID != id || enMongo.ID != id || !enRedis.Activo || !enMongo.Activo {
			t.Fatalf("modelo:activo = %q (%v) y modelos activo = %q (%v); se esperaba %q en los dos", enRedis.ID, err1, enMongo.ID, err2, id)
		}
	}

	t.Run("sin nada guardado, el de reports", func(t *testing.T) {
		a, cache, modelos := almacenesEnMemoria(t)
		if m := a.modeloActivo(sembrado, logMudo()); m != sembrado {
			t.Fatalf("modelo = %s", m.ID)
		}
		comprobarGuardado(t, cache, modelos, modelo.IDSembrado)
	})

	t.Run("un modelo entrenado en Redis", func(t *testing.T) {
		a, cache, modelos := almacenesEnMemoria(t)
		cache.GuardarModeloActivo(ctx, entrenado(t))
		var log bytes.Buffer
		m := a.modeloActivo(sembrado, logA(&log))
		if m.ID != "m-e-20261119-1530" || m.Origen != "e-20261119-1530" || m.Inercia != 12345.5 {
			t.Fatalf("modelo = %+v; se esperaba el entrenado que estaba en Redis", m)
		}
		if !strings.Contains(log.String(), "Redis (modelo:activo)") {
			t.Errorf("el log no dice de dónde salió el modelo:\n%s", log.String())
		}
		comprobarGuardado(t, cache, modelos, "m-e-20261119-1530")
	})

	t.Run("Redis vacío y un modelo entrenado activo en MongoDB", func(t *testing.T) {
		a, cache, modelos := almacenesEnMemoria(t)
		modelos.Guardar(ctx, entrenado(t))
		if m := a.modeloActivo(sembrado, logMudo()); m.ID != "m-e-20261119-1530" {
			t.Fatalf("modelo = %s; se esperaba el entrenado que estaba en MongoDB", m.ID)
		}
		comprobarGuardado(t, cache, modelos, "m-e-20261119-1530")
	})

	t.Run("una copia vieja del sembrado en Redis no tapa a reports", func(t *testing.T) {
		a, cache, modelos := almacenesEnMemoria(t)
		viejo := sembrado.Documento()
		viejo.Inercia, viejo.Activo = 999, true
		cache.GuardarModeloActivo(ctx, viejo)
		if m := a.modeloActivo(sembrado, logMudo()); m != sembrado {
			t.Fatalf("modelo = %+v; se esperaba el recién leído de reports/", m)
		}
		if enRedis, _ := cache.ModeloActivo(ctx); enRedis.Inercia != sembrado.Inercia {
			t.Fatalf("Redis sigue con la copia vieja: inercia %v", enRedis.Inercia)
		}
		comprobarGuardado(t, cache, modelos, modelo.IDSembrado)
	})

	t.Run("un modelo guardado que no se puede usar", func(t *testing.T) {
		a, cache, modelos := almacenesEnMemoria(t)
		roto := entrenado(t)
		roto.CentroidesStd = roto.CentroidesStd[:1]
		cache.GuardarModeloActivo(ctx, roto)
		var log bytes.Buffer
		if m := a.modeloActivo(sembrado, logA(&log)); m != sembrado {
			t.Fatalf("modelo = %s; con un documento roto se arranca con el de reports/", m.ID)
		}
		if !strings.Contains(log.String(), "level=WARN") || !strings.Contains(log.String(), "m-e-20261119-1530") {
			t.Errorf("el log tiene que avisar y nombrar el modelo descartado:\n%s", log.String())
		}
		comprobarGuardado(t, cache, modelos, modelo.IDSembrado)
	})

	// Las dos bases pueden guardar modelos distintos: ActivarModelo escribe
	// primero en una y después en la otra, y cualquiera de las dos escrituras
	// puede fallar. Gana el más nuevo por creado, esté donde esté.
	viejo, nuevo := entrenado(t), entrenado(t)
	viejo.Creado = time.Date(2026, 11, 19, 15, 30, 0, 0, time.UTC)
	nuevo.ID, nuevo.Origen, nuevo.Creado = "entrenado-e-20261120-1000", "e-20261120-1000", time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)

	t.Run("un modelo:activo viejo en Redis no le gana al activo más nuevo de MongoDB", func(t *testing.T) {
		a, cache, modelos := almacenesEnMemoria(t)
		cache.GuardarModeloActivo(ctx, viejo)
		modelos.Guardar(ctx, nuevo)
		var log bytes.Buffer
		if m := a.modeloActivo(sembrado, logA(&log)); m.ID != nuevo.ID {
			t.Fatalf("modelo = %s; se esperaba el más nuevo, %s, que estaba en MongoDB\n%s", m.ID, nuevo.ID, log.String())
		}
		if !strings.Contains(log.String(), "MongoDB (modelos)") {
			t.Errorf("el log no dice de dónde salió el modelo:\n%s", log.String())
		}
		comprobarGuardado(t, cache, modelos, nuevo.ID) // y Redis queda al día
	})

	t.Run("un activo viejo en MongoDB no le gana a un modelo:activo más nuevo", func(t *testing.T) {
		a, cache, modelos := almacenesEnMemoria(t)
		cache.GuardarModeloActivo(ctx, nuevo)
		modelos.Guardar(ctx, viejo)
		if m := a.modeloActivo(sembrado, logMudo()); m.ID != nuevo.ID {
			t.Fatalf("modelo = %s; se esperaba el más nuevo, %s, que estaba en Redis", m.ID, nuevo.ID)
		}
		comprobarGuardado(t, cache, modelos, nuevo.ID) // y MongoDB queda al día
	})

	t.Run("el mismo modelo en las dos, con el creado de MongoDB al milisegundo", func(t *testing.T) {
		a, cache, modelos := almacenesEnMemoria(t)
		enRedis, enMongo := nuevo, nuevo
		enRedis.Creado = nuevo.Creado.Add(123456 * time.Nanosecond) // Redis guarda los nanosegundos; MongoDB, no
		cache.GuardarModeloActivo(ctx, enRedis)
		modelos.Guardar(ctx, enMongo)
		var log bytes.Buffer
		if m := a.modeloActivo(sembrado, logA(&log)); m.ID != nuevo.ID || !strings.Contains(log.String(), "MongoDB (modelos)") {
			t.Fatalf("modelo = %s; con el mismo creado al milisegundo se toma el de MongoDB\n%s", m.ID, log.String())
		}
	})

	t.Run("el más nuevo no se puede usar: queda el otro", func(t *testing.T) {
		a, cache, modelos := almacenesEnMemoria(t)
		roto := nuevo
		roto.CentroidesStd = roto.CentroidesStd[:1]
		modelos.Guardar(ctx, roto)
		cache.GuardarModeloActivo(ctx, viejo)
		if m := a.modeloActivo(sembrado, logMudo()); m.ID != viejo.ID {
			t.Fatalf("modelo = %s; el de MongoDB está roto y se esperaba el de Redis, %s", m.ID, viejo.ID)
		}
		comprobarGuardado(t, cache, modelos, viejo.ID)
	})

	t.Run("MongoDB tiene activo el sembrado y Redis, uno entrenado", func(t *testing.T) {
		a, cache, modelos := almacenesEnMemoria(t)
		copia := sembrado.Documento()
		copia.Activo = true
		modelos.Guardar(ctx, copia)
		cache.GuardarModeloActivo(ctx, viejo)
		if m := a.modeloActivo(sembrado, logMudo()); m.ID != viejo.ID {
			t.Fatalf("modelo = %s; se esperaba el entrenado de Redis, %s", m.ID, viejo.ID)
		}
		comprobarGuardado(t, cache, modelos, viejo.ID)
	})

	t.Run("Redis no responde y MongoDB tiene el entrenado", func(t *testing.T) {
		a, _, modelos := almacenesEnMemoria(t)
		a.cacheModelo = cacheRota{}
		modelos.Guardar(ctx, entrenado(t))
		if m := a.modeloActivo(sembrado, logMudo()); m.ID != "m-e-20261119-1530" {
			t.Fatalf("modelo = %s; se esperaba el de MongoDB", m.ID)
		}
	})
}

type cacheRota struct{}

func (cacheRota) GuardarModeloActivo(context.Context, almacen.Modelo) error {
	return errors.New("Redis no responde")
}

func (cacheRota) ModeloActivo(context.Context) (almacen.Modelo, error) {
	return almacen.Modelo{}, errors.New("Redis no responde")
}

// Cada operación del arranque tiene su propio plazo: un Redis colgado, que
// consume entero el suyo, no le quita tiempo a MongoDB, que responde. La API
// arranca con el modelo entrenado que MongoDB tiene activo y lo deja guardado.
func TestUnRedisColgadoNoDejaSinPlazoAMongoDB(t *testing.T) {
	original := plazoArranque
	plazoArranque = 50 * time.Millisecond
	t.Cleanup(func() { plazoArranque = original })

	ctx := context.Background()
	a, _, modelos := almacenesEnMemoria(t)
	a.cacheModelo = cacheColgada{}
	a.modelos = modelosConPlazo{modelos}
	modelos.Guardar(ctx, entrenado(t))
	var log bytes.Buffer
	inicio := time.Now()
	m := a.modeloActivo(sembradoDelRepo(t), logA(&log))
	if m.ID != "m-e-20261119-1530" {
		t.Fatalf("modelo = %s; con Redis colgado se esperaba el entrenado de MongoDB\n%s", m.ID, log.String())
	}
	if activo, err := modelos.Activo(ctx); err != nil || activo.ID != m.ID {
		t.Fatalf("modelos activo = %q, %v", activo.ID, err)
	}
	// Redis consumió sus dos plazos (leer y guardar) y nada más.
	if d := time.Since(inicio); d < 2*plazoArranque || d > 2*plazoArranque+time.Second {
		t.Fatalf("el arranque tardó %v con un plazo de %v por operación", d, plazoArranque)
	}
	if n := strings.Count(log.String(), "level=WARN"); n != 2 {
		t.Fatalf("se esperaban dos avisos, los de Redis:\n%s", log.String())
	}
}

// cacheColgada no responde hasta que vence el plazo de quien la llama.
type cacheColgada struct{}

func (cacheColgada) GuardarModeloActivo(ctx context.Context, _ almacen.Modelo) error {
	<-ctx.Done()
	return ctx.Err()
}

func (cacheColgada) ModeloActivo(ctx context.Context) (almacen.Modelo, error) {
	<-ctx.Done()
	return almacen.Modelo{}, ctx.Err()
}

// modelosConPlazo falla, como una base real, si el contexto ya venció.
type modelosConPlazo struct{ *memoria.Modelos }

func (m modelosConPlazo) Guardar(ctx context.Context, mod almacen.Modelo) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return m.Modelos.Guardar(ctx, mod)
}

func (m modelosConPlazo) Activo(ctx context.Context) (almacen.Modelo, error) {
	if err := ctx.Err(); err != nil {
		return almacen.Modelo{}, err
	}
	return m.Modelos.Activo(ctx)
}
