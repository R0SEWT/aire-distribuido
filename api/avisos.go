package api

import (
	"context"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/R0SEWT/aire-distribuido/almacen"
)

// cadaAvisoPorOmision es cada cuánto, a lo sumo, sale por el WebSocket el
// estado de una misma estación. Para quien mira el panel, cuatro veces por
// segundo es tiempo real; para el hub, es el tope que hace que una ráfaga no
// llene el búfer de nadie: con las 33 estaciones del catálogo cambiando sin
// parar son 132 eventos por segundo por cliente, y sin el tope cmd/carga
// emitiría uno por predicción, miles por segundo.
const cadaAvisoPorOmision = 250 * time.Millisecond

// avisosRegimen convierte los cambios de estado de las estaciones en eventos
// estacion.regimen del WebSocket, agrupados por estación (spec del frontend,
// §6): de una misma estación sale a lo sumo un evento por intervalo.
//
// Lo alimenta el escritor, con lo que la caché dice que quedó guardado
// (anotar): un estado que no reemplazó al guardado nunca llega aquí, así que
// el evento y GET /api/regimenes dicen lo mismo.
//
// Un cambio en una estación que no avisó en el último intervalo sale
// enseguida: una lectura suelta se ve en el panel sin espera. Si la estación
// vuelve a cambiar antes de que pase el intervalo, el estado queda pendiente
// y cada cambio posterior pisa al anterior; al cumplirse el intervalo sale
// solo el último. El evento es el estado de la estación, no cada lectura: los
// intermedios no hacen falta.
//
// anotar nunca bloquea ni depende de que la goroutine esté corriendo: guarda
// en un mapa con candado, que no crece más allá de una entrada por estación,
// y deja una señal en un canal de un solo lugar.
type avisosRegimen struct {
	cada     time.Duration
	difundir func(estadoEstacion)
	hay      chan struct{} // hay pendientes nuevos; con búfer de 1

	mu         sync.Mutex
	pendientes map[string]almacen.RegimenEstacion

	// Lo que entró y lo que pasó con ello: recibidos = emitidos + agrupados +
	// los que siguen pendientes.
	recibidos atomic.Uint64 // estados que la caché guardó
	emitidos  atomic.Uint64 // eventos difundidos
	agrupados atomic.Uint64 // estados que otro posterior de su estación pisó antes de salir
}

func nuevosAvisosRegimen(cada time.Duration, difundir func(estadoEstacion)) *avisosRegimen {
	if cada <= 0 {
		cada = cadaAvisoPorOmision
	}
	return &avisosRegimen{cada: cada, difundir: difundir, hay: make(chan struct{}, 1),
		pendientes: map[string]almacen.RegimenEstacion{}}
}

// anotar recibe los regímenes que quedaron guardados en la caché, por estación.
func (a *avisosRegimen) anotar(guardados map[string]almacen.RegimenEstacion) {
	if len(guardados) == 0 {
		return
	}
	a.mu.Lock()
	for estacion, r := range guardados {
		if _, ok := a.pendientes[estacion]; ok {
			a.agrupados.Add(1)
		}
		a.pendientes[estacion] = r
	}
	a.mu.Unlock()
	a.recibidos.Add(uint64(len(guardados)))
	select {
	case a.hay <- struct{}{}:
	default: // ya hay una señal: la goroutine va a mirar el mapa entero
	}
}

// alDia dice si todo lo anotado ya salió (o lo pisó un estado posterior).
func (a *avisosRegimen) alDia() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.pendientes) == 0 && a.recibidos.Load() == a.emitidos.Load()+a.agrupados.Load()
}

// correr emite los avisos hasta que ctx se cancela; antes de volver emite lo
// que quedó pendiente, sin esperar su intervalo, para que el último estado de
// cada estación salga antes de que el hub despida a los clientes.
func (a *avisosRegimen) correr(ctx context.Context) {
	ultimo := map[string]time.Time{} // cuándo salió el último aviso de cada estación
	reloj := time.NewTimer(time.Hour)
	reloj.Stop()
	defer reloj.Stop()
	for {
		select {
		case <-ctx.Done():
			a.emitir(func(string) bool { return true })
			return
		case <-a.hay:
		case <-reloj.C:
		}
		ahora := time.Now()
		var proximo time.Time // el vencimiento más cercano entre los que siguen pendientes
		a.emitir(func(estacion string) bool {
			vence := ultimo[estacion].Add(a.cada)
			if !vence.After(ahora) {
				ultimo[estacion] = ahora
				return true
			}
			if proximo.IsZero() || vence.Before(proximo) {
				proximo = vence
			}
			return false
		})
		if !proximo.IsZero() {
			reloj.Reset(time.Until(proximo))
		}
	}
}

// emitir saca del mapa los pendientes de las estaciones que toca dice que ya
// pueden avisar y los difunde, en orden de estación.
func (a *avisosRegimen) emitir(toca func(estacion string) bool) {
	var listos []estadoEstacion
	a.mu.Lock()
	for estacion, r := range a.pendientes {
		if toca(estacion) {
			listos = append(listos, nuevoEstadoEstacion(estacion, r))
			delete(a.pendientes, estacion)
		}
	}
	a.mu.Unlock()
	slices.SortFunc(listos, func(x, y estadoEstacion) int { return strings.Compare(x.Estacion, y.Estacion) })
	for _, estado := range listos {
		a.difundir(estado)
		a.emitidos.Add(1)
	}
}
