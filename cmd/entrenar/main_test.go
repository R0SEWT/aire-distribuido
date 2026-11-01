package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/R0SEWT/aire-distribuido/kmeans"
)

var featuresReceta = []string{"pm10", "fraccion_fina", "viento_velocidad", "humedad"}

// preparar escribe en un directorio temporal lo que cmd/cargar dejaría: el gold
// sintético y su estandarización, más la receta. Devuelve las rutas.
func preparar(t *testing.T, nivel string) (opciones, *kmeans.Sintetico) {
	t.Helper()
	dir := t.TempDir()
	s, err := kmeans.GoldSintetico(kmeans.OpcionesSintetico{Features: featuresReceta, Log1p: []string{"pm10"},
		Nivel: nivel, Estaciones: 4, Horas: 600, Ruido: 0.1, Semilla: 3})
	if err != nil {
		t.Fatalf("GoldSintetico: %v", err)
	}
	var gold bytes.Buffer
	if err := kmeans.EscribirGold(&gold, s.Gold); err != nil {
		t.Fatal(err)
	}
	escribir(t, filepath.Join(dir, "data/gold/oefa_features.csv"), gold.Bytes())
	est, err := json.Marshal(s.Est)
	if err != nil {
		t.Fatal(err)
	}
	escribir(t, filepath.Join(dir, "reports/estandarizacion.json"), est)
	receta := `{"features": ["pm10","fraccion_fina","viento_velocidad","humedad"], "log1p": ["pm10"],
		"nivel": "` + nivel + `", "k": 5, "semilla": 42, "tolerancia": 1e-6, "max_iter": 100, "workers": 3, "chunk": 250}`
	escribir(t, filepath.Join(dir, "configs/kmeans.json"), []byte(receta))

	return opciones{
		receta:          filepath.Join(dir, "configs/kmeans.json"),
		gold:            filepath.Join(dir, "data/gold/oefa_features.csv"),
		estandarizacion: filepath.Join(dir, "reports/estandarizacion.json"),
		salida:          filepath.Join(dir, "reports/kmeans.json"),
		asignaciones:    filepath.Join(dir, "data/gold/asignaciones.csv"),
		regimenes:       filepath.Join(dir, "reports/regimenes.json"),
		codoSalida:      filepath.Join(dir, "reports/codo.json"),
		codoN:           1000,
	}, s
}

func escribir(t *testing.T, ruta string, b []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ruta, b, 0o600); err != nil {
		t.Fatal(err)
	}
}

func leerJSON(t *testing.T, ruta string, v any) {
	t.Helper()
	b, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("leer %s: %v", ruta, err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("%s no es JSON válido: %v", ruta, err)
	}
}

func TestEntrenarDeCaboARaboEnNivelAbsoluto(t *testing.T) {
	op, s := preparar(t, kmeans.NivelAbsoluto)
	var resumen bytes.Buffer
	if err := correr(op, &resumen); err != nil {
		t.Fatalf("correr: %v", err)
	}

	var km salidaKMeans
	leerJSON(t, op.salida, &km)
	if km.K != 5 || km.KEfectivo != 5 || km.Semilla != 42 || km.Workers != 3 || km.Chunk != 250 {
		t.Errorf("parámetros: k=%d k_ef=%d semilla=%d workers=%d chunk=%d", km.K, km.KEfectivo, km.Semilla, km.Workers, km.Chunk)
	}
	if km.Gold.N != s.Gold.Datos.N || km.Gold.Sha256 == "" || km.Gold.Estaciones != 4 {
		t.Errorf("gold: %+v", km.Gold)
	}
	if len(km.CentroidesEstandarizados) != 5 || len(km.CentroidesEstandarizados[0]) != 4 {
		t.Fatalf("centroides estandarizados: %v", km.CentroidesEstandarizados)
	}
	if len(km.CentroidesDesEstandarizados) != 5 {
		t.Fatalf("con nivel absoluto los centroides des-estandarizados existen: %v", km.CentroidesDesEstandarizados)
	}
	// El centroide des-estandarizado es exactamente la inversa del estandarizado.
	for j := range km.CentroidesEstandarizados {
		x := make([]float64, 4)
		if err := s.Est.Invertir(km.CentroidesEstandarizados[j], "", x); err != nil {
			t.Fatal(err)
		}
		for f := range x {
			if math.Float64bits(x[f]) != math.Float64bits(km.CentroidesDesEstandarizados[j][f]) {
				t.Errorf("centroide %d, feature %d: %v vs %v", j, f, km.CentroidesDesEstandarizados[j][f], x[f])
			}
		}
	}
	total := 0
	for _, n := range km.Tamanos {
		total += n
	}
	if total != s.Gold.Datos.N || km.Iteraciones == 0 || km.Paro == "" || !(km.Inercia > 0) {
		t.Errorf("resultado: tamaños suman %d, iteraciones %d, paro %q, inercia %v", total, km.Iteraciones, km.Paro, km.Inercia)
	}
	if km.Metadatos.VersionGo == "" {
		t.Error("faltan los metadatos de la máquina")
	}

	// Asignaciones: una fila por lectura, en el orden del gold.
	f, err := os.Open(op.asignaciones)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Scan()
	if sc.Text() != "estacion,fecha_hora,regimen" {
		t.Fatalf("cabecera de asignaciones: %q", sc.Text())
	}
	filas := 0
	for sc.Scan() {
		c := strings.Split(sc.Text(), ",")
		if len(c) != 3 {
			t.Fatalf("fila %d de asignaciones: %q", filas, sc.Text())
		}
		reg, err := strconv.Atoi(c[2])
		if err != nil || reg < 0 || reg >= 5 {
			t.Fatalf("fila %d: régimen %q", filas, c[2])
		}
		if c[0] != s.Gold.Estacion[filas] || c[1] != s.Gold.FechaHora[filas].Format("2006-01-02T15:04:05-07:00") {
			t.Fatalf("fila %d: %s %s, se esperaba %s %s", filas, c[0], c[1], s.Gold.Estacion[filas], s.Gold.FechaHora[filas])
		}
		filas++
	}
	if filas != s.Gold.Datos.N {
		t.Errorf("%d asignaciones, se esperaban %d", filas, s.Gold.Datos.N)
	}

	var rg salidaRegimenes
	leerJSON(t, op.regimenes, &rg)
	if len(rg.Regimenes) != 5 || rg.ECA.PM25 != 50 || rg.ECA.PM10 != 100 {
		t.Fatalf("regímenes: %d, ECA %+v", len(rg.Regimenes), rg.ECA)
	}
	// La etiqueta que llega al JSON es la de las medias del propio régimen.
	asign := leerAsignaciones(t, op.asignaciones)
	for _, r := range rg.Regimenes {
		if r.PM25Medio == nil || r.PM10Medio == nil || len(r.Perfil) != 4 {
			t.Fatalf("régimen %d sin perfil completo: %+v", r.Regimen, r)
		}
		if eca, _ := kmeans.EtiquetaECA(r.PM25Medio, r.PM10Medio); r.ECA != eca {
			t.Errorf("régimen %d: ECA %q con PM2.5 %.1f y PM10 %.1f", r.Regimen, r.ECA, *r.PM25Medio, *r.PM10Medio)
		}
		// Y en un régimen puro coincide con la del régimen plantado. Con una sola
		// semilla K-Means puede partir un régimen plantado en dos y juntar otros
		// dos (un óptimo local; la calibración usa 5 semillas por eso): solo se
		// compara donde el régimen encontrado es uno solo de los plantados.
		cuenta := map[int]int{}
		for i, a := range asign {
			if a == r.Regimen {
				cuenta[s.Plantado[i]]++
			}
		}
		for p, c := range cuenta {
			if float64(c) < 0.95*float64(r.Tamano) {
				continue
			}
			plan := s.Regimenes[p].Valor
			pm25, pm10 := plan["pm2_5"], plan["pm10"]
			if quiero, _ := kmeans.EtiquetaECA(&pm25, &pm10); r.ECA != quiero {
				t.Errorf("régimen %d (plantado %q): ECA %q, se esperaba %q", r.Regimen, s.Regimenes[p].Nombre, r.ECA, quiero)
			}
		}
	}
	for _, quiero := range []string{"régimen", "ECA", "inercia"} {
		if !strings.Contains(resumen.String(), quiero) {
			t.Errorf("el resumen no menciona %q:\n%s", quiero, resumen.String())
		}
	}
	if _, err := os.Stat(op.codoSalida); err == nil {
		t.Error("sin -codo no debería escribirse codo.json")
	}
}

func leerAsignaciones(t *testing.T, ruta string) []int {
	t.Helper()
	b, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatal(err)
	}
	var out []int
	for _, fila := range strings.Split(strings.TrimSpace(string(b)), "\n")[1:] {
		c := strings.Split(fila, ",")
		j, err := strconv.Atoi(c[len(c)-1])
		if err != nil {
			t.Fatalf("asignación %q", fila)
		}
		out = append(out, j)
	}
	return out
}

func TestEntrenarEnNivelEstacionNoDesEstandarizaLosCentroides(t *testing.T) {
	op, _ := preparar(t, kmeans.NivelEstacion)
	if err := correr(op, &bytes.Buffer{}); err != nil {
		t.Fatalf("correr: %v", err)
	}
	var km salidaKMeans
	leerJSON(t, op.salida, &km)
	if km.CentroidesDesEstandarizados != nil {
		t.Errorf("con nivel = estacion los centroides des-estandarizados no existen: %v", km.CentroidesDesEstandarizados)
	}
	if !strings.Contains(km.NotaDesEstandarizacion, "estacion") {
		t.Errorf("la nota debería explicar por qué no hay centroides en unidades: %q", km.NotaDesEstandarizacion)
	}
	// Los perfiles sí están en unidades: se invierten fila por fila con la
	// estadística de su estación.
	var rg salidaRegimenes
	leerJSON(t, op.regimenes, &rg)
	for _, r := range rg.Regimenes {
		if r.Tamano > 0 && (r.PM10Medio == nil || r.ECA == kmeans.ECANoEvaluable) {
			t.Errorf("régimen %d: el ECA se evalúa en absoluto también con nivel = estacion", r.Regimen)
		}
	}
}

func TestEntrenarConCodoEscribeLasInercias(t *testing.T) {
	op, _ := preparar(t, kmeans.NivelAbsoluto)
	op.codo = "2:6"
	if err := correr(op, &bytes.Buffer{}); err != nil {
		t.Fatalf("correr: %v", err)
	}
	var cd salidaCodo
	leerJSON(t, op.codoSalida, &cd)
	if len(cd.Puntos) != 5 || cd.Puntos[0].K != 2 || cd.Puntos[4].K != 6 {
		t.Fatalf("puntos del codo: %+v", cd.Puntos)
	}
	for i := 1; i < len(cd.Puntos); i++ {
		if !(cd.Puntos[i].Inercia < cd.Puntos[i-1].Inercia) {
			t.Errorf("la inercia no baja de k=%d a k=%d", cd.Puntos[i-1].K, cd.Puntos[i].K)
		}
	}
	if cd.Puntos[0].N != 1000 {
		t.Errorf("el codo usó %d filas, se pidió una submuestra de 1000", cd.Puntos[0].N)
	}
}

func TestEntrenarRechazaUnaEstandarizacionDeOtraReceta(t *testing.T) {
	op, s := preparar(t, kmeans.NivelAbsoluto)
	otra := *s.Est
	otra.Log1p = nil
	b, _ := json.Marshal(otra)
	escribir(t, op.estandarizacion, b)
	if err := correr(op, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "log1p") {
		t.Fatalf("se esperaba un error por log1p distinto, fue: %v", err)
	}
}

func TestEntrenarRechazaUnaEstandarizacionDeOtroGold(t *testing.T) {
	op, s := preparar(t, kmeans.NivelAbsoluto)
	otra := *s.Est
	otra.N = s.Gold.Datos.N + 1
	b, _ := json.Marshal(otra)
	escribir(t, op.estandarizacion, b)
	if err := correr(op, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "filas") {
		t.Fatalf("se esperaba un error por n distinto, fue: %v", err)
	}
}

func TestRangoK(t *testing.T) {
	a, b, err := rangoK("2:10")
	if err != nil || a != 2 || b != 10 {
		t.Fatalf("rangoK(2:10) = %d, %d, %v", a, b, err)
	}
	for _, malo := range []string{"", "2", "2-10", "a:3", "3:b", "5:2", "0:3"} {
		if _, _, err := rangoK(malo); err == nil {
			t.Errorf("rangoK(%q): se esperaba error", malo)
		}
	}
}
