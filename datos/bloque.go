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
// cascada (y sigue la numeración de reglas de la spec): cada fila descartada se
// atribuye a la primera regla que falla, así que leídas = procesadas +
// Σ descartadas, sin solapes. Hasta hora_no_en_punto se deciden en el worker,
// fila por fila; el periodo «-», la hora y las reglas de valor, en el
// recolector, que ya conoce el día completo de cada fila; bloque_en_cero, por
// día; los duplicados, al pivotar.
type regla int

const (
	reglaColumnas               regla = iota // la fila no tiene tantos campos como la cabecera
	reglaEstacion                            // estación vacía
	reglaPeriodo                             // regla 1: periodo no horario (o «-» sin hora y con el día incompleto)
	reglaParametro                           // ninguna columna de parámetro con valor, o más de una
	reglaFecha                               // regla 2: fecha no parseable
	reglaFechaFueraDeRango                   // regla 2: fuera de [fecha_desde, fecha_hasta)
	reglaHoraNoEnPunto                       // regla 2: hora con minutos o segundos (política descartar)
	reglaHora                                // regla 2: hora ilegible, o sin hora y su día no tiene filas_por_dia filas contiguas
	reglaValorNoNumerico                     // regla 3: texto (<LD, ND, ...), NaN o infinito
	reglaCentinela                           // regla 3: código del equipo (-999, 9999, ...)
	reglaUnidad                              // regla 6: unidad que no está en la tabla del parámetro
	reglaNegativo                            // regla 3: negativo en un parámetro que no los admite
	reglaFueraDeRango                        // regla 4: fuera del rango físico del parámetro
	reglaBloqueEnCero                        // un día entero en 0 exacto: equipo sin medir
	reglaDuplicadoUnidad                     // regla 5/6: misma lectura en otra unidad; se prefiere la canónica
	reglaDuplicadoExacto                     // regla 5: misma (estación, parámetro, fecha-hora) y mismo valor
	reglaDuplicadoValorDistinto              // regla 5: misma clave, otro valor; se conserva el primero
	numReglas
)

var nombresReglas = [numReglas]string{
	"columnas", "estacion", "periodo", "parametro", "fecha", "fecha_fuera_de_rango",
	"hora_no_en_punto", "hora", "valor_no_numerico", "centinela", "unidad", "negativo", "fuera_de_rango",
	"bloque_en_cero", "duplicado_unidad", "duplicado_exacto", "duplicado_valor_distinto",
}

// Reglas devuelve los nombres de las reglas de descarte en orden de cascada.
// Son las claves de descartadas_por_regla en reports/limpieza.json.
func Reglas() []string { return slices.Clone(nombresReglas[:]) }

// registro es una fila larga que llegó a tener clave de día: (estación,
// parámetro, fecha, unidad, periodo). Se guarda aunque una regla de valor la
// haya descartado, porque ocupa su posición en el día para la hora por posición.
type registro struct {
	t      int64   // instante de fecha_hora si la hora es explícita; medianoche del día si no
	valor  float64 // ya en la unidad canónica
	grupo  int32   // índice del día (en el parcial y luego en el acumulador)
	est    uint16  // estación (índice del bloque y luego global)
	param  uint8
	flags  uint8
	motivo uint8 // regla+1 que la descarta, anotada en el worker y contada en el recolector; 0 = ninguna
}

const (
	regExplicita   uint8 = 1 << iota // trae hora
	regCondicional                   // periodo horario solo si el día está completo
	regConvertida                    // viene de una unidad no canónica: cede ante la canónica
	regDescartada                    // ya contada en una regla; solo ocupa su posición en el día
	regInferida                      // hora reconstruida por posición (la pone el recolector)
)

// claveGrupo es el día de una serie: (estación, parámetro, fecha, unidad,
// periodo). Unidad y periodo son índices de texto (del bloque o globales).
type claveGrupo struct {
	dia     int64
	est     uint16
	unidad  uint16
	periodo uint16
	param   uint8
}

type grupoBloque struct {
	clave           claveGrupo
	n               int32
	primera, ultima int64 // número de registro (dentro del bloque) de su primera y última fila
}

// ubicacionVista es una combinación de columnas de ubicación y cuántas filas
// la traen. Una estación con más de una es sospechosa (spec: CA-VMP-2).
type ubicacionVista struct {
	Campos map[string]string `json:"campos"`
	Filas  int64             `json:"filas"`
}

type estacionBloque struct {
	nombre      string
	leidas      int64
	ubicaciones []ubicacionVista
	porClave    map[string]int // clave cruda de ubicación → índice en ubicaciones
	ultima      []byte         // clave cruda de la última ubicación vista
	actual      int
}

// entradaTexto cachea, por bloque, la decisión sobre un texto crudo (periodo
// o unidad) y cuenta cuántas veces apareció.
type entradaTexto struct {
	id     uint16 // índice en la tabla de textos del bloque
	nombre string // decodificado y recortado
	n      int64
	clase  uint8   // periodo: periodoDescartado, periodoHorario o periodoCondicional
	valida bool    // unidad aceptada para el parámetro
	factor float64 // unidad: a la canónica
}

// parcial es lo que un worker entrega por bloque. No comparte nada con otros
// workers: estaciones, textos y días son índices locales que el recolector
// traduce a globales.
type parcial struct {
	indice int
	err    error
	// colaAbierta es el offset donde empieza un registro que sigue abierto
	// (campo entrecomillado con salto de línea) al terminar el bloque, o -1.
	// El bloque siguiente empezó dentro de ese campo: el recolector lo vuelve
	// a procesar desde aquí.
	colaAbierta int64

	estaciones []estacionBloque
	unidades   []string // texto de cada índice de unidad
	periodos   []string // texto de cada índice de periodo
	grupos     []grupoBloque
	registros  []registro // en orden de archivo

	leidas       int64
	descartadas  [numReglas]int64
	horaTruncada int64

	vistosPeriodos    map[string]int64
	vistosParametros  map[string]int64 // columna → filas con valor
	vistosUnidades    [numParametros]map[string]int64
	vistosNoNumericos map[string]int64
}

const (
	fechaOK uint8 = iota
	fechaInvalida
	fechaFueraDeRango
)

type fechaParseada struct {
	dia    int64 // medianoche en la zona, segundos Unix
	estado uint8
}

const (
	horaOK uint8 = iota
	horaSin
	horaInvalida
	horaNoEnPunto
)

type horaParseada struct {
	seg    int64 // segundos desde la medianoche, ya en punto si se truncó
	estado uint8
	trunca bool // se truncó (política truncar)
}

// maxCache acota las cachés de fechas y horas de cada worker. Con lecturas
// horarias hay ~365 fechas por año, así que casi nunca se vacían.
const maxCache = 1 << 20

// trabajador es el estado de una goroutine del pool. El búfer y las cachés de
// fechas y horas son suyos: se reutilizan entre bloques y nadie más los toca.
// Las cachés son funciones puras de los bytes, así que no cambian resultados.
type trabajador struct {
	reg    *reglas
	cols   indicesColumnas
	delim  byte
	buf    []byte
	div    divisor
	fechas map[string]fechaParseada
	horas  map[string]horaParseada
	tmpV   []byte
	tmpU   []byte
}

func nuevoTrabajador(reg *reglas, cols indicesColumnas, delim byte) *trabajador {
	return &trabajador{reg: reg, cols: cols, delim: delim,
		fechas: map[string]fechaParseada{}, horas: map[string]horaParseada{}}
}

// estadoBloque son las cachés de un bloque: se crean al empezarlo y se
// vuelcan en el parcial al terminarlo.
type estadoBloque struct {
	p          *parcial
	estaciones map[string]uint16 // texto crudo → índice
	porNombre  map[string]uint16 // nombre normalizado → índice
	periodos   map[string]*entradaTexto
	unidades   [numParametros]map[string]*entradaTexto
	grupos     map[claveGrupo]int32
	ultimo     claveGrupo
	ultimoID   int32
}

// procesar lee el bloque con ReadAt (seguro para uso concurrente sobre el mismo
// *os.File), valida cada fila (validarFila) y devuelve el parcial.
func (w *trabajador) procesar(r io.ReaderAt, indice int, rg rango) *parcial {
	p := &parcial{
		indice:            indice,
		vistosPeriodos:    map[string]int64{},
		vistosParametros:  map[string]int64{},
		vistosNoNumericos: map[string]int64{},
	}
	n := int(rg.hasta - rg.desde)
	if cap(w.buf) < n {
		w.buf = make([]byte, n)
	}
	datos := w.buf[:n]
	if leidos, err := r.ReadAt(datos, rg.desde); leidos < n {
		p.err = fmt.Errorf("leer el bloque %d (bytes %d a %d): %w", indice, rg.desde, rg.hasta, err)
		return p
	}
	eb := &estadoBloque{
		p:          p,
		estaciones: map[string]uint16{},
		porNombre:  map[string]uint16{},
		periodos:   map[string]*entradaTexto{},
		grupos:     map[claveGrupo]int32{},
		ultimoID:   -1,
	}
	p.colaAbierta = -1
	for ini := 0; ini < n; {
		// Un registro termina en un salto de línea con una cantidad par de
		// comillas (RFC 4180: las comillas dentro de un campo van dobladas).
		// Si es impar, un campo entrecomillado sigue abierto: hay 100 así en el
		// CSV de OEFA (saltos de línea en DESCRIPCION_UBICACION) y se une la
		// línea física siguiente, con un tope para que un archivo roto no se
		// lleve el bloque entero.
		fin := siguienteFin(datos, ini)
		comillas := bytes.Count(datos[ini:fin], comilla)
		for extra := 0; comillas%2 == 1; extra++ {
			if fin >= n {
				p.colaAbierta = rg.desde + int64(ini)
				break
			}
			if extra == maxLineasPorRegistro {
				p.err = fmt.Errorf("bloque %d, registro que empieza en el byte %d: %w en %d líneas",
					indice, rg.desde+int64(ini), errComillaAbierta, maxLineasPorRegistro)
				return p
			}
			sig := siguienteFin(datos, fin)
			comillas += bytes.Count(datos[fin:sig], comilla)
			fin = sig
		}
		if p.colaAbierta >= 0 {
			break
		}
		linea := recortarFin(datos[ini:fin])
		inicioRegistro := rg.desde + int64(ini)
		ini = fin
		if len(linea) == 0 {
			continue // una línea vacía no es una fila
		}
		p.leidas++
		campos, err := w.div.dividir(linea, w.delim)
		if err != nil {
			p.err = fmt.Errorf("bloque %d, registro que empieza en el byte %d: %w", indice, inicioRegistro, err)
			return p
		}
		w.validarFila(eb, campos)
	}
	for _, e := range eb.periodos {
		p.vistosPeriodos[e.nombre] += e.n
	}
	for q, m := range eb.unidades {
		for _, e := range m {
			if p.vistosUnidades[q] == nil {
				p.vistosUnidades[q] = map[string]int64{}
			}
			p.vistosUnidades[q][e.nombre] += e.n
		}
	}
	return p
}

// maxLineasPorRegistro acota cuántas líneas físicas puede ocupar un registro.
const maxLineasPorRegistro = 64

var comilla = []byte{'"'}

// siguienteFin devuelve el índice siguiente al '\n' que termina la línea que
// empieza en ini (o el fin del búfer).
func siguienteFin(datos []byte, ini int) int {
	if i := bytes.IndexByte(datos[ini:], '\n'); i >= 0 {
		return ini + i + 1
	}
	return len(datos)
}

// recortarFin quita el '\n' y el '\r' finales de una línea (CRLF del CSV de OEFA).
func recortarFin(linea []byte) []byte {
	linea = bytes.TrimSuffix(linea, []byte{'\n'})
	return bytes.TrimSuffix(linea, []byte{'\r'})
}

// validarFila aplica la cascada de reglas que se deciden con la fila sola. Si
// la fila llega a tener clave de día, agrega un registro (descartado o no:
// ocupa su posición en el día); si no, solo cuenta la regla.
func (w *trabajador) validarFila(eb *estadoBloque, campos [][]byte) {
	p, reg, cols := eb.p, w.reg, w.cols
	if len(campos) != cols.total {
		p.descartadas[reglaColumnas]++
		return
	}
	crudo := bytes.TrimSpace(campos[cols.estacion])
	if len(crudo) == 0 || reg.nulos[string(crudo)] {
		p.descartadas[reglaEstacion]++
		return
	}
	ei, ok := eb.estaciones[string(crudo)]
	if !ok {
		ei = w.registrarEstacion(eb, crudo)
		eb.estaciones[string(crudo)] = ei
	}
	est := &p.estaciones[ei]
	est.leidas++
	w.verUbicacion(est, campos)

	// Parámetro: la única columna de parámetro con valor.
	pi, nvals := -1, 0
	var valor []byte
	for _, c := range cols.params {
		v := bytes.TrimSpace(campos[c.indice])
		if reg.nulos[string(v)] {
			continue
		}
		nvals++
		if nvals == 1 {
			pi, valor = c.param, v
			p.vistosParametros[c.nombre]++
		}
	}

	// Periodo (regla 1) antes que parámetro. Un periodo «por grupo» («-») es
	// horario con hora explícita o con su día completo; una fila sin parámetro
	// no tiene día, así que sin hora explícita cae aquí y no en parametro.
	var flags uint8
	var per *entradaTexto
	if cols.periodo >= 0 {
		per = w.buscarTexto(eb.periodos, campos[cols.periodo], &p.periodos, func(e *entradaTexto) {
			e.clase = reg.periodos[normalizar(e.nombre)]
		})
		per.n++
		switch per.clase {
		case periodoDescartado:
			p.descartadas[reglaPeriodo]++
			return
		case periodoCondicional:
			flags |= regCondicional
			if nvals != 1 && (cols.hora < 0 || w.hora(campos[cols.hora]).estado != horaOK) {
				p.descartadas[reglaPeriodo]++
				return
			}
		}
	}
	if nvals != 1 {
		p.descartadas[reglaParametro]++
		return
	}

	f := w.fecha(campos[cols.fecha])
	switch f.estado {
	case fechaInvalida:
		p.descartadas[reglaFecha]++
		return
	case fechaFueraDeRango:
		p.descartadas[reglaFechaFueraDeRango]++
		return
	}
	t := f.dia
	var motivo regla = -1
	if cols.hora >= 0 {
		h := w.hora(campos[cols.hora])
		switch h.estado {
		case horaInvalida:
			motivo = reglaHora
		case horaNoEnPunto:
			motivo = reglaHoraNoEnPunto
		case horaOK:
			if h.trunca {
				p.horaTruncada++
			}
			flags |= regExplicita
			t += h.seg + w.ajusteExplicita()
		}
	} else {
		flags |= regExplicita // sin columna de hora, la fecha es el instante
	}

	// Unidad: se busca antes de las reglas de valor para que «vistos» cuente
	// todas las filas con parámetro.
	if eb.unidades[pi] == nil {
		eb.unidades[pi] = map[string]*entradaTexto{}
	}
	var crudoUnidad []byte
	if cols.unidad >= 0 {
		crudoUnidad = campos[cols.unidad]
	}
	u := w.buscarTexto(eb.unidades[pi], crudoUnidad, &p.unidades, func(e *entradaTexto) {
		e.valida, e.factor = true, 1
		if tabla := reg.unidades[pi]; tabla != nil && cols.unidad >= 0 {
			e.factor, e.valida = tabla[normalizar(e.nombre)]
		}
	})
	u.n++

	// La fila ya tiene día: entra a su grupo aunque una regla la descarte,
	// porque ocupa una posición. La regla se anota y se cuenta en el
	// recolector, en el orden de la cascada (la hora va antes que el valor).
	r := registro{t: t, est: ei, param: uint8(pi), flags: flags}
	r.grupo = eb.grupo(claveGrupo{dia: f.dia, est: ei, unidad: u.id, periodo: idTexto(per), param: uint8(pi)}, p.leidas-1)
	if m, ok := w.reglasDeValor(p, &r, valor, u, pi); !ok && motivo < 0 {
		motivo = m
	}
	if motivo >= 0 {
		r.motivo = uint8(motivo) + 1
	}
	p.registros = append(p.registros, r)
}

// reglasDeValor aplica las reglas 3, 4 y 6 y deja el valor en la unidad canónica.
// No cuenta nada: devuelve la regla que falla para que el recolector la cuente
// en su lugar de la cascada.
func (w *trabajador) reglasDeValor(p *parcial, r *registro, crudo []byte, u *entradaTexto, pi int) (regla, bool) {
	reg := w.reg
	v, ok := w.valor(crudo)
	switch {
	case !ok:
		verNoNumerico(p, decodificar(crudo, reg.cod))
		return reglaValorNoNumerico, false
	case slices.Contains(reg.centinelas, v):
		return reglaCentinela, false
	case !u.valida:
		return reglaUnidad, false
	case v < 0 && !reg.negativos[pi]:
		return reglaNegativo, false
	}
	if u.factor != 1 {
		v = float64(v * u.factor)
		r.flags |= regConvertida
	}
	if v == 0 {
		v = 0 // -0 se escribiría "-0" en el silver
	}
	if reg.conRango[pi] && (v < reg.rango[pi][0] || v > reg.rango[pi][1]) {
		return reglaFueraDeRango, false
	}
	r.valor = v
	return 0, true
}

// ajusteExplicita lleva una hora explícita al instante que se escribe en
// fecha_hora: si la hora marca el inicio del intervalo y fecha_hora es su fin,
// se suma una hora; al revés, se resta.
func (w *trabajador) ajusteExplicita() int64 {
	ajuste := int64(0)
	if !w.reg.horaInicio {
		ajuste -= 3600 // la hora es el fin: el intervalo empezó una hora antes
	}
	if w.reg.convFin {
		ajuste += 3600
	}
	return ajuste
}

func idTexto(e *entradaTexto) uint16 {
	if e == nil {
		return 0
	}
	return e.id
}

// grupo devuelve el índice del día en el bloque y anota el número de registro
// de la fila (para saber después si el día es contiguo). Las filas de un día
// vienen seguidas, así que casi siempre acierta la comparación con el último.
func (eb *estadoBloque) grupo(k claveGrupo, num int64) int32 {
	id := eb.ultimoID
	if id < 0 || k != eb.ultimo {
		var ok bool
		id, ok = eb.grupos[k]
		if !ok {
			id = int32(len(eb.p.grupos))
			eb.grupos[k] = id
			eb.p.grupos = append(eb.p.grupos, grupoBloque{clave: k, primera: num})
		}
		eb.ultimo, eb.ultimoID = k, id
	}
	g := &eb.p.grupos[id]
	g.n++
	g.ultima = num
	return id
}

// buscarTexto devuelve la entrada cacheada para un texto crudo; la primera vez
// lo decodifica, le da un índice en tabla y deja que decidir la complete. La
// búsqueda con string(crudo) no asigna memoria.
func (w *trabajador) buscarTexto(cache map[string]*entradaTexto, crudo []byte, tabla *[]string, decidir func(*entradaTexto)) *entradaTexto {
	crudo = bytes.TrimSpace(crudo)
	if e, ok := cache[string(crudo)]; ok {
		return e
	}
	e := &entradaTexto{nombre: strings.TrimSpace(decodificar(crudo, w.reg.cod)), id: uint16(len(*tabla))}
	*tabla = append(*tabla, e.nombre)
	decidir(e)
	cache[string(crudo)] = e
	return e
}

// nombreEstacion normaliza el nombre de una estación: espacios colapsados y
// mayúsculas, para que «ca-hua-01 » y «CA-HUA-01» sean la misma.
func nombreEstacion(s string) string {
	return strings.ToUpper(strings.Join(strings.Fields(s), " "))
}

func (w *trabajador) registrarEstacion(eb *estadoBloque, crudo []byte) uint16 {
	nombre := nombreEstacion(decodificar(crudo, w.reg.cod))
	if i, ok := eb.porNombre[nombre]; ok {
		return i
	}
	i := uint16(len(eb.p.estaciones))
	eb.p.estaciones = append(eb.p.estaciones, estacionBloque{nombre: nombre, porClave: map[string]int{}, actual: -1})
	eb.porNombre[nombre] = i
	return i
}

// verUbicacion cuenta la combinación de columnas de ubicación de la fila. Casi
// siempre es la misma que la anterior de la estación: se compara con ella
// byte a byte y solo se busca en el mapa si cambió.
func (w *trabajador) verUbicacion(est *estacionBloque, campos [][]byte) {
	if len(w.cols.ubicacion) == 0 {
		return
	}
	w.tmpU = w.tmpU[:0]
	for _, u := range w.cols.ubicacion {
		w.tmpU = append(append(w.tmpU, bytes.TrimSpace(campos[u.indice])...), 0x1f)
	}
	if est.actual >= 0 && bytes.Equal(w.tmpU, est.ultima) {
		est.ubicaciones[est.actual].Filas++
		return
	}
	i, ok := est.porClave[string(w.tmpU)]
	if !ok {
		vista := ubicacionVista{Campos: map[string]string{}}
		for _, u := range w.cols.ubicacion {
			v := strings.TrimSpace(decodificar(bytes.TrimSpace(campos[u.indice]), w.reg.cod))
			if w.reg.nulos[v] {
				continue
			}
			if u.nombre == "ubigeo" {
				v = rellenarUbigeo(v)
			}
			vista.Campos[u.nombre] = v
		}
		i = len(est.ubicaciones)
		est.ubicaciones = append(est.ubicaciones, vista)
		est.porClave[string(w.tmpU)] = i
	}
	est.ultima = append(est.ultima[:0], w.tmpU...)
	est.actual = i
	est.ubicaciones[i].Filas++
}

// rellenarUbigeo devuelve el UBIGEO como texto de 6 dígitos: el CSV lo trae
// como número y pierde el cero inicial (21801 es 021801, Áncash).
func rellenarUbigeo(v string) string {
	if len(v) >= 6 || strings.Trim(v, "0123456789") != "" {
		return v
	}
	return strings.Repeat("0", 6-len(v)) + v
}

// maxNoNumericos acota cuántos textos distintos se inventarían por bloque; el
// resto se cuenta junto. Son raros (<LD, ND, ---), pero un archivo roto podría
// traer millones distintos.
const maxNoNumericos = 1000

// verNoNumerico inventaría un texto que estaba en una columna de parámetro y
// no es un número. Política: no se imputa (un <LD no es cero); se descarta por
// valor_no_numerico y se cuenta aquí para decidir con el EDA.
func verNoNumerico(p *parcial, texto string) {
	if _, ok := p.vistosNoNumericos[texto]; !ok && len(p.vistosNoNumericos) >= maxNoNumericos {
		texto = "(otros)"
	}
	p.vistosNoNumericos[texto]++
}

// fecha parsea la fecha del día (medianoche en la zona de la config).
func (w *trabajador) fecha(crudo []byte) fechaParseada {
	crudo = bytes.TrimSpace(crudo)
	if f, ok := w.fechas[string(crudo)]; ok {
		return f
	}
	s := string(crudo)
	f := fechaParseada{estado: fechaInvalida}
	for _, formato := range w.reg.formatosFecha {
		t, err := time.ParseInLocation(formato, s, w.reg.zona)
		if err != nil {
			continue
		}
		f = fechaParseada{dia: t.Unix(), estado: fechaOK}
		if f.dia < w.reg.desde || f.dia >= w.reg.hasta {
			f.estado = fechaFueraDeRango
		}
		break
	}
	if len(w.fechas) >= maxCache {
		clear(w.fechas)
	}
	w.fechas[s] = f
	return f
}

// hora parsea HORA_INICIO: vacío/NA es «sin hora»; se quita el prefijo de Excel
// (1899-12-31) y, si no queda nada, es la medianoche o «sin hora» según la config.
func (w *trabajador) hora(crudo []byte) horaParseada {
	crudo = bytes.TrimSpace(crudo)
	if h, ok := w.horas[string(crudo)]; ok {
		return h
	}
	s := string(crudo)
	h := w.parsearHora(s)
	if len(w.horas) >= maxCache {
		clear(w.horas)
	}
	w.horas[s] = h
	return h
}

func (w *trabajador) parsearHora(s string) horaParseada {
	reg := w.reg
	if reg.nulos[s] {
		return horaParseada{estado: horaSin}
	}
	resto := s
	if reg.horaPrefijo != "" && strings.HasPrefix(s, reg.horaPrefijo) {
		resto = strings.TrimSpace(s[len(reg.horaPrefijo):])
		if resto == "" {
			if reg.soloPrefijoEs == 1 {
				return horaParseada{estado: horaSin}
			}
			return horaParseada{seg: 0, estado: horaOK}
		}
	}
	for _, formato := range reg.formatosHora {
		t, err := time.Parse(formato, resto)
		if err != nil {
			continue
		}
		seg := int64(t.Hour()*3600 + t.Minute()*60 + t.Second())
		if seg%3600 != 0 {
			if !reg.horaTruncar {
				return horaParseada{estado: horaNoEnPunto}
			}
			return horaParseada{seg: seg - seg%3600, estado: horaOK, trunca: true}
		}
		return horaParseada{seg: seg, estado: horaOK}
	}
	return horaParseada{estado: horaInvalida}
}

// valor parsea un número; texto, NaN e infinito no son lecturas.
func (w *trabajador) valor(crudo []byte) (float64, bool) {
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
