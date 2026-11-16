package protocolo_test

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/R0SEWT/aire-distribuido/protocolo"
)

// unoDeCadaTipoV2 tiene un mensaje válido de cada tipo nuevo o ampliado de la v2.
func unoDeCadaTipoV2() []protocolo.Mensaje {
	return []protocolo.Mensaje{
		{Tipo: protocolo.Hola, Version: protocolo.Version2, Nodo: "pc-07", Workers: 1, GPU: "NVIDIA T1000 8GB", VramGB: 8},
		{Tipo: protocolo.Tarea, Tarea: "mapa-1", Manifiesto: "http://10.11.37.73:8000/manifiesto.json",
			Huella: "sha256:ab12", Dias: []string{"2018-01-01", "2018-01-02"}, Validacion: []string{"CA-SB-01"},
			Hiper: json.RawMessage(`{"lote":8,"lr":0.001,"semilla":7}`)},
		{Tipo: protocolo.Listo, Tarea: "mapa-1", Muestras: 1234},
		protocolo.Mensaje{Tipo: protocolo.Pesos, Ronda: 3, Pasos: 200}.ConVector([]float32{0.5, -1.25, 3}),
		protocolo.Mensaje{Tipo: protocolo.Local, Ronda: 3, Muestras: 1600, Perdida: 0.25, PerdidaVal: 0.5,
			MsComputo: 81234.5}.ConVector([]float32{0.75, -1, 3.5}),
	}
}

func TestIdaYVueltaDeLosMensajesV2(t *testing.T) {
	for _, m := range unoDeCadaTipoV2() {
		t.Run(string(m.Tipo), func(t *testing.T) {
			var buf bytes.Buffer
			if err := protocolo.NuevoCodificador(&buf).Escribir(m); err != nil {
				t.Fatalf("Escribir: %v", err)
			}
			linea, carga, ok := bytes.Cut(buf.Bytes(), []byte("\n"))
			if !ok {
				t.Fatalf("no hay línea JSON: %q", buf.Bytes())
			}
			if len(carga) != len(m.Carga) || !bytes.Equal(carga, m.Carga) {
				t.Fatalf("después del '\\n' tienen que ir exactamente los %d bytes de la carga; van %d", len(m.Carga), len(carga))
			}
			conBytes := strings.Contains(string(linea), fmt.Sprintf(`"bytes":%d`, len(m.Carga)))
			if (len(m.Carga) > 0) != conBytes {
				t.Errorf("la línea tiene que llevar \"bytes\" si y solo si hay carga: %s", linea)
			}

			dec := protocolo.NuevoDecodificador(&buf)
			got, err := dec.Leer()
			if err != nil {
				t.Fatalf("Leer: %v", err)
			}
			if !reflect.DeepEqual(got, m) {
				t.Errorf("ida y vuelta:\n got %+v\nwant %+v", got, m)
			}
			if _, err := dec.Leer(); !errors.Is(err, io.EOF) {
				t.Errorf("después del único mensaje se esperaba io.EOF, salió %v", err)
			}
		})
	}
}

// Un vector de varios MB cruza el cable sin cambiar un bit, y el flujo sigue
// sincronizado: el mensaje de la v1 que viene detrás se lee entero.
func TestCargaGrandeCruzaElCableBitABit(t *testing.T) {
	const n = 3 << 20 // 12 MB de float32
	v := make([]float32, n)
	raros := []float32{0, float32(math.Copysign(0, -1)), math.SmallestNonzeroFloat32, math.MaxFloat32,
		-math.MaxFloat32, 1.0 / 3, float32(math.Inf(1)), math.Float32frombits(0x7fc00001)} // un NaN con carga
	for i := range v {
		if i < len(raros) {
			v[i] = raros[i]
		} else {
			v[i] = float32(i%9973) * 0.001
		}
	}
	coord, nodo := net.Pipe()
	defer coord.Close()
	escrito := make(chan error, 1)
	go func() {
		defer nodo.Close()
		cod := protocolo.NuevoCodificador(nodo)
		local := protocolo.Mensaje{Tipo: protocolo.Local, Ronda: 1, Muestras: 10}.ConVector(v)
		if err := cod.Escribir(local); err != nil {
			escrito <- err
			return
		}
		escrito <- cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Pong})
	}()

	dec := protocolo.NuevoDecodificador(coord)
	m, err := dec.LeerConPlazo(30 * time.Second)
	if err != nil {
		t.Fatalf("Leer LOCAL: %v", err)
	}
	if m.N != n || m.Bytes != 4*n {
		t.Fatalf("LOCAL con n=%d y bytes=%d; se esperaban %d y %d", m.N, m.Bytes, n, 4*n)
	}
	got, err := m.Vector()
	if err != nil {
		t.Fatal(err)
	}
	for i := range v {
		if math.Float32bits(got[i]) != math.Float32bits(v[i]) {
			t.Fatalf("posición %d: %v (%#x) volvió como %v (%#x)", i, v[i], math.Float32bits(v[i]), got[i], math.Float32bits(got[i]))
		}
	}
	if m, err := dec.LeerConPlazo(5 * time.Second); err != nil || m.Tipo != protocolo.Pong {
		t.Fatalf("detrás de la carga venía un PONG: %+v, %v", m, err)
	}
	if err := <-escrito; err != nil {
		t.Fatal(err)
	}
}

// Los float32 viajan en little-endian, como los escribe numpy en un PC x86.
func TestElVectorEsFloat32LittleEndian(t *testing.T) {
	m := protocolo.Mensaje{Tipo: protocolo.Pesos, Ronda: 1, Pasos: 1}.ConVector([]float32{1, -2})
	want := binary.LittleEndian.AppendUint32(binary.LittleEndian.AppendUint32(nil, math.Float32bits(1)), math.Float32bits(-2))
	if !bytes.Equal(m.Carga, want) || m.N != 2 || m.Bytes != 8 {
		t.Fatalf("carga %x (n=%d, bytes=%d); se esperaba %x, n=2, bytes=8", m.Carga, m.N, m.Bytes, want)
	}
	if _, err := (protocolo.Mensaje{Tipo: protocolo.Pesos, N: 3, Bytes: 8, Carga: want}).Vector(); err == nil {
		t.Error("8 bytes no son 3 float32: Vector debió fallar")
	}
}

func TestCargaTruncadaEsUnErrorClaroYPegajoso(t *testing.T) {
	var buf bytes.Buffer
	m := protocolo.Mensaje{Tipo: protocolo.Pesos, Ronda: 2, Pasos: 50}.ConVector(make([]float32, 100))
	if err := protocolo.NuevoCodificador(&buf).Escribir(m); err != nil {
		t.Fatal(err)
	}
	truncado := buf.Bytes()[:buf.Len()-150] // faltan 150 de los 400 bytes
	dec := protocolo.NuevoDecodificador(bytes.NewReader(truncado))
	_, err := dec.Leer()
	if !errors.Is(err, io.ErrUnexpectedEOF) || !strings.Contains(err.Error(), "250 de 400") || !strings.Contains(err.Error(), "PESOS") {
		t.Fatalf("se esperaba io.ErrUnexpectedEOF que diga cuántos bytes llegaron; salió %v", err)
	}
	if _, err2 := dec.Leer(); !errors.Is(err2, io.ErrUnexpectedEOF) {
		t.Errorf("el error tiene que ser pegajoso; la segunda lectura dio %v", err2)
	}
}

func TestCargaMayorQueMaxCargaSeRechaza(t *testing.T) {
	// Al leer: se rechaza por la cabecera, sin reservar 256 MB, y el flujo
	// queda inservible (no se puede saltar una carga que no se va a leer).
	n := protocolo.MaxCarga/4 + 1
	linea := fmt.Sprintf(`{"tipo":"LOCAL","ronda":1,"n":%d,"bytes":%d,"muestras":1}`+"\n", n, 4*n)
	dec := protocolo.NuevoDecodificador(strings.NewReader(linea + "basura"))
	_, err := dec.Leer()
	if !errors.Is(err, protocolo.ErrMensajeInvalido) || !strings.Contains(err.Error(), "MaxCarga") {
		t.Fatalf("se esperaba ErrMensajeInvalido que nombre MaxCarga; salió %v", err)
	}
	if _, err2 := dec.Leer(); !errors.Is(err2, protocolo.ErrMensajeInvalido) {
		t.Errorf("el error tiene que ser pegajoso; la segunda lectura dio %v", err2)
	}

	// Al escribir: no sale nada.
	var buf bytes.Buffer
	grande := protocolo.Mensaje{Tipo: protocolo.Pesos, Ronda: 1, Pasos: 1, N: n, Bytes: 4 * n, Carga: []byte{1, 2, 3, 4}}
	if err := protocolo.NuevoCodificador(&buf).Escribir(grande); !errors.Is(err, protocolo.ErrMensajeInvalido) ||
		!strings.Contains(err.Error(), "MaxCarga") {
		t.Fatalf("se esperaba ErrMensajeInvalido que nombre MaxCarga; salió %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("un mensaje rechazado no escribe nada; salió %d bytes", buf.Len())
	}
}

func TestElCodificadorNoMandaMensajesV2Invalidos(t *testing.T) {
	v := []float32{1, 2}
	pesos := protocolo.Mensaje{Tipo: protocolo.Pesos, Ronda: 1, Pasos: 10}.ConVector(v)
	local := protocolo.Mensaje{Tipo: protocolo.Local, Ronda: 1, Muestras: 3}.ConVector(v)
	tarea := protocolo.Mensaje{Tipo: protocolo.Tarea, Tarea: "t", Manifiesto: "m.json", Huella: "sha256:ab", Dias: []string{"2018-01-01"}}
	con := func(m protocolo.Mensaje, f func(*protocolo.Mensaje)) protocolo.Mensaje { f(&m); return m }
	casos := map[string]protocolo.Mensaje{
		"HOLA con vram negativa":     {Tipo: protocolo.Hola, Version: 2, Nodo: "n", VramGB: -1},
		"HOLA con carga":             protocolo.Mensaje{Tipo: protocolo.Hola, Version: 2, Nodo: "n"}.ConVector(v),
		"TAREA sin id":               con(tarea, func(m *protocolo.Mensaje) { m.Tarea = "" }),
		"TAREA sin manifiesto":       con(tarea, func(m *protocolo.Mensaje) { m.Manifiesto = "" }),
		"TAREA sin huella":           con(tarea, func(m *protocolo.Mensaje) { m.Huella = "" }),
		"TAREA sin días":             con(tarea, func(m *protocolo.Mensaje) { m.Dias = nil }),
		"TAREA con un día mal":       con(tarea, func(m *protocolo.Mensaje) { m.Dias = []string{"2018-1-1"} }),
		"TAREA con estación vacía":   con(tarea, func(m *protocolo.Mensaje) { m.Validacion = []string{""} }),
		"TAREA con hiper roto":       con(tarea, func(m *protocolo.Mensaje) { m.Hiper = json.RawMessage(`{"lote":`) }),
		"LISTO v2 muestras negativa": {Tipo: protocolo.Listo, Tarea: "t", Muestras: -1},
		"PESOS sin ronda":            con(pesos, func(m *protocolo.Mensaje) { m.Ronda = 0 }),
		"PESOS sin pasos":            con(pesos, func(m *protocolo.Mensaje) { m.Pasos = 0 }),
		"PESOS sin carga":            {Tipo: protocolo.Pesos, Ronda: 1, Pasos: 1, N: 2, Bytes: 8},
		"PESOS bytes no es 4n":       con(pesos, func(m *protocolo.Mensaje) { m.N = 3 }),
		"PESOS bytes no es la carga": con(pesos, func(m *protocolo.Mensaje) { m.Bytes = 4 }),
		"LOCAL sin ronda":            con(local, func(m *protocolo.Mensaje) { m.Ronda = 0 }),
		"LOCAL muestras negativas":   con(local, func(m *protocolo.Mensaje) { m.Muestras = -1 }),
		"LOCAL pérdida negativa":     con(local, func(m *protocolo.Mensaje) { m.Perdida = -0.5 }),
		"LOCAL pérdida NaN":          con(local, func(m *protocolo.Mensaje) { m.PerdidaVal = math.NaN() }),
		"LOCAL ms_computo negativo":  con(local, func(m *protocolo.Mensaje) { m.MsComputo = -1 }),
	}
	for nombre, m := range casos {
		t.Run(nombre, func(t *testing.T) {
			var buf bytes.Buffer
			err := protocolo.NuevoCodificador(&buf).Escribir(m)
			if !errors.Is(err, protocolo.ErrMensajeInvalido) {
				t.Fatalf("se esperaba ErrMensajeInvalido, salió %v", err)
			}
			if buf.Len() != 0 {
				t.Errorf("un mensaje inválido no debe escribir nada, salió %q", buf.String())
			}
		})
	}
}

// Un mensaje con "bytes" que su tipo no admite se rechaza, pero su carga se
// consume: el mensaje siguiente se lee bien.
func TestUnaCargaFueraDeLugarSeRechazaSinDesincronizar(t *testing.T) {
	flujo := `{"tipo":"PONG","bytes":4}` + "\nabcd" + `{"tipo":"PING"}` + "\n"
	dec := protocolo.NuevoDecodificador(strings.NewReader(flujo))
	if _, err := dec.Leer(); !errors.Is(err, protocolo.ErrMensajeInvalido) {
		t.Fatalf("un PONG con carga es inválido; salió %v", err)
	}
	if m, err := dec.Leer(); err != nil || m.Tipo != protocolo.Ping {
		t.Fatalf("el PING de después tenía que leerse entero: %+v, %v", m, err)
	}
	if _, err := protocolo.NuevoDecodificador(strings.NewReader(`{"tipo":"PESOS","ronda":1,"pasos":1,"n":1,"bytes":-4}` + "\n")).Leer(); !errors.Is(err, protocolo.ErrMensajeInvalido) {
		t.Errorf("bytes negativo: se esperaba ErrMensajeInvalido, salió %v", err)
	}
}

// Un plazo que vence a mitad de la carga no pierde lo ya leído: la lectura
// siguiente la completa, igual que con una línea a medias.
func TestPlazoVencidoAMitadDeLaCargaNoDesincroniza(t *testing.T) {
	coord, nodo := net.Pipe()
	defer coord.Close()
	defer nodo.Close()

	var buf bytes.Buffer
	v := make([]float32, 1000)
	for i := range v {
		v[i] = float32(i) / 7
	}
	if err := protocolo.NuevoCodificador(&buf).Escribir(protocolo.Mensaje{Tipo: protocolo.Local, Ronda: 4, Muestras: 9}.ConVector(v)); err != nil {
		t.Fatal(err)
	}
	todo := buf.Bytes()
	corte := bytes.IndexByte(todo, '\n') + 1 + 1500 // la línea y 1500 de los 4000 bytes
	escrito := make(chan error, 1)
	go func() {
		_, err := nodo.Write(todo[:corte])
		escrito <- err
	}()

	dec := protocolo.NuevoDecodificador(coord)
	if _, err := dec.LeerConPlazo(50 * time.Millisecond); !errors.Is(err, protocolo.ErrPlazoVencido) {
		t.Fatalf("se esperaba ErrPlazoVencido, salió %v", err)
	}
	if err := <-escrito; err != nil {
		t.Fatal(err)
	}
	go func() {
		_, err := nodo.Write(todo[corte:])
		escrito <- err
	}()
	m, err := dec.LeerConPlazo(5 * time.Second)
	if err != nil {
		t.Fatalf("después del plazo vencido: %v", err)
	}
	got, err := m.Vector()
	if err != nil || m.Ronda != 4 || !reflect.DeepEqual(got, v) {
		t.Fatalf("el LOCAL se reconstruyó mal: ronda %d, %v", m.Ronda, err)
	}
	if err := <-escrito; err != nil {
		t.Fatal(err)
	}
}

// El formato en el cable de la v2: la línea JSON y, detrás del '\n', la carga.
func ExampleMensaje_ConVector() {
	var buf bytes.Buffer
	cod := protocolo.NuevoCodificador(&buf)
	cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Hola, Version: protocolo.Version2, Nodo: "pc-07", Workers: 1,
		GPU: "NVIDIA T1000 8GB", VramGB: 8})
	cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Tarea, Tarea: "mapa-1", Manifiesto: "http://10.11.37.73:8000/manifiesto.json",
		Huella: "sha256:ab12", Dias: []string{"2018-01-08", "2018-01-09"}, Validacion: []string{"CA-SB-01"},
		Hiper: json.RawMessage(`{"lote": 8, "lr": 0.001}`)})
	cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Listo, Tarea: "mapa-1", Muestras: 1234})
	cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Pesos, Ronda: 1, Pasos: 200}.ConVector([]float32{1, -2}))
	cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Local, Ronda: 1, Muestras: 1600, Perdida: 0.25, PerdidaVal: 0.5,
		MsComputo: 812.5}.ConVector([]float32{1.5, -2}))
	for len(buf.Bytes()) > 0 {
		linea, _ := buf.ReadBytes('\n')
		fmt.Print(string(linea))
		var m struct{ Bytes int }
		json.Unmarshal(linea, &m)
		if m.Bytes > 0 {
			fmt.Printf("%x\n", buf.Next(m.Bytes))
		}
	}
	// Output:
	// {"tipo":"HOLA","version":2,"nodo":"pc-07","workers":1,"gpu":"NVIDIA T1000 8GB","vram_gb":8}
	// {"tipo":"TAREA","huella":"sha256:ab12","tarea":"mapa-1","manifiesto":"http://10.11.37.73:8000/manifiesto.json","dias":["2018-01-08","2018-01-09"],"validacion":["CA-SB-01"],"hiper":{"lote":8,"lr":0.001}}
	// {"tipo":"LISTO","tarea":"mapa-1","muestras":1234}
	// {"tipo":"PESOS","n":2,"ronda":1,"pasos":200,"bytes":8}
	// 0000803f000000c0
	// {"tipo":"LOCAL","n":2,"ronda":1,"bytes":8,"muestras":1600,"perdida":0.25,"perdida_val":0.5,"ms_computo":812.5}
	// 0000c03f000000c0
}
