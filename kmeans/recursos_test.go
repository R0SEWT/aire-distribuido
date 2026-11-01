package kmeans

import "testing"

func TestRecursosTraeMemoriaYTiempoDeCPU(t *testing.T) {
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
	if r.MaxRSSMB <= 0 {
		t.Errorf("MaxRSSMB = %v, se esperaba algo positivo", r.MaxRSSMB)
	}
	if r.HeapMB <= 0 {
		t.Errorf("HeapMB = %v, se esperaba algo positivo", r.HeapMB)
	}
	if r.CPUUsuarioS < 0 || r.CPUSistemaS < 0 {
		t.Errorf("tiempos de CPU negativos: %v, %v", r.CPUUsuarioS, r.CPUSistemaS)
	}
	if r.TotalAsignadoMB < r.HeapMB {
		t.Errorf("TotalAsignadoMB (%v) debería ser al menos el heap vivo (%v)", r.TotalAsignadoMB, r.HeapMB)
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
