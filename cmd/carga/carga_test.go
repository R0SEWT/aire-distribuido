package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPercentilEsElRangoMasCercano(t *testing.T) {
	cien := make([]float64, 100)
	for i := range cien {
		cien[i] = float64(i + 1)
	}
	casos := []struct {
		nombre string
		xs     []float64
		q      float64
		quiero float64
	}{
		{"p50 de 1..100", cien, 0.50, 50},
		{"p95 de 1..100", cien, 0.95, 95},
		{"p99 de 1..100", cien, 0.99, 99},
		{"p100 es el máximo", cien, 1, 100},
		{"un solo valor", []float64{7}, 0.99, 7},
		{"p50 de cuatro", []float64{1, 2, 3, 4}, 0.50, 2},
		{"p99 de cuatro es el último", []float64{1, 2, 3, 4}, 0.99, 4},
		{"sin muestras", nil, 0.50, 0},
	}
	for _, c := range casos {
		if got := percentil(c.xs, c.q); got != c.quiero {
			t.Errorf("%s: percentil = %v, quiero %v", c.nombre, got, c.quiero)
		}
	}
}

func TestEnteros(t *testing.T) {
	got, err := enteros("1, 4,16,64")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 || got[0] != 1 || got[1] != 4 || got[2] != 16 || got[3] != 64 {
		t.Fatalf("enteros = %v", got)
	}
	for _, mala := range []string{"", "0", "4,-1", "dos", "1,,2"} {
		if _, err := enteros(mala); err == nil {
			t.Errorf("enteros(%q) no dio error", mala)
		}
	}
}

const csvChico = `caso,estacion,fecha_hora,pm2_5,pm10,regimen_esperado,lecturas_esperadas
regimen_0,CA-AYAS-01,2022-11-10T13:00:00-05:00,16.43,37.75,0,1
regimen_1,CA-CG-02,2022-06-04T06:00:00-05:00,73.34,107.42,1,1
regimen_2,CA-CH-03,2021-11-19T08:00:00-05:00,32.2,46.6,2,1
`

func TestLeerLecturas(t *testing.T) {
	ls, err := leerLecturas(strings.NewReader(csvChico))
	if err != nil {
		t.Fatal(err)
	}
	if len(ls) != 3 {
		t.Fatalf("leí %d lecturas, quiero 3", len(ls))
	}
	if ls[1].regimen != 1 {
		t.Errorf("régimen esperado de la segunda = %d, quiero 1", ls[1].regimen)
	}
	// El cuerpo es el pedido tal cual lo recibe la API.
	var cuerpo struct {
		Estacion  string  `json:"estacion"`
		FechaHora string  `json:"fecha_hora"`
		PM25      float64 `json:"pm2_5"`
		PM10      float64 `json:"pm10"`
	}
	if err := json.Unmarshal(ls[1].cuerpo, &cuerpo); err != nil {
		t.Fatal(err)
	}
	if cuerpo.Estacion != "CA-CG-02" || cuerpo.FechaHora != "2022-06-04T06:00:00-05:00" || cuerpo.PM25 != 73.34 || cuerpo.PM10 != 107.42 {
		t.Errorf("cuerpo = %+v", cuerpo)
	}
}

func TestLeerLecturasRechazaUnArchivoMalo(t *testing.T) {
	malos := map[string]string{
		"sin filas":     "estacion,fecha_hora,pm2_5,pm10,regimen_esperado\n",
		"falta columna": "estacion,fecha_hora,pm2_5,pm10\nCA-X,2022-01-01T00:00:00-05:00,1,2\n",
		"pm no número":  "estacion,fecha_hora,pm2_5,pm10,regimen_esperado\nCA-X,2022-01-01T00:00:00-05:00,uno,2,0\n",
		"régimen malo":  "estacion,fecha_hora,pm2_5,pm10,regimen_esperado\nCA-X,2022-01-01T00:00:00-05:00,1,2,alto\n",
		"fecha mala":    "estacion,fecha_hora,pm2_5,pm10,regimen_esperado\nCA-X,ayer,1,2,0\n",
		"estación vacía": "estacion,fecha_hora,pm2_5,pm10,regimen_esperado\n" +
			",2022-01-01T00:00:00-05:00,1,2,0\n",
	}
	for nombre, csv := range malos {
		if _, err := leerLecturas(strings.NewReader(csv)); err == nil {
			t.Errorf("%s: no dio error", nombre)
		}
	}
}

// Los datos de prueba versionados son la entrada por omisión del comando.
func TestLeeLasLecturasDelRepositorio(t *testing.T) {
	f, err := os.Open(filepath.Join("..", "..", "docs", "postman", "lecturas_prueba.csv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	ls, err := leerLecturas(f)
	if err != nil {
		t.Fatal(err)
	}
	porRegimen := map[int]int{}
	for _, l := range ls {
		porRegimen[l.regimen]++
	}
	for g := range 3 {
		if porRegimen[g] < 10 {
			t.Errorf("régimen %d: %d lecturas, quiero al menos 10", g, porRegimen[g])
		}
	}
}

// apiFalsa imita lo que cmd/carga usa de la API: el token, la salud y la
// predicción. predecir decide la respuesta de cada predicción; nil responde
// 200 con el régimen del pedido número n según regimenes.
type apiFalsa struct {
	*httptest.Server
	predicciones atomic.Int64
	sinToken     atomic.Int64
	cuerposMalos atomic.Int64
	validas      atomic.Int64 // predicciones con token y cuerpo bien formado
	metricas     atomic.Int64 // consultas a /api/metricas
	// escritura decide lo que /api/metricas dice de cada almacén; nil = todas
	// las predicciones válidas ya están escritas en los dos.
	escritura func(validas, consulta int64) (mongo, redis almacenEscritura)
}

const (
	tokenDePrueba = "token-de-prueba"
	tokenDeAdmin  = "token-de-admin"
)

func nuevaAPIFalsa(t *testing.T, predecir func(w http.ResponseWriter, estacion string)) *apiFalsa {
	t.Helper()
	a := &apiFalsa{}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/token", func(w http.ResponseWriter, r *http.Request) {
		var c struct{ Usuario, Clave string }
		_ = json.NewDecoder(r.Body).Decode(&c)
		switch {
		case c.Usuario == "demo" && c.Clave == "secreta":
			_, _ = io.WriteString(w, `{"token":"`+tokenDePrueba+`","rol":"usuario","expira":"2026-11-21T23:30:00-05:00"}`)
		case c.Usuario == "admin" && c.Clave == "secreta-admin":
			_, _ = io.WriteString(w, `{"token":"`+tokenDeAdmin+`","rol":"admin","expira":"2026-11-21T23:30:00-05:00"}`)
		default:
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"error":"usuario o clave incorrectos"}`)
		}
	})
	mux.HandleFunc("GET /api/metricas", func(w http.ResponseWriter, r *http.Request) {
		consulta := a.metricas.Add(1)
		if r.Header.Get("Authorization") != "Bearer "+tokenDeAdmin {
			w.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(w, `{"error":"se requiere el rol admin"}`)
			return
		}
		validas := a.validas.Load()
		todas := almacenEscritura{Escritas: uint64(validas), Lotes: uint64(validas)}
		mongo, redis := todas, todas
		if a.escritura != nil {
			mongo, redis = a.escritura(validas, consulta)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"endpoints": map[string]any{}, "nodos_en_pool": 3,
			"escritura": map[string]almacenEscritura{"mongo": mongo, "redis": redis},
		})
	})
	mux.HandleFunc("GET /api/salud", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"api":"ok","mongo":"ok","redis":"ok","nodos_en_pool":3,"modelo_activo":"sembrado-kmeans-json"}`)
	})
	mux.HandleFunc("POST /api/predicciones", func(w http.ResponseWriter, r *http.Request) {
		a.predicciones.Add(1)
		if r.Header.Get("Authorization") != "Bearer "+tokenDePrueba {
			a.sinToken.Add(1)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var l struct {
			Estacion  string   `json:"estacion"`
			FechaHora string   `json:"fecha_hora"`
			PM25      *float64 `json:"pm2_5"`
			PM10      *float64 `json:"pm10"`
		}
		if err := json.NewDecoder(r.Body).Decode(&l); err != nil || l.Estacion == "" || l.FechaHora == "" || l.PM25 == nil || l.PM10 == nil {
			a.cuerposMalos.Add(1)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		a.validas.Add(1)
		predecir(w, l.Estacion)
	})
	a.Server = httptest.NewServer(mux)
	t.Cleanup(a.Close)
	return a
}

// regimenDe es el régimen de cada estación de csvChico.
var regimenDe = map[string]int{"CA-AYAS-01": 0, "CA-CG-02": 1, "CA-CH-03": 2}

func responderBien(w http.ResponseWriter, estacion string) {
	_ = json.NewEncoder(w).Encode(map[string]any{"estacion": estacion, "regimen": regimenDe[estacion]})
}

func lecturasChicas(t *testing.T) []lectura {
	t.Helper()
	ls, err := leerLecturas(strings.NewReader(csvChico))
	if err != nil {
		t.Fatal(err)
	}
	return ls
}

func TestMedirNivelHaceTodosLosPedidos(t *testing.T) {
	api := nuevaAPIFalsa(t, responderBien)
	c := nuevoCliente(api.URL, tokenDePrueba, 5*time.Second)

	n, err := medirNivel(context.Background(), c, lecturasChicas(t), 8, 200, 16, 100)
	if err != nil {
		t.Fatal(err)
	}
	if n.Clientes != 8 || n.Pedidos != 200 {
		t.Errorf("nivel = %d clientes y %d pedidos, quiero 8 y 200", n.Clientes, n.Pedidos)
	}
	if n.Errores != 0 || n.RespuestasDistintas != 0 {
		t.Errorf("errores = %d (%v), distintas = %d; quiero 0", n.Errores, n.ErroresPorTipo, n.RespuestasDistintas)
	}
	// El calentamiento llega a la API, pero no se mide.
	if got := api.predicciones.Load(); got != 216 {
		t.Errorf("la API recibió %d predicciones, quiero 200 medidas + 16 de calentamiento", got)
	}
	if api.sinToken.Load() != 0 || api.cuerposMalos.Load() != 0 {
		t.Errorf("pedidos sin token: %d, con el cuerpo malo: %d", api.sinToken.Load(), api.cuerposMalos.Load())
	}
	if !(n.P50Ms > 0 && n.P50Ms <= n.P95Ms && n.P95Ms <= n.P99Ms && n.P99Ms <= n.MaxMs) {
		t.Errorf("percentiles fuera de orden: p50 %v, p95 %v, p99 %v, máx %v", n.P50Ms, n.P95Ms, n.P99Ms, n.MaxMs)
	}
	if n.PedidosPorSegundo <= 0 || n.DuracionS <= 0 {
		t.Errorf("pedidos por segundo = %v en %v s", n.PedidosPorSegundo, n.DuracionS)
	}
	if !n.Cumple {
		t.Errorf("sin errores y con p99 = %v ms bajo el límite, el nivel tiene que cumplir", n.P99Ms)
	}
}

// Con C clientes tiene que haber C pedidos en vuelo a la vez, y nunca más.
func TestMedirNivelUsaClientesConcurrentes(t *testing.T) {
	const clientes = 6
	var enVuelo, maximo atomic.Int64
	lleno := make(chan struct{})
	var una sync.Once
	api := nuevaAPIFalsa(t, func(w http.ResponseWriter, estacion string) {
		n := enVuelo.Add(1)
		defer enVuelo.Add(-1)
		for {
			m := maximo.Load()
			if n <= m || maximo.CompareAndSwap(m, n) {
				break
			}
		}
		if n == clientes {
			una.Do(func() { close(lleno) })
		}
		// La barrera: nadie responde hasta que los C clientes están adentro.
		select {
		case <-lleno:
		case <-time.After(3 * time.Second):
		}
		responderBien(w, estacion)
	})
	c := nuevoCliente(api.URL, tokenDePrueba, 10*time.Second)

	n, err := medirNivel(context.Background(), c, lecturasChicas(t), clientes, 60, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if n.Errores != 0 {
		t.Fatalf("errores = %d (%v)", n.Errores, n.ErroresPorTipo)
	}
	if got := maximo.Load(); got != clientes {
		t.Errorf("hubo hasta %d pedidos a la vez, quiero exactamente %d", got, clientes)
	}
}

func TestMedirNivelCuentaErroresYRespuestasDistintas(t *testing.T) {
	var n atomic.Int64
	api := nuevaAPIFalsa(t, func(w http.ResponseWriter, estacion string) {
		switch i := n.Add(1); {
		case i%10 == 0: // 6 de 60
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, `{"error":"no hay modelo activo"}`)
		case i%10 == 5: // 6 de 60: responde, pero otro régimen
			_ = json.NewEncoder(w).Encode(map[string]any{"estacion": estacion, "regimen": regimenDe[estacion] + 1})
		default:
			responderBien(w, estacion)
		}
	})
	c := nuevoCliente(api.URL, tokenDePrueba, 5*time.Second)

	niv, err := medirNivel(context.Background(), c, lecturasChicas(t), 4, 60, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if niv.Errores != 6 || niv.ErroresPorTipo["http 503"] != 6 {
		t.Errorf("errores = %d (%v), quiero 6 de http 503", niv.Errores, niv.ErroresPorTipo)
	}
	if niv.RespuestasDistintas != 6 {
		t.Errorf("respuestas con otro régimen = %d, quiero 6", niv.RespuestasDistintas)
	}
	if niv.Cumple {
		t.Error("un nivel con errores no cumple")
	}
}

func TestMedirNivelNoCumpleSiElP99PasaElLimite(t *testing.T) {
	api := nuevaAPIFalsa(t, func(w http.ResponseWriter, estacion string) {
		time.Sleep(5 * time.Millisecond)
		responderBien(w, estacion)
	})
	c := nuevoCliente(api.URL, tokenDePrueba, 5*time.Second)

	niv, err := medirNivel(context.Background(), c, lecturasChicas(t), 2, 20, 0, 1) // límite de 1 ms
	if err != nil {
		t.Fatal(err)
	}
	if niv.Cumple || niv.SobreElLimite != 20 {
		t.Errorf("cumple = %v con %d pedidos sobre el límite; quiero false y 20", niv.Cumple, niv.SobreElLimite)
	}
}

func TestMedirNivelSeCancela(t *testing.T) {
	api := nuevaAPIFalsa(t, responderBien)
	c := nuevoCliente(api.URL, tokenDePrueba, 5*time.Second)
	ctx, cancelar := context.WithCancel(context.Background())
	cancelar()
	if _, err := medirNivel(ctx, c, lecturasChicas(t), 2, 50, 0, 100); err == nil {
		t.Fatal("con el contexto cancelado no dio error")
	}
}

func configDePrueba(t *testing.T, url string) config {
	t.Helper()
	dir := t.TempDir()
	lecturas := filepath.Join(dir, "lecturas.csv")
	if err := os.WriteFile(lecturas, []byte(csvChico), 0o644); err != nil {
		t.Fatal(err)
	}
	return config{
		url: url, usuario: "demo", clave: "secreta", lecturas: lecturas, clientes: []int{1, 4},
		pedidos: 40, calentamiento: 4, limiteMs: 100, plazo: 5 * time.Second,
		salida: filepath.Join(dir, "reports", "api_latencia.json"),
		admin:  "admin", claveAdmin: "secreta-admin", esperaEscrituras: 2 * time.Second,
	}
}

func TestCorrerEscribeElInforme(t *testing.T) {
	api := nuevaAPIFalsa(t, responderBien)
	cfg := configDePrueba(t, api.URL)

	var pantalla bytes.Buffer
	inf, err := correr(context.Background(), cfg, &pantalla)
	if err != nil {
		t.Fatal(err)
	}
	if !inf.Cumple {
		t.Error("el informe tiene que cumplir")
	}

	crudo, err := os.ReadFile(cfg.salida)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(crudo, []byte("secreta")) || bytes.Contains(crudo, []byte(tokenDePrueba)) || bytes.Contains(crudo, []byte(tokenDeAdmin)) {
		t.Error("el informe no puede traer la clave ni el token")
	}
	var leido struct {
		Metadatos struct {
			Momento time.Time `json:"momento"`
			Maquina string    `json:"maquina"`
			Commit  string    `json:"commit"`
		} `json:"metadatos"`
		Parametros struct {
			URL             string `json:"url"`
			Ruta            string `json:"ruta"`
			Lecturas        string `json:"lecturas"`
			LecturasSha256  string `json:"lecturas_sha256"`
			Filas           int    `json:"filas"`
			Clientes        []int  `json:"clientes"`
			PedidosPorNivel int    `json:"pedidos_por_nivel"`
			Calentamiento   int    `json:"calentamiento"`
			LimiteMs        int    `json:"limite_ms"`
		} `json:"parametros"`
		Servidor map[string]any `json:"servidor"`
		Niveles  []struct {
			Clientes          int     `json:"clientes"`
			Pedidos           int     `json:"pedidos"`
			Errores           int     `json:"errores"`
			P50Ms             float64 `json:"p50_ms"`
			P95Ms             float64 `json:"p95_ms"`
			P99Ms             float64 `json:"p99_ms"`
			MaxMs             float64 `json:"max_ms"`
			PedidosPorSegundo float64 `json:"pedidos_por_segundo"`
		} `json:"niveles"`
		Cumple bool `json:"cumple"`
	}
	if err := json.Unmarshal(crudo, &leido); err != nil {
		t.Fatal(err)
	}
	if leido.Metadatos.Momento.IsZero() || leido.Metadatos.Maquina == "" {
		t.Errorf("faltan la fecha o la máquina: %+v", leido.Metadatos)
	}
	p := leido.Parametros
	if p.URL != api.URL || p.Ruta != "POST /api/predicciones" || p.Filas != 3 || p.PedidosPorNivel != 40 ||
		p.Calentamiento != 4 || p.LimiteMs != 100 || len(p.LecturasSha256) != 64 || len(p.Clientes) != 2 {
		t.Errorf("parámetros = %+v", p)
	}
	if leido.Servidor["modelo_activo"] != "sembrado-kmeans-json" {
		t.Errorf("servidor = %v: quiero lo que dijo /api/salud", leido.Servidor)
	}
	if len(leido.Niveles) != 2 || leido.Niveles[0].Clientes != 1 || leido.Niveles[1].Clientes != 4 {
		t.Fatalf("niveles = %+v", leido.Niveles)
	}
	for _, n := range leido.Niveles {
		if n.Pedidos != 40 || n.Errores != 0 || n.P50Ms <= 0 || n.P99Ms > n.MaxMs || n.PedidosPorSegundo <= 0 {
			t.Errorf("nivel = %+v", n)
		}
	}
	// La tabla sale por pantalla, una fila por nivel.
	for _, quiero := range []string{"clientes", "p50", "p95", "p99", "pedidos/s", "errores"} {
		if !strings.Contains(pantalla.String(), quiero) {
			t.Errorf("la tabla no trae %q:\n%s", quiero, pantalla.String())
		}
	}
}

func TestCorrerFallaConLaClaveMala(t *testing.T) {
	api := nuevaAPIFalsa(t, responderBien)
	cfg := configDePrueba(t, api.URL)
	cfg.clave = "otra"

	_, err := correr(context.Background(), cfg, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("err = %v: quiero que diga que el token no salió (401)", err)
	}
	if _, errArchivo := os.Stat(cfg.salida); errArchivo == nil {
		t.Error("sin token no se mide nada: no tiene que haber informe")
	}
	if api.predicciones.Load() != 0 {
		t.Error("sin token no se manda ninguna predicción")
	}
}

func TestCorrerInformaQueNoCumple(t *testing.T) {
	api := nuevaAPIFalsa(t, func(w http.ResponseWriter, estacion string) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	cfg := configDePrueba(t, api.URL)

	inf, err := correr(context.Background(), cfg, io.Discard)
	if err != nil {
		t.Fatal(err) // medir errores no es un error del comando: el informe se escribe igual
	}
	if inf.Cumple {
		t.Error("con todas las predicciones en 500, el informe no cumple")
	}
	if _, err := os.Stat(cfg.salida); err != nil {
		t.Errorf("el informe se escribe aunque no cumpla: %v", err)
	}
}

// Las predicciones responden 200 antes de escribirse: lo que pasó con ellas en
// MongoDB y en Redis sale de /api/metricas, antes y después de la corrida.
func TestCorrerMideLasEscrituras(t *testing.T) {
	api := nuevaAPIFalsa(t, responderBien)
	cfg := configDePrueba(t, api.URL)

	var pantalla bytes.Buffer
	inf, err := correr(context.Background(), cfg, &pantalla)
	if err != nil {
		t.Fatal(err)
	}
	const aceptadas = 2 * (40 + 4) // dos niveles, con su calentamiento
	e := inf.Escrituras
	if !e.Medido || e.PrediccionesAceptadas != aceptadas || !e.Completas {
		t.Fatalf("escrituras = %+v: quiero medido, %d aceptadas y completas", e, aceptadas)
	}
	for nombre, a := range map[string]*almacenEscritura{"mongo": e.Mongo, "redis": e.Redis} {
		if a == nil || a.Escritas != aceptadas || a.Descartadas != 0 || a.Fallidas != 0 {
			t.Errorf("%s = %+v: quiero %d escritas y nada perdido", nombre, a, aceptadas)
		}
	}
	if e.Perdidas() != 0 {
		t.Errorf("perdidas = %d, quiero 0", e.Perdidas())
	}
	if !strings.Contains(pantalla.String(), "escrituras") || strings.Contains(pantalla.String(), "AVISO") {
		t.Errorf("la pantalla tiene que decir las escrituras, sin aviso:\n%s", pantalla.String())
	}

	crudo, err := os.ReadFile(cfg.salida)
	if err != nil {
		t.Fatal(err)
	}
	var leido struct {
		Escrituras struct {
			Medido                bool `json:"medido"`
			PrediccionesAceptadas int  `json:"predicciones_aceptadas"`
			Completas             bool `json:"completas"`
			Mongo                 struct {
				Escritas    int `json:"escritas"`
				Descartadas int `json:"descartadas"`
				Fallidas    int `json:"fallidas"`
				Lotes       int `json:"lotes"`
			} `json:"mongo"`
			Redis struct {
				Escritas int `json:"escritas"`
			} `json:"redis"`
		} `json:"escrituras"`
	}
	if err := json.Unmarshal(crudo, &leido); err != nil {
		t.Fatal(err)
	}
	le := leido.Escrituras
	if !le.Medido || !le.Completas || le.PrediccionesAceptadas != aceptadas || le.Mongo.Escritas != aceptadas || le.Redis.Escritas != aceptadas || le.Mongo.Lotes == 0 {
		t.Errorf("escrituras en el archivo = %+v", le)
	}
}

// Solo cuenta lo de esta corrida: lo que la API ya traía de antes se resta.
func TestCorrerRestaLasEscriturasAnteriores(t *testing.T) {
	api := nuevaAPIFalsa(t, responderBien)
	api.escritura = func(validas, _ int64) (almacenEscritura, almacenEscritura) {
		return almacenEscritura{Escritas: 1000 + uint64(validas), Descartadas: 7, Fallidas: 3, Lotes: 500 + uint64(validas)},
			almacenEscritura{Escritas: 2000 + uint64(validas), Lotes: 900 + uint64(validas)}
	}
	cfg := configDePrueba(t, api.URL)

	inf, err := correr(context.Background(), cfg, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	e := inf.Escrituras
	if e.Mongo.Escritas != 88 || e.Mongo.Descartadas != 0 || e.Mongo.Fallidas != 0 || e.Mongo.Lotes != 88 || e.Redis.Escritas != 88 {
		t.Errorf("mongo = %+v, redis = %+v: quiero solo las 88 de esta corrida", e.Mongo, e.Redis)
	}
}

func TestCorrerDejaALaVistaLasEscriturasPerdidas(t *testing.T) {
	api := nuevaAPIFalsa(t, responderBien)
	// MongoDB descarta una de cada cuatro y Redis rechaza dos.
	api.escritura = func(validas, _ int64) (almacenEscritura, almacenEscritura) {
		v := uint64(validas)
		fallidas := min(v, 2)
		return almacenEscritura{Escritas: v - v/4, Descartadas: v / 4, Lotes: v}, almacenEscritura{Escritas: v - fallidas, Fallidas: fallidas, Lotes: v}
	}
	cfg := configDePrueba(t, api.URL)

	var pantalla bytes.Buffer
	inf, err := correr(context.Background(), cfg, &pantalla)
	if err != nil {
		t.Fatal(err)
	}
	e := inf.Escrituras
	if e.Mongo.Descartadas != 22 || e.Redis.Fallidas != 2 || e.Perdidas() != 24 || !e.Completas {
		t.Errorf("escrituras = %+v (mongo %+v, redis %+v): quiero 22 descartadas en mongo y 2 fallidas en redis", e, e.Mongo, e.Redis)
	}
	if !strings.Contains(pantalla.String(), "AVISO") {
		t.Errorf("con escrituras perdidas la pantalla tiene que avisar:\n%s", pantalla.String())
	}
	// La latencia se cumplió: «cumple» es de los pedidos, y las escrituras van aparte.
	if !inf.Cumple {
		t.Error("cumple es de la latencia y los errores de los pedidos; las escrituras perdidas se informan aparte")
	}
}

// La API escribe por lotes después de responder: se espera a que los dos
// almacenes den cuenta de todas las predicciones.
func TestCorrerEsperaAQueSeEscriban(t *testing.T) {
	api := nuevaAPIFalsa(t, responderBien)
	api.escritura = func(validas, consulta int64) (almacenEscritura, almacenEscritura) {
		// Cada consulta ve 20 predicciones escritas más, hasta llegar a todas.
		v := uint64(min(validas, max(0, consulta-1)*20))
		return almacenEscritura{Escritas: v, Lotes: v}, almacenEscritura{Escritas: v, Lotes: v}
	}
	cfg := configDePrueba(t, api.URL)

	inf, err := correr(context.Background(), cfg, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if e := inf.Escrituras; !e.Completas || e.Mongo.Escritas != 88 || e.Redis.Escritas != 88 {
		t.Errorf("escrituras = %+v (mongo %+v): quiero haber esperado a las 88", e, e.Mongo)
	}
	if api.metricas.Load() < 4 {
		t.Errorf("consultó /api/metricas %d veces: tenía que volver a preguntar hasta verlas todas", api.metricas.Load())
	}
}

func TestCorrerNoEsperaMasQueElPlazo(t *testing.T) {
	api := nuevaAPIFalsa(t, responderBien)
	api.escritura = func(validas, _ int64) (almacenEscritura, almacenEscritura) {
		v := uint64(validas) / 2 // la mitad nunca aparece
		return almacenEscritura{Escritas: v, Lotes: v}, almacenEscritura{Escritas: v, Lotes: v}
	}
	cfg := configDePrueba(t, api.URL)
	cfg.esperaEscrituras = 150 * time.Millisecond

	var pantalla bytes.Buffer
	inicio := time.Now()
	inf, err := correr(context.Background(), cfg, &pantalla)
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(inicio) > 5*time.Second {
		t.Errorf("tardó %v: el plazo de espera era de 150 ms", time.Since(inicio))
	}
	e := inf.Escrituras
	if e.Completas || e.Mongo.Escritas != 44 || e.SinCuenta() != 44 {
		t.Errorf("escrituras = %+v (mongo %+v): quiero incompletas, con 44 escritas y 44 sin cuenta", e, e.Mongo)
	}
	if !strings.Contains(pantalla.String(), "AVISO") {
		t.Errorf("si no dieron cuenta de todas, la pantalla tiene que avisar:\n%s", pantalla.String())
	}
}

func TestCorrerSinAdminNoMideLasEscrituras(t *testing.T) {
	api := nuevaAPIFalsa(t, responderBien)
	cfg := configDePrueba(t, api.URL)
	cfg.claveAdmin = ""

	var pantalla bytes.Buffer
	inf, err := correr(context.Background(), cfg, &pantalla)
	if err != nil {
		t.Fatal(err)
	}
	e := inf.Escrituras
	if e.Medido || e.Motivo == "" || e.Mongo != nil || e.Redis != nil {
		t.Errorf("escrituras = %+v: sin clave de admin no se miden, y se dice por qué", e)
	}
	if api.metricas.Load() != 0 {
		t.Errorf("sin admin no se consulta /api/metricas (%d consultas)", api.metricas.Load())
	}
	if !strings.Contains(pantalla.String(), "CARGA_CLAVE_ADMIN") {
		t.Errorf("la pantalla tiene que decir cómo medirlas:\n%s", pantalla.String())
	}
	if !inf.Cumple || len(inf.Niveles) != 2 {
		t.Error("sin admin se mide la latencia igual")
	}
}

func TestCorrerFallaSiElAdminNoPuedeLeerLasMetricas(t *testing.T) {
	api := nuevaAPIFalsa(t, responderBien)
	cfg := configDePrueba(t, api.URL)
	cfg.admin, cfg.claveAdmin = "demo", "secreta" // existe, pero es de rol usuario

	_, err := correr(context.Background(), cfg, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("err = %v: quiero que diga que /api/metricas respondió 403", err)
	}
	if api.predicciones.Load() != 0 {
		t.Error("si se pidió medir las escrituras y no se puede, no se mide nada")
	}
}

func TestLeerConfig(t *testing.T) {
	entorno := map[string]string{"CARGA_CLAVE": "del-entorno", "CARGA_CLAVE_ADMIN": "admin-del-entorno"}
	cfg, err := leerConfig([]string{"-clientes", "2,8", "-pedidos", "100", "-url", "http://localhost:18080/"}, func(k string) string { return entorno[k] })
	if err != nil {
		t.Fatal(err)
	}
	if cfg.clave != "del-entorno" || cfg.url != "http://localhost:18080" || len(cfg.clientes) != 2 || cfg.pedidos != 100 {
		t.Errorf("config = %+v", cfg)
	}
	if cfg.salida != filepath.Join("reports", "api_latencia.json") || cfg.limiteMs != 100 {
		t.Errorf("valores por omisión: %+v", cfg)
	}
	if cfg.admin != "admin" || cfg.claveAdmin != "admin-del-entorno" || cfg.esperaEscrituras <= 0 {
		t.Errorf("admin = %q, clave del admin = %q, espera = %v", cfg.admin, cfg.claveAdmin, cfg.esperaEscrituras)
	}
	// La clave del admin es opcional: sin ella no se miden las escrituras.
	solo, err := leerConfig(nil, func(k string) string { return map[string]string{"CARGA_CLAVE": "x"}[k] })
	if err != nil || solo.claveAdmin != "" {
		t.Errorf("sin CARGA_CLAVE_ADMIN: config = %+v, err = %v", solo, err)
	}

	malas := [][]string{
		{"-clientes", "0"},
		{"-pedidos", "0"},
		{"-url", "localhost:8080"},
		{"-calentamiento", "-1"},
		{"de-mas"},
	}
	for _, args := range malas {
		if _, err := leerConfig(args, func(k string) string { return entorno[k] }); err == nil {
			t.Errorf("leerConfig(%v) no dio error", args)
		}
	}
	if _, err := leerConfig(nil, func(string) string { return "" }); err == nil {
		t.Error("sin clave tiene que dar error")
	}
}
