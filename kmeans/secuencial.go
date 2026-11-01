package kmeans

import (
	"fmt"
	"math"
)

// Motivos por los que para el algoritmo.
const (
	ParoTolerancia = "tolerancia"
	ParoMaxIter    = "max_iter"
)

// Opciones son los parámetros del contrato numérico (ver el comentario del paquete).
// TolAbs y TolRel definen el criterio de parada:
//
//	max_j ||mu_j(t) - mu_j(t-1)||inf  <=  TolAbs + TolRel * max_j ||mu_j(t-1)||inf
type Opciones struct {
	K       int
	MaxIter int
	TolAbs  float64
	TolRel  float64
	// AlIterar, si no es nil, se llama al terminar cada iteración con su número
	// (desde 1) y la inercia de esa iteración. Es para que quien llama muestre
	// progreso; el paquete no sabe nada de terminales. Corre en la goroutine que
	// llamó a Secuencial o Concurrente, nunca en un worker.
	AlIterar func(it int, inercia float64)
}

// Resultado es lo que produce una corrida. Asignaciones e Inercia corresponden
// a Centroides: se recalculan al final con los centroides ya actualizados, para
// que el número publicado describa el modelo publicado.
type Resultado struct {
	Centroides   []float64 // K*D, por filas
	Asignaciones []int     // N
	Inercia      float64   // con los centroides finales
	Inercias     []float64 // una por iteración, con los centroides que produjeron esa asignación
	Iteraciones  int
	Paro         string
	Vacios       int // clusters sin puntos en la última iteración
	K            int
	D            int
}

// Centroide devuelve las D coordenadas del centroide j.
func (r *Resultado) Centroide(j int) []float64 {
	return r.Centroides[j*r.D : (j+1)*r.D]
}

// Secuencial corre Lloyd en un solo hilo. Es el T_secuencial del speedup: la
// referencia es el mismo algoritmo sin goroutines, no una versión degradada.
//
// No modifica ni d ni centroidesIniciales.
func Secuencial(d *Datos, centroidesIniciales []float64, op Opciones) (*Resultado, error) {
	if err := validar(d, centroidesIniciales, op); err != nil {
		return nil, err
	}
	k, dim, n := op.K, d.D, d.N

	cent := append([]float64(nil), centroidesIniciales...)
	previos := make([]float64, len(cent))
	asign := make([]int, n)
	sumas := make([]float64, k*dim)
	conteos := make([]int, k)

	r := &Resultado{K: k, D: dim, Paro: ParoMaxIter}

	for it := 1; it <= op.MaxIter; it++ {
		for i := range sumas {
			sumas[i] = 0
		}
		for j := range conteos {
			conteos[j] = 0
		}
		inercia := 0.0

		// Asignación: los centroides son de solo lectura.
		for i := 0; i < n; i++ {
			x := d.X[i*dim : (i+1)*dim]
			j, dist2 := masCercano(x, cent, k, dim)
			asign[i] = j
			inercia += dist2
			acumular(sumas, conteos, x, j, dim)
		}

		copy(previos, cent)
		r.Vacios = actualizar(cent, sumas, conteos, k, dim)
		r.Inercias = append(r.Inercias, inercia)
		r.Iteraciones = it
		if op.AlIterar != nil {
			op.AlIterar(it, inercia)
		}

		if converge(cent, previos, op) {
			r.Paro = ParoTolerancia
			break
		}
	}

	// Pasada final: asignaciones e inercia contra los centroides finales.
	inerciaFinal := 0.0
	for i := 0; i < n; i++ {
		x := d.X[i*dim : (i+1)*dim]
		j, dist2 := masCercano(x, cent, k, dim)
		asign[i] = j
		inerciaFinal += dist2
	}

	r.Centroides = cent
	r.Asignaciones = asign
	r.Inercia = inerciaFinal
	return r, nil
}

// masCercano devuelve el índice del centroide más cercano y la distancia
// euclídea al cuadrado. El empate lo gana el índice menor.
func masCercano(x, cent []float64, k, dim int) (int, float64) {
	mejor, mejorDist := 0, math.Inf(1)
	for j := 0; j < k; j++ {
		c := cent[j*dim : (j+1)*dim]
		dist := 0.0
		for f := 0; f < dim; f++ {
			delta := x[f] - c[f]
			dist += delta * delta
		}
		if dist < mejorDist {
			mejor, mejorDist = j, dist
		}
	}
	return mejor, mejorDist
}

func acumular(sumas []float64, conteos []int, x []float64, j, dim int) {
	s := sumas[j*dim : (j+1)*dim]
	for f := 0; f < dim; f++ {
		s[f] += x[f]
	}
	conteos[j]++
}

// actualizar reemplaza cada centroide por la media de sus puntos y devuelve
// cuántos clusters quedaron vacíos. Un cluster vacío conserva su centroide
// anterior: re-sembrarlo con el punto más lejano haría que el resultado
// dependiera del orden de recorrido.
func actualizar(cent, sumas []float64, conteos []int, k, dim int) int {
	vacios := 0
	for j := 0; j < k; j++ {
		if conteos[j] == 0 {
			vacios++
			continue
		}
		c := cent[j*dim : (j+1)*dim]
		s := sumas[j*dim : (j+1)*dim]
		inv := 1 / float64(conteos[j])
		for f := 0; f < dim; f++ {
			c[f] = s[f] * inv
		}
	}
	return vacios
}

// converge aplica el criterio de parada sobre el desplazamiento de los centroides.
func converge(cent, previos []float64, op Opciones) bool {
	desp, escala := 0.0, 0.0
	for i := range cent {
		if delta := math.Abs(cent[i] - previos[i]); delta > desp {
			desp = delta
		}
		if mag := math.Abs(previos[i]); mag > escala {
			escala = mag
		}
	}
	return desp <= op.TolAbs+op.TolRel*escala
}

func validar(d *Datos, cent []float64, op Opciones) error {
	if d == nil || d.N == 0 {
		return fmt.Errorf("no hay datos que agrupar")
	}
	if d.D <= 0 || len(d.X) != d.N*d.D {
		return fmt.Errorf("datos inconsistentes: N=%d, D=%d, len(X)=%d", d.N, d.D, len(d.X))
	}
	if op.K <= 0 {
		return fmt.Errorf("k debe ser positivo, es %d", op.K)
	}
	if op.K > d.N {
		return fmt.Errorf("k=%d es mayor que la cantidad de filas (%d)", op.K, d.N)
	}
	if op.MaxIter <= 0 {
		return fmt.Errorf("max_iter debe ser positivo, es %d", op.MaxIter)
	}
	if len(cent) != op.K*d.D {
		return fmt.Errorf("centroides iniciales: %d valores, se esperaban k*D = %d", len(cent), op.K*d.D)
	}
	for i, v := range cent {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return fmt.Errorf("centroides iniciales: valor no finito en la posición %d (%v)", i, v)
		}
	}
	return nil
}
