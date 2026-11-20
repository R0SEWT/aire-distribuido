package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/R0SEWT/aire-distribuido/cluster"
	"github.com/R0SEWT/aire-distribuido/kmeans"
	"github.com/R0SEWT/aire-distribuido/protocolo"
)

var silencio = log.New(io.Discard, "", 0)

const chunkReceta = 64

// archivos escribe en un directorio temporal lo que dejan cmd/cargar y
// cmd/entrenar: gold sintético, estandarización, receta, centroides iniciales
// de la semilla 42 y un reports/kmeans.json con el Lloyd concurrente de esa
// semilla. Devuelve las opciones que apuntan ahí y la referencia.
func archivos(t *testing.T) (opciones, *kmeans.Resultado) {
	t.Helper()
	dir := t.TempDir()
	s, err := kmeans.GoldSintetico(kmeans.OpcionesSintetico{Features: []string{"pm2_5", "pm10"}, Nivel: kmeans.NivelAbsoluto,
		Estaciones: 3, Horas: 400, Ruido: 0.25, Semilla: 5})
	if err != nil {
		t.Fatal(err)
	}
	op := opciones{
		receta:          filepath.Join(dir, "configs/kmeans.json"),
		gold:            filepath.Join(dir, "data/gold/oefa_features.csv"),
		estandarizacion: filepath.Join(dir, "reports/estandarizacion.json"),
		inicios:         filepath.Join(dir, "reports/centroides_iniciales.json"),
		referencia:      filepath.Join(dir, "reports/kmeans.json"),
		salida:          filepath.Join(dir, "reports/kmeans_distribuido.json"),
		nodos:           1,
		replicas:        1,
		plazo:           5 * time.Second,
		plazoHola:       time.Second,
		plazoCarga:      5 * time.Second,
		plazoRonda:      2 * time.Second,
		tolerancia:      toleranciaPorOmision,
	}
	var gold bytes.Buffer
	if err := kmeans.EscribirGold(&gold, s.Gold); err != nil {
		t.Fatal(err)
	}
	escribir(t, op.gold, gold.Bytes())
	escribirJSON(t, op.estandarizacion, s.Est)
	escribir(t, op.receta, []byte(fmt.Sprintf(`{"features": ["pm2_5", "pm10"], "log1p": [], "nivel": "absoluto",
		"k": 3, "semilla": 42, "semillas": 10, "tolerancia": 1e-6, "max_iter": 100, "workers": 0, "chunk": %d}`, chunkReceta)))

	receta, err := kmeans.LeerReceta(op.receta)
	if err != nil {
		t.Fatal(err)
	}
	sha, err := kmeans.Sha256Archivo(op.gold)
	if err != nil {
		t.Fatal(err)
	}
	cent, kEf, err := kmeans.KMeansPP(s.Gold.Datos, 3, 42)
	if err != nil || kEf != 3 {
		t.Fatalf("KMeansPP: %d, %v", kEf, err)
	}
	inicios := kmeans.NuevoArchivoInicios(sha, receta, 2, []kmeans.Inicio{{Semilla: 42, KEfectivo: 3, Centroides: cent}})
	if err := kmeans.GuardarInicios(op.inicios, inicios); err != nil {
		t.Fatal(err)
	}
	conc := receta.OpcionesConc()
	conc.Workers = 2
	ref, err := kmeans.Concurrente(s.Gold.Datos, cent, conc)
	if err != nil {
		t.Fatal(err)
	}
	escribirReferencia(t, op.referencia, sha, ref)
	return op, ref
}

// escribirReferencia escribe los campos de reports/kmeans.json que el
// coordinador compara, con los nombres que usa cmd/entrenar.
func escribirReferencia(t *testing.T, ruta, sha string, r *kmeans.Resultado) {
	t.Helper()
	tamanos := make([]int, r.K)
	for _, j := range r.Asignaciones {
		tamanos[j]++
	}
	var cent [][]float64
	for j := range r.K {
		cent = append(cent, r.Centroide(j))
	}
	escribirJSON(t, ruta, map[string]any{
		"gold": map[string]any{"sha256": sha}, "semilla": 42, "chunk": chunkReceta,
		"iteraciones": r.Iteraciones, "paro": r.Paro, "inercia": r.Inercia, "inercias_por_iteracion": r.Inercias,
		"tamanos": tamanos, "centroides_estandarizados": cent,
		"arranques": []map[string]any{{"semilla": 42, "inercia": r.Inercia, "iteraciones": r.Iteraciones}},
	})
}

func escribir(t *testing.T, ruta string, b []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ruta, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func escribirJSON(t *testing.T, ruta string, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	escribir(t, ruta, b)
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

// lanzarNodos lanza n nodos de verdad (una corrida cada uno) con el gold de op.
// Devuelve por errs cómo terminó cada uno.
func lanzarNodos(t *testing.T, ln net.Listener, gold string, n int) <-chan error {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	errs := make(chan error, n)
	addr := ln.Addr().String()
	for i := range n {
		nd := &cluster.Nodo{Nombre: fmt.Sprintf("pc-%02d", i+1), Workers: 2, Chunk: chunkReceta, Gold: gold, Log: silencio}
		go func() {
			errs <- nd.Correr(ctx, func(ctx context.Context) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "tcp", addr)
			}, 5*time.Millisecond, true)
		}()
	}
	return errs
}

func leerSalida(t *testing.T, ruta string) salida {
	t.Helper()
	b, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("leer %s: %v", ruta, err)
	}
	var s salida
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("%s no es JSON válido: %v", ruta, err)
	}
	return s
}

func TestUnNodoDaElModeloDeEntrenarBitABit(t *testing.T) {
	op, ref := archivos(t)
	ln := escuchar(t)
	errs := lanzarNodos(t, ln, op.gold, 1)

	var out bytes.Buffer
	if err := correr(context.Background(), op, ln, silencio, &out, nil); err != nil {
		t.Fatalf("correr: %v\n%s", err, out.String())
	}
	if err := <-errs; err != nil {
		t.Fatalf("el nodo terminó con error: %v", err)
	}
	s := leerSalida(t, op.salida)
	c := s.Referencia
	if c == nil || !c.Comparable || !c.BitABit || !c.DentroDeTolerancia || c.DifMaxCentroides != 0 {
		t.Fatalf("con un nodo y el chunk de la receta tenía que ser bit a bit: %+v", c)
	}
	if s.Iteraciones != ref.Iteraciones || s.Inercia != ref.Inercia {
		t.Fatalf("%d iteraciones e inercia %v; la referencia %d y %v", s.Iteraciones, s.Inercia, ref.Iteraciones, ref.Inercia)
	}
	if len(s.Nodos) != 1 || s.Nodos[0].Rango != (protocolo.Rango{Desde: 0, Hasta: s.Gold.N}) {
		t.Fatalf("un nodo tenía que llevarse todo el gold: %+v", s.Nodos)
	}
	if !strings.Contains(out.String(), "BIT A BIT") {
		t.Fatalf("el resumen no dice que es bit a bit:\n%s", out.String())
	}
}

func TestTresNodosConReplicasVirtuales(t *testing.T) {
	op, ref := archivos(t)
	op.nodos, op.replicas = 3, 2
	ln := escuchar(t)
	errs := lanzarNodos(t, ln, op.gold, 3)

	var out bytes.Buffer
	if err := correr(context.Background(), op, ln, silencio, &out, nil); err != nil {
		t.Fatalf("correr: %v\n%s", err, out.String())
	}
	for range 3 {
		if err := <-errs; err != nil {
			t.Fatalf("un nodo terminó con error: %v", err)
		}
	}
	s := leerSalida(t, op.salida)
	n := len(ref.Asignaciones)
	if s.Gold.N != n || s.Gold.Replicas != 2 || s.Gold.FilasTabla != 2*n {
		t.Fatalf("gold %+v: se esperaban %d filas, 2 réplicas y %d filas en la tabla", s.Gold, n, 2*n)
	}
	desde := 0
	for i, nd := range s.Nodos {
		if nd.Rango.Desde != desde || nd.Filas != nd.Rango.Len() {
			t.Fatalf("nodo %d: rango %+v no sigue al anterior (%d)", i, nd.Rango, desde)
		}
		desde = nd.Rango.Hasta
	}
	if desde != 2*n || s.Nodos[0].Nodo != "pc-01" || s.Nodos[2].Nodo != "pc-03" {
		t.Fatalf("los rangos no cubren la tabla virtual en orden de nodo: %+v", s.Nodos)
	}
	c := s.Referencia
	if c == nil || !c.Comparable || !c.DentroDeTolerancia || !c.MismasIteraciones || !c.MismosTamanos || c.BitABit {
		t.Fatalf("con 3 nodos y 2 réplicas: dentro de tolerancia, mismas iteraciones y tamaños dobles, no bit a bit: %+v", c)
	}
	if len(s.MsRonda) != s.Iteraciones || len(s.MsFusion) != s.Iteraciones || s.MsRondas.P50 <= 0 {
		t.Fatalf("tiempos por iteración: %d de ronda y %d de fusión para %d iteraciones (p50 de ronda %v)",
			len(s.MsRonda), len(s.MsFusion), s.Iteraciones, s.MsRondas.P50)
	}
}

// nodoMudo se presenta, responde el PING de la espera y carga su rango como
// un nodo sano; después no responde nada.
func nodoMudo(t *testing.T, ln net.Listener) {
	t.Helper()
	go func() {
		conn, err := net.Dial("tcp", ln.Addr().String())
		if err != nil {
			return
		}
		defer conn.Close()
		cod, dec := protocolo.NuevoCodificador(conn), protocolo.NuevoDecodificador(conn)
		cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Hola, Version: protocolo.Version, Nodo: "pc-mudo", Workers: 1})
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

func TestUnNodoQueNoRespondeNoDejaUnModeloAMedias(t *testing.T) {
	op, _ := archivos(t)
	op.nodos, op.plazoRonda = 2, 200*time.Millisecond
	escribir(t, op.salida, []byte("modelo anterior\n"))
	ln := escuchar(t)
	errs := lanzarNodos(t, ln, op.gold, 1)
	nodoMudo(t, ln)

	err := correr(context.Background(), op, ln, silencio, io.Discard, nil)
	if !errors.Is(err, protocolo.ErrPlazoVencido) || !strings.Contains(err.Error(), `"pc-mudo"`) {
		t.Fatalf("se esperaba un plazo vencido que nombre a pc-mudo; llegó %v", err)
	}
	if b, _ := os.ReadFile(op.salida); string(b) != "modelo anterior\n" {
		t.Fatalf("una corrida abortada no puede tocar %s; quedó:\n%.200s", op.salida, b)
	}
	// El nodo sano se entera de por qué se cortó la corrida.
	if err := <-errs; !errors.Is(err, cluster.ErrCoordinador) || !strings.Contains(err.Error(), "pc-mudo") {
		t.Fatalf("el nodo sano tenía que recibir el ERROR del coordinador; terminó con %v", err)
	}
}

func TestSiVenceLaEsperaConMenosNodosNoHayCorrida(t *testing.T) {
	op, _ := archivos(t)
	op.nodos, op.plazo = 2, 300*time.Millisecond
	ln := escuchar(t)
	errs := lanzarNodos(t, ln, op.gold, 1)

	err := correr(context.Background(), op, ln, silencio, io.Discard, nil)
	if err == nil || !strings.Contains(err.Error(), "1 de 2") {
		t.Fatalf("se esperaba un error que diga cuántos llegaron; llegó %v", err)
	}
	if _, err := os.Stat(op.salida); !os.IsNotExist(err) {
		t.Fatalf("sin corrida no puede haber %s (%v)", op.salida, err)
	}
	if err := <-errs; !errors.Is(err, cluster.ErrCoordinador) {
		t.Fatalf("el nodo que llegó tenía que recibir un ERROR; terminó con %v", err)
	}
}

func TestConCtrlCSeCorreConLosQueLlegaron(t *testing.T) {
	op, _ := archivos(t)
	op.nodos = 3
	ln := escuchar(t)
	errs := lanzarNodos(t, ln, op.gold, 2)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(300 * time.Millisecond) // Ctrl+C cuando ya llegaron los dos
		cancel()
	}()
	if err := correr(ctx, op, ln, silencio, io.Discard, nil); err != nil {
		t.Fatalf("correr: %v", err)
	}
	for range 2 {
		if err := <-errs; err != nil {
			t.Fatalf("un nodo terminó con error: %v", err)
		}
	}
	if s := leerSalida(t, op.salida); s.NodosEsperados != 3 || len(s.Nodos) != 2 {
		t.Fatalf("se esperaban 3 nodos y se corrió con %d", len(s.Nodos))
	}
}

func TestUnaDiferenciaFueraDeToleranciaSeEscribeYSeAvisa(t *testing.T) {
	op, ref := archivos(t)
	// La referencia tiene un centroide corrido 1e-6: como si una fila hubiera
	// cambiado de cluster.
	sha, _ := kmeans.Sha256Archivo(op.gold)
	ref.Centroides[0] += 1e-6
	escribirReferencia(t, op.referencia, sha, ref)
	ln := escuchar(t)
	lanzarNodos(t, ln, op.gold, 1)

	var out bytes.Buffer
	err := correr(context.Background(), op, ln, silencio, &out, nil)
	if !errors.Is(err, errFueraDeTolerancia) {
		t.Fatalf("se esperaba errFueraDeTolerancia; llegó %v", err)
	}
	s := leerSalida(t, op.salida) // el modelo está completo: se escribe igual
	if c := s.Referencia; c.DentroDeTolerancia || c.DifMaxCentroides < 0.9e-6 {
		t.Fatalf("la comparación no ve la diferencia: %+v", c)
	}
	if !strings.Contains(out.String(), "FUERA DE TOLERANCIA") {
		t.Fatalf("el resumen no avisa:\n%s", out.String())
	}
}

func TestCompararConOtraSemillaOOtroGold(t *testing.T) {
	r := &kmeans.Resultado{K: 1, D: 2, Centroides: []float64{1, 2}, Inercia: 10, Inercias: []float64{12, 10}, Iteraciones: 2}
	ref := &referencia{Semilla: 42, Iteraciones: 2, Inercia: 10, Inercias: []float64{12, 10}, Tamanos: []int{5},
		Centroides: [][]float64{{1, 2}}, Arranques: []kmeans.Arranque{{Semilla: 42, Inercia: 10, Iteraciones: 2},
			{Semilla: 43, Inercia: 10.5, Iteraciones: 3}}}
	ref.Gold.Sha256 = "aaa"

	if c := comparar(ref, "ref.json", "aaa", 42, 1, r, []int{5}, 1e-9); !c.Comparable || !c.BitABit {
		t.Fatalf("misma semilla y mismo gold: %+v", c)
	}
	if c := comparar(ref, "ref.json", "bbb", 42, 1, r, []int{5}, 1e-9); c.Comparable || !strings.Contains(c.Motivo, "otro gold") {
		t.Fatalf("otro gold no es comparable: %+v", c)
	}
	c := comparar(ref, "ref.json", "aaa", 43, 1, r, []int{5}, 1e-9)
	if c.Comparable || c.MismasIteraciones || c.Iteraciones != 3 || !strings.Contains(c.Motivo, "semilla 42") {
		t.Fatalf("otra semilla: solo inercia e iteraciones del arranque 43: %+v", c)
	}
	if c := comparar(ref, "ref.json", "aaa", 99, 1, r, []int{5}, 1e-9); c.Comparable || c.Iteraciones != 0 {
		t.Fatalf("una semilla que no está: %+v", c)
	}
}

func TestReplicasQuePasanElTopeDeLosNodosSeRechazanSinEsperar(t *testing.T) {
	op, _ := archivos(t)
	p, err := preparar(op)
	if err != nil {
		t.Fatal(err)
	}
	n := p.gold.N
	// Con 4 nodos y el tope por omisión de cada uno, la tabla más grande tiene
	// 4 × MaxFilasPorOmision filas.
	op.nodos = 4
	maximo := 4 * cluster.MaxFilasPorOmision / n
	op.replicas = maximo + 1
	op.plazo = 30 * time.Second

	inicio := time.Now()
	err = correr(context.Background(), op, escuchar(t), silencio, io.Discard, nil)
	if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("-replicas %d", maximo)) ||
		!strings.Contains(err.Error(), "-max-filas") {
		t.Fatalf("se esperaba un error que diga el máximo (-replicas %d) y de dónde sale (-max-filas); llegó %v", maximo, err)
	}
	if d := time.Since(inicio); d > 5*time.Second {
		t.Fatalf("el error tardó %v: tiene que salir antes de esperar a los nodos", d)
	}
	op.replicas = maximo
	if _, err := preparar(op); err != nil {
		t.Fatalf("-replicas %d entra justo en el tope: %v", maximo, err)
	}
}

func TestElTopeDeFilasPorNodoEsConfigurable(t *testing.T) {
	op, _ := archivos(t)
	p, err := preparar(op)
	if err != nil {
		t.Fatal(err)
	}
	n := p.gold.N
	// Nodos que cargan hasta n filas: con 4 entran 4 réplicas y ni una más.
	op.nodos, op.maxFilas = 4, n
	op.replicas = 4
	if _, err := preparar(op); err != nil {
		t.Fatalf("4 réplicas entre 4 nodos de %d filas entran justo: %v", n, err)
	}
	op.replicas = 5
	if _, err := preparar(op); err == nil || !strings.Contains(err.Error(), "-replicas 4") ||
		!strings.Contains(err.Error(), fmt.Sprint(n)) {
		t.Fatalf("se esperaba un error con el máximo (-replicas 4) y el tope (%d); llegó %v", n, err)
	}
	// Un -replicas que desbordaría réplicas × n también se rechaza con el máximo.
	op.replicas = 1 << 62
	if _, err := preparar(op); err == nil || !strings.Contains(err.Error(), "-replicas 4") {
		t.Fatalf("se esperaba un error con el máximo (-replicas 4); llegó %v", err)
	}
	// Ni una copia: el error no propone -replicas 0.
	op.replicas, op.maxFilas = 1, n/4-1
	if _, err := preparar(op); err == nil || strings.Contains(err.Error(), "-replicas 0") ||
		!strings.Contains(err.Error(), "más nodos") {
		t.Fatalf("se esperaba un error que pida más nodos; llegó %v", err)
	}
}

func TestConCtrlCYMenosNodosElTopeSeCompruebaOtraVez(t *testing.T) {
	// Se pidieron 3 nodos y 3 réplicas: n filas por nodo, justo el tope. Con
	// Ctrl+C llegan 2 y a cada uno le tocarían 1,5 n: no se manda ningún CARGAR.
	op, _ := archivos(t)
	p, err := preparar(op)
	if err != nil {
		t.Fatal(err)
	}
	op.nodos, op.replicas, op.maxFilas = 3, 3, p.gold.N
	ln := escuchar(t)
	errs := lanzarNodos(t, ln, op.gold, 2)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(300 * time.Millisecond) // Ctrl+C cuando ya llegaron los dos
		cancel()
	}()

	err = correr(ctx, op, ln, silencio, io.Discard, nil)
	if err == nil || !strings.Contains(err.Error(), "con 2 nodos") || !strings.Contains(err.Error(), "-replicas 2") {
		t.Fatalf("se esperaba un error con el máximo para los 2 nodos que llegaron (-replicas 2); llegó %v", err)
	}
	if _, err := os.Stat(op.salida); !os.IsNotExist(err) {
		t.Fatalf("sin corrida no puede haber %s (%v)", op.salida, err)
	}
	for range 2 {
		if err := <-errs; !errors.Is(err, cluster.ErrCoordinador) || !strings.Contains(err.Error(), "-replicas 2") {
			t.Fatalf("cada nodo tenía que recibir el ERROR con el motivo; terminó con %v", err)
		}
	}
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

func TestUnNodoQueMuereTrasElHolaNoOcupaUnCupo(t *testing.T) {
	// -nodos 2. pc-00 se presenta y se apaga; después llegan pc-01 y pc-02.
	// La corrida sale con los dos vivos, no se aborta en la carga por pc-00.
	op, _ := archivos(t)
	op.nodos = 2
	ln := escuchar(t)
	llego := &avisador{marca: "] pc-00 ", visto: make(chan struct{})}
	errs := make(chan (<-chan error), 1)
	go func() {
		conn, err := net.Dial("tcp", ln.Addr().String())
		if err != nil {
			t.Error(err)
			return
		}
		hola := protocolo.Mensaje{Tipo: protocolo.Hola, Version: protocolo.Version, Nodo: "pc-00", Workers: 2}
		if err := protocolo.NuevoCodificador(conn).Escribir(hola); err != nil {
			t.Error(err)
		}
		conn.Close()
		<-llego.visto // el coordinador ya contó a pc-00: ahora llegan los vivos
		errs <- lanzarNodos(t, ln, op.gold, 2)
	}()

	if err := correr(context.Background(), op, ln, log.New(llego, "", 0), io.Discard, nil); err != nil {
		t.Fatalf("correr: %v", err)
	}
	s := leerSalida(t, op.salida)
	if len(s.Nodos) != 2 || s.Nodos[0].Nodo != "pc-01" || s.Nodos[1].Nodo != "pc-02" {
		t.Fatalf("la corrida tenía que salir con pc-01 y pc-02: %+v", s.Nodos)
	}
	vivos := <-errs
	for range 2 {
		if err := <-vivos; err != nil {
			t.Fatalf("un nodo terminó con error: %v", err)
		}
	}
}
