// Comando api es la API coordinadora de la PC4 (spec §6): REST con JWT,
// predicción del régimen de una lectura en menos de 100 ms, latencia por ruta,
// el contrato OpenAPI y Swagger UI. El clúster, los entrenamientos, MongoDB,
// Redis y el WebSocket llegan después: por ahora el almacén es el doble en
// memoria (almacen/memoria), sembrado con la ficha de estaciones.
//
// Al arrancar siembra el modelo desde reports/kmeans.json, reports/regimenes.json
// y reports/estandarizacion.json, y no arranca si no son de la misma corrida.
// Se corre desde la raíz del repo:
//
//	export JWT_SECRETO=$(openssl rand -hex 32)
//	export API_USUARIOS='ana:una-clave-larga:admin,demo:otra-clave:usuario'
//	go run ./cmd/api                     # http://localhost:8080/docs
//
// Variables de entorno, las dos obligatorias:
//
//   - JWT_SECRETO: la clave HS256, de al menos 32 bytes.
//   - API_USUARIOS: «usuario:clave:rol» separados por comas, con rol usuario o
//     admin.
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
	fs.BoolVar(&op.demo, "demo", false, "sin API_USUARIOS, usar los usuarios de demostración demo/demo y admin/admin (solo desarrollo)")
	if err := fs.Parse(args); err != nil {
		return opciones{}, err
	}
	if fs.NArg() > 0 {
		return opciones{}, fmt.Errorf("argumentos de más: %v", fs.Args())
	}
	return op, nil
}

// armar lee la configuración, siembra el modelo y las estaciones y crea el
// servidor. Todo lo que puede fallar falla aquí, antes de escuchar.
func armar(op opciones, getenv func(string) string, log *slog.Logger) (*api.Servidor, error) {
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
	cache := memoria.NuevaCache()
	srv, err := api.Nuevo(api.Config{
		Modelo:       m,
		Firmador:     firmador,
		Usuarios:     usuarios,
		Catalogo:     ficha,
		Estaciones:   estaciones,
		Predicciones: memoria.NuevasPredicciones(),
		Cache:        cache,
		Mongo:        estaciones,
		Redis:        cache,
		OpenAPI:      contrato,
		TamCola:      op.cola,
		Log:          log,
	})
	if err != nil {
		return nil, err
	}
	log.Info("modelo sembrado", "id", m.ID, "k", m.K(), "features", m.Features(), "iteraciones", m.Iteraciones)
	log.Info("almacén en memoria (sin MongoDB ni Redis)", "estaciones", len(ficha))
	return srv, nil
}

// correr levanta la API hasta que ctx se cancela; entonces deja de aceptar
// pedidos, espera a los que están en curso y vacía la cola de escritura.
// escuchando, si no es nil, recibe la dirección real (útil con -addr :0).
func correr(ctx context.Context, args []string, getenv func(string) string, salida io.Writer, escuchando func(string)) error {
	op, err := leerOpciones(args)
	if err != nil {
		return err
	}
	log := slog.New(slog.NewTextHandler(salida, nil))
	srv, err := armar(op, getenv, log)
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
	if escuchando != nil {
		escuchando(ln.Addr().String())
	}
	select {
	case err := <-sirviendo:
		return err
	case <-ctx.Done():
	}
	log.Info("apagando: se terminan los pedidos en curso y se vacía la cola de escritura")
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
