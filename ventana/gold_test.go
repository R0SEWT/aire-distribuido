package ventana

import (
	"encoding/csv"
	"errors"
	"io"
	"os"
	"strconv"
	"testing"

	"github.com/R0SEWT/aire-distribuido/kmeans"
)

// La ventana, recorriendo el gold, reproduce las cifras con que el equipo
// decidió la regla de la alerta: de las horas con media de 24 h calculable
// (18 lecturas o más), cuántas superan el ECA, y cuánto acertarían el régimen
// alto o la lectura horaria si se usaran como alerta. Se salta con -short o
// sin el gold (no se versiona).
func TestLaVentanaSobreElGold(t *testing.T) {
	if testing.Short() {
		t.Skip("-short: no se lee el gold completo")
	}
	const rutaGold, rutaAsign = "../data/gold/oefa_features.csv", "../data/gold/asignaciones.csv"
	if _, err := os.Stat(rutaGold); err != nil {
		t.Skipf("no está el gold en %s: %v", rutaGold, err)
	}
	est, err := kmeans.LeerEstandarizacion("../reports/estandarizacion.json")
	if err != nil {
		t.Fatal(err)
	}
	g, err := kmeans.LeerGold(rutaGold, []string{kmeans.FeaturePM25, kmeans.FeaturePM10})
	if err != nil {
		t.Fatal(err)
	}
	regimen := leerRegimenes(t, rutaAsign, g.Datos.N)
	const alto = 1 // el régimen que excede el ECA en reports/regimenes.json

	v := Nuevas()
	x := make([]float64, 2)
	var calculables, sobreECA int
	var altoSi, altoAcierta, horariaSi, horariaAcierta int
	bandas := map[string]int{}
	for i := 0; i < g.Datos.N; i++ {
		if err := est.Invertir(g.Datos.Punto(i), g.Estacion[i], x); err != nil {
			t.Fatal(err)
		}
		m := v.Agregar(g.Estacion[i], g.FechaHora[i], x[0], x[1])
		if m.Lecturas < MinimoLecturas {
			continue
		}
		calculables++
		alerta := Alerta(m)
		if alerta {
			sobreECA++
		}
		bandas[*INCA(m)]++
		if regimen != nil && regimen[i] == alto {
			altoSi++
			if alerta {
				altoAcierta++
			}
		}
		if x[0] > kmeans.ECAPM25 || x[1] > kmeans.ECAPM10 {
			horariaSi++
			if alerta {
				horariaAcierta++
			}
		}
	}
	pct := func(a, b int) float64 { return 100 * float64(a) / float64(b) }
	t.Logf("%d horas con media de 24 h calculable; %d sobre el ECA (%.1f %%); INCA %v",
		calculables, sobreECA, pct(sobreECA, calculables), bandas)
	t.Logf("lectura horaria sobre el umbral como alerta: precisión %.0f %%, recall %.0f %%",
		pct(horariaAcierta, horariaSi), pct(horariaAcierta, sobreECA))
	if regimen != nil {
		t.Logf("régimen alto como alerta: precisión %.0f %%, recall %.0f %%", pct(altoAcierta, altoSi), pct(altoAcierta, sobreECA))
	}
	if calculables != 64748 || sobreECA != 1184 {
		t.Fatalf("%d horas calculables y %d sobre el ECA; el análisis del equipo dio 64 748 y 1 184", calculables, sobreECA)
	}
}

func leerRegimenes(t *testing.T, ruta string, n int) []int {
	t.Helper()
	f, err := os.Open(ruta)
	if err != nil {
		t.Logf("sin %s: no se mide el régimen alto", ruta)
		return nil
	}
	defer f.Close()
	cr := csv.NewReader(f)
	if _, err := cr.Read(); err != nil {
		t.Fatal(err)
	}
	reg := make([]int, 0, n)
	for {
		fila, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		j, err := strconv.Atoi(fila[2])
		if err != nil {
			t.Fatal(err)
		}
		reg = append(reg, j)
	}
	if len(reg) != n {
		t.Fatalf("%s tiene %d filas y el gold %d", ruta, len(reg), n)
	}
	return reg
}
