package cluster

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/R0SEWT/aire-distribuido/protocolo"
)

// nodoFalso llama a addr, se presenta y atiende PING y FIN. Devuelve por fin
// nil si recibió FIN, el ERROR del coordinador como error, o el error de la
// conexión si se cortó sin ninguno de los dos.
func nodoFalso(t *testing.T, addr, nombre string, version int) <-chan error {
	t.Helper()
	fin := make(chan error, 1)
	go func() {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			fin <- err
			return
		}
		defer conn.Close()
		cod, dec := protocolo.NuevoCodificador(conn), protocolo.NuevoDecodificador(conn)
		if err := cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Hola, Version: version, Nodo: nombre, Workers: 4}); err != nil {
			fin <- err
			return
		}
		for {
			m, err := dec.Leer()
			if err != nil {
				fin <- err
				return
			}
			switch m.Tipo {
			case protocolo.Ping:
				cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Pong})
			case protocolo.Fin:
				fin <- nil
				return
			case protocolo.Error:
				fin <- errors.New("ERROR: " + m.Mensaje)
				return
			}
		}
	}()
	return fin
}

func escuchar(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	return ln
}

func nombres(ms []*Miembro) string {
	var s []string
	for _, m := range ms {
		s = append(s, m.Enlace.Nodo)
	}
	return strings.Join(s, ",")
}

func TestEsperarNodosJuntaLosNNodosOrdenadosPorNombre(t *testing.T) {
	ln := escuchar(t)
	for _, n := range []string{"pc-03", "pc-01", "pc-02"} {
		nodoFalso(t, ln.Addr().String(), n, protocolo.Version)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ms := EsperarNodos(ctx, ln, 3, time.Second, silencio)
	if got := nombres(ms); got != "pc-01,pc-02,pc-03" {
		t.Fatalf("nodos = %s", got)
	}
	if ms[0].Enlace.Workers != 4 || ms[0].Direccion == "" {
		t.Fatalf("faltan datos del HOLA: %+v", ms[0])
	}
	Terminar(Enlaces(ms), time.Second)
}

func TestEsperarNodosVenceElPlazoConLosQueLlegaron(t *testing.T) {
	ln := escuchar(t)
	nodoFalso(t, ln.Addr().String(), "pc-01", protocolo.Version)
	nodoFalso(t, ln.Addr().String(), "pc-02", protocolo.Version)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	ms := EsperarNodos(ctx, ln, 3, time.Second, silencio)
	if len(ms) != 2 {
		t.Fatalf("se esperaban los 2 que llegaron, hay %d (%s)", len(ms), nombres(ms))
	}
	Terminar(Enlaces(ms), time.Second)
}

func TestEsperarNodosRechazaOtraVersionYLeAvisa(t *testing.T) {
	ln := escuchar(t)
	viejo := nodoFalso(t, ln.Addr().String(), "pc-viejo", protocolo.Version+1)
	nodoFalso(t, ln.Addr().String(), "pc-01", protocolo.Version)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ms := EsperarNodos(ctx, ln, 1, time.Second, silencio)
	if got := nombres(ms); got != "pc-01" {
		t.Fatalf("nodos = %s", got)
	}
	if err := <-viejo; err == nil || !strings.Contains(err.Error(), "versión") {
		t.Fatalf("el nodo con otra versión tenía que recibir un ERROR que la nombre; recibió %v", err)
	}
	Terminar(Enlaces(ms), time.Second)
}

func TestEsperarNodosUnaReconexionReemplazaALaConexionVieja(t *testing.T) {
	ln := escuchar(t)
	primera := nodoFalso(t, ln.Addr().String(), "pc-01", protocolo.Version)
	segunda := nodoFalso(t, ln.Addr().String(), "pc-01", protocolo.Version)
	nodoFalso(t, ln.Addr().String(), "pc-02", protocolo.Version)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ms := EsperarNodos(ctx, ln, 2, time.Second, silencio)
	if got := nombres(ms); got != "pc-01,pc-02" {
		t.Fatalf("nodos = %s", got)
	}
	Terminar(Enlaces(ms), time.Second)
	// Una de las dos conexiones de pc-01 se cerró sin FIN; la otra recibió el FIN.
	a, b := <-primera, <-segunda
	if (a == nil) == (b == nil) {
		t.Fatalf("se esperaba un FIN y un cierre; llegaron %v y %v", a, b)
	}
}

func TestAbortarLeAvisaACadaNodoPorQue(t *testing.T) {
	ln := escuchar(t)
	fin := nodoFalso(t, ln.Addr().String(), "pc-01", protocolo.Version)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ms := EsperarNodos(ctx, ln, 1, time.Second, silencio)

	Abortar(Enlaces(ms), "ronda 3 abortada: nodo \"pc-02\": plazo vencido", time.Second)
	if err := <-fin; err == nil || !strings.Contains(err.Error(), "pc-02") {
		t.Fatalf("el nodo tenía que recibir un ERROR con el motivo; recibió %v", err)
	}
}

func TestPercentilPorRangoMasCercano(t *testing.T) {
	xs := []time.Duration{5, 1, 4, 2, 3, 10, 9, 8, 7, 6}
	for _, c := range []struct {
		p    float64
		want time.Duration
	}{{50, 5}, {95, 10}, {100, 10}, {10, 1}, {0, 1}} {
		if got := Percentil(xs, c.p); got != c.want {
			t.Errorf("Percentil(%v) = %v, se esperaba %v", c.p, got, c.want)
		}
	}
	if got := Percentil(nil, 50); got != 0 {
		t.Errorf("percentil de nada = %v", got)
	}
}
