package api

import (
	"context"

	"github.com/R0SEWT/aire-distribuido/modelo"
)

// guardarModeloActivo deja el modelo recién activado como activo en MongoDB
// (colección modelos) y en Redis (clave modelo:activo), para que la API vuelva
// a arrancar con él (spec §6.2 y §7). Si un almacén falla queda en el log: el
// modelo ya está activo en memoria y las predicciones no dependen de esto.
func (s *Servidor) guardarModeloActivo(m *modelo.Modelo) {
	if m == nil || (s.modelos == nil && s.cacheModelo == nil) {
		return
	}
	doc := m.Documento()
	doc.Activo = true
	if s.cacheModelo != nil {
		ctx, cancel := context.WithTimeout(context.Background(), plazoEscritura)
		err := s.cacheModelo.GuardarModeloActivo(ctx, doc)
		cancel()
		if err != nil {
			s.log.Error("no se guardó el modelo activo en Redis", "id", m.ID, "err", err)
		}
	}
	if s.modelos != nil {
		ctx, cancel := context.WithTimeout(context.Background(), plazoEscritura)
		err := s.modelos.Guardar(ctx, doc)
		cancel()
		if err != nil {
			s.log.Error("no se guardó el modelo activo en MongoDB", "id", m.ID, "err", err)
		}
	}
}
