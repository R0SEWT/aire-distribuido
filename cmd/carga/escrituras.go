package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// La API responde cada predicción sin esperar a las bases: la encola y una
// goroutine por almacén la escribe por lotes; si una cola se llena, la
// predicción se descarta y el pedido responde 200 igual (spec de la PC4,
// §6.2). Por eso la latencia y los códigos HTTP no dicen si lo predicho quedó
// guardado. Eso lo cuenta la API en GET /api/metricas, que es de rol admin:
// cmd/carga lo lee antes y después de la corrida y deja la diferencia en el
// informe.

// almacenEscritura es el esquema EscrituraAlmacen del contrato.
type almacenEscritura struct {
	Escritas    uint64 `json:"escritas"`
	Descartadas uint64 `json:"descartadas"`
	Fallidas    uint64 `json:"fallidas"`
	Lotes       uint64 `json:"lotes"`
}

// cuenta es de cuántas predicciones ya se sabe qué pasó: cada una termina
// escrita, descartada o fallida.
func (a almacenEscritura) cuenta() uint64 { return a.Escritas + a.Descartadas + a.Fallidas }

// menos resta los contadores que la API ya traía antes de la corrida. Si la
// API se reinició en el medio, los contadores vuelven a cero y la resta se
// queda en cero en vez de dar la vuelta.
func (a almacenEscritura) menos(b almacenEscritura) almacenEscritura {
	resta := func(x, y uint64) uint64 {
		if x < y {
			return 0
		}
		return x - y
	}
	return almacenEscritura{resta(a.Escritas, b.Escritas), resta(a.Descartadas, b.Descartadas), resta(a.Fallidas, b.Fallidas), resta(a.Lotes, b.Lotes)}
}

// escrituras es lo que pasó en los almacenes con las predicciones de la
// corrida, calentamiento incluido.
type escrituras struct {
	// Medido es false si no se pasó la clave de un admin; Motivo dice por qué.
	Medido bool   `json:"medido"`
	Motivo string `json:"motivo,omitempty"`
	// PrediccionesAceptadas son los pedidos que respondieron 200.
	PrediccionesAceptadas int `json:"predicciones_aceptadas"`
	// Completas: los dos almacenes dieron cuenta de todas las aceptadas
	// (escritas, descartadas o fallidas) antes de que venciera la espera.
	Completas bool    `json:"completas"`
	EsperaS   float64 `json:"espera_s"`
	// Lo que cada almacén sumó durante la corrida. Si otro cliente usa la API
	// al mismo tiempo, sus predicciones también entran aquí.
	Mongo *almacenEscritura `json:"mongo,omitempty"`
	Redis *almacenEscritura `json:"redis,omitempty"`
}

// Perdidas son las predicciones aceptadas que algún almacén no guardó:
// descartadas por una cola llena o rechazadas por la base.
func (e escrituras) Perdidas() uint64 {
	var n uint64
	for _, a := range []*almacenEscritura{e.Mongo, e.Redis} {
		if a != nil {
			n += a.Descartadas + a.Fallidas
		}
	}
	return n
}

// SinCuenta son las aceptadas de las que el almacén más atrasado todavía no
// dijo nada cuando venció la espera.
func (e escrituras) SinCuenta() uint64 {
	var falta uint64
	for _, a := range []*almacenEscritura{e.Mongo, e.Redis} {
		if a != nil && a.cuenta() < uint64(e.PrediccionesAceptadas) {
			falta = max(falta, uint64(e.PrediccionesAceptadas)-a.cuenta())
		}
	}
	return falta
}

type fotoEscritura struct{ mongo, redis almacenEscritura }

// pedirEscritura lee el detalle por almacén de GET /api/metricas.
func pedirEscritura(ctx context.Context, hc *http.Client, base, token string) (fotoEscritura, error) {
	pedido, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/metricas", nil)
	if err != nil {
		return fotoEscritura{}, err
	}
	pedido.Header.Set("Authorization", "Bearer "+token)
	resp, err := hc.Do(pedido)
	if err != nil {
		return fotoEscritura{}, fmt.Errorf("leyendo /api/metricas: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fotoEscritura{}, fmt.Errorf("GET /api/metricas respondió %d: el usuario de -admin tiene que tener el rol admin", resp.StatusCode)
	}
	var m struct {
		Escritura *struct {
			Mongo *almacenEscritura `json:"mongo"`
			Redis *almacenEscritura `json:"redis"`
		} `json:"escritura"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&m); err != nil {
		return fotoEscritura{}, fmt.Errorf("GET /api/metricas: %w", err)
	}
	if m.Escritura == nil || m.Escritura.Mongo == nil || m.Escritura.Redis == nil {
		return fotoEscritura{}, fmt.Errorf("GET /api/metricas no trae escritura.mongo y escritura.redis")
	}
	return fotoEscritura{*m.Escritura.Mongo, *m.Escritura.Redis}, nil
}

// pausaEscrituras es cada cuánto se vuelve a leer /api/metricas mientras los
// almacenes terminan de escribir.
const pausaEscrituras = 50 * time.Millisecond

// esperarEscrituras lee /api/metricas hasta que los dos almacenes dan cuenta
// de las predicciones aceptadas o vence el plazo, y devuelve la diferencia con
// la foto de antes de la corrida.
func esperarEscrituras(ctx context.Context, hc *http.Client, base, token string, antes fotoEscritura, aceptadas int, plazo time.Duration) (escrituras, error) {
	inicio := time.Now()
	for {
		ahora, err := pedirEscritura(ctx, hc, base, token)
		if err != nil {
			return escrituras{}, err
		}
		mongo, redis := ahora.mongo.menos(antes.mongo), ahora.redis.menos(antes.redis)
		completas := mongo.cuenta() >= uint64(aceptadas) && redis.cuenta() >= uint64(aceptadas)
		if completas || time.Since(inicio) >= plazo {
			return escrituras{Medido: true, PrediccionesAceptadas: aceptadas, Completas: completas,
				EsperaS: time.Since(inicio).Seconds(), Mongo: &mongo, Redis: &redis}, nil
		}
		select {
		case <-ctx.Done():
			return escrituras{}, fmt.Errorf("esperando las escrituras: %w", ctx.Err())
		case <-time.After(pausaEscrituras):
		}
	}
}

// imprimirEscrituras deja a la vista lo que pasó en cada almacén, con un aviso
// si algo se perdió o no se llegó a saber.
func imprimirEscrituras(w io.Writer, e escrituras) {
	if !e.Medido {
		fmt.Fprintf(w, "\nescrituras: no se midieron (%s)\n", e.Motivo)
		return
	}
	fmt.Fprintf(w, "\nescrituras de las %d predicciones aceptadas (GET /api/metricas, después menos antes; espera de %.2f s):\n",
		e.PrediccionesAceptadas, e.EsperaS)
	for _, a := range []struct {
		nombre string
		e      *almacenEscritura
	}{{"mongo", e.Mongo}, {"redis", e.Redis}} {
		fmt.Fprintf(w, "  %s  %d escritas · %d descartadas · %d fallidas · %d lotes\n", a.nombre, a.e.Escritas, a.e.Descartadas, a.e.Fallidas, a.e.Lotes)
	}
	if n := e.Perdidas(); n > 0 {
		fmt.Fprintf(w, "AVISO: %d escrituras perdidas (descartadas por una cola llena o rechazadas por la base): esas predicciones respondieron 200 y no quedaron guardadas\n", n)
	}
	if !e.Completas {
		fmt.Fprintf(w, "AVISO: tras %.2f s los almacenes todavía no dieron cuenta de %d predicciones (-espera-escrituras)\n", e.EsperaS, e.SinCuenta())
	}
}
