//go:build !unix

package kmeans

import (
	"slices"
	"testing"
)

func TestFueraDeUnixElRSSYLaCPUQuedanMarcados(t *testing.T) {
	r := MedirRecursos()
	want := []string{"max_rss_mb", "cpu_usuario_s", "cpu_sistema_s"}
	if !slices.Equal(r.NoDisponible, want) {
		t.Fatalf("no_disponible = %v, se esperaba %v", r.NoDisponible, want)
	}
	if r.MaxRSSMB != 0 || r.CPUUsuarioS != 0 || r.CPUSistemaS != 0 {
		t.Fatalf("lo no disponible tiene que quedar en 0: %+v", r)
	}
}
