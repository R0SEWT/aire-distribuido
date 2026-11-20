package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/R0SEWT/aire-distribuido/protocolo"
)

// errCoordinador: el coordinador mandó un ERROR (por ejemplo, otra versión del
// protocolo). Con -una el nodo termina; si no, vuelve a llamar.
var errCoordinador = errors.New("el coordinador informó un error")

type nodo struct {
	nombre  string
	workers int
	log     *log.Logger
}

// correr llama al coordinador, atiende una corrida y vuelve a llamar, hasta que
// se cancela ctx o, con una, termina la primera corrida. Devuelve nil solo con
// una y una corrida que acabó en FIN.
func (n *nodo) correr(ctx context.Context, llamar func(context.Context) (net.Conn, error), espera time.Duration, una bool) error {
	for {
		conn, err := n.conectar(ctx, llamar, espera)
		if err != nil {
			return err
		}
		n.log.Printf("conectado desde %s a %s", conn.LocalAddr(), conn.RemoteAddr())

		// Si se cancela ctx (Ctrl+C) a mitad de una corrida, cerrar la conexión
		// destraba la lectura de atender.
		cortar := context.AfterFunc(ctx, func() { conn.Close() })
		err = n.atender(conn)
		cortar()
		conn.Close()

		switch {
		case ctx.Err() != nil:
			return ctx.Err()
		case err == nil && una:
			return nil
		case err == nil:
			n.log.Printf("corrida terminada; espero la siguiente")
		case una:
			return err
		default:
			n.log.Printf("%v; vuelvo a llamar", err)
		}
		if !dormir(ctx, espera) {
			return ctx.Err()
		}
	}
}

// conectar reintenta cada espera hasta abrir la conexión o hasta que se cancela
// ctx. No llena la pantalla: avisa en el primer fallo y luego cada ~30 s.
func (n *nodo) conectar(ctx context.Context, llamar func(context.Context) (net.Conn, error), espera time.Duration) (net.Conn, error) {
	var ultimoAviso time.Time
	for intento := 1; ; intento++ {
		conn, err := llamar(ctx)
		if err == nil {
			return conn, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if time.Since(ultimoAviso) >= 30*time.Second {
			n.log.Printf("el coordinador no responde (intento %d): %v; sigo intentando", intento, err)
			ultimoAviso = time.Now()
		}
		if !dormir(ctx, espera) {
			return nil, ctx.Err()
		}
	}
}

// atender habla el protocolo sobre una conexión abierta: manda el HOLA y
// responde hasta el FIN. Devuelve nil solo si la corrida terminó con FIN.
func (n *nodo) atender(conn net.Conn) error {
	cod, dec := protocolo.NuevoCodificador(conn), protocolo.NuevoDecodificador(conn)
	hola := protocolo.Mensaje{Tipo: protocolo.Hola, Version: protocolo.Version, Nodo: n.nombre, Workers: n.workers}
	if err := cod.Escribir(hola); err != nil {
		return fmt.Errorf("enviar HOLA: %w", err)
	}

	pongs := 0
	for {
		m, err := dec.Leer()
		if err != nil {
			return fmt.Errorf("la conexión se cortó antes del FIN (%d PONG enviados): %w", pongs, err)
		}
		switch m.Tipo {
		case protocolo.Ping:
			if err := cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Pong}); err != nil {
				return fmt.Errorf("enviar PONG: %w", err)
			}
			pongs++
		case protocolo.Fin:
			n.log.Printf("FIN: corrida OK, %d PONG enviados", pongs)
			return nil
		case protocolo.Error:
			return fmt.Errorf("%w: %s", errCoordinador, m.Mensaje)
		case protocolo.Cargar, protocolo.Iterar:
			texto := fmt.Sprintf("el nodo %s todavía no implementa %s: solo responde PING (smoke test)", n.nombre, m.Tipo)
			n.log.Print(texto)
			if err := cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Error, Mensaje: texto}); err != nil {
				return fmt.Errorf("enviar ERROR: %w", err)
			}
		default:
			return fmt.Errorf("%w: un nodo no espera %s", protocolo.ErrProtocolo, m.Tipo)
		}
	}
}

// dormir espera d o hasta que se cancela ctx; dice si llegó a esperar d.
func dormir(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}
