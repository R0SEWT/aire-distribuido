package kmeans

import (
	"runtime"
	"syscall"
)

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
}

// MedirRecursos combina las estadísticas del runtime de Go con las del sistema.
// Getrusage da el máximo histórico de RSS y el tiempo de CPU consumido, que es
// lo que permite comparar tiempo de CPU contra tiempo de pared y ver cuánto
// paralelismo hubo de verdad.
func MedirRecursos() Recursos {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	r := Recursos{
		HeapMB:          float64(m.HeapAlloc) / (1 << 20),
		TotalAsignadoMB: float64(m.TotalAlloc) / (1 << 20),
		SistemaMB:       float64(m.Sys) / (1 << 20),
		NumGC:           m.NumGC,
	}
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err == nil {
		// En Linux, ru_maxrss viene en kilobytes.
		r.MaxRSSMB = float64(ru.Maxrss) / 1024
		r.CPUUsuarioS = float64(ru.Utime.Sec) + float64(ru.Utime.Usec)/1e6
		r.CPUSistemaS = float64(ru.Stime.Sec) + float64(ru.Stime.Usec)/1e6
	}
	return r
}
