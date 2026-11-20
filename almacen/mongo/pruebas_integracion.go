//go:build integracion

package mongo

import "context"

// BorrarBase borra la base de datos entera. Solo existe con la etiqueta de
// compilación integracion: es para que las pruebas de otros paquetes limpien la
// base temporal que crearon, sin importar el driver. La API no la tiene.
func (a *Almacen) BorrarBase(ctx context.Context) error { return a.db.Drop(ctx) }
