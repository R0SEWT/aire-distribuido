package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/R0SEWT/aire-distribuido/protocolo"
)

var silencio = log.New(io.Discard, "", 0)

// nodoFalso llama a addr, se presenta y atiende PING y FIN. Con mudo lee los
// PING pero no responde. Devuelve por fin el último mensaje que recibió (FIN o
// ERROR), o un error si la conexión se cortó sin ninguno de los dos.
func nodoFalso(t *testing.T, addr, nombre string, version int, mudo bool) <-chan error {
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
				if !mudo {
					cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Pong})
				}
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

func nombres(ms []*miembro) string {
	var s []string
	for _, m := range ms {
		s = append(s, m.enlace.Nodo)
	}
	return strings.Join(s, ",")
}

func TestEsperarNodosJuntaLosNNodosOrdenadosPorNombre(t *testing.T) {
	ln := escuchar(t)
	for _, n := range []string{"pc-03", "pc-01", "pc-02"} {
		nodoFalso(t, ln.Addr().String(), n, protocolo.Version, false)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ms := esperarNodos(ctx, ln, 3, time.Second, silencio)
	if got := nombres(ms); got != "pc-01,pc-02,pc-03" {
		t.Fatalf("nodos = %s", got)
	}
	if ms[0].enlace.Workers != 4 || ms[0].direccion == "" {
		t.Fatalf("faltan datos del HOLA: %+v", ms[0])
	}
	cerrar(ms, time.Second)
}

func TestEsperarNodosVenceElPlazoConLosQueLlegaron(t *testing.T) {
	ln := escuchar(t)
	nodoFalso(t, ln.Addr().String(), "pc-01", protocolo.Version, false)
	nodoFalso(t, ln.Addr().String(), "pc-02", protocolo.Version, false)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	ms := esperarNodos(ctx, ln, 3, time.Second, silencio)
	if len(ms) != 2 {
		t.Fatalf("se esperaban los 2 que llegaron, hay %d (%s)", len(ms), nombres(ms))
	}
	cerrar(ms, time.Second)
}

func TestEsperarNodosRechazaOtraVersionYLeAvisa(t *testing.T) {
	ln := escuchar(t)
	viejo := nodoFalso(t, ln.Addr().String(), "pc-viejo", protocolo.Version+1, false)
	nodoFalso(t, ln.Addr().String(), "pc-01", protocolo.Version, false)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ms := esperarNodos(ctx, ln, 1, time.Second, silencio)
	if got := nombres(ms); got != "pc-01" {
		t.Fatalf("nodos = %s", got)
	}
	if err := <-viejo; err == nil || !strings.Contains(err.Error(), "versión") {
		t.Fatalf("el nodo con otra versión tenía que recibir un ERROR que la nombre; recibió %v", err)
	}
	cerrar(ms, time.Second)
}

func TestEsperarNodosUnaReconexionReemplazaALaConexionVieja(t *testing.T) {
	ln := escuchar(t)
	primera := nodoFalso(t, ln.Addr().String(), "pc-01", protocolo.Version, false)
	segunda := nodoFalso(t, ln.Addr().String(), "pc-01", protocolo.Version, false)
	nodoFalso(t, ln.Addr().String(), "pc-02", protocolo.Version, false)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ms := esperarNodos(ctx, ln, 2, time.Second, silencio)
	if got := nombres(ms); got != "pc-01,pc-02" {
		t.Fatalf("nodos = %s", got)
	}
	cerrar(ms, time.Second)
	// Una de las dos conexiones de pc-01 se cerró sin FIN; la otra recibió el FIN.
	a, b := <-primera, <-segunda
	if (a == nil) == (b == nil) {
		t.Fatalf("se esperaba un FIN y un cierre; llegaron %v y %v", a, b)
	}
}

func TestMedirJuntaLatidosYSacaAlNodoQueNoResponde(t *testing.T) {
	ln := escuchar(t)
	nodoFalso(t, ln.Addr().String(), "pc-01", protocolo.Version, false)
	nodoFalso(t, ln.Addr().String(), "pc-mudo", protocolo.Version, true)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ms := esperarNodos(ctx, ln, 2, time.Second, silencio)

	barreras := medir(ms, 4, 150*time.Millisecond)
	sano, mudo := ms[0], ms[1]
	if len(sano.latidos) != 4 || sano.err != nil {
		t.Fatalf("pc-01: %d latidos, err %v; se esperaban 4 sin error", len(sano.latidos), sano.err)
	}
	if !errors.Is(mudo.err, protocolo.ErrPlazoVencido) || len(mudo.latidos) != 0 {
		t.Fatalf("pc-mudo: err %v, %d latidos; se esperaba plazo vencido y ningún latido", mudo.err, len(mudo.latidos))
	}
	// La ronda 1 falló por pc-mudo; las 3 siguientes, solo con pc-01, se completan.
	if len(barreras) != 3 {
		t.Fatalf("se esperaban 3 rondas completas, hubo %d", len(barreras))
	}
	cerrar(ms, time.Second)
}

func TestCerrarMandaFinALosSanos(t *testing.T) {
	ln := escuchar(t)
	fin := nodoFalso(t, ln.Addr().String(), "pc-01", protocolo.Version, false)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ms := esperarNodos(ctx, ln, 1, time.Second, silencio)

	cerrar(ms, time.Second)
	if err := <-fin; err != nil {
		t.Fatalf("el nodo tenía que recibir FIN; recibió %v", err)
	}
}

func TestPercentilPorRangoMasCercano(t *testing.T) {
	xs := []time.Duration{5, 1, 4, 2, 3, 10, 9, 8, 7, 6}
	for _, c := range []struct {
		p    float64
		want time.Duration
	}{{50, 5}, {95, 10}, {100, 10}, {10, 1}, {0, 1}} {
		if got := percentil(xs, c.p); got != c.want {
			t.Errorf("percentil(%v) = %v, se esperaba %v", c.p, got, c.want)
		}
	}
	if got := percentil(nil, 50); got != 0 {
		t.Errorf("percentil de nada = %v", got)
	}
}

func TestReporteMarcaFallasEImprimeElVeredicto(t *testing.T) {
	ln := escuchar(t)
	nodoFalso(t, ln.Addr().String(), "pc-01", protocolo.Version, false)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ms := esperarNodos(ctx, ln, 1, time.Second, silencio)
	barreras := medir(ms, 3, time.Second)
	cerrar(ms, time.Second)

	r := armarReporte(ms, barreras, 2, 3, time.Second)
	if r.OK || r.Conectados != 1 || r.Nodos[0].Latido.N != 3 || len(r.Nodos[0].Muestras) != 3 {
		t.Fatalf("reporte inesperado: %+v", r)
	}
	var out bytes.Buffer
	imprimir(&out, r)
	if !strings.Contains(out.String(), "INCOMPLETO: llegaron 1 de 2") {
		t.Fatalf("el veredicto no dice que faltó un nodo:\n%s", out.String())
	}
}
