package datos

import (
	"math"
	"math/rand/v2"
	"strings"
	"testing"
)

// dosPasadas es la referencia: media en una pasada, varianza poblacional con
// la media ya conocida en la segunda. Lenta pero sin cancelación.
func dosPasadas(x []float64, d, desde, hasta int) (media, desv []float64) {
	n := float64(hasta - desde)
	media, desv = make([]float64, d), make([]float64, d)
	for i := desde; i < hasta; i++ {
		for j := 0; j < d; j++ {
			media[j] += x[i*d+j]
		}
	}
	for j := range media {
		media[j] /= n
	}
	for i := desde; i < hasta; i++ {
		for j := 0; j < d; j++ {
			e := x[i*d+j] - media[j]
			desv[j] += e * e
		}
	}
	for j := range desv {
		desv[j] = math.Sqrt(desv[j] / n)
	}
	return media, desv
}

func cerca(a, b float64) bool {
	return math.Abs(a-b) <= 1e-9*math.Max(1, math.Abs(b))
}

func TestWelfordChanIgualADosPasadas(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	const n, d = 5003, 3
	x := make([]float64, n*d)
	for i := 0; i < n; i++ {
		x[i*d] = 1000 + 3*rng.NormFloat64()        // presión: media grande, desviación chica
		x[i*d+1] = math.Exp(2 + rng.NormFloat64()) // PM: cola larga
		x[i*d+2] = 1e-3 * rng.NormFloat64()        // escala chica
	}
	mediaRef, desvRef := dosPasadas(x, d, 0, n)
	for _, chunk := range []int{1, 7, 100, 1000, n, 2 * n} {
		for _, w := range []int{1, 3, 8} {
			partes := chunksAbsolutos(n, chunk)
			m := momentosPorChunks(x, d, partes, w)
			total := fusionarEnOrden(m, d)
			for j := 0; j < d; j++ {
				if !cerca(total.media[j], mediaRef[j]) || !cerca(total.desv()[j], desvRef[j]) {
					t.Fatalf("chunk %d, W=%d, feature %d: Welford/Chan (%v, %v) vs dos pasadas (%v, %v)",
						chunk, w, j, total.media[j], total.desv()[j], mediaRef[j], desvRef[j])
				}
			}
			if total.n != n {
				t.Fatalf("n = %d, quiero %d", total.n, n)
			}
		}
	}
}

func TestWelfordChanDeterministaParaCualquierW(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 4))
	const n, d = 2000, 2
	x := make([]float64, n*d)
	for i := range x {
		x[i] = 1000 + rng.NormFloat64()
	}
	partes := chunksAbsolutos(n, 37)
	ref := fusionarEnOrden(momentosPorChunks(x, d, partes, 1), d)
	for _, w := range []int{2, 5, 16} {
		otro := fusionarEnOrden(momentosPorChunks(x, d, partes, w), d)
		for j := 0; j < d; j++ {
			if math.Float64bits(ref.media[j]) != math.Float64bits(otro.media[j]) ||
				math.Float64bits(ref.m2[j]) != math.Float64bits(otro.m2[j]) {
				t.Fatalf("W=%d cambió el resultado bit a bit", w)
			}
		}
	}
}

func TestDesviacionCeroDaZCero(t *testing.T) {
	ancha := &TablaAncha{
		Parametros: []string{"humedad", "pm10"},
		Estaciones: []string{"A"},
		Estacion:   []uint16{0, 0, 0},
		Hora:       []int64{0, 3600, 7200},
		Valores:    []float64{50, 10, 50, 20, 50, 30},
		N:          3,
	}
	rec := Receta{Features: []string{"humedad", "pm10"}, Nivel: NivelAbsoluto}
	ds, est, err := construirGold(ancha, rec, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if est.Desv[0] != 0 {
		t.Fatalf("la humedad es constante, desviación %v", est.Desv[0])
	}
	for i := 0; i < ds.N; i++ {
		if ds.X[i*2] != 0 {
			t.Errorf("una feature constante tiene que quedar en 0, fila %d = %v", i, ds.X[i*2])
		}
	}
	if !cerca(ds.X[1], -math.Sqrt(1.5)) {
		t.Errorf("z de pm10 en la primera fila: %v", ds.X[1])
	}
}

func TestTransformacionesDelGold(t *testing.T) {
	m := generarMuestra()
	res := cargarBytes(t, m.csv, ConfigPorDefecto(), RecetaProvisional(), opMuestra)
	ds, est := res.Dataset, res.Estandarizacion
	if ds.D != 4 || len(est.Media) != 4 || est.N != ds.N {
		t.Fatalf("forma: D=%d, media %d, n %d vs N %d", ds.D, len(est.Media), est.N, ds.N)
	}
	// Reconstruir cada fila desde el silver y la estandarización publicada.
	col := map[string]int{}
	for j, p := range res.Ancha.Parametros {
		col[p] = j
	}
	pa := len(res.Ancha.Parametros)
	k, fracUno := 0, 0
	for i := 0; i < res.Ancha.N && k < ds.N; i++ {
		if res.Ancha.Estacion[i] != ds.Estacion[k] || res.Ancha.Hora[i] != ds.Hora[k] {
			continue
		}
		fila := res.Ancha.Valores[i*pa : (i+1)*pa]
		pm10, pm25 := fila[col["pm10"]], fila[col["pm2_5"]]
		frac := math.Min(pm25/pm10, 1)
		if pm25 > pm10 {
			fracUno++
		}
		crudos := []float64{math.Log1p(pm10), frac, fila[col["viento_velocidad"]], fila[col["humedad"]]}
		for j, v := range crudos {
			z := (v - est.Media[j]) / est.Desv[j]
			if !cerca(ds.X[k*4+j], z) {
				t.Fatalf("fila %d, feature %s: gold %v, reconstruido %v", k, ds.Features[j], ds.X[k*4+j], z)
			}
		}
		k++
	}
	if k != ds.N {
		t.Fatalf("solo se reconstruyeron %d de %d filas del gold", k, ds.N)
	}
	if fracUno == 0 {
		t.Error("la muestra tiene violaciones pm2_5 > pm10 completas: la fracción se recorta a 1")
	}
	// El gold estandarizado tiene media 0 y desviación 1 en cada columna.
	media, desv := dosPasadas(ds.X, ds.D, 0, ds.N)
	for j := range media {
		if math.Abs(media[j]) > 1e-9 || !cerca(desv[j], 1) {
			t.Errorf("feature %s: media %v, desv %v", ds.Features[j], media[j], desv[j])
		}
	}
}

func TestNivelEstacionEstandarizaCadaEstacionPorSeparado(t *testing.T) {
	m := generarMuestra()
	rec := RecetaProvisional()
	rec.Nivel = NivelEstacion
	res := cargarBytes(t, m.csv, ConfigPorDefecto(), rec, opMuestra)
	abs := cargarBytes(t, m.csv, ConfigPorDefecto(), RecetaProvisional(), opMuestra)
	ds, est := res.Dataset, res.Estandarizacion
	conFilas := map[uint16]bool{}
	for _, e := range ds.Estacion {
		conFilas[e] = true
	}
	if len(est.PorEstacion) != len(conFilas) || len(conFilas) < 2 {
		t.Fatalf("por_estacion tiene %d estaciones, el gold tiene filas de %d", len(est.PorEstacion), len(conFilas))
	}
	// Las features sin estandarizar se recuperan del gold absoluto (misma
	// selección de filas) deshaciendo su z-score global.
	crudo := make([]float64, len(abs.Dataset.X))
	for i := range crudo {
		j := i % abs.Dataset.D
		crudo[i] = abs.Dataset.X[i]*abs.Estandarizacion.Desv[j] + abs.Estandarizacion.Media[j]
	}
	desde := 0
	for e := range ds.Estaciones {
		hasta := desde
		for hasta < ds.N && int(ds.Estacion[hasta]) == e {
			hasta++
		}
		if hasta == desde {
			continue // una estación sin filas completas (SINT-D no mide viento) no se estandariza
		}
		nombre := ds.Estaciones[e]
		pe, ok := est.PorEstacion[nombre]
		if !ok || pe.N != hasta-desde {
			t.Fatalf("%s: por_estacion %+v, filas %d", nombre, pe, hasta-desde)
		}
		media, desv := dosPasadas(ds.X, ds.D, desde, hasta)
		mediaCruda, desvCruda := dosPasadas(crudo, ds.D, desde, hasta)
		for j := range media {
			if math.Abs(media[j]) > 1e-9 || !cerca(desv[j], 1) {
				t.Errorf("%s, %s: el z-score por estación da media %v y desv %v", nombre, ds.Features[j], media[j], desv[j])
			}
			if math.Abs(pe.Media[j]-mediaCruda[j]) > 1e-9*math.Max(1, math.Abs(mediaCruda[j])) ||
				math.Abs(pe.Desv[j]-desvCruda[j]) > 1e-9*math.Max(1, desvCruda[j]) {
				t.Errorf("%s, %s: por_estacion (%v, %v), dos pasadas (%v, %v)",
					nombre, ds.Features[j], pe.Media[j], pe.Desv[j], mediaCruda[j], desvCruda[j])
			}
		}
		desde = hasta
	}
	if desde != ds.N {
		t.Fatal("las filas del gold no quedaron agrupadas por estación")
	}
	// Las medias por estación tienen que diferir: si no, no se estandarizó por separado.
	a, b := est.PorEstacion[ds.Estaciones[ds.Estacion[0]]], est.PorEstacion[ds.Estaciones[ds.Estacion[ds.N-1]]]
	if a.Media[0] == b.Media[0] {
		t.Error("dos estaciones con la misma media de pm10: sospechoso")
	}
	if len(abs.Estandarizacion.PorEstacion) != 0 {
		t.Error("con nivel absoluto no va por_estacion")
	}
}

func TestFeatureAusenteDejaElGoldVacio(t *testing.T) {
	m := generarMuestra()
	rec := RecetaProvisional()
	rec.Features = []string{"pm10", "no2"} // nadie mide NO2 en la muestra
	rec.Log1p = nil
	res := cargarBytes(t, m.csv, ConfigPorDefecto(), rec, opMuestra)
	if res.Dataset.N != 0 {
		t.Errorf("sin NO2 no hay filas completas, hay %d", res.Dataset.N)
	}
	avisa := false
	for _, a := range res.Limpieza.Avisos {
		if strings.Contains(a, "no2") {
			avisa = true
		}
	}
	if !avisa {
		t.Errorf("tiene que avisar que la feature no2 no está en el silver: %v", res.Limpieza.Avisos)
	}
}
