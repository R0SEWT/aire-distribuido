package cluster

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/R0SEWT/aire-distribuido/kmeans"
	"github.com/R0SEWT/aire-distribuido/protocolo"
)

// Tolerancia separa los dos efectos que pueden apartar el modelo distribuido
// del de una sola máquina. Reasociar la suma de los parciales de K nodos
// cambia los centroides en unos pocos ulp (del orden de 1e-16 en el espacio
// estandarizado, donde son de orden 1). Que una sola fila cambie de cluster
// mueve un centroide del orden de su distancia a la fila dividida por el
// tamaño del cluster: con el cluster más grande del gold real (48 161 filas) y
// una fila de la frontera, más de 1e-5. 1e-9 queda a cuatro órdenes de
// magnitud de cada uno: dentro, es redondeo; fuera, es otra asignación. Es el
// -tolerancia por omisión de cmd/distribuido y la compuerta con que la API
// decide si activa el modelo de un entrenamiento.
const Tolerancia = 1e-9

// Cargar le manda a cada nodo su rango del gold identificado por huella
// (CARGAR) y espera su LISTO, a todos a la vez y con un mismo instante límite:
// la carga es una barrera más, la primera de la corrida. rangos[i] es el del
// nodo enlaces[i], así que tienen que venir en orden de filas (Repartir).
// Devuelve cuánto tardó cada nodo; si alguno falla, el error nombra a cada uno
// que falló y la corrida no puede seguir.
func Cargar(enlaces []*protocolo.Enlace, rangos []protocolo.Rango, huella string, limite time.Time) ([]time.Duration, error) {
	return CargarCtx(context.Background(), enlaces, rangos, huella, limite)
}

// CargarCtx es Cargar con un contexto. Si se cancela ctx mientras los nodos
// cargan, vuelve enseguida con un error que cumple errors.Is(err, ErrCancelada)
// en vez de esperar al límite: cierra las conexiones de todos los nodos, que
// es lo único que destraba la lectura de un LISTO, igual que LloydCtx a mitad
// de una ronda.
func CargarCtx(ctx context.Context, enlaces []*protocolo.Enlace, rangos []protocolo.Rango, huella string, limite time.Time) ([]time.Duration, error) {
	if len(enlaces) == 0 || len(enlaces) != len(rangos) {
		return nil, fmt.Errorf("%d nodos y %d rangos", len(enlaces), len(rangos))
	}
	if ctx.Err() != nil {
		return nil, fmt.Errorf("%w antes de la carga: %w", ErrCancelada, context.Cause(ctx))
	}
	dejarDeCortar := context.AfterFunc(ctx, func() {
		for _, e := range enlaces {
			e.Cerrar()
		}
	})
	defer dejarDeCortar()
	tiempos := make([]time.Duration, len(enlaces))
	errs := make([]error, len(enlaces))
	var barrera sync.WaitGroup
	for i, e := range enlaces {
		barrera.Go(func() {
			// Cada goroutine escribe solo su índice y usa solo su enlace.
			inicio := time.Now()
			errs[i] = protocolo.AsignarRango(e, rangos[i], huella, limite)
			tiempos[i] = time.Since(inicio)
		})
	}
	barrera.Wait()
	if err := errors.Join(errs...); err != nil {
		// El error de una carga que se cortó por la cancelación es el de la
		// cancelación, no el del nodo al que se le cerró la conexión.
		if ctx.Err() != nil {
			return nil, fmt.Errorf("%w durante la carga: %w", ErrCancelada, context.Cause(ctx))
		}
		return nil, fmt.Errorf("carga abortada: %w", err)
	}
	return tiempos, nil
}

// Corrida es lo que deja Lloyd sobre el clúster.
type Corrida struct {
	// Resultado tiene los centroides finales, la inercia con esos centroides, la
	// inercia de cada iteración, las iteraciones, el motivo de paro, el
	// movimiento y los clusters vacíos de la última iteración. No tiene
	// Asignaciones: quedan repartidas en los nodos, y el protocolo no las trae.
	Resultado *kmeans.Resultado
	// Tamanos son los conteos de la pasada final: filas por cluster con los
	// centroides finales.
	Tamanos []int
	// Rondas es, por iteración, la barrera de red: desde que sale el primer
	// ITERAR hasta que llega el último PARCIAL (protocolo.Ronda).
	Rondas []time.Duration
	// Fusiones es, por iteración, lo que el coordinador hace después de la
	// barrera: validar los parciales, Fusionar en orden de nodo y
	// CerrarIteracion (actualizar los centroides y el criterio de parada).
	// Una iteración dura Rondas[i] + Fusiones[i].
	Fusiones []time.Duration
	// PasadaFinal es la barrera de la ronda extra con los centroides finales.
	PasadaFinal time.Duration
	// UltimaRonda es el número de la última ronda enviada: la corrida siguiente
	// sobre las mismas conexiones (otra semilla) empieza en UltimaRonda+1.
	UltimaRonda int
}

// Ronda es lo que Lloyd informa al terminar cada iteración (OpcionesLloyd.AlRonda).
type Ronda struct {
	Iter    int     // número de iteración, desde 1
	Inercia float64 // la de esta iteración: con los centroides que produjeron la asignación
	// Barrera es la barrera de red: desde que sale el primer ITERAR hasta que
	// llega el último PARCIAL. Fusion es lo que el coordinador hace después.
	Barrera time.Duration
	Fusion  time.Duration
	// PorNodo es lo que tardó cada nodo, en el orden de los enlaces
	// (protocolo.Enlace.TiempoRonda). La barrera va al ritmo del más lento.
	PorNodo []time.Duration
}

// OpcionesLloyd es lo que LloydCtx necesita además del contrato numérico.
type OpcionesLloyd struct {
	Plazo   time.Duration // plazo común de cada ronda
	Primera int           // número de la primera ronda (ver Lloyd)
	// AlRonda, si no es nil, se llama al cerrar cada iteración, en la goroutine
	// que llamó a LloydCtx y antes de empezar la ronda siguiente: lo que tarde
	// se suma al total de la corrida, no a las mediciones de la ronda. El
	// coordinador de la API lo usa para repartir cada ronda a sus consumidores.
	// La pasada final no se informa: no es una iteración.
	AlRonda func(Ronda)
}

// ErrCancelada: la corrida terminó porque se canceló su contexto (la API se
// está apagando), no porque fallara un nodo. Se compara con errors.Is; el
// error que la envuelve trae además la causa del contexto.
var ErrCancelada = errors.New("corrida cancelada")

// Lloyd corre K-Means sobre los nodos ya cargados (Cargar), desde los
// centroides cent0 y con el contrato numérico de op: el mismo de una sola
// máquina. Cada iteración es una protocolo.Ronda con plazo común (fan-out del
// ITERAR, fan-in de los PARCIAL), kmeans.Fusionar de los parciales EN ORDEN DE
// NODO —el de enlaces, que es el de las filas— y kmeans.CerrarIteracion, la
// misma función que cierra cada iteración de Secuencial y Concurrente: el
// criterio de parada no es una copia. Al final hace una pasada más con los
// centroides finales, como Concurrente, para que la inercia publicada sea la
// del modelo publicado.
//
// Las rondas se numeran desde primera y crecen de a una (protocolo.Ronda exige
// que crezcan durante toda la vida de cada conexión).
//
// Si un nodo cae, responde ERROR o no responde dentro de plazo, la ronda se
// aborta y Lloyd devuelve el error de Ronda, que nombra a cada nodo que falló,
// y ningún resultado: un modelo con menos nodos sería otro modelo
// (docs/diseno.md, §4.7). Quien llama decide qué hacer con las conexiones
// (Abortar).
//
// Es LloydCtx sin cancelación ni gancho por ronda: lo que usa cmd/distribuido.
func Lloyd(enlaces []*protocolo.Enlace, cent0 []float64, op kmeans.Opciones, plazo time.Duration, primera int) (*Corrida, error) {
	c, err := LloydCtx(context.Background(), enlaces, cent0, op, OpcionesLloyd{Plazo: plazo, Primera: primera})
	if err != nil {
		return nil, err
	}
	return c, nil
}

// LloydCtx es Lloyd con dos cosas más, que son las que necesita el coordinador
// dentro de la API (spec de la PC4, §5.2): un gancho por ronda (ol.AlRonda) y
// un contexto para cancelar la corrida.
//
// Si se cancela ctx, LloydCtx vuelve enseguida con un error que cumple
// errors.Is(err, ErrCancelada): entre dos rondas no envía la siguiente; a
// mitad de una, cierra las conexiones de todos los nodos, que es lo único que
// destraba la lectura de un PARCIAL (los nodos leen EOF y vuelven a llamar).
// En ese caso ya no se les puede mandar el ERROR de Abortar.
//
// Cuando la corrida no termina (un nodo falló o se canceló), además del error
// devuelve lo que alcanzó a hacer: una Corrida con las rondas completas, sus
// tiempos y sus inercias, y con Resultado.Centroides en nil. No es un modelo
// —los centroides de una corrida a medias no se publican— pero las rondas
// hechas no se pierden. Si el error es de los argumentos, la Corrida es nil.
func LloydCtx(ctx context.Context, enlaces []*protocolo.Enlace, cent0 []float64, op kmeans.Opciones, ol OpcionesLloyd) (*Corrida, error) {
	k := op.K
	if k <= 0 || len(cent0) == 0 || len(cent0)%k != 0 {
		return nil, fmt.Errorf("%d centroides iniciales no son k×D con k = %d", len(cent0), k)
	}
	if op.MaxIter <= 0 {
		return nil, fmt.Errorf("max_iter debe ser positivo, es %d", op.MaxIter)
	}
	if ol.Primera <= 0 || ol.Plazo <= 0 {
		return nil, fmt.Errorf("la primera ronda (%d) y el plazo (%v) tienen que ser positivos", ol.Primera, ol.Plazo)
	}
	dim := len(cent0) / k
	cent := slices.Clone(cent0)
	r := &kmeans.Resultado{K: k, D: dim, Paro: kmeans.ParoMaxIter}
	c := &Corrida{Resultado: r, UltimaRonda: ol.Primera - 1}

	// A mitad de una ronda cada goroutine de protocolo.Ronda está leyendo de su
	// conexión: cerrarlas es lo que las despierta. Cerrar un net.Conn es seguro
	// desde otra goroutine.
	dejarDeCortar := context.AfterFunc(ctx, func() {
		for _, e := range enlaces {
			e.Cerrar()
		}
	})
	defer dejarDeCortar()
	cancelada := func() error {
		if ctx.Err() == nil {
			return nil
		}
		return fmt.Errorf("%w tras %d rondas completas: %w", ErrCancelada, len(c.Rondas), context.Cause(ctx))
	}
	// ronda es una barrera con sus dos salidas malas: cancelada o con un nodo
	// que falló. El error de una ronda que se cortó por la cancelación es el
	// de la cancelación, no el del nodo al que se le cerró la conexión.
	ronda := func(n int) ([]protocolo.Mensaje, time.Duration, error) {
		if err := cancelada(); err != nil {
			return nil, 0, err
		}
		inicio := time.Now()
		ms, err := protocolo.Ronda(enlaces, n, k, cent, inicio.Add(ol.Plazo))
		c.UltimaRonda = n
		if err != nil {
			if errC := cancelada(); errC != nil {
				return nil, 0, errC
			}
			return nil, 0, err
		}
		return ms, time.Since(inicio), nil
	}

	n := ol.Primera
	for it := 1; it <= op.MaxIter; it++ {
		ms, barrera, err := ronda(n)
		if err != nil {
			return c, err
		}
		fusion := time.Now()
		p, err := fusionar(enlaces, ms, n, k, dim)
		if err != nil {
			return c, err
		}
		// Ronda ya volvió: ninguna goroutine sigue leyendo cent mientras se actualiza.
		convergio := kmeans.CerrarIteracion(r, cent, p, it, op)
		c.Rondas = append(c.Rondas, barrera)
		c.Fusiones = append(c.Fusiones, time.Since(fusion))
		if ol.AlRonda != nil {
			porNodo := make([]time.Duration, len(enlaces))
			for i, e := range enlaces {
				porNodo[i] = e.TiempoRonda
			}
			ol.AlRonda(Ronda{Iter: it, Inercia: p.Inercia, Barrera: barrera, Fusion: c.Fusiones[it-1], PorNodo: porNodo})
		}
		n++
		if convergio {
			break
		}
	}

	ms, barrera, err := ronda(n)
	if err != nil {
		return c, fmt.Errorf("pasada final: %w", err)
	}
	c.PasadaFinal = barrera
	p, err := fusionar(enlaces, ms, n, k, dim)
	if err != nil {
		return c, fmt.Errorf("pasada final: %w", err)
	}
	r.Inercia = p.Inercia
	r.Centroides = cent
	c.Tamanos = p.Conteos
	return c, nil
}

// fusionar suma los PARCIAL de una ronda, que vienen en orden de nodo, en ese
// orden.
func fusionar(enlaces []*protocolo.Enlace, ms []protocolo.Mensaje, ronda, k, dim int) (kmeans.Parcial, error) {
	ps := make([]kmeans.Parcial, len(ms))
	for i, m := range ms {
		ps[i] = kmeans.Parcial{Sumas: m.Sumas, Conteos: m.Conteos, Inercia: m.Inercia}
		// Ronda ya comprobó forma y conservación; esto es la última defensa
		// antes de fusionar: un parcial corrupto arruina los centroides de todos.
		if err := ps[i].Validar(k, dim); err != nil {
			return kmeans.Parcial{}, fmt.Errorf("ronda %d abortada: nodo %q: %w: %v", ronda, enlaces[i].Nodo, protocolo.ErrProtocolo, err)
		}
	}
	p := kmeans.Fusionar(ps, k, dim)
	// Cada parcial es finito, pero su suma puede no serlo: con valores
	// desmesurados la inercia o una suma desbordan a +Inf. Una ronda así no
	// tiene centroides ni inercia que publicar (JSON ni siquiera puede
	// escribirlos): la corrida se aborta aquí.
	if err := p.Validar(k, dim); err != nil {
		return kmeans.Parcial{}, fmt.Errorf("ronda %d abortada: la fusión de los parciales de los %d nodos no es finita: %w: %v "+
			"(algún nodo mandó valores desmesurados)", ronda, len(enlaces), protocolo.ErrProtocolo, err)
	}
	return p, nil
}
