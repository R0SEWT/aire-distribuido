// Comando smoke es el coordinador de prueba del clúster de la PC4. Escucha,
// espera a que se presenten N nodos (cmd/nodo), mide la ida y vuelta con cada
// uno con protocolo.Latido —a todos a la vez, como la barrera de una iteración—
// y cierra la corrida con FIN. No carga datos ni itera: comprueba que la red,
// los firewalls y el binario de cada equipo están bien antes de distribuir el
// K-Means, y deja medida la latencia que la barrera va a pagar en cada ronda
// (docs/diseno.md, sección 4.5).
//
//	go run ./cmd/smoke -nodos 4
//	go run ./cmd/smoke -nodos 20 -latidos 200 -salida reports/smoke_cluster.json
//
// Mientras espera, Ctrl+C empieza a medir con los nodos que ya llegaron. Sale
// con código 0 solo si llegaron los N y todos respondieron todos los latidos.
// Los nodos vuelven a llamar solos, así que se puede correr varias veces
// seguidas sin tocar los equipos.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"text/tabwriter"
	"time"

	"github.com/R0SEWT/aire-distribuido/cluster"
)

func main() {
	escuchar := flag.String("escuchar", ":7070", "dirección y puerto donde escucha el coordinador")
	nodos := flag.Int("nodos", 0, "cuántos nodos esperar (obligatorio)")
	plazo := flag.Duration("plazo", 10*time.Minute, "cuánto esperar a que lleguen los nodos")
	plazoHola := flag.Duration("plazo-hola", 10*time.Second, "plazo de cada conexión para mandar su HOLA")
	latidos := flag.Int("latidos", 50, "rondas de latidos (un PING a todos los nodos por ronda)")
	plazoLatido := flag.Duration("plazo-latido", 2*time.Second, "plazo de cada ronda de latidos")
	salida := flag.String("salida", "", "si no está vacío, escribe ahí el reporte en JSON")
	flag.Parse()

	if *nodos <= 0 || *latidos <= 0 {
		fmt.Fprintln(os.Stderr, "uso: smoke -nodos N [-escuchar :7070] [-latidos 50] [-salida reporte.json]")
		os.Exit(2)
	}
	lg := log.New(os.Stdout, "", log.Ltime)

	ln, err := net.Listen("tcp", *escuchar)
	if err != nil {
		lg.Fatalf("no puedo escuchar en %s: %v", *escuchar, err)
	}
	lg.Printf("escuchando en %s; los nodos pueden llamar a:", ln.Addr())
	for _, d := range cluster.DireccionesLocales(ln.Addr()) {
		lg.Printf("    %s", d)
	}
	lg.Printf("espero %d nodos durante %v (Ctrl+C empieza con los que hayan llegado)", *nodos, *plazo)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	ctx, cancel := context.WithTimeoutCause(ctx, *plazo, errors.New("venció el plazo"))
	miembros := esperarNodos(ctx, ln, *nodos, *plazoHola, lg)
	cancel()
	stop() // desde aquí, Ctrl+C vuelve a terminar el programa
	ln.Close()

	if len(miembros) == 0 {
		lg.Fatal("no llegó ningún nodo")
	}

	lg.Printf("mido %d rondas de latidos con %d nodos", *latidos, len(miembros))
	inicio := time.Now()
	barreras := medir(miembros, *latidos, *plazoLatido)
	duracion := time.Since(inicio)
	cerrar(miembros, 2*time.Second)

	r := armarReporte(miembros, barreras, *nodos, *latidos, duracion)
	imprimir(os.Stdout, r)
	if *salida != "" {
		if err := escribirJSON(*salida, r); err != nil {
			lg.Fatalf("no pude escribir %s: %v", *salida, err)
		}
		lg.Printf("reporte en %s", *salida)
	}
	if !r.OK {
		os.Exit(1)
	}
}

// reporte es lo que deja una corrida del smoke test, en milisegundos.
type reporte struct {
	Fecha      string        `json:"fecha"`
	Esperados  int           `json:"nodos_esperados"`
	Conectados int           `json:"nodos_conectados"`
	Rondas     int           `json:"rondas"`
	DuracionS  float64       `json:"duracion_s"`
	OK         bool          `json:"ok"`
	Barrera    estadistica   `json:"barrera"`
	Nodos      []reporteNodo `json:"nodos"`
}

type estadistica struct {
	N   int     `json:"n"`
	P50 float64 `json:"p50_ms"`
	P95 float64 `json:"p95_ms"`
	Max float64 `json:"max_ms"`
}

type reporteNodo struct {
	Nodo      string      `json:"nodo"`
	Direccion string      `json:"direccion"`
	Workers   int         `json:"workers"`
	LlegadaS  float64     `json:"llegada_s"`
	Latido    estadistica `json:"latido"`
	Muestras  []float64   `json:"muestras_ms"`
	Error     string      `json:"error,omitempty"`
}

func armarReporte(miembros []*miembro, barreras []time.Duration, esperados, rondas int, duracion time.Duration) reporte {
	r := reporte{
		Fecha:      time.Now().Format(time.RFC3339),
		Esperados:  esperados,
		Conectados: len(miembros),
		Rondas:     rondas,
		DuracionS:  duracion.Seconds(),
		OK:         len(miembros) == esperados,
		Barrera:    estadisticaDe(barreras),
	}
	for _, m := range miembros {
		n := reporteNodo{
			Nodo:      m.Enlace.Nodo,
			Direccion: m.Direccion,
			Workers:   m.Enlace.Workers,
			LlegadaS:  m.Llegada.Seconds(),
			Latido:    estadisticaDe(m.latidos),
			Muestras:  make([]float64, len(m.latidos)),
		}
		for i, d := range m.latidos {
			n.Muestras[i] = ms(d)
		}
		if m.err != nil {
			n.Error = m.err.Error()
			r.OK = false
		}
		r.Nodos = append(r.Nodos, n)
	}
	return r
}

func estadisticaDe(xs []time.Duration) estadistica {
	return estadistica{N: len(xs), P50: ms(cluster.Percentil(xs, 50)), P95: ms(cluster.Percentil(xs, 95)), Max: ms(cluster.Percentil(xs, 100))}
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

func imprimir(w io.Writer, r reporte) {
	fmt.Fprintln(w)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "nodo\tdirección\tworkers\tllegó\tlatidos\tp50\tp95\tmáx\t")
	for _, n := range r.Nodos {
		fmt.Fprintf(tw, "%s\t%s\t%d\t%.1fs\t%d/%d\t%.2f ms\t%.2f ms\t%.2f ms\t%s\n",
			n.Nodo, n.Direccion, n.Workers, n.LlegadaS, n.Latido.N, r.Rondas,
			n.Latido.P50, n.Latido.P95, n.Latido.Max, n.Error)
	}
	tw.Flush()
	fmt.Fprintf(w, "\nbarrera con los %d nodos a la vez: p50 %.2f ms · p95 %.2f ms · máx %.2f ms (%d de %d rondas completas)\n",
		r.Conectados, r.Barrera.P50, r.Barrera.P95, r.Barrera.Max, r.Barrera.N, r.Rondas)
	switch {
	case r.OK:
		fmt.Fprintf(w, "SMOKE TEST OK: %d de %d nodos, todos los latidos respondidos\n", r.Conectados, r.Esperados)
	case r.Conectados < r.Esperados:
		fmt.Fprintf(w, "SMOKE TEST INCOMPLETO: llegaron %d de %d nodos\n", r.Conectados, r.Esperados)
	default:
		fmt.Fprintln(w, "SMOKE TEST CON FALLAS: hay nodos que dejaron de responder (ver la última columna)")
	}
}

func escribirJSON(ruta string, r reporte) error {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ruta, append(b, '\n'), 0o644)
}
