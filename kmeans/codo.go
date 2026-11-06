package kmeans

import "fmt"

// PuntoCodo es la inercia de una corrida con k clusters sobre la submuestra.
type PuntoCodo struct {
	K           int     `json:"k"`
	KEfectivo   int     `json:"k_efectivo"`
	N           int     `json:"n"`
	Semilla     int64   `json:"semilla"` // la elegida entre las semillas probadas
	Inercia     float64 `json:"inercia"`
	Iteraciones int     `json:"iteraciones"`
	Paro        string  `json:"paro"`
}

// Codo corre K-Means para cada k en [kMin, kMax] sobre una submuestra
// sistemática de n filas (n <= 0 o n >= N usa todas) y devuelve la inercia de
// cada k. Usa el mismo procedimiento que el modelo final, MejorDeSemillas con
// semillas arranques a partir de semilla (paralelas = 0, automático): con una
// sola semilla un óptimo local puede hacer que la inercia suba con k y el codo
// se lea mal. op.K se ignora.
//
// La submuestra es sistemática, nunca las primeras n filas: el gold va ordenado
// por estación y fecha-hora, y las horas consecutivas de una estación están
// autocorrelacionadas. La silueta no se calcula aquí (es O(n²) y va en la PC4).
func Codo(d *Datos, kMin, kMax, n int, semilla int64, semillas int, op OpcionesConc) ([]PuntoCodo, error) {
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
		m, err := MejorDeSemillas(sub, k, semilla, semillas, op, 0)
		if err != nil {
			return nil, fmt.Errorf("codo, k = %d: %w", k, err)
		}
		a := m.Arranques[m.Elegido]
		puntos = append(puntos, PuntoCodo{K: k, KEfectivo: a.KEfectivo, N: sub.N, Semilla: a.Semilla,
			Inercia: a.Inercia, Iteraciones: a.Iteraciones, Paro: a.Paro})
	}
	return puntos, nil
}
