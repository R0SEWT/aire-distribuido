package protocolo

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// MaxLinea es el tamaño máximo de un mensaje en bytes. Un PARCIAL con k = 50 y
// D = 10 ocupa unos 15 KB; un megabyte deja margen de sobra y evita que un par
// roto o malicioso haga crecer el búfer sin límite.
const MaxLinea = 1 << 20

var (
	// ErrPlazoVencido indica que no llegó (o no salió) un mensaje antes del
	// plazo. Se compara con errors.Is.
	ErrPlazoVencido = errors.New("plazo vencido")
	// ErrSinPlazos indica que el lector o el escritor no admiten plazos: no
	// tienen SetReadDeadline o SetWriteDeadline. Un net.Conn sí los tiene.
	ErrSinPlazos = errors.New("la conexión no admite plazos")
)

// Codificador escribe mensajes, uno por línea. Es seguro para uso concurrente:
// un Mutex hace que cada línea salga entera, porque en un nodo la goroutine que
// responde PONG y la que manda el PARCIAL escriben en la misma conexión.
type Codificador struct {
	mu    sync.Mutex
	w     *bufio.Writer
	plazo interface{ SetWriteDeadline(time.Time) error }
	err   error // pegajoso: tras un fallo a medio escribir, el flujo ya no es confiable
}

// NuevoCodificador escribe sobre w. Si w tiene SetWriteDeadline (un net.Conn),
// EscribirAntesDe puede usar plazos.
func NuevoCodificador(w io.Writer) *Codificador {
	c := &Codificador{w: bufio.NewWriter(w)}
	c.plazo, _ = w.(interface{ SetWriteDeadline(time.Time) error })
	return c
}

// Escribir valida m, lo escribe en una línea y vacía el búfer: un mensaje del
// protocolo tiene que salir ya, porque del otro lado hay una barrera esperándolo.
func (c *Codificador) Escribir(m Mensaje) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.escribir(m)
}

// EscribirAntesDe es Escribir con un plazo absoluto. Si vence, el Codificador
// queda inutilizable: pudo salir media línea y el otro extremo ya no puede
// confiar en el flujo.
func (c *Codificador) EscribirAntesDe(m Mensaje, limite time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.plazo == nil {
		return ErrSinPlazos
	}
	if err := c.plazo.SetWriteDeadline(limite); err != nil {
		return fmt.Errorf("fijar plazo de escritura: %w", err)
	}
	defer c.plazo.SetWriteDeadline(time.Time{})
	return c.escribir(m)
}

func (c *Codificador) escribir(m Mensaje) error {
	if c.err != nil {
		return c.err
	}
	if err := m.Validar(); err != nil {
		return err // no se escribió nada: el flujo sigue sano
	}
	linea, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("codificar %s: %w", m.Tipo, err)
	}
	if len(linea)+1 > MaxLinea {
		return fmt.Errorf("%w: %s ocupa %d bytes, el máximo es %d", ErrMensajeInvalido, m.Tipo, len(linea)+1, MaxLinea)
	}
	linea = append(linea, '\n')
	if _, err := c.w.Write(linea); err != nil {
		c.err = traducir("escribir "+string(m.Tipo), err)
		return c.err
	}
	if err := c.w.Flush(); err != nil {
		c.err = traducir("escribir "+string(m.Tipo), err)
		return c.err
	}
	return nil
}

// Decodificador lee mensajes, uno por línea. No es seguro para uso concurrente:
// cada conexión tiene una sola goroutine lectora.
//
// Si vence un plazo a mitad de una línea, lo ya leído se guarda y la siguiente
// lectura continúa desde ahí: un plazo vencido no desincroniza el flujo, y por
// eso el coordinador puede seguir usando la conexión de un nodo lento y
// descartar después su PARCIAL rezagado.
type Decodificador struct {
	r         *bufio.Reader
	plazo     interface{ SetReadDeadline(time.Time) error }
	pendiente []byte // línea a medio leer cuando venció un plazo
	err       error  // pegajoso: línea demasiado larga, flujo truncado
}

// NuevoDecodificador lee de r. Si r tiene SetReadDeadline (un net.Conn),
// LeerAntesDe y LeerConPlazo pueden usar plazos.
func NuevoDecodificador(r io.Reader) *Decodificador {
	d := &Decodificador{r: bufio.NewReaderSize(r, 64<<10)}
	d.plazo, _ = r.(interface{ SetReadDeadline(time.Time) error })
	return d
}

// Leer bloquea hasta el siguiente mensaje. Devuelve io.EOF si el otro extremo
// cerró limpiamente entre dos mensajes, e io.ErrUnexpectedEOF si cerró a mitad
// de uno.
func (d *Decodificador) Leer() (Mensaje, error) {
	return d.leer()
}

// LeerConPlazo es Leer con un plazo relativo. Si vence devuelve un error que
// cumple errors.Is(err, ErrPlazoVencido), y el Decodificador sigue usable.
func (d *Decodificador) LeerConPlazo(plazo time.Duration) (Mensaje, error) {
	return d.LeerAntesDe(time.Now().Add(plazo))
}

// LeerAntesDe es Leer con un plazo absoluto. Es lo que usa la barrera: todos
// los nodos de una ronda comparten el mismo instante límite, y descartar un
// rezagado o un PONG no lo corre.
func (d *Decodificador) LeerAntesDe(limite time.Time) (Mensaje, error) {
	if d.plazo == nil {
		return Mensaje{}, ErrSinPlazos
	}
	if err := d.plazo.SetReadDeadline(limite); err != nil {
		return Mensaje{}, fmt.Errorf("fijar plazo de lectura: %w", err)
	}
	defer d.plazo.SetReadDeadline(time.Time{})
	return d.leer()
}

func (d *Decodificador) leer() (Mensaje, error) {
	if d.err != nil {
		return Mensaje{}, d.err
	}
	for {
		trozo, err := d.r.ReadSlice('\n')
		d.pendiente = append(d.pendiente, trozo...)
		if len(d.pendiente) > MaxLinea {
			d.err = fmt.Errorf("%w: línea de más de %d bytes", ErrMensajeInvalido, MaxLinea)
			return Mensaje{}, d.err
		}
		switch {
		case err == nil:
			linea := d.pendiente
			d.pendiente = d.pendiente[:0]
			return decodificar(linea)
		case errors.Is(err, bufio.ErrBufferFull):
			continue // la línea es más larga que el búfer de bufio: seguir juntando
		case errors.Is(err, io.EOF):
			if len(d.pendiente) > 0 {
				d.err = fmt.Errorf("leer: el otro extremo cerró a mitad de un mensaje (%d bytes): %w",
					len(d.pendiente), io.ErrUnexpectedEOF)
				return Mensaje{}, d.err
			}
			return Mensaje{}, io.EOF
		default:
			// Plazo vencido o error de red: lo leído queda en d.pendiente.
			return Mensaje{}, traducir("leer", err)
		}
	}
}

// decodificar convierte una línea en un Mensaje. Rechaza campos desconocidos:
// los dos extremos se compilan del mismo repo, y un campo que el otro no
// entiende es un desajuste de versiones que conviene ver, no ignorar.
func decodificar(linea []byte) (Mensaje, error) {
	var m Mensaje
	dec := json.NewDecoder(bytes.NewReader(linea))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return Mensaje{}, fmt.Errorf("%w: no es un mensaje JSON válido: %v (línea: %.80q)", ErrMensajeInvalido, err, linea)
	}
	if dec.More() {
		return Mensaje{}, fmt.Errorf("%w: más de un objeto JSON en la línea (%.80q)", ErrMensajeInvalido, linea)
	}
	if err := m.Validar(); err != nil {
		return Mensaje{}, err
	}
	return m, nil
}

// traducir agrega contexto a un error de E/S y, si fue un plazo, lo hace
// reconocible con errors.Is(err, ErrPlazoVencido) sin perder el original.
func traducir(contexto string, err error) error {
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return fmt.Errorf("%s: %w (%w)", contexto, ErrPlazoVencido, err)
	}
	return fmt.Errorf("%s: %w", contexto, err)
}
