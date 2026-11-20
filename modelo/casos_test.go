package modelo

import (
	"cmp"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"math/rand"
	"os"
	"slices"
	"strconv"
	"testing"

	"github.com/R0SEWT/aire-distribuido/kmeans"
)

// -actualizar regenera testdata/casos_gold.json con los datos locales (que no
// se versionan):
//
//	go test ./modelo -run CasosFijos -actualizar
var actualizar = flag.Bool("actualizar", false, "regenera testdata/casos_gold.json desde data/silver y data/gold")

const (
	rutaCasos  = "testdata/casos_gold.json"
	rutaSilver = "../data/silver/oefa_ancho.csv"
)

// archivoCasos son lecturas reales en µg/m³ tomadas del silver, con el régimen
// que les dio el entrenamiento (data/gold/asignaciones.csv). No salen de
// invertir el gold con la estandarización que usa Predecir: la entrada es la
// medición original y la respuesta esperada es la de cmd/entrenar.
type archivoCasos struct {
	Nota       string     `json:"nota"`
	GoldSha256 string     `json:"gold_sha256"`
	Casos      []casoGold `json:"casos"`
}

type casoGold struct {
	Estacion  string  `json:"estacion"`
	FechaHora string  `json:"fecha_hora"`
	PM25      float64 `json:"pm2_5"`
	PM10      float64 `json:"pm10"`
	Regimen   int     `json:"regimen"`
}

// Corre en el CI: casos fijos del gold, versionados, contra el modelo de reports/.
func TestPrediccionCoincideConElEntrenamientoEnCasosFijos(t *testing.T) {
	if *actualizar {
		generarCasos(t)
	}
	b, err := os.ReadFile(rutaCasos)
	if err != nil {
		t.Fatal(err)
	}
	var a archivoCasos
	if err := json.Unmarshal(b, &a); err != nil {
		t.Fatal(err)
	}
	if sha := shaDelGoldDelModelo(t); a.GoldSha256 != sha {
		t.Fatalf("los casos son del gold %s y el modelo de reports/, del %s: regenerarlos con -actualizar", a.GoldSha256, sha)
	}
	m := sembrarDelRepo(t)
	porRegimen := map[int]int{}
	for _, c := range a.Casos {
		r, err := m.Predecir(Lectura{Estacion: c.Estacion, PM25: c.PM25, PM10: c.PM10})
		if err != nil {
			t.Fatalf("%s %s: %v", c.Estacion, c.FechaHora, err)
		}
		if r.Regimen != c.Regimen {
			t.Errorf("%s %s (pm2_5 = %v, pm10 = %v): Predecir da %d y el entrenamiento, %d",
				c.Estacion, c.FechaHora, c.PM25, c.PM10, r.Regimen, c.Regimen)
		}
		porRegimen[c.Regimen]++
	}
	if len(a.Casos) < 150 || len(porRegimen) != m.K() {
		t.Fatalf("%d casos, por régimen %v: se esperaban al menos 150 y de los %d regímenes", len(a.Casos), porRegimen, m.K())
	}
}

func shaDelGoldDelModelo(t *testing.T) string {
	t.Helper()
	var rk reporteKmeans
	if err := leerJSON(reportsDelRepo.Kmeans, &rk); err != nil {
		t.Fatal(err)
	}
	return rk.Gold.Sha256
}

// generarCasos elige, de las lecturas completas del silver: las 60 más cerca
// de la frontera entre dos regímenes (las que un error de redondeo cambiaría
// de régimen) y 40 al azar de cada régimen, con semilla fija. La
// estandarización se usa solo para elegir, no para la respuesta esperada.
func generarCasos(t *testing.T) {
	t.Helper()
	asign := leerAsignacionesPorClave(t)
	f, err := os.Open(rutaSilver)
	if err != nil {
		t.Fatalf("-actualizar necesita el silver local: %v", err)
	}
	defer f.Close()
	cr := csv.NewReader(f)
	cab, err := cr.Read()
	if err != nil {
		t.Fatal(err)
	}
	col := func(nombre string) int {
		i := slices.Index(cab, nombre)
		if i < 0 {
			t.Fatalf("el silver no tiene la columna %s", nombre)
		}
		return i
	}
	cEst, cFecha, c25, c10 := col("estacion"), col("fecha_hora"), col("pm2_5"), col("pm10")

	m := sembrarDelRepo(t)
	est, err := kmeans.LeerEstandarizacion(reportsDelRepo.Estandarizacion)
	if err != nil {
		t.Fatal(err)
	}
	cent := m.CentroidesEstandarizados()
	type candidato struct {
		caso   casoGold
		margen float64 // distancia² al segundo centroide menos al primero
	}
	var todos []candidato
	for {
		fila, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if fila[c25] == "" || fila[c10] == "" {
			continue
		}
		reg, ok := asign[fila[cEst]+" "+fila[cFecha]]
		if !ok {
			continue
		}
		pm25, err1 := strconv.ParseFloat(fila[c25], 64)
		pm10, err2 := strconv.ParseFloat(fila[c10], 64)
		if err1 != nil || err2 != nil {
			t.Fatalf("%s %s: %v %v", fila[cEst], fila[cFecha], err1, err2)
		}
		z := make([]float64, 2)
		if err := est.Estandarizar([]float64{pm25, pm10}, fila[cEst], z); err != nil {
			t.Fatal(err)
		}
		d := make([]float64, m.K())
		for j := range d {
			for f := range z {
				delta := z[f] - cent[j*2+f]
				d[j] += delta * delta
			}
		}
		slices.Sort(d)
		todos = append(todos, candidato{casoGold{fila[cEst], fila[cFecha], pm25, pm10, reg}, d[1] - d[0]})
	}
	if len(todos) != len(asign) {
		t.Fatalf("%d lecturas completas del silver con régimen y %d filas en asignaciones.csv", len(todos), len(asign))
	}
	porMargen := make([]int, len(todos))
	for i := range porMargen {
		porMargen[i] = i
	}
	slices.SortStableFunc(porMargen, func(a, b int) int { return cmp.Compare(todos[a].margen, todos[b].margen) })
	elegidos := map[int]bool{}
	for _, i := range porMargen[:60] {
		elegidos[i] = true
	}
	alAzar := map[int]int{}
	rng := rand.New(rand.NewSource(20261119))
	for _, i := range rng.Perm(len(todos)) {
		if reg := todos[i].caso.Regimen; !elegidos[i] && alAzar[reg] < 40 {
			elegidos[i] = true
			alAzar[reg]++
		}
	}
	idx := make([]int, 0, len(elegidos))
	for i := range elegidos {
		idx = append(idx, i)
	}
	slices.Sort(idx) // en el orden del silver: por estación y fecha_hora
	a := archivoCasos{
		Nota: "Lecturas reales del silver (µg/m³) con el régimen de data/gold/asignaciones.csv: las 60 más cerca " +
			"de una frontera y 40 al azar por régimen. Se regenera con go test ./modelo -run CasosFijos -actualizar.",
		GoldSha256: shaDelGoldDelModelo(t),
	}
	for _, i := range idx {
		a.Casos = append(a.Casos, todos[i].caso)
	}
	b, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll("testdata", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rutaCasos, append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("%s: %d casos de %d lecturas; margen máximo de las de frontera %.3g", rutaCasos, len(a.Casos), len(todos), todos[porMargen[59]].margen)
}

// leerAsignacionesPorClave lee data/gold/asignaciones.csv como «estación
// fecha_hora» → régimen.
func leerAsignacionesPorClave(t *testing.T) map[string]int {
	t.Helper()
	f, err := os.Open(rutaAsignaciones)
	if err != nil {
		t.Fatalf("-actualizar necesita %s: %v", rutaAsignaciones, err)
	}
	defer f.Close()
	cr := csv.NewReader(f)
	if _, err := cr.Read(); err != nil {
		t.Fatal(err)
	}
	asign := map[string]int{}
	for {
		fila, err := cr.Read()
		if errors.Is(err, io.EOF) {
			return asign
		}
		if err != nil {
			t.Fatal(err)
		}
		j, err := strconv.Atoi(fila[2])
		if err != nil {
			t.Fatal(err)
		}
		asign[fila[0]+" "+fila[1]] = j
	}
}
