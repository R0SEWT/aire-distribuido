// Comando distribuido es el coordinador del K-Means distribuido de la PC4
// (docs/diseno.md, sección 4): corre el Lloyd de la receta sobre un clúster de
// nodos (cmd/nodo), cada uno con un rango contiguo del gold, con una barrera de
// red por iteración.
//
//  1. Lee configs/kmeans.json, el gold (para su huella y su número de filas),
//     reports/estandarizacion.json y los centroides iniciales de la semilla de
//     la receta (o de -semilla) de reports/centroides_iniciales.json.
//  2. Escucha y espera a que se presenten -nodos nodos (HOLA). Ctrl+C mientras
//     espera empieza con los que llegaron; si vence -plazo sin los N, no corre.
//  3. Reparte la tabla en rangos contiguos iguales, en orden de nodo (por
//     nombre), y se los manda a todos a la vez (CARGAR → LISTO). Con -replicas
//     r la tabla es virtual: r × N filas, la fila i es la i mod N del gold.
//     Un -replicas que le dejaría a algún nodo más filas que su tope
//     (-max-filas, el mismo de cmd/nodo) se rechaza antes de esperar, con un
//     error que dice el máximo.
//  4. Corre Lloyd: cada iteración es una ronda ITERAR → PARCIAL con plazo
//     -plazo-ronda, la fusión en orden de nodo y kmeans.CerrarIteracion, hasta
//     la tolerancia o max_iter de la receta; luego una pasada final.
//  5. Escribe reports/kmeans_distribuido.json: centroides, inercia,
//     iteraciones, nodos y rangos, el tiempo de cada ronda separado del de la
//     fusión y la diferencia con reports/kmeans.json (cmd/entrenar, misma
//     semilla).
//
// Si un nodo cae o no responde a tiempo, la corrida se aborta con un error que
// lo nombra, los demás nodos reciben un ERROR con el motivo, y no se escribe
// nada: termina con código 1. Si el modelo se aparta de la referencia más de
// -tolerancia, se escribe igual (es un modelo completo) y termina con código 1.
//
//	go run ./cmd/distribuido -nodos 3
//	go run ./cmd/distribuido -nodos 20 -replicas 10 -salida reports/kmeans_distribuido_lab_20.json
//
// Las semillas: este comando corre una (la de la receta o -semilla). Las diez
// de la receta (semillas: 10) se pueden correr con -semilla 42 … 51 y quedarse
// con la de menor inercia, como hace cmd/entrenar; hacerlo en una sola corrida
// es repetir cluster.Lloyd sobre las mismas conexiones ya cargadas, con la
// numeración de rondas que sigue (Corrida.UltimaRonda + 1).
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
	flag.StringVar(&op.receta, "config", "configs/kmeans.json", "receta congelada por la calibración")
	flag.StringVar(&op.gold, "gold", "data/gold/oefa_features.csv", "gold de cmd/cargar: su huella y su número de filas")
	flag.StringVar(&op.estandarizacion, "estandarizacion", "reports/estandarizacion.json", "medias y desviaciones de cmd/cargar")
	flag.StringVar(&op.inicios, "centroides", "reports/centroides_iniciales.json", "centroides iniciales de cada semilla (cmd/entrenar)")
	flag.StringVar(&op.referencia, "referencia", "reports/kmeans.json", "modelo de cmd/entrenar con el que se compara; vacío = no se compara")
	flag.StringVar(&op.salida, "salida", "reports/kmeans_distribuido.json", "modelo distribuido, tiempos y comparación")
	flag.StringVar(&op.escuchar, "escuchar", ":7070", "dirección y puerto donde escucha el coordinador")
	flag.IntVar(&op.nodos, "nodos", 0, "cuántos nodos esperar (obligatorio)")
	flag.IntVar(&op.replicas, "replicas", 1, "réplicas virtuales del gold: la tabla tiene replicas × N filas")
	flag.IntVar(&op.maxFilas, "max-filas", cluster.MaxFilasPorOmision,
		"tope de filas por nodo, el -max-filas con que se lanzaron los nodos: un -replicas que lo pasa no se reparte")
	flag.Int64Var(&op.semilla, "semilla", 0, "semilla cuyos centroides iniciales se usan; sin el flag, la de la receta")
	flag.DurationVar(&op.plazo, "plazo", 10*time.Minute, "cuánto esperar a que lleguen los nodos")
	flag.DurationVar(&op.plazoHola, "plazo-hola", 10*time.Second, "plazo de cada conexión para mandar su HOLA")
	flag.DurationVar(&op.plazoCarga, "plazo-carga", 2*time.Minute, "plazo para que todos los nodos carguen su rango")
	flag.DurationVar(&op.plazoRonda, "plazo-ronda", 10*time.Second, "plazo de cada ronda: un nodo que no responde en ese tiempo aborta la corrida")
	flag.Float64Var(&op.tolerancia, "tolerancia", toleranciaPorOmision,
		"diferencia máxima con -referencia: absoluta en los centroides, relativa en la inercia")
	flag.Parse()
	flag.Visit(func(f *flag.Flag) { op.conSemilla = op.conSemilla || f.Name == "semilla" })

	if op.nodos <= 0 || op.maxFilas <= 0 {
		fmt.Fprintln(os.Stderr, "uso: distribuido -nodos N [-replicas R] [-max-filas F] [-semilla S] [-escuchar :7070] [-salida reports/kmeans_distribuido.json]")
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
	// ahí vuelve a terminar el programa (stop), sin escribir nada.
	if err := correr(ctx, op, ln, lg, os.Stdout, stop); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
