package kmeans

import "runtime"

// Recursos es una foto del consumo del proceso. Se toma entre fases para poder
// decir cuál de ellas mueve la aguja: el RSS máximo del proceso puede venir del
// parseo del CSV y no del clustering, y atribuírselo al clustering sería un
// error de lectura, no de medición.
type Recursos struct {
	MaxRSSMB        float64 `json:"max_rss_mb"`        // máximo histórico del proceso (monótono)
	HeapMB          float64 `json:"heap_mb"`           // heap vivo ahora
	TotalAsignadoMB float64 `json:"total_asignado_mb"` // acumulado desde el arranque
	SistemaMB       float64 `json:"sistema_mb"`        // reservado al sistema operativo
	NumGC           uint32  `json:"num_gc"`
	CPUUsuarioS     float64 `json:"cpu_usuario_s"`
	CPUSistemaS     float64 `json:"cpu_sistema_s"`
	// NoDisponible nombra, con su nombre en el JSON, los campos que esta
	// plataforma no puede medir y quedaron en 0: en Windows, el RSS máximo y el
	// tiempo de CPU (recursos_otros.go). En Linux y los demás Unix se mide todo,
	// queda vacío y no se escribe, así que el JSON de siempre no cambia.
	NoDisponible []string `json:"no_disponible,omitempty"`
}

// MedirRecursos combina las estadísticas del runtime de Go, que existen en
// todas las plataformas, con las del sistema operativo (medirSistema), que
// dependen de él: en Unix, Getrusage da el máximo histórico de RSS y el tiempo
// de CPU consumido, que es lo que permite comparar tiempo de CPU contra tiempo
// de pared y ver cuánto paralelismo hubo de verdad.
func MedirRecursos() Recursos {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	r := Recursos{
		HeapMB:          float64(m.HeapAlloc) / (1 << 20),
		TotalAsignadoMB: float64(m.TotalAlloc) / (1 << 20),
		SistemaMB:       float64(m.Sys) / (1 << 20),
		NumGC:           m.NumGC,
	}
	medirSistema(&r)
	return r
}

// camposDelSistema son los campos que llena medirSistema, con su nombre en el JSON.
var camposDelSistema = []string{"max_rss_mb", "cpu_usuario_s", "cpu_sistema_s"}
