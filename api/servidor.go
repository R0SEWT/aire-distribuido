// Package api es la API coordinadora de la PC4 (spec §6): REST sobre net/http
// con JWT HS256, latencia medida por ruta y el contrato de docs/api/openapi.yaml.
// Esta es la primera parte: salud, token, predicción, estaciones, modelo
// activo, métricas, el contrato y Swagger UI. El clúster, los entrenamientos y
// el WebSocket llegan después.
//
// El camino de una predicción no espera a ningún almacén: el modelo vive en
// memoria detrás de un atomic.Pointer, la media de 24 h de cada estación
// también (paquete ventana) y la escritura va a dos colas con búfer, una por
// almacén, que se vacían por lotes (escritor.go).
package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	Modelo   *modelo.Modelo
	Firmador *Firmador
	Usuarios map[string]Usuario
	// Catalogo son las estaciones que la API acepta: la ficha
	// (reports/estaciones_ficha.csv), que es la única fuente del catálogo. Se
	// guarda en memoria para validar una lectura sin consultar ninguna base.
	Catalogo     []almacen.Estacion
	Estaciones   almacen.Estaciones
	Predicciones almacen.Predicciones
	Cache        almacen.Cache
	// Modelos y CacheModelo son donde ActivarModelo guarda el modelo activo
	// (la colección modelos y la clave modelo:activo). Pueden ser nil: el
	// modelo se activa igual, solo en memoria.
	Modelos      almacen.Modelos
	CacheModelo  almacen.CacheModelo
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
	catalogo     map[string]bool
	estaciones   almacen.Estaciones
	cache        almacen.Cache
	modelos      almacenesDeModelo
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
	case len(cfg.Catalogo) == 0:
		return nil, errors.New("api: el catálogo de estaciones está vacío")
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
	catalogo := make(map[string]bool, len(cfg.Catalogo))
	for _, e := range cfg.Catalogo {
		catalogo[e.ID] = true
	}
	s := &Servidor{
		firmador:   cfg.Firmador,
		usuarios:   cfg.Usuarios,
		catalogo:   catalogo,
		estaciones: cfg.Estaciones,
		cache:      cfg.Cache,
		modelos:    almacenesDeModelo{cfg.Modelos, cfg.CacheModelo},
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

// ServeHTTP atiende con el mux. Si ninguna ruta coincide, el mux decide si es
// 404 o 405 (con Allow); la respuesta sale con el esquema Error en JSON, como
// todas las de la API. Las redirecciones del mux (a la ruta limpia) pasan tal cual.
func (s *Servidor) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h, patron := s.mux.Handler(r)
	if patron != "" {
		s.mux.ServeHTTP(w, r) // el mux, no h, para que llene r.PathValue
		return
	}
	c := &capturado{cabecera: http.Header{}, codigo: http.StatusOK}
	h.ServeHTTP(c, r)
	switch c.codigo {
	case http.StatusNotFound:
		escribirError(w, http.StatusNotFound, "no existe la ruta "+r.Method+" "+r.URL.Path)
	case http.StatusMethodNotAllowed:
		w.Header().Set("Allow", c.cabecera.Get("Allow"))
		escribirError(w, http.StatusMethodNotAllowed, "la ruta "+r.URL.Path+" no admite "+r.Method)
	default:
		for k, v := range c.cabecera {
			w.Header()[k] = v
		}
		w.WriteHeader(c.codigo)
		_, _ = w.Write(c.cuerpo.Bytes())
	}
}

// capturado guarda la respuesta del handler de respaldo del mux.
type capturado struct {
	cabecera http.Header
	codigo   int
	cuerpo   bytes.Buffer
}

func (c *capturado) Header() http.Header         { return c.cabecera }
func (c *capturado) WriteHeader(codigo int)      { c.codigo = codigo }
func (c *capturado) Write(b []byte) (int, error) { return c.cuerpo.Write(b) }

// Escribir corre las goroutines que guardan las predicciones hasta que ctx se
// cancela; antes de volver escribe lo que quedó en las colas, sin pasarse de
// plazoVaciado.
func (s *Servidor) Escribir(ctx context.Context) { s.escritor.correr(ctx) }

// ActivarModelo reemplaza el modelo activo. Las predicciones en curso terminan
// con el anterior; ninguna toma un lock. Después lo guarda como activo en
// MongoDB y Redis, si la API los tiene (modelo_activo.go): eso puede tardar
// hasta plazoEscritura por almacén, así que no se llama desde un handler.
func (s *Servidor) ActivarModelo(m *modelo.Modelo) {
	s.modelo.Store(m)
	s.guardarModeloActivo(m)
}

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

const msgNoCodificable = "no se pudo codificar la respuesta"

// responder responde v en JSON. Codifica a un búfer antes de escribir el
// encabezado: si v no se puede codificar (un +Inf, un NaN), responde 500 con el
// esquema Error y lo deja en el log, en vez de un 200 con el cuerpo vacío.
func (s *Servidor) responder(w http.ResponseWriter, r *http.Request, codigo int, v any) {
	cuerpo, err := codificar(v)
	if err != nil {
		s.log.Error(msgNoCodificable, "metodo", r.Method, "ruta", r.URL.Path, "err", err)
		escribirError(w, http.StatusInternalServerError, msgNoCodificable)
		return
	}
	escribirCuerpo(w, codigo, cuerpo)
}

// escribirError responde con el esquema Error. Un objeto de un solo texto
// siempre se puede codificar.
func escribirError(w http.ResponseWriter, codigo int, mensaje string) {
	cuerpo, _ := codificar(map[string]string{"error": mensaje})
	escribirCuerpo(w, codigo, cuerpo)
}

// codificar es JSON sin escapar <, > y &: es una API, no HTML.
func codificar(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func escribirCuerpo(w http.ResponseWriter, codigo int, cuerpo []byte) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(codigo)
	_, _ = w.Write(cuerpo)
}

// limiteCuerpo: un pedido de la API es un objeto chico; más que esto es un error.
const limiteCuerpo = 64 << 10

var (
	errCuerpoGrande = fmt.Errorf("el cuerpo no puede superar %d KiB", limiteCuerpo>>10)
	errDatosDespues = errors.New("hay datos después del objeto JSON")
	msgCuerpoGrande = errCuerpoGrande.Error()
)

// leerJSON decodifica el cuerpo en v: un solo valor JSON y nada después, salvo
// espacios. Un cuerpo de más de limiteCuerpo devuelve errCuerpoGrande.
func leerJSON(w http.ResponseWriter, r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, limiteCuerpo))
	err := dec.Decode(v)
	if err == nil {
		// Un segundo valor, o un «]» o «}» suelto, no es io.EOF.
		var resto json.RawMessage
		if err = dec.Decode(&resto); errors.Is(err, io.EOF) {
			return nil
		}
		if err == nil {
			err = errDatosDespues
		}
	}
	var grande *http.MaxBytesError
	if errors.As(err, &grande) {
		return errCuerpoGrande
	}
	return err
}

// enLima expresa un instante en la hora de Lima (UTC-5), como los ejemplos del contrato.
func enLima(t time.Time) time.Time { return t.In(kmeans.ZonaPeru) }

// mismaClave compara dos claves en tiempo constante: compara sus sha256, que
// miden lo mismo, así que tampoco se filtra el largo.
func mismaClave(a, b string) bool {
	ha, hb := sha256.Sum256([]byte(a)), sha256.Sum256([]byte(b))
	return subtle.ConstantTimeCompare(ha[:], hb[:]) == 1
}
