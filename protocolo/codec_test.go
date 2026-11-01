package protocolo_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/R0SEWT/aire-distribuido/protocolo"
)

// unoDeCadaTipo tiene un mensaje válido de cada uno de los nueve tipos.
func unoDeCadaTipo() []protocolo.Mensaje {
	return []protocolo.Mensaje{
		{Tipo: protocolo.Hola, Version: protocolo.Version, Nodo: "gorgo", Workers: 11},
		{Tipo: protocolo.Cargar, Rango: &protocolo.Rango{Desde: 0, Hasta: 250000}, Huella: "sha256:ab12"},
		{Tipo: protocolo.Listo, N: 250000, Huella: "sha256:ab12"},
		{Tipo: protocolo.Iterar, Iter: 7, Centroides: []float64{0.5, -1.25, 3, 0}},
		{Tipo: protocolo.Parcial, Iter: 7, Sumas: []float64{10.5, -2, 0, 4}, Conteos: []int{3, 0}, Inercia: 12.75},
		{Tipo: protocolo.Fin},
		{Tipo: protocolo.Ping},
		{Tipo: protocolo.Pong},
		{Tipo: protocolo.Error, Mensaje: "rango fuera de la tabla: tiene 1000 filas"},
	}
}

func TestIdaYVueltaDeCadaMensaje(t *testing.T) {
	for _, m := range unoDeCadaTipo() {
		t.Run(string(m.Tipo), func(t *testing.T) {
			var buf bytes.Buffer
			if err := protocolo.NuevoCodificador(&buf).Escribir(m); err != nil {
				t.Fatalf("Escribir: %v", err)
			}
			linea := buf.String()
			if !strings.HasSuffix(linea, "\n") || strings.Count(linea, "\n") != 1 {
				t.Fatalf("se esperaba exactamente una línea terminada en \\n, salió %q", linea)
			}
			if !strings.Contains(linea, `"tipo":"`+string(m.Tipo)+`"`) {
				t.Errorf("la línea no lleva su tipo: %q", linea)
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

// La fusión del coordinador es determinista solo si el cable no cambia ningún
// bit. encoding/json escribe la representación más corta que se relee igual.
func TestLosFloatCruzanElCableBitABit(t *testing.T) {
	raros := []float64{
		0.1, 1.0 / 3, -2.0 / 3, math.Pi, 1e-300, 5e-324, // el subnormal más chico
		math.MaxFloat64, -math.MaxFloat64, math.SmallestNonzeroFloat64,
		math.Copysign(0, -1), // -0 no es 0 bit a bit
		123456789.123456789, 4531798.747970126,
	}
	var buf bytes.Buffer
	m := protocolo.Mensaje{Tipo: protocolo.Iterar, Iter: 1, Centroides: raros}
	if err := protocolo.NuevoCodificador(&buf).Escribir(m); err != nil {
		t.Fatal(err)
	}
	got, err := protocolo.NuevoDecodificador(&buf).Leer()
	if err != nil {
		t.Fatal(err)
	}
	for i := range raros {
		if math.Float64bits(got.Centroides[i]) != math.Float64bits(raros[i]) {
			t.Errorf("posición %d: %v (%#x) volvió como %v (%#x)", i,
				raros[i], math.Float64bits(raros[i]), got.Centroides[i], math.Float64bits(got.Centroides[i]))
		}
	}
}

func TestElCodificadorNoMandaMensajesInvalidos(t *testing.T) {
	casos := map[string]protocolo.Mensaje{
		"sin tipo":                 {},
		"tipo desconocido":         {Tipo: "ITERA"},
		"HOLA sin nodo":            {Tipo: protocolo.Hola, Version: 1},
		"HOLA sin versión":         {Tipo: protocolo.Hola, Nodo: "gorgo"},
		"CARGAR sin rango":         {Tipo: protocolo.Cargar, Huella: "x"},
		"CARGAR con rango vacío":   {Tipo: protocolo.Cargar, Rango: &protocolo.Rango{Desde: 5, Hasta: 5}, Huella: "x"},
		"CARGAR sin huella":        {Tipo: protocolo.Cargar, Rango: &protocolo.Rango{Desde: 0, Hasta: 5}},
		"LISTO sin filas":          {Tipo: protocolo.Listo, Huella: "x"},
		"ITERAR sin iter":          {Tipo: protocolo.Iterar, Centroides: []float64{1}},
		"ITERAR sin centroides":    {Tipo: protocolo.Iterar, Iter: 1},
		"ITERAR con NaN":           {Tipo: protocolo.Iterar, Iter: 1, Centroides: []float64{1, math.NaN()}},
		"ITERAR con Inf":           {Tipo: protocolo.Iterar, Iter: 1, Centroides: []float64{math.Inf(1)}},
		"PARCIAL sin conteos":      {Tipo: protocolo.Parcial, Iter: 1, Sumas: []float64{1}},
		"PARCIAL que no cuadra":    {Tipo: protocolo.Parcial, Iter: 1, Sumas: []float64{1, 2, 3}, Conteos: []int{1, 1}},
		"PARCIAL conteo negativo":  {Tipo: protocolo.Parcial, Iter: 1, Sumas: []float64{1, 2}, Conteos: []int{-1, 1}},
		"PARCIAL inercia negativa": {Tipo: protocolo.Parcial, Iter: 1, Sumas: []float64{1}, Conteos: []int{1}, Inercia: -1},
		"ERROR sin texto":          {Tipo: protocolo.Error},
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

func TestElDecodificadorRechazaLineasInvalidas(t *testing.T) {
	casos := map[string]string{
		"no es JSON":         "hola\n",
		"campo desconocido":  `{"tipo":"PING","prioridad":1}` + "\n",
		"tipo desconocido":   `{"tipo":"ITERA","iter":1}` + "\n",
		"dos objetos":        `{"tipo":"PING"}{"tipo":"PONG"}` + "\n",
		"falta un campo":     `{"tipo":"ITERAR","iter":3}` + "\n",
		"conteo no entero":   `{"tipo":"PARCIAL","iter":1,"sumas":[1],"conteos":[1.5]}` + "\n",
		"centroide en texto": `{"tipo":"ITERAR","iter":1,"centroides":["NaN"]}` + "\n",
	}
	for nombre, linea := range casos {
		t.Run(nombre, func(t *testing.T) {
			_, err := protocolo.NuevoDecodificador(strings.NewReader(linea)).Leer()
			if !errors.Is(err, protocolo.ErrMensajeInvalido) {
				t.Fatalf("se esperaba ErrMensajeInvalido, salió %v", err)
			}
		})
	}
}

func TestLineaDemasiadoLargaSeCorta(t *testing.T) {
	larga := `{"tipo":"ERROR","mensaje":"` + strings.Repeat("x", protocolo.MaxLinea) + `"}` + "\n"
	dec := protocolo.NuevoDecodificador(strings.NewReader(larga))
	if _, err := dec.Leer(); !errors.Is(err, protocolo.ErrMensajeInvalido) {
		t.Fatalf("se esperaba ErrMensajeInvalido, salió %v", err)
	}
	// El error es pegajoso: el flujo quedó a mitad de una línea.
	if _, err := dec.Leer(); !errors.Is(err, protocolo.ErrMensajeInvalido) {
		t.Errorf("la segunda lectura debió repetir el error, salió %v", err)
	}
}

func TestCierreLimpioYCierreAMitadDeMensaje(t *testing.T) {
	limpio := protocolo.NuevoDecodificador(strings.NewReader(`{"tipo":"FIN"}` + "\n"))
	if m, err := limpio.Leer(); err != nil || m.Tipo != protocolo.Fin {
		t.Fatalf("Leer = %+v, %v", m, err)
	}
	if _, err := limpio.Leer(); err != io.EOF {
		t.Errorf("cierre entre mensajes: se esperaba io.EOF, salió %v", err)
	}

	truncado := protocolo.NuevoDecodificador(strings.NewReader(`{"tipo":"PARCIAL","iter":3,"sum`))
	if _, err := truncado.Leer(); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("cierre a mitad de mensaje: se esperaba io.ErrUnexpectedEOF, salió %v", err)
	}
}

func TestSinPlazosSeDiceConClaridad(t *testing.T) {
	if _, err := protocolo.NuevoDecodificador(strings.NewReader("")).LeerConPlazo(time.Second); !errors.Is(err, protocolo.ErrSinPlazos) {
		t.Errorf("leer: se esperaba ErrSinPlazos, salió %v", err)
	}
	err := protocolo.NuevoCodificador(io.Discard).EscribirAntesDe(protocolo.Mensaje{Tipo: protocolo.Ping}, time.Now())
	if !errors.Is(err, protocolo.ErrSinPlazos) {
		t.Errorf("escribir: se esperaba ErrSinPlazos, salió %v", err)
	}
}

// Un plazo que vence a mitad de una línea no puede perder lo ya leído: el
// coordinador sigue usando la conexión del nodo lento en la ronda siguiente.
func TestPlazoVencidoNoDesincronizaElFlujo(t *testing.T) {
	coord, nodo := net.Pipe()
	defer coord.Close()
	defer nodo.Close()

	linea := `{"tipo":"PARCIAL","iter":4,"sumas":[1.5,2.5],"conteos":[2],"inercia":0.25}` + "\n"
	mitad := len(linea) / 2
	escrito := make(chan error, 1)
	go func() {
		_, err := io.WriteString(nodo, linea[:mitad])
		escrito <- err
	}()

	dec := protocolo.NuevoDecodificador(coord)
	_, err := dec.LeerConPlazo(50 * time.Millisecond)
	if !errors.Is(err, protocolo.ErrPlazoVencido) {
		t.Fatalf("se esperaba ErrPlazoVencido, salió %v", err)
	}
	if err := <-escrito; err != nil {
		t.Fatal(err)
	}

	go func() {
		_, err := io.WriteString(nodo, linea[mitad:])
		escrito <- err
	}()
	m, err := dec.LeerConPlazo(time.Second)
	if err != nil {
		t.Fatalf("después del plazo vencido: %v", err)
	}
	if m.Iter != 4 || !reflect.DeepEqual(m.Sumas, []float64{1.5, 2.5}) || m.Inercia != 0.25 {
		t.Errorf("el mensaje se reconstruyó mal: %+v", m)
	}
	if err := <-escrito; err != nil {
		t.Fatal(err)
	}
}

// En un nodo escriben a la vez la goroutine que responde PONG y la que manda el
// PARCIAL. Ninguna línea puede salir mezclada con otra. Con -race.
func TestEscritoresConcurrentesNoMezclanLineas(t *testing.T) {
	const escritores, porEscritor = 8, 200
	lector, escritor := net.Pipe()
	defer lector.Close()

	cod := protocolo.NuevoCodificador(escritor)
	var wg sync.WaitGroup
	wg.Add(escritores)
	for g := 0; g < escritores; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < porEscritor; i++ {
				m := protocolo.Mensaje{Tipo: protocolo.Error, Mensaje: fmt.Sprintf("%d-%d", g, i)}
				if err := cod.Escribir(m); err != nil {
					t.Errorf("escritor %d: %v", g, err)
					return
				}
			}
		}()
	}
	go func() {
		wg.Wait()
		escritor.Close()
	}()

	siguiente := make([]int, escritores) // cada escritor tiene que llegar en su orden
	dec := protocolo.NuevoDecodificador(lector)
	total := 0
	for {
		m, err := dec.Leer()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("mensaje %d: %v", total, err)
		}
		var g, i int
		if _, err := fmt.Sscanf(m.Mensaje, "%d-%d", &g, &i); err != nil {
			t.Fatalf("mensaje mezclado: %q", m.Mensaje)
		}
		if i != siguiente[g] {
			t.Fatalf("escritor %d: llegó el %d y se esperaba el %d", g, i, siguiente[g])
		}
		siguiente[g]++
		total++
	}
	if total != escritores*porEscritor {
		t.Errorf("llegaron %d mensajes, se esperaban %d", total, escritores*porEscritor)
	}
}

// El formato en el cable, tal cual. Sirve de documentación y de prueba.
func ExampleCodificador() {
	var buf bytes.Buffer
	cod := protocolo.NuevoCodificador(&buf)
	cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Hola, Version: protocolo.Version, Nodo: "colab-1", Workers: 2})
	cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Cargar, Rango: &protocolo.Rango{Desde: 0, Hasta: 1000}, Huella: "sha256:ab12"})
	cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Listo, N: 1000, Huella: "sha256:ab12"})
	cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Iterar, Iter: 3, Centroides: []float64{0.1, -1.5, 2, 0.25}})
	cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Parcial, Iter: 3, Sumas: []float64{4.5, -9, 30, 2.5}, Conteos: []int{600, 400}, Inercia: 812.0625})
	cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Fin})
	fmt.Print(buf.String())
	// Output:
	// {"tipo":"HOLA","version":1,"nodo":"colab-1","workers":2}
	// {"tipo":"CARGAR","rango":{"desde":0,"hasta":1000},"huella":"sha256:ab12"}
	// {"tipo":"LISTO","huella":"sha256:ab12","n":1000}
	// {"tipo":"ITERAR","iter":3,"centroides":[0.1,-1.5,2,0.25]}
	// {"tipo":"PARCIAL","iter":3,"sumas":[4.5,-9,30,2.5],"conteos":[600,400],"inercia":812.0625}
	// {"tipo":"FIN"}
}
