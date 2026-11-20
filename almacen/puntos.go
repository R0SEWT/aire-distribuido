package almacen

import "context"

// PuntoControl es el valor de la clave entrenamiento:{id}:punto de Redis: los
// centroides estandarizados de la última ronda completa. Se escribe en cada
// ronda como registro; la PC4 no reanuda desde él (spec §5.2).
type PuntoControl struct {
	Iter       int         `json:"iter"`
	Centroides [][]float64 `json:"centroides"`
}

// PuntosControl son las claves entrenamiento:{id}:punto de Redis.
type PuntosControl interface {
	GuardarPunto(ctx context.Context, entrenamiento string, p PuntoControl) error
	// Punto devuelve el último punto de control de un entrenamiento o
	// ErrNoEncontrado.
	Punto(ctx context.Context, entrenamiento string) (PuntoControl, error)
}
