package almacen

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

// columnasFicha son las columnas de reports/estaciones_ficha.csv que van al
// documento de la estación; el resto de la ficha (cobertura, periodos, notas)
// se ignora.
var columnasFicha = []string{"estacion", "objeto_fiscalizado", "tipo_actividad", "departamento", "provincia", "distrito", "lat", "lon"}

// LeerFichaEstaciones lee reports/estaciones_ficha.csv y devuelve las
// estaciones en el orden del archivo, con las mismas reglas que
// docs/datos/seed.js usa para sembrar MongoDB: un texto vacío, «-» o «NA» es
// null; la estación no puede faltar ni repetirse. A diferencia del seed, lat y
// lon son obligatorias, porque el contrato (docs/api/openapi.yaml, Estacion) no
// las deja ser null.
func LeerFichaEstaciones(r io.Reader) ([]Estacion, error) {
	cr := csv.NewReader(r)
	cab, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("ficha de estaciones: cabecera: %w", err)
	}
	indice := map[string]int{}
	for i, c := range cab {
		indice[strings.TrimPrefix(c, "\ufeff")] = i
	}
	for _, c := range columnasFicha {
		if _, ok := indice[c]; !ok {
			return nil, fmt.Errorf("ficha de estaciones: falta la columna %s", c)
		}
	}
	var es []Estacion
	vistas := map[string]bool{}
	for fila := 2; ; fila++ {
		campos, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("ficha de estaciones: %w", err)
		}
		campo := func(c string) *string { return textoONulo(campos[indice[c]]) }
		id := campo("estacion")
		if id == nil {
			return nil, fmt.Errorf("ficha de estaciones, fila %d: estación vacía", fila)
		}
		if vistas[*id] {
			return nil, fmt.Errorf("ficha de estaciones: la estación %s está repetida", *id)
		}
		vistas[*id] = true
		lat, err := coordenada(campo("lat"), 90)
		if err != nil {
			return nil, fmt.Errorf("ficha de estaciones, %s: lat %w", *id, err)
		}
		lon, err := coordenada(campo("lon"), 180)
		if err != nil {
			return nil, fmt.Errorf("ficha de estaciones, %s: lon %w", *id, err)
		}
		es = append(es, Estacion{
			ID:                *id,
			ObjetoFiscalizado: campo("objeto_fiscalizado"),
			TipoActividad:     campo("tipo_actividad"),
			Departamento:      campo("departamento"),
			Provincia:         campo("provincia"),
			Distrito:          campo("distrito"),
			Lat:               lat,
			Lon:               lon,
		})
	}
	if len(es) == 0 {
		return nil, fmt.Errorf("ficha de estaciones: no hay estaciones")
	}
	return es, nil
}

func textoONulo(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" || strings.EqualFold(s, "NA") {
		return nil
	}
	return &s
}

func coordenada(s *string, limite float64) (float64, error) {
	if s == nil {
		return 0, fmt.Errorf("ausente")
	}
	v, err := strconv.ParseFloat(*s, 64)
	if err != nil {
		return 0, fmt.Errorf("no numérica: %q", *s)
	}
	if math.IsNaN(v) || v < -limite || v > limite {
		return 0, fmt.Errorf("fuera de [-%g, %g]: %g", limite, limite, v)
	}
	return v, nil
}
