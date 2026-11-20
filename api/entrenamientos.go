package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/R0SEWT/aire-distribuido/almacen"
	"github.com/R0SEWT/aire-distribuido/cluster"
	"github.com/R0SEWT/aire-distribuido/kmeans"
	"github.com/R0SEWT/aire-distribuido/modelo"
	"github.com/R0SEWT/aire-distribuido/protocolo"
)

// Entrenador es lo que la API necesita, además de los nodos, para entrenar: el
// gold en memoria (de él salen la huella y el tamaño de la tabla que se
// reparte, el K-Means secuencial con que se compara y el perfil de los
// regímenes del modelo nuevo), la receta y la estandarización de ese gold.
// LeerEntrenador lo arma desde los archivos.
type Entrenador struct {
	Gold            *kmeans.Gold
	Sha256          string // del archivo del gold, en hexadecimal
	Receta          kmeans.Receta
	Estandarizacion *kmeans.Estandarizacion
	// Inicios son los centroides iniciales guardados por semilla
	// (reports/centroides_iniciales.json). Con una semilla que esté ahí, un
	// entrenamiento parte de los mismos centroides que cmd/entrenar; con otra,
	// o sin archivo, se calculan con k-means++ sobre el gold.
	Inicios *kmeans.ArchivoInicios

	PlazoCarga time.Duration // CARGAR → LISTO de todos los nodos (30 s)
	PlazoRonda time.Duration // plazo común de cada ronda (10 s)
	// MaxFilas es el tope de filas por nodo con que se valida replicas: el
	// -max-filas de los nodos (0 = cluster.MaxFilasPorOmision).
	MaxFilas int
}

// LeerEntrenador carga la receta, la estandarización y el gold, y comprueba
// que sean de la misma corrida. inicios puede ser "" o no existir.
func LeerEntrenador(receta, estandarizacion, gold, inicios string) (*Entrenador, error) {
	rec, err := kmeans.LeerReceta(receta)
	if err != nil {
		return nil, err
	}
	est, err := kmeans.LeerEstandarizacion(estandarizacion)
	if err != nil {
		return nil, err
	}
	if err := est.Coincide(rec); err != nil {
		return nil, fmt.Errorf("%s: %w", estandarizacion, err)
	}
	g, err := kmeans.LeerGold(gold, rec.Features)
	if err != nil {
		return nil, err
	}
	if est.N != g.Datos.N {
		return nil, fmt.Errorf("%s dice n = %d y %s tiene %d filas: no es su estandarización", estandarizacion, est.N, gold, g.Datos.N)
	}
	sha, err := kmeans.Sha256Archivo(gold)
	if err != nil {
		return nil, err
	}
	e := &Entrenador{Gold: g, Sha256: sha, Receta: rec, Estandarizacion: est}
	if inicios != "" {
		if arch, err := kmeans.LeerInicios(inicios); err == nil {
			e.Inicios = &arch
		}
	}
	return e, nil
}

func (e *Entrenador) plazoCarga() time.Duration {
	if e.PlazoCarga <= 0 {
		return 30 * time.Second
	}
	return e.PlazoCarga
}

func (e *Entrenador) plazoRonda() time.Duration {
	if e.PlazoRonda <= 0 {
		return 10 * time.Second
	}
	return e.PlazoRonda
}

func (e *Entrenador) maxFilas() int {
	if e.MaxFilas <= 0 {
		return cluster.MaxFilasPorOmision
	}
	return e.MaxFilas
}

// inicio devuelve los centroides iniciales de una semilla: los guardados, si
// el archivo es de este gold y de esta receta y tiene esa semilla; si no,
// k-means++ sobre el gold.
func (e *Entrenador) inicio(semilla int64) (kmeans.Inicio, string, error) {
	if e.Inicios != nil {
		if ins, err := e.Inicios.Para(e.Sha256, e.Receta, e.Gold.Datos.D, semilla, 1); err == nil {
			return ins[0], "guardados", nil
		}
	}
	cent, kEf, err := kmeans.KMeansPP(e.Gold.Datos, e.Receta.K, semilla)
	if err != nil {
		return kmeans.Inicio{}, "", err
	}
	return kmeans.Inicio{Semilla: semilla, KEfectivo: kEf, Centroides: cent}, "k-means++", nil
}

// Nombres de los eventos del WebSocket (esquema EventoWS del contrato).
const (
	eventoRonda = "entrenamiento.ronda"
	eventoFin   = "entrenamiento.fin"
)

var errApagando = errors.New("la API se está apagando")

// coordinador es el coordinador del clúster dentro de la API (spec de la PC4,
// §5.2): lanza un entrenamiento por pedido, de a uno, y reparte lo que va
// pasando.
//
// Un entrenamiento corre en su propia goroutine: el pedido responde 202 con el
// id y no la espera. Solo hay uno a la vez: sem es un semáforo de capacidad 1
// (un canal con búfer de un lugar); quien no puede tomarlo responde 409.
//
// Cada ronda tiene TRES consumidores, y cada uno tiene su propio canal: el
// almacén (una goroutine por entrenamiento que escribe en Entrenamientos, con
// un canal donde caben todas las rondas, así que nunca frena a Lloyd ni pierde
// una), el hub del WebSocket (su canal de difusión) y el punto de control en
// Redis (un buzón de un lugar, porque solo importa el último: puntos.go). No
// comparten un canal: de un canal compartido cada ronda la recibiría uno solo.
type coordinador struct {
	s              *Servidor
	pool           *cluster.Pool
	ent            *Entrenador
	entrenamientos almacen.Entrenamientos
	log            *slog.Logger

	sem chan struct{} // capacidad 1: el entrenamiento en curso

	mu       sync.Mutex
	apagado  bool
	enCurso  string         // id del entrenamiento en curso, "" si no hay
	ultimo   string         // id del último que se lanzó
	corridas sync.WaitGroup // las goroutines de los entrenamientos

	ctx      context.Context // se cancela al apagar: corta el entrenamiento en curso
	cancelar context.CancelCauseFunc
}

func nuevoCoordinador(s *Servidor, pool *cluster.Pool, ent *Entrenador, entrenamientos almacen.Entrenamientos) *coordinador {
	c := &coordinador{s: s, pool: pool, ent: ent, entrenamientos: entrenamientos, log: s.log, sem: make(chan struct{}, 1)}
	c.ctx, c.cancelar = context.WithCancelCause(context.Background())
	return c
}

// apagar corta el entrenamiento en curso, si hay, y vuelve cuando dejó su
// estado final (abortado) en el almacén y su evento de fin en el hub.
func (c *coordinador) apagar() {
	c.mu.Lock()
	c.apagado = true
	c.mu.Unlock()
	c.cancelar(errApagando)
	c.corridas.Wait()
}

func (c *coordinador) ultimoEntrenamiento() *string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ultimo == "" {
		return nil
	}
	id := c.ultimo
	return &id
}

// Mensajes de 400 de POST /api/entrenamientos.
const (
	msgCuerpoEntrenamiento = "el cuerpo tiene que ser un objeto JSON con nodos, replicas y semilla, los tres enteros"
	msgNodos               = "nodos tiene que ser un entero mayor o igual a 1"
	msgReplicas            = "replicas tiene que ser un entero mayor o igual a 1"
	msgSemilla             = "semilla tiene que ser un entero (int64)"
)

// leerPedidoEntrenamiento valida el cuerpo campo por campo, en el orden del
// esquema PedidoEntrenamiento, y devuelve el mensaje del primer campo inválido.
func leerPedidoEntrenamiento(w http.ResponseWriter, r *http.Request) (almacen.ParametrosEntrenamiento, string) {
	var c struct {
		Nodos    *int   `json:"nodos"`
		Replicas *int   `json:"replicas"`
		Semilla  *int64 `json:"semilla"`
	}
	if err := leerJSON(w, r, &c); err != nil {
		if errors.Is(err, errCuerpoGrande) {
			return almacen.ParametrosEntrenamiento{}, msgCuerpoGrande
		}
		var te *json.UnmarshalTypeError
		if errors.As(err, &te) {
			switch te.Field {
			case "nodos":
				return almacen.ParametrosEntrenamiento{}, msgNodos
			case "replicas":
				return almacen.ParametrosEntrenamiento{}, msgReplicas
			case "semilla":
				return almacen.ParametrosEntrenamiento{}, msgSemilla
			}
		}
		return almacen.ParametrosEntrenamiento{}, msgCuerpoEntrenamiento
	}
	switch {
	case c.Nodos == nil || *c.Nodos < 1:
		return almacen.ParametrosEntrenamiento{}, msgNodos
	case c.Replicas == nil || *c.Replicas < 1:
		return almacen.ParametrosEntrenamiento{}, msgReplicas
	case c.Semilla == nil:
		return almacen.ParametrosEntrenamiento{}, msgSemilla
	}
	return almacen.ParametrosEntrenamiento{Nodos: *c.Nodos, Replicas: *c.Replicas, Semilla: *c.Semilla}, ""
}

// POST /api/entrenamientos
//
// En orden: se valida el pedido (400), se toma el semáforo (409 si hay un
// entrenamiento en curso), se sacan los nodos del pool (422 si no hay
// suficientes libres), se guarda el entrenamiento en curso y se responde 202
// con su id. El entrenamiento sigue en su goroutine.
func (s *Servidor) iniciarEntrenamiento(w http.ResponseWriter, r *http.Request) {
	c := s.coord
	par, msg := leerPedidoEntrenamiento(w, r)
	if msg != "" {
		escribirError(w, http.StatusBadRequest, msg)
		return
	}
	// Antes que el semáforo: durante el apagado la respuesta es 503, también
	// si el entrenamiento en curso todavía se está cortando (sería un 409).
	if s.apagando.Load() {
		escribirError(w, http.StatusServiceUnavailable, errApagando.Error())
		return
	}
	if c.ent == nil {
		escribirError(w, http.StatusServiceUnavailable, "esta API no puede entrenar: arrancó sin el gold (-gold)")
		return
	}
	n := c.ent.Gold.Datos.N
	if par.Nodos > n {
		escribirError(w, http.StatusBadRequest, fmt.Sprintf("nodos no puede superar las %d filas del gold", n))
		return
	}
	if maximo := cluster.MaxReplicas(n, par.Nodos, c.ent.maxFilas()); par.Replicas > maximo {
		escribirError(w, http.StatusBadRequest, fmt.Sprintf("replicas %d es demasiado: con %d nodos que cargan hasta %d filas cada uno "+
			"y un gold de %d filas, el máximo es %d", par.Replicas, par.Nodos, c.ent.maxFilas(), n, maximo))
		return
	}
	inicio, origen, err := c.ent.inicio(par.Semilla)
	if err != nil {
		s.log.Error("centroides iniciales", "semilla", par.Semilla, "err", err)
		escribirError(w, http.StatusInternalServerError, "no se pudieron calcular los centroides iniciales")
		return
	}

	// El semáforo: un entrenamiento a la vez. No se espera: si está tomado, 409.
	select {
	case c.sem <- struct{}{}:
	default:
		c.mu.Lock()
		enCurso := c.enCurso
		c.mu.Unlock()
		if enCurso == "" { // otro pedido lo tomó y todavía no tiene id
			escribirError(w, http.StatusConflict, "ya hay un entrenamiento en curso")
		} else {
			escribirError(w, http.StatusConflict, "ya hay un entrenamiento en curso: "+enCurso)
		}
		return
	}
	lanzado := false
	defer func() {
		if !lanzado {
			<-c.sem
		}
	}()

	c.mu.Lock()
	if c.apagado {
		c.mu.Unlock()
		escribirError(w, http.StatusServiceUnavailable, errApagando.Error())
		return
	}
	c.corridas.Add(1) // con el candado: nunca después de que apagar empezó a esperar
	c.mu.Unlock()
	defer func() {
		if !lanzado {
			c.corridas.Done()
		}
	}()

	var miembros []*cluster.Miembro
	if c.pool == nil {
		err = &cluster.ErrorFaltanNodos{Pedidos: par.Nodos}
	} else {
		miembros, err = c.pool.Tomar(par.Nodos)
	}
	if err != nil {
		escribirError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	devolver := func(motivo string) {
		c.pool.Soltar(miembros, motivo)
		cluster.Abortar(cluster.Enlaces(miembros), motivo, 2*time.Second)
	}
	rangos, err := cluster.Repartir(par.Replicas*n, len(miembros))
	if err != nil {
		devolver(err.Error())
		escribirError(w, http.StatusBadRequest, err.Error())
		return
	}

	e := almacen.Entrenamiento{Inicio: enLima(s.ahora()).Truncate(time.Millisecond), Estado: almacen.EstadoEnCurso, Parametros: par}
	for i, m := range miembros {
		e.Nodos = append(e.Nodos, almacen.NodoEntrenamiento{Nombre: m.Enlace.Nodo, Direccion: m.Direccion, Workers: m.Enlace.Workers,
			Desde: rangos[i].Desde, Hasta: rangos[i].Hasta})
	}
	if e.ID, err = c.crear(r.Context(), e); err != nil {
		devolver("el entrenamiento no se pudo registrar")
		s.log.Error("crear el entrenamiento", "err", err)
		escribirError(w, http.StatusServiceUnavailable, "MongoDB no disponible")
		return
	}
	c.mu.Lock()
	c.enCurso, c.ultimo = e.ID, e.ID
	c.mu.Unlock()
	s.log.Info("entrenamiento lanzado", "id", e.ID, "nodos", len(miembros), "replicas", par.Replicas, "semilla", par.Semilla,
		"centroides_iniciales", origen)

	lanzado = true
	go c.correr(e, miembros, rangos, inicio)
	s.responder(w, r, http.StatusAccepted, struct {
		ID     string `json:"id"`
		Estado string `json:"estado"`
	}{e.ID, almacen.EstadoEnCurso})
}

// crear guarda el entrenamiento con un id nuevo: e-AAAAMMDD-HHMM en la hora de
// Lima, como en el contrato; si en ese minuto ya hubo otro, con un sufijo
// (-2, -3…). Un entrenamiento dura milisegundos, así que dos en el mismo
// minuto es lo normal en una demostración.
func (c *coordinador) crear(ctx context.Context, e almacen.Entrenamiento) (string, error) {
	base := "e-" + e.Inicio.Format("20060102-1504")
	for i := 1; i <= 10000; i++ {
		e.ID = base
		if i > 1 {
			e.ID = fmt.Sprintf("%s-%d", base, i)
		}
		err := c.entrenamientos.Crear(ctx, e)
		if err == nil {
			return e.ID, nil
		}
		if !errors.Is(err, almacen.ErrYaExiste) {
			return "", err
		}
	}
	return "", fmt.Errorf("no quedó un id libre con la base %s", base)
}

// eventoDeRonda es EventoRonda en el contrato: la ronda con el id de su entrenamiento.
type eventoDeRonda struct {
	ID string `json:"id"`
	almacen.Ronda
}

// correr es la goroutine de un entrenamiento. Devuelve el semáforo al final.
func (c *coordinador) correr(e almacen.Entrenamiento, miembros []*cluster.Miembro, rangos []protocolo.Rango, inicio kmeans.Inicio) {
	defer c.corridas.Done()
	enlaces := cluster.Enlaces(miembros)
	op := c.ent.Receta.OpcionesConc().Opciones
	op.K = inicio.KEfectivo

	// El consumidor del almacén: su canal tiene lugar para todas las rondas de
	// la corrida (a lo sumo max_iter), así que mandarle nunca espera.
	paraAlmacen := make(chan almacen.Ronda, op.MaxIter)
	guardadas := make(chan struct{})
	go func() {
		defer close(guardadas)
		for r := range paraAlmacen {
			c.conPlazo(func(ctx context.Context) error { return c.entrenamientos.AgregarRonda(ctx, e.ID, r) }, "guardar la ronda", e.ID)
		}
	}()
	puntos := c.abrirPuntos(e.ID) // el punto de control de cada ronda (puntos.go)
	alRonda := func(r cluster.Ronda) {
		ronda := almacen.Ronda{Iter: r.Iter, Inercia: r.Inercia, MsRonda: ms(r.Barrera), MsFusion: ms(r.Fusion),
			MsPorNodo: make(map[string]float64, len(enlaces))}
		for i, en := range enlaces {
			ronda.MsPorNodo[en.Nodo] = ms(r.PorNodo[i])
		}
		paraAlmacen <- ronda                                                 // al almacén, por su canal
		c.difundir(eventoRonda, eventoDeRonda{ID: e.ID, Ronda: ronda}, e.ID) // al hub, por el suyo
		puntos.dejar(puntoDe(r.Iter, op.K, r.Centroides))                    // a Redis, por su buzón
	}

	// La corrida: CARGAR a todos, Lloyd y FIN. ms_total va del primer CARGAR
	// al FIN, como ms.total de cmd/distribuido.
	t0 := time.Now()
	var corrida *cluster.Corrida
	// La carga y Lloyd llevan el contexto del coordinador: al apagar, ninguno
	// de los dos espera a su plazo.
	_, err := cluster.CargarCtx(c.ctx, enlaces, rangos, cluster.HuellaDeSha256(c.ent.Sha256), t0.Add(c.ent.plazoCarga()))
	if err == nil {
		corrida, err = cluster.LloydCtx(c.ctx, enlaces, inicio.Centroides, op,
			cluster.OpcionesLloyd{Plazo: c.ent.plazoRonda(), Primera: 1, AlRonda: alRonda})
	}
	close(paraAlmacen)
	<-guardadas // las rondas ya están en el almacén antes de escribir el estado final
	// Y el punto de la última ronda completa, en Redis: también si la corrida
	// se abortó, que es cuando dice hasta dónde llegó.
	puntos.cerrar()

	cierre := almacen.CierreEntrenamiento{Estado: almacen.EstadoTerminado}
	if err != nil {
		cierre = almacen.CierreEntrenamiento{Estado: almacen.EstadoAbortado, Error: err.Error()}
		c.pool.Soltar(miembros, "entrenamiento "+e.ID+" abortado")
		cluster.Abortar(enlaces, err.Error(), 2*time.Second)
		for _, nodo := range nodosConError(err) {
			c.s.registrarEvento(cluster.EventoPool{TS: time.Now(), Tipo: cluster.EventoError, Nodo: nodo,
				Detalle: "hizo abortar el entrenamiento " + e.ID + ": " + err.Error()})
		}
	} else {
		c.pool.Soltar(miembros, "entrenamiento "+e.ID+" terminado")
		if errFin := cluster.Terminar(enlaces, 2*time.Second); errFin != nil {
			c.log.Warn("el modelo está completo, pero no todos los nodos recibieron el FIN", "id", e.ID, "err", errFin)
		}
		r := corrida.Resultado
		cierre.Iteraciones, cierre.Paro, cierre.MsTotal = r.Iteraciones, r.Paro, ms(time.Since(t0))
		cierre.DifSecuencial, cierre.ModeloID, cierre.Aviso = c.cerrarConModelo(e, corrida, inicio, op)
	}
	cierre.Fin = enLima(c.s.ahora()).Truncate(time.Millisecond)
	c.conPlazo(func(ctx context.Context) error { return c.entrenamientos.Cerrar(ctx, e.ID, cierre) }, "cerrar el entrenamiento", e.ID)

	// El semáforo se devuelve antes de avisar el fin, y el detalle ya tiene el
	// estado final: quien reaccione al evento no recibe un 409 ni lee un
	// entrenamiento a medio cerrar. Otro entrenamiento todavía tiene que
	// esperar a los nodos: se les cerró la conexión y vuelven a llamar después
	// de su pausa entre intentos (el -espera de cmd/nodo, 2 s); hasta entonces
	// el pool está vacío y el pedido recibe 422.
	c.mu.Lock()
	c.enCurso = ""
	c.mu.Unlock()
	<-c.sem
	if err != nil {
		c.log.Warn("entrenamiento abortado", "id", e.ID, "err", err)
		c.difundir(eventoFin, struct {
			ID    string `json:"id"`
			Error string `json:"error"`
		}{e.ID, cierre.Error}, e.ID)
		return
	}
	c.log.Info("entrenamiento terminado", "id", e.ID, "iteraciones", cierre.Iteraciones, "paro", cierre.Paro,
		"dif_secuencial", cierre.DifSecuencial, "ms_total", cierre.MsTotal, "modelo", cierre.ModeloID, "aviso", cierre.Aviso)
	// El fin es el resumen del detalle, sin la identificación.
	res := resumir(almacen.Entrenamiento{ID: e.ID, Estado: cierre.Estado, Iteraciones: cierre.Iteraciones, Paro: cierre.Paro,
		DifSecuencial: cierre.DifSecuencial, MsTotal: cierre.MsTotal, ModeloID: cierre.ModeloID, Aviso: cierre.Aviso})
	c.difundir(eventoFin, struct {
		ID            string   `json:"id"`
		Iteraciones   *int     `json:"iteraciones"`
		Paro          string   `json:"paro"`
		DifSecuencial *float64 `json:"dif_secuencial"`
		MsTotal       *float64 `json:"ms_total"`
		ModeloID      string   `json:"modelo_id,omitempty"`
		Aviso         string   `json:"aviso,omitempty"`
	}{e.ID, res.Iteraciones, res.Paro, res.DifSecuencial, res.MsTotal, res.ModeloID, res.Aviso}, e.ID)
}

// cerrarConModelo hace lo que sigue a una corrida que terminó bien: la compara
// con el K-Means secuencial sobre el mismo gold y desde los mismos centroides
// (dif_secuencial: la mayor diferencia absoluta entre centroides, en el espacio
// estandarizado) y, si replicas es 1, arma el modelo nuevo y lo activa. Con
// réplicas la corrida es una medición: los centroides son los mismos salvo
// redondeo, pero la tabla no es el gold, y no se activa nada.
//
// La comparación es también la compuerta del modelo: el coordinador no ve las
// filas de los nodos, solo comprueba la forma de cada PARCIAL y que sus conteos
// sumen su rango, así que un nodo con otros datos o con un cálculo roto puede
// llevar la corrida a otros centroides sin que nada falle. Si dif_secuencial
// supera cluster.Tolerancia (dentro es redondeo; fuera, otra asignación), el
// entrenamiento queda terminado, con sus números a la vista, pero el modelo NO
// se activa y sigue el anterior.
//
// Devuelve la diferencia, el id del modelo activado ("" si no se activó) y,
// si tenía que activarlo y no lo hizo, el aviso que dice por qué.
func (c *coordinador) cerrarConModelo(e almacen.Entrenamiento, corrida *cluster.Corrida, inicio kmeans.Inicio, op kmeans.Opciones) (dif float64, modeloID, aviso string) {
	r, d := corrida.Resultado, c.ent.Gold.Datos
	dif = math.NaN()
	sec, err := kmeans.Secuencial(d, inicio.Centroides, op)
	if err != nil {
		c.log.Error("K-Means secuencial de referencia", "id", e.ID, "err", err)
	} else {
		dif = 0
		for i := range r.Centroides {
			dif = max(dif, math.Abs(r.Centroides[i]-sec.Centroides[i]))
		}
	}
	if e.Parametros.Replicas != 1 {
		return dif, "", ""
	}
	noActivado := func(motivo string) (float64, string, string) {
		aviso := "el modelo no se activó y sigue el anterior: " + motivo
		c.log.Error("entrenamiento terminado sin activar su modelo", "id", e.ID, "aviso", aviso)
		return dif, "", aviso
	}
	switch {
	case err != nil:
		return noActivado("no se pudo comparar con el K-Means secuencial")
	case !(dif <= cluster.Tolerancia): // también si es NaN
		return noActivado(fmt.Sprintf("dif_secuencial %.3g supera la tolerancia %g; los centroides no son los del K-Means "+
			"secuencial sobre este gold (algún nodo devolvió parciales que no corresponden a sus filas)", dif, cluster.Tolerancia))
	}
	// Las asignaciones quedan repartidas en los nodos y el protocolo no las
	// trae: se recalculan aquí, con los centroides finales, para perfilar cada
	// régimen frente al ECA.
	asign := make([]int, d.N)
	kmeans.Asignar(d, r.Centroides, 0, d.N, asign)
	regimenes, err := kmeans.Perfilar(c.ent.Gold, asign, r.K, c.ent.Estandarizacion)
	var m *modelo.Modelo
	if err == nil {
		m, err = modelo.Nuevo(modelo.Piezas{ID: "entrenado-" + e.ID, Receta: c.ent.Receta, Estandarizacion: c.ent.Estandarizacion,
			Centroides: r.Centroides, Iteraciones: r.Iteraciones, Inercia: r.Inercia, Regimenes: regimenes,
			// La procedencia se guarda con el modelo: al arrancar, la API se
			// queda con el más nuevo por Creado.
			Origen: e.ID, Creado: enLima(c.s.ahora()).Truncate(time.Millisecond), GoldSHA256: c.ent.Sha256})
	}
	if err != nil {
		return noActivado("no se pudo armar con estos centroides: " + err.Error())
	}
	c.s.ActivarModelo(m)
	return dif, m.ID, ""
}

// difundir manda un evento al hub. Si el hub lo descarta, queda en el log: el
// entrenamiento no depende de que alguien lo esté mirando.
func (c *coordinador) difundir(evento string, datos any, id string) {
	if err := c.s.hub.Difundir(evento, datos, rolesEventosEntrenamiento...); err != nil {
		c.log.Warn("evento no difundido", "evento", evento, "id", id, "err", err)
	}
}

// conPlazo corre una escritura en el almacén con plazoEscritura. No usa el
// contexto del coordinador: al apagar, el estado final todavía se escribe.
func (c *coordinador) conPlazo(f func(context.Context) error, que, id string) {
	ctx, cancel := context.WithTimeout(context.Background(), plazoEscritura)
	defer cancel()
	if err := f(ctx); err != nil {
		c.log.Error("no se pudo "+que, "id", id, "err", err)
	}
}

// nodosConError devuelve los nodos que un error de la corrida nombra: cada
// *protocolo.ErrorNodo del árbol de errores, sin repetir y en el orden en que
// aparecen. Una cancelación no nombra a ninguno.
func nodosConError(err error) []string {
	var nombres []string
	visto := map[string]bool{}
	var recorrer func(error)
	recorrer = func(err error) {
		if err == nil {
			return
		}
		if en, ok := err.(*protocolo.ErrorNodo); ok && !visto[en.Nodo] {
			visto[en.Nodo] = true
			nombres = append(nombres, en.Nodo)
		}
		switch x := err.(type) {
		case interface{ Unwrap() []error }:
			for _, e := range x.Unwrap() {
				recorrer(e)
			}
		case interface{ Unwrap() error }:
			recorrer(x.Unwrap())
		}
	}
	recorrer(err)
	return nombres
}

func ms(d time.Duration) float64 { return float64(d.Nanoseconds()) / 1e6 }

// resumenEntrenamiento es EntrenamientoResumen en el contrato. Qué campos trae
// depende del estado: en curso, solo los de identificación; terminado, además
// iteraciones, paro, dif_secuencial y ms_total, y modelo_id si activó su
// modelo o aviso si tenía que activarlo y no lo hizo; abortado, además error.
type resumenEntrenamiento struct {
	ID            string   `json:"id"`
	Estado        string   `json:"estado"`
	Nodos         []string `json:"nodos"`
	Replicas      int      `json:"replicas"`
	Iteraciones   *int     `json:"iteraciones,omitempty"`
	Paro          string   `json:"paro,omitempty"`
	DifSecuencial *float64 `json:"dif_secuencial,omitempty"`
	MsTotal       *float64 `json:"ms_total,omitempty"`
	ModeloID      string   `json:"modelo_id,omitempty"`
	Aviso         string   `json:"aviso,omitempty"`
	Error         string   `json:"error,omitempty"`
}

func resumir(e almacen.Entrenamiento) resumenEntrenamiento {
	r := resumenEntrenamiento{ID: e.ID, Estado: e.Estado, Nodos: make([]string, len(e.Nodos)), Replicas: e.Parametros.Replicas}
	for i, n := range e.Nodos {
		r.Nodos[i] = n.Nombre
	}
	switch e.Estado {
	case almacen.EstadoTerminado:
		r.Iteraciones, r.Paro, r.MsTotal = &e.Iteraciones, e.Paro, &e.MsTotal
		r.ModeloID, r.Aviso = e.ModeloID, e.Aviso
		// Si el secuencial de referencia no se pudo correr, no hay diferencia
		// que informar (y NaN no se puede escribir en JSON).
		if !math.IsNaN(e.DifSecuencial) {
			r.DifSecuencial = &e.DifSecuencial
		}
	case almacen.EstadoAbortado:
		r.Error = e.Error
	}
	return r
}

// GET /api/entrenamientos
func (s *Servidor) listarEntrenamientos(w http.ResponseWriter, r *http.Request) {
	es, err := s.coord.entrenamientos.Listar(r.Context())
	if err != nil {
		s.log.Error("listar entrenamientos", "err", err)
		escribirError(w, http.StatusServiceUnavailable, "MongoDB no disponible")
		return
	}
	out := make([]resumenEntrenamiento, len(es))
	for i, e := range es {
		out[i] = resumir(e)
	}
	s.responder(w, r, http.StatusOK, out)
}

// GET /api/entrenamientos/{id}
func (s *Servidor) consultarEntrenamiento(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	e, err := s.coord.entrenamientos.Obtener(r.Context(), id)
	if err != nil {
		if errors.Is(err, almacen.ErrNoEncontrado) {
			escribirError(w, http.StatusNotFound, "no existe el entrenamiento "+recortar(id, maxEstacion))
			return
		}
		s.log.Error("consultar el entrenamiento", "id", id, "err", err)
		escribirError(w, http.StatusServiceUnavailable, "MongoDB no disponible")
		return
	}
	rondas := e.Rondas
	if rondas == nil {
		rondas = []almacen.Ronda{}
	}
	s.responder(w, r, http.StatusOK, struct {
		resumenEntrenamiento
		Rondas []almacen.Ronda `json:"rondas"`
	}{resumir(e), rondas})
}

// GET /api/cluster/nodos
func (s *Servidor) listarNodos(w http.ResponseWriter, r *http.Request) {
	type nodo struct {
		Nombre     string  `json:"nombre"`
		Direccion  string  `json:"direccion"`
		Workers    int     `json:"workers"`
		LatenciaMs float64 `json:"latencia_ms"`
		Libre      bool    `json:"libre"`
	}
	out := []nodo{}
	if s.pool != nil {
		for _, n := range s.pool.Nodos() {
			out = append(out, nodo{n.Nombre, n.Direccion, n.Workers, ms(n.Latencia), n.Libre})
		}
	}
	s.responder(w, r, http.StatusOK, out)
}
