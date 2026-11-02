package kmeans

import (
	"bufio"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
)

// ZonaPeru es la hora de Lima, UTC-5 sin horario de verano. El gold escribe
// fecha_hora en RFC3339 con -05:00 y la hora del día de los perfiles es esta.
var ZonaPeru = time.FixedZone("-05", -5*3600)

// Gold es data/gold/oefa_features.csv cargado en memoria: la matriz que ve el
// modelo y, paralelos a sus filas, la estación y la fecha-hora de cada lectura.
//
// Estacion y FechaHora NO son features (una estación es un identificador, no
// una coordenada; la hora y el mes arrastrarían el clustering a «día y noche»):
// van aparte para decir después dónde y cuándo aparece cada régimen. Se eligió
// un struct con Datos adentro, en vez de agregar campos a Datos, para que el
// núcleo del K-Means (Parciales, Secuencial, Concurrente) siga sin saber nada
// del dominio.
type Gold struct {
	Datos     *Datos
	Features  []string    // nombres de las D columnas de Datos, en orden
	Estacion  []string    // N; las filas de una misma estación comparten la cadena
	FechaHora []time.Time // N; en ZonaPeru
}

// Estaciones devuelve las estaciones en el orden en que aparecen.
func (g *Gold) Estaciones() []string {
	var out []string
	for i, e := range g.Estacion {
		if i == 0 || e != g.Estacion[i-1] {
			out = append(out, e)
		}
	}
	return out
}

// LeerGold carga el gold desde una ruta y valida su cabecera contra las
// features de la receta: estacion,fecha_hora y luego las features en ese orden.
func LeerGold(ruta string, features []string) (*Gold, error) {
	f, err := os.Open(ruta)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	g, err := LeerGoldDe(f, features)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ruta, err)
	}
	return g, nil
}

// LeerGoldDe carga el gold desde un lector. El contrato con el cargador:
//
//   - cabecera exacta estacion,fecha_hora,<features de la receta en orden>; una
//     permutación de columnas se leería sin error y daría un K-Means sobre
//     features equivocadas, así que se compara completa;
//   - solo filas completas y valores finitos (el K-Means no admite NaN);
//   - fecha_hora en RFC3339 con offset -05:00;
//   - filas ordenadas por estación y fecha_hora: las de una estación van juntas
//     y con fecha_hora estrictamente creciente (sin horas repetidas). El orden
//     de las estaciones entre sí lo decide el cargador; lo que se exige es que
//     sea un orden, porque de él dependen k-means++ (elige índices), la
//     submuestra sistemática y la suma por chunks.
//
// Usa encoding/csv: el cargador (datos.Dataset.EscribirGold) entrecomilla el
// nombre de una estación que tiene coma, comilla o salto de línea, con las
// comillas dobladas (RFC 4180), y partir por coma rechazaría «A,B» por columna
// extra. Las estaciones reales (CA-LI-01) no las tienen, pero la muestra
// sintética sí puede.
func LeerGoldDe(r io.Reader, features []string) (*Gold, error) {
	if len(features) == 0 {
		return nil, fmt.Errorf("gold: la receta no trae features")
	}
	esperada := "estacion,fecha_hora," + strings.Join(features, ",")

	cr := csv.NewReader(bufio.NewReaderSize(r, 1<<20))
	cr.FieldsPerRecord = -1 // la cantidad de columnas se valida aquí, con un mensaje propio
	cr.ReuseRecord = true
	cab, err := cr.Read()
	if errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("gold vacío: no tiene ni cabecera")
	}
	if err != nil {
		return nil, fmt.Errorf("leer la cabecera del gold: %w", err)
	}
	if got := strings.Join(cab, ","); got != esperada {
		return nil, fmt.Errorf("la cabecera del gold no coincide con la receta: es %q, se esperaba %q", got, esperada)
	}

	dim := len(features)
	g := &Gold{
		Datos:    &Datos{D: dim, X: make([]float64, 0, dim*4096)},
		Features: append([]string(nil), features...),
	}
	internas := map[string]string{} // una sola cadena por estación
	terminadas := map[string]bool{} // estaciones cuyo bloque ya se cerró
	for {
		campos, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("leer el gold: %w", err)
		}
		linea, _ := cr.FieldPos(0)
		if len(campos) != dim+2 {
			return nil, fmt.Errorf("línea %d del gold: %d columnas, se esperaban %d", linea, len(campos), dim+2)
		}

		est := campos[0]
		if est == "" {
			return nil, fmt.Errorf("línea %d del gold: estación vacía", linea)
		}
		if e, ok := internas[est]; ok {
			est = e
		} else {
			est = strings.Clone(est)
			internas[est] = est
		}
		t, err := time.Parse(time.RFC3339, campos[1])
		if err != nil {
			return nil, fmt.Errorf("línea %d del gold, fecha_hora: %q no es RFC3339", linea, campos[1])
		}
		if _, off := t.Zone(); off != -5*3600 {
			return nil, fmt.Errorf("línea %d del gold, fecha_hora: %q no está en -05:00", linea, campos[1])
		}
		t = t.In(ZonaPeru)

		if n := len(g.Estacion); n > 0 {
			previa := g.Estacion[n-1]
			switch {
			case est != previa && terminadas[est]:
				return nil, fmt.Errorf("línea %d del gold: la estación %s reaparece después de %s; "+
					"el gold debe venir ordenado por estación y fecha_hora", linea, est, previa)
			case est != previa:
				terminadas[previa] = true
			case !t.After(g.FechaHora[n-1]):
				return nil, fmt.Errorf("línea %d del gold: %s %s no va después de %s; "+
					"el gold debe venir en orden por fecha_hora y sin horas repetidas",
					linea, est, campos[1], g.FechaHora[n-1].Format(time.RFC3339))
			}
		}

		for f := 0; f < dim; f++ {
			v, err := strconv.ParseFloat(campos[f+2], 64)
			if err != nil {
				return nil, fmt.Errorf("línea %d del gold, columna %s: %q no es un número", linea, features[f], campos[f+2])
			}
			// Un NaN contamina el centroide entero y el criterio de parada deja
			// de tener sentido.
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return nil, fmt.Errorf("línea %d del gold, columna %s: valor no finito (%v)", linea, features[f], v)
			}
			g.Datos.X = append(g.Datos.X, v)
		}
		g.Datos.N++
		g.Estacion = append(g.Estacion, est)
		g.FechaHora = append(g.FechaHora, t)
	}
	if g.Datos.N == 0 {
		return nil, fmt.Errorf("el gold no tiene filas: solo la cabecera")
	}
	return g, nil
}

// EscribirGold escribe un Gold con el formato del contrato del cargador
// (floats con FormatFloat(x, 'g', -1, 64), que vuelve al mismo float64). En el
// sistema el gold lo escribe cmd/cargar; esto es para el gold sintético de los
// tests y de los ensayos sin datos reales.
func EscribirGold(w io.Writer, g *Gold) error {
	bw := bufio.NewWriter(w)
	bw.WriteString("estacion,fecha_hora," + strings.Join(g.Features, ",") + "\n")
	d := g.Datos
	var buf []byte
	for i := 0; i < d.N; i++ {
		buf = appendCampo(buf[:0], g.Estacion[i])
		buf = append(buf, ',')
		buf = g.FechaHora[i].In(ZonaPeru).AppendFormat(buf, time.RFC3339)
		for _, v := range d.Punto(i) {
			buf = append(buf, ',')
			buf = strconv.AppendFloat(buf, v, 'g', -1, 64)
		}
		buf = append(buf, '\n')
		if _, err := bw.Write(buf); err != nil {
			return err
		}
	}
	return bw.Flush()
}

// appendCampo entrecomilla el nombre de una estación solo si lo necesita,
// como el cargador (RFC 4180: comillas dobladas).
func appendCampo(buf []byte, s string) []byte {
	if !strings.ContainsAny(s, ",\"\r\n") {
		return append(buf, s...)
	}
	buf = append(buf, '"')
	buf = append(buf, strings.ReplaceAll(s, `"`, `""`)...)
	return append(buf, '"')
}
