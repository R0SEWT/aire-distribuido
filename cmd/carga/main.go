// Comando carga mide la latencia de POST /api/predicciones del lado del
// cliente, con varios niveles de clientes concurrentes, y deja el resultado en
// reports/api_latencia.json (spec de la PC4, §6.2 y §9). Es la evidencia de que
// la API responde en menos de 100 ms: /api/metricas mide dentro del servidor;
// esto mide lo que ve quien llama, con la red, el JWT y el JSON incluidos.
//
//	CARGA_CLAVE=… go run ./cmd/carga                      # contra http://localhost:8080
//	CARGA_CLAVE=… go run ./cmd/carga -url http://localhost:18080 -clientes 1,4,16,64 -pedidos 5000
//
// Cómo mide:
//
//   - Pide un token con -usuario y la clave (CARGA_CLAVE o -clave) y lo usa en
//     todos los pedidos. Cualquier rol puede predecir.
//
//   - Manda las lecturas de -lecturas (los datos de prueba del Runner de
//     Postman) en orden y en ronda. Repetirlas no cambia el estado de la API:
//     la misma estación y hora reemplaza a la anterior.
//
//   - Por cada nivel, C goroutines cliente toman turnos de un canal: siempre
//     hay C pedidos en vuelo. Cada una guarda sus latencias en su propia lista
//     y se juntan al final; no comparten nada más que el http.Client.
//
//   - La latencia de un pedido va desde antes de enviarlo hasta tener el cuerpo
//     completo de la respuesta. Antes de medir, cada nivel hace -calentamiento
//     pedidos que se descartan: abren las conexiones, que después se reutilizan
//     (keep-alive), así que el saludo TCP no entra en la medida.
//
//   - Los percentiles son por rango más cercano sobre los pedidos que
//     respondieron 200. Los demás se cuentan como errores, por tipo. También
//     se cuenta la respuesta 200 cuyo régimen no es el esperado del archivo.
//
//   - La API responde 200 antes de escribir la predicción en MongoDB y en
//     Redis, y si una cola se llena la descarta: eso no se ve en la latencia.
//     Con la clave de un admin en CARGA_CLAVE_ADMIN (y -admin, si no se llama
//     admin), lee GET /api/metricas antes y después y deja en el informe
//     cuántas predicciones se escribieron, se descartaron y fallaron en cada
//     almacén durante la corrida. Sin esa clave lo dice y mide solo la latencia.
//
// Sale con 1 si algún nivel no cumple: tuvo errores, alguna respuesta con otro
// régimen, o un p99 de -limite-ms o más. El informe se escribe igual. Las
// escrituras perdidas no cambian ese resultado, que es de los pedidos: salen
// aparte, con un AVISO en pantalla y en «escrituras» del informe.
//
// Se corre desde la raíz del repo, con la API levantada (docker compose up).
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/R0SEWT/aire-distribuido/kmeans"
)

type config struct {
	url            string
	usuario, clave string
	lecturas       string
	clientes       []int
	pedidos        int
	calentamiento  int
	limiteMs       int
	plazo          time.Duration
	salida         string
	// Para leer /api/metricas: si claveAdmin está vacía, no se miden las escrituras.
	admin, claveAdmin string
	esperaEscrituras  time.Duration
}

func leerConfig(args []string, getenv func(string) string) (config, error) {
	var c config
	var clientes string
	fs := flag.NewFlagSet("carga", flag.ContinueOnError)
	fs.StringVar(&c.url, "url", "http://localhost:8080", "dirección de la API")
	fs.StringVar(&c.usuario, "usuario", "demo", "usuario con que se pide el token")
	fs.StringVar(&c.clave, "clave", "", "clave del usuario; mejor en la variable CARGA_CLAVE, que no queda en el historial")
	fs.StringVar(&c.lecturas, "lecturas", filepath.Join("docs", "postman", "lecturas_prueba.csv"), "CSV con las lecturas que se mandan")
	fs.StringVar(&clientes, "clientes", "1,4,16,64", "niveles de clientes concurrentes, separados por comas")
	fs.IntVar(&c.pedidos, "pedidos", 5000, "pedidos medidos por nivel")
	fs.IntVar(&c.calentamiento, "calentamiento", 200, "pedidos previos por nivel, que no se miden")
	fs.IntVar(&c.limiteMs, "limite-ms", 100, "el p99 de cada nivel tiene que quedar por debajo")
	fs.DurationVar(&c.plazo, "plazo", 5*time.Second, "plazo de cada pedido")
	fs.StringVar(&c.salida, "salida", filepath.Join("reports", "api_latencia.json"), "dónde se escribe el informe")
	fs.StringVar(&c.admin, "admin", "admin", "usuario de rol admin con que se lee /api/metricas; su clave va en la variable CARGA_CLAVE_ADMIN")
	fs.DurationVar(&c.esperaEscrituras, "espera-escrituras", 15*time.Second, "cuánto esperar, al terminar, a que MongoDB y Redis den cuenta de las predicciones")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	if fs.NArg() > 0 {
		return config{}, fmt.Errorf("argumentos de más: %v", fs.Args())
	}
	var err error
	if c.clientes, err = enteros(clientes); err != nil {
		return config{}, fmt.Errorf("-clientes: %w", err)
	}
	if u, err := url.Parse(c.url); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return config{}, fmt.Errorf("-url %q tiene que ser como http://localhost:8080", c.url)
	}
	c.url = strings.TrimRight(c.url, "/")
	switch {
	case c.pedidos < 1:
		return config{}, errors.New("-pedidos tiene que ser al menos 1")
	case c.calentamiento < 0:
		return config{}, errors.New("-calentamiento no puede ser negativo")
	case c.limiteMs < 1:
		return config{}, errors.New("-limite-ms tiene que ser al menos 1")
	case c.plazo <= 0:
		return config{}, errors.New("-plazo tiene que ser positivo")
	case c.esperaEscrituras <= 0:
		return config{}, errors.New("-espera-escrituras tiene que ser positivo")
	}
	c.claveAdmin = getenv("CARGA_CLAVE_ADMIN")
	if c.clave == "" {
		c.clave = getenv("CARGA_CLAVE")
	}
	if c.clave == "" {
		return config{}, errors.New("falta la clave: variable CARGA_CLAVE o -clave")
	}
	return c, nil
}

// informe es reports/api_latencia.json.
type informe struct {
	Metadatos  kmeans.Metadatos `json:"metadatos"`
	Parametros parametros       `json:"parametros"`
	// Servidor es lo que respondió GET /api/salud antes de medir: contra qué
	// se midió (almacenes, nodos en el pool y modelo activo).
	Servidor json.RawMessage `json:"servidor"`
	Medicion string          `json:"medicion"`
	Niveles  []nivel         `json:"niveles"`
	Cumple   bool            `json:"cumple"`
	// Escrituras es lo que pasó en MongoDB y en Redis con las predicciones de
	// la corrida. No entra en Cumple, que es de los pedidos.
	Escrituras escrituras `json:"escrituras"`
}

type parametros struct {
	URL             string `json:"url"`
	Ruta            string `json:"ruta"`
	Lecturas        string `json:"lecturas"`
	LecturasSha256  string `json:"lecturas_sha256"`
	Filas           int    `json:"filas"`
	Clientes        []int  `json:"clientes"`
	PedidosPorNivel int    `json:"pedidos_por_nivel"`
	Calentamiento   int    `json:"calentamiento"`
	LimiteMs        int    `json:"limite_ms"`
	PlazoPedido     string `json:"plazo_pedido"`
}

const notaMedicion = "Latencia del lado del cliente: desde antes de enviar el pedido hasta leer el cuerpo completo de la respuesta, " +
	"con un JWT y sobre conexiones keep-alive ya abiertas por el calentamiento. Cada nivel tiene siempre `clientes` pedidos en vuelo. " +
	"Percentiles por rango más cercano sobre los pedidos que respondieron 200; pedidos_por_segundo son esos pedidos sobre duracion_s. " +
	"respuestas_distintas: respondieron 200 con un régimen que no es el regimen_esperado del archivo. " +
	"cumple: sin errores, sin respuestas distintas y p99_ms < limite_ms. " +
	"escrituras: lo que GET /api/metricas sumó en cada almacén entre antes y después de la corrida, con el calentamiento; la API responde " +
	"antes de escribir, así que una predicción descartada o fallida respondió 200 igual. No entra en cumple."

const motivoSinAdmin = "falta la clave de un usuario admin en la variable CARGA_CLAVE_ADMIN: GET /api/metricas es de rol admin"

func correr(ctx context.Context, cfg config, pantalla io.Writer) (informe, error) {
	f, err := os.Open(cfg.lecturas)
	if err != nil {
		return informe{}, err
	}
	lecturas, err := leerLecturas(f)
	f.Close()
	if err != nil {
		return informe{}, fmt.Errorf("%s: %w", cfg.lecturas, err)
	}
	sha, err := kmeans.Sha256Archivo(cfg.lecturas)
	if err != nil {
		return informe{}, err
	}

	hc := &http.Client{Timeout: cfg.plazo}
	salud, err := pedirSalud(ctx, hc, cfg.url)
	if err != nil {
		return informe{}, err
	}
	token, err := pedirToken(ctx, hc, cfg.url, cfg.usuario, cfg.clave)
	if err != nil {
		return informe{}, err
	}
	// Antes de medir: si se pidió contar las escrituras y no se puede, se
	// sabe ahora y no después de la corrida.
	var tokenAdmin string
	var antes fotoEscritura
	if cfg.claveAdmin != "" {
		if tokenAdmin, err = pedirToken(ctx, hc, cfg.url, cfg.admin, cfg.claveAdmin); err != nil {
			return informe{}, err
		}
		if antes, err = pedirEscritura(ctx, hc, cfg.url, tokenAdmin); err != nil {
			return informe{}, err
		}
	}

	inf := informe{
		Metadatos: kmeans.RecolectarMetadatos(),
		Parametros: parametros{
			URL: cfg.url, Ruta: "POST " + rutaPrediccion, Lecturas: filepath.ToSlash(cfg.lecturas), LecturasSha256: sha,
			Filas: len(lecturas), Clientes: cfg.clientes, PedidosPorNivel: cfg.pedidos, Calentamiento: cfg.calentamiento,
			LimiteMs: cfg.limiteMs, PlazoPedido: cfg.plazo.String(),
		},
		Servidor: salud,
		Medicion: notaMedicion,
		Cumple:   true,
	}
	fmt.Fprintf(pantalla, "POST %s%s · %d lecturas de %s · %d pedidos por nivel (+%d de calentamiento) · límite %d ms\n",
		cfg.url, rutaPrediccion, len(lecturas), cfg.lecturas, cfg.pedidos, cfg.calentamiento, cfg.limiteMs)
	fmt.Fprintf(pantalla, "servidor: %s\n", salud)
	var almacenes struct{ Mongo, Redis string }
	if json.Unmarshal(salud, &almacenes) == nil && (almacenes.Mongo != "ok" || almacenes.Redis != "ok") {
		fmt.Fprintf(pantalla, "AVISO: /api/salud dice mongo = %q y redis = %q: las predicciones van a responder, pero no todas se van a guardar\n",
			almacenes.Mongo, almacenes.Redis)
	}
	fmt.Fprintln(pantalla)

	c := nuevoCliente(cfg.url, token, cfg.plazo)
	defer c.http.CloseIdleConnections()
	aceptadas := 0
	for _, clientes := range cfg.clientes {
		n, err := medirNivel(ctx, c, lecturas, clientes, cfg.pedidos, cfg.calentamiento, cfg.limiteMs)
		if err != nil {
			return informe{}, err
		}
		inf.Niveles = append(inf.Niveles, n)
		inf.Cumple = inf.Cumple && n.Cumple
		aceptadas += n.aceptadas
	}
	imprimirTabla(pantalla, inf)

	inf.Escrituras = escrituras{Motivo: motivoSinAdmin}
	if tokenAdmin != "" {
		if inf.Escrituras, err = esperarEscrituras(ctx, hc, cfg.url, tokenAdmin, antes, aceptadas, cfg.esperaEscrituras); err != nil {
			return informe{}, err
		}
	}
	imprimirEscrituras(pantalla, inf.Escrituras)

	if err := os.MkdirAll(filepath.Dir(cfg.salida), 0o755); err != nil {
		return informe{}, err
	}
	crudo, err := json.MarshalIndent(inf, "", "  ")
	if err != nil {
		return informe{}, err
	}
	if err := os.WriteFile(cfg.salida, append(crudo, '\n'), 0o644); err != nil {
		return informe{}, err
	}
	fmt.Fprintf(pantalla, "\ninforme: %s\n", cfg.salida)
	return inf, nil
}

func imprimirTabla(w io.Writer, inf informe) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', tabwriter.AlignRight)
	fmt.Fprintln(tw, "clientes\tpedidos\terrores\tdistintas\tp50 ms\tp95 ms\tp99 ms\tmáx ms\tpedidos/s\t< límite\t")
	for _, n := range inf.Niveles {
		cumple := "sí"
		if !n.Cumple {
			cumple = "NO"
		}
		fmt.Fprintf(tw, "%d\t%d\t%d\t%d\t%.3f\t%.3f\t%.3f\t%.3f\t%.0f\t%s\t\n",
			n.Clientes, n.Pedidos, n.Errores, n.RespuestasDistintas, n.P50Ms, n.P95Ms, n.P99Ms, n.MaxMs, n.PedidosPorSegundo, cumple)
	}
	tw.Flush()
	for _, n := range inf.Niveles {
		if len(n.ErroresPorTipo) > 0 {
			fmt.Fprintf(w, "errores con %d clientes: %v\n", n.Clientes, n.ErroresPorTipo)
		}
	}
}

// pedirSalud devuelve el cuerpo de GET /api/salud, que es una ruta libre.
func pedirSalud(ctx context.Context, hc *http.Client, base string) (json.RawMessage, error) {
	pedido, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/salud", nil)
	if err != nil {
		return nil, err
	}
	resp, err := hc.Do(pedido)
	if err != nil {
		return nil, fmt.Errorf("la API no responde en %s: %w", base, err)
	}
	defer resp.Body.Close()
	cuerpo, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK || !json.Valid(cuerpo) {
		return nil, fmt.Errorf("GET /api/salud respondió %d", resp.StatusCode)
	}
	var compacto bytes.Buffer
	if err := json.Compact(&compacto, cuerpo); err != nil {
		return nil, err
	}
	return json.RawMessage(compacto.Bytes()), nil
}

func pedirToken(ctx context.Context, hc *http.Client, base, usuario, clave string) (string, error) {
	cuerpo, err := json.Marshal(map[string]string{"usuario": usuario, "clave": clave})
	if err != nil {
		return "", err
	}
	pedido, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/auth/token", bytes.NewReader(cuerpo))
	if err != nil {
		return "", err
	}
	pedido.Header.Set("Content-Type", "application/json")
	resp, err := hc.Do(pedido)
	if err != nil {
		return "", fmt.Errorf("pidiendo el token: %w", err)
	}
	defer resp.Body.Close()
	var r struct {
		Token string `json:"token"`
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("POST /api/auth/token respondió %d para el usuario %s: revisa el usuario y su clave (CARGA_CLAVE, CARGA_CLAVE_ADMIN)", resp.StatusCode, usuario)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&r); err != nil || r.Token == "" {
		return "", errors.New("POST /api/auth/token no devolvió un token")
	}
	return r.Token, nil
}

func main() {
	cfg, err := leerConfig(os.Args[1:], os.Getenv)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, "carga:", err)
		os.Exit(2)
	}
	ctx, parar := signal.NotifyContext(context.Background(), os.Interrupt)
	defer parar()
	inf, err := correr(ctx, cfg, os.Stdout)
	if err != nil {
		fmt.Fprintln(os.Stderr, "carga:", err)
		os.Exit(1)
	}
	if !inf.Cumple {
		fmt.Fprintf(os.Stderr, "carga: algún nivel no cumple (errores, respuestas distintas o p99 ≥ %d ms)\n", cfg.limiteMs)
		os.Exit(1)
	}
}
