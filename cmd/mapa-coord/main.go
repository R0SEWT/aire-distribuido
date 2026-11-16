// Comando mapa-coord es el coordinador del entrenamiento distribuido del mapa
// nacional de PM2.5 y PM10 (docs/specs/2026-11-14-mapa-nacional-design.md, §5):
// local SGD sobre los nodos del laboratorio, que entrenan en su GPU con PyTorch
// (lab/mapa/modelo/nodo_mapa.py) y hablan la versión 2 del protocolo
// (protocolo/doc.go). El coordinador no sabe de capas: los pesos son un vector
// opaco de float32.
//
//  1. Lee -pesos-iniciales (float32 little-endian crudo, el pesos_0.bin de
//     lab/mapa/modelo/), -hiper (JSON que se reenvía tal cual en la TAREA) y el
//     manifiesto (-manifiesto, URL http(s) o ruta) para sacar la lista de días
//     y su huella. Un error en los archivos se ve antes de esperar a nadie.
//  2. Escucha y espera a que se presenten -nodos nodos con un HOLA v2. Ctrl+C
//     mientras espera empieza con los que llegaron; si vence -plazo sin los N,
//     no hay entrenamiento.
//  3. Reparte los días en bloques de 7 barajados con -semilla y repartidos en
//     ronda, en orden de nodo (por nombre), y le manda a cada nodo su TAREA; el
//     nodo baja sus días, los verifica con el manifiesto y responde LISTO con
//     las horas con etiqueta que tiene (-plazo-tarea).
//  4. Corre -rondas rondas PESOS → LOCAL con -pasos pasos cada una y plazo
//     -plazo-ronda, y promedia los pesos ponderados por las muestras de cada
//     nodo, en orden de nodo.
//  5. Después de cada ronda completa reescribe, de forma atómica, -pesos-finales
//     (el punto de control) y -salida (pérdidas ponderadas, ms de cómputo y de
//     transferencia por nodo, ms del promedio y bytes movidos, por ronda).
//
// Si un nodo cae o no responde a tiempo, la ronda se aborta con un error que lo
// nombra, los demás nodos reciben un ERROR con el motivo, -salida queda con
// "estado": "abortada" y -pesos-finales con los pesos de la última ronda
// completa; termina con código 1.
//
//	go run ./cmd/mapa-coord -escuchar :7071 -nodos 4 \
//	    -manifiesto http://10.11.37.73:8000/mapa/manifiesto.json \
//	    -pesos-iniciales lab/mapa/modelo/pesos_0.bin -hiper lab/mapa/modelo/hiper.json \
//	    -rondas 20 -pasos 200 -validacion CA-SB-01,CA-HU-02 \
//	    -salida reports/mapa_entrenamiento.json -pesos-finales pesos_final.bin
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"time"

	"github.com/R0SEWT/aire-distribuido/cluster"
)

func main() {
	var op opciones
	flag.StringVar(&op.escuchar, "escuchar", ":7071", "dirección y puerto donde escucha el coordinador")
	flag.IntVar(&op.nodos, "nodos", 0, "cuántos nodos esperar (obligatorio)")
	flag.StringVar(&op.manifiesto, "manifiesto", "", "manifiesto.json de los datos: URL http(s) o ruta (obligatorio); va tal cual en la TAREA")
	flag.StringVar(&op.pesosIniciales, "pesos-iniciales", "", "vector inicial de pesos, float32 little-endian crudo (obligatorio)")
	flag.IntVar(&op.rondas, "rondas", 10, "R: cuántas veces se promedian los pesos")
	flag.IntVar(&op.pasos, "pasos", 200, "H: pasos de entrenamiento de cada nodo entre dos promedios")
	flag.StringVar(&op.hiper, "hiper", "", "JSON con los hiperparámetros (lote, tasa, semilla…), que se reenvía en la TAREA")
	flag.StringVar(&op.validacion, "validacion", "", "estaciones de validación separadas por comas: no entran en la pérdida de entrenamiento")
	flag.StringVar(&op.salida, "salida", "reports/mapa_entrenamiento.json", "medición por ronda; se reescribe tras cada ronda")
	flag.StringVar(&op.pesosFinales, "pesos-finales", "pesos_final.bin", "pesos globales de la última ronda completa (punto de control)")
	flag.Uint64Var(&op.semilla, "semilla", 42, "semilla del barajado de los bloques de días")
	flag.StringVar(&op.tarea, "tarea", "", "id de la tarea; sin el flag, mapa-AAAAMMDD-HHMMSS")
	flag.DurationVar(&op.plazo, "plazo", 10*time.Minute, "cuánto esperar a que lleguen los nodos")
	flag.DurationVar(&op.plazoHola, "plazo-hola", 10*time.Second, "plazo de cada conexión para mandar su HOLA")
	flag.DurationVar(&op.plazoTarea, "plazo-tarea", 30*time.Minute, "plazo para que todos los nodos bajen y verifiquen sus días")
	flag.DurationVar(&op.plazoRonda, "plazo-ronda", 30*time.Minute, "plazo de cada ronda: un nodo que no responde en ese tiempo aborta el entrenamiento")
	flag.Parse()

	if op.nodos <= 0 || op.manifiesto == "" || op.pesosIniciales == "" {
		fmt.Fprintln(os.Stderr, "uso: mapa-coord -nodos N -manifiesto URL|ruta -pesos-iniciales pesos_0.bin "+
			"[-rondas R] [-pasos H] [-hiper hiper.json] [-validacion E1,E2] [-escuchar :7071] "+
			"[-salida reports/mapa_entrenamiento.json] [-pesos-finales pesos_final.bin]")
		os.Exit(2)
	}
	lg := log.New(os.Stdout, "", log.Ltime)

	ln, err := net.Listen("tcp", op.escuchar)
	if err != nil {
		lg.Fatalf("no puedo escuchar en %s: %v", op.escuchar, err)
	}
	defer ln.Close()
	lg.Printf("escuchando en %s; los nodos pueden llamar a:", ln.Addr())
	for _, d := range cluster.DireccionesLocales(ln.Addr()) {
		lg.Printf("    %s", d)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	// Ctrl+C mientras se esperan los nodos empieza con los que llegaron; desde
	// ahí vuelve a terminar el programa (stop). El punto de control queda.
	if err := correr(ctx, op, ln, lg, os.Stdout, stop); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// opciones son los flags. Los de la spec (§5.4) más la semilla del reparto, el
// id de la tarea y los plazos, como en cmd/distribuido.
type opciones struct {
	escuchar       string
	nodos          int
	manifiesto     string
	pesosIniciales string
	hiper          string
	validacion     string // estaciones separadas por comas
	salida         string
	pesosFinales   string
	rondas, pasos  int
	semilla        uint64
	tarea          string

	plazo, plazoHola, plazoTarea, plazoRonda time.Duration
}
