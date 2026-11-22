package modelo

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/R0SEWT/aire-distribuido/kmeans"
)

var reportsDelRepo = Archivos{
	Kmeans:          "../reports/kmeans.json",
	Regimenes:       "../reports/regimenes.json",
	Estandarizacion: "../reports/estandarizacion.json",
}

func sembrarDelRepo(t testing.TB) *Modelo {
	t.Helper()
	m, err := Sembrar(reportsDelRepo)
	if err != nil {
		t.Fatalf("Sembrar: %v", err)
	}
	return m
}

func TestSembrarDesdeLosReportsDelRepo(t *testing.T) {
	m := sembrarDelRepo(t)
	if m.ID != IDSembrado || m.K() != 3 || !slices.Equal(m.Features(), []string{"pm2_5", "pm10"}) {
		t.Fatalf("modelo %q, k = %d, features %v", m.ID, m.K(), m.Features())
	}
	if m.Iteraciones != 44 || m.Inercia != 50950.36689106275 {
		t.Fatalf("iteraciones %d, inercia %v", m.Iteraciones, m.Inercia)
	}
	// Los valores de reports/regimenes.json (spec §6.5: 9.0/17.6, 65.1/186.7 y 24.4/58.2).
	want := []Regimen{
		{Regimen: 0, PM25Medio: 9.037549677124652, PM10Medio: 17.63909927119451, ExcedeECA: false, Tamano: 48161},
		{Regimen: 1, PM25Medio: 65.12396484374996, PM10Medio: 186.6618229166667, ExcedeECA: true, Tamano: 1536},
		{Regimen: 2, PM25Medio: 24.381915182987196, PM10Medio: 58.22880069238359, ExcedeECA: false, Tamano: 16176},
	}
	if !slices.Equal(m.Regimenes, want) {
		t.Fatalf("regímenes\n%+v\nse esperaba\n%+v", m.Regimenes, want)
	}
	// Los centroides en unidades originales son los de reports/kmeans.json.
	desEst := [][]float64{
		{9.037549677124646, 17.63909927119453},
		{65.12396484375, 186.66182291666667},
		{24.38191518298714, 58.22880069238377},
	}
	if len(m.CentroidesOriginales) != 3 {
		t.Fatalf("centroides originales: %v", m.CentroidesOriginales)
	}
	for j, c := range m.CentroidesOriginales {
		for f := range c {
			if math.Abs(c[f]-desEst[j][f]) > 1e-9 {
				t.Fatalf("centroide %d = %v, reports/kmeans.json dice %v", j, c, desEst[j])
			}
		}
	}
}

func TestPredecirElEjemploDelContrato(t *testing.T) {
	m := sembrarDelRepo(t)
	r, err := m.Predecir(Lectura{Estacion: "CA-AYAS-01", PM25: 70.2, PM10: 190.5})
	if err != nil {
		t.Fatal(err)
	}
	if r.Regimen != 1 || !r.ExcedeECA || r.PM25Medio != 65.12396484374996 || r.PM10Medio != 186.6618229166667 {
		t.Fatalf("Predecir = %+v, se esperaba el régimen 1 (65.1 y 186.7 µg/m³, excede el ECA)", r)
	}
	// Aire limpio: régimen 0. El nivel es absoluto, así que la estación no
	// influye, ni siquiera una que el modelo nunca vio.
	r, err = m.Predecir(Lectura{Estacion: "CA-XX-99", PM25: 8, PM10: 15})
	if err != nil || r.Regimen != 0 || r.ExcedeECA {
		t.Fatalf("Predecir(8, 15) = %+v, %v; se esperaba el régimen 0", r, err)
	}
}

func TestPredecirRechazaValoresQueNoSonLecturas(t *testing.T) {
	m := sembrarDelRepo(t)
	for _, l := range []Lectura{
		{PM25: -1, PM10: 10}, {PM25: 1, PM10: -0.5},
		{PM25: math.NaN(), PM10: 1}, {PM25: 1, PM10: math.Inf(1)},
	} {
		var el *ErrorLectura
		if _, err := m.Predecir(l); !errors.As(err, &el) {
			t.Errorf("Predecir(%+v) = %v, se esperaba un *ErrorLectura", l, err)
		}
	}
}

// Una receta con log1p: la lectura se lleva a log1p antes del z-score. Los
// centroides están puestos para que sin log1p la lectura cayera en el otro
// régimen.
func TestPredecirAplicaLog1pDondeLoPideLaReceta(t *testing.T) {
	dir := reportsSinteticos(t, sintetico{
		features: `["pm2_5", "pm10"]`, log1p: `["pm10"]`,
		media: `[0, 0]`, desv: `[1, 1]`,
		centroides: `[[0, 4.6], [0, 100]]`,
	})
	m, err := Sembrar(dir)
	if err != nil {
		t.Fatal(err)
	}
	// log1p(100) = 4.615: régimen 0. Sin log1p, 100 caería en el régimen 1.
	if r, err := m.Predecir(Lectura{PM25: 0, PM10: 100}); err != nil || r.Regimen != 0 {
		t.Fatalf("Predecir(pm10 = 100) = %+v, %v; se esperaba el régimen 0 (log1p aplicado)", r, err)
	}
	// Y el z-score usa la media y la desviación en la escala log1p.
	dir = reportsSinteticos(t, sintetico{
		features: `["pm2_5", "pm10"]`, log1p: `["pm10"]`,
		media: `[10, 2]`, desv: `[5, 0.5]`,
		centroides: `[[0, 0], [0, 5.2]]`,
	})
	if m, err = Sembrar(dir); err != nil {
		t.Fatal(err)
	}
	// z(pm10 = 100) = (log1p(100) - 2) / 0.5 = 5.23: régimen 1.
	if r, err := m.Predecir(Lectura{PM25: 10, PM10: 100}); err != nil || r.Regimen != 1 {
		t.Fatalf("Predecir = %+v, %v; se esperaba el régimen 1", r, err)
	}
}

func TestPredecirConFraccionFina(t *testing.T) {
	dir := reportsSinteticos(t, sintetico{
		features: `["pm10", "fraccion_fina"]`, log1p: `[]`,
		media: `[0, 0]`, desv: `[1, 1]`,
		centroides: `[[20, 0.2], [20, 0.9]]`,
	})
	m, err := Sembrar(dir)
	if err != nil {
		t.Fatal(err)
	}
	// fraccion_fina = pm2_5/pm10 = 0.85: régimen 1.
	if r, err := m.Predecir(Lectura{PM25: 17, PM10: 20}); err != nil || r.Regimen != 1 {
		t.Fatalf("Predecir = %+v, %v; se esperaba el régimen 1", r, err)
	}
	// Como el cargador, se recorta a 1: PM2.5 está contenido en PM10.
	if r, err := m.Predecir(Lectura{PM25: 30, PM10: 20}); err != nil || r.Regimen != 1 {
		t.Fatalf("Predecir con pm2_5 > pm10 = %+v, %v; se esperaba el régimen 1", r, err)
	}
	var el *ErrorLectura
	if _, err := m.Predecir(Lectura{PM25: 1, PM10: 0}); !errors.As(err, &el) || !strings.Contains(err.Error(), "fraccion_fina") {
		t.Fatalf("con pm10 = 0 se esperaba un error de fraccion_fina, llegó %v", err)
	}
}

func TestPredecirConNivelPorEstacion(t *testing.T) {
	dir := reportsSinteticos(t, sintetico{
		features: `["pm2_5", "pm10"]`, log1p: `[]`, nivel: "estacion",
		porEstacion: `{"CA-A": {"n": 1, "media": [10, 20], "desv": [2, 4]}}`,
		centroides:  `[[0, 0], [3, 3]]`,
	})
	m, err := Sembrar(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r, err := m.Predecir(Lectura{Estacion: "CA-A", PM25: 16, PM10: 32}); err != nil || r.Regimen != 1 {
		t.Fatalf("Predecir = %+v, %v; se esperaba el régimen 1 (z = 3, 3)", r, err)
	}
	if m.CentroidesOriginales != nil {
		t.Fatal("con nivel = estacion los centroides no tienen unidades originales")
	}
	var el *ErrorLectura
	if _, err := m.Predecir(Lectura{Estacion: "CA-B", PM25: 1, PM10: 1}); !errors.As(err, &el) {
		t.Fatalf("una estación sin estadística da %v, se esperaba un *ErrorLectura", err)
	}
}

func TestSembrarRechazaReportsQueNoCoinciden(t *testing.T) {
	base := sintetico{features: `["pm2_5", "pm10"]`, log1p: `[]`, media: `[0, 0]`, desv: `[1, 1]`, centroides: `[[0, 0], [1, 1]]`}
	casos := map[string]func(s *sintetico){
		"feature que no sale de la lectura": func(s *sintetico) { s.features = `["pm2_5", "no2"]`; s.centroides = `[[0, 0], [1, 1]]` },
		"centroide con otra dimensión":      func(s *sintetico) { s.centroides = `[[0, 0], [1, 1, 1]]` },
		"menos regímenes que centroides":    func(s *sintetico) { s.regimenes = `[` + regimenSintetico(0, "no_excede") + `]` },
		"regímenes fuera de orden": func(s *sintetico) {
			s.regimenes = `[` + regimenSintetico(1, "excede") + `,` + regimenSintetico(0, "no_excede") + `]`
		},
		"régimen sin pm2_5_medio": func(s *sintetico) {
			s.regimenes = `[{"regimen": 0, "tamano": 1, "pm10_medio": 1, "eca": "no_excede"},` + regimenSintetico(1, "excede") + `]`
		},
		"etiqueta ECA desconocida": func(s *sintetico) {
			s.regimenes = `[` + regimenSintetico(0, "quizas") + `,` + regimenSintetico(1, "excede") + `]`
		},
		"gold distinto en kmeans y regímenes": func(s *sintetico) { s.shaRegimenes = "otro" },
		"estandarización de otro gold":        func(s *sintetico) { s.nEstandarizacion = 7 },
		"estandarización de otra receta":      func(s *sintetico) { s.log1pEstandarizacion = `["pm10"]` },
	}
	for nombre, cambiar := range casos {
		t.Run(nombre, func(t *testing.T) {
			s := base
			cambiar(&s)
			m, err := Sembrar(reportsSinteticos(t, s))
			if err == nil {
				t.Fatalf("Sembrar aceptó los reports: %+v", m)
			}
			t.Log(err)
		})
	}
	if _, err := Sembrar(Archivos{Kmeans: "no-existe.json", Regimenes: "x", Estandarizacion: "y"}); err == nil {
		t.Fatal("Sembrar sin archivos no falló")
	}
}

// sintetico describe unos reports mínimos con el formato de cmd/entrenar y
// cmd/cargar. Los campos vacíos toman un valor por omisión coherente.
type sintetico struct {
	features, log1p, nivel, media, desv, porEstacion, centroides, regimenes string
	shaRegimenes, log1pEstandarizacion                                      string
	nEstandarizacion                                                        int
}

func regimenSintetico(j int, eca string) string {
	r := strings.NewReplacer("J", string(rune('0'+j)), "ECA", eca)
	return r.Replace(`{"regimen": J, "tamano": 10J, "pm2_5_medio": 1J.5, "pm10_medio": 2J.5, "eca": "ECA", "por_hora": [], "por_estacion": {}}`)
}

func reportsSinteticos(t *testing.T, s sintetico) Archivos {
	t.Helper()
	if s.nivel == "" {
		s.nivel = "absoluto"
	}
	if s.regimenes == "" {
		s.regimenes = `[` + regimenSintetico(0, "no_excede") + `,` + regimenSintetico(1, "excede") + `]`
	}
	if s.shaRegimenes == "" {
		s.shaRegimenes = "abc"
	}
	if s.log1pEstandarizacion == "" {
		s.log1pEstandarizacion = s.log1p
	}
	if s.nEstandarizacion == 0 {
		s.nEstandarizacion = 100
	}
	estadistica := `"media": ` + s.media + `, "desv": ` + s.desv
	if s.nivel == "estacion" {
		estadistica = `"por_estacion": ` + s.porEstacion
	}
	r := strings.NewReplacer(
		"FEATURES", s.features, "LOG1P_EST", s.log1pEstandarizacion, "LOG1P", s.log1p, "NIVEL", s.nivel,
		"CENTROIDES", s.centroides, "REGIMENES", s.regimenes, "SHA_REG", s.shaRegimenes,
		"ESTADISTICA", estadistica, "N_EST", strconv.Itoa(s.nEstandarizacion),
	)
	dir := t.TempDir()
	escribir := func(nombre, contenido string) string {
		ruta := filepath.Join(dir, nombre)
		if err := os.WriteFile(ruta, []byte(r.Replace(contenido)), 0o644); err != nil {
			t.Fatal(err)
		}
		return ruta
	}
	return Archivos{
		Kmeans: escribir("kmeans.json", `{
			"receta": {"features": FEATURES, "log1p": LOG1P, "nivel": "NIVEL", "k": 2, "semilla": 42,
			           "tolerancia": 1e-6, "max_iter": 100, "workers": 0, "chunk": 1000},
			"gold": {"sha256": "abc", "n": 100},
			"k": 2, "iteraciones": 7, "inercia": 12.5, "features": FEATURES,
			"centroides_estandarizados": CENTROIDES}`),
		Regimenes: escribir("regimenes.json", `{
			"gold": {"sha256": "SHA_REG", "n": 100}, "k": 2, "nivel": "NIVEL", "features": FEATURES,
			"regimenes": REGIMENES}`),
		Estandarizacion: escribir("estandarizacion.json", `{
			"features": FEATURES, "log1p": LOG1P_EST, "nivel": "NIVEL", "n": N_EST, ESTADISTICA}`),
	}
}

func BenchmarkPredecir(b *testing.B) {
	m := sembrarDelRepo(b)
	l := Lectura{Estacion: "CA-AYAS-01", PM25: 70.2, PM10: 190.5}
	for b.Loop() {
		if _, err := m.Predecir(l); err != nil {
			b.Fatal(err)
		}
	}
}

// Nuevo arma desde las piezas de un entrenamiento el mismo modelo que Sembrar
// arma desde los reports, y rechaza piezas que no cuadran.
func TestNuevoArmaElMismoModeloQueSembrar(t *testing.T) {
	sembrado := sembrarDelRepo(t)
	est, err := kmeans.LeerEstandarizacion("../reports/estandarizacion.json")
	if err != nil {
		t.Fatal(err)
	}
	var rr struct {
		Regimenes []kmeans.Regimen `json:"regimenes"`
	}
	b, err := os.ReadFile("../reports/regimenes.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &rr); err != nil {
		t.Fatal(err)
	}
	piezas := Piezas{ID: "entrenado-e-1", Receta: sembrado.Receta, Estandarizacion: est,
		Centroides: sembrado.CentroidesEstandarizados(), Iteraciones: sembrado.Iteraciones, Inercia: sembrado.Inercia,
		Regimenes: rr.Regimenes}
	m, err := Nuevo(piezas)
	if err != nil {
		t.Fatal(err)
	}
	if m.ID != "entrenado-e-1" || !slices.Equal(m.Regimenes, sembrado.Regimenes) ||
		!slices.Equal(m.CentroidesEstandarizados(), sembrado.CentroidesEstandarizados()) || m.K() != sembrado.K() {
		t.Fatalf("Nuevo armó otro modelo: %+v", m)
	}
	l := Lectura{Estacion: "CA-AYAS-01", PM25: 70.2, PM10: 190.5}
	a, _ := m.Predecir(l)
	c, _ := sembrado.Predecir(l)
	if a != c {
		t.Fatalf("predicen distinto: %+v y %+v", a, c)
	}

	casos := map[string]func(p *Piezas){
		"sin id":                func(p *Piezas) { p.ID = "" },
		"sin estandarización":   func(p *Piezas) { p.Estandarizacion = nil },
		"centroides de más":     func(p *Piezas) { p.Centroides = append(slices.Clone(p.Centroides), 1) },
		"centroide no finito":   func(p *Piezas) { p.Centroides = slices.Clone(p.Centroides); p.Centroides[1] = math.NaN() },
		"menos regímenes":       func(p *Piezas) { p.Regimenes = p.Regimenes[:1] },
		"sin regímenes":         func(p *Piezas) { p.Regimenes = nil },
		"receta con otro log1p": func(p *Piezas) { p.Receta.Log1p = []string{"pm10"} },
	}
	for nombre, cambiar := range casos {
		p := piezas
		cambiar(&p)
		if m, err := Nuevo(p); err == nil {
			t.Errorf("%s: Nuevo aceptó las piezas: %+v", nombre, m)
		}
	}
}
