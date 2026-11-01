package kmeans

import "testing"

func TestSubmuestraTomaElTamanoPedido(t *testing.T) {
	d := nubes(250) // 1000 filas
	s, err := Submuestra(d, 100)
	if err != nil {
		t.Fatalf("Submuestra: %v", err)
	}
	if s.N != 100 {
		t.Errorf("N = %d, se esperaban 100", s.N)
	}
	if len(s.X) != 100*d.D {
		t.Errorf("len(X) = %d, se esperaba %d", len(s.X), 100*d.D)
	}
}

func TestSubmuestraEsSistematicaYNoElPrimerBloque(t *testing.T) {
	// Tomar las primeras n filas sesgaría la muestra: el gold está ordenado por
	// estación y fecha-hora. El muestreo sistemático recorre todo con paso fijo.
	d := nubes(250)
	s, err := Submuestra(d, 10)
	if err != nil {
		t.Fatalf("Submuestra: %v", err)
	}
	// Con paso 100, el último punto de la muestra debe venir del final del dataset.
	ultimo := s.Punto(9)
	esperado := d.Punto(900)
	for i := range ultimo {
		if ultimo[i] != esperado[i] {
			t.Fatalf("el último punto de la muestra no es el del índice 900: %v vs %v", ultimo, esperado)
		}
	}
}

func TestSubmuestraEsDeterminista(t *testing.T) {
	d := nubes(100)
	a, err := Submuestra(d, 37)
	if err != nil {
		t.Fatalf("Submuestra: %v", err)
	}
	b, err := Submuestra(d, 37)
	if err != nil {
		t.Fatalf("Submuestra: %v", err)
	}
	for i := range a.X {
		if a.X[i] != b.X[i] {
			t.Fatalf("dos submuestras del mismo tamaño difieren en %d", i)
		}
	}
}

func TestSubmuestraDelTotalDevuelveTodo(t *testing.T) {
	d := nubes(25)
	s, err := Submuestra(d, d.N)
	if err != nil {
		t.Fatalf("Submuestra: %v", err)
	}
	if s.N != d.N {
		t.Fatalf("N = %d, se esperaban %d", s.N, d.N)
	}
	for i := range d.X {
		if s.X[i] != d.X[i] {
			t.Fatalf("la submuestra completa difiere en %d", i)
		}
	}
}

func TestSubmuestraRechazaTamanosImposibles(t *testing.T) {
	d := nubes(10)
	for _, n := range []int{0, -5, d.N + 1} {
		if _, err := Submuestra(d, n); err == nil {
			t.Errorf("se esperaba error con n = %d", n)
		}
	}
}
