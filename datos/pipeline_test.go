package datos

import (
	"bytes"
	"io"
	"runtime"
	"strings"
	"testing"
	"time"
)

// lectorLento demora las lecturas que empiezan en el offset dado: con eso el
// bloque 0 llega al recolector después que los demás.
type lectorLento struct {
	r      io.ReaderAt
	desde  int64
	espera time.Duration
}

func (l lectorLento) ReadAt(p []byte, off int64) (int, error) {
	if off == l.desde && len(p) > 1024 {
		time.Sleep(l.espera)
	}
	return l.r.ReadAt(p, off)
}

func TestParcialesFueraDeOrden(t *testing.T) {
	m := generarMuestra()
	op := Opciones{Workers: 8, Bloque: 8 << 10, Chunk: 97}
	ref := cargarBytes(t, m.csv, ConfigPorDefecto(), RecetaProvisional(), Opciones{Workers: 1, Bloque: op.Bloque, Chunk: 97, secuencial: true})
	inicioDatos := int64(len(cabeceraMuestra) + 2) // el bloque 0 empieza después de la cabecera y su CRLF
	lento := lectorLento{bytes.NewReader(m.csv), inicioDatos, 150 * time.Millisecond}
	res, err := CargarDesde(lento, int64(len(m.csv)), "lento.csv", ConfigPorDefecto(), RecetaProvisional(), op)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(silverDe(t, ref), silverDe(t, res)) || !bytes.Equal(goldDe(t, ref), goldDe(t, res)) {
		t.Fatal("con el bloque 0 llegando último, el resultado cambia")
	}
}

func TestErrorEnBloqueIntermedioNoDejaGoroutines(t *testing.T) {
	var filas []string
	for i := 0; i < 400; i++ {
		filas = append(filas, filaOEFA(nil))
	}
	// Una comilla que no cierra a mitad de archivo: el registro se extiende
	// hasta el tope de líneas y la carga falla.
	filas[200] = strings.Replace(filas[200], `"Punto de prueba"`, `"Punto de prueba`, 1)
	csv := csvOEFA(filas...)
	antes := runtime.NumGoroutine()
	for range 5 {
		_, err := CargarDesde(bytes.NewReader(csv), int64(len(csv)), "roto.csv", ConfigPorDefecto(), RecetaProvisional(),
			Opciones{Workers: 8, Bloque: 1 << 10})
		if err == nil || !strings.Contains(err.Error(), "sin cerrar") {
			t.Fatalf("tenía que fallar por la comilla, error: %v", err)
		}
	}
	// Las goroutines del pipeline terminan solas; se les da un momento.
	for i := 0; i < 50 && runtime.NumGoroutine() > antes; i++ {
		time.Sleep(20 * time.Millisecond)
	}
	if despues := runtime.NumGoroutine(); despues > antes {
		t.Fatalf("quedaron goroutines colgadas: %d antes, %d después", antes, despues)
	}
}
