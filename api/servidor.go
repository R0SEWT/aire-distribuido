// Package api es la API coordinadora de la PC4 (spec §6): REST sobre net/http
// con JWT HS256, latencia medida por ruta y el contrato de docs/api/openapi.yaml.
// Esta es la primera parte: salud, token, predicción, estaciones, modelo
// activo, métricas, el contrato y Swagger UI. El clúster, los entrenamientos y
// el WebSocket llegan después.
//
// El camino de una predicción no espera a ningún almacén: el modelo vive en
// memoria detrás de un atomic.Pointer, la media de 24 h de cada estación
// también (paquete ventana) y la escritura va a una cola con búfer
// (escritor.go).
package api

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/R0SEWT/aire-distribuido/almacen"
	"github.com/R0SEWT/aire-distribuido/kmeans"
	"github.com/R0SEWT/aire-distribuido/modelo"
	"github.com/R0SEWT/aire-distribuido/ventana"
)

// Config es lo que necesita el servidor. Modelo puede ser nil: la API arranca
// igual y responde 503 a las predicciones hasta que se active uno.
type Config struct {
	Modelo       *modelo.Modelo
	Firmador     *Firmador
	Usuarios     map[string]Usuario
	Estaciones   almacen.Estaciones
	Predicciones almacen.Predicciones
	Cache        almacen.Cache
	Mongo, Redis almacen.Pinger // lo que /api/salud informa como mongo y redis
	OpenAPI      []byte         // docs/api/openapi.yaml, que se sirve tal cual
	TamCola      int            // búfer de la cola de escritura
	Log          *slog.Logger
}

// Servidor es la API. Implementa http.Handler.
type Servidor struct {
	modelo       atomic.Pointer[modelo.Modelo]
	firmador     *Firmador
	usuarios     map[string]Usuario
	estaciones   almacen.Estaciones
	cache        almacen.Cache
	mongo, redis almacen.Pinger
	openapi      []byte
	log          *slog.Logger
	metricas     *Metricas
	ventanas     *ventana.Ventanas
	escritor     *escritor
	mux          *http.ServeMux
	ahora        func() time.Time
}

// Nuevo arma el servidor y registra las rutas. Para que las predicciones se
// guarden hay que correr Escribir en otra goroutine.
func Nuevo(cfg Config) (*Servidor, error) {
	switch {
	case cfg.Firmador == nil:
		return nil, errors.New("api: falta el firmador del JWT")
	case len(cfg.Usuarios) == 0:
		return nil, errors.New("api: no hay usuarios")
	case cfg.Estaciones == nil || cfg.Predicciones == nil || cfg.Cache == nil || cfg.Mongo == nil || cfg.Redis == nil:
		return nil, errors.New("api: falta un almacén")
	case len(cfg.OpenAPI) == 0:
		return nil, errors.New("api: falta el contrato OpenAPI")
	case cfg.TamCola <= 0:
		return nil, fmt.Errorf("api: la cola de escritura tiene que tener búfer, tiene %d", cfg.TamCola)
	}
	log := cfg.Log
	if log == nil {
		log = slog.Default()
	}
	s := &Servidor{
		firmador:   cfg.Firmador,
		usuarios:   cfg.Usuarios,
		estaciones: cfg.Estaciones,
		cache:      cfg.Cache,
		mongo:      cfg.Mongo,
		redis:      cfg.Redis,
		openapi:    cfg.OpenAPI,
		log:        log,
		metricas:   NuevasMetricas(),
		ventanas:   ventana.Nuevas(),
		escritor:   nuevoEscritor(cfg.TamCola, cfg.Predicciones, cfg.Cache, log),
		mux:        http.NewServeMux(),
		ahora:      time.Now,
	}
	s.modelo.Store(cfg.Modelo)

	cualquiera := []string{RolUsuario, RolAdmin}
	s.ruta("POST /api/auth/token", nil, s.crearToken)
	s.ruta("GET /api/salud", nil, s.salud)
	s.ruta("POST /api/predicciones", cualquiera, s.predecir)
	s.ruta("GET /api/estaciones", cualquiera, s.listarEstaciones)
	s.ruta("GET /api/estaciones/{id}/regimen", cualquiera, s.regimenEstacion)
	s.ruta("GET /api/modelos/activo", cualquiera, s.modeloActivo)
	s.ruta("GET /api/metricas", []string{RolAdmin}, s.consultarMetricas)
	s.ruta("GET /openapi.yaml", nil, s.servirOpenAPI)
	s.ruta("GET /docs", nil, s.servirDocs)
	return s, nil
}

func (s *Servidor) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }

// Escribir corre la goroutine que guarda las predicciones hasta que ctx se
// cancela; antes de volver escribe lo que quedó en la cola.
func (s *Servidor) Escribir(ctx context.Context) { s.escritor.correr(ctx) }

// ActivarModelo reemplaza el modelo activo. Las predicciones en curso terminan
// con el anterior; ninguna toma un lock.
func (s *Servidor) ActivarModelo(m *modelo.Modelo) { s.modelo.Store(m) }

// ruta registra un handler medido. roles nil es una ruta libre; si no, hace
// falta un JWT válido con alguno de esos roles.
func (s *Servidor) ruta(patron string, roles []string, h http.HandlerFunc) {
	var sig http.Handler = h
	if roles != nil {
		sig = s.exigir(roles, h)
	}
	s.mux.Handle(patron, s.metricas.Medir(patron, sig))
}

const msgNoAutenticado = "token ausente, inválido o expirado"

func (s *Servidor) exigir(roles []string, sig http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok, ok := tokenBearer(r)
		if !ok {
			noAutenticado(w)
			return
		}
		rec, err := s.firmador.Verificar(tok)
		if err != nil {
			noAutenticado(w)
			return
		}
		if !slices.Contains(roles, rec.Rol) {
			escribirError(w, http.StatusForbidden, "se requiere el rol "+strings.Join(roles, " o "))
			return
		}
		sig.ServeHTTP(w, r)
	})
}

// tokenBearer saca el token de «Authorization: Bearer <token>». El nombre del
// esquema no distingue mayúsculas (RFC 7235, §2.1).
func tokenBearer(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	const esquema = "bearer "
	if len(h) <= len(esquema) || !strings.EqualFold(h[:len(esquema)], esquema) {
		return "", false
	}
	return strings.TrimSpace(h[len(esquema):]), true
}

func noAutenticado(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="aire-distribuido"`)
	escribirError(w, http.StatusUnauthorized, msgNoAutenticado)
}

// escribirJSON responde con v en JSON. Sin escapar <, > y &: es una API, no HTML.
func escribirJSON(w http.ResponseWriter, codigo int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(codigo)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func escribirError(w http.ResponseWriter, codigo int, mensaje string) {
	escribirJSON(w, codigo, map[string]string{"error": mensaje})
}

// limiteCuerpo: un pedido de la API es un objeto chico; más que esto es un error.
const limiteCuerpo = 64 << 10

// leerJSON decodifica el cuerpo en v: un solo valor JSON, sin nada después.
func leerJSON(w http.ResponseWriter, r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, limiteCuerpo))
	if err := dec.Decode(v); err != nil {
		return err
	}
	if dec.More() {
		return errors.New("hay datos después del objeto JSON")
	}
	return nil
}

// enLima expresa un instante en la hora de Lima (UTC-5), como los ejemplos del contrato.
func enLima(t time.Time) time.Time { return t.In(kmeans.ZonaPeru) }

// mismaClave compara dos claves en tiempo constante: compara sus sha256, que
// miden lo mismo, así que tampoco se filtra el largo.
func mismaClave(a, b string) bool {
	ha, hb := sha256.Sum256([]byte(a)), sha256.Sum256([]byte(b))
	return subtle.ConstantTimeCompare(ha[:], hb[:]) == 1
}
