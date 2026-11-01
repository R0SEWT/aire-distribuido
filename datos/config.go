package datos

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode"
)

// ParametrosCanonicos son los nombres internos de los parámetros, en orden de
// bytes: es el orden de las columnas del silver. El índice de un parámetro en
// este arreglo es su posición en el vector de cada fila ancha.
var ParametrosCanonicos = [...]string{
	"co", "humedad", "no2", "o3", "pm10", "pm2_5", "precipitacion", "presion",
	"radiacion", "so2", "temperatura", "viento_direccion", "viento_velocidad",
}

// numParametros es P, el largo del vector de una fila ancha. Cabe en un uint16
// (las máscaras de conflicto por fila usan un bit por parámetro).
const numParametros = len(ParametrosCanonicos)

// FeatureFraccionFina es la única feature derivada: pm2_5/pm10.
const FeatureFraccionFina = "fraccion_fina"

func indiceParametro(nombre string) int {
	for i, p := range ParametrosCanonicos {
		if p == nombre {
			return i
		}
	}
	return -1
}

var (
	idxPM25 = indiceParametro("pm2_5")
	idxPM10 = indiceParametro("pm10")
)

// ConfigLimpieza es configs/limpieza.json: todo lo que depende del formato real
// del CSV de OEFA y de los umbrales de las reglas. La semántica de cada campo
// está en doc.go.
type ConfigLimpieza struct {
	Supuestos        []string                      `json:"_supuestos,omitempty"`
	Delimitador      string                        `json:"delimitador"`
	Codificacion     string                        `json:"codificacion"`
	SeparadorDecimal string                        `json:"separador_decimal"`
	ZonaHoraria      string                        `json:"zona_horaria"`
	Columnas         Columnas                      `json:"columnas"`
	Parametros       map[string]MapeoParametro     `json:"parametros"`
	PeriodosHorarios []string                      `json:"periodos_horarios"`
	FormatosFecha    []string                      `json:"formatos_fecha"`
	FechaDesde       string                        `json:"fecha_desde"`
	FechaHasta       string                        `json:"fecha_hasta"`
	Centinelas       []float64                     `json:"centinelas"`
	AdmitenNegativos []string                      `json:"admiten_negativos"`
	Rangos           map[string][2]float64         `json:"rangos"`
	Unidades         map[string]map[string]float64 `json:"unidades"`
}

// Columnas mapea cada columna lógica al nombre real de la cabecera. Vacío
// significa «el CSV no la tiene». Estación, fecha, parámetro y valor son
// obligatorias.
type Columnas struct {
	Estacion     string `json:"estacion"`
	Fecha        string `json:"fecha"`
	Hora         string `json:"hora"`
	Parametro    string `json:"parametro"`
	Valor        string `json:"valor"`
	Periodo      string `json:"periodo"`
	Unidad       string `json:"unidad"`
	Departamento string `json:"departamento"`
	Provincia    string `json:"provincia"`
	Distrito     string `json:"distrito"`
	Ubigeo       string `json:"ubigeo"`
	Este         string `json:"este"`
	Norte        string `json:"norte"`
	Altitud      string `json:"altitud"`
	Zona         string `json:"zona"`
	Datum        string `json:"datum"`
}

// columnasUbicacion son las columnas que solo se guardan una vez por estación
// (regla 9, TB2). Que falten no impide cargar: se avisa en el reporte.
func (c Columnas) ubicacion() [][2]string {
	return [][2]string{
		{"departamento", c.Departamento}, {"provincia", c.Provincia}, {"distrito", c.Distrito},
		{"ubigeo", c.Ubigeo}, {"este", c.Este}, {"norte", c.Norte}, {"altitud", c.Altitud},
		{"zona", c.Zona}, {"datum", c.Datum},
	}
}

// MapeoParametro lleva un nombre real de parámetro a su canónico, con un factor
// opcional para cuando la unidad viene en el nombre («CO (mg/m3)»). En el JSON
// se escribe como texto ("pm10") o como objeto ({"canonico": "co", "factor": 1000}).
type MapeoParametro struct {
	Canonico string  `json:"canonico"`
	Factor   float64 `json:"factor"`
}

func (m *MapeoParametro) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) > 0 && b[0] == '"' {
		m.Factor = 1
		return json.Unmarshal(b, &m.Canonico)
	}
	type plano MapeoParametro
	p := plano{Factor: 1}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return err
	}
	*m = MapeoParametro(p)
	return nil
}

func (m MapeoParametro) MarshalJSON() ([]byte, error) {
	if m.Factor == 1 {
		return json.Marshal(m.Canonico)
	}
	type plano MapeoParametro
	return json.Marshal(plano(m))
}

// ConfigPorDefecto son los supuestos del 31/10 sobre el formato del CSV de
// OEFA, que nadie ha visto todavía. Describen exactamente el formato de la
// muestra sintética de data/sample/, y configs/limpieza.json arranca igual.
// Lo primero que se ajusta el lunes con el archivo en disco es el JSON, no esto.
func ConfigPorDefecto() ConfigLimpieza {
	return ConfigLimpieza{
		Supuestos: []string{
			"SUPUESTOS del 31/10, sin el CSV en disco: se verifican con el primer megabyte del archivo.",
			"Delimitador ';', UTF-8 (con o sin BOM), punto decimal, sin campos entrecomillados con saltos de línea.",
			"Formato largo: una fila por (estación, parámetro, fecha-hora); la estación es NOMBRE_PUNTO.",
			"Fecha y hora en una sola columna, dd/mm/aaaa HH:MM[:SS], hora local de Lima (UTC-5, sin horario de verano).",
			"Unidades en columna aparte; CO puede venir en mg/m3 y se lleva a ug/m3.",
			"Centinelas, rangos físicos y rango de fechas son provisionales hasta el EDA.",
		},
		Delimitador:      ";",
		Codificacion:     "utf-8",
		SeparadorDecimal: ".",
		ZonaHoraria:      "America/Lima",
		Columnas: Columnas{
			Estacion:     "NOMBRE_PUNTO",
			Fecha:        "FECHA",
			Parametro:    "PARAMETRO",
			Valor:        "VALOR",
			Periodo:      "PERIODO",
			Unidad:       "UNIDAD",
			Departamento: "DEPARTAMENTO",
			Provincia:    "PROVINCIA",
			Distrito:     "DISTRITO",
			Ubigeo:       "UBIGEO",
			Este:         "ESTE",
			Norte:        "NORTE",
			Altitud:      "ALTITUD",
			Zona:         "ZONA",
			Datum:        "DATUM",
		},
		Parametros: map[string]MapeoParametro{
			"PM2.5":                     {"pm2_5", 1},
			"PM 2.5":                    {"pm2_5", 1},
			"PM2,5":                     {"pm2_5", 1},
			"PM10":                      {"pm10", 1},
			"PM 10":                     {"pm10", 1},
			"NO2":                       {"no2", 1},
			"SO2":                       {"so2", 1},
			"CO":                        {"co", 1},
			"O3":                        {"o3", 1},
			"Temperatura":               {"temperatura", 1},
			"Temperatura del aire":      {"temperatura", 1},
			"Humedad Relativa":          {"humedad", 1},
			"Humedad":                   {"humedad", 1},
			"Velocidad del Viento":      {"viento_velocidad", 1},
			"Dirección del Viento":      {"viento_direccion", 1},
			"Presión Atmosférica":       {"presion", 1},
			"Presión":                   {"presion", 1},
			"Precipitación":             {"precipitacion", 1},
			"Radiación Solar":           {"radiacion", 1},
			"Radiación Solar Global":    {"radiacion", 1},
			"Velocidad de Viento":       {"viento_velocidad", 1},
			"Direccion de Viento":       {"viento_direccion", 1},
			"Presion Barometrica":       {"presion", 1},
			"Temperatura Ambiente":      {"temperatura", 1},
			"Humedad Relativa del Aire": {"humedad", 1},
		},
		PeriodosHorarios: []string{"Horario", "Hora", "1 h", "1h", "1 hora", "60 min"},
		FormatosFecha: []string{
			"02/01/2006 15:04",
			"02/01/2006 15:04:05",
			"2/1/2006 15:04",
			"2006-01-02 15:04:05",
			"2006-01-02 15:04",
			"2006-01-02T15:04:05",
		},
		FechaDesde:       "2000-01-01",
		FechaHasta:       "2026-07-01",
		Centinelas:       []float64{-999, -9999, 9999, 99999},
		AdmitenNegativos: []string{"temperatura"},
		Rangos: map[string][2]float64{
			"pm2_5":            {0, 1000},
			"pm10":             {0, 2000},
			"no2":              {0, 2000},
			"so2":              {0, 10000},
			"co":               {0, 50000},
			"o3":               {0, 1000},
			"temperatura":      {-30, 50},
			"humedad":          {0, 100},
			"viento_velocidad": {0, 60},
			"viento_direccion": {0, 360},
			"presion":          {400, 1100},
			"precipitacion":    {0, 200},
			"radiacion":        {0, 1500},
		},
		Unidades: map[string]map[string]float64{
			"pm2_5":            {"ug/m3": 1},
			"pm10":             {"ug/m3": 1},
			"no2":              {"ug/m3": 1},
			"so2":              {"ug/m3": 1},
			"o3":               {"ug/m3": 1},
			"co":               {"ug/m3": 1, "mg/m3": 1000},
			"temperatura":      {"°c": 1, "c": 1},
			"humedad":          {"%": 1},
			"viento_velocidad": {"m/s": 1},
			"viento_direccion": {"°": 1, "grados": 1},
			"presion":          {"hpa": 1, "mbar": 1, "mb": 1},
			"precipitacion":    {"mm": 1},
			"radiacion":        {"w/m2": 1},
		},
	}
}

// LeerConfigLimpieza lee configs/limpieza.json. Un campo desconocido es un
// error: un umbral mal escrito no puede quedar en silencio con su valor cero.
func LeerConfigLimpieza(ruta string) (ConfigLimpieza, error) {
	var c ConfigLimpieza
	crudo, err := os.ReadFile(ruta)
	if err != nil {
		return c, err
	}
	dec := json.NewDecoder(bytes.NewReader(crudo))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return c, fmt.Errorf("%s: %w", ruta, err)
	}
	return c, nil
}

// GuardarJSON escribe v indentado, creando el directorio si hace falta.
func GuardarJSON(ruta string, v any) error {
	if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ruta, append(b, '\n'), 0o644)
}

// codificacion del archivo de entrada.
type codificacion int

const (
	codUTF8 codificacion = iota
	codLatin1
	codWindows1252
)

// reglas es la config ya validada y lista para el camino caliente: búsquedas
// por índice de parámetro en vez de mapas por nombre.
type reglas struct {
	delimitador  byte // 0 = detectar en la cabecera
	cod          codificacion
	decimal      byte
	zona         *time.Location
	columnas     Columnas
	parametros   map[string]MapeoParametro // nombre normalizado → canónico
	periodos     map[string]bool           // valores normalizados que cuentan como horario
	formatos     []string
	desde, hasta int64 // [desde, hasta) en segundos Unix
	centinelas   []float64
	negativos    [numParametros]bool
	conRango     [numParametros]bool
	rango        [numParametros][2]float64
	unidades     [numParametros]map[string]float64 // nil = sin chequeo de unidad
}

func (c ConfigLimpieza) compilar() (*reglas, error) {
	r := &reglas{columnas: c.Columnas, formatos: c.FormatosFecha}
	var errs []error
	mal := func(formato string, a ...any) { errs = append(errs, fmt.Errorf(formato, a...)) }

	switch d := c.Delimitador; {
	case d == "" || d == "auto":
		r.delimitador = 0
	case d == `\t`:
		r.delimitador = '\t'
	case len(d) == 1 && d != `"` && d != "\n" && d != "\r":
		r.delimitador = d[0]
	default:
		mal("delimitador %q: tiene que ser un solo carácter, \\t o \"auto\"", d)
	}

	switch strings.ToLower(strings.ReplaceAll(c.Codificacion, "_", "-")) {
	case "", "utf-8", "utf8":
		r.cod = codUTF8
	case "latin1", "latin-1", "iso-8859-1":
		r.cod = codLatin1
	case "windows-1252", "cp1252":
		r.cod = codWindows1252
	default:
		mal("codificación %q: se admite utf-8, latin1 o windows-1252", c.Codificacion)
	}

	switch c.SeparadorDecimal {
	case "", ".":
		r.decimal = '.'
	case ",":
		r.decimal = ','
	default:
		mal("separador decimal %q: tiene que ser \".\" o \",\"", c.SeparadorDecimal)
	}

	zona := c.ZonaHoraria
	if zona == "" {
		zona = "America/Lima"
	}
	loc, err := time.LoadLocation(zona)
	if err != nil {
		mal("zona horaria %q: %v", zona, err)
		loc = time.UTC
	}
	r.zona = loc

	for nombre, valor := range map[string]string{
		"estacion": c.Columnas.Estacion, "fecha": c.Columnas.Fecha,
		"parametro": c.Columnas.Parametro, "valor": c.Columnas.Valor,
	} {
		if strings.TrimSpace(valor) == "" {
			mal("columnas.%s es obligatoria", nombre)
		}
	}

	r.parametros = make(map[string]MapeoParametro, len(c.Parametros))
	nombres := make([]string, 0, len(c.Parametros))
	for real := range c.Parametros {
		nombres = append(nombres, real)
	}
	slices.Sort(nombres) // mensajes de error deterministas
	for _, real := range nombres {
		m := c.Parametros[real]
		if indiceParametro(m.Canonico) < 0 {
			mal("parametros[%q] = %q: no es un parámetro canónico (%s)", real, m.Canonico,
				strings.Join(ParametrosCanonicos[:], ", "))
			continue
		}
		if !(m.Factor > 0) || math.IsInf(m.Factor, 0) {
			mal("parametros[%q]: el factor tiene que ser positivo, es %v", real, m.Factor)
			continue
		}
		clave := normalizar(real)
		if previo, ok := r.parametros[clave]; ok && previo != m {
			mal("parametros: %q colisiona con otro nombre que normaliza a %q y va a otro canónico", real, clave)
			continue
		}
		r.parametros[clave] = m
	}
	if len(r.parametros) == 0 {
		mal("parametros está vacío: ninguna fila pasaría la regla de parámetro")
	}

	r.periodos = make(map[string]bool, len(c.PeriodosHorarios))
	for _, p := range c.PeriodosHorarios {
		r.periodos[normalizar(p)] = true
	}
	if c.Columnas.Periodo != "" && len(r.periodos) == 0 {
		mal("periodos_horarios está vacío y hay columna de periodo: se descartaría todo")
	}

	if len(c.FormatosFecha) == 0 {
		mal("formatos_fecha está vacío")
	}
	desde, err1 := parsearLimite(c.FechaDesde, loc)
	hasta, err2 := parsearLimite(c.FechaHasta, loc)
	switch {
	case err1 != nil:
		mal("fecha_desde: %v", err1)
	case err2 != nil:
		mal("fecha_hasta: %v", err2)
	case !desde.Before(hasta):
		mal("fecha_desde (%s) tiene que ser anterior a fecha_hasta (%s)", c.FechaDesde, c.FechaHasta)
	}
	r.desde, r.hasta = desde.Unix(), hasta.Unix()

	r.centinelas = slices.Clone(c.Centinelas)

	for _, p := range c.AdmitenNegativos {
		i := indiceParametro(p)
		if i < 0 {
			mal("admiten_negativos: %q no es un parámetro canónico", p)
			continue
		}
		r.negativos[i] = true
	}

	for p, lim := range c.Rangos {
		i := indiceParametro(p)
		if i < 0 {
			mal("rangos: %q no es un parámetro canónico", p)
			continue
		}
		if !(lim[0] <= lim[1]) {
			mal("rangos[%q] = %v: el mínimo supera al máximo", p, lim)
			continue
		}
		r.conRango[i], r.rango[i] = true, lim
	}

	for p, us := range c.Unidades {
		i := indiceParametro(p)
		if i < 0 {
			mal("unidades: %q no es un parámetro canónico", p)
			continue
		}
		r.unidades[i] = make(map[string]float64, len(us))
		for u, f := range us {
			if !(f > 0) || math.IsInf(f, 0) {
				mal("unidades[%q][%q]: el factor tiene que ser positivo, es %v", p, u, f)
				continue
			}
			r.unidades[i][normalizar(u)] = f
		}
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("config de limpieza inválida: %w", errors.Join(errs...))
	}
	return r, nil
}

// parsearLimite acepta una fecha (aaaa-mm-dd, medianoche en la zona) o RFC3339.
func parsearLimite(s string, loc *time.Location) (time.Time, error) {
	if t, err := time.ParseInLocation("2006-01-02", s, loc); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, s)
}

// normalizar lleva un nombre (de parámetro, periodo, unidad o columna) a una
// forma comparable: minúsculas, sin tildes, µ y μ como u, superíndices como
// dígitos y espacios colapsados. Así «Humedad  relativa» y «HUMEDAD RELATIVA»
// son el mismo parámetro, y «µg/m³» la misma unidad que «ug/m3».
func normalizar(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	espacio := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			espacio = b.Len() > 0
			continue
		}
		if espacio {
			b.WriteByte(' ')
			espacio = false
		}
		r = unicode.ToLower(r)
		switch r {
		case 'á', 'à', 'ä', 'â':
			r = 'a'
		case 'é', 'è', 'ë', 'ê':
			r = 'e'
		case 'í', 'ì', 'ï', 'î':
			r = 'i'
		case 'ó', 'ò', 'ö', 'ô':
			r = 'o'
		case 'ú', 'ù', 'ü', 'û', 'µ', 'μ':
			r = 'u'
		case 'ñ':
			r = 'n'
		case '³':
			r = '3'
		case '²':
			r = '2'
		}
		b.WriteRune(r)
	}
	return b.String()
}

// Niveles de estandarización de la receta.
const (
	NivelAbsoluto = "absoluto"
	NivelEstacion = "estacion"
)

// ChunkPorDefecto es el tamaño de chunk (filas) de la estandarización cuando ni
// la receta ni el flag lo fijan.
const ChunkPorDefecto = 10000

// Receta es configs/kmeans.json, que define el agente del K-Means. El cargador
// solo usa features, log1p, nivel y chunk; el resto viaja tal cual.
type Receta struct {
	Features   []string `json:"features"`
	Log1p      []string `json:"log1p"`
	Nivel      string   `json:"nivel"`
	K          int      `json:"k"`
	Semilla    int64    `json:"semilla"`
	Tolerancia float64  `json:"tolerancia"`
	MaxIter    int      `json:"max_iter"`
	Workers    int      `json:"workers"`
	Chunk      int      `json:"chunk"`
}

// RecetaProvisional es la receta de la spec si la calibración no cierra a
// tiempo: PM10 y fracción fina, viento y humedad, log1p en PM10, absoluto.
func RecetaProvisional() Receta {
	return Receta{
		Features:   []string{"pm10", FeatureFraccionFina, "viento_velocidad", "humedad"},
		Log1p:      []string{"pm10"},
		Nivel:      NivelAbsoluto,
		K:          5,
		Semilla:    42,
		Tolerancia: 1e-6,
		MaxIter:    100,
		Workers:    0,
		Chunk:      ChunkPorDefecto,
	}
}

// LeerReceta lee configs/kmeans.json. Si el archivo no existe devuelve la
// receta provisional y provisional = true; si existe y está roto, es un error.
// No rechaza campos desconocidos: el archivo es de otro paquete y puede crecer.
func LeerReceta(ruta string) (r Receta, provisional bool, err error) {
	crudo, err := os.ReadFile(ruta)
	if errors.Is(err, os.ErrNotExist) {
		return RecetaProvisional(), true, nil
	}
	if err != nil {
		return r, false, err
	}
	if err := json.Unmarshal(crudo, &r); err != nil {
		return r, false, fmt.Errorf("%s: %w", ruta, err)
	}
	if err := r.Validar(); err != nil {
		return r, false, fmt.Errorf("%s: %w", ruta, err)
	}
	return r, false, nil
}

// Validar comprueba lo que el cargador necesita de la receta.
func (r Receta) Validar() error {
	if len(r.Features) == 0 {
		return errors.New("receta: features está vacío")
	}
	vistas := map[string]bool{}
	for _, f := range r.Features {
		if f != FeatureFraccionFina && indiceParametro(f) < 0 {
			return fmt.Errorf("receta: la feature %q no es un parámetro canónico ni %q", f, FeatureFraccionFina)
		}
		if vistas[f] {
			return fmt.Errorf("receta: la feature %q está repetida", f)
		}
		vistas[f] = true
	}
	for _, f := range r.Log1p {
		if !vistas[f] {
			return fmt.Errorf("receta: log1p incluye %q, que no está en features", f)
		}
	}
	if r.Nivel != NivelAbsoluto && r.Nivel != NivelEstacion {
		return fmt.Errorf("receta: nivel %q, tiene que ser %q o %q", r.Nivel, NivelAbsoluto, NivelEstacion)
	}
	if r.Chunk < 0 {
		return fmt.Errorf("receta: chunk negativo (%d)", r.Chunk)
	}
	return nil
}
