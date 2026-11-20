package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/R0SEWT/aire-distribuido/almacen"
	"github.com/R0SEWT/aire-distribuido/modelo"
	"github.com/R0SEWT/aire-distribuido/ventana"
)

// plazoPing es lo que /api/salud espera a cada almacén.
const plazoPing = time.Second

// POST /api/auth/token
func (s *Servidor) crearToken(w http.ResponseWriter, r *http.Request) {
	var c struct {
		Usuario *string `json:"usuario"`
		Clave   *string `json:"clave"`
	}
	if err := leerJSON(w, r, &c); err != nil || c.Usuario == nil || c.Clave == nil || *c.Usuario == "" || *c.Clave == "" {
		escribirError(w, http.StatusBadRequest, "el cuerpo tiene que ser un objeto JSON con usuario y clave")
		return
	}
	u, existe := s.usuarios[*c.Usuario]
	// La clave se compara siempre, exista o no el usuario, para no delatar
	// cuáles existen por el tiempo de respuesta.
	if claveOK := mismaClave(*c.Clave, u.Clave); !claveOK || !existe {
		escribirError(w, http.StatusUnauthorized, "usuario o clave incorrectos")
		return
	}
	tok, expira, err := s.firmador.Emitir(*c.Usuario, u.Rol)
	if err != nil {
		s.log.Error("no se pudo firmar el token", "err", err)
		escribirError(w, http.StatusInternalServerError, "no se pudo emitir el token")
		return
	}
	s.responder(w, r, http.StatusOK, struct {
		Token  string `json:"token"`
		Rol    string `json:"rol"`
		Expira string `json:"expira"`
	}{tok, u.Rol, enLima(expira).Format(time.RFC3339)})
}

// GET /api/salud
func (s *Servidor) salud(w http.ResponseWriter, r *http.Request) {
	var mongo, redis string
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); mongo = estado(r.Context(), s.mongo) }()
	go func() { defer wg.Done(); redis = estado(r.Context(), s.redis) }()
	wg.Wait()
	var activo *string
	if m := s.modelo.Load(); m != nil {
		activo = &m.ID
	}
	s.responder(w, r, http.StatusOK, struct {
		API          string  `json:"api"`
		Mongo        string  `json:"mongo"`
		Redis        string  `json:"redis"`
		NodosEnPool  int     `json:"nodos_en_pool"`
		ModeloActivo *string `json:"modelo_activo"`
	}{"ok", mongo, redis, s.nodosEnPool(), activo})
}

func estado(ctx context.Context, p almacen.Pinger) string {
	ctx, cancel := context.WithTimeout(ctx, plazoPing)
	defer cancel()
	if err := p.Ping(ctx); err != nil {
		return "error"
	}
	return "ok"
}

// nodosEnPool es 0 hasta que exista cluster.Pool (el coordinador del clúster
// dentro de la API, spec §5.2).
func (s *Servidor) nodosEnPool() int { return 0 }

// Mensajes de 400 de POST /api/predicciones, por campo del esquema Lectura.
const (
	msgCuerpoLectura = "el cuerpo tiene que ser un objeto JSON con estacion, fecha_hora, pm2_5 y pm10"
	msgEstacion      = "estacion tiene que ser un texto no vacío"
	msgFechaHora     = "fecha_hora tiene que ser una fecha RFC3339 con zona, como 2026-11-14T15:00:00-05:00"
	msgFechaFutura   = "fecha_hora no puede estar más de 1 hora adelante de la hora del servidor"
)

// margenFuturo es cuánto puede adelantarse la fecha_hora de una lectura al
// reloj del servidor: los relojes de quien mide y de la API no están
// sincronizados al segundo. Más allá se rechaza, porque el régimen de la
// estación no retrocede y una fecha lejana lo dejaría clavado. Las fechas
// pasadas se aceptan todas: los datos históricos son el caso normal.
const margenFuturo = time.Hour

// Topes del esquema Lectura: el código de una estación del catálogo y una
// concentración que un monitor de PM puede medir.
const (
	maxEstacion = 64
	maxPM       = 10000.0
)

var msgEstacionLarga = fmt.Sprintf("estacion no puede superar %d caracteres", maxEstacion)

// recortar deja un texto que mandó el cliente en a lo sumo n caracteres antes
// de repetirlo en un mensaje de error.
func recortar(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "…"
}

func msgPMNegativo(campo string) string { return campo + " tiene que ser un número no negativo" }
func msgPMTope(campo string) string {
	return fmt.Sprintf("%s no puede superar %.0f µg/m³", campo, maxPM)
}

type lectura struct {
	estacion  string
	fechaHora time.Time
	pm25      float64
	pm10      float64
}

// leerLectura valida el cuerpo de una predicción campo por campo, en el orden
// del esquema, y devuelve el mensaje del primer campo inválido. La estación se
// compara sin los espacios de los bordes contra el catálogo en memoria.
func (s *Servidor) leerLectura(w http.ResponseWriter, r *http.Request) (lectura, string) {
	var c struct {
		Estacion  *string  `json:"estacion"`
		FechaHora *string  `json:"fecha_hora"`
		PM25      *float64 `json:"pm2_5"`
		PM10      *float64 `json:"pm10"`
	}
	if err := leerJSON(w, r, &c); err != nil {
		if errors.Is(err, errCuerpoGrande) {
			return lectura{}, msgCuerpoGrande
		}
		var te *json.UnmarshalTypeError
		if errors.As(err, &te) {
			switch te.Field {
			case "estacion":
				return lectura{}, msgEstacion
			case "fecha_hora":
				return lectura{}, msgFechaHora
			case "pm2_5", "pm10":
				// Un número que no cabe en float64 (1e999) es «number 1e999».
				if strings.HasPrefix(te.Value, "number ") && !strings.HasPrefix(te.Value, "number -") {
					return lectura{}, msgPMTope(te.Field)
				}
				return lectura{}, msgPMNegativo(te.Field)
			}
		}
		return lectura{}, msgCuerpoLectura
	}
	if c.Estacion == nil {
		return lectura{}, msgEstacion
	}
	estacion := strings.TrimSpace(*c.Estacion)
	switch {
	case estacion == "":
		return lectura{}, msgEstacion
	case utf8.RuneCountInString(estacion) > maxEstacion:
		return lectura{}, msgEstacionLarga
	case !s.catalogo[estacion]:
		return lectura{}, "estación desconocida: " + estacion
	}
	if c.FechaHora == nil {
		return lectura{}, msgFechaHora
	}
	fecha, err := time.Parse(time.RFC3339, *c.FechaHora)
	if err != nil {
		return lectura{}, msgFechaHora
	}
	if fecha.After(s.ahora().Add(margenFuturo)) {
		return lectura{}, msgFechaFutura
	}
	for _, pm := range []struct {
		campo string
		v     *float64
	}{{"pm2_5", c.PM25}, {"pm10", c.PM10}} {
		switch {
		case pm.v == nil || *pm.v < 0:
			return lectura{}, msgPMNegativo(pm.campo)
		case *pm.v > maxPM:
			return lectura{}, msgPMTope(pm.campo)
		}
	}
	return lectura{estacion: estacion, fechaHora: fecha, pm25: *c.PM25, pm10: *c.PM10}, ""
}

// POST /api/predicciones
//
// El camino del pedido (spec §6.2): leer y validar la lectura, estandarizarla y
// asignarle el centroide más cercano con el modelo activo (en memoria), sumarla
// a la ventana de 24 h de su estación para la alerta y el INCA, y encolar el
// registro. ms es el tiempo hasta ese punto, medido en el servidor; la
// escritura en el almacén la hace otra goroutine.
//
// excede_eca es del régimen (el tipo de aire de la lectura); alerta e inca son
// de la media de 24 h de la estación. No se mezclan.
func (s *Servidor) predecir(w http.ResponseWriter, r *http.Request) {
	inicio := time.Now()
	m := s.modelo.Load()
	if m == nil {
		escribirError(w, http.StatusServiceUnavailable, "no hay modelo activo")
		return
	}
	l, msg := s.leerLectura(w, r)
	if msg != "" {
		escribirError(w, http.StatusBadRequest, msg)
		return
	}
	reg, err := m.Predecir(modelo.Lectura{Estacion: l.estacion, PM25: l.pm25, PM10: l.pm10})
	if err != nil {
		var el *modelo.ErrorLectura
		if errors.As(err, &el) {
			escribirError(w, http.StatusBadRequest, el.Motivo)
			return
		}
		s.log.Error("predicción", "err", err)
		escribirError(w, http.StatusInternalServerError, "no se pudo predecir")
		return
	}
	media := s.ventanas.Agregar(l.estacion, l.fechaHora, l.pm25, l.pm10)
	alerta, inca := ventana.Alerta(media), ventana.INCA(media)
	ms := float64(time.Since(inicio).Nanoseconds()) / 1e6
	s.escritor.encolar(almacen.Prediccion{
		TS:        enLima(s.ahora()).Truncate(time.Millisecond),
		Estacion:  l.estacion,
		FechaHora: enLima(ventana.Hora(l.fechaHora)), // una lectura por estación y hora
		PM25:      l.pm25,
		PM10:      l.pm10,
		Regimen:   reg.Regimen,
		ExcedeECA: reg.ExcedeECA,
		Media24h:  media,
		Alerta:    alerta,
		INCA:      inca,
		ModeloID:  m.ID,
		Ms:        ms,
	})
	s.responder(w, r, http.StatusOK, struct {
		Estacion  string           `json:"estacion"`
		Regimen   int              `json:"regimen"`
		ExcedeECA bool             `json:"excede_eca"`
		Media24h  almacen.Media24h `json:"media_24h"`
		Alerta    bool             `json:"alerta"`
		INCA      *string          `json:"inca"`
		PM25Medio float64          `json:"pm2_5_medio_regimen"`
		PM10Medio float64          `json:"pm10_medio_regimen"`
		ModeloID  string           `json:"modelo_id"`
		Ms        float64          `json:"ms"`
	}{l.estacion, reg.Regimen, reg.ExcedeECA, media, alerta, inca, reg.PM25Medio, reg.PM10Medio, m.ID, ms})
}

// GET /api/estaciones
func (s *Servidor) listarEstaciones(w http.ResponseWriter, r *http.Request) {
	es, err := s.estaciones.Listar(r.Context())
	if err != nil {
		s.log.Error("listar estaciones", "err", err)
		escribirError(w, http.StatusServiceUnavailable, "MongoDB no disponible")
		return
	}
	if es == nil {
		es = []almacen.Estacion{}
	}
	s.responder(w, r, http.StatusOK, es)
}

// GET /api/estaciones/{id}/regimen
//
// Si la estación existe se decide con el catálogo en memoria, como en la
// predicción; el régimen sale de la caché.
func (s *Servidor) regimenEstacion(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !s.catalogo[id] {
		escribirError(w, http.StatusNotFound, "estación desconocida: "+recortar(id, maxEstacion))
		return
	}
	reg, err := s.cache.Regimen(r.Context(), id)
	if err != nil {
		if errors.Is(err, almacen.ErrNoEncontrado) {
			escribirError(w, http.StatusNotFound, fmt.Sprintf("la estación %s todavía no tiene un régimen registrado", id))
			return
		}
		s.log.Error("régimen de la estación", "id", id, "err", err)
		escribirError(w, http.StatusServiceUnavailable, "Redis no disponible")
		return
	}
	reg.FechaHora, reg.TS = enLima(reg.FechaHora), enLima(reg.TS)
	s.responder(w, r, http.StatusOK, reg)
}

// GET /api/modelos/activo
func (s *Servidor) modeloActivo(w http.ResponseWriter, r *http.Request) {
	m := s.modelo.Load()
	if m == nil {
		escribirError(w, http.StatusServiceUnavailable, "no hay modelo activo")
		return
	}
	s.responder(w, r, http.StatusOK, struct {
		ID          string           `json:"id"`
		K           int              `json:"k"`
		Features    []string         `json:"features"`
		Iteraciones int              `json:"iteraciones"`
		Inercia     float64          `json:"inercia"`
		Centroides  [][]float64      `json:"centroides,omitempty"`
		Regimenes   []modelo.Regimen `json:"regimenes"`
	}{m.ID, m.K(), m.Features(), m.Iteraciones, m.Inercia, m.CentroidesOriginales, m.Regimenes})
}

// GET /api/metricas
func (s *Servidor) consultarMetricas(w http.ResponseWriter, r *http.Request) {
	s.responder(w, r, http.StatusOK, struct {
		Endpoints             map[string]resumenRuta     `json:"endpoints"`
		NodosEnPool           int                        `json:"nodos_en_pool"`
		UltimoEntrenamiento   *string                    `json:"ultimo_entrenamiento"`
		EscriturasDescartadas uint64                     `json:"escrituras_descartadas"`
		EscriturasFallidas    uint64                     `json:"escrituras_fallidas"`
		Escritura             map[string]estadoEscritura `json:"escritura"`
	}{s.metricas.Resumen(), s.nodosEnPool(), nil, s.escritor.descartadas(), s.escritor.fallidas(), s.escritor.estado()})
}

// GET /openapi.yaml: el contrato tal cual está en docs/api/openapi.yaml.
func (s *Servidor) servirOpenAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	_, _ = w.Write(s.openapi)
}

// GET /docs
func (s *Servidor) servirDocs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(paginaDocs))
}

// paginaDocs carga Swagger UI desde un CDN con la versión fija (spec §6.3: si
// el laboratorio no tiene salida a internet, se embebe).
const paginaDocs = `<!doctype html>
<html lang="es">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>API de aire-distribuido</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.17.14/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.17.14/swagger-ui-bundle.js"></script>
  <script>
    window.ui = SwaggerUIBundle({url: "/openapi.yaml", dom_id: "#swagger-ui", persistAuthorization: true});
  </script>
</body>
</html>
`
