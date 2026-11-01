package datos

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// rango es un bloque del archivo, [desde, hasta) en bytes. Empieza al inicio
// de una línea y termina justo después de un salto de línea (o en el fin del
// archivo), así que ninguna fila queda partida entre dos bloques.
type rango struct{ desde, hasta int64 }

// particionar parte [ini, tam) en bloques de al menos `bloque` bytes alineados
// al siguiente salto de línea. Lee solo unos pocos bytes alrededor de cada
// corte: con 1.3 GB y 64 MB son unas 21 lecturas pequeñas.
//
// Un campo entrecomillado puede contener saltos de línea (el CSV de OEFA tiene
// 100): si un corte cae dentro de uno, el recolector lo cose (ver cosedora).
func particionar(r io.ReaderAt, tam, ini, bloque int64) ([]rango, error) {
	if bloque < 1 {
		return nil, fmt.Errorf("el bloque tiene que tener al menos 1 byte, tiene %d", bloque)
	}
	var rangos []rango
	for desde := ini; desde < tam; {
		if bloque >= tam-desde {
			rangos = append(rangos, rango{desde, tam})
			break
		}
		// Se busca desde hasta-1: si ese byte ya es '\n', el bloque mide
		// exactamente B y el siguiente empieza en una línea nueva.
		p, err := siguienteSalto(r, tam, desde+bloque-1)
		if err != nil {
			return nil, err
		}
		if p < 0 {
			rangos = append(rangos, rango{desde, tam})
			break
		}
		rangos = append(rangos, rango{desde, p + 1})
		desde = p + 1
	}
	return rangos, nil
}

// siguienteSalto devuelve la posición del primer '\n' en [desde, tam), o -1.
func siguienteSalto(r io.ReaderAt, tam, desde int64) (int64, error) {
	buf := make([]byte, 64*1024)
	for pos := desde; pos < tam; {
		n := int64(len(buf))
		if n > tam-pos {
			n = tam - pos
		}
		leidos, err := r.ReadAt(buf[:n], pos)
		if int64(leidos) < n && err != nil {
			return 0, fmt.Errorf("leer el byte %d: %w", pos, err)
		}
		if i := bytes.IndexByte(buf[:n], '\n'); i >= 0 {
			return pos + int64(i), nil
		}
		pos += n
	}
	return -1, nil
}

// maxCabecera acota cuánto se lee para encontrar la primera línea.
const maxCabecera = 1 << 20

var bomUTF8 = []byte{0xEF, 0xBB, 0xBF}

// cabecera es la primera línea del archivo, ya decodificada.
type cabecera struct {
	nombres     []string
	delimitador byte
	bom         bool
	fin         int64 // primer byte de los datos
}

// leerCabecera lee y parte la primera línea. Quita el BOM de UTF-8 si está y,
// si la config dice "auto", elige el delimitador más frecuente entre ; , \t y |.
func leerCabecera(r io.ReaderAt, tam int64, reg *reglas) (*cabecera, error) {
	if tam == 0 {
		return nil, errors.New("el archivo está vacío: no tiene ni cabecera")
	}
	n := min(tam, maxCabecera)
	buf := make([]byte, n)
	if leidos, err := r.ReadAt(buf, 0); int64(leidos) < n {
		return nil, fmt.Errorf("leer la cabecera: %w", err)
	}
	cab := &cabecera{}
	linea := buf
	if i := bytes.IndexByte(buf, '\n'); i >= 0 {
		linea, cab.fin = buf[:i], int64(i)+1
	} else if tam > maxCabecera {
		return nil, fmt.Errorf("la primera línea mide más de %d bytes: ¿es un CSV?", maxCabecera)
	} else {
		cab.fin = tam
	}
	linea = bytes.TrimSuffix(linea, []byte{'\r'})
	if bytes.HasPrefix(linea, bomUTF8) {
		linea, cab.bom = linea[len(bomUTF8):], true
	}

	cab.delimitador = reg.delimitador
	if cab.delimitador == 0 {
		cab.delimitador = detectarDelimitador(linea)
	}
	var d divisor
	campos, err := d.dividir(linea, cab.delimitador)
	if err != nil {
		return nil, fmt.Errorf("cabecera: %w", err)
	}
	for _, c := range campos {
		cab.nombres = append(cab.nombres, strings.TrimSpace(decodificar(c, reg.cod)))
	}
	return cab, nil
}

func detectarDelimitador(linea []byte) byte {
	mejor, cuenta := byte(';'), 0
	for _, c := range []byte{';', ',', '\t', '|'} {
		if n := bytes.Count(linea, []byte{c}); n > cuenta {
			mejor, cuenta = c, n
		}
	}
	return mejor
}

// indicesColumnas son las posiciones de cada columna lógica en la fila, o -1
// si el CSV no la tiene.
type indicesColumnas struct {
	estacion, fecha, hora, periodo, unidad int
	params                                 []columnaParametro
	ubicacion                              []columnaUbicacion
	total                                  int
}

type columnaParametro struct {
	indice int    // en la fila
	param  int    // canónico
	nombre string // como viene en la cabecera
}

type columnaUbicacion struct {
	nombre string // lógico: departamento, provincia, ...
	indice int
}

// resolverColumnas busca cada columna configurada en la cabecera, comparando
// nombres normalizados. Faltar una de las que deciden reglas es un error que
// muestra la cabecera real; faltar una de ubicación o una de parámetro (si
// queda al menos una) es solo un aviso.
func resolverColumnas(nombres []string, reg *reglas) (indicesColumnas, []string, error) {
	c := reg.columnas
	pos := make(map[string]int, len(nombres))
	for i, n := range nombres {
		clave := normalizar(n)
		if _, ok := pos[clave]; !ok {
			pos[clave] = i
		}
	}
	cols := indicesColumnas{total: len(nombres)}
	var errs []error
	var avisos []string
	buscar := func(logica, nombre string, obligatoria bool) int {
		if strings.TrimSpace(nombre) == "" {
			if obligatoria {
				errs = append(errs, fmt.Errorf("columnas.%s no está configurada", logica))
			}
			return -1
		}
		i, ok := pos[normalizar(nombre)]
		if !ok {
			errs = append(errs, fmt.Errorf("la cabecera no tiene la columna %q (columnas.%s)", nombre, logica))
			return -1
		}
		return i
	}
	cols.estacion = buscar("estacion", c.Estacion, true)
	cols.fecha = buscar("fecha", c.Fecha, true)
	cols.hora = buscar("hora", c.Hora, false)
	cols.periodo = buscar("periodo", c.Periodo, false)
	cols.unidad = buscar("unidad", c.Unidad, false)
	for i, n := range nombres {
		if q, ok := reg.paramColumnas[normalizar(n)]; ok {
			cols.params = append(cols.params, columnaParametro{indice: i, param: q, nombre: n})
		}
	}
	if len(cols.params) == 0 {
		errs = append(errs, errors.New("la cabecera no tiene ninguna de las columnas de parametros_en_columnas"))
	} else if len(cols.params) < len(reg.paramColumnas) {
		avisos = append(avisos, fmt.Sprintf("la cabecera tiene %d de las %d columnas de parametros_en_columnas",
			len(cols.params), len(reg.paramColumnas)))
	}
	for _, u := range c.ubicacion() {
		if strings.TrimSpace(u[1]) == "" {
			continue
		}
		i, ok := pos[normalizar(u[1])]
		if !ok {
			avisos = append(avisos, fmt.Sprintf("la cabecera no tiene la columna de ubicación %q (columnas.%s): se omite", u[1], u[0]))
			continue
		}
		cols.ubicacion = append(cols.ubicacion, columnaUbicacion{u[0], i})
	}
	if len(errs) > 0 {
		return cols, nil, fmt.Errorf("%w\ncabecera real (%d columnas): %s", errors.Join(errs...), len(nombres),
			strings.Join(nombres, ", "))
	}
	return cols, avisos, nil
}

// errComillaAbierta indica un campo entrecomillado que no se cierra en la
// línea: o tiene saltos de línea dentro (el worker une la línea siguiente) o
// el archivo está roto (error, si no cierra en maxLineasPorRegistro líneas o
// antes del fin del archivo).
var errComillaAbierta = errors.New("campo entrecomillado sin cerrar")

// divisor parte líneas en campos sin asignar memoria por línea: los campos son
// subslices de la línea (o de aux, si hubo que quitar comillas dobles).
type divisor struct {
	campos [][]byte
	aux    []byte
}

// dividir parte una línea (sin el salto final) por el delimitador. Acepta
// campos entrecomillados al estilo RFC 4180 ("a;b", "dijo ""hola"""). Los
// campos devueltos valen hasta la siguiente llamada.
func (d *divisor) dividir(linea []byte, delim byte) ([][]byte, error) {
	d.campos = d.campos[:0]
	if bytes.IndexByte(linea, '"') < 0 { // camino rápido: el caso normal
		for {
			i := bytes.IndexByte(linea, delim)
			if i < 0 {
				d.campos = append(d.campos, linea)
				return d.campos, nil
			}
			d.campos = append(d.campos, linea[:i])
			linea = linea[i+1:]
		}
	}
	// aux nunca crece más que la línea, así que no se realoca y los campos
	// ya entregados siguen apuntando a memoria válida.
	if cap(d.aux) < len(linea) {
		d.aux = make([]byte, 0, len(linea))
	}
	d.aux = d.aux[:0]
	for {
		if len(linea) > 0 && linea[0] == '"' {
			ini, i := len(d.aux), 1
			for {
				j := bytes.IndexByte(linea[i:], '"')
				if j < 0 {
					return nil, errComillaAbierta
				}
				d.aux = append(d.aux, linea[i:i+j]...)
				i += j + 1
				if i < len(linea) && linea[i] == '"' {
					d.aux = append(d.aux, '"')
					i++
					continue
				}
				break
			}
			d.campos = append(d.campos, d.aux[ini:len(d.aux):len(d.aux)])
			resto := linea[i:]
			k := bytes.IndexByte(resto, delim)
			if k < 0 {
				return d.campos, nil
			}
			linea = resto[k+1:]
			continue
		}
		i := bytes.IndexByte(linea, delim)
		if i < 0 {
			d.campos = append(d.campos, linea)
			return d.campos, nil
		}
		d.campos = append(d.campos, linea[:i])
		linea = linea[i+1:]
	}
}

// windows1252 son los caracteres de 0x80 a 0x9F, donde difiere de Latin-1.
var windows1252 = [32]rune{
	'€', '�', '‚', 'ƒ', '„', '…', '†', '‡', 'ˆ', '‰', 'Š', '‹', 'Œ', '�', 'Ž', '�',
	'�', '‘', '’', '“', '”', '•', '–', '—', '˜', '™', 'š', '›', 'œ', '�', 'ž', 'Ÿ',
}

// decodificar pasa un campo a UTF-8. Solo se llama al ver por primera vez un
// valor de texto (estación, parámetro, periodo, unidad) en un bloque, nunca
// por fila: los números y las fechas son ASCII en cualquier codificación.
func decodificar(b []byte, cod codificacion) string {
	switch cod {
	case codLatin1, codWindows1252:
		var s strings.Builder
		s.Grow(len(b) + len(b)/4)
		for _, c := range b {
			switch {
			case c < 0x80:
				s.WriteByte(c)
			case c < 0xA0 && cod == codWindows1252:
				s.WriteRune(windows1252[c-0x80])
			default:
				s.WriteRune(rune(c))
			}
		}
		return s.String()
	default:
		if utf8.Valid(b) {
			return string(b)
		}
		return strings.ToValidUTF8(string(b), "�")
	}
}
