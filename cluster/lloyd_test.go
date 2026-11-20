package cluster

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/R0SEWT/aire-distribuido/kmeans"
	"github.com/R0SEWT/aire-distribuido/protocolo"
)

// goldConRuido es un gold de n filas cuyos valores NO son exactos en binario
// (senos): las sumas dependen del orden, como en el gold real.
func goldConRuido(t *testing.T, n int) (string, *kmeans.Gold) {
	t.Helper()
	g := &kmeans.Gold{Datos: &kmeans.Datos{D: 2}, Features: []string{"pm2_5", "pm10"}}
	t0 := time.Date(2024, 1, 1, 0, 0, 0, 0, kmeans.ZonaPeru)
	for i := range n {
		cx, cy := [3]float64{-1.3, 0.2, 2.7}[i%3], [3]float64{-0.9, 2.1, 0.4}[i%3]
		g.Datos.X = append(g.Datos.X, cx+1.7*math.Sin(float64(i)*0.37), cy+1.9*math.Cos(float64(i)*0.53))
		g.Datos.N++
		g.Estacion = append(g.Estacion, fmt.Sprintf("CA-E%02d-01", i/500))
		g.FechaHora = append(g.FechaHora, t0.Add(time.Duration(i%500)*time.Hour))
	}
	ruta := filepath.Join(t.TempDir(), "oefa_features.csv")
	f, err := os.Create(ruta)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := kmeans.EscribirGold(f, g); err != nil {
		t.Fatal(err)
	}
	return ruta, g
}

func marcar(addr string) func(context.Context) (net.Conn, error) {
	return func(ctx context.Context) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, "tcp", addr)
	}
}

// prueba es un clúster en loopback: k nodos de verdad (Nodo.Correr, una sola
// corrida cada uno) con el gold de ruta, presentados, con su rango cargado.
type prueba struct {
	enlaces []*protocolo.Enlace
	cortar  []context.CancelFunc // cortar[i] mata al nodo i: cierra su conexión
	errs    chan error
}

func levantar(t *testing.T, ruta string, n, k, chunk int) *prueba {
	t.Helper()
	ln := escuchar(t)
	p := &prueba{errs: make(chan error, k)}
	for i := range k {
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		p.cortar = append(p.cortar, cancel)
		nd := &Nodo{Nombre: fmt.Sprintf("nodo-%d", i+1), Workers: 1 + i%3, Chunk: chunk, Gold: ruta, Log: silencio}
		go func() { p.errs <- nd.Correr(ctx, marcar(ln.Addr().String()), 5*time.Millisecond, true) }()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ms := EsperarNodos(ctx, ln, k, time.Second, silencio)
	if len(ms) != k {
		t.Fatalf("llegaron %d de %d nodos", len(ms), k)
	}
	p.enlaces = Enlaces(ms)
	t.Cleanup(func() {
		for _, e := range p.enlaces {
			e.Cerrar()
		}
	})
	rangos, err := Repartir(n, k)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Cargar(p.enlaces, rangos, huellaDe(t, ruta), time.Now().Add(5*time.Second)); err != nil {
		t.Fatalf("Cargar: %v", err)
	}
	return p
}

// terminar cierra la corrida con FIN y comprueba que cada nodo la dio por buena.
func (p *prueba) terminar(t *testing.T) {
	t.Helper()
	if err := Terminar(p.enlaces, time.Second); err != nil {
		t.Fatalf("Terminar: %v", err)
	}
	for range p.enlaces {
		if err := <-p.errs; err != nil {
			t.Fatalf("un nodo terminó con error: %v", err)
		}
	}
}

func opcionesLloyd(k int) kmeans.Opciones {
	return kmeans.Opciones{K: k, MaxIter: 100, TolAbs: 1e-9, TolRel: 1e-9}
}

func concurrente(t *testing.T, d *kmeans.Datos, cent []float64, chunk int) *kmeans.Resultado {
	t.Helper()
	r, err := kmeans.Concurrente(d, cent, kmeans.OpcionesConc{Opciones: opcionesLloyd(len(cent) / d.D), Workers: 3, Chunk: chunk})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func inicio(t *testing.T, d *kmeans.Datos, k int, semilla int64) []float64 {
	t.Helper()
	cent, kEf, err := kmeans.KMeansPP(d, k, semilla)
	if err != nil || kEf != k {
		t.Fatalf("KMeansPP: k efectivo %d, %v", kEf, err)
	}
	return cent
}

func difMax(a, b []float64) float64 {
	m := 0.0
	for i := range a {
		m = max(m, math.Abs(a[i]-b[i]))
	}
	return m
}

func mismosBits(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if math.Float64bits(a[i]) != math.Float64bits(b[i]) {
			return false
		}
	}
	return true
}

func comprobarCorrida(t *testing.T, c *Corrida, n int) {
	t.Helper()
	r := c.Resultado
	if len(c.Rondas) != r.Iteraciones || len(c.Fusiones) != r.Iteraciones || len(r.Inercias) != r.Iteraciones ||
		c.UltimaRonda != r.Iteraciones+1 {
		t.Fatalf("%d rondas y %d fusiones medidas, %d inercias, %d iteraciones y última ronda %d",
			len(c.Rondas), len(c.Fusiones), len(r.Inercias), r.Iteraciones, c.UltimaRonda)
	}
	for i := range c.Rondas {
		// Una ronda de red nunca mide 0; la fusión puede, con un reloj grueso.
		if c.Rondas[i] <= 0 || c.Fusiones[i] < 0 {
			t.Fatalf("iteración %d: ronda %v y fusión %v", i+1, c.Rondas[i], c.Fusiones[i])
		}
	}
	total := 0
	for _, x := range c.Tamanos {
		total += x
	}
	if total != n {
		t.Fatalf("los tamaños suman %d y el gold tiene %d filas", total, n)
	}
}

func TestLloydConUnNodoEsElConcurrenteBitABit(t *testing.T) {
	// Un nodo con todo el gold y el mismo chunk suma exactamente en el mismo
	// orden que Concurrente: la fusión del coordinador es 0 + su parcial.
	const n, chunk = 2000, 64
	ruta, g := goldConRuido(t, n)
	cent := inicio(t, g.Datos, 3, 42)
	want := concurrente(t, g.Datos, cent, chunk)

	p := levantar(t, ruta, n, 1, chunk)
	c, err := Lloyd(p.enlaces, cent, opcionesLloyd(3), 2*time.Second, 1)
	if err != nil {
		t.Fatalf("Lloyd: %v", err)
	}
	p.terminar(t)
	comprobarCorrida(t, c, n)
	r := c.Resultado
	if !mismosBits(r.Centroides, want.Centroides) || !mismosBits(r.Inercias, want.Inercias) ||
		math.Float64bits(r.Inercia) != math.Float64bits(want.Inercia) {
		t.Fatalf("no es bit a bit el concurrente: dif. máx. de centroides %.3g, inercia %.17g vs %.17g",
			difMax(r.Centroides, want.Centroides), r.Inercia, want.Inercia)
	}
	if r.Iteraciones != want.Iteraciones || r.Paro != want.Paro {
		t.Fatalf("%d iteraciones (%s), Concurrente %d (%s)", r.Iteraciones, r.Paro, want.Iteraciones, want.Paro)
	}
}

func TestLloydConDatosExactosEsBitABitConCualquierCantidadDeNodos(t *testing.T) {
	// Con valores múltiplos de 1/4 toda suma de filas es exacta, así que los
	// centroides no dependen del árbol de suma: un rango mal repartido, una
	// fila perdida o una fusión fuera de orden se verían aquí sin tolerancia.
	// La inercia suma distancias a centroides que no son exactos: esa sí se
	// compara con tolerancia.
	const n, chunk = 3000, 64
	ruta, g := goldDePrueba(t, n)
	cent := inicio(t, g.Datos, 3, 7)
	want := concurrente(t, g.Datos, cent, chunk)
	if want.Iteraciones < 3 {
		t.Fatalf("los datos convergen en %d iteraciones: el test no prueba nada", want.Iteraciones)
	}
	t.Logf("Concurrente converge en %d iteraciones", want.Iteraciones)

	for _, k := range []int{1, 2, 3, 5} {
		p := levantar(t, ruta, n, k, chunk)
		c, err := Lloyd(p.enlaces, cent, opcionesLloyd(3), 2*time.Second, 1)
		if err != nil {
			t.Fatalf("%d nodos: Lloyd: %v", k, err)
		}
		p.terminar(t)
		comprobarCorrida(t, c, n)
		r := c.Resultado
		if !mismosBits(r.Centroides, want.Centroides) || r.Iteraciones != want.Iteraciones {
			t.Fatalf("%d nodos: %d iteraciones (Concurrente %d), dif. máx. de centroides %.3g",
				k, r.Iteraciones, want.Iteraciones, difMax(r.Centroides, want.Centroides))
		}
		if d := math.Abs(r.Inercia-want.Inercia) / want.Inercia; d > 1e-12 {
			t.Fatalf("%d nodos: inercia %.17g, Concurrente %.17g (dif. relativa %.3g)", k, r.Inercia, want.Inercia, d)
		}
	}
}

// lloydEnMemoria es la cuenta que hace el clúster, hecha en un solo proceso:
// el parcial de cada rango en chunks, la fusión en orden de rango y
// CerrarIteracion. Es la referencia bit a bit del distribuido con esos rangos.
func lloydEnMemoria(d *kmeans.Datos, rangos []protocolo.Rango, cent0 []float64, op kmeans.Opciones, chunk int) *kmeans.Resultado {
	cent := slices.Clone(cent0)
	r := &kmeans.Resultado{}
	parcial := func() kmeans.Parcial {
		var ps []kmeans.Parcial
		for _, rg := range rangos {
			propias := &kmeans.Datos{X: d.X[rg.Desde*d.D : rg.Hasta*d.D], N: rg.Len(), D: d.D}
			ps = append(ps, kmeans.ParcialesPorChunks(propias, cent, 1, chunk))
		}
		return kmeans.Fusionar(ps, op.K, d.D)
	}
	for it := 1; it <= op.MaxIter; it++ {
		if kmeans.CerrarIteracion(r, cent, parcial(), it, op) {
			break
		}
	}
	r.Inercia = parcial().Inercia
	r.Centroides = cent
	return r
}

func TestLloydConVariosNodosCoincideConToleranciaYEsDeterminista(t *testing.T) {
	// Con datos reales el árbol de suma cambia con la cantidad de nodos y los
	// últimos bits también: la igualdad con el concurrente es con tolerancia
	// (docs/diseno.md, §4.4). Con los mismos nodos y rangos es bit a bit, y es
	// exactamente la cuenta de lloydEnMemoria: rangos, chunks y fusión en orden
	// de nodo.
	const n, chunk = 2000, 64
	ruta, g := goldConRuido(t, n)
	cent := inicio(t, g.Datos, 3, 42)
	want := concurrente(t, g.Datos, cent, chunk)
	rangos, _ := Repartir(n, 3)
	exacto := lloydEnMemoria(g.Datos, rangos, cent, opcionesLloyd(3), chunk)

	var primera *kmeans.Resultado
	for range 2 {
		p := levantar(t, ruta, n, 3, chunk)
		c, err := Lloyd(p.enlaces, cent, opcionesLloyd(3), 2*time.Second, 1)
		if err != nil {
			t.Fatalf("Lloyd: %v", err)
		}
		p.terminar(t)
		comprobarCorrida(t, c, n)
		r := c.Resultado
		if r.Iteraciones != want.Iteraciones {
			t.Fatalf("%d iteraciones, Concurrente %d", r.Iteraciones, want.Iteraciones)
		}
		if d := difMax(r.Centroides, want.Centroides); d > 1e-12 {
			t.Fatalf("dif. máx. de centroides con el concurrente %.3g", d)
		}
		if !mismosBits(r.Centroides, exacto.Centroides) || !mismosBits(r.Inercias, exacto.Inercias) ||
			r.Inercia != exacto.Inercia || r.Iteraciones != exacto.Iteraciones {
			t.Fatalf("no es la cuenta en memoria con los mismos rangos: dif. máx. de centroides %.3g",
				difMax(r.Centroides, exacto.Centroides))
		}
		if primera == nil {
			primera = r
			continue
		}
		if !mismosBits(r.Centroides, primera.Centroides) || !mismosBits(r.Inercias, primera.Inercias) {
			t.Fatal("dos corridas con los mismos nodos y rangos no dieron lo mismo bit a bit")
		}
	}
}

func TestReplicasVirtualesDanLosMismosCentroides(t *testing.T) {
	// Con r réplicas virtuales la tabla tiene r × N filas y la fila i es la
	// i mod N (docs/diseno.md, §4.3). Cada fila aparece r veces, así que sumas y
	// conteos se multiplican por r y los centroides son los mismos salvo
	// redondeo; también las iteraciones. La inercia es r veces la de una copia y
	// cada cluster tiene r veces sus filas, exactamente.
	const n, chunk, r = 2000, 64, 3
	ruta, g := goldConRuido(t, n)
	cent := inicio(t, g.Datos, 3, 42)
	want := concurrente(t, g.Datos, cent, chunk)
	tamanos := make([]int, 3)
	for _, j := range want.Asignaciones {
		tamanos[j]++
	}

	// Dos nodos sobre 6000 filas virtuales: los dos rangos cruzan el final del gold.
	p := levantar(t, ruta, r*n, 2, chunk)
	c, err := Lloyd(p.enlaces, cent, opcionesLloyd(3), 2*time.Second, 1)
	if err != nil {
		t.Fatalf("Lloyd: %v", err)
	}
	p.terminar(t)
	comprobarCorrida(t, c, r*n)
	res := c.Resultado
	if res.Iteraciones != want.Iteraciones {
		t.Fatalf("%d iteraciones con %d réplicas, %d con una", res.Iteraciones, r, want.Iteraciones)
	}
	if d := difMax(res.Centroides, want.Centroides); d > 1e-12 {
		t.Fatalf("con %d réplicas los centroides se apartan %.3g de los de una", r, d)
	}
	if d := math.Abs(res.Inercia/r-want.Inercia) / want.Inercia; d > 1e-12 {
		t.Fatalf("inercia %.17g con %d réplicas; %d × %.17g (dif. relativa %.3g)", res.Inercia, r, r, want.Inercia, d)
	}
	for j := range tamanos {
		if c.Tamanos[j] != r*tamanos[j] {
			t.Fatalf("tamaños %v con %d réplicas; con una %v", c.Tamanos, r, tamanos)
		}
	}
}

func TestUnNodoCaidoAbortaLaCorridaConUnErrorQueLoNombra(t *testing.T) {
	const n = 600
	ruta, g := goldConRuido(t, n)
	p := levantar(t, ruta, n, 3, 64)
	p.cortar[1]() // se apaga nodo-2 después de cargar su rango

	c, err := Lloyd(p.enlaces, inicio(t, g.Datos, 3, 1), opcionesLloyd(3), 2*time.Second, 1)
	if c != nil || err == nil {
		t.Fatalf("una corrida con un nodo caído no puede devolver un modelo (err %v)", err)
	}
	if !strings.Contains(err.Error(), `"nodo-2"`) || strings.Contains(err.Error(), `"nodo-1"`) {
		t.Fatalf("el error tiene que nombrar al nodo caído y solo a él: %v", err)
	}
}

// nodoMudo se presenta, responde el PING de la espera y carga su rango como
// un nodo sano; después no responde nada.
func nodoMudo(t *testing.T, addr string) {
	t.Helper()
	go func() {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			return
		}
		defer conn.Close()
		cod, dec := protocolo.NuevoCodificador(conn), protocolo.NuevoDecodificador(conn)
		cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Hola, Version: protocolo.Version, Nodo: "mudo", Workers: 1})
		for {
			m, err := dec.Leer()
			if err != nil {
				return
			}
			switch m.Tipo {
			case protocolo.Ping:
				cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Pong})
			case protocolo.Cargar:
				cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Listo, N: m.Rango.Len(), Huella: m.Huella})
			}
		}
	}()
}

func TestUnNodoQueNoRespondeVenceElPlazoDeLaRonda(t *testing.T) {
	const n = 600
	ruta, g := goldConRuido(t, n)
	ln := escuchar(t)
	sano := &Nodo{Nombre: "a-sano", Workers: 2, Chunk: 64, Gold: ruta, Log: silencio}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go sano.Correr(ctx, marcar(ln.Addr().String()), 5*time.Millisecond, true)
	nodoMudo(t, ln.Addr().String())

	espera, cancelEspera := context.WithTimeout(ctx, 5*time.Second)
	defer cancelEspera()
	enlaces := Enlaces(EsperarNodos(espera, ln, 2, time.Second, silencio))
	defer Terminar(enlaces, time.Second)
	rangos, _ := Repartir(n, len(enlaces))
	if _, err := Cargar(enlaces, rangos, huellaDe(t, ruta), time.Now().Add(5*time.Second)); err != nil {
		t.Fatalf("Cargar: %v", err)
	}

	inicioRonda := time.Now()
	c, err := Lloyd(enlaces, inicio(t, g.Datos, 3, 1), opcionesLloyd(3), 200*time.Millisecond, 1)
	if c != nil || !errors.Is(err, protocolo.ErrPlazoVencido) || !strings.Contains(err.Error(), `"mudo"`) {
		t.Fatalf("se esperaba un plazo vencido que nombre al nodo mudo; llegó %v", err)
	}
	if d := time.Since(inicioRonda); d > 2*time.Second {
		t.Fatalf("la ronda tardó %v en abortar con un plazo de 200 ms", d)
	}
}

func TestCargarRechazaUnNodoConOtroGold(t *testing.T) {
	const n = 300
	ruta, _ := goldConRuido(t, n)
	otro, _ := goldDePrueba(t, n)
	ln := escuchar(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for i, gold := range []string{ruta, otro} {
		nd := &Nodo{Nombre: fmt.Sprintf("nodo-%d", i+1), Workers: 1, Chunk: 64, Gold: gold, Log: silencio}
		go nd.Correr(ctx, marcar(ln.Addr().String()), 5*time.Millisecond, true)
	}
	espera, cancelEspera := context.WithTimeout(ctx, 5*time.Second)
	defer cancelEspera()
	enlaces := Enlaces(EsperarNodos(espera, ln, 2, time.Second, silencio))
	defer Terminar(enlaces, time.Second)
	rangos, _ := Repartir(n, 2)

	_, err := Cargar(enlaces, rangos, huellaDe(t, ruta), time.Now().Add(5*time.Second))
	if !errors.Is(err, protocolo.ErrInformado) || !strings.Contains(err.Error(), `"nodo-2"`) ||
		!strings.Contains(err.Error(), "huella") {
		t.Fatalf("se esperaba el ERROR de huella de nodo-2; llegó %v", err)
	}
}
