package kmeans

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"
)

// Arranque es una corrida completa con una semilla: k-means++ y Lloyd.
type Arranque struct {
	Semilla             int64   `json:"semilla"`
	KEfectivo           int     `json:"k_efectivo"`
	Inercia             float64 `json:"inercia"`
	Iteraciones         int     `json:"iteraciones"`
	Paro                string  `json:"paro"`
	Vacios              int     `json:"clusters_vacios"`
	CentroidesIniciales string  `json:"centroides_iniciales_sha256"`
	Workers             int     `json:"workers"`
	MsKMeansPP          float64 `json:"ms_kmeanspp"`
	MsLloyd             float64 `json:"ms_lloyd"`
}

// Inicio son los centroides iniciales de una semilla: lo que k-means++
// produce y lo que se materializa en reports/centroides_iniciales.json.
type Inicio struct {
	Semilla    int64
	KEfectivo  int
	Centroides []float64 // KEfectivo*D, por filas
}

// Multiarranque es lo que devuelve MejorDeSemillas: el resultado elegido y la
// traza de todas las semillas probadas.
type Multiarranque struct {
	Mejor     *Resultado
	Elegido   int        // índice en Arranques del resultado elegido
	Arranques []Arranque // uno por semilla, en orden de semilla
	Inicios   []Inicio   // los centroides iniciales de cada semilla, en el mismo orden
	Cupos     []int      // workers de cada corrida simultánea; len(Cupos) corren a la vez
}

// MejorDeSemillas corre el K-Means completo (k-means++ y el Lloyd concurrente)
// con las semillas semilla, semilla+1, …, semilla+n-1 y se queda con el de
// menor inercia final; si dos empatan bit a bit, con el de la semilla menor.
// Lloyd converge a mínimos locales: reiniciar con varias semillas es la
// práctica estándar (scikit-learn usa n_init = 10).
//
// Es el paralelismo por semillas de la spec §7: las corridas no comparten nada
// salvo los datos, que son de solo lectura, así que corren a la vez sin
// sincronizarse hasta el final. El presupuesto de op.Workers goroutines se
// reparte entre las corridas simultáneas con RepartirSemillas (paralelas = 0
// es automático; ver ahí).
//
// Es determinista: cada corrida da lo mismo para cualquier cantidad de workers
// (la reducción va en orden de chunk) y la elección es el mínimo bajo un orden
// total, (inercia, semilla), que no depende de en qué orden terminen. op.K y
// op.AlIterar se ignoran: k es el parámetro y las corridas simultáneas no
// pueden compartir un callback de progreso.
func MejorDeSemillas(d *Datos, k int, semilla int64, n int, op OpcionesConc, paralelas int) (*Multiarranque, error) {
	if k <= 0 {
		return nil, fmt.Errorf("k debe ser positivo, es %d", k)
	}
	return correrArranques(d, n, op, paralelas, func(i int) (Inicio, float64, error) {
		t0 := time.Now()
		cent, kEf, err := KMeansPP(d, k, semilla+int64(i))
		if err != nil {
			return Inicio{Semilla: semilla + int64(i)}, 0, err
		}
		return Inicio{Semilla: semilla + int64(i), KEfectivo: kEf, Centroides: cent},
			float64(time.Since(t0).Nanoseconds()) / 1e6, nil
	})
}

// MejorDeInicios es MejorDeSemillas partiendo de centroides iniciales ya
// materializados (reports/centroides_iniciales.json) en vez de correr
// k-means++: la misma semilla no garantiza los mismos centroides si cambia el
// generador o el orden en que se consume; el archivo sí. Con los inicios que
// produjo MejorDeSemillas da el mismo resultado bit a bit.
func MejorDeInicios(d *Datos, inicios []Inicio, op OpcionesConc, paralelas int) (*Multiarranque, error) {
	for _, in := range inicios {
		if in.KEfectivo <= 0 || len(in.Centroides) != in.KEfectivo*d.D {
			return nil, fmt.Errorf("semilla %d: %d valores para k=%d y D=%d", in.Semilla, len(in.Centroides), in.KEfectivo, d.D)
		}
	}
	return correrArranques(d, len(inicios), op, paralelas, func(i int) (Inicio, float64, error) {
		return inicios[i], 0, nil
	})
}

// correrArranques reparte las n corridas entre las corridas simultáneas que
// decide RepartirSemillas; iniciar(i) da los centroides iniciales de la i-ésima.
func correrArranques(d *Datos, n int, op OpcionesConc, paralelas int,
	iniciar func(i int) (Inicio, float64, error)) (*Multiarranque, error) {
	if n <= 0 {
		return nil, fmt.Errorf("hace falta al menos una semilla, se pidieron %d", n)
	}
	if paralelas < 0 {
		return nil, fmt.Errorf("corridas simultáneas no puede ser negativo, es %d", paralelas)
	}
	if op.Workers <= 0 {
		return nil, fmt.Errorf("workers debe ser positivo, es %d", op.Workers)
	}

	m := &Multiarranque{Elegido: -1, Arranques: make([]Arranque, n), Inicios: make([]Inicio, n),
		Cupos: RepartirSemillas(n, op.Workers, paralelas)}
	errs := make([]error, n)
	indices := make(chan int, n)
	for i := 0; i < n; i++ {
		indices <- i
	}
	close(indices)

	var mu sync.Mutex // protege Mejor y Elegido; Arranques[i] e Inicios[i] los escribe solo quien corrió i
	var wg sync.WaitGroup
	wg.Add(len(m.Cupos))
	for _, workers := range m.Cupos {
		go func() {
			defer wg.Done()
			for i := range indices {
				in, msInit, err := iniciar(i)
				if err != nil {
					errs[i] = fmt.Errorf("semilla %d: %w", in.Semilla, err)
					continue
				}
				r, a, err := arrancar(d, in, op, workers)
				if err != nil {
					errs[i] = fmt.Errorf("semilla %d: %w", in.Semilla, err)
					continue
				}
				a.MsKMeansPP = msInit
				m.Arranques[i], m.Inicios[i] = a, in
				mu.Lock()
				if m.Mejor == nil || r.Inercia < m.Mejor.Inercia || (r.Inercia == m.Mejor.Inercia && i < m.Elegido) {
					m.Mejor, m.Elegido = r, i
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return m, nil
}

func arrancar(d *Datos, in Inicio, op OpcionesConc, workers int) (*Resultado, Arranque, error) {
	opi := op
	opi.K, opi.Workers, opi.AlIterar = in.KEfectivo, workers, nil
	t0 := time.Now()
	r, err := Concurrente(d, in.Centroides, opi)
	if err != nil {
		return nil, Arranque{}, err
	}
	return r, Arranque{
		Semilla: in.Semilla, KEfectivo: in.KEfectivo, Inercia: r.Inercia, Iteraciones: r.Iteraciones, Paro: r.Paro,
		Vacios: r.Vacios, CentroidesIniciales: HashCentroides(in.Centroides), Workers: workers,
		MsLloyd: float64(time.Since(t0).Nanoseconds()) / 1e6,
	}, nil
}

// RepartirSemillas decide cuántas corridas van a la vez y con cuántos workers
// cada una: devuelve un cupo de workers por corrida simultánea. Los cupos
// suman el presupuesto (o hay uno por corrida si se piden más corridas
// simultáneas que workers), así que no se sobresuscribe la máquina.
//
// paralelas = 0 es automático: round(√min(n, workers)) corridas a la vez (3
// con 10 semillas y 14 workers), con los workers repartidos entre ellas. Ni
// los extremos ganan, medido con cmd/entrenar -paralelas sobre el gold
// sintético (ver doc.go): en serie con todo el pool (paralelas = 1) cada
// corrida paga una barrera por iteración con eficiencia de ~50 % a 14 workers;
// todas a la vez con un worker (paralelas = n) el tiempo es el de la semilla
// más lenta, porque las iteraciones hasta converger varían mucho entre semillas
// (de 18 a 82 en el sintético) y en un procesador híbrido algunas caen en
// núcleos lentos. Con pocas corridas a la vez cada una usa pocos workers, que es
// donde la barrera rinde, y la cola de semillas reparte la carga sola.
func RepartirSemillas(n, workers, paralelas int) []int {
	p := paralelas
	if p == 0 {
		p = int(math.Round(math.Sqrt(float64(min(n, workers)))))
	}
	p = max(1, min(p, n))
	base, resto := workers/p, workers%p
	if base == 0 {
		base, resto = 1, 0
	}
	cupos := make([]int, p)
	for s := range cupos {
		cupos[s] = base
		if s < resto {
			cupos[s]++
		}
	}
	return cupos
}
