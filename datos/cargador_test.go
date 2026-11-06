package datos

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// cargarBytes corre el cargador sobre un CSV en memoria.
func cargarBytes(t *testing.T, csv []byte, cfg ConfigLimpieza, rec Receta, op Opciones) *Resultado {
	t.Helper()
	res, err := CargarDesde(bytes.NewReader(csv), int64(len(csv)), "prueba.csv", cfg, rec, op)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func silverDe(t *testing.T, res *Resultado) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := res.Ancha.EscribirSilver(&b); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func goldDe(t *testing.T, res *Resultado) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := res.Dataset.EscribirGold(&b); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

var opMuestra = Opciones{Workers: 4, Bloque: 16 << 10, Chunk: 97}

// filaOEFA arma una fila con el esquema real: estación EST X, PM10 = 33.5 µg/m³,
// periodo 1 hora, 2024-03-04 a las 05:00 (hora explícita). mod la ajusta.
func filaOEFA(mod func(c map[string]string)) string {
	c := map[string]string{
		"NOMBRE_EVALUACION": "Ev", "COMPONENTE_AMBIENTAL": "Aire", "PROCEDENCIA_MUESTRA": "Aire ambiental",
		"NOMBRE_PUNTO": "EST X", "ESTE": "1", "NORTE": "2", "ALTITUD": "3", "ZONA": "18", "DATUM": "WGS84",
		"DEPARTAMENTO": "LIMA", "PROVINCIA": "LIMA", "DISTRITO": "LIMA", "UBIGEO": "150101",
		"DESCRIPCION_UBICACION": "Punto de prueba", "TIPO_MUESTRA": "Simple", "TIPO_ANALISIS": "-",
		"PERIODO": "1 hora", "FECHA_INICIO": "2024-03-04", "HORA_INICIO": "1899-12-31 05:00:00",
		"FECHA_FIN": "NA", "HORA_FIN": "NA", "UNIDAD_MEDIDA": "ug/m3", "FECHA_CORTE": "20240429",
	}
	for _, p := range columnasOEFA[22:35] {
		c[p] = "-"
	}
	c["PM10"] = "33.5"
	if mod != nil {
		mod(c)
	}
	return filaSint{campos: c}.texto()
}

func csvOEFA(filas ...string) []byte {
	return []byte(cabeceraMuestra + "\r\n" + strings.Join(filas, "\r\n") + "\r\n")
}

// dia24 son las 24 filas sin hora de un día de una serie, con valores v(h).
func dia24(n int, mod func(h int, c map[string]string)) []string {
	var out []string
	for h := 0; h < n; h++ {
		out = append(out, filaOEFA(func(c map[string]string) {
			c["HORA_INICIO"] = "NA"
			mod(h, c)
		}))
	}
	return out
}

func TestReglasCuentanLoSembrado(t *testing.T) {
	m := generarMuestra()
	res := cargarBytes(t, m.csv, ConfigPorDefecto(), RecetaProvisional(), opMuestra)
	por := res.Limpieza.FilasLargas.DescartadasPorRegla
	for _, r := range Reglas() {
		t.Run(r, func(t *testing.T) {
			if por[r] != m.descartes[r] {
				t.Errorf("regla %s: descartó %d filas, se sembraron %d", r, por[r], m.descartes[r])
			}
		})
	}
	if len(por) != len(Reglas()) {
		t.Errorf("el reporte tiene que listar todas las reglas, aun en cero: %v", por)
	}
}

func TestConservacionDeFilasLargas(t *testing.T) {
	m := generarMuestra()
	res := cargarBytes(t, m.csv, ConfigPorDefecto(), RecetaProvisional(), opMuestra)
	fl := res.Limpieza.FilasLargas
	var suma int64
	for _, n := range fl.DescartadasPorRegla {
		suma += n
	}
	if fl.Leidas != m.leidas {
		t.Errorf("leídas %d, el archivo tiene %d registros (uno ocupa dos líneas)", fl.Leidas, m.leidas)
	}
	if fl.Descartadas != suma || fl.Leidas != fl.Procesadas+fl.Descartadas {
		t.Errorf("leídas %d, procesadas %d, descartadas %d (por regla %d)", fl.Leidas, fl.Procesadas, fl.Descartadas, suma)
	}
	if fl.Procesadas != fl.HoraExplicita+fl.HoraInferida || fl.HoraInferida == 0 || fl.HoraExplicita == 0 {
		t.Errorf("procesadas %d ≠ hora explícita %d + inferida %d", fl.Procesadas, fl.HoraExplicita, fl.HoraInferida)
	}
	var leidas, procesadas int64
	for _, e := range res.Limpieza.Estaciones {
		leidas += e.LargasLeidas
		procesadas += e.LargasProcesadas
	}
	sinEstacion := fl.DescartadasPorRegla["columnas"] + fl.DescartadasPorRegla["estacion"]
	if leidas+sinEstacion != fl.Leidas || procesadas != fl.Procesadas {
		t.Errorf("por estación: leídas %d + %d sin estación, procesadas %d; totales %d y %d",
			leidas, sinEstacion, procesadas, fl.Leidas, fl.Procesadas)
	}
}

func TestSilverCoincideConLaVerdadSembrada(t *testing.T) {
	m := generarMuestra()
	res := cargarBytes(t, m.csv, ConfigPorDefecto(), RecetaProvisional(), opMuestra)
	got, quiero := silverDe(t, res), m.silverEsperado()
	if !bytes.Equal(got, quiero) {
		gl, ql := strings.Split(string(got), "\n"), strings.Split(string(quiero), "\n")
		for i := 0; i < min(len(gl), len(ql)); i++ {
			if gl[i] != ql[i] {
				t.Fatalf("silver distinto de la verdad en la línea %d:\n got    %s\n quiero %s", i+1, gl[i], ql[i])
			}
		}
		t.Fatalf("silver con %d líneas, la verdad tiene %d", len(gl), len(ql))
	}
	if res.Limpieza.FilasAnchas.Total != len(m.verdad) {
		t.Errorf("filas anchas %d, la verdad tiene %d", res.Limpieza.FilasAnchas.Total, len(m.verdad))
	}
}

func TestMarcadas(t *testing.T) {
	m := generarMuestra()
	res := cargarBytes(t, m.csv, ConfigPorDefecto(), RecetaProvisional(), opMuestra)
	marc := res.Limpieza.FilasLargas.MarcadasPorRegla
	if marc["duplicado_valor_distinto"] != m.conflictos {
		t.Errorf("celdas con duplicado de otro valor: %d, se sembraron %d", marc["duplicado_valor_distinto"], m.conflictos)
	}
	if marc["unidad_convertida"] != m.convertida {
		t.Errorf("celdas convertidas: %d, la verdad dice %d (%v)", marc["unidad_convertida"], m.convertida,
			res.Limpieza.FilasLargas.UnidadConvertidaPorParametro)
	}
	if got := res.Limpieza.FilasAnchas.Pm25MayorPm10; got != m.violac {
		t.Errorf("filas con pm2_5 > pm10: %d, se sembraron %d", got, m.violac)
	}
	if res.Limpieza.FilasAnchas.HoraInferida == 0 || res.Limpieza.FilasAnchas.HoraInferida > res.Limpieza.FilasAnchas.Total {
		t.Errorf("filas anchas con hora inferida: %d de %d", res.Limpieza.FilasAnchas.HoraInferida, res.Limpieza.FilasAnchas.Total)
	}
}

func TestCompletitudParaElGold(t *testing.T) {
	m := generarMuestra()
	res := cargarBytes(t, m.csv, ConfigPorDefecto(), RecetaProvisional(), opMuestra)
	quiero := 0
	for _, c := range m.verdad {
		pm10, pm25 := c.v[idxPM10], c.v[idxPM25]
		if !math.IsNaN(pm10) && pm10 > 0 && !math.IsNaN(pm25) &&
			!math.IsNaN(c.v[indiceParametro("viento_velocidad")]) && !math.IsNaN(c.v[indiceParametro("humedad")]) {
			quiero++
		}
	}
	if res.Dataset.N != quiero || res.Limpieza.FilasAnchas.CompletasGold != quiero {
		t.Errorf("filas completas: dataset %d, reporte %d, la verdad dice %d",
			res.Dataset.N, res.Limpieza.FilasAnchas.CompletasGold, quiero)
	}
	if quiero == 0 || res.Dataset.N >= res.Ancha.N {
		t.Error("la muestra tiene huecos: algunas filas anchas, pero no todas, tienen que estar completas")
	}
}

// resultadoComparable quita lo que legítimamente cambia entre corridas.
func resultadoComparable(r *ReporteLimpieza) ReporteLimpieza {
	c := *r
	c.Workers, c.DuracionS, c.FasesS = 0, 0, nil
	return c
}

// sinBloque quita además lo que depende del tamaño de bloque y no del dato.
func sinBloque(r *ReporteLimpieza) ReporteLimpieza {
	c := resultadoComparable(r)
	c.BloqueBytes, c.Bloques = 0, 0
	return c
}

func mismoDataset(t *testing.T, a, b *Dataset) {
	t.Helper()
	if a.N != b.N || a.D != b.D || !slices.Equal(a.Features, b.Features) ||
		!slices.Equal(a.Estaciones, b.Estaciones) || !slices.Equal(a.Estacion, b.Estacion) ||
		!slices.Equal(a.Hora, b.Hora) || len(a.X) != len(b.X) {
		t.Fatal("los Dataset difieren en forma, estaciones u horas")
	}
	for i := range a.X {
		if math.Float64bits(a.X[i]) != math.Float64bits(b.X[i]) {
			t.Fatalf("X[%d] difiere bit a bit: %v vs %v", i, a.X[i], b.X[i])
		}
	}
}

// TestMismoResultadoConUnoYOchoWorkers compara el pipeline con W = 1 y W = 8
// contra el oráculo secuencial (procesar y fusionar sin goroutines ni
// canales), para varios tamaños de bloque: con 1 KB cada día de 24 filas y el
// registro multilínea cruzan bordes de bloque.
func TestMismoResultadoConUnoYOchoWorkers(t *testing.T) {
	m := generarMuestra()
	porEstacion := RecetaProvisional()
	porEstacion.Nivel = NivelEstacion
	for _, receta := range []Receta{RecetaProvisional(), porEstacion} {
		var ref *Resultado
		for _, bloque := range []int64{1 << 10, 7 << 10, 64 << 10, 64 << 20} {
			oraculo := cargarBytes(t, m.csv, configMuestra(), receta, Opciones{Workers: 1, Bloque: bloque, Chunk: 97, secuencial: true})
			for _, w := range []int{1, 8} {
				res := cargarBytes(t, m.csv, configMuestra(), receta, Opciones{Workers: w, Bloque: bloque, Chunk: 97})
				if !bytes.Equal(silverDe(t, oraculo), silverDe(t, res)) || !bytes.Equal(goldDe(t, oraculo), goldDe(t, res)) {
					t.Fatalf("nivel %s, B=%d, W=%d: silver o gold distinto del oráculo secuencial", receta.Nivel, bloque, w)
				}
				mismoDataset(t, oraculo.Dataset, res.Dataset)
				if !reflect.DeepEqual(resultadoComparable(oraculo.Limpieza), resultadoComparable(res.Limpieza)) {
					t.Fatalf("nivel %s, B=%d, W=%d: el reporte de limpieza difiere del oráculo", receta.Nivel, bloque, w)
				}
				if !reflect.DeepEqual(oraculo.Estandarizacion, res.Estandarizacion) {
					t.Fatalf("nivel %s, B=%d, W=%d: la estandarización difiere del oráculo", receta.Nivel, bloque, w)
				}
			}
			if bloque == 1<<10 && oraculo.Limpieza.Bloques < 100 {
				t.Fatalf("con B=1 KB la muestra tendría que partirse en cientos de bloques, dio %d", oraculo.Limpieza.Bloques)
			}
			if ref == nil {
				ref = oraculo
				continue
			}
			if !bytes.Equal(silverDe(t, ref), silverDe(t, oraculo)) || !bytes.Equal(goldDe(t, ref), goldDe(t, oraculo)) ||
				!reflect.DeepEqual(sinBloque(ref.Limpieza), sinBloque(oraculo.Limpieza)) {
				t.Fatalf("nivel %s: B=%d cambia el resultado respecto de B=1 KB", receta.Nivel, bloque)
			}
		}
	}
}

func TestFilasOrdenadasPorEstacionYFecha(t *testing.T) {
	m := generarMuestra()
	res := cargarBytes(t, m.csv, ConfigPorDefecto(), RecetaProvisional(), opMuestra)
	ordenadas := func(est []string, idx []uint16, hora []int64) bool {
		for i := 1; i < len(idx); i++ {
			a, b := est[idx[i-1]], est[idx[i]]
			if a > b || (a == b && hora[i-1] >= hora[i]) {
				return false
			}
		}
		return true
	}
	if !ordenadas(res.Ancha.Estaciones, res.Ancha.Estacion, res.Ancha.Hora) {
		t.Error("las filas anchas no están ordenadas por (estación, fecha_hora) sin repetidos")
	}
	if !ordenadas(res.Dataset.Estaciones, res.Dataset.Estacion, res.Dataset.Hora) {
		t.Error("las filas del gold no están ordenadas por (estación, fecha_hora)")
	}
	if !slices.Equal(res.Ancha.Estaciones, []string{"SINT-A", "SINT-B", "SINT-C", "SINT-D"}) {
		t.Errorf("estaciones %q («sint-d» tiene que ser SINT-D)", res.Ancha.Estaciones)
	}
}

func TestBloqueMasGrandeQueElArchivo(t *testing.T) {
	m := generarMuestra()
	grande := cargarBytes(t, m.csv, ConfigPorDefecto(), RecetaProvisional(), Opciones{Workers: 4, Bloque: 1 << 40, Chunk: 97})
	if grande.Limpieza.Bloques != 1 {
		t.Errorf("un bloque más grande que el archivo da un solo bloque, dio %d", grande.Limpieza.Bloques)
	}
	chico := cargarBytes(t, m.csv, ConfigPorDefecto(), RecetaProvisional(), opMuestra)
	if !bytes.Equal(silverDe(t, grande), silverDe(t, chico)) || !bytes.Equal(goldDe(t, grande), goldDe(t, chico)) {
		t.Error("el resultado no puede depender del tamaño de bloque")
	}
}

func TestArchivoDeUnaSolaLinea(t *testing.T) {
	for _, csv := range []string{cabeceraMuestra, cabeceraMuestra + "\n", cabeceraMuestra + "\r\n"} {
		res := cargarBytes(t, []byte(csv), ConfigPorDefecto(), RecetaProvisional(), opMuestra)
		if res.Limpieza.FilasLargas.Leidas != 0 || res.Ancha.N != 0 || res.Dataset.N != 0 {
			t.Fatalf("solo cabecera: %+v", res.Limpieza.FilasLargas)
		}
		if got := string(silverDe(t, res)); got != "estacion,fecha_hora,hora_inferida\n" {
			t.Errorf("silver vacío: %q", got)
		}
		if got := string(goldDe(t, res)); got != "estacion,fecha_hora,pm10,fraccion_fina,viento_velocidad,humedad\n" {
			t.Errorf("gold vacío: %q", got)
		}
		if res.Estandarizacion.N != 0 || len(res.Estandarizacion.Media) != 4 {
			t.Errorf("estandarización vacía: %+v", res.Estandarizacion)
		}
	}
	// Una fila de datos sin salto final.
	res := cargarBytes(t, []byte(cabeceraMuestra+"\r\n"+filaOEFA(nil)), ConfigPorDefecto(), RecetaProvisional(), opMuestra)
	if got := string(silverDe(t, res)); got != "estacion,fecha_hora,pm10,hora_inferida\nEST X,2024-03-04T06:00:00-05:00,33.5,0\n" {
		t.Errorf("una fila sin salto final: %q", got)
	}
}

func TestArchivoVacioFalla(t *testing.T) {
	if _, err := CargarDesde(bytes.NewReader(nil), 0, "vacio.csv", ConfigPorDefecto(), RecetaProvisional(), opMuestra); err == nil {
		t.Fatal("un archivo vacío tiene que fallar")
	}
}

func TestCabeceraConYSinBOM(t *testing.T) {
	m := generarMuestra()
	sin := cargarBytes(t, m.csv, ConfigPorDefecto(), RecetaProvisional(), opMuestra)
	con := cargarBytes(t, append(slices.Clone(bomUTF8), m.csv...), ConfigPorDefecto(), RecetaProvisional(), opMuestra)
	if !con.Limpieza.BOM || sin.Limpieza.BOM {
		t.Error("el reporte tiene que decir si había BOM")
	}
	if !bytes.Equal(silverDe(t, con), silverDe(t, sin)) {
		t.Error("el BOM cambió el resultado: la primera columna no se reconoció")
	}
}

func TestDelimitadorPuntoYComaYComa(t *testing.T) {
	m := generarMuestra()
	base := silverDe(t, cargarBytes(t, m.csv, ConfigPorDefecto(), RecetaProvisional(), opMuestra))
	conComa := bytes.ReplaceAll(m.csv, []byte{';'}, []byte{','})
	for _, delim := range []string{",", "auto"} {
		cfg := ConfigPorDefecto()
		cfg.Delimitador = delim
		res := cargarBytes(t, conComa, cfg, RecetaProvisional(), opMuestra)
		if !bytes.Equal(silverDe(t, res), base) {
			t.Errorf("delimitador %q: el silver con comas difiere del de punto y coma", delim)
		}
		if res.Limpieza.Delimitador != "," {
			t.Errorf("delimitador %q: el reporte dice %q", delim, res.Limpieza.Delimitador)
		}
	}
	if _, err := CargarDesde(bytes.NewReader(conComa), int64(len(conComa)), "x", ConfigPorDefecto(), RecetaProvisional(), opMuestra); err == nil ||
		!strings.Contains(err.Error(), "cabecera real") {
		t.Errorf("delimitador equivocado: tiene que fallar mostrando la cabecera real, error: %v", err)
	}
}

func TestFinDeLineaLF(t *testing.T) {
	m := generarMuestra()
	base := silverDe(t, cargarBytes(t, m.csv, ConfigPorDefecto(), RecetaProvisional(), opMuestra))
	lf := bytes.ReplaceAll(m.csv, []byte("\r\n"), []byte("\n"))
	if !bytes.Equal(silverDe(t, cargarBytes(t, lf, ConfigPorDefecto(), RecetaProvisional(), opMuestra)), base) {
		t.Error("LF en vez de CRLF cambió el resultado")
	}
}

// Un campo entrecomillado con saltos de línea (como DESCRIPCION_UBICACION en
// 100 registros del CSV real) es un solo registro, aunque cruce bordes de bloque.
func TestCampoMultilinea(t *testing.T) {
	multi := filaOEFA(func(c map[string]string) {
		c["DESCRIPCION_UBICACION"] = "Ubicado en la azotea, Mz. O Lote 11, \r\r\n– A.H. Virgen de Guadalupe\r\r\nfin"
		c["HORA_INICIO"] = "1899-12-31 07:00:00"
	})
	var filas []string
	for h := 0; h < 30; h++ {
		filas = append(filas, filaOEFA(func(c map[string]string) { c["NOMBRE_PUNTO"] = "EST Y" }))
	}
	csv := csvOEFA(append(append(filas[:15:15], multi), filas[15:]...)...)
	var ref []byte
	for _, op := range []Opciones{{Workers: 1, Bloque: 1 << 30}, {Workers: 8, Bloque: 200}, {Workers: 3, Bloque: 97}, {Workers: 1, Bloque: 50, secuencial: true}} {
		res := cargarBytes(t, csv, ConfigPorDefecto(), RecetaProvisional(), op)
		if res.Limpieza.FilasLargas.Leidas != 31 || res.Limpieza.FilasLargas.DescartadasPorRegla["columnas"] != 0 {
			t.Fatalf("bloque %d: %d registros leídos (quiero 31), columnas %d", op.Bloque,
				res.Limpieza.FilasLargas.Leidas, res.Limpieza.FilasLargas.DescartadasPorRegla["columnas"])
		}
		s := silverDe(t, res)
		if !strings.Contains(string(s), "EST X,2024-03-04T08:00:00-05:00,33.5,0") {
			t.Fatalf("bloque %d: falta la fila del registro multilínea:\n%s", op.Bloque, s)
		}
		if ref == nil {
			ref = s
		} else if !bytes.Equal(ref, s) {
			t.Fatalf("bloque %d: el registro multilínea cambia el resultado", op.Bloque)
		}
	}
	// Una comilla que nunca cierra es un error, no un registro infinito.
	roto := csvOEFA(filaOEFA(nil), strings.Replace(filaOEFA(nil), `"Punto de prueba"`, `"Punto de prueba`, 1), filaOEFA(nil))
	if _, err := CargarDesde(bytes.NewReader(roto), int64(len(roto)), "x", ConfigPorDefecto(), RecetaProvisional(), opMuestra); err == nil ||
		!strings.Contains(err.Error(), "sin cerrar") {
		t.Fatalf("una comilla sin cerrar tiene que fallar explicándolo, error: %v", err)
	}
}

func TestDuplicadoEntreBloquesConservaElPrimero(t *testing.T) {
	var filas []string
	filas = append(filas, filaOEFA(func(c map[string]string) { c["PM10"] = "10" }))
	for h := 0; h < 60; h++ { // relleno para que el duplicado caiga en otro bloque
		filas = append(filas, filaOEFA(func(c map[string]string) { c["NOMBRE_PUNTO"] = "EST Y" }))
	}
	filas = append(filas,
		filaOEFA(func(c map[string]string) { c["PM10"] = "20" }),
		filaOEFA(func(c map[string]string) { c["PM10"] = "10" }))
	csv := csvOEFA(filas...)
	// El 20 y el segundo 10 caen juntos, lejos del primero: el segundo 10 es
	// duplicado exacto del valor conservado aunque en su bloque lo preceda un 20.
	for _, op := range []Opciones{{Workers: 1, Bloque: 512}, {Workers: 8, Bloque: 512}, {Workers: 8, Bloque: 1 << 30}} {
		res := cargarBytes(t, csv, ConfigPorDefecto(), RecetaProvisional(), op)
		silver := string(silverDe(t, res))
		if !strings.HasPrefix(silver, "estacion,fecha_hora,pm10,hora_inferida\nEST X,2024-03-04T06:00:00-05:00,10,0\n") {
			t.Errorf("W=%d, B=%d: se tiene que conservar el primer valor en orden de archivo:\n%s", op.Workers, op.Bloque, silver[:80])
		}
		fl := res.Limpieza.FilasLargas
		// 59 del relleno (las 60 filas de EST Y son la misma celda) más el segundo 10.
		if fl.DescartadasPorRegla["duplicado_valor_distinto"] != 1 || fl.DescartadasPorRegla["duplicado_exacto"] != 60 ||
			fl.MarcadasPorRegla["duplicado_valor_distinto"] != 1 {
			t.Errorf("W=%d, B=%d: conteos de duplicados %+v", op.Workers, op.Bloque, fl.DescartadasPorRegla)
		}
	}
}

func TestHoraPorPosicion(t *testing.T) {
	serie := func(n int, est string) []string {
		return dia24(n, func(h int, c map[string]string) {
			c["NOMBRE_PUNTO"] = est
			c["PM10"] = strconvF(float64(10 + h))
		})
	}
	// 24 filas: hora de inicio = posición; fecha_hora = inicio + 1 h (la última,
	// de 23:00 a 24:00, es las 00:00 del día siguiente).
	res := cargarBytes(t, csvOEFA(serie(24, "A")...), ConfigPorDefecto(), RecetaProvisional(), opMuestra)
	lineas := strings.Split(strings.TrimSpace(string(silverDe(t, res))), "\n")
	if len(lineas) != 25 || lineas[1] != "A,2024-03-04T01:00:00-05:00,10,1" || lineas[24] != "A,2024-03-05T00:00:00-05:00,33,1" {
		t.Fatalf("hora por posición:\n%s\n…\n%s", lineas[1], lineas[len(lineas)-1])
	}
	fl := res.Limpieza.FilasLargas
	if fl.HoraInferida != 24 || fl.HoraExplicita != 0 {
		t.Errorf("hora inferida %d, explícita %d", fl.HoraInferida, fl.HoraExplicita)
	}
	// 23 y 25 filas: el día no está completo y se descarta entero (regla hora).
	for _, n := range []int{23, 25} {
		res := cargarBytes(t, csvOEFA(serie(n, "A")...), ConfigPorDefecto(), RecetaProvisional(), opMuestra)
		if res.Limpieza.FilasLargas.DescartadasPorRegla["hora"] != int64(n) || res.Ancha.N != 0 {
			t.Errorf("%d filas: regla hora %d, filas anchas %d", n, res.Limpieza.FilasLargas.DescartadasPorRegla["hora"], res.Ancha.N)
		}
	}
	// Una fila inválida conserva su posición: las demás siguen teniendo su hora.
	conInvalida := serie(24, "A")
	conInvalida[3] = strings.Replace(conInvalida[3], `"13"`, `"<LD"`, 1)
	res = cargarBytes(t, csvOEFA(conInvalida...), ConfigPorDefecto(), RecetaProvisional(), opMuestra)
	if s := string(silverDe(t, res)); !strings.Contains(s, "A,2024-03-04T05:00:00-05:00,14,1") || strings.Contains(s, "T04:00:00-05:00") {
		t.Errorf("la fila inválida tiene que dejar su hueco sin correr a las demás:\n%s", s)
	}
	// 24 filas que no son contiguas (otra fila en medio): el día no está completo.
	partido := serie(24, "A")
	partido = append(partido[:12:12], append([]string{filaOEFA(nil)}, partido[12:]...)...)
	res = cargarBytes(t, csvOEFA(partido...), ConfigPorDefecto(), RecetaProvisional(), opMuestra)
	if res.Limpieza.FilasLargas.DescartadasPorRegla["hora"] != 24 {
		t.Errorf("24 filas no contiguas: regla hora %d, quiero 24", res.Limpieza.FilasLargas.DescartadasPorRegla["hora"])
	}
	// Un día entero en 0 es equipo sin medir; con un valor distinto de 0, no
	// (y sus horas en 0 van a cero_como_faltante, una por una).
	ceros := dia24(24, func(h int, c map[string]string) { c["NOMBRE_PUNTO"], c["PM10"] = "A", "0" })
	res = cargarBytes(t, csvOEFA(ceros...), ConfigPorDefecto(), RecetaProvisional(), opMuestra)
	if res.Limpieza.FilasLargas.DescartadasPorRegla["bloque_en_cero"] != 24 || res.Limpieza.FilasLargas.BloqueEnCeroPorParametro["pm10"] != 24 {
		t.Errorf("día en cero: %+v", res.Limpieza.FilasLargas)
	}
	ceros[7] = strings.Replace(ceros[7], `"0"`, `"0.5"`, 1)
	res = cargarBytes(t, csvOEFA(ceros...), ConfigPorDefecto(), RecetaProvisional(), opMuestra)
	if por := res.Limpieza.FilasLargas.DescartadasPorRegla; por["bloque_en_cero"] != 0 || por["cero_como_faltante"] != 23 ||
		res.Limpieza.FilasLargas.Procesadas != 1 {
		t.Errorf("un día con un valor distinto de 0 no es bloque_en_cero: %+v", por)
	}
	// Desactivada: sin hora, regla hora.
	cfg := ConfigPorDefecto()
	cfg.HoraPorPosicion.Activar = false
	res = cargarBytes(t, csvOEFA(serie(24, "A")...), cfg, RecetaProvisional(), opMuestra)
	if res.Limpieza.FilasLargas.DescartadasPorRegla["hora"] != 24 {
		t.Errorf("sin hora por posición: regla hora %d", res.Limpieza.FilasLargas.DescartadasPorRegla["hora"])
	}
	// Convención inicio: fecha_hora es el inicio del intervalo.
	cfg = ConfigPorDefecto()
	cfg.HoraPorPosicion.Convencion = HoraInicio
	res = cargarBytes(t, csvOEFA(serie(24, "A")...), cfg, RecetaProvisional(), opMuestra)
	if s := string(silverDe(t, res)); !strings.Contains(s, "A,2024-03-04T00:00:00-05:00,10,1") {
		t.Errorf("convención inicio:\n%s", s[:120])
	}
}

func strconvF(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

func TestHoraExplicita(t *testing.T) {
	casos := []struct {
		nombre, hora, cfgExplicita, cfgNoEnPunto string
		silver, regla                            string
	}{
		{"medianoche (solo el prefijo de Excel)", "1899-12-31", "", "", "2024-03-04T01:00:00-05:00", ""},
		{"hora de inicio", "1899-12-31 05:00:00", "", "", "2024-03-04T06:00:00-05:00", ""},
		{"última hora del día", "1899-12-31 23:00:00", "", "", "2024-03-05T00:00:00-05:00", ""},
		{"hora como fin del intervalo", "1899-12-31 05:00:00", HoraFin, "", "2024-03-04T05:00:00-05:00", ""},
		{"sin segundos", "1899-12-31 05:00", "", "", "2024-03-04T06:00:00-05:00", ""},
		{"no en punto: se descarta", "1899-12-31 05:59:00", "", "", "", "hora_no_en_punto"},
		{"no en punto: se trunca", "1899-12-31 05:59:00", "", HoraTruncar, "2024-03-04T06:00:00-05:00", ""},
		{"hora ilegible", "1899-12-31 25:00:00", "", "", "", "hora"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			cfg := ConfigPorDefecto()
			cfg.HoraExplicita, cfg.HoraNoEnPunto = c.cfgExplicita, c.cfgNoEnPunto
			res := cargarBytes(t, csvOEFA(filaOEFA(func(f map[string]string) { f["HORA_INICIO"] = c.hora })), cfg, RecetaProvisional(), opMuestra)
			fl := res.Limpieza.FilasLargas
			if c.regla != "" {
				if fl.DescartadasPorRegla[c.regla] != 1 {
					t.Fatalf("se esperaba la regla %s: %v", c.regla, fl.DescartadasPorRegla)
				}
				return
			}
			if s := string(silverDe(t, res)); !strings.Contains(s, "EST X,"+c.silver+",33.5,0") {
				t.Fatalf("silver:\n%s", s)
			}
			if c.cfgNoEnPunto == HoraTruncar && fl.MarcadasPorRegla["hora_no_en_punto"] != 1 {
				t.Errorf("la hora truncada tiene que marcarse: %v", fl.MarcadasPorRegla)
			}
		})
	}
}

func TestPeriodoCondicional(t *testing.T) {
	meteo := func(n int, hora func(h int) string) []string {
		var out []string
		for h := 0; h < n; h++ {
			out = append(out, filaOEFA(func(c map[string]string) {
				c["PERIODO"], c["UNIDAD_MEDIDA"], c["PM10"], c["TEMPERATURA"] = "-", "°c", "-", "15"
				c["HORA_INICIO"] = hora(h)
			}))
		}
		return out
	}
	na := func(int) string { return "NA" }
	res := cargarBytes(t, csvOEFA(meteo(24, na)...), ConfigPorDefecto(), RecetaProvisional(), opMuestra)
	if res.Limpieza.FilasLargas.Procesadas != 24 {
		t.Errorf("«-» con el día completo es horario: %+v", res.Limpieza.FilasLargas.DescartadasPorRegla)
	}
	explicita := func(h int) string { return "1899-12-31 0" + string(rune('0'+h)) + ":00:00" }
	res = cargarBytes(t, csvOEFA(meteo(3, explicita)...), ConfigPorDefecto(), RecetaProvisional(), opMuestra)
	if res.Limpieza.FilasLargas.Procesadas != 3 {
		t.Errorf("«-» con hora explícita es horario aunque el día esté incompleto: %v", res.Limpieza.FilasLargas.DescartadasPorRegla)
	}
	res = cargarBytes(t, csvOEFA(meteo(23, na)...), ConfigPorDefecto(), RecetaProvisional(), opMuestra)
	if res.Limpieza.FilasLargas.DescartadasPorRegla["periodo"] != 23 {
		t.Errorf("«-» sin hora y con el día incompleto no es horario (periodo va antes que hora): %v",
			res.Limpieza.FilasLargas.DescartadasPorRegla)
	}
	for _, p := range []string{"24 horas", "24 Horas", "3 horas móviles", "Móvil 3 horas", "Móvil 8 horas", "Media móvil de 8 horas"} {
		res := cargarBytes(t, csvOEFA(filaOEFA(func(c map[string]string) { c["PERIODO"] = p })), ConfigPorDefecto(), RecetaProvisional(), opMuestra)
		if res.Limpieza.FilasLargas.DescartadasPorRegla["periodo"] != 1 {
			t.Errorf("%q tiene que descartarse por periodo", p)
		}
	}
	res = cargarBytes(t, csvOEFA(filaOEFA(func(c map[string]string) { c["PERIODO"] = " 1 HORA " })), ConfigPorDefecto(), RecetaProvisional(), opMuestra)
	if res.Limpieza.FilasLargas.Procesadas != 1 {
		t.Error("el periodo se compara normalizado: « 1 HORA » es horario")
	}
}

func TestUnidades(t *testing.T) {
	gas := func(unidad, valor, hora string) string {
		return filaOEFA(func(c map[string]string) {
			c["PM10"], c["DIOXIDO_AZUFRE"], c["UNIDAD_MEDIDA"], c["HORA_INICIO"] = "-", valor, unidad, hora
		})
	}
	h5 := "1899-12-31 05:00:00"
	casos := []struct {
		nombre string
		filas  []string
		silver string
		reglas map[string]int64
		conv   int64
	}{
		// 10 × 2.62 en float64 es 26.200000000000003: el silver escribe el producto
		// exacto (FormatFloat 'g' -1), que la auditoría en Python reproduce igual.
		{"solo ppb: se convierte", []string{gas("ppb", "10", h5)}, "26.200000000000003", nil, 1},
		{"ppb y después ug/m3: gana ug/m3", []string{gas("ppb", "10", h5), gas("ug/m3", "26.5", h5)}, "26.5",
			map[string]int64{"duplicado_unidad": 1}, 0},
		{"ug/m3 y después ppb: gana ug/m3", []string{gas("ug/m3", "26.5", h5), gas("ppb", "10", h5)}, "26.5",
			map[string]int64{"duplicado_unidad": 1}, 0},
		{"unidad desconocida", []string{gas("ppm", "10", h5)}, "", map[string]int64{"unidad": 1}, 0},
		{"unidad con mayúsculas y símbolos", []string{gas("µg/m³", "26", h5)}, "26", nil, 0},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			res := cargarBytes(t, csvOEFA(c.filas...), ConfigPorDefecto(), RecetaProvisional(), opMuestra)
			fl := res.Limpieza.FilasLargas
			for r, n := range c.reglas {
				if fl.DescartadasPorRegla[r] != n {
					t.Errorf("regla %s: %d, quiero %d", r, fl.DescartadasPorRegla[r], n)
				}
			}
			if fl.MarcadasPorRegla["unidad_convertida"] != c.conv {
				t.Errorf("convertidas %d, quiero %d", fl.MarcadasPorRegla["unidad_convertida"], c.conv)
			}
			if c.silver != "" && !strings.Contains(string(silverDe(t, res)), ","+c.silver+",0") {
				t.Errorf("silver:\n%s", silverDe(t, res))
			}
		})
	}
	// Presión en mmHg a hPa; dirección del viento con «°c» (error de OEFA) se acepta;
	// temperatura en «ºC» con el ordinal en vez del signo de grado se acepta.
	filas := []string{
		filaOEFA(func(c map[string]string) {
			c["PM10"], c["PRESION_BAROMETRICA"], c["UNIDAD_MEDIDA"] = "-", "750", "mmHg"
		}),
		filaOEFA(func(c map[string]string) { c["PM10"], c["DIRECCION_VIENTO"], c["UNIDAD_MEDIDA"] = "-", "270", "°c" }),
		filaOEFA(func(c map[string]string) { c["PM10"], c["TEMPERATURA"], c["UNIDAD_MEDIDA"] = "-", "15", "ºC" }),
	}
	res := cargarBytes(t, csvOEFA(filas...), ConfigPorDefecto(), RecetaProvisional(), opMuestra)
	quiero := "estacion,fecha_hora,presion,temperatura,viento_direccion,hora_inferida\nEST X,2024-03-04T06:00:00-05:00,999.9150000000001,15,270,0\n"
	if got := string(silverDe(t, res)); got != quiero {
		t.Errorf("meteorología:\n%s\nquiero\n%s", got, quiero)
	}
}

func TestUbicacionesDistintas(t *testing.T) {
	m := generarMuestra()
	res := cargarBytes(t, m.csv, ConfigPorDefecto(), RecetaProvisional(), opMuestra)
	d := res.Limpieza.Estaciones["SINT-D"]
	if len(d.UbicacionesDistintas) != 2 || d.Ubicacion["norte"] != "9073000" {
		t.Fatalf("SINT-D tiene dos juegos de coordenadas: %+v", d)
	}
	var filas int64
	for _, u := range d.UbicacionesDistintas {
		filas += u.Filas
	}
	if filas != d.LargasLeidas {
		t.Errorf("las ubicaciones suman %d filas, la estación leyó %d", filas, d.LargasLeidas)
	}
	if a := res.Limpieza.Estaciones["SINT-A"]; len(a.UbicacionesDistintas) != 0 {
		t.Errorf("SINT-A tiene una sola ubicación: %+v", a.UbicacionesDistintas)
	}
	avisa := slices.ContainsFunc(res.Limpieza.Avisos, func(a string) bool { return strings.Contains(a, "SINT-D") })
	if !avisa {
		t.Errorf("tiene que avisar de SINT-D: %v", res.Limpieza.Avisos)
	}
}

func TestVistosParaAjustarLaConfig(t *testing.T) {
	m := generarMuestra()
	res := cargarBytes(t, m.csv, ConfigPorDefecto(), RecetaProvisional(), opMuestra)
	v := res.Limpieza.Vistos
	if v.ValoresNoNumericos["<LD"] != 2 || v.ValoresNoNumericos["ND"] != 1 {
		t.Errorf("valores no numéricos: %v", v.ValoresNoNumericos)
	}
	if v.Parametros["PM10"] == 0 || v.Parametros["SULFURO_HIDROGENO"] == 0 || len(v.ParametrosNoMapeados) != 0 {
		t.Errorf("parámetros vistos: %v / %v", v.Parametros, v.ParametrosNoMapeados)
	}
	if v.Periodos["-"] == 0 || v.Periodos["Media móvil de 8 horas"] != 24 || v.Periodos["24 Horas"] != 1 {
		t.Errorf("periodos vistos: %v", v.Periodos)
	}
	if v.Unidades["so2"]["ppb"] == 0 || v.Unidades["presion"]["mmHg"] == 0 || v.Unidades["pm10"]["ppm"] != 1 {
		t.Errorf("unidades vistas: %v", v.Unidades)
	}
}

func TestReportePorEstacion(t *testing.T) {
	m := generarMuestra()
	res := cargarBytes(t, m.csv, configMuestra(), RecetaProvisional(), opMuestra)
	c, ok := res.Limpieza.Estaciones["SINT-C"]
	if !ok {
		t.Fatalf("falta la estación C: %v", res.Limpieza.Estaciones)
	}
	if c.Ubicacion["departamento"] != "JUNÍN" || c.Ubicacion["zona"] != "18" {
		t.Errorf("ubicación: %v", c.Ubicacion)
	}
	if c.Desde != "2024-03-04T01:00:00-05:00" || c.Hasta != "2024-03-09T00:00:00-05:00" {
		t.Errorf("rango de fechas de C: %s a %s", c.Desde, c.Hasta)
	}
	var total int
	for _, e := range res.Limpieza.Estaciones {
		total += e.Anchas
	}
	if total != res.Limpieza.FilasAnchas.Total {
		t.Errorf("las anchas por estación suman %d, el total es %d", total, res.Limpieza.FilasAnchas.Total)
	}
}

func TestCargarDesdeArchivo(t *testing.T) {
	m := generarMuestra()
	ruta := filepath.Join(t.TempDir(), "m.csv")
	if err := os.WriteFile(ruta, m.csv, 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Cargar(ruta, ConfigPorDefecto(), RecetaProvisional(), opMuestra)
	if err != nil {
		t.Fatal(err)
	}
	if res.Limpieza.Entrada != ruta || res.Limpieza.Bytes != int64(len(m.csv)) {
		t.Errorf("entrada %q de %d bytes", res.Limpieza.Entrada, res.Limpieza.Bytes)
	}
	if _, err := Cargar(filepath.Join(t.TempDir(), "no.csv"), ConfigPorDefecto(), RecetaProvisional(), opMuestra); err == nil {
		t.Error("un archivo inexistente tiene que fallar")
	}
}

func TestWorkersCeroUsaTodosLosNucleos(t *testing.T) {
	m := generarMuestra()
	res := cargarBytes(t, m.csv, ConfigPorDefecto(), RecetaProvisional(), Opciones{})
	if res.Limpieza.Workers < 1 || res.Limpieza.BloqueBytes != BloquePorDefecto || res.Limpieza.Chunk != ChunkPorDefecto {
		t.Errorf("valores por defecto: workers %d, bloque %d, chunk %d",
			res.Limpieza.Workers, res.Limpieza.BloqueBytes, res.Limpieza.Chunk)
	}
}

func TestSeparadorDecimalComa(t *testing.T) {
	cfg := ConfigPorDefecto()
	cfg.SeparadorDecimal = ","
	res := cargarBytes(t, csvOEFA(filaOEFA(func(c map[string]string) { c["PM10"] = "33,5" })), cfg, RecetaProvisional(), opMuestra)
	if got := string(silverDe(t, res)); !strings.Contains(got, ",33.5,0") {
		t.Errorf("decimal con coma: %q", got)
	}
}

func TestCodificacionLatin1(t *testing.T) {
	utf8 := csvOEFA(filaOEFA(func(c map[string]string) { c["NOMBRE_PUNTO"], c["DEPARTAMENTO"] = "EST Junín", "JUNÍN" }))
	var latin []byte
	for _, r := range string(utf8) {
		latin = append(latin, byte(r)) // todos los caracteres de la prueba caben en Latin-1
	}
	cfg := ConfigPorDefecto()
	cfg.Codificacion = "latin1"
	res := cargarBytes(t, latin, cfg, RecetaProvisional(), opMuestra)
	if !strings.Contains(string(silverDe(t, res)), "EST JUNÍN,") {
		t.Errorf("latin1: %q", silverDe(t, res))
	}
	if res.Limpieza.Estaciones["EST JUNÍN"].Ubicacion["departamento"] != "JUNÍN" {
		t.Errorf("ubicación mal decodificada: %+v", res.Limpieza.Estaciones)
	}
}

// TestBordesDeLasReglas: como en el TP, el valor justo en el umbral se queda y
// el siguiente se va, con la config por defecto.
func TestBordesDeLasReglas(t *testing.T) {
	casos := []struct {
		nombre, columna, unidad, fecha, valor, regla, silver string
	}{
		{"pm2_5 en el máximo", "PM2.5", "ug/m3", "2024-03-04", "1000", "", "1000"},
		{"pm2_5 sobre el máximo", "PM2.5", "ug/m3", "2024-03-04", "1000.1", "fuera_de_rango", ""},
		{"pm10 en cero: sin dato", "PM10", "ug/m3", "2024-03-04", "0", "cero_como_faltante", ""},
		{"pm10 menos cero: sin dato", "PM10", "ug/m3", "2024-03-04", "-0.0", "cero_como_faltante", ""},
		{"pm10 casi cero", "PM10", "ug/m3", "2024-03-04", "0.01", "", "0.01"},
		{"viento en cero: calma", "VELOCIDAD_VIENTO", "m/s", "2024-03-04", "0", "", "0"},
		{"viento menos cero", "VELOCIDAD_VIENTO", "m/s", "2024-03-04", "-0.0", "", "0"},
		{"pm10 negativo", "PM10", "ug/m3", "2024-03-04", "-0.1", "negativo", ""},
		{"temperatura negativa válida", "TEMPERATURA", "°c", "2024-03-04", "-5.5", "", "-5.5"},
		{"temperatura en el mínimo", "TEMPERATURA", "°c", "2024-03-04", "-30", "", "-30"},
		{"temperatura bajo el mínimo", "TEMPERATURA", "°c", "2024-03-04", "-30.1", "fuera_de_rango", ""},
		{"humedad en 100", "HUMEDAD_RELATIVA", "%", "2024-03-04", "100", "", "100"},
		{"humedad sobre 100", "HUMEDAD_RELATIVA", "%", "2024-03-04", "100.01", "fuera_de_rango", ""},
		{"centinela", "PM10", "ug/m3", "2024-03-04", "-999", "centinela", ""},
		{"casi centinela", "PM10", "ug/m3", "2024-03-04", "-999.5", "negativo", ""},
		{"CO en ppb en el máximo", "MONOXIDO_CARBONO", "ppb", "2024-03-04", "43668.1", "", "49999.9745"},
		{"CO en ppb sobre el máximo", "MONOXIDO_CARBONO", "ppb", "2024-03-04", "43669", "fuera_de_rango", ""},
		{"valor con espacios", "PM10", "ug/m3", "2024-03-04", " 12.5 ", "", "12.5"},
		{"valor infinito", "PM10", "ug/m3", "2024-03-04", "Inf", "valor_no_numerico", ""},
		{"límite de detección", "PM10", "ug/m3", "2024-03-04", "<LD", "valor_no_numerico", ""},
		{"primer día válido", "PM10", "ug/m3", "2000-01-01", "12", "", "12"},
		{"antes del inicio", "PM10", "ug/m3", "1999-12-31", "12", "fecha_fuera_de_rango", ""},
		{"último día válido", "PM10", "ug/m3", "2026-06-30", "12", "", "12"},
		{"fecha de corte excluida", "PM10", "ug/m3", "2026-07-01", "12", "fecha_fuera_de_rango", ""},
		{"fecha imposible", "PM10", "ug/m3", "2023-02-29", "12", "fecha", ""},
		{"fecha NA", "PM10", "ug/m3", "NA", "12", "fecha", ""},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			fila := filaOEFA(func(f map[string]string) {
				f["PM10"], f[c.columna], f["UNIDAD_MEDIDA"], f["FECHA_INICIO"] = "-", c.valor, c.unidad, c.fecha
			})
			cfg := ConfigPorDefecto()
			cfg.BloquesEnCero = nil // una sola fila en 0 sería un «día en cero»: aquí se prueba el borde del rango
			res := cargarBytes(t, csvOEFA(fila), cfg, RecetaProvisional(), opMuestra)
			fl := res.Limpieza.FilasLargas
			if c.regla != "" {
				if fl.DescartadasPorRegla[c.regla] != 1 || fl.Procesadas != 0 {
					t.Fatalf("se esperaba un descarte por %q: %+v", c.regla, fl.DescartadasPorRegla)
				}
				return
			}
			if fl.Procesadas != 1 {
				t.Fatalf("se esperaba conservarla: %+v", fl.DescartadasPorRegla)
			}
			lineas := strings.Split(strings.TrimSpace(string(silverDe(t, res))), "\n")
			campos := strings.Split(lineas[1], ",")
			if got := campos[len(campos)-2]; got != c.silver {
				t.Errorf("silver %q, quiero %q", got, c.silver)
			}
		})
	}
}

// TestMuestraReal carga el recorte del CSV real de OEFA (data/sample/oefa_real_muestra.csv).
func TestMuestraReal(t *testing.T) {
	csv, err := os.ReadFile(rutaMuestraReal)
	if err != nil {
		t.Fatal(err)
	}
	res := cargarBytes(t, csv, ConfigPorDefecto(), RecetaProvisional(), Opciones{Workers: 4, Bloque: 64 << 10})
	l := res.Limpieza
	fl := l.FilasLargas
	if fl.Leidas != 4491 || fl.DescartadasPorRegla["columnas"] != 0 || len(l.Cabecera) != 36 || l.BOM {
		t.Fatalf("lectura: %d registros (quiero 4491; dos ocupan tres líneas), columnas %d, cabecera %d",
			fl.Leidas, fl.DescartadasPorRegla["columnas"], len(l.Cabecera))
	}
	if fl.Leidas != fl.Procesadas+fl.Descartadas || fl.Procesadas != fl.HoraExplicita+fl.HoraInferida {
		t.Fatalf("conservación: %+v", fl)
	}
	// Presión 0 mmHg (sensor sin dato) cae en fuera_de_rango; cada gas y la
	// presión de CA-JUPR-01 vienen en dos unidades, así que no queda ninguno
	// convertido. Los días de CA-JUPR-01 con filas vacías conservan su hora por el
	// bloque físico: ninguno queda en la regla hora. Los gases de CA-TA-01 tienen
	// horas sueltas en 0.
	if fl.HoraExplicita == 0 || fl.HoraInferida == 0 || fl.DescartadasPorRegla["duplicado_unidad"] == 0 ||
		fl.DescartadasPorRegla["periodo"] == 0 || fl.DescartadasPorRegla["sin_valor"] == 0 ||
		fl.HoraPorBloqueFisico == 0 || fl.DescartadasPorRegla["hora"] != 0 ||
		fl.DescartadasPorRegla["cero_como_faltante"] == 0 || fl.FueraDeRangoPorParametro["presion"] == 0 {
		t.Errorf("la muestra real tiene hora explícita e inferida, gases en dos unidades, móviles, filas vacías y ceros sueltos: %+v", fl)
	}
	if len(l.Estaciones["CA-VMP-2"].UbicacionesDistintas) < 2 {
		t.Errorf("CA-VMP-2 aparece con dos juegos de coordenadas: %+v", l.Estaciones["CA-VMP-2"])
	}
	// CO de CA-TA-01 en estos días viene en 0 en las dos unidades (bloque_en_cero).
	for _, p := range []string{"pm10", "pm2_5", "so2", "o3", "h2s", "temperatura", "presion"} {
		if !slices.Contains(res.Ancha.Parametros, p) {
			t.Errorf("falta %s en el silver: %v", p, res.Ancha.Parametros)
		}
	}
	otro := cargarBytes(t, csv, ConfigPorDefecto(), RecetaProvisional(), Opciones{Workers: 8, Bloque: 3 << 10, Chunk: 50})
	uno := cargarBytes(t, csv, ConfigPorDefecto(), RecetaProvisional(), Opciones{Workers: 1, Bloque: 3 << 10, Chunk: 50, secuencial: true})
	if !bytes.Equal(silverDe(t, res), silverDe(t, otro)) || !bytes.Equal(goldDe(t, uno), goldDe(t, otro)) {
		t.Error("la muestra real con bloques de 3 KB y W = 8 da otro resultado")
	}
}
