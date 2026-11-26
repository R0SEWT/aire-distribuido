package api

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"runtime"
	"strings"
	"testing"
	"time"
)

// hubDePrueba es un hub corriendo, con tiempos cortos.
type hubDePrueba struct {
	*Hub
	parar   context.CancelFunc
	termino chan struct{}
}

func opcionesHubDePrueba() opcionesHub {
	return opcionesHub{buferCliente: 8, buferDifusion: 256, cadaPing: time.Hour, plazoPong: time.Hour,
		plazoEscritura: 2 * time.Second, graciaCierre: 200 * time.Millisecond}
}

func levantarHub(t *testing.T, op opcionesHub) *hubDePrueba {
	t.Helper()
	h := &hubDePrueba{Hub: nuevoHub(op, logMudo), termino: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	h.parar = cancel
	go func() { h.Correr(ctx); close(h.termino) }()
	t.Cleanup(func() { cancel(); <-h.termino })
	return h
}

// conectar anota en el hub un extremo de net.Pipe y devuelve el otro como
// cliente. net.Pipe no tiene búfer: una escritura del servidor espera a que el
// cliente lea, que es justo lo que hace falta para probar a un cliente lento.
func (h *hubDePrueba) conectar(t *testing.T, rol string, expira time.Time) *clienteWSPrueba {
	t.Helper()
	servidor, cliente := net.Pipe()
	t.Cleanup(func() { cliente.Close() })
	if !h.Atender(servidor, bufio.NewReader(servidor), rol, expira, nil) {
		t.Fatal("el hub no admitió al cliente")
	}
	return &clienteWSPrueba{conn: cliente, r: bufio.NewReader(cliente)}
}

func esperarHasta(t *testing.T, que string, cond func() bool) {
	t.Helper()
	limite := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(limite) {
			t.Fatalf("no pasó a tiempo: %s", que)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func TestHubDifundeATodosEnOrdenConElSobreDelContrato(t *testing.T) {
	const eventos = 40
	op := opcionesHubDePrueba()
	op.buferCliente = eventos // una ráfaga entera cabe en el búfer: aquí nadie es lento
	h := levantarHub(t, op)
	clientes := []*clienteWSPrueba{h.conectar(t, RolAdmin, time.Time{}), h.conectar(t, RolAdmin, time.Time{}), h.conectar(t, RolUsuario, time.Time{})}

	recibidos := make(chan error, len(clientes))
	for _, c := range clientes {
		go func() {
			for i := range eventos {
				var sobre struct {
					Evento string `json:"evento"`
					Datos  struct {
						ID   string `json:"id"`
						Iter int    `json:"iter"`
					} `json:"datos"`
				}
				texto, err := c.siguienteTexto()
				if err != nil {
					recibidos <- err
					return
				}
				if err := json.Unmarshal([]byte(texto), &sobre); err != nil || sobre.Evento != "entrenamiento.ronda" ||
					sobre.Datos.ID != "e-1" || sobre.Datos.Iter != i+1 || strings.Contains(texto, "\n") {
					recibidos <- fmt.Errorf("evento %d: %q (%v)", i+1, texto, err)
					return
				}
			}
			recibidos <- nil
		}()
	}
	for i := range eventos {
		if err := h.Difundir("entrenamiento.ronda", map[string]any{"id": "e-1", "iter": i + 1}); err != nil {
			t.Fatal(err)
		}
	}
	for range clientes {
		if err := <-recibidos; err != nil {
			t.Fatal(err)
		}
	}
}

func TestHubFiltraPorRol(t *testing.T) {
	h := levantarHub(t, opcionesHubDePrueba())
	admin, usuario := h.conectar(t, RolAdmin, time.Time{}), h.conectar(t, RolUsuario, time.Time{})
	h.Difundir("entrenamiento.ronda", map[string]int{"iter": 1}, RolAdmin)
	h.Difundir("para.todos", map[string]int{"n": 2})

	if got := admin.leerTexto(t); !strings.Contains(got, "entrenamiento.ronda") {
		t.Fatalf("el admin recibió %q", got)
	}
	if got := admin.leerTexto(t); !strings.Contains(got, "para.todos") {
		t.Fatalf("el admin recibió %q", got)
	}
	if got := usuario.leerTexto(t); !strings.Contains(got, "para.todos") {
		t.Fatalf("el usuario recibió %q: el evento de admin no es para él", got)
	}
}

// Un cliente que no lee llena su búfer: el hub lo da de baja y los demás
// reciben todo, sin esperar.
func TestHubDesconectaAlClienteLentoSinFrenarALosDemas(t *testing.T) {
	op := opcionesHubDePrueba()
	op.buferCliente = 4
	h := levantarHub(t, op)
	rapido, lento := h.conectar(t, RolAdmin, time.Time{}), h.conectar(t, RolAdmin, time.Time{})

	const eventos = 60
	leidos := make(chan int, 1)
	go func() {
		n := 0
		for n < eventos {
			if got, err := rapido.siguienteTexto(); err != nil || !strings.Contains(got, fmt.Sprintf(`"n":%d}`, n)) {
				break
			}
			n++
		}
		leidos <- n
	}()
	inicio := time.Now()
	for i := range eventos {
		if err := h.Difundir("x", map[string]int{"n": i}); err != nil {
			t.Fatal(err)
		}
		// Difundir no espera a nadie; se deja al cliente rápido ir leyendo,
		// porque net.Pipe no tiene búfer y él también se llenaría.
		esperarHasta(t, "que el cliente rápido lea", func() bool { return len(h.difusion) == 0 })
		time.Sleep(time.Millisecond)
	}
	if n := <-leidos; n != eventos {
		t.Fatalf("el cliente rápido recibió %d de %d eventos", n, eventos)
	}
	if d := time.Since(inicio); d > 3*time.Second {
		t.Fatalf("difundir tardó %v: el cliente lento frenó a los demás", d)
	}
	if h.lentos.Load() != 1 || h.conectados.Load() != 1 {
		t.Fatalf("lentos = %d, conectados = %d; el lento tenía que salir y el rápido quedarse", h.lentos.Load(), h.conectados.Load())
	}

	// El lento, cuando por fin lee: lo que su escritora ya estaba mandando y
	// después el cierre 1008; lo que quedó en su búfer ya no se manda.
	if got := lento.leerTexto(t); !strings.Contains(got, `"n":0}`) {
		t.Fatalf("lo primero que lee el lento es %q", got)
	}
	if codigo, motivo := lento.leerCierre(t); codigo != cierrePolitica || !strings.Contains(motivo, "lento") {
		t.Fatalf("cierre = %d %q", codigo, motivo)
	}
	lento.mandar(opCierre, []byte{0x03, 0xF0})
	if !lento.seCorto() {
		t.Fatal("la conexión del cliente lento tenía que cerrarse")
	}
}

// El mismo caso con un cliente que nunca lee: su conexión se corta igual, por
// el plazo de escritura.
func TestHubCortaAlClienteQueNoLeeNada(t *testing.T) {
	op := opcionesHubDePrueba()
	op.buferCliente, op.plazoEscritura = 2, 50*time.Millisecond
	h := levantarHub(t, op)
	lento := h.conectar(t, RolAdmin, time.Time{})
	for i := range 10 {
		h.Difundir("x", map[string]int{"n": i})
	}
	esperarHasta(t, "la baja del cliente lento", func() bool { return h.lentos.Load() == 1 && h.conectados.Load() == 0 })
	time.Sleep(150 * time.Millisecond) // pasa el plazo de escritura
	if !lento.seCorto() {
		t.Fatal("la conexión tenía que estar cortada")
	}
}

func TestHubRespondeElPingYElCierreDelCliente(t *testing.T) {
	h := levantarHub(t, opcionesHubDePrueba())
	c := h.conectar(t, RolAdmin, time.Time{})

	c.mandar(opPing, []byte("sigues ahí"))
	if op, carga, err := c.leer(); err != nil || op != opPong || string(carga) != "sigues ahí" {
		t.Fatalf("respuesta al ping = %#x %q, %v", op, carga, err)
	}
	// Lo que el cliente mande como datos se lee y se descarta, también fragmentado.
	c.mandar(opTexto, []byte("hola"))
	c.conn.Write(tramaCliente(opTexto, []byte("par"), false))
	c.mandar(opPing, nil) // un control en medio de un mensaje fragmentado es válido
	c.conn.Write(tramaCliente(opContinuacion, []byte("tido"), true))
	if op, _, err := c.leer(); err != nil || op != opPong {
		t.Fatalf("pong = %#x, %v", op, err)
	}
	h.Difundir("x", 1)
	if got := c.leerTexto(t); got != `{"evento":"x","datos":1}` {
		t.Fatalf("después de mandar datos el cliente sigue conectado: %q", got)
	}

	c.mandar(opCierre, []byte{0x03, 0xE8}) // 1000
	if codigo, _ := c.leerCierre(t); codigo != cierreNormal {
		t.Fatalf("el cierre se responde con el mismo código: %d", codigo)
	}
	if !c.seCorto() {
		t.Fatal("después del cierre la conexión se corta")
	}
	esperarHasta(t, "la baja", func() bool { return h.conectados.Load() == 0 })
}

func TestHubCierraAlQueRompeElProtocolo(t *testing.T) {
	h := levantarHub(t, opcionesHubDePrueba())
	casos := map[string]struct {
		trama  []byte
		codigo int
	}{
		"sin máscara":                  {tramaWS(opTexto, []byte("hola")), cierreProtocolo},
		"continuación sin mensaje":     {tramaCliente(opContinuacion, []byte("x"), true), cierreProtocolo},
		"cierre de un byte":            {tramaCliente(opCierre, []byte{1}, true), cierreProtocolo},
		"trama más grande que el tope": {tramaCliente(opBinario, make([]byte, maxCargaCliente+1), true)[:14], cierreGrande},
	}
	for nombre, caso := range casos {
		c := h.conectar(t, RolAdmin, time.Time{})
		go c.conn.Write(caso.trama)
		if codigo, _ := c.leerCierre(t); codigo != caso.codigo {
			t.Errorf("%s: cierre %d, se esperaba %d", nombre, codigo, caso.codigo)
		}
		if !c.seCorto() {
			t.Errorf("%s: la conexión tenía que cortarse", nombre)
		}
	}
	esperarHasta(t, "las bajas", func() bool { return h.conectados.Load() == 0 })
}

// El servidor manda pings; el cliente que responde sigue, el que calla se corta.
func TestHubMandaPingsYCortaAlQueNoDaSenalesDeVida(t *testing.T) {
	op := opcionesHubDePrueba()
	op.cadaPing, op.plazoPong = 20*time.Millisecond, 60*time.Millisecond
	h := levantarHub(t, op)
	vivo, mudo := h.conectar(t, RolAdmin, time.Time{}), h.conectar(t, RolAdmin, time.Time{})

	pings := 0
	limite := time.Now().Add(300 * time.Millisecond)
	for time.Now().Before(limite) {
		op, carga, err := vivo.leer()
		if err != nil {
			t.Fatalf("el cliente que responde los pings se cortó: %v", err)
		}
		if op == opPing {
			pings++
			vivo.mandar(opPong, carga)
		}
	}
	if pings < 5 {
		t.Fatalf("llegaron %d pings en 300 ms con uno cada 20 ms", pings)
	}
	// El mudo leyó sus pings (si no, net.Pipe trabaría a su escritora) pero
	// nunca respondió: pasado cadaPing + plazoPong, afuera.
	for {
		if _, _, err := mudo.leer(); err != nil {
			break
		}
	}
	esperarHasta(t, "la baja del cliente mudo", func() bool { return h.conectados.Load() == 1 })
}

func TestHubCierraAlClienteCuandoVenceSuToken(t *testing.T) {
	h := levantarHub(t, opcionesHubDePrueba())
	c := h.conectar(t, RolUsuario, time.Now().Add(50*time.Millisecond))
	if codigo, motivo := c.leerCierre(t); codigo != cierrePolitica || !strings.Contains(motivo, "token") {
		t.Fatalf("cierre = %d %q", codigo, motivo)
	}
	c.mandar(opCierre, []byte{0x03, 0xF0})
	if !c.seCorto() {
		t.Fatal("la conexión tenía que cortarse")
	}
	esperarHasta(t, "la baja", func() bool { return h.conectados.Load() == 0 })
}

// Al apagarse: cierre 1001 a cada cliente, Correr vuelve y no queda ninguna
// goroutine del hub ni de sus clientes.
func TestHubAlApagarseAvisaATodosYNoDejaGoroutines(t *testing.T) {
	antes := runtime.NumGoroutine()
	h := levantarHub(t, opcionesHubDePrueba())
	clientes := []*clienteWSPrueba{h.conectar(t, RolAdmin, time.Time{}), h.conectar(t, RolUsuario, time.Time{})}
	h.Difundir("x", 1)
	for _, c := range clientes {
		c.leerTexto(t)
	}

	h.parar()
	for _, c := range clientes {
		if codigo, motivo := c.leerCierre(t); codigo != cierreSeVa || !strings.Contains(motivo, "apagando") {
			t.Fatalf("cierre = %d %q", codigo, motivo)
		}
		c.mandar(opCierre, []byte{0x03, 0xE9})
		if !c.seCorto() {
			t.Fatal("la conexión tenía que cortarse")
		}
	}
	select {
	case <-h.termino:
	case <-time.After(5 * time.Second):
		t.Fatal("Correr no volvió")
	}
	if err := h.Difundir("x", 2); err != errHubCerrado {
		t.Fatalf("Difundir a un hub apagado = %v", err)
	}
	servidor, cliente := net.Pipe()
	defer cliente.Close()
	if h.Atender(servidor, bufio.NewReader(servidor), RolAdmin, time.Time{}, nil) {
		t.Fatal("un hub apagado no admite clientes")
	}
	esperarHasta(t, fmt.Sprintf("que terminen las goroutines (había %d, hay %d)", antes, runtime.NumGoroutine()),
		func() bool { return runtime.NumGoroutine() <= antes })
}

// Si el hub no corre, Difundir no bloquea: cuenta el descarte.
func TestDifundirNuncaBloquea(t *testing.T) {
	op := opcionesHubDePrueba()
	op.buferDifusion = 3
	h := nuevoHub(op, logMudo)
	for i := range 5 {
		err := h.Difundir("x", i)
		if (i < 3) != (err == nil) {
			t.Fatalf("evento %d: %v", i, err)
		}
	}
	if h.descartados.Load() != 2 {
		t.Fatalf("descartados = %d", h.descartados.Load())
	}
	if err := h.Difundir("x", func() {}); err == nil {
		t.Fatal("un dato que no se puede codificar tiene que dar error")
	}
}
