package cluster

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"sync"
	"time"

	"github.com/R0SEWT/aire-distribuido/protocolo"
)

// CargarTarea le manda a cada nodo su TAREA y espera su LISTO, a todos a la vez
// y con un mismo instante límite, como Cargar en el K-Means. tarea trae lo que
// comparten todos (id, manifiesto y su huella, validación, hiper); dias[i] son
// los días del nodo enlaces[i] (RepartirDias). Devuelve cuánto tardó cada nodo
// en bajar y verificar sus días; si alguno falla, el error nombra a cada uno
// que falló y el entrenamiento no puede empezar.
func CargarTarea(enlaces []*protocolo.Enlace, tarea protocolo.Mensaje, dias [][]string, limite time.Time) ([]time.Duration, error) {
	if len(enlaces) == 0 || len(enlaces) != len(dias) {
		return nil, fmt.Errorf("%d nodos y %d repartos de días", len(enlaces), len(dias))
	}
	tiempos := make([]time.Duration, len(enlaces))
	errs := make([]error, len(enlaces))
	var barrera sync.WaitGroup
	for i, e := range enlaces {
		t := tarea
		t.Dias = dias[i]
		barrera.Go(func() {
			// Cada goroutine escribe solo su índice y usa solo su enlace.
			inicio := time.Now()
			errs[i] = protocolo.AsignarTarea(e, t, limite)
			tiempos[i] = time.Since(inicio)
		})
	}
	barrera.Wait()
	if err := errors.Join(errs...); err != nil {
		return nil, fmt.Errorf("tarea abortada: %w", err)
	}
	return tiempos, nil
}

// Local es lo que devolvió un nodo en una ronda de local SGD, con lo que tardó.
type Local struct {
	Pesos      []float32 // sus pesos tras los pasos de la ronda
	Muestras   int       // las que vio en la ronda: su peso en el promedio
	Perdida    float64   // de entrenamiento, la que el nodo informa
	PerdidaVal float64   // en las estaciones de validación, la que el nodo informa
	// Computo es lo que el nodo dice que tardó en sus pasos (ms_computo).
	Computo time.Duration
	// Envio es lo que tardó en salir el PESOS: escribirlo en la conexión.
	Envio time.Duration
	// Total va desde que empezó a salir el PESOS hasta que llegó el LOCAL
	// entero. Total − Computo es lo que costó la red: mandar los pesos,
	// esperar y recibirlos de vuelta.
	Total time.Duration
}

// RondaLocal es una ronda de local SGD a través de la red. Manda el mismo PESOS
// {ronda, pasos, pesos} a todos los nodos a la vez (fan-out, una goroutine por
// nodo; el vector se codifica una sola vez), espera el LOCAL de esa ronda de
// cada uno (fan-in) y vuelve cuando cada nodo respondió o falló, y como muy
// tarde en limite.
//
// Los LOCAL vuelven EN EL ORDEN DE enlaces, no en el de llegada: es el orden en
// que Promediar acumula, y por eso el promedio no depende de qué nodo terminó
// primero.
//
// Antes de enviar nada rechaza un límite ya vencido, nodos repetidos, cerrados
// o sin TAREA confirmada, y un número de ronda que no crece: en esos casos
// ningún nodo recibe el PESOS. Si un nodo no responde a tiempo, cierra, manda
// ERROR, rompe el protocolo o devuelve pesos no finitos (un NaN arruinaría el
// promedio de todos), la ronda se aborta y el error nombra a cada nodo que
// falló; no se devuelve ningún LOCAL, porque un promedio con menos nodos sería
// otro modelo.
func RondaLocal(enlaces []*protocolo.Enlace, ronda, pasos int, pesos []float32, limite time.Time) ([]Local, error) {
	if len(enlaces) == 0 {
		return nil, errors.New("ronda sin nodos")
	}
	if !limite.After(time.Now()) {
		return nil, fmt.Errorf("ronda %d: %w antes de empezar; no se envió nada", ronda, protocolo.ErrPlazoVencido)
	}
	vistos := make(map[*protocolo.Enlace]bool, len(enlaces))
	for _, e := range enlaces {
		switch {
		case vistos[e]:
			return nil, fmt.Errorf("ronda %d: el nodo %q aparece dos veces", ronda, e.Nodo)
		case e.Cod.Roto() != nil:
			return nil, &protocolo.ErrorNodo{Nodo: e.Nodo, Err: fmt.Errorf("el enlace está cerrado: %w", e.Cod.Roto())}
		case e.Tarea == "":
			return nil, fmt.Errorf("ronda %d: el nodo %q no tiene una TAREA confirmada (falta TAREA y LISTO)", ronda, e.Nodo)
		case ronda <= e.UltimaRonda():
			return nil, fmt.Errorf("ronda %d: el número de ronda tiene que crecer y el nodo %q ya recibió la %d",
				ronda, e.Nodo, e.UltimaRonda())
		}
		vistos[e] = true
	}
	m := protocolo.Mensaje{Tipo: protocolo.Pesos, Ronda: ronda, Pasos: pasos}.ConVector(pesos)
	if err := m.Validar(); err != nil {
		return nil, fmt.Errorf("ronda %d: %w", ronda, err)
	}

	plazo := time.Until(limite).Round(time.Millisecond)
	locales := make([]Local, len(enlaces))
	errs := make([]error, len(enlaces))
	var barrera sync.WaitGroup
	for i, e := range enlaces {
		barrera.Go(func() {
			// Cada goroutine escribe solo su índice y usa solo su enlace; la
			// carga de m la leen todas y no la escribe nadie.
			locales[i], errs[i] = rondaDeUnNodo(e, m, limite, plazo)
		})
	}
	barrera.Wait()
	if err := errors.Join(errs...); err != nil {
		return nil, fmt.Errorf("ronda %d abortada: %w", ronda, err)
	}
	return locales, nil
}

// rondaDeUnNodo es la parte de RondaLocal que corresponde a un nodo.
func rondaDeUnNodo(e *protocolo.Enlace, pesos protocolo.Mensaje, limite time.Time, plazo time.Duration) (Local, error) {
	inicio := time.Now()
	if err := protocolo.EnviarPesos(e, pesos, limite); err != nil {
		return Local{}, err
	}
	envio := time.Since(inicio)
	m, err := protocolo.RecibirLocal(e, pesos.Ronda, pesos.N, limite)
	var en *protocolo.ErrorNodo
	if errors.Is(err, protocolo.ErrPlazoVencido) && errors.As(err, &en) {
		en.Err = fmt.Errorf("no envió su LOCAL dentro del plazo de la ronda (%v): %w", plazo, en.Err)
	}
	if err != nil {
		return Local{}, err
	}
	total := time.Since(inicio)
	v, err := m.Vector()
	if err != nil {
		return Local{}, &protocolo.ErrorNodo{Nodo: e.Nodo, Err: err}
	}
	for i, x := range v {
		if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
			return Local{}, &protocolo.ErrorNodo{Nodo: e.Nodo,
				Err: fmt.Errorf("sus pesos tienen un %v en la posición %d: el entrenamiento divergió", x, i)}
		}
	}
	return Local{Pesos: v, Muestras: m.Muestras, Perdida: m.Perdida, PerdidaVal: m.PerdidaVal,
		Computo: time.Duration(m.MsComputo * float64(time.Millisecond)), Envio: envio, Total: total}, nil
}

// Promediar es el promedio de los vectores ponderado por las muestras de cada
// uno: Σ mₖ·vₖ / Σ mₖ. Acumula en float64 EN EL ORDEN DE vectores (el de los
// nodos) y baja a float32 al final, así que el resultado es el mismo bit a bit
// cada vez, llegue cada LOCAL cuando llegue. Un nodo con 0 muestras no pesa;
// si nadie tiene muestras no hay promedio.
func Promediar(vectores [][]float32, muestras []int) ([]float32, error) {
	if len(vectores) == 0 || len(vectores) != len(muestras) {
		return nil, fmt.Errorf("%d vectores y %d pesos", len(vectores), len(muestras))
	}
	n := len(vectores[0])
	total := 0
	for k, v := range vectores {
		switch {
		case len(v) != n || n == 0:
			return nil, fmt.Errorf("el vector %d tiene %d valores y el 0 tiene %d", k, len(v), n)
		case muestras[k] < 0:
			return nil, fmt.Errorf("el vector %d tiene muestras negativas (%d)", k, muestras[k])
		}
		total += muestras[k]
	}
	if total == 0 {
		return nil, errors.New("ningún nodo tiene muestras: no hay promedio")
	}
	acum := make([]float64, n)
	for k, v := range vectores {
		w := float64(muestras[k])
		for i, x := range v {
			acum[i] += w * float64(x)
		}
	}
	out := make([]float32, n)
	for i, a := range acum {
		out[i] = float32(a / float64(total))
	}
	return out, nil
}

// OpcionesSGD son las del entrenamiento con local SGD.
type OpcionesSGD struct {
	Rondas int           // R: cuántas veces se promedia
	Pasos  int           // H: pasos de cada nodo entre dos promedios
	Plazo  time.Duration // de cada ronda, del primer PESOS al último LOCAL
}

// RondaSGD es una ronda completa: lo que mandaron los nodos, el promedio y lo
// que costó.
type RondaSGD struct {
	Ronda int
	// Pesos son los globales tras el promedio: los que van en el PESOS de la
	// ronda siguiente y los que se guardan como punto de control.
	Pesos []float32
	// Locales, en orden de nodo, sin sus vectores (ya promediados).
	Locales []Local
	// Muestras es Σ mₖ; Perdida y PerdidaVal, los promedios de las de cada
	// nodo ponderados por sus muestras, acumulados en orden de nodo.
	Muestras            int
	Perdida, PerdidaVal float64
	// Barrera va del primer PESOS al último LOCAL; Promedio es lo que tardó
	// Promediar.
	Barrera, Promedio time.Duration
	// Bytes es la carga binaria que cruzó la red: un PESOS y un LOCAL por nodo.
	Bytes int64
}

// SGDLocal entrena con local SGD sobre los nodos que ya confirmaron su TAREA
// (CargarTarea): op.Rondas rondas numeradas desde 1, cada una una RondaLocal
// con op.Pasos pasos y plazo op.Plazo, y el promedio de sus pesos ponderado por
// sus muestras (Promediar), que es el punto de partida de la ronda siguiente.
// Devuelve los pesos globales de la última ronda.
//
// Después de cada ronda completa llama a tras (si no es nil), que es donde
// quien llama guarda el punto de control y la medición; si tras devuelve un
// error, el entrenamiento se detiene con ese error. Si una ronda aborta,
// SGDLocal devuelve su error, que nombra a cada nodo que falló, y lo último
// que vio tras es la última ronda completa. Qué hacer con las conexiones lo
// decide quien llama (Abortar o Terminar).
func SGDLocal(enlaces []*protocolo.Enlace, pesos0 []float32, op OpcionesSGD, tras func(*RondaSGD) error) ([]float32, error) {
	if op.Rondas <= 0 || op.Pasos <= 0 || op.Plazo <= 0 {
		return nil, fmt.Errorf("rondas (%d), pasos (%d) y plazo (%v) tienen que ser positivos", op.Rondas, op.Pasos, op.Plazo)
	}
	pesos := slices.Clone(pesos0)
	for ronda := 1; ronda <= op.Rondas; ronda++ {
		inicio := time.Now()
		locales, err := RondaLocal(enlaces, ronda, op.Pasos, pesos, inicio.Add(op.Plazo))
		if err != nil {
			return nil, err
		}
		r := &RondaSGD{Ronda: ronda, Locales: locales, Barrera: time.Since(inicio)}

		vectores := make([][]float32, len(locales))
		muestras := make([]int, len(locales))
		for i, l := range locales {
			vectores[i], muestras[i] = l.Pesos, l.Muestras
		}
		promedio := time.Now()
		pesos, err = Promediar(vectores, muestras)
		if err != nil {
			return nil, fmt.Errorf("ronda %d abortada: %w", ronda, err)
		}
		r.Promedio = time.Since(promedio)
		r.Pesos = pesos

		for i := range locales {
			l := &r.Locales[i]
			l.Pesos = nil // ya están en el promedio; no retener N vectores
			r.Muestras += l.Muestras
			r.Perdida += float64(l.Muestras) * l.Perdida
			r.PerdidaVal += float64(l.Muestras) * l.PerdidaVal
		}
		r.Perdida /= float64(r.Muestras)
		r.PerdidaVal /= float64(r.Muestras)
		r.Bytes = int64(len(enlaces)) * 2 * 4 * int64(len(pesos))

		if tras != nil {
			if err := tras(r); err != nil {
				return nil, fmt.Errorf("tras la ronda %d: %w", ronda, err)
			}
		}
	}
	return pesos, nil
}
