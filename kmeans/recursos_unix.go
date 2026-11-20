//go:build unix

package kmeans

import (
	"runtime"
	"slices"
	"syscall"
)

// medirSistema completa el RSS máximo y el tiempo de CPU con Getrusage, que
// solo existe en Unix: en Windows este archivo no se compila
// (recursos_otros.go).
func medirSistema(r *Recursos) {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		r.NoDisponible = slices.Clone(camposDelSistema)
		return
	}
	// ru_maxrss viene en kilobytes en Linux y los BSD, y en bytes en macOS.
	kb := float64(ru.Maxrss)
	if runtime.GOOS == "darwin" || runtime.GOOS == "ios" {
		kb /= 1024
	}
	r.MaxRSSMB = kb / 1024
	r.CPUUsuarioS = float64(ru.Utime.Sec) + float64(ru.Utime.Usec)/1e6
	r.CPUSistemaS = float64(ru.Stime.Sec) + float64(ru.Stime.Usec)/1e6
}
