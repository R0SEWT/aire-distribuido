package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/R0SEWT/aire-distribuido/almacen"
	"github.com/R0SEWT/aire-distribuido/almacen/memoria"
	"github.com/R0SEWT/aire-distribuido/almacen/mongo"
	"github.com/R0SEWT/aire-distribuido/almacen/redis"
	"github.com/R0SEWT/aire-distribuido/modelo"
)

// baseMongoPorOmision es la base de datos que siembra docs/datos/seed.js.
const baseMongoPorOmision = "aire_distribuido"

// plazoArranque es lo que la API espera a cada base al arrancar, para el aviso
// del log y para leer y guardar el modelo activo.
const plazoArranque = 3 * time.Second

// almacenes es lo que la API guarda y consulta, ya elegido entre las bases
// reales y el doble en memoria.
type almacenes struct {
	// En MongoDB.
	estaciones     almacen.Estaciones
	predicciones   almacen.Predicciones
	modelos        almacen.Modelos
	entrenamientos almacen.Entrenamientos
	eventos        almacen.Eventos
	// En Redis.
	cache       almacen.Cache
	cacheModelo almacen.CacheModelo
	puntos      almacen.PuntosControl

	mongo, redis almacen.Pinger // lo que /api/salud informa
	cierres      []func()
}

// cerrar cierra las conexiones que se hayan abierto.
func (a *almacenes) cerrar() {
	for _, c := range a.cierres {
		c()
	}
}

// abrirAlmacenes elige cada almacén por las variables de entorno:
//
//   - MONGO_URI (por ejemplo mongodb://mongo:27017): con ella, las colecciones
//     están en MongoDB, en la base MONGO_DB (por omisión aire_distribuido); sin
//     ella, en el doble en memoria, con las estaciones de la ficha.
//   - REDIS_ADDR (host:puerto): con ella, la caché es Redis, en la base
//     REDIS_DB (por omisión la 0); sin ella, el doble en memoria.
//
// Una variable mal escrita impide arrancar. Una base que no responde, no: la
// predicción no depende de ellas, /api/salud dice cuál falta, /api/metricas
// cuenta lo que no se pudo escribir y el driver se vuelve a conectar solo.
func abrirAlmacenes(getenv func(string) string, ficha []almacen.Estacion, log *slog.Logger) (*almacenes, error) {
	a := &almacenes{}
	if uri := getenv("MONGO_URI"); uri != "" {
		base := cmp.Or(getenv("MONGO_DB"), baseMongoPorOmision)
		m, err := mongo.Conectar(uri, base)
		if err != nil {
			return nil, fmt.Errorf("MONGO_URI: %w", err)
		}
		a.cierres = append(a.cierres, func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_ = m.Cerrar(ctx)
		})
		a.estaciones, a.predicciones, a.modelos, a.mongo = m.Estaciones(), m.Predicciones(), m.Modelos(), m
		a.entrenamientos, a.eventos = m.Entrenamientos(), m.Eventos()
		avisar(log, "MongoDB", m, "base", base)
	} else {
		if base := getenv("MONGO_DB"); base != "" {
			log.Warn("MONGO_DB no tiene efecto: falta MONGO_URI", "base", base)
		}
		estaciones, err := memoria.NuevasEstaciones(ficha)
		if err != nil {
			return nil, err
		}
		a.estaciones, a.predicciones, a.modelos, a.mongo = estaciones, memoria.NuevasPredicciones(), memoria.NuevosModelos(), estaciones
		a.entrenamientos, a.eventos = memoria.NuevosEntrenamientos(), memoria.NuevosEventos()
		log.Warn("sin MONGO_URI: las colecciones están en memoria y se pierden al apagar", "estaciones", len(ficha))
	}
	if addr := getenv("REDIS_ADDR"); addr != "" {
		db := 0
		if v := getenv("REDIS_DB"); v != "" {
			var err error
			if db, err = strconv.Atoi(v); err != nil || db < 0 {
				a.cerrar()
				return nil, fmt.Errorf("REDIS_DB tiene que ser un número de base de datos (0, 1, …), es %q", v)
			}
		}
		r, err := redis.Conectar(redis.Opciones{Addr: addr, DB: db})
		if err != nil {
			a.cerrar()
			return nil, fmt.Errorf("REDIS_ADDR: %w", err)
		}
		a.cierres = append(a.cierres, func() { _ = r.Cerrar() })
		a.cache, a.cacheModelo, a.puntos, a.redis = r, r, r, r
		avisar(log, "Redis", r, "addr", addr, "db", db)
	} else {
		cache := memoria.NuevaCache()
		a.cache, a.cacheModelo, a.puntos, a.redis = cache, cache, cache, cache
		log.Warn("sin REDIS_ADDR: la caché está en memoria y se pierde al apagar")
	}
	return a, nil
}

// avisar deja en el log si la base responde al arrancar.
func avisar(log *slog.Logger, nombre string, p almacen.Pinger, datos ...any) {
	ctx, cancel := context.WithTimeout(context.Background(), plazoArranque)
	defer cancel()
	if err := p.Ping(ctx); err != nil {
		log.Warn(nombre+" no responde: la API arranca igual y /api/salud lo informa", append(datos, "err", err)...)
		return
	}
	log.Info(nombre+" conectado", datos...)
}

// modeloActivo decide con qué modelo arranca la API (spec §6.2) y lo deja
// guardado como activo en Redis y en MongoDB.
//
// Si Redis (modelo:activo) o, en su defecto, MongoDB (el modelo con activo =
// true) guardan un modelo que entrenó la API, se arranca con ese: es el
// arranque en caliente, que no pierde un entrenamiento al reiniciar. Si no hay
// ninguno, si lo guardado no se puede usar o si es el modelo sembrado, se
// arranca con sembrado, el que se acaba de leer de reports/: los archivos son
// la fuente del modelo sembrado, y así un reports/ nuevo no queda tapado por
// una copia vieja en Redis.
func (a *almacenes) modeloActivo(sembrado *modelo.Modelo, log *slog.Logger) *modelo.Modelo {
	ctx, cancel := context.WithTimeout(context.Background(), plazoArranque)
	defer cancel()
	m, fuente := sembrado, "reports/"
	for _, f := range []struct {
		nombre string
		leer   func(context.Context) (almacen.Modelo, error)
	}{
		{"Redis (modelo:activo)", a.cacheModelo.ModeloActivo},
		{"MongoDB (modelos)", a.modelos.Activo},
	} {
		doc, err := f.leer(ctx)
		if errors.Is(err, almacen.ErrNoEncontrado) || (err == nil && doc.Origen == almacen.OrigenSembrado) {
			continue
		}
		if err != nil {
			log.Warn("no se pudo leer el modelo activo", "de", f.nombre, "err", err)
			continue
		}
		guardado, err := modelo.DesdeDocumento(doc)
		if err != nil {
			log.Warn("el modelo activo guardado no se puede usar", "de", f.nombre, "err", err)
			continue
		}
		m, fuente = guardado, f.nombre
		break
	}
	doc := m.Documento()
	doc.Activo = true
	if err := a.cacheModelo.GuardarModeloActivo(ctx, doc); err != nil {
		log.Warn("no se guardó el modelo activo en Redis", "err", err)
	}
	if err := a.modelos.Guardar(ctx, doc); err != nil {
		log.Warn("no se guardó el modelo activo en MongoDB", "err", err)
	}
	log.Info("modelo activo", "id", m.ID, "origen", m.Origen, "leido_de", fuente,
		"k", m.K(), "features", m.Features(), "iteraciones", m.Iteraciones)
	return m
}
