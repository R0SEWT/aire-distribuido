package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func configDePrueba(t *testing.T) config {
	t.Helper()
	dir := t.TempDir()
	receta := filepath.Join(dir, "kmeans.json")
	if err := os.WriteFile(receta, []byte(`{"features": ["pm10","fraccion_fina","viento_velocidad","humedad"],
		"log1p": ["pm10"], "nivel": "absoluto", "k": 5, "semilla": 42, "tolerancia": 1e-6, "max_iter": 50,
		"workers": 0, "chunk": 10000}`), 0o600); err != nil {
		t.Fatal(err)
	}
	return config{
		receta: receta, sintetico: 3400, workers: "1,2,4", chunks: "100,1000", repes: 3, calent: 1,
		recorte: 0.10, boot: 200, semillaOrden: 7, tamanos: "0", iterFijas: 5,
		experimentos: "fijas,convergencia", salida: filepath.Join(dir, "reports", "benchmark_kmeans.json"),
	}
}

func TestBenchmarkSobreDatosSinteticos(t *testing.T) {
	c := configDePrueba(t)
	var out bytes.Buffer
	if err := correr(c, &out); err != nil {
		t.Fatalf("correr: %v", err)
	}
	b, err := os.ReadFile(c.salida)
	if err != nil {
		t.Fatalf("no se escribió el informe: %v", err)
	}
	var inf informe
	if err := json.Unmarshal(b, &inf); err != nil {
		t.Fatalf("el informe no es JSON válido: %v", err)
	}
	if inf.Datos["sintetico"] != true {
		t.Error("el informe tiene que decir que los datos son sintéticos")
	}
	// 2 experimentos × (1 secuencial + 3 workers × 2 chunks), 4 rondas cada uno.
	if len(inf.Corridas) != 2*7*4 {
		t.Errorf("%d corridas, se esperaban %d", len(inf.Corridas), 2*7*4)
	}
	if len(inf.Resumen) != 2*7 {
		t.Fatalf("%d filas de resumen, se esperaban 14", len(inf.Resumen))
	}
	for _, r := range inf.Resumen {
		if r.Repeticiones != 3 {
			t.Errorf("%+v: %d repeticiones medidas, se esperaban 3 (el calentamiento no cuenta)", r, r.Repeticiones)
		}
		if !(r.MedianaMs > 0) || !(r.MsPorIteracion > 0) {
			t.Errorf("%s %s W=%d chunk=%d: mediana %v, ms por iteración %v", r.Experimento, r.Modo, r.Workers, r.Chunk, r.MedianaMs, r.MsPorIteracion)
		}
		if r.Modo == "seq" {
			if r.Speedup != nil {
				t.Error("el secuencial no tiene speedup contra sí mismo")
			}
			continue
		}
		if r.Speedup == nil || !(r.Speedup.Inferior <= r.Speedup.Punto && r.Speedup.Punto <= r.Speedup.Superior) {
			t.Errorf("W=%d chunk=%d: speedup %+v", r.Workers, r.Chunk, r.Speedup)
		}
		if !(r.SpeedupMediana > 0) {
			t.Errorf("W=%d chunk=%d: speedup de medianas %v", r.Workers, r.Chunk, r.SpeedupMediana)
		}
	}
	if len(inf.Determinismo) != 2*2 {
		t.Fatalf("determinismo: %+v", inf.Determinismo)
	}
	for _, d := range inf.Determinismo {
		if !d.IdenticoParaTodoW {
			t.Errorf("%+v: con el chunk fijo el resultado debe ser idéntico bit a bit para todo W", d)
		}
	}
	if !strings.Contains(out.String(), "speedup") {
		t.Errorf("la tabla impresa no tiene speedup:\n%s", out.String())
	}
}

func TestEnteros(t *testing.T) {
	v, err := enteros("1, 2,4,,8", 1)
	if err != nil || len(v) != 4 || v[3] != 8 {
		t.Fatalf("enteros = %v, %v", v, err)
	}
	for _, malo := range []string{"", "1,a", ",", "0", "-2"} {
		if _, err := enteros(malo, 1); err == nil {
			t.Errorf("enteros(%q): se esperaba error", malo)
		}
	}
	// Para los tamaños, 0 significa el gold completo.
	if v, err := enteros("0,1000", 0); err != nil || v[0] != 0 {
		t.Errorf("enteros(0,1000) con mínimo 0 = %v, %v", v, err)
	}
}
