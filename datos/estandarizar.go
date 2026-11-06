package datos

import (
	"math"
	"slices"
	"sync"
	"time"

	"github.com/R0SEWT/aire-distribuido/kmeans"
)

// Dataset es el gold en memoria, con el mismo contrato que kmeans.Datos del TP:
// X guarda los puntos por filas (el punto i ocupa X[i*D : (i+1)*D]), todos
// finitos. Estacion y Hora no son features: sirven para agregar después del
// clustering (régimen × estación, régimen × hora).
type Dataset struct {
	X          []float64
	N          int
	D          int
	Features   []string       // nombres de las columnas de X, en el orden de la receta
	Estaciones []string       // la misma lista que la tabla ancha
	Estacion   []uint16       // por fila, índice en Estaciones
	Hora       []int64        // por fila, segundos Unix
	Zona       *time.Location // zona en que se escribe fecha_hora
}

// Punto devuelve las features de la fila i. Comparte memoria con X.
func (d *Dataset) Punto(i int) []float64 { return d.X[i*d.D : (i+1)*d.D] }

// ReporteEstandarizacion es reports/estandarizacion.json: lo que el agente del
// K-Means necesita para des-estandarizar los centroides. La desviación es la
// poblacional (ddof = 0), como en el TP; una feature con desviación 0 queda en 0.
type ReporteEstandarizacion struct {
	Features    []string                       `json:"features"`
	Log1p       []string                       `json:"log1p"`
	Nivel       string                         `json:"nivel"`
	N           int                            `json:"n"`
	Media       []float64                      `json:"media"`
	Desv        []float64                      `json:"desv"`
	PorEstacion map[string]EstadisticaEstacion `json:"por_estacion,omitempty"`
	Ddof        int                            `json:"ddof"`
	Chunk       int                            `json:"chunk"`
	// Procedencia (Anotar): máquina, Go y commit, y versión de cada config.
	Metadatos *kmeans.Metadatos         `json:"metadatos,omitempty"`
	Configs   map[string]VersionArchivo `json:"configs,omitempty"`
}

// EstadisticaEstacion son la media y la desviación de una estación (nivel = estacion).
type EstadisticaEstacion struct {
	N     int       `json:"n"`
	Media []float64 `json:"media"`
	Desv  []float64 `json:"desv"`
}

// parte es un rango de filas [desde, hasta). Con nivel = estacion, est es la
// estación de todas sus filas (un chunk nunca cruza dos estaciones).
type parte struct{ desde, hasta, est int }

func chunksAbsolutos(n, chunk int) []parte {
	var ps []parte
	for i := 0; i < n; i += chunk {
		ps = append(ps, parte{i, min(i+chunk, n), -1})
	}
	return ps
}

// chunksPorEstacion parte las filas (agrupadas por estación, que es como
// salen del pivot) en chunks que no cruzan de una estación a otra.
func chunksPorEstacion(est []uint16, chunk int) []parte {
	var ps []parte
	for ini := 0; ini < len(est); {
		fin := ini
		for fin < len(est) && est[fin] == est[ini] {
			fin++
		}
		for i := ini; i < fin; i += chunk {
			ps = append(ps, parte{i, min(i+chunk, fin), int(est[ini])})
		}
		ini = fin
	}
	return ps
}

// momentos son n, media y M2 (suma de cuadrados de desvíos) por feature.
type momentos struct {
	n     int
	media []float64
	m2    []float64
}

// welford recorre un chunk una sola vez. Es estable aunque la media sea grande
// frente a la desviación (presión: ~1000 hPa ± pocos hPa), donde
// Σx² − (Σx)²/n pierde casi todos los dígitos por cancelación.
func welford(x []float64, d int, p parte) momentos {
	m := momentos{media: make([]float64, d), m2: make([]float64, d)}
	for i := p.desde; i < p.hasta; i++ {
		m.n++
		inv := 1 / float64(m.n)
		for j, v := range x[i*d : (i+1)*d] {
			// float64(...) obliga a redondear el producto antes de sumar: sin
			// esto arm64 los funde en un FMA y el resultado cambia en el último
			// bit respecto de amd64 (lo mismo que se hizo en kmeans/).
			delta := v - m.media[j]
			m.media[j] += float64(delta * inv)
			m.m2[j] += float64(delta * (v - m.media[j]))
		}
	}
	return m
}

// fusionarMomentos combina dos chunks con la fórmula de Chan, Golub y LeVeque.
// No es asociativa en punto flotante: por eso se fusiona siempre en orden de
// chunk, y con el chunk fijo el resultado es el mismo bit a bit para cualquier W.
func fusionarMomentos(a, b momentos) momentos {
	if a.n == 0 {
		return momentos{n: b.n, media: slices.Clone(b.media), m2: slices.Clone(b.m2)}
	}
	if b.n == 0 {
		return momentos{n: a.n, media: slices.Clone(a.media), m2: slices.Clone(a.m2)}
	}
	n := a.n + b.n
	na, nb, nn := float64(a.n), float64(b.n), float64(n)
	out := momentos{n: n, media: make([]float64, len(a.media)), m2: make([]float64, len(a.m2))}
	for j := range a.media {
		delta := b.media[j] - a.media[j]
		out.media[j] = a.media[j] + float64(delta*nb/nn)
		out.m2[j] = a.m2[j] + b.m2[j] + float64(delta*delta*na*nb/nn)
	}
	return out
}

func (m momentos) desv() []float64 {
	out := make([]float64, len(m.m2))
	if m.n == 0 {
		return out
	}
	for j, s := range m.m2 {
		out[j] = math.Sqrt(s / float64(m.n))
	}
	return out
}

func momentosPorChunks(x []float64, d int, partes []parte, workers int) []momentos {
	out := make([]momentos, len(partes))
	enParalelo(workers, len(partes), func(c int) { out[c] = welford(x, d, partes[c]) })
	return out
}

func fusionarEnOrden(ms []momentos, d int) momentos {
	total := momentos{media: make([]float64, d), m2: make([]float64, d)}
	for _, m := range ms {
		total = fusionarMomentos(total, m)
	}
	return total
}

// enParalelo corre f(0..n-1) con un pool de hasta `workers` goroutines que
// toman índices de un canal. Cada f escribe solo su propio índice de salida,
// así que no hace falta Mutex; el WaitGroup es la barrera.
func enParalelo(workers, n int, f func(i int)) {
	if n == 0 {
		return
	}
	trabajos := make(chan int)
	var wg sync.WaitGroup
	for range max(1, min(workers, n)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range trabajos {
				f(i)
			}
		}()
	}
	for i := range n {
		trabajos <- i
	}
	close(trabajos)
	wg.Wait()
}

// fuenteFeature dice de dónde sale cada columna del gold.
type fuenteFeature struct {
	col      int // columna en la tabla ancha, -1 si no está
	fraccion bool
	log      bool
}

// construirGold aplica la regla 8 y la estandarización sobre la tabla ancha ya
// deduplicada y ordenada: (1) features derivadas y log1p, y filas completas,
// por chunks en paralelo y concatenadas en orden; (2) Welford por chunk y
// fusión de Chan en orden de chunk; (3) z-score en paralelo por chunks.
func construirGold(t *TablaAncha, rec Receta, workers, chunk int) (*Dataset, *ReporteEstandarizacion, error) {
	if err := rec.Validar(); err != nil {
		return nil, nil, err
	}
	d := len(rec.Features)
	col := func(nombre string) int { return slices.Index(t.Parametros, nombre) }
	fuentes := make([]fuenteFeature, d)
	for j, f := range rec.Features {
		fuentes[j] = fuenteFeature{col: col(f), fraccion: f == FeatureFraccionFina, log: slices.Contains(rec.Log1p, f)}
	}
	c10, c25, pa := col("pm10"), col("pm2_5"), len(t.Parametros)

	// (1) Features y completitud.
	type trozo struct {
		x     []float64
		filas []int32
	}
	partesAnchas := chunksAbsolutos(t.N, chunk)
	trozos := make([]trozo, len(partesAnchas))
	enParalelo(workers, len(partesAnchas), func(c int) {
		var tr trozo
		vec := make([]float64, d)
		for i := partesAnchas[c].desde; i < partesAnchas[c].hasta; i++ {
			fila := t.Valores[i*pa : (i+1)*pa]
			completa := true
			for j, f := range fuentes {
				v := math.NaN()
				switch {
				case f.fraccion:
					// PM2.5 está contenido en PM10: una fracción > 1 es error de
					// medición; se recorta a 1 y se cuenta en el reporte.
					if c10 >= 0 && c25 >= 0 && fila[c10] > 0 && !math.IsNaN(fila[c25]) {
						v = min(max(fila[c25]/fila[c10], 0), 1)
					}
				case f.col >= 0:
					v = fila[f.col]
				}
				if f.log {
					v = math.Log1p(v)
				}
				if math.IsNaN(v) || math.IsInf(v, 0) {
					completa = false
					break
				}
				vec[j] = v
			}
			if completa {
				tr.x = append(tr.x, vec...)
				tr.filas = append(tr.filas, int32(i))
			}
		}
		trozos[c] = tr
	})
	n := 0
	for _, tr := range trozos {
		n += len(tr.filas)
	}
	ds := &Dataset{
		X:          make([]float64, 0, n*d),
		N:          n,
		D:          d,
		Features:   slices.Clone(rec.Features),
		Estaciones: t.Estaciones,
		Estacion:   make([]uint16, 0, n),
		Hora:       make([]int64, 0, n),
		Zona:       t.Zona,
	}
	for _, tr := range trozos {
		ds.X = append(ds.X, tr.x...)
		for _, i := range tr.filas {
			ds.Estacion = append(ds.Estacion, t.Estacion[i])
			ds.Hora = append(ds.Hora, t.Hora[i])
		}
	}

	// (2) Media y desviación.
	partes := chunksAbsolutos(n, chunk)
	if rec.Nivel == NivelEstacion {
		partes = chunksPorEstacion(ds.Estacion, chunk)
	}
	ms := momentosPorChunks(ds.X, d, partes, workers)
	global := fusionarEnOrden(ms, d)
	rep := &ReporteEstandarizacion{
		Features: slices.Clone(rec.Features),
		Log1p:    append([]string{}, rec.Log1p...),
		Nivel:    rec.Nivel,
		N:        n,
		Media:    global.media,
		Desv:     global.desv(),
		Chunk:    chunk,
	}
	medias := map[int][]float64{-1: rep.Media}
	desvs := map[int][]float64{-1: rep.Desv}
	if rec.Nivel == NivelEstacion {
		porEst := map[int]momentos{}
		for c, p := range partes { // en orden de chunk
			porEst[p.est] = fusionarMomentos(porEst[p.est], ms[c])
		}
		rep.PorEstacion = make(map[string]EstadisticaEstacion, len(porEst))
		for e, m := range porEst {
			medias[e], desvs[e] = m.media, m.desv()
			rep.PorEstacion[ds.Estaciones[e]] = EstadisticaEstacion{N: m.n, Media: m.media, Desv: desvs[e]}
		}
	}

	// (3) z-score. Cada chunk escribe solo sus filas.
	enParalelo(workers, len(partes), func(c int) {
		p := partes[c]
		media, desv := medias[p.est], desvs[p.est]
		for i := p.desde; i < p.hasta; i++ {
			fila := ds.X[i*d : (i+1)*d]
			for j := range fila {
				if desv[j] > 0 {
					fila[j] = (fila[j] - media[j]) / desv[j]
				} else {
					fila[j] = 0
				}
			}
		}
	})
	return ds, rep, nil
}
