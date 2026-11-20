//go:build !unix

package kmeans

import "slices"

// medirSistema, fuera de Unix (Windows en los PC del laboratorio): la
// biblioteca estándar no da el RSS máximo ni el tiempo de CPU del proceso con
// una llamada portable, así que quedan en 0 y se nombran en NoDisponible. La
// memoria del runtime de Go (heap, total asignado, reservado, GC) sí se mide, y
// los tiempos de pared los toma cada comando por su cuenta.
func medirSistema(r *Recursos) {
	r.NoDisponible = slices.Clone(camposDelSistema)
}
