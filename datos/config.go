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
	"co", "h2s", "humedad", "no2", "o3", "pm10", "pm2_5", "precipitacion", "presion",
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
	Supuestos            []string                      `json:"_supuestos,omitempty"`
	Delimitador          string                        `json:"delimitador"`
	Codificacion         string                        `json:"codificacion"`
	SeparadorDecimal     string                        `json:"separador_decimal"`
	ZonaHoraria          string                        `json:"zona_horaria"`
	ValoresVacios        []string                      `json:"valores_vacios"`
	Columnas             Columnas                      `json:"columnas"`
	ParametrosEnColumnas map[string]string             `json:"parametros_en_columnas"`
	PeriodosHorarios     []string                      `json:"periodos_horarios"`
	PeriodosPorGrupo     []string                      `json:"periodos_horarios_por_grupo"`
	FormatosFecha        []string                      `json:"formatos_fecha"`
	FormatosHora         []string                      `json:"formatos_hora"`
	HoraPrefijoIgnorar   string                        `json:"hora_prefijo_ignorar"`
	HoraSoloPrefijo      string                        `json:"hora_solo_prefijo"`
	HoraExplicita        string                        `json:"hora_explicita"`
	HoraNoEnPunto        string                        `json:"hora_no_en_punto"`
	HoraPorPosicion      HoraPorPosicion               `json:"hora_por_posicion"`
	FechaDesde           string                        `json:"fecha_desde"`
	FechaHasta           string                        `json:"fecha_hasta"`
	Centinelas           []float64                     `json:"centinelas"`
	AdmitenNegativos     []string                      `json:"admiten_negativos"`
	Rangos               map[string][2]float64         `json:"rangos"`
	Unidades             map[string]map[string]float64 `json:"unidades"`
	BloquesEnCero        []string                      `json:"bloques_en_cero_como_faltante"`
	// Parametros es del modo parámetro/valor, que se retiró. Se acepta vacío
	// para leer las configs del laboratorio en Python, que aún lo escriben.
	Parametros map[string]any `json:"parametros,omitempty"`
}

// HoraPorPosicion reconstruye la hora cuando la fila no la trae: las filas de un
// mismo (estación, parámetro, fecha, unidad, periodo) son las horas del día en
// orden, si son exactamente FilasPorDia. Convencion dice qué instante del
// intervalo horario se escribe en fecha_hora: "fin" (OEFA) o "inicio"; vale
// también para las horas explícitas.
type HoraPorPosicion struct {
	Activar     bool   `json:"activar"`
	FilasPorDia int    `json:"filas_por_dia"`
	Convencion  string `json:"convencion"`
}

// Columnas mapea cada columna lógica al nombre real de la cabecera. Vacío
// significa «el CSV no la tiene». Estación y fecha son obligatorias; los
// parámetros van en parametros_en_columnas.
type Columnas struct {
	Estacion     string `json:"estacion"`
	Fecha        string `json:"fecha"`
	Hora         string `json:"hora"`
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
	// Del modo parámetro/valor, retirado: se aceptan vacíos.
	Parametro string `json:"parametro,omitempty"`
	Valor     string `json:"valor,omitempty"`
}

// ubicacion son las columnas que describen dónde está la estación (regla 9,
// TB2). Que falten no impide cargar: se avisa en el reporte.
func (c Columnas) ubicacion() [][2]string {
	return [][2]string{
		{"departamento", c.Departamento}, {"provincia", c.Provincia}, {"distrito", c.Distrito},
		{"ubigeo", c.Ubigeo}, {"este", c.Este}, {"norte", c.Norte}, {"altitud", c.Altitud},
		{"zona", c.Zona}, {"datum", c.Datum},
	}
}

// ConfigPorDefecto es el formato real del CSV de OEFA, verificado el 01/11 sobre
// el archivo completo (spec §4.1), y es exactamente el de las dos muestras de
// data/sample/. configs/limpieza.json arranca igual; los umbrales (centinelas,
// rangos, fechas) siguen siendo provisionales hasta el EDA.
func ConfigPorDefecto() ConfigLimpieza {
	return ConfigLimpieza{
		Supuestos: []string{
			"Formato verificado el 01/11 sobre el CSV completo: 36 columnas, ';', textos entre comillas, UTF-8 sin BOM, punto decimal.",
			"Una columna por parámetro y cada fila con un solo valor; '-' y 'NA' son vacíos.",
			"HORA_INICIO es el inicio del intervalo: 1899-12-31 HH:00:00 (artefacto de Excel); 1899-12-31 solo es la medianoche (posición 0 del día, verificado en 4,691 días).",
			"Sin hora (NA, 94 %): la hora es la posición dentro del (estación, parámetro, fecha, unidad, periodo) si tiene 24 filas; en los días con hora explícita, hora = posición en el 100 %.",
			"fecha_hora del silver es el fin del intervalo (convención de OEFA): la lectura de 00:00 a 01:00 se fecha a las 01:00.",
			"Gases en ppb y ug/m3 a la vez: se prefiere ug/m3; ppb se convierte a 25 °C y 1 atm si es la única (el EDA midió 1.000-1.004 entre gemelas). Presión: mmHg a hPa. DIRECCION_VIENTO trae unidad '°c' por error: son grados.",
			"Un día entero en 0 exacto es equipo sin medir, no una medición (EDA: 95-99.8 % de los ceros vienen así); la precipitación queda fuera: un día seco es 0.",
			"Centinelas, rangos físicos y rango de fechas son provisionales hasta el EDA.",
		},
		Delimitador:      ";",
		Codificacion:     "utf-8",
		SeparadorDecimal: ".",
		ZonaHoraria:      "America/Lima",
		ValoresVacios:    []string{"-", "NA", ""},
		Columnas: Columnas{
			Estacion:     "NOMBRE_PUNTO",
			Fecha:        "FECHA_INICIO",
			Hora:         "HORA_INICIO",
			Periodo:      "PERIODO",
			Unidad:       "UNIDAD_MEDIDA",
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
		ParametrosEnColumnas: map[string]string{
			"PM10":                "pm10",
			"PM2.5":               "pm2_5",
			"DIOXIDO_AZUFRE":      "so2",
			"MONOXIDO_CARBONO":    "co",
			"OZONO":               "o3",
			"SULFURO_HIDROGENO":   "h2s",
			"TEMPERATURA":         "temperatura",
			"HUMEDAD_RELATIVA":    "humedad",
			"VELOCIDAD_VIENTO":    "viento_velocidad",
			"DIRECCION_VIENTO":    "viento_direccion",
			"PRESION_BAROMETRICA": "presion",
			"PRECIPITACION":       "precipitacion",
			"RADIACION_SOLAR":     "radiacion",
		},
		PeriodosHorarios:   []string{"1 hora"},
		PeriodosPorGrupo:   []string{"-"},
		FormatosFecha:      []string{"2006-01-02"},
		FormatosHora:       []string{"15:04:05", "15:04"},
		HoraPrefijoIgnorar: "1899-12-31",
		HoraSoloPrefijo:    HoraMedianoche,
		HoraExplicita:      HoraInicio,
		HoraNoEnPunto:      HoraDescartar,
		HoraPorPosicion:    HoraPorPosicion{Activar: true, FilasPorDia: 24, Convencion: HoraFin},
		FechaDesde:         "2000-01-01",
		FechaHasta:         "2026-07-01",
		Centinelas:         []float64{-999, -9999, 9999, 99999},
		AdmitenNegativos:   []string{"temperatura"},
		Rangos: map[string][2]float64{
			"pm2_5":            {0, 1000},
			"pm10":             {0, 2000},
			"no2":              {0, 2000},
			"so2":              {0, 10000},
			"co":               {0, 50000},
			"o3":               {0, 1000},
			"h2s":              {0, 5000},
			"temperatura":      {-30, 50},
			"humedad":          {0, 100},
			"viento_velocidad": {0, 60},
			"viento_direccion": {0, 360},
			"presion":          {400, 1100},
			"precipitacion":    {0, 200},
			"radiacion":        {0, 1500},
		},
		// La unidad con factor 1 es la canónica y se prefiere; las demás se
		// convierten (gases: µg/m³ por ppb a 25 °C y 1 atm).
		Unidades: map[string]map[string]float64{
			"pm2_5":            {"ug/m3": 1},
			"pm10":             {"ug/m3": 1},
			"no2":              {"ug/m3": 1, "ppb": 1.88},
			"so2":              {"ug/m3": 1, "ppb": 2.62},
			"co":               {"ug/m3": 1, "ppb": 1.145, "mg/m3": 1000},
			"o3":               {"ug/m3": 1, "ppb": 1.96},
			"h2s":              {"ug/m3": 1, "ppb": 1.39},
			"temperatura":      {"°c": 1, "c": 1},
			"humedad":          {"%": 1},
			"viento_velocidad": {"m/s": 1},
			"viento_direccion": {"°": 1, "grados": 1, "°c": 1}, // «°c» es un error de OEFA: son grados
			"presion":          {"hpa": 1, "mbar": 1, "mb": 1, "mmhg": 1.33322},
			"precipitacion":    {"mm": 1},
			"radiacion":        {"w/m2": 1},
		},
		BloquesEnCero: []string{"pm10", "pm2_5", "no2", "so2", "co", "o3", "h2s", "temperatura", "humedad",
			"viento_velocidad", "viento_direccion", "presion", "radiacion"},
	}
}

// Valores de las opciones de hora.
const (
	HoraMedianoche = "medianoche" // hora_solo_prefijo: el prefijo solo es 00:00
	HoraSinHora    = "sin_hora"   // hora_solo_prefijo: el prefijo solo es «sin hora»
	HoraInicio     = "inicio"     // la hora marca el inicio del intervalo
	HoraFin        = "fin"        // la hora marca el fin del intervalo
	HoraDescartar  = "descartar"  // hora_no_en_punto: regla hora_no_en_punto
	HoraTruncar    = "truncar"    // hora_no_en_punto: se lleva a la hora en punto y se marca
)

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

// Clases de periodo.
const (
	periodoDescartado  uint8 = iota // regla periodo
	periodoHorario                  // cuenta como horario
	periodoCondicional              // horario solo con hora explícita o con su día completo
)

// reglas es la config ya validada y lista para el camino caliente: búsquedas
// por índice de parámetro en vez de mapas por nombre.
type reglas struct {
	delimitador   byte // 0 = detectar en la cabecera
	cod           codificacion
	decimal       byte
	zona          *time.Location
	nulos         map[string]bool // valores crudos (recortados) que son «vacío» (valores_vacios)
	columnas      Columnas
	paramColumnas map[string]int      // nombre de columna normalizado → índice canónico
	periodos      map[string]uint8    // periodo normalizado → clase
	enCero        [numParametros]bool // bloques_en_cero_como_faltante
	formatosFecha []string
	formatosHora  []string
	horaPrefijo   string
	soloPrefijoEs uint8 // 0 = medianoche, 1 = sin hora
	horaInicio    bool  // la hora explícita marca el inicio del intervalo
	horaTruncar   bool  // hora_no_en_punto = truncar
	posicion      bool  // hora_por_posicion.activar
	filasPorDia   int
	convFin       bool // fecha_hora = fin del intervalo
	desde, hasta  int64
	centinelas    []float64
	negativos     [numParametros]bool
	conRango      [numParametros]bool
	rango         [numParametros][2]float64
	unidades      [numParametros]map[string]float64 // nil = sin chequeo de unidad
}

func (c ConfigLimpieza) compilar() (*reglas, error) {
	r := &reglas{columnas: c.Columnas, formatosFecha: c.FormatosFecha, formatosHora: c.FormatosHora,
		horaPrefijo: c.HoraPrefijoIgnorar}
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

	r.nulos = map[string]bool{"": true}
	for _, v := range c.ValoresVacios {
		r.nulos[strings.TrimSpace(v)] = true
	}

	if strings.TrimSpace(c.Columnas.Estacion) == "" {
		mal("columnas.estacion es obligatoria")
	}
	if strings.TrimSpace(c.Columnas.Fecha) == "" {
		mal("columnas.fecha es obligatoria")
	}
	if c.Columnas.Parametro != "" || c.Columnas.Valor != "" || len(c.Parametros) > 0 {
		mal("el modo parámetro/valor (columnas.parametro, columnas.valor, parametros) se retiró: usar parametros_en_columnas")
	}
	if len(r.formatosHora) == 0 {
		r.formatosHora = []string{"15:04:05", "15:04"}
	}

	r.paramColumnas = make(map[string]int, len(c.ParametrosEnColumnas))
	nombres := make([]string, 0, len(c.ParametrosEnColumnas))
	for col := range c.ParametrosEnColumnas {
		nombres = append(nombres, col)
	}
	slices.Sort(nombres) // mensajes de error deterministas
	for _, col := range nombres {
		canon := c.ParametrosEnColumnas[col]
		i := indiceParametro(canon)
		if i < 0 {
			mal("parametros_en_columnas[%q] = %q: no es un parámetro canónico (%s)", col, canon,
				strings.Join(ParametrosCanonicos[:], ", "))
			continue
		}
		clave := normalizar(col)
		if previo, ok := r.paramColumnas[clave]; ok && previo != i {
			mal("parametros_en_columnas: %q colisiona con otra columna que normaliza a %q", col, clave)
			continue
		}
		r.paramColumnas[clave] = i
	}
	if len(r.paramColumnas) == 0 {
		mal("parametros_en_columnas está vacío: ninguna fila tendría parámetro")
	}

	r.periodos = map[string]uint8{}
	for _, p := range c.PeriodosHorarios {
		r.periodos[normalizar(p)] = periodoHorario
	}
	for _, p := range c.PeriodosPorGrupo {
		if _, ok := r.periodos[normalizar(p)]; ok {
			mal("el periodo %q está en periodos_horarios y en periodos_horarios_por_grupo", p)
		}
		r.periodos[normalizar(p)] = periodoCondicional
	}
	if c.Columnas.Periodo != "" && len(r.periodos) == 0 {
		mal("no hay periodos horarios y hay columna de periodo: se descartaría todo")
	}

	if len(c.FormatosFecha) == 0 {
		mal("formatos_fecha está vacío")
	}
	switch c.HoraSoloPrefijo {
	case "", HoraMedianoche:
	case HoraSinHora:
		r.soloPrefijoEs = 1
	default:
		mal("hora_solo_prefijo %q: tiene que ser %q o %q", c.HoraSoloPrefijo, HoraMedianoche, HoraSinHora)
	}
	switch c.HoraExplicita {
	case "", HoraInicio:
		r.horaInicio = true
	case HoraFin:
	default:
		mal("hora_explicita %q: tiene que ser %q o %q", c.HoraExplicita, HoraInicio, HoraFin)
	}
	switch c.HoraNoEnPunto {
	case "", HoraDescartar:
	case HoraTruncar:
		r.horaTruncar = true
	default:
		mal("hora_no_en_punto %q: tiene que ser %q o %q", c.HoraNoEnPunto, HoraDescartar, HoraTruncar)
	}
	hp := c.HoraPorPosicion
	r.posicion, r.filasPorDia = hp.Activar, hp.FilasPorDia
	if hp.Activar && hp.FilasPorDia < 1 {
		mal("hora_por_posicion.filas_por_dia = %d: tiene que ser positivo", hp.FilasPorDia)
	}
	switch hp.Convencion {
	case "", HoraFin:
		r.convFin = true
	case HoraInicio:
	default:
		mal("hora_por_posicion.convencion %q: tiene que ser %q o %q", hp.Convencion, HoraFin, HoraInicio)
	}
	if len(c.PeriodosPorGrupo) > 0 && r.filasPorDia < 1 {
		mal("periodos_horarios_por_grupo necesita hora_por_posicion.filas_por_dia")
	}
	for _, p := range c.BloquesEnCero {
		i := indiceParametro(p)
		if i < 0 {
			mal("bloques_en_cero_como_faltante: %q no es un parámetro canónico", p)
			continue
		}
		r.enCero[i] = true
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
		case 'º': // ordinal, que a veces se usa por el signo de grado
			r = '°'
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
