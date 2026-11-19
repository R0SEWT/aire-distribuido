package api

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
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
