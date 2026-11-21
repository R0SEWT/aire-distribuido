package api

import (
	"math"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/R0SEWT/aire-distribuido/kmeans"
)

func TestHistogramaPercentiles(t *testing.T) {
	var h histograma
	if r := h.resumir(); r != (resumenRuta{}) {
		t.Fatalf("histograma vacío: %+v", r)
	}
	// 1, 2, …, 1000 µs: p50 = 500 µs, p95 = 950 µs, p99 = 990 µs.
	for us := 1; us <= 1000; us++ {
		h.observar(time.Duration(us) * time.Microsecond)
	}
	r := h.resumir()
	if r.Pedidos != 1000 {
		t.Fatalf("pedidos = %d", r.Pedidos)
	}
	// El percentil sale del límite superior de su cubeta: nunca por debajo del
	// valor exacto y a lo sumo 2^(1/16) - 1 = 4.4 % por encima.
	for _, c := range []struct {
		nombre      string
		got, exacto float64
	}{{"p50", r.P50, 0.5}, {"p95", r.P95, 0.95}, {"p99", r.P99, 0.99}} {
		if c.got < c.exacto || c.got > c.exacto*1.045 {
			t.Errorf("%s = %v ms, el exacto es %v ms", c.nombre, c.got, c.exacto)
		}
	}
}

func TestHistogramaColas(t *testing.T) {
	var h histograma
	for range 98 {
		h.observar(100 * time.Microsecond)
	}
	h.observar(50 * time.Millisecond)
	h.observar(3 * time.Minute) // fuera de escala: va a la última cubeta
	r := h.resumir()
	if r.P50 < 0.1 || r.P50 > 0.1045 {
		t.Errorf("p50 = %v ms, se esperaba ~0.1", r.P50)
	}
	if r.P99 < 50 || r.P99 > 52.3 {
		t.Errorf("p99 = %v ms, se esperaba ~50 (el pedido 99 de 100)", r.P99)
	}
	var cero histograma
	cero.observar(0)
	if r := cero.resumir(); r.Pedidos != 1 || r.P99 != 0.001 {
		t.Errorf("una duración 0 va a la primera cubeta (≤ 1 µs): %+v", r)
	}
}

func TestMedirCuentaCadaPedidoPorRuta(t *testing.T) {
	m := NuevasMetricas()
	h := m.Medir("GET /x", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	m.Medir("GET /nunca", http.NotFoundHandler())
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 50 {
				h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/x", nil))
			}
		}()
	}
	wg.Wait()
	res := m.Resumen()
	if len(res) != 1 || res["GET /x"].Pedidos != 400 {
		t.Fatalf("Resumen = %+v; se esperaban 400 pedidos en GET /x y nada de la ruta sin pedidos", res)
	}
}

// Los campos obligatorios del esquema ProcesoAPI del contrato.
var camposProcesoAPI = []string{"cpu_usuario_s", "cpu_sistema_s", "en_marcha_s", "nucleos", "heap_mb", "sistema_mb", "max_rss_mb", "goroutines"}

// procesoAPI pide /api/metricas y devuelve su proceso_api, comprobando que
// traiga los campos del contrato y nada más.
func procesoAPI(t *testing.T, e *entorno, admin string) map[string]float64 {
	t.Helper()
	m := objeto(t, e.pedir(t, "GET", "/api/metricas", admin, ""), 200, "proceso_api", "endpoints", "escritura", "puntos_de_control")
	crudo, ok := m["proceso_api"].(map[string]any)
	if !ok {
		t.Fatalf("proceso_api = %v", m["proceso_api"])
	}
	p := map[string]float64{}
	for _, c := range camposProcesoAPI {
		v, ok := crudo[c].(float64)
		if !ok || v < 0 {
			t.Fatalf("proceso_api.%s = %v; tiene que ser un número no negativo", c, crudo[c])
		}
		p[c] = v
	}
	_, hayNoDisponible := crudo["no_disponible"]
	if extras := len(crudo) - len(camposProcesoAPI); extras != 0 && !(extras == 1 && hayNoDisponible) {
		t.Fatalf("proceso_api trae campos que el contrato no nombra: %v", crudo)
	}
	return p
}

// /api/metricas publica el CPU y la memoria del proceso de la API: tiempo de
// CPU acumulado, que crece cuando el proceso trabaja, y con el que quien
// consulta calcula el uso entre dos lecturas.
func TestMetricasPublicanElCPUYLaMemoriaDelProcesoDeLaAPI(t *testing.T) {
	e := nuevoEntorno(t)
	admin := e.token(t, "admin", "clave-admin")
	uno := procesoAPI(t, e, admin)
	if uno["nucleos"] != float64(runtime.GOMAXPROCS(0)) || uno["goroutines"] < 1 || uno["en_marcha_s"] <= 0 ||
		uno["heap_mb"] <= 0 || uno["sistema_mb"] < uno["heap_mb"] {
		t.Fatalf("proceso_api = %v", uno)
	}
	if len(kmeans.MedirRecursos().NoDisponible) > 0 {
		t.Skip("esta plataforma no mide el tiempo de CPU ni el RSS máximo (recursos_otros.go)")
	}
	if uno["max_rss_mb"] <= 0 || uno["cpu_usuario_s"]+uno["cpu_sistema_s"] <= 0 {
		t.Fatalf("sin CPU ni RSS medidos: %v", uno)
	}
	// 150 ms con un núcleo ocupado: el CPU acumulado tiene que notarlo.
	x := 0.0
	for limite := time.Now().Add(150 * time.Millisecond); time.Now().Before(limite); {
		for i := range 10000 {
			x += math.Sqrt(float64(i))
		}
	}
	_ = x
	dos := procesoAPI(t, e, admin)
	cpu := (dos["cpu_usuario_s"] + dos["cpu_sistema_s"]) - (uno["cpu_usuario_s"] + uno["cpu_sistema_s"])
	reloj := dos["en_marcha_s"] - uno["en_marcha_s"]
	if reloj < 0.15 || cpu < 0.05 || dos["max_rss_mb"] < uno["max_rss_mb"] {
		t.Fatalf("entre dos consultas: %.3f s de CPU en %.3f s de reloj (RSS máximo de %.1f a %.1f MB)", cpu, reloj, uno["max_rss_mb"], dos["max_rss_mb"])
	}
	// El uso que calcularía el panel: entre nada y todos los núcleos.
	if uso := cpu / reloj; uso <= 0 || uso > dos["nucleos"]*1.5 {
		t.Fatalf("uso de CPU = %.2f núcleos, con %v disponibles", uso, dos["nucleos"])
	}
}
