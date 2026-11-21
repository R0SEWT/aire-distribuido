package api

import (
	"math"
	"net/http"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/R0SEWT/aire-distribuido/kmeans"
)

// Cubetas del histograma de latencias: 16 por octava a partir de 1 µs, hasta
// 2^27 µs (unos 134 s). Cada cubeta es 2^(1/16) = 4.4 % más ancha que la
// anterior, así que un percentil se conoce con ese error relativo, sin guardar
// cada medición y con memoria fija (433 contadores por ruta).
const (
	cubetasPorOctava = 16
	octavas          = 27
	numCubetas       = cubetasPorOctava*octavas + 1
	limiteMinimo     = time.Microsecond
)

// histograma cuenta pedidos por cubeta con contadores atómicos: observar no
// toma locks, y los handlers lo llaman desde todas sus goroutines a la vez.
type histograma struct {
	cubetas [numCubetas]atomic.Uint64
}

func (h *histograma) observar(d time.Duration) {
	i := 0
	if d > limiteMinimo {
		i = int(math.Ceil(math.Log2(float64(d)/float64(limiteMinimo)) * cubetasPorOctava))
		i = min(i, numCubetas-1)
	}
	h.cubetas[i].Add(1)
}

// resumenRuta es lo que /api/metricas publica por endpoint (MetricaEndpoint
// en el contrato).
type resumenRuta struct {
	Pedidos uint64  `json:"pedidos"`
	P50     float64 `json:"p50_ms"`
	P95     float64 `json:"p95_ms"`
	P99     float64 `json:"p99_ms"`
}

// resumir calcula los percentiles por rango más cercano: el p-ésimo es el
// límite superior de la cubeta donde el acumulado alcanza ceil(p·n). Es una
// cota: el valor exacto está en esa cubeta, nunca por encima de lo informado.
func (h *histograma) resumir() resumenRuta {
	var conteos [numCubetas]uint64
	var total uint64
	for i := range h.cubetas {
		conteos[i] = h.cubetas[i].Load()
		total += conteos[i]
	}
	if total == 0 {
		return resumenRuta{}
	}
	percentil := func(p float64) float64 {
		rango := max(uint64(math.Ceil(p*float64(total))), 1)
		var acum uint64
		for i, c := range conteos {
			acum += c
			if acum >= rango {
				return msCubeta(i)
			}
		}
		return msCubeta(numCubetas - 1)
	}
	return resumenRuta{Pedidos: total, P50: percentil(0.50), P95: percentil(0.95), P99: percentil(0.99)}
}

// msCubeta es el límite superior de la cubeta i en milisegundos, con cuatro
// cifras significativas.
func msCubeta(i int) float64 {
	ms := float64(limiteMinimo) / float64(time.Millisecond) * math.Exp2(float64(i)/cubetasPorOctava)
	v, _ := strconv.ParseFloat(strconv.FormatFloat(ms, 'g', 4, 64), 64)
	return v
}

// Metricas mide la latencia de cada pedido por ruta. La ruta es el patrón con
// que se registró el handler («POST /api/predicciones»), no la URL: así
// /api/estaciones/{id}/regimen es una sola serie.
type Metricas struct {
	mu    sync.RWMutex
	rutas map[string]*histograma
}

func NuevasMetricas() *Metricas { return &Metricas{rutas: map[string]*histograma{}} }

// Medir envuelve un handler: toma el tiempo desde que el pedido entra hasta que
// el handler termina, incluida la verificación del JWT.
func (m *Metricas) Medir(ruta string, sig http.Handler) http.Handler {
	m.mu.Lock()
	h, ok := m.rutas[ruta]
	if !ok {
		h = &histograma{}
		m.rutas[ruta] = h
	}
	m.mu.Unlock()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inicio := time.Now()
		sig.ServeHTTP(w, r)
		h.observar(time.Since(inicio))
	})
}

// Resumen devuelve los percentiles de las rutas que recibieron algún pedido.
func (m *Metricas) Resumen() map[string]resumenRuta {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := map[string]resumenRuta{}
	for ruta, h := range m.rutas {
		if r := h.resumir(); r.Pedidos > 0 {
			res[ruta] = r
		}
	}
	return res
}

// arranqueProceso es cuándo arrancó el proceso, con el margen de la
// inicialización de los paquetes: es contra lo que se compara el tiempo de CPU
// que el sistema operativo acumula desde el arranque.
var arranqueProceso = time.Now()

// estadoProceso es lo que /api/metricas publica en proceso_api (ProcesoAPI en
// el contrato): el CPU y la memoria de ESTE proceso, la API, que es el
// coordinador del clúster. No son de los nodos: el protocolo no transporta su
// CPU, y de ellos solo se conoce el tiempo de respuesta por ronda. Por eso los
// campos llevan el nombre de lo que son.
//
// El tiempo de CPU es el acumulado que da el sistema operativo, no un
// porcentaje: el uso entre dos consultas lo calcula quien consulta, como
// Δ(cpu_usuario_s + cpu_sistema_s) / Δen_marcha_s. Así la ruta no guarda
// estado ni depende de cada cuánto la llaman.
type estadoProceso struct {
	CPUUsuarioS  float64  `json:"cpu_usuario_s"`
	CPUSistemaS  float64  `json:"cpu_sistema_s"`
	EnMarchaS    float64  `json:"en_marcha_s"`
	Nucleos      int      `json:"nucleos"`
	HeapMB       float64  `json:"heap_mb"`
	SistemaMB    float64  `json:"sistema_mb"`
	MaxRSSMB     float64  `json:"max_rss_mb"`
	Goroutines   int      `json:"goroutines"`
	NoDisponible []string `json:"no_disponible,omitempty"`
}

// medirProceso toma la foto con kmeans.MedirRecursos, lo mismo que usan
// cmd/entrenar y cmd/benchmark para sus informes. Cuesta una lectura de las
// estadísticas del runtime (decenas de microsegundos): es por consulta de un
// admin, no por pedido.
func medirProceso() estadoProceso {
	r := kmeans.MedirRecursos()
	return estadoProceso{
		CPUUsuarioS: milesimas(r.CPUUsuarioS), CPUSistemaS: milesimas(r.CPUSistemaS),
		EnMarchaS: milesimas(time.Since(arranqueProceso).Seconds()), Nucleos: runtime.GOMAXPROCS(0),
		HeapMB: milesimas(r.HeapMB), SistemaMB: milesimas(r.SistemaMB), MaxRSSMB: milesimas(r.MaxRSSMB),
		Goroutines: runtime.NumGoroutine(), NoDisponible: r.NoDisponible,
	}
}

// milesimas redondea a tres decimales: milisegundos de CPU y kilobytes.
func milesimas(v float64) float64 { return math.Round(v*1000) / 1000 }
