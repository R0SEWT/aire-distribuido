//go:build integracion

package redis

import "context"

// Vaciar borra todas las claves de la base de datos del cliente (FLUSHDB).
// Solo existe con la etiqueta de compilación integracion: es para que las
// pruebas dejen limpia la base que usaron. La API no la tiene.
func (c *Cache) Vaciar(ctx context.Context) error { return c.cliente.FlushDB(ctx).Err() }
