package kmeans

import (
	"fmt"
	"math"
	"slices"
	"testing"
)

func f64(v float64) *float64 { return &v }

func TestEtiquetaECA(t *testing.T) {
	casos := []struct {
		nombre     string
		pm25, pm10 *float64
		quiero     string
		por        []string
	}{
		{"excede por PM2.5", f64(60), f64(80), ECAExcede, []string{FeaturePM25}},
		{"excede por PM10", f64(30), f64(150), ECAExcede, []string{FeaturePM10}},
		{"excede por los dos", f64(51), f64(101), ECAExcede, []string{FeaturePM25, FeaturePM10}},
		{"no excede", f64(30), f64(80), ECANoExcede, nil},
		// «Supera» es estricto: justo en el umbral no excede.
		{"en el umbral", f64(50), f64(100), ECANoExcede, nil},
		{"sin PM2.5 y PM10 alto", nil, f64(150), ECAExcede, []string{FeaturePM10}},
		// Sin PM2.5 no se puede decir que NO excede: podría exceder por PM2.5.
		{"sin PM2.5 y PM10 bajo", nil, f64(80), ECANoEvaluable, nil},
		{"sin PM10 y PM2.5 bajo", f64(20), nil, ECANoEvaluable, nil},
		{"sin nada", nil, nil, ECANoEvaluable, nil},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			got, por := EtiquetaECA(c.pm25, c.pm10)
			if got != c.quiero || !slices.Equal(por, c.por) {
				t.Errorf("EtiquetaECA = %q %v, se esperaba %q %v", got, por, c.quiero, c.por)
			}
		})
	}
}

// entrenarSintetico corre la tubería completa de cmd/entrenar sobre un gold
// sintético: k-means++, Concurrente y Perfilar.
func entrenarSintetico(t *testing.T, s *Sintetico, k int) ([]Regimen, *Resultado) {
	t.Helper()
	d := s.Gold.Datos
	cent, kEf, err := KMeansPP(d, k, 42)
	if err != nil || kEf != k {
		t.Fatalf("KMeansPP: k efectivo %d, %v", kEf, err)
	}
	r, err := Concurrente(d, cent, OpcionesConc{Opciones: Opciones{K: k, MaxIter: 100, TolAbs: 1e-6, TolRel: 1e-6}, Workers: 4, Chunk: 500})
	if err != nil {
		t.Fatalf("Concurrente: %v", err)
	}
	regs, err := Perfilar(s.Gold, r.Asignaciones, k, s.Est)
	if err != nil {
		t.Fatalf("Perfilar: %v", err)
	}
	return regs, r
}

// mayoritario devuelve el régimen plantado más frecuente entre las filas de un
// régimen encontrado y su pureza.
func mayoritario(s *Sintetico, asign []int, j int) (int, float64) {
	cuenta := map[int]int{}
	total := 0
	for i, a := range asign {
		if a == j {
			cuenta[s.Plantado[i]]++
			total++
		}
	}
	mejor, n := -1, 0
	for p, c := range cuenta {
		if c > n || (c == n && p < mejor) {
			mejor, n = p, c
		}
	}
	return mejor, float64(n) / float64(total)
}

func TestPerfilarEtiquetaBienLosRegimenesPlantados(t *testing.T) {
	for _, nivel := range []string{NivelAbsoluto, NivelEstacion} {
		t.Run(nivel, func(t *testing.T) {
			s := sinteticoDe(t, nivel)
			regs, r := entrenarSintetico(t, s, 5)
			if len(regs) != 5 {
				t.Fatalf("%d regímenes, se esperaban 5", len(regs))
			}
			vistos := map[int]bool{}
			for j, reg := range regs {
				// «limpio» y «moderado» se tocan un poco con 10 % de ruido (más con
				// el z-score por estación, que se estima con 500 horas): 95 % de
				// pureza basta para decir que el régimen es el plantado.
				p, pureza := mayoritario(s, r.Asignaciones, j)
				if pureza < 0.95 {
					t.Fatalf("régimen %d: pureza %.3f frente al plantado %d", j, pureza, p)
				}
				vistos[p] = true
				plan := s.Regimenes[p]
				quiero, _ := EtiquetaECA(f64(plan.Valor[FeaturePM25]), f64(plan.Valor[FeaturePM10]))
				if reg.ECA != quiero {
					t.Errorf("régimen %d (%s): ECA %q, se esperaba %q (PM2.5 %.1f, PM10 %.1f)",
						j, plan.Nombre, reg.ECA, quiero, *reg.PM25Medio, *reg.PM10Medio)
				}
				if reg.PM25Origen != "fraccion_fina*pm10" {
					t.Errorf("régimen %d: PM2.5 sale de %q, se esperaba fraccion_fina*pm10", j, reg.PM25Origen)
				}
				// El perfil está en unidades originales: cerca del valor plantado
				// (el ruido es multiplicativo de 10 %; la humedad se recorta a 100).
				for _, f := range featuresPrueba {
					if rel := math.Abs(reg.Perfil[f]/plan.Valor[f] - 1); rel > 0.05 {
						t.Errorf("régimen %d (%s), %s: perfil %.3f, plantado %.3f", j, plan.Nombre, f, reg.Perfil[f], plan.Valor[f])
					}
				}
			}
			if len(vistos) != 5 {
				t.Errorf("los 5 regímenes encontrados corresponden a %d plantados", len(vistos))
			}
		})
	}
}

func TestPerfilarCuentaTodasLasFilasPorHoraYPorEstacion(t *testing.T) {
	s := sinteticoDe(t, NivelAbsoluto)
	regs, r := entrenarSintetico(t, s, 5)
	total := 0
	for j, reg := range regs {
		porHora, porEst := 0, 0
		for _, c := range reg.PorHora {
			porHora += c
		}
		for _, c := range reg.PorEstacion {
			porEst += c
		}
		if porHora != reg.Tamano || porEst != reg.Tamano {
			t.Errorf("régimen %d: tamaño %d, por hora %d, por estación %d", j, reg.Tamano, porHora, porEst)
		}
		cuenta := 0
		for _, a := range r.Asignaciones {
			if a == j {
				cuenta++
			}
		}
		if cuenta != reg.Tamano {
			t.Errorf("régimen %d: tamaño %d, asignaciones %d", j, reg.Tamano, cuenta)
		}
		if math.Abs(reg.Fraccion-float64(reg.Tamano)/float64(s.Gold.Datos.N)) > 1e-15 {
			t.Errorf("régimen %d: fracción %v", j, reg.Fraccion)
		}
		total += reg.Tamano
	}
	if total != s.Gold.Datos.N {
		t.Errorf("los tamaños suman %d de %d filas", total, s.Gold.Datos.N)
	}
	// La hora es la de Lima: la primera fila del gold sintético es 00:00-05:00,
	// y el régimen de esa fila cuenta una lectura a las 0 horas.
	if regs[r.Asignaciones[0]].PorHora[0] == 0 {
		t.Error("la fila de las 00:00 no se contó en la hora 0")
	}
}

func TestPerfilarSegunLoQueLaRecetaPermiteEvaluar(t *testing.T) {
	casos := []struct {
		features []string
		origen   string
	}{
		{[]string{FeaturePM25, FeaturePM10, "humedad"}, FeaturePM25},
		{[]string{FeaturePM10, "humedad"}, ""},
		{[]string{"viento_velocidad", "humedad"}, ""},
	}
	for _, c := range casos {
		t.Run(fmt.Sprint(c.features), func(t *testing.T) {
			s, err := GoldSintetico(OpcionesSintetico{Features: c.features, Nivel: NivelAbsoluto,
				Estaciones: 3, Horas: 300, Ruido: 0.1, Semilla: 5})
			if err != nil {
				t.Fatal(err)
			}
			regs, r := entrenarSintetico(t, s, 5)
			for j, reg := range regs {
				if reg.PM25Origen != c.origen {
					t.Errorf("régimen %d: origen de PM2.5 %q, se esperaba %q", j, reg.PM25Origen, c.origen)
				}
				tienePM10 := slices.Contains(c.features, FeaturePM10)
				if (reg.PM10Medio != nil) != tienePM10 {
					t.Errorf("régimen %d: PM10 medio presente = %v", j, reg.PM10Medio != nil)
				}
				if !tienePM10 && reg.ECA != ECANoEvaluable {
					t.Errorf("régimen %d: sin PM no se puede evaluar el ECA, dio %q", j, reg.ECA)
				}
				if tienePM10 && c.origen == "" {
					p, _ := mayoritario(s, r.Asignaciones, j)
					if s.Regimenes[p].Valor[FeaturePM10] > ECAPM10 && reg.ECA != ECAExcede {
						t.Errorf("régimen %d: PM10 %.0f sin PM2.5 debería exceder, dio %q", j, *reg.PM10Medio, reg.ECA)
					}
					if s.Regimenes[p].Valor[FeaturePM10] < ECAPM10 && reg.ECA != ECANoEvaluable {
						t.Errorf("régimen %d: PM10 bajo sin PM2.5 no es evaluable, dio %q", j, reg.ECA)
					}
				}
			}
		})
	}
}

func TestPerfilarConUnRegimenVacio(t *testing.T) {
	s := sinteticoDe(t, NivelAbsoluto)
	asign := make([]int, s.Gold.Datos.N) // todo al régimen 0; el 1 queda vacío
	regs, err := Perfilar(s.Gold, asign, 2, s.Est)
	if err != nil {
		t.Fatalf("Perfilar: %v", err)
	}
	if regs[1].Tamano != 0 || regs[1].Perfil != nil || regs[1].ECA != ECANoEvaluable {
		t.Errorf("régimen vacío: %+v", regs[1])
	}
	if regs[0].Tamano != s.Gold.Datos.N {
		t.Errorf("régimen 0: tamaño %d", regs[0].Tamano)
	}
}

func TestPerfilarRechazaEntradasQueNoCuadran(t *testing.T) {
	s := sinteticoDe(t, NivelAbsoluto)
	if _, err := Perfilar(s.Gold, make([]int, 3), 2, s.Est); err == nil {
		t.Error("asignaciones de otro largo: se esperaba error")
	}
	malas := make([]int, s.Gold.Datos.N)
	malas[7] = 9
	if _, err := Perfilar(s.Gold, malas, 2, s.Est); err == nil {
		t.Error("asignación fuera de [0, k): se esperaba error")
	}
	otra := *s.Est
	otra.Features = []string{"humedad", "pm10", "fraccion_fina", "viento_velocidad"}
	if _, err := Perfilar(s.Gold, make([]int, s.Gold.Datos.N), 2, &otra); err == nil {
		t.Error("estandarización de otras features: se esperaba error")
	}
}
