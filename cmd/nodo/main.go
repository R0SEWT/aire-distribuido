// Comando nodo es un nodo de cómputo del K-Means distribuido (PC4). Abre la
// conexión hacia el coordinador, se presenta con HOLA y atiende lo que le pide
// (docs/diseno.md, sección 4.2: el nodo llama, el coordinador nunca):
//
//   - CARGAR {rango, huella}: comprueba que el sha256 de su gold es la huella,
//     lee las filas [desde, hasta) y responde LISTO; si falta el gold, es otro
//     o el rango tiene más de -max-filas filas, responde un ERROR legible;
//   - ITERAR {iter, centroides}: calcula sumas, conteos e inercia de su rango
//     con -workers goroutines en chunks de -chunk filas y responde PARCIAL;
//   - FIN: termina la corrida y libera la partición;
//   - PING: responde PONG (el smoke test, cmd/smoke).
//
// Si el coordinador todavía no escucha, se reinicia o cierra la corrida, el
// nodo vuelve a llamar solo: se lanza una vez en cada equipo y sirve para varias
// corridas seguidas. Se detiene con Ctrl+C o, con -una, tras la primera corrida.
//
//	go run ./cmd/nodo -coordinador 10.11.37.73:7070 -gold data/gold/oefa_features.csv
//
// Sin -gold usa el oefa_features.csv que está junto al ejecutable, que es como
// queda en los Windows del laboratorio: lab/armar-usb.sh compila nodo.exe con
// GOOS=windows GOARCH=amd64 y copia el gold al lado. No necesita instalación ni
// permisos de administrador: solo abre conexiones de salida.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"time"

	"github.com/R0SEWT/aire-distribuido/cluster"
)

func main() {
	nombre := nombrePorOmision()
	coordinador := flag.String("coordinador", "", "dirección del coordinador, IP:puerto (obligatoria)")
	flag.StringVar(&nombre, "nombre", nombre, "nombre con el que el nodo se presenta en el HOLA; tiene que ser único en el clúster")
	// GOMAXPROCS y no NumCPU: desde Go 1.25 respeta el límite de CPU de un
	// contenedor, como workers = 0 en configs/kmeans.json.
	workers := flag.Int("workers", runtime.GOMAXPROCS(0), "núcleos que el nodo declara en el HOLA y usa para calcular")
	chunk := flag.Int("chunk", cluster.ChunkPorOmision,
		"filas por chunk; con el chunk de configs/kmeans.json, un solo nodo da el modelo de cmd/entrenar bit a bit")
	maxFilas := flag.Int("max-filas", cluster.MaxFilasPorOmision,
		"tope de filas de un CARGAR (8 bytes por fila y feature en memoria); un rango más grande se responde con ERROR")
	gold := flag.String("gold", goldJuntoAlEjecutable(), "gold del que el nodo lee su rango")
	espera := flag.Duration("espera", 2*time.Second, "pausa entre intentos de conexión")
	una := flag.Bool("una", false, "terminar después de la primera corrida en vez de volver a llamar")
	flag.Parse()

	if *coordinador == "" || nombre == "" || *workers <= 0 || *chunk <= 0 || *maxFilas <= 0 {
		fmt.Fprintln(os.Stderr, "uso: nodo -coordinador IP:puerto [-nombre N] [-workers W] [-chunk C] [-max-filas F] [-gold oefa_features.csv] [-una]")
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	n := &cluster.Nodo{
		Nombre:   nombre,
		Workers:  *workers,
		Chunk:    *chunk,
		MaxFilas: *maxFilas,
		Gold:     *gold,
		Log:      log.New(os.Stdout, "["+nombre+"] ", log.Ltime),
	}
	marcador := &net.Dialer{Timeout: 3 * time.Second}
	llamar := func(ctx context.Context) (net.Conn, error) {
		return marcador.DialContext(ctx, "tcp", *coordinador)
	}

	n.Log.Printf("nodo %s (%s/%s, %d workers, chunk %d, hasta %d filas) llamando a %s; Ctrl+C para salir",
		nombre, runtime.GOOS, runtime.GOARCH, *workers, *chunk, *maxFilas, *coordinador)
	// La huella al arrancar sirve para comparar a ojo con la que imprime el
	// coordinador; si el gold falta, el smoke test igual funciona.
	if h, err := cluster.Huella(*gold); err != nil {
		n.Log.Printf("AVISO: no puedo leer el gold %s (%v); el smoke test funciona, pero CARGAR va a responder ERROR", *gold, err)
	} else {
		n.Log.Printf("gold %s, huella %.19s…", *gold, h)
	}
	err := n.Correr(ctx, llamar, *espera, *una)
	switch {
	case ctx.Err() != nil:
		n.Log.Printf("detenido con Ctrl+C")
	case err != nil:
		n.Log.Printf("fin: %v", err)
		os.Exit(1)
	}
}

// goldJuntoAlEjecutable es oefa_features.csv en la carpeta de nodo.exe; si no
// se puede saber dónde está el ejecutable, en la carpeta actual.
func goldJuntoAlEjecutable() string {
	const archivo = "oefa_features.csv"
	exe, err := os.Executable()
	if err != nil {
		return archivo
	}
	return filepath.Join(filepath.Dir(exe), archivo)
}

// nombrePorOmision es el nombre del equipo y el PID del proceso (H45-022-4312).
// Solo con el nombre del equipo, dos nodos lanzados en la misma máquina sin
// -nombre se presentarían igual, y el coordinador, que identifica a los nodos
// por su nombre, tomaría al segundo por una reconexión del primero: se
// desalojarían uno al otro cada vez que el desplazado vuelve a llamar. El
// protocolo no cambia: el nombre es un texto libre del HOLA.
func nombrePorOmision() string {
	equipo, err := os.Hostname()
	if err != nil || equipo == "" {
		equipo = "nodo"
	}
	return fmt.Sprintf("%s-%d", equipo, os.Getpid())
}
