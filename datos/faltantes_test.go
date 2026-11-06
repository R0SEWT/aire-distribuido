package datos

import (
	"encoding/json"
	"math"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// Pruebas de los tres hallazgos del análisis de procedencia (docs/eda-procedencia.md):
// el 0 exacto como «sin dato» hora a hora, las filas vacías de la plantilla de
// 2022 que ocupan su hora, y la zona UTM mal cargada de CA-HUAR-01.

// vaciar deja la fila sin ningún valor: todas las columnas de parámetro en «-».
func vaciar(c map[string]string) {
	for _, p := range columnasOEFA[22:35] {
		c[p] = "-"
	}
}

// diaPM10 son las 24 filas sin hora de un día de PM10 de la estación A, con
// PM10 = 10 + hora; mod ajusta la fila de la hora h.
func diaPM10(mod func(h int, c map[string]string)) []string {
	return dia24(24, func(h int, c map[string]string) {
		c["NOMBRE_PUNTO"], c["TIPO_ANALISIS"] = "A", "Concentración de partículas en PM10"
		c["PM10"] = strconvF(float64(10 + h))
		if mod != nil {
			mod(h, c)
		}
	})
}

func lineasSilver(t *testing.T, res *Resultado) []string {
	t.Helper()
	return strings.Split(strings.TrimSpace(string(silverDe(t, res))), "\n")
}

func TestCeroComoFaltante(t *testing.T) {
	// Horas sueltas en 0 dentro de un día con datos: se descartan y se cuentan.
	ceros := diaPM10(func(h int, c map[string]string) {
		if h == 3 || h == 7 {
			c["PM10"] = "0"
		}
	})
	res := cargarBytes(t, csvOEFA(ceros...), ConfigPorDefecto(), RecetaProvisional(), opMuestra)
	fl := res.Limpieza.FilasLargas
	if fl.DescartadasPorRegla["cero_como_faltante"] != 2 || fl.CeroComoFaltantePorParametro["pm10"] != 2 ||
		fl.Procesadas != 22 || fl.DescartadasPorRegla["bloque_en_cero"] != 0 {
		t.Fatalf("dos horas en 0: %+v", fl)
	}
	s := string(silverDe(t, res))
	if strings.Contains(s, "T04:00:00") || strings.Contains(s, "T08:00:00") || !strings.Contains(s, "A,2024-03-04T05:00:00-05:00,14,1") {
		t.Errorf("las horas en 0 no llegan al silver y las demás conservan su hora:\n%s", s)
	}
	// El día entero en 0 sigue siendo bloque_en_cero (va antes en la cascada).
	todo := diaPM10(func(_ int, c map[string]string) { c["PM10"] = "0" })
	res = cargarBytes(t, csvOEFA(todo...), ConfigPorDefecto(), RecetaProvisional(), opMuestra)
	if fl := res.Limpieza.FilasLargas; fl.DescartadasPorRegla["bloque_en_cero"] != 24 || fl.DescartadasPorRegla["cero_como_faltante"] != 0 {
		t.Errorf("día entero en 0: %+v", fl.DescartadasPorRegla)
	}
	// Sin la regla, el 0 es una lectura.
	cfg := ConfigPorDefecto()
	cfg.CeroComoFaltante = nil
	res = cargarBytes(t, csvOEFA(ceros...), cfg, RecetaProvisional(), opMuestra)
	if fl := res.Limpieza.FilasLargas; fl.Procesadas != 24 || fl.DescartadasPorRegla["cero_como_faltante"] != 0 {
		t.Errorf("sin cero_como_faltante el 0 se conserva: %+v", fl.DescartadasPorRegla)
	}
	// Meteorología y precipitación en 0 dentro de un día con datos: se conservan
	// (viento en calma, hora seca). Hora explícita: una sola fila no es un «día en 0».
	h5 := "1899-12-31 05:00:00"
	cfg = ConfigPorDefecto()
	cfg.BloquesEnCero = nil
	met := []string{
		filaOEFA(func(c map[string]string) { c["PM10"], c["VELOCIDAD_VIENTO"], c["UNIDAD_MEDIDA"] = "-", "0", "m/s" }),
		filaOEFA(func(c map[string]string) { c["PM10"], c["PRECIPITACION"], c["UNIDAD_MEDIDA"] = "-", "0", "mm" }),
		filaOEFA(func(c map[string]string) { c["PM10"], c["TEMPERATURA"], c["UNIDAD_MEDIDA"] = "-", "0", "°c" }),
	}
	res = cargarBytes(t, csvOEFA(met...), cfg, RecetaProvisional(), opMuestra)
	if got, quiero := string(silverDe(t, res)), "estacion,fecha_hora,precipitacion,temperatura,viento_velocidad,hora_inferida\nEST X,2024-03-04T06:00:00-05:00,0,0,0,0\n"; got != quiero {
		t.Errorf("meteorología en 0:\n%s\nquiero\n%s", got, quiero)
	}
	// SO2 en µg/m³ en 0 y en ppb con valor, a la misma hora: el 0 no es una
	// lectura, así que no compite; queda el de ppb, convertido.
	gas := func(unidad, valor string) string {
		return filaOEFA(func(c map[string]string) {
			c["PM10"], c["DIOXIDO_AZUFRE"], c["UNIDAD_MEDIDA"], c["HORA_INICIO"] = "-", valor, unidad, h5
		})
	}
	res = cargarBytes(t, csvOEFA(gas("ug/m3", "0"), gas("ppb", "1.5")), cfg, RecetaProvisional(), opMuestra)
	fl = res.Limpieza.FilasLargas
	quiero := strconv.FormatFloat(1.5*2.62, 'g', -1, 64)
	if fl.DescartadasPorRegla["cero_como_faltante"] != 1 || fl.DescartadasPorRegla["duplicado_unidad"] != 0 ||
		fl.MarcadasPorRegla["unidad_convertida"] != 1 || !strings.Contains(string(silverDe(t, res)), ","+quiero+",0") {
		t.Errorf("SO2 en 0 y en ppb: %+v\n%s", fl, silverDe(t, res))
	}
}

func TestFilasVaciasOcupanSuHora(t *testing.T) {
	// Un día de 24 filas con 3 vacías intercaladas: las 21 con valor conservan
	// su hora (la posición cuenta las vacías) y las vacías van a sin_valor.
	huecos := map[int]bool{5: true, 6: true, 17: true}
	conHuecos := diaPM10(func(h int, c map[string]string) {
		if huecos[h] {
			vaciar(c)
		}
	})
	res := cargarBytes(t, csvOEFA(conHuecos...), ConfigPorDefecto(), RecetaProvisional(), opMuestra)
	fl := res.Limpieza.FilasLargas
	if fl.Procesadas != 21 || fl.DescartadasPorRegla["sin_valor"] != 3 || fl.DescartadasPorRegla["hora"] != 0 ||
		fl.DescartadasPorRegla["parametro"] != 0 || fl.HoraPorBloqueFisico != 21 || fl.HoraInferida != 21 {
		t.Fatalf("día con huecos vacíos: %+v", fl)
	}
	lineas := lineasSilver(t, res)
	quiero := []string{"A,2024-03-04T05:00:00-05:00,14,1", "A,2024-03-04T08:00:00-05:00,17,1", "A,2024-03-05T00:00:00-05:00,33,1"}
	for _, q := range quiero {
		if !slices.Contains(lineas, q) {
			t.Errorf("falta %q en el silver:\n%s", q, strings.Join(lineas, "\n"))
		}
	}
	if len(lineas) != 22 || slices.ContainsFunc(lineas, func(l string) bool { return strings.Contains(l, "T06:00:00") }) {
		t.Errorf("las horas vacías no tienen fila ancha:\n%s", strings.Join(lineas, "\n"))
	}

	// El día empieza con 16 horas vacías (CA-JUPR-01, presión, junio de 2022).
	tarde := diaPM10(func(h int, c map[string]string) {
		if h < 16 {
			vaciar(c)
		}
	})
	res = cargarBytes(t, csvOEFA(tarde...), ConfigPorDefecto(), RecetaProvisional(), opMuestra)
	lineas = lineasSilver(t, res)
	if len(lineas) != 9 || lineas[1] != "A,2024-03-04T17:00:00-05:00,26,1" || lineas[8] != "A,2024-03-05T00:00:00-05:00,33,1" {
		t.Errorf("día que empieza vacío:\n%s", strings.Join(lineas, "\n"))
	}

	casos := []struct {
		nombre string
		filas  []string
		hora   int64 // filas con valor que caen en la regla hora
	}{
		{"una fila vacía de más: 25 filas físicas", append(slices.Clone(conHuecos), filaOEFA(func(c map[string]string) {
			c["NOMBRE_PUNTO"], c["TIPO_ANALISIS"], c["HORA_INICIO"] = "A", "Concentración de partículas en PM10", "NA"
			vaciar(c)
		})), 21},
		{"una vacía de otro tipo de análisis corta el bloque", func() []string {
			f := slices.Clone(conHuecos)
			f[6] = strings.Replace(f[6], "Concentración de partículas en PM10", "Concentración de partículas en PM2.5", 1)
			return f
		}(), 21},
		{"dos parámetros en el mismo bloque físico", diaPM10(func(h int, c map[string]string) {
			c["TIPO_ANALISIS"] = "-"
			if h >= 12 {
				c["PM10"], c["PM2.5"] = "-", "8"
			}
			if h == 3 {
				vaciar(c)
			}
		}), 23},
		{"23 filas físicas", conHuecos[:23], 20},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			res := cargarBytes(t, csvOEFA(c.filas...), ConfigPorDefecto(), RecetaProvisional(), opMuestra)
			if fl := res.Limpieza.FilasLargas; fl.DescartadasPorRegla["hora"] != c.hora || fl.Procesadas != 0 {
				t.Errorf("regla hora %d (quiero %d), procesadas %d", fl.DescartadasPorRegla["hora"], c.hora, fl.Procesadas)
			}
		})
	}

	// La fila vacía ocupa su posición aunque el bloque físico cruce bordes de
	// bloque del archivo: mismo resultado que el oráculo secuencial, para cada B.
	otroDia := func(filas []string, fecha string) []string {
		out := make([]string, len(filas))
		for i, f := range filas {
			out[i] = strings.Replace(f, "2024-03-04", fecha, 1)
		}
		return out
	}
	csv := csvOEFA(append(append(slices.Clone(conHuecos), otroDia(tarde, "2024-03-05")...), otroDia(diaPM10(nil), "2024-03-06")...)...)
	for _, b := range []int64{300, 1 << 10, 7 << 10} {
		oraculo := cargarBytes(t, csv, ConfigPorDefecto(), RecetaProvisional(), Opciones{Workers: 1, Bloque: b, Chunk: 97, secuencial: true})
		otro := cargarBytes(t, csv, ConfigPorDefecto(), RecetaProvisional(), Opciones{Workers: 8, Bloque: b, Chunk: 97})
		if string(silverDe(t, oraculo)) != string(silverDe(t, otro)) || oraculo.Limpieza.FilasLargas.HoraPorBloqueFisico != 21+8 {
			t.Errorf("B=%d: silver distinto del oráculo o bloque físico mal cosido: %+v", b, otro.Limpieza.FilasLargas)
		}
	}
}

// TestFilasVaciasConPeriodoGuion: una fila vacía de periodo «-» sin hora no tiene
// día de parámetro: sigue en la regla periodo, pero también ocupa su posición.
func TestFilasVaciasConPeriodoGuion(t *testing.T) {
	meteo := dia24(24, func(h int, c map[string]string) {
		c["NOMBRE_PUNTO"], c["PERIODO"], c["UNIDAD_MEDIDA"], c["PM10"], c["TEMPERATURA"] = "A", "-", "°c", "-", strconvF(float64(h))
		if h == 2 {
			vaciar(c)
		}
	})
	res := cargarBytes(t, csvOEFA(meteo...), ConfigPorDefecto(), RecetaProvisional(), opMuestra)
	fl := res.Limpieza.FilasLargas
	if fl.Procesadas != 23 || fl.DescartadasPorRegla["periodo"] != 1 || fl.DescartadasPorRegla["sin_valor"] != 0 {
		t.Errorf("«-» con una fila vacía: %+v", fl.DescartadasPorRegla)
	}
}

func TestCorreccionDeUbicacion(t *testing.T) {
	m := generarMuestra()
	res := cargarBytes(t, m.csv, configMuestra(), RecetaProvisional(), opMuestra)
	c := res.Limpieza.Estaciones["SINT-C"]
	if c.Ubicacion["zona"] != "18" || c.UbicacionOriginal["zona"] != "19" || len(c.UbicacionOriginal) != 1 {
		t.Fatalf("SINT-C: ubicación %v, original %v", c.Ubicacion, c.UbicacionOriginal)
	}
	if got := res.Limpieza.FilasLargas.MarcadasPorRegla["ubicacion_corregida"]; got != c.LargasLeidas {
		t.Errorf("ubicacion_corregida %d, SINT-C leyó %d filas", got, c.LargasLeidas)
	}
	// lat/lon en WGS84 con la zona corregida: Junín (con la zona 19 caería en la Amazonía).
	lat, _ := strconv.ParseFloat(c.Ubicacion["lat"], 64)
	lon, _ := strconv.ParseFloat(c.Ubicacion["lon"], 64)
	if lat > -11 || lat < -12 || lon > -75.5 || lon < -76.5 {
		t.Errorf("SINT-C en (%s, %s), tiene que caer en Junín", c.Ubicacion["lat"], c.Ubicacion["lon"])
	}
	if a := res.Limpieza.Estaciones["SINT-A"]; a.Ubicacion["lat"] == "" || a.UbicacionOriginal != nil {
		t.Errorf("SINT-A tiene lat/lon y nada corregido: %+v", a)
	}
	// Las ubicaciones distintas de SINT-D siguen con sus campos tal cual (sin lat/lon).
	for _, u := range res.Limpieza.Estaciones["SINT-D"].UbicacionesDistintas {
		if _, ok := u.Campos["lat"]; ok {
			t.Errorf("ubicaciones_distintas no lleva lat/lon: %v", u.Campos)
		}
	}
	// Una corrección de una estación que no está en el archivo se avisa.
	avisa := slices.ContainsFunc(res.Limpieza.Avisos, func(a string) bool { return strings.Contains(a, "CA-HUAR-01") })
	if !avisa {
		t.Errorf("tiene que avisar que CA-HUAR-01 no está en el archivo: %v", res.Limpieza.Avisos)
	}
	// Sin la corrección, la zona es la del CSV y no se marca nada.
	res = cargarBytes(t, m.csv, ConfigPorDefecto(), RecetaProvisional(), opMuestra)
	if c := res.Limpieza.Estaciones["SINT-C"]; c.Ubicacion["zona"] != "19" || c.UbicacionOriginal != nil ||
		res.Limpieza.FilasLargas.MarcadasPorRegla["ubicacion_corregida"] != 0 {
		t.Errorf("sin corrección: %+v, marcadas %v", c, res.Limpieza.FilasLargas.MarcadasPorRegla)
	}
}

func TestCorreccionesUbicacionEnLaConfig(t *testing.T) {
	var cfg ConfigLimpieza
	if err := json.Unmarshal([]byte(`{"correcciones_ubicacion": {"ca-huar-01 ": {"zona": 18, "departamento": "PASCO"}}}`), &cfg); err != nil {
		t.Fatal(err)
	}
	if got := cfg.CorreccionesUbicacion["ca-huar-01 "]; got["zona"] != "18" || got["departamento"] != "PASCO" {
		t.Fatalf("número y texto: %v", got)
	}
	base := ConfigPorDefecto()
	base.CorreccionesUbicacion = cfg.CorreccionesUbicacion
	reg, err := base.compilar()
	if err != nil {
		t.Fatal(err)
	}
	if reg.correcciones["CA-HUAR-01"]["zona"] != "18" {
		t.Errorf("el nombre de la estación se normaliza: %v", reg.correcciones)
	}
	if err := json.Unmarshal([]byte(`{"correcciones_ubicacion": {"X": {"zona": true}}}`), &cfg); err == nil {
		t.Error("un booleano no es un valor de ubicación")
	}
	for nombre, corr := range map[string]map[string]map[string]ValorUbicacion{
		"columna que no es de ubicación": {"X": {"altura": "1"}},
		"valor vacío":                    {"X": {"zona": " "}},
		"valor nulo":                     {"X": {"zona": "NA"}},
		"estación vacía":                 {" ": {"zona": "18"}},
		"estación repetida":              {"x": {"zona": "18"}, "X ": {"zona": "17"}},
	} {
		t.Run(nombre, func(t *testing.T) {
			c := ConfigPorDefecto()
			c.CorreccionesUbicacion = corr
			if _, err := c.compilar(); err == nil {
				t.Fatal("la corrección inválida compiló")
			}
		})
	}
	c := ConfigPorDefecto()
	c.CeroComoFaltante = []string{"benceno"}
	if _, err := c.compilar(); err == nil {
		t.Error("cero_como_faltante con un no canónico compiló")
	}
}

func TestUTMALatLon(t *testing.T) {
	casos := []struct {
		nombre               string
		este, norte          float64
		zona                 int
		lat, lon, tolerancia float64
	}{
		// Sobre el meridiano central y el ecuador la inversa es exacta.
		{"ecuador, meridiano central de la 18", 500000, 10000000, 18, 0, -75, 1e-9},
		// reports/estaciones_ficha.csv (notebook 04, misma fórmula, 5 decimales).
		{"CA-HUAR-01 con zona 18", 358735, 8806535, 18, -10.79368, -76.29212, 6e-6},
		{"CA-TA-01", 469239, 9494220, 17, -4.5758, -81.27732, 6e-6},
		{"CA-ESP-01", 240086, 8361727, 19, -14.80603, -71.41485, 6e-6},
		{"CA-TAL-01", 321044, 8108673, 19, -17.09943, -70.68192, 6e-6},
		// Con la zona 19 del CSV, CA-HUAR-01 caería en la Amazonía (-10.79, -70.29).
		{"CA-HUAR-01 con zona 19", 358735, 8806535, 19, -10.79368, -70.29212, 6e-6},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			lat, lon := utmALatLon(c.este, c.norte, c.zona, true)
			if math.Abs(lat-c.lat) > c.tolerancia || math.Abs(lon-c.lon) > c.tolerancia {
				t.Errorf("(%.7f, %.7f), quiero (%.5f, %.5f)", lat, lon, c.lat, c.lon)
			}
		})
	}
	if lat, _ := utmALatLon(500000, 0, 18, false); lat != 0 {
		t.Errorf("hemisferio norte: el ecuador tiene norte 0, da lat %v", lat)
	}
}
