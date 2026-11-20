package main

import (
	"context"
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

// toleranciaPorOmision separa los dos efectos que pueden apartar el modelo
// distribuido del de cmd/entrenar. Reasociar la suma de los parciales de K
// nodos cambia los centroides en unos pocos ulp (del orden de 1e-16 en el
// espacio estandarizado, donde son de orden 1). Que una sola fila cambie de
// cluster mueve un centroide del orden de su distancia a la fila dividida por
// el tamaño del cluster: con el cluster más grande del gold real (48 161
// filas) y una fila de la frontera, más de 1e-5. 1e-9 queda a cuatro órdenes
// de magnitud de cada uno: dentro, es redondeo; fuera, es otra asignación.
const toleranciaPorOmision = 1e-9

var (
	errPlazoEspera       = errors.New("venció el plazo de espera")
	errFueraDeTolerancia = errors.New("el modelo distribuido se aparta de la referencia más que la tolerancia")
)

type opciones struct {
	receta, gold, estandarizacion, inicios, referencia, salida string

	escuchar   string
	nodos      int
	replicas   int
	maxFilas   int // tope de filas por nodo, el -max-filas de los nodos; 0 = cluster.MaxFilasPorOmision
	semilla    int64
	conSemilla bool // -semilla se pasó: si no, la de la receta

	plazo, plazoHola, plazoCarga, plazoRonda time.Duration
	tolerancia                               float64
}

// preparacion es lo que se lee antes de esperar a los nodos: un error de
// archivos se ve en el acto, no después de juntar veinte equipos.
type preparacion struct {
	receta kmeans.Receta
	est    *kmeans.Estandarizacion
	gold   infoGold
	huella string
	inicio kmeans.Inicio
	op     kmeans.Opciones
	ref    *referencia
}

type infoGold struct {
	Ruta       string `json:"ruta"`
	Sha256     string `json:"sha256"`
	N          int    `json:"n"`
	D          int    `json:"d"`
	Estaciones int    `json:"estaciones"`
	// Replicas y FilasTabla: la tabla que se reparte tiene Replicas × N filas
	// y la fila i es la i mod N del gold (réplicas virtuales, docs/diseno.md, §4.3).
	Replicas   int `json:"replicas"`
	FilasTabla int `json:"filas_tabla"`
}

// referencia son los campos de reports/kmeans.json (cmd/entrenar) que se
// comparan. Los demás se ignoran.
type referencia struct {
	Gold struct {
		Sha256 string `json:"sha256"`
	} `json:"gold"`
	Semilla     int64             `json:"semilla"`
	Chunk       int               `json:"chunk"`
	Iteraciones int               `json:"iteraciones"`
	Paro        string            `json:"paro"`
	Inercia     float64           `json:"inercia"`
	Inercias    []float64         `json:"inercias_por_iteracion"`
	Tamanos     []int             `json:"tamanos"`
	Centroides  [][]float64       `json:"centroides_estandarizados"`
	Arranques   []kmeans.Arranque `json:"arranques"`
}

func preparar(op opciones) (*preparacion, error) {
	switch {
	case op.nodos < 1:
		return nil, fmt.Errorf("-nodos tiene que ser al menos 1, es %d", op.nodos)
	case op.replicas < 1:
		return nil, fmt.Errorf("-replicas tiene que ser al menos 1, es %d", op.replicas)
	case op.maxFilas < 0:
		return nil, fmt.Errorf("-max-filas no puede ser negativo, es %d", op.maxFilas)
	case op.plazo <= 0 || op.plazoHola <= 0 || op.plazoCarga <= 0 || op.plazoRonda <= 0:
		return nil, errors.New("los plazos tienen que ser positivos")
	case op.tolerancia < 0 || math.IsNaN(op.tolerancia):
		return nil, fmt.Errorf("-tolerancia inválida (%v)", op.tolerancia)
	}
	receta, err := kmeans.LeerReceta(op.receta)
	if err != nil {
		return nil, err
	}
	est, err := kmeans.LeerEstandarizacion(op.estandarizacion)
	if err != nil {
		return nil, err
	}
	if err := est.Coincide(receta); err != nil {
		return nil, fmt.Errorf("%s: %w", op.estandarizacion, err)
	}
	g, err := kmeans.LeerGold(op.gold, receta.Features)
	if err != nil {
		return nil, err
	}
	n := g.Datos.N
	if est.N != n {
		return nil, fmt.Errorf("%s dice n = %d y %s tiene %d filas: no es su estandarización", op.estandarizacion, est.N, op.gold, n)
	}
	// Antes de juntar a nadie: cada nodo rechaza un rango de más filas que su
	// tope, y con un -replicas mal escrito la corrida moriría en el CARGAR.
	if err := comprobarReplicas(op.replicas, n, op.nodos, op.topeFilas()); err != nil {
		return nil, err
	}
	sha, err := kmeans.Sha256Archivo(op.gold)
	if err != nil {
		return nil, err
	}
	semilla := receta.Semilla
	if op.conSemilla {
		semilla = op.semilla
	}
	arch, err := kmeans.LeerInicios(op.inicios)
	if err != nil {
		return nil, err
	}
	inicios, err := arch.Para(sha, receta, g.Datos.D, semilla, 1)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op.inicios, err)
	}
	p := &preparacion{
		receta: receta, est: est, huella: cluster.HuellaDeSha256(sha), inicio: inicios[0], op: receta.OpcionesConc().Opciones,
		gold: infoGold{Ruta: op.gold, Sha256: sha, N: n, D: g.Datos.D, Estaciones: len(g.Estaciones()),
			Replicas: op.replicas, FilasTabla: op.replicas * n},
	}
	p.op.K = p.inicio.KEfectivo
	if op.referencia != "" {
		b, err := os.ReadFile(op.referencia)
		if err != nil {
			return nil, fmt.Errorf("referencia: %w (pasa -referencia \"\" para no comparar)", err)
		}
		p.ref = &referencia{}
		if err := json.Unmarshal(b, p.ref); err != nil {
			return nil, fmt.Errorf("%s: %w", op.referencia, err)
		}
	}
	return p, nil
}

// topeFilas es el tope de filas por nodo con el que se valida -replicas.
func (op opciones) topeFilas() int {
	if op.maxFilas <= 0 {
		return cluster.MaxFilasPorOmision
	}
	return op.maxFilas
}

// comprobarReplicas rechaza un -replicas que a algún nodo le dejaría un rango
// de más filas que su tope, y dice cuál es el máximo.
func comprobarReplicas(replicas, n, nodos, tope int) error {
	maximo := cluster.MaxReplicas(n, nodos, tope)
	switch {
	case replicas <= maximo:
		return nil
	case maximo < 1:
		return fmt.Errorf("el gold tiene %d filas y %d nodos que cargan hasta %d filas cada uno (-max-filas) "+
			"no alcanzan ni para una copia: hacen falta más nodos o un -max-filas más alto en los nodos y aquí", n, nodos, tope)
	}
	return fmt.Errorf("-replicas %d es demasiado: son %d filas por nodo y cada uno carga hasta %d (-max-filas); "+
		"con %d nodos y un gold de %d filas el máximo es -replicas %d",
		replicas, filasPorNodo(replicas, n, nodos), tope, nodos, n, maximo)
}

// filasPorNodo es el rango más grande de Repartir: réplicas × n filas entre
// nodos, redondeado hacia arriba. En float64 porque solo va a un mensaje y
// réplicas × n puede no caber en un int.
func filasPorNodo(replicas, n, nodos int) int64 {
	return int64(math.Ceil(float64(replicas) * float64(n) / float64(nodos)))
}

// correr es el comando entero salvo los flags y el listener: prepara, espera
// los nodos, distribuye, escribe y resume. trasEspera, si no es nil, se llama
// al terminar la espera (main restablece ahí el Ctrl+C normal).
func correr(ctx context.Context, op opciones, ln net.Listener, lg *log.Logger, w io.Writer, trasEspera func()) error {
	p, err := preparar(op)
	if err != nil {
		return err
	}
	lg.Printf("gold %s: %d filas × %d features, huella %.19s…", p.gold.Ruta, p.gold.N, p.gold.D, p.huella)
	if p.gold.Replicas > 1 {
		lg.Printf("tabla virtual: %d réplicas, %d filas", p.gold.Replicas, p.gold.FilasTabla)
	}
	if p.receta.Chunk != cluster.ChunkPorOmision {
		lg.Printf("aviso: la receta usa chunk %d y los nodos usan %d si no se les dice otra cosa: "+
			"lánzalos con -chunk %d para que un nodo dé el modelo de cmd/entrenar bit a bit",
			p.receta.Chunk, cluster.ChunkPorOmision, p.receta.Chunk)
	}

	inicio := time.Now()
	miembros, err := reunir(ctx, ln, op, lg)
	espera := time.Since(inicio)
	if trasEspera != nil {
		trasEspera()
	}
	if err != nil {
		return err
	}
	s, err := distribuir(miembros, p, op, lg)
	if err != nil {
		return err
	}
	s.NodosEsperados = op.nodos
	s.Ms.EsperaNodos = msDe(espera)
	if err := guardarJSON(op.salida, s); err != nil {
		return err
	}
	imprimir(w, s, op.salida)
	if c := s.Referencia; c != nil && c.Comparable && !c.DentroDeTolerancia {
		return fmt.Errorf("%w: dif. máx. de centroides %.3g, inercia %.3g (relativa), iteraciones %d contra %d; "+
			"el modelo quedó en %s", errFueraDeTolerancia, c.DifMaxCentroides, c.DifInerciaRelativa,
			s.Iteraciones, c.Iteraciones, op.salida)
	}
	return nil
}

// reunir espera los nodos y cierra el listener: los que lleguen tarde
// reintentan solos y entran en la corrida siguiente. Si vence el plazo sin los
// N no hay corrida (se pidieron N; una medición con menos, rotulada como de N,
// confunde); con Ctrl+C sí, con los que llegaron: lo decidió el operador.
func reunir(ctx context.Context, ln net.Listener, op opciones, lg *log.Logger) ([]*cluster.Miembro, error) {
	espera, cancel := context.WithTimeoutCause(ctx, op.plazo, errPlazoEspera)
	defer cancel()
	lg.Printf("espero %d nodos durante %v (Ctrl+C empieza con los que hayan llegado)", op.nodos, op.plazo)
	miembros := cluster.EsperarNodos(espera, ln, op.nodos, op.plazoHola, lg)
	ln.Close()
	if len(miembros) == op.nodos {
		return miembros, nil
	}
	if len(miembros) == 0 || errors.Is(context.Cause(espera), errPlazoEspera) {
		err := fmt.Errorf("llegaron %d de %d nodos en %v: no hay corrida (Ctrl+C mientras espera empieza con los que haya)",
			len(miembros), op.nodos, op.plazo)
		cluster.Abortar(cluster.Enlaces(miembros), err.Error(), 2*time.Second)
		return nil, err
	}
	lg.Printf("Ctrl+C: corro con los %d de %d nodos que llegaron", len(miembros), op.nodos)
	return miembros, nil
}

// distribuir reparte, carga y corre Lloyd. Cualquier falla aborta la corrida:
// los nodos reciben un ERROR con el motivo y no se devuelve ningún modelo.
func distribuir(miembros []*cluster.Miembro, p *preparacion, op opciones, lg *log.Logger) (*salida, error) {
	enlaces := cluster.Enlaces(miembros)
	abortar := func(err error) error {
		cluster.Abortar(enlaces, err.Error(), 2*time.Second)
		return err
	}
	// Otra vez, con los nodos que hay: con Ctrl+C pueden ser menos que -nodos y
	// a cada uno le tocan más filas.
	if err := comprobarReplicas(p.gold.Replicas, p.gold.N, len(enlaces), op.topeFilas()); err != nil {
		return nil, abortar(err)
	}
	rangos, err := cluster.Repartir(p.gold.FilasTabla, len(enlaces))
	if err != nil {
		return nil, abortar(err)
	}

	inicio := time.Now()
	cargas, err := cluster.Cargar(enlaces, rangos, p.huella, inicio.Add(op.plazoCarga))
	if err != nil {
		return nil, abortar(err)
	}
	carga := time.Since(inicio)
	lg.Printf("los %d nodos cargaron su rango en %.0f ms; empieza Lloyd (k = %d, semilla %d)",
		len(enlaces), msDe(carga), p.op.K, p.inicio.Semilla)

	inicioLloyd := time.Now()
	c, err := cluster.Lloyd(enlaces, p.inicio.Centroides, p.op, op.plazoRonda, 1)
	if err != nil {
		return nil, abortar(err)
	}
	lloyd := time.Since(inicioLloyd)
	if err := cluster.Terminar(enlaces, 2*time.Second); err != nil {
		lg.Printf("aviso: el modelo está completo, pero no todos los nodos recibieron el FIN: %v", err)
	}
	total := time.Since(inicio)

	s, err := armarSalida(p, op, c)
	if err != nil {
		return nil, err
	}
	for i, m := range miembros {
		s.Nodos = append(s.Nodos, infoNodo{Nodo: m.Enlace.Nodo, Direccion: m.Direccion, Workers: m.Enlace.Workers,
			Rango: rangos[i], Filas: rangos[i].Len(), MsCarga: msDe(cargas[i]), Rezagados: m.Enlace.Rezagados})
	}
	var rondas, fusiones time.Duration
	for i := range c.Rondas {
		rondas += c.Rondas[i]
		fusiones += c.Fusiones[i]
	}
	s.Ms = tiempos{Carga: msDe(carga), Rondas: msDe(rondas), Fusiones: msDe(fusiones), Lloyd: msDe(lloyd),
		PasadaFinal: msDe(c.PasadaFinal), Total: msDe(total)}
	return s, nil
}

// salida es reports/kmeans_distribuido.json. Los nombres siguen a los de
// reports/kmeans.json donde dicen lo mismo.
type salida struct {
	Metadatos       kmeans.Metadatos `json:"metadatos"`
	Receta          kmeans.Receta    `json:"receta"`
	Gold            infoGold         `json:"gold"`
	Huella          string           `json:"huella"`
	Estandarizacion string           `json:"estandarizacion"`
	Modo            string           `json:"modo"`

	K                   int         `json:"k"`
	KEfectivo           int         `json:"k_efectivo"`
	Semilla             int64       `json:"semilla"`
	CentroidesIniciales infoInicios `json:"centroides_iniciales"`
	NotaSemillas        string      `json:"nota_semillas"`
	TolAbs              float64     `json:"tol_abs"`
	TolRel              float64     `json:"tol_rel"`
	MaxIter             int         `json:"max_iter"`

	NodosEsperados int        `json:"nodos_esperados"`
	Nodos          []infoNodo `json:"nodos"`
	Reparto        string     `json:"reparto"`
	PlazoRondaMs   float64    `json:"plazo_ronda_ms"`

	Iteraciones int       `json:"iteraciones"`
	Paro        string    `json:"paro"`
	Movimiento  float64   `json:"movimiento_ultima_iteracion"`
	Vacios      int       `json:"clusters_vacios"`
	Inercia     float64   `json:"inercia"`
	Inercias    []float64 `json:"inercias_por_iteracion"`
	Tamanos     []int     `json:"tamanos"`

	Features                    []string    `json:"features"`
	CentroidesEstandarizados    [][]float64 `json:"centroides_estandarizados"`
	CentroidesDesEstandarizados [][]float64 `json:"centroides_des_estandarizados,omitempty"`

	MsRonda     []float64 `json:"ms_ronda_por_iteracion"`
	MsFusion    []float64 `json:"ms_fusion_por_iteracion"`
	MsRondas    resumenMs `json:"ms_ronda"`
	MsFusiones  resumenMs `json:"ms_fusion"`
	Ms          tiempos   `json:"ms"`
	NotaTiempos string    `json:"nota_tiempos"`

	Referencia *comparacion `json:"referencia,omitempty"`
}

type infoInicios struct {
	Archivo string `json:"archivo"`
	Sha256  string `json:"sha256"`
}

type infoNodo struct {
	Nodo      string          `json:"nodo"`
	Direccion string          `json:"direccion"`
	Workers   int             `json:"workers"`
	Rango     protocolo.Rango `json:"rango"`
	Filas     int             `json:"filas"`
	MsCarga   float64         `json:"ms_carga"`
	Rezagados int             `json:"rezagados"`
}

type resumenMs struct {
	Media float64 `json:"media"`
	P50   float64 `json:"p50"`
	P95   float64 `json:"p95"`
	Max   float64 `json:"max"`
}

// tiempos en ms; ver notaTiempos.
type tiempos struct {
	EsperaNodos float64 `json:"espera_nodos"`
	Carga       float64 `json:"carga"`
	Rondas      float64 `json:"rondas"`
	Fusiones    float64 `json:"fusiones"`
	Lloyd       float64 `json:"lloyd"`
	PasadaFinal float64 `json:"pasada_final"`
	Total       float64 `json:"total"`
}

// comparacion es la diferencia con reports/kmeans.json. Con r réplicas la
// inercia y los tamaños de la referencia se multiplican por r.
type comparacion struct {
	Archivo                string  `json:"archivo"`
	SemillaReferencia      int64   `json:"semilla_referencia"`
	Comparable             bool    `json:"comparable"`
	Motivo                 string  `json:"motivo,omitempty"`
	Iteraciones            int     `json:"iteraciones_referencia"`
	MismasIteraciones      bool    `json:"mismas_iteraciones"`
	DifMaxCentroides       float64 `json:"dif_max_centroides"`
	DifInerciaRelativa     float64 `json:"dif_inercia_relativa"`
	DifMaxInerciasRelativa float64 `json:"dif_max_inercias_por_iteracion_relativa"`
	MismosTamanos          bool    `json:"mismos_tamanos"`
	BitABit                bool    `json:"bit_a_bit"`
	Tolerancia             float64 `json:"tolerancia"`
	DentroDeTolerancia     bool    `json:"dentro_de_tolerancia"`
	Nota                   string  `json:"nota"`
}

const (
	modo = "distribuido por datos: rangos contiguos de filas, una ronda ITERAR → PARCIAL por iteración con " +
		"barrera y plazo, fusión de los parciales en orden de nodo (docs/diseno.md, §4)"
	reparto = "rangos contiguos iguales (difieren a lo sumo en una fila), en orden de nodo por nombre; " +
		"con replicas > 1, sobre la tabla virtual (fila i = fila i mod N del gold)"
	notaSemillas = "Una semilla por corrida: la de la receta o -semilla. Para las semillas de la receta, se corre " +
		"una vez por semilla (semilla, semilla+1, …) y se elige la de menor inercia final (empate: la semilla menor), " +
		"como cmd/entrenar. En una sola corrida sería repetir cluster.Lloyd sobre las mismas conexiones ya cargadas, " +
		"numerando las rondas desde Corrida.UltimaRonda + 1."
	notaTiempos = "ms_ronda_por_iteracion: la barrera de red de cada iteración, desde que sale el primer ITERAR hasta " +
		"que llega el último PARCIAL (cómputo de los nodos más latencia). ms_fusion_por_iteracion: lo que hace el " +
		"coordinador después: validar los parciales, fusionarlos en orden de nodo y CerrarIteracion (actualizar los " +
		"centroides y el criterio de parada). ms.carga: CARGAR → LISTO de todos los nodos. ms.lloyd: desde el primer " +
		"ITERAR hasta el fin de la pasada final. ms.total: desde el primer CARGAR hasta el FIN; no incluye " +
		"ms.espera_nodos, que depende de cuándo se lanzan los nodos."
	notaTolerancia = "Centroides: diferencia absoluta en el espacio estandarizado; inercia: relativa. Reasociar la " +
		"suma de los parciales de K nodos cambia los últimos bits (del orden de 1e-16); que una fila cambie de " +
		"cluster mueve un centroide más de 1e-5 en el gold real. La tolerancia queda entre los dos: dentro, la " +
		"diferencia es redondeo; fuera, es otra asignación. Con un nodo, replicas = 1 y el chunk de la receta, " +
		"el resultado es el de cmd/entrenar bit a bit."
)

func armarSalida(p *preparacion, op opciones, c *cluster.Corrida) (*salida, error) {
	r := c.Resultado
	s := &salida{
		Metadatos: kmeans.RecolectarMetadatos(), Receta: p.receta, Gold: p.gold, Huella: p.huella,
		Estandarizacion: op.estandarizacion, Modo: modo,
		K: p.receta.K, KEfectivo: r.K, Semilla: p.inicio.Semilla,
		CentroidesIniciales: infoInicios{Archivo: op.inicios, Sha256: kmeans.HashCentroides(p.inicio.Centroides)},
		NotaSemillas:        notaSemillas, TolAbs: p.op.TolAbs, TolRel: p.op.TolRel, MaxIter: p.op.MaxIter,
		Reparto: reparto, PlazoRondaMs: msDe(op.plazoRonda),
		Iteraciones: r.Iteraciones, Paro: r.Paro, Movimiento: r.Movimiento, Vacios: r.Vacios,
		Inercia: r.Inercia, Inercias: r.Inercias, Tamanos: c.Tamanos, Features: p.receta.Features,
		MsRonda: msDeTodos(c.Rondas), MsFusion: msDeTodos(c.Fusiones),
		MsRondas: resumir(c.Rondas), MsFusiones: resumir(c.Fusiones), NotaTiempos: notaTiempos,
	}
	for j := range r.K {
		s.CentroidesEstandarizados = append(s.CentroidesEstandarizados, append([]float64(nil), r.Centroide(j)...))
		if p.est.CentroidesEnUnidades() {
			x := make([]float64, r.D)
			if err := p.est.Invertir(r.Centroide(j), "", x); err != nil {
				return nil, err
			}
			s.CentroidesDesEstandarizados = append(s.CentroidesDesEstandarizados, x)
		}
	}
	if p.ref != nil {
		s.Referencia = comparar(p.ref, op.referencia, p.gold.Sha256, p.inicio.Semilla, p.gold.Replicas, r, c.Tamanos, op.tolerancia)
	}
	return s, nil
}

// comparar mide la diferencia con el modelo de cmd/entrenar. Solo es completa
// si la referencia es del mismo gold y publica los centroides de esta semilla
// (la elegida entre las que probó); de otra semilla, reports/kmeans.json solo
// guarda la inercia y las iteraciones de su arranque.
func comparar(ref *referencia, archivo, sha string, semilla int64, replicas int, r *kmeans.Resultado, tamanos []int, tol float64) *comparacion {
	c := &comparacion{Archivo: archivo, SemillaReferencia: ref.Semilla, Tolerancia: tol, Nota: notaTolerancia}
	if ref.Gold.Sha256 != sha {
		c.Motivo = fmt.Sprintf("%s es de otro gold (sha256 %.12s…; el de esta corrida es %.12s…)", archivo, ref.Gold.Sha256, sha)
		return c
	}
	escala := float64(replicas)
	if ref.Semilla != semilla {
		for _, a := range ref.Arranques {
			if a.Semilla == semilla {
				c.Motivo = fmt.Sprintf("%s publica los centroides de la semilla %d; de la semilla %d solo guarda la inercia "+
					"y las iteraciones de su arranque", archivo, ref.Semilla, semilla)
				c.Iteraciones = a.Iteraciones
				c.MismasIteraciones = a.Iteraciones == r.Iteraciones
				c.DifInerciaRelativa = relativa(r.Inercia/escala, a.Inercia)
				return c
			}
		}
		c.Motivo = fmt.Sprintf("%s no tiene la semilla %d", archivo, semilla)
		return c
	}
	if len(ref.Centroides) != r.K {
		c.Motivo = fmt.Sprintf("%s tiene %d centroides y esta corrida %d", archivo, len(ref.Centroides), r.K)
		return c
	}
	for _, fila := range ref.Centroides {
		if len(fila) != r.D {
			c.Motivo = fmt.Sprintf("%s tiene centroides de %d features y esta corrida de %d", archivo, len(fila), r.D)
			return c
		}
	}

	c.Comparable = true
	c.Iteraciones = ref.Iteraciones
	c.MismasIteraciones = ref.Iteraciones == r.Iteraciones
	bits := replicas == 1 && c.MismasIteraciones && len(r.Inercias) == len(ref.Inercias) &&
		mismoBit(r.Inercia, ref.Inercia)
	for j, fila := range ref.Centroides {
		for f, b := range fila {
			a := r.Centroides[j*r.D+f]
			c.DifMaxCentroides = max(c.DifMaxCentroides, math.Abs(a-b))
			bits = bits && mismoBit(a, b)
		}
	}
	c.DifInerciaRelativa = relativa(r.Inercia/escala, ref.Inercia)
	for i := range min(len(r.Inercias), len(ref.Inercias)) {
		c.DifMaxInerciasRelativa = max(c.DifMaxInerciasRelativa, relativa(r.Inercias[i]/escala, ref.Inercias[i]))
		bits = bits && mismoBit(r.Inercias[i], ref.Inercias[i])
	}
	c.MismosTamanos = len(tamanos) == len(ref.Tamanos)
	for j := range min(len(tamanos), len(ref.Tamanos)) {
		c.MismosTamanos = c.MismosTamanos && tamanos[j] == replicas*ref.Tamanos[j]
	}
	c.BitABit = bits && c.MismosTamanos
	c.DentroDeTolerancia = c.MismasIteraciones && c.MismosTamanos && c.DifMaxCentroides <= tol &&
		c.DifInerciaRelativa <= tol && c.DifMaxInerciasRelativa <= tol
	return c
}

func mismoBit(a, b float64) bool { return math.Float64bits(a) == math.Float64bits(b) }

func relativa(a, b float64) float64 {
	if b == 0 {
		return math.Abs(a)
	}
	return math.Abs(a-b) / math.Abs(b)
}

func msDe(d time.Duration) float64 { return float64(d.Nanoseconds()) / 1e6 }

func msDeTodos(ds []time.Duration) []float64 {
	out := make([]float64, len(ds))
	for i, d := range ds {
		out[i] = msDe(d)
	}
	return out
}

func resumir(ds []time.Duration) resumenMs {
	if len(ds) == 0 {
		return resumenMs{}
	}
	var suma time.Duration
	for _, d := range ds {
		suma += d
	}
	return resumenMs{Media: msDe(suma) / float64(len(ds)), P50: msDe(cluster.Percentil(ds, 50)),
		P95: msDe(cluster.Percentil(ds, 95)), Max: msDe(cluster.Percentil(ds, 100))}
}

// guardarJSON escribe en un temporal y lo renombra: si algo falla a mitad, el
// archivo anterior queda intacto y nunca hay un modelo a medias.
func guardarJSON(ruta string, v any) error {
	if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("%s: %w", ruta, err)
	}
	tmp := ruta + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, ruta)
}

func imprimir(w io.Writer, s *salida, ruta string) {
	fmt.Fprintf(w, "\ngold        %s · %d filas × %d features · huella %.19s…\n", s.Gold.Ruta, s.Gold.N, s.Gold.D, s.Huella)
	if s.Gold.Replicas > 1 {
		fmt.Fprintf(w, "tabla       virtual: %d réplicas · %d filas (fila i = fila i mod %d)\n", s.Gold.Replicas, s.Gold.FilasTabla, s.Gold.N)
	}
	fmt.Fprintf(w, "receta      %s · nivel %s · k=%d · semilla %d · tolerancia %g · max_iter %d\n",
		strings.Join(s.Features, ", "), s.Receta.Nivel, s.KEfectivo, s.Semilla, s.TolAbs, s.MaxIter)
	fmt.Fprintf(w, "nodos       %d de %d · rangos contiguos iguales, en orden de nodo\n", len(s.Nodos), s.NodosEsperados)
	for _, n := range s.Nodos {
		fmt.Fprintf(w, "            %-14s %-22s %3d workers  [%d, %d)  %d filas  carga %.0f ms\n",
			n.Nodo, n.Direccion, n.Workers, n.Rango.Desde, n.Rango.Hasta, n.Filas, n.MsCarga)
	}
	fmt.Fprintf(w, "resultado   %d iteraciones (paro=%s) · inercia %.6e · tamaños %v\n", s.Iteraciones, s.Paro, s.Inercia, s.Tamanos)
	fmt.Fprintf(w, "ronda       p50 %.3f ms · p95 %.3f ms · máx %.3f ms · media %.3f ms  (barrera: ITERAR → último PARCIAL)\n",
		s.MsRondas.P50, s.MsRondas.P95, s.MsRondas.Max, s.MsRondas.Media)
	fmt.Fprintf(w, "fusión      p50 %.3f ms · p95 %.3f ms · máx %.3f ms · media %.3f ms  (fusión en orden de nodo y actualización)\n",
		s.MsFusiones.P50, s.MsFusiones.P95, s.MsFusiones.Max, s.MsFusiones.Media)
	fmt.Fprintf(w, "tiempos     carga %.1f ms · lloyd %.1f ms (pasada final %.2f ms) · total %.1f ms · espera de nodos %.1f s\n",
		s.Ms.Carga, s.Ms.Lloyd, s.Ms.PasadaFinal, s.Ms.Total, s.Ms.EsperaNodos/1000)
	if c := s.Referencia; c != nil {
		fmt.Fprintf(w, "referencia  %s (semilla %d): %s\n", c.Archivo, c.SemillaReferencia, veredicto(c, s.Iteraciones))
	}
	fmt.Fprintf(w, "→ %s\n", ruta)
}

func veredicto(c *comparacion, iteraciones int) string {
	detalle := fmt.Sprintf("%d iteraciones contra %d · dif. máx. de centroides %.3g · inercia %.3g relativa",
		iteraciones, c.Iteraciones, c.DifMaxCentroides, c.DifInerciaRelativa)
	switch {
	case !c.Comparable:
		return "no comparable: " + c.Motivo
	case c.BitABit:
		return "BIT A BIT (" + detalle + ")"
	case c.DentroDeTolerancia:
		return fmt.Sprintf("DENTRO DE TOLERANCIA %g (%s)", c.Tolerancia, detalle)
	default:
		return fmt.Sprintf("FUERA DE TOLERANCIA %g (%s)", c.Tolerancia, detalle)
	}
}
