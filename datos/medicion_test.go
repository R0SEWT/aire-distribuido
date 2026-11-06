package datos

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func escribirMuestra(t testing.TB) string {
	t.Helper()
	ruta := filepath.Join(t.TempDir(), "oefa_muestra.csv")
	if err := os.WriteFile(ruta, generarMuestra().csv, 0o644); err != nil {
		t.Fatal(err)
	}
	return ruta
}

// Benchmark liviano: la muestra con 1 y 4 workers da lo mismo, y la medición
// trae lo que el informe necesita.
func TestBenchmarkLivianoUnoYCuatroWorkers(t *testing.T) {
	ruta := escribirMuestra(t)
	bm, res, err := MedirCarga(ruta, ConfigPorDefecto(), RecetaProvisional(), []int{1, 4}, 2, Opciones{Bloque: 8 << 10, Chunk: 97})
	if err != nil {
		t.Fatal(err)
	}
	if !bm.Identicos {
		t.Fatal("1 y 4 workers dieron silver o gold distintos")
	}
	if len(bm.Mediciones) != 2 || bm.BaseWorkers != 1 || bm.Repeticiones != 2 {
		t.Fatalf("medición mal armada: %+v", bm)
	}
	for _, m := range bm.Mediciones {
		if len(m.TiemposS) != 2 || m.MedianaS <= 0 || m.Speedup <= 0 {
			t.Errorf("W=%d: %+v", m.Workers, m)
		}
		if m.Sha256Silver != bm.Mediciones[0].Sha256Silver || m.Sha256Gold != bm.Mediciones[0].Sha256Gold {
			t.Errorf("W=%d: huellas distintas", m.Workers)
		}
	}
	if bm.Mediciones[0].Speedup != 1 {
		t.Errorf("el speedup de la base es 1, es %v", bm.Mediciones[0].Speedup)
	}
	if bm.FilasLargas != generarMuestra().leidas || bm.Bloques < 2 || bm.CPUs != runtime.NumCPU() {
		t.Errorf("metadatos: %+v", bm)
	}
	var b bytes.Buffer
	if err := res.Ancha.EscribirSilver(&b); err != nil || b.Len() == 0 {
		t.Fatal("la medición tiene que devolver el último resultado para escribir las salidas")
	}
}

func TestMedirCargaRechazaArgumentos(t *testing.T) {
	ruta := escribirMuestra(t)
	casos := map[string]func() error{
		"sin workers": func() error {
			_, _, err := MedirCarga(ruta, ConfigPorDefecto(), RecetaProvisional(), nil, 1, Opciones{})
			return err
		},
		"sin repeticiones": func() error {
			_, _, err := MedirCarga(ruta, ConfigPorDefecto(), RecetaProvisional(), []int{1}, 0, Opciones{})
			return err
		},
		"workers negativo": func() error {
			_, _, err := MedirCarga(ruta, ConfigPorDefecto(), RecetaProvisional(), []int{-2}, 1, Opciones{})
			return err
		},
		"archivo inexistente": func() error {
			_, _, err := MedirCarga(ruta+".no", ConfigPorDefecto(), RecetaProvisional(), []int{1}, 1, Opciones{})
			return err
		},
	}
	for nombre, f := range casos {
		if f() == nil {
			t.Errorf("%s: tenía que fallar", nombre)
		}
	}
}

func TestMediana(t *testing.T) {
	if m := mediana([]float64{3, 1, 2}); m != 2 {
		t.Errorf("impar: %v", m)
	}
	if m := mediana([]float64{4, 1, 3, 2}); m != 2.5 {
		t.Errorf("par: %v", m)
	}
}

func BenchmarkCargarMuestra(b *testing.B) {
	ruta := escribirMuestra(b)
	for _, w := range []int{1, 4} {
		b.Run("W="+string(rune('0'+w)), func(b *testing.B) {
			for b.Loop() {
				if _, err := Cargar(ruta, ConfigPorDefecto(), RecetaProvisional(), Opciones{Workers: w, Bloque: 64 << 10}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// TestCorpusCompleto corre sobre el CSV real si OEFA_CSV apunta a él. Lo salta
// el CI (-short) y cualquier máquina sin el archivo. Usa configs/limpieza.json,
// que es la que se ajusta al formato real.
//
//	OEFA_CSV=data/bronze/oefa.csv go test ./datos -run TestCorpusCompleto -v
func TestCorpusCompleto(t *testing.T) {
	if testing.Short() {
		t.Skip("-short: el corpus completo no está en el CI")
	}
	ruta := os.Getenv("OEFA_CSV")
	if ruta == "" {
		t.Skip("OEFA_CSV no está definido")
	}
	if !filepath.IsAbs(ruta) {
		ruta = filepath.Join("..", ruta)
	}
	cfg, err := LeerConfigLimpieza(filepath.Join("..", "configs", "limpieza.json"))
	if err != nil {
		t.Fatal(err)
	}
	rec, _, err := LeerReceta(filepath.Join("..", "configs", "kmeans.json"))
	if err != nil {
		t.Fatal(err)
	}
	bm, res, err := MedirCarga(ruta, cfg, rec, []int{1, runtime.NumCPU()}, 1, Opciones{})
	if err != nil {
		t.Fatal(err)
	}
	if !bm.Identicos {
		t.Fatal("1 y NumCPU workers dieron resultados distintos sobre el corpus")
	}
	fl := res.Limpieza.FilasLargas
	if fl.Leidas != fl.Procesadas+fl.Descartadas {
		t.Fatalf("no se conservan las filas: %+v", fl)
	}
	t.Logf("filas largas %d (procesadas %d), anchas %d, gold %d; W=1 %.2f s, W=%d %.2f s",
		fl.Leidas, fl.Procesadas, res.Ancha.N, res.Dataset.N,
		bm.Mediciones[0].MedianaS, bm.Mediciones[1].Workers, bm.Mediciones[1].MedianaS)
}
