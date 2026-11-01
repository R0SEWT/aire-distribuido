// Comando entrenar corre el K-Means de la receta sobre el gold de OEFA y
// escribe el modelo, las asignaciones y el perfil de cada régimen.
//
// Lee configs/kmeans.json (la receta congelada por la calibración),
// data/gold/oefa_features.csv y reports/estandarizacion.json (lo que deja
// cmd/cargar), corre k-means++ y el Lloyd concurrente, y escribe:
//
//   - reports/kmeans.json: centroides estandarizados y des-estandarizados,
//     tamaños, inercia, iteraciones, paro, k, semilla y metadatos de la máquina;
//   - data/gold/asignaciones.csv: estacion,fecha_hora,regimen;
//   - reports/regimenes.json: por régimen, el perfil en unidades originales, la
//     etiqueta frente al ECA y la distribución por hora del día y por estación;
//   - con -codo 2:10, reports/codo.json: la inercia por k sobre una submuestra.
//
// Se corre desde la raíz del repo: go run ./cmd/entrenar
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/R0SEWT/aire-distribuido/kmeans"
)

type opciones struct {
	receta, gold, estandarizacion   string
	salida, asignaciones, regimenes string
	codo, codoSalida                string
	codoN                           int
}

type infoGold struct {
	Ruta       string `json:"ruta"`
	Sha256     string `json:"sha256"`
	N          int    `json:"n"`
	D          int    `json:"d"`
	Estaciones int    `json:"estaciones"`
}

type tiempos struct {
	Carga     float64 `json:"carga"`
	Init      float64 `json:"inicializacion"`
	Clusterin float64 `json:"clustering"`
	Perfiles  float64 `json:"perfiles"`
	Total     float64 `json:"total"`
}

type salidaKMeans struct {
	Metadatos       kmeans.Metadatos `json:"metadatos"`
	Receta          kmeans.Receta    `json:"receta"`
	Gold            infoGold         `json:"gold"`
	Estandarizacion string           `json:"estandarizacion"`

	K                   int     `json:"k"`
	KEfectivo           int     `json:"k_efectivo"`
	Semilla             int64   `json:"semilla"`
	Workers             int     `json:"workers"`
	Chunk               int     `json:"chunk"`
	TolAbs              float64 `json:"tol_abs"`
	TolRel              float64 `json:"tol_rel"`
	MaxIter             int     `json:"max_iter"`
	CentroidesIniciales string  `json:"centroides_iniciales_sha256"`

	Iteraciones int       `json:"iteraciones"`
	Paro        string    `json:"paro"`
	Movimiento  float64   `json:"movimiento_ultima_iteracion"`
	Vacios      int       `json:"clusters_vacios"`
	Inercia     float64   `json:"inercia"`
	Inercias    []float64 `json:"inercias_por_iteracion"`
	Tamanos     []int     `json:"tamanos"`

	Features                    []string    `json:"features"`
	CentroidesEstandarizados    [][]float64 `json:"centroides_estandarizados"`
	CentroidesDesEstandarizados [][]float64 `json:"centroides_des_estandarizados"`
	NotaDesEstandarizacion      string      `json:"nota_des_estandarizacion"`

	Ms       tiempos         `json:"ms"`
	Recursos kmeans.Recursos `json:"recursos"`
}

type umbralesECA struct {
	PM25  float64 `json:"pm2_5_ugm3"`
	PM10  float64 `json:"pm10_ugm3"`
	Norma string  `json:"norma"`
	Nota  string  `json:"nota"`
}

type salidaRegimenes struct {
	Metadatos  kmeans.Metadatos `json:"metadatos"`
	Gold       infoGold         `json:"gold"`
	K          int              `json:"k"`
	Nivel      string           `json:"nivel"`
	Features   []string         `json:"features"`
	ECA        umbralesECA      `json:"eca"`
	NotaPerfil string           `json:"nota_perfil"`
	Regimenes  []kmeans.Regimen `json:"regimenes"`
}

type salidaCodo struct {
	Metadatos kmeans.Metadatos   `json:"metadatos"`
	Gold      infoGold           `json:"gold"`
	Receta    kmeans.Receta      `json:"receta"`
	Workers   int                `json:"workers"`
	Puntos    []kmeans.PuntoCodo `json:"puntos"`
	Nota      string             `json:"nota"`
}

const (
	notaCentroidesAbsoluto = "Centroides llevados a unidades originales invirtiendo el z-score y luego log1p " +
		"(reports/estandarizacion.json). En las features con log1p el centroide invertido es la media de los " +
		"logaritmos llevada de vuelta (del orden de una media geométrica), menor que la media aritmética; la " +
		"media aritmética de cada régimen está en reports/regimenes.json (perfil)."
	notaCentroidesEstacion = "Con nivel = estacion cada estación se estandarizó con su propia media y desviación: " +
		"un centroide promedia filas de estaciones distintas y no tiene unidades absolutas, así que los " +
		"centroides des-estandarizados no existen. Los perfiles de reports/regimenes.json sí están en unidades: " +
		"se calculan fila por fila con la estadística de la estación de cada lectura."
	notaPerfil = "perfil: media aritmética de cada feature en unidades originales del silver (PM en µg/m³), " +
		"des-estandarizando cada lectura del régimen. pm2_5_medio: la feature pm2_5 o, si no está, " +
		"fraccion_fina × pm10 lectura por lectura. por_hora: hora del día en Lima (UTC-5), de 0 a 23."
	notaECA = "El ECA es un promedio de 24 h. Que la media horaria de un régimen lo supere dice que el régimen " +
		"es peligroso, no que haya un incumplimiento; una lectura horaria alta tampoco lo es por sí sola. " +
		"excede: PM2.5 medio > 50 o PM10 medio > 100. no_excede: los dos disponibles y ninguno supera. " +
		"no_evaluable: falta el contaminante que podría superar el umbral."
)

func main() {
	var op opciones
	flag.StringVar(&op.receta, "config", "configs/kmeans.json", "receta congelada por la calibración")
	flag.StringVar(&op.gold, "gold", "data/gold/oefa_features.csv", "gold de cmd/cargar")
	flag.StringVar(&op.estandarizacion, "estandarizacion", "reports/estandarizacion.json", "medias y desviaciones de cmd/cargar")
	flag.StringVar(&op.salida, "salida", "reports/kmeans.json", "modelo: centroides, tamaños, inercia")
	flag.StringVar(&op.asignaciones, "asignaciones", "data/gold/asignaciones.csv", "régimen de cada lectura")
	flag.StringVar(&op.regimenes, "regimenes", "reports/regimenes.json", "perfil, ECA, horas y estaciones por régimen")
	flag.StringVar(&op.codo, "codo", "", "rango de k para el codo, p. ej. 2:10; vacío = no se calcula")
	flag.StringVar(&op.codoSalida, "codo-salida", "reports/codo.json", "inercia por k")
	flag.IntVar(&op.codoN, "codo-n", 200000, "filas de la submuestra sistemática del codo; 0 = todas")
	flag.CommandLine.Parse(argumentos(os.Args))

	if err := correr(op, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func correr(op opciones, w io.Writer) error {
	inicio := time.Now()
	var kMin, kMax int
	if op.codo != "" {
		var err error
		if kMin, kMax, err = rangoK(op.codo); err != nil {
			return err
		}
	}

	receta, err := kmeans.LeerReceta(op.receta)
	if err != nil {
		return err
	}
	est, err := kmeans.LeerEstandarizacion(op.estandarizacion)
	if err != nil {
		return err
	}
	if err := est.Coincide(receta); err != nil {
		return fmt.Errorf("%s: %w", op.estandarizacion, err)
	}

	t0 := time.Now()
	g, err := kmeans.LeerGold(op.gold, receta.Features)
	if err != nil {
		return err
	}
	msCarga := ms(time.Since(t0))
	if err := mismoGold(g, est); err != nil {
		return fmt.Errorf("%s no es la estandarización de %s: %w", op.estandarizacion, op.gold, err)
	}
	hash, err := kmeans.Sha256Archivo(op.gold)
	if err != nil {
		return err
	}
	meta := kmeans.RecolectarMetadatos()
	info := infoGold{Ruta: op.gold, Sha256: hash, N: g.Datos.N, D: g.Datos.D, Estaciones: len(g.Estaciones())}
	conc := receta.OpcionesConc()

	if op.codo != "" {
		puntos, err := kmeans.Codo(g.Datos, kMin, kMax, op.codoN, receta.Semilla, conc)
		if err != nil {
			return err
		}
		if err := guardarJSON(op.codoSalida, salidaCodo{Metadatos: meta, Gold: info, Receta: receta,
			Workers: conc.Workers, Puntos: puntos,
			Nota: "Inercia con los centroides finales sobre una submuestra sistemática del gold, " +
				"k-means++ con la semilla de la receta para cada k. La silueta va en la PC4."}); err != nil {
			return err
		}
		imprimirCodo(w, puntos, op.codoSalida)
	}

	t0 = time.Now()
	cent, kEf, err := kmeans.KMeansPP(g.Datos, receta.K, receta.Semilla)
	if err != nil {
		return err
	}
	msInit := ms(time.Since(t0))
	if kEf < receta.K {
		fmt.Fprintf(os.Stderr, "aviso: k-means++ solo encontró %d puntos distintos; k efectivo %d en vez de %d\n",
			kEf, kEf, receta.K)
	}
	hashInicial := kmeans.HashCentroides(cent)
	conc.K = kEf

	t0 = time.Now()
	r, err := kmeans.Concurrente(g.Datos, cent, conc)
	if err != nil {
		return err
	}
	msClustering := ms(time.Since(t0))

	t0 = time.Now()
	regs, err := kmeans.Perfilar(g, r.Asignaciones, kEf, est)
	if err != nil {
		return err
	}
	msPerfiles := ms(time.Since(t0))

	s := salidaKMeans{
		Metadatos: meta, Receta: receta, Gold: info, Estandarizacion: op.estandarizacion,
		K: receta.K, KEfectivo: kEf, Semilla: receta.Semilla, Workers: conc.Workers, Chunk: conc.Chunk,
		TolAbs: conc.TolAbs, TolRel: conc.TolRel, MaxIter: conc.MaxIter, CentroidesIniciales: hashInicial,
		Iteraciones: r.Iteraciones, Paro: r.Paro, Movimiento: r.Movimiento, Vacios: r.Vacios,
		Inercia: r.Inercia, Inercias: r.Inercias, Features: receta.Features,
	}
	for _, reg := range regs {
		s.Tamanos = append(s.Tamanos, reg.Tamano)
	}
	for j := 0; j < kEf; j++ {
		s.CentroidesEstandarizados = append(s.CentroidesEstandarizados, append([]float64(nil), r.Centroide(j)...))
	}
	if est.CentroidesEnUnidades() {
		s.NotaDesEstandarizacion = notaCentroidesAbsoluto
		for j := 0; j < kEf; j++ {
			x := make([]float64, g.Datos.D)
			if err := est.Invertir(r.Centroide(j), "", x); err != nil {
				return err
			}
			s.CentroidesDesEstandarizados = append(s.CentroidesDesEstandarizados, x)
		}
	} else {
		s.NotaDesEstandarizacion = notaCentroidesEstacion
	}

	if err := escribirAsignaciones(op.asignaciones, g, r.Asignaciones); err != nil {
		return err
	}
	if err := guardarJSON(op.regimenes, salidaRegimenes{
		Metadatos: meta, Gold: info, K: kEf, Nivel: receta.Nivel, Features: receta.Features,
		ECA: umbralesECA{PM25: kmeans.ECAPM25, PM10: kmeans.ECAPM10,
			Norma: "DS 003-2017-MINAM, valores de 24 horas", Nota: notaECA},
		NotaPerfil: notaPerfil, Regimenes: regs,
	}); err != nil {
		return err
	}
	s.Ms = tiempos{Carga: msCarga, Init: msInit, Clusterin: msClustering, Perfiles: msPerfiles,
		Total: ms(time.Since(inicio))}
	s.Recursos = kmeans.MedirRecursos()
	if err := guardarJSON(op.salida, s); err != nil {
		return err
	}

	imprimir(w, s, regs, receta, op)
	return nil
}

// mismoGold comprueba que la estandarización se haya calculado sobre este
// gold: mismas filas en total y, por estación, mismas filas por estación. Una
// estandarización vieja daría perfiles en unidades equivocadas sin ningún error.
func mismoGold(g *kmeans.Gold, e *kmeans.Estandarizacion) error {
	if e.N != g.Datos.N {
		return fmt.Errorf("dice n = %d y el gold tiene %d filas", e.N, g.Datos.N)
	}
	if e.Nivel != kmeans.NivelEstacion {
		return nil
	}
	porEstacion := map[string]int{}
	for _, est := range g.Estacion {
		porEstacion[est]++
	}
	for est, n := range porEstacion {
		s, ok := e.PorEstacion[est]
		if !ok {
			return fmt.Errorf("la estación %s del gold no está en por_estacion", est)
		}
		if s.N != n {
			return fmt.Errorf("la estación %s tiene n = %d en por_estacion y %d filas en el gold", est, s.N, n)
		}
	}
	return nil
}

// rangoK lee "2:10".
func rangoK(s string) (int, int, error) {
	a, b, ok := strings.Cut(s, ":")
	if !ok {
		return 0, 0, fmt.Errorf("-codo %q: se esperaba kmin:kmax, p. ej. 2:10", s)
	}
	kMin, err1 := strconv.Atoi(a)
	kMax, err2 := strconv.Atoi(b)
	if err1 != nil || err2 != nil || kMin <= 0 || kMax < kMin {
		return 0, 0, fmt.Errorf("-codo %q: se esperaba kmin:kmax con 0 < kmin <= kmax", s)
	}
	return kMin, kMax, nil
}

func escribirAsignaciones(ruta string, g *kmeans.Gold, asign []int) error {
	if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
		return err
	}
	f, err := os.Create(ruta)
	if err != nil {
		return err
	}
	bw := bufio.NewWriter(f)
	bw.WriteString("estacion,fecha_hora,regimen\n")
	var buf []byte
	for i, j := range asign {
		buf = append(buf[:0], g.Estacion[i]...)
		buf = append(buf, ',')
		buf = g.FechaHora[i].In(kmeans.ZonaPeru).AppendFormat(buf, time.RFC3339)
		buf = append(buf, ',')
		buf = strconv.AppendInt(buf, int64(j), 10)
		buf = append(buf, '\n')
		bw.Write(buf)
	}
	if err := bw.Flush(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func guardarJSON(ruta string, v any) error {
	if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("%s: %w", ruta, err)
	}
	return os.WriteFile(ruta, append(b, '\n'), 0o644)
}

func ms(d time.Duration) float64 { return float64(d.Nanoseconds()) / 1e6 }

func imprimirCodo(w io.Writer, puntos []kmeans.PuntoCodo, ruta string) {
	fmt.Fprintf(w, "codo        submuestra de %d filas → %s\n", puntos[0].N, ruta)
	for i, p := range puntos {
		caida := ""
		if i > 0 {
			caida = fmt.Sprintf("  (%.1f %% menos que k=%d)", 100*(1-p.Inercia/puntos[i-1].Inercia), puntos[i-1].K)
		}
		fmt.Fprintf(w, "            k=%-2d inercia %.6e · %d iteraciones%s\n", p.K, p.Inercia, p.Iteraciones, caida)
	}
	fmt.Fprintln(w)
}

func imprimir(w io.Writer, s salidaKMeans, regs []kmeans.Regimen, receta kmeans.Receta, op opciones) {
	var feats []string
	for _, f := range receta.Features {
		if receta.ConLog1p(f) {
			f += " (log1p)"
		}
		feats = append(feats, f)
	}
	fmt.Fprintf(w, "gold        %s · %d filas × %d features · %d estaciones · sha256 %s…\n",
		s.Gold.Ruta, s.Gold.N, s.Gold.D, s.Gold.Estaciones, s.Gold.Sha256[:12])
	fmt.Fprintf(w, "receta      %s · nivel %s · k=%d · semilla %d\n",
		strings.Join(feats, ", "), receta.Nivel, s.K, s.Semilla)
	fmt.Fprintf(w, "modo        concurrente · %d workers · chunk %d · %s · %s\n",
		s.Workers, s.Chunk, s.Metadatos.Maquina, s.Metadatos.VersionGo)
	fmt.Fprintf(w, "tiempos     carga %.0f ms · k-means++ %.0f ms · clustering %.0f ms · perfiles %.0f ms · total %.0f ms\n",
		s.Ms.Carga, s.Ms.Init, s.Ms.Clusterin, s.Ms.Perfiles, s.Ms.Total)
	fmt.Fprintf(w, "resultado   %d iteraciones (paro=%s) · %d clusters vacíos · inercia %.6e\n\n",
		s.Iteraciones, s.Paro, s.Vacios, s.Inercia)

	fmt.Fprintf(w, "%-8s %9s %6s", "régimen", "lecturas", "%")
	for _, f := range receta.Features {
		fmt.Fprintf(w, " %16s", f)
	}
	fmt.Fprintf(w, " %9s  %s\n", "PM2.5", "ECA (24 h)")
	for _, r := range regs {
		fmt.Fprintf(w, "%-8d %9d %5.1f%%", r.Regimen, r.Tamano, 100*r.Fraccion)
		for _, f := range receta.Features {
			if r.Perfil == nil {
				fmt.Fprintf(w, " %16s", "—")
			} else {
				fmt.Fprintf(w, " %16.2f", r.Perfil[f])
			}
		}
		pm25 := "—"
		if r.PM25Medio != nil {
			pm25 = fmt.Sprintf("%.1f", *r.PM25Medio)
		}
		eca := r.ECA
		if len(r.ECAPor) > 0 {
			eca += " por " + strings.Join(r.ECAPor, " y ")
		}
		fmt.Fprintf(w, " %9s  %s\n", pm25, eca)
	}
	fmt.Fprintln(w, "\n(perfil: media aritmética en unidades originales; el ECA es de 24 h, ver reports/regimenes.json)")

	fmt.Fprintln(w, "\ndónde y cuándo (las 3 horas del día y las 3 estaciones con más lecturas de cada régimen)")
	for _, r := range regs {
		if r.Tamano == 0 {
			fmt.Fprintf(w, "  régimen %d  vacío\n", r.Regimen)
			continue
		}
		fmt.Fprintf(w, "  régimen %d  horas %s · estaciones %s\n", r.Regimen, topHoras(r, 3), topEstaciones(r, 3))
	}
	fmt.Fprintf(w, "\n→ %s · %s · %s\n", op.salida, op.regimenes, op.asignaciones)
}

func topHoras(r kmeans.Regimen, n int) string {
	horas := make([]int, 24)
	for h := range horas {
		horas[h] = h
	}
	sort.SliceStable(horas, func(a, b int) bool { return r.PorHora[horas[a]] > r.PorHora[horas[b]] })
	var out []string
	for _, h := range horas[:n] {
		out = append(out, fmt.Sprintf("%02dh (%.0f %%)", h, 100*float64(r.PorHora[h])/float64(r.Tamano)))
	}
	return strings.Join(out, ", ")
}

func topEstaciones(r kmeans.Regimen, n int) string {
	var est []string
	for e := range r.PorEstacion {
		est = append(est, e)
	}
	sort.Slice(est, func(a, b int) bool {
		if r.PorEstacion[est[a]] != r.PorEstacion[est[b]] {
			return r.PorEstacion[est[a]] > r.PorEstacion[est[b]]
		}
		return est[a] < est[b]
	})
	if len(est) > n {
		est = est[:n]
	}
	var out []string
	for _, e := range est {
		out = append(out, fmt.Sprintf("%s (%.0f %%)", e, 100*float64(r.PorEstacion[e])/float64(r.Tamano)))
	}
	return strings.Join(out, ", ")
}
