package protocolo_test

import (
	"errors"
	"io"
	"math"
	"net"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/R0SEWT/aire-distribuido/protocolo"
)

const huellaGold = "sha256:0f1e2d"

// nodoFalso hace de nodo de cómputo sobre un extremo de net.Pipe: se presenta,
// confirma el CARGAR y responde cada ITERAR con la asignación de Lloyd sobre sus
// filas. Los campos opcionales cambian ese comportamiento para cada prueba.
type nodoFalso struct {
	nombre string
	filas  [][]float64

	// responder, si no es nil, decide qué mandar ante un ITERAR; recibe el
	// PARCIAL honesto ya calculado. Devolver nil es no responder.
	responder func(iterar, honesto protocolo.Mensaje) []protocolo.Mensaje
	// trasResponder, si no es nil, corre después de escribir la respuesta.
	trasResponder func()
	// alCargar, si no es nil, reemplaza el LISTO honesto.
	alCargar func(cargar protocolo.Mensaje) protocolo.Mensaje
}

func (nf *nodoFalso) correr(conn net.Conn) error {
	defer conn.Close()
	cod, dec := protocolo.NuevoCodificador(conn), protocolo.NuevoDecodificador(conn)
	hola := protocolo.Mensaje{Tipo: protocolo.Hola, Version: protocolo.Version, Nodo: nf.nombre, Workers: 2}
	if err := cod.Escribir(hola); err != nil {
		return ignorarCierre(err)
	}
	for {
		m, err := dec.Leer()
		if err != nil {
			return ignorarCierre(err)
		}
		var resp []protocolo.Mensaje
		switch m.Tipo {
		case protocolo.Cargar:
			listo := protocolo.Mensaje{Tipo: protocolo.Listo, N: len(nf.filas), Huella: m.Huella}
			if nf.alCargar != nil {
				listo = nf.alCargar(m)
			}
			resp = []protocolo.Mensaje{listo}
		case protocolo.Iterar:
			honesto := parcialDe(nf.filas, m)
			resp = []protocolo.Mensaje{honesto}
			if nf.responder != nil {
				resp = nf.responder(m, honesto)
			}
		case protocolo.Ping:
			resp = []protocolo.Mensaje{{Tipo: protocolo.Pong}}
		case protocolo.Fin:
			return nil
		}
		for _, r := range resp {
			if err := cod.Escribir(r); err != nil {
				return ignorarCierre(err)
			}
		}
		if m.Tipo == protocolo.Iterar && nf.trasResponder != nil {
			nf.trasResponder()
		}
	}
}

func ignorarCierre(err error) error {
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe) {
		return nil
	}
	return err
}

// parcialDe es lo que un nodo calcula en una ronda: cada fila al centroide más
// cercano (desempate por menor índice), sumas k*D, conteos k e inercia. En la
// PC4 es kmeans.Parciales con el worker pool del nodo.
func parcialDe(filas [][]float64, iterar protocolo.Mensaje) protocolo.Mensaje {
	dim := len(filas[0])
	cent := iterar.Centroides
	k := len(cent) / dim
	p := protocolo.Mensaje{Tipo: protocolo.Parcial, Iter: iterar.Iter,
		Sumas: make([]float64, k*dim), Conteos: make([]int, k)}
	for _, x := range filas {
		mejor, mejorDist := 0, math.Inf(1)
		for j := 0; j < k; j++ {
			dist := 0.0
			for f := 0; f < dim; f++ {
				delta := x[f] - cent[j*dim+f]
				dist += delta * delta
			}
			if dist < mejorDist {
				mejor, mejorDist = j, dist
			}
		}
		for f := 0; f < dim; f++ {
			p.Sumas[mejor*dim+f] += x[f]
		}
		p.Conteos[mejor]++
		p.Inercia += mejorDist
	}
	return p
}

// fusionarEnOrden suma los parciales en el orden del slice, que Ronda garantiza
// que es el orden de nodo. En la PC4 es kmeans.Fusionar.
func fusionarEnOrden(ps []protocolo.Mensaje) (sumas []float64, conteos []int, inercia float64) {
	sumas = make([]float64, len(ps[0].Sumas))
	conteos = make([]int, len(ps[0].Conteos))
	for _, p := range ps {
		for i := range sumas {
			sumas[i] += p.Sumas[i]
		}
		for j := range conteos {
			conteos[j] += p.Conteos[j]
		}
		inercia += p.Inercia
	}
	return sumas, conteos, inercia
}

// montar conecta los nodos falsos a un coordinador por net.Pipe y hace el
// apretón de manos completo (HOLA, CARGAR, LISTO) con rangos contiguos en el
// orden dado. Al terminar la prueba cierra las conexiones y espera a los nodos.
func montar(t *testing.T, nodos ...*nodoFalso) []*protocolo.Enlace {
	t.Helper()
	var wg sync.WaitGroup
	t.Cleanup(wg.Wait) // se registra primero: corre después de los cierres

	limite := time.Now().Add(5 * time.Second)
	enlaces := make([]*protocolo.Enlace, 0, len(nodos))
	desde := 0
	for _, nf := range nodos {
		coord, extremo := net.Pipe()
		t.Cleanup(func() { coord.Close() })
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := nf.correr(extremo); err != nil {
				t.Errorf("nodo %s: %v", nf.nombre, err)
			}
		}()

		e := protocolo.NuevoEnlace(coord)
		if err := protocolo.RecibirHola(e, limite); err != nil {
			t.Fatal(err)
		}
		r := protocolo.Rango{Desde: desde, Hasta: desde + len(nf.filas)}
		if err := protocolo.AsignarRango(e, r, huellaGold, limite); err != nil {
			t.Fatal(err)
		}
		desde = r.Hasta
		enlaces = append(enlaces, e)
	}
	return enlaces
}

// esperarA bloquea hasta que se cierre c, con un tope para no colgar la prueba
// si algo falló antes.
func esperarA(c <-chan struct{}) {
	select {
	case <-c:
	case <-time.After(5 * time.Second):
	}
}

func TestRondaConDosNodosFusionaEnOrdenDeNodo(t *testing.T) {
	filasA := [][]float64{{0, 0}, {1, 0}, {0, 1}, {10, 10}}
	filasB := [][]float64{{11, 10}, {10, 11}, {1, 1}, {11, 11}}

	// b responde primero y a espera a que b termine: el orden de llegada es
	// b, a; el de nodo es a, b.
	turno := make(chan struct{})
	a := &nodoFalso{nombre: "a", filas: filasA,
		responder: func(_, honesto protocolo.Mensaje) []protocolo.Mensaje {
			esperarA(turno)
			return []protocolo.Mensaje{honesto}
		}}
	b := &nodoFalso{nombre: "b", filas: filasB, trasResponder: func() { close(turno) }}
	enlaces := montar(t, a, b)

	if enlaces[0].Nodo != "a" || enlaces[1].Nodo != "b" || enlaces[1].Rango != (protocolo.Rango{Desde: 4, Hasta: 8}) {
		t.Fatalf("apretón de manos: %+v %+v", *enlaces[0], *enlaces[1])
	}

	const k = 2
	cent := []float64{0, 0, 10, 10}
	ps, err := protocolo.Ronda(enlaces, 1, k, cent, time.Now().Add(5*time.Second))
	if err != nil {
		t.Fatal(err)
	}

	// Orden de nodo, no de llegada.
	if !reflect.DeepEqual(ps[0].Conteos, []int{3, 1}) || !reflect.DeepEqual(ps[1].Conteos, []int{1, 3}) {
		t.Fatalf("los parciales no vienen en orden de nodo: a=%v b=%v", ps[0].Conteos, ps[1].Conteos)
	}

	// La fusión de los dos nodos es la iteración de Lloyd sobre todas las filas.
	sumas, conteos, inercia := fusionarEnOrden(ps)
	todo := parcialDe(append(append([][]float64{}, filasA...), filasB...), protocolo.Mensaje{Iter: 1, Centroides: cent})
	if !reflect.DeepEqual(sumas, todo.Sumas) || !reflect.DeepEqual(conteos, todo.Conteos) || inercia != todo.Inercia {
		t.Fatalf("fusión: sumas %v conteos %v inercia %v; una sola máquina da %v %v %v",
			sumas, conteos, inercia, todo.Sumas, todo.Conteos, todo.Inercia)
	}
	nuevos := make([]float64, len(cent))
	for j := 0; j < k; j++ {
		for f := 0; f < 2; f++ {
			nuevos[j*2+f] = sumas[j*2+f] / float64(conteos[j])
		}
	}
	if want := []float64{0.5, 0.5, 10.5, 10.5}; !reflect.DeepEqual(nuevos, want) {
		t.Errorf("centroides nuevos %v, se esperaban %v", nuevos, want)
	}

	for _, e := range enlaces {
		if err := e.Cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Fin}); err != nil {
			t.Errorf("FIN a %s: %v", e.Nodo, err)
		}
	}
}

// Con tres nodos el orden sí cambia el resultado: 1 + 1e100 - 1e100 es 0 si se
// suma en orden de nodo y 1 si se suma en el orden en que llegaron (al revés).
// Ronda devuelve el orden de nodo, así que el número no depende de la red.
func TestFusionEnOrdenDeNodoNoDeLlegada(t *testing.T) {
	turno1, turno2 := make(chan struct{}), make(chan struct{})
	n0 := &nodoFalso{nombre: "n0", filas: [][]float64{{1}},
		responder: func(_, h protocolo.Mensaje) []protocolo.Mensaje { esperarA(turno1); return []protocolo.Mensaje{h} }}
	n1 := &nodoFalso{nombre: "n1", filas: [][]float64{{1e100}},
		responder:     func(_, h protocolo.Mensaje) []protocolo.Mensaje { esperarA(turno2); return []protocolo.Mensaje{h} },
		trasResponder: func() { close(turno1) }}
	n2 := &nodoFalso{nombre: "n2", filas: [][]float64{{-1e100}}, trasResponder: func() { close(turno2) }}
	enlaces := montar(t, n0, n1, n2)

	ps, err := protocolo.Ronda(enlaces, 1, 1, []float64{0}, time.Now().Add(5*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	sumas, _, _ := fusionarEnOrden(ps)
	if sumas[0] != 0 {
		t.Errorf("en orden de nodo (1 + 1e100) - 1e100 = 0, salió %v", sumas[0])
	}
	llegada := (ps[2].Sumas[0] + ps[1].Sumas[0]) + ps[0].Sumas[0]
	if llegada != 1 {
		t.Fatalf("la prueba no discrimina: en orden de llegada debió dar 1, dio %v", llegada)
	}
}

func TestNodoQueNoRespondeVenceElPlazoYDaUnErrorClaro(t *testing.T) {
	sano := &nodoFalso{nombre: "sano", filas: [][]float64{{0}, {1}}}
	mudo := &nodoFalso{nombre: "mudo", filas: [][]float64{{2}, {3}},
		responder: func(_, _ protocolo.Mensaje) []protocolo.Mensaje { return nil }}
	enlaces := montar(t, sano, mudo)

	const plazo = 100 * time.Millisecond
	inicio := time.Now()
	ps, err := protocolo.Ronda(enlaces, 1, 1, []float64{0}, inicio.Add(plazo))
	transcurrido := time.Since(inicio)
	t.Logf("error: %v", err)

	if err == nil || ps != nil {
		t.Fatalf("la ronda debió abortar: ps=%v err=%v", ps, err)
	}
	if !errors.Is(err, protocolo.ErrPlazoVencido) {
		t.Errorf("se esperaba ErrPlazoVencido, salió %v", err)
	}
	var en *protocolo.ErrorNodo
	if !errors.As(err, &en) || en.Nodo != "mudo" {
		t.Errorf("el error tiene que nombrar al nodo mudo, nombra %+v", en)
	}
	if texto := err.Error(); !strings.Contains(texto, "ronda 1 abortada") || strings.Contains(texto, "sano") {
		t.Errorf("error poco claro: %q", texto)
	}
	// Ni antes del plazo ni colgada: la barrera termina o aborta.
	if transcurrido < plazo || transcurrido > 2*time.Second {
		t.Errorf("la ronda volvió a los %v con un plazo de %v", transcurrido, plazo)
	}
}

// El nodo lento no responde la ronda 1, que se aborta; su PARCIAL de la ronda 1
// llega durante la ronda 2 y se descarta por su iter, sin contaminar la fusión.
func TestParcialRezagadoSeDescarta(t *testing.T) {
	rapido := &nodoFalso{nombre: "rapido", filas: [][]float64{{0}, {4}}}
	var pendiente *protocolo.Mensaje
	lento := &nodoFalso{nombre: "lento", filas: [][]float64{{8}, {9}},
		responder: func(iterar, honesto protocolo.Mensaje) []protocolo.Mensaje {
			if iterar.Iter == 1 {
				pendiente = &honesto // la calcula, pero tarde
				return nil
			}
			viejo := *pendiente
			viejo.Sumas = []float64{-999} // lo que traiga, no puede llegar a la fusión
			return []protocolo.Mensaje{viejo, honesto}
		}}
	enlaces := montar(t, rapido, lento)

	_, err := protocolo.Ronda(enlaces, 1, 1, []float64{0}, time.Now().Add(100*time.Millisecond))
	if !errors.Is(err, protocolo.ErrPlazoVencido) {
		t.Fatalf("ronda 1: se esperaba ErrPlazoVencido, salió %v", err)
	}

	ps, err := protocolo.Ronda(enlaces, 2, 1, []float64{5}, time.Now().Add(5*time.Second))
	if err != nil {
		t.Fatalf("ronda 2: %v", err)
	}
	if ps[1].Iter != 2 || !reflect.DeepEqual(ps[1].Sumas, []float64{17}) {
		t.Errorf("del nodo lento se esperaba el PARCIAL de la ronda 2 con suma 17, llegó %+v", ps[1])
	}
	if enlaces[1].Rezagados != 1 || enlaces[0].Rezagados != 0 {
		t.Errorf("rezagados: rapido=%d lento=%d, se esperaba 0 y 1", enlaces[0].Rezagados, enlaces[1].Rezagados)
	}
}

func TestRondaIgnoraUnPongIntercalado(t *testing.T) {
	nf := &nodoFalso{nombre: "n", filas: [][]float64{{1}},
		responder: func(_, h protocolo.Mensaje) []protocolo.Mensaje {
			return []protocolo.Mensaje{{Tipo: protocolo.Pong}, h}
		}}
	enlaces := montar(t, nf)
	ps, err := protocolo.Ronda(enlaces, 1, 1, []float64{0}, time.Now().Add(5*time.Second))
	if err != nil || ps[0].Sumas[0] != 1 {
		t.Fatalf("ps=%v err=%v", ps, err)
	}
}

func TestRondaAbortaSiElNodoRespondeMal(t *testing.T) {
	casos := []struct {
		nombre    string
		responder func(iterar, honesto protocolo.Mensaje) []protocolo.Mensaje
		causa     error
		texto     string
	}{
		{"PARCIAL de una ronda futura", func(_, h protocolo.Mensaje) []protocolo.Mensaje {
			h.Iter = 5
			return []protocolo.Mensaje{h}
		}, protocolo.ErrProtocolo, "no se envió"},
		{"ERROR del nodo", func(_, _ protocolo.Mensaje) []protocolo.Mensaje {
			return []protocolo.Mensaje{{Tipo: protocolo.Error, Mensaje: "sin memoria para la partición"}}
		}, protocolo.ErrInformado, "sin memoria"},
		{"conteos que no suman su rango", func(_, h protocolo.Mensaje) []protocolo.Mensaje {
			h.Conteos = []int{h.Conteos[0] + 1}
			return []protocolo.Mensaje{h}
		}, protocolo.ErrProtocolo, "suman 3"},
		{"otro k", func(_, h protocolo.Mensaje) []protocolo.Mensaje {
			h.Conteos, h.Sumas = []int{2, 0}, []float64{1, 0}
			return []protocolo.Mensaje{h}
		}, protocolo.ErrProtocolo, "k=1"},
		{"un mensaje fuera de lugar", func(_, _ protocolo.Mensaje) []protocolo.Mensaje {
			return []protocolo.Mensaje{{Tipo: protocolo.Listo, N: 2, Huella: huellaGold}}
		}, protocolo.ErrProtocolo, "se esperaba PARCIAL y llegó LISTO"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			nf := &nodoFalso{nombre: "n", filas: [][]float64{{1}, {2}}, responder: c.responder}
			enlaces := montar(t, nf)
			_, err := protocolo.Ronda(enlaces, 1, 1, []float64{0}, time.Now().Add(5*time.Second))
			if !errors.Is(err, c.causa) || !strings.Contains(err.Error(), c.texto) {
				t.Errorf("se esperaba %v con %q, salió %v", c.causa, c.texto, err)
			}
		})
	}
}

func TestRondaValidaSusArgumentosAntesDeEnviar(t *testing.T) {
	enlaces := montar(t, &nodoFalso{nombre: "n", filas: [][]float64{{1}}})
	limite := time.Now().Add(5 * time.Second)
	if _, err := protocolo.Ronda(enlaces, 1, 1, []float64{0}, limite); err != nil {
		t.Fatal(err)
	}
	if _, err := protocolo.Ronda(enlaces, 1, 1, []float64{0}, limite); err == nil || !strings.Contains(err.Error(), "tiene que crecer") {
		t.Errorf("repetir la ronda 1: %v", err)
	}
	if _, err := protocolo.Ronda([]*protocolo.Enlace{enlaces[0], enlaces[0]}, 2, 1, []float64{0}, limite); err == nil {
		t.Error("un mismo nodo dos veces en la ronda debió fallar")
	}
	if _, err := protocolo.Ronda(enlaces, 2, 2, []float64{0, 1, 2}, limite); err == nil {
		t.Error("3 centroides con k=2 debió fallar")
	}
	if _, err := protocolo.Ronda(nil, 2, 1, []float64{0}, limite); err == nil {
		t.Error("una ronda sin nodos debió fallar")
	}
	// Nada de lo rechazado llegó al nodo: la ronda 2 sigue disponible.
	if _, err := protocolo.Ronda(enlaces, 2, 1, []float64{0}, limite); err != nil {
		t.Errorf("ronda 2 después de los rechazos: %v", err)
	}
}

func TestHolaConOtraVersionSeRechazaYSeLeAvisaAlNodo(t *testing.T) {
	coord, extremo := net.Pipe()
	defer coord.Close()
	recibido := make(chan protocolo.Mensaje, 1)
	go func() {
		defer extremo.Close()
		cod, dec := protocolo.NuevoCodificador(extremo), protocolo.NuevoDecodificador(extremo)
		cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Hola, Version: protocolo.Version + 1, Nodo: "colab-viejo"})
		m, _ := dec.Leer()
		recibido <- m
	}()

	err := protocolo.RecibirHola(protocolo.NuevoEnlace(coord), time.Now().Add(5*time.Second))
	if !errors.Is(err, protocolo.ErrProtocolo) || !strings.Contains(err.Error(), "colab-viejo") {
		t.Errorf("se esperaba un rechazo que nombre al nodo, salió %v", err)
	}
	if m := <-recibido; m.Tipo != protocolo.Error || !strings.Contains(m.Mensaje, "actualiza el nodo") {
		t.Errorf("el nodo debió recibir un ERROR explicativo, recibió %+v", m)
	}
}

func TestAsignarRangoVerificaHuellaYFilas(t *testing.T) {
	casos := map[string]func(protocolo.Mensaje) protocolo.Mensaje{
		"otra huella": func(c protocolo.Mensaje) protocolo.Mensaje {
			return protocolo.Mensaje{Tipo: protocolo.Listo, N: c.Rango.Len(), Huella: "sha256:otro"}
		},
		"otras filas": func(c protocolo.Mensaje) protocolo.Mensaje {
			return protocolo.Mensaje{Tipo: protocolo.Listo, N: c.Rango.Len() - 1, Huella: c.Huella}
		},
	}
	for nombre, alCargar := range casos {
		t.Run(nombre, func(t *testing.T) {
			coord, extremo := net.Pipe()
			defer coord.Close()
			nf := &nodoFalso{nombre: "n", filas: [][]float64{{1}}, alCargar: alCargar}
			listo := make(chan error, 1)
			go func() { listo <- nf.correr(extremo) }()

			e := protocolo.NuevoEnlace(coord)
			limite := time.Now().Add(5 * time.Second)
			if err := protocolo.RecibirHola(e, limite); err != nil {
				t.Fatal(err)
			}
			err := protocolo.AsignarRango(e, protocolo.Rango{Desde: 10, Hasta: 20}, huellaGold, limite)
			if !errors.Is(err, protocolo.ErrProtocolo) {
				t.Errorf("se esperaba ErrProtocolo, salió %v", err)
			}
			if e.Rango.Len() != 0 {
				t.Errorf("un rango no confirmado no se registra, quedó %+v", e.Rango)
			}
			coord.Close()
			if err := <-listo; err != nil {
				t.Error(err)
			}
		})
	}
}

func TestLatidoMideLaIdaYVuelta(t *testing.T) {
	enlaces := montar(t, &nodoFalso{nombre: "n", filas: [][]float64{{1}}})
	rtt, err := protocolo.Latido(enlaces[0], time.Now().Add(5*time.Second))
	if err != nil || rtt <= 0 {
		t.Fatalf("rtt=%v err=%v", rtt, err)
	}
}
