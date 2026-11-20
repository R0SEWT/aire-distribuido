package api

import (
	"context"

	"github.com/R0SEWT/aire-distribuido/almacen"
	"github.com/R0SEWT/aire-distribuido/modelo"
)

// almacenesDeModelo es dónde se guarda el modelo activo. Cualquiera de los dos
// puede ser nil.
type almacenesDeModelo struct {
	mongo almacen.Modelos
	redis almacen.CacheModelo
}

// guardarModeloActivo deja el modelo recién activado como activo en MongoDB
// (colección modelos) y en Redis (clave modelo:activo), para que la API vuelva
// a arrancar con él (spec §6.2 y §7). Si un almacén falla queda en el log: el
// modelo ya está activo en memoria y las predicciones no dependen de esto.
func (s *Servidor) guardarModeloActivo(m *modelo.Modelo) {
	if m == nil || (s.modelos.mongo == nil && s.modelos.redis == nil) {
		return
	}
	doc := m.Documento()
	doc.Activo = true
	if s.modelos.redis != nil {
		ctx, cancel := context.WithTimeout(context.Background(), plazoEscritura)
		err := s.modelos.redis.GuardarModeloActivo(ctx, doc)
		cancel()
		if err != nil {
			s.log.Error("no se guardó el modelo activo en Redis", "id", m.ID, "err", err)
		}
	}
	if s.modelos.mongo != nil {
		ctx, cancel := context.WithTimeout(context.Background(), plazoEscritura)
		err := s.modelos.mongo.Guardar(ctx, doc)
		cancel()
		if err != nil {
			s.log.Error("no se guardó el modelo activo en MongoDB", "id", m.ID, "err", err)
		}
	}
}
