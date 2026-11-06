package datos

import (
	"bufio"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// EscribirSilver escribe data/silver/oefa_ancho.csv: cabecera
// estacion,fecha_hora seguida de los parámetros presentes en orden alfabético
// y, al final, hora_inferida (1 si alguna celda de la fila tiene la hora
// reconstruida por posición, 0 si no); coma como delimitador, vacío = faltante,
// fecha_hora en RFC3339 con su desfase (-05:00) y valores con
// FormatFloat(x, 'g', -1, 64), que se leen de vuelta al mismo float64.
func (t *TablaAncha) EscribirSilver(w io.Writer) error {
	return escribirFilas(w, t.Parametros, t.N, len(t.Parametros), t.Valores, t.Estaciones, t.Estacion, t.Hora, t.Zona, t.HoraInferida)
}

// EscribirGold escribe data/gold/oefa_features.csv con el mismo formato que el
// silver y las features de la receta ya transformadas y estandarizadas.
func (d *Dataset) EscribirGold(w io.Writer) error {
	return escribirFilas(w, d.Features, d.N, d.D, d.X, d.Estaciones, d.Estacion, d.Hora, d.Zona, nil)
}

// escribirFilas escribe el formato común del silver y el gold. inferida, si no
// es nil, agrega la columna hora_inferida al final.
func escribirFilas(w io.Writer, columnas []string, n, d int, x []float64, estaciones []string,
	estacion []uint16, hora []int64, zona *time.Location, inferida []bool) error {
	if zona == nil {
		zona = time.UTC
	}
	bw := bufio.NewWriterSize(w, 1<<20)
	bw.WriteString("estacion,fecha_hora")
	for _, c := range columnas {
		bw.WriteByte(',')
		bw.WriteString(c)
	}
	if inferida != nil {
		bw.WriteString(",hora_inferida")
	}
	bw.WriteByte('\n')
	var buf []byte
	for i := 0; i < n; i++ {
		buf = appendCampo(buf[:0], estaciones[estacion[i]])
		buf = append(buf, ',')
		buf = time.Unix(hora[i], 0).In(zona).AppendFormat(buf, time.RFC3339)
		for _, v := range x[i*d : (i+1)*d] {
			buf = append(buf, ',')
			if !math.IsNaN(v) {
				buf = strconv.AppendFloat(buf, v, 'g', -1, 64)
			}
		}
		if inferida != nil {
			if inferida[i] {
				buf = append(buf, ",1"...)
			} else {
				buf = append(buf, ",0"...)
			}
		}
		buf = append(buf, '\n')
		if _, err := bw.Write(buf); err != nil {
			return err
		}
	}
	return bw.Flush()
}

// appendCampo entrecomilla el nombre de la estación solo si lo necesita (RFC 4180).
func appendCampo(buf []byte, s string) []byte {
	if !strings.ContainsAny(s, ",\"\r\n") {
		return append(buf, s...)
	}
	buf = append(buf, '"')
	buf = append(buf, strings.ReplaceAll(s, `"`, `""`)...)
	return append(buf, '"')
}

// EscribirArchivo crea ruta (y su directorio) y escribe en ella.
func EscribirArchivo(ruta string, escribir func(io.Writer) error) error {
	if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
		return err
	}
	f, err := os.Create(ruta)
	if err != nil {
		return err
	}
	if err := escribir(f); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
