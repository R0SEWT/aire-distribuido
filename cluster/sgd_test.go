package cluster

import (
	"context"
	"errors"
	"math"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/R0SEWT/aire-distribuido/protocolo"
)

func TestPromediarPonderaPorMuestras(t *testing.T) {
	got, err := Promediar([][]float32{{0, 2, -1}, {4, 6, 3}}, []int{1, 3})
	if err != nil {
		t.Fatal(err)
	}
	if want := []float32{3, 5, 2}; !reflect.DeepEqual(got, want) {
		t.Fatalf("promedio %v, se esperaba %v", got, want)
	}
	// Un nodo sin muestras en la ronda no pesa.
	got, err = Promediar([][]float32{{1}, {100}}, []int{2, 0})
	if err != nil || got[0] != 1 {
		t.Fatalf("con muestras 2 y 0: %v, %v", got, err)
	}
}

// Sumar en punto flotante no es asociativo: (1 + 1e20) − 1e20 es 0 en float64
// y 1 + (1e20 − 1e20) es 1. Promediar acumula en orden de nodo, así que el
// resultado depende solo de ese orden, no de cuál LOCAL llegó primero.
func TestPromediarAcumulaEnOrdenDeNodo(t *testing.T) {
	enOrden, err := Promediar([][]float32{{1}, {1e20}, {-1e20}}, []int{1, 1, 1})
	if err != nil {
		t.Fatal(err)
	}
	otroOrden, _ := Promediar([][]float32{{1e20}, {-1e20}, {1}}, []int{1, 1, 1})
	if enOrden[0] != 0 || otroOrden[0] != float32(1.0/3) {
		t.Fatalf("en orden de nodo %v (se esperaba 0) y en otro orden %v (1/3): la prueba no discrimina", enOrden[0], otroOrden[0])
	}
	for range 10 {
		otra, _ := Promediar([][]float32{{1}, {1e20}, {-1e20}}, []int{1, 1, 1})
		if math.Float32bits(otra[0]) != math.Float32bits(enOrden[0]) {
			t.Fatal("el mismo promedio dio otro resultado")
		}
	}
}

func TestPromediarRechazaEntradasQueNoCuadran(t *testing.T) {
	casos := map[string]struct {
		vs []([]float32)
		ms []int
	}{
		"sin vectores":         {nil, nil},
		"largos distintos":     {[][]float32{{1, 2}, {1}}, []int{1, 1}},
		"muestras de menos":    {[][]float32{{1}, {2}}, []int{1}},
		"muestras negativas":   {[][]float32{{1}, {2}}, []int{3, -1}},
		"ninguna muestra":      {[][]float32{{1}, {2}}, []int{0, 0}},
		"vectores sin valores": {[][]float32{{}, {}}, []int{1, 1}},
	}
	for nombre, c := range casos {
		t.Run(nombre, func(t *testing.T) {
			if _, err := Promediar(c.vs, c.ms); err == nil {
				t.Fatal("se esperaba un error")
			}
		})
	}
}

// nodoMapa llama a addr como un nodo del mapa (HOLA v2), confirma su TAREA y
// "entrena" sumando suma a cada peso. Devuelve por el canal nil si recibió
// FIN, el ERROR del coordinador como error, o el error de la conexión.
type nodoMapa struct {
	nombre   string
	suma     float32
	muestras int // en el LISTO y en cada LOCAL
	perdida  float64

	cerrarEn int // ronda en la que cierra la conexión al recibir el PESOS
	mudoEn   int // ronda que no responde
	nanEn    int // ronda en la que devuelve un NaN

	tareas chan protocolo.Mensaje // si no es nil, recibe la TAREA
}

func (nf *nodoMapa) lanzar(t *testing.T, addr string) <-chan error {
	t.Helper()
	fin := make(chan error, 1)
	go func() { fin <- nf.correr(addr) }()
	return fin
}

func (nf *nodoMapa) correr(addr string) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	cod, dec := protocolo.NuevoCodificador(conn), protocolo.NuevoDecodificador(conn)
	hola := protocolo.Mensaje{Tipo: protocolo.Hola, Version: protocolo.Version2, Nodo: nf.nombre, Workers: 1, GPU: "T1000", VramGB: 8}
	if err := cod.Escribir(hola); err != nil {
		return err
	}
	for {
		m, err := dec.Leer()
		if err != nil {
			return err
		}
		switch m.Tipo {
		case protocolo.Tarea:
			if nf.tareas != nil {
				nf.tareas <- m
			}
			err = cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Listo, Tarea: m.Tarea, Muestras: nf.muestras})
		case protocolo.Pesos:
			if m.Ronda == nf.cerrarEn {
				return errors.New("cerré a mitad de la ronda")
			}
			if m.Ronda == nf.mudoEn {
				continue
			}
			v, err := m.Vector()
			if err != nil {
				return err
			}
			for i := range v {
				v[i] += nf.suma
			}
			if m.Ronda == nf.nanEn {
				v[len(v)-1] = float32(math.NaN())
			}
			err = cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Local, Ronda: m.Ronda, Muestras: nf.muestras,
				Perdida: nf.perdida, PerdidaVal: 2 * nf.perdida, MsComputo: 1}.ConVector(v))
			if err != nil {
				return err
			}
		case protocolo.Ping:
			err = cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Pong})
		case protocolo.Fin:
			return nil
		case protocolo.Error:
			return errors.New("ERROR: " + m.Mensaje)
		}
		if err != nil {
			return err
		}
	}
}

// juntarMapa lanza los nodos, los espera con EsperarNodosVersion (v2) y les
// asigna sus días con CargarTarea. Devuelve los enlaces en orden de nodo.
func juntarMapa(t *testing.T, nodos ...*nodoMapa) ([]*protocolo.Enlace, []<-chan error) {
	t.Helper()
	ln := escuchar(t)
	fines := make([]<-chan error, len(nodos))
	for i, nf := range nodos {
		fines[i] = nf.lanzar(t, ln.Addr().String())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ms := EsperarNodosVersion(ctx, ln, len(nodos), protocolo.Version2, time.Second, silencio)
	if len(ms) != len(nodos) {
		t.Fatalf("llegaron %d de %d nodos", len(ms), len(nodos))
	}
	enlaces := Enlaces(ms)
	t.Cleanup(func() { Terminar(enlaces, time.Second) })

	dias, err := RepartirDias(diasSeguidos(7*len(nodos)), len(nodos), 1)
	if err != nil {
		t.Fatal(err)
	}
	tarea := protocolo.Mensaje{Tipo: protocolo.Tarea, Tarea: "mapa-prueba", Manifiesto: "manifiesto.json", Huella: "sha256:ab12"}
	if _, err := CargarTarea(enlaces, tarea, dias, time.Now().Add(5*time.Second)); err != nil {
		t.Fatal(err)
	}
	return enlaces, fines
}

// Cada nodo suma una constante distinta y declara otras muestras: el promedio
// de cada ronda es pesos + Σ mₖ·cₖ / Σ mₖ, exacto con estos números (1·1 +
// 2·2 + 5·3 = 20 y 20/8 = 2.5).
func TestSGDLocalPromediaExactoEnCadaRonda(t *testing.T) {
	tareas := make(chan protocolo.Mensaje, 3)
	nodos := []*nodoMapa{
		{nombre: "pc-03", suma: 3, muestras: 5, perdida: 0.5, tareas: tareas},
		{nombre: "pc-01", suma: 1, muestras: 1, perdida: 1, tareas: tareas},
		{nombre: "pc-02", suma: 2, muestras: 2, perdida: 0.25, tareas: tareas},
	}
	enlaces, fines := juntarMapa(t, nodos...)
	if enlaces[0].Nodo != "pc-01" || enlaces[2].Muestras != 5 || enlaces[1].GPU != "T1000" {
		t.Fatalf("enlaces: %+v %+v %+v", *enlaces[0], *enlaces[1], *enlaces[2])
	}
	var vistos []string
	for range 3 {
		m := <-tareas
		if len(m.Dias) != 7 || m.Tarea != "mapa-prueba" {
			t.Fatalf("TAREA con %d días: %+v", len(m.Dias), m)
		}
		vistos = append(vistos, m.Dias...)
	}
	if len(vistos) != 21 {
		t.Fatalf("los tres nodos recibieron %d días, se esperaban 21", len(vistos))
	}

	pesos0 := []float32{0.5, -1, 1024, 0}
	var rondas []RondaSGD
	final, err := SGDLocal(enlaces, pesos0, OpcionesSGD{Rondas: 3, Pasos: 50, Plazo: 5 * time.Second}, func(r *RondaSGD) error {
		rondas = append(rondas, *r)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for i, r := range rondas {
		desplazamiento := 2.5 * float32(i+1)
		want := []float32{0.5 + desplazamiento, -1 + desplazamiento, 1024 + desplazamiento, desplazamiento}
		if r.Ronda != i+1 || !reflect.DeepEqual(r.Pesos, want) {
			t.Fatalf("ronda %d: pesos %v, se esperaban %v", r.Ronda, r.Pesos, want)
		}
		// Pérdidas ponderadas en orden de nodo: (1·1 + 2·0.25 + 5·0.5)/8 = 0.5.
		if r.Muestras != 8 || r.Perdida != 0.5 || r.PerdidaVal != 1 {
			t.Errorf("ronda %d: muestras %d, pérdida %v, validación %v; se esperaban 8, 0.5 y 1", r.Ronda, r.Muestras, r.Perdida, r.PerdidaVal)
		}
		if r.Bytes != 3*2*4*int64(len(pesos0)) || len(r.Locales) != 3 || r.Locales[0].Muestras != 1 || r.Locales[2].Computo != time.Millisecond {
			t.Errorf("ronda %d: bytes %d, locales %+v", r.Ronda, r.Bytes, r.Locales)
		}
		if r.Barrera <= 0 || r.Locales[1].Total <= 0 || r.Locales[1].Envio <= 0 || r.Locales[1].Pesos != nil {
			t.Errorf("ronda %d: tiempos sin medir o vectores locales retenidos: %+v", r.Ronda, r.Locales[1])
		}
	}
	if len(rondas) != 3 || !reflect.DeepEqual(final, rondas[2].Pesos) {
		t.Fatalf("%d rondas; final %v", len(rondas), final)
	}
	if !reflect.DeepEqual(pesos0, []float32{0.5, -1, 1024, 0}) {
		t.Errorf("SGDLocal modificó los pesos iniciales: %v", pesos0)
	}
	if err := Terminar(enlaces, time.Second); err != nil {
		t.Fatal(err)
	}
	for i, fin := range fines {
		if err := <-fin; err != nil {
			t.Errorf("nodo %d: %v", i, err)
		}
	}
}

func TestUnNodoQueCierraAMitadDeRondaAbortaYLoNombra(t *testing.T) {
	enlaces, fines := juntarMapa(t,
		&nodoMapa{nombre: "pc-01", suma: 1, muestras: 1},
		&nodoMapa{nombre: "pc-02", suma: 2, muestras: 1, cerrarEn: 2})
	var completas []int
	_, err := SGDLocal(enlaces, []float32{0, 0}, OpcionesSGD{Rondas: 4, Pasos: 10, Plazo: 5 * time.Second}, func(r *RondaSGD) error {
		completas = append(completas, r.Ronda)
		return nil
	})
	t.Logf("error: %v", err)
	var en *protocolo.ErrorNodo
	if err == nil || !errors.As(err, &en) || en.Nodo != "pc-02" || !strings.Contains(err.Error(), "ronda 2 abortada") {
		t.Fatalf("se esperaba un error de la ronda 2 que nombre a pc-02; salió %v", err)
	}
	if strings.Contains(err.Error(), "pc-01") {
		t.Errorf("el error nombra a un nodo sano: %v", err)
	}
	if !reflect.DeepEqual(completas, []int{1}) {
		t.Errorf("rondas completas %v; solo la 1 tenía que llegar al punto de control", completas)
	}
	Abortar(enlaces, err.Error(), time.Second)
	if err := <-fines[0]; err == nil || !strings.Contains(err.Error(), "pc-02") {
		t.Errorf("el nodo sano tenía que recibir un ERROR que nombre a pc-02; terminó con %v", err)
	}
}

func TestUnNodoMudoVenceElPlazoDeLaRonda(t *testing.T) {
	enlaces, _ := juntarMapa(t,
		&nodoMapa{nombre: "pc-01", suma: 1, muestras: 1},
		&nodoMapa{nombre: "pc-02", suma: 1, muestras: 1, mudoEn: 1})
	const plazo = 150 * time.Millisecond
	inicio := time.Now()
	_, err := RondaLocal(enlaces, 1, 10, []float32{1}, inicio.Add(plazo))
	transcurrido := time.Since(inicio)
	t.Logf("error: %v", err)
	var en *protocolo.ErrorNodo
	if !errors.Is(err, protocolo.ErrPlazoVencido) || !errors.As(err, &en) || en.Nodo != "pc-02" || !strings.Contains(err.Error(), "dentro del plazo") {
		t.Fatalf("se esperaba un plazo vencido que nombre a pc-02; salió %v", err)
	}
	if transcurrido < plazo || transcurrido > 3*time.Second {
		t.Errorf("la ronda volvió a los %v con un plazo de %v", transcurrido, plazo)
	}
}

func TestPesosNoFinitosAbortanLaRonda(t *testing.T) {
	enlaces, _ := juntarMapa(t,
		&nodoMapa{nombre: "pc-01", suma: 1, muestras: 1, nanEn: 1},
		&nodoMapa{nombre: "pc-02", suma: 1, muestras: 1})
	_, err := RondaLocal(enlaces, 1, 10, []float32{1, 2, 3}, time.Now().Add(5*time.Second))
	t.Logf("error: %v", err)
	var en *protocolo.ErrorNodo
	if err == nil || !errors.As(err, &en) || en.Nodo != "pc-01" || !strings.Contains(err.Error(), "posición 2") {
		t.Fatalf("se esperaba un error que nombre a pc-01 y la posición del NaN; salió %v", err)
	}
}

func TestRondaLocalValidaAntesDeEnviar(t *testing.T) {
	enlaces, _ := juntarMapa(t, &nodoMapa{nombre: "pc-01", suma: 1, muestras: 1})
	limite := time.Now().Add(5 * time.Second)
	if _, err := RondaLocal(nil, 1, 10, []float32{1}, limite); err == nil {
		t.Error("una ronda sin nodos debió fallar")
	}
	if _, err := RondaLocal(enlaces, 1, 0, []float32{1}, limite); err == nil {
		t.Error("una ronda sin pasos debió fallar")
	}
	if _, err := RondaLocal(enlaces, 1, 10, nil, limite); err == nil {
		t.Error("una ronda sin pesos debió fallar")
	}
	if _, err := RondaLocal([]*protocolo.Enlace{enlaces[0], enlaces[0]}, 1, 10, []float32{1}, limite); err == nil {
		t.Error("un mismo nodo dos veces debió fallar")
	}
	if _, err := RondaLocal(enlaces, 1, 10, []float32{1}, time.Now().Add(-time.Second)); !errors.Is(err, protocolo.ErrPlazoVencido) {
		t.Errorf("límite vencido: se esperaba ErrPlazoVencido, salió %v", err)
	}
	sinTarea := protocolo.NuevoEnlace(nil)
	sinTarea.Nodo = "pc-sin-tarea"
	if _, err := RondaLocal([]*protocolo.Enlace{sinTarea}, 1, 10, []float32{1}, limite); err == nil || !strings.Contains(err.Error(), "TAREA") {
		t.Errorf("un nodo sin tarea confirmada debió fallar; salió %v", err)
	}
	// Nada de lo rechazado salió: la ronda 1 sigue disponible.
	ls, err := RondaLocal(enlaces, 1, 10, []float32{1}, limite)
	if err != nil || ls[0].Pesos[0] != 2 {
		t.Fatalf("ronda 1 después de los rechazos: %+v, %v", ls, err)
	}
	if _, err := RondaLocal(enlaces, 1, 10, []float32{1}, limite); err == nil || !strings.Contains(err.Error(), "crecer") {
		t.Errorf("repetir la ronda 1: %v", err)
	}
}

func TestEsperarNodosVersionRechazaALosDeLaOtraVersion(t *testing.T) {
	ln := escuchar(t)
	viejo := nodoFalso(t, ln.Addr().String(), "pc-kmeans", protocolo.Version)
	(&nodoMapa{nombre: "pc-mapa", muestras: 1}).lanzar(t, ln.Addr().String())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ms := EsperarNodosVersion(ctx, ln, 1, protocolo.Version2, time.Second, silencio)
	if got := nombres(ms); got != "pc-mapa" || ms[0].Enlace.GPU != "T1000" {
		t.Fatalf("nodos = %s", got)
	}
	if err := <-viejo; err == nil || !strings.Contains(err.Error(), "versión") {
		t.Fatalf("el nodo del K-Means tenía que recibir un ERROR que nombre la versión; recibió %v", err)
	}
	Terminar(Enlaces(ms), time.Second)
}

// Con un vector de varios MB por nodo, como los de la red del mapa a escala
// chica: tres nodos sobre TCP, la carga de ida y vuelta y el promedio exacto.
func TestSGDLocalConUnVectorDeVariosMB(t *testing.T) {
	if testing.Short() {
		t.Skip("mueve ~50 MB por TCP")
	}
	enlaces, _ := juntarMapa(t,
		&nodoMapa{nombre: "pc-01", suma: 1, muestras: 1},
		&nodoMapa{nombre: "pc-02", suma: 2, muestras: 2},
		&nodoMapa{nombre: "pc-03", suma: 3, muestras: 5})
	const n = 1 << 20 // 4 MB de float32 por mensaje
	pesos0 := make([]float32, n)
	for i := range pesos0 {
		pesos0[i] = float32(i%4096) / 8 // múltiplos de 1/8: las sumas son exactas
	}
	final, err := SGDLocal(enlaces, pesos0, OpcionesSGD{Rondas: 2, Pasos: 1, Plazo: 30 * time.Second}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i, w := range pesos0 {
		if final[i] != w+5 {
			t.Fatalf("posición %d: %v, se esperaba %v", i, final[i], w+5)
		}
	}
}
