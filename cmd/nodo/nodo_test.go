package main

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/R0SEWT/aire-distribuido/protocolo"
)

func nodoDePrueba() *nodo {
	return &nodo{nombre: "pc-07", workers: 8, log: log.New(io.Discard, "", 0)}
}

// coordinadorFalso habla el lado del coordinador sobre un extremo de net.Pipe:
// lee el HOLA y después corre guion. Devuelve el HOLA por hola y el error del
// guion por errs.
func coordinadorFalso(t *testing.T, conn net.Conn, guion func(*protocolo.Codificador, *protocolo.Decodificador) error) (<-chan protocolo.Mensaje, <-chan error) {
	t.Helper()
	hola := make(chan protocolo.Mensaje, 1)
	errs := make(chan error, 1)
	go func() {
		defer conn.Close()
		cod, dec := protocolo.NuevoCodificador(conn), protocolo.NuevoDecodificador(conn)
		m, err := dec.Leer()
		if err != nil {
			errs <- err
			return
		}
		hola <- m
		errs <- guion(cod, dec)
	}()
	return hola, errs
}

func esperarTipo(dec *protocolo.Decodificador, tipo protocolo.Tipo) (protocolo.Mensaje, error) {
	m, err := dec.LeerConPlazo(2 * time.Second)
	if err != nil {
		return m, err
	}
	if m.Tipo != tipo {
		return m, errors.New("se esperaba " + string(tipo) + " y llegó " + string(m.Tipo))
	}
	return m, nil
}

func TestAtenderSePresentaRespondePingYTerminaConFin(t *testing.T) {
	nodoConn, coordConn := net.Pipe()
	hola, errs := coordinadorFalso(t, coordConn, func(cod *protocolo.Codificador, dec *protocolo.Decodificador) error {
		for range 3 {
			if err := cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Ping}); err != nil {
				return err
			}
			if _, err := esperarTipo(dec, protocolo.Pong); err != nil {
				return err
			}
		}
		return cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Fin})
	})

	if err := nodoDePrueba().atender(nodoConn); err != nil {
		t.Fatalf("atender: %v", err)
	}
	if err := <-errs; err != nil {
		t.Fatalf("coordinador: %v", err)
	}
	h := <-hola
	if h.Tipo != protocolo.Hola || h.Version != protocolo.Version || h.Nodo != "pc-07" || h.Workers != 8 {
		t.Fatalf("HOLA inesperado: %+v", h)
	}
}

func TestAtenderRespondeErrorACargarYSigueAtendiendo(t *testing.T) {
	nodoConn, coordConn := net.Pipe()
	_, errs := coordinadorFalso(t, coordConn, func(cod *protocolo.Codificador, dec *protocolo.Decodificador) error {
		cargar := protocolo.Mensaje{Tipo: protocolo.Cargar, Rango: &protocolo.Rango{Desde: 0, Hasta: 10}, Huella: "sha256:00"}
		if err := cod.Escribir(cargar); err != nil {
			return err
		}
		m, err := esperarTipo(dec, protocolo.Error)
		if err != nil {
			return err
		}
		if !strings.Contains(m.Mensaje, "CARGAR") {
			return errors.New("el ERROR no nombra el CARGAR: " + m.Mensaje)
		}
		if err := cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Ping}); err != nil {
			return err
		}
		if _, err := esperarTipo(dec, protocolo.Pong); err != nil {
			return err
		}
		return cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Fin})
	})

	if err := nodoDePrueba().atender(nodoConn); err != nil {
		t.Fatalf("atender: %v", err)
	}
	if err := <-errs; err != nil {
		t.Fatalf("coordinador: %v", err)
	}
}

func TestAtenderDevuelveElErrorDelCoordinador(t *testing.T) {
	nodoConn, coordConn := net.Pipe()
	_, errs := coordinadorFalso(t, coordConn, func(cod *protocolo.Codificador, _ *protocolo.Decodificador) error {
		return cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Error, Mensaje: "versión de protocolo 1, el coordinador habla la 2"})
	})

	err := nodoDePrueba().atender(nodoConn)
	if !errors.Is(err, errCoordinador) || !strings.Contains(err.Error(), "versión") {
		t.Fatalf("se esperaba errCoordinador con el texto del coordinador, llegó %v", err)
	}
	if err := <-errs; err != nil {
		t.Fatalf("coordinador: %v", err)
	}
}

func TestAtenderSinFinEsUnError(t *testing.T) {
	nodoConn, coordConn := net.Pipe()
	_, errs := coordinadorFalso(t, coordConn, func(*protocolo.Codificador, *protocolo.Decodificador) error {
		return nil // cierra la conexión sin FIN
	})

	if err := nodoDePrueba().atender(nodoConn); err == nil {
		t.Fatal("una conexión que se corta sin FIN tiene que ser un error")
	}
	if err := <-errs; err != nil {
		t.Fatalf("coordinador: %v", err)
	}
}

func TestCorrerReintentaHastaQueElCoordinadorEscucha(t *testing.T) {
	var intentos atomic.Int32
	llamar := func(context.Context) (net.Conn, error) {
		if intentos.Add(1) <= 2 {
			return nil, errors.New("connection refused")
		}
		nodoConn, coordConn := net.Pipe()
		coordinadorFalso(t, coordConn, func(cod *protocolo.Codificador, _ *protocolo.Decodificador) error {
			return cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Fin})
		})
		return nodoConn, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := nodoDePrueba().correr(ctx, llamar, time.Millisecond, true); err != nil {
		t.Fatalf("correr: %v", err)
	}
	if got := intentos.Load(); got != 3 {
		t.Fatalf("se esperaban 3 intentos de conexión, hubo %d", got)
	}
}

func TestCorrerVuelveALlamarDespuesDeCadaCorrida(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var corridas atomic.Int32
	llamar := func(context.Context) (net.Conn, error) {
		nodoConn, coordConn := net.Pipe()
		coordinadorFalso(t, coordConn, func(cod *protocolo.Codificador, _ *protocolo.Decodificador) error {
			if corridas.Add(1) == 3 {
				cancel() // a la tercera, el usuario aprieta Ctrl+C
			}
			return cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Fin})
		})
		return nodoConn, nil
	}

	err := nodoDePrueba().correr(ctx, llamar, time.Millisecond, false)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("sin -una el nodo solo termina al cancelarlo; devolvió %v", err)
	}
	if got := corridas.Load(); got < 3 {
		t.Fatalf("se esperaban al menos 3 corridas, hubo %d", got)
	}
}

func TestCorrerSeDetieneConCtrlCAunqueNadieEscuche(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	llamar := func(context.Context) (net.Conn, error) { return nil, errors.New("connection refused") }

	err := nodoDePrueba().correr(ctx, llamar, 5*time.Millisecond, false)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("se esperaba el error del contexto, llegó %v", err)
	}
}
