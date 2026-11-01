package datos

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
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

var opMuestra = Opciones{Workers: 4, Bloque: 8 << 10, Chunk: 97}

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
		t.Errorf("leídas %d, el archivo tiene %d filas de datos", fl.Leidas, m.leidas)
	}
	if fl.Descartadas != suma {
		t.Errorf("descartadas %d, la suma por regla da %d", fl.Descartadas, suma)
	}
	if fl.Leidas != fl.Procesadas+fl.Descartadas {
		t.Errorf("leídas (%d) != procesadas (%d) + descartadas (%d)", fl.Leidas, fl.Procesadas, fl.Descartadas)
	}
	var porEstacion int64
	for _, e := range res.Limpieza.Estaciones {
		porEstacion += e.LargasProcesadas
	}
	if porEstacion != fl.Procesadas {
		t.Errorf("las procesadas por estación suman %d, el total es %d", porEstacion, fl.Procesadas)
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
				t.Fatalf("silver distinto de la verdad en la línea %d:\n got   %s\n quiero %s", i+1, gl[i], ql[i])
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
	if got := res.Limpieza.FilasLargas.MarcadasPorRegla["duplicado_valor_distinto"]; got != m.conflictos {
		t.Errorf("celdas marcadas por duplicado con valor distinto: %d, se sembraron %d", got, m.conflictos)
	}
	if got := res.Limpieza.FilasAnchas.Pm25MayorPm10; got != m.violac {
		t.Errorf("filas con pm2_5 > pm10: %d, se sembraron %d", got, m.violac)
	}
}

func TestCompletitudParaElGold(t *testing.T) {
	m := generarMuestra()
	res := cargarBytes(t, m.csv, ConfigPorDefecto(), RecetaProvisional(), opMuestra)
	quiero := 0
	for _, f := range m.verdad {
		pm10, pm25 := f[idxPM10], f[idxPM25]
		if !math.IsNaN(pm10) && pm10 > 0 && !math.IsNaN(pm25) &&
			!math.IsNaN(f[indiceParametro("viento_velocidad")]) && !math.IsNaN(f[indiceParametro("humedad")]) {
			quiero++
		}
	}
	if res.Dataset.N != quiero || res.Limpieza.FilasAnchas.CompletasGold != quiero {
		t.Errorf("filas completas: dataset %d, reporte %d, la verdad dice %d",
			res.Dataset.N, res.Limpieza.FilasAnchas.CompletasGold, quiero)
	}
	if res.Dataset.N >= res.Ancha.N {
		t.Error("la muestra tiene huecos: no todas las filas anchas pueden estar completas")
	}
	var suma int
	for _, e := range res.Limpieza.Estaciones {
		suma += e.CompletasGold
	}
	if suma != quiero {
		t.Errorf("completas por estación suman %d, quiero %d", suma, quiero)
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

func TestMismoResultadoConUnoYOchoWorkers(t *testing.T) {
	m := generarMuestra()
	for _, receta := range []Receta{RecetaProvisional(), func() Receta {
		r := RecetaProvisional()
		r.Nivel = NivelEstacion
		return r
	}()} {
		var ref *Resultado
		for _, bloque := range []int64{1 << 10, 7 << 10, 64 << 10, 64 << 20} {
			uno := cargarBytes(t, m.csv, ConfigPorDefecto(), receta, Opciones{Workers: 1, Bloque: bloque, Chunk: 97})
			ocho := cargarBytes(t, m.csv, ConfigPorDefecto(), receta, Opciones{Workers: 8, Bloque: bloque, Chunk: 97})
			if !bytes.Equal(silverDe(t, uno), silverDe(t, ocho)) {
				t.Fatalf("nivel %s, B=%d: el silver cambia entre W=1 y W=8", receta.Nivel, bloque)
			}
			if !bytes.Equal(goldDe(t, uno), goldDe(t, ocho)) {
				t.Fatalf("nivel %s, B=%d: el gold cambia entre W=1 y W=8", receta.Nivel, bloque)
			}
			mismoDataset(t, uno.Dataset, ocho.Dataset)
			if !reflect.DeepEqual(resultadoComparable(uno.Limpieza), resultadoComparable(ocho.Limpieza)) {
				t.Fatalf("nivel %s, B=%d: el reporte de limpieza cambia entre W=1 y W=8", receta.Nivel, bloque)
			}
			if !reflect.DeepEqual(uno.Estandarizacion, ocho.Estandarizacion) {
				t.Fatalf("nivel %s, B=%d: la estandarización cambia entre W=1 y W=8", receta.Nivel, bloque)
			}
			if bloque == 1<<10 && uno.Limpieza.Bloques < 20 {
				t.Fatalf("con B=1 KB la muestra tendría que partirse en muchos bloques, dio %d", uno.Limpieza.Bloques)
			}
			// Tampoco el tamaño de bloque cambia nada: ni el silver, ni el gold
			// (mismo chunk), ni los conteos por regla.
			if ref == nil {
				ref = uno
				continue
			}
			if !bytes.Equal(silverDe(t, ref), silverDe(t, ocho)) || !bytes.Equal(goldDe(t, ref), goldDe(t, ocho)) {
				t.Fatalf("nivel %s: B=%d cambia el silver o el gold respecto de B=1 KB", receta.Nivel, bloque)
			}
			if !reflect.DeepEqual(sinBloque(ref.Limpieza), sinBloque(ocho.Limpieza)) {
				t.Fatalf("nivel %s: B=%d cambia los conteos de limpieza respecto de B=1 KB", receta.Nivel, bloque)
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
	if !slices.IsSorted(res.Ancha.Estaciones) {
		t.Error("la lista de estaciones tiene que estar ordenada")
	}
	if !ordenadas(res.Ancha.Estaciones, res.Ancha.Estacion, res.Ancha.Hora) {
		t.Error("las filas anchas no están ordenadas por (estación, fecha_hora) sin repetidos")
	}
	if !ordenadas(res.Dataset.Estaciones, res.Dataset.Estacion, res.Dataset.Hora) {
		t.Error("las filas del gold no están ordenadas por (estación, fecha_hora)")
	}
	if !slices.Equal(res.Ancha.Estaciones, []string{"SINT-A Lima Norte", "SINT-B Arequipa", "SINT-C Junín", "SINT-D Ucayali"}) {
		t.Errorf("estaciones %q", res.Ancha.Estaciones)
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
		if got := string(silverDe(t, res)); got != "estacion,fecha_hora\n" {
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
	fila := "I;P;EST X;LIMA;LIMA;LIMA;150101;1;2;3;18L;WGS84;PM10;ug/m3;04/03/2024 05:00;33.5;Horario"
	res := cargarBytes(t, []byte(cabeceraMuestra+"\n"+fila), ConfigPorDefecto(), RecetaProvisional(), opMuestra)
	if got := string(silverDe(t, res)); got != "estacion,fecha_hora,pm10\nEST X,2024-03-04T05:00:00-05:00,33.5\n" {
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
	if !bytes.HasPrefix(m.csv, bomUTF8) {
		t.Fatal("la muestra trae BOM a propósito")
	}
	con := cargarBytes(t, m.csv, ConfigPorDefecto(), RecetaProvisional(), opMuestra)
	sin := cargarBytes(t, m.csv[len(bomUTF8):], ConfigPorDefecto(), RecetaProvisional(), opMuestra)
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
	// Con la config en ';' y el archivo en ',', la cabecera no tiene las columnas.
	if _, err := CargarDesde(bytes.NewReader(conComa), int64(len(conComa)), "x", ConfigPorDefecto(), RecetaProvisional(), opMuestra); err == nil ||
		!strings.Contains(err.Error(), "cabecera real") {
		t.Errorf("delimitador equivocado: tiene que fallar mostrando la cabecera real, error: %v", err)
	}
}

func TestFinDeLineaCRLF(t *testing.T) {
	m := generarMuestra()
	base := silverDe(t, cargarBytes(t, m.csv, ConfigPorDefecto(), RecetaProvisional(), opMuestra))
	crlf := bytes.ReplaceAll(m.csv, []byte("\n"), []byte("\r\n"))
	if !bytes.Equal(silverDe(t, cargarBytes(t, crlf, ConfigPorDefecto(), RecetaProvisional(), opMuestra)), base) {
		t.Error("CRLF cambió el resultado")
	}
}

func TestCampoConSaltoDeLineaFallaConMensajeClaro(t *testing.T) {
	csv := cabeceraMuestra + "\n" +
		"I;\"procedencia con\nsalto\";EST X;LIMA;LIMA;LIMA;150101;1;2;3;18L;WGS84;PM10;ug/m3;04/03/2024 05:00;33.5;Horario\n"
	_, err := CargarDesde(strings.NewReader(csv), int64(len(csv)), "x", ConfigPorDefecto(), RecetaProvisional(), opMuestra)
	if err == nil || !strings.Contains(err.Error(), "salto de línea") {
		t.Fatalf("un campo con salto de línea tiene que fallar explicándolo, error: %v", err)
	}
}

func TestCamposEntrecomilladosSinSaltoSeAceptan(t *testing.T) {
	csv := cabeceraMuestra + "\n" +
		"I;\"Vigilancia; continua\";\"EST X\";LIMA;LIMA;LIMA;150101;1;2;3;18L;WGS84;\"PM10\";ug/m3;04/03/2024 05:00;\"33.5\";Horario\n"
	res := cargarBytes(t, []byte(csv), ConfigPorDefecto(), RecetaProvisional(), opMuestra)
	if got := string(silverDe(t, res)); got != "estacion,fecha_hora,pm10\nEST X,2024-03-04T05:00:00-05:00,33.5\n" {
		t.Errorf("campos entrecomillados: %q", got)
	}
}

func TestDuplicadoEntreBloquesConservaElPrimero(t *testing.T) {
	fila := func(est, fecha, valor string) string {
		return "I;P;" + est + ";LIMA;LIMA;LIMA;150101;1;2;3;18L;WGS84;PM10;ug/m3;" + fecha + ";" + valor + ";Horario\n"
	}
	var b strings.Builder
	b.WriteString(cabeceraMuestra + "\n")
	b.WriteString(fila("EST X", "04/03/2024 05:00", "10"))
	for h := 0; h < 200; h++ { // relleno para que el duplicado caiga en otro bloque
		b.WriteString(fila("EST Y", inicioMuestra.Add(timeHora(h)).Format("02/01/2006 15:04"), "5"))
	}
	b.WriteString(fila("EST X", "04/03/2024 05:00", "20"))
	b.WriteString(fila("EST X", "04/03/2024 05:00", "10"))
	// El 20 y el segundo 10 caen juntos en el mismo bloque, lejos del primero:
	// el segundo 10 es duplicado exacto del valor conservado aunque en su
	// bloque lo preceda un 20. Con un solo bloque tiene que dar lo mismo.
	for _, op := range []Opciones{{Workers: 1, Bloque: 512}, {Workers: 8, Bloque: 512}, {Workers: 8, Bloque: 1 << 30}} {
		w := op.Workers
		res := cargarBytes(t, []byte(b.String()), ConfigPorDefecto(), RecetaProvisional(), op)
		if op.Bloque == 512 && res.Limpieza.Bloques < 10 {
			t.Fatalf("la prueba necesita varios bloques, hay %d", res.Limpieza.Bloques)
		}
		silver := string(silverDe(t, res))
		if !strings.HasPrefix(silver, "estacion,fecha_hora,pm10\nEST X,2024-03-04T05:00:00-05:00,10\n") {
			t.Errorf("W=%d: se tiene que conservar el primer valor en orden de archivo:\n%s", w, silver[:80])
		}
		fl := res.Limpieza.FilasLargas
		if fl.DescartadasPorRegla["duplicado_valor_distinto"] != 1 || fl.DescartadasPorRegla["duplicado_exacto"] != 1 ||
			fl.MarcadasPorRegla["duplicado_valor_distinto"] != 1 {
			t.Errorf("W=%d: conteos de duplicados %+v", w, fl)
		}
	}
}

func TestSeparadorDecimalComa(t *testing.T) {
	csv := cabeceraMuestra + "\n" +
		"I;P;EST X;LIMA;LIMA;LIMA;150101;1;2;3;18L;WGS84;PM10;ug/m3;04/03/2024 05:00;33,5;Horario\n" +
		"I;P;EST X;LIMA;LIMA;LIMA;150101;1;2;3;18L;WGS84;PM2.5;ug/m3;04/03/2024 05:00;\"12,25\";Horario\n"
	cfg := ConfigPorDefecto()
	cfg.SeparadorDecimal = ","
	res := cargarBytes(t, []byte(csv), cfg, RecetaProvisional(), opMuestra)
	if got := string(silverDe(t, res)); got != "estacion,fecha_hora,pm10,pm2_5\nEST X,2024-03-04T05:00:00-05:00,33.5,12.25\n" {
		t.Errorf("decimal con coma: %q", got)
	}
}

func TestCodificacionLatin1(t *testing.T) {
	utf8 := cabeceraMuestra + "\n" +
		"I;P;EST Junín;JUNÍN;YAULI;LA OROYA;120801;1;2;3;18L;WGS84;Dirección del Viento;°;04/03/2024 05:00;270;Horario\n"
	latin := make([]byte, 0, len(utf8))
	for _, r := range utf8 {
		latin = append(latin, byte(r)) // todos los caracteres de la prueba caben en Latin-1
	}
	cfg := ConfigPorDefecto()
	cfg.Codificacion = "latin1"
	res := cargarBytes(t, latin, cfg, RecetaProvisional(), opMuestra)
	if got := string(silverDe(t, res)); got != "estacion,fecha_hora,viento_direccion\nEST Junín,2024-03-04T05:00:00-05:00,270\n" {
		t.Errorf("latin1: %q", got)
	}
	if res.Limpieza.Estaciones["EST Junín"].Ubicacion["departamento"] != "JUNÍN" {
		t.Errorf("ubicación mal decodificada: %+v", res.Limpieza.Estaciones)
	}
}

func TestHoraEnColumnaAparte(t *testing.T) {
	csv := "ESTACION;PARAMETRO;FECHA;HORA;VALOR\n" +
		"EST X;PM10;04/03/2024;05:00;33.5\n" +
		"EST X;PM10;04/03/2024;06:00;34\n"
	cfg := ConfigPorDefecto()
	cfg.Columnas = Columnas{Estacion: "ESTACION", Fecha: "FECHA", Hora: "HORA", Parametro: "PARAMETRO", Valor: "VALOR"}
	res := cargarBytes(t, []byte(csv), cfg, RecetaProvisional(), opMuestra)
	quiero := "estacion,fecha_hora,pm10\nEST X,2024-03-04T05:00:00-05:00,33.5\nEST X,2024-03-04T06:00:00-05:00,34\n"
	if got := string(silverDe(t, res)); got != quiero {
		t.Errorf("hora aparte: %q", got)
	}
	if res.Limpieza.FilasLargas.DescartadasPorRegla["periodo"] != 0 {
		t.Error("sin columna de periodo, la regla 1 no aplica")
	}
}

func TestUnidadEnElNombreDelParametro(t *testing.T) {
	csv := "ESTACION;PARAMETRO;FECHA;VALOR\n" +
		"EST X;CO (mg/m3);04/03/2024 05:00;1.25\n"
	cfg := ConfigPorDefecto()
	cfg.Columnas = Columnas{Estacion: "ESTACION", Fecha: "FECHA", Parametro: "PARAMETRO", Valor: "VALOR"}
	cfg.Parametros = map[string]MapeoParametro{"CO (mg/m3)": {"co", 1000}}
	res := cargarBytes(t, []byte(csv), cfg, RecetaProvisional(), opMuestra)
	if got := string(silverDe(t, res)); got != "estacion,fecha_hora,co\nEST X,2024-03-04T05:00:00-05:00,1250\n" {
		t.Errorf("factor en el mapeo: %q", got)
	}
}

func TestVistosParaAjustarLaConfig(t *testing.T) {
	m := generarMuestra()
	res := cargarBytes(t, m.csv, ConfigPorDefecto(), RecetaProvisional(), opMuestra)
	v := res.Limpieza.Vistos
	if v.ParametrosNoMapeados["Benceno"] != 5 {
		t.Errorf("parámetros no mapeados: %v", v.ParametrosNoMapeados)
	}
	if v.Parametros["PM10"] == 0 || v.Parametros["Benceno"] != 5 {
		t.Errorf("parámetros vistos: %v", v.Parametros)
	}
	if v.Periodos["Diario"] != 14 || v.Periodos["Mensual"] != 1 || v.Periodos["Horario"] == 0 {
		t.Errorf("periodos vistos: %v", v.Periodos)
	}
	if v.Unidades["no2"]["ppb"] != 1 || v.Unidades["co"]["mg/m3"] == 0 || v.Unidades["co"]["ppm"] != 1 {
		t.Errorf("unidades vistas: %v", v.Unidades)
	}
}

func TestReportePorEstacion(t *testing.T) {
	m := generarMuestra()
	res := cargarBytes(t, m.csv, ConfigPorDefecto(), RecetaProvisional(), opMuestra)
	c, ok := res.Limpieza.Estaciones["SINT-C Junín"]
	if !ok {
		t.Fatalf("falta la estación C: %v", res.Limpieza.Estaciones)
	}
	if c.Ubicacion["departamento"] != "JUNÍN" || c.Ubicacion["zona"] != "18L" {
		t.Errorf("ubicación: %v", c.Ubicacion)
	}
	if c.Desde != "2024-03-04T00:00:00-05:00" || c.Hasta != "2024-03-10T23:00:00-05:00" {
		t.Errorf("rango de fechas de C: %s a %s", c.Desde, c.Hasta)
	}
	if c.Anchas != horasMuestra-24 {
		t.Errorf("C estuvo apagada 24 h: filas anchas %d, quiero %d", c.Anchas, horasMuestra-24)
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

func timeHora(h int) time.Duration { return time.Duration(h) * time.Hour }
