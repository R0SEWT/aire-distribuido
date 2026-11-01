package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/R0SEWT/aire-distribuido/kmeans"
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
	for _, c := range inf.Corridas {
		if c.Experimento == "fijas" && c.Iteraciones != 5 {
			t.Errorf("fijas: una corrida hizo %d iteraciones, se pidieron 5", c.Iteraciones)
		}
		if c.Modo == "conc" && c.Experimento == "convergencia" && c.DistintasIteraciones != (c.Iteraciones != c.IteracionesSecuencial) {
			t.Errorf("convergencia W=%d chunk=%d: marcada %v con %d iteraciones y %d el secuencial",
				c.Workers, c.Chunk, c.DistintasIteraciones, c.Iteraciones, c.IteracionesSecuencial)
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

func TestFijasNoParaEnUnPuntoFijoExacto(t *testing.T) {
	// Dos nubes que convergen exacto en dos iteraciones: con tolerancia 0 el
	// experimento de trabajo fijo pararía ahí (desplazamiento 0 <= 0) y
	// mediría menos trabajo del que dice.
	d := &kmeans.Datos{X: []float64{0, 0, 0, 1, 10, 10, 10, 11}, N: 4, D: 2}
	cent := []float64{0, 0, 10, 10}
	r, err := kmeans.Secuencial(d, cent, opcionesFijas(2, 7))
	if err != nil {
		t.Fatal(err)
	}
	if r.Iteraciones != 7 || r.Paro != kmeans.ParoMaxIter {
		t.Errorf("trabajo fijo: %d iteraciones (paro %s), se pidieron 7", r.Iteraciones, r.Paro)
	}
	conv, err := kmeans.Secuencial(d, cent, kmeans.Opciones{K: 2, MaxIter: 7})
	if err != nil {
		t.Fatal(err)
	}
	if conv.Iteraciones >= 7 {
		t.Fatalf("el caso no llega a un punto fijo exacto: %d iteraciones", conv.Iteraciones)
	}
}

func TestMarcarIteracionesDistintasAlSecuencial(t *testing.T) {
	cs := []corrida{
		{Experimento: "convergencia", Tamano: 10, Modo: "seq", Iteraciones: 12},
		{Experimento: "convergencia", Tamano: 10, Modo: "conc", Workers: 2, Chunk: 5, Iteraciones: 12},
		{Experimento: "convergencia", Tamano: 10, Modo: "conc", Workers: 4, Chunk: 5, Iteraciones: 13},
		{Experimento: "convergencia", Tamano: 20, Modo: "seq", Iteraciones: 9},
		{Experimento: "convergencia", Tamano: 20, Modo: "conc", Workers: 2, Chunk: 5, Iteraciones: 9},
	}
	marcarIteraciones(cs)
	for i, quiero := range []bool{false, false, true, false, false} {
		if cs[i].DistintasIteraciones != quiero {
			t.Errorf("corrida %d (%s n=%d W=%d, %d iteraciones): marcada = %v", i, cs[i].Modo, cs[i].Tamano,
				cs[i].Workers, cs[i].Iteraciones, cs[i].DistintasIteraciones)
		}
	}
	if cs[2].IteracionesSecuencial != 12 {
		t.Errorf("la corrida marcada tiene que decir cuántas hizo el secuencial: %d", cs[2].IteracionesSecuencial)
	}
	// Dos rondas medidas por configuración, para que haya desviación.
	dos := append([]corrida(nil), cs...)
	for _, c := range cs {
		c.Ronda = 1
		dos = append(dos, c)
	}
	for i := range dos {
		dos[i].MsClustering = float64(10 + i)
	}
	rs, err := resumir(dos, 0, 10, 1)
	if err != nil {
		t.Fatalf("resumir: %v", err)
	}
	contadas := 0
	for _, r := range rs {
		if r.Workers == 4 && r.Tamano == 10 {
			contadas = r.CorridasConOtrasIteraciones
		} else if r.CorridasConOtrasIteraciones != 0 {
			t.Errorf("%+v no debería tener corridas marcadas", r)
		}
	}
	if contadas != 2 {
		t.Errorf("el resumen de W=4 tiene que contar sus dos corridas marcadas, cuenta %d", contadas)
	}
}
