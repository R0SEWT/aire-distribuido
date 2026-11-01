// Comando benchmark mide el K-Means secuencial contra el concurrente sobre el
// gold, para la tabla de speedup de la PC3. Portado del benchmark del TP.
//
// El protocolo está fijado acá y no se negocia en tiempo de corrida, que es lo
// que evita que dos ejecuciones "según el mismo criterio" den cifras distintas:
//
//   - N repeticiones medidas por configuración, más un calentamiento que se descarta.
//   - Orden aleatorizado por bloques: en cada ronda se barajan las configuraciones,
//     para que el orden no favorezca a ninguna (la caché y el turbo del
//     procesador premian a la que corre segunda).
//   - Se reportan mediana y media recortada simétrica. El speedup con su
//     intervalo es mediaRecortada(seq) / mediaRecortada(conc), con IC bootstrap
//     sobre las dos muestras (ICBootstrapCociente, como en el TP), nunca el
//     promedio de los cocientes por corrida; el cociente de medianas va al lado.
//   - Se mide solo el clustering: la carga del CSV y k-means++ quedan fuera.
//   - Dos experimentos: iteraciones fijas (mismo trabajo) y hasta convergencia
//     con la tolerancia de la receta (mismo resultado).
//   - Con el chunk fijo, el resultado tiene que ser idéntico bit a bit para
//     cualquier cantidad de workers; se comprueba en cada corrida y va al informe.
//
// k, la semilla, la tolerancia y max_iter salen de configs/kmeans.json. Con
// -sintetico N se mide sobre un gold sintético de N filas (34 estaciones) en
// vez del real: sirve para ensayar el comando, y el informe lo marca como
// sintético. Se guardan los tiempos crudos además de los agregados.
//
// Se corre desde la raíz del repo: go run ./cmd/benchmark
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/R0SEWT/aire-distribuido/kmeans"
)

type config struct {
	receta, gold        string
	sintetico           int
	workers, chunks     string
	repes, calent, boot int
	recorte             float64
	semillaOrden        int64
	tamanos             string
	iterFijas           int
	experimentos        string
	salida              string
}

type corrida struct {
	Experimento    string  `json:"experimento"`
	Tamano         int     `json:"tamano"`
	Modo           string  `json:"modo"`
	Workers        int     `json:"workers,omitempty"`
	Chunk          int     `json:"chunk,omitempty"`
	Ronda          int     `json:"ronda"`
	Orden          int     `json:"orden_en_la_ronda"`
	Calentamiento  bool    `json:"calentamiento"`
	MsClustering   float64 `json:"ms_clustering"`
	MsPorIteracion float64 `json:"ms_por_iteracion"`
	Iteraciones    int     `json:"iteraciones"`
	Paro           string  `json:"paro"`
	Inercia        float64 `json:"inercia"`
}

type resumen struct {
	Experimento      string                    `json:"experimento"`
	Tamano           int                       `json:"tamano"`
	Modo             string                    `json:"modo"`
	Workers          int                       `json:"workers,omitempty"`
	Chunk            int                       `json:"chunk,omitempty"`
	Repeticiones     int                       `json:"repeticiones"`
	MedianaMs        float64                   `json:"mediana_ms"`
	MediaRecortadaMs float64                   `json:"media_recortada_ms"`
	DesvMs           float64                   `json:"desviacion_ms"`
	MinMs            float64                   `json:"min_ms"`
	MaxMs            float64                   `json:"max_ms"`
	MsPorIteracion   float64                   `json:"mediana_ms_por_iteracion"`
	Speedup          *kmeans.IntervaloCociente `json:"speedup,omitempty"`
	SpeedupMediana   float64                   `json:"speedup_medianas,omitempty"`
	Eficiencia       float64                   `json:"eficiencia_por_worker,omitempty"`
	Iteraciones      int                       `json:"iteraciones"`
	Inercia          float64                   `json:"inercia"`
}

// determinismo dice si, con un chunk dado, todas las corridas concurrentes
// dieron exactamente la misma inercia sin importar W.
type determinismo struct {
	Experimento       string  `json:"experimento"`
	Tamano            int     `json:"tamano"`
	Chunk             int     `json:"chunk"`
	Workers           []int   `json:"workers"`
	Inercia           float64 `json:"inercia"`
	IdenticoParaTodoW bool    `json:"identico_bit_a_bit_para_todo_w"`
}

type informe struct {
	Metadatos    kmeans.Metadatos  `json:"metadatos"`
	Receta       kmeans.Receta     `json:"receta"`
	Protocolo    map[string]any    `json:"protocolo"`
	Datos        map[string]any    `json:"datos"`
	Centroides   map[string]string `json:"centroides_iniciales_sha256"`
	Corridas     []corrida         `json:"corridas"`
	Resumen      []resumen         `json:"resumen"`
	Determinismo []determinismo    `json:"determinismo"`
	Recursos     kmeans.Recursos   `json:"recursos"`
	DuracionMin  float64           `json:"duracion_total_min"`
}

func main() {
	var c config
	flag.StringVar(&c.receta, "config", "configs/kmeans.json", "receta: k, semilla, tolerancia, max_iter")
	flag.StringVar(&c.gold, "gold", "data/gold/oefa_features.csv", "gold de cmd/cargar")
	flag.IntVar(&c.sintetico, "sintetico", 0, "si > 0, medir sobre un gold sintético de tantas filas en vez del real")
	flag.StringVar(&c.workers, "workers", "1,2,4,8,11", "valores de W")
	flag.StringVar(&c.chunks, "chunk", "1000,10000", "tamaños de chunk, en filas")
	flag.IntVar(&c.repes, "repes", 10, "repeticiones medidas por configuración")
	flag.IntVar(&c.calent, "calentamiento", 1, "rondas de calentamiento descartadas")
	flag.Float64Var(&c.recorte, "recorte", 0.10, "proporción recortada por extremo")
	flag.IntVar(&c.boot, "bootstrap", 2000, "réplicas del bootstrap")
	flag.Int64Var(&c.semillaOrden, "semilla-orden", 7, "semilla del barajado de configuraciones y del bootstrap")
	flag.StringVar(&c.tamanos, "tamanos", "0", "filas de la submuestra sistemática; 0 = gold completo")
	flag.IntVar(&c.iterFijas, "iter-fijas", 10, "iteraciones del experimento de trabajo fijo")
	flag.StringVar(&c.experimentos, "experimentos", "fijas,convergencia", "fijas | convergencia")
	flag.StringVar(&c.salida, "salida", "reports/benchmark_kmeans.json", "informe JSON")
	flag.CommandLine.Parse(argumentos(os.Args))

	if err := correr(c, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func correr(c config, w io.Writer) error {
	inicioTodo := time.Now()
	workers, err := enteros(c.workers, 1)
	if err != nil {
		return fmt.Errorf("-workers: %w", err)
	}
	chunks, err := enteros(c.chunks, 1)
	if err != nil {
		return fmt.Errorf("-chunk: %w", err)
	}
	tamanos, err := enteros(c.tamanos, 0)
	if err != nil {
		return fmt.Errorf("-tamanos: %w", err)
	}
	if c.repes < 2 {
		return fmt.Errorf("-repes: hacen falta al menos 2 repeticiones para la desviación, hay %d", c.repes)
	}
	experimentos := strings.Split(c.experimentos, ",")
	for _, e := range experimentos {
		if e != "fijas" && e != "convergencia" {
			return fmt.Errorf("-experimentos: %q, se esperaba fijas o convergencia", e)
		}
	}
	receta, err := kmeans.LeerReceta(c.receta)
	if err != nil {
		return err
	}

	d, datos, err := cargar(c, receta)
	if err != nil {
		return err
	}

	inf := informe{
		Metadatos:  kmeans.RecolectarMetadatos(),
		Receta:     receta,
		Centroides: map[string]string{},
		Datos:      datos,
		Protocolo: map[string]any{
			"repeticiones_medidas": c.repes, "rondas_calentamiento": c.calent,
			"recorte_por_extremo": c.recorte, "replicas_bootstrap": c.boot,
			"orden":         "aleatorizado por bloques; se guarda el orden de cada ronda",
			"semilla_orden": c.semillaOrden, "workers": workers, "chunks": chunks,
			"iteraciones_fijas": c.iterFijas, "max_iter_convergencia": receta.MaxIter,
			"tolerancia_convergencia": receta.Tolerancia,
			"se_mide":                 "solo el clustering; carga y k-means++ quedan fuera",
			"speedup":                 "media recortada secuencial / media recortada concurrente, IC bootstrap 95 %",
		},
	}

	for _, exper := range experimentos {
		for _, tam := range tamanos {
			// 0 = gold completo. Otro tamaño pasa por Submuestra, que falla si es
			// mayor que N: pedir más filas de las que hay es un error de
			// configuración, no "el completo".
			sub := d
			if tam > 0 {
				if sub, err = kmeans.Submuestra(d, tam); err != nil {
					return fmt.Errorf("tamaño %d: %w", tam, err)
				}
			}
			n := sub.N
			cent, kEf, err := kmeans.KMeansPP(sub, receta.K, receta.Semilla)
			if err != nil {
				return fmt.Errorf("k-means++ con n=%d: %w", n, err)
			}
			inf.Centroides[fmt.Sprintf("%s/%d", exper, n)] = kmeans.HashCentroides(cent)

			op := kmeans.Opciones{K: kEf, MaxIter: c.iterFijas}
			if exper == "convergencia" {
				op = kmeans.Opciones{K: kEf, MaxIter: receta.MaxIter, TolAbs: receta.Tolerancia, TolRel: receta.Tolerancia}
			}

			configs := []corrida{{Experimento: exper, Tamano: n, Modo: "seq"}}
			for _, ch := range chunks {
				for _, p := range workers {
					configs = append(configs, corrida{Experimento: exper, Tamano: n, Modo: "conc", Workers: p, Chunk: ch})
				}
			}

			rng := rand.New(rand.NewSource(c.semillaOrden))
			for ronda := 0; ronda < c.calent+c.repes; ronda++ {
				orden := rng.Perm(len(configs))
				for pos, idx := range orden {
					cr := configs[idx]
					cr.Ronda, cr.Orden = ronda, pos
					cr.Calentamiento = ronda < c.calent

					runtime.GC()
					inicio := time.Now()
					var r *kmeans.Resultado
					if cr.Modo == "seq" {
						r, err = kmeans.Secuencial(sub, cent, op)
					} else {
						r, err = kmeans.Concurrente(sub, cent,
							kmeans.OpcionesConc{Opciones: op, Workers: cr.Workers, Chunk: cr.Chunk})
					}
					if err != nil {
						return err
					}
					cr.MsClustering = float64(time.Since(inicio).Nanoseconds()) / 1e6
					cr.Iteraciones, cr.Paro, cr.Inercia = r.Iteraciones, r.Paro, r.Inercia
					cr.MsPorIteracion = cr.MsClustering / float64(r.Iteraciones+1) // +1: la pasada final
					inf.Corridas = append(inf.Corridas, cr)
				}
				fmt.Fprintf(os.Stderr, "\r%s n=%d ronda %d/%d", exper, n, ronda+1, c.calent+c.repes)
			}
			fmt.Fprintln(os.Stderr)
		}
	}

	if inf.Resumen, err = resumir(inf.Corridas, c.recorte, c.boot, c.semillaOrden); err != nil {
		return err
	}
	inf.Determinismo = comprobarDeterminismo(inf.Corridas)
	inf.Recursos = kmeans.MedirRecursos()
	inf.DuracionMin = time.Since(inicioTodo).Minutes()

	if err := os.MkdirAll(filepath.Dir(c.salida), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(inf, "", " ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(c.salida, append(b, '\n'), 0o644); err != nil {
		return err
	}

	imprimirTabla(w, inf)
	fmt.Fprintf(w, "\n%d corridas en %.1f min → %s\n", len(inf.Corridas), inf.DuracionMin, c.salida)
	for _, det := range inf.Determinismo {
		if !det.IdenticoParaTodoW {
			return fmt.Errorf("%s n=%d chunk=%d: el resultado cambió con W; la reducción en orden de chunk está rota",
				det.Experimento, det.Tamano, det.Chunk)
		}
	}
	return nil
}

// cargar lee el gold real con las features de la receta o arma uno sintético.
func cargar(c config, receta kmeans.Receta) (*kmeans.Datos, map[string]any, error) {
	if c.sintetico > 0 {
		const estaciones = 34
		horas := (c.sintetico + estaciones - 1) / estaciones
		s, err := kmeans.GoldSintetico(kmeans.OpcionesSintetico{Features: receta.Features, Log1p: receta.Log1p,
			Nivel: receta.Nivel, Estaciones: estaciones, Horas: horas, Ruido: 0.25, Semilla: 1})
		if err != nil {
			return nil, nil, err
		}
		fmt.Fprintf(os.Stderr, "gold sintético: %d filas × %d features (no son datos reales)\n", s.Gold.Datos.N, s.Gold.Datos.D)
		return s.Gold.Datos, map[string]any{"sintetico": true, "n": s.Gold.Datos.N, "d": s.Gold.Datos.D,
			"nota": "gold sintético de kmeans.GoldSintetico: sirve para ensayar el comando, no describe el aire de nadie"}, nil
	}
	fmt.Fprintf(os.Stderr, "leyendo %s\n", c.gold)
	g, err := kmeans.LeerGold(c.gold, receta.Features)
	if err != nil {
		return nil, nil, err
	}
	hash, err := kmeans.Sha256Archivo(c.gold)
	if err != nil {
		return nil, nil, err
	}
	return g.Datos, map[string]any{"sintetico": false, "ruta": c.gold, "sha256": hash,
		"n": g.Datos.N, "d": g.Datos.D, "estaciones": len(g.Estaciones())}, nil
}

// clave ordena el resumen: por experimento y tamaño, el secuencial primero y
// después los concurrentes por chunk y workers.
func clave(c corrida) string {
	modo := 1
	if c.Modo == "seq" {
		modo = 0
	}
	return fmt.Sprintf("%s|%09d|%d|%06d|%03d", c.Experimento, c.Tamano, modo, c.Chunk, c.Workers)
}

// resumir agrega las corridas medidas (sin calentamiento) por configuración.
func resumir(corridas []corrida, recorte float64, boot int, semilla int64) ([]resumen, error) {
	tiempos := map[string][]float64{}
	porIter := map[string][]float64{}
	ultima := map[string]corrida{}
	var claves []string
	for _, c := range corridas {
		if c.Calentamiento {
			continue
		}
		k := clave(c)
		if _, visto := tiempos[k]; !visto {
			claves = append(claves, k)
		}
		tiempos[k] = append(tiempos[k], c.MsClustering)
		porIter[k] = append(porIter[k], c.MsPorIteracion)
		ultima[k] = c
	}
	sort.Strings(claves)

	var salida []resumen
	for _, k := range claves {
		ts := tiempos[k]
		c := ultima[k]
		media, err := kmeans.MediaRecortada(ts, recorte)
		if err != nil {
			return nil, err
		}
		mediana, err := kmeans.Mediana(ts)
		if err != nil {
			return nil, err
		}
		desv, err := kmeans.DesviacionEstandar(ts)
		if err != nil {
			return nil, err
		}
		medianaIter, err := kmeans.Mediana(porIter[k])
		if err != nil {
			return nil, err
		}
		orden := append([]float64(nil), ts...)
		sort.Float64s(orden)

		r := resumen{
			Experimento: c.Experimento, Tamano: c.Tamano, Modo: c.Modo, Workers: c.Workers, Chunk: c.Chunk,
			Repeticiones: len(ts), MedianaMs: mediana, MediaRecortadaMs: media, DesvMs: desv,
			MinMs: orden[0], MaxMs: orden[len(orden)-1], MsPorIteracion: medianaIter,
			Iteraciones: c.Iteraciones, Inercia: c.Inercia,
		}
		if c.Modo == "conc" {
			seq := tiempos[clave(corrida{Experimento: c.Experimento, Tamano: c.Tamano, Modo: "seq"})]
			if len(seq) > 0 {
				ic, err := kmeans.ICBootstrapCociente(seq, ts, recorte, boot, semilla)
				if err != nil {
					return nil, err
				}
				medianaSeq, err := kmeans.Mediana(seq)
				if err != nil {
					return nil, err
				}
				r.Speedup = &ic
				r.SpeedupMediana = medianaSeq / mediana
				r.Eficiencia = ic.Punto / float64(c.Workers)
			}
		}
		salida = append(salida, r)
	}
	return salida, nil
}

// comprobarDeterminismo agrupa las corridas concurrentes por experimento,
// tamaño y chunk, y dice si todas dieron la misma inercia bit a bit.
func comprobarDeterminismo(corridas []corrida) []determinismo {
	idx := map[string]int{}
	var out []determinismo
	vistosW := map[string]map[int]bool{}
	for _, c := range corridas {
		if c.Modo != "conc" {
			continue
		}
		k := fmt.Sprintf("%s|%d|%d", c.Experimento, c.Tamano, c.Chunk)
		i, ok := idx[k]
		if !ok {
			i = len(out)
			idx[k] = i
			vistosW[k] = map[int]bool{}
			out = append(out, determinismo{Experimento: c.Experimento, Tamano: c.Tamano, Chunk: c.Chunk,
				Inercia: c.Inercia, IdenticoParaTodoW: true})
		}
		if !vistosW[k][c.Workers] {
			vistosW[k][c.Workers] = true
			out[i].Workers = append(out[i].Workers, c.Workers)
		}
		if math.Float64bits(c.Inercia) != math.Float64bits(out[i].Inercia) {
			out[i].IdenticoParaTodoW = false
		}
	}
	for i := range out {
		sort.Ints(out[i].Workers)
	}
	sort.Slice(out, func(a, b int) bool {
		if out[a].Experimento != out[b].Experimento {
			return out[a].Experimento < out[b].Experimento
		}
		if out[a].Tamano != out[b].Tamano {
			return out[a].Tamano < out[b].Tamano
		}
		return out[a].Chunk < out[b].Chunk
	})
	return out
}

func imprimirTabla(w io.Writer, inf informe) {
	fmt.Fprintf(w, "%s · %s · GOMAXPROCS %d de %d CPUs lógicas\n",
		inf.Metadatos.Maquina, inf.Metadatos.VersionGo, inf.Metadatos.GOMAXPROCS, inf.Metadatos.CPUsLogicas)
	if inf.Datos["sintetico"] == true {
		fmt.Fprintln(w, "DATOS SINTÉTICOS: ensayo del comando, no cifras del informe")
	}
	fmt.Fprintf(w, "\n%-13s %9s %-5s %6s %7s %11s %11s %9s %20s %8s %6s\n",
		"experimento", "n", "modo", "chunk", "workers", "mediana", "recortada", "ms/iter", "speedup [IC 95%]", "sp. med", "efic")
	for _, r := range inf.Resumen {
		sp, spm, efic, ch, wk := "", "", "", "", ""
		if r.Speedup != nil {
			sp = fmt.Sprintf("%.2fx [%.2f, %.2f]", r.Speedup.Punto, r.Speedup.Inferior, r.Speedup.Superior)
			spm = fmt.Sprintf("%.2fx", r.SpeedupMediana)
			efic = fmt.Sprintf("%.0f%%", 100*r.Eficiencia)
		}
		if r.Modo == "conc" {
			ch, wk = strconv.Itoa(r.Chunk), strconv.Itoa(r.Workers)
		}
		fmt.Fprintf(w, "%-13s %9d %-5s %6s %7s %9.1fms %9.1fms %9.2f %20s %8s %6s\n",
			r.Experimento, r.Tamano, r.Modo, ch, wk, r.MedianaMs, r.MediaRecortadaMs, r.MsPorIteracion, sp, spm, efic)
	}
	fmt.Fprintln(w)
	for _, d := range inf.Determinismo {
		estado := "idéntico bit a bit"
		if !d.IdenticoParaTodoW {
			estado = "DISTINTO"
		}
		fmt.Fprintf(w, "%s n=%d chunk=%d: W ∈ %v → %s (inercia %.17g)\n",
			d.Experimento, d.Tamano, d.Chunk, d.Workers, estado, d.Inercia)
	}
}

// enteros lee una lista separada por comas de enteros >= minimo.
func enteros(csv string, minimo int) ([]int, error) {
	var out []int
	for _, campo := range strings.Split(csv, ",") {
		campo = strings.TrimSpace(campo)
		if campo == "" {
			continue
		}
		v, err := strconv.Atoi(campo)
		if err != nil {
			return nil, fmt.Errorf("%q no es un entero", campo)
		}
		if v < minimo {
			return nil, fmt.Errorf("%d es menor que %d", v, minimo)
		}
		out = append(out, v)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("lista vacía")
	}
	return out, nil
}
