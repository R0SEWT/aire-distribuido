package protocolo_test

import (
	"errors"
	"net"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/R0SEWT/aire-distribuido/protocolo"
)

// nodoV2 hace de nodo del mapa nacional sobre un extremo de net.Pipe: se
// presenta con un HOLA v2, confirma la TAREA y responde cada PESOS con los
// pesos más uno. Los campos opcionales cambian ese comportamiento.
type nodoV2 struct {
	nombre   string
	version  int // 0 = protocolo.Version2
	muestras int

	// alTarea, si no es nil, reemplaza el LISTO honesto.
	alTarea func(tarea protocolo.Mensaje) protocolo.Mensaje
	// responder, si no es nil, decide qué mandar ante un PESOS; recibe el
	// LOCAL honesto ya calculado. Devolver nil es no responder.
	responder func(pesos, honesto protocolo.Mensaje) []protocolo.Mensaje
}

func (nf *nodoV2) correr(conn net.Conn) error {
	defer conn.Close()
	cod, dec := protocolo.NuevoCodificador(conn), protocolo.NuevoDecodificador(conn)
	version := nf.version
	if version == 0 {
		version = protocolo.Version2
	}
	hola := protocolo.Mensaje{Tipo: protocolo.Hola, Version: version, Nodo: nf.nombre, Workers: 1, GPU: "falsa", VramGB: 4}
	if err := cod.Escribir(hola); err != nil {
		return ignorarCierre(err)
	}
	for {
		m, err := dec.Leer()
		if err != nil {
			return ignorarCierre(err)
		}
		var resp []protocolo.Mensaje
		switch m.Tipo {
		case protocolo.Tarea:
			listo := protocolo.Mensaje{Tipo: protocolo.Listo, Tarea: m.Tarea, Muestras: nf.muestras}
			if nf.alTarea != nil {
				listo = nf.alTarea(m)
			}
			resp = []protocolo.Mensaje{listo}
		case protocolo.Pesos:
			v, err := m.Vector()
			if err != nil {
				return err
			}
			for i := range v {
				v[i]++
			}
			honesto := protocolo.Mensaje{Tipo: protocolo.Local, Ronda: m.Ronda, Muestras: nf.muestras, Perdida: 0.5}.ConVector(v)
			resp = []protocolo.Mensaje{honesto}
			if nf.responder != nil {
				resp = nf.responder(m, honesto)
			}
		case protocolo.Ping:
			resp = []protocolo.Mensaje{{Tipo: protocolo.Pong}}
		case protocolo.Fin, protocolo.Error:
			return nil
		}
		for _, r := range resp {
			if err := cod.Escribir(r); err != nil {
				return ignorarCierre(err)
			}
		}
	}
}

// montarV2 conecta un nodo del mapa por net.Pipe y recibe su HOLA v2. Al
// terminar la prueba cierra la conexión y espera al nodo.
func montarV2(t *testing.T, nf *nodoV2) *protocolo.Enlace {
	t.Helper()
	var wg sync.WaitGroup
	t.Cleanup(wg.Wait)
	coord, extremo := net.Pipe()
	t.Cleanup(func() { coord.Close() })
	wg.Go(func() {
		if err := nf.correr(extremo); err != nil {
			t.Errorf("nodo %s: %v", nf.nombre, err)
		}
	})
	e := protocolo.NuevoEnlace(coord)
	if err := protocolo.RecibirHolaVersion(e, protocolo.Version2, time.Now().Add(5*time.Second)); err != nil {
		t.Fatal(err)
	}
	return e
}

func tareaDePrueba() protocolo.Mensaje {
	return protocolo.Mensaje{Tipo: protocolo.Tarea, Tarea: "mapa-prueba", Manifiesto: "manifiesto.json",
		Huella: "sha256:ab12", Dias: []string{"2018-01-01", "2018-01-02"}}
}

func pesosDePrueba(ronda int, v []float32) protocolo.Mensaje {
	return protocolo.Mensaje{Tipo: protocolo.Pesos, Ronda: ronda, Pasos: 10}.ConVector(v)
}

func TestHolaV2GuardaLaGPUYLaTareaLasMuestras(t *testing.T) {
	e := montarV2(t, &nodoV2{nombre: "pc-07", muestras: 321})
	if e.Nodo != "pc-07" || e.GPU != "falsa" || e.VramGB != 4 || e.Workers != 1 {
		t.Fatalf("datos del HOLA v2: %+v", e)
	}
	if err := protocolo.AsignarTarea(e, tareaDePrueba(), time.Now().Add(5*time.Second)); err != nil {
		t.Fatal(err)
	}
	if e.Tarea != "mapa-prueba" || e.Muestras != 321 {
		t.Fatalf("tras el LISTO: tarea %q, muestras %d", e.Tarea, e.Muestras)
	}

	limite := time.Now().Add(5 * time.Second)
	if err := protocolo.EnviarPesos(e, pesosDePrueba(1, []float32{1, 2.5}), limite); err != nil {
		t.Fatal(err)
	}
	m, err := protocolo.RecibirLocal(e, 1, 2, limite)
	if err != nil {
		t.Fatal(err)
	}
	if v, err := m.Vector(); err != nil || !reflect.DeepEqual(v, []float32{2, 3.5}) || m.Muestras != 321 {
		t.Fatalf("LOCAL: %v, muestras %d, %v", v, m.Muestras, err)
	}
	if e.UltimaRonda() != 1 {
		t.Errorf("UltimaRonda = %d, se esperaba 1", e.UltimaRonda())
	}
}

// Cada coordinador habla una sola versión: el del K-Means (v1) rechaza a un
// nodo del mapa y el del mapa (v2) a un nodo del K-Means, y los dos le avisan.
func TestCadaCoordinadorAceptaSoloSuVersion(t *testing.T) {
	casos := []struct {
		nombre      string
		versionNodo int
		recibir     func(*protocolo.Enlace, time.Time) error
	}{
		{"nodo v2 al coordinador del K-Means", protocolo.Version2, protocolo.RecibirHola},
		{"nodo v1 al coordinador del mapa", protocolo.Version, func(e *protocolo.Enlace, l time.Time) error {
			return protocolo.RecibirHolaVersion(e, protocolo.Version2, l)
		}},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			coord, extremo := net.Pipe()
			defer coord.Close()
			recibido := make(chan protocolo.Mensaje, 1)
			go func() {
				defer extremo.Close()
				cod, dec := protocolo.NuevoCodificador(extremo), protocolo.NuevoDecodificador(extremo)
				cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Hola, Version: c.versionNodo, Nodo: "pc-otra"})
				m, _ := dec.Leer()
				recibido <- m
			}()
			err := c.recibir(protocolo.NuevoEnlace(coord), time.Now().Add(5*time.Second))
			if !errors.Is(err, protocolo.ErrProtocolo) || !strings.Contains(err.Error(), "pc-otra") {
				t.Errorf("se esperaba un rechazo que nombre al nodo, salió %v", err)
			}
			if m := <-recibido; m.Tipo != protocolo.Error || !strings.Contains(m.Mensaje, "versión") {
				t.Errorf("el nodo debió recibir un ERROR que nombre la versión, recibió %+v", m)
			}
		})
	}
}

func TestAsignarTareaVerificaElLISTO(t *testing.T) {
	casos := map[string]func(protocolo.Mensaje) protocolo.Mensaje{
		"otra tarea": func(protocolo.Mensaje) protocolo.Mensaje {
			return protocolo.Mensaje{Tipo: protocolo.Listo, Tarea: "mapa-de-ayer", Muestras: 5}
		},
		"un LISTO de la v1": func(protocolo.Mensaje) protocolo.Mensaje {
			return protocolo.Mensaje{Tipo: protocolo.Listo, N: 5, Huella: "sha256:ab12"}
		},
	}
	for nombre, alTarea := range casos {
		t.Run(nombre, func(t *testing.T) {
			e := montarV2(t, &nodoV2{nombre: "pc-01", alTarea: alTarea})
			err := protocolo.AsignarTarea(e, tareaDePrueba(), time.Now().Add(5*time.Second))
			var en *protocolo.ErrorNodo
			if !errors.Is(err, protocolo.ErrProtocolo) || !errors.As(err, &en) || en.Nodo != "pc-01" {
				t.Fatalf("se esperaba ErrProtocolo que nombre a pc-01, salió %v", err)
			}
			if e.Tarea != "" {
				t.Errorf("una tarea no confirmada no se registra, quedó %q", e.Tarea)
			}
		})
	}
	e := montarV2(t, &nodoV2{nombre: "pc-02"})
	if err := protocolo.AsignarTarea(e, protocolo.Mensaje{Tipo: protocolo.Ping}, time.Now().Add(5*time.Second)); err == nil {
		t.Error("AsignarTarea con un PING debió fallar")
	}
}

// El nodo no responde la ronda 1 (abortada); su LOCAL de la ronda 1 llega
// durante la 2 junto con un PONG suelto, y los dos se descartan.
func TestRecibirLocalDescartaRezagadosYPongs(t *testing.T) {
	var pendiente *protocolo.Mensaje
	e := montarV2(t, &nodoV2{nombre: "lento", muestras: 3,
		responder: func(pesos, honesto protocolo.Mensaje) []protocolo.Mensaje {
			if pesos.Ronda == 1 {
				pendiente = &honesto
				return nil
			}
			viejo := *pendiente
			viejo.Carga = make([]byte, len(viejo.Carga)) // lo que traiga no puede llegar al promedio
			return []protocolo.Mensaje{{Tipo: protocolo.Pong}, viejo, honesto}
		}})
	if err := protocolo.AsignarTarea(e, tareaDePrueba(), time.Now().Add(5*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := protocolo.EnviarPesos(e, pesosDePrueba(1, []float32{0}), time.Now().Add(5*time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := protocolo.RecibirLocal(e, 1, 1, time.Now().Add(50*time.Millisecond)); !errors.Is(err, protocolo.ErrPlazoVencido) {
		t.Fatalf("ronda 1: se esperaba ErrPlazoVencido, salió %v", err)
	}

	limite := time.Now().Add(5 * time.Second)
	if err := protocolo.EnviarPesos(e, pesosDePrueba(2, []float32{10}), limite); err != nil {
		t.Fatal(err)
	}
	m, err := protocolo.RecibirLocal(e, 2, 1, limite)
	if err != nil {
		t.Fatalf("ronda 2: %v", err)
	}
	if v, _ := m.Vector(); m.Ronda != 2 || v[0] != 11 || e.Rezagados != 1 {
		t.Fatalf("ronda 2: LOCAL %d con %v y %d rezagados; se esperaba el de la ronda 2 con 11 y un rezagado", m.Ronda, v, e.Rezagados)
	}
}

func TestRecibirLocalAbortaSiElNodoRespondeMal(t *testing.T) {
	casos := []struct {
		nombre    string
		responder func(pesos, honesto protocolo.Mensaje) []protocolo.Mensaje
		causa     error
		texto     string
	}{
		{"LOCAL de una ronda futura", func(_, h protocolo.Mensaje) []protocolo.Mensaje {
			h.Ronda = 5
			return []protocolo.Mensaje{h}
		}, protocolo.ErrProtocolo, "no se envió"},
		{"otro número de parámetros", func(_, _ protocolo.Mensaje) []protocolo.Mensaje {
			return []protocolo.Mensaje{protocolo.Mensaje{Tipo: protocolo.Local, Ronda: 1, Muestras: 1}.ConVector([]float32{1, 2, 3})}
		}, protocolo.ErrProtocolo, "3 parámetros"},
		{"ERROR del nodo", func(_, _ protocolo.Mensaje) []protocolo.Mensaje {
			return []protocolo.Mensaje{{Tipo: protocolo.Error, Mensaje: "CUDA out of memory"}}
		}, protocolo.ErrInformado, "CUDA"},
		{"un mensaje fuera de lugar", func(_, _ protocolo.Mensaje) []protocolo.Mensaje {
			return []protocolo.Mensaje{{Tipo: protocolo.Listo, Tarea: "mapa-prueba"}}
		}, protocolo.ErrProtocolo, "se esperaba LOCAL y llegó LISTO"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			e := montarV2(t, &nodoV2{nombre: "pc-03", responder: c.responder})
			limite := time.Now().Add(5 * time.Second)
			if err := protocolo.EnviarPesos(e, pesosDePrueba(1, []float32{1, 2}), limite); err != nil {
				t.Fatal(err)
			}
			_, err := protocolo.RecibirLocal(e, 1, 2, limite)
			var en *protocolo.ErrorNodo
			if !errors.Is(err, c.causa) || !strings.Contains(err.Error(), c.texto) || !errors.As(err, &en) || en.Nodo != "pc-03" {
				t.Errorf("se esperaba %v con %q que nombre a pc-03, salió %v", c.causa, c.texto, err)
			}
		})
	}
}

func TestEnviarPesosValidaAntesDeEnviar(t *testing.T) {
	e := montarV2(t, &nodoV2{nombre: "pc-04"})
	limite := time.Now().Add(5 * time.Second)
	if err := protocolo.EnviarPesos(e, pesosDePrueba(1, []float32{1}), limite); err != nil {
		t.Fatal(err)
	}
	if _, err := protocolo.RecibirLocal(e, 1, 1, limite); err != nil {
		t.Fatal(err)
	}
	if err := protocolo.EnviarPesos(e, pesosDePrueba(1, []float32{1}), limite); err == nil || !strings.Contains(err.Error(), "tiene que crecer") {
		t.Errorf("repetir la ronda 1: %v", err)
	}
	if err := protocolo.EnviarPesos(e, protocolo.Mensaje{Tipo: protocolo.Ping}, limite); err == nil {
		t.Error("EnviarPesos con un PING debió fallar")
	}
	if err := protocolo.EnviarPesos(e, pesosDePrueba(2, []float32{1}), time.Now().Add(-time.Second)); !errors.Is(err, protocolo.ErrPlazoVencido) {
		t.Errorf("límite vencido: se esperaba ErrPlazoVencido, salió %v", err)
	}
	// Nada de lo rechazado salió: la ronda 2 sigue disponible y el flujo sano.
	if err := protocolo.EnviarPesos(e, pesosDePrueba(2, []float32{1}), limite); err != nil {
		t.Fatal(err)
	}
	if m, err := protocolo.RecibirLocal(e, 2, 1, limite); err != nil || m.Ronda != 2 {
		t.Fatalf("ronda 2 después de los rechazos: %+v, %v", m, err)
	}
}
