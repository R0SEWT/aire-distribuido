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
	quiero := []string{"pm2_5", "pm10", "no2", "so2", "co", "o3", "h2s", "temperatura", "humedad",
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
	reg, err := cfg.compilar()
	if err != nil {
		t.Fatal(err)
	}
	// Los tres hallazgos del análisis de procedencia están en la config versionada.
	if reg.correcciones["CA-HUAR-01"]["zona"] != "18" || !reg.ceroFaltante[idxPM25] || !reg.ceroFaltante[idxPM10] ||
		reg.ceroFaltante[indiceParametro("precipitacion")] || cfg.Columnas.TipoAnalisis != "TIPO_ANALISIS" {
		t.Errorf("configs/limpieza.json: correcciones %v, cero_como_faltante %v, tipo_analisis %q",
			reg.correcciones, cfg.CeroComoFaltante, cfg.Columnas.TipoAnalisis)
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
		"falta la columna de estación":  func(c *ConfigLimpieza) { c.Columnas.Estacion = "" },
		"falta la columna de fecha":     func(c *ConfigLimpieza) { c.Columnas.Fecha = "" },
		"canónico inexistente":          func(c *ConfigLimpieza) { c.ParametrosEnColumnas["BENCENO"] = "benceno" },
		"sin columnas de parámetro":     func(c *ConfigLimpieza) { c.ParametrosEnColumnas = nil },
		"columnas que colisionan": func(c *ConfigLimpieza) {
			c.ParametrosEnColumnas["pm 10"] = "pm2_5"
			c.ParametrosEnColumnas["PM 10"] = "pm10"
		},
		"periodo en las dos listas":     func(c *ConfigLimpieza) { c.PeriodosPorGrupo = []string{"1 Hora"} },
		"sin periodos horarios":         func(c *ConfigLimpieza) { c.PeriodosHorarios, c.PeriodosPorGrupo = nil, nil },
		"sin formatos de fecha":         func(c *ConfigLimpieza) { c.FormatosFecha = nil },
		"modo parámetro/valor":          func(c *ConfigLimpieza) { c.Columnas.Parametro = "PARAMETRO" },
		"parametros del modo viejo":     func(c *ConfigLimpieza) { c.Parametros = map[string]any{"PM10": "pm10"} },
		"hora_solo_prefijo desconocida": func(c *ConfigLimpieza) { c.HoraSoloPrefijo = "mediodia" },
		"hora_explicita desconocida":    func(c *ConfigLimpieza) { c.HoraExplicita = "medio" },
		"hora_no_en_punto desconocida":  func(c *ConfigLimpieza) { c.HoraNoEnPunto = "redondear" },
		"convención desconocida":        func(c *ConfigLimpieza) { c.HoraPorPosicion.Convencion = "medio" },
		"filas por día cero":            func(c *ConfigLimpieza) { c.HoraPorPosicion.FilasPorDia = 0 },
		"rango de fechas al revés":      func(c *ConfigLimpieza) { c.FechaDesde, c.FechaHasta = c.FechaHasta, c.FechaDesde },
		"rango físico al revés":         func(c *ConfigLimpieza) { c.Rangos["pm10"] = [2]float64{10, 0} },
		"rango de un no canónico":       func(c *ConfigLimpieza) { c.Rangos["benceno"] = [2]float64{0, 1} },
		"unidad con factor cero":        func(c *ConfigLimpieza) { c.Unidades["co"]["ppb"] = 0 },
		"negativo de un no canónico":    func(c *ConfigLimpieza) { c.AdmitenNegativos = []string{"benceno"} },
		"bloque en cero no canónico":    func(c *ConfigLimpieza) { c.BloquesEnCero = []string{"benceno"} },
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

func TestNormalizar(t *testing.T) {
	casos := map[string]string{
		"  Humedad   Relativa ": "humedad relativa",
		"Dirección del Viento":  "direccion del viento",
		"µg/m³":                 "ug/m3",
		"μg/m3":                 "ug/m3",
		"W/m²":                  "w/m2",
		"PM2.5":                 "pm2.5",
		"AÑO":                   "ano",
		"ºC":                    "°c",
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

// TestConfigDelLaboratorio: la config que escribe notebooks/aire_lab.py trae las
// claves vacías del modo parámetro/valor y no trae formatos_hora; tiene que
// leerse igual.
func TestConfigDelLaboratorio(t *testing.T) {
	cfg := ConfigPorDefecto()
	cfg.FormatosHora, cfg.HoraSoloPrefijo, cfg.HoraExplicita, cfg.HoraNoEnPunto = nil, "", "", ""
	ruta := filepath.Join(t.TempDir(), "limpieza_propuesta_real.json")
	if err := GuardarJSON(ruta, cfg); err != nil {
		t.Fatal(err)
	}
	crudo, _ := os.ReadFile(ruta)
	texto := strings.Replace(string(crudo), `"datum": "DATUM"`, `"datum": "DATUM", "parametro": "", "valor": ""`, 1)
	texto = strings.Replace(texto, `"zona_horaria"`, `"parametros": {}, "zona_horaria"`, 1)
	if err := os.WriteFile(ruta, []byte(texto), 0o644); err != nil {
		t.Fatal(err)
	}
	leida, err := LeerConfigLimpieza(ruta)
	if err != nil {
		t.Fatal(err)
	}
	reg, err := leida.compilar()
	if err != nil {
		t.Fatal(err)
	}
	if !reg.horaInicio || reg.soloPrefijoEs != 0 || reg.horaTruncar || len(reg.formatosHora) != 2 {
		t.Errorf("valores por defecto de la hora: %+v", reg)
	}
}
