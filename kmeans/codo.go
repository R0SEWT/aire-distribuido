package kmeans

import "fmt"

// PuntoCodo es la inercia de una corrida con k clusters sobre la submuestra.
type PuntoCodo struct {
	K           int     `json:"k"`
	KEfectivo   int     `json:"k_efectivo"`
	N           int     `json:"n"`
	Inercia     float64 `json:"inercia"`
	Iteraciones int     `json:"iteraciones"`
	Paro        string  `json:"paro"`
}

// Codo corre K-Means (k-means++ con la semilla dada y el Lloyd concurrente)
// para cada k en [kMin, kMax] sobre una submuestra sistemática de n filas
// (n <= 0 o n >= N usa todas) y devuelve la inercia de cada k. op.K se ignora.
//
// La submuestra es sistemática, nunca las primeras n filas: el gold va ordenado
// por estación y fecha-hora, y las horas consecutivas de una estación están
// autocorrelacionadas. La silueta no se calcula aquí (es O(n²) y va en la PC4).
func Codo(d *Datos, kMin, kMax, n int, semilla int64, op OpcionesConc) ([]PuntoCodo, error) {
	if kMin <= 0 || kMax < kMin {
		return nil, fmt.Errorf("codo: rango de k inválido [%d, %d]", kMin, kMax)
	}
	sub := d
	if n > 0 && n < d.N {
		var err error
		if sub, err = Submuestra(d, n); err != nil {
			return nil, err
		}
	}
	if kMax > sub.N {
		return nil, fmt.Errorf("codo: k = %d es mayor que la submuestra (%d filas)", kMax, sub.N)
	}

	var puntos []PuntoCodo
	for k := kMin; k <= kMax; k++ {
		cent, kEf, err := KMeansPP(sub, k, semilla)
		if err != nil {
			return nil, fmt.Errorf("codo, k = %d: %w", k, err)
		}
		opk := op
		opk.K = kEf
		r, err := Concurrente(sub, cent, opk)
		if err != nil {
			return nil, fmt.Errorf("codo, k = %d: %w", k, err)
		}
		puntos = append(puntos, PuntoCodo{K: k, KEfectivo: kEf, N: sub.N,
			Inercia: r.Inercia, Iteraciones: r.Iteraciones, Paro: r.Paro})
	}
	return puntos, nil
}
