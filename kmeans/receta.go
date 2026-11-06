package kmeans

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"runtime"
)

// Niveles de estandarización.
const (
	NivelAbsoluto = "absoluto"
	NivelEstacion = "estacion"
)

// ParametrosCanonicos son los nombres de parámetro que el cargador usa en el
// gold. FraccionFina es derivada: pm2_5 / pm10.
var ParametrosCanonicos = []string{
	"pm2_5", "pm10", "no2", "so2", "co", "o3", "temperatura", "humedad",
	"viento_velocidad", "viento_direccion", "presion", "precipitacion", "radiacion",
}

// Nombres de las features que el perfil de régimen trata en especial.
const (
	FeaturePM25         = "pm2_5"
	FeaturePM10         = "pm10"
	FeatureFraccionFina = "fraccion_fina"
)

// Receta es configs/kmeans.json: lo que la calibración en Colab congela y Go
// solo lee. Cada campo está documentado en el comentario del paquete (doc.go).
type Receta struct {
	Features   []string `json:"features"`
	Log1p      []string `json:"log1p"`
	Nivel      string   `json:"nivel"`
	K          int      `json:"k"`
	Semilla    int64    `json:"semilla"`
	Semillas   int      `json:"semillas"`
	Tolerancia float64  `json:"tolerancia"`
	MaxIter    int      `json:"max_iter"`
	Workers    int      `json:"workers"`
	Chunk      int      `json:"chunk"`
}

// LeerReceta lee y valida configs/kmeans.json.
func LeerReceta(ruta string) (Receta, error) {
	f, err := os.Open(ruta)
	if err != nil {
		return Receta{}, err
	}
	defer f.Close()
	r, err := DecodificarReceta(f)
	if err != nil {
		return Receta{}, fmt.Errorf("%s: %w", ruta, err)
	}
	return r, nil
}

// DecodificarReceta lee una receta en JSON y la valida. Rechaza campos
// desconocidos: un "max_iters" mal escrito no debe ignorarse en silencio y
// dejar la corrida con el valor por omisión.
func DecodificarReceta(rd io.Reader) (Receta, error) {
	dec := json.NewDecoder(rd)
	dec.DisallowUnknownFields()
	var r Receta
	if err := dec.Decode(&r); err != nil {
		return Receta{}, fmt.Errorf("receta: %w", err)
	}
	if dec.More() {
		return Receta{}, fmt.Errorf("receta: hay datos después del objeto JSON")
	}
	if err := r.Validar(); err != nil {
		return Receta{}, err
	}
	if r.Semillas == 0 {
		r.Semillas = 1 // ausente: una sola semilla, como antes de que existiera el campo
	}
	return r, nil
}

// Validar comprueba que la receta tenga sentido antes de cargar nada.
func (r Receta) Validar() error {
	if len(r.Features) == 0 {
		return fmt.Errorf("receta: no hay features")
	}
	canonicas := map[string]bool{FeatureFraccionFina: true}
	for _, p := range ParametrosCanonicos {
		canonicas[p] = true
	}
	vistas := map[string]bool{}
	for _, f := range r.Features {
		if !canonicas[f] {
			return fmt.Errorf("receta: %q no es un parámetro canónico (%v o %s)", f, ParametrosCanonicos, FeatureFraccionFina)
		}
		if vistas[f] {
			return fmt.Errorf("receta: la feature %q está repetida", f)
		}
		vistas[f] = true
	}
	conLog := map[string]bool{}
	for _, f := range r.Log1p {
		if !vistas[f] {
			return fmt.Errorf("receta: log1p sobre %q, que no es una de las features", f)
		}
		if conLog[f] {
			return fmt.Errorf("receta: %q está repetida en log1p", f)
		}
		conLog[f] = true
	}
	if r.Nivel != NivelAbsoluto && r.Nivel != NivelEstacion {
		return fmt.Errorf("receta: nivel %q, se esperaba %q o %q", r.Nivel, NivelAbsoluto, NivelEstacion)
	}
	if r.K <= 0 {
		return fmt.Errorf("receta: k debe ser positivo, es %d", r.K)
	}
	if r.Semillas < 0 {
		return fmt.Errorf("receta: semillas no puede ser negativo (0 o ausente = 1), es %d", r.Semillas)
	}
	if r.MaxIter <= 0 {
		return fmt.Errorf("receta: max_iter debe ser positivo, es %d", r.MaxIter)
	}
	if r.Tolerancia < 0 || math.IsNaN(r.Tolerancia) || math.IsInf(r.Tolerancia, 0) {
		return fmt.Errorf("receta: tolerancia inválida (%v)", r.Tolerancia)
	}
	if r.Workers < 0 {
		return fmt.Errorf("receta: workers no puede ser negativo (0 = GOMAXPROCS), es %d", r.Workers)
	}
	if r.Chunk <= 0 {
		return fmt.Errorf("receta: chunk debe ser positivo, es %d", r.Chunk)
	}
	return nil
}

// OpcionesConc traduce la receta al contrato numérico del paquete. La
// tolerancia se usa como TolAbs y como TolRel (en el TP ambas valían lo
// mismo), y workers = 0 es runtime.GOMAXPROCS(0): desde Go 1.25 respeta el
// límite de CPU del contenedor (cgroup), y NumCPU no; en Docker Compose
// pediría más goroutines que núcleos asignados.
func (r Receta) OpcionesConc() OpcionesConc {
	w := r.Workers
	if w == 0 {
		w = runtime.GOMAXPROCS(0)
	}
	return OpcionesConc{
		Opciones: Opciones{K: r.K, MaxIter: r.MaxIter, TolAbs: r.Tolerancia, TolRel: r.Tolerancia},
		Workers:  w,
		Chunk:    r.Chunk,
	}
}

// ConLog1p dice si la feature f de la receta va con log1p.
func (r Receta) ConLog1p(f string) bool {
	for _, l := range r.Log1p {
		if l == f {
			return true
		}
	}
	return false
}
