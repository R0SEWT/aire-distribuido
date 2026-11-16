package cluster

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"
)

// DiasPorBloque es el tamaño de los bloques en que se reparten los días del
// entrenamiento del mapa (docs/specs/2026-11-14-mapa-nacional-design.md, §5.2).
const DiasPorBloque = 7

// RepartirDias reparte los días entre los nodos para el local SGD: los ordena,
// los agrupa en bloques de DiasPorBloque días seguidos (el último puede ser más
// corto), baraja los bloques con la semilla y los reparte en ronda, el bloque j
// al nodo j mod nodos. Así cada nodo ve todas las estaciones del año; con
// bloques contiguos por nodo, cada uno vería otra estación y el promedio se
// desviaría. Devuelve los días de cada nodo en orden de calendario.
//
// "Seguidos" es en el orden de la lista: si al manifiesto le falta un día, el
// bloque que lo cruza salta ese hueco.
//
// Es determinista: los mismos días (en cualquier orden), los mismos nodos y la
// misma semilla dan el mismo reparto. El barajado es Shuffle de math/rand/v2
// sobre PCG(semilla, 0).
func RepartirDias(dias []string, nodos int, semilla uint64) ([][]string, error) {
	if nodos <= 0 {
		return nil, fmt.Errorf("no se pueden repartir días entre %d nodos", nodos)
	}
	ds, err := diasValidos(dias)
	if err != nil {
		return nil, err
	}
	var bloques [][]string
	for len(ds) > 0 {
		n := min(DiasPorBloque, len(ds))
		bloques = append(bloques, ds[:n])
		ds = ds[n:]
	}
	if len(bloques) < nodos {
		return nil, fmt.Errorf("%d días son %d bloques de %d y hay %d nodos: algún nodo se quedaría sin días",
			len(dias), len(bloques), DiasPorBloque, nodos)
	}
	rng := rand.New(rand.NewPCG(semilla, 0))
	rng.Shuffle(len(bloques), func(i, j int) { bloques[i], bloques[j] = bloques[j], bloques[i] })

	reparto := make([][]string, nodos)
	for j, b := range bloques {
		reparto[j%nodos] = append(reparto[j%nodos], b...)
	}
	for _, r := range reparto {
		slices.Sort(r)
	}
	return reparto, nil
}

// diasValidos devuelve una copia ordenada de los días, o un error si alguno no
// es AAAA-MM-DD o está repetido.
func diasValidos(dias []string) ([]string, error) {
	if len(dias) == 0 {
		return nil, fmt.Errorf("no hay días")
	}
	for _, d := range dias {
		if _, err := time.Parse(time.DateOnly, d); err != nil || len(d) != len(time.DateOnly) {
			return nil, fmt.Errorf("el día %q no es AAAA-MM-DD", d)
		}
	}
	ds := slices.Clone(dias)
	slices.Sort(ds)
	for i := 1; i < len(ds); i++ {
		if ds[i] == ds[i-1] {
			return nil, fmt.Errorf("el día %s está repetido", ds[i])
		}
	}
	return ds, nil
}

// Manifiesto es lo que el coordinador del mapa necesita de manifiesto.json
// (spec del mapa, §3.4): los días que hay y la huella con que los nodos
// verifican que leen el mismo archivo. El resto (canales, grilla, estáticos)
// es para los nodos.
type Manifiesto struct {
	Fuente string   // URL o ruta de donde se leyó
	Huella string   // "sha256:" y el sha256 de sus bytes, tal como se leyeron
	Dias   []string // AAAA-MM-DD, en orden
}

// maxManifiesto acota lo que se lee del manifiesto: con 1 826 días y el sha256
// de cada archivo ocupa unos 300 KB.
const maxManifiesto = 64 << 20

// LeerManifiesto lee manifiesto.json de una URL http(s):// o de una ruta local.
// Los días son los de los archivos dias/AAAA-MM-DD.npy que lista con su sha256:
// las claves del objeto "archivos" (ruta → {"sha256", "bytes"}) o, si no está,
// del objeto "sha256" (ruta → sha256), como los lee lab/mapa/modelo/datos.py.
// No hay un campo aparte con la lista de días: el día que no tiene su sha256
// no se puede verificar, así que no se reparte. La huella es la del archivo
// entero: es la que va en la TAREA, y el nodo la compara con el sha256 del
// manifiesto que baja.
func LeerManifiesto(fuente string) (*Manifiesto, error) {
	b, err := leerFuente(fuente)
	if err != nil {
		return nil, fmt.Errorf("manifiesto %s: %w", fuente, err)
	}
	var campos struct {
		Archivos map[string]json.RawMessage `json:"archivos"`
		Sha256   map[string]json.RawMessage `json:"sha256"`
	}
	if err := json.Unmarshal(b, &campos); err != nil {
		return nil, fmt.Errorf("manifiesto %s: %w", fuente, err)
	}
	archivos := campos.Archivos
	if len(archivos) == 0 {
		archivos = campos.Sha256
	}
	if len(archivos) == 0 {
		return nil, fmt.Errorf("manifiesto %s: no lista sus archivos (falta el objeto \"archivos\" o \"sha256\")", fuente)
	}
	var dias []string
	for ruta := range archivos {
		if dia, ok := strings.CutPrefix(ruta, "dias/"); ok {
			dia, ok = strings.CutSuffix(dia, ".npy")
			if ok {
				dias = append(dias, dia)
			}
		}
	}
	if len(dias) == 0 {
		return nil, fmt.Errorf("manifiesto %s: no lista ningún dias/AAAA-MM-DD.npy", fuente)
	}
	if dias, err = diasValidos(dias); err != nil {
		return nil, fmt.Errorf("manifiesto %s: %w", fuente, err)
	}
	suma := sha256.Sum256(b)
	return &Manifiesto{Fuente: fuente, Huella: HuellaDeSha256(hex.EncodeToString(suma[:])), Dias: dias}, nil
}

func leerFuente(fuente string) ([]byte, error) {
	if !strings.HasPrefix(fuente, "http://") && !strings.HasPrefix(fuente, "https://") {
		f, err := os.Open(fuente)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		return leerAcotado(f)
	}
	cliente := &http.Client{Timeout: time.Minute}
	resp, err := cliente.Get(fuente)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("el servidor respondió %s", resp.Status)
	}
	return leerAcotado(resp.Body)
}

func leerAcotado(r io.Reader) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, maxManifiesto+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxManifiesto {
		return nil, fmt.Errorf("ocupa más de %d MB", maxManifiesto>>20)
	}
	return b, nil
}
