package kmeans

import "fmt"

// MasCercano devuelve el índice del centroide más cercano a un punto ya
// estandarizado y su distancia euclídea al cuadrado, con el mismo contrato que
// Lloyd (Parciales y Asignar): el empate lo gana el índice menor y los
// productos no se fusionan en FMA. Es lo que usa la API para asignar una
// lectura al régimen del modelo activo, de modo que una lectura del gold cae en
// el mismo régimen en que la dejó el entrenamiento.
//
// centroides son k*D valores por filas, con D = len(x). Un punto vacío o unos
// centroides que no son múltiplo de D son un error de programación y entran en
// pánico, como en Parciales.
func MasCercano(x, centroides []float64) (int, float64) {
	dim := len(x)
	if dim == 0 || len(centroides) == 0 || len(centroides)%dim != 0 {
		panic(fmt.Sprintf("kmeans.MasCercano: %d centroides no son k*D con D = %d", len(centroides), dim))
	}
	return masCercano(x, centroides, len(centroides)/dim, dim)
}
