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
// El intervalo se cuenta desde que el aviso anterior de la estación terminó
// de salir (no desde que se decidió mandarlo): entre dos avisos de una misma
// estación pasa, como mínimo, el intervalo entero.
//
// Un aviso solo se da por emitido si el hub lo aceptó. Si lo rechaza (su
// canal de difusión está lleno), el estado vuelve a quedar pendiente y se
// reintenta en la pasada siguiente, un intervalo después, salvo que entre
// tanto llegue uno más nuevo de esa estación, que lo pisa: el último estado
// de una estación no se pierde.
//
// anotar nunca bloquea ni depende de que la goroutine esté corriendo: guarda
// en un mapa con candado, que no crece más allá de una entrada por estación,
// y deja una señal en un canal de un solo lugar.
type avisosRegimen struct {
	cada time.Duration
	// difundir manda un aviso y dice si quedó aceptado; false es «todavía
	// no»: se reintenta.
	difundir func(estadoEstacion) bool
	ahora    func() time.Time // el reloj; las pruebas lo cambian
	hay      chan struct{}    // hay pendientes nuevos; con búfer de 1

	mu         sync.Mutex
	pendientes map[string]almacen.RegimenEstacion

	// Lo que entró y lo que pasó con ello: recibidos = emitidos + agrupados +
	// los que siguen pendientes.
	recibidos  atomic.Uint64 // estados que la caché guardó
	emitidos   atomic.Uint64 // eventos que el hub aceptó
	agrupados  atomic.Uint64 // estados que otro posterior de su estación pisó antes de salir
	rechazados atomic.Uint64 // intentos que el hub no aceptó (se reintentan)
}

func nuevosAvisosRegimen(cada time.Duration, difundir func(estadoEstacion) bool) *avisosRegimen {
	if cada <= 0 {
		cada = cadaAvisoPorOmision
	}
	return &avisosRegimen{cada: cada, difundir: difundir, ahora: time.Now, hay: make(chan struct{}, 1),
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
	ultimo := map[string]time.Time{} // cuándo terminó de salir el último aviso de cada estación
	reloj := time.NewTimer(time.Hour)
	reloj.Stop()
	defer reloj.Stop()
	for {
		select {
		case <-ctx.Done():
			a.pasada(ultimo, true)
			return
		case <-a.hay:
		case <-reloj.C:
		}
		if proximo := a.pasada(ultimo, false); !proximo.IsZero() {
			reloj.Reset(proximo.Sub(a.ahora()))
		}
	}
}

// pasada mira los pendientes una vez: difunde, en orden de estación, los de
// las estaciones cuyo intervalo ya se cumplió (todos, si esTodo) y devuelve
// cuándo hay que volver a mirar: el vencimiento más cercano entre los que
// siguen pendientes, o el tiempo cero si no queda ninguno. ultimo es de quien
// llama: cuándo terminó de salir el último aviso de cada estación.
func (a *avisosRegimen) pasada(ultimo map[string]time.Time, esTodo bool) (proximo time.Time) {
	antes := func(t time.Time) {
		if proximo.IsZero() || t.Before(proximo) {
			proximo = t
		}
	}
	ahora := a.ahora()
	var listos []estadoEstacion
	a.mu.Lock()
	for estacion, r := range a.pendientes {
		if vence := ultimo[estacion].Add(a.cada); esTodo || !vence.After(ahora) {
			listos = append(listos, nuevoEstadoEstacion(estacion, r))
			delete(a.pendientes, estacion)
		} else {
			antes(vence)
		}
	}
	a.mu.Unlock()
	if len(listos) == 0 {
		return proximo
	}
	slices.SortFunc(listos, func(x, y estadoEstacion) int { return strings.Compare(x.Estacion, y.Estacion) })
	var devueltos []estadoEstacion
	for _, estado := range listos {
		if a.difundir(estado) {
			a.emitidos.Add(1)
		} else {
			a.rechazados.Add(1)
			devueltos = append(devueltos, estado)
		}
	}
	// El intervalo de cada estación corre desde ahora, que es después de que
	// su aviso salió: el siguiente no puede salir antes de un intervalo
	// entero. Una estación rechazada espera lo mismo para su reintento.
	despues := a.ahora()
	for _, estado := range listos {
		ultimo[estado.Estacion] = despues
	}
	if len(devueltos) > 0 {
		a.mu.Lock()
		for _, estado := range devueltos {
			if _, hayMasNuevo := a.pendientes[estado.Estacion]; hayMasNuevo {
				a.agrupados.Add(1) // mientras se intentaba llegó otro: ese es el estado
			} else {
				a.pendientes[estado.Estacion] = estado.RegimenEstacion
			}
		}
		a.mu.Unlock()
		antes(despues.Add(a.cada))
	}
	return proximo
}
