package kmeans

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
)

// ArchivoInicios es reports/centroides_iniciales.json: los centroides
// iniciales de cada semilla, materializados. k-means++ corre una sola vez por
// semilla y todo lo demás (otra corrida de cmd/entrenar, la versión distribuida
// de la PC4) parte de este archivo, porque la misma semilla no garantiza los
// mismos centroides si cambia el generador o el orden en que se consume.
//
// Los centroides viven en el espacio estandarizado del gold, así que el archivo
// dice de qué gold (sha256) y de qué receta (features, log1p, nivel, k) son:
// con otro gold o con otra receta no sirven y Para lo rechaza.
type ArchivoInicios struct {
	Nota       string           `json:"nota"`
	GoldSha256 string           `json:"gold_sha256"`
	Features   []string         `json:"features"`
	Log1p      []string         `json:"log1p"`
	Nivel      string           `json:"nivel"`
	K          int              `json:"k"`
	D          int              `json:"d"`
	Inicios    []InicioGuardado `json:"inicios"`
}

// InicioGuardado es un Inicio en el archivo: los centroides por filas, para
// que se lean sin hacer cuentas, y su sha256 (HashCentroides) para detectar una
// edición a mano.
type InicioGuardado struct {
	Semilla    int64       `json:"semilla"`
	KEfectivo  int         `json:"k_efectivo"`
	Sha256     string      `json:"sha256"`
	Centroides [][]float64 `json:"centroides"`
}

const notaInicios = "Centroides iniciales de k-means++, uno por semilla, en el espacio estandarizado del gold. " +
	"cmd/entrenar los escribe la primera vez y después parte de aquí; si cambia el gold o la receta, " +
	"bórralo para que se regenere. encoding/json escribe cada float64 con la representación más corta " +
	"que se relee igual: la ida y vuelta es exacta."

// NuevoArchivoInicios arma el archivo para los inicios de una corrida.
func NuevoArchivoInicios(goldSha256 string, r Receta, dim int, inicios []Inicio) ArchivoInicios {
	a := ArchivoInicios{Nota: notaInicios, GoldSha256: goldSha256, Features: slices.Clone(r.Features),
		Log1p: slices.Clone(r.Log1p), Nivel: r.Nivel, K: r.K, D: dim}
	for _, in := range inicios {
		g := InicioGuardado{Semilla: in.Semilla, KEfectivo: in.KEfectivo, Sha256: HashCentroides(in.Centroides)}
		for j := 0; j < in.KEfectivo; j++ {
			g.Centroides = append(g.Centroides, slices.Clone(in.Centroides[j*dim:(j+1)*dim]))
		}
		a.Inicios = append(a.Inicios, g)
	}
	return a
}

// GuardarInicios escribe el archivo, creando el directorio si hace falta.
func GuardarInicios(ruta string, a ArchivoInicios) error {
	if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return fmt.Errorf("%s: %w", ruta, err)
	}
	return os.WriteFile(ruta, append(b, '\n'), 0o644)
}

// LeerInicios lee el archivo y comprueba su forma: cada semilla con k_efectivo
// filas de D valores finitos, y centroides que coinciden con su sha256.
func LeerInicios(ruta string) (ArchivoInicios, error) {
	var a ArchivoInicios
	b, err := os.ReadFile(ruta)
	if err != nil {
		return a, err
	}
	if err := json.Unmarshal(b, &a); err != nil {
		return a, fmt.Errorf("%s: %w", ruta, err)
	}
	if a.K <= 0 || a.D <= 0 || len(a.Inicios) == 0 {
		return a, fmt.Errorf("%s: archivo de centroides iniciales vacío o sin k y d", ruta)
	}
	for _, g := range a.Inicios {
		if g.KEfectivo <= 0 || g.KEfectivo > a.K || len(g.Centroides) != g.KEfectivo {
			return a, fmt.Errorf("%s: la semilla %d trae %d centroides y k_efectivo = %d", ruta, g.Semilla, len(g.Centroides), g.KEfectivo)
		}
		for _, c := range g.Centroides {
			if len(c) != a.D {
				return a, fmt.Errorf("%s: la semilla %d trae un centroide de %d valores y d = %d", ruta, g.Semilla, len(c), a.D)
			}
			if i, mal := noFinito(c); mal {
				return a, fmt.Errorf("%s: la semilla %d trae un valor no finito (%v)", ruta, g.Semilla, c[i])
			}
		}
		if h := HashCentroides(slices.Concat(g.Centroides...)); h != g.Sha256 {
			return a, fmt.Errorf("%s: los centroides de la semilla %d no son los de su sha256 (dice %.12s…, son %.12s…)",
				ruta, g.Semilla, g.Sha256, h)
		}
	}
	return a, nil
}

// Para devuelve los inicios de las semillas semilla … semilla+n-1, en ese
// orden, después de comprobar que el archivo es de este gold y de esta receta.
// Si no lo es, o le faltan semillas, el error dice que se borre: el archivo es
// un caché de la corrida, y regenerarlo es correr k-means++ otra vez.
func (a ArchivoInicios) Para(goldSha256 string, r Receta, dim int, semilla int64, n int) ([]Inicio, error) {
	const salida = "; bórralo para que cmd/entrenar lo regenere, o corre con -centroides \"\""
	mal := func(formato string, args ...any) error {
		return fmt.Errorf("los centroides iniciales guardados son de otra corrida: "+formato+salida, args...)
	}
	if a.GoldSha256 != goldSha256 {
		return nil, mal("gold %.16s…, el actual es %.16s…", a.GoldSha256, goldSha256)
	}
	if !slices.Equal(a.Features, r.Features) || a.D != dim {
		return nil, mal("features %v, la receta pide %v", a.Features, r.Features)
	}
	l1, l2 := slices.Clone(a.Log1p), slices.Clone(r.Log1p)
	slices.Sort(l1)
	slices.Sort(l2)
	if !slices.Equal(l1, l2) || a.Nivel != r.Nivel {
		return nil, mal("log1p %v y nivel %q, la receta pide %v y %q", a.Log1p, a.Nivel, r.Log1p, r.Nivel)
	}
	if a.K != r.K {
		return nil, mal("k = %d, la receta pide %d", a.K, r.K)
	}
	porSemilla := map[int64]InicioGuardado{}
	for _, g := range a.Inicios {
		porSemilla[g.Semilla] = g
	}
	out := make([]Inicio, 0, n)
	for i := 0; i < n; i++ {
		s := semilla + int64(i)
		g, ok := porSemilla[s]
		if !ok {
			return nil, fmt.Errorf("los centroides iniciales guardados no tienen la semilla %d (se pidieron %d a %d)"+salida,
				s, semilla, semilla+int64(n)-1)
		}
		out = append(out, Inicio{Semilla: s, KEfectivo: g.KEfectivo, Centroides: slices.Concat(g.Centroides...)})
	}
	return out, nil
}

// noFinito devuelve la posición del primer NaN o ±Inf.
func noFinito(xs []float64) (int, bool) {
	for i, x := range xs {
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return i, true
		}
	}
	return 0, false
}
