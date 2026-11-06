package datos

import (
	"math"
	"strconv"
	"strings"
)

// Elipsoide WGS84 y constantes de la proyección UTM.
const (
	wgs84A       = 6378137.0         // semieje mayor, m
	wgs84F       = 1 / 298.257223563 // achatamiento
	utmK0        = 0.9996            // factor de escala en el meridiano central
	utmFalsoE    = 500000.0          // falso este, m
	utmFalsoNSur = 10000000.0        // falso norte del hemisferio sur, m
)

// utmALatLon es la inversa de la proyección UTM sobre WGS84 con la serie
// estándar de Snyder (USGS, 1987, pp. 63-64): latitud del pie de la
// perpendicular y desarrollo en D. El error es menor que un metro dentro de la
// zona; es la misma fórmula que notebooks/aire_lab.py (utm_a_latlon). Devuelve
// grados decimales; sur indica el hemisferio (falso norte de 10 000 km).
func utmALatLon(este, norte float64, zona int, sur bool) (lat, lon float64) {
	e2 := wgs84F * (2 - wgs84F)
	ep2 := e2 / (1 - e2)
	x := este - utmFalsoE
	y := norte
	if sur {
		y -= utmFalsoNSur
	}
	lon0 := float64((zona-1)*6-180+3) * math.Pi / 180
	mu := y / utmK0 / (wgs84A * (1 - e2/4 - 3*e2*e2/64 - 5*e2*e2*e2/256))
	e1 := (1 - math.Sqrt(1-e2)) / (1 + math.Sqrt(1-e2))
	phi1 := mu + (3*e1/2-27*math.Pow(e1, 3)/32)*math.Sin(2*mu) +
		(21*e1*e1/16-55*math.Pow(e1, 4)/32)*math.Sin(4*mu) +
		(151*math.Pow(e1, 3)/96)*math.Sin(6*mu) + (1097*math.Pow(e1, 4)/512)*math.Sin(8*mu)
	sen, cos, tan := math.Sin(phi1), math.Cos(phi1), math.Tan(phi1)
	c1 := ep2 * cos * cos
	t1 := tan * tan
	n1 := wgs84A / math.Sqrt(1-e2*sen*sen)
	r1 := wgs84A * (1 - e2) / math.Pow(1-e2*sen*sen, 1.5)
	d := x / (n1 * utmK0)
	lat = phi1 - (n1*tan/r1)*(d*d/2-(5+3*t1+10*c1-4*c1*c1-9*ep2)*math.Pow(d, 4)/24+
		(61+90*t1+298*c1+45*t1*t1-252*ep2-3*c1*c1)*math.Pow(d, 6)/720)
	lon = lon0 + (d-(1+2*t1+c1)*math.Pow(d, 3)/6+
		(5-2*c1+28*t1-3*c1*c1+8*ep2+24*t1*t1)*math.Pow(d, 5)/120)/cos
	return lat * 180 / math.Pi, lon * 180 / math.Pi
}

// latLonDeUbicacion calcula lat/lon en WGS84 desde las columnas este, norte y
// zona de una ubicación (ya corregida), en el hemisferio sur: todas las
// estaciones de OEFA están en el Perú. ok es falso si falta o no se lee alguna
// columna, si la zona no es de 1 a 60, o si el datum no es WGS84 (otro datum,
// como PSAD56, corre el punto unos cientos de metros: no se convierte).
func latLonDeUbicacion(campos map[string]string) (lat, lon float64, ok bool) {
	if d := campos["datum"]; d != "" && strings.ReplaceAll(normalizar(d), " ", "") != "wgs84" {
		return 0, 0, false
	}
	este, err1 := strconv.ParseFloat(campos["este"], 64)
	norte, err2 := strconv.ParseFloat(campos["norte"], 64)
	zona, err3 := strconv.Atoi(strings.TrimSpace(campos["zona"]))
	if err1 != nil || err2 != nil || err3 != nil || zona < 1 || zona > 60 {
		return 0, 0, false
	}
	lat, lon = utmALatLon(este, norte, zona, true)
	return lat, lon, true
}
