package kmeans

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"slices"
)

// Estandarizacion es reports/estandarizacion.json, que escribe el cargador:
// con qué media y desviación se llevó cada feature a z-score. Media y Desv están
// en la escala YA transformada (después de log1p), así que invertir es
//
//	x = z*desv + media          y, si la feature va con log1p,   x = expm1(x).
//
// Con nivel = estacion cada estación tiene su media y su desviación
// (PorEstacion); Media y Desv son entonces las del total y no sirven para
// invertir una fila.
type Estandarizacion struct {
	Features    []string                       `json:"features"`
	Log1p       []string                       `json:"log1p"`
	Nivel       string                         `json:"nivel"`
	N           int                            `json:"n"`
	Media       []float64                      `json:"media"`
	Desv        []float64                      `json:"desv"`
	PorEstacion map[string]EstadisticaEstacion `json:"por_estacion,omitempty"`
}

// EstadisticaEstacion es la media y la desviación de una estación.
type EstadisticaEstacion struct {
	N     int       `json:"n"`
	Media []float64 `json:"media"`
	Desv  []float64 `json:"desv"`
}

// LeerEstandarizacion lee y valida reports/estandarizacion.json. Acepta campos
// que no conoce: el archivo lo escribe otra parte del sistema y puede llevar
// más información (método, conteos) que aquí no hace falta.
func LeerEstandarizacion(ruta string) (*Estandarizacion, error) {
	b, err := os.ReadFile(ruta)
	if err != nil {
		return nil, err
	}
	var e Estandarizacion
	if err := json.Unmarshal(b, &e); err != nil {
		return nil, fmt.Errorf("%s: %w", ruta, err)
	}
	if err := e.Validar(); err != nil {
		return nil, fmt.Errorf("%s: %w", ruta, err)
	}
	return &e, nil
}

// Validar comprueba la forma del archivo y que las desviaciones sean
// positivas: con desviación cero el z-score no está definido.
func (e *Estandarizacion) Validar() error {
	d := len(e.Features)
	if d == 0 {
		return fmt.Errorf("estandarización sin features")
	}
	for _, l := range e.Log1p {
		if !slices.Contains(e.Features, l) {
			return fmt.Errorf("estandarización: log1p sobre %q, que no es una de las features", l)
		}
	}
	revisar := func(quien string, media, desv []float64) error {
		if len(media) != d || len(desv) != d {
			return fmt.Errorf("estandarización %s: %d medias y %d desviaciones para %d features", quien, len(media), len(desv), d)
		}
		for f := 0; f < d; f++ {
			if math.IsNaN(media[f]) || math.IsInf(media[f], 0) {
				return fmt.Errorf("estandarización %s: media de %s no finita", quien, e.Features[f])
			}
			if !(desv[f] > 0) || math.IsInf(desv[f], 0) {
				return fmt.Errorf("estandarización %s: desviación de %s inválida (%v)", quien, e.Features[f], desv[f])
			}
		}
		return nil
	}
	switch e.Nivel {
	case NivelAbsoluto:
		if err := revisar("global", e.Media, e.Desv); err != nil {
			return err
		}
	case NivelEstacion:
		if len(e.PorEstacion) == 0 {
			return fmt.Errorf("estandarización por estación sin por_estacion")
		}
		if len(e.Media) > 0 || len(e.Desv) > 0 {
			if err := revisar("global", e.Media, e.Desv); err != nil {
				return err
			}
		}
		for est, s := range e.PorEstacion {
			if err := revisar("de "+est, s.Media, s.Desv); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("estandarización: nivel %q, se esperaba %q o %q", e.Nivel, NivelAbsoluto, NivelEstacion)
	}
	return nil
}

// Coincide comprueba que el gold se haya estandarizado con esta receta: las
// mismas features en el mismo orden, el mismo log1p y el mismo nivel. Si no, los
// centroides des-estandarizados saldrían en unidades equivocadas.
func (e *Estandarizacion) Coincide(r Receta) error {
	if !slices.Equal(e.Features, r.Features) {
		return fmt.Errorf("la estandarización es de las features %v y la receta pide %v", e.Features, r.Features)
	}
	a, b := slices.Clone(e.Log1p), slices.Clone(r.Log1p)
	slices.Sort(a)
	slices.Sort(b)
	if !slices.Equal(a, b) {
		return fmt.Errorf("la estandarización aplicó log1p a %v y la receta pide %v", e.Log1p, r.Log1p)
	}
	if e.Nivel != r.Nivel {
		return fmt.Errorf("la estandarización es de nivel %q y la receta pide %q", e.Nivel, r.Nivel)
	}
	return nil
}

// CentroidesEnUnidades dice si un centroide se puede llevar a unidades
// originales. Con nivel = estacion no: un centroide promedia filas de
// estaciones con medias distintas, y «1,5 desviaciones sobre lo normal de su
// estación» no es un número de µg/m³.
func (e *Estandarizacion) CentroidesEnUnidades() bool {
	return e.Nivel == NivelAbsoluto
}

// parametros devuelve la media y la desviación con que se estandarizó una fila
// de la estación dada. Con nivel = absoluto la estación no importa.
func (e *Estandarizacion) parametros(estacion string) ([]float64, []float64, error) {
	if e.Nivel == NivelAbsoluto {
		return e.Media, e.Desv, nil
	}
	if estacion == "" {
		return nil, nil, fmt.Errorf("con nivel = estacion hace falta la estación para des-estandarizar")
	}
	s, ok := e.PorEstacion[estacion]
	if !ok {
		return nil, nil, fmt.Errorf("la estación %s no tiene estadísticas en por_estacion", estacion)
	}
	return s.Media, s.Desv, nil
}

// Invertir lleva un vector estandarizado z (D valores) a unidades originales y
// lo escribe en dst. estacion se ignora con nivel = absoluto.
func (e *Estandarizacion) Invertir(z []float64, estacion string, dst []float64) error {
	media, desv, err := e.parametros(estacion)
	if err != nil {
		return err
	}
	for f := range e.Features {
		x := float64(z[f]*desv[f]) + media[f] // sin FMA: ver noFusionar en secuencial.go
		if slices.Contains(e.Log1p, e.Features[f]) {
			x = math.Expm1(x)
		}
		dst[f] = x
	}
	return nil
}

// Estandarizar es la inversa de Invertir: log1p si corresponde y z-score. En
// el sistema lo hace el cargador; aquí sirve para los datos sintéticos y para
// comprobar que Invertir lo deshace.
func (e *Estandarizacion) Estandarizar(x []float64, estacion string, dst []float64) error {
	media, desv, err := e.parametros(estacion)
	if err != nil {
		return err
	}
	for f := range e.Features {
		t := x[f]
		if slices.Contains(e.Log1p, e.Features[f]) {
			t = math.Log1p(t)
		}
		dst[f] = (t - media[f]) / desv[f]
	}
	return nil
}
