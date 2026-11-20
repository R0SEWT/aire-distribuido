package cluster

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/R0SEWT/aire-distribuido/protocolo"
)

// nodoFalso llama a addr, se presenta y atiende PING y FIN. Devuelve por fin
// nil si recibió FIN, el ERROR del coordinador como error, o el error de la
// conexión si se cortó sin ninguno de los dos.
func nodoFalso(t *testing.T, addr, nombre string, version int) <-chan error {
	t.Helper()
	fin := make(chan error, 1)
	go func() {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			fin <- err
			return
		}
		defer conn.Close()
		cod, dec := protocolo.NuevoCodificador(conn), protocolo.NuevoDecodificador(conn)
		if err := cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Hola, Version: version, Nodo: nombre, Workers: 4}); err != nil {
			fin <- err
			return
		}
		for {
			m, err := dec.Leer()
			if err != nil {
				fin <- err
				return
			}
			switch m.Tipo {
			case protocolo.Ping:
				cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Pong})
			case protocolo.Fin:
				fin <- nil
				return
			case protocolo.Error:
				fin <- errors.New("ERROR: " + m.Mensaje)
				return
			}
		}
	}()
	return fin
}

func escuchar(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	return ln
}

func nombres(ms []*Miembro) string {
	var s []string
	for _, m := range ms {
		s = append(s, m.Enlace.Nodo)
	}
	return strings.Join(s, ",")
}

func TestEsperarNodosJuntaLosNNodosOrdenadosPorNombre(t *testing.T) {
	ln := escuchar(t)
	for _, n := range []string{"pc-03", "pc-01", "pc-02"} {
		nodoFalso(t, ln.Addr().String(), n, protocolo.Version)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ms := EsperarNodos(ctx, ln, 3, time.Second, silencio)
	if got := nombres(ms); got != "pc-01,pc-02,pc-03" {
		t.Fatalf("nodos = %s", got)
	}
	if ms[0].Enlace.Workers != 4 || ms[0].Direccion == "" {
		t.Fatalf("faltan datos del HOLA: %+v", ms[0])
	}
	Terminar(Enlaces(ms), time.Second)
}

func TestEsperarNodosVenceElPlazoConLosQueLlegaron(t *testing.T) {
	ln := escuchar(t)
	nodoFalso(t, ln.Addr().String(), "pc-01", protocolo.Version)
	nodoFalso(t, ln.Addr().String(), "pc-02", protocolo.Version)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	ms := EsperarNodos(ctx, ln, 3, time.Second, silencio)
	if len(ms) != 2 {
		t.Fatalf("se esperaban los 2 que llegaron, hay %d (%s)", len(ms), nombres(ms))
	}
	Terminar(Enlaces(ms), time.Second)
}

func TestEsperarNodosRechazaOtraVersionYLeAvisa(t *testing.T) {
	ln := escuchar(t)
	viejo := nodoFalso(t, ln.Addr().String(), "pc-viejo", protocolo.Version+1)
	// pc-01 llama cuando pc-viejo ya tiene su respuesta: si llamaran a la vez,
	// la espera podría completarse con pc-01 antes de aceptar a pc-viejo, y
	// eso es otro caso (TestEsperarNodosAtiendeUnHolaQueLlegaAlTerminar).
	rechazo := make(chan error, 1)
	go func() {
		rechazo <- <-viejo
		nodoFalso(t, ln.Addr().String(), "pc-01", protocolo.Version)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ms := EsperarNodos(ctx, ln, 1, time.Second, silencio)
	if got := nombres(ms); got != "pc-01" {
		t.Fatalf("nodos = %s", got)
	}
	if err := <-rechazo; err == nil || !strings.Contains(err.Error(), "versión") {
		t.Fatalf("el nodo con otra versión tenía que recibir un ERROR que la nombre; recibió %v", err)
	}
	Terminar(Enlaces(ms), time.Second)
}

// avisador es la salida de un log que avisa, cerrando visto, la primera vez
// que se escribe una línea con marca.
type avisador struct {
	marca string
	visto chan struct{}
	una   sync.Once
}

func (a *avisador) Write(p []byte) (int, error) {
	if strings.Contains(string(p), a.marca) {
		a.una.Do(func() { close(a.visto) })
	}
	return len(p), nil
}

func TestEsperarNodosAtiendeUnHolaQueLlegaAlTerminar(t *testing.T) {
	// Una conexión ya aceptada que manda su HOLA justo cuando la espera se
	// completó con otros nodos tiene un momento para terminar de presentarse:
	// si es de otra versión, se entera por el ERROR y no por un EOF.
	ln := escuchar(t)
	// tardio conecta antes que pc-01, así que se acepta antes; no dice nada
	// hasta que el log anuncia que el grupo está completo.
	tardio, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer tardio.Close()
	completo := &avisador{marca: "[1/1]", visto: make(chan struct{})}
	respuesta := make(chan error, 1)
	go func() {
		<-completo.visto
		cod, dec := protocolo.NuevoCodificador(tardio), protocolo.NuevoDecodificador(tardio)
		if err := cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Hola, Version: protocolo.Version + 1, Nodo: "pc-viejo", Workers: 1}); err != nil {
			respuesta <- err
			return
		}
		m, err := dec.LeerConPlazo(5 * time.Second)
		if err == nil && m.Tipo != protocolo.Error {
			err = errors.New("llegó " + string(m.Tipo))
		}
		if err == nil {
			err = errors.New("ERROR: " + m.Mensaje)
		}
		respuesta <- err
	}()
	nodoFalso(t, ln.Addr().String(), "pc-01", protocolo.Version)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Con un minuto para el HOLA, el saludo de tardio no termina por su plazo.
	ms := EsperarNodos(ctx, ln, 1, time.Minute, log.New(completo, "", 0))
	sinGoroutinesDeReunir(t)
	if got := nombres(ms); got != "pc-01" {
		t.Fatalf("nodos = %q", got)
	}
	if err := <-respuesta; !strings.Contains(err.Error(), "ERROR: ") || !strings.Contains(err.Error(), "versión") {
		t.Fatalf("el nodo de otra versión tenía que recibir un ERROR que la nombre; recibió %v", err)
	}
	Terminar(Enlaces(ms), time.Second)
}

// nodoQueMuere llama a addr, se presenta y corta la conexión sin esperar nada:
// un equipo que se apagó después del HOLA.
func nodoQueMuere(t *testing.T, addr, nombre string) {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	hola := protocolo.Mensaje{Tipo: protocolo.Hola, Version: protocolo.Version, Nodo: nombre, Workers: 4}
	if err := protocolo.NuevoCodificador(conn).Escribir(hola); err != nil {
		t.Fatal(err)
	}
}

func TestEsperarNodosSueltaAlNodoQueMurioDespuesDelHola(t *testing.T) {
	// Se esperan 2. pc-a se presenta y muere; después llegan pc-b y pc-c, vivos.
	// pc-a no puede ocupar un cupo: el grupo es pc-b y pc-c.
	ln := escuchar(t)
	addr := ln.Addr().String()
	llegoA := &avisador{marca: "] pc-a ", visto: make(chan struct{})}
	llegoB := &avisador{marca: "] pc-b ", visto: make(chan struct{})}
	fines := make(chan (<-chan error), 2)
	go func() {
		// En este orden: pc-a tiene que estar en el grupo cuando llega pc-b.
		nodoQueMuere(t, addr, "pc-a")
		<-llegoA.visto
		fines <- nodoFalso(t, addr, "pc-b", protocolo.Version)
		<-llegoB.visto
		fines <- nodoFalso(t, addr, "pc-c", protocolo.Version)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ms := EsperarNodos(ctx, ln, 2, time.Second, log.New(io.MultiWriter(llegoA, llegoB), "", 0))
	if got := nombres(ms); got != "pc-b,pc-c" {
		t.Fatalf("nodos = %q; pc-a murió después del HOLA y pc-c estaba disponible", got)
	}
	sinGoroutinesDeReunir(t)
	Terminar(Enlaces(ms), time.Second)
	for range 2 {
		if err := <-<-fines; err != nil {
			t.Fatalf("pc-b y pc-c tenían que recibir FIN; uno recibió %v", err)
		}
	}
}

func TestEsperarNodosSueltaAlNodoQueNoRespondeElPing(t *testing.T) {
	// Un nodo colgado no cierra la conexión: se nota porque no contesta el PING
	// dentro del plazo. Sin nadie que lo reemplace, la espera vence sin él.
	ln := escuchar(t)
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	hola := protocolo.Mensaje{Tipo: protocolo.Hola, Version: protocolo.Version, Nodo: "pc-colgado", Workers: 4}
	if err := protocolo.NuevoCodificador(conn).Escribir(hola); err != nil {
		t.Fatal(err)
	}
	nodoFalso(t, ln.Addr().String(), "pc-01", protocolo.Version)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	ms := EsperarNodos(ctx, ln, 2, 100*time.Millisecond, silencio)
	if got := nombres(ms); got != "pc-01" {
		t.Fatalf("nodos = %q; pc-colgado no respondió el PING y no puede quedar en el grupo", got)
	}
	Terminar(Enlaces(ms), time.Second)
}

func TestEsperarNodosUnaReconexionReemplazaALaConexionVieja(t *testing.T) {
	ln := escuchar(t)
	primera := nodoFalso(t, ln.Addr().String(), "pc-01", protocolo.Version)
	segunda := nodoFalso(t, ln.Addr().String(), "pc-01", protocolo.Version)
	nodoFalso(t, ln.Addr().String(), "pc-02", protocolo.Version)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ms := EsperarNodos(ctx, ln, 2, time.Second, silencio)
	if got := nombres(ms); got != "pc-01,pc-02" {
		t.Fatalf("nodos = %s", got)
	}
	Terminar(Enlaces(ms), time.Second)
	// Si la espera se completó antes de aceptar la segunda conexión de pc-01,
	// quedó en la cola del listener: se corta al cerrarlo, como hacen los comandos.
	ln.Close()
	// Una de las dos conexiones de pc-01 se cerró sin FIN; la otra recibió el FIN.
	a, b := <-primera, <-segunda
	if (a == nil) == (b == nil) {
		t.Fatalf("se esperaba un FIN y un cierre; llegaron %v y %v", a, b)
	}
}

// goroutinesDeReunir devuelve las pilas de las goroutines que están en
// reunir.go o nacieron ahí: el aceptador y los saludos de EsperarNodos.
func goroutinesDeReunir() []string {
	buf := make([]byte, 1<<20)
	buf = buf[:runtime.Stack(buf, true)]
	var out []string
	for _, g := range strings.Split(string(buf), "\n\n") {
		if strings.Contains(g, "cluster/reunir.go") {
			out = append(out, g)
		}
	}
	return out
}

// sinGoroutinesDeReunir falla si queda viva alguna goroutine de EsperarNodos.
// Espera un momento: una goroutine que ya avisó que terminó puede tardar un
// instante en desaparecer de las pilas.
func sinGoroutinesDeReunir(t *testing.T) {
	t.Helper()
	var quedan []string
	for limite := time.Now().Add(time.Second); time.Now().Before(limite); time.Sleep(5 * time.Millisecond) {
		if quedan = goroutinesDeReunir(); len(quedan) == 0 {
			return
		}
	}
	t.Fatalf("EsperarNodos volvió y dejó %d goroutines vivas:\n%s", len(quedan), strings.Join(quedan, "\n\n"))
}

func TestEsperarNodosVariasVecesSobreElMismoListener(t *testing.T) {
	// Un pool de nodos reutiliza el listener: cada espera tiene que recibir a
	// todos sus nodos. Si el aceptador de la espera anterior siguiera en
	// Accept, se quedaría con la primera conexión de la siguiente y la cerraría.
	ln := escuchar(t)
	for vuelta := 1; vuelta <= 3; vuelta++ {
		var fines []<-chan error
		for _, n := range []string{"pc-01", "pc-02", "pc-03"} {
			fines = append(fines, nodoFalso(t, ln.Addr().String(), n, protocolo.Version))
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		ms := EsperarNodos(ctx, ln, 3, time.Second, silencio)
		cancel()
		if got := nombres(ms); got != "pc-01,pc-02,pc-03" {
			t.Fatalf("vuelta %d: nodos = %q", vuelta, got)
		}
		Terminar(Enlaces(ms), time.Second)
		for i, fin := range fines {
			if err := <-fin; err != nil {
				t.Fatalf("vuelta %d: el nodo %d tenía que recibir FIN; recibió %v", vuelta, i+1, err)
			}
		}
	}
}

func TestEsperarNodosNoDejaGoroutinesAlVolver(t *testing.T) {
	// callado abre una conexión y no manda el HOLA: su saludo queda en curso.
	callado := func(t *testing.T, ln net.Listener) net.Conn {
		t.Helper()
		conn, err := net.Dial("tcp", ln.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { conn.Close() })
		return conn
	}

	t.Run("con el grupo completo", func(t *testing.T) {
		ln := escuchar(t)
		nodoFalso(t, ln.Addr().String(), "pc-01", protocolo.Version)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		ms := EsperarNodos(ctx, ln, 1, time.Second, silencio)
		sinGoroutinesDeReunir(t)
		if len(ms) != 1 {
			t.Fatalf("llegaron %d nodos de 1", len(ms))
		}
		Terminar(Enlaces(ms), time.Second)
	})
	t.Run("con el plazo vencido y un saludo a medias", func(t *testing.T) {
		ln := escuchar(t)
		conn := callado(t, ln)
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		// Con un minuto para el HOLA, el saludo no termina solo.
		if ms := EsperarNodos(ctx, ln, 1, time.Minute, silencio); len(ms) != 0 {
			t.Fatalf("no se presentó nadie y hay %d nodos", len(ms))
		}
		sinGoroutinesDeReunir(t)
		// A quien no llegó a presentarse se le cierra la conexión.
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		if _, err := conn.Read(make([]byte, 1)); err == nil || errors.Is(err, os.ErrDeadlineExceeded) {
			t.Fatalf("la conexión sin HOLA tenía que cerrarse al terminar la espera; Read devolvió %v", err)
		}
	})
	t.Run("el listener queda como estaba", func(t *testing.T) {
		// La espera no deja un plazo puesto en el listener: quien llama puede
		// seguir aceptando por su cuenta.
		ln := escuchar(t)
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		EsperarNodos(ctx, ln, 1, time.Second, silencio)
		sinGoroutinesDeReunir(t)
		conn := callado(t, ln)
		aceptada, err := ln.Accept()
		if err != nil {
			t.Fatalf("Accept después de la espera: %v", err)
		}
		aceptada.Close()
		conn.Close()
	})
}

// soloListener tapa SetDeadline: es un net.Listener cualquiera, como el de
// crypto/tls o uno de prueba.
type soloListener struct{ net.Listener }

func TestEsperarNodosConUnListenerSinPlazos(t *testing.T) {
	// Sin SetDeadline no hay cómo despertar al aceptador: la espera funciona
	// igual y vuelve, y el aceptador termina cuando se cierra el listener.
	ln := soloListener{escuchar(t)}
	nodoFalso(t, ln.Addr().String(), "pc-02", protocolo.Version)
	nodoFalso(t, ln.Addr().String(), "pc-01", protocolo.Version)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ms := EsperarNodos(ctx, ln, 2, time.Second, silencio)
	if got := nombres(ms); got != "pc-01,pc-02" {
		t.Fatalf("nodos = %q", got)
	}
	Terminar(Enlaces(ms), time.Second)
	ln.Close()
	sinGoroutinesDeReunir(t)
}

func TestAbortarLeAvisaACadaNodoPorQue(t *testing.T) {
	ln := escuchar(t)
	fin := nodoFalso(t, ln.Addr().String(), "pc-01", protocolo.Version)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ms := EsperarNodos(ctx, ln, 1, time.Second, silencio)

	Abortar(Enlaces(ms), "ronda 3 abortada: nodo \"pc-02\": plazo vencido", time.Second)
	if err := <-fin; err == nil || !strings.Contains(err.Error(), "pc-02") {
		t.Fatalf("el nodo tenía que recibir un ERROR con el motivo; recibió %v", err)
	}
}

func TestPercentilPorRangoMasCercano(t *testing.T) {
	xs := []time.Duration{5, 1, 4, 2, 3, 10, 9, 8, 7, 6}
	for _, c := range []struct {
		p    float64
		want time.Duration
	}{{50, 5}, {95, 10}, {100, 10}, {10, 1}, {0, 1}} {
		if got := Percentil(xs, c.p); got != c.want {
			t.Errorf("Percentil(%v) = %v, se esperaba %v", c.p, got, c.want)
		}
	}
	if got := Percentil(nil, 50); got != 0 {
		t.Errorf("percentil de nada = %v", got)
	}
}
