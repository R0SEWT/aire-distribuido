package cluster

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/R0SEWT/aire-distribuido/protocolo"
)

// poolDePrueba es un Pool corriendo sobre loopback con latidos rápidos y sus
// eventos a la vista.
type poolDePrueba struct {
	*Pool
	addr    string
	parar   context.CancelFunc
	termino chan struct{}

	mu      sync.Mutex
	eventos []EventoPool
}

func levantarPool(t *testing.T, cadaLatido time.Duration) *poolDePrueba {
	t.Helper()
	ln := escuchar(t)
	pp := &poolDePrueba{addr: ln.Addr().String(), termino: make(chan struct{})}
	pp.Pool = NuevoPool(ln, OpcionesPool{PlazoHola: time.Second, CadaLatido: cadaLatido, PlazoLatido: 200 * time.Millisecond,
		Log: silencio, AlEvento: func(e EventoPool) {
			pp.mu.Lock()
			pp.eventos = append(pp.eventos, e)
			pp.mu.Unlock()
		}})
	ctx, cancel := context.WithCancel(context.Background())
	pp.parar = cancel
	go func() { pp.Correr(ctx); close(pp.termino) }()
	t.Cleanup(func() { cancel(); <-pp.termino })
	return pp
}

func (pp *poolDePrueba) vistos() string {
	pp.mu.Lock()
	defer pp.mu.Unlock()
	var s []string
	for _, e := range pp.eventos {
		s = append(s, e.Tipo+":"+e.Nodo)
	}
	return strings.Join(s, " ")
}

// hasta espera a que se cumpla la condición; si no, falla con lo que hay.
func hasta(t *testing.T, que string, cond func() bool) {
	t.Helper()
	limite := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(limite) {
			t.Fatalf("no pasó a tiempo: %s", que)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func (pp *poolDePrueba) esperarNodos(t *testing.T, n int) {
	t.Helper()
	hasta(t, fmt.Sprintf("%d nodos en el pool (hay %d; eventos: %s)", n, pp.Cantidad(), pp.vistos()), func() bool { return pp.Cantidad() == n })
}

func TestPoolRecibeNodosYLosListaPorNombre(t *testing.T) {
	pp := levantarPool(t, time.Hour)
	for _, nombre := range []string{"pc-b", "pc-a", "pc-c"} {
		nodoFalso(t, pp.addr, nombre, protocolo.Version)
	}
	pp.esperarNodos(t, 3)

	nodos := pp.Nodos()
	for i, want := range []string{"pc-a", "pc-b", "pc-c"} {
		n := nodos[i]
		if n.Nombre != want || !n.Libre || n.Workers != 4 || n.Latencia <= 0 {
			t.Errorf("nodo %d = %+v; se esperaba %s libre, con 4 workers y latencia medida", i, n, want)
		}
		if host, _, err := net.SplitHostPort(n.Direccion); err != nil || host != "127.0.0.1" {
			t.Errorf("dirección de %s = %q", n.Nombre, n.Direccion)
		}
	}
	if got := pp.vistos(); strings.Count(got, "alta:") != 3 || strings.Contains(got, "baja:") {
		t.Errorf("eventos = %q; se esperaban tres altas", got)
	}
}

func TestPoolTomarSeLlevaLosPrimerosPorNombreYSoltarLosSaca(t *testing.T) {
	pp := levantarPool(t, time.Hour)
	fines := map[string]<-chan error{}
	for _, nombre := range []string{"pc-c", "pc-a", "pc-b"} {
		fines[nombre] = nodoFalso(t, pp.addr, nombre, protocolo.Version)
	}
	pp.esperarNodos(t, 3)

	ms, err := pp.Tomar(2)
	if err != nil || nombres(ms) != "pc-a,pc-b" {
		t.Fatalf("Tomar(2) = %q, %v; se esperaban pc-a y pc-b, en orden", nombres(ms), err)
	}
	libres := map[string]bool{}
	for _, n := range pp.Nodos() {
		libres[n.Nombre] = n.Libre
	}
	if len(libres) != 3 || libres["pc-a"] || libres["pc-b"] || !libres["pc-c"] {
		t.Fatalf("durante la corrida: libres = %v", libres)
	}

	// Con uno libre, pedir dos no se lleva ninguno.
	var faltan *ErrorFaltanNodos
	if _, err := pp.Tomar(2); !errors.As(err, &faltan) || faltan.Libres != 1 || faltan.Pedidos != 2 ||
		err.Error() != "hay 1 nodo libre en el pool y se pidieron 2" {
		t.Fatalf("Tomar(2) con un nodo libre = %v", err)
	}
	if pp.Cantidad() != 3 {
		t.Fatalf("un Tomar que falla no cambia el pool: hay %d", pp.Cantidad())
	}

	// La corrida termina: FIN, se cierran las conexiones y se sueltan.
	if err := Terminar(Enlaces(ms), time.Second); err != nil {
		t.Fatal(err)
	}
	pp.Soltar(ms, "corrida de prueba terminada")
	for _, nombre := range []string{"pc-a", "pc-b"} {
		if err := <-fines[nombre]; err != nil {
			t.Fatalf("%s tenía que recibir FIN: %v", nombre, err)
		}
	}
	if got := pp.Nodos(); len(got) != 1 || got[0].Nombre != "pc-c" || !got[0].Libre {
		t.Fatalf("después de soltar queda %+v", got)
	}
	if got := pp.vistos(); !strings.HasSuffix(got, "baja:pc-a baja:pc-b") {
		t.Errorf("eventos = %q; soltar deja una baja por nodo", got)
	}
	if _, err := pp.Tomar(0); err == nil {
		t.Error("Tomar(0) tiene que fallar")
	}
}

// Un nodo que se cae sale del pool en el latido siguiente y deja una baja.
func TestPoolDaDeBajaAlNodoQueSeCae(t *testing.T) {
	pp := levantarPool(t, 20*time.Millisecond)
	nodoFalso(t, pp.addr, "pc-vivo", protocolo.Version)
	conn, err := net.Dial("tcp", pp.addr)
	if err != nil {
		t.Fatal(err)
	}
	cod, dec := protocolo.NuevoCodificador(conn), protocolo.NuevoDecodificador(conn)
	cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Hola, Version: protocolo.Version, Nodo: "pc-fragil", Workers: 2})
	if m, err := dec.Leer(); err != nil || m.Tipo != protocolo.Ping {
		t.Fatalf("el pool le toma el pulso al entrar: %v, %v", m.Tipo, err)
	}
	cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Pong})
	pp.esperarNodos(t, 2)

	conn.Close() // se apaga
	pp.esperarNodos(t, 1)
	if got := pp.Nodos(); got[0].Nombre != "pc-vivo" {
		t.Fatalf("quedó %+v", got)
	}
	hasta(t, "la baja de pc-fragil ("+pp.vistos()+")", func() bool { return strings.Contains(pp.vistos(), "baja:pc-fragil") })
	// El que sigue vivo no se toca, y su latencia se sigue midiendo.
	time.Sleep(60 * time.Millisecond)
	if got := pp.vistos(); strings.Contains(got, "baja:pc-vivo") {
		t.Fatalf("eventos = %q", got)
	}
}

// Un nodo colgado (no cierra, pero no responde) también sale, por el plazo del
// latido; y Tomar no lo entrega aunque el latido periódico todavía no haya corrido.
func TestPoolTomarNoEntregaUnNodoQueDejoDeResponder(t *testing.T) {
	pp := levantarPool(t, time.Hour) // sin latido periódico: solo el de Tomar
	nodoFalso(t, pp.addr, "pc-vivo", protocolo.Version)
	conn, err := net.Dial("tcp", pp.addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	cod, dec := protocolo.NuevoCodificador(conn), protocolo.NuevoDecodificador(conn)
	cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Hola, Version: protocolo.Version, Nodo: "pc-colgado", Workers: 2})
	dec.Leer()
	cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Pong}) // responde el primero y se cuelga
	pp.esperarNodos(t, 2)

	var faltan *ErrorFaltanNodos
	if _, err := pp.Tomar(2); !errors.As(err, &faltan) || faltan.Libres != 1 {
		t.Fatalf("Tomar(2) = %v; pc-colgado no respondió el PING y no cuenta", err)
	}
	if got := pp.vistos(); !strings.Contains(got, "baja:pc-colgado") {
		t.Fatalf("eventos = %q", got)
	}
	if ms, err := pp.Tomar(1); err != nil || nombres(ms) != "pc-vivo" {
		t.Fatalf("Tomar(1) = %q, %v", nombres(ms), err)
	}
}

func TestPoolUnaReconexionReemplazaALaConexionVieja(t *testing.T) {
	pp := levantarPool(t, time.Hour)
	vieja := nodoFalso(t, pp.addr, "pc-01", protocolo.Version)
	pp.esperarNodos(t, 1)
	dirVieja := pp.Nodos()[0].Direccion
	nodoFalso(t, pp.addr, "pc-01", protocolo.Version)
	hasta(t, "que la conexión nueva reemplace a la vieja", func() bool {
		n := pp.Nodos()
		return len(n) == 1 && n[0].Direccion != dirVieja
	})
	if err := <-vieja; err == nil {
		t.Fatal("la conexión vieja se tenía que cortar sin FIN")
	}
	if got := pp.vistos(); got != "alta:pc-01 baja:pc-01 alta:pc-01" {
		t.Fatalf("eventos = %q", got)
	}
}

func TestPoolRechazaOtraVersionYAlQueNoRespondeElPrimerPing(t *testing.T) {
	pp := levantarPool(t, time.Hour)
	if err := <-nodoFalso(t, pp.addr, "pc-viejo", protocolo.Version+1); err == nil || !strings.Contains(err.Error(), "versión de protocolo") {
		t.Fatalf("a un nodo con otra versión le tiene que llegar el ERROR: %v", err)
	}
	nodoQueMuere(t, pp.addr, "pc-fugaz")
	hasta(t, "el error de pc-fugaz ("+pp.vistos()+")", func() bool { return strings.Contains(pp.vistos(), "error:pc-fugaz") })
	if pp.Cantidad() != 0 {
		t.Fatalf("no entró nadie y hay %d", pp.Cantidad())
	}
}

// Al cancelar, Correr vuelve, cierra a los libres y no deja goroutines; las
// conexiones de una corrida en curso no son suyas.
func TestPoolAlCerrarSueltaALosLibresYNoDejaGoroutines(t *testing.T) {
	pp := levantarPool(t, 20*time.Millisecond)
	libre := nodoFalso(t, pp.addr, "pc-b", protocolo.Version)
	ocupado := nodoFalso(t, pp.addr, "pc-a", protocolo.Version)
	pp.esperarNodos(t, 2)
	ms, err := pp.Tomar(1)
	if err != nil {
		t.Fatal(err)
	}

	pp.parar()
	<-pp.termino
	if err := <-libre; err == nil {
		t.Fatal("al nodo libre se le tenía que cerrar la conexión")
	}
	sinGoroutinesDeReunir(t)
	if _, err := protocolo.Latido(ms[0].Enlace, time.Now().Add(time.Second)); err != nil {
		t.Fatalf("la conexión de la corrida sigue viva después de cerrar el pool: %v", err)
	}
	Terminar(Enlaces(ms), time.Second)
	if err := <-ocupado; err != nil {
		t.Fatalf("el nodo de la corrida tenía que recibir su FIN: %v", err)
	}
	// Después de cerrar, quien llegue tarde no queda en un pool que nadie atiende.
	if _, err := pp.Tomar(1); err == nil {
		t.Fatal("un pool cerrado no entrega nodos")
	}
}

// Nodos de verdad: después del FIN vuelven a llamar solos y quedan libres para
// la corrida siguiente, que es lo que hace el coordinador de la API.
func TestPoolLosNodosVuelvenSolosDespuesDeUnaCorrida(t *testing.T) {
	const n = 600
	ruta, _ := goldConRuido(t, n)
	pp := levantarPool(t, 20*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for i := range 2 {
		nd := &Nodo{Nombre: fmt.Sprintf("nodo-%d", i+1), Workers: 1, Chunk: 64, Gold: ruta, Log: silencio}
		go nd.Correr(ctx, marcar(pp.addr), 5*time.Millisecond, false)
	}
	for corrida := range 3 {
		var ms []*Miembro
		hasta(t, fmt.Sprintf("dos nodos libres para la corrida %d", corrida+1), func() bool {
			var err error
			ms, err = pp.Tomar(2)
			return err == nil
		})
		rangos, _ := Repartir(n, 2)
		if _, err := Cargar(Enlaces(ms), rangos, huellaDe(t, ruta), time.Now().Add(5*time.Second)); err != nil {
			t.Fatalf("corrida %d: %v", corrida+1, err)
		}
		pp.Soltar(ms, "corrida terminada")
		if err := Terminar(Enlaces(ms), time.Second); err != nil {
			t.Fatalf("corrida %d: %v", corrida+1, err)
		}
	}
}
