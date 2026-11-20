package cluster

import (
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/R0SEWT/aire-distribuido/kmeans"
	"github.com/R0SEWT/aire-distribuido/protocolo"
)

// Cargar le manda a cada nodo su rango del gold identificado por huella
// (CARGAR) y espera su LISTO, a todos a la vez y con un mismo instante límite:
// la carga es una barrera más, la primera de la corrida. rangos[i] es el del
// nodo enlaces[i], así que tienen que venir en orden de filas (Repartir).
// Devuelve cuánto tardó cada nodo; si alguno falla, el error nombra a cada uno
// que falló y la corrida no puede seguir.
func Cargar(enlaces []*protocolo.Enlace, rangos []protocolo.Rango, huella string, limite time.Time) ([]time.Duration, error) {
	if len(enlaces) == 0 || len(enlaces) != len(rangos) {
		return nil, fmt.Errorf("%d nodos y %d rangos", len(enlaces), len(rangos))
	}
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
func Lloyd(enlaces []*protocolo.Enlace, cent0 []float64, op kmeans.Opciones, plazo time.Duration, primera int) (*Corrida, error) {
	k := op.K
	if k <= 0 || len(cent0) == 0 || len(cent0)%k != 0 {
		return nil, fmt.Errorf("%d centroides iniciales no son k×D con k = %d", len(cent0), k)
	}
	if op.MaxIter <= 0 {
		return nil, fmt.Errorf("max_iter debe ser positivo, es %d", op.MaxIter)
	}
	if primera <= 0 || plazo <= 0 {
		return nil, fmt.Errorf("la primera ronda (%d) y el plazo (%v) tienen que ser positivos", primera, plazo)
	}
	dim := len(cent0) / k
	cent := slices.Clone(cent0)
	r := &kmeans.Resultado{K: k, D: dim, Paro: kmeans.ParoMaxIter}
	c := &Corrida{Resultado: r}

	ronda := primera
	for it := 1; it <= op.MaxIter; it++ {
		inicio := time.Now()
		ms, err := protocolo.Ronda(enlaces, ronda, k, cent, inicio.Add(plazo))
		if err != nil {
			return nil, err
		}
		fusion := time.Now()
		p, err := fusionar(enlaces, ms, ronda, k, dim)
		if err != nil {
			return nil, err
		}
		// Ronda ya volvió: ninguna goroutine sigue leyendo cent mientras se actualiza.
		convergio := kmeans.CerrarIteracion(r, cent, p, it, op)
		fin := time.Now()
		c.Rondas = append(c.Rondas, fusion.Sub(inicio))
		c.Fusiones = append(c.Fusiones, fin.Sub(fusion))
		ronda++
		if convergio {
			break
		}
	}

	inicio := time.Now()
	ms, err := protocolo.Ronda(enlaces, ronda, k, cent, inicio.Add(plazo))
	if err != nil {
		return nil, fmt.Errorf("pasada final: %w", err)
	}
	c.PasadaFinal = time.Since(inicio)
	p, err := fusionar(enlaces, ms, ronda, k, dim)
	if err != nil {
		return nil, fmt.Errorf("pasada final: %w", err)
	}
	c.UltimaRonda = ronda
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
	return kmeans.Fusionar(ps, k, dim), nil
}
