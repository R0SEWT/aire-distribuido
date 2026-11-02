package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestArgumentosDescartaLaRutaDuplicadaDeAndroid(t *testing.T) {
	// En Android el binario se lanza vía el enlazador del sistema y recibe su
	// propia ruta dos veces: flag.Parse se detenía en la copia y descartaba todos
	// los flags. Se vio en un Pixel 9a con Termux.
	casos := []struct {
		nombre string
		args   []string
		quiere []string
	}{
		{"android", []string{"/h/benchmark", "/h/benchmark", "-modo", "seq"}, []string{"-modo", "seq"}},
		{"normal", []string{"/h/benchmark", "-modo", "seq"}, []string{"-modo", "seq"}},
		{"sin flags", []string{"./benchmark"}, []string{}},
		{"primer argumento distinto", []string{"/h/benchmark", "otra", "-k", "3"}, []string{"otra", "-k", "3"}},
	}
	// El caso real de Termux: "./benchmark" y la ruta absoluta del mismo archivo.
	dir := t.TempDir()
	bin := filepath.Join(dir, "benchmark")
	if err := os.WriteFile(bin, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	previo, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(previo)
	casos = append(casos, struct {
		nombre string
		args   []string
		quiere []string
	}{"relativa y absoluta", []string{"./benchmark", bin, "-modo", "seq"}, []string{"-modo", "seq"}})

	for _, c := range casos {
		got := argumentos(c.args)
		if len(got) != len(c.quiere) {
			t.Fatalf("%s: %q, se esperaba %q", c.nombre, got, c.quiere)
		}
		for i := range got {
			if got[i] != c.quiere[i] {
				t.Fatalf("%s: %q, se esperaba %q", c.nombre, got, c.quiere)
			}
		}
	}
}
