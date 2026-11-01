package kmeans

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"math/rand"
)

// KMeansPP elige k centroides iniciales con muestreo proporcional a D²
// (Arthur y Vassilvitskii, 2007): el primero uniforme y cada siguiente con
// probabilidad proporcional a su distancia al cuadrado al centroide más
// cercano ya elegido.
//
// Devuelve también el k efectivo: si se acaban los puntos distintos —todos los
// restantes coinciden con algún centroide ya elegido, o sea suma de D² igual a
// cero— se reduce k en vez de devolver centroides repetidos, y el llamador debe
// registrarlo.
//
// Corre una sola vez por semilla y su resultado se materializa
// (reports/centroides_iniciales.json, ver ArchivoInicios), porque la misma
// semilla no garantiza los mismos centroides si cambia el generador o el orden
// en que se consume.
func KMeansPP(d *Datos, k int, semilla int64) ([]float64, int, error) {
	if d == nil || d.N == 0 {
		return nil, 0, fmt.Errorf("no hay datos para inicializar")
	}
	if k <= 0 {
		return nil, 0, fmt.Errorf("k debe ser positivo, es %d", k)
	}
	if k > d.N {
		return nil, 0, fmt.Errorf("k=%d es mayor que la cantidad de filas (%d)", k, d.N)
	}

	rng := rand.New(rand.NewSource(semilla))
	dim := d.D
	cent := make([]float64, 0, k*dim)

	elegido := rng.Intn(d.N)
	cent = append(cent, d.Punto(elegido)...)

	// dist2[i] es la distancia al cuadrado del punto i al centroide más cercano
	// ya elegido. Se actualiza de a uno, sin recalcular todo en cada paso.
	dist2 := make([]float64, d.N)
	for i := range dist2 {
		dist2[i] = math.Inf(1)
	}
	kEfectivo := 1
	actualizarDist2(dist2, d, cent[:dim])

	for kEfectivo < k {
		suma := 0.0
		for _, v := range dist2 {
			suma += v
		}
		if suma <= 0 {
			// No quedan puntos distintos de los ya elegidos.
			break
		}
		objetivo := rng.Float64() * suma
		acum, idx := 0.0, d.N-1
		for i, v := range dist2 {
			acum += v
			if acum >= objetivo {
				idx = i
				break
			}
		}
		inicio := len(cent)
		cent = append(cent, d.Punto(idx)...)
		actualizarDist2(dist2, d, cent[inicio:])
		kEfectivo++
	}
	return cent, kEfectivo, nil
}

func actualizarDist2(dist2 []float64, d *Datos, c []float64) {
	for i := 0; i < d.N; i++ {
		x := d.Punto(i)
		dist := 0.0
		for f := 0; f < d.D; f++ {
			delta := x[f] - c[f]
			dist += float64(delta * delta) // sin FMA: ver noFusionar en secuencial.go
		}
		if dist < dist2[i] {
			dist2[i] = dist
		}
	}
}

// HashCentroides es el sha256 de los bits de los centroides. Es la evidencia de
// que la corrida secuencial y la concurrente partieron exactamente de lo mismo.
func HashCentroides(cent []float64) string {
	h := sha256.New()
	var buf [8]byte
	for _, v := range cent {
		binary.LittleEndian.PutUint64(buf[:], math.Float64bits(v))
		h.Write(buf[:])
	}
	return hex.EncodeToString(h.Sum(nil))
}
