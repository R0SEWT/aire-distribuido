package ventana

import (
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/R0SEWT/aire-distribuido/almacen"
)

var t0 = time.Date(2026, 11, 14, 0, 0, 0, 0, time.FixedZone("-05", -5*3600))

func hora(h int) time.Time { return t0.Add(time.Duration(h) * time.Hour) }

func TestConMenosDe18LecturasNoHayMedia(t *testing.T) {
	v := Nuevas()
	var m almacen.Media24h
	for h := 1; h <= 17; h++ {
		m = v.Agregar("CA-A", hora(h), 80, 150)
	}
	if m.Lecturas != 17 || m.PM25 != nil || m.PM10 != nil {
		t.Fatalf("con 17 lecturas: %+v, se esperaban pm2_5 y pm10 null", describir(m))
	}
	if Alerta(m) || INCA(m) != nil {
		t.Fatalf("con 17 lecturas la alerta es false y el INCA null: alerta %v, inca %v", Alerta(m), INCA(m))
	}
	m = v.Agregar("CA-A", hora(18), 80, 150)
	if m.Lecturas != 18 || m.PM25 == nil || *m.PM25 != 80 || *m.PM10 != 150 {
		t.Fatalf("con 18 lecturas: %+v, se esperaba la media 80 y 150", describir(m))
	}
	if !Alerta(m) || *INCA(m) != INCAMala {
		t.Fatalf("media 80 y 150: alerta %v, inca %v; se esperaba alerta y banda mala", Alerta(m), *INCA(m))
	}
}

// La ventana es (t − 24 h, t]: la lectura de hace exactamente 24 h ya no
// entra, la de un segundo después sí.
func TestLaVentanaEsDe24HorasAbiertaAtras(t *testing.T) {
	v := Nuevas()
	v.Agregar("CA-A", t0, 1000, 1000)
	v.Agregar("CA-A", t0.Add(time.Second), 10, 10)
	for h := 2; h <= 24; h++ {
		v.Agregar("CA-A", hora(h), 10, 10)
	}
	m := v.Agregar("CA-A", hora(24), 10, 10) // repetida: reemplaza, no suma
	if m.Lecturas != 24 || *m.PM25 != 10 {
		t.Fatalf("en (t0, t0 + 24 h] hay 24 lecturas de 10: %+v", describir(m))
	}
}

func TestUnaLecturaRepetidaReemplazaALaAnterior(t *testing.T) {
	v := Nuevas()
	for h := 1; h <= 18; h++ {
		v.Agregar("CA-A", hora(h), 10, 20)
	}
	// La misma fecha_hora en otra zona horaria es la misma lectura.
	m := v.Agregar("CA-A", hora(18).UTC(), 46, 56)
	if m.Lecturas != 18 || *m.PM25 != 12 || *m.PM10 != 22 {
		t.Fatalf("tras reemplazar la última: %+v, se esperaba 18 lecturas de media 12 y 22", describir(m))
	}
}

// Las lecturas pueden llegar desordenadas (Postman y la J3 mandan fechas
// históricas): cada una se mide con su propia ventana (t − 24 h, t], y el
// resultado final no depende del orden de llegada.
func TestLecturasDesordenadas(t *testing.T) {
	horas := rand.New(rand.NewSource(3)).Perm(24)
	v := Nuevas()
	for _, h := range horas {
		v.Agregar("CA-A", hora(h+1), float64(h+1), 2*float64(h+1))
	}
	// La ventana de la hora 20 son las horas 1 a 20, aunque ya llegaron 21 a 24.
	m := v.Agregar("CA-A", hora(20), 20, 40)
	if m.Lecturas != 20 || *m.PM25 != 10.5 || *m.PM10 != 21 {
		t.Fatalf("ventana de la hora 20: %+v, se esperaban 20 lecturas de media 10.5 y 21", describir(m))
	}
	ordenadas := Nuevas()
	for h := 1; h <= 24; h++ {
		ordenadas.Agregar("CA-A", hora(h), float64(h), 2*float64(h))
	}
	a, b := v.Agregar("CA-A", hora(24), 24, 48), ordenadas.Agregar("CA-A", hora(24), 24, 48)
	if a.Lecturas != b.Lecturas || *a.PM25 != *b.PM25 || *a.PM10 != *b.PM10 {
		t.Fatalf("desordenadas %+v, ordenadas %+v: tienen que dar lo mismo", describir(a), describir(b))
	}
}

// Lo que queda fuera de la ventana de la lectura más reciente se descarta: una
// lectura más vieja que eso no se guarda y no cambia las ventanas siguientes.
func TestLoViejoSeDescarta(t *testing.T) {
	v := Nuevas()
	for h := 100; h < 124; h++ {
		v.Agregar("CA-A", hora(h), 10, 10)
	}
	if m := v.Agregar("CA-A", hora(10), 500, 500); m.Lecturas != 1 || m.PM25 != nil {
		t.Fatalf("una lectura de hace cuatro días: %+v, su ventana es solo ella", describir(m))
	}
	if m := v.Agregar("CA-A", hora(123), 10, 10); m.Lecturas != 24 || *m.PM25 != 10 {
		t.Fatalf("la lectura vieja no se guardó: %+v", describir(m))
	}
	// Al avanzar 30 horas, la ventana se vacía hasta la nueva lectura.
	if m := v.Agregar("CA-A", hora(153), 10, 10); m.Lecturas != 1 {
		t.Fatalf("30 horas después: %+v", describir(m))
	}
	if n := v.lecturasGuardadas("CA-A"); n != 1 {
		t.Fatalf("quedaron %d lecturas guardadas, se esperaba 1", n)
	}
}

func TestLasEstacionesNoSeMezclan(t *testing.T) {
	v := Nuevas()
	for h := 1; h <= 18; h++ {
		v.Agregar("CA-A", hora(h), 100, 100)
	}
	if m := v.Agregar("CA-B", hora(18), 5, 5); m.Lecturas != 1 {
		t.Fatalf("CA-B: %+v", describir(m))
	}
}

func TestAlertaEsElECADe24Horas(t *testing.T) {
	casos := []struct {
		pm25, pm10 float64
		alerta     bool
	}{
		{50, 100, false}, {50.01, 0, true}, {0, 100.01, true}, {49.99, 99.99, false},
	}
	for _, c := range casos {
		if got := Alerta(media(c.pm25, c.pm10, 18)); got != c.alerta {
			t.Errorf("media %v y %v: alerta %v, se esperaba %v", c.pm25, c.pm10, got, c.alerta)
		}
	}
}

func TestINCACortesExactos(t *testing.T) {
	casos := []struct {
		pm25, pm10 float64
		banda      string
	}{
		{0, 75, INCABuena}, {0, 75.01, INCAModerada}, {0, 150, INCAModerada}, {0, 150.01, INCAMala},
		{0, 250, INCAMala}, {0, 250.01, INCAUmbralDeCuidado},
		{12.5, 0, INCABuena}, {12.51, 0, INCAModerada}, {25, 0, INCAModerada}, {25.01, 0, INCAMala},
		{125, 0, INCAMala}, {125.01, 0, INCAUmbralDeCuidado},
		// La peor de las dos: un PM2.5 de 30 es mala aunque cumpla el ECA (50).
		{30, 40, INCAMala}, {5, 260, INCAUmbralDeCuidado}, {12.5, 75, INCABuena},
	}
	for _, c := range casos {
		got := INCA(media(c.pm25, c.pm10, 24))
		if got == nil || *got != c.banda {
			t.Errorf("media %v y %v: INCA %v, se esperaba %s", c.pm25, c.pm10, describirBanda(got), c.banda)
		}
	}
	// Los mismos cortes con 18 lecturas reales por la ventana.
	for _, c := range []struct {
		pm10  float64
		banda string
	}{{75, INCABuena}, {75.01, INCAModerada}} {
		v := Nuevas()
		var m almacen.Media24h
		for h := 1; h <= 18; h++ {
			m = v.Agregar("CA-A", hora(h), 0, c.pm10)
		}
		if got := INCA(m); got == nil || *got != c.banda {
			t.Errorf("18 lecturas de PM10 = %v: INCA %v, se esperaba %s", c.pm10, describirBanda(got), c.banda)
		}
	}
}

// Con -race: muchos pedidos de la misma estación a la vez. Al final la
// ventana tiene las 24 horas, cada una con su último valor, y la media no
// depende del orden en que llegaron.
func TestConcurrenciaSobreLaMismaEstacion(t *testing.T) {
	v := Nuevas()
	var wg sync.WaitGroup
	for g := range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 200 {
				h := (g*7+i)%24 + 1
				m := v.Agregar("CA-A", hora(h), float64(h), float64(h))
				if m.Lecturas < 1 || m.Lecturas > 24 {
					t.Errorf("lecturas = %d", m.Lecturas)
				}
				v.Agregar("CA-B", hora(h), 1, 1) // otra estación, en paralelo
			}
		}()
	}
	wg.Wait()
	m := v.Agregar("CA-A", hora(24), 24, 24)
	if m.Lecturas != 24 || *m.PM25 != 12.5 {
		t.Fatalf("tras la carga concurrente: %+v, se esperaban 24 lecturas de media 12.5", describir(m))
	}
}

func media(pm25, pm10 float64, n int) almacen.Media24h {
	return almacen.Media24h{PM25: &pm25, PM10: &pm10, Lecturas: n}
}

func describir(m almacen.Media24h) map[string]any {
	d := map[string]any{"lecturas": m.Lecturas, "pm2_5": nil, "pm10": nil}
	if m.PM25 != nil {
		d["pm2_5"] = *m.PM25
	}
	if m.PM10 != nil {
		d["pm10"] = *m.PM10
	}
	return d
}

func describirBanda(b *string) string {
	if b == nil {
		return "null"
	}
	return *b
}

func BenchmarkAgregar(b *testing.B) {
	v := Nuevas()
	h := 0
	for b.Loop() {
		v.Agregar("CA-A", hora(h%48), 20, 40)
		h++
	}
}
