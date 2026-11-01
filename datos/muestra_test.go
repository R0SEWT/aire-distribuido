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

// La muestra sintética de data/sample/oefa_muestra.csv sale de aquí, con el
// esquema real del CSV de OEFA (spec §4.1). Es reproducible (semilla fija y
// PCG, que no depende de la versión de Go) y TestMuestraVersionadaAlDia exige
// que el archivo versionado sea byte a byte lo que genera este código:
//
//	go test ./datos -run TestMuestraVersionadaAlDia -actualizar
//
// Además del CSV, el generador devuelve la verdad: la hora que quiso darle a
// cada fila (no la que se deduce por posición), su valor en la unidad canónica
// y qué regla debe descartarla. Los tests comparan contra eso.

var actualizar = flag.Bool("actualizar", false, "regenera data/sample/oefa_muestra.csv")

const (
	rutaMuestra     = "../data/sample/oefa_muestra.csv"
	rutaMuestraReal = "../data/sample/oefa_real_muestra.csv"
	semillaMuestra  = 20261101
	diasMuestra     = 5
)

// columnasOEFA es la cabecera real, en orden.
var columnasOEFA = []string{
	"NOMBRE_EVALUACION", "COMPONENTE_AMBIENTAL", "PROCEDENCIA_MUESTRA", "NOMBRE_PUNTO", "ESTE", "NORTE",
	"ALTITUD", "ZONA", "DATUM", "DEPARTAMENTO", "PROVINCIA", "DISTRITO", "UBIGEO", "DESCRIPCION_UBICACION",
	"TIPO_MUESTRA", "TIPO_ANALISIS", "PERIODO", "FECHA_INICIO", "HORA_INICIO", "FECHA_FIN", "HORA_FIN",
	"UNIDAD_MEDIDA", "DIOXIDO_AZUFRE", "DIRECCION_VIENTO", "HUMEDAD_RELATIVA", "PM10", "PM2.5",
	"MONOXIDO_CARBONO", "OZONO", "PRECIPITACION", "PRESION_BAROMETRICA", "RADIACION_SOLAR",
	"SULFURO_HIDROGENO", "TEMPERATURA", "VELOCIDAD_VIENTO", "FECHA_CORTE",
}

// sinComillas son las columnas que el CSV real escribe sin comillas.
var sinComillas = map[string]bool{"ESTE": true, "NORTE": true, "ALTITUD": true, "ZONA": true, "UBIGEO": true,
	"FECHA_INICIO": true, "HORA_INICIO": true, "FECHA_FIN": true, "HORA_FIN": true, "FECHA_CORTE": true}

var cabeceraMuestra = func() string {
	var b strings.Builder
	for i, c := range columnasOEFA {
		if i > 0 {
			b.WriteByte(';')
		}
		b.WriteString(`"` + c + `"`)
	}
	return b.String()
}()

var inicioMuestra = time.Date(2024, 3, 4, 0, 0, 0, 0, time.FixedZone("-05", -5*3600))

type estacionSint struct {
	nombre, evaluacion, este, norte, altitud, zona, depto, prov, dist, ubigeo, descripcion string
}

var estacionesSint = []estacionSint{
	{"SINT-A", "Vigilancia sintética A", "276500", "8682300", "150", "18", "LIMA", "LIMA", "COMAS", "150110", "Punto sintético A, Lima."},
	{"SINT-B", "Vigilancia sintética B", "226000", "8187000", "2400", "19", "AREQUIPA", "AREQUIPA", "CERRO COLORADO", "40104", "Punto sintético B, Arequipa."},
	{"SINT-C", "Vigilancia sintética C", "401000", "8724000", "3750", "18", "JUNÍN", "YAULI", "LA OROYA", "120801", "Punto sintético C, Junín."},
	{"SINT-D", "Vigilancia sintética D", "552000", "9073000", "155", "18", "UCAYALI", "CORONEL PORTILLO", "CALLERIA", "250101", "Punto sintético D, Ucayali."},
}

// filaSint es una fila del CSV más lo que se sabe de ella.
type filaSint struct {
	campos map[string]string // columna → texto (sin comillas)
	// Verdad:
	regla    string // regla que la descarta antes del pivot, "" si llega al pivot
	est      string
	canon    string
	instante int64   // fecha_hora (fin del intervalo), si llega al pivot
	valor    float64 // en la unidad canónica, si llega al pivot
	prio     int     // 0 canónica, 1 convertida
	inferida bool
	truncada bool // fila malformada: se escribe con menos columnas
}

func (f filaSint) texto() string {
	cols := columnasOEFA
	if f.truncada {
		cols = cols[:10]
	}
	var b strings.Builder
	for i, c := range cols {
		if i > 0 {
			b.WriteByte(';')
		}
		v := f.campos[c]
		if sinComillas[c] {
			b.WriteString(v)
		} else {
			b.WriteString(`"` + v + `"`)
		}
	}
	return b.String()
}

// serie es (estación, parámetro, unidad, periodo) durante varios días: un
// bloque de 24 filas por día, como en el CSV real.
type serie struct {
	est      int
	columna  string
	canon    string
	unidad   string
	factor   float64 // a la canónica
	periodo  string
	tipo     string
	explicit bool // HORA_INICIO con la hora; si no, NA
	dias     []int
	valor    func(d, h int) float64
}

type muestraGenerada struct {
	csv        []byte
	filas      []filaSint
	leidas     int64
	descartes  map[string]int64
	conflictos int64
	convertida int64
	violac     int
	verdad     map[claveVerdad]*celdaVerdad
}

type claveVerdad struct {
	est string
	t   int64
}

type celdaVerdad struct {
	v          [numParametros]float64
	prio       [numParametros]int
	inferida   [numParametros]bool
	convertida [numParametros]bool
}

// pcg da uniformes sin depender de cómo rand/v2 implementa Float64 en cada versión.
type pcg struct{ *rand.PCG }

func (p pcg) unif() float64 { return float64(p.Uint64()>>11) / (1 << 53) }

func generarMuestra() *muestraGenerada {
	rng := pcg{rand.NewPCG(semillaMuestra, semillaMuestra^0x9E3779B97F4A7C15)}
	m := &muestraGenerada{descartes: map[string]int64{}, verdad: map[claveVerdad]*celdaVerdad{}}
	for _, r := range Reglas() {
		m.descartes[r] = 0
	}
	todos := []int{0, 1, 2, 3, 4}
	ciclo := func(h int) float64 { return math.Sin(2 * math.Pi * (float64(h) - 9) / 24) }
	r1 := func(x float64) float64 { return math.Round(x*10) / 10 }
	pm10 := map[[3]int]float64{} // (est, día, hora) → PM10, para que PM2.5 quede por debajo
	valPM10 := func(base float64, e int) func(d, h int) float64 {
		return func(d, h int) float64 {
			v := r1(base * (0.7 + 0.6*rng.unif() + 0.3*math.Sin(2*math.Pi*(float64(h)-4)/24)))
			pm10[[3]int{e, d, h}] = v
			return v
		}
	}
	valPM25 := func(e int) func(d, h int) float64 {
		return func(d, h int) float64 { return r1(pm10[[3]int{e, d, h}] * (0.35 + 0.35*rng.unif())) }
	}
	so2ppb := map[[2]int]float64{}
	gasPPB := func(base, amp float64, guardar map[[2]int]float64) func(d, h int) float64 {
		return func(d, h int) float64 {
			v := r1(base + amp*rng.unif())
			if guardar != nil {
				guardar[[2]int{d, h}] = v
			}
			return v
		}
	}
	gasUG := func(fuente map[[2]int]float64, f float64) func(d, h int) float64 {
		return func(d, h int) float64 { return r1(fuente[[2]int{d, h}] * f) }
	}
	o3ug, h2sppb := map[[2]int]float64{}, map[[2]int]float64{}
	meteo := func(base, amp, ruido float64) func(d, h int) float64 {
		return func(d, h int) float64 { return r1(base + amp*ciclo(h) + ruido*(rng.unif()-0.5)) }
	}
	hum := func(d, h int) float64 { return r1(math.Min(99, math.Max(5, 65-25*ciclo(h)+10*rng.unif()))) }
	viento := func(d, h int) float64 { return r1(0.5 + 4*rng.unif()) }
	dir := func(d, h int) float64 { return math.Round(360 * rng.unif()) }
	pm := "Concentración de partículas en "
	met := "Datos Meteorológicos"
	series := []serie{
		{0, "PM10", "pm10", "ug/m3", 1, "1 hora", pm + "PM10", false, todos, valPM10(80, 0)},
		{0, "PM2.5", "pm2_5", "ug/m3", 1, "1 hora", pm + "PM2.5", false, todos, valPM25(0)},
		{0, "DIOXIDO_AZUFRE", "so2", "ppb", 2.62, "1 hora", "-", false, todos, gasPPB(4, 8, so2ppb)},
		{0, "DIOXIDO_AZUFRE", "so2", "ug/m3", 1, "1 hora", "-", false, todos, gasUG(so2ppb, 2.62)},
		{0, "DIOXIDO_AZUFRE", "so2", "ug/m3", 1, "3 horas móviles", "-", false, []int{0}, gasPPB(10, 20, nil)},
		{0, "TEMPERATURA", "temperatura", "°c", 1, "-", met, false, todos, meteo(19, 4, 1)},
		{0, "HUMEDAD_RELATIVA", "humedad", "%", 1, "-", met, false, todos, hum},
		{0, "VELOCIDAD_VIENTO", "viento_velocidad", "m/s", 1, "-", met, false, todos, viento},
		{0, "DIRECCION_VIENTO", "viento_direccion", "°c", 1, "-", met, false, todos, dir},
		{0, "PRESION_BAROMETRICA", "presion", "mmHg", 1.33322, "-", met, false, todos, meteo(755, 1.5, 0.5)},
		{1, "PM10", "pm10", "ug/m3", 1, "1 hora", pm + "PM10", true, todos, valPM10(55, 1)},
		{1, "PM2.5", "pm2_5", "ug/m3", 1, "1 hora", pm + "PM2.5", true, todos, valPM25(1)},
		{1, "MONOXIDO_CARBONO", "co", "ppb", 1.145, "1 hora", "-", false, todos, gasPPB(300, 600, nil)},
		{1, "OZONO", "o3", "ug/m3", 1, "1 hora", "-", false, todos, gasPPB(20, 60, o3ug)},
		{1, "OZONO", "o3", "ppb", 1.96, "1 hora", "-", false, todos, gasUG(o3ug, 1/1.96)},
		{1, "TEMPERATURA", "temperatura", "°c", 1, "1 hora", met, false, todos, meteo(14, 7, 1)},
		{1, "HUMEDAD_RELATIVA", "humedad", "%", 1, "1 hora", met, false, todos, hum},
		{1, "VELOCIDAD_VIENTO", "viento_velocidad", "m/s", 1, "1 hora", met, false, todos, viento},
		{1, "PRESION_BAROMETRICA", "presion", "hPa", 1, "1 hora", met, false, todos, meteo(752, 2, 0.5)},
		{2, "PM10", "pm10", "ug/m3", 1, "1 hora", pm + "PM10", false, todos, valPM10(45, 2)},
		{2, "PM2.5", "pm2_5", "ug/m3", 1, "1 hora", pm + "PM2.5", false, todos, valPM25(2)},
		{2, "SULFURO_HIDROGENO", "h2s", "ppb", 1.39, "1 hora", "-", false, todos, gasPPB(1, 6, h2sppb)},
		{2, "SULFURO_HIDROGENO", "h2s", "ug/m3", 1, "1 hora", "-", false, todos, gasUG(h2sppb, 1.39)},
		{2, "TEMPERATURA", "temperatura", "°c", 1, "-", met, false, todos, meteo(3, 8, 1)},
		{2, "HUMEDAD_RELATIVA", "humedad", "%", 1, "-", met, false, todos, hum},
		{2, "VELOCIDAD_VIENTO", "viento_velocidad", "m/s", 1, "-", met, false, todos, viento},
		{3, "PM10", "pm10", "ug/m3", 1, "1 hora", pm + "PM10", false, todos, valPM10(30, 3)},
		{3, "PM2.5", "pm2_5", "ug/m3", 1, "1 hora", pm + "PM2.5", false, []int{0, 1, 2, 3, 4}, valPM25(3)},
		{3, "PM10", "pm10", "ug/m3", 1, "Media móvil de 8 horas", pm + "PM10", false, []int{0}, valPM10(30, 9)},
	}

	// Anomalías de valor, en su lugar dentro del día (la fila conserva su posición).
	type celdaSint struct {
		serie, d, h int
	}
	reemplazos := map[celdaSint][2]string{ // texto, regla
		{0, 1, 5}:   {"<LD", "valor_no_numerico"},
		{20, 0, 7}:  {"ND", "valor_no_numerico"},
		{3, 2, 4}:   {"<LD", "valor_no_numerico"}, // SO2 en ug: queda el de ppb, convertido
		{16, 2, 3}:  {"-999", "centinela"},
		{1, 2, 9}:   {"-3.5", "negativo"},
		{24, 1, 12}: {"120.5", "fuera_de_rango"},
		{0, 3, 1}:   {"2500", "fuera_de_rango"},
	}
	violaciones := map[[3]int]bool{{0, 0, 5}: true, {0, 0, 6}: true} // A, día 0: PM2.5 > PM10
	faltaUltima := map[[2]int]bool{{19, 2}: true, {23, 3}: true}     // (serie, día) con 23 filas: hora, o periodo si es «-»
	enCero := map[[2]int]bool{{22, 4}: true}                         // H2S en ug/m3 de C, día 4: todo en 0 (queda el de ppb)
	partido := map[[2]int]bool{{27, 2}: true}                        // PM2.5 de D, día 2: 24 filas en dos tramos, no contiguas

	var filas []filaSint
	base := func(e int) map[string]string {
		s := estacionesSint[e]
		c := map[string]string{
			"NOMBRE_EVALUACION": s.evaluacion, "COMPONENTE_AMBIENTAL": "Aire", "PROCEDENCIA_MUESTRA": "Aire ambiental",
			"NOMBRE_PUNTO": s.nombre, "ESTE": s.este, "NORTE": s.norte, "ALTITUD": s.altitud, "ZONA": s.zona,
			"DATUM": "WGS84", "DEPARTAMENTO": s.depto, "PROVINCIA": s.prov, "DISTRITO": s.dist, "UBIGEO": s.ubigeo,
			"DESCRIPCION_UBICACION": s.descripcion, "TIPO_MUESTRA": "Simple", "FECHA_FIN": "NA", "HORA_FIN": "NA",
			"FECHA_CORTE": "20240429",
		}
		for _, p := range columnasOEFA[22:35] {
			c[p] = "-"
		}
		return c
	}
	fechaDia := func(d int) string { return inicioMuestra.AddDate(0, 0, d).Format("2006-01-02") }
	instante := func(d, h int) int64 { return inicioMuestra.AddDate(0, 0, d).Add(time.Duration(h+1) * time.Hour).Unix() }
	horaTexto := func(h int) string {
		if h == 0 {
			return "1899-12-31"
		}
		return "1899-12-31 " + time.Date(0, 1, 1, h, 0, 0, 0, time.UTC).Format("15:04:05")
	}
	fila := func(si, d, h int) filaSint {
		s := series[si]
		c := base(s.est)
		c["TIPO_ANALISIS"], c["PERIODO"], c["UNIDAD_MEDIDA"] = s.tipo, s.periodo, s.unidad
		c["FECHA_INICIO"] = fechaDia(d)
		c["HORA_INICIO"] = "NA"
		if s.explicit {
			c["HORA_INICIO"] = horaTexto(h)
		}
		v := s.valor(d, h)
		if si == 1 && violaciones[[3]int{0, d, h}] {
			v = pm10[[3]int{0, d, h}] + 2
		}
		texto := strconv.FormatFloat(v, 'f', -1, 64)
		f := filaSint{campos: c, est: estacionesSint[s.est].nombre, canon: s.canon, instante: instante(d, h),
			inferida: !s.explicit}
		if s.factor != 1 {
			f.prio = 1
		}
		if s.periodo != "1 hora" && s.periodo != "-" {
			f.regla = "periodo"
		}
		if r, ok := reemplazos[celdaSint{si, d, h}]; ok {
			texto, f.regla = r[0], r[1]
		}
		if enCero[[2]int{si, d}] {
			texto, f.regla = "0", "bloque_en_cero"
		}
		c[s.columna] = texto
		if f.regla == "" {
			crudo, _ := strconv.ParseFloat(texto, 64)
			f.valor = crudo
			if s.factor != 1 {
				f.valor = float64(crudo * s.factor)
			}
		}
		return f
	}
	dia := func(si, d int) []filaSint {
		s := series[si]
		var out []filaSint
		for h := 0; h < 24; h++ {
			if faltaUltima[[2]int{si, d}] && h == 23 {
				continue
			}
			f := fila(si, d, h)
			// La hora (y antes, el periodo «-») va primero en la cascada que
			// las reglas de valor: un día incompleto o partido se descarta entero.
			switch {
			case faltaUltima[[2]int{si, d}] && s.periodo == "-":
				f.regla = "periodo"
			case faltaUltima[[2]int{si, d}] || partido[[2]int{si, d}]:
				f.regla = "hora"
			}
			out = append(out, f)
		}
		return out
	}

	for si, s := range series {
		for _, d := range s.dias {
			switch {
			case si == 27 && d == 2: // PM2.5 de D, día 2, partido en dos tramos de 12 filas
				dos := dia(si, 2)
				filas = append(filas, dos[:12]...)
				filas = append(filas, dia(si, 3)...)
				filas = append(filas, dos[12:]...)
			case si == 27 && d == 3:
				// ya va dentro del día 2
			default:
				filas = append(filas, dia(si, d)...)
			}
			if si == 21 && d == 0 { // C: un día de meteorología vacía (regla parametro)
				for h := 0; h < 24; h++ {
					c := base(2)
					c["TIPO_ANALISIS"], c["PERIODO"], c["UNIDAD_MEDIDA"] = met, "1 hora", "°c"
					c["FECHA_INICIO"], c["HORA_INICIO"] = fechaDia(1), "NA"
					filas = append(filas, filaSint{campos: c, regla: "parametro"}) // sin ningún valor
				}
			}
		}
	}
	// B: duplicados explícitos (exacto y con otro valor) y una unidad desconocida.
	for i := range filas {
		f := filas[i]
		if f.est == "SINT-B" && f.canon == "pm10" && f.instante == instante(3, 10) {
			dup := f
			dup.campos = clonar(f.campos)
			filas = slices.Insert(filas, i+1, dup)
			break
		}
	}
	for i := range filas {
		f := filas[i]
		if f.est == "SINT-B" && f.canon == "pm2_5" && f.instante == instante(3, 11) {
			dup := f
			dup.campos = clonar(f.campos)
			dup.valor = f.valor + 1
			dup.campos["PM2.5"] = strconv.FormatFloat(dup.valor, 'f', -1, 64)
			filas = slices.Insert(filas, i+1, dup)
			break
		}
	}
	// B: una hora que no está en punto (se descarta y su celda queda vacía).
	for i := range filas {
		f := &filas[i]
		if f.est == "SINT-B" && f.canon == "pm2_5" && f.instante == instante(1, 6) {
			f.campos["HORA_INICIO"] = "1899-12-31 06:30:00"
			f.regla = "hora_no_en_punto"
		}
	}
	// D: una fila con el nombre en minúsculas (misma estación) y un día con otras coordenadas.
	for i := range filas {
		f := &filas[i]
		if f.est == "SINT-D" && f.canon == "pm10" && f.instante == instante(1, 3) && f.regla == "" {
			f.campos["NOMBRE_PUNTO"] = "sint-d"
		}
		if f.est == "SINT-D" && f.canon == "pm10" && f.campos["FECHA_INICIO"] == fechaDia(4) {
			f.campos["NORTE"] = "9073100"
		}
	}
	extra := func(e int, mod func(c map[string]string), regla string) filaSint {
		c := base(e)
		c["TIPO_ANALISIS"], c["PERIODO"], c["UNIDAD_MEDIDA"] = pm+"PM10", "1 hora", "ug/m3"
		c["FECHA_INICIO"], c["HORA_INICIO"], c["PM10"] = fechaDia(0), "NA", "40.1"
		mod(c)
		return filaSint{campos: c, regla: regla}
	}
	filas = append(filas,
		extra(1, func(c map[string]string) {
			c["UNIDAD_MEDIDA"], c["HORA_INICIO"], c["FECHA_INICIO"] = "ppm", horaTexto(10), fechaDia(4)
		}, "unidad"),
		extra(3, func(c map[string]string) { c["PERIODO"] = "24 horas" }, "periodo"),
		extra(3, func(c map[string]string) {
			c["PERIODO"], c["FECHA_INICIO"] = "24 Horas", fechaDia(1)
			c["DESCRIPCION_UBICACION"] = "Punto sintético D, Ucayali,\r\r\nsegunda línea del campo."
		}, "periodo"),
		extra(0, func(c map[string]string) { c["NOMBRE_PUNTO"] = "" }, "estacion"),
		extra(0, func(c map[string]string) { c["PM2.5"] = "20.0" }, "parametro"),
		extra(0, func(c map[string]string) { c["FECHA_INICIO"] = "NA" }, "fecha"),
		extra(0, func(c map[string]string) { c["FECHA_INICIO"] = "2024-02-30" }, "fecha"),
		extra(0, func(c map[string]string) { c["FECHA_INICIO"] = "1999-12-31" }, "fecha_fuera_de_rango"),
	)
	trunca := extra(0, func(map[string]string) {}, "columnas")
	trunca.truncada = true
	filas = append(filas, trunca)

	// Verdad: lo que llega al pivot, aplicado en orden de archivo.
	for _, f := range filas {
		if f.regla != "" {
			m.descartes[f.regla]++
			continue
		}
		k := claveVerdad{f.est, f.instante}
		c := m.verdad[k]
		if c == nil {
			c = &celdaVerdad{}
			for i := range c.v {
				c.v[i] = math.NaN()
			}
			m.verdad[k] = c
		}
		q := indiceParametro(f.canon)
		poner := func() {
			c.v[q], c.prio[q], c.inferida[q], c.convertida[q] = f.valor, f.prio, f.inferida, f.prio == 1
		}
		switch {
		case math.IsNaN(c.v[q]):
			poner()
		case f.prio < c.prio[q]:
			m.descartes["duplicado_unidad"]++
			poner()
		case f.prio > c.prio[q]:
			m.descartes["duplicado_unidad"]++
		case f.valor == c.v[q]:
			m.descartes["duplicado_exacto"]++
		default:
			m.descartes["duplicado_valor_distinto"]++
			m.conflictos++
		}
	}
	for _, c := range m.verdad {
		for q := range c.v {
			if !math.IsNaN(c.v[q]) && c.convertida[q] {
				m.convertida++
			}
		}
		if c.v[idxPM25] > c.v[idxPM10] {
			m.violac++
		}
	}
	m.filas = filas
	m.leidas = int64(len(filas))

	var b bytes.Buffer
	b.WriteString(cabeceraMuestra + "\r\n")
	for _, f := range filas {
		b.WriteString(f.texto() + "\r\n")
	}
	m.csv = b.Bytes()
	return m
}

func clonar(m map[string]string) map[string]string {
	c := make(map[string]string, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}

// silverEsperado arma el silver que debe salir de la verdad sembrada, con el
// formato del contrato, sin pasar por el cargador.
func (m *muestraGenerada) silverEsperado() []byte {
	claves := make([]claveVerdad, 0, len(m.verdad))
	var presentes [numParametros]bool
	for k, c := range m.verdad {
		claves = append(claves, k)
		for q, v := range c.v {
			if !math.IsNaN(v) {
				presentes[q] = true
			}
		}
	}
	slices.SortFunc(claves, func(a, b claveVerdad) int {
		if c := strings.Compare(a.est, b.est); c != 0 {
			return c
		}
		return int(a.t - b.t)
	})
	var b bytes.Buffer
	b.WriteString("estacion,fecha_hora")
	for q, ok := range presentes {
		if ok {
			b.WriteString("," + ParametrosCanonicos[q])
		}
	}
	b.WriteString(",hora_inferida\n")
	for _, k := range claves {
		c := m.verdad[k]
		b.WriteString(k.est + "," + time.Unix(k.t, 0).In(inicioMuestra.Location()).Format(time.RFC3339))
		inferida := false
		for q, ok := range presentes {
			if !ok {
				continue
			}
			b.WriteByte(',')
			if v := c.v[q]; !math.IsNaN(v) {
				b.WriteString(strconv.FormatFloat(v, 'g', -1, 64))
				inferida = inferida || c.inferida[q]
			}
		}
		if inferida {
			b.WriteString(",1\n")
		} else {
			b.WriteString(",0\n")
		}
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
		if r == "hora_no_en_punto" || m.descartes[r] > 0 {
			continue
		}
		t.Errorf("la muestra no tiene ningún caso de la regla %q", r)
	}
	if m.descartes["hora_no_en_punto"] != 1 || m.conflictos == 0 || m.violac == 0 || m.convertida == 0 {
		t.Errorf("faltan casos: %+v, conflictos %d, violaciones %d, convertidas %d", m.descartes, m.conflictos, m.violac, m.convertida)
	}
	negativas := 0
	for _, c := range m.verdad {
		if c.v[indiceParametro("temperatura")] < 0 {
			negativas++
		}
	}
	if negativas == 0 {
		t.Error("la muestra tiene que tener temperaturas bajo cero válidas")
	}
}
