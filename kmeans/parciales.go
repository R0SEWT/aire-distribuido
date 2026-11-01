package kmeans

import (
	"fmt"
	"math"
)

// Parcial son los acumuladores de un tramo de filas: sumas y conteos por
// cluster, más la inercia del tramo medida contra los centroides con los que se
// asignó. Es lo que un worker calcula sobre un chunk en cada iteración, y lo que
// un nodo de la PC4 calculará sobre su partición: viaja por la red tal cual,
// por eso sus campos se exportan con nombre JSON.
type Parcial struct {
	Sumas   []float64 `json:"sumas"`   // k*D, por filas
	Conteos []int     `json:"conteos"` // k
	Inercia float64   `json:"inercia"`
}

// Parciales asigna las filas [desde, hasta) de d a los centroides y devuelve
// sumas k*D, conteos k e inercia. Es lo que un nodo calcula sobre su partición
// en cada iteración. Los centroides son de solo lectura: varias goroutines
// pueden llamar a Parciales a la vez sobre tramos distintos.
//
// k sale de len(centroides)/d.D. Un tramo fuera de [0, N] o centroides que no
// son múltiplo de D son un error de programación y entran en pánico.
func Parciales(d *Datos, centroides []float64, desde, hasta int) Parcial {
	return tramo(d, centroides, desde, hasta, nil)
}

// Asignar hace lo mismo que Parciales y además escribe en asign[i] el cluster
// de cada fila i del tramo; no toca asign fuera de [desde, hasta). Es la pasada
// final, la que produce las asignaciones y la inercia contra los centroides
// finales. Tramos disjuntos escriben posiciones disjuntas de asign, así que
// varias goroutines pueden compartir el mismo arreglo.
func Asignar(d *Datos, centroides []float64, desde, hasta int, asign []int) Parcial {
	if len(asign) < hasta {
		panic(fmt.Sprintf("kmeans.Asignar: asign tiene %d posiciones y el tramo llega a %d", len(asign), hasta))
	}
	return tramo(d, centroides, desde, hasta, asign)
}

func tramo(d *Datos, cent []float64, desde, hasta int, asign []int) Parcial {
	if d == nil || d.D <= 0 || len(cent) == 0 || len(cent)%d.D != 0 {
		panic(fmt.Sprintf("kmeans.Parciales: %d centroides no son k*D", len(cent)))
	}
	if desde < 0 || hasta < desde || hasta > d.N {
		panic(fmt.Sprintf("kmeans.Parciales: tramo [%d, %d) fuera de [0, %d]", desde, hasta, d.N))
	}
	dim := d.D
	k := len(cent) / dim
	p := nuevoParcial(k, dim)
	inercia := 0.0
	for i := desde; i < hasta; i++ {
		x := d.X[i*dim : (i+1)*dim]
		j, dist2 := masCercano(x, cent, k, dim)
		if asign != nil {
			asign[i] = j
		}
		inercia += dist2
		acumular(p.Sumas, p.Conteos, x, j, dim)
	}
	p.Inercia = inercia
	return p
}

func nuevoParcial(k, dim int) Parcial {
	return Parcial{Sumas: make([]float64, k*dim), Conteos: make([]int, k)}
}

// Fusionar suma parciales en el orden dado. Reducción exacta: el orden fija el
// resultado bit a bit. Empieza de cero y suma uno tras otro, sin reordenar ni
// sumar por pares: la suma en punto flotante no es asociativa, y quien llama
// elige el orden (de chunk en una máquina, de nodo en el clúster) para que el
// resultado no dependa del planificador ni de qué nodo respondió primero.
//
// Un parcial que no tiene k*dim sumas y k conteos es un error de programación y
// entra en pánico; lo que llega por la red se valida antes con Parcial.Validar.
func Fusionar(ps []Parcial, k, dim int) Parcial {
	f := nuevoParcial(k, dim)
	inercia := 0.0
	for c := range ps {
		p := &ps[c]
		if len(p.Sumas) != k*dim || len(p.Conteos) != k {
			panic(fmt.Sprintf("kmeans.Fusionar: el parcial %d tiene %d sumas y %d conteos, se esperaban %d y %d",
				c, len(p.Sumas), len(p.Conteos), k*dim, k))
		}
		for i := range f.Sumas {
			f.Sumas[i] += p.Sumas[i]
		}
		for j := range f.Conteos {
			f.Conteos[j] += p.Conteos[j]
		}
		inercia += p.Inercia
	}
	f.Inercia = inercia
	return f
}

// Validar comprueba que el parcial tenga la forma de k clusters de dim
// features y valores que tengan sentido. Es para lo que llega de otro proceso:
// un parcial corrupto fusionado en silencio arruina los centroides de todos.
func (p Parcial) Validar(k, dim int) error {
	if len(p.Sumas) != k*dim {
		return fmt.Errorf("parcial con %d sumas, se esperaban k*D = %d", len(p.Sumas), k*dim)
	}
	if len(p.Conteos) != k {
		return fmt.Errorf("parcial con %d conteos, se esperaban k = %d", len(p.Conteos), k)
	}
	for j, c := range p.Conteos {
		if c < 0 {
			return fmt.Errorf("parcial con conteo negativo en el cluster %d (%d)", j, c)
		}
	}
	for i, s := range p.Sumas {
		if math.IsNaN(s) || math.IsInf(s, 0) {
			return fmt.Errorf("parcial con suma no finita en la posición %d (%v)", i, s)
		}
	}
	if math.IsNaN(p.Inercia) || math.IsInf(p.Inercia, 0) || p.Inercia < 0 {
		return fmt.Errorf("parcial con inercia inválida (%v)", p.Inercia)
	}
	return nil
}

// Vacios cuenta los clusters que no recibieron ninguna fila.
func (p Parcial) Vacios() int {
	vacios := 0
	for _, c := range p.Conteos {
		if c == 0 {
			vacios++
		}
	}
	return vacios
}

// Actualizar recalcula los centroides a partir de un parcial fusionado y dice
// cuánto se movieron: el mayor desplazamiento de una coordenada,
// max_j ||mu_j(t) - mu_j(t-1)||inf, que es la norma del criterio de parada del
// TP. Un cluster vacío conserva su centroide anterior: re-sembrarlo con el punto
// más lejano haría que el resultado dependiera del orden de recorrido.
//
// Cada centroide se calcula como suma * (1/conteo), igual que en el TP: dividir
// en vez de multiplicar por el inverso cambiaría el último bit.
func Actualizar(cent []float64, p Parcial) (movimiento float64) {
	k := len(p.Conteos)
	if k == 0 || len(cent)%k != 0 || len(p.Sumas) != len(cent) {
		panic(fmt.Sprintf("kmeans.Actualizar: %d centroides, %d sumas y %d conteos no cuadran",
			len(cent), len(p.Sumas), k))
	}
	dim := len(cent) / k
	for j := 0; j < k; j++ {
		if p.Conteos[j] == 0 {
			continue
		}
		c := cent[j*dim : (j+1)*dim]
		s := p.Sumas[j*dim : (j+1)*dim]
		inv := 1 / float64(p.Conteos[j])
		for f := 0; f < dim; f++ {
			nuevo := s[f] * inv
			if delta := math.Abs(nuevo - c[f]); delta > movimiento {
				movimiento = delta
			}
			c[f] = nuevo
		}
	}
	return movimiento
}
