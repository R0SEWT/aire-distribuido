package kmeans

import "testing"

// seisNubes: seis grupos bien separados en 3 dimensiones, con dispersión.
func seisNubes() *Datos {
	d := &Datos{D: 3}
	centros := [][3]float64{{0, 0, 0}, {20, 0, 0}, {0, 20, 0}, {0, 0, 20}, {20, 20, 0}, {20, 0, 20}}
	for i := 0; i < 400; i++ {
		c := centros[i%len(centros)]
		d.X = append(d.X, c[0]+float64(i%7)*0.3, c[1]+float64(i%5)*0.4, c[2]+float64(i%3)*0.5)
		d.N++
	}
	return d
}

func TestCodoDaInerciasDecrecientesEnK(t *testing.T) {
	d := seisNubes()
	op := OpcionesConc{Opciones: Opciones{MaxIter: 100, TolAbs: 1e-9, TolRel: 1e-9}, Workers: 4, Chunk: 50}
	puntos, err := Codo(d, 1, 8, 300, 42, 1, op)
	if err != nil {
		t.Fatalf("Codo: %v", err)
	}
	if len(puntos) != 8 {
		t.Fatalf("%d puntos, se esperaban 8", len(puntos))
	}
	for i, p := range puntos {
		if p.K != i+1 || p.KEfectivo != p.K || p.N != 300 {
			t.Fatalf("punto %d: %+v", i, p)
		}
		if i > 0 && !(p.Inercia < puntos[i-1].Inercia) {
			t.Errorf("la inercia no bajó de k=%d a k=%d: %v → %v", i, i+1, puntos[i-1].Inercia, p.Inercia)
		}
	}
	// El codo está en el k verdadero: de 5 a 6 la inercia cae mucho más que de 6 a 7.
	caida56 := puntos[4].Inercia / puntos[5].Inercia
	caida67 := puntos[5].Inercia / puntos[6].Inercia
	if caida56 < 5*caida67 {
		t.Errorf("no se ve el codo en k=6: J5/J6 = %.2f, J6/J7 = %.2f", caida56, caida67)
	}
}

func TestCodoEsDeterministaYUsaLaSubmuestra(t *testing.T) {
	d := seisNubes()
	op := OpcionesConc{Opciones: Opciones{MaxIter: 50, TolAbs: 1e-9, TolRel: 1e-9}, Workers: 3, Chunk: 17}
	a, err := Codo(d, 2, 4, 100, 7, 3, op)
	if err != nil {
		t.Fatalf("Codo: %v", err)
	}
	op.Workers = 8
	b, err := Codo(d, 2, 4, 100, 7, 3, op)
	if err != nil {
		t.Fatalf("Codo: %v", err)
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("k=%d: %+v vs %+v", a[i].K, a[i], b[i])
		}
	}
	todo, err := Codo(d, 2, 2, 0, 7, 1, op)
	if err != nil {
		t.Fatalf("Codo: %v", err)
	}
	if todo[0].N != d.N {
		t.Errorf("n = 0 debería usar todas las filas, usó %d", todo[0].N)
	}
}

func TestCodoRechazaRangosInvalidos(t *testing.T) {
	d := seisNubes()
	op := OpcionesConc{Opciones: Opciones{MaxIter: 10}, Workers: 2, Chunk: 10}
	for _, c := range [][3]int{{0, 3, 100}, {4, 3, 100}, {2, 200, 100}} {
		if _, err := Codo(d, c[0], c[1], c[2], 1, 1, op); err == nil {
			t.Errorf("k de %d a %d con n=%d: se esperaba error", c[0], c[1], c[2])
		}
	}
}
