// Comando nodo es un nodo de cómputo del K-Means distribuido (PC4). Abre la
// conexión hacia el coordinador, se presenta con HOLA y atiende lo que le pide
// (docs/diseno.md, sección 4.2: el nodo llama, el coordinador nunca).
//
// Por ahora es la mitad de nodo del smoke test del clúster: responde cada PING
// con un PONG y termina la corrida con el FIN. A un CARGAR o un ITERAR responde
// con un ERROR legible hasta que la PC4 los implemente.
//
// Si el coordinador todavía no escucha, se reinicia o cierra la corrida, el
// nodo vuelve a llamar solo: se lanza una vez en cada equipo y sirve para varias
// corridas seguidas. Se detiene con Ctrl+C o, con -una, tras la primera corrida.
//
//	go run ./cmd/nodo -coordinador 10.11.37.73:7070
//
// Para los Windows del laboratorio se compila desde Linux con lab/armar-usb.sh,
// que hace GOOS=windows GOARCH=amd64 go build ./cmd/nodo. No necesita
// instalación ni permisos de administrador: solo abre conexiones de salida.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime"
	"time"
)

func main() {
	nombre, _ := os.Hostname()
	coordinador := flag.String("coordinador", "", "dirección del coordinador, IP:puerto (obligatoria)")
	flag.StringVar(&nombre, "nombre", nombre, "nombre con el que el nodo se presenta en el HOLA")
	workers := flag.Int("workers", runtime.NumCPU(), "núcleos que el nodo declara en el HOLA")
	espera := flag.Duration("espera", 2*time.Second, "pausa entre intentos de conexión")
	una := flag.Bool("una", false, "terminar después de la primera corrida en vez de volver a llamar")
	flag.Parse()

	if *coordinador == "" || nombre == "" {
		fmt.Fprintln(os.Stderr, "uso: nodo -coordinador IP:puerto [-nombre N] [-workers W] [-una]")
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	n := &nodo{
		nombre:  nombre,
		workers: *workers,
		log:     log.New(os.Stdout, "["+nombre+"] ", log.Ltime),
	}
	marcador := &net.Dialer{Timeout: 3 * time.Second}
	llamar := func(ctx context.Context) (net.Conn, error) {
		return marcador.DialContext(ctx, "tcp", *coordinador)
	}

	n.log.Printf("nodo %s (%s/%s, %d workers) llamando a %s; Ctrl+C para salir",
		nombre, runtime.GOOS, runtime.GOARCH, *workers, *coordinador)
	err := n.correr(ctx, llamar, *espera, *una)
	switch {
	case ctx.Err() != nil:
		n.log.Printf("detenido con Ctrl+C")
	case err != nil:
		n.log.Printf("fin: %v", err)
		os.Exit(1)
	}
}
