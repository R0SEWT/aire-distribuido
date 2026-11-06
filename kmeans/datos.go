package kmeans

// Datos es la matriz que ve el modelo: X guarda los puntos uno tras otro, por
// filas (el punto i ocupa X[i*D : (i+1)*D]). Un arreglo plano en vez de
// [][]float64 evita N indirecciones y deja las features de una lectura
// contiguas en caché.
//
// Invariante: todos los valores de X son finitos. LeerGold lo garantiza al
// cargar (rechaza NaN e Inf) y Submuestra lo conserva. Secuencial y Concurrente
// NO lo vuelven a comprobar, a propósito: recorrer los N*D valores es una pasada
// serial completa sobre los datos y caería dentro del tiempo que mide el
// benchmark, sesgando el speedup. Quien construya Datos a mano es responsable
// de cumplirlo.
type Datos struct {
	X []float64
	N int
	D int
}

// Punto devuelve las features de la fila i. Comparte memoria con X: no se modifica.
func (d *Datos) Punto(i int) []float64 {
	return d.X[i*d.D : (i+1)*d.D]
}
