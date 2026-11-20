package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/R0SEWT/aire-distribuido/almacen"
	"github.com/R0SEWT/aire-distribuido/almacen/memoria"
	"github.com/R0SEWT/aire-distribuido/kmeans"
	"github.com/R0SEWT/aire-distribuido/modelo"
)

const lecturaEjemplo = `{"estacion": "CA-AYAS-01", "fecha_hora": "2026-11-14T15:00:00-05:00", "pm2_5": 70.2, "pm10": 190.5}`

// Los campos obligatorios de los esquemas Prediccion y RegimenEstacion del contrato.
var (
	camposPrediccion = []string{"estacion", "regimen", "excede_eca", "media_24h", "alerta", "inca",
		"pm2_5_medio_regimen", "pm10_medio_regimen", "modelo_id", "ms"}
	camposRegimenEstacion = []string{"regimen", "excede_eca", "media_24h", "alerta", "inca", "ts"}
)

type entorno struct {
	srv   *Servidor
	pred  *memoria.Predicciones
	cache *memoria.Cache
	ent   *memoria.Entrenamientos
	ev    *memoria.Eventos
}

// nuevoEntorno arma la API como cmd/api, con el modelo sembrado desde
// reports/, la ficha de estaciones y el doble en memoria, y deja corriendo la
// goroutine que escribe las predicciones.
func nuevoEntorno(t *testing.T, cambiar ...func(*Config)) *entorno {
	t.Helper()
	m, err := modelo.Sembrar(modelo.Archivos{
		Kmeans: "../reports/kmeans.json", Regimenes: "../reports/regimenes.json", Estandarizacion: "../reports/estandarizacion.json",
	})
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open("../reports/estaciones_ficha.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	ficha, err := almacen.LeerFichaEstaciones(f)
	if err != nil {
		t.Fatal(err)
	}
	est, err := memoria.NuevasEstaciones(ficha)
	if err != nil {
		t.Fatal(err)
	}
	contrato, err := os.ReadFile("../docs/api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	firmador, err := NuevoFirmador(secretoDePrueba, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	usuarios, _ := LeerUsuarios("demo:demo:usuario,admin:clave-admin:admin")
	e := &entorno{pred: memoria.NuevasPredicciones(), cache: memoria.NuevaCache(),
		ent: memoria.NuevosEntrenamientos(), ev: memoria.NuevosEventos()}
	cfg := Config{
		Entrenamientos: e.ent, Eventos: e.ev,
		Modelo: m, Firmador: firmador, Usuarios: usuarios,
		Catalogo: ficha, Estaciones: est, Predicciones: e.pred, Cache: e.cache, Mongo: est, Redis: e.cache,
		OpenAPI: contrato, TamCola: 64, Log: logMudo,
	}
	for _, c := range cambiar {
		c(&cfg)
	}
	e.srv, err = Nuevo(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	listo, coordinado := make(chan struct{}), make(chan struct{})
	go func() { e.srv.Escribir(ctx); close(listo) }()
	go func() { e.srv.Coordinar(ctx); close(coordinado) }()
	t.Cleanup(func() { cancel(); <-listo; <-coordinado })
	return e
}

func (e *entorno) pedir(t testing.TB, metodo, ruta, token, cuerpo string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if cuerpo == "" {
		r = httptest.NewRequest(metodo, ruta, nil)
	} else {
		r = httptest.NewRequest(metodo, ruta, strings.NewReader(cuerpo))
		r.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	e.srv.ServeHTTP(w, r)
	return w
}

func (e *entorno) token(t testing.TB, usuario, clave string) string {
	t.Helper()
	w := e.pedir(t, "POST", "/api/auth/token", "", `{"usuario": "`+usuario+`", "clave": "`+clave+`"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("token de %s: %d %s", usuario, w.Code, w.Body)
	}
	var tok struct{ Token string }
	json.Unmarshal(w.Body.Bytes(), &tok)
	return tok.Token
}

// objeto decodifica la respuesta JSON y comprueba el código y que estén los
// campos obligatorios del esquema del contrato.
func objeto(t *testing.T, w *httptest.ResponseRecorder, codigo int, campos ...string) map[string]any {
	t.Helper()
	if w.Code != codigo {
		t.Fatalf("código %d, se esperaba %d: %s", w.Code, codigo, w.Body)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Content-Type %q", ct)
	}
	var obj map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &obj); err != nil {
		t.Fatalf("la respuesta no es un objeto JSON: %v: %s", err, w.Body)
	}
	for _, c := range campos {
		if _, ok := obj[c]; !ok {
			t.Fatalf("falta el campo %q en %s", c, w.Body)
		}
	}
	return obj
}

func mensajeDeError(t *testing.T, w *httptest.ResponseRecorder, codigo int, mensaje string) {
	t.Helper()
	obj := objeto(t, w, codigo, "error")
	if obj["error"] != mensaje {
		t.Fatalf("error %q, se esperaba %q", obj["error"], mensaje)
	}
}

func TestSalud(t *testing.T) {
	e := nuevoEntorno(t)
	obj := objeto(t, e.pedir(t, "GET", "/api/salud", "", ""), 200, "api", "mongo", "redis", "nodos_en_pool", "modelo_activo")
	want := map[string]any{"api": "ok", "mongo": "ok", "redis": "ok", "nodos_en_pool": 0.0, "modelo_activo": modelo.IDSembrado}
	for k, v := range want {
		if obj[k] != v {
			t.Errorf("%s = %v, se esperaba %v", k, obj[k], v)
		}
	}
}

func TestSaludConUnAlmacenCaido(t *testing.T) {
	e := nuevoEntorno(t, func(c *Config) { c.Mongo = pingRoto{} })
	obj := objeto(t, e.pedir(t, "GET", "/api/salud", "", ""), 200)
	if obj["mongo"] != "error" || obj["redis"] != "ok" {
		t.Fatalf("salud = %v; se esperaba mongo = error y redis = ok", obj)
	}
}

func TestToken(t *testing.T) {
	e := nuevoEntorno(t)
	for usuario, c := range map[string]struct{ clave, rol string }{
		"demo": {"demo", RolUsuario}, "admin": {"clave-admin", RolAdmin},
	} {
		w := e.pedir(t, "POST", "/api/auth/token", "", `{"usuario": "`+usuario+`", "clave": "`+c.clave+`"}`)
		obj := objeto(t, w, 200, "token", "rol", "expira")
		if obj["rol"] != c.rol {
			t.Errorf("rol de %s = %v", usuario, obj["rol"])
		}
		expira, err := time.Parse(time.RFC3339, obj["expira"].(string))
		if err != nil || !strings.HasSuffix(obj["expira"].(string), "-05:00") {
			t.Errorf("expira = %v (%v): tiene que ser RFC3339 en hora de Lima", obj["expira"], err)
		}
		if d := time.Until(expira); d < 59*time.Minute || d > time.Hour {
			t.Errorf("el token vence en %v, se esperaba una hora", d)
		}
	}
	mensajeDeError(t, e.pedir(t, "POST", "/api/auth/token", "", `{"usuario": "demo", "clave": "otra"}`), 401, "usuario o clave incorrectos")
	mensajeDeError(t, e.pedir(t, "POST", "/api/auth/token", "", `{"usuario": "nadie", "clave": "demo"}`), 401, "usuario o clave incorrectos")
	for _, cuerpo := range []string{`no es json`, `{"usuario": "demo"}`, `{"usuario": "", "clave": "demo"}`, `{"usuario": 3, "clave": "demo"}`, `[]`} {
		objeto(t, e.pedir(t, "POST", "/api/auth/token", "", cuerpo), 400, "error")
	}
}

func TestRutasProtegidasSinTokenValido(t *testing.T) {
	e := nuevoEntorno(t)
	vencido, _ := NuevoFirmador(secretoDePrueba, time.Hour)
	vencido.ahora = func() time.Time { return time.Now().Add(-2 * time.Hour) }
	tokVencido, _, _ := vencido.Emitir("demo", RolUsuario)
	rutas := [][2]string{
		{"POST", "/api/predicciones"}, {"GET", "/api/estaciones"}, {"GET", "/api/estaciones/CA-AYAS-01/regimen"},
		{"GET", "/api/modelos/activo"}, {"GET", "/api/metricas"},
	}
	for _, ruta := range rutas {
		for nombre, auth := range map[string]string{
			"sin encabezado": "", "otro esquema": "Basic ZGVtbzpkZW1v", "token roto": "Bearer abc.def.ghi",
			"token vencido": "Bearer " + tokVencido,
		} {
			r := httptest.NewRequest(ruta[0], ruta[1], strings.NewReader(lecturaEjemplo))
			if auth != "" {
				r.Header.Set("Authorization", auth)
			}
			w := httptest.NewRecorder()
			e.srv.ServeHTTP(w, r)
			if w.Code != 401 || w.Header().Get("WWW-Authenticate") == "" {
				t.Fatalf("%s %s, %s: código %d, WWW-Authenticate %q", ruta[0], ruta[1], nombre, w.Code, w.Header().Get("WWW-Authenticate"))
			}
			mensajeDeError(t, w, 401, "token ausente, inválido o expirado")
		}
	}
}

func TestMetricasSoloParaAdmin(t *testing.T) {
	e := nuevoEntorno(t)
	mensajeDeError(t, e.pedir(t, "GET", "/api/metricas", e.token(t, "demo", "demo"), ""), 403, "se requiere el rol admin")
	objeto(t, e.pedir(t, "GET", "/api/metricas", e.token(t, "admin", "clave-admin"), ""), 200)
	// El esquema Bearer no distingue mayúsculas (RFC 7235).
	r := httptest.NewRequest("GET", "/api/metricas", nil)
	r.Header.Set("Authorization", "bearer "+e.token(t, "admin", "clave-admin"))
	w := httptest.NewRecorder()
	e.srv.ServeHTTP(w, r)
	objeto(t, w, 200)
}

func TestPrediccionDelEjemploDelContrato(t *testing.T) {
	e := nuevoEntorno(t)
	for _, usuario := range [][2]string{{"demo", "demo"}, {"admin", "clave-admin"}} {
		w := e.pedir(t, "POST", "/api/predicciones", e.token(t, usuario[0], usuario[1]), lecturaEjemplo)
		obj := objeto(t, w, 200, camposPrediccion...)
		if obj["estacion"] != "CA-AYAS-01" || obj["regimen"] != 1.0 || obj["excede_eca"] != true ||
			obj["pm2_5_medio_regimen"] != 65.12396484374996 || obj["pm10_medio_regimen"] != 186.6618229166667 ||
			obj["modelo_id"] != modelo.IDSembrado {
			t.Fatalf("predicción %s", w.Body)
		}
		// Una sola lectura de la estación (la segunda vuelta repite la fecha_hora
		// y la reemplaza): no hay media de 24 h, así que no hay alerta ni INCA,
		// aunque el régimen exceda el ECA.
		media := obj["media_24h"].(map[string]any)
		if media["lecturas"] != 1.0 || media["pm2_5"] != nil || media["pm10"] != nil || obj["alerta"] != false || obj["inca"] != nil {
			t.Fatalf("media_24h, alerta e inca con una lectura: %s", w.Body)
		}
		if ms, ok := obj["ms"].(float64); !ok || ms < 0 || ms >= 100 {
			t.Fatalf("ms = %v", obj["ms"])
		}
	}
}

func TestPrediccionRechazaLecturasInvalidas(t *testing.T) {
	e := nuevoEntorno(t)
	tok := e.token(t, "demo", "demo")
	const (
		msgJSON   = "el cuerpo tiene que ser un objeto JSON con estacion, fecha_hora, pm2_5 y pm10"
		msgEst    = "estacion tiene que ser un texto no vacío"
		msgFecha  = "fecha_hora tiene que ser una fecha RFC3339 con zona, como 2026-11-14T15:00:00-05:00"
		msgPM25   = "pm2_5 tiene que ser un número no negativo"
		msgPM10   = "pm10 tiene que ser un número no negativo"
		maxPM25   = "pm2_5 no puede superar 10000 µg/m³"
		maxPM10   = "pm10 no puede superar 10000 µg/m³"
		fechaBien = `"fecha_hora": "2026-11-14T15:00:00-05:00"`
	)
	casos := map[string][2]string{
		"no es JSON":            {`estacion=CA-AYAS-01`, msgJSON},
		"es un arreglo":         {`[1, 2]`, msgJSON},
		"JSON cortado":          {`{"estacion": "CA-AYAS-01"`, msgJSON},
		"basura después":        {lecturaEjemplo + ` {}`, msgJSON},
		"corchete después":      {lecturaEjemplo + `]`, msgJSON},
		"llave después":         {lecturaEjemplo + `}`, msgJSON},
		"estación desconocida":  {`{"estacion": "CA-XX-99", ` + fechaBien + `, "pm2_5": 1, "pm10": 2}`, "estación desconocida: CA-XX-99"},
		"estación muy larga":    {`{"estacion": "` + strings.Repeat("A", 65) + `", ` + fechaBien + `, "pm2_5": 1, "pm10": 2}`, "estacion no puede superar 64 caracteres"},
		"pm2_5 enorme":          {`{"estacion": "CA-AYAS-01", ` + fechaBien + `, "pm2_5": 1e308, "pm10": 2}`, maxPM25},
		"pm10 sobre el tope":    {`{"estacion": "CA-AYAS-01", ` + fechaBien + `, "pm2_5": 1, "pm10": 10000.01}`, maxPM10},
		"pm2_5 fuera de float":  {`{"estacion": "CA-AYAS-01", ` + fechaBien + `, "pm2_5": 1e999, "pm10": 2}`, maxPM25},
		"pm10 muy negativo":     {`{"estacion": "CA-AYAS-01", ` + fechaBien + `, "pm2_5": 1, "pm10": -1e999}`, msgPM10},
		"sin estación":          {`{` + fechaBien + `, "pm2_5": 1, "pm10": 2}`, msgEst},
		"estación vacía":        {`{"estacion": "  ", ` + fechaBien + `, "pm2_5": 1, "pm10": 2}`, msgEst},
		"estación numérica":     {`{"estacion": 7, ` + fechaBien + `, "pm2_5": 1, "pm10": 2}`, msgEst},
		"sin fecha":             {`{"estacion": "CA-AYAS-01", "pm2_5": 1, "pm10": 2}`, msgFecha},
		"fecha sin zona":        {`{"estacion": "CA-AYAS-01", "fecha_hora": "2026-11-14T15:00:00", "pm2_5": 1, "pm10": 2}`, msgFecha},
		"fecha con espacio":     {`{"estacion": "CA-AYAS-01", "fecha_hora": "2026-11-14 15:00:00-05:00", "pm2_5": 1, "pm10": 2}`, msgFecha},
		"pm2_5 negativo":        {`{"estacion": "CA-AYAS-01", ` + fechaBien + `, "pm2_5": -0.1, "pm10": 2}`, msgPM25},
		"pm2_5 texto":           {`{"estacion": "CA-AYAS-01", ` + fechaBien + `, "pm2_5": "70.2", "pm10": 2}`, msgPM25},
		"pm2_5 null":            {`{"estacion": "CA-AYAS-01", ` + fechaBien + `, "pm2_5": null, "pm10": 2}`, msgPM25},
		"sin pm2_5":             {`{"estacion": "CA-AYAS-01", ` + fechaBien + `, "pm10": 2}`, msgPM25},
		"pm10 negativo":         {`{"estacion": "CA-AYAS-01", ` + fechaBien + `, "pm2_5": 1, "pm10": -2}`, msgPM10},
		"pm10 fuera de float64": {`{"estacion": "CA-AYAS-01", ` + fechaBien + `, "pm2_5": 1, "pm10": 1e999}`, maxPM10},
	}
	for nombre, c := range casos {
		t.Run(nombre, func(t *testing.T) {
			mensajeDeError(t, e.pedir(t, "POST", "/api/predicciones", tok, c[0]), 400, c[1])
		})
	}
	if n := len(e.pred.Todas()); n != 0 {
		t.Fatalf("se guardaron %d predicciones de pedidos inválidos", n)
	}
}

func TestSinModeloActivo(t *testing.T) {
	e := nuevoEntorno(t, func(c *Config) { c.Modelo = nil })
	tok := e.token(t, "demo", "demo")
	mensajeDeError(t, e.pedir(t, "POST", "/api/predicciones", tok, lecturaEjemplo), 503, "no hay modelo activo")
	mensajeDeError(t, e.pedir(t, "GET", "/api/modelos/activo", tok, ""), 503, "no hay modelo activo")
	obj := objeto(t, e.pedir(t, "GET", "/api/salud", "", ""), 200, "modelo_activo")
	if obj["modelo_activo"] != nil {
		t.Fatalf("modelo_activo = %v, se esperaba null", obj["modelo_activo"])
	}
	// Al activar un modelo, las predicciones vuelven a responder.
	m, _ := modelo.Sembrar(modelo.Archivos{Kmeans: "../reports/kmeans.json", Regimenes: "../reports/regimenes.json", Estandarizacion: "../reports/estandarizacion.json"})
	e.srv.ActivarModelo(m)
	objeto(t, e.pedir(t, "POST", "/api/predicciones", tok, lecturaEjemplo), 200)
}

func TestEstaciones(t *testing.T) {
	e := nuevoEntorno(t)
	w := e.pedir(t, "GET", "/api/estaciones", e.token(t, "demo", "demo"), "")
	if w.Code != 200 {
		t.Fatalf("código %d: %s", w.Code, w.Body)
	}
	var es []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &es); err != nil {
		t.Fatal(err)
	}
	if len(es) != 33 {
		t.Fatalf("%d estaciones, se esperaban 33", len(es))
	}
	campos := []string{"id", "objeto_fiscalizado", "tipo_actividad", "departamento", "provincia", "distrito", "lat", "lon"}
	ids := []string{}
	for _, est := range es {
		for _, c := range campos {
			if _, ok := est[c]; !ok {
				t.Fatalf("falta %s en %v", c, est)
			}
		}
		ids = append(ids, est["id"].(string))
		switch est["id"] {
		case "CA-AYAS-01":
			if est["objeto_fiscalizado"] != "Unidad minera Antamina" || est["distrito"] != "SAN MARCOS" || est["lat"] != -9.52384 || est["lon"] != -77.02043 {
				t.Errorf("CA-AYAS-01 = %v", est)
			}
		case "CA-HUAR-01": // la ficha trae NA en la ubicación
			if est["departamento"] != nil {
				t.Errorf("CA-HUAR-01: departamento = %v, se esperaba null", est["departamento"])
			}
		}
	}
	if !slices.IsSorted(ids) {
		t.Errorf("las estaciones no vienen ordenadas por id: %v", ids)
	}
}

func TestRegimenDeUnaEstacion(t *testing.T) {
	e := nuevoEntorno(t)
	tok := e.token(t, "demo", "demo")
	mensajeDeError(t, e.pedir(t, "GET", "/api/estaciones/CA-XX-99/regimen", tok, ""), 404, "estación desconocida: CA-XX-99")
	// Un código enorme no se repite entero en el mensaje.
	mensajeDeError(t, e.pedir(t, "GET", "/api/estaciones/"+strings.Repeat("A", 5000)+"/regimen", tok, ""), 404,
		"estación desconocida: "+strings.Repeat("A", 64)+"…")
	mensajeDeError(t, e.pedir(t, "GET", "/api/estaciones/CA-AYAS-01/regimen", tok, ""), 404,
		"la estación CA-AYAS-01 todavía no tiene un régimen registrado")

	antes := time.Now()
	objeto(t, e.pedir(t, "POST", "/api/predicciones", tok, lecturaEjemplo), 200)
	// La escritura es asíncrona: el régimen aparece en cuanto la vacía el escritor.
	var w *httptest.ResponseRecorder
	for limite := time.Now().Add(2 * time.Second); time.Now().Before(limite); time.Sleep(5 * time.Millisecond) {
		if w = e.pedir(t, "GET", "/api/estaciones/CA-AYAS-01/regimen", tok, ""); w.Code == 200 {
			break
		}
	}
	obj := objeto(t, w, 200, camposRegimenEstacion...)
	if obj["regimen"] != 1.0 || obj["excede_eca"] != true || obj["alerta"] != false || obj["inca"] != nil ||
		obj["media_24h"].(map[string]any)["lecturas"] != 1.0 {
		t.Fatalf("régimen = %s", w.Body)
	}
	ts, err := time.Parse(time.RFC3339, obj["ts"].(string))
	if err != nil || ts.Before(antes.Add(-time.Second)) || !strings.HasSuffix(obj["ts"].(string), "-05:00") {
		t.Fatalf("ts = %v (%v): tiene que ser el momento de la predicción, en hora de Lima", obj["ts"], err)
	}
	// Y la predicción quedó guardada con su lectura.
	todas := e.pred.Todas()
	if len(todas) != 1 || todas[0].Estacion != "CA-AYAS-01" || todas[0].PM25 != 70.2 || todas[0].PM10 != 190.5 ||
		todas[0].ModeloID != modelo.IDSembrado || todas[0].FechaHora.Format(time.RFC3339) != "2026-11-14T15:00:00-05:00" {
		t.Fatalf("predicciones guardadas: %+v", todas)
	}
}

func lecturaJSON(estacion string, hora int, pm25, pm10 float64) string {
	fecha := time.Date(2026, 11, 13, 0, 0, 0, 0, kmeans.ZonaPeru).Add(time.Duration(hora) * time.Hour)
	b, _ := json.Marshal(map[string]any{"estacion": estacion, "fecha_hora": fecha.Format(time.RFC3339), "pm2_5": pm25, "pm10": pm10})
	return string(b)
}

// La alerta y el INCA son de la media de 24 h de la estación (18 lecturas o
// más), no del régimen: una hora del régimen alto en un día limpio no alerta,
// y un día sucio alerta aunque la última hora sea limpia.
func TestAlertaEINCASalenDeLaMediaDe24h(t *testing.T) {
	e := nuevoEntorno(t)
	tok := e.token(t, "demo", "demo")
	predecir := func(cuerpo string) map[string]any {
		t.Helper()
		return objeto(t, e.pedir(t, "POST", "/api/predicciones", tok, cuerpo), 200, camposPrediccion...)
	}

	// Día sucio: 17 horas en el régimen alto todavía no alcanzan para la media.
	var obj map[string]any
	for h := 1; h <= 17; h++ {
		obj = predecir(lecturaJSON("CA-CP-01", h, 70.2, 190.5))
		if obj["alerta"] != false || obj["inca"] != nil || obj["media_24h"].(map[string]any)["pm2_5"] != nil {
			t.Fatalf("hora %d, sin 18 lecturas: %v", h, obj)
		}
	}
	// La hora 18 es limpia (régimen 0), pero la media de 24 h supera el ECA.
	obj = predecir(lecturaJSON("CA-CP-01", 18, 5, 10))
	media := obj["media_24h"].(map[string]any)
	pm25, pm10 := (17*70.2+5)/18, (17*190.5+10)/18
	if obj["regimen"] != 0.0 || obj["excede_eca"] != false || media["lecturas"] != 18.0 ||
		!cerca(media["pm2_5"], pm25) || !cerca(media["pm10"], pm10) || obj["alerta"] != true || obj["inca"] != "mala" {
		t.Fatalf("hora 18 limpia en un día sucio (media %.2f y %.2f): %v", pm25, pm10, obj)
	}

	// Día limpio: 17 horas limpias y una del régimen alto.
	for h := 1; h <= 17; h++ {
		predecir(lecturaJSON("CA-AYAS-01", h, 5, 10))
	}
	obj = predecir(lecturaJSON("CA-AYAS-01", 18, 70.2, 190.5))
	if obj["regimen"] != 1.0 || obj["excede_eca"] != true || obj["alerta"] != false || obj["inca"] != "buena" {
		t.Fatalf("hora 18 del régimen alto en un día limpio: %v", obj)
	}

	// El régimen de la estación guarda lo mismo que respondió la última predicción.
	var w *httptest.ResponseRecorder
	for limite := time.Now().Add(2 * time.Second); time.Now().Before(limite); time.Sleep(5 * time.Millisecond) {
		w = e.pedir(t, "GET", "/api/estaciones/CA-CP-01/regimen", tok, "")
		if w.Code == 200 && objeto(t, w, 200)["media_24h"].(map[string]any)["lecturas"] == 18.0 {
			break
		}
	}
	reg := objeto(t, w, 200, camposRegimenEstacion...)
	if reg["regimen"] != 0.0 || reg["alerta"] != true || reg["inca"] != "mala" || !cerca(reg["media_24h"].(map[string]any)["pm10"], pm10) {
		t.Fatalf("régimen de CA-CP-01: %s", w.Body)
	}
}

// El código de la estación se compara sin los espacios de los bordes y se
// responde limpio; el tope de PM incluye 10 000.
func TestEstacionConEspaciosYTopeDePM(t *testing.T) {
	e := nuevoEntorno(t)
	tok := e.token(t, "demo", "demo")
	obj := objeto(t, e.pedir(t, "POST", "/api/predicciones", tok,
		`{"estacion": "  CA-HUA-01 ", "fecha_hora": "2026-11-14T15:00:00-05:00", "pm2_5": 10000, "pm10": 10000}`), 200, camposPrediccion...)
	if obj["estacion"] != "CA-HUA-01" {
		t.Fatalf("estacion = %q, se esperaba el código sin espacios", obj["estacion"])
	}
}

// Quitar el límite del cuerpo hace fallar esta prueba: el cuerpo es una
// lectura válida con un campo de relleno de 100 KiB.
func TestCuerpoMasGrandeQueElLimite(t *testing.T) {
	e := nuevoEntorno(t)
	cuerpo := `{"relleno": "` + strings.Repeat("x", 100<<10) + `", ` + lecturaEjemplo[1:]
	mensajeDeError(t, e.pedir(t, "POST", "/api/predicciones", e.token(t, "demo", "demo"), cuerpo), 400,
		"el cuerpo no puede superar 64 KiB")
}

func TestRutaInexistenteYMetodoNoPermitidoEnJSON(t *testing.T) {
	e := nuevoEntorno(t)
	mensajeDeError(t, e.pedir(t, "GET", "/api/no-existe", "", ""), 404, "no existe la ruta GET /api/no-existe")
	w := e.pedir(t, "GET", "/api/predicciones", "", "")
	mensajeDeError(t, w, 405, "la ruta /api/predicciones no admite GET")
	if allow := w.Header().Get("Allow"); allow != "POST" {
		t.Fatalf("Allow = %q, se esperaba POST", allow)
	}
	// Una ruta registrada con un parámetro sigue funcionando por el envoltorio.
	if w := e.pedir(t, "GET", "/api/estaciones/CA-XX-99/regimen", e.token(t, "demo", "demo"), ""); w.Code != 404 {
		t.Fatalf("GET /api/estaciones/CA-XX-99/regimen = %d", w.Code)
	}
	// La redirección del mux a la ruta limpia no se toca.
	if w := e.pedir(t, "GET", "/api//salud", "", ""); w.Code/100 != 3 || w.Header().Get("Location") != "/api/salud" {
		t.Fatalf("GET /api//salud = %d, Location %q", w.Code, w.Header().Get("Location"))
	}
}

// Si una respuesta no se puede codificar (un +Inf), se responde 500 con el
// esquema Error y queda en el log, en vez de un 200 con el cuerpo vacío.
func TestRespuestaQueNoSeCodificaEs500(t *testing.T) {
	var log strings.Builder
	e := nuevoEntorno(t, func(c *Config) { c.Log = slog.New(slog.NewTextHandler(&escritorSeguro{b: &log}, nil)) })
	w := httptest.NewRecorder()
	e.srv.responder(w, httptest.NewRequest("GET", "/x", nil), http.StatusOK, map[string]float64{"x": math.Inf(1)})
	mensajeDeError(t, w, 500, "no se pudo codificar la respuesta")
	if !strings.Contains(log.String(), "no se pudo codificar la respuesta") {
		t.Fatalf("el error no quedó en el log: %q", log.String())
	}
}

// El régimen de la estación es el de su lectura más reciente por fecha_hora:
// una lectura de cinco días antes no lo reemplaza. Lleva la hora de su lectura.
func TestRegimenNoRetrocedeConUnaLecturaMasAntigua(t *testing.T) {
	e := nuevoEntorno(t)
	tok := e.token(t, "demo", "demo")
	for h := 1; h <= 24; h++ {
		objeto(t, e.pedir(t, "POST", "/api/predicciones", tok, lecturaJSON("CA-MAC-01", h, 80, 150)), 200)
	}
	vieja := objeto(t, e.pedir(t, "POST", "/api/predicciones", tok, lecturaJSON("CA-MAC-01", 24-5*24, 5, 5)), 200, camposPrediccion...)
	if vieja["alerta"] != false || vieja["media_24h"].(map[string]any)["lecturas"] != 1.0 {
		t.Fatalf("la lectura de cinco días antes se mide con su propia ventana: %v", vieja)
	}
	// Se espera a que el escritor procese las 25 predicciones.
	for limite := time.Now().Add(2 * time.Second); len(e.pred.Todas()) < 25 && time.Now().Before(limite); {
		time.Sleep(5 * time.Millisecond)
	}
	w := e.pedir(t, "GET", "/api/estaciones/CA-MAC-01/regimen", tok, "")
	reg := objeto(t, w, 200, camposRegimenEstacion...)
	if reg["fecha_hora"] != "2026-11-14T00:00:00-05:00" || reg["alerta"] != true ||
		reg["media_24h"].(map[string]any)["lecturas"] != 24.0 {
		t.Fatalf("régimen de CA-MAC-01 tras una lectura vieja: %s", w.Body)
	}
}

// Una fecha futura no borra la ventana de la estación, y lecturas a menos de
// una hora una de otra son la misma lectura (el contrato dice a lo sumo 24).
func TestFechaFuturaYSubHoraria(t *testing.T) {
	e := nuevoEntorno(t)
	tok := e.token(t, "demo", "demo")
	predecir := func(cuerpo string) map[string]any {
		t.Helper()
		return objeto(t, e.pedir(t, "POST", "/api/predicciones", tok, cuerpo), 200, camposPrediccion...)
	}
	for h := 1; h <= 24; h++ {
		predecir(lecturaJSON("CA-ROM-01", h, 80, 150))
	}
	predecir(`{"estacion": "CA-ROM-01", "fecha_hora": "2099-01-01T00:00:00-05:00", "pm2_5": 5, "pm10": 5}`)
	if obj := predecir(lecturaJSON("CA-ROM-01", 25, 80, 150)); obj["alerta"] != true || obj["media_24h"].(map[string]any)["lecturas"] != 24.0 {
		t.Fatalf("la hora siguiente a una lectura de 2099: %v", obj)
	}
	for seg := range 30 {
		fecha := time.Date(2026, 11, 20, 10, 0, seg, 0, kmeans.ZonaPeru).Format(time.RFC3339)
		obj := predecir(`{"estacion": "CA-JUPR-01", "fecha_hora": "` + fecha + `", "pm2_5": 1, "pm10": 1}`)
		if obj["media_24h"].(map[string]any)["lecturas"] != 1.0 {
			t.Fatalf("lectura %d en el mismo minuto: %v; se esperaba una sola lectura en la ventana", seg, obj)
		}
	}
}

func cerca(v any, want float64) bool {
	f, ok := v.(float64)
	return ok && f > want-1e-9 && f < want+1e-9
}

func TestModeloActivo(t *testing.T) {
	e := nuevoEntorno(t)
	w := e.pedir(t, "GET", "/api/modelos/activo", e.token(t, "demo", "demo"), "")
	obj := objeto(t, w, 200, "id", "k", "features", "iteraciones", "inercia", "regimenes")
	if obj["id"] != modelo.IDSembrado || obj["k"] != 3.0 || obj["iteraciones"] != 44.0 || obj["inercia"] != 50950.36689106275 {
		t.Fatalf("modelo = %s", w.Body)
	}
	var cuerpo struct {
		Features   []string
		Centroides [][]float64
		Regimenes  []map[string]any
	}
	json.Unmarshal(w.Body.Bytes(), &cuerpo)
	if !slices.Equal(cuerpo.Features, []string{"pm2_5", "pm10"}) || len(cuerpo.Centroides) != 3 || len(cuerpo.Regimenes) != 3 {
		t.Fatalf("modelo = %s", w.Body)
	}
	r1 := cuerpo.Regimenes[1]
	if r1["regimen"] != 1.0 || r1["excede_eca"] != true || r1["tamano"] != 1536.0 || r1["pm2_5_medio"] != 65.12396484374996 {
		t.Fatalf("régimen 1 = %v", r1)
	}
}

func TestOpenAPISeSirveSinCambios(t *testing.T) {
	e := nuevoEntorno(t)
	w := e.pedir(t, "GET", "/openapi.yaml", "", "")
	contrato, _ := os.ReadFile("../docs/api/openapi.yaml")
	if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), contrato) {
		t.Fatalf("código %d; el cuerpo no es docs/api/openapi.yaml", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/yaml" {
		t.Fatalf("Content-Type %q", ct)
	}
}

func TestDocsCargaSwaggerUI(t *testing.T) {
	e := nuevoEntorno(t)
	w := e.pedir(t, "GET", "/docs", "", "")
	cuerpo := w.Body.String()
	if w.Code != 200 || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/html") ||
		!strings.Contains(cuerpo, "swagger-ui-bundle.js") || !strings.Contains(cuerpo, `"/openapi.yaml"`) {
		t.Fatalf("código %d, Content-Type %q:\n%s", w.Code, w.Header().Get("Content-Type"), cuerpo)
	}
}

func TestMetricasPorEndpoint(t *testing.T) {
	e := nuevoEntorno(t)
	tok := e.token(t, "demo", "demo")
	for range 20 {
		objeto(t, e.pedir(t, "POST", "/api/predicciones", tok, lecturaEjemplo), 200)
	}
	e.pedir(t, "GET", "/api/salud", "", "")
	w := e.pedir(t, "GET", "/api/metricas", e.token(t, "admin", "clave-admin"), "")
	obj := objeto(t, w, 200, "endpoints", "nodos_en_pool", "ultimo_entrenamiento", "escrituras_descartadas")
	if obj["nodos_en_pool"] != 0.0 || obj["ultimo_entrenamiento"] != nil || obj["escrituras_descartadas"] != 0.0 {
		t.Fatalf("métricas = %s", w.Body)
	}
	var m struct {
		Endpoints map[string]resumenRuta
	}
	json.Unmarshal(w.Body.Bytes(), &m)
	p := m.Endpoints["POST /api/predicciones"]
	if p.Pedidos != 20 || p.P50 <= 0 || p.P50 > p.P95 || p.P95 > p.P99 {
		t.Fatalf("POST /api/predicciones = %+v", p)
	}
	if m.Endpoints["GET /api/salud"].Pedidos != 1 || m.Endpoints["POST /api/auth/token"].Pedidos != 2 {
		t.Fatalf("endpoints = %+v", m.Endpoints)
	}
}

// El requisito de la spec (§6.2): la predicción responde en menos de 100 ms,
// también con clientes concurrentes. Se mide el pedido completo dentro del
// servidor (JWT, JSON, predicción y encolado), con el detector de carreras.
func TestPrediccionEnMenosDe100ms(t *testing.T) {
	e := nuevoEntorno(t, func(c *Config) { c.TamCola = 4096 })
	tok := e.token(t, "demo", "demo")
	const clientes, porCliente = 16, 100
	var (
		mu       sync.Mutex
		tiempos  []time.Duration
		wg       sync.WaitGroup
		fallaron int
	)
	for range clientes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range porCliente {
				inicio := time.Now()
				w := e.pedir(t, "POST", "/api/predicciones", tok, lecturaEjemplo)
				d := time.Since(inicio)
				mu.Lock()
				tiempos = append(tiempos, d)
				if w.Code != 200 {
					fallaron++
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if fallaron > 0 {
		t.Fatalf("%d pedidos no respondieron 200", fallaron)
	}
	slices.Sort(tiempos)
	p99 := tiempos[len(tiempos)*99/100]
	t.Logf("%d pedidos con %d clientes: p50 %v, p99 %v, máximo %v", len(tiempos), clientes,
		tiempos[len(tiempos)/2], p99, tiempos[len(tiempos)-1])
	if p99 >= 100*time.Millisecond {
		t.Fatalf("p99 = %v, el requisito es < 100 ms", p99)
	}
}

// Con la cola de escritura llena, la predicción responde igual y el descarte
// se ve en /api/metricas.
func TestColaLlenaNoFrenaLaPrediccion(t *testing.T) {
	bloqueo := &prediccionesBloqueadas{entro: make(chan struct{}, 1), soltar: make(chan struct{})}
	defer close(bloqueo.soltar)
	e := nuevoEntorno(t, func(c *Config) { c.Predicciones = bloqueo; c.TamCola = 1 })
	tok := e.token(t, "demo", "demo")
	objeto(t, e.pedir(t, "POST", "/api/predicciones", tok, lecturaEjemplo), 200)
	<-bloqueo.entro // el escritor quedó trabado en la primera
	for range 5 {
		objeto(t, e.pedir(t, "POST", "/api/predicciones", tok, lecturaEjemplo), 200)
	}
	obj := objeto(t, e.pedir(t, "GET", "/api/metricas", e.token(t, "admin", "clave-admin"), ""), 200)
	if obj["escrituras_descartadas"] != 4.0 {
		t.Fatalf("escrituras_descartadas = %v, se esperaban 4 (una en curso y una en la cola de 1)", obj["escrituras_descartadas"])
	}
}

func TestNuevoExigeLoNecesario(t *testing.T) {
	e := nuevoEntorno(t)
	base := func() Config {
		return Config{
			Firmador: &Firmador{}, Usuarios: map[string]Usuario{"a": {"b", RolUsuario}},
			Catalogo:   []almacen.Estacion{{ID: "CA-A"}},
			Estaciones: e.srv.estaciones, Predicciones: e.pred, Cache: e.cache, Mongo: e.cache, Redis: e.cache,
			OpenAPI: []byte("x"), TamCola: 1, Entrenamientos: e.ent, Eventos: e.ev,
		}
	}
	if _, err := Nuevo(base()); err != nil {
		t.Fatalf("la configuración base tiene que valer: %v", err)
	}
	for nombre, cambiar := range map[string]func(*Config){
		"sin firmador":     func(c *Config) { c.Firmador = nil },
		"sin usuarios":     func(c *Config) { c.Usuarios = nil },
		"sin catálogo":     func(c *Config) { c.Catalogo = nil },
		"sin estaciones":   func(c *Config) { c.Estaciones = nil },
		"sin predicciones": func(c *Config) { c.Predicciones = nil },
		"sin caché":        func(c *Config) { c.Cache = nil },
		"sin contrato":     func(c *Config) { c.OpenAPI = nil },
		"cola de 0":        func(c *Config) { c.TamCola = 0 },
	} {
		cfg := base()
		cambiar(&cfg)
		if _, err := Nuevo(cfg); err == nil {
			t.Errorf("%s: Nuevo no falló", nombre)
		}
	}
}

type pingRoto struct{}

func (pingRoto) Ping(context.Context) error { return errors.New("sin conexión") }

// escritorSeguro deja que el log del servidor y la prueba compartan el texto.
type escritorSeguro struct {
	mu sync.Mutex
	b  *strings.Builder
}

func (e *escritorSeguro) Write(p []byte) (int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.b.Write(p)
}
