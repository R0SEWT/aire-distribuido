package datos

import (
	"bytes"
	"flag"
	"math"
	"math/rand/v2"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// La muestra sintética de data/sample/ sale de aquí. Es reproducible (semilla
// fija, PCG y un Fisher-Yates propio, que no dependen de la versión de Go) y
// TestMuestraVersionadaAlDia exige que el archivo versionado sea byte a byte lo
// que genera este código. Para regenerarla:
//
//	go test ./datos -run TestMuestraVersionadaAlDia -actualizar
//
// Además del CSV, el generador devuelve la verdad: cuántas filas debe descartar
// cada regla y qué valor debe quedar en cada celda del silver. Así los tests
// comparan contra lo que se sembró, no contra lo que el cargador produjo ayer.

var actualizar = flag.Bool("actualizar", false, "regenera data/sample/oefa_muestra.csv")

const (
	rutaMuestra     = "../data/sample/oefa_muestra.csv"
	semillaMuestra  = 20261031
	horasMuestra    = 7 * 24
	cabeceraMuestra = "ID_INFORME;PROCEDENCIA_MUESTRA;NOMBRE_PUNTO;DEPARTAMENTO;PROVINCIA;DISTRITO;UBIGEO;" +
		"ESTE;NORTE;ALTITUD;ZONA;DATUM;PARAMETRO;UNIDAD;FECHA;VALOR;PERIODO"
)

var inicioMuestra = time.Date(2024, 3, 4, 0, 0, 0, 0, time.FixedZone("-05", -5*3600))

type paramSint struct {
	real, unidad, canon string
	factor              float64 // de la unidad de la muestra a la canónica
	decimales           int
}

type estacionSint struct {
	nombre, informe, depto, prov, dist, ubigeo, este, norte, altitud, zona string
	formatoFecha                                                           string
	params                                                                 []paramSint
	pm10Base, tempBase, tempAmp                                            float64
	// falta dice si la estación no reporta ese parámetro a esa hora (huecos).
	falta func(canon string, h int) bool
}

var (
	pPM25   = paramSint{"PM2.5", "ug/m3", "pm2_5", 1, 1}
	pPM10   = paramSint{"PM10", "ug/m3", "pm10", 1, 1}
	pNO2    = paramSint{"NO2", "ug/m3", "no2", 1, 1}
	pSO2    = paramSint{"SO2", "ug/m3", "so2", 1, 1}
	pCOug   = paramSint{"CO", "ug/m3", "co", 1, 1}
	pCOmg   = paramSint{"CO", "mg/m3", "co", 1000, 2}
	pTemp   = paramSint{"Temperatura", "°C", "temperatura", 1, 1}
	pHum    = paramSint{"Humedad Relativa", "%", "humedad", 1, 1}
	pViento = paramSint{"Velocidad del Viento", "m/s", "viento_velocidad", 1, 1}
)

func estacionesMuestra() []estacionSint {
	nunca := func(string, int) bool { return false }
	return []estacionSint{
		{
			nombre: "SINT-A Lima Norte", informe: "INF-SINT-001", depto: "LIMA", prov: "LIMA", dist: "COMAS",
			ubigeo: "150110", este: "276500", norte: "8682300", altitud: "150", zona: "18L",
			formatoFecha: "02/01/2006 15:04",
			params:       []paramSint{pPM25, pPM10, pNO2, pSO2, pCOug, pTemp, pHum, pViento},
			pm10Base:     80, tempBase: 19, tempAmp: 4, falta: nunca,
		},
		{
			nombre: "SINT-B Arequipa", informe: "INF-SINT-002", depto: "AREQUIPA", prov: "AREQUIPA", dist: "CERRO COLORADO",
			ubigeo: "040104", este: "226000", norte: "8187000", altitud: "2400", zona: "19K",
			formatoFecha: "02/01/2006 15:04",
			params:       []paramSint{pPM25, pPM10, pNO2, pCOmg, pTemp, pHum, pViento},
			pm10Base:     55, tempBase: 14, tempAmp: 7, falta: nunca,
		},
		{
			// Estación de altura: temperaturas bajo cero de noche (válidas) y huecos:
			// un día entero apagada (horas 40-63) y sin viento las horas 100-111.
			nombre: "SINT-C Junín", informe: "INF-SINT-003", depto: "JUNÍN", prov: "YAULI", dist: "LA OROYA",
			ubigeo: "120801", este: "401000", norte: "8724000", altitud: "3750", zona: "18L",
			formatoFecha: "02/01/2006 15:04",
			params:       []paramSint{pPM25, pPM10, pSO2, pTemp, pHum, pViento},
			pm10Base:     45, tempBase: 3, tempAmp: 8,
			falta: func(canon string, h int) bool {
				return (h >= 40 && h < 64) || (canon == "viento_velocidad" && h >= 100 && h < 112)
			},
		},
		{
			// Fecha con segundos (segundo formato de la config) y sin PM2.5 las horas 120-130.
			nombre: "SINT-D Ucayali", informe: "INF-SINT-004", depto: "UCAYALI", prov: "CORONEL PORTILLO", dist: "CALLERIA",
			ubigeo: "250101", este: "552000", norte: "9073000", altitud: "155", zona: "18L",
			formatoFecha: "02/01/2006 15:04:05",
			params:       []paramSint{pPM25, pPM10, pTemp, pHum, pViento},
			pm10Base:     30, tempBase: 26, tempAmp: 5,
			falta: func(canon string, h int) bool {
				return canon == "pm2_5" && h >= 120 && h <= 130
			},
		},
	}
}

// celdaSint identifica una lectura sembrada: (estación, parámetro canónico, hora).
type celdaSint struct {
	est   int
	canon string
	h     int
}

// Anomalías que reemplazan el valor (o la unidad) de una lectura normal: la
// fila sigue en el archivo pero su valor no debe llegar al silver.
var reemplazosMuestra = []struct {
	celda         celdaSint
	valor, unidad string
	regla         string
}{
	{celdaSint{0, "pm10", 10}, "-999", "", "centinela"},
	{celdaSint{1, "humedad", 20}, "9999", "", "centinela"},
	{celdaSint{3, "temperatura", 30}, "-9999", "", "centinela"},
	{celdaSint{0, "no2", 11}, "", "", "valor_no_numerico"},
	{celdaSint{2, "so2", 12}, "ND", "", "valor_no_numerico"},
	{celdaSint{3, "pm10", 13}, "NaN", "", "valor_no_numerico"},
	{celdaSint{0, "pm2_5", 14}, "-5.0", "", "negativo"},
	{celdaSint{1, "viento_velocidad", 15}, "-0.4", "", "negativo"},
	{celdaSint{0, "pm2_5", 16}, "1500", "", "fuera_de_rango"},
	{celdaSint{2, "humedad", 17}, "120.5", "", "fuera_de_rango"},
	{celdaSint{3, "viento_velocidad", 18}, "80", "", "fuera_de_rango"},
	{celdaSint{0, "no2", 19}, "", "ppb", "unidad"},
	{celdaSint{1, "co", 21}, "", "ppm", "unidad"},
}

// Duplicados: los exactos copian la línea; los de valor distinto cambian el
// valor. «junto» los pone justo después del original; si no, al final.
var duplicadosMuestra = []struct {
	celda    celdaSint
	distinto bool
	junto    bool
}{
	{celdaSint{0, "pm10", 50}, false, true},
	{celdaSint{1, "temperatura", 60}, false, true},
	{celdaSint{2, "pm2_5", 70}, false, false},
	{celdaSint{3, "humedad", 80}, false, false},
	{celdaSint{0, "humedad", 90}, true, true},
	{celdaSint{1, "pm10", 91}, true, false},
	{celdaSint{3, "temperatura", 92}, true, false},
}

// Horas en que PM2.5 supera a PM10 (error de medición que se marca, no se descarta).
var violacionesMuestra = []celdaSint{{0, "", 100}, {0, "", 101}, {3, "", 102}, {1, "", 103}}

// muestraGenerada es el CSV más la verdad que permite verificar el cargador.
type muestraGenerada struct {
	csv        []byte
	leidas     int64
	descartes  map[string]int64 // regla → filas largas que debe descartar
	conflictos int64            // celdas marcadas por duplicado con valor distinto
	violac     int              // filas anchas con pm2_5 > pm10
	estaciones []string
	verdad     map[claveVerdad]*[numParametros]float64
}

type claveVerdad struct {
	est string
	h   int
}

// pcg envuelve el generador para tener uniformes y enteros sin depender de
// cómo rand/v2 implementa Float64 o Shuffle en cada versión de Go.
type pcg struct{ *rand.PCG }

func (p pcg) unif() float64 { return float64(p.Uint64()>>11) / (1 << 53) }
func (p pcg) entero(n int) int {
	return int(p.Uint64() % uint64(n))
}

func generarMuestra() *muestraGenerada {
	rng := pcg{rand.NewPCG(semillaMuestra, semillaMuestra^0x9E3779B97F4A7C15)}
	ests := estacionesMuestra()
	m := &muestraGenerada{
		descartes: map[string]int64{},
		verdad:    map[claveVerdad]*[numParametros]float64{},
	}
	for _, r := range Reglas() {
		m.descartes[r] = 0
	}
	for _, e := range ests {
		m.estaciones = append(m.estaciones, e.nombre)
	}

	reemplazo := map[celdaSint]int{}
	for i, r := range reemplazosMuestra {
		reemplazo[r.celda] = i
	}
	violacion := map[[2]int]bool{}
	for _, v := range violacionesMuestra {
		violacion[[2]int{v.est, v.h}] = true
	}

	linea := func(e estacionSint, p paramSint, fecha, valor, periodo string) string {
		return strings.Join([]string{e.informe, "Vigilancia ambiental (sintético)", e.nombre, e.depto, e.prov,
			e.dist, e.ubigeo, e.este, e.norte, e.altitud, e.zona, "WGS84", p.real, p.unidad, fecha, valor, periodo}, ";")
	}
	fijar := func(est string, h int, canon string, v float64) {
		k := claveVerdad{est, h}
		fila := m.verdad[k]
		if fila == nil {
			fila = new([numParametros]float64)
			for i := range fila {
				fila[i] = math.NaN()
			}
			m.verdad[k] = fila
		}
		fila[indiceParametro(canon)] = v
	}

	var lineas []string
	original := map[celdaSint]string{}
	for ei, e := range ests {
		for h := 0; h < horasMuestra; h++ {
			hd := float64(h % 24)
			ciclo := math.Sin(2 * math.Pi * (hd - 9) / 24) // máximo a las 15 h
			pm10 := e.pm10Base * (0.7 + 0.6*rng.unif() + 0.3*math.Sin(2*math.Pi*(hd-4)/24))
			pm10 = math.Round(pm10*10) / 10
			frac := 0.35 + 0.35*rng.unif()
			pm25 := math.Round(frac*pm10*10) / 10
			if violacion[[2]int{ei, h}] {
				pm25 = pm10 + 3
			}
			valores := map[string]float64{
				"pm10":             pm10,
				"pm2_5":            pm25,
				"no2":              20 + 40*rng.unif(),
				"so2":              10 + 20*rng.unif() + 300*rng.unif()*b2f(ei == 2),
				"co":               400 + 800*rng.unif(),
				"temperatura":      e.tempBase + e.tempAmp*ciclo + rng.unif() - 0.5,
				"humedad":          math.Min(99, math.Max(5, 65-25*ciclo+10*rng.unif())),
				"viento_velocidad": 0.5 + 4*rng.unif(),
			}
			fecha := inicioMuestra.Add(time.Duration(h) * time.Hour).Format(e.formatoFecha)
			for _, p := range e.params {
				if e.falta(p.canon, h) {
					continue
				}
				v := valores[p.canon] / p.factor
				texto := strconv.FormatFloat(v, 'f', p.decimales, 64)
				unidad := p.unidad
				celda := celdaSint{ei, p.canon, h}
				if i, ok := reemplazo[celda]; ok {
					r := reemplazosMuestra[i]
					if r.unidad != "" {
						unidad = r.unidad
					} else {
						texto = r.valor
					}
					m.descartes[r.regla]++
				} else {
					parseado, _ := strconv.ParseFloat(texto, 64)
					fijar(e.nombre, h, p.canon, parseado*p.factor)
				}
				pp := p
				pp.unidad = unidad
				l := linea(e, pp, fecha, texto, "Horario")
				original[celda] = l
				lineas = append(lineas, l)
			}
		}
	}

	// Filas extra que no tocan el silver.
	a, d := ests[0], ests[3]
	dia := func(n int) string {
		return inicioMuestra.Add(time.Duration(n) * 24 * time.Hour).Format("02/01/2006 15:04")
	}
	for n := 0; n < 7; n++ {
		lineas = append(lineas, linea(a, pPM10, dia(n), "61.2", "Diario"))
		lineas = append(lineas, linea(d, pPM25, dia(n), "12.4", "Diario"))
		m.descartes["periodo"] += 2
	}
	lineas = append(lineas, linea(a, pPM10, dia(0), "58.0", "Mensual"))
	m.descartes["periodo"]++
	for _, f := range []string{"31/02/2024 10:00", "2024/03/04 25:00", ""} {
		lineas = append(lineas, linea(a, pPM10, f, "40.1", "Horario"))
		m.descartes["fecha"]++
	}
	for _, f := range []string{"01/01/1999 00:00", "15/03/2030 12:00"} {
		lineas = append(lineas, linea(a, pPM10, f, "40.1", "Horario"))
		m.descartes["fecha_fuera_de_rango"]++
	}
	benceno := paramSint{"Benceno", "ug/m3", "", 1, 1}
	for h := 0; h < 5; h++ {
		lineas = append(lineas, linea(a, benceno, inicioMuestra.Add(time.Duration(h)*time.Hour).Format(a.formatoFecha), "1.2", "Horario"))
		m.descartes["parametro"]++
	}
	sinNombre := a
	sinNombre.nombre = "   "
	lineas = append(lineas, linea(sinNombre, pPM10, dia(1), "40.1", "Horario"))
	m.descartes["estacion"]++
	lineas = append(lineas, "INF-SINT-001;Vigilancia ambiental (sintético);SINT-A Lima Norte;LIMA")
	lineas = append(lineas, linea(a, pPM10, dia(2), "40.1", "Horario")+";sobra")
	m.descartes["columnas"] += 2

	// Orden del archivo: mezclado, como un CSV que no viene ordenado por estación.
	for i := len(lineas) - 1; i > 0; i-- {
		j := rng.entero(i + 1)
		lineas[i], lineas[j] = lineas[j], lineas[i]
	}

	var alFinal []string
	for _, dup := range duplicadosMuestra {
		orig := original[dup.celda]
		copia := orig
		if dup.distinto {
			campos := strings.Split(orig, ";")
			v, _ := strconv.ParseFloat(campos[15], 64)
			campos[15] = strconv.FormatFloat(v+1, 'f', 1, 64)
			copia = strings.Join(campos, ";")
			m.descartes["duplicado_valor_distinto"]++
			m.conflictos++
		} else {
			m.descartes["duplicado_exacto"]++
		}
		if dup.junto {
			i := slices.Index(lineas, orig)
			lineas = slices.Insert(lineas, i+1, copia)
		} else {
			alFinal = append(alFinal, copia)
		}
	}
	lineas = append(lineas, alFinal...)
	m.violac = len(violacionesMuestra)
	m.leidas = int64(len(lineas))

	var b bytes.Buffer
	b.Write(bomUTF8)
	b.WriteString(cabeceraMuestra)
	b.WriteByte('\n')
	for _, l := range lineas {
		b.WriteString(l)
		b.WriteByte('\n')
	}
	m.csv = b.Bytes()
	return m
}

func b2f(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// silverEsperado arma el silver que debe salir de la verdad sembrada, con el
// formato del contrato, sin pasar por el cargador.
func (m *muestraGenerada) silverEsperado() []byte {
	claves := make([]claveVerdad, 0, len(m.verdad))
	var presentes [numParametros]bool
	for k, fila := range m.verdad {
		claves = append(claves, k)
		for p, v := range fila {
			if !math.IsNaN(v) {
				presentes[p] = true
			}
		}
	}
	slices.SortFunc(claves, func(a, b claveVerdad) int {
		if c := strings.Compare(a.est, b.est); c != 0 {
			return c
		}
		return a.h - b.h
	})
	var b bytes.Buffer
	b.WriteString("estacion,fecha_hora")
	for p, ok := range presentes {
		if ok {
			b.WriteString("," + ParametrosCanonicos[p])
		}
	}
	b.WriteByte('\n')
	for _, k := range claves {
		b.WriteString(k.est)
		b.WriteByte(',')
		b.WriteString(inicioMuestra.Add(time.Duration(k.h) * time.Hour).Format(time.RFC3339))
		for p, ok := range presentes {
			if !ok {
				continue
			}
			b.WriteByte(',')
			if v := m.verdad[k][p]; !math.IsNaN(v) {
				b.WriteString(strconv.FormatFloat(v, 'g', -1, 64))
			}
		}
		b.WriteByte('\n')
	}
	return b.Bytes()
}

func TestMuestraVersionadaAlDia(t *testing.T) {
	m := generarMuestra()
	if *actualizar {
		if err := os.WriteFile(rutaMuestra, m.csv, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("muestra regenerada: %d filas largas, %d bytes", m.leidas, len(m.csv))
	}
	versionada, err := os.ReadFile(rutaMuestra)
	if err != nil {
		t.Fatalf("%v (genérala con: go test ./datos -run TestMuestraVersionadaAlDia -actualizar)", err)
	}
	if !bytes.Equal(versionada, m.csv) {
		t.Fatalf("data/sample/oefa_muestra.csv no es lo que genera muestra_test.go; regenérala con -actualizar")
	}
	if m.leidas < 3000 || m.leidas > 5000 {
		t.Errorf("la muestra tiene %d filas largas; el diseño pide entre 3000 y 5000", m.leidas)
	}
}

// Comprobación de la propia muestra: que el generador siembre lo que dice.
func TestMuestraSiembraCadaRegla(t *testing.T) {
	m := generarMuestra()
	for _, r := range Reglas() {
		if m.descartes[r] == 0 {
			t.Errorf("la muestra no tiene ningún caso de la regla %q", r)
		}
	}
	if m.conflictos == 0 || m.violac == 0 {
		t.Error("la muestra tiene que tener duplicados con valor distinto y violaciones pm2_5 > pm10")
	}
	negativas := 0
	for _, fila := range m.verdad {
		if fila[indiceParametro("temperatura")] < 0 {
			negativas++
		}
	}
	if negativas == 0 {
		t.Error("la muestra tiene que tener temperaturas bajo cero válidas")
	}
}
