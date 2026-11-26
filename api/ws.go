package api

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

// El WebSocket de la API (spec de la PC4, §6.4) está escrito sobre net/http y
// net, sin dependencias: es la parte de RFC 6455 que hace falta para que el
// servidor mande tramas de texto y el cliente solo escuche.
//
//   - El apretón de manos (§4.2): se valida el pedido, se responde 101 con
//     Sec-WebSocket-Accept y la conexión se le quita a net/http (Hijack).
//   - Tramas del servidor (§5.2): sin máscara y sin fragmentar; texto, ping,
//     pong y cierre.
//   - Tramas del cliente: tienen que venir enmascaradas (§5.3). Se atienden
//     ping (se responde pong), pong y cierre (se responde el cierre); los datos
//     que mande se leen y se descartan, porque este WebSocket solo envía.
//
// No hay extensiones (compresión): no se negocian y los bits RSV tienen que
// venir en cero.

// guidWS es la constante de RFC 6455 con la que se arma Sec-WebSocket-Accept.
const guidWS = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// Códigos de operación de una trama (RFC 6455, §5.2).
const (
	opContinuacion byte = 0x0
	opTexto        byte = 0x1
	opBinario      byte = 0x2
	opCierre       byte = 0x8
	opPing         byte = 0x9
	opPong         byte = 0xA
)

// Códigos de cierre (RFC 6455, §7.4.1).
const (
	cierreNormal    = 1000
	cierreSeVa      = 1001 // el servidor se apaga
	cierreProtocolo = 1002
	cierrePolitica  = 1008 // el cliente no lee al ritmo de los eventos
	cierreGrande    = 1009
)

// maxCargaCliente es el tope de una trama del cliente. No se espera que mande
// datos: alcanza para un ping o un cierre (125 bytes) y sobra para ignorar un
// mensaje chico sin cortar la conexión.
const maxCargaCliente = 4 << 10

var (
	errProtocoloWS = errors.New("websocket: el cliente rompió el protocolo")
	errTramaGrande = errors.New("websocket: trama demasiado grande")
)

// claveDeAceptacion es el Sec-WebSocket-Accept que corresponde a un
// Sec-WebSocket-Key: base64(sha1(clave + guid)). Es lo que le prueba al
// cliente que quien respondió entiende WebSocket.
func claveDeAceptacion(clave string) string {
	h := sha1.Sum([]byte(clave + guidWS))
	return base64.StdEncoding.EncodeToString(h[:])
}

// tieneToken dice si alguno de los valores del encabezado, que es una lista
// separada por comas, contiene el token, sin distinguir mayúsculas
// («Connection: keep-alive, Upgrade»).
func tieneToken(h http.Header, nombre, token string) bool {
	for _, v := range h.Values(nombre) {
		for _, t := range strings.Split(v, ",") {
			if strings.EqualFold(strings.TrimSpace(t), token) {
				return true
			}
		}
	}
	return false
}

// validarApreton comprueba que el pedido sea el apretón de manos de un cliente
// WebSocket (RFC 6455, §4.2.1) y devuelve su Sec-WebSocket-Key. Si no lo es,
// devuelve el mensaje para el 400.
func validarApreton(r *http.Request) (clave, mensaje string) {
	switch {
	case r.Method != http.MethodGet:
		return "", "el apretón de manos de WebSocket es un GET"
	case !r.ProtoAtLeast(1, 1):
		return "", "WebSocket necesita HTTP/1.1"
	case !tieneToken(r.Header, "Connection", "upgrade") || !tieneToken(r.Header, "Upgrade", "websocket"):
		return "", "esta ruta es un WebSocket: faltan los encabezados Connection: Upgrade y Upgrade: websocket"
	case r.Header.Get("Sec-WebSocket-Version") != "13":
		return "", "Sec-WebSocket-Version tiene que ser 13"
	}
	clave = r.Header.Get("Sec-WebSocket-Key")
	if b, err := base64.StdEncoding.DecodeString(clave); err != nil || len(b) != 16 {
		return "", "Sec-WebSocket-Key tiene que ser 16 bytes en base64"
	}
	return clave, ""
}

// tomarConexion le quita la conexión a net/http (Hijack) y la devuelve ya sin
// los plazos del servidor, junto con el lector que trae lo que net/http
// hubiera leído de más. No responde el 101: lo escribe quien se queda con la
// conexión (respuesta101), después de anotarla en el hub. Si no se puede
// tomar, responde 500.
func tomarConexion(w http.ResponseWriter) (net.Conn, *bufio.Reader, error) {
	conn, rw, err := http.NewResponseController(w).Hijack()
	if err != nil {
		escribirError(w, http.StatusInternalServerError, "el servidor no puede abrir un WebSocket en esta conexión")
		return nil, nil, err
	}
	// ReadTimeout y WriteTimeout del http.Server son plazos absolutos puestos
	// sobre esta conexión para el pedido que acaba de llegar: si no se quitan,
	// la cortan a los pocos segundos aunque ya no sea de net/http.
	if err := conn.SetDeadline(time.Time{}); err != nil {
		conn.Close()
		return nil, nil, err
	}
	return conn, rw.Reader, nil
}

// respuesta101 es la respuesta que completa el apretón de manos (RFC 6455,
// §4.2.2). Si subprotocolo no es vacío, lo confirma en Sec-WebSocket-Protocol.
func respuesta101(clave, subprotocolo string) []byte {
	resp := "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + claveDeAceptacion(clave) + "\r\n"
	if subprotocolo != "" {
		resp += "Sec-WebSocket-Protocol: " + subprotocolo + "\r\n"
	}
	return []byte(resp + "\r\n")
}

// tramaWS arma una trama del servidor: completa (FIN = 1), sin máscara, con el
// largo en 7 bits, en 16 o en 64 según el tamaño (RFC 6455, §5.2).
func tramaWS(op byte, carga []byte) []byte {
	n := len(carga)
	b := make([]byte, 0, n+10)
	b = append(b, 0x80|op)
	switch {
	case n <= 125:
		b = append(b, byte(n))
	case n <= 0xFFFF:
		b = append(b, 126, byte(n>>8), byte(n))
	default:
		b = append(b, 127)
		b = binary.BigEndian.AppendUint64(b, uint64(n))
	}
	return append(b, carga...)
}

// tramaDeCierre arma un cierre con su código y su motivo, que se recorta a lo
// que entra en una trama de control (125 bytes) sin partir una runa.
func tramaDeCierre(codigo int, motivo string) []byte {
	for len(motivo) > 123 {
		_, tam := utf8.DecodeLastRuneInString(motivo)
		motivo = motivo[:len(motivo)-tam]
	}
	carga := append([]byte{byte(codigo >> 8), byte(codigo)}, motivo...)
	return tramaWS(opCierre, carga)
}

// tramaLeida es una trama del cliente, ya sin máscara.
type tramaLeida struct {
	fin   bool
	op    byte
	carga []byte
}

// leerTrama lee una trama del cliente y valida lo que RFC 6455 exige de ella:
// sin bits reservados (no hay extensiones), con máscara, con un código de
// operación conocido, y las de control completas y de a lo sumo 125 bytes. Una
// carga de más de max devuelve errTramaGrande sin leerla.
func leerTrama(r *bufio.Reader, max int) (tramaLeida, error) {
	var cab [2]byte
	if _, err := io.ReadFull(r, cab[:]); err != nil {
		return tramaLeida{}, err
	}
	t := tramaLeida{fin: cab[0]&0x80 != 0, op: cab[0] & 0x0F}
	if cab[0]&0x70 != 0 {
		return t, fmt.Errorf("%w: bits reservados sin una extensión negociada", errProtocoloWS)
	}
	if cab[1]&0x80 == 0 {
		return t, fmt.Errorf("%w: una trama del cliente tiene que venir enmascarada", errProtocoloWS)
	}
	n := uint64(cab[1] & 0x7F)
	control := t.op&0x8 != 0
	switch t.op {
	case opContinuacion, opTexto, opBinario, opCierre, opPing, opPong:
	default:
		return t, fmt.Errorf("%w: código de operación %#x", errProtocoloWS, t.op)
	}
	if control && (!t.fin || n > 125) {
		return t, fmt.Errorf("%w: una trama de control va completa y con a lo sumo 125 bytes", errProtocoloWS)
	}
	switch n {
	case 126:
		var ext [2]byte
		if _, err := io.ReadFull(r, ext[:]); err != nil {
			return t, err
		}
		n = uint64(binary.BigEndian.Uint16(ext[:]))
	case 127:
		var ext [8]byte
		if _, err := io.ReadFull(r, ext[:]); err != nil {
			return t, err
		}
		n = binary.BigEndian.Uint64(ext[:])
	}
	if n > uint64(max) {
		return t, errTramaGrande
	}
	var mascara [4]byte
	if _, err := io.ReadFull(r, mascara[:]); err != nil {
		return t, err
	}
	t.carga = make([]byte, n)
	if _, err := io.ReadFull(r, t.carga); err != nil {
		return t, err
	}
	for i := range t.carga {
		t.carga[i] ^= mascara[i%4]
	}
	return t, nil
}

// codigoDeCierre saca el código de la carga de un cierre: 1000 si no trae
// ninguno. Una carga de un solo byte no es válida (RFC 6455, §5.5.1).
func codigoDeCierre(carga []byte) (int, error) {
	switch {
	case len(carga) == 0:
		return cierreNormal, nil
	case len(carga) == 1:
		return 0, fmt.Errorf("%w: un cierre con un solo byte de carga", errProtocoloWS)
	}
	return int(binary.BigEndian.Uint16(carga)), nil
}
