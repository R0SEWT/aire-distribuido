package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
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
		centroides:      filepath.Join(dir, "reports/centroides_iniciales.json"),
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

func TestEntrenarConUnaSemillaEsLaCorridaDeSiempre(t *testing.T) {
	// La receta de preparar no trae «semillas»: corre una sola, y el modelo
	// tiene que ser bit a bit el de k-means++ con la semilla 42 y el Lloyd
	// concurrente, como antes de que existieran las semillas.
	op, s := preparar(t, kmeans.NivelAbsoluto)
	if err := correr(op, &bytes.Buffer{}); err != nil {
		t.Fatalf("correr: %v", err)
	}
	var km salidaKMeans
	leerJSON(t, op.salida, &km)
	if km.Semillas != 1 || len(km.Arranques) != 1 || km.Semilla != 42 || km.SemillaBase != 42 || km.ArranqueElegido != 0 {
		t.Fatalf("semillas %d, arranques %d, semilla %d, base %d, elegido %d",
			km.Semillas, len(km.Arranques), km.Semilla, km.SemillaBase, km.ArranqueElegido)
	}
	d := s.Gold.Datos
	cent, _, err := kmeans.KMeansPP(d, 5, 42)
	if err != nil {
		t.Fatal(err)
	}
	r, err := kmeans.Concurrente(d, cent, kmeans.OpcionesConc{
		Opciones: kmeans.Opciones{K: 5, MaxIter: 100, TolAbs: 1e-6, TolRel: 1e-6}, Workers: 3, Chunk: 250})
	if err != nil {
		t.Fatal(err)
	}
	if math.Float64bits(km.Inercia) != math.Float64bits(r.Inercia) || km.Iteraciones != r.Iteraciones {
		t.Errorf("inercia %v en %d iteraciones; la corrida directa da %v en %d", km.Inercia, km.Iteraciones, r.Inercia, r.Iteraciones)
	}
	for j, c := range km.CentroidesEstandarizados {
		for f, v := range c {
			if math.Float64bits(v) != math.Float64bits(r.Centroide(j)[f]) {
				t.Fatalf("centroide %d, feature %d: %.17g vs %.17g", j, f, v, r.Centroide(j)[f])
			}
		}
	}
	if km.CentroidesInicialesSha != kmeans.HashCentroides(cent) || km.CentroidesIniciales.Sha256 != km.CentroidesInicialesSha {
		t.Error("el hash de los centroides iniciales no es el de k-means++ con la semilla 42")
	}
}

func TestEntrenarConDiezSemillasRecuperaLosRegimenesPlantados(t *testing.T) {
	// El caso del óptimo local: con la semilla 42 sola, K-Means parte «polvo» en
	// dos y junta «limpio» con «moderado». Con 10 semillas se queda con la de
	// menor inercia, que recupera los cinco regímenes y sus etiquetas.
	op, s := preparar(t, kmeans.NivelAbsoluto)
	op.semillas = 10
	var resumen bytes.Buffer
	if err := correr(op, &resumen); err != nil {
		t.Fatalf("correr: %v", err)
	}
	var km salidaKMeans
	leerJSON(t, op.salida, &km)
	if km.Semillas != 10 || len(km.Arranques) != 10 {
		t.Fatalf("semillas %d, %d arranques", km.Semillas, len(km.Arranques))
	}
	mejor := 0
	for i, a := range km.Arranques {
		if a.Semilla != 42+int64(i) {
			t.Errorf("arranque %d: semilla %d", i, a.Semilla)
		}
		if a.Inercia < km.Arranques[mejor].Inercia {
			mejor = i
		}
	}
	if km.ArranqueElegido != mejor || km.Semilla != km.Arranques[mejor].Semilla ||
		math.Float64bits(km.Inercia) != math.Float64bits(km.Arranques[mejor].Inercia) {
		t.Errorf("elegido %d (semilla %d, inercia %v); el de menor inercia es el %d", km.ArranqueElegido, km.Semilla, km.Inercia, mejor)
	}
	if !strings.Contains(resumen.String(), "← elegida") {
		t.Errorf("el resumen no marca la semilla elegida:\n%s", resumen.String())
	}

	asign := leerAsignaciones(t, op.asignaciones)
	var rg salidaRegimenes
	leerJSON(t, op.regimenes, &rg)
	plantados := map[int]bool{}
	excede := 0
	for _, r := range rg.Regimenes {
		cuenta := map[int]int{}
		for i, a := range asign {
			if a == r.Regimen {
				cuenta[s.Plantado[i]]++
			}
		}
		for p, c := range cuenta {
			if float64(c) >= 0.95*float64(r.Tamano) {
				plantados[p] = true
			}
		}
		if r.ECA == kmeans.ECAExcede {
			excede++
		}
	}
	if len(plantados) != 5 {
		t.Errorf("con 10 semillas se recuperan %d de 5 regímenes plantados", len(plantados))
	}
	if excede != 3 {
		t.Errorf("%d regímenes exceden el ECA, se plantaron 3 (polvo, combustión y episodio fuerte)", excede)
	}
}

func TestEntrenarElFlagSemillasPisaLaReceta(t *testing.T) {
	op, _ := preparar(t, kmeans.NivelAbsoluto)
	b, err := os.ReadFile(op.receta)
	if err != nil {
		t.Fatal(err)
	}
	escribir(t, op.receta, []byte(strings.Replace(string(b), `"k": 5`, `"k": 5, "semillas": 10`, 1)))
	op.semillas = 2
	if err := correr(op, &bytes.Buffer{}); err != nil {
		t.Fatalf("correr: %v", err)
	}
	var km salidaKMeans
	leerJSON(t, op.salida, &km)
	if km.Semillas != 2 || len(km.Arranques) != 2 || km.Receta.Semillas != 10 {
		t.Errorf("con -semillas 2 sobre una receta de 10: semillas %d, %d arranques, receta %d",
			km.Semillas, len(km.Arranques), km.Receta.Semillas)
	}
}

func TestEntrenarMaterializaYReusaLosCentroidesIniciales(t *testing.T) {
	op, _ := preparar(t, kmeans.NivelAbsoluto)
	op.semillas = 3
	if err := correr(op, &bytes.Buffer{}); err != nil {
		t.Fatalf("primera corrida: %v", err)
	}
	var primera salidaKMeans
	leerJSON(t, op.salida, &primera)
	if primera.CentroidesIniciales.Origen != "generados" || primera.CentroidesIniciales.Archivo != op.centroides {
		t.Fatalf("primera corrida: %+v", primera.CentroidesIniciales)
	}
	arch, err := kmeans.LeerInicios(op.centroides)
	if err != nil {
		t.Fatalf("no se materializaron los centroides iniciales: %v", err)
	}
	if len(arch.Inicios) != 3 || arch.GoldSha256 != primera.Gold.Sha256 {
		t.Fatalf("archivo: %d semillas, gold %s", len(arch.Inicios), arch.GoldSha256)
	}

	// La segunda corrida parte del archivo y da lo mismo bit a bit.
	if err := correr(op, &bytes.Buffer{}); err != nil {
		t.Fatalf("segunda corrida: %v", err)
	}
	var segunda salidaKMeans
	leerJSON(t, op.salida, &segunda)
	if segunda.CentroidesIniciales.Origen != "leidos" {
		t.Errorf("la segunda corrida debió leer el archivo: %+v", segunda.CentroidesIniciales)
	}
	if math.Float64bits(segunda.Inercia) != math.Float64bits(primera.Inercia) || segunda.Semilla != primera.Semilla ||
		!reflect.DeepEqual(segunda.CentroidesEstandarizados, primera.CentroidesEstandarizados) {
		t.Errorf("desde el archivo: inercia %v semilla %d; con k-means++: %v %d", segunda.Inercia, segunda.Semilla, primera.Inercia, primera.Semilla)
	}

	// El archivo manda: si la semilla 42 trae los centroides de la 43, la
	// corrida parte de esos.
	arch.Inicios[0].Centroides, arch.Inicios[0].Sha256 = arch.Inicios[1].Centroides, arch.Inicios[1].Sha256
	if err := kmeans.GuardarInicios(op.centroides, arch); err != nil {
		t.Fatal(err)
	}
	op.semillas = 1
	if err := correr(op, &bytes.Buffer{}); err != nil {
		t.Fatalf("tercera corrida: %v", err)
	}
	var tercera salidaKMeans
	leerJSON(t, op.salida, &tercera)
	if tercera.CentroidesIniciales.Sha256 != arch.Inicios[1].Sha256 {
		t.Errorf("la corrida no partió del archivo: sha256 %s, el archivo dice %s", tercera.CentroidesIniciales.Sha256, arch.Inicios[1].Sha256)
	}

	// Pedir semillas que el archivo no tiene, o correr sobre otro gold, falla
	// y dice qué hacer.
	op.semillas = 5
	if err := correr(op, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "bórralo") {
		t.Errorf("semillas que el archivo no tiene: %v", err)
	}
	op.semillas = 1
	otro, _ := preparar(t, kmeans.NivelAbsoluto)
	otro.centroides = op.centroides
	b, err := os.ReadFile(otro.gold)
	if err != nil {
		t.Fatal(err)
	}
	escribir(t, otro.gold, append(b, []byte("EST99,2024-01-01T00:00:00-05:00,0,0,0,0\n")...))
	var e kmeans.Estandarizacion
	leerJSON(t, otro.estandarizacion, &e)
	e.N++
	e2, _ := json.Marshal(e)
	escribir(t, otro.estandarizacion, e2)
	if err := correr(otro, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "otra corrida") {
		t.Errorf("centroides iniciales de otro gold: %v", err)
	}

	// Sin archivo (-centroides ""), no se lee ni se escribe nada.
	sin, _ := preparar(t, kmeans.NivelAbsoluto)
	sin.centroides = ""
	if err := correr(sin, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	var km salidaKMeans
	leerJSON(t, sin.salida, &km)
	if km.CentroidesIniciales.Origen != "sin_archivo" {
		t.Errorf("con -centroides vacío: %+v", km.CentroidesIniciales)
	}
}

func TestRegimenesDicenLaConvencionDeLaHora(t *testing.T) {
	op, _ := preparar(t, kmeans.NivelAbsoluto)
	if err := correr(op, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	var rg salidaRegimenes
	leerJSON(t, op.regimenes, &rg)
	if rg.ConvencionHora != "fin" || !strings.Contains(rg.NotaPerfil, "00:00–01:00") {
		t.Errorf("convencion_hora %q, nota %q", rg.ConvencionHora, rg.NotaPerfil)
	}
	var km salidaKMeans
	leerJSON(t, op.salida, &km)
	if !strings.Contains(km.NotaTiemposArranques, "benchmark") {
		t.Errorf("kmeans.json tiene que advertir que los ms por semilla no sirven para speedup: %q", km.NotaTiemposArranques)
	}
}
