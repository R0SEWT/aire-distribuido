package datos

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestParametrosCanonicosEnOrdenAlfabetico(t *testing.T) {
	if !slices.IsSorted(ParametrosCanonicos[:]) {
		t.Fatalf("los parámetros canónicos tienen que estar en orden de bytes: %v", ParametrosCanonicos)
	}
	quiero := []string{"pm2_5", "pm10", "no2", "so2", "co", "o3", "temperatura", "humedad",
		"viento_velocidad", "viento_direccion", "presion", "precipitacion", "radiacion"}
	if len(quiero) != len(ParametrosCanonicos) {
		t.Fatalf("hay %d canónicos, el contrato dice %d", len(ParametrosCanonicos), len(quiero))
	}
	for _, p := range quiero {
		if indiceParametro(p) < 0 {
			t.Errorf("falta el canónico %q", p)
		}
	}
}

func TestConfigPorDefectoCompila(t *testing.T) {
	if _, err := ConfigPorDefecto().compilar(); err != nil {
		t.Fatal(err)
	}
}

func TestConfigVersionadaEsValida(t *testing.T) {
	cfg, err := LeerConfigLimpieza(filepath.Join("..", "configs", "limpieza.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cfg.compilar(); err != nil {
		t.Fatal(err)
	}
}

func TestConfigRechazaCamposDesconocidos(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "c.json")
	if err := os.WriteFile(ruta, []byte(`{"delimitadr": ";"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LeerConfigLimpieza(ruta)
	if err == nil || !strings.Contains(err.Error(), "delimitadr") {
		t.Fatalf("un campo mal escrito tiene que fallar y nombrarse, error: %v", err)
	}
}

func TestConfigInvalida(t *testing.T) {
	casos := map[string]func(c *ConfigLimpieza){
		"delimitador de dos caracteres": func(c *ConfigLimpieza) { c.Delimitador = ";;" },
		"codificación desconocida":      func(c *ConfigLimpieza) { c.Codificacion = "ebcdic" },
		"separador decimal raro":        func(c *ConfigLimpieza) { c.SeparadorDecimal = ";" },
		"zona horaria inexistente":      func(c *ConfigLimpieza) { c.ZonaHoraria = "Marte/Olympus" },
		"falta la columna de valor":     func(c *ConfigLimpieza) { c.Columnas.Valor = "" },
		"canónico inexistente":          func(c *ConfigLimpieza) { c.Parametros["Benceno"] = MapeoParametro{Canonico: "benceno", Factor: 1} },
		"factor no positivo":            func(c *ConfigLimpieza) { c.Parametros["PM10"] = MapeoParametro{Canonico: "pm10", Factor: 0} },
		"nombres que colisionan":        func(c *ConfigLimpieza) { c.Parametros["pm 10"] = MapeoParametro{Canonico: "pm2_5", Factor: 1} },
		"sin formatos de fecha":         func(c *ConfigLimpieza) { c.FormatosFecha = nil },
		"rango de fechas al revés":      func(c *ConfigLimpieza) { c.FechaDesde, c.FechaHasta = c.FechaHasta, c.FechaDesde },
		"rango físico al revés":         func(c *ConfigLimpieza) { c.Rangos["pm10"] = [2]float64{10, 0} },
		"rango de un no canónico":       func(c *ConfigLimpieza) { c.Rangos["benceno"] = [2]float64{0, 1} },
		"unidad con factor cero":        func(c *ConfigLimpieza) { c.Unidades["co"]["mg/m3"] = 0 },
		"negativo de un no canónico":    func(c *ConfigLimpieza) { c.AdmitenNegativos = []string{"benceno"} },
	}
	for nombre, romper := range casos {
		t.Run(nombre, func(t *testing.T) {
			c := ConfigPorDefecto()
			romper(&c)
			if _, err := c.compilar(); err == nil {
				t.Fatal("la config inválida compiló")
			}
		})
	}
}

func TestMapeoParametroAceptaTextoUObjeto(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "c.json")
	cfg := ConfigPorDefecto()
	cfg.Parametros = nil
	if err := GuardarJSON(ruta, cfg); err != nil {
		t.Fatal(err)
	}
	crudo, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatal(err)
	}
	reemplazo := `"parametros": {"PM10": "pm10", "CO (mg/m3)": {"canonico": "co", "factor": 1000}}`
	texto := strings.Replace(string(crudo), `"parametros": null`, reemplazo, 1)
	if err := os.WriteFile(ruta, []byte(texto), 0o644); err != nil {
		t.Fatal(err)
	}
	leida, err := LeerConfigLimpieza(ruta)
	if err != nil {
		t.Fatal(err)
	}
	if m := leida.Parametros["PM10"]; m.Canonico != "pm10" || m.Factor != 1 {
		t.Errorf("PM10: %+v", m)
	}
	if m := leida.Parametros["CO (mg/m3)"]; m.Canonico != "co" || m.Factor != 1000 {
		t.Errorf("CO: %+v", m)
	}
}

func TestNormalizar(t *testing.T) {
	casos := map[string]string{
		"  Humedad   Relativa ": "humedad relativa",
		"Dirección del Viento":  "direccion del viento",
		"µg/m³":                 "ug/m3",
		"μg/m3":                 "ug/m3",
		"W/m²":                  "w/m2",
		"PM2.5":                 "pm2.5",
		"AÑO":                   "ano",
	}
	for entrada, quiero := range casos {
		if got := normalizar(entrada); got != quiero {
			t.Errorf("normalizar(%q) = %q, quiero %q", entrada, got, quiero)
		}
	}
}

func TestRecetaPorDefectoSiNoExisteElArchivo(t *testing.T) {
	r, provisional, err := LeerReceta(filepath.Join(t.TempDir(), "no-existe.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !provisional {
		t.Error("sin archivo, la receta tiene que marcarse como provisional")
	}
	if !slices.Equal(r.Features, []string{"pm10", "fraccion_fina", "viento_velocidad", "humedad"}) ||
		!slices.Equal(r.Log1p, []string{"pm10"}) || r.Nivel != NivelAbsoluto {
		t.Errorf("receta provisional distinta del contrato: %+v", r)
	}
}

func TestRecetaSeLeeDelArchivo(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "kmeans.json")
	texto := `{"features": ["pm2_5", "pm10"], "log1p": ["pm2_5", "pm10"], "nivel": "estacion",
		"k": 4, "semilla": 7, "tolerancia": 1e-6, "max_iter": 50, "workers": 0, "chunk": 5000, "campo_futuro": 1}`
	if err := os.WriteFile(ruta, []byte(texto), 0o644); err != nil {
		t.Fatal(err)
	}
	r, provisional, err := LeerReceta(ruta)
	if err != nil {
		t.Fatal(err)
	}
	if provisional || r.Nivel != NivelEstacion || r.Chunk != 5000 || len(r.Features) != 2 {
		t.Errorf("receta mal leída: %+v (provisional=%v)", r, provisional)
	}
}

func TestRecetaInvalida(t *testing.T) {
	casos := map[string]Receta{
		"feature desconocida": {Features: []string{"pm10", "hora"}, Nivel: NivelAbsoluto},
		"sin features":        {Nivel: NivelAbsoluto},
		"feature repetida":    {Features: []string{"pm10", "pm10"}, Nivel: NivelAbsoluto},
		"log1p fuera":         {Features: []string{"pm10"}, Log1p: []string{"pm2_5"}, Nivel: NivelAbsoluto},
		"nivel desconocido":   {Features: []string{"pm10"}, Nivel: "region"},
		"chunk negativo":      {Features: []string{"pm10"}, Nivel: NivelAbsoluto, Chunk: -1},
	}
	for nombre, r := range casos {
		t.Run(nombre, func(t *testing.T) {
			if err := r.Validar(); err == nil {
				t.Fatal("la receta inválida pasó")
			}
		})
	}
}

func TestRecetaArchivoRoto(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "kmeans.json")
	if err := os.WriteFile(ruta, []byte(`{"features": [`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LeerReceta(ruta); err == nil || errors.Is(err, os.ErrNotExist) {
		t.Fatalf("un kmeans.json roto tiene que fallar, no caer en la receta provisional: %v", err)
	}
}
