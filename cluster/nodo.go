package cluster

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/R0SEWT/aire-distribuido/kmeans"
	"github.com/R0SEWT/aire-distribuido/protocolo"
)

// ChunkPorOmision es el chunk de configs/kmeans.json. El nodo calcula su
// parcial en chunks de este tamaño fusionados en orden: con el mismo chunk que
// la receta, un solo nodo da el modelo de cmd/entrenar bit a bit. El protocolo
// no lo transmite, así que es un flag del nodo (-chunk).
const ChunkPorOmision = 1000

// ErrCoordinador: el coordinador mandó un ERROR (otra versión del protocolo,
// una corrida abortada). Con -una el nodo termina; si no, vuelve a llamar.
var ErrCoordinador = errors.New("el coordinador informó un error")

// Nodo es un nodo de cómputo: llama al coordinador y atiende sus corridas
// (docs/diseno.md, §4.2 y §4.6).
type Nodo struct {
	Nombre  string // el del HOLA
	Workers int    // núcleos que declara en el HOLA y goroutines de su pool
	Chunk   int    // filas por chunk del pool; 0 = ChunkPorOmision
	Gold    string // ruta del gold local, del que lee su rango
	Log     *log.Logger
}

// particion es el rango del gold que el nodo tiene en memoria entre CARGAR y FIN.
type particion struct {
	rango   protocolo.Rango
	datos   *kmeans.Datos
	rondas  int
	calculo time.Duration
}

// Huella identifica el gold en CARGAR y LISTO: «sha256:» seguido del sha256
// del archivo en hexadecimal, el mismo que reports/kmeans.json guarda en
// gold.sha256. El coordinador y el nodo la calculan con esta misma función.
func Huella(ruta string) (string, error) {
	h, err := kmeans.Sha256Archivo(ruta)
	if err != nil {
		return "", err
	}
	return HuellaDeSha256(h), nil
}

// HuellaDeSha256 arma la huella a partir del sha256 en hexadecimal.
func HuellaDeSha256(hex string) string { return "sha256:" + hex }

// Correr llama al coordinador, atiende una corrida y vuelve a llamar, hasta que
// se cancela ctx o, con una, termina la primera corrida. Devuelve nil solo con
// una y una corrida que acabó en FIN.
func (n *Nodo) Correr(ctx context.Context, llamar func(context.Context) (net.Conn, error), espera time.Duration, una bool) error {
	for {
		conn, err := n.conectar(ctx, llamar, espera)
		if err != nil {
			return err
		}
		n.Log.Printf("conectado desde %s a %s", conn.LocalAddr(), conn.RemoteAddr())

		// Si se cancela ctx (Ctrl+C) a mitad de una corrida, cerrar la conexión
		// destraba la lectura de Atender.
		cortar := context.AfterFunc(ctx, func() { conn.Close() })
		err = n.Atender(conn)
		cortar()
		conn.Close()

		switch {
		case ctx.Err() != nil:
			return ctx.Err()
		case err == nil && una:
			return nil
		case err == nil:
			n.Log.Printf("corrida terminada; espero la siguiente")
		case una:
			return err
		default:
			n.Log.Printf("%v; vuelvo a llamar", err)
		}
		if !dormir(ctx, espera) {
			return ctx.Err()
		}
	}
}

// conectar reintenta cada espera hasta abrir la conexión o hasta que se cancela
// ctx. No llena la pantalla: avisa en el primer fallo y luego cada ~30 s.
func (n *Nodo) conectar(ctx context.Context, llamar func(context.Context) (net.Conn, error), espera time.Duration) (net.Conn, error) {
	var ultimoAviso time.Time
	for intento := 1; ; intento++ {
		conn, err := llamar(ctx)
		if err == nil {
			return conn, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if time.Since(ultimoAviso) >= 30*time.Second {
			n.Log.Printf("el coordinador no responde (intento %d): %v; sigo intentando", intento, err)
			ultimoAviso = time.Now()
		}
		if !dormir(ctx, espera) {
			return nil, ctx.Err()
		}
	}
}

// Atender habla el protocolo sobre una conexión abierta: manda el HOLA y
// responde hasta el FIN. Devuelve nil solo si la corrida terminó con FIN.
//
// Atiende un mensaje a la vez: mientras calcula un PARCIAL no lee, así que un
// PING que llega en medio se responde al terminar. No importa, porque el
// coordinador solo manda PING fuera de las rondas (docs/diseno.md, §4.6), y
// así el nodo no tiene ninguna goroutine que coordinar con la partición.
func (n *Nodo) Atender(conn net.Conn) error {
	cod, dec := protocolo.NuevoCodificador(conn), protocolo.NuevoDecodificador(conn)
	hola := protocolo.Mensaje{Tipo: protocolo.Hola, Version: protocolo.Version, Nodo: n.Nombre, Workers: n.Workers}
	if err := cod.Escribir(hola); err != nil {
		return fmt.Errorf("enviar HOLA: %w", err)
	}

	var part *particion
	pongs := 0
	for {
		m, err := dec.Leer()
		if err != nil {
			return fmt.Errorf("la conexión se cortó antes del FIN (%d PONG enviados): %w", pongs, err)
		}
		var resp protocolo.Mensaje
		switch m.Tipo {
		case protocolo.Ping:
			resp = protocolo.Mensaje{Tipo: protocolo.Pong}
			pongs++
		case protocolo.Cargar:
			// Un CARGAR nuevo reemplaza la partición anterior, también si falla.
			part = nil
			p, err := n.cargar(*m.Rango, m.Huella)
			if err != nil {
				resp = n.error("CARGAR [%d, %d): %v", m.Rango.Desde, m.Rango.Hasta, err)
				break
			}
			part = p
			resp = protocolo.Mensaje{Tipo: protocolo.Listo, N: p.datos.N, Huella: m.Huella}
		case protocolo.Iterar:
			resp = n.iterar(part, m)
		case protocolo.Fin:
			n.Log.Print(resumen(part, pongs))
			return nil // la partición se libera con la sesión
		case protocolo.Error:
			return fmt.Errorf("%w: %s", ErrCoordinador, m.Mensaje)
		default:
			return fmt.Errorf("%w: un nodo no espera %s", protocolo.ErrProtocolo, m.Tipo)
		}
		if err := cod.Escribir(resp); err != nil {
			return fmt.Errorf("enviar %s: %w", resp.Tipo, err)
		}
	}
}

// error arma un ERROR para el coordinador y lo deja también en la ventana del
// nodo, donde lo ve quien está frente al equipo.
func (n *Nodo) error(formato string, args ...any) protocolo.Mensaje {
	texto := fmt.Sprintf("nodo %s: ", n.Nombre) + fmt.Sprintf(formato, args...)
	n.Log.Print(texto)
	return protocolo.Mensaje{Tipo: protocolo.Error, Mensaje: texto}
}

// cargar lee las filas r del gold local después de comprobar que es el
// archivo que el coordinador pide: con otra tabla, u ordenada distinto, el
// rango sumaría otras filas y nadie lo notaría (docs/diseno.md, §4.3).
func (n *Nodo) cargar(r protocolo.Rango, huella string) (*particion, error) {
	inicio := time.Now()
	if n.Gold == "" {
		return nil, errors.New("el nodo no tiene gold: pásalo con -gold")
	}
	propia, err := Huella(n.Gold)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("no encuentro el gold en %s: cópialo junto a nodo.exe o indícalo con -gold", n.Gold)
	}
	if err != nil {
		return nil, fmt.Errorf("no puedo leer el gold %s: %w", n.Gold, err)
	}
	if propia != huella {
		return nil, fmt.Errorf("mi gold %s tiene la huella %.19s… y el coordinador pide %.19s…: "+
			"no es el mismo archivo; copia el gold de esta corrida", n.Gold, propia, huella)
	}
	features, err := featuresDelGold(n.Gold)
	if err != nil {
		return nil, err
	}
	g, err := kmeans.LeerGold(n.Gold, features)
	if err != nil {
		return nil, err
	}
	// Se copia el rango y el resto del gold queda para el recolector: el nodo
	// guarda en memoria solo sus filas.
	datos := FilasVirtuales(g.Datos, r)
	virtual := ""
	if r.Hasta > g.Datos.N {
		virtual = fmt.Sprintf(" (tabla virtual: fila i = fila i mod %d del gold)", g.Datos.N)
	}
	n.Log.Printf("CARGAR [%d, %d): %d filas × %d features (%s) de %s en %v%s; huella %.19s… verificada",
		r.Desde, r.Hasta, datos.N, datos.D, strings.Join(features, ", "), n.Gold,
		time.Since(inicio).Round(time.Millisecond), virtual, propia)
	return &particion{rango: r, datos: datos}, nil
}

// FilasVirtuales copia las filas r de la tabla virtual del gold g: la fila i es
// la fila i mod N de g. Con r dentro de [0, N) son las filas tal cual; más
// allá del final, el gold se repite (réplicas virtuales, docs/diseno.md, §4.3:
// medir con r × N filas sin escribir un CSV r veces más grande). Las filas se
// copian de verdad, así que el cómputo por ronda es el de r × N filas.
func FilasVirtuales(g *kmeans.Datos, r protocolo.Rango) *kmeans.Datos {
	d := g.D
	x := make([]float64, 0, r.Len()*d)
	for i := r.Desde; i < r.Hasta; {
		fila := i % g.N
		tramo := min(r.Hasta-i, g.N-fila) // hasta el final del rango o del gold
		x = append(x, g.X[fila*d:(fila+tramo)*d]...)
		i += tramo
	}
	return &kmeans.Datos{X: x, N: r.Len(), D: d}
}

// featuresDelGold lee la cabecera del gold: estacion,fecha_hora y después las
// features, que son las de la receta (contrato del cargador). El nodo no lee
// la receta: toma las columnas del archivo cuya huella ya verificó.
func featuresDelGold(ruta string) ([]string, error) {
	f, err := os.Open(ruta)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	linea, err := bufio.NewReader(f).ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("leer la cabecera de %s: %w", ruta, err)
	}
	cols := strings.Split(strings.TrimRight(linea, "\r\n"), ",")
	if len(cols) < 3 || cols[0] != "estacion" || cols[1] != "fecha_hora" {
		return nil, fmt.Errorf("%s no tiene la cabecera de un gold (estacion,fecha_hora,features…): %q", ruta, linea)
	}
	return cols[2:], nil
}

// iterar calcula el PARCIAL de la partición con los centroides de la ronda.
func (n *Nodo) iterar(p *particion, m protocolo.Mensaje) protocolo.Mensaje {
	if p == nil {
		return n.error("ITERAR %d sin partición: falta un CARGAR que haya salido bien", m.Iter)
	}
	d := p.datos.D
	if len(m.Centroides)%d != 0 {
		return n.error("ITERAR %d: %d centroides no son k×D con D = %d", m.Iter, len(m.Centroides), d)
	}
	if p.rondas == 0 {
		n.Log.Printf("ITERAR: primera ronda (%d), k = %d", m.Iter, len(m.Centroides)/d)
	}
	inicio := time.Now()
	parcial := kmeans.ParcialesPorChunks(p.datos, m.Centroides, max(1, n.Workers), n.chunk())
	p.calculo += time.Since(inicio)
	p.rondas++
	return protocolo.Mensaje{Tipo: protocolo.Parcial, Iter: m.Iter, Sumas: parcial.Sumas, Conteos: parcial.Conteos, Inercia: parcial.Inercia}
}

func (n *Nodo) chunk() int {
	if n.Chunk <= 0 {
		return ChunkPorOmision
	}
	return n.Chunk
}

// resumen es la línea del FIN. El cómputo medio por ronda, comparado con el
// tiempo de la barrera que mide el coordinador, dice cuánto de cada iteración
// es red y cuánto es cálculo (docs/diseno.md, §4.5).
func resumen(p *particion, pongs int) string {
	if p == nil || p.rondas == 0 {
		return fmt.Sprintf("FIN: corrida OK, %d PONG enviados", pongs)
	}
	return fmt.Sprintf("FIN: corrida OK, %d rondas sobre [%d, %d); cómputo medio %.3f ms por ronda",
		p.rondas, p.rango.Desde, p.rango.Hasta, float64(p.calculo.Microseconds())/1000/float64(p.rondas))
}

// dormir espera d o hasta que se cancela ctx; dice si llegó a esperar d.
func dormir(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}
