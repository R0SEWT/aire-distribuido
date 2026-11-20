package kmeans

import (
	"encoding/json"
	"testing"
)

func TestRecursosTraeLaMemoriaDelRuntime(t *testing.T) {
	// Trabajo real para que haya algo que medir.
	d := nubes(2000)
	cent, _, err := KMeansPP(d, 4, 1)
	if err != nil {
		t.Fatalf("KMeansPP: %v", err)
	}
	if _, err := Secuencial(d, cent, Opciones{K: 4, MaxIter: 5, TolAbs: 0, TolRel: 0}); err != nil {
		t.Fatalf("Secuencial: %v", err)
	}

	r := MedirRecursos()
	if r.HeapMB <= 0 || r.SistemaMB <= 0 {
		t.Errorf("HeapMB = %v y SistemaMB = %v, se esperaba algo positivo", r.HeapMB, r.SistemaMB)
	}
	if r.CPUUsuarioS < 0 || r.CPUSistemaS < 0 {
		t.Errorf("tiempos de CPU negativos: %v, %v", r.CPUUsuarioS, r.CPUSistemaS)
	}
	if r.TotalAsignadoMB < r.HeapMB {
		t.Errorf("TotalAsignadoMB (%v) debería ser al menos el heap vivo (%v)", r.TotalAsignadoMB, r.HeapMB)
	}
}

func TestLoNoDisponibleSonCamposDelJSONEnCero(t *testing.T) {
	// Un campo que la plataforma no mide se escribe en 0 y se nombra en
	// no_disponible: quien lee el JSON no confunde «no se midió» con «cero».
	r := MedirRecursos()
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var campos map[string]any
	if err := json.Unmarshal(b, &campos); err != nil {
		t.Fatal(err)
	}
	for _, c := range r.NoDisponible {
		v, ok := campos[c]
		if !ok {
			t.Errorf("no_disponible nombra %q, que no es un campo del JSON", c)
		} else if v != 0.0 {
			t.Errorf("%q no está disponible y vale %v; tenía que quedar en 0", c, v)
		}
	}
}
