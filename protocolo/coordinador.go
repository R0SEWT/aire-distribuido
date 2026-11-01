package protocolo

import (
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

var (
	// ErrInformado: el nodo mandó un ERROR. El texto del nodo va en el error.
	ErrInformado = errors.New("el nodo informó un error")
	// ErrProtocolo: llegó algo que el protocolo no permite en ese momento (un
	// tipo inesperado, un PARCIAL de una ronda que no se envió, formas que no
	// cuadran). Es un bug o un desajuste de versiones, no un nodo lento.
	ErrProtocolo = errors.New("violación del protocolo")
)

// ErrorNodo dice qué nodo falló. Envuelve la causa: errors.Is(err,
// ErrPlazoVencido) distingue un nodo lento o caído de uno que respondió mal.
type ErrorNodo struct {
	Nodo string
	Err  error
}

func (e *ErrorNodo) Error() string { return fmt.Sprintf("nodo %q: %v", e.Nodo, e.Err) }
func (e *ErrorNodo) Unwrap() error { return e.Err }

// Enlace es lo que el coordinador sabe de un nodo conectado: los dos extremos
// de su conexión y lo que el nodo declaró. Lo usa una sola goroutine a la vez;
// Ronda le asigna una por nodo.
type Enlace struct {
	Cod *Codificador
	Dec *Decodificador

	Nodo    string // del HOLA
	Workers int    // del HOLA: núcleos que el nodo dice usar
	Rango   Rango  // del CARGAR que el nodo confirmó con LISTO

	// Rezagados cuenta los PARCIAL de rondas anteriores que llegaron tarde y se
	// descartaron. Es una métrica del panel de administración.
	Rezagados int

	ultimaIter int // última ronda enviada por esta conexión
}

// NuevoEnlace envuelve la conexión con un nodo. conn tiene que admitir plazos
// (un net.Conn): sin ellos un nodo caído bloquearía la barrera para siempre.
func NuevoEnlace(conn io.ReadWriter) *Enlace {
	return &Enlace{Cod: NuevoCodificador(conn), Dec: NuevoDecodificador(conn)}
}

func (e *Enlace) nombre() string {
	if e.Nodo == "" {
		return "(sin HOLA)"
	}
	return e.Nodo
}

func (e *Enlace) fallo(formato string, args ...any) error {
	return &ErrorNodo{Nodo: e.nombre(), Err: fmt.Errorf(formato, args...)}
}

// RecibirHola espera el HOLA con el que el nodo abre la sesión y comprueba la
// versión. Si no coincide, le avisa con un ERROR antes de devolver el error.
func RecibirHola(e *Enlace, limite time.Time) error {
	m, err := e.esperar(Hola, 0, limite)
	if err != nil {
		return e.fallo("esperando HOLA: %w", err)
	}
	if m.Version != Version {
		texto := fmt.Sprintf("versión de protocolo %d, el coordinador habla la %d: actualiza el nodo", m.Version, Version)
		_ = e.Cod.EscribirAntesDe(Mensaje{Tipo: Error, Mensaje: texto}, limite)
		return &ErrorNodo{Nodo: m.Nodo, Err: fmt.Errorf("%w: %s", ErrProtocolo, texto)}
	}
	e.Nodo, e.Workers = m.Nodo, m.Workers
	return nil
}

// AsignarRango le manda CARGAR con las filas r del gold identificado por
// huella y espera su LISTO. Comprueba que el nodo cargó exactamente esas filas del mismo
// archivo: un rango sobre otra tabla, u ordenada distinto, suma otras filas.
func AsignarRango(e *Enlace, r Rango, huella string, limite time.Time) error {
	if err := e.Cod.EscribirAntesDe(Mensaje{Tipo: Cargar, Rango: &r, Huella: huella}, limite); err != nil {
		return e.fallo("enviar CARGAR: %w", err)
	}
	m, err := e.esperar(Listo, 0, limite)
	if err != nil {
		return e.fallo("esperando LISTO: %w", err)
	}
	if m.Huella != huella {
		return e.fallo("%w: cargó el gold %s y se le pidió %s", ErrProtocolo, m.Huella, huella)
	}
	if m.N != r.Len() {
		return e.fallo("%w: cargó %d filas y su rango [%d, %d) tiene %d", ErrProtocolo, m.N, r.Desde, r.Hasta, r.Len())
	}
	e.Rango = r
	return nil
}

// Ronda es la barrera de una iteración a través de la red. Manda ITERAR{iter,
// centroides} a todos los nodos a la vez (fan-out, una goroutine por nodo),
// espera el PARCIAL de esa ronda de cada uno (fan-in) y vuelve cuando cada nodo
// respondió o falló, y como muy tarde en limite: nunca se queda esperando para
// siempre.
//
// Los parciales vuelven EN EL ORDEN DE enlaces, no en el de llegada: quien
// llama los fusiona en ese orden y el resultado no depende de qué nodo
// respondió primero (sumar en punto flotante no es asociativo).
//
// Todos los nodos comparten el mismo instante límite. Si alguno no responde a
// tiempo, responde ERROR o rompe el protocolo, la ronda se aborta y el error
// nombra a cada nodo que falló; no se devuelve ningún parcial, porque una
// fusión con menos nodos sería otro modelo. Reasignar el rango del nodo caído
// es una extensión que no se implementa (docs/diseno.md, sección 4.7).
//
// iter tiene que ser mayor que cualquier ronda ya enviada por esas conexiones:
// así un PARCIAL que llega tarde de una ronda abortada se reconoce y se
// descarta.
func Ronda(enlaces []*Enlace, iter, k int, centroides []float64, limite time.Time) ([]Mensaje, error) {
	if len(enlaces) == 0 {
		return nil, errors.New("ronda sin nodos")
	}
	if k <= 0 || len(centroides) == 0 || len(centroides)%k != 0 {
		return nil, fmt.Errorf("ronda %d: %d centroides no son k*D con k=%d", iter, len(centroides), k)
	}
	vistos := make(map[*Enlace]bool, len(enlaces))
	for _, e := range enlaces {
		if vistos[e] {
			return nil, fmt.Errorf("ronda %d: el nodo %q aparece dos veces", iter, e.nombre())
		}
		vistos[e] = true
		if iter <= e.ultimaIter {
			return nil, fmt.Errorf("ronda %d: el número de ronda tiene que crecer y el nodo %q ya recibió la %d",
				iter, e.nombre(), e.ultimaIter)
		}
	}

	plazo := time.Until(limite).Round(time.Millisecond)
	parciales := make([]Mensaje, len(enlaces))
	errs := make([]error, len(enlaces))
	var barrera sync.WaitGroup
	barrera.Add(len(enlaces))
	for i, e := range enlaces {
		go func() {
			defer barrera.Done()
			// Cada goroutine escribe solo su índice: no hace falta Mutex.
			parciales[i], errs[i] = e.ronda(iter, k, centroides, limite, plazo)
		}()
	}
	barrera.Wait()

	if err := errors.Join(errs...); err != nil {
		return nil, fmt.Errorf("ronda %d abortada: %w", iter, err)
	}
	return parciales, nil
}

// ronda es la parte de Ronda que corresponde a un nodo.
func (e *Enlace) ronda(iter, k int, centroides []float64, limite time.Time, plazo time.Duration) (Mensaje, error) {
	// Se marca antes de enviar: desde este momento, cualquier PARCIAL de una
	// ronda anterior es un rezagado.
	e.ultimaIter = iter
	if err := e.Cod.EscribirAntesDe(Mensaje{Tipo: Iterar, Iter: iter, Centroides: centroides}, limite); err != nil {
		return Mensaje{}, e.fallo("enviar ITERAR: %w", err)
	}
	m, err := e.esperar(Parcial, iter, limite)
	if errors.Is(err, ErrPlazoVencido) {
		return Mensaje{}, e.fallo("no envió su PARCIAL dentro del plazo (%v): %w", plazo, err)
	}
	if err != nil {
		return Mensaje{}, e.fallo("esperando PARCIAL: %w", err)
	}

	dim := len(centroides) / k
	if len(m.Conteos) != k || len(m.Sumas) != k*dim {
		return Mensaje{}, e.fallo("%w: PARCIAL con %d conteos y %d sumas; se esperaban k=%d y k*D=%d",
			ErrProtocolo, len(m.Conteos), len(m.Sumas), k, k*dim)
	}
	// Conservación: cada fila del rango se asigna a exactamente un cluster.
	if n := e.Rango.Len(); n > 0 {
		total := 0
		for _, c := range m.Conteos {
			total += c
		}
		if total != n {
			return Mensaje{}, e.fallo("%w: sus conteos suman %d y su rango tiene %d filas", ErrProtocolo, total, n)
		}
	}
	return m, nil
}

// Latido manda PING y espera el PONG. Devuelve la ida y vuelta, que es la
// latencia que la barrera paga en cada iteración con ese nodo.
func Latido(e *Enlace, limite time.Time) (time.Duration, error) {
	inicio := time.Now()
	if err := e.Cod.EscribirAntesDe(Mensaje{Tipo: Ping}, limite); err != nil {
		return 0, e.fallo("enviar PING: %w", err)
	}
	if _, err := e.esperar(Pong, 0, limite); err != nil {
		return 0, e.fallo("esperando PONG: %w", err)
	}
	return time.Since(inicio), nil
}

// esperar lee hasta un mensaje del tipo pedido (si es PARCIAL, de la ronda
// iter). En el camino descarta los PONG sueltos y los PARCIAL rezagados; un
// ERROR del nodo o cualquier otra cosa termina la espera con error.
func (e *Enlace) esperar(tipo Tipo, iter int, limite time.Time) (Mensaje, error) {
	for {
		m, err := e.Dec.LeerAntesDe(limite)
		if err != nil {
			return Mensaje{}, err
		}
		switch {
		case m.Tipo == tipo && (tipo != Parcial || m.Iter == iter):
			return m, nil
		case m.Tipo == Pong:
			continue
		case m.Tipo == Parcial && m.Iter <= e.ultimaIter:
			e.Rezagados++
			continue
		case m.Tipo == Parcial:
			return Mensaje{}, fmt.Errorf("%w: PARCIAL de la ronda %d, que no se envió (la última es la %d)",
				ErrProtocolo, m.Iter, e.ultimaIter)
		case m.Tipo == Error:
			return Mensaje{}, fmt.Errorf("%w: %s", ErrInformado, m.Mensaje)
		default:
			return Mensaje{}, fmt.Errorf("%w: se esperaba %s y llegó %s", ErrProtocolo, tipo, m.Tipo)
		}
	}
}
