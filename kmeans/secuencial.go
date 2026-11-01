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
	Movimiento   float64 // desplazamiento máximo de los centroides en la última iteración
	Vacios       int     // clusters sin puntos en la última iteración
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
// Está escrito sobre las mismas tres piezas que la versión concurrente y la
// distribuida: cada iteración es Parciales sobre un solo tramo, [0, N), y
// Actualizar. No hay Fusionar porque hay un solo parcial.
//
// No modifica ni d ni centroidesIniciales.
func Secuencial(d *Datos, centroidesIniciales []float64, op Opciones) (*Resultado, error) {
	if err := validar(d, centroidesIniciales, op); err != nil {
		return nil, err
	}
	cent := append([]float64(nil), centroidesIniciales...)
	r := &Resultado{K: op.K, D: d.D, Paro: ParoMaxIter}

	for it := 1; it <= op.MaxIter; it++ {
		// Asignación: los centroides son de solo lectura.
		if cerrarIteracion(r, cent, Parciales(d, cent, 0, d.N), it, op) {
			break
		}
	}

	// Pasada final: asignaciones e inercia contra los centroides finales.
	asign := make([]int, d.N)
	r.Inercia = Asignar(d, cent, 0, d.N, asign).Inercia
	r.Centroides = cent
	r.Asignaciones = asign
	return r, nil
}

// cerrarIteracion es lo que el coordinador hace al final de cada iteración,
// igual en la versión secuencial y en la concurrente: actualiza los centroides
// con el parcial ya fusionado, registra la inercia y aplica el criterio de
// parada. Devuelve true si convergió.
//
// El umbral se calcula con los centroides ANTES de actualizarlos, como en el TP:
// max_j ||mu_j(t) - mu_j(t-1)||inf <= TolAbs + TolRel * max_j ||mu_j(t-1)||inf.
func cerrarIteracion(r *Resultado, cent []float64, p Parcial, it int, op Opciones) bool {
	umbral := op.TolAbs + float64(op.TolRel*maxAbs(cent))
	mov := Actualizar(cent, p)
	r.Vacios = p.Vacios()
	r.Movimiento = mov
	r.Inercias = append(r.Inercias, p.Inercia)
	r.Iteraciones = it
	if op.AlIterar != nil {
		op.AlIterar(it, p.Inercia)
	}
	if mov <= umbral {
		r.Paro = ParoTolerancia
		return true
	}
	return false
}

func maxAbs(xs []float64) float64 {
	m := 0.0
	for _, x := range xs {
		if a := math.Abs(x); a > m {
			m = a
		}
	}
	return m
}

// masCercano devuelve el índice del centroide más cercano y la distancia
// euclídea al cuadrado. El empate lo gana el índice menor.
//
// noFusionar: el compilador de Go fusiona a*b + c en una sola instrucción FMA,
// que redondea una vez en vez de dos y cambia el último bit. Lo hace en arm64
// (FMADDD; el Pixel es un nodo candidato de la PC4) y también en amd64 cuando
// se compila con GOAMD64=v3 (VFMADD231SD). Entonces dos nodos compilados
// distinto darían distancias, asignaciones y parciales distintos para los
// mismos datos. La especificación de Go garantiza que una conversión explícita
// float64(a*b) redondea el producto e impide la fusión; por eso todo producto
// que se suma en este paquete va envuelto así. Comprobado con
// go build -gcflags=-S: cero FMA en arm64 y en amd64 v3 (el TP sin este cambio
// tenía cinco en v3). Con el GOAMD64=v1 por omisión no había fusión: el
// resultado es el mismo bit a bit, como comprueban los tests contra el TP.
func masCercano(x, cent []float64, k, dim int) (int, float64) {
	mejor, mejorDist := 0, math.Inf(1)
	for j := 0; j < k; j++ {
		c := cent[j*dim : (j+1)*dim]
		dist := 0.0
		for f := 0; f < dim; f++ {
			delta := x[f] - c[f]
			dist += float64(delta * delta) // sin FMA: ver noFusionar
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
