// Comando api es la API coordinadora de la PC4 (spec §6): REST con JWT,
// predicción del régimen de una lectura en menos de 100 ms, latencia por ruta,
// el contrato OpenAPI y Swagger UI; y el coordinador del clúster (§5.2): los
// nodos (cmd/nodo) la llaman por TCP en -nodos-addr, quedan en su pool, y un
// admin lanza un entrenamiento distribuido por HTTP y lo sigue por WebSocket.
// MongoDB y Redis llegan después: por ahora el almacén es el doble en memoria
// (almacen/memoria), sembrado con la ficha de estaciones.
//
// Al arrancar siembra el modelo desde reports/kmeans.json, reports/regimenes.json
// y reports/estandarizacion.json, y no arranca si no son de la misma corrida.
// Se corre desde la raíz del repo:
//
//	export JWT_SECRETO=$(openssl rand -hex 32)
//	export API_USUARIOS='ana:una-clave-larga:admin,demo:otra-clave:usuario'
//	go run ./cmd/api                     # http://localhost:8080/docs
//
// Variables de entorno; las dos primeras son obligatorias:
//
//   - JWT_SECRETO: la clave HS256, de al menos 32 bytes.
//   - API_USUARIOS: «usuario:clave:rol» separados por comas, con rol usuario o
//     admin.
//   - CORS_ORIGENES: los orígenes que pueden llamar a la API desde un
//     navegador, separados por comas («http://localhost:5173»), o «*». Sin
//     ella, solo el mismo origen (Swagger UI). La necesita la SPA del TB2.
//
// Para entrenar hace falta el gold (-gold), el mismo archivo que tienen los
// nodos: de él salen la huella y el tamaño de la tabla que se reparte, la
// comparación con el K-Means secuencial y el perfil del modelo nuevo. Si no
// está, la API arranca igual, con un aviso: predice y recibe nodos, pero
// POST /api/entrenamientos responde 503.
//
//	go run ./cmd/nodo -coordinador localhost:7070 -nombre nodo-1   # en otra terminal
//
// Para desarrollo, -demo arranca sin API_USUARIOS con los usuarios de
// demostración demo/demo (usuario) y admin/admin (admin), con un aviso en el
// log. No es para una API que se pueda alcanzar desde otra máquina.
//
// Con -salud no levanta la API: pide /api/salud a la que ya corre en -addr y
// sale con 0 si responde. Es el healthcheck de Docker (spec §8), porque la
// imagen distroless no tiene curl.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/R0SEWT/aire-distribuido/almacen"
	"github.com/R0SEWT/aire-distribuido/almacen/memoria"
	"github.com/R0SEWT/aire-distribuido/api"
	"github.com/R0SEWT/aire-distribuido/cluster"
	"github.com/R0SEWT/aire-distribuido/modelo"
)

const usuariosDemo = "demo:demo:usuario,admin:admin:admin"

type opciones struct {
	addr                               string
	kmeans, regimenes, estandarizacion string
	estaciones, openapi                string
	duracionJWT                        time.Duration
	cola                               int
	salud, demo                        bool

	// El coordinador del clúster.
	nodosAddr             string // "" = sin clúster
	gold, receta, inicios string
	latido, plazoHola     time.Duration
	plazoCarga            time.Duration
	plazoRonda            time.Duration
	maxFilas              int
}

func main() {
	op, err := leerOpciones(os.Args[1:])
	if err != nil {
		os.Exit(2)
	}
	if op.salud {
		if err := consultarSalud(urlSalud(op.addr)); err != nil {
			fmt.Fprintln(os.Stderr, "api -salud:", err)
			os.Exit(1)
		}
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := correr(ctx, os.Args[1:], os.Getenv, os.Stderr, nil); err != nil {
		fmt.Fprintln(os.Stderr, "api:", err)
		os.Exit(1)
	}
}

func leerOpciones(args []string) (opciones, error) {
	var op opciones
	fs := flag.NewFlagSet("api", flag.ContinueOnError)
	fs.StringVar(&op.addr, "addr", ":8080", "dirección HTTP")
	fs.StringVar(&op.kmeans, "kmeans", "reports/kmeans.json", "modelo entrenado (cmd/entrenar)")
	fs.StringVar(&op.regimenes, "regimenes", "reports/regimenes.json", "perfil de los regímenes (cmd/entrenar)")
	fs.StringVar(&op.estandarizacion, "estandarizacion", "reports/estandarizacion.json", "media y desviación del gold (cmd/cargar)")
	fs.StringVar(&op.estaciones, "estaciones", "reports/estaciones_ficha.csv", "ficha de estaciones")
	fs.StringVar(&op.openapi, "openapi", "docs/api/openapi.yaml", "contrato que se sirve en /openapi.yaml")
	fs.DurationVar(&op.duracionJWT, "jwt-duracion", time.Hour, "validez de un token")
	fs.IntVar(&op.cola, "cola", 1024, "búfer de la cola de escritura de predicciones")
	fs.BoolVar(&op.salud, "salud", false, "consultar /api/salud de la API en -addr y salir con 0 o 1")
	fs.StringVar(&op.nodosAddr, "nodos-addr", ":7070", "dirección TCP donde llaman los nodos del clúster; \"\" = sin clúster")
	fs.StringVar(&op.gold, "gold", "data/gold/oefa_features.csv", "gold con que se entrena, el mismo de los nodos; si no existe, la API no entrena")
	fs.StringVar(&op.receta, "receta", "configs/kmeans.json", "receta del K-Means de un entrenamiento")
	fs.StringVar(&op.inicios, "inicios", "reports/centroides_iniciales.json", "centroides iniciales guardados por semilla; con otra semilla se calculan con k-means++")
	fs.DurationVar(&op.latido, "latido", 5*time.Second, "cada cuánto se les toma el pulso a los nodos libres")
	fs.DurationVar(&op.plazoHola, "plazo-hola", 5*time.Second, "lo que un nodo tiene para presentarse después de conectar")
	fs.DurationVar(&op.plazoCarga, "plazo-carga", 30*time.Second, "lo que los nodos tienen para cargar su rango del gold")
	fs.DurationVar(&op.plazoRonda, "plazo-ronda", 10*time.Second, "plazo común de cada ronda de un entrenamiento")
	fs.IntVar(&op.maxFilas, "max-filas", cluster.MaxFilasPorOmision, "tope de filas por nodo, el -max-filas de los nodos: acota replicas")
	fs.BoolVar(&op.demo, "demo", false, "sin API_USUARIOS, usar los usuarios de demostración demo/demo y admin/admin (solo desarrollo)")
	if err := fs.Parse(args); err != nil {
		return opciones{}, err
	}
	if fs.NArg() > 0 {
		return opciones{}, fmt.Errorf("argumentos de más: %v", fs.Args())
	}
	if op.latido <= 0 || op.plazoHola <= 0 || op.plazoCarga <= 0 || op.plazoRonda <= 0 {
		return opciones{}, errors.New("los plazos del clúster tienen que ser positivos")
	}
	if op.maxFilas <= 0 {
		return opciones{}, fmt.Errorf("-max-filas tiene que ser positivo, es %d", op.maxFilas)
	}
	return op, nil
}

// armar lee la configuración, siembra el modelo y las estaciones y crea el
// servidor. Todo lo que puede fallar falla aquí, antes de escuchar. nodos es el
// listener donde llaman los nodos del clúster, o nil si la API no tiene clúster.
func armar(op opciones, getenv func(string) string, log *slog.Logger, nodos net.Listener) (*api.Servidor, error) {
	secreto := getenv("JWT_SECRETO")
	if secreto == "" {
		return nil, errors.New("falta la variable de entorno JWT_SECRETO (por ejemplo: export JWT_SECRETO=$(openssl rand -hex 32))")
	}
	firmador, err := api.NuevoFirmador([]byte(secreto), op.duracionJWT)
	if err != nil {
		return nil, err
	}
	listaUsuarios := getenv("API_USUARIOS")
	switch {
	case listaUsuarios != "" && op.demo:
		log.Warn("-demo no tiene efecto: API_USUARIOS está definida")
	case listaUsuarios == "" && op.demo:
		log.Warn("-demo: usuarios de demostración demo/demo (usuario) y admin/admin (admin); solo para desarrollo")
		listaUsuarios = usuariosDemo
	case listaUsuarios == "":
		return nil, errors.New("falta la variable de entorno API_USUARIOS (usuario:clave:rol separados por comas; " +
			"para desarrollo, -demo usa los usuarios de demostración)")
	}
	usuarios, err := api.LeerUsuarios(listaUsuarios)
	if err != nil {
		return nil, err
	}
	m, err := modelo.Sembrar(modelo.Archivos{Kmeans: op.kmeans, Regimenes: op.regimenes, Estandarizacion: op.estandarizacion})
	if err != nil {
		return nil, err
	}
	f, err := os.Open(op.estaciones)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	ficha, err := almacen.LeerFichaEstaciones(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op.estaciones, err)
	}
	estaciones, err := memoria.NuevasEstaciones(ficha)
	if err != nil {
		return nil, err
	}
	contrato, err := os.ReadFile(op.openapi)
	if err != nil {
		return nil, err
	}
	origenes, err := api.LeerOrigenesCORS(getenv("CORS_ORIGENES"))
	if err != nil {
		return nil, err
	}
	entrenador, err := leerEntrenador(op, log)
	if err != nil {
		return nil, err
	}
	cache := memoria.NuevaCache()
	srv, err := api.Nuevo(api.Config{
		Entrenamientos: memoria.NuevosEntrenamientos(),
		Eventos:        memoria.NuevosEventos(),
		Nodos:          nodos,
		Pool:           cluster.OpcionesPool{PlazoHola: op.plazoHola, CadaLatido: op.latido},
		Entrenador:     entrenador,
		OrigenesCORS:   origenes,
		Modelo:         m,
		Firmador:       firmador,
		Usuarios:       usuarios,
		Catalogo:       ficha,
		Estaciones:     estaciones,
		Predicciones:   memoria.NuevasPredicciones(),
		Cache:          cache,
		Mongo:          estaciones,
		Redis:          cache,
		OpenAPI:        contrato,
		TamCola:        op.cola,
		Log:            log,
	})
	if err != nil {
		return nil, err
	}
	log.Info("modelo sembrado", "id", m.ID, "k", m.K(), "features", m.Features(), "iteraciones", m.Iteraciones)
	log.Info("almacén en memoria (sin MongoDB ni Redis)", "estaciones", len(ficha))
	if len(origenes) > 0 {
		log.Info("CORS", "origenes", origenes)
	}
	return srv, nil
}

// leerEntrenador carga lo que hace falta para entrenar. Sin gold (-gold "" o
// un archivo que no existe) devuelve nil con un aviso: la API arranca y
// predice, pero no entrena. Un gold que existe y no cuadra con la receta o con
// la estandarización sí es un error: mejor no arrancar que entrenar con otra tabla.
func leerEntrenador(op opciones, log *slog.Logger) (*api.Entrenador, error) {
	if op.gold == "" {
		log.Warn("sin gold (-gold \"\"): la API no puede entrenar")
		return nil, nil
	}
	e, err := api.LeerEntrenador(op.receta, op.estandarizacion, op.gold, op.inicios)
	if errors.Is(err, fs.ErrNotExist) {
		log.Warn("la API no puede entrenar: falta un archivo (el gold no se versiona: cópialo o indícalo con -gold)", "err", err)
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("lo necesario para entrenar: %w", err)
	}
	e.PlazoCarga, e.PlazoRonda, e.MaxFilas = op.plazoCarga, op.plazoRonda, op.maxFilas
	log.Info("gold para entrenar", "ruta", op.gold, "filas", e.Gold.Datos.N, "features", e.Receta.Features, "k", e.Receta.K,
		"sha256", e.Sha256[:12], "centroides_iniciales_guardados", e.Inicios != nil)
	if e.Receta.Chunk != cluster.ChunkPorOmision {
		log.Warn("la receta usa otro chunk que el que los nodos usan por omisión: lánzalos con -chunk para que un nodo dé el modelo de cmd/entrenar bit a bit",
			"receta", e.Receta.Chunk, "nodos", cluster.ChunkPorOmision)
	}
	return e, nil
}

// correr levanta la API hasta que ctx se cancela; entonces corta el
// entrenamiento en curso, despide a los nodos y a los clientes del WebSocket,
// deja de aceptar pedidos, espera a los que están en curso y vacía la cola de
// escritura. escuchando, si no es nil, recibe las direcciones reales de HTTP y
// de los nodos (útil con :0); la de los nodos es "" si no hay clúster.
func correr(ctx context.Context, args []string, getenv func(string) string, salida io.Writer, escuchando func(http, nodos string)) error {
	op, err := leerOpciones(args)
	if err != nil {
		return err
	}
	log := slog.New(slog.NewTextHandler(salida, nil))
	var lnNodos net.Listener
	dirNodos := ""
	if op.nodosAddr != "" {
		if lnNodos, err = net.Listen("tcp", op.nodosAddr); err != nil {
			return fmt.Errorf("escuchar a los nodos: %w", err)
		}
		defer lnNodos.Close()
		dirNodos = lnNodos.Addr().String()
	}
	srv, err := armar(op, getenv, log, lnNodos)
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", op.addr)
	if err != nil {
		return err
	}
	ctxEscritor, pararEscritor := context.WithCancel(context.Background())
	escritorListo := make(chan struct{})
	go func() { srv.Escribir(ctxEscritor); close(escritorListo) }()
	defer func() { pararEscritor(); <-escritorListo }()
	// El coordinador (hub del WebSocket, pool de nodos y eventos) se apaga
	// antes que el servidor HTTP: Shutdown no espera a las conexiones que ya
	// son WebSocket, y un entrenamiento en curso tiene que quedar abortado.
	ctxCoordinador, pararCoordinador := context.WithCancel(context.Background())
	coordinadorListo := make(chan struct{})
	go func() { srv.Coordinar(ctxCoordinador); close(coordinadorListo) }()
	defer func() { pararCoordinador(); <-coordinadorListo }()

	hs := &http.Server{
		Handler:           srv,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       2 * time.Minute,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}
	sirviendo := make(chan error, 1)
	go func() { sirviendo <- hs.Serve(ln) }()
	log.Info("API escuchando", "addr", ln.Addr().String(), "docs", "/docs")
	if lnNodos != nil {
		log.Info("clúster: los nodos llaman aquí", "addr", dirNodos, "ejemplo", "nodo -coordinador <IP>:<puerto>")
	} else {
		log.Warn("sin clúster (-nodos-addr \"\"): no entran nodos y no se puede entrenar")
	}
	if escuchando != nil {
		escuchando(ln.Addr().String(), dirNodos)
	}
	select {
	case err := <-sirviendo:
		return err
	case <-ctx.Done():
	}
	log.Info("apagando: se corta el entrenamiento en curso, se terminan los pedidos y se vacía la cola de escritura")
	pararCoordinador()
	<-coordinadorListo
	apagar, cancelar := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelar()
	if err := hs.Shutdown(apagar); err != nil {
		return err
	}
	if err := <-sirviendo; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// urlSalud arma la URL de /api/salud de la API que escucha en addr. Una
// dirección sin host o comodín (:8080, 0.0.0.0, [::]) se consulta en 127.0.0.1.
func urlSalud(addr string) string {
	host, puerto, err := net.SplitHostPort(addr)
	if err != nil {
		host, puerto = "", addr
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, puerto) + "/api/salud"
}

// consultarSalud pide /api/salud y falla si no responde 200 con api = ok.
func consultarSalud(url string) error {
	cliente := http.Client{Timeout: 3 * time.Second}
	resp, err := cliente.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var s struct {
		API string `json:"api"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil || resp.StatusCode != http.StatusOK || s.API != "ok" {
		return fmt.Errorf("%s respondió %d (api = %q, %v)", url, resp.StatusCode, s.API, err)
	}
	return nil
}
