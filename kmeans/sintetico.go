package kmeans

import (
	"fmt"
	"math"
	"math/rand"
	"slices"
	"time"
)

// RegimenPlantado es un régimen de aire sintético: el valor típico de cada
// parámetro, en unidades originales (µg/m³, m/s, %).
type RegimenPlantado struct {
	Nombre string
	Valor  map[string]float64
}

// RegimenesDePrueba son cinco regímenes bien separados, con la etiqueta ECA
// que les toca escrita en el nombre. PM2.5 se plantea como fraccion_fina × pm10
// para que las dos formas de calcularlo coincidan.
func RegimenesDePrueba() []RegimenPlantado {
	r := func(nombre string, pm10, ff, viento, humedad, temp, presion float64) RegimenPlantado {
		return RegimenPlantado{Nombre: nombre, Valor: map[string]float64{
			FeaturePM10: pm10, FeatureFraccionFina: ff, FeaturePM25: pm10 * ff,
			"viento_velocidad": viento, "humedad": humedad, "temperatura": temp, "presion": presion,
		}}
	}
	return []RegimenPlantado{
		r("limpio (no excede)", 25, 0.40, 3.5, 70, 18, 1010),
		r("polvo con viento (excede por PM10)", 180, 0.20, 6.0, 40, 25, 1005),
		r("combustión con inversión (excede por PM2.5)", 80, 0.80, 0.8, 92, 15, 1015),
		r("moderado (no excede)", 50, 0.55, 2.0, 80, 20, 1012),
		r("episodio fuerte (excede por los dos)", 300, 0.50, 1.0, 60, 22, 1008),
	}
}

// OpcionesSintetico describe un gold sintético: Estaciones × Horas filas,
// horas consecutivas desde 2024-01-01T00:00-05:00, cada fila de un régimen
// plantado con ruido multiplicativo exp(Ruido·N(0,1)).
type OpcionesSintetico struct {
	Features   []string
	Log1p      []string
	Nivel      string
	Estaciones int
	Horas      int
	Ruido      float64
	Semilla    int64
	Regimenes  []RegimenPlantado // nil = RegimenesDePrueba()
}

// Sintetico es un gold sintético con su estandarización coherente y la verdad
// que se plantó, para que los tests puedan comprobar contra ella.
type Sintetico struct {
	Gold       *Gold
	Est        *Estandarizacion
	Plantado   []int     // N: régimen plantado de cada fila
	Originales []float64 // N*D: valores en unidades originales, antes de log1p y z-score
	Regimenes  []RegimenPlantado
}

// GoldSintetico arma un gold con regímenes plantados y lo estandariza como lo
// haría el cargador (log1p y z-score con desviación poblacional, global o por
// estación). Es para los tests y para ensayar los comandos sin el gold real;
// ninguna cifra que salga de aquí describe el aire de nadie.
func GoldSintetico(op OpcionesSintetico) (*Sintetico, error) {
	regs := op.Regimenes
	if regs == nil {
		regs = RegimenesDePrueba()
	}
	if op.Estaciones <= 0 || op.Horas <= 0 || len(op.Features) == 0 || len(regs) == 0 {
		return nil, fmt.Errorf("gold sintético: hacen falta estaciones, horas, features y regímenes")
	}
	if op.Estaciones > 99 {
		return nil, fmt.Errorf("gold sintético: a lo más 99 estaciones")
	}
	for _, f := range op.Features {
		for _, r := range regs {
			if _, ok := r.Valor[f]; !ok {
				return nil, fmt.Errorf("gold sintético: el régimen %q no tiene valor para %s", r.Nombre, f)
			}
		}
	}

	dim, n := len(op.Features), op.Estaciones*op.Horas
	s := &Sintetico{
		Gold: &Gold{
			Datos:     &Datos{D: dim, N: n, X: make([]float64, n*dim)},
			Features:  slices.Clone(op.Features),
			Estacion:  make([]string, n),
			FechaHora: make([]time.Time, n),
		},
		Est: &Estandarizacion{Features: slices.Clone(op.Features), Log1p: slices.Clone(op.Log1p),
			Nivel: op.Nivel, N: n},
		Plantado:   make([]int, n),
		Originales: make([]float64, n*dim),
		Regimenes:  regs,
	}

	rng := rand.New(rand.NewSource(op.Semilla))
	inicio := time.Date(2024, 1, 1, 0, 0, 0, 0, ZonaPeru)
	transformados := make([]float64, n*dim)
	for e := 0; e < op.Estaciones; e++ {
		nombre := fmt.Sprintf("EST%02d", e+1)
		for h := 0; h < op.Horas; h++ {
			i := e*op.Horas + h
			// Bloques de 6 horas que rotan por régimen, corridos por estación:
			// cada régimen aparece en todas las estaciones y a varias horas.
			reg := (e + h/6) % len(regs)
			s.Gold.Estacion[i] = nombre
			s.Gold.FechaHora[i] = inicio.Add(time.Duration(h) * time.Hour)
			s.Plantado[i] = reg
			for f, feat := range op.Features {
				x := regs[reg].Valor[feat] * math.Exp(op.Ruido*rng.NormFloat64())
				switch feat {
				case FeatureFraccionFina:
					x = math.Min(x, 1)
				case "humedad":
					x = math.Min(x, 100)
				}
				s.Originales[i*dim+f] = x
				if slices.Contains(op.Log1p, feat) {
					x = math.Log1p(x)
				}
				transformados[i*dim+f] = x
			}
		}
	}

	media, desv := momentos(transformados, dim, 0, n)
	s.Est.Media, s.Est.Desv = media, desv
	if op.Nivel == NivelEstacion {
		s.Est.PorEstacion = map[string]EstadisticaEstacion{}
		for e := 0; e < op.Estaciones; e++ {
			m, d := momentos(transformados, dim, e*op.Horas, (e+1)*op.Horas)
			s.Est.PorEstacion[fmt.Sprintf("EST%02d", e+1)] = EstadisticaEstacion{N: op.Horas, Media: m, Desv: d}
		}
	}
	if err := s.Est.Validar(); err != nil {
		return nil, fmt.Errorf("gold sintético: %w", err)
	}
	for i := 0; i < n; i++ {
		if err := s.Est.Estandarizar(s.Originales[i*dim:(i+1)*dim], s.Gold.Estacion[i], s.Gold.Datos.X[i*dim:(i+1)*dim]); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// momentos: media y desviación poblacional de las filas [desde, hasta), en dos
// pasadas.
func momentos(x []float64, dim, desde, hasta int) ([]float64, []float64) {
	media, desv := make([]float64, dim), make([]float64, dim)
	cnt := float64(hasta - desde)
	for i := desde; i < hasta; i++ {
		for f := 0; f < dim; f++ {
			media[f] += x[i*dim+f]
		}
	}
	for f := range media {
		media[f] /= cnt
	}
	for i := desde; i < hasta; i++ {
		for f := 0; f < dim; f++ {
			d := x[i*dim+f] - media[f]
			desv[f] += float64(d * d) // sin FMA: ver noFusionar en secuencial.go
		}
	}
	for f := range desv {
		desv[f] = math.Sqrt(desv[f] / cnt)
	}
	return media, desv
}
