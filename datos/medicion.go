package datos

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"os"
	"runtime"
	"slices"
	"time"

	"github.com/R0SEWT/aire-distribuido/kmeans"
)

// Benchmark es reports/benchmark_cargador.json: tiempos de la carga completa
// (lectura, limpieza, pivot y gold, sin escribir los CSV) para cada W.
type Benchmark struct {
	Entrada      string     `json:"entrada"`
	Bytes        int64      `json:"bytes"`
	FilasLargas  int64      `json:"filas_largas"`
	FilasAnchas  int        `json:"filas_anchas"`
	BloqueBytes  int64      `json:"bloque_bytes"`
	Bloques      int        `json:"bloques"`
	Chunk        int        `json:"chunk"`
	Repeticiones int        `json:"repeticiones"`
	Orden        string     `json:"orden"`
	CPUs         int        `json:"cpus"`
	GOMAXPROCS   int        `json:"gomaxprocs"`
	Go           string     `json:"go"`
	Plataforma   string     `json:"plataforma"`
	Host         string     `json:"host"`
	Fecha        string     `json:"fecha"`
	BaseWorkers  int        `json:"base_workers"`
	Mediciones   []Medicion `json:"mediciones"`
	Identicos    bool       `json:"identicos"`
	// Agregados para el informe: sha256 de la entrada (calculado una vez,
	// fuera de las mediciones), cómo se estima el IC del speedup, y la
	// procedencia de la corrida.
	Sha256    string                    `json:"sha256,omitempty"`
	Protocolo ProtocoloSpeedup          `json:"protocolo"`
	Metadatos *kmeans.Metadatos         `json:"metadatos,omitempty"`
	Configs   map[string]VersionArchivo `json:"configs,omitempty"`
}

// ProtocoloSpeedup dice cómo se calcularon los dos speedups de cada medición,
// con los nombres de campo del protocolo de cmd/benchmark.
type ProtocoloSpeedup struct {
	Calentamiento     int     `json:"calentamiento"`
	NotaRecorte       string  `json:"nota_recorte,omitempty"`
	Speedup           string  `json:"speedup"`
	SpeedupIC         string  `json:"speedup_ic"`
	RecortePorExtremo float64 `json:"recorte_por_extremo"`
	ReplicasBootstrap int     `json:"replicas_bootstrap"`
	SemillaBootstrap  int64   `json:"semilla_bootstrap"`
}

// Medicion son las repeticiones de un W. Speedup es la mediana de la base
// (el primer W de la lista) sobre la mediana de este W. SpeedupIC es el
// cociente de medias recortadas con su intervalo bootstrap del 95 %, como el
// campo speedup de cmd/benchmark (punto, inferior, superior, nivel).
type Medicion struct {
	Workers         int                       `json:"workers"`
	TiemposS        []float64                 `json:"tiempos_s"`
	MedianaS        float64                   `json:"mediana_s"`
	Speedup         float64                   `json:"speedup"`
	Sha256Silver    string                    `json:"sha256_silver"`
	Sha256Gold      string                    `json:"sha256_gold"`
	MediaRecortadaS float64                   `json:"media_recortada_s"`
	SpeedupIC       *kmeans.IntervaloCociente `json:"speedup_ic,omitempty"`
}

// MedirCarga repite la carga reps veces para cada W de workers (0 =
// runtime.NumCPU()), con el mismo bloque y chunk de op. Las corridas se
// intercalan (en cada repetición se corre cada W una vez) para que la deriva
// térmica o de caché no se cargue toda a un W, y antes de cada una se fuerza
// un GC para que la basura de la anterior no se cobre dentro del tiempo
// medido. Cada corrida se verifica con el sha256 de su silver y su gold:
// Identicos dice si todas dieron lo mismo. Devuelve también el resultado de
// la última corrida, para escribir las salidas sin cargar otra vez.
func MedirCarga(ruta string, cfg ConfigLimpieza, rec Receta, workers []int, reps int, op Opciones) (*Benchmark, *Resultado, error) {
	if len(workers) == 0 {
		return nil, nil, errors.New("benchmark: hace falta al menos un valor de workers")
	}
	if reps < 1 {
		return nil, nil, fmt.Errorf("benchmark: repeticiones = %d, tiene que ser al menos 1", reps)
	}
	ws := make([]int, len(workers))
	for i, w := range workers {
		switch {
		case w < 0:
			return nil, nil, fmt.Errorf("benchmark: workers = %d", w)
		case w == 0:
			ws[i] = runtime.NumCPU()
		default:
			ws[i] = w
		}
	}
	f, err := os.Open(ruta)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, nil, err
	}

	host, _ := os.Hostname()
	bm := &Benchmark{
		Entrada:      ruta,
		Bytes:        info.Size(),
		Repeticiones: reps,
		Orden:        "intercalado: en cada repetición se corre cada W una vez, en el orden de la lista",
		CPUs:         runtime.NumCPU(),
		GOMAXPROCS:   runtime.GOMAXPROCS(0),
		Go:           runtime.Version(),
		Plataforma:   runtime.GOOS + "/" + runtime.GOARCH,
		Host:         host,
		Fecha:        time.Now().Format(time.RFC3339),
		BaseWorkers:  ws[0],
		Identicos:    true,
	}
	bm.Mediciones = make([]Medicion, len(ws))
	// Calentamiento: una corrida descartada con el W más alto, que trae el
	// archivo a la caché de páginas y hace crecer el heap antes de medir.
	runtime.GC()
	cal := op
	cal.Workers = slices.Max(ws)
	if _, err := CargarDesde(f, info.Size(), ruta, cfg, rec, cal); err != nil {
		return nil, nil, err
	}
	var ultimo *Resultado
	var referencia [2]string
	for rep := range reps {
		for i, w := range ws {
			ultimo = nil
			runtime.GC()
			o := op
			o.Workers = w
			inicio := time.Now()
			res, err := CargarDesde(f, info.Size(), ruta, cfg, rec, o)
			dt := time.Since(inicio).Seconds()
			if err != nil {
				return nil, nil, err
			}
			huella, err := huellas(res)
			if err != nil {
				return nil, nil, err
			}
			if rep == 0 && i == 0 {
				referencia = huella
			} else if huella != referencia {
				bm.Identicos = false
			}
			m := &bm.Mediciones[i]
			m.Workers = w
			m.TiemposS = append(m.TiemposS, dt)
			m.Sha256Silver, m.Sha256Gold = huella[0], huella[1]
			ultimo = res
		}
	}
	for i := range bm.Mediciones {
		bm.Mediciones[i].MedianaS = mediana(bm.Mediciones[i].TiemposS)
	}
	b := BootstrapPorDefecto
	bm.Protocolo = ProtocoloSpeedup{
		Calentamiento:     1,
		Speedup:           "mediana con la base / mediana con W (cociente de medianas)",
		SpeedupIC:         "media recortada con la base / media recortada con W, IC bootstrap 95 % (kmeans.ICBootstrapCociente)",
		RecortePorExtremo: b.Recorte,
		ReplicasBootstrap: b.Replicas,
		SemillaBootstrap:  b.Semilla,
	}
	if int(float64(reps)*b.Recorte) == 0 {
		bm.Protocolo.NotaRecorte = fmt.Sprintf("con %d repeticiones el recorte del %.0f %% no descarta ninguna: "+
			"la media recortada es la media; hacen falta al menos %d", reps, 100*b.Recorte, int(math.Ceil(1/b.Recorte)))
	}
	for i := range bm.Mediciones {
		m := &bm.Mediciones[i]
		m.Speedup = bm.Mediciones[0].MedianaS / m.MedianaS
		if m.SpeedupIC, m.MediaRecortadaS, err = speedupConIC(bm.Mediciones[0].TiemposS, m.TiemposS, b); err != nil {
			return nil, nil, fmt.Errorf("benchmark: IC del speedup con W=%d: %w", m.Workers, err)
		}
	}
	l := ultimo.Limpieza
	bm.FilasLargas, bm.FilasAnchas = l.FilasLargas.Leidas, l.FilasAnchas.Total
	bm.BloqueBytes, bm.Bloques, bm.Chunk = l.BloqueBytes, l.Bloques, l.Chunk
	return bm, ultimo, nil
}

// huellas son los sha256 del silver y del gold tal como se escribirían.
func huellas(res *Resultado) ([2]string, error) {
	s, g := sha256.New(), sha256.New()
	if err := res.Ancha.EscribirSilver(s); err != nil {
		return [2]string{}, err
	}
	if err := res.Dataset.EscribirGold(g); err != nil {
		return [2]string{}, err
	}
	return [2]string{hex.EncodeToString(s.Sum(nil)), hex.EncodeToString(g.Sum(nil))}, nil
}

func mediana(xs []float64) float64 {
	s := slices.Clone(xs)
	slices.Sort(s)
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}
