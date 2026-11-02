package datos

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestParticionarAlineaASaltoDeLinea(t *testing.T) {
	texto := "cab\nuno\ndos\ntres\ncuatro\ncinco\n"
	ini := int64(len("cab\n"))
	for b := int64(1); b <= int64(len(texto))+5; b++ {
		rangos, err := particionar(strings.NewReader(texto), int64(len(texto)), ini, b)
		if err != nil {
			t.Fatal(err)
		}
		var juntas []string
		pos := ini
		for _, r := range rangos {
			if r.desde != pos {
				t.Fatalf("B=%d: el rango %v no empieza donde terminó el anterior (%d)", b, r, pos)
			}
			if r.hasta <= r.desde {
				t.Fatalf("B=%d: rango vacío %v", b, r)
			}
			trozo := texto[r.desde:r.hasta]
			if !strings.HasSuffix(trozo, "\n") {
				t.Fatalf("B=%d: el bloque %q no termina en salto de línea", b, trozo)
			}
			juntas = append(juntas, strings.Split(strings.TrimSuffix(trozo, "\n"), "\n")...)
			pos = r.hasta
		}
		if pos != int64(len(texto)) {
			t.Fatalf("B=%d: los rangos no cubren el archivo", b)
		}
		if !slices.Equal(juntas, []string{"uno", "dos", "tres", "cuatro", "cinco"}) {
			t.Fatalf("B=%d: líneas %q", b, juntas)
		}
	}
}

func TestParticionarSinSaltoFinal(t *testing.T) {
	texto := "cab\nuno\ndos"
	rangos, err := particionar(strings.NewReader(texto), int64(len(texto)), 4, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(rangos) != 2 || rangos[1].hasta != int64(len(texto)) {
		t.Fatalf("rangos %v", rangos)
	}
}

func TestParticionarBloqueMasGrandeQueElArchivo(t *testing.T) {
	texto := "cab\nuno\ndos\n"
	rangos, err := particionar(strings.NewReader(texto), int64(len(texto)), 4, 1<<30)
	if err != nil {
		t.Fatal(err)
	}
	if len(rangos) != 1 || rangos[0] != (rango{4, int64(len(texto))}) {
		t.Fatalf("un bloque más grande que el archivo tiene que dar un solo rango: %v", rangos)
	}
}

func TestParticionarSoloCabecera(t *testing.T) {
	texto := "cab\n"
	rangos, err := particionar(strings.NewReader(texto), int64(len(texto)), 4, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rangos) != 0 {
		t.Fatalf("sin datos no hay bloques: %v", rangos)
	}
}

func TestDividirCampos(t *testing.T) {
	casos := []struct {
		linea  string
		delim  byte
		quiero []string
	}{
		{"a;b;c", ';', []string{"a", "b", "c"}},
		{"a;;c;", ';', []string{"a", "", "c", ""}},
		{"", ';', []string{""}},
		{`"a;b";c`, ';', []string{"a;b", "c"}},
		{`"dijo ""hola""";2`, ';', []string{`dijo "hola"`, "2"}},
		{`x,"1,5",y`, ',', []string{"x", "1,5", "y"}},
		{`a"b;c`, ';', []string{`a"b`, "c"}},
		{"\"\";x", ';', []string{"", "x"}},
	}
	var d divisor
	for _, c := range casos {
		campos, err := d.dividir([]byte(c.linea), c.delim)
		if err != nil {
			t.Fatalf("%q: %v", c.linea, err)
		}
		var got []string
		for _, f := range campos {
			got = append(got, string(f))
		}
		if !slices.Equal(got, c.quiero) {
			t.Errorf("dividir(%q) = %q, quiero %q", c.linea, got, c.quiero)
		}
	}
}

func TestDividirComillaSinCerrarFalla(t *testing.T) {
	var d divisor
	_, err := d.dividir([]byte(`a;"b`), ';')
	if !errors.Is(err, errComillaAbierta) {
		t.Fatalf("una comilla sin cerrar tiene que dar errComillaAbierta, dio %v", err)
	}
}

func TestLeerCabecera(t *testing.T) {
	casos := []struct {
		nombre string
		texto  string
		delim  string
		quiero byte
		campos []string
	}{
		{"punto y coma", "A;B;C\nx", ";", ';', []string{"A", "B", "C"}},
		{"coma", "A,B,C\r\nx", ",", ',', []string{"A", "B", "C"}},
		{"con BOM", "\xEF\xBB\xBFA;B;C\n", ";", ';', []string{"A", "B", "C"}},
		{"auto punto y coma", "A;B;C,D\n", "auto", ';', []string{"A", "B", "C,D"}},
		{"auto tab", "A\tB\tC\n", "", '\t', []string{"A", "B", "C"}},
		{"entrecomillada", "\"A\";\" B \";C\n", ";", ';', []string{"A", "B", "C"}},
		{"una sola línea sin salto", "A;B;C", ";", ';', []string{"A", "B", "C"}},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			cfg := ConfigPorDefecto()
			cfg.Delimitador = c.delim
			reg, err := cfg.compilar()
			if err != nil {
				t.Fatal(err)
			}
			cab, err := leerCabecera(strings.NewReader(c.texto), int64(len(c.texto)), reg)
			if err != nil {
				t.Fatal(err)
			}
			if cab.delimitador != c.quiero {
				t.Errorf("delimitador %q, quiero %q", cab.delimitador, c.quiero)
			}
			if !slices.Equal(cab.nombres, c.campos) {
				t.Errorf("campos %q, quiero %q", cab.nombres, c.campos)
			}
			fin := strings.IndexByte(c.texto, '\n') + 1
			if fin == 0 {
				fin = len(c.texto)
			}
			if cab.fin != int64(fin) {
				t.Errorf("fin %d, quiero %d", cab.fin, fin)
			}
		})
	}
}

func TestLeerCabeceraArchivoVacio(t *testing.T) {
	reg, _ := ConfigPorDefecto().compilar()
	if _, err := leerCabecera(strings.NewReader(""), 0, reg); err == nil {
		t.Fatal("un archivo vacío no tiene cabecera: tiene que fallar")
	}
}

func TestResolverColumnas(t *testing.T) {
	reg, _ := ConfigPorDefecto().compilar()
	nombres := columnasOEFA
	cols, avisos, err := resolverColumnas(nombres, reg)
	if err != nil {
		t.Fatal(err)
	}
	if len(avisos) != 0 {
		t.Errorf("la cabecera real tiene todas las columnas: avisos %v", avisos)
	}
	if nombres[cols.estacion] != "NOMBRE_PUNTO" || nombres[cols.hora] != "HORA_INICIO" || len(cols.params) != 13 {
		t.Errorf("columnas mal resueltas: %+v", cols)
	}

	// Mayúsculas, tildes y espacios no importan al resolver.
	cfg := ConfigPorDefecto()
	cfg.Columnas = Columnas{Estacion: "NOMBRE_PUNTO", Fecha: "FECHA_INICIO"}
	reg2, _ := cfg.compilar()
	cols, _, err = resolverColumnas([]string{"nombre_punto", " Fecha_Inicio ", "pm10"}, reg2)
	if err != nil || cols.fecha != 1 || len(cols.params) != 1 || cols.params[0].param != idxPM10 {
		t.Fatalf("resolución tolerante: %+v, %v", cols, err)
	}

	// Una obligatoria que falta es un error que lista la cabecera real.
	_, _, err = resolverColumnas([]string{"ESTACION", "FECHA", "PM10"}, reg)
	if err == nil || !strings.Contains(err.Error(), "ESTACION") {
		t.Fatalf("tiene que fallar y mostrar la cabecera real: %v", err)
	}
	// Ninguna columna de parámetro: error.
	if _, _, err = resolverColumnas([]string{"NOMBRE_PUNTO", "FECHA_INICIO", "HORA_INICIO", "PERIODO", "UNIDAD_MEDIDA"}, reg); err == nil {
		t.Fatal("sin columnas de parámetro tiene que fallar")
	}

	// Una de ubicación o de parámetro que falta solo se avisa.
	sinZona := slices.Clone(columnasOEFA)
	sinZona[slices.Index(sinZona, "ZONA")] = "ZONA_UTM"
	sinZona[slices.Index(sinZona, "OZONO")] = "O3"
	_, avisos, err = resolverColumnas(sinZona, reg)
	if err != nil || len(avisos) != 2 {
		t.Fatalf("una columna de ubicación o de parámetro faltante es un aviso: %v, %v", avisos, err)
	}
}

func TestDecodificar(t *testing.T) {
	latin1 := []byte{'J', 'u', 'n', 0xED, 'n'}
	if got := decodificar(latin1, codLatin1); got != "Junín" {
		t.Errorf("latin1: %q", got)
	}
	cp := []byte{0x93, 'x', 0x94, ' ', 0xB5, 'g'}
	if got := decodificar(cp, codWindows1252); got != "“x” µg" {
		t.Errorf("windows-1252: %q", got)
	}
	if got := decodificar([]byte("Junín"), codUTF8); got != "Junín" {
		t.Errorf("utf-8: %q", got)
	}
	if got := decodificar([]byte{'a', 0xFF}, codUTF8); !bytes.Equal([]byte(got), []byte("a�")) {
		t.Errorf("utf-8 inválido tiene que reemplazarse: %q", got)
	}
}
