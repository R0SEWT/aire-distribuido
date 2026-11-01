package kmeans

import (
	"runtime"
	"strings"
	"testing"
)

const recetaBuena = `{"features": ["pm10","fraccion_fina","viento_velocidad","humedad"], "log1p": ["pm10"],
 "nivel": "absoluto", "k": 5, "semilla": 42, "tolerancia": 1e-6, "max_iter": 100, "workers": 0, "chunk": 10000}`

func TestLeerRecetaDelRepoEsValida(t *testing.T) {
	r, err := LeerReceta("../configs/kmeans.json")
	if err != nil {
		t.Fatalf("configs/kmeans.json no es una receta válida: %v", err)
	}
	if len(r.Features) == 0 || r.K <= 0 {
		t.Fatalf("receta vacía: %+v", r)
	}
}

func TestDecodificarRecetaLeeTodosLosCampos(t *testing.T) {
	r, err := DecodificarReceta(strings.NewReader(recetaBuena))
	if err != nil {
		t.Fatalf("DecodificarReceta: %v", err)
	}
	if strings.Join(r.Features, ",") != "pm10,fraccion_fina,viento_velocidad,humedad" {
		t.Errorf("features = %v", r.Features)
	}
	if len(r.Log1p) != 1 || r.Log1p[0] != "pm10" || r.Nivel != NivelAbsoluto {
		t.Errorf("log1p = %v, nivel = %q", r.Log1p, r.Nivel)
	}
	if r.K != 5 || r.Semilla != 42 || r.Tolerancia != 1e-6 || r.MaxIter != 100 || r.Workers != 0 || r.Chunk != 10000 {
		t.Errorf("receta leída: %+v", r)
	}
	if r.Semillas != 1 {
		t.Errorf("sin el campo semillas la receta corre una sola, como antes; dio %d", r.Semillas)
	}
	r, err = DecodificarReceta(strings.NewReader(strings.Replace(recetaBuena, `"k": 5`, `"k": 5, "semillas": 10`, 1)))
	if err != nil || r.Semillas != 10 {
		t.Errorf("semillas = %d, %v; se esperaba 10", r.Semillas, err)
	}
}

func TestLaRecetaDelRepoCorreDiezSemillas(t *testing.T) {
	r, err := LeerReceta("../configs/kmeans.json")
	if err != nil {
		t.Fatal(err)
	}
	if r.Semillas != 10 {
		t.Errorf("la receta provisional pasa a 10 semillas, tiene %d", r.Semillas)
	}
}

func TestRecetaOpcionesConc(t *testing.T) {
	r, err := DecodificarReceta(strings.NewReader(recetaBuena))
	if err != nil {
		t.Fatalf("DecodificarReceta: %v", err)
	}
	op := r.OpcionesConc()
	if op.Workers != runtime.NumCPU() {
		t.Errorf("workers 0 debería ser NumCPU (%d), es %d", runtime.NumCPU(), op.Workers)
	}
	if op.K != 5 || op.MaxIter != 100 || op.Chunk != 10000 || op.TolAbs != 1e-6 || op.TolRel != 1e-6 {
		t.Errorf("opciones: %+v", op)
	}
	r.Workers = 3
	if r.OpcionesConc().Workers != 3 {
		t.Error("workers explícito no se respetó")
	}
}

func TestRecetaRechazaLoQueNoTieneSentido(t *testing.T) {
	casos := map[string]string{
		"campo desconocido":     strings.Replace(recetaBuena, `"chunk"`, `"chunks"`, 1),
		"sin features":          strings.Replace(recetaBuena, `["pm10","fraccion_fina","viento_velocidad","humedad"]`, `[]`, 1),
		"feature no canónica":   strings.Replace(recetaBuena, `"humedad"]`, `"humedad_relativa"]`, 1),
		"feature repetida":      strings.Replace(recetaBuena, `"humedad"]`, `"pm10"]`, 1),
		"log1p fuera de las fs": strings.Replace(recetaBuena, `"log1p": ["pm10"]`, `"log1p": ["pm2_5"]`, 1),
		"log1p repetido":        strings.Replace(recetaBuena, `"log1p": ["pm10"]`, `"log1p": ["pm10","pm10"]`, 1),
		"nivel desconocido":     strings.Replace(recetaBuena, `"absoluto"`, `"relativo"`, 1),
		"k cero":                strings.Replace(recetaBuena, `"k": 5`, `"k": 0`, 1),
		"max_iter cero":         strings.Replace(recetaBuena, `"max_iter": 100`, `"max_iter": 0`, 1),
		"tolerancia negativa":   strings.Replace(recetaBuena, `1e-6`, `-1e-6`, 1),
		"workers negativo":      strings.Replace(recetaBuena, `"workers": 0`, `"workers": -1`, 1),
		"chunk cero":            strings.Replace(recetaBuena, `"chunk": 10000`, `"chunk": 0`, 1),
		"semillas negativas":    strings.Replace(recetaBuena, `"k": 5`, `"k": 5, "semillas": -2`, 1),
		"basura al final":       recetaBuena + ` {}`,
		"no es JSON":            `features = ["pm10"]`,
	}
	for nombre, js := range casos {
		t.Run(nombre, func(t *testing.T) {
			if _, err := DecodificarReceta(strings.NewReader(js)); err == nil {
				t.Fatal("se esperaba error")
			}
		})
	}
}
