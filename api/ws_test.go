package api

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

// tramaCliente arma una trama como la manda un cliente: enmascarada.
func tramaCliente(op byte, carga []byte, fin bool) []byte {
	b := []byte{op}
	if fin {
		b[0] |= 0x80
	}
	n := len(carga)
	switch {
	case n <= 125:
		b = append(b, 0x80|byte(n))
	case n <= 0xFFFF:
		b = append(b, 0x80|126, byte(n>>8), byte(n))
	default:
		b = append(b, 0x80|127)
		b = binary.BigEndian.AppendUint64(b, uint64(n))
	}
	mascara := [4]byte{0x37, 0xfa, 0x21, 0x3d}
	b = append(b, mascara[:]...)
	for i, c := range carga {
		b = append(b, c^mascara[i%4])
	}
	return b
}

// leerTramaServidor lee una trama como la manda el servidor: sin máscara y
// completa. Es el lado del cliente de las pruebas.
func leerTramaServidor(r *bufio.Reader) (op byte, carga []byte, err error) {
	var cab [2]byte
	if _, err := io.ReadFull(r, cab[:]); err != nil {
		return 0, nil, err
	}
	if cab[0]&0x80 == 0 || cab[0]&0x70 != 0 || cab[1]&0x80 != 0 {
		return 0, nil, fmt.Errorf("el servidor mandó una trama fragmentada, con bits reservados o enmascarada: %#x %#x", cab[0], cab[1])
	}
	n := uint64(cab[1] & 0x7F)
	switch n {
	case 126:
		var ext [2]byte
		if _, err := io.ReadFull(r, ext[:]); err != nil {
			return 0, nil, err
		}
		n = uint64(binary.BigEndian.Uint16(ext[:]))
	case 127:
		var ext [8]byte
		if _, err := io.ReadFull(r, ext[:]); err != nil {
			return 0, nil, err
		}
		n = binary.BigEndian.Uint64(ext[:])
	}
	carga = make([]byte, n)
	_, err = io.ReadFull(r, carga)
	return cab[0] & 0x0F, carga, err
}

// clienteWSPrueba es un cliente WebSocket mínimo sobre una conexión abierta.
type clienteWSPrueba struct {
	conn net.Conn
	r    *bufio.Reader
}

// leer devuelve la siguiente trama del servidor, con un tope de espera.
func (c *clienteWSPrueba) leer() (byte, []byte, error) {
	c.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	return leerTramaServidor(c.r)
}

// leerTexto devuelve la siguiente trama de texto y responde los pings que
// lleguen antes, como un cliente de verdad.
func (c *clienteWSPrueba) leerTexto(t testing.TB) string {
	t.Helper()
	texto, err := c.siguienteTexto()
	if err != nil {
		t.Fatal(err)
	}
	return texto
}

// siguienteTexto es leerTexto para una goroutine que no es la de la prueba,
// que no puede llamar a t.Fatal.
func (c *clienteWSPrueba) siguienteTexto() (string, error) {
	for {
		op, carga, err := c.leer()
		if err != nil {
			return "", fmt.Errorf("esperando una trama de texto: %w", err)
		}
		switch op {
		case opTexto:
			return string(carga), nil
		case opPing:
			c.mandar(opPong, carga)
		default:
			return "", fmt.Errorf("se esperaba texto y llegó %#x %q", op, carga)
		}
	}
}

// leerCierre devuelve el código y el motivo del cierre que tiene que llegar.
func (c *clienteWSPrueba) leerCierre(t testing.TB) (int, string) {
	t.Helper()
	for {
		op, carga, err := c.leer()
		if err != nil {
			t.Fatalf("esperando el cierre: %v", err)
		}
		switch {
		case op == opPing:
		case op == opCierre && len(carga) >= 2:
			return int(binary.BigEndian.Uint16(carga)), string(carga[2:])
		default:
			t.Fatalf("se esperaba un cierre y llegó %#x %q", op, carga)
		}
	}
}

// seCorto dice si la conexión se cerró (lo siguiente que se lee es un error).
func (c *clienteWSPrueba) seCorto() bool {
	for {
		op, _, err := c.leer()
		if err != nil {
			var ne net.Error
			return !(errors.As(err, &ne) && ne.Timeout())
		}
		if op != opPing {
			return false
		}
	}
}

func (c *clienteWSPrueba) mandar(op byte, carga []byte) error {
	c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, err := c.conn.Write(tramaCliente(op, carga, true))
	return err
}

func TestClaveDeAceptacionEsLaDelRFC(t *testing.T) {
	// El ejemplo de RFC 6455, §1.3.
	if got := claveDeAceptacion("dGhlIHNhbXBsZSBub25jZQ=="); got != "s3pPLMBiTxaQ9kYGzzhZRbK+xOo=" {
		t.Fatalf("Sec-WebSocket-Accept = %q", got)
	}
}

func TestTramaDelServidorSegunElLargo(t *testing.T) {
	// El ejemplo de RFC 6455, §5.7: «Hello» en una trama de texto sin máscara.
	if got := tramaWS(opTexto, []byte("Hello")); !bytes.Equal(got, []byte{0x81, 0x05, 'H', 'e', 'l', 'l', 'o'}) {
		t.Fatalf("trama = %x", got)
	}
	for _, c := range []struct{ n, cabecera int }{{0, 2}, {125, 2}, {126, 4}, {65535, 4}, {65536, 10}} {
		carga := bytes.Repeat([]byte{'x'}, c.n)
		trama := tramaWS(opTexto, carga)
		if len(trama) != c.cabecera+c.n {
			t.Errorf("con %d bytes la cabecera mide %d, se esperaban %d", c.n, len(trama)-c.n, c.cabecera)
		}
		op, vuelta, err := leerTramaServidor(bufio.NewReader(bytes.NewReader(trama)))
		if err != nil || op != opTexto || !bytes.Equal(vuelta, carga) {
			t.Errorf("con %d bytes la trama no se relee igual: %v", c.n, err)
		}
	}
}

func TestTramaDeCierreRecortaElMotivoSinPartirUnaRuna(t *testing.T) {
	trama := tramaDeCierre(cierrePolitica, strings.Repeat("ñ", 100)) // 200 bytes
	op, carga, err := leerTramaServidor(bufio.NewReader(bytes.NewReader(trama)))
	if err != nil || op != opCierre || len(carga) > 125 || binary.BigEndian.Uint16(carga) != cierrePolitica {
		t.Fatalf("cierre = %#x, %d bytes, %v", op, len(carga), err)
	}
	if motivo := string(carga[2:]); motivo != strings.Repeat("ñ", 61) {
		t.Fatalf("motivo = %q (%d bytes)", motivo, len(motivo))
	}
}

func TestLeerTramaDelCliente(t *testing.T) {
	leer := func(b []byte) (tramaLeida, error) {
		return leerTrama(bufio.NewReader(bytes.NewReader(b)), maxCargaCliente)
	}

	// El ejemplo de RFC 6455, §5.7: «Hello» enmascarado.
	tr, err := leer([]byte{0x81, 0x85, 0x37, 0xfa, 0x21, 0x3d, 0x7f, 0x9f, 0x4d, 0x51, 0x58})
	if err != nil || !tr.fin || tr.op != opTexto || string(tr.carga) != "Hello" {
		t.Fatalf("trama = %+v, %v", tr, err)
	}
	for _, n := range []int{0, 125, 126, 4096} {
		carga := bytes.Repeat([]byte{'z'}, n)
		if tr, err := leer(tramaCliente(opBinario, carga, true)); err != nil || !bytes.Equal(tr.carga, carga) {
			t.Errorf("%d bytes: %v", n, err)
		}
	}
	if tr, err := leer(tramaCliente(opTexto, []byte("a"), false)); err != nil || tr.fin {
		t.Errorf("un fragmento de datos es válido: %+v, %v", tr, err)
	}

	sinMascara := tramaWS(opTexto, []byte("hola"))
	reservado := tramaCliente(opTexto, []byte("hola"), true)
	reservado[0] |= 0x40
	casos := map[string]struct {
		trama []byte
		want  error
	}{
		"sin máscara":                 {sinMascara, errProtocoloWS},
		"bits reservados":             {reservado, errProtocoloWS},
		"código desconocido":          {tramaCliente(0x3, nil, true), errProtocoloWS},
		"control de más de 125":       {tramaCliente(opPing, bytes.Repeat([]byte{'p'}, 126), true), errProtocoloWS},
		"control fragmentado":         {tramaCliente(opPing, nil, false), errProtocoloWS},
		"más grande que el tope":      {tramaCliente(opTexto, bytes.Repeat([]byte{'x'}, maxCargaCliente+1), true), errTramaGrande},
		"largo de 64 bits gigante":    {[]byte{0x82, 0xFF, 0x7F, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}, errTramaGrande},
		"cortada a mitad de la carga": {tramaCliente(opTexto, []byte("hola"), true)[:8], io.ErrUnexpectedEOF},
	}
	for nombre, c := range casos {
		if _, err := leer(c.trama); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, se esperaba %v", nombre, err, c.want)
		}
	}
	if _, err := codigoDeCierre([]byte{1}); !errors.Is(err, errProtocoloWS) {
		t.Errorf("un cierre de un byte = %v", err)
	}
	if c, err := codigoDeCierre(nil); c != cierreNormal || err != nil {
		t.Errorf("un cierre sin carga = %d, %v", c, err)
	}
}

func TestValidarApreton(t *testing.T) {
	bueno := func() *http.Request {
		r, _ := http.NewRequest("GET", "/ws", nil)
		r.Header.Set("Connection", "keep-alive, Upgrade")
		r.Header.Set("Upgrade", "WebSocket")
		r.Header.Set("Sec-WebSocket-Version", "13")
		r.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
		return r
	}
	if clave, msg := validarApreton(bueno()); msg != "" || clave != "dGhlIHNhbXBsZSBub25jZQ==" {
		t.Fatalf("un apretón válido = %q, %q", clave, msg)
	}
	casos := map[string]func(r *http.Request){
		"sin Upgrade":     func(r *http.Request) { r.Header.Del("Upgrade") },
		"sin Connection":  func(r *http.Request) { r.Header.Set("Connection", "keep-alive") },
		"otra versión":    func(r *http.Request) { r.Header.Set("Sec-WebSocket-Version", "8") },
		"clave corta":     func(r *http.Request) { r.Header.Set("Sec-WebSocket-Key", "YWJj") },
		"clave no base64": func(r *http.Request) { r.Header.Set("Sec-WebSocket-Key", "no es base64!!") },
		"no es GET":       func(r *http.Request) { r.Method = "POST" },
		"HTTP/1.0":        func(r *http.Request) { r.ProtoMajor, r.ProtoMinor = 1, 0 },
	}
	for nombre, cambiar := range casos {
		r := bueno()
		cambiar(r)
		if _, msg := validarApreton(r); msg == "" {
			t.Errorf("%s: se aceptó el apretón", nombre)
		}
	}
}
