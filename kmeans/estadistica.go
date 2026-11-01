package kmeans

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
)

// MediaRecortada ordena la muestra, descarta una proporción de cada extremo y
// promedia el resto. Es el estimador que pide la rúbrica: una corrida que se va
// a las nubes porque el sistema hizo otra cosa contamina la media, pero no la
// media recortada.
//
// prop es por extremo: 0.10 sobre 20 observaciones descarta 2 abajo y 2 arriba.
func MediaRecortada(xs []float64, prop float64) (float64, error) {
	if len(xs) == 0 {
		return 0, fmt.Errorf("muestra vacía")
	}
	if prop < 0 || prop >= 0.5 {
		return 0, fmt.Errorf("la proporción de recorte debe estar en [0, 0.5), es %v", prop)
	}
	orden := append([]float64(nil), xs...)
	sort.Float64s(orden)

	// floor: con prop < 0.5 el recorte nunca puede vaciar la muestra, porque
	// floor(n*prop) < n/2. Con pocas observaciones puede dar 0 y no recortar nada.
	recorte := int(math.Floor(float64(len(orden)) * prop))
	orden = orden[recorte : len(orden)-recorte]

	suma := 0.0
	for _, x := range orden {
		suma += x
	}
	return suma / float64(len(orden)), nil
}

// Mediana devuelve la mediana de la muestra sin alterarla.
func Mediana(xs []float64) (float64, error) {
	if len(xs) == 0 {
		return 0, fmt.Errorf("muestra vacía")
	}
	orden := append([]float64(nil), xs...)
	sort.Float64s(orden)
	mitad := len(orden) / 2
	if len(orden)%2 == 1 {
		return orden[mitad], nil
	}
	return (orden[mitad-1] + orden[mitad]) / 2, nil
}

// DesviacionEstandar es la muestral, con n-1 en el denominador.
func DesviacionEstandar(xs []float64) (float64, error) {
	if len(xs) < 2 {
		return 0, fmt.Errorf("hacen falta al menos 2 observaciones, hay %d", len(xs))
	}
	media := 0.0
	for _, x := range xs {
		media += x
	}
	media /= float64(len(xs))

	suma := 0.0
	for _, x := range xs {
		d := x - media
		suma += d * d
	}
	return math.Sqrt(suma / float64(len(xs)-1)), nil
}

// IntervaloCociente es el speedup puntual y su intervalo de confianza.
type IntervaloCociente struct {
	Punto    float64 `json:"punto"`
	Inferior float64 `json:"inferior"`
	Superior float64 `json:"superior"`
	Nivel    float64 `json:"nivel"`
}

// ICBootstrapCociente estima la incertidumbre del speedup.
//
// El speedup es un cociente de dos medias recortadas, así que no tiene una
// fórmula de error simple: se remuestrean con reemplazo las dos muestras, se
// recalcula el cociente en cada réplica y se toman los percentiles 2.5 y 97.5.
// La semilla se fija para que el número del informe se pueda reproducir.
func ICBootstrapCociente(sec, con []float64, prop float64, repeticiones int, semilla int64) (IntervaloCociente, error) {
	if len(sec) == 0 || len(con) == 0 {
		return IntervaloCociente{}, fmt.Errorf("hacen falta las dos muestras (secuencial %d, concurrente %d)", len(sec), len(con))
	}
	if repeticiones <= 0 {
		return IntervaloCociente{}, fmt.Errorf("repeticiones debe ser positivo, es %d", repeticiones)
	}
	punto, err := cociente(sec, con, prop)
	if err != nil {
		return IntervaloCociente{}, err
	}

	rng := rand.New(rand.NewSource(semilla))
	replicas := make([]float64, 0, repeticiones)
	muestraSec := make([]float64, len(sec))
	muestraCon := make([]float64, len(con))
	for r := 0; r < repeticiones; r++ {
		for i := range muestraSec {
			muestraSec[i] = sec[rng.Intn(len(sec))]
		}
		for i := range muestraCon {
			muestraCon[i] = con[rng.Intn(len(con))]
		}
		c, err := cociente(muestraSec, muestraCon, prop)
		if err != nil {
			continue // una réplica degenerada no invalida el resto
		}
		replicas = append(replicas, c)
	}
	if len(replicas) == 0 {
		return IntervaloCociente{}, fmt.Errorf("ninguna réplica del bootstrap fue válida")
	}
	sort.Float64s(replicas)

	return IntervaloCociente{
		Punto:    punto,
		Inferior: percentil(replicas, 0.025),
		Superior: percentil(replicas, 0.975),
		Nivel:    0.95,
	}, nil
}

func cociente(sec, con []float64, prop float64) (float64, error) {
	a, err := MediaRecortada(sec, prop)
	if err != nil {
		return 0, err
	}
	b, err := MediaRecortada(con, prop)
	if err != nil {
		return 0, err
	}
	if b == 0 {
		return 0, fmt.Errorf("el tiempo concurrente recortado es cero")
	}
	return a / b, nil
}

// percentil sobre una muestra ya ordenada, por el método del más cercano.
func percentil(ordenada []float64, p float64) float64 {
	if len(ordenada) == 1 {
		return ordenada[0]
	}
	idx := int(math.Round(p * float64(len(ordenada)-1)))
	if idx < 0 {
		idx = 0
	}
	if idx >= len(ordenada) {
		idx = len(ordenada) - 1
	}
	return ordenada[idx]
}
