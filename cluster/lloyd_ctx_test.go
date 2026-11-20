package cluster

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/R0SEWT/aire-distribuido/kmeans"
	"github.com/R0SEWT/aire-distribuido/protocolo"
)

// El gancho recibe cada iteración, en orden, con lo mismo que queda en la
// Corrida y con el tiempo de cada nodo; el modelo no cambia por tenerlo.
func TestLloydCtxInformaCadaRonda(t *testing.T) {
	const n, k = 900, 3
	ruta, g := goldConRuido(t, n)
	p := levantar(t, ruta, n, 3, 64)
	cent0 := inicio(t, g.Datos, k, 7)

	var rondas []Ronda
	c, err := LloydCtx(context.Background(), p.enlaces, cent0, opcionesLloyd(k),
		OpcionesLloyd{Plazo: 2 * time.Second, Primera: 1, AlRonda: func(r Ronda) { rondas = append(rondas, r) }})
	if err != nil {
		t.Fatal(err)
	}
	p.terminar(t)
	comprobarCorrida(t, c, n)

	if len(rondas) != c.Resultado.Iteraciones {
		t.Fatalf("el gancho recibió %d rondas y hubo %d iteraciones", len(rondas), c.Resultado.Iteraciones)
	}
	for i, r := range rondas {
		if r.Iter != i+1 || r.Inercia != c.Resultado.Inercias[i] || r.Barrera != c.Rondas[i] || r.Fusion != c.Fusiones[i] {
			t.Fatalf("ronda %d: el gancho recibió %+v y la corrida dice inercia %v, barrera %v, fusión %v",
				i+1, r, c.Resultado.Inercias[i], c.Rondas[i], c.Fusiones[i])
		}
		if len(r.PorNodo) != len(p.enlaces) {
			t.Fatalf("ronda %d: %d tiempos por nodo para %d nodos", i+1, len(r.PorNodo), len(p.enlaces))
		}
		for j, d := range r.PorNodo {
			if d <= 0 || d > r.Barrera {
				t.Fatalf("ronda %d: el nodo %d tardó %v y la barrera %v", i+1, j, d, r.Barrera)
			}
		}
	}

	// El gancho no toca el resultado: es el de Lloyd sin gancho, que es el de
	// la fusión en orden de nodo.
	rangos, _ := Repartir(n, 3)
	if want := lloydEnMemoria(g.Datos, rangos, cent0, opcionesLloyd(k), 64); !mismosBits(c.Resultado.Centroides, want.Centroides) {
		t.Fatal("con el gancho los centroides ya no son los de la fusión en orden de nodo")
	}
}

// Cancelar entre dos rondas: no se envía la siguiente, el error dice que fue
// una cancelación y las rondas hechas vuelven con el error.
func TestLloydCtxSeCancelaEntreRondasYConservaLasHechas(t *testing.T) {
	const n, k = 900, 3
	ruta, g := goldConRuido(t, n)
	p := levantar(t, ruta, n, 2, 64)

	motivo := errors.New("la API se está apagando")
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	hechas := 0
	c, err := LloydCtx(ctx, p.enlaces, inicio(t, g.Datos, k, 7), kmeans.Opciones{K: k, MaxIter: 100, SinParoPorTolerancia: true},
		OpcionesLloyd{Plazo: 2 * time.Second, Primera: 1, AlRonda: func(Ronda) {
			if hechas++; hechas == 2 {
				cancel(motivo)
			}
		}})
	if !errors.Is(err, ErrCancelada) || !errors.Is(err, motivo) {
		t.Fatalf("se esperaba ErrCancelada con su causa; llegó %v", err)
	}
	if c == nil || len(c.Rondas) != 2 || len(c.Fusiones) != 2 || len(c.Resultado.Inercias) != 2 || c.UltimaRonda != 2 {
		t.Fatalf("la corrida cancelada tiene que traer sus 2 rondas hechas: %+v", c)
	}
	if c.Resultado.Centroides != nil {
		t.Fatal("una corrida a medias no publica centroides")
	}
}

// Cancelar a mitad de una ronda: no espera al plazo de la ronda.
func TestLloydCtxSeCancelaAMitadDeUnaRonda(t *testing.T) {
	const n = 600
	ruta, g := goldConRuido(t, n)
	ln := escuchar(t)
	sano := &Nodo{Nombre: "a-sano", Workers: 2, Chunk: 64, Gold: ruta, Log: silencio}
	ctxNodo, cancelNodo := context.WithCancel(context.Background())
	defer cancelNodo()
	go sano.Correr(ctxNodo, marcar(ln.Addr().String()), 5*time.Millisecond, true)
	nodoMudo(t, ln.Addr().String())
	espera, cancelEspera := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelEspera()
	enlaces := Enlaces(EsperarNodos(espera, ln, 2, time.Second, silencio))
	defer Terminar(enlaces, time.Second)
	rangos, _ := Repartir(n, len(enlaces))
	if _, err := Cargar(enlaces, rangos, huellaDe(t, ruta), time.Now().Add(5*time.Second)); err != nil {
		t.Fatalf("Cargar: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)
	inicioCorrida := time.Now()
	c, err := LloydCtx(ctx, enlaces, inicio(t, g.Datos, 3, 1), opcionesLloyd(3), OpcionesLloyd{Plazo: time.Minute, Primera: 1})
	if !errors.Is(err, ErrCancelada) {
		t.Fatalf("se esperaba ErrCancelada; llegó %v", err)
	}
	if strings.Contains(err.Error(), "mudo") {
		t.Errorf("el error de una cancelación no culpa a un nodo: %v", err)
	}
	if d := time.Since(inicioCorrida); d > 5*time.Second {
		t.Fatalf("tardó %v en cancelarse con un plazo de ronda de un minuto", d)
	}
	if c == nil || len(c.Rondas) != 0 {
		t.Fatalf("se canceló en la primera ronda: no hay rondas hechas (%+v)", c)
	}
}

// Si cae un nodo, LloydCtx devuelve las rondas que sí terminaron junto con el
// error que lo nombra.
func TestLloydCtxConUnNodoCaidoConservaLasRondasHechas(t *testing.T) {
	const n, k = 900, 3
	ruta, g := goldConRuido(t, n)
	p := levantar(t, ruta, n, 3, 64)

	hechas := 0
	c, err := LloydCtx(context.Background(), p.enlaces, inicio(t, g.Datos, k, 7),
		kmeans.Opciones{K: k, MaxIter: 100, SinParoPorTolerancia: true},
		OpcionesLloyd{Plazo: 2 * time.Second, Primera: 1, AlRonda: func(Ronda) {
			if hechas++; hechas == 3 {
				p.cortar[1]() // nodo-2 se apaga después de la tercera ronda
				time.Sleep(50 * time.Millisecond)
			}
		}})
	if err == nil || errors.Is(err, ErrCancelada) || !strings.Contains(err.Error(), `"nodo-2"`) {
		t.Fatalf("el error tiene que nombrar al nodo caído: %v", err)
	}
	if c == nil || len(c.Rondas) != 3 || c.Resultado.Centroides != nil {
		t.Fatalf("tienen que quedar las 3 rondas hechas y ningún modelo: %+v", c)
	}
}

// nodoQueNoCarga se presenta y responde los PING, pero nunca confirma el
// CARGAR. Devuelve un canal que se cierra cuando se le corta la conexión.
func nodoQueNoCarga(t *testing.T, addr, nombre string) <-chan struct{} {
	t.Helper()
	cortado := make(chan struct{})
	go func() {
		defer close(cortado)
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			return
		}
		defer conn.Close()
		cod, dec := protocolo.NuevoCodificador(conn), protocolo.NuevoDecodificador(conn)
		cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Hola, Version: protocolo.Version, Nodo: nombre, Workers: 1})
		for {
			m, err := dec.Leer()
			if err != nil {
				return
			}
			if m.Tipo == protocolo.Ping {
				cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Pong})
			}
		}
	}()
	return cortado
}

// Cancelar mientras los nodos cargan: no espera al plazo de la carga.
func TestCargarCtxSeCancelaSinEsperarElPlazo(t *testing.T) {
	ln := escuchar(t)
	cortado := nodoQueNoCarga(t, ln.Addr().String(), "lento")
	espera, cancelEspera := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelEspera()
	enlaces := Enlaces(EsperarNodos(espera, ln, 1, time.Second, silencio))
	if len(enlaces) != 1 {
		t.Fatal("el nodo no llegó")
	}
	defer enlaces[0].Cerrar()

	motivo := errors.New("la API se está apagando")
	ctx, cancel := context.WithCancelCause(context.Background())
	time.AfterFunc(50*time.Millisecond, func() { cancel(motivo) })
	inicio := time.Now()
	_, err := CargarCtx(ctx, enlaces, []protocolo.Rango{{Desde: 0, Hasta: 10}}, "sha256:abc", time.Now().Add(time.Minute))
	if !errors.Is(err, ErrCancelada) || !errors.Is(err, motivo) || errors.Is(err, protocolo.ErrPlazoVencido) || strings.Contains(err.Error(), "lento") {
		t.Fatalf("se esperaba ErrCancelada con su causa, sin culpar al nodo; llegó %v", err)
	}
	if d := time.Since(inicio); d > 5*time.Second {
		t.Fatalf("tardó %v en cancelarse con un plazo de carga de un minuto", d)
	}
	select {
	case <-cortado:
	case <-time.After(5 * time.Second):
		t.Fatal("al nodo no se le cortó la conexión")
	}
	// Con el contexto ya cancelado no manda nada.
	if _, err := CargarCtx(ctx, enlaces, []protocolo.Rango{{Desde: 0, Hasta: 10}}, "sha256:abc", time.Now().Add(time.Minute)); !errors.Is(err, ErrCancelada) {
		t.Fatalf("con el contexto cancelado: %v", err)
	}
}

// nodoDesmesurado carga su rango y responde cada ITERAR con un PARCIAL bien
// formado (todas sus filas en el cluster 0) cuya inercia es enorme pero finita.
func nodoDesmesurado(t *testing.T, addr, nombre string, inercia float64) {
	t.Helper()
	go func() {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			return
		}
		defer conn.Close()
		cod, dec := protocolo.NuevoCodificador(conn), protocolo.NuevoDecodificador(conn)
		cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Hola, Version: protocolo.Version, Nodo: nombre, Workers: 1})
		filas := 0
		for {
			m, err := dec.Leer()
			if err != nil {
				return
			}
			switch m.Tipo {
			case protocolo.Ping:
				cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Pong})
			case protocolo.Cargar:
				filas = m.Rango.Len()
				cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Listo, N: filas, Huella: m.Huella})
			case protocolo.Iterar:
				k := 3
				conteos := make([]int, k)
				conteos[0] = filas
				cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Parcial, Iter: m.Iter, Sumas: make([]float64, len(m.Centroides)),
					Conteos: conteos, Inercia: inercia})
			}
		}
	}()
}

// Dos parciales finitos cuya suma desborda: la ronda no se publica, la corrida
// aborta con un error que lo dice y el gancho no recibe nada.
func TestLloydAbortaSiLaFusionNoEsFinita(t *testing.T) {
	ln := escuchar(t)
	nodoDesmesurado(t, ln.Addr().String(), "a", 1.5e308)
	nodoDesmesurado(t, ln.Addr().String(), "b", 1.5e308)
	espera, cancelEspera := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelEspera()
	enlaces := Enlaces(EsperarNodos(espera, ln, 2, time.Second, silencio))
	if len(enlaces) != 2 {
		t.Fatal("no llegaron los dos nodos")
	}
	defer Abortar(enlaces, "fin de la prueba", time.Second)
	rangos, _ := Repartir(100, 2)
	if _, err := Cargar(enlaces, rangos, "sha256:abc", time.Now().Add(5*time.Second)); err != nil {
		t.Fatal(err)
	}
	rondas := 0
	c, err := LloydCtx(context.Background(), enlaces, []float64{0, 0, 1, 1, 2, 2}, opcionesLloyd(3),
		OpcionesLloyd{Plazo: 2 * time.Second, Primera: 1, AlRonda: func(Ronda) { rondas++ }})
	if err == nil || !errors.Is(err, protocolo.ErrProtocolo) || !strings.Contains(err.Error(), "no es finita") || !strings.Contains(err.Error(), "ronda 1") {
		t.Fatalf("se esperaba el aborto por una fusión no finita; llegó %v", err)
	}
	if rondas != 0 || c == nil || len(c.Rondas) != 0 || len(c.Resultado.Inercias) != 0 || c.Resultado.Centroides != nil {
		t.Fatalf("la ronda con +Inf no se publica: %d por el gancho, corrida %+v", rondas, c)
	}
}
