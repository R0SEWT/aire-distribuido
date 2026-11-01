package datos

import (
	"cmp"
	"fmt"
	"io"
	"math"
	"math/bits"
	"slices"
	"sync"
	"time"
)

// claveGlobal es (estación, fecha-hora) con la estación como índice global.
type claveGlobal struct {
	est int32
	t   int64
}

type estacionGlobal struct {
	nombre             string
	ubicacion          map[string]string
	leidas, procesadas int64
}

// acumulador es el estado del recolector. Solo lo toca la goroutine que llama
// a leerBloques: los workers le pasan sus parciales por un canal.
type acumulador struct {
	porNombre  map[string]int32
	estaciones []*estacionGlobal
	filas      map[claveGlobal]int32
	claves     []claveGlobal
	valores    []float64
	conflicto  []uint16

	leidas, procesadas int64
	descartadas        [numReglas]int64
	fueraDeRango       [numParametros]int64
	periodos           map[string]int64
	parametros         map[string]int64
	noMapeados         map[string]int64
	unidades           [numParametros]map[string]int64
	bloques            int
}

func nuevoAcumulador() *acumulador {
	return &acumulador{
		porNombre:  map[string]int32{},
		filas:      map[claveGlobal]int32{},
		periodos:   map[string]int64{},
		parametros: map[string]int64{},
		noMapeados: map[string]int64{},
	}
}

// leerBloques es el pipeline de la parte A: un productor reparte los índices de
// bloque por un canal, W workers los procesan con su propio estado, y esta
// goroutine recolecta. Los parciales llegan en cualquier orden; el recolector
// los guarda hasta que llega el siguiente en orden de índice y los fusiona así,
// de modo que «el primero» de un duplicado es el primero del archivo y el
// resultado no depende de W ni del planificador.
func leerBloques(r io.ReaderAt, rangos []rango, reg *reglas, cols indicesColumnas, delim byte, workers int) (*acumulador, error) {
	trabajos := make(chan int)
	resultados := make(chan *parcial)
	fin := make(chan struct{}) // se cierra ante el primer error: corta al productor y a los workers

	var wg sync.WaitGroup
	for range min(workers, max(len(rangos), 1)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := nuevoTrabajador(reg, cols, delim)
			for i := range trabajos {
				select {
				case resultados <- w.procesar(r, i, rangos[i]):
				case <-fin:
					return
				}
			}
		}()
	}
	go func() {
		defer close(trabajos)
		for i := range rangos {
			select {
			case trabajos <- i:
			case <-fin:
				return
			}
		}
	}()
	go func() {
		wg.Wait()
		close(resultados)
	}()

	a := nuevoAcumulador()
	pendientes := map[int]*parcial{}
	siguiente := 0
	var primerErr error
	for p := range resultados {
		if primerErr != nil {
			continue // se drena el canal para que los workers terminen
		}
		if p.err != nil {
			primerErr = p.err
			close(fin)
			continue
		}
		pendientes[p.indice] = p
		for q, ok := pendientes[siguiente]; ok; q, ok = pendientes[siguiente] {
			delete(pendientes, siguiente)
			a.fusionar(q)
			siguiente++
		}
	}
	if primerErr != nil {
		return nil, primerErr
	}
	if siguiente != len(rangos) {
		return nil, fmt.Errorf("se fusionaron %d de %d bloques", siguiente, len(rangos))
	}
	return a, nil
}

// fusionar incorpora un parcial (regla 5 y regla 7, el pivot). Una celda que
// ya tiene valor se queda con él: el parcial actual viene después en el
// archivo. La primera lectura de una celda en el bloque que resulta duplicado
// de un bloque anterior pasa de procesada a descartada.
func (a *acumulador) fusionar(p *parcial) {
	mapa := make([]int32, len(p.estaciones))
	for i, e := range p.estaciones {
		g, ok := a.porNombre[e.nombre]
		if !ok {
			g = int32(len(a.estaciones))
			a.porNombre[e.nombre] = g
			a.estaciones = append(a.estaciones, &estacionGlobal{nombre: e.nombre, ubicacion: e.ubicacion})
		}
		a.estaciones[g].leidas += e.leidas
		a.estaciones[g].procesadas += e.procesadas
		mapa[i] = g
	}
	a.leidas += p.leidas
	a.procesadas += p.procesadas
	for r, n := range p.descartadas {
		a.descartadas[r] += n
	}
	for q, n := range p.fueraDeRango {
		a.fueraDeRango[q] += n
	}
	for _, e := range p.periodos {
		a.periodos[e.nombre] += e.n
	}
	for _, e := range p.parametros {
		a.parametros[e.nombre] += e.n
		if e.param < 0 {
			a.noMapeados[e.nombre] += e.n
		}
	}
	for q, es := range p.unidades {
		for _, e := range es {
			if a.unidades[q] == nil {
				a.unidades[q] = map[string]int64{}
			}
			a.unidades[q][e.nombre] += e.n
		}
	}

	// Primero las primeras lecturas de cada celda del bloque; después sus
	// repetidas, que en el archivo vienen detrás. Así cada duplicado se compara
	// contra el valor que de verdad se conserva: el primero en orden de archivo.
	globales := make([]int32, len(p.claves))
	for li, k := range p.claves {
		gk := claveGlobal{mapa[k.est], k.t}
		vals := p.valores[li*numParametros : (li+1)*numParametros]
		gi, ok := a.filas[gk]
		globales[li] = gi
		if !ok {
			globales[li] = int32(len(a.claves))
			a.filas[gk] = globales[li]
			a.claves = append(a.claves, gk)
			a.valores = append(a.valores, vals...)
			a.conflicto = append(a.conflicto, 0)
			continue
		}
		for q, v := range vals {
			if math.IsNaN(v) {
				continue
			}
			if a.duplicado(gi, q, v) {
				a.procesadas--
				a.estaciones[gk.est].procesadas--
			}
		}
	}
	for _, r := range p.repetidas {
		a.duplicado(globales[r.fila], r.param, r.v)
	}
	a.bloques++
}

// duplicado compara una lectura con la celda (fila, q). Si la celda está vacía
// la ocupa y devuelve false; si no, cuenta el duplicado (exacto o con otro
// valor, que además marca la celda) y devuelve true.
func (a *acumulador) duplicado(fila int32, q int, v float64) bool {
	celda := &a.valores[int(fila)*numParametros+q]
	switch {
	case math.IsNaN(*celda):
		*celda = v
		return false
	case *celda == v:
		a.descartadas[reglaDuplicadoExacto]++
	default:
		a.descartadas[reglaDuplicadoValorDistinto]++
		a.conflicto[fila] |= 1 << q
	}
	return true
}

// TablaAncha es el silver en memoria: una fila por (estación, fecha-hora),
// ordenada por estación y luego por fecha-hora, con los parámetros canónicos
// presentes en orden alfabético. NaN es «sin lectura».
type TablaAncha struct {
	Parametros []string       // canónicos con al menos un valor, en orden alfabético
	Estaciones []string       // estaciones con al menos una fila, en orden alfabético
	Estacion   []uint16       // por fila, índice en Estaciones
	Hora       []int64        // por fila, segundos Unix
	Valores    []float64      // N × len(Parametros), por filas
	N          int            // filas
	Zona       *time.Location // zona en que se escribe fecha_hora
}

// estadoTabla son los conteos que salen al armar la tabla.
type estadoTabla struct {
	celdasEnConflicto int64
	pm25MayorPm10     int
}

// tabla vuelca el mapa ordenando las claves por (estación, fecha-hora). Nunca
// itera el mapa: su orden en Go es aleatorio y rompería el determinismo.
func (a *acumulador) tabla(zona *time.Location) (*TablaAncha, estadoTabla, error) {
	var st estadoTabla
	conFilas := make([]bool, len(a.estaciones))
	for _, k := range a.claves {
		conFilas[k.est] = true
	}
	var nombres []string
	for g, ok := range conFilas {
		if ok {
			nombres = append(nombres, a.estaciones[g].nombre)
		}
	}
	slices.Sort(nombres)
	if len(nombres) > math.MaxUint16+1 {
		return nil, st, fmt.Errorf("%d estaciones no caben en un uint16", len(nombres))
	}
	posicion := make([]int32, len(a.estaciones))
	for g, e := range a.estaciones {
		i, ok := slices.BinarySearch(nombres, e.nombre)
		posicion[g] = int32(i)
		if !ok {
			posicion[g] = -1
		}
	}

	orden := make([]int32, len(a.claves))
	for i := range orden {
		orden[i] = int32(i)
	}
	// Las claves son únicas, así que el orden es total y el resultado no
	// depende de que SortFunc sea inestable.
	slices.SortFunc(orden, func(i, j int32) int {
		ki, kj := a.claves[i], a.claves[j]
		if c := cmp.Compare(posicion[ki.est], posicion[kj.est]); c != 0 {
			return c
		}
		return cmp.Compare(ki.t, kj.t)
	})

	var presentes []int
	for q := range numParametros {
		for i := range a.claves {
			if !math.IsNaN(a.valores[i*numParametros+q]) {
				presentes = append(presentes, q)
				break
			}
		}
	}

	t := &TablaAncha{
		Estaciones: nombres,
		Estacion:   make([]uint16, len(orden)),
		Hora:       make([]int64, len(orden)),
		Valores:    make([]float64, len(orden)*len(presentes)),
		N:          len(orden),
		Zona:       zona,
	}
	for _, q := range presentes {
		t.Parametros = append(t.Parametros, ParametrosCanonicos[q])
	}
	pa := len(presentes)
	for i, o := range orden {
		k := a.claves[o]
		t.Estacion[i] = uint16(posicion[k.est])
		t.Hora[i] = k.t
		fila := a.valores[int(o)*numParametros : (int(o)+1)*numParametros]
		for j, q := range presentes {
			t.Valores[i*pa+j] = fila[q]
		}
		if pm25, pm10 := fila[idxPM25], fila[idxPM10]; pm25 > pm10 { // NaN compara falso
			st.pm25MayorPm10++
		}
	}
	for _, c := range a.conflicto {
		st.celdasEnConflicto += int64(bits.OnesCount16(c))
	}
	return t, st, nil
}
