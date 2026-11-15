package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/R0SEWT/aire-distribuido/protocolo"
)

var silencio = log.New(io.Discard, "", 0)

// diasDePrueba son n días seguidos desde el 1 de enero de 2018.
func diasDePrueba(n int) []string {
	t0 := time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC)
	dias := make([]string, n)
	for i := range dias {
		dias[i] = t0.AddDate(0, 0, i).Format(time.DateOnly)
	}
	return dias
}

// archivos escribe en un directorio temporal lo que deja lab/mapa/: un
// manifiesto que lista 30 días (cinco bloques: cuatro de 7 y uno de 2), los pesos
// iniciales y los hiperparámetros con saltos de línea (tienen que viajar en
// una sola línea). Devuelve las opciones que apuntan ahí.
func archivos(t *testing.T, pesos0 []float32) opciones {
	t.Helper()
	dir := t.TempDir()
	op := opciones{
		nodos:          1,
		manifiesto:     filepath.Join(dir, "data/mapa/manifiesto.json"),
		pesosIniciales: filepath.Join(dir, "pesos_0.bin"),
		hiper:          filepath.Join(dir, "hiper.json"),
		validacion:     "CA-SB-01, CA-HU-02",
		salida:         filepath.Join(dir, "reports/mapa_entrenamiento.json"),
		pesosFinales:   filepath.Join(dir, "pesos_final.bin"),
		rondas:         3,
		pasos:          50,
		semilla:        42,
		tarea:          "mapa-prueba",
		plazo:          5 * time.Second,
		plazoHola:      time.Second,
		plazoTarea:     5 * time.Second,
		plazoRonda:     5 * time.Second,
	}
	// Como el de lab/mapa/datos/preparar.py: los días son los dias/AAAA-MM-DD.npy de "archivos".
	archivos := map[string]any{"estaticos.npy": map[string]any{"sha256": "ee", "bytes": 1}}
	for _, d := range diasDePrueba(30) {
		archivos["dias/"+d+".npy"] = map[string]any{"sha256": "aa", "bytes": 1}
	}
	man, _ := json.MarshalIndent(map[string]any{"version": 1, "grilla": map[string]int{"filas": 24, "columnas": 16},
		"dias": map[string]any{"ruta": "dias/AAAA-MM-DD.npy", "dtype": "float16"}, "archivos": archivos}, "", "  ")
	escribir(t, op.manifiesto, man)
	b, err := binary.Append(nil, binary.LittleEndian, pesos0)
	if err != nil {
		t.Fatal(err)
	}
	escribir(t, op.pesosIniciales, b)
	escribir(t, op.hiper, []byte("{\n  \"lote\": 8,\n  \"lr\": 0.001,\n  \"semilla\": 7\n}\n"))
	return op
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

func escuchar(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	return ln
}

// nodoFalso hace lo que hará lab/mapa/modelo/nodo_mapa.py, sin entrenar: se
// presenta con un HOLA v2, baja el manifiesto de la TAREA (ruta o URL) y
// comprueba su huella como lo haría con sus días, y en cada PESOS "entrena"
// sumando su constante a cada peso.
type nodoFalso struct {
	nombre   string
	suma     float32
	muestras int
	perdida  float64
	cerrarEn int // ronda en la que cierra la conexión al recibir el PESOS
	mudo     bool

	tarea chan protocolo.Mensaje // recibe la TAREA (con búfer)
}

func (nf *nodoFalso) lanzar(ln net.Listener) <-chan error {
	nf.tarea = make(chan protocolo.Mensaje, 1)
	fin := make(chan error, 1)
	go func() { fin <- nf.correr(ln.Addr().String()) }()
	return fin
}

func (nf *nodoFalso) correr(addr string) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	cod, dec := protocolo.NuevoCodificador(conn), protocolo.NuevoDecodificador(conn)
	if err := cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Hola, Version: protocolo.Version2, Nodo: nf.nombre,
		Workers: 1, GPU: "NVIDIA T1000 8GB", VramGB: 8}); err != nil {
		return err
	}
	for {
		m, err := dec.Leer()
		if err != nil {
			return err
		}
		var resp protocolo.Mensaje
		switch m.Tipo {
		case protocolo.Tarea:
			nf.tarea <- m
			if err := verificarManifiesto(m.Manifiesto, m.Huella); err != nil {
				resp = protocolo.Mensaje{Tipo: protocolo.Error, Mensaje: nf.nombre + ": " + err.Error()}
				break
			}
			resp = protocolo.Mensaje{Tipo: protocolo.Listo, Tarea: m.Tarea, Muestras: 100 * nf.muestras}
		case protocolo.Pesos:
			if m.Ronda == nf.cerrarEn {
				return errors.New("cerré a mitad de la ronda")
			}
			if nf.mudo {
				continue
			}
			v, err := m.Vector()
			if err != nil {
				return err
			}
			for i := range v {
				v[i] += nf.suma
			}
			resp = protocolo.Mensaje{Tipo: protocolo.Local, Ronda: m.Ronda, Muestras: nf.muestras,
				Perdida: nf.perdida, PerdidaVal: nf.perdida / 2, MsComputo: 3}.ConVector(v)
		case protocolo.Ping:
			resp = protocolo.Mensaje{Tipo: protocolo.Pong}
		case protocolo.Fin:
			return nil
		case protocolo.Error:
			return errors.New("ERROR: " + m.Mensaje)
		}
		if err := cod.Escribir(resp); err != nil {
			return err
		}
	}
}

func verificarManifiesto(fuente, huella string) error {
	var b []byte
	var err error
	if strings.HasPrefix(fuente, "http://") {
		var resp *http.Response
		if resp, err = http.Get(fuente); err == nil {
			b, err = io.ReadAll(resp.Body)
			resp.Body.Close()
		}
	} else {
		b, err = os.ReadFile(fuente)
	}
	if err != nil {
		return err
	}
	suma := sha256.Sum256(b)
	if got := "sha256:" + hex.EncodeToString(suma[:]); got != huella {
		return fmt.Errorf("mi manifiesto es %s y la TAREA dice %s", got, huella)
	}
	return nil
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

func leerPesosDe(t *testing.T, ruta string) []float32 {
	t.Helper()
	b, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatal(err)
	}
	v := make([]float32, len(b)/4)
	if _, err := binary.Decode(b, binary.LittleEndian, v); err != nil {
		t.Fatal(err)
	}
	return v
}

// Cada nodo suma una constante y declara otras muestras: 1·1 + 2·2 + 5·3 = 20
// y 20/8 = 2.5, así que tras R rondas los pesos son pesos₀ + 2.5·R exactos.
func TestTresNodosPromedianExactoYDejanElPuntoDeControl(t *testing.T) {
	pesos0 := []float32{0.5, -1, 1024, 0, 3.25}
	op := archivos(t, pesos0)
	op.nodos = 3
	ln := escuchar(t)
	nodos := []*nodoFalso{
		{nombre: "pc-03", suma: 3, muestras: 5, perdida: 0.5},
		{nombre: "pc-01", suma: 1, muestras: 1, perdida: 1},
		{nombre: "pc-02", suma: 2, muestras: 2, perdida: 0.25},
	}
	fines := make([]<-chan error, len(nodos))
	for i, nf := range nodos {
		fines[i] = nf.lanzar(ln)
	}

	var out bytes.Buffer
	if err := correr(context.Background(), op, ln, silencio, &out, nil); err != nil {
		t.Fatalf("correr: %v\n%s", err, out.String())
	}
	for i, fin := range fines {
		if err := <-fin; err != nil {
			t.Fatalf("nodo %s: %v", nodos[i].nombre, err)
		}
	}

	want := make([]float32, len(pesos0))
	for i, w := range pesos0 {
		want[i] = w + 2.5*3
	}
	if got := leerPesosDe(t, op.pesosFinales); !reflect.DeepEqual(got, want) {
		t.Fatalf("pesos finales %v, se esperaban %v", got, want)
	}

	// La TAREA: días repartidos sin repetir, validación e hiper reenviados.
	var todos []string
	for _, nf := range nodos {
		m := <-nf.tarea
		if m.Tarea != "mapa-prueba" || m.Manifiesto != op.manifiesto ||
			!reflect.DeepEqual(m.Validacion, []string{"CA-SB-01", "CA-HU-02"}) ||
			string(m.Hiper) != `{"lote":8,"lr":0.001,"semilla":7}` {
			t.Fatalf("TAREA de %s: %+v (hiper %s)", nf.nombre, m, m.Hiper)
		}
		todos = append(todos, m.Dias...)
	}
	slices.Sort(todos)
	if !reflect.DeepEqual(todos, diasDePrueba(30)) {
		t.Fatalf("los nodos recibieron %d días; tenían que ser los 30 del manifiesto, sin repetir", len(todos))
	}

	s := leerSalida(t, op.salida)
	if s.Estado != "completa" || s.Error != "" || len(s.Rondas) != 3 || s.Parametros != 5 || s.NodosEsperados != 3 {
		t.Fatalf("salida: estado %q, error %q, %d rondas, %d parámetros", s.Estado, s.Error, len(s.Rondas), s.Parametros)
	}
	if s.PesosFinales.Ronda != 3 || s.PesosFinales.Archivo != op.pesosFinales || len(s.PesosFinales.Sha256) != 64 {
		t.Errorf("punto de control: %+v", s.PesosFinales)
	}
	var hiper bytes.Buffer
	json.Compact(&hiper, s.Hiper)
	if s.Manifiesto.Dias != 30 || !strings.HasPrefix(s.Manifiesto.Huella, "sha256:") || hiper.String() != `{"lote":8,"lr":0.001,"semilla":7}` {
		t.Errorf("manifiesto %+v, hiper %s", s.Manifiesto, s.Hiper)
	}
	if len(s.Nodos) != 3 || s.Nodos[0].Nodo != "pc-01" || s.Nodos[2].Muestras != 500 || s.Nodos[1].GPU != "NVIDIA T1000 8GB" {
		t.Fatalf("nodos: %+v", s.Nodos)
	}
	for i, r := range s.Rondas {
		// (1·1 + 2·0.25 + 5·0.5)/8 = 0.5 y la mitad en validación.
		if r.Ronda != i+1 || r.Perdida != 0.5 || r.PerdidaVal != 0.25 || r.Muestras != 8 || r.Bytes != 3*2*4*5 {
			t.Errorf("ronda %d: %+v", i+1, r)
		}
		if len(r.Nodos) != 3 || r.Nodos[2].Nodo != "pc-03" || r.Nodos[2].MsComputo != 3 || r.Nodos[0].MsTotal <= 0 || r.MsBarrera <= 0 {
			t.Errorf("ronda %d, tiempos por nodo: %+v", i+1, r.Nodos)
		}
	}
	if !strings.Contains(out.String(), "completa") {
		t.Errorf("el resumen no dice que el entrenamiento terminó:\n%s", out.String())
	}
}

func TestElManifiestoPuedeVenirPorHTTP(t *testing.T) {
	op := archivos(t, []float32{1})
	contenido, err := os.ReadFile(op.manifiesto)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write(contenido) }))
	defer srv.Close()
	op.manifiesto, op.rondas, op.hiper, op.validacion = srv.URL+"/manifiesto.json", 1, "", ""
	ln := escuchar(t)
	nf := &nodoFalso{nombre: "pc-01", suma: 1, muestras: 1}
	fin := nf.lanzar(ln)
	if err := correr(context.Background(), op, ln, silencio, io.Discard, nil); err != nil {
		t.Fatal(err)
	}
	if err := <-fin; err != nil {
		t.Fatal(err)
	}
	if m := <-nf.tarea; m.Manifiesto != op.manifiesto || len(m.Dias) != 30 || m.Hiper != nil || m.Validacion != nil {
		t.Fatalf("TAREA: %+v", m)
	}
	if got := leerPesosDe(t, op.pesosFinales); !reflect.DeepEqual(got, []float32{2}) {
		t.Fatalf("pesos finales %v", got)
	}
}

func TestUnNodoQueCierraAbortaYDejaLaUltimaRondaCompleta(t *testing.T) {
	op := archivos(t, []float32{0, 1})
	op.nodos, op.rondas = 2, 4
	ln := escuchar(t)
	sano := (&nodoFalso{nombre: "pc-01", suma: 1, muestras: 1}).lanzar(ln)
	(&nodoFalso{nombre: "pc-02", suma: 3, muestras: 1, cerrarEn: 2}).lanzar(ln)

	err := correr(context.Background(), op, ln, silencio, io.Discard, nil)
	var en *protocolo.ErrorNodo
	if err == nil || !errors.As(err, &en) || en.Nodo != "pc-02" || !strings.Contains(err.Error(), "ronda 2") {
		t.Fatalf("se esperaba un error de la ronda 2 que nombre a pc-02; salió %v", err)
	}
	// Punto de control: los pesos de la ronda 1, (1 + 3)/2 = 2 más los iniciales.
	if got := leerPesosDe(t, op.pesosFinales); !reflect.DeepEqual(got, []float32{2, 3}) {
		t.Fatalf("pesos finales %v; tenían que ser los de la ronda 1", got)
	}
	s := leerSalida(t, op.salida)
	if s.Estado != "abortada" || !strings.Contains(s.Error, "pc-02") || len(s.Rondas) != 1 || s.PesosFinales.Ronda != 1 {
		t.Fatalf("salida: estado %q, error %q, %d rondas, punto de control de la ronda %d", s.Estado, s.Error, len(s.Rondas), s.PesosFinales.Ronda)
	}
	if err := <-sano; err == nil || !strings.Contains(err.Error(), "pc-02") {
		t.Fatalf("el nodo sano tenía que recibir un ERROR que nombre a pc-02; terminó con %v", err)
	}
}

func TestUnNodoMudoVenceElPlazoDeLaRonda(t *testing.T) {
	op := archivos(t, []float32{0})
	op.nodos, op.plazoRonda = 2, 200*time.Millisecond
	ln := escuchar(t)
	(&nodoFalso{nombre: "pc-01", suma: 1, muestras: 1}).lanzar(ln)
	(&nodoFalso{nombre: "pc-mudo", suma: 1, muestras: 1, mudo: true}).lanzar(ln)

	inicio := time.Now()
	err := correr(context.Background(), op, ln, silencio, io.Discard, nil)
	if !errors.Is(err, protocolo.ErrPlazoVencido) || !strings.Contains(err.Error(), `"pc-mudo"`) {
		t.Fatalf("se esperaba un plazo vencido que nombre a pc-mudo; salió %v", err)
	}
	if d := time.Since(inicio); d > 3*time.Second {
		t.Errorf("tardó %v con un plazo de ronda de %v", d, op.plazoRonda)
	}
	if _, err := os.Stat(op.pesosFinales); !os.IsNotExist(err) {
		t.Errorf("sin ninguna ronda completa no hay punto de control (%v)", err)
	}
	if s := leerSalida(t, op.salida); s.Estado != "abortada" || len(s.Rondas) != 0 {
		t.Errorf("salida: estado %q, %d rondas", s.Estado, len(s.Rondas))
	}
}

func TestSiVenceLaEsperaConMenosNodosNoHayCorrida(t *testing.T) {
	op := archivos(t, []float32{0})
	op.nodos, op.plazo = 2, 300*time.Millisecond
	ln := escuchar(t)
	fin := (&nodoFalso{nombre: "pc-01", suma: 1, muestras: 1}).lanzar(ln)

	err := correr(context.Background(), op, ln, silencio, io.Discard, nil)
	if err == nil || !strings.Contains(err.Error(), "1 de 2") {
		t.Fatalf("se esperaba un error que diga cuántos llegaron; salió %v", err)
	}
	if _, err := os.Stat(op.salida); !os.IsNotExist(err) {
		t.Fatalf("sin corrida no puede haber %s (%v)", op.salida, err)
	}
	if err := <-fin; err == nil || !strings.Contains(err.Error(), "ERROR") {
		t.Fatalf("el nodo que llegó tenía que recibir un ERROR; terminó con %v", err)
	}
}

func TestConCtrlCSeEntrenaConLosQueLlegaron(t *testing.T) {
	op := archivos(t, []float32{0})
	op.nodos, op.rondas = 3, 1
	ln := escuchar(t)
	fines := []<-chan error{
		(&nodoFalso{nombre: "pc-01", suma: 1, muestras: 1}).lanzar(ln),
		(&nodoFalso{nombre: "pc-02", suma: 3, muestras: 1}).lanzar(ln),
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(300 * time.Millisecond) // Ctrl+C cuando ya llegaron los dos
		cancel()
	}()
	if err := correr(ctx, op, ln, silencio, io.Discard, nil); err != nil {
		t.Fatalf("correr: %v", err)
	}
	for _, fin := range fines {
		if err := <-fin; err != nil {
			t.Fatal(err)
		}
	}
	if s := leerSalida(t, op.salida); s.NodosEsperados != 3 || len(s.Nodos) != 2 {
		t.Fatalf("se esperaban 3 nodos y se entrenó con %d", len(s.Nodos))
	}
}

// Un error en los archivos se ve antes de esperar a los nodos.
func TestPrepararRechazaEntradasMalas(t *testing.T) {
	casos := map[string]struct {
		cambiar func(t *testing.T, op *opciones)
		texto   string
	}{
		"pesos que no son float32": {func(t *testing.T, op *opciones) { escribir(t, op.pesosIniciales, []byte{1, 2, 3}) }, "múltiplo de 4"},
		"pesos vacíos":             {func(t *testing.T, op *opciones) { escribir(t, op.pesosIniciales, nil) }, "vacío"},
		"pesos con NaN": {func(t *testing.T, op *opciones) {
			b, _ := binary.Append(nil, binary.LittleEndian, []float32{1, float32(math.NaN())})
			escribir(t, op.pesosIniciales, b)
		}, "posición 1"},
		"hiper roto":              {func(t *testing.T, op *opciones) { escribir(t, op.hiper, []byte(`{"lote":`)) }, "hiper"},
		"validación con hueco":    {func(t *testing.T, op *opciones) { op.validacion = "CA-SB-01,,CA-HU-02" }, "estación vacía"},
		"menos bloques que nodos": {func(t *testing.T, op *opciones) { op.nodos = 6 }, "5 bloques"},
		"sin rondas":              {func(t *testing.T, op *opciones) { op.rondas = 0 }, "rondas"},
		"sin pasos":               {func(t *testing.T, op *opciones) { op.pasos = 0 }, "pasos"},
		"manifiesto que no está":  {func(t *testing.T, op *opciones) { op.manifiesto += ".no" }, "manifiesto"},
	}
	for nombre, c := range casos {
		t.Run(nombre, func(t *testing.T) {
			op := archivos(t, []float32{1, 2})
			c.cambiar(t, &op)
			if _, err := preparar(op); err == nil || !strings.Contains(err.Error(), c.texto) {
				t.Fatalf("se esperaba un error con %q, salió %v", c.texto, err)
			}
		})
	}
}
