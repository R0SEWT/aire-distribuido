package kmeans

import "fmt"

// Submuestra devuelve n filas tomadas con paso fijo a lo largo de todo el
// dataset. Es determinista y sin aleatoriedad, para que el barrido de tamaños
// del benchmark se pueda repetir.
//
// Sistemática y no las primeras n filas: el gold está ordenado por estación y
// fecha-hora, así que el primer bloque sería la primera estación (y sus
// primeros meses) y no la red completa. Las horas consecutivas de una estación
// están fuertemente autocorrelacionadas: el paso fijo recorre todas.
func Submuestra(d *Datos, n int) (*Datos, error) {
	if d == nil || d.N == 0 {
		return nil, fmt.Errorf("no hay datos de los que tomar una submuestra")
	}
	if n <= 0 || n > d.N {
		return nil, fmt.Errorf("tamaño de submuestra inválido: %d de %d filas", n, d.N)
	}
	if n == d.N {
		return d, nil
	}

	s := &Datos{D: d.D, N: n, X: make([]float64, 0, n*d.D)}
	paso := float64(d.N) / float64(n)
	for i := 0; i < n; i++ {
		idx := int(float64(i) * paso)
		if idx >= d.N {
			idx = d.N - 1
		}
		s.X = append(s.X, d.Punto(idx)...)
	}
	return s, nil
}
