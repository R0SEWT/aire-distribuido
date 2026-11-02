package datos

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"math/bits"
	"slices"
	"strings"
	"time"
)

type estacionGlobal struct {
	nombre      string
	leidas      int64
	ubicaciones []ubicacionVista
	porClave    map[string]int
}

// acumulador es el estado del recolector. Solo lo toca la goroutine que corre
// leerBloques: los workers le pasan sus parciales por un canal. Guarda todos
// los registros en orden de archivo, porque la hora por posición solo se puede
// decidir cuando se conoce el día completo, y un día puede cruzar bloques.
type acumulador struct {
	porNombre    map[string]uint16
	estaciones   []*estacionGlobal
	porUnidad    map[string]uint16
	porPeriodo   map[string]uint16
	grupos       map[claveGrupo]int32
	tamGrupo     []int32
	primera      []int64 // número de registro (en el archivo) de la primera fila de cada día
	ultima       []int64 // y de la última: el día es contiguo si ultima-primera+1 == tamGrupo
	tamTramo     []int32 // filas de cada bloque físico, con o sin valor
	paramTramo   []int8  // parámetro de sus filas con valor (tramoSinParametro, tramoMixto)
	tramoAbierto int32   // bloque físico de la última fila del bloque anterior, o -1
	claveAbierta string  // y su clave: el bloque siguiente lo continúa si empieza con la misma
	registros    []registro
	desbordadas  bool // más estaciones o textos de los que caben en un uint16
	leidas       int64
	descartadas  [numReglas]int64
	fueraDeRango [numParametros]int64
	enCero       [numParametros]int64
	ceroFaltante [numParametros]int64
	horaTruncada int64
	horaFisica   int64 // filas con la hora de su posición en el bloque físico (día de parámetro incompleto)
	periodos     map[string]int64
	parametros   map[string]int64
	unidades     [numParametros]map[string]int64
	noNumericos  map[string]int64
	bloques      int
	tFusion      time.Duration
}

func nuevoAcumulador() *acumulador {
	return &acumulador{
		porNombre:    map[string]uint16{},
		porUnidad:    map[string]uint16{},
		porPeriodo:   map[string]uint16{},
		grupos:       map[claveGrupo]int32{},
		tramoAbierto: -1,
		periodos:     map[string]int64{},
		parametros:   map[string]int64{},
		noNumericos:  map[string]int64{},
	}
}

// indiceGlobal devuelve el índice de un texto en una tabla global, creándolo.
func (a *acumulador) indiceGlobal(m map[string]uint16, texto string) uint16 {
	if i, ok := m[texto]; ok {
		return i
	}
	if len(m) > math.MaxUint16 {
		a.desbordadas = true
		return 0
	}
	i := uint16(len(m))
	m[texto] = i
	return i
}

// fusionar incorpora un parcial: traduce sus índices locales (estaciones,
// textos, días) a globales y agrega sus registros al final, que es su lugar en
// el orden del archivo porque los parciales llegan en orden de bloque.
func (a *acumulador) fusionar(p *parcial) {
	estaciones := make([]uint16, len(p.estaciones))
	for i, e := range p.estaciones {
		g, ok := a.porNombre[e.nombre]
		if !ok {
			if len(a.estaciones) > math.MaxUint16 {
				a.desbordadas = true
			}
			g = uint16(len(a.estaciones))
			a.porNombre[e.nombre] = g
			a.estaciones = append(a.estaciones, &estacionGlobal{nombre: e.nombre, porClave: map[string]int{}})
		}
		eg := a.estaciones[g]
		eg.leidas += e.leidas
		for _, u := range e.ubicaciones {
			clave := claveUbicacion(u.Campos) + "\x1e" + claveUbicacion(u.Original)
			j, ok := eg.porClave[clave]
			if !ok {
				j = len(eg.ubicaciones)
				eg.porClave[clave] = j
				eg.ubicaciones = append(eg.ubicaciones, ubicacionVista{Campos: u.Campos, Original: u.Original})
			}
			eg.ubicaciones[j].Filas += u.Filas
		}
		estaciones[i] = g
	}
	unidades := make([]uint16, len(p.unidades))
	for i, t := range p.unidades {
		unidades[i] = a.indiceGlobal(a.porUnidad, t)
	}
	periodos := make([]uint16, max(len(p.periodos), 1)) // sin columna de periodo, todo es el índice 0
	for i, t := range p.periodos {
		periodos[i] = a.indiceGlobal(a.porPeriodo, t)
	}
	// Los números de registro del bloque se vuelven del archivo sumando los
	// registros de los bloques anteriores, que ya se fusionaron (en orden).
	base := a.leidas
	grupos := make([]int32, len(p.grupos))
	for i, g := range p.grupos {
		k := g.clave
		k.est, k.unidad, k.periodo = estaciones[k.est], unidades[k.unidad], periodos[k.periodo]
		id, ok := a.grupos[k]
		if !ok {
			id = int32(len(a.tamGrupo))
			a.grupos[k] = id
			a.tamGrupo = append(a.tamGrupo, 0)
			a.primera = append(a.primera, base+g.primera)
			a.ultima = append(a.ultima, 0)
		}
		a.tamGrupo[id] += g.n
		a.ultima[id] = base + g.ultima
		grupos[i] = id
	}
	// Bloques físicos: todos son nuevos salvo el primero, que sigue al último
	// del bloque anterior si este terminó dentro de él y la clave coincide.
	// Entonces sus filas se corren las que ese bloque físico ya tenía.
	tramos := make([]int32, len(p.tramos))
	corrimiento := int32(0)
	for i, t := range p.tramos {
		if i == 0 && p.claveInicial != "" && a.tramoAbierto >= 0 && p.claveInicial == a.claveAbierta {
			g := a.tramoAbierto
			corrimiento = a.tamTramo[g]
			a.tamTramo[g] += t.n
			a.paramTramo[g] = combinarParametro(a.paramTramo[g], t.param)
			tramos[0] = g
			continue
		}
		tramos[i] = int32(len(a.tamTramo))
		a.tamTramo = append(a.tamTramo, t.n)
		a.paramTramo = append(a.paramTramo, t.param)
	}
	if p.leidas > 0 { // un bloque sin filas no corta el bloque físico
		a.tramoAbierto, a.claveAbierta = -1, ""
		if p.claveFinal != "" {
			a.tramoAbierto, a.claveAbierta = tramos[len(tramos)-1], p.claveFinal
		}
	}
	desde := len(a.registros)
	a.registros = append(a.registros, p.registros...)
	for i := desde; i < len(a.registros); i++ {
		r := &a.registros[i]
		if r.tramo == 0 && corrimiento > 0 {
			r.posTramo = uint8(min(int32(r.posTramo)+corrimiento, math.MaxUint8))
		}
		r.est, r.grupo, r.tramo = estaciones[r.est], grupos[r.grupo], tramos[r.tramo]
	}

	a.leidas += p.leidas
	a.horaTruncada += p.horaTruncada
	for r, n := range p.descartadas {
		a.descartadas[r] += n
	}
	for k, n := range p.vistosPeriodos {
		a.periodos[k] += n
	}
	for k, n := range p.vistosParametros {
		a.parametros[k] += n
	}
	for k, n := range p.vistosNoNumericos {
		a.noNumericos[k] += n
	}
	for q, m := range p.vistosUnidades {
		for k, n := range m {
			if a.unidades[q] == nil {
				a.unidades[q] = map[string]int64{}
			}
			a.unidades[q][k] += n
		}
	}
	a.bloques++
}

// claveUbicacion identifica una combinación de columnas de ubicación.
func claveUbicacion(campos map[string]string) string {
	claves := make([]string, 0, len(campos))
	for k := range campos {
		claves = append(claves, k)
	}
	slices.Sort(claves)
	var b strings.Builder
	for _, k := range claves {
		b.WriteString(k + "=" + campos[k] + "\x1f")
	}
	return b.String()
}

// resolverHoras recorre los registros en orden de archivo, que es el orden
// dentro de cada día, y aplica la parte de la cascada que necesita el día
// completo. Un día (estación, parámetro, fecha, unidad, periodo) está completo
// si tiene exactamente filas_por_dia filas contiguas en el archivo, o si su
// bloque físico las tiene: filas_por_dia filas contiguas de la misma (estación,
// fecha, unidad, periodo, tipo de análisis), de un solo parámetro, contando las
// vacías (en la plantilla de 2022 un hueco es una fila vacía que ocupa su
// hora). Para cada fila, la primera regla que falla:
//
//   - periodo: periodo «-» (por grupo) sin hora explícita y con el día incompleto;
//   - hora_no_en_punto u hora ilegible, anotadas por el worker;
//   - hora: sin hora y con el día incompleto (o sin hora por posición);
//   - las reglas de valor anotadas por el worker (3, 4 y 6).
//
// Las que pasan reciben su hora: la explícita ya está en t; la que falta es la
// posición en el día (0 a 23, hora de inicio; en el día del parámetro si está
// completo y si no en el bloque físico) y fecha_hora es el fin (o el inicio) de
// ese intervalo. Las filas descartadas siguen ocupando su posición: son una
// hora del día con una lectura inválida, no una hora que falta.
//
// Después, bloque_en_cero: si todas las lecturas válidas de un día de un
// parámetro de bloques_en_cero_como_faltante son 0 exacto, el día es un equipo
// sin medir y se descarta entero. Y cero_como_faltante: en los días que quedan,
// un 0 exacto de un parámetro de esa lista es una hora sin dato.
func (a *acumulador) resolverHoras(reg *reglas) {
	pos := make([]int32, len(a.tamGrupo))
	validas := make([]int32, len(a.tamGrupo))
	ceros := make([]int32, len(a.tamGrupo))
	sello := int64(0)
	if reg.convFin {
		sello = 1
	}
	for i := range a.registros {
		r := &a.registros[i]
		g := r.grupo
		p := pos[g]
		pos[g]++
		diaCompleto := reg.filasPorDia > 0 && a.tamGrupo[g] == int32(reg.filasPorDia) &&
			a.ultima[g]-a.primera[g]+1 == int64(reg.filasPorDia)
		fisicoCompleto := !diaCompleto && reg.filasPorDia > 0 && a.tamTramo[r.tramo] == int32(reg.filasPorDia) &&
			a.paramTramo[r.tramo] == int8(r.param)
		completo := diaCompleto || fisicoCompleto
		explicita := r.flags&regExplicita != 0
		var motivo regla = -1
		switch {
		case r.flags&regCondicional != 0 && !explicita && !completo:
			motivo = reglaPeriodo
		case r.motivo != 0 && (regla(r.motivo-1) == reglaHora || regla(r.motivo-1) == reglaHoraNoEnPunto):
			motivo = regla(r.motivo - 1)
		case !explicita && (!reg.posicion || !completo):
			motivo = reglaHora
		case r.motivo != 0:
			motivo = regla(r.motivo - 1)
		}
		if motivo >= 0 {
			a.descartadas[motivo]++
			if motivo == reglaFueraDeRango {
				a.fueraDeRango[r.param]++
			}
			r.flags |= regDescartada
			continue
		}
		if !explicita {
			if fisicoCompleto {
				p = int32(r.posTramo)
				a.horaFisica++
			}
			r.t += (int64(p) + sello) * 3600
			r.flags |= regInferida
		}
		if reg.enCero[r.param] {
			validas[g]++
			if r.valor == 0 {
				ceros[g]++
			}
		}
	}
	for i := range a.registros {
		r := &a.registros[i]
		if r.flags&regDescartada != 0 {
			continue
		}
		switch {
		case reg.enCero[r.param] && ceros[r.grupo] == validas[r.grupo]:
			a.descartadas[reglaBloqueEnCero]++
			a.enCero[r.param]++
		case reg.ceroFaltante[r.param] && r.valor == 0:
			a.descartadas[reglaCeroComoFaltante]++
			a.ceroFaltante[r.param]++
		default:
			continue
		}
		r.flags |= regDescartada
	}
}

var filaVacia = func() (f [numParametros]float64) {
	for i := range f {
		f[i] = math.NaN()
	}
	return f
}()

// pivoteEstacion es la parte de la tabla ancha de una estación, ya ordenada
// por fecha_hora, con sus conteos de duplicados.
type pivoteEstacion struct {
	horas      []int64
	valores    []float64 // filas × numParametros
	inferida   []uint16  // por fila: bits de celdas con hora reconstruida
	convertida []uint16  // por fila: bits de celdas convertidas desde otra unidad
	conflicto  []uint16  // por fila: bits de celdas con un duplicado de otro valor
	dup        [3]int64  // duplicado_unidad, duplicado_exacto, duplicado_valor_distinto
}

// pivotar arma la tabla ancha por estación, en paralelo (regla 7). Las celdas
// de dos estaciones nunca chocan, así que cada estación es una tarea
// independiente; dentro de ella los registros se aplican en orden de archivo
// y el resultado no depende de W. Las estaciones grandes se reparten primero.
func (a *acumulador) pivotar(workers int) []*pivoteEstacion {
	inicio := make([]int, len(a.estaciones)+1)
	for _, r := range a.registros {
		if r.flags&regDescartada == 0 {
			inicio[r.est+1]++
		}
	}
	for e := range a.estaciones {
		inicio[e+1] += inicio[e]
	}
	idx := make([]int32, inicio[len(a.estaciones)])
	llenar := slices.Clone(inicio)
	for i, r := range a.registros {
		if r.flags&regDescartada == 0 {
			idx[llenar[r.est]] = int32(i)
			llenar[r.est]++
		}
	}
	orden := make([]int, len(a.estaciones))
	for e := range orden {
		orden[e] = e
	}
	slices.SortStableFunc(orden, func(x, y int) int {
		return cmp.Compare(inicio[y+1]-inicio[y], inicio[x+1]-inicio[x])
	})
	out := make([]*pivoteEstacion, len(a.estaciones))
	enParalelo(workers, len(orden), func(k int) {
		e := orden[k]
		out[e] = pivotarEstacion(a.registros, idx[inicio[e]:inicio[e+1]])
	})
	return out
}

// pivotarEstacion aplica las reglas de duplicado de una estación. Una celda
// ocupada se queda con su valor, salvo que llegue la misma lectura en la unidad
// canónica (el valor convertido cede: duplicado_unidad). Entre lecturas de la
// misma prioridad gana la primera del archivo: exacto si el valor es igual,
// valor_distinto (y se marca la celda) si no.
func pivotarEstacion(regs []registro, idx []int32) *pivoteEstacion {
	pe := &pivoteEstacion{}
	filas := map[int64]int32{}
	var prio []uint8
	poner := func(c, f int, bit uint16, r *registro, pr uint8) {
		pe.valores[c], prio[c] = r.valor, pr
		pe.inferida[f] &^= bit
		pe.convertida[f] &^= bit
		if r.flags&regInferida != 0 {
			pe.inferida[f] |= bit
		}
		if r.flags&regConvertida != 0 {
			pe.convertida[f] |= bit
		}
	}
	for _, ri := range idx {
		r := &regs[ri]
		f, ok := filas[r.t]
		if !ok {
			f = int32(len(pe.horas))
			filas[r.t] = f
			pe.horas = append(pe.horas, r.t)
			pe.valores = append(pe.valores, filaVacia[:]...)
			prio = append(prio, make([]uint8, numParametros)...)
			pe.inferida = append(pe.inferida, 0)
			pe.convertida = append(pe.convertida, 0)
			pe.conflicto = append(pe.conflicto, 0)
		}
		c, bit := int(f)*numParametros+int(r.param), uint16(1)<<r.param
		var pr uint8
		if r.flags&regConvertida != 0 {
			pr = 1
		}
		switch {
		case math.IsNaN(pe.valores[c]):
			poner(c, int(f), bit, r, pr)
		case pr < prio[c]:
			pe.dup[0]++
			poner(c, int(f), bit, r, pr)
			pe.conflicto[f] &^= bit
		case pr > prio[c]:
			pe.dup[0]++
		case pe.valores[c] == r.valor:
			pe.dup[1]++
		default:
			pe.dup[2]++
			pe.conflicto[f] |= bit
		}
	}
	// Ordenar por fecha_hora: las claves son únicas, así que el orden es total.
	orden := make([]int32, len(pe.horas))
	for i := range orden {
		orden[i] = int32(i)
	}
	slices.SortFunc(orden, func(x, y int32) int { return cmp.Compare(pe.horas[x], pe.horas[y]) })
	ord := &pivoteEstacion{dup: pe.dup,
		horas: make([]int64, len(orden)), valores: make([]float64, len(pe.valores)),
		inferida: make([]uint16, len(orden)), convertida: make([]uint16, len(orden)), conflicto: make([]uint16, len(orden))}
	for i, o := range orden {
		ord.horas[i] = pe.horas[o]
		copy(ord.valores[i*numParametros:(i+1)*numParametros], pe.valores[int(o)*numParametros:(int(o)+1)*numParametros])
		ord.inferida[i], ord.convertida[i], ord.conflicto[i] = pe.inferida[o], pe.convertida[o], pe.conflicto[o]
	}
	return ord
}

// TablaAncha es el silver en memoria: una fila por (estación, fecha-hora),
// ordenada por estación y luego por fecha-hora, con los parámetros canónicos
// presentes en orden alfabético. NaN es «sin lectura».
type TablaAncha struct {
	Parametros   []string       // canónicos con al menos un valor, en orden alfabético
	Estaciones   []string       // estaciones con al menos una fila, en orden alfabético
	Estacion     []uint16       // por fila, índice en Estaciones
	Hora         []int64        // por fila, segundos Unix del instante de fecha_hora
	Valores      []float64      // N × len(Parametros), por filas
	HoraInferida []bool         // por fila: alguna celda tiene hora reconstruida por posición
	N            int            // filas
	Zona         *time.Location // zona en que se escribe fecha_hora
}

// estadoTabla son los conteos que salen al armar la tabla.
type estadoTabla struct {
	procesadas            int64 // celdas con valor: filas largas que llegaron al silver
	inferidas             int64 // de ellas, con hora reconstruida
	convertidas           [numParametros]int64
	celdasEnConflicto     int64
	pm25MayorPm10         int
	filasInferidas        int
	procesadasPorEstacion map[string]int64
}

// tabla concatena los pivotes por estación en orden alfabético y deja solo
// los parámetros presentes. También suma los duplicados al acumulador.
func (a *acumulador) tabla(pivotes []*pivoteEstacion, zona *time.Location) (*TablaAncha, estadoTabla, error) {
	st := estadoTabla{procesadasPorEstacion: map[string]int64{}}
	if a.desbordadas {
		return nil, st, errors.New("más estaciones, unidades o periodos distintos de los que caben en un uint16")
	}
	var conFilas []int
	var presentes [numParametros]bool
	for e, pe := range pivotes {
		a.descartadas[reglaDuplicadoUnidad] += pe.dup[0]
		a.descartadas[reglaDuplicadoExacto] += pe.dup[1]
		a.descartadas[reglaDuplicadoValorDistinto] += pe.dup[2]
		if len(pe.horas) > 0 {
			conFilas = append(conFilas, e)
		}
		for i, v := range pe.valores {
			if !math.IsNaN(v) {
				presentes[i%numParametros] = true
			}
		}
	}
	slices.SortFunc(conFilas, func(x, y int) int { return strings.Compare(a.estaciones[x].nombre, a.estaciones[y].nombre) })
	var cols []int
	t := &TablaAncha{Zona: zona}
	for q, ok := range presentes {
		if ok {
			cols = append(cols, q)
			t.Parametros = append(t.Parametros, ParametrosCanonicos[q])
		}
	}
	for _, e := range conFilas {
		t.N += len(pivotes[e].horas)
	}
	t.Estacion = make([]uint16, 0, t.N)
	t.Hora = make([]int64, 0, t.N)
	t.HoraInferida = make([]bool, 0, t.N)
	t.Valores = make([]float64, 0, t.N*len(cols))
	for k, e := range conFilas {
		pe, nombre := pivotes[e], a.estaciones[e].nombre
		t.Estaciones = append(t.Estaciones, nombre)
		for f := range pe.horas {
			fila := pe.valores[f*numParametros : (f+1)*numParametros]
			for _, q := range cols {
				t.Valores = append(t.Valores, fila[q])
			}
			t.Estacion = append(t.Estacion, uint16(k))
			t.Hora = append(t.Hora, pe.horas[f])
			t.HoraInferida = append(t.HoraInferida, pe.inferida[f] != 0)
			llenas := 0
			for q, v := range fila {
				if !math.IsNaN(v) {
					llenas++
					if pe.convertida[f]&(1<<q) != 0 {
						st.convertidas[q]++
					}
				}
			}
			st.procesadas += int64(llenas)
			st.procesadasPorEstacion[nombre] += int64(llenas)
			st.inferidas += int64(bits.OnesCount16(pe.inferida[f]))
			st.celdasEnConflicto += int64(bits.OnesCount16(pe.conflicto[f]))
			if pe.inferida[f] != 0 {
				st.filasInferidas++
			}
			if pm25, pm10 := fila[idxPM25], fila[idxPM10]; pm25 > pm10 { // NaN compara falso
				st.pm25MayorPm10++
			}
		}
	}
	if len(t.Estaciones) > math.MaxUint16+1 {
		return nil, st, fmt.Errorf("%d estaciones no caben en un uint16", len(t.Estaciones))
	}
	return t, st, nil
}
