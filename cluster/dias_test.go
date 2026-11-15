package cluster

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

// diasSeguidos devuelve n días consecutivos desde el 1 de enero de 2018.
func diasSeguidos(n int) []string {
	t0 := time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC)
	dias := make([]string, n)
	for i := range dias {
		dias[i] = t0.AddDate(0, 0, i).Format(time.DateOnly)
	}
	return dias
}

func TestRepartirDiasEsDeterministaYCubreTodoSinRepetir(t *testing.T) {
	dias := diasSeguidos(100) // 14 bloques de 7 y uno de 2
	desordenados := slices.Clone(dias)
	slices.Reverse(desordenados)

	a, err := RepartirDias(desordenados, 3, 42)
	if err != nil {
		t.Fatal(err)
	}
	b, err := RepartirDias(dias, 3, 42)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatal("con la misma semilla y los mismos días el reparto tiene que ser el mismo, venga en el orden que venga")
	}
	if c, _ := RepartirDias(dias, 3, 43); reflect.DeepEqual(a, c) {
		t.Error("otra semilla dio el mismo reparto: los bloques no se barajan")
	}

	var todos []string
	bloquesPorNodo := make([]int, len(a))
	for i, ds := range a {
		if !slices.IsSorted(ds) {
			t.Errorf("los días del nodo %d no están en orden: %v", i, ds)
		}
		todos = append(todos, ds...)
		for _, d := range ds {
			if slices.Index(dias, d)%DiasPorBloque == 0 {
				bloquesPorNodo[i]++ // el primer día de cada bloque
			}
		}
	}
	slices.Sort(todos)
	if !reflect.DeepEqual(todos, dias) {
		t.Fatalf("el reparto no cubre los 100 días exactamente una vez: %d días", len(todos))
	}
	// Los bloques no se parten: los 7 días de un bloque van al mismo nodo.
	dueno := map[string]int{}
	for i, ds := range a {
		for _, d := range ds {
			dueno[d] = i
		}
	}
	for j, d := range dias {
		if dueno[d] != dueno[dias[j-j%DiasPorBloque]] {
			t.Fatalf("el día %s no está con el resto de su bloque", d)
		}
	}
	// En ronda: 15 bloques entre 3 nodos son 5 cada uno.
	if !reflect.DeepEqual(bloquesPorNodo, []int{5, 5, 5}) {
		t.Errorf("bloques por nodo %v, se esperaban 5 cada uno", bloquesPorNodo)
	}
	// Barajados: el nodo 0 no se queda con los primeros bloques del periodo.
	if reflect.DeepEqual(a[0][:7], dias[:7]) && reflect.DeepEqual(a[1][:7], dias[7:14]) && reflect.DeepEqual(a[2][:7], dias[14:21]) {
		t.Error("los bloques salieron en el orden del calendario: no se barajaron")
	}
}

func TestRepartirDiasRechazaLoQueNoSePuedeRepartir(t *testing.T) {
	casos := map[string]struct {
		dias  []string
		nodos int
		texto string
	}{
		"menos bloques que nodos": {diasSeguidos(14), 3, "2 bloques"},
		"sin nodos":               {diasSeguidos(14), 0, "nodos"},
		"día repetido":            {append(diasSeguidos(14), "2018-01-03"), 2, "2018-01-03"},
		"día mal escrito":         {append(diasSeguidos(14), "2018-1-30"), 2, "2018-1-30"},
	}
	for nombre, c := range casos {
		t.Run(nombre, func(t *testing.T) {
			if _, err := RepartirDias(c.dias, c.nodos, 1); err == nil || !strings.Contains(err.Error(), c.texto) {
				t.Fatalf("se esperaba un error con %q, salió %v", c.texto, err)
			}
		})
	}
}

// manifiestoDePrueba tiene la forma del de lab/mapa/datos/preparar.py: los
// días salen de las claves dias/AAAA-MM-DD.npy de "archivos" (en desorden, y
// mezcladas con otros archivos); el "dias" de arriba describe el formato y no
// es la lista.
const manifiestoDePrueba = `{
  "version": 1,
  "grilla": {"filas": 24, "columnas": 16},
  "periodo": {"desde": "2018-01-01", "hasta": "2018-01-03", "dias": 3},
  "dias": {"ruta": "dias/AAAA-MM-DD.npy", "dtype": "float16", "forma": [24, 13, 24, 16]},
  "canales": [{"nombre": "t2m", "media": 290.1, "desviacion": 5.2}],
  "archivos": {
    "dias/2018-01-03.npy": {"sha256": "cc", "bytes": 10},
    "estaticos.npy": {"sha256": "ee", "bytes": 10},
    "dias/2018-01-01.npy": {"sha256": "aa", "bytes": 10},
    "dias/2018-01-02.npy": {"sha256": "bb", "bytes": 10},
    "etiquetas.csv": {"sha256": "ff", "bytes": 10}
  }
}
`

func TestLeerManifiestoDeUnArchivoYPorHTTP(t *testing.T) {
	suma := sha256.Sum256([]byte(manifiestoDePrueba))
	huella := HuellaDeSha256(hex.EncodeToString(suma[:]))

	ruta := filepath.Join(t.TempDir(), "manifiesto.json")
	if err := os.WriteFile(ruta, []byte(manifiestoDePrueba), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mapa/manifiesto.json" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(manifiestoDePrueba))
	}))
	defer srv.Close()

	for _, fuente := range []string{ruta, srv.URL + "/mapa/manifiesto.json"} {
		m, err := LeerManifiesto(fuente)
		if err != nil {
			t.Fatalf("%s: %v", fuente, err)
		}
		if m.Fuente != fuente || m.Huella != huella || !reflect.DeepEqual(m.Dias, []string{"2018-01-01", "2018-01-02", "2018-01-03"}) {
			t.Errorf("%s: %+v; se esperaba la huella %s y los tres días en orden", fuente, m, huella)
		}
	}
	if _, err := LeerManifiesto(srv.URL + "/otro.json"); err == nil || !strings.Contains(err.Error(), "404") {
		t.Errorf("un 404 tiene que ser un error que lo diga, salió %v", err)
	}
}

// Como lab/mapa/modelo/datos.py: sin "archivos", vale un mapa "sha256" de ruta
// a sha256.
func TestLeerManifiestoConElMapaSha256(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "manifiesto.json")
	os.WriteFile(ruta, []byte(`{"sha256": {"dias/2018-02-01.npy": "aa", "dias/2018-01-31.npy": "bb", "estaticos.npy": "cc"}}`), 0o644)
	m, err := LeerManifiesto(ruta)
	if err != nil || !reflect.DeepEqual(m.Dias, []string{"2018-01-31", "2018-02-01"}) {
		t.Fatalf("días %v, %v", m, err)
	}
}

func TestLeerManifiestoRechazaLoQueNoSirve(t *testing.T) {
	malos := map[string]struct{ contenido, texto string }{
		"sin archivos":         {`{"version": 1}`, "archivos"},
		"sin días":             {`{"archivos": {"estaticos.npy": {"sha256": "aa"}}}`, "dias/"},
		"no es JSON":           {`{"archivos": {`, "manifiesto"},
		"día mal escrito":      {`{"archivos": {"dias/2018-1-01.npy": {"sha256": "aa"}}}`, "2018-1-01"},
		"archivos no es mapa":  {`{"archivos": ["dias/2018-01-01.npy"]}`, "manifiesto"},
		"el archivo no existe": {"", "manifiesto"},
	}
	for nombre, c := range malos {
		t.Run(nombre, func(t *testing.T) {
			r := filepath.Join(t.TempDir(), "manifiesto.json")
			if c.contenido != "" {
				os.WriteFile(r, []byte(c.contenido), 0o644)
			}
			if _, err := LeerManifiesto(r); err == nil || !strings.Contains(err.Error(), c.texto) {
				t.Errorf("se esperaba un error con %q, salió %v", c.texto, err)
			}
		})
	}
}
