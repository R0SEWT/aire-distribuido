package api

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/R0SEWT/aire-distribuido/almacen"
)

// plazoEscritura es lo que puede tardar una escritura en el almacén antes de
// darla por fallida.
const plazoEscritura = 2 * time.Second

// escritor saca la persistencia del camino del pedido (spec de la PC4, §6.2):
// el handler encola la predicción en un canal con búfer y responde sin
// esperar; una sola goroutine la guarda en Predicciones (MongoDB) y actualiza
// en la Cache (Redis) lo último de la estación: régimen, media de 24 h, alerta
// e INCA. Si la cola está llena, la predicción no se guarda: se cuenta el
// descarte y el pedido responde igual.
type escritor struct {
	cola         chan almacen.Prediccion
	predicciones almacen.Predicciones
	cache        almacen.Cache
	log          *slog.Logger
	descartes    atomic.Uint64 // cola llena: lo que publica /api/metricas
	fallos       atomic.Uint64 // el almacén devolvió error
}

func nuevoEscritor(tam int, p almacen.Predicciones, c almacen.Cache, log *slog.Logger) *escritor {
	return &escritor{cola: make(chan almacen.Prediccion, tam), predicciones: p, cache: c, log: log}
}

// encolar nunca bloquea: devuelve false si la cola estaba llena.
func (e *escritor) encolar(p almacen.Prediccion) bool {
	select {
	case e.cola <- p:
		return true
	default:
		e.descartes.Add(1)
		return false
	}
}

// correr escribe lo que llega a la cola hasta que ctx se cancela; entonces
// escribe lo que quedó encolado y vuelve. El canal no se cierra nunca: un
// handler que todavía esté respondiendo no puede mandar a un canal cerrado.
func (e *escritor) correr(ctx context.Context) {
	for {
		select {
		case p := <-e.cola:
			e.escribir(p)
		case <-ctx.Done():
			for {
				select {
				case p := <-e.cola:
					e.escribir(p)
				default:
					return
				}
			}
		}
	}
}

func (e *escritor) escribir(p almacen.Prediccion) {
	ctx, cancel := context.WithTimeout(context.Background(), plazoEscritura)
	defer cancel()
	if err := e.predicciones.Guardar(ctx, p); err != nil {
		e.fallos.Add(1)
		e.log.Error("no se guardó la predicción", "estacion", p.Estacion, "err", err)
	}
	r := almacen.RegimenEstacion{
		Regimen: p.Regimen, ExcedeECA: p.ExcedeECA, Media24h: p.Media24h, Alerta: p.Alerta, INCA: p.INCA, TS: p.TS,
	}
	if err := e.cache.GuardarRegimen(ctx, p.Estacion, r); err != nil {
		e.fallos.Add(1)
		e.log.Error("no se actualizó el régimen de la estación", "estacion", p.Estacion, "err", err)
	}
}
