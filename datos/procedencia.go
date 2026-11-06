package datos

import (
	"errors"
	"fmt"
	"os"

	"github.com/R0SEWT/aire-distribuido/kmeans"
)

// VersionArchivo identifica la versión exacta de un archivo usado en la
// corrida (la entrada o una config). Existe = false con sha256 vacío es una
// config que no estaba: la receta provisional en vez de configs/kmeans.json.
type VersionArchivo struct {
	Ruta   string `json:"ruta"`
	Sha256 string `json:"sha256,omitempty"`
	Existe bool   `json:"existe"`
}

// VersionDeArchivo calcula el sha256 de ruta. Que no exista no es un error.
func VersionDeArchivo(ruta string) (VersionArchivo, error) {
	v := VersionArchivo{Ruta: ruta}
	info, err := os.Stat(ruta)
	if errors.Is(err, os.ErrNotExist) {
		return v, nil
	}
	if err != nil {
		return v, err
	}
	if info.IsDir() {
		return v, fmt.Errorf("%s es un directorio, no un archivo", ruta)
	}
	if v.Sha256, err = kmeans.Sha256Archivo(ruta); err != nil {
		return v, err
	}
	v.Existe = true
	return v, nil
}

// Procedencia es lo que hace reproducible una corrida: en qué máquina, con qué
// Go y qué commit (los mismos metadatos que escribe cmd/entrenar), sobre qué
// archivo de entrada y con qué versión de cada config. La arma quien conoce
// las rutas (cmd/cargar); la carga en sí no la necesita.
type Procedencia struct {
	Metadatos     kmeans.Metadatos
	Sha256Entrada string
	Configs       map[string]VersionArchivo // "limpieza" y "kmeans"
}

// Anotar copia la procedencia en los dos reportes de la carga.
func (r *Resultado) Anotar(p Procedencia) {
	meta := p.Metadatos
	r.Limpieza.Sha256 = p.Sha256Entrada
	r.Limpieza.Metadatos = &meta
	r.Limpieza.Configs = p.Configs
	r.Estandarizacion.Metadatos = &meta
	r.Estandarizacion.Configs = p.Configs
}

// Anotar copia la procedencia en el benchmark.
func (b *Benchmark) Anotar(p Procedencia) {
	meta := p.Metadatos
	b.Sha256 = p.Sha256Entrada
	b.Metadatos = &meta
	b.Configs = p.Configs
}

// Bootstrap fija cómo se estima el intervalo del speedup: los mismos valores
// por defecto que cmd/benchmark del K-Means, para que las dos tablas del
// informe se lean igual.
type Bootstrap struct {
	Recorte  float64 // proporción recortada por extremo en la media recortada
	Replicas int
	Semilla  int64
}

// BootstrapPorDefecto: recorte del 10 % por extremo, 2000 réplicas, semilla 7.
var BootstrapPorDefecto = Bootstrap{Recorte: 0.10, Replicas: 2000, Semilla: 7}

// speedupConIC es media recortada(base) / media recortada(w) con su IC
// bootstrap del 95 % (kmeans.ICBootstrapCociente), y la media recortada de w.
func speedupConIC(base, w []float64, b Bootstrap) (*kmeans.IntervaloCociente, float64, error) {
	ic, err := kmeans.ICBootstrapCociente(base, w, b.Recorte, b.Replicas, b.Semilla)
	if err != nil {
		return nil, 0, err
	}
	recortada, err := kmeans.MediaRecortada(w, b.Recorte)
	if err != nil {
		return nil, 0, err
	}
	return &ic, recortada, nil
}
