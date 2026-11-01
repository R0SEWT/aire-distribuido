package kmeans

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func recetaPrueba(nivel string) Receta {
	return Receta{Features: featuresPrueba, Log1p: []string{"pm10"}, Nivel: nivel,
		K: 5, Semilla: 42, Tolerancia: 1e-6, MaxIter: 100, Chunk: 1000}
}

func sinteticoDe(t *testing.T, nivel string) *Sintetico {
	t.Helper()
	s, err := GoldSintetico(OpcionesSintetico{Features: featuresPrueba, Log1p: []string{"pm10"},
		Nivel: nivel, Estaciones: 4, Horas: 500, Ruido: 0.1, Semilla: 7})
	if err != nil {
		t.Fatalf("GoldSintetico: %v", err)
	}
	return s
}

func TestInvertirDeshaceLaEstandarizacionSintetica(t *testing.T) {
	// log1p y expm1 redondean cada una; la ida y vuelta es exacta salvo por los
	// últimos bits. 1e-12 relativo es cuatro órdenes más que el error esperado
	// y siete menos que cualquier diferencia que importe en µg/m³.
	for _, nivel := range []string{NivelAbsoluto, NivelEstacion} {
		t.Run(nivel, func(t *testing.T) {
			s := sinteticoDe(t, nivel)
			d := s.Gold.Datos
			x := make([]float64, d.D)
			for i := 0; i < d.N; i++ {
				if err := s.Est.Invertir(d.Punto(i), s.Gold.Estacion[i], x); err != nil {
					t.Fatalf("Invertir: %v", err)
				}
				for f := range x {
					quiero := s.Originales[i*d.D+f]
					if math.Abs(x[f]-quiero) > 1e-12*math.Max(1, math.Abs(quiero)) {
						t.Fatalf("fila %d, %s: %.17g, se esperaba %.17g", i, s.Gold.Features[f], x[f], quiero)
					}
				}
			}
		})
	}
}

func TestEstandarizarEInvertirSonExactosConEscalasDiadicas(t *testing.T) {
	// Sin log1p y con desviaciones potencias de dos, restar y dividir no
	// redondean: la inversión tiene que ser bit a bit.
	e := &Estandarizacion{Features: []string{"humedad", "viento_velocidad"}, Nivel: NivelAbsoluto,
		N: 3, Media: []float64{70, 3}, Desv: []float64{8, 0.5}}
	if err := e.Validar(); err != nil {
		t.Fatalf("Validar: %v", err)
	}
	for _, x := range [][]float64{{70, 3}, {86, 2.5}, {41, 7.25}} {
		z := make([]float64, 2)
		if err := e.Estandarizar(x, "", z); err != nil {
			t.Fatal(err)
		}
		vuelta := make([]float64, 2)
		if err := e.Invertir(z, "", vuelta); err != nil {
			t.Fatal(err)
		}
		for f := range x {
			if math.Float64bits(vuelta[f]) != math.Float64bits(x[f]) {
				t.Errorf("%v → %v → %v", x, z, vuelta)
			}
		}
	}
}

func TestLeerEstandarizacionYCoincidirConLaReceta(t *testing.T) {
	for _, nivel := range []string{NivelAbsoluto, NivelEstacion} {
		t.Run(nivel, func(t *testing.T) {
			s := sinteticoDe(t, nivel)
			ruta := filepath.Join(t.TempDir(), "estandarizacion.json")
			js, err := json.MarshalIndent(s.Est, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(ruta, js, 0o600); err != nil {
				t.Fatal(err)
			}
			e, err := LeerEstandarizacion(ruta)
			if err != nil {
				t.Fatalf("LeerEstandarizacion: %v", err)
			}
			if err := e.Coincide(recetaPrueba(nivel)); err != nil {
				t.Fatalf("Coincide: %v", err)
			}
			if nivel == NivelEstacion && len(e.PorEstacion) != 4 {
				t.Errorf("por_estacion trae %d estaciones, se esperaban 4", len(e.PorEstacion))
			}
		})
	}
}

func TestEstandarizacionQueNoEsLaDeLaRecetaSeRechaza(t *testing.T) {
	s := sinteticoDe(t, NivelAbsoluto)
	casos := map[string]func(r *Receta){
		"features en otro orden": func(r *Receta) { r.Features = []string{"fraccion_fina", "pm10", "viento_velocidad", "humedad"} },
		"otro log1p":             func(r *Receta) { r.Log1p = nil },
		"otro nivel":             func(r *Receta) { r.Nivel = NivelEstacion },
	}
	for nombre, cambiar := range casos {
		t.Run(nombre, func(t *testing.T) {
			r := recetaPrueba(NivelAbsoluto)
			cambiar(&r)
			if err := s.Est.Coincide(r); err == nil {
				t.Fatal("se esperaba error: el gold se estandarizó con otra receta")
			}
		})
	}
}

func TestEstandarizacionInvalidaSeRechaza(t *testing.T) {
	base := `{"features":["pm10","humedad"],"log1p":["pm10"],"nivel":"absoluto","n":10,"media":[3,70],"desv":[1,8]}`
	casos := map[string]string{
		"desviación cero":           strings.Replace(base, `"desv":[1,8]`, `"desv":[0,8]`, 1),
		"desviación negativa":       strings.Replace(base, `"desv":[1,8]`, `"desv":[1,-8]`, 1),
		"media de otro largo":       strings.Replace(base, `"media":[3,70]`, `"media":[3]`, 1),
		"log1p que no es feature":   strings.Replace(base, `"log1p":["pm10"]`, `"log1p":["co"]`, 1),
		"nivel desconocido":         strings.Replace(base, `"absoluto"`, `"zona"`, 1),
		"estación sin por_estacion": strings.Replace(base, `"absoluto"`, `"estacion"`, 1),
		"por_estacion de otro largo": strings.Replace(base, `"absoluto","n":10`,
			`"estacion","por_estacion":{"CA":{"n":10,"media":[1],"desv":[1]}},"n":10`, 1),
		"no es JSON": `features: pm10`,
	}
	for nombre, js := range casos {
		t.Run(nombre, func(t *testing.T) {
			ruta := filepath.Join(t.TempDir(), "e.json")
			if err := os.WriteFile(ruta, []byte(js), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := LeerEstandarizacion(ruta); err == nil {
				t.Fatal("se esperaba error")
			}
		})
	}
}

func TestInvertirEnNivelEstacionNecesitaLaEstacion(t *testing.T) {
	s := sinteticoDe(t, NivelEstacion)
	x := make([]float64, 4)
	z := s.Gold.Datos.Punto(0)
	if err := s.Est.Invertir(z, "", x); err == nil {
		t.Error("un centroide con nivel = estacion no tiene unidades absolutas: Invertir sin estación debería fallar")
	}
	if err := s.Est.Invertir(z, "NO-EXISTE", x); err == nil {
		t.Error("una estación sin estadísticas debería fallar")
	}
	if s.Est.CentroidesEnUnidades() {
		t.Error("con nivel = estacion los centroides no se pueden des-estandarizar")
	}
	if !sinteticoDe(t, NivelAbsoluto).Est.CentroidesEnUnidades() {
		t.Error("con nivel = absoluto los centroides sí se pueden des-estandarizar")
	}
}
