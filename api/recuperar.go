package api

import (
	"context"
	"time"

	"github.com/R0SEWT/aire-distribuido/almacen"
)

// errReinicio es el error con que queda un entrenamiento que estaba en curso
// cuando la API se cayó o se reinició.
const errReinicio = "la API se reinició mientras el entrenamiento estaba en curso"

// RecuperarEntrenamientos recupera del almacén, al arrancar, lo que el
// coordinador lleva solo en memoria y que si no se perdería con cada reinicio:
//
//   - cuál fue el último entrenamiento (el más reciente por inicio), que es lo
//     que /api/metricas informa en ultimo_entrenamiento;
//   - los entrenamientos que quedaron en curso: el proceso que los corría ya
//     no existe, así que se cierran como abortados, con el error que lo dice.
//     Si no, quedarían en_curso para siempre. Sus rondas guardadas no se tocan.
//
// Cada operación sobre el almacén tiene su propio plazo. Si el almacén no
// responde no se recupera nada, queda el aviso en el log y la API arranca
// igual. Se llama una vez, antes de atender pedidos; supone una sola API por
// base de datos (con dos, una cerraría los entrenamientos en curso de la otra).
func (s *Servidor) RecuperarEntrenamientos(plazo time.Duration) {
	c := s.coord
	conPlazo := func(op func(context.Context) error) error {
		ctx, cancel := context.WithTimeout(context.Background(), plazo)
		defer cancel()
		return op(ctx)
	}
	var lista []almacen.Entrenamiento
	if err := conPlazo(func(ctx context.Context) (err error) { lista, err = c.entrenamientos.Listar(ctx); return }); err != nil {
		s.log.Warn("no se pudo leer el historial de entrenamientos: ultimo_entrenamiento queda vacío hasta el próximo", "err", err)
		return
	}
	if len(lista) == 0 {
		return
	}
	c.mu.Lock()
	if c.ultimo == "" {
		c.ultimo = lista[0].ID // Listar va del más reciente al más antiguo
	}
	enCurso := c.enCurso
	c.mu.Unlock()
	for _, e := range lista {
		if e.Estado != almacen.EstadoEnCurso || e.ID == enCurso {
			continue
		}
		cierre := almacen.CierreEntrenamiento{Fin: enLima(s.ahora()).Truncate(time.Millisecond), Estado: almacen.EstadoAbortado, Error: errReinicio}
		if err := conPlazo(func(ctx context.Context) error { return c.entrenamientos.Cerrar(ctx, e.ID, cierre) }); err != nil {
			s.log.Warn("no se pudo cerrar un entrenamiento que quedó en curso", "id", e.ID, "err", err)
			continue
		}
		s.log.Warn("entrenamiento que quedó en curso al reiniciar: se cerró como abortado", "id", e.ID, "inicio", e.Inicio)
	}
}
