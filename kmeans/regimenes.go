package kmeans

import (
	"fmt"
	"slices"
)

// Umbrales del ECA de aire para 24 horas (DS 003-2017-MINAM), en µg/m³.
//
// El ECA es un promedio de 24 horas: que la media de un régimen de lecturas
// horarias lo supere dice que ese régimen es peligroso, no que haya habido un
// incumplimiento. Una lectura horaria alta tampoco lo es por sí sola.
const (
	ECAPM25 = 50.0
	ECAPM10 = 100.0
)

// Etiquetas frente al ECA.
const (
	ECAExcede      = "excede"
	ECANoExcede    = "no_excede"
	ECANoEvaluable = "no_evaluable"
)

// Origen del PM2.5 de un régimen cuando no es una feature.
const OrigenFraccionPorPM10 = "fraccion_fina*pm10"

// Regimen es el perfil de un cluster: cuántas lecturas tiene, cuánto valen en
// promedio sus parámetros en unidades originales, si su nivel de PM supera el
// ECA y dónde y cuándo aparece.
//
// Perfil es la media ARITMÉTICA de cada feature en unidades originales, sobre
// las filas del régimen: cada fila se des-estandariza (z-score y log1p
// invertidos, con la estadística de su estación si el nivel es «estacion») y
// después se promedia. No es el centroide des-estandarizado: con log1p, invertir
// el centroide da la media de los logaritmos, que es menor que la media de los
// valores (desigualdad de Jensen), y el ECA se compara con una media aritmética.
type Regimen struct {
	Regimen  int                `json:"regimen"`
	Tamano   int                `json:"tamano"`
	Fraccion float64            `json:"fraccion"`
	Perfil   map[string]float64 `json:"perfil"`

	// PM25Medio es la media de PM2.5 en µg/m³: la de la feature pm2_5 si está
	// en la receta; si no, la de fraccion_fina × pm10 fila por fila (que
	// reconstruye el PM2.5 de cada lectura); si tampoco, nil.
	PM25Medio  *float64 `json:"pm2_5_medio"`
	PM25Origen string   `json:"pm2_5_origen,omitempty"`
	PM10Medio  *float64 `json:"pm10_medio"`
	ECA        string   `json:"eca"`
	ECAPor     []string `json:"eca_por,omitempty"`

	PorHora     [24]int        `json:"por_hora"` // hora del día en Lima (UTC-5)
	PorEstacion map[string]int `json:"por_estacion"`
}

// EtiquetaECA decide si un régimen excede el ECA: si su PM2.5 medio supera
// 50 µg/m³ o su PM10 medio supera 100 µg/m³. Devuelve además qué umbrales
// superó. Con un solo contaminante disponible se puede afirmar que excede,
// pero no que no excede (podría exceder por el otro): eso es no_evaluable.
func EtiquetaECA(pm25, pm10 *float64) (string, []string) {
	var por []string
	if pm25 != nil && *pm25 > ECAPM25 {
		por = append(por, FeaturePM25)
	}
	if pm10 != nil && *pm10 > ECAPM10 {
		por = append(por, FeaturePM10)
	}
	switch {
	case len(por) > 0:
		return ECAExcede, por
	case pm25 != nil && pm10 != nil:
		return ECANoExcede, nil
	default:
		return ECANoEvaluable, nil
	}
}

// Perfilar arma el perfil de cada uno de los k regímenes a partir de las
// asignaciones. Recorre el gold una vez: des-estandariza cada fila y acumula
// por régimen las medias, el PM2.5, la hora del día y la estación.
func Perfilar(g *Gold, asign []int, k int, e *Estandarizacion) ([]Regimen, error) {
	d := g.Datos
	if len(asign) != d.N {
		return nil, fmt.Errorf("perfilar: %d asignaciones para %d filas", len(asign), d.N)
	}
	if k <= 0 {
		return nil, fmt.Errorf("perfilar: k debe ser positivo, es %d", k)
	}
	if !slices.Equal(e.Features, g.Features) {
		return nil, fmt.Errorf("perfilar: el gold tiene las features %v y la estandarización %v", g.Features, e.Features)
	}
	iPM25 := slices.Index(g.Features, FeaturePM25)
	iPM10 := slices.Index(g.Features, FeaturePM10)
	iFF := slices.Index(g.Features, FeatureFraccionFina)
	origen := ""
	switch {
	case iPM25 >= 0:
		origen = FeaturePM25
	case iFF >= 0 && iPM10 >= 0:
		origen = OrigenFraccionPorPM10
	}

	dim := d.D
	sumas := make([]float64, k*dim)
	sumaPM25 := make([]float64, k)
	regs := make([]Regimen, k)
	for j := range regs {
		regs[j].Regimen = j
		regs[j].PorEstacion = map[string]int{}
	}

	x := make([]float64, dim)
	for i := 0; i < d.N; i++ {
		j := asign[i]
		if j < 0 || j >= k {
			return nil, fmt.Errorf("perfilar: la fila %d tiene el régimen %d, fuera de [0, %d)", i, j, k)
		}
		if err := e.Invertir(d.Punto(i), g.Estacion[i], x); err != nil {
			return nil, fmt.Errorf("perfilar, fila %d: %w", i, err)
		}
		s := sumas[j*dim : (j+1)*dim]
		for f := range x {
			s[f] += x[f]
		}
		switch origen {
		case FeaturePM25:
			sumaPM25[j] += x[iPM25]
		case OrigenFraccionPorPM10:
			sumaPM25[j] += float64(x[iFF] * x[iPM10]) // sin FMA: ver noFusionar en secuencial.go
		}
		r := &regs[j]
		r.Tamano++
		r.PorHora[g.FechaHora[i].In(ZonaPeru).Hour()]++
		r.PorEstacion[g.Estacion[i]]++
	}

	for j := range regs {
		r := &regs[j]
		r.Fraccion = float64(r.Tamano) / float64(d.N)
		if r.Tamano == 0 {
			r.ECA = ECANoEvaluable
			continue
		}
		inv := 1 / float64(r.Tamano)
		r.Perfil = make(map[string]float64, dim)
		for f, nombre := range g.Features {
			r.Perfil[nombre] = sumas[j*dim+f] * inv
		}
		if origen != "" {
			v := sumaPM25[j] * inv
			r.PM25Medio, r.PM25Origen = &v, origen
		}
		if iPM10 >= 0 {
			v := r.Perfil[FeaturePM10]
			r.PM10Medio = &v
		}
		r.ECA, r.ECAPor = EtiquetaECA(r.PM25Medio, r.PM10Medio)
	}
	return regs, nil
}
