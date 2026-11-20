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

// nodoFalso llama a addr, se presenta y atiende PING y FIN. Con mudo responde
// solo el primer PING, el de la espera de nodos (cluster.EsperarNodos comprueba
// que cada uno sigue ahí), y después los lee pero no responde. Devuelve por fin el último mensaje que recibió (FIN o
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
		for pings := 0; ; {
			m, err := dec.Leer()
			if err != nil {
				fin <- err
				return
			}
			switch m.Tipo {
			case protocolo.Ping:
				if pings++; !mudo || pings == 1 {
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
