package kmeans

import (
	"bytes"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

var featuresPrueba = []string{"pm10", "fraccion_fina", "viento_velocidad", "humedad"}

const cabeceraPrueba = "estacion,fecha_hora,pm10,fraccion_fina,viento_velocidad,humedad"

func goldDe(filas ...string) string {
	return cabeceraPrueba + "\n" + strings.Join(filas, "\n") + "\n"
}

func TestLeerGoldCargaFeaturesEstacionYFechaHora(t *testing.T) {
	csv := goldDe(
		"CA-01,2024-01-01T00:00:00-05:00,1.5,-0.25,0.125,2",
		"CA-01,2024-01-01T01:00:00-05:00,-1,0.5,3,4",
		"LO-02,2024-01-01T00:00:00-05:00,7,8,9,10",
	)
	g, err := LeerGoldDe(strings.NewReader(csv), featuresPrueba)
	if err != nil {
		t.Fatalf("LeerGoldDe: %v", err)
	}
	d := g.Datos
	if d.N != 3 || d.D != 4 || len(d.X) != 12 {
		t.Fatalf("N=%d D=%d len(X)=%d, se esperaban 3, 4 y 12", d.N, d.D, len(d.X))
	}
	if d.X[0] != 1.5 || d.X[3] != 2 || d.X[4] != -1 || d.X[11] != 10 {
		t.Errorf("X = %v", d.X)
	}
	if strings.Join(g.Estacion, ",") != "CA-01,CA-01,LO-02" {
		t.Errorf("Estacion = %v", g.Estacion)
	}
	quiero := time.Date(2024, 1, 1, 1, 0, 0, 0, ZonaPeru)
	if !g.FechaHora[1].Equal(quiero) || g.FechaHora[1].Hour() != 1 {
		t.Errorf("FechaHora[1] = %v, se esperaba %v", g.FechaHora[1], quiero)
	}
	if g.FechaHora[1].Format(time.RFC3339) != "2024-01-01T01:00:00-05:00" {
		t.Errorf("la fecha no vuelve a escribirse igual: %s", g.FechaHora[1].Format(time.RFC3339))
	}
	if strings.Join(g.Features, ",") != strings.Join(featuresPrueba, ",") {
		t.Errorf("Features = %v", g.Features)
	}
	if est := g.Estaciones(); strings.Join(est, ",") != "CA-01,LO-02" {
		t.Errorf("Estaciones() = %v", est)
	}
}

func TestLeerGoldRechazaCabeceraQueNoCoincideConLaReceta(t *testing.T) {
	fila := "\nCA-01,2024-01-01T00:00:00-05:00,1,2,3,4\n"
	casos := map[string]string{
		// Mismo número de columnas con dos features permutadas: si no se valida,
		// el K-Means corre sobre features equivocadas y nadie se entera.
		"features permutadas": "estacion,fecha_hora,fraccion_fina,pm10,viento_velocidad,humedad",
		"otra feature":        "estacion,fecha_hora,pm10,fraccion_fina,viento_velocidad,temperatura",
		"falta una feature":   "estacion,fecha_hora,pm10,fraccion_fina,viento_velocidad",
		"sobra una columna":   cabeceraPrueba + ",temperatura",
		"sin estacion":        "fecha_hora,estacion,pm10,fraccion_fina,viento_velocidad,humedad",
		"punto y coma":        strings.ReplaceAll(cabeceraPrueba, ",", ";"),
	}
	for nombre, cab := range casos {
		t.Run(nombre, func(t *testing.T) {
			_, err := LeerGoldDe(strings.NewReader(cab+fila), featuresPrueba)
			if err == nil {
				t.Fatal("se esperaba error por cabecera distinta de la receta")
			}
			if !strings.Contains(err.Error(), cabeceraPrueba) {
				t.Errorf("el error debería decir qué cabecera se esperaba, fue: %v", err)
			}
		})
	}
}

func TestLeerGoldRechazaFilasMalas(t *testing.T) {
	ok := "CA-01,2024-01-01T00:00:00-05:00,1,2,3,4"
	casos := map[string]struct {
		csv, debeDecir string
	}{
		"vacío":                 {"", "vacío"},
		"sin filas":             {cabeceraPrueba + "\n", "filas"},
		"columnas de menos":     {goldDe(ok, "CA-01,2024-01-01T01:00:00-05:00,1,2,3"), "línea 3"},
		"no numérico":           {goldDe("CA-01,2024-01-01T00:00:00-05:00,1,dos,3,4"), "fraccion_fina"},
		"NaN":                   {goldDe("CA-01,2024-01-01T00:00:00-05:00,NaN,2,3,4"), "no finito"},
		"Inf":                   {goldDe("CA-01,2024-01-01T00:00:00-05:00,1,2,+Inf,4"), "no finito"},
		"fecha inválida":        {goldDe("CA-01,2024-01-01 00:00,1,2,3,4"), "fecha_hora"},
		"fecha en UTC":          {goldDe("CA-01,2024-01-01T05:00:00Z,1,2,3,4"), "-05:00"},
		"fecha con otro offset": {goldDe("CA-01,2024-01-01T00:00:00-04:00,1,2,3,4"), "-05:00"},
		"estación vacía":        {goldDe(",2024-01-01T00:00:00-05:00,1,2,3,4"), "estación vacía"},
		"hora repetida":         {goldDe(ok, ok), "orden"},
		"hora hacia atrás": {goldDe("CA-01,2024-01-01T02:00:00-05:00,1,2,3,4",
			"CA-01,2024-01-01T01:00:00-05:00,1,2,3,4"), "orden"},
		"estación que reaparece": {goldDe(ok, "LO-02,2024-01-01T00:00:00-05:00,1,2,3,4",
			"CA-01,2024-01-01T05:00:00-05:00,1,2,3,4"), "CA-01"},
	}
	for nombre, c := range casos {
		t.Run(nombre, func(t *testing.T) {
			_, err := LeerGoldDe(strings.NewReader(c.csv), featuresPrueba)
			if err == nil {
				t.Fatal("se esperaba error")
			}
			if !strings.Contains(err.Error(), c.debeDecir) {
				t.Errorf("el error debería mencionar %q, fue: %v", c.debeDecir, err)
			}
		})
	}
}

func TestLeerGoldAceptaCRLFYFinalSinSaltoDeLinea(t *testing.T) {
	csv := cabeceraPrueba + "\r\nCA-01,2024-01-01T00:00:00-05:00,1,2,3,4\r\nCA-01,2024-01-01T01:00:00-05:00,1,2,3,4"
	g, err := LeerGoldDe(strings.NewReader(csv), featuresPrueba)
	if err != nil {
		t.Fatalf("LeerGoldDe: %v", err)
	}
	if g.Datos.N != 2 || g.Datos.X[7] != 4 {
		t.Errorf("N = %d, X = %v", g.Datos.N, g.Datos.X)
	}
}

func TestLeerGoldNoPierdePrecision(t *testing.T) {
	// El cargador escribe con FormatFloat(x, 'g', -1, 64), que es la
	// representación más corta que vuelve al mismo float64: la ida y vuelta
	// tiene que ser exacta.
	rng := rand.New(rand.NewSource(1))
	var b strings.Builder
	b.WriteString(cabeceraPrueba + "\n")
	var quiero []float64
	inicio := time.Date(2024, 1, 1, 0, 0, 0, 0, ZonaPeru)
	for i := 0; i < 200; i++ {
		b.WriteString("CA-01," + inicio.Add(time.Duration(i)*time.Hour).Format(time.RFC3339))
		for f := 0; f < 4; f++ {
			v := rng.NormFloat64() * math.Pow(10, float64(rng.Intn(9)-4))
			quiero = append(quiero, v)
			b.WriteString("," + strconv.FormatFloat(v, 'g', -1, 64))
		}
		b.WriteString("\n")
	}
	g, err := LeerGoldDe(strings.NewReader(b.String()), featuresPrueba)
	if err != nil {
		t.Fatalf("LeerGoldDe: %v", err)
	}
	for i, v := range quiero {
		if math.Float64bits(g.Datos.X[i]) != math.Float64bits(v) {
			t.Fatalf("valor %d: %.17g, se esperaba %.17g", i, g.Datos.X[i], v)
		}
	}
}

func TestEscribirGoldYLeerGoldSonInversos(t *testing.T) {
	s, err := GoldSintetico(OpcionesSintetico{Features: featuresPrueba, Log1p: []string{"pm10"},
		Nivel: NivelAbsoluto, Estaciones: 3, Horas: 50, Ruido: 0.1, Semilla: 1})
	if err != nil {
		t.Fatalf("GoldSintetico: %v", err)
	}
	ruta := filepath.Join(t.TempDir(), "gold.csv")
	var buf bytes.Buffer
	if err := EscribirGold(&buf, s.Gold); err != nil {
		t.Fatalf("EscribirGold: %v", err)
	}
	if err := os.WriteFile(ruta, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	g, err := LeerGold(ruta, featuresPrueba)
	if err != nil {
		t.Fatalf("LeerGold: %v", err)
	}
	if g.Datos.N != s.Gold.Datos.N {
		t.Fatalf("N = %d, se esperaban %d", g.Datos.N, s.Gold.Datos.N)
	}
	for i := range g.Datos.X {
		if math.Float64bits(g.Datos.X[i]) != math.Float64bits(s.Gold.Datos.X[i]) {
			t.Fatalf("X[%d] cambió en la ida y vuelta", i)
		}
	}
	for i := range g.Estacion {
		if g.Estacion[i] != s.Gold.Estacion[i] || !g.FechaHora[i].Equal(s.Gold.FechaHora[i]) {
			t.Fatalf("fila %d: %s %v vs %s %v", i, g.Estacion[i], g.FechaHora[i], s.Gold.Estacion[i], s.Gold.FechaHora[i])
		}
	}
}

func TestLeerGoldFallaSiNoExiste(t *testing.T) {
	if _, err := LeerGold(filepath.Join(t.TempDir(), "no.csv"), featuresPrueba); err == nil {
		t.Fatal("se esperaba error")
	}
}

func TestLeerGoldDecodificaLasComillasCSV(t *testing.T) {
	// El cargador entrecomilla una estación con coma, comilla o salto de línea
	// y dobla las comillas (RFC 4180).
	csv := cabeceraPrueba + "\n" +
		`"A,B",2024-01-01T00:00:00-05:00,1,2,3,4` + "\n" +
		`"dijo ""hola""",2024-01-01T00:00:00-05:00,5,6,7,8` + "\n" +
		"\"línea\npartida\",2024-01-01T00:00:00-05:00,9,10,11,12\n" +
		"CA-LI-01,2024-01-01T00:00:00-05:00,13,14,15,16\n"
	g, err := LeerGoldDe(strings.NewReader(csv), featuresPrueba)
	if err != nil {
		t.Fatalf("LeerGoldDe: %v", err)
	}
	if want := []string{"A,B", `dijo "hola"`, "línea\npartida", "CA-LI-01"}; !slices.Equal(g.Estacion, want) {
		t.Errorf("estaciones %q, se esperaban %q", g.Estacion, want)
	}
	if g.Datos.N != 4 || g.Datos.X[4] != 5 || g.Datos.X[15] != 16 {
		t.Errorf("N = %d, X = %v", g.Datos.N, g.Datos.X)
	}
	// Una comilla sin cerrar es un error con su línea, no una fila mal partida.
	_, err = LeerGoldDe(strings.NewReader(cabeceraPrueba+"\n"+`"A,B,2024-01-01T00:00:00-05:00,1,2,3,4`+"\n"), featuresPrueba)
	if err == nil {
		t.Error("comilla sin cerrar: se esperaba error")
	}
}

func TestEscribirGoldEntrecomillaLoQueHaceFalta(t *testing.T) {
	inicio := time.Date(2024, 1, 1, 0, 0, 0, 0, ZonaPeru)
	g := &Gold{
		Datos:     &Datos{X: []float64{1, 2, 3, 4, 5, 6, 7, 8}, N: 2, D: 4},
		Features:  featuresPrueba,
		Estacion:  []string{"A,B", `C"D`},
		FechaHora: []time.Time{inicio, inicio},
	}
	var buf bytes.Buffer
	if err := EscribirGold(&buf, g); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"A,B",`) || !strings.Contains(buf.String(), `"C""D",`) {
		t.Errorf("no se entrecomilló como RFC 4180:\n%s", buf.String())
	}
	leido, err := LeerGoldDe(&buf, featuresPrueba)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(leido.Estacion, g.Estacion) {
		t.Errorf("ida y vuelta: %q", leido.Estacion)
	}
}
