// Comando cargar es la parte A del Trabajo Final: carga el CSV largo de OEFA
// con el cargador concurrente del paquete datos y escribe el silver, el gold y
// los reportes de limpieza y estandarización. Con -bench repite la carga para
// varios W y escribe reports/benchmark_cargador.json.
//
//	go run ./cmd/cargar -entrada data/bronze/oefa.csv
//	go run ./cmd/cargar -entrada data/bronze/oefa.csv -workers 1,2,4,8,11 -bench 5
//
// Al final imprime un resumen legible: es lo que se captura para el informe.
package main

import (
	"cmp"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/R0SEWT/aire-distribuido/datos"
	"github.com/R0SEWT/aire-distribuido/kmeans"
)

func main() {
	if err := ejecutar(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "cargar:", err)
		os.Exit(1)
	}
}

type rutas struct {
	config, kmeans, entrada, silver, gold, reports string
}

func ejecutar(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("cargar", flag.ContinueOnError)
	var r rutas
	fs.StringVar(&r.config, "config", "configs/limpieza.json", "config de limpieza: formato del CSV y umbrales de las reglas")
	fs.StringVar(&r.kmeans, "kmeans", "configs/kmeans.json", "receta del K-Means (features, log1p, nivel, chunk); si no existe, la provisional")
	fs.StringVar(&r.entrada, "entrada", "", "CSV largo de OEFA (obligatorio)")
	fs.StringVar(&r.silver, "silver", "data/silver/oefa_ancho.csv", "salida: tabla ancha limpia")
	fs.StringVar(&r.gold, "gold", "data/gold/oefa_features.csv", "salida: features estandarizadas para el K-Means")
	fs.StringVar(&r.reports, "reports", "reports", "directorio de los reportes JSON")
	workers := fs.String("workers", "0", "goroutines (0 = runtime.NumCPU()); con -bench, lista separada por comas: 1,2,4,8")
	bloque := fs.String("bloque", "64MB", "tamaño de bloque: bytes, o con sufijo KB, MB o GB (potencias de 1024)")
	chunk := fs.Int("chunk", 0, "filas por chunk de la estandarización (0 = el de la receta, o 10000)")
	bench := fs.Int("bench", 0, "repeticiones por cada W de -workers; escribe reports/benchmark_cargador.json")
	if err := fs.Parse(args); err != nil {
		return err
	}
	switch {
	case fs.NArg() > 0:
		return fmt.Errorf("argumentos de más: %q (todo va por flags)", fs.Args())
	case r.entrada == "":
		return errors.New("falta -entrada: la ruta del CSV de OEFA")
	case *bench < 0:
		return fmt.Errorf("-bench %d: tiene que ser 0 o positivo", *bench)
	case *chunk < 0:
		return fmt.Errorf("-chunk %d: tiene que ser 0 o positivo", *chunk)
	}
	ws, err := parsearWorkers(*workers)
	if err != nil {
		return err
	}
	if *bench == 0 && len(ws) != 1 {
		return fmt.Errorf("-workers %s: una lista solo tiene sentido con -bench", *workers)
	}
	tamBloque, err := parsearTamano(*bloque)
	if err != nil {
		return err
	}
	cfg, err := datos.LeerConfigLimpieza(r.config)
	if err != nil {
		return err
	}
	rec, provisional, err := datos.LeerReceta(r.kmeans)
	if err != nil {
		return err
	}
	// Procedencia: los mismos metadatos que cmd/entrenar y la versión exacta
	// de cada config, leída junto con ella.
	proc := datos.Procedencia{Metadatos: kmeans.RecolectarMetadatos(), Configs: map[string]datos.VersionArchivo{}}
	for nombre, ruta := range map[string]string{"limpieza": r.config, "kmeans": r.kmeans} {
		if proc.Configs[nombre], err = datos.VersionDeArchivo(ruta); err != nil {
			return err
		}
	}

	op := datos.Opciones{Bloque: tamBloque, Chunk: *chunk}
	var res *datos.Resultado
	var bm *datos.Benchmark
	if *bench > 0 {
		bm, res, err = datos.MedirCarga(r.entrada, cfg, rec, ws, *bench, op)
	} else {
		op.Workers = ws[0]
		res, err = datos.Cargar(r.entrada, cfg, rec, op)
	}
	if err != nil {
		return err
	}
	// El sha256 de la entrada se calcula en una pasada aparte, después de la
	// carga: dentro de ella competiría por CPU y disco con los workers y
	// distorsionaría la duración y el benchmark. Con -bench, una sola vez.
	if proc.Sha256Entrada, err = kmeans.Sha256Archivo(r.entrada); err != nil {
		return err
	}
	res.Anotar(proc)
	if bm != nil {
		bm.Anotar(proc)
	}

	escritos, err := escribirSalidas(r, res, bm)
	if err != nil {
		return err
	}
	imprimirResumen(out, r, provisional, res, bm, escritos)
	return nil
}

func escribirSalidas(r rutas, res *datos.Resultado, bm *datos.Benchmark) ([]string, error) {
	type salida struct {
		ruta string
		f    func() error
	}
	limpieza := filepath.Join(r.reports, "limpieza.json")
	estand := filepath.Join(r.reports, "estandarizacion.json")
	// Primero los CSV, cronometrados: su tiempo va a fases_s.salida de
	// limpieza.json, que se escribe después.
	inicio := time.Now()
	csvs := []salida{
		{r.silver, func() error { return datos.EscribirArchivo(r.silver, res.Ancha.EscribirSilver) }},
		{r.gold, func() error { return datos.EscribirArchivo(r.gold, res.Dataset.EscribirGold) }},
	}
	jsons := []salida{
		{limpieza, func() error { return datos.GuardarJSON(limpieza, res.Limpieza) }},
		{estand, func() error { return datos.GuardarJSON(estand, res.Estandarizacion) }},
	}
	if bm != nil {
		ruta := filepath.Join(r.reports, "benchmark_cargador.json")
		jsons = append(jsons, salida{ruta, func() error { return datos.GuardarJSON(ruta, bm) }})
	}
	var escritos []string
	for i, s := range append(csvs, jsons...) {
		if i == len(csvs) {
			res.Limpieza.FasesS["salida"] = time.Since(inicio).Seconds()
		}
		if err := s.f(); err != nil {
			return nil, fmt.Errorf("escribir %s: %w", s.ruta, err)
		}
		escritos = append(escritos, s.ruta)
	}
	return escritos, nil
}

// parsearWorkers lee "4" o "1,2,4,8". 0 significa runtime.NumCPU().
func parsearWorkers(s string) ([]int, error) {
	var ws []int
	for _, parte := range strings.Split(s, ",") {
		w, err := strconv.Atoi(strings.TrimSpace(parte))
		if err != nil || w < 0 {
			return nil, fmt.Errorf("-workers %q: se espera un entero ≥ 0 o una lista separada por comas", s)
		}
		ws = append(ws, w)
	}
	return ws, nil
}

// parsearTamano lee "64MB", "512KB", "1GB" o un número de bytes. Los sufijos
// son potencias de 1024 (MB = MiB), que es lo que se entiende por «bloque de 64 MB».
func parsearTamano(s string) (int64, error) {
	t := strings.ToUpper(strings.TrimSpace(s))
	mult := int64(1)
	for _, suf := range []struct {
		sufijos []string
		mult    int64
	}{
		{[]string{"GIB", "GB", "G"}, 1 << 30},
		{[]string{"MIB", "MB", "M"}, 1 << 20},
		{[]string{"KIB", "KB", "K"}, 1 << 10},
		{[]string{"B"}, 1},
	} {
		if i := slices.IndexFunc(suf.sufijos, func(x string) bool { return strings.HasSuffix(t, x) }); i >= 0 {
			t, mult = strings.TrimSpace(strings.TrimSuffix(t, suf.sufijos[i])), suf.mult
			break
		}
	}
	n, err := strconv.ParseInt(t, 10, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("-bloque %q: se espera un tamaño positivo como 64MB, 512KB o 1048576", s)
	}
	return n * mult, nil
}

// miles escribe un entero con separador de miles (coma, como se usa en Perú).
func miles(n int64) string {
	s := strconv.FormatInt(n, 10)
	signo := ""
	if n < 0 {
		signo, s = "-", s[1:]
	}
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	return signo + b.String()
}

func imprimirResumen(out io.Writer, r rutas, provisional bool, res *datos.Resultado, bm *datos.Benchmark, escritos []string) {
	l, e := res.Limpieza, res.Estandarizacion
	receta := r.kmeans
	if provisional {
		receta += " no existe: receta provisional"
	}
	fmt.Fprintf(out, "Cargador concurrente OEFA (parte A)\n\n")
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "entrada\t%s (%s bytes, delimitador %q, BOM %s)\n", l.Entrada, miles(l.Bytes), l.Delimitador, siNo(l.BOM))
	fmt.Fprintf(tw, "sha256\t%s\n", l.Sha256)
	fmt.Fprintf(tw, "config\t%s (sha256 %s)\n", r.config, corto(l.Configs["limpieza"].Sha256))
	fmt.Fprintf(tw, "receta\t%s\n", receta)
	if m := l.Metadatos; m != nil {
		sucio := ""
		if m.ArbolSucio {
			sucio = ", árbol sucio"
		}
		fmt.Fprintf(tw, "corrida\tcommit %s%s · %s · %s · %d CPU\n", corto(m.Commit), sucio, m.Maquina, m.VersionGo, m.CPUsLogicas)
	}
	fmt.Fprintf(tw, "paralelismo\t%d workers, %d bloques de %s bytes, chunk de %s filas\n",
		l.Workers, l.Bloques, miles(l.BloqueBytes), miles(int64(l.Chunk)))
	f := l.FasesS
	fmt.Fprintf(tw, "duración\t%.3f s: lectura y parseo %.3f s (fusión serial %.3f s, solapada), horas %.3f s, "+
		"pivot %.3f s, tabla %.3f s, gold %.3f s\n", l.DuracionS, f["lectura_parseo"], f["fusion"], f["horas"],
		f["pivot"], f["tabla"], f["gold_estandarizacion"])
	fmt.Fprintf(tw, "hora\tfecha_hora = %s del intervalo horario\n", l.ConvencionHora)
	tw.Flush()

	fl := l.FilasLargas
	fmt.Fprintf(out, "\nFilas largas\n")
	filas := [][2]string{
		{"leídas", miles(fl.Leidas)},
		{"procesadas", miles(fl.Procesadas)},
		{"descartadas", miles(fl.Descartadas)},
	}
	for _, regla := range datos.Reglas() {
		filas = append(filas, [2]string{"  " + regla, miles(fl.DescartadasPorRegla[regla])})
	}
	ancho := 0
	for _, f := range filas {
		ancho = max(ancho, len([]rune(f[0])))
	}
	for _, f := range filas {
		fmt.Fprintf(out, "  %s%s %14s\n", f[0], strings.Repeat(" ", ancho-len([]rune(f[0]))), f[1])
	}
	fmt.Fprintf(out, "  procesadas con hora explícita %s · con hora inferida por posición %s\n",
		miles(fl.HoraExplicita), miles(fl.HoraInferida))
	fmt.Fprintf(out, "  hora por posición en el bloque físico (día con filas vacías): %s filas largas\n", miles(fl.HoraPorBloqueFisico))
	fmt.Fprintf(out, "  marcadas (se conservan): duplicado_valor_distinto %s · unidad_convertida %s · hora_no_en_punto %s celdas"+
		" · ubicacion_corregida %s filas\n",
		miles(fl.MarcadasPorRegla["duplicado_valor_distinto"]), miles(fl.MarcadasPorRegla["unidad_convertida"]),
		miles(fl.MarcadasPorRegla["hora_no_en_punto"]), miles(fl.MarcadasPorRegla["ubicacion_corregida"]))
	if len(fl.UnidadConvertidaPorParametro) > 0 {
		fmt.Fprintf(out, "  convertidas por parámetro: %s\n", conteos(fl.UnidadConvertidaPorParametro))
	}
	if len(fl.FueraDeRangoPorParametro) > 0 {
		fmt.Fprintf(out, "  fuera de rango por parámetro: %s\n", conteos(fl.FueraDeRangoPorParametro))
	}
	if len(fl.CeroComoFaltantePorParametro) > 0 {
		fmt.Fprintf(out, "  cero como faltante por parámetro: %s\n", conteos(fl.CeroComoFaltantePorParametro))
	}

	fa := l.FilasAnchas
	fmt.Fprintf(out, "\nFilas anchas (estación × fecha-hora)\n")
	fmt.Fprintf(out, "  total %s · completas para el gold %s · pm2_5 > pm10 %s · con hora inferida %s\n",
		miles(int64(fa.Total)), miles(int64(fa.CompletasGold)), miles(int64(fa.Pm25MayorPm10)), miles(int64(fa.HoraInferida)))
	fmt.Fprintf(out, "  parámetros presentes: %s\n", strings.Join(l.ParametrosPresentes, ", "))
	feats := make([]string, len(e.Features))
	for i, f := range e.Features {
		feats[i] = f
		if slices.Contains(e.Log1p, f) {
			feats[i] += " (log1p)"
		}
	}
	fmt.Fprintf(out, "  gold: %s · nivel %s · n %s\n", strings.Join(feats, ", "), e.Nivel, miles(int64(e.N)))

	fmt.Fprintf(out, "\nEstaciones (%d)\n", len(l.Estaciones))
	tw = tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "  estación\tleídas\tprocesadas\tanchas\tgold\tdesde\thasta\n")
	nombres := make([]string, 0, len(l.Estaciones))
	for n := range l.Estaciones {
		nombres = append(nombres, n)
	}
	slices.Sort(nombres)
	for _, n := range nombres {
		s := l.Estaciones[n]
		fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\t%s\t%s\t%s\n", n, miles(s.LargasLeidas), miles(s.LargasProcesadas),
			miles(int64(s.Anchas)), miles(int64(s.CompletasGold)), guion(s.Desde), guion(s.Hasta))
	}
	tw.Flush()

	v := l.Vistos
	if len(v.ParametrosNoMapeados) > 0 {
		fmt.Fprintf(out, "\nParámetros sin mapeo (se descartan): %s\n", conteos(v.ParametrosNoMapeados))
	}
	if len(v.Periodos) > 0 {
		fmt.Fprintf(out, "Periodos vistos: %s\n", conteos(v.Periodos))
	}
	if len(v.ValoresNoNumericos) > 0 {
		fmt.Fprintf(out, "Valores no numéricos (se descartan, no se imputan): %s\n", conteos(v.ValoresNoNumericos))
	}
	if len(l.Avisos) > 0 {
		fmt.Fprintf(out, "\nAvisos\n")
		for _, a := range l.Avisos {
			fmt.Fprintf(out, "  - %s\n", a)
		}
	}

	if bm != nil {
		fmt.Fprintf(out, "\nBenchmark (%d repeticiones intercaladas, mediana; base W=%d; %d CPU, %s, %s)\n",
			bm.Repeticiones, bm.BaseWorkers, bm.CPUs, bm.Go, bm.Host)
		tw = tabwriter.NewWriter(out, 0, 0, 2, ' ', tabwriter.AlignRight)
		fmt.Fprintf(tw, "  W\tmediana_s\tspeedup\trecortada_s\tspeedup [IC 95 %%]\ttiempos_s\t\n")
		for _, m := range bm.Mediciones {
			ts := make([]string, len(m.TiemposS))
			for i, t := range m.TiemposS {
				ts[i] = strconv.FormatFloat(t, 'f', 3, 64)
			}
			ic := "-"
			if m.SpeedupIC != nil {
				ic = fmt.Sprintf("%.2f [%.2f, %.2f]", m.SpeedupIC.Punto, m.SpeedupIC.Inferior, m.SpeedupIC.Superior)
			}
			fmt.Fprintf(tw, "  %d\t%.3f\t%.2f\t%.3f\t%s\t%s\t\n", m.Workers, m.MedianaS, m.Speedup, m.MediaRecortadaS, ic, strings.Join(ts, " "))
		}
		tw.Flush()
		fmt.Fprintf(out, "  speedup = cociente de medianas; IC: medias recortadas (%.0f %% por extremo), bootstrap de %d réplicas\n",
			100*bm.Protocolo.RecortePorExtremo, bm.Protocolo.ReplicasBootstrap)
		fmt.Fprintf(out, "  silver y gold idénticos en todas las corridas: %s\n", siNo(bm.Identicos))
	}

	fmt.Fprintf(out, "\nEscrito\n")
	for _, s := range escritos {
		fmt.Fprintf(out, "  %s\n", s)
	}
}

// conteos escribe un mapa nombre → n ordenado de mayor a menor.
func conteos(m map[string]int64) string {
	nombres := make([]string, 0, len(m))
	for n := range m {
		nombres = append(nombres, n)
	}
	slices.SortFunc(nombres, func(a, b string) int {
		if m[a] != m[b] {
			return cmp.Compare(m[b], m[a])
		}
		return strings.Compare(a, b)
	})
	partes := make([]string, len(nombres))
	for i, n := range nombres {
		partes[i] = fmt.Sprintf("%s (%s)", n, miles(m[n]))
	}
	return strings.Join(partes, ", ")
}

// corto abrevia un hash a 12 caracteres para el resumen; el JSON lo trae completo.
func corto(h string) string {
	if len(h) > 12 {
		return h[:12] + "…"
	}
	if h == "" {
		return "-"
	}
	return h
}

func siNo(b bool) string {
	if b {
		return "sí"
	}
	return "no"
}

func guion(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
