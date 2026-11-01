package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/R0SEWT/aire-distribuido/datos"
)

const muestra = "../../data/sample/oefa_muestra.csv"

// entorno arma una config por defecto y rutas de salida en un directorio temporal.
func entorno(t *testing.T) (dir string, args []string) {
	t.Helper()
	dir = t.TempDir()
	config := filepath.Join(dir, "limpieza.json")
	if err := datos.GuardarJSON(config, datos.ConfigPorDefecto()); err != nil {
		t.Fatal(err)
	}
	return dir, []string{
		"-config", config,
		"-kmeans", filepath.Join(dir, "no-existe.json"),
		"-entrada", muestra,
		"-silver", filepath.Join(dir, "silver", "oefa_ancho.csv"),
		"-gold", filepath.Join(dir, "gold", "oefa_features.csv"),
		"-reports", filepath.Join(dir, "reports"),
		"-bloque", "16KB",
	}
}

func leerJSON(t *testing.T, ruta string, v any) {
	t.Helper()
	b, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("%s: %v", ruta, err)
	}
}

func TestCargarLaMuestra(t *testing.T) {
	dir, args := entorno(t)
	var out bytes.Buffer
	if err := ejecutar(append(args, "-workers", "4"), &out); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"silver/oefa_ancho.csv", "gold/oefa_features.csv", "reports/limpieza.json", "reports/estandarizacion.json"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("falta %s: %v", f, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "reports", "benchmark_cargador.json")); err == nil {
		t.Error("sin -bench no se escribe el benchmark")
	}
	var lim datos.ReporteLimpieza
	leerJSON(t, filepath.Join(dir, "reports", "limpieza.json"), &lim)
	if lim.FilasLargas.Leidas != 4236 || lim.Workers != 4 || lim.BloqueBytes != 16<<10 {
		t.Errorf("reporte de limpieza: leídas %d, workers %d, bloque %d", lim.FilasLargas.Leidas, lim.Workers, lim.BloqueBytes)
	}
	var est datos.ReporteEstandarizacion
	leerJSON(t, filepath.Join(dir, "reports", "estandarizacion.json"), &est)
	if !slices.Equal(est.Features, []string{"pm10", "fraccion_fina", "viento_velocidad", "humedad"}) || est.N == 0 {
		t.Errorf("estandarización: %+v", est)
	}
	gold, _ := os.ReadFile(filepath.Join(dir, "gold", "oefa_features.csv"))
	if !strings.HasPrefix(string(gold), "estacion,fecha_hora,pm10,fraccion_fina,viento_velocidad,humedad\n") {
		t.Errorf("cabecera del gold: %q", strings.SplitN(string(gold), "\n", 2)[0])
	}
	resumen := out.String()
	for _, s := range []string{"leídas", "4,236", "duplicado_valor_distinto", "SINT-C Junín", "receta provisional", "Benceno"} {
		if !strings.Contains(resumen, s) {
			t.Errorf("el resumen no menciona %q:\n%s", s, resumen)
		}
	}
}

func TestCargarConBenchmark(t *testing.T) {
	dir, args := entorno(t)
	var out bytes.Buffer
	if err := ejecutar(append(args, "-workers", "1,2", "-bench", "2"), &out); err != nil {
		t.Fatal(err)
	}
	var bm datos.Benchmark
	leerJSON(t, filepath.Join(dir, "reports", "benchmark_cargador.json"), &bm)
	if len(bm.Mediciones) != 2 || !bm.Identicos || len(bm.Mediciones[1].TiemposS) != 2 {
		t.Errorf("benchmark: %+v", bm)
	}
	if !strings.Contains(out.String(), "speedup") || !strings.Contains(out.String(), "idénticos") {
		t.Errorf("el resumen tiene que traer la tabla del benchmark:\n%s", out.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "silver", "oefa_ancho.csv")); err != nil {
		t.Error("con -bench también se escriben las salidas de la última corrida")
	}
}

func TestErroresDeUso(t *testing.T) {
	_, args := entorno(t)
	sinEntrada := slices.Clone(args)
	i := slices.Index(sinEntrada, "-entrada")
	sinEntrada = slices.Delete(sinEntrada, i, i+2)
	casos := map[string][]string{
		"sin -entrada":         sinEntrada,
		"lista sin -bench":     append(slices.Clone(args), "-workers", "1,2"),
		"workers inválido":     append(slices.Clone(args), "-workers", "dos"),
		"bloque inválido":      append(slices.Clone(args), "-bloque", "64XB"),
		"argumento posicional": append(slices.Clone(args), "sobra"),
		"config inexistente":   append(slices.Clone(args), "-config", "no/existe.json"),
		"bench negativo":       append(slices.Clone(args), "-bench", "-1"),
	}
	for nombre, a := range casos {
		if err := ejecutar(a, &bytes.Buffer{}); err == nil {
			t.Errorf("%s: tenía que fallar", nombre)
		}
	}
}

func TestParsearTamano(t *testing.T) {
	casos := map[string]int64{
		"64MB": 64 << 20, "64mb": 64 << 20, "64M": 64 << 20, "64MiB": 64 << 20,
		"512KB": 512 << 10, "1GB": 1 << 30, "4096": 4096, "100B": 100, " 2 MB ": 2 << 20,
	}
	for s, quiero := range casos {
		got, err := parsearTamano(s)
		if err != nil || got != quiero {
			t.Errorf("parsearTamano(%q) = %d, %v; quiero %d", s, got, err, quiero)
		}
	}
	for _, s := range []string{"", "MB", "-1MB", "0", "1.5MB", "64XB"} {
		if _, err := parsearTamano(s); err == nil {
			t.Errorf("parsearTamano(%q) tenía que fallar", s)
		}
	}
}

func TestParsearWorkers(t *testing.T) {
	got, err := parsearWorkers("1, 2,4,8")
	if err != nil || !slices.Equal(got, []int{1, 2, 4, 8}) {
		t.Errorf("lista: %v, %v", got, err)
	}
	if got, err := parsearWorkers("0"); err != nil || !slices.Equal(got, []int{0}) {
		t.Errorf("cero: %v, %v", got, err)
	}
	for _, s := range []string{"", "1,,2", "-1", "x"} {
		if _, err := parsearWorkers(s); err == nil {
			t.Errorf("parsearWorkers(%q) tenía que fallar", s)
		}
	}
}

func TestMiles(t *testing.T) {
	casos := map[int64]string{0: "0", 999: "999", 1000: "1,000", 4236: "4,236", 1317949839: "1,317,949,839", -1234: "-1,234"}
	for n, quiero := range casos {
		if got := miles(n); got != quiero {
			t.Errorf("miles(%d) = %q, quiero %q", n, got, quiero)
		}
	}
}
