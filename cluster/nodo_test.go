package cluster

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/R0SEWT/aire-distribuido/kmeans"
	"github.com/R0SEWT/aire-distribuido/protocolo"
)

var silencio = log.New(io.Discard, "", 0)

// goldDePrueba escribe un gold de n filas con dos features en un directorio
// temporal: dos estaciones, horas consecutivas, valores múltiplos de 1/4. Con
// esos valores toda suma de filas es exacta en float64, así que el resultado no
// depende del orden en que se sume (ver lloyd_test.go).
func goldDePrueba(t *testing.T, n int) (string, *kmeans.Gold) {
	t.Helper()
	g := &kmeans.Gold{Datos: &kmeans.Datos{D: 2}, Features: []string{"pm2_5", "pm10"}}
	t0 := time.Date(2024, 1, 1, 0, 0, 0, 0, kmeans.ZonaPeru)
	for i := range n {
		est, h := "CA-AAA-01", i
		if i >= n/2 {
			est, h = "CA-BBB-02", i-n/2
		}
		// Tres nubes que se solapan, alrededor de (-2, -2), (0, 3) y (4, 1):
		// Lloyd tarda varias iteraciones en separarlas.
		cx, cy := [3]float64{-2, 0, 4}[i%3], [3]float64{-2, 3, 1}[i%3]
		g.Datos.X = append(g.Datos.X, cx+float64((i*7)%23-11)*0.25, cy+float64((i*11)%29-14)*0.25)
		g.Datos.N++
		g.Estacion = append(g.Estacion, est)
		g.FechaHora = append(g.FechaHora, t0.Add(time.Duration(h)*time.Hour))
	}
	ruta := filepath.Join(t.TempDir(), "oefa_features.csv")
	f, err := os.Create(ruta)
	if err != nil {
		t.Fatal(err)
	}
	if err := kmeans.EscribirGold(f, g); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return ruta, g
}

func huellaDe(t *testing.T, ruta string) string {
	t.Helper()
	h, err := Huella(ruta)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func nodoDePrueba(gold string) *Nodo {
	return &Nodo{Nombre: "pc-07", Workers: 3, Chunk: 4, Gold: gold, Log: silencio}
}

// coordinadorFalso habla el lado del coordinador sobre un extremo de net.Pipe:
// lee el HOLA y después corre guion. Devuelve el HOLA por hola y el error del
// guion por errs.
func coordinadorFalso(t *testing.T, conn net.Conn, guion func(*protocolo.Codificador, *protocolo.Decodificador) error) (<-chan protocolo.Mensaje, <-chan error) {
	t.Helper()
	hola := make(chan protocolo.Mensaje, 1)
	errs := make(chan error, 1)
	go func() {
		defer conn.Close()
		cod, dec := protocolo.NuevoCodificador(conn), protocolo.NuevoDecodificador(conn)
		m, err := dec.Leer()
		if err != nil {
			errs <- err
			return
		}
		hola <- m
		errs <- guion(cod, dec)
	}()
	return hola, errs
}

func esperarTipo(dec *protocolo.Decodificador, tipo protocolo.Tipo) (protocolo.Mensaje, error) {
	m, err := dec.LeerConPlazo(2 * time.Second)
	if err != nil {
		return m, err
	}
	if m.Tipo != tipo {
		return m, errors.New("se esperaba " + string(tipo) + " y llegó " + string(m.Tipo) + " " + m.Mensaje)
	}
	return m, nil
}

// pedir manda m y espera una respuesta del tipo dado.
func pedir(cod *protocolo.Codificador, dec *protocolo.Decodificador, m protocolo.Mensaje, tipo protocolo.Tipo) (protocolo.Mensaje, error) {
	if err := cod.Escribir(m); err != nil {
		return protocolo.Mensaje{}, err
	}
	return esperarTipo(dec, tipo)
}

// atenderCon corre el nodo contra el guion y falla el test si alguno de los
// dos termina con error.
func atenderCon(t *testing.T, n *Nodo, guion func(*protocolo.Codificador, *protocolo.Decodificador) error) protocolo.Mensaje {
	t.Helper()
	nodoConn, coordConn := net.Pipe()
	hola, errs := coordinadorFalso(t, coordConn, guion)
	if err := n.Atender(nodoConn); err != nil {
		t.Fatalf("Atender: %v", err)
	}
	if err := <-errs; err != nil {
		t.Fatalf("coordinador: %v", err)
	}
	return <-hola
}

func cargar(desde, hasta int, huella string) protocolo.Mensaje {
	return protocolo.Mensaje{Tipo: protocolo.Cargar, Rango: &protocolo.Rango{Desde: desde, Hasta: hasta}, Huella: huella}
}

var fin = protocolo.Mensaje{Tipo: protocolo.Fin}

func TestAtenderSePresentaRespondePingYTerminaConFin(t *testing.T) {
	h := atenderCon(t, nodoDePrueba(""), func(cod *protocolo.Codificador, dec *protocolo.Decodificador) error {
		for range 3 {
			if _, err := pedir(cod, dec, protocolo.Mensaje{Tipo: protocolo.Ping}, protocolo.Pong); err != nil {
				return err
			}
		}
		return cod.Escribir(fin)
	})
	if h.Tipo != protocolo.Hola || h.Version != protocolo.Version || h.Nodo != "pc-07" || h.Workers != 3 {
		t.Fatalf("HOLA inesperado: %+v", h)
	}
}

func TestCargarConLaHuellaBuenaRespondeListo(t *testing.T) {
	ruta, _ := goldDePrueba(t, 30)
	huella := huellaDe(t, ruta)
	atenderCon(t, nodoDePrueba(ruta), func(cod *protocolo.Codificador, dec *protocolo.Decodificador) error {
		m, err := pedir(cod, dec, cargar(10, 25, huella), protocolo.Listo)
		if err != nil {
			return err
		}
		if m.N != 15 || m.Huella != huella {
			return errors.New("LISTO con otra cantidad de filas u otra huella: " + m.Huella)
		}
		return cod.Escribir(fin)
	})
}

func TestCargarConOtraHuellaRespondeErrorYSigueAtendiendo(t *testing.T) {
	ruta, _ := goldDePrueba(t, 30)
	atenderCon(t, nodoDePrueba(ruta), func(cod *protocolo.Codificador, dec *protocolo.Decodificador) error {
		m, err := pedir(cod, dec, cargar(0, 10, "sha256:"+strings.Repeat("ab", 32)), protocolo.Error)
		if err != nil {
			return err
		}
		if !strings.Contains(m.Mensaje, "huella") || !strings.Contains(m.Mensaje, ruta) {
			return errors.New("el ERROR no dice que la huella no coincide ni con qué archivo: " + m.Mensaje)
		}
		// Sin partición, un ITERAR también es un ERROR, y el nodo sigue vivo.
		iterar := protocolo.Mensaje{Tipo: protocolo.Iterar, Iter: 1, Centroides: []float64{0, 0}}
		if m, err = pedir(cod, dec, iterar, protocolo.Error); err != nil {
			return err
		}
		if !strings.Contains(m.Mensaje, "CARGAR") {
			return errors.New("el ERROR de un ITERAR sin partición no lo dice: " + m.Mensaje)
		}
		if _, err := pedir(cod, dec, protocolo.Mensaje{Tipo: protocolo.Ping}, protocolo.Pong); err != nil {
			return err
		}
		return cod.Escribir(fin)
	})
}

func TestCargarSinGoldRespondeUnErrorLegible(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "no-esta.csv")
	atenderCon(t, nodoDePrueba(ruta), func(cod *protocolo.Codificador, dec *protocolo.Decodificador) error {
		m, err := pedir(cod, dec, cargar(0, 10, "sha256:00"), protocolo.Error)
		if err != nil {
			return err
		}
		if !strings.Contains(m.Mensaje, ruta) || !strings.Contains(m.Mensaje, "-gold") {
			return errors.New("el ERROR no dice qué archivo falta ni cómo indicarlo: " + m.Mensaje)
		}
		return cod.Escribir(fin)
	})
}

func TestCargarMasAllaDelFinalTomaLasFilasConModulo(t *testing.T) {
	// Réplicas virtuales (docs/diseno.md, §4.3): la tabla tiene r × N filas y la
	// fila i es la fila i mod N del gold. Un rango que cruza el final del gold,
	// o que empieza después, se arma repitiendo filas.
	const n = 30
	ruta, g := goldDePrueba(t, n)
	huella := huellaDe(t, ruta)
	desde, hasta := 25, 97 // filas 25…29, tres vueltas completas y 0…6
	var x []float64
	for i := desde; i < hasta; i++ {
		x = append(x, g.Datos.Punto(i%n)...)
	}
	propias := &kmeans.Datos{X: x, N: hasta - desde, D: 2}
	cent := []float64{-2, -2, 0, 3, 4, 1}

	atenderCon(t, nodoDePrueba(ruta), func(cod *protocolo.Codificador, dec *protocolo.Decodificador) error {
		m, err := pedir(cod, dec, cargar(desde, hasta, huella), protocolo.Listo)
		if err != nil {
			return err
		}
		if m.N != hasta-desde {
			return fmt.Errorf("LISTO con %d filas, el rango virtual tiene %d", m.N, hasta-desde)
		}
		m, err = pedir(cod, dec, protocolo.Mensaje{Tipo: protocolo.Iterar, Iter: 1, Centroides: cent}, protocolo.Parcial)
		if err != nil {
			return err
		}
		mismoParcial(t, kmeans.Parcial{Sumas: m.Sumas, Conteos: m.Conteos, Inercia: m.Inercia},
			kmeans.ParcialesPorChunks(propias, cent, 1, 4))
		// Un rango que empieza después del final también vale.
		if m, err = pedir(cod, dec, cargar(3*n+5, 3*n+12, huella), protocolo.Listo); err != nil {
			return err
		}
		if m.N != 7 {
			return fmt.Errorf("LISTO con %d filas, se pidieron 7", m.N)
		}
		return cod.Escribir(fin)
	})
}

func TestIterarDevuelveElParcialDeSuRango(t *testing.T) {
	ruta, g := goldDePrueba(t, 60)
	huella := huellaDe(t, ruta)
	desde, hasta := 13, 47
	// Lo que el nodo tiene que calcular: sus filas, en chunks de 4 (el Chunk
	// del nodo), fusionadas en orden de chunk.
	propias := &kmeans.Datos{X: g.Datos.X[desde*2 : hasta*2], N: hasta - desde, D: 2}
	rondas := [][]float64{{-2, -2, 0, 3, 4, 1}, {-1.9, -2.6, 0.3, 2.4, 4.4, 0.4}}

	atenderCon(t, nodoDePrueba(ruta), func(cod *protocolo.Codificador, dec *protocolo.Decodificador) error {
		if _, err := pedir(cod, dec, cargar(desde, hasta, huella), protocolo.Listo); err != nil {
			return err
		}
		for i, cent := range rondas {
			m, err := pedir(cod, dec, protocolo.Mensaje{Tipo: protocolo.Iterar, Iter: i + 1, Centroides: cent}, protocolo.Parcial)
			if err != nil {
				return err
			}
			want := kmeans.ParcialesPorChunks(propias, cent, 1, 4)
			if m.Iter != i+1 {
				t.Errorf("ronda %d: el PARCIAL dice iter %d", i+1, m.Iter)
			}
			mismoParcial(t, kmeans.Parcial{Sumas: m.Sumas, Conteos: m.Conteos, Inercia: m.Inercia}, want)
			total := 0
			for _, c := range m.Conteos {
				total += c
			}
			if total != hasta-desde {
				t.Errorf("ronda %d: los conteos suman %d y el rango tiene %d filas", i+1, total, hasta-desde)
			}
		}
		return cod.Escribir(fin)
	})
}

func TestIterarConCentroidesQueNoSonKPorDRespondeError(t *testing.T) {
	ruta, _ := goldDePrueba(t, 30)
	huella := huellaDe(t, ruta)
	atenderCon(t, nodoDePrueba(ruta), func(cod *protocolo.Codificador, dec *protocolo.Decodificador) error {
		if _, err := pedir(cod, dec, cargar(0, 30, huella), protocolo.Listo); err != nil {
			return err
		}
		m, err := pedir(cod, dec, protocolo.Mensaje{Tipo: protocolo.Iterar, Iter: 1, Centroides: []float64{1, 2, 3}}, protocolo.Error)
		if err != nil {
			return err
		}
		if !strings.Contains(m.Mensaje, "3 centroides") {
			return errors.New("el ERROR no explica la forma: " + m.Mensaje)
		}
		return cod.Escribir(fin)
	})
}

func TestAtenderDevuelveElErrorDelCoordinador(t *testing.T) {
	nodoConn, coordConn := net.Pipe()
	_, errs := coordinadorFalso(t, coordConn, func(cod *protocolo.Codificador, _ *protocolo.Decodificador) error {
		return cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Error, Mensaje: "versión de protocolo 1, el coordinador habla la 2"})
	})

	err := nodoDePrueba("").Atender(nodoConn)
	if !errors.Is(err, ErrCoordinador) || !strings.Contains(err.Error(), "versión") {
		t.Fatalf("se esperaba ErrCoordinador con el texto del coordinador, llegó %v", err)
	}
	if err := <-errs; err != nil {
		t.Fatalf("coordinador: %v", err)
	}
}

func TestAtenderSinFinEsUnError(t *testing.T) {
	nodoConn, coordConn := net.Pipe()
	_, errs := coordinadorFalso(t, coordConn, func(*protocolo.Codificador, *protocolo.Decodificador) error {
		return nil // cierra la conexión sin FIN
	})

	if err := nodoDePrueba("").Atender(nodoConn); err == nil {
		t.Fatal("una conexión que se corta sin FIN tiene que ser un error")
	}
	if err := <-errs; err != nil {
		t.Fatalf("coordinador: %v", err)
	}
}

func TestCorrerReintentaHastaQueElCoordinadorEscucha(t *testing.T) {
	var intentos atomic.Int32
	llamar := func(context.Context) (net.Conn, error) {
		if intentos.Add(1) <= 2 {
			return nil, errors.New("connection refused")
		}
		nodoConn, coordConn := net.Pipe()
		coordinadorFalso(t, coordConn, func(cod *protocolo.Codificador, _ *protocolo.Decodificador) error {
			return cod.Escribir(fin)
		})
		return nodoConn, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := nodoDePrueba("").Correr(ctx, llamar, time.Millisecond, true); err != nil {
		t.Fatalf("Correr: %v", err)
	}
	if got := intentos.Load(); got != 3 {
		t.Fatalf("se esperaban 3 intentos de conexión, hubo %d", got)
	}
}

func TestCorrerVuelveALlamarDespuesDeCadaCorrida(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var corridas atomic.Int32
	llamar := func(context.Context) (net.Conn, error) {
		nodoConn, coordConn := net.Pipe()
		coordinadorFalso(t, coordConn, func(cod *protocolo.Codificador, _ *protocolo.Decodificador) error {
			if corridas.Add(1) == 3 {
				cancel() // a la tercera, el usuario aprieta Ctrl+C
			}
			return cod.Escribir(fin)
		})
		return nodoConn, nil
	}

	err := nodoDePrueba("").Correr(ctx, llamar, time.Millisecond, false)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("sin -una el nodo solo termina al cancelarlo; devolvió %v", err)
	}
	if got := corridas.Load(); got < 3 {
		t.Fatalf("se esperaban al menos 3 corridas, hubo %d", got)
	}
}

func TestCorrerSeDetieneConCtrlCAunqueNadieEscuche(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	llamar := func(context.Context) (net.Conn, error) { return nil, errors.New("connection refused") }

	err := nodoDePrueba("").Correr(ctx, llamar, 5*time.Millisecond, false)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("se esperaba el error del contexto, llegó %v", err)
	}
}

func mismoParcial(t *testing.T, got, want kmeans.Parcial) {
	t.Helper()
	if got.Inercia != want.Inercia || len(got.Sumas) != len(want.Sumas) || len(got.Conteos) != len(want.Conteos) {
		t.Fatalf("parcial %+v, se esperaba %+v", got, want)
	}
	for i := range want.Sumas {
		if got.Sumas[i] != want.Sumas[i] {
			t.Fatalf("parcial %+v, se esperaba %+v", got, want)
		}
	}
	for j := range want.Conteos {
		if got.Conteos[j] != want.Conteos[j] {
			t.Fatalf("parcial %+v, se esperaba %+v", got, want)
		}
	}
}
