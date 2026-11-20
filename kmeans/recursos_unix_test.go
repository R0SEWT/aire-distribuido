//go:build unix

package kmeans

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEnUnixRecursosTraeRSSYTiempoDeCPU(t *testing.T) {
	d := nubes(2000)
	cent, _, err := KMeansPP(d, 4, 1)
	if err != nil {
		t.Fatalf("KMeansPP: %v", err)
	}
	if _, err := Secuencial(d, cent, Opciones{K: 4, MaxIter: 5, TolAbs: 0, TolRel: 0}); err != nil {
		t.Fatalf("Secuencial: %v", err)
	}
	r := MedirRecursos()
	if r.MaxRSSMB <= 0 {
		t.Errorf("MaxRSSMB = %v, se esperaba algo positivo", r.MaxRSSMB)
	}
	if r.CPUUsuarioS+r.CPUSistemaS <= 0 {
		t.Errorf("tiempo de CPU %v + %v: después de calcular tiene que ser positivo", r.CPUUsuarioS, r.CPUSistemaS)
	}
	if len(r.NoDisponible) != 0 {
		t.Errorf("en Unix todo se mide, y no_disponible dice %v", r.NoDisponible)
	}
	// El JSON de cmd/benchmark y cmd/entrenar no cambia: no_disponible no se escribe.
	b, _ := json.Marshal(r)
	if strings.Contains(string(b), "no_disponible") {
		t.Errorf("en Unix el JSON no tiene que llevar no_disponible: %s", b)
	}
}

func TestMaxRSSNoBajaEntreMediciones(t *testing.T) {
	// El máximo histórico del proceso es monótono: sirve para distinguir el
	// pico de la carga del CSV del pico del clustering.
	antes := MedirRecursos()
	d := nubes(5000)
	_ = d
	despues := MedirRecursos()
	if despues.MaxRSSMB < antes.MaxRSSMB {
		t.Errorf("MaxRSS bajó de %v a %v", antes.MaxRSSMB, despues.MaxRSSMB)
	}
}
