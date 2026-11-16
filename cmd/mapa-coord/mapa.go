package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/R0SEWT/aire-distribuido/cluster"
	"github.com/R0SEWT/aire-distribuido/kmeans"
	"github.com/R0SEWT/aire-distribuido/protocolo"
)

var errPlazoEspera = errors.New("venció el plazo de espera")

// preparacion es lo que se lee antes de esperar a los nodos: un error de
// archivos se ve en el acto, no después de juntar veinte equipos.
type preparacion struct {
	man        *cluster.Manifiesto
	pesos0     []float32
	sha0       string
	hiper      json.RawMessage // compacto: viaja en la línea de la TAREA
	validacion []string
	tarea      string
}

func preparar(op opciones) (*preparacion, error) {
	switch {
	case op.rondas <= 0 || op.pasos <= 0:
		return nil, fmt.Errorf("-rondas (%d) y -pasos (%d) tienen que ser positivos", op.rondas, op.pasos)
	case op.plazo <= 0 || op.plazoHola <= 0 || op.plazoTarea <= 0 || op.plazoRonda <= 0:
		return nil, errors.New("los plazos tienen que ser positivos")
	}
	p := &preparacion{tarea: op.tarea}
	if p.tarea == "" {
		p.tarea = "mapa-" + time.Now().Format("20060102-150405")
	}
	var err error
	if p.pesos0, p.sha0, err = leerPesos(op.pesosIniciales); err != nil {
		return nil, err
	}
	if op.hiper != "" {
		b, err := os.ReadFile(op.hiper)
		if err != nil {
			return nil, fmt.Errorf("hiper: %w", err)
		}
		var compacto bytes.Buffer
		if err := json.Compact(&compacto, b); err != nil {
			return nil, fmt.Errorf("hiper %s no es JSON válido: %w", op.hiper, err)
		}
		p.hiper = compacto.Bytes()
	}
	if op.validacion != "" {
		for e := range strings.SplitSeq(op.validacion, ",") {
			e = strings.TrimSpace(e)
			if e == "" {
				return nil, fmt.Errorf("-validacion %q tiene una estación vacía", op.validacion)
			}
			p.validacion = append(p.validacion, e)
		}
	}
	if p.man, err = cluster.LeerManifiesto(op.manifiesto); err != nil {
		return nil, err
	}
	// Se reparte de verdad cuando se sabe cuántos nodos llegaron; esto solo
	// comprueba que alcanzan los días para los que se pidieron.
	if _, err := cluster.RepartirDias(p.man.Dias, op.nodos, op.semilla); err != nil {
		return nil, fmt.Errorf("manifiesto %s: %w", op.manifiesto, err)
	}
	return p, nil
}

// leerPesos lee un vector de float32 little-endian crudo y su sha256.
func leerPesos(ruta string) ([]float32, string, error) {
	b, err := os.ReadFile(ruta)
	if err != nil {
		return nil, "", fmt.Errorf("pesos iniciales: %w", err)
	}
	switch {
	case len(b) == 0:
		return nil, "", fmt.Errorf("pesos iniciales %s: el archivo está vacío", ruta)
	case len(b)%4 != 0:
		return nil, "", fmt.Errorf("pesos iniciales %s: %d bytes no es múltiplo de 4 (float32)", ruta, len(b))
	case len(b) > protocolo.MaxCarga:
		return nil, "", fmt.Errorf("pesos iniciales %s: %d bytes, más que la carga máxima del protocolo (%d)", ruta, len(b), protocolo.MaxCarga)
	}
	v := make([]float32, len(b)/4)
	if _, err := binary.Decode(b, binary.LittleEndian, v); err != nil {
		return nil, "", fmt.Errorf("pesos iniciales %s: %w", ruta, err)
	}
	for i, x := range v {
		if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
			return nil, "", fmt.Errorf("pesos iniciales %s: %v en la posición %d", ruta, x, i)
		}
	}
	suma := sha256.Sum256(b)
	return v, hex.EncodeToString(suma[:]), nil
}

// correr es el comando entero salvo los flags y el listener: prepara, espera
// los nodos, entrena, escribe y resume. trasEspera, si no es nil, se llama al
// terminar la espera (main restablece ahí el Ctrl+C normal).
func correr(ctx context.Context, op opciones, ln net.Listener, lg *log.Logger, w io.Writer, trasEspera func()) error {
	p, err := preparar(op)
	if err != nil {
		return err
	}
	lg.Printf("tarea %s: %d parámetros, manifiesto %s con %d días (huella %.19s…), %d rondas de %d pasos",
		p.tarea, len(p.pesos0), p.man.Fuente, len(p.man.Dias), p.man.Huella, op.rondas, op.pasos)

	inicio := time.Now()
	miembros, err := reunir(ctx, ln, op, lg)
	espera := time.Since(inicio)
	if trasEspera != nil {
		trasEspera()
	}
	if err != nil {
		return err
	}
	s, err := entrenar(miembros, p, op, lg, espera)
	if s != nil {
		imprimir(w, s, op)
	}
	return err
}

// reunir espera los nodos v2 y cierra el listener, como en cmd/distribuido: si
// vence el plazo sin los N no hay entrenamiento; con Ctrl+C sí, con los que
// llegaron.
func reunir(ctx context.Context, ln net.Listener, op opciones, lg *log.Logger) ([]*cluster.Miembro, error) {
	espera, cancel := context.WithTimeoutCause(ctx, op.plazo, errPlazoEspera)
	defer cancel()
	lg.Printf("espero %d nodos (protocolo v2) durante %v (Ctrl+C empieza con los que hayan llegado)", op.nodos, op.plazo)
	miembros := cluster.EsperarNodosVersion(espera, ln, op.nodos, protocolo.Version2, op.plazoHola, lg)
	ln.Close()
	if len(miembros) == op.nodos {
		return miembros, nil
	}
	if len(miembros) == 0 || errors.Is(context.Cause(espera), errPlazoEspera) {
		err := fmt.Errorf("llegaron %d de %d nodos en %v: no hay entrenamiento (Ctrl+C mientras espera empieza con los que haya)",
			len(miembros), op.nodos, op.plazo)
		cluster.Abortar(cluster.Enlaces(miembros), err.Error(), 2*time.Second)
		return nil, err
	}
	lg.Printf("Ctrl+C: entreno con los %d de %d nodos que llegaron", len(miembros), op.nodos)
	return miembros, nil
}

// entrenar reparte los días, manda las TAREA y corre el local SGD, escribiendo
// el punto de control y la medición tras cada ronda. Cualquier falla aborta: los
// nodos reciben un ERROR con el motivo y -salida queda "abortada", con las
// rondas que sí se completaron. Devuelve la salida también con error, para el
// resumen.
func entrenar(miembros []*cluster.Miembro, p *preparacion, op opciones, lg *log.Logger, espera time.Duration) (*salida, error) {
	enlaces := cluster.Enlaces(miembros)
	inicio := time.Now()
	validacion := p.validacion
	if validacion == nil {
		validacion = []string{} // en el JSON, [] y no null
	}
	s := &salida{
		Metadatos: kmeans.RecolectarMetadatos(), Estado: "en curso", Tarea: p.tarea,
		Manifiesto: infoManifiesto{Fuente: p.man.Fuente, Huella: p.man.Huella, Dias: len(p.man.Dias)},
		Validacion: validacion, Hiper: p.hiper, Semilla: op.semilla, Parametros: len(p.pesos0),
		PesosIniciales: infoPesos{Archivo: op.pesosIniciales, Sha256: p.sha0},
		PesosFinales:   infoPesos{Archivo: op.pesosFinales},
		RondasPedidas:  op.rondas, Pasos: op.pasos, PlazoRondaMs: msDe(op.plazoRonda),
		NodosEsperados: op.nodos, Reparto: reparto, Rondas: []infoRonda{}, NotaTiempos: notaTiempos,
	}
	s.Ms.EsperaNodos = msDe(espera)
	abortar := func(err error) (*salida, error) {
		cluster.Abortar(enlaces, err.Error(), 2*time.Second)
		s.Estado, s.Error = "abortada", err.Error()
		s.Ms.Total = msDe(time.Since(inicio))
		actualizarNodos(s, miembros)
		if e := guardarJSON(op.salida, s); e != nil {
			lg.Printf("aviso: no pude escribir %s: %v", op.salida, e)
		}
		return s, err
	}

	dias, err := cluster.RepartirDias(p.man.Dias, len(enlaces), op.semilla)
	if err != nil {
		return abortar(err)
	}
	for i, m := range miembros {
		s.Nodos = append(s.Nodos, infoNodo{Nodo: m.Enlace.Nodo, Direccion: m.Direccion, GPU: m.Enlace.GPU,
			VramGB: m.Enlace.VramGB, Workers: m.Enlace.Workers, Dias: dias[i]})
	}
	tarea := protocolo.Mensaje{Tipo: protocolo.Tarea, Tarea: p.tarea, Manifiesto: op.manifiesto, Huella: p.man.Huella,
		Validacion: p.validacion, Hiper: p.hiper}
	tiempos, err := cluster.CargarTarea(enlaces, tarea, dias, time.Now().Add(op.plazoTarea))
	if err != nil {
		return abortar(err)
	}
	s.Ms.Tarea = msDe(time.Since(inicio))
	for i := range s.Nodos {
		s.Nodos[i].Muestras, s.Nodos[i].MsTarea = enlaces[i].Muestras, msDe(tiempos[i])
	}
	lg.Printf("los %d nodos tienen sus días (%.1f s); empieza el local SGD", len(enlaces), s.Ms.Tarea/1000)
	if err := guardarJSON(op.salida, s); err != nil {
		return abortar(err)
	}

	inicioSGD := time.Now()
	sgd := cluster.OpcionesSGD{Rondas: op.rondas, Pasos: op.pasos, Plazo: op.plazoRonda}
	_, err = cluster.SGDLocal(enlaces, p.pesos0, sgd, func(r *cluster.RondaSGD) error {
		sha, err := guardarPesos(op.pesosFinales, r.Pesos)
		if err != nil {
			return err
		}
		s.PesosFinales.Ronda, s.PesosFinales.Sha256 = r.Ronda, sha
		s.Rondas = append(s.Rondas, armarRonda(r, miembros))
		s.Ms.Entrenamiento = msDe(time.Since(inicioSGD))
		s.Ms.Total = msDe(time.Since(inicio))
		actualizarNodos(s, miembros)
		lg.Printf("ronda %d/%d: pérdida %.6g · validación %.6g · %d muestras · barrera %.1f s · promedio %.0f ms · %.1f MB",
			r.Ronda, op.rondas, r.Perdida, r.PerdidaVal, r.Muestras, r.Barrera.Seconds(), msDe(r.Promedio), float64(r.Bytes)/1e6)
		return guardarJSON(op.salida, s)
	})
	if err != nil {
		return abortar(err)
	}
	if err := cluster.Terminar(enlaces, 2*time.Second); err != nil {
		lg.Printf("aviso: los pesos están completos, pero no todos los nodos recibieron el FIN: %v", err)
	}
	s.Estado = "completa"
	s.Ms.Entrenamiento = msDe(time.Since(inicioSGD))
	s.Ms.Total = msDe(time.Since(inicio))
	return s, guardarJSON(op.salida, s)
}

// actualizarNodos copia los contadores de los enlaces. Se llama entre rondas,
// cuando ninguna goroutine los está usando.
func actualizarNodos(s *salida, miembros []*cluster.Miembro) {
	for i := range s.Nodos {
		s.Nodos[i].Rezagados = miembros[i].Enlace.Rezagados
	}
}

func armarRonda(r *cluster.RondaSGD, miembros []*cluster.Miembro) infoRonda {
	ir := infoRonda{Ronda: r.Ronda, Perdida: r.Perdida, PerdidaVal: r.PerdidaVal, Muestras: r.Muestras,
		MsBarrera: msDe(r.Barrera), MsPromedio: msDe(r.Promedio), Bytes: r.Bytes}
	for i, l := range r.Locales {
		ir.Nodos = append(ir.Nodos, rondaNodo{Nodo: miembros[i].Enlace.Nodo, Muestras: l.Muestras,
			Perdida: l.Perdida, PerdidaVal: l.PerdidaVal, MsComputo: msDe(l.Computo), MsEnvio: msDe(l.Envio),
			MsTransferencia: max(0, msDe(l.Total-l.Computo)), MsTotal: msDe(l.Total)})
	}
	return ir
}

// salida es -salida (reports/mapa_entrenamiento.json). Se reescribe tras cada
// ronda completa: si el entrenamiento se corta, dice hasta dónde llegó.
type salida struct {
	Metadatos kmeans.Metadatos `json:"metadatos"`
	Estado    string           `json:"estado"` // "en curso", "completa" o "abortada"
	Error     string           `json:"error,omitempty"`

	Tarea          string          `json:"tarea"`
	Manifiesto     infoManifiesto  `json:"manifiesto"`
	Validacion     []string        `json:"validacion"`
	Hiper          json.RawMessage `json:"hiper,omitempty"`
	Semilla        uint64          `json:"semilla"`
	Parametros     int             `json:"parametros"`
	PesosIniciales infoPesos       `json:"pesos_iniciales"`
	PesosFinales   infoPesos       `json:"pesos_finales"`

	RondasPedidas  int        `json:"rondas_pedidas"`
	Pasos          int        `json:"pasos"`
	PlazoRondaMs   float64    `json:"plazo_ronda_ms"`
	NodosEsperados int        `json:"nodos_esperados"`
	Nodos          []infoNodo `json:"nodos"`
	Reparto        string     `json:"reparto"`

	Rondas      []infoRonda `json:"rondas"`
	Ms          tiempos     `json:"ms"`
	NotaTiempos string      `json:"nota_tiempos"`
}

type infoManifiesto struct {
	Fuente string `json:"fuente"`
	Huella string `json:"huella"`
	Dias   int    `json:"dias"`
}

// infoPesos dice qué archivo es y su sha256. En pesos_finales, Ronda es la
// ronda cuyo promedio tiene el archivo (0: todavía ninguna).
type infoPesos struct {
	Archivo string `json:"archivo"`
	Sha256  string `json:"sha256"`
	Ronda   int    `json:"ronda,omitempty"`
}

type infoNodo struct {
	Nodo      string   `json:"nodo"`
	Direccion string   `json:"direccion"`
	GPU       string   `json:"gpu"`
	VramGB    float64  `json:"vram_gb"`
	Workers   int      `json:"workers"`
	Muestras  int      `json:"muestras"` // horas con etiqueta en sus días (LISTO)
	MsTarea   float64  `json:"ms_tarea"`
	Rezagados int      `json:"rezagados"`
	Dias      []string `json:"dias"`
}

type infoRonda struct {
	Ronda      int         `json:"ronda"`
	Perdida    float64     `json:"perdida"`
	PerdidaVal float64     `json:"perdida_val"`
	Muestras   int         `json:"muestras"`
	MsBarrera  float64     `json:"ms_barrera"`
	MsPromedio float64     `json:"ms_promedio"`
	Bytes      int64       `json:"bytes"`
	Nodos      []rondaNodo `json:"nodos"`
}

type rondaNodo struct {
	Nodo            string  `json:"nodo"`
	Muestras        int     `json:"muestras"`
	Perdida         float64 `json:"perdida"`
	PerdidaVal      float64 `json:"perdida_val"`
	MsComputo       float64 `json:"ms_computo"`
	MsEnvio         float64 `json:"ms_envio"`
	MsTransferencia float64 `json:"ms_transferencia"`
	MsTotal         float64 `json:"ms_total"`
}

type tiempos struct {
	EsperaNodos   float64 `json:"espera_nodos"`
	Tarea         float64 `json:"tarea"`
	Entrenamiento float64 `json:"entrenamiento"`
	Total         float64 `json:"total"`
}

const (
	reparto = "bloques de 7 días seguidos del manifiesto, barajados con la semilla (math/rand/v2, PCG(semilla, 0)) " +
		"y repartidos en ronda entre los nodos en orden de nombre; cada nodo recibe sus días en orden de calendario"
	notaTiempos = "Por ronda: ms_barrera va del primer PESOS al último LOCAL; ms_promedio es el promedio ponderado " +
		"en orden de nodo; bytes es la carga binaria que cruzó la red (un PESOS y un LOCAL por nodo). Por nodo: " +
		"ms_computo es lo que el nodo informa que tardó en sus pasos; ms_envio, escribir el PESOS en la conexión; " +
		"ms_total, desde que empieza a salir el PESOS hasta que llega el LOCAL entero; ms_transferencia = ms_total − " +
		"ms_computo, lo que costó la red (mandar, esperar y recibir). perdida y perdida_val de la ronda son las de " +
		"cada nodo ponderadas por sus muestras. ms.tarea va del reparto al último LISTO (bajar y verificar los días); " +
		"ms.total no incluye ms.espera_nodos."
)

func msDe(d time.Duration) float64 { return float64(d.Nanoseconds()) / 1e6 }

// guardarPesos escribe el vector en float32 little-endian, de forma atómica, y
// devuelve su sha256.
func guardarPesos(ruta string, v []float32) (string, error) {
	b, err := binary.Append(make([]byte, 0, 4*len(v)), binary.LittleEndian, v)
	if err != nil {
		return "", err
	}
	if err := escribirAtomico(ruta, b); err != nil {
		return "", fmt.Errorf("punto de control: %w", err)
	}
	suma := sha256.Sum256(b)
	return hex.EncodeToString(suma[:]), nil
}

func guardarJSON(ruta string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("%s: %w", ruta, err)
	}
	return escribirAtomico(ruta, append(b, '\n'))
}

// escribirAtomico escribe en un temporal del mismo directorio, lo baja a disco
// y lo renombra: si algo falla a mitad (o se corta la luz), el archivo anterior
// queda intacto y nunca hay un punto de control a medias.
func escribirAtomico(ruta string, b []byte) error {
	dir := filepath.Dir(ruta)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, filepath.Base(ruta)+".tmp*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name()) // no hace nada si el rename salió bien
	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), ruta)
}

func imprimir(w io.Writer, s *salida, op opciones) {
	fmt.Fprintf(w, "\ntarea       %s · %s\n", s.Tarea, s.Estado)
	fmt.Fprintf(w, "manifiesto  %s · %d días · huella %.19s…\n", s.Manifiesto.Fuente, s.Manifiesto.Dias, s.Manifiesto.Huella)
	fmt.Fprintf(w, "modelo      %d parámetros (%.1f MB) · %d rondas de %d pasos · validación %s\n",
		s.Parametros, float64(4*s.Parametros)/1e6, s.RondasPedidas, s.Pasos, strings.Join(s.Validacion, ","))
	fmt.Fprintf(w, "nodos       %d de %d · bloques de 7 días barajados con la semilla %d\n", len(s.Nodos), s.NodosEsperados, s.Semilla)
	for _, n := range s.Nodos {
		fmt.Fprintf(w, "            %-14s %-22s %-20s %3d días  %7d muestras  tarea %.1f s\n",
			n.Nodo, n.Direccion, n.GPU, len(n.Dias), n.Muestras, n.MsTarea/1000)
	}
	for _, r := range s.Rondas {
		fmt.Fprintf(w, "ronda %3d   pérdida %.6g · validación %.6g · barrera %.2f s · promedio %.1f ms\n",
			r.Ronda, r.Perdida, r.PerdidaVal, r.MsBarrera/1000, r.MsPromedio)
	}
	if s.Error != "" {
		fmt.Fprintf(w, "error       %s\n", s.Error)
	}
	if s.PesosFinales.Ronda > 0 {
		fmt.Fprintf(w, "pesos       %s (ronda %d, sha256 %.12s…)\n", s.PesosFinales.Archivo, s.PesosFinales.Ronda, s.PesosFinales.Sha256)
	}
	fmt.Fprintf(w, "→ %s\n", op.salida)
}
