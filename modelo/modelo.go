// Package modelo es el modelo activo de la API: los centroides del K-Means con
// la receta y la estandarización con que se entrenó, y el perfil de cada
// régimen frente al ECA. Predecir asigna una lectura en µg/m³ a su régimen.
//
// El modelo se siembra al arrancar desde los reports que deja el entrenamiento
// (spec de la PC4, §6.2): reports/kmeans.json (receta y centroides
// estandarizados, de cmd/entrenar), reports/regimenes.json (perfil de cada
// régimen y etiqueta ECA, de cmd/entrenar) y reports/estandarizacion.json
// (media y desviación del gold, de cmd/cargar). Sembrar comprueba que los tres
// sean de la misma corrida antes de aceptar el modelo.
//
// Un *Modelo no cambia después de creado: varias goroutines pueden predecir
// con él a la vez, sin locks, y la API lo reemplaza entero con un
// atomic.Pointer cuando hay un modelo nuevo.
package modelo

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"slices"

	"github.com/R0SEWT/aire-distribuido/kmeans"
)

// IDSembrado es el id del modelo que se siembra desde reports/ (spec §6.5).
const IDSembrado = "sembrado-kmeans-json"

// Archivos son las rutas de los tres reports con que se siembra el modelo.
type Archivos struct {
	Kmeans          string // reports/kmeans.json
	Regimenes       string // reports/regimenes.json
	Estandarizacion string // reports/estandarizacion.json
}

// Regimen es el perfil de un régimen que la API publica: sus medias horarias
// de PM2.5 y PM10 en µg/m³, si excede el ECA y cuántas lecturas del gold tiene.
// Los índices no implican un orden de gravedad.
type Regimen struct {
	Regimen   int     `json:"regimen"`
	PM25Medio float64 `json:"pm2_5_medio"`
	PM10Medio float64 `json:"pm10_medio"`
	ExcedeECA bool    `json:"excede_eca"`
	Tamano    int     `json:"tamano"`
}

// Modelo es un modelo listo para predecir.
type Modelo struct {
	ID          string
	Receta      kmeans.Receta
	Iteraciones int
	Inercia     float64
	Regimenes   []Regimen // uno por centroide, en orden de índice
	// CentroidesOriginales son los k centroides en unidades originales, en el
	// orden de las features. Es nil con nivel = estacion: un centroide mezcla
	// estaciones con medias distintas y no tiene unidades.
	CentroidesOriginales [][]float64

	centroides []float64 // k*D, estandarizados, por filas
	est        *kmeans.Estandarizacion
}

// K es la cantidad de regímenes.
func (m *Modelo) K() int { return len(m.Regimenes) }

// Features son las features del modelo, en el orden de los centroides.
func (m *Modelo) Features() []string { return slices.Clone(m.Receta.Features) }

// CentroidesEstandarizados devuelve una copia de los k*D centroides
// estandarizados, por filas: los que usa Predecir.
func (m *Modelo) CentroidesEstandarizados() []float64 { return slices.Clone(m.centroides) }

// Lectura es lo que llega a POST /api/predicciones, en µg/m³ y sin
// estandarizar. La estación solo se usa con nivel = estacion.
type Lectura struct {
	Estacion string
	PM25     float64
	PM10     float64
}

// ErrorLectura dice por qué una lectura no se puede predecir con este modelo.
// La API lo responde como 400.
type ErrorLectura struct{ Motivo string }

func (e *ErrorLectura) Error() string { return e.Motivo }

// Predecir asigna una lectura a su régimen: arma las features de la receta
// (pm2_5, pm10 o fraccion_fina = pm2_5/pm10 recortada a [0, 1], como el
// cargador), aplica log1p donde la receta lo pide y el z-score del gold, y toma
// el centroide más cercano con el contrato de kmeans (desempate por el menor
// índice). No toca ningún almacén.
func (m *Modelo) Predecir(l Lectura) (Regimen, error) {
	if !(l.PM25 >= 0) || math.IsInf(l.PM25, 0) {
		return Regimen{}, &ErrorLectura{fmt.Sprintf("pm2_5 tiene que ser un número no negativo, es %v", l.PM25)}
	}
	if !(l.PM10 >= 0) || math.IsInf(l.PM10, 0) {
		return Regimen{}, &ErrorLectura{fmt.Sprintf("pm10 tiene que ser un número no negativo, es %v", l.PM10)}
	}
	d := len(m.Receta.Features)
	x, z := make([]float64, d), make([]float64, d)
	for f, nombre := range m.Receta.Features {
		switch nombre {
		case kmeans.FeaturePM25:
			x[f] = l.PM25
		case kmeans.FeaturePM10:
			x[f] = l.PM10
		case kmeans.FeatureFraccionFina:
			if l.PM10 <= 0 {
				return Regimen{}, &ErrorLectura{"con pm10 = 0 no se puede calcular fraccion_fina (pm2_5/pm10), que el modelo usa"}
			}
			x[f] = min(max(l.PM25/l.PM10, 0), 1)
		}
	}
	if err := m.est.Estandarizar(x, l.Estacion, z); err != nil {
		return Regimen{}, &ErrorLectura{err.Error()}
	}
	j, _ := kmeans.MasCercano(z, m.centroides)
	return m.Regimenes[j], nil
}

// derivables son las features que se arman con una lectura de pm2_5 y pm10:
// lo único que recibe la API.
var derivables = []string{kmeans.FeaturePM25, kmeans.FeaturePM10, kmeans.FeatureFraccionFina}

// reporteKmeans es lo que se usa de reports/kmeans.json.
type reporteKmeans struct {
	Receta      kmeans.Receta `json:"receta"`
	Gold        infoGold      `json:"gold"`
	K           int           `json:"k"`
	Iteraciones int           `json:"iteraciones"`
	Inercia     float64       `json:"inercia"`
	Features    []string      `json:"features"`
	Centroides  [][]float64   `json:"centroides_estandarizados"`
}

// reporteRegimenes es lo que se usa de reports/regimenes.json.
type reporteRegimenes struct {
	Gold      infoGold         `json:"gold"`
	K         int              `json:"k"`
	Nivel     string           `json:"nivel"`
	Features  []string         `json:"features"`
	Regimenes []kmeans.Regimen `json:"regimenes"`
}

type infoGold struct {
	Sha256 string `json:"sha256"`
	N      int    `json:"n"`
}

// Sembrar lee los tres reports y arma el modelo IDSembrado. Falla si no son de
// la misma corrida (otro gold, otra receta, otro k) o si el modelo usa una
// feature que no sale de una lectura de pm2_5 y pm10.
func Sembrar(a Archivos) (*Modelo, error) {
	var rk reporteKmeans
	if err := leerJSON(a.Kmeans, &rk); err != nil {
		return nil, err
	}
	var rr reporteRegimenes
	if err := leerJSON(a.Regimenes, &rr); err != nil {
		return nil, err
	}
	est, err := kmeans.LeerEstandarizacion(a.Estandarizacion)
	if err != nil {
		return nil, err
	}
	m, err := armar(rk, rr, est)
	if err != nil {
		return nil, fmt.Errorf("sembrar el modelo desde %s, %s y %s: %w", a.Kmeans, a.Regimenes, a.Estandarizacion, err)
	}
	return m, nil
}

func leerJSON(ruta string, v any) error {
	b, err := os.ReadFile(ruta)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("%s: %w", ruta, err)
	}
	return nil
}

func armar(rk reporteKmeans, rr reporteRegimenes, est *kmeans.Estandarizacion) (*Modelo, error) {
	rec := rk.Receta
	if !slices.Equal(rk.Features, rec.Features) || !slices.Equal(rr.Features, rec.Features) {
		return nil, fmt.Errorf("features distintas: receta %v, kmeans.json %v, regimenes.json %v", rec.Features, rk.Features, rr.Features)
	}
	if rr.Nivel != rec.Nivel {
		return nil, fmt.Errorf("regimenes.json es de nivel %q y la receta, de %q", rr.Nivel, rec.Nivel)
	}
	if rk.Gold.Sha256 != rr.Gold.Sha256 || rk.Gold.N != rr.Gold.N {
		return nil, fmt.Errorf("kmeans.json y regimenes.json son de golds distintos (%s, n = %d; %s, n = %d)",
			rk.Gold.Sha256, rk.Gold.N, rr.Gold.Sha256, rr.Gold.N)
	}
	if est.N != rk.Gold.N {
		return nil, fmt.Errorf("la estandarización es de un gold de %d filas y el modelo, de uno de %d", est.N, rk.Gold.N)
	}
	k, d := rk.K, len(rec.Features)
	if k <= 0 || len(rk.Centroides) != k || rr.K != k || len(rr.Regimenes) != k {
		return nil, fmt.Errorf("k no coincide: kmeans.json dice %d con %d centroides; regimenes.json, %d con %d regímenes",
			rk.K, len(rk.Centroides), rr.K, len(rr.Regimenes))
	}
	centroides := make([]float64, 0, k*d)
	for j, c := range rk.Centroides {
		if len(c) != d {
			return nil, fmt.Errorf("el centroide %d tiene %d valores y hay %d features", j, len(c), d)
		}
		centroides = append(centroides, c...)
	}
	return Nuevo(Piezas{
		ID: IDSembrado, Receta: rec, Estandarizacion: est, Centroides: centroides,
		Iteraciones: rk.Iteraciones, Inercia: rk.Inercia, Regimenes: rr.Regimenes,
	})
}

// Piezas es lo que hace falta para armar un modelo: lo que deja un
// entrenamiento, venga de los reports (Sembrar) o del clúster (el coordinador
// de la API).
type Piezas struct {
	ID              string
	Receta          kmeans.Receta
	Estandarizacion *kmeans.Estandarizacion // la del gold con que se entrenó
	Centroides      []float64               // k*D, estandarizados, por filas
	Iteraciones     int
	Inercia         float64
	// Regimenes es el perfil de cada centroide, en orden de índice: lo que
	// devuelve kmeans.Perfilar con las asignaciones de esos centroides.
	Regimenes []kmeans.Regimen
}

// Nuevo arma un modelo listo para predecir y comprueba que las piezas sean
// coherentes entre sí: una receta válida cuyas features salen de una lectura
// de pm2_5 y pm10, la estandarización de esa receta, k centroides finitos de D
// valores y un régimen con su perfil por centroide. No copia Centroides: quien
// llama no los vuelve a tocar.
func Nuevo(p Piezas) (*Modelo, error) {
	rec, est := p.Receta, p.Estandarizacion
	if p.ID == "" {
		return nil, fmt.Errorf("el modelo no tiene id")
	}
	if est == nil {
		return nil, fmt.Errorf("el modelo no tiene estandarización")
	}
	if err := rec.Validar(); err != nil {
		return nil, err
	}
	for _, f := range rec.Features {
		if !slices.Contains(derivables, f) {
			return nil, fmt.Errorf("el modelo usa la feature %q, que no sale de una lectura de pm2_5 y pm10", f)
		}
	}
	if err := est.Coincide(rec); err != nil {
		return nil, err
	}
	k, d := len(p.Regimenes), len(rec.Features)
	if k == 0 || len(p.Centroides) != k*d {
		return nil, fmt.Errorf("%d valores de centroides no son k×D con %d regímenes y %d features", len(p.Centroides), k, d)
	}
	for i, v := range p.Centroides {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, fmt.Errorf("el centroide %d no es finito: %v", i/d, p.Centroides[i/d*d:(i/d+1)*d])
		}
	}
	m := &Modelo{
		ID:          p.ID,
		Receta:      rec,
		Iteraciones: p.Iteraciones,
		Inercia:     p.Inercia,
		Regimenes:   make([]Regimen, k),
		centroides:  p.Centroides,
		est:         est,
	}
	for j, r := range p.Regimenes {
		if r.Regimen != j {
			return nil, fmt.Errorf("regímenes: en la posición %d está el régimen %d", j, r.Regimen)
		}
		if r.PM25Medio == nil || r.PM10Medio == nil {
			return nil, fmt.Errorf("regímenes: al régimen %d le falta pm2_5_medio o pm10_medio", j)
		}
		if r.ECA != kmeans.ECAExcede && r.ECA != kmeans.ECANoExcede && r.ECA != kmeans.ECANoEvaluable {
			return nil, fmt.Errorf("regímenes: el régimen %d tiene la etiqueta ECA %q", j, r.ECA)
		}
		m.Regimenes[j] = Regimen{
			Regimen:   j,
			PM25Medio: *r.PM25Medio,
			PM10Medio: *r.PM10Medio,
			ExcedeECA: r.ECA == kmeans.ECAExcede,
			Tamano:    r.Tamano,
		}
	}
	if est.CentroidesEnUnidades() {
		m.CentroidesOriginales = make([][]float64, k)
		for j := range k {
			m.CentroidesOriginales[j] = make([]float64, d)
			if err := est.Invertir(m.centroides[j*d:(j+1)*d], "", m.CentroidesOriginales[j]); err != nil {
				return nil, err
			}
		}
	}
	return m, nil
}
