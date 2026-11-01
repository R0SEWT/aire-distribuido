package datos

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
)

// regla identifica por qué se descartó una fila larga. El orden es el de la
// cascada: cada fila descartada se atribuye a la primera regla que falla, así
// que leídas = procesadas + Σ descartadas, sin solapes.
type regla int

const (
	reglaColumnas               regla = iota // la fila no tiene tantos campos como la cabecera
	reglaEstacion                            // estación vacía
	reglaPeriodo                             // regla 1: periodo no horario
	reglaParametro                           // parámetro sin mapeo a un canónico
	reglaFecha                               // regla 2: fecha y hora no parseables
	reglaFechaFueraDeRango                   // regla 2: fuera de [fecha_desde, fecha_hasta)
	reglaValorNoNumerico                     // regla 3: vacío, texto, NaN o infinito
	reglaCentinela                           // regla 3: código del equipo (-999, 9999, ...)
	reglaUnidad                              // regla 6: unidad que no está en la tabla del parámetro
	reglaNegativo                            // regla 3: negativo en un parámetro que no los admite
	reglaFueraDeRango                        // regla 4: fuera del rango físico del parámetro
	reglaDuplicadoExacto                     // regla 5: misma (estación, parámetro, fecha-hora) y mismo valor
	reglaDuplicadoValorDistinto              // regla 5: misma clave, otro valor; se conserva el primero
	numReglas
)

var nombresReglas = [numReglas]string{
	"columnas", "estacion", "periodo", "parametro", "fecha", "fecha_fuera_de_rango",
	"valor_no_numerico", "centinela", "unidad", "negativo", "fuera_de_rango",
	"duplicado_exacto", "duplicado_valor_distinto",
}

// Reglas devuelve los nombres de las reglas de descarte en orden de cascada.
// Son las claves de descartadas_por_regla en reports/limpieza.json.
func Reglas() []string { return slices.Clone(nombresReglas[:]) }

// claveLocal es (estación, fecha-hora) dentro de un bloque; la estación es un
// índice en la tabla de estaciones del bloque.
type claveLocal struct {
	est int32
	t   int64
}

type estacionBloque struct {
	nombre             string
	ubicacion          map[string]string
	leidas, procesadas int64
}

// entradaTexto cachea, por bloque, la decisión sobre un valor de texto crudo
// (un periodo, un parámetro o una unidad) y cuenta cuántas veces apareció.
type entradaTexto struct {
	nombre string // decodificado a UTF-8 y recortado
	n      int64
	valida bool    // periodo horario, o unidad aceptada
	param  int     // índice canónico, o -1 si el parámetro no está mapeado
	factor float64 // a la unidad canónica
}

// parcial es lo que un worker entrega por bloque: su mapa (estación,
// fecha-hora) → vector de parámetros, como claves en orden de aparición más
// valores planos, y sus conteos. No comparte nada con otros workers.
type parcial struct {
	indice int
	err    error

	estaciones []estacionBloque
	claves     []claveLocal
	valores    []float64 // len(claves)*numParametros; NaN = sin lectura
	// repetidas son las lecturas de una celda que ya tenía valor en el bloque,
	// en orden de archivo. El worker no las clasifica: si son duplicado exacto
	// o con otro valor depende del valor que se conserve, que puede venir de
	// un bloque anterior. Lo decide el recolector.
	repetidas []repetida

	leidas, procesadas int64
	descartadas        [numReglas]int64
	fueraDeRango       [numParametros]int64

	periodos   []entradaTexto
	parametros []entradaTexto
	unidades   [numParametros][]entradaTexto
}

type repetida struct {
	fila  int32 // índice en claves
	param int
	v     float64
}

const (
	fechaOK uint8 = iota
	fechaInvalida
	fechaFueraDeRango
)

type fechaParseada struct {
	t      int64
	estado uint8
}

// maxCacheFechas acota la caché de fechas de cada worker. Con lecturas
// horarias hay ~9 000 fechas distintas por año, así que casi nunca se vacía.
const maxCacheFechas = 1 << 20

// trabajador es el estado de una goroutine del pool. El búfer y las cachés de
// fechas son suyos: se reutilizan entre bloques y nadie más los toca. La caché
// de fechas es una función pura de los bytes, así que reutilizarla no cambia
// ningún resultado.
type trabajador struct {
	reg    *reglas
	cols   indicesColumnas
	delim  byte
	buf    []byte
	div    divisor
	fechas map[string]fechaParseada
	tmpF   []byte
	tmpV   []byte
}

func nuevoTrabajador(reg *reglas, cols indicesColumnas, delim byte) *trabajador {
	return &trabajador{reg: reg, cols: cols, delim: delim, fechas: make(map[string]fechaParseada)}
}

var filaVacia = func() (f [numParametros]float64) {
	for i := range f {
		f[i] = math.NaN()
	}
	return f
}()

// procesar lee el bloque con ReadAt (seguro para uso concurrente sobre el mismo
// *os.File), parsea sus filas largas, aplica las reglas 1 a 6 y acumula el
// pivot parcial. Los duplicados se apartan aquí y se clasifican en el
// recolector, que es el único que sabe qué valor se conserva.
func (w *trabajador) procesar(r io.ReaderAt, indice int, rg rango) *parcial {
	p := &parcial{indice: indice}
	n := int(rg.hasta - rg.desde)
	if cap(w.buf) < n {
		w.buf = make([]byte, n)
	}
	datos := w.buf[:n]
	if leidos, err := r.ReadAt(datos, rg.desde); leidos < n {
		p.err = fmt.Errorf("leer el bloque %d (bytes %d a %d): %w", indice, rg.desde, rg.hasta, err)
		return p
	}

	var (
		estaciones = map[string]int32{}
		porNombre  = map[string]int32{}
		periodos   = map[string]*entradaTexto{}
		params     = map[string]*entradaTexto{}
		unidades   [numParametros]map[string]*entradaTexto
		filas      = map[claveLocal]int32{}
		cols       = w.cols
		reg        = w.reg
	)
	pos := rg.desde
	for len(datos) > 0 {
		var linea []byte
		if i := bytes.IndexByte(datos, '\n'); i >= 0 {
			linea, datos = datos[:i], datos[i+1:]
		} else {
			linea, datos = datos, nil
		}
		inicioLinea := pos
		pos += int64(len(linea)) + 1
		if k := len(linea); k > 0 && linea[k-1] == '\r' {
			linea = linea[:k-1]
		}
		if len(linea) == 0 {
			continue // una línea vacía no es una fila
		}
		p.leidas++

		campos, err := w.div.dividir(linea, w.delim)
		if err != nil {
			p.err = fmt.Errorf("bloque %d, línea que empieza en el byte %d: %w", indice, inicioLinea, err)
			return p
		}
		if len(campos) != cols.total {
			p.descartadas[reglaColumnas]++
			continue
		}

		crudo := bytes.TrimSpace(campos[cols.estacion])
		if len(crudo) == 0 {
			p.descartadas[reglaEstacion]++
			continue
		}
		ei, ok := estaciones[string(crudo)]
		if !ok {
			ei = w.registrarEstacion(p, porNombre, crudo, campos)
			estaciones[string(crudo)] = ei
		}
		est := &p.estaciones[ei]
		est.leidas++

		// Periodo, parámetro y unidad se buscan en todas las filas, antes de la
		// cascada, para que los conteos de «vistos» no dependan del orden de las
		// reglas: son lo que se mira el lunes para ajustar la config.
		horario := true
		if cols.periodo >= 0 {
			e := w.buscarTexto(periodos, campos[cols.periodo], func(e *entradaTexto) {
				e.valida = reg.periodos[normalizar(e.nombre)]
			})
			e.n++
			horario = e.valida
		}
		par := w.buscarTexto(params, campos[cols.parametro], func(e *entradaTexto) {
			e.param, e.factor = -1, 1
			if m, ok := reg.parametros[normalizar(e.nombre)]; ok {
				e.param, e.factor = indiceParametro(m.Canonico), m.Factor
			}
		})
		par.n++
		pi, factor, unidadOK := par.param, par.factor, true
		if pi >= 0 && cols.unidad >= 0 {
			if unidades[pi] == nil {
				unidades[pi] = map[string]*entradaTexto{}
			}
			u := w.buscarTexto(unidades[pi], campos[cols.unidad], func(e *entradaTexto) {
				e.valida, e.factor = true, 1
				if tabla := reg.unidades[pi]; tabla != nil {
					e.factor, e.valida = tabla[normalizar(e.nombre)]
				}
			})
			u.n++
			unidadOK = u.valida
			factor *= u.factor
		}

		switch {
		case !horario:
			p.descartadas[reglaPeriodo]++
			continue
		case pi < 0:
			p.descartadas[reglaParametro]++
			continue
		}
		t, estado := w.fecha(campos)
		switch estado {
		case fechaInvalida:
			p.descartadas[reglaFecha]++
			continue
		case fechaFueraDeRango:
			p.descartadas[reglaFechaFueraDeRango]++
			continue
		}
		v, ok := w.valor(campos[cols.valor])
		switch {
		case !ok:
			p.descartadas[reglaValorNoNumerico]++
			continue
		case slices.Contains(reg.centinelas, v):
			p.descartadas[reglaCentinela]++
			continue
		case !unidadOK:
			p.descartadas[reglaUnidad]++
			continue
		case v < 0 && !reg.negativos[pi]:
			p.descartadas[reglaNegativo]++
			continue
		}
		if factor != 1 {
			v *= factor
		}
		if v == 0 {
			v = 0 // -0 se escribiría "-0" en el silver
		}
		if reg.conRango[pi] && (v < reg.rango[pi][0] || v > reg.rango[pi][1]) {
			p.descartadas[reglaFueraDeRango]++
			p.fueraDeRango[pi]++
			continue
		}

		// Pivot parcial: en orden de archivo, la primera lectura de cada celda
		// ocupa el vector; las siguientes quedan como repetidas (regla 5).
		k := claveLocal{ei, t}
		fi, ok := filas[k]
		if !ok {
			fi = int32(len(p.claves))
			filas[k] = fi
			p.claves = append(p.claves, k)
			p.valores = append(p.valores, filaVacia[:]...)
		}
		if celda := &p.valores[int(fi)*numParametros+pi]; math.IsNaN(*celda) {
			*celda = v
			p.procesadas++
			est.procesadas++
		} else {
			p.repetidas = append(p.repetidas, repetida{fi, pi, v})
		}
	}

	for _, e := range periodos {
		p.periodos = append(p.periodos, *e)
	}
	for _, e := range params {
		p.parametros = append(p.parametros, *e)
	}
	for q, m := range unidades {
		for _, e := range m {
			p.unidades[q] = append(p.unidades[q], *e)
		}
	}
	return p
}

// buscarTexto devuelve la entrada cacheada para un valor crudo; la primera vez
// lo decodifica y deja que decidir complete la entrada. La búsqueda con
// string(crudo) no asigna memoria: solo se asigna al ver un valor nuevo.
func (w *trabajador) buscarTexto(cache map[string]*entradaTexto, crudo []byte, decidir func(*entradaTexto)) *entradaTexto {
	crudo = bytes.TrimSpace(crudo)
	if e, ok := cache[string(crudo)]; ok {
		return e
	}
	e := &entradaTexto{nombre: strings.TrimSpace(decodificar(crudo, w.reg.cod))}
	decidir(e)
	cache[string(crudo)] = e
	return e
}

// registrarEstacion agrega una estación a la tabla del bloque, con la ubicación
// de la primera fila en que aparece.
func (w *trabajador) registrarEstacion(p *parcial, porNombre map[string]int32, crudo []byte, campos [][]byte) int32 {
	nombre := strings.TrimSpace(decodificar(crudo, w.reg.cod))
	if i, ok := porNombre[nombre]; ok {
		return i
	}
	e := estacionBloque{nombre: nombre}
	for _, u := range w.cols.ubicacion {
		if v := strings.TrimSpace(decodificar(bytes.TrimSpace(campos[u.indice]), w.reg.cod)); v != "" {
			if e.ubicacion == nil {
				e.ubicacion = map[string]string{}
			}
			e.ubicacion[u.nombre] = v
		}
	}
	i := int32(len(p.estaciones))
	p.estaciones = append(p.estaciones, e)
	porNombre[nombre] = i
	return i
}

// fecha parsea la fecha (y la hora, si viene en otra columna) probando los
// formatos de la config en orden, en la zona horaria de la config.
func (w *trabajador) fecha(campos [][]byte) (int64, uint8) {
	crudo := bytes.TrimSpace(campos[w.cols.fecha])
	if w.cols.hora >= 0 {
		w.tmpF = append(append(append(w.tmpF[:0], crudo...), ' '), bytes.TrimSpace(campos[w.cols.hora])...)
		crudo = w.tmpF
	}
	if f, ok := w.fechas[string(crudo)]; ok {
		return f.t, f.estado
	}
	s := string(crudo)
	f := fechaParseada{estado: fechaInvalida}
	for _, formato := range w.reg.formatos {
		t, err := time.ParseInLocation(formato, s, w.reg.zona)
		if err != nil {
			continue
		}
		f = fechaParseada{t: t.Unix(), estado: fechaOK}
		if f.t < w.reg.desde || f.t >= w.reg.hasta {
			f.estado = fechaFueraDeRango
		}
		break
	}
	if len(w.fechas) >= maxCacheFechas {
		clear(w.fechas)
	}
	w.fechas[s] = f
	return f.t, f.estado
}

// valor parsea un número; vacío, texto, NaN e infinito no son lecturas.
func (w *trabajador) valor(crudo []byte) (float64, bool) {
	crudo = bytes.TrimSpace(crudo)
	if len(crudo) == 0 {
		return 0, false
	}
	if w.reg.decimal == ',' && bytes.IndexByte(crudo, ',') >= 0 {
		w.tmpV = append(w.tmpV[:0], crudo...)
		for i, c := range w.tmpV {
			if c == ',' {
				w.tmpV[i] = '.'
			}
		}
		crudo = w.tmpV
	}
	v, err := strconv.ParseFloat(string(crudo), 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	return v, true
}
