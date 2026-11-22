package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

const rutaPrediccion = "/api/predicciones"

// lectura es un pedido listo para enviar y el régimen que tiene que volver.
type lectura struct {
	cuerpo  []byte // el JSON de la Lectura del contrato
	regimen int
}

// leerLecturas lee los datos de prueba: un CSV con encabezado y, al menos, las
// columnas estacion, fecha_hora, pm2_5, pm10 y regimen_esperado (las demás, las
// que usa el Runner de Postman, se ignoran). Arma el cuerpo de cada pedido una
// sola vez, para que codificar el JSON no entre en lo que se mide.
func leerLecturas(r io.Reader) ([]lectura, error) {
	lector := csv.NewReader(r)
	encabezado, err := lector.Read()
	if err != nil {
		return nil, fmt.Errorf("leyendo el encabezado: %w", err)
	}
	col := map[string]int{}
	for i, nombre := range encabezado {
		col[strings.TrimSpace(nombre)] = i
	}
	for _, nombre := range []string{"estacion", "fecha_hora", "pm2_5", "pm10", "regimen_esperado"} {
		if _, ok := col[nombre]; !ok {
			return nil, fmt.Errorf("falta la columna %s", nombre)
		}
	}
	var out []lectura
	for fila := 2; ; fila++ {
		campos, err := lector.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("fila %d: %w", fila, err)
		}
		estacion, fecha := campos[col["estacion"]], campos[col["fecha_hora"]]
		if estacion == "" {
			return nil, fmt.Errorf("fila %d: estacion vacía", fila)
		}
		if _, err := time.Parse(time.RFC3339, fecha); err != nil {
			return nil, fmt.Errorf("fila %d: fecha_hora %q no es RFC3339", fila, fecha)
		}
		pm25, err := strconv.ParseFloat(campos[col["pm2_5"]], 64)
		if err != nil {
			return nil, fmt.Errorf("fila %d: pm2_5 %q no es un número", fila, campos[col["pm2_5"]])
		}
		pm10, err := strconv.ParseFloat(campos[col["pm10"]], 64)
		if err != nil {
			return nil, fmt.Errorf("fila %d: pm10 %q no es un número", fila, campos[col["pm10"]])
		}
		regimen, err := strconv.Atoi(campos[col["regimen_esperado"]])
		if err != nil {
			return nil, fmt.Errorf("fila %d: regimen_esperado %q no es un entero", fila, campos[col["regimen_esperado"]])
		}
		cuerpo, err := json.Marshal(struct {
			Estacion  string  `json:"estacion"`
			FechaHora string  `json:"fecha_hora"`
			PM25      float64 `json:"pm2_5"`
			PM10      float64 `json:"pm10"`
		}{estacion, fecha, pm25, pm10})
		if err != nil {
			return nil, fmt.Errorf("fila %d: %w", fila, err)
		}
		out = append(out, lectura{cuerpo: cuerpo, regimen: regimen})
	}
	if len(out) == 0 {
		return nil, errors.New("no hay ninguna lectura")
	}
	return out, nil
}

// cliente es lo que comparten las goroutines de un nivel: la dirección, el
// token y un http.Client. El http.Client es seguro para uso concurrente y
// reutiliza las conexiones (keep-alive): cada goroutine termina hablando por
// la suya.
type cliente struct {
	url   string
	token string
	http  *http.Client
}

func nuevoCliente(url, token string, plazo time.Duration) *cliente {
	// El transporte por omisión guarda solo 2 conexiones libres por host: con
	// 64 clientes, las demás se cerrarían y se volverían a abrir en cada pedido,
	// y se mediría el saludo TCP en vez de la API.
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.MaxIdleConns = 0 // sin tope global
	tr.MaxIdleConnsPerHost = 1024
	return &cliente{url: strings.TrimRight(url, "/"), token: token, http: &http.Client{Transport: tr, Timeout: plazo}}
}

// resultado es lo que una goroutine cliente midió. Cada una escribe en el
// suyo, así que no hay nada compartido que proteger; se juntan al final.
type resultado struct {
	ms        []float64      // latencia de cada pedido que respondió 200
	errores   map[string]int // por tipo: «http 503», «red»
	distintas int            // respondió 200 con un régimen que no es el esperado
}

// predecir manda una lectura y mide, del lado del cliente, desde antes de
// enviar el pedido hasta tener el cuerpo completo de la respuesta. Decodificar
// el JSON para comparar el régimen queda fuera de la medida.
func (c *cliente) predecir(ctx context.Context, l lectura, res *resultado) {
	inicio := time.Now()
	pedido, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url+rutaPrediccion, bytes.NewReader(l.cuerpo))
	if err != nil {
		res.errores["pedido"]++
		return
	}
	pedido.Header.Set("Content-Type", "application/json")
	pedido.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.http.Do(pedido)
	if err != nil {
		res.errores["red"]++
		return
	}
	cuerpo, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	transcurrido := time.Since(inicio)
	if err != nil {
		res.errores["red"]++
		return
	}
	if resp.StatusCode != http.StatusOK {
		res.errores[fmt.Sprintf("http %d", resp.StatusCode)]++
		return
	}
	res.ms = append(res.ms, float64(transcurrido.Nanoseconds())/1e6)
	var p struct {
		Regimen *int `json:"regimen"`
	}
	if err := json.Unmarshal(cuerpo, &p); err != nil || p.Regimen == nil || *p.Regimen != l.regimen {
		res.distintas++
	}
}

// disparar reparte n pedidos entre `clientes` goroutines y espera a que
// terminen. Es un worker pool: una goroutine productora pone los turnos en un
// canal y cada cliente toma el siguiente apenas termina el suyo, de modo que
// siempre hay `clientes` pedidos en vuelo (lazo cerrado). El pedido i manda la
// lectura (desde+i) mod len(lecturas). Devuelve un resultado por cliente y lo
// que tardó el conjunto.
func disparar(ctx context.Context, c *cliente, lecturas []lectura, clientes, n, desde int) ([]resultado, time.Duration) {
	turnos := make(chan int)
	go func() {
		defer close(turnos)
		for i := range n {
			select {
			case turnos <- i:
			case <-ctx.Done():
				return
			}
		}
	}()

	resultados := make([]resultado, clientes)
	var wg sync.WaitGroup
	inicio := time.Now()
	for k := range clientes {
		res := &resultados[k]
		res.errores = map[string]int{}
		res.ms = make([]float64, 0, n/clientes+1)
		wg.Go(func() {
			for i := range turnos {
				c.predecir(ctx, lecturas[(desde+i)%len(lecturas)], res)
			}
		})
	}
	wg.Wait()
	return resultados, time.Since(inicio)
}

// nivel es la medición con una cantidad de clientes concurrentes.
type nivel struct {
	Clientes            int            `json:"clientes"`
	Pedidos             int            `json:"pedidos"`
	Errores             int            `json:"errores"`
	ErroresPorTipo      map[string]int `json:"errores_por_tipo,omitempty"`
	RespuestasDistintas int            `json:"respuestas_distintas"`
	DuracionS           float64        `json:"duracion_s"`
	PedidosPorSegundo   float64        `json:"pedidos_por_segundo"`
	P50Ms               float64        `json:"p50_ms"`
	P95Ms               float64        `json:"p95_ms"`
	P99Ms               float64        `json:"p99_ms"`
	MaxMs               float64        `json:"max_ms"`
	MediaMs             float64        `json:"media_ms"`
	SobreElLimite       int            `json:"sobre_el_limite"`
	Cumple              bool           `json:"cumple"`

	// aceptadas son los pedidos que respondieron 200, con los del
	// calentamiento: los que la API encoló para escribir.
	aceptadas int
}

// medirNivel calienta las conexiones con `calentamiento` pedidos que no se
// miden y después mide `pedidos` con `clientes` goroutines. Un nivel cumple si
// no hubo errores ni respuestas con otro régimen y el p99 quedó bajo limiteMs.
func medirNivel(ctx context.Context, c *cliente, lecturas []lectura, clientes, pedidos, calentamiento, limiteMs int) (nivel, error) {
	aceptadas := 0
	if calentamiento > 0 {
		previos, _ := disparar(ctx, c, lecturas, clientes, calentamiento, 0)
		for _, r := range previos {
			aceptadas += len(r.ms)
		}
	}
	resultados, duracion := disparar(ctx, c, lecturas, clientes, pedidos, calentamiento)
	if err := ctx.Err(); err != nil {
		return nivel{}, fmt.Errorf("medición con %d clientes interrumpida: %w", clientes, err)
	}

	n := nivel{Clientes: clientes, Pedidos: pedidos, ErroresPorTipo: map[string]int{}, DuracionS: duracion.Seconds()}
	var ms []float64
	for _, r := range resultados {
		ms = append(ms, r.ms...)
		n.RespuestasDistintas += r.distintas
		for tipo, cuantos := range r.errores {
			n.ErroresPorTipo[tipo] += cuantos
			n.Errores += cuantos
		}
	}
	n.aceptadas = aceptadas + len(ms)
	slices.Sort(ms)
	var suma float64
	for _, x := range ms {
		suma += x
		if x >= float64(limiteMs) {
			n.SobreElLimite++
		}
	}
	if len(ms) > 0 {
		n.MediaMs = suma / float64(len(ms))
		n.MaxMs = ms[len(ms)-1]
	}
	n.P50Ms, n.P95Ms, n.P99Ms = percentil(ms, 0.50), percentil(ms, 0.95), percentil(ms, 0.99)
	if n.DuracionS > 0 {
		n.PedidosPorSegundo = float64(len(ms)) / n.DuracionS
	}
	n.Cumple = n.Errores == 0 && n.RespuestasDistintas == 0 && len(ms) > 0 && n.P99Ms < float64(limiteMs)
	return n, nil
}

// percentil devuelve el percentil q (0 < q ≤ 1) por rango más cercano: el
// menor valor que tiene al menos el q de las muestras en él o por debajo. Es
// siempre una latencia que se midió, no una interpolación. ordenadas tiene que
// venir de menor a mayor; sin muestras devuelve 0.
func percentil(ordenadas []float64, q float64) float64 {
	if len(ordenadas) == 0 {
		return 0
	}
	// El redondeo evita que 0.95 × 100 = 95.00000000000001 suba un rango.
	pos := int(math.Ceil(math.Round(q*float64(len(ordenadas))*1e9) / 1e9))
	return ordenadas[min(max(pos, 1), len(ordenadas))-1]
}

// enteros lee una lista como «1,4,16,64», de enteros positivos.
func enteros(lista string) ([]int, error) {
	var out []int
	for parte := range strings.SplitSeq(lista, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(parte))
		if err != nil || n < 1 {
			return nil, fmt.Errorf("%q no es una lista de enteros positivos separados por comas", lista)
		}
		out = append(out, n)
	}
	return out, nil
}
