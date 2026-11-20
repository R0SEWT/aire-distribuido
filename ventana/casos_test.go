package ventana

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"math"
	"os"
	"slices"
	"strconv"
	"testing"
	"time"
)

// -actualizar regenera testdata/tramo_gold.json con el silver local (que no se
// versiona):
//
//	go test ./ventana -run TramoReal -actualizar
var actualizar = flag.Bool("actualizar", false, "regenera testdata/tramo_gold.json desde data/silver")

const (
	rutaTramo  = "testdata/tramo_gold.json"
	rutaSilver = "../data/silver/oefa_ancho.csv"
	largoTramo = 72
)

// tramoGold son 72 lecturas horarias reales y seguidas de una estación (del
// silver, en µg/m³), elegidas para que la alerta cambie, con lo que la ventana
// tiene que responder a cada una calculado aparte: un recorrido ingenuo sobre
// las lecturas ya recibidas, sin el código de este paquete.
type tramoGold struct {
	Nota     string        `json:"nota"`
	Estacion string        `json:"estacion"`
	Lecturas []lecturaGold `json:"lecturas"`
}

type lecturaGold struct {
	FechaHora string   `json:"fecha_hora"`
	PM25      float64  `json:"pm2_5"`
	PM10      float64  `json:"pm10"`
	Esperado  esperado `json:"esperado"`
	hora      time.Time
}

type esperado struct {
	Lecturas int      `json:"lecturas"`
	PM25     *float64 `json:"pm2_5"`
	PM10     *float64 `json:"pm10"`
	Alerta   bool     `json:"alerta"`
	INCA     *string  `json:"inca"`
}

// Corre en el CI: la ventana sobre un tramo real, lectura por lectura.
func TestVentanaSobreUnTramoReal(t *testing.T) {
	if *actualizar {
		generarTramo(t)
	}
	b, err := os.ReadFile(rutaTramo)
	if err != nil {
		t.Fatal(err)
	}
	var tr tramoGold
	if err := json.Unmarshal(b, &tr); err != nil {
		t.Fatal(err)
	}
	v := Nuevas()
	alertas, sinMedia := 0, 0
	for _, l := range tr.Lecturas {
		fh, err := time.Parse(time.RFC3339, l.FechaHora)
		if err != nil {
			t.Fatal(err)
		}
		m := v.Agregar(tr.Estacion, fh, l.PM25, l.PM10)
		e := l.Esperado
		if m.Lecturas != e.Lecturas || !mismoValor(m.PM25, e.PM25) || !mismoValor(m.PM10, e.PM10) ||
			Alerta(m) != e.Alerta || !mismaBanda(INCA(m), e.INCA) {
			t.Errorf("%s %s: lecturas %d, media %v, alerta %v, inca %v; se esperaba %d, %v y %v, %v, %v",
				tr.Estacion, l.FechaHora, m.Lecturas, describir(m), Alerta(m), describirBanda(INCA(m)),
				e.Lecturas, valor(e.PM25), valor(e.PM10), e.Alerta, describirBanda(e.INCA))
		}
		if e.Alerta {
			alertas++
		}
		if e.PM25 == nil {
			sinMedia++
		}
	}
	if len(tr.Lecturas) != largoTramo || alertas == 0 || alertas == len(tr.Lecturas)-sinMedia || sinMedia == 0 {
		t.Fatalf("el tramo no cubre los casos: %d lecturas, %d con alerta, %d sin media", len(tr.Lecturas), alertas, sinMedia)
	}
}

func mismoValor(a, b *float64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return math.Abs(*a-*b) <= 1e-9*math.Max(1, math.Abs(*b))
}

func mismaBanda(a, b *string) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

func valor(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

// generarTramo recorre las lecturas completas del silver, estación por
// estación, y se queda con las primeras 72 seguidas de una estación en las que
// la alerta cambia. Lo esperado se calcula con las reglas del contrato
// escritas de nuevo, sin Agregar, Alerta ni INCA.
func generarTramo(t *testing.T) {
	t.Helper()
	f, err := os.Open(rutaSilver)
	if err != nil {
		t.Fatalf("-actualizar necesita el silver local: %v", err)
	}
	defer f.Close()
	cr := csv.NewReader(f)
	cab, err := cr.Read()
	if err != nil {
		t.Fatal(err)
	}
	cEst, cFecha, c25, c10 := slices.Index(cab, "estacion"), slices.Index(cab, "fecha_hora"), slices.Index(cab, "pm2_5"), slices.Index(cab, "pm10")
	porEstacion := map[string][]lecturaGold{}
	var orden []string
	for {
		fila, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if fila[c25] == "" || fila[c10] == "" {
			continue
		}
		fh, err := time.Parse(time.RFC3339, fila[cFecha])
		if err != nil {
			t.Fatal(err)
		}
		pm25, _ := strconv.ParseFloat(fila[c25], 64)
		pm10, _ := strconv.ParseFloat(fila[c10], 64)
		if _, ok := porEstacion[fila[cEst]]; !ok {
			orden = append(orden, fila[cEst])
		}
		porEstacion[fila[cEst]] = append(porEstacion[fila[cEst]], lecturaGold{FechaHora: fila[cFecha], PM25: pm25, PM10: pm10, hora: fh})
	}
	for _, est := range orden {
		ls := porEstacion[est]
		for inicio := 0; inicio+largoTramo <= len(ls); inicio += largoTramo / 2 {
			tramo := slices.Clone(ls[inicio : inicio+largoTramo])
			conAlerta, sinAlerta := 0, 0
			for i := range tramo {
				tramo[i].Esperado = esperadoIngenuo(tramo[:i+1])
				switch {
				case tramo[i].Esperado.PM25 == nil:
				case tramo[i].Esperado.Alerta:
					conAlerta++
				default:
					sinAlerta++
				}
			}
			if conAlerta >= 5 && sinAlerta >= 5 {
				escribirTramo(t, tramoGold{
					Nota: "72 lecturas horarias seguidas del silver (µg/m³) en las que la alerta cambia, con lo " +
						"esperado calculado aparte. Se regenera con go test ./ventana -run TramoReal -actualizar.",
					Estacion: est, Lecturas: tramo,
				})
				t.Logf("%s: %s desde %s, %d con alerta y %d sin alerta", rutaTramo, est, tramo[0].FechaHora, conAlerta, sinAlerta)
				return
			}
		}
	}
	t.Fatal("ningún tramo de 72 lecturas tiene la alerta en los dos estados")
}

// esperadoIngenuo es la ventana de la última lectura de ls, recorriendo todas:
// las de hora en (t − 24 h, t], con una por hora (la última recibida gana).
func esperadoIngenuo(ls []lecturaGold) esperado {
	t := ls[len(ls)-1].hora.Truncate(time.Hour)
	porHora := map[int64]lecturaGold{}
	for _, l := range ls {
		h := l.hora.Truncate(time.Hour)
		if h.After(t.Add(-24*time.Hour)) && !h.After(t) {
			porHora[h.Unix()] = l
		}
	}
	horas := make([]int64, 0, len(porHora))
	for h := range porHora {
		horas = append(horas, h)
	}
	slices.Sort(horas)
	e := esperado{Lecturas: len(horas)}
	if len(horas) < 18 {
		return e
	}
	var s25, s10 float64
	for _, h := range horas {
		s25 += porHora[h].PM25
		s10 += porHora[h].PM10
	}
	m25, m10 := s25/float64(len(horas)), s10/float64(len(horas))
	e.PM25, e.PM10 = &m25, &m10
	e.Alerta = m25 > 50 || m10 > 100
	nivel := func(x float64, cortes ...float64) int {
		for i, c := range cortes {
			if x <= c {
				return i
			}
		}
		return len(cortes)
	}
	nombres := []string{"buena", "moderada", "mala", "umbral_de_cuidado"}
	b := nombres[max(nivel(m10, 75, 150, 250), nivel(m25, 12.5, 25, 125))]
	e.INCA = &b
	return e
}

func escribirTramo(t *testing.T, tr tramoGold) {
	t.Helper()
	b, err := json.MarshalIndent(tr, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll("testdata", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rutaTramo, append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}
