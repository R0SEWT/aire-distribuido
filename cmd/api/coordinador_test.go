package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/R0SEWT/aire-distribuido/cluster"
	"github.com/R0SEWT/aire-distribuido/kmeans"
)

const chunkSintetico = 64

// repoSintetico escribe en un directorio temporal lo que dejan cmd/cargar y
// cmd/entrenar sobre un gold sintético: el gold, su estandarización, la receta,
// reports/kmeans.json (el Lloyd concurrente de la semilla 42) y
// reports/regimenes.json. Devuelve los argumentos de la API que apuntan ahí, la
// ruta del gold para los nodos y ese resultado de referencia.
func repoSintetico(t *testing.T) (args []string, gold string, ref *kmeans.Resultado) {
	t.Helper()
	dir := t.TempDir()
	s, err := kmeans.GoldSintetico(kmeans.OpcionesSintetico{Features: []string{"pm2_5", "pm10"}, Nivel: kmeans.NivelAbsoluto,
		Estaciones: 3, Horas: 400, Ruido: 0.25, Semilla: 5})
	if err != nil {
		t.Fatal(err)
	}
	escribir := func(nombre string, contenido []byte) string {
		ruta := filepath.Join(dir, nombre)
		if err := os.WriteFile(ruta, contenido, 0o644); err != nil {
			t.Fatal(err)
		}
		return ruta
	}
	enJSON := func(v any) []byte {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	var csv bytes.Buffer
	if err := kmeans.EscribirGold(&csv, s.Gold); err != nil {
		t.Fatal(err)
	}
	gold = escribir("oefa_features.csv", csv.Bytes())
	recetaJSON := fmt.Sprintf(`{"features": ["pm2_5", "pm10"], "log1p": [], "nivel": "absoluto", "k": 3, "semilla": 42,
		"semillas": 1, "tolerancia": 1e-6, "max_iter": 100, "workers": 0, "chunk": %d}`, chunkSintetico)
	receta, err := kmeans.DecodificarReceta(strings.NewReader(recetaJSON))
	if err != nil {
		t.Fatal(err)
	}
	sha, err := kmeans.Sha256Archivo(gold)
	if err != nil {
		t.Fatal(err)
	}
	// La referencia se calcula sobre el gold releído del archivo, que es lo que
	// tienen la API y los nodos.
	g, err := kmeans.LeerGold(gold, receta.Features)
	if err != nil {
		t.Fatal(err)
	}
	cent, kEf, err := kmeans.KMeansPP(g.Datos, receta.K, 42)
	if err != nil || kEf != 3 {
		t.Fatalf("KMeansPP: %d, %v", kEf, err)
	}
	conc := receta.OpcionesConc()
	conc.Workers = 2
	if ref, err = kmeans.Concurrente(g.Datos, cent, conc); err != nil {
		t.Fatal(err)
	}
	regimenes, err := kmeans.Perfilar(g, ref.Asignaciones, ref.K, s.Est)
	if err != nil {
		t.Fatal(err)
	}
	var centroides [][]float64
	for j := range ref.K {
		centroides = append(centroides, ref.Centroide(j))
	}
	infoGold := map[string]any{"sha256": sha, "n": g.Datos.N}
	args = []string{
		"-addr", "127.0.0.1:0", "-nodos-addr", "127.0.0.1:0", "-demo", "-latido", "20ms",
		"-estaciones", "../../reports/estaciones_ficha.csv", "-openapi", "../../docs/api/openapi.yaml",
		"-gold", gold, "-inicios", "",
		"-receta", escribir("kmeans_receta.json", []byte(recetaJSON)),
		"-estandarizacion", escribir("estandarizacion.json", enJSON(s.Est)),
		"-kmeans", escribir("kmeans.json", enJSON(map[string]any{"receta": receta, "gold": infoGold, "k": ref.K,
			"iteraciones": ref.Iteraciones, "inercia": ref.Inercia, "features": receta.Features, "centroides_estandarizados": centroides})),
		"-regimenes", escribir("regimenes.json", enJSON(map[string]any{"gold": infoGold, "k": ref.K, "nivel": receta.Nivel,
			"features": receta.Features, "regimenes": regimenes})),
	}
	return args, gold, ref
}

// wsDePrueba es un cliente WebSocket mínimo: el apretón de manos y la lectura
// de las tramas del servidor, que vienen completas y sin máscara.
type wsDePrueba struct {
	conn net.Conn
	r    *bufio.Reader
}

func abrirWS(t *testing.T, dir, ruta string) *wsDePrueba {
	t.Helper()
	conn, err := net.Dial("tcp", dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	fmt.Fprintf(conn, "GET %s HTTP/1.1\r\nHost: %s\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n"+
		"Sec-WebSocket-Version: 13\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n\r\n", ruta, dir)
	r := bufio.NewReader(conn)
	resp, err := http.ReadResponse(r, nil)
	if err != nil || resp.StatusCode != http.StatusSwitchingProtocols || resp.Header.Get("Sec-WebSocket-Accept") != "s3pPLMBiTxaQ9kYGzzhZRbK+xOo=" {
		t.Fatalf("apretón de manos: %v, %v", resp, err)
	}
	return &wsDePrueba{conn: conn, r: r}
}

// leer devuelve el código de operación y la carga de la siguiente trama que no
// sea un ping.
func (w *wsDePrueba) leer(t *testing.T) (byte, []byte) {
	t.Helper()
	for {
		w.conn.SetReadDeadline(time.Now().Add(10 * time.Second))
		var cab [2]byte
		if _, err := io.ReadFull(w.r, cab[:]); err != nil {
			t.Fatalf("leyendo del WebSocket: %v", err)
		}
		n := uint64(cab[1] & 0x7F)
		switch n {
		case 126:
			var ext [2]byte
			io.ReadFull(w.r, ext[:])
			n = uint64(binary.BigEndian.Uint16(ext[:]))
		case 127:
			var ext [8]byte
			io.ReadFull(w.r, ext[:])
			n = binary.BigEndian.Uint64(ext[:])
		}
		carga := make([]byte, n)
		if _, err := io.ReadFull(w.r, carga); err != nil {
			t.Fatalf("leyendo del WebSocket: %v", err)
		}
		if cab[0] != 0x80|0x9 { // un ping
			return cab[0] & 0x0F, carga
		}
	}
}

type eventoWS struct {
	Evento string         `json:"evento"`
	Datos  map[string]any `json:"datos"`
}

// hastaElFin lee los eventos del entrenamiento id hasta su fin.
func (w *wsDePrueba) hastaElFin(t *testing.T, id string) (rondas []map[string]any, fin map[string]any) {
	t.Helper()
	for {
		op, carga := w.leer(t)
		var e eventoWS
		if err := json.Unmarshal(carga, &e); op != 0x1 || err != nil || e.Datos["id"] != id {
			t.Fatalf("trama %#x %q: %v", op, carga, err)
		}
		if e.Evento == "entrenamiento.fin" {
			return rondas, e.Datos
		}
		rondas = append(rondas, e.Datos)
	}
}

// El comando entero: la API con su pool en un puerto y HTTP en otro, tres
// nodos de verdad por loopback, un entrenamiento con 1 y otro con 3 lanzados
// por HTTP, las rondas por WebSocket, el detalle por GET, y el apagado.
func TestElCoordinadorDePuntaAPunta(t *testing.T) {
	args, gold, ref := repoSintetico(t)
	ctx, apagar := context.WithCancel(context.Background())
	var salida bytes.Buffer
	dirs := make(chan [2]string, 1)
	fin := make(chan error, 1)
	go func() {
		fin <- correr(ctx, args, entornoCon(map[string]string{"JWT_SECRETO": secreto, "CORS_ORIGENES": "http://localhost:5173"}),
			&bufferSeguro{b: &salida}, func(h, n string) { dirs <- [2]string{h, n} })
	}()
	var dirHTTP, dirNodos string
	select {
	case d := <-dirs:
		dirHTTP, dirNodos = d[0], d[1]
	case err := <-fin:
		t.Fatalf("correr terminó antes de escuchar: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("la API no empezó a escuchar")
	}
	base := "http://" + dirHTTP
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("log de la API:\n%s", salida.String())
		}
	})

	ctxNodos, pararNodos := context.WithCancel(context.Background())
	defer pararNodos()
	for i := range 3 {
		nd := &cluster.Nodo{Nombre: fmt.Sprintf("nodo-%d", i+1), Workers: 2, Chunk: chunkSintetico, Gold: gold, Log: log.New(io.Discard, "", 0)}
		go nd.Correr(ctxNodos, func(ctx context.Context) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "tcp", dirNodos)
		}, 5*time.Millisecond, false)
	}
	admin := pedir(t, "POST", base+"/api/auth/token", "", `{"usuario": "admin", "clave": "admin"}`)["token"].(string)
	libres := func() int {
		r, _ := http.NewRequest("GET", base+"/api/cluster/nodos", nil)
		r.Header.Set("Authorization", "Bearer "+admin)
		resp, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var nodos []struct{ Libre bool }
		json.NewDecoder(resp.Body).Decode(&nodos)
		n := 0
		for _, nd := range nodos {
			if nd.Libre {
				n++
			}
		}
		return n
	}
	esperarLibres := func(n int) {
		t.Helper()
		limite := time.Now().Add(5 * time.Second)
		for libres() != n {
			if time.Now().After(limite) {
				t.Fatalf("no llegaron a estar libres %d nodos (hay %d)", n, libres())
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	esperarLibres(3)
	if s := pedir(t, "GET", base+"/api/salud", "", ""); s["nodos_en_pool"] != 3.0 {
		t.Fatalf("salud = %v", s)
	}

	ws := abrirWS(t, dirHTTP, "/ws?token="+admin)
	entrenar := func(nodos int) (map[string]any, []map[string]any, map[string]any) {
		t.Helper()
		r, _ := http.NewRequest("POST", base+"/api/entrenamientos", strings.NewReader(fmt.Sprintf(`{"nodos": %d, "replicas": 1, "semilla": 42}`, nodos)))
		r.Header.Set("Authorization", "Bearer "+admin)
		resp, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		var aceptado map[string]any
		json.NewDecoder(resp.Body).Decode(&aceptado)
		resp.Body.Close()
		if resp.StatusCode != http.StatusAccepted || aceptado["estado"] != "en_curso" {
			t.Fatalf("POST con %d nodos: %d %v", nodos, resp.StatusCode, aceptado)
		}
		id := aceptado["id"].(string)
		rondas, fin := ws.hastaElFin(t, id)
		return pedir(t, "GET", base+"/api/entrenamientos/"+id, admin, ""), rondas, fin
	}

	// Con un nodo: el modelo que queda activo es el de kmeans.json bit a bit.
	// encoding/json escribe cada float64 con la representación más corta que se
	// relee igual, así que comparar los números del JSON es comparar los bits.
	det, rondas, finWS := entrenar(1)
	if det["estado"] != "terminado" || det["iteraciones"] != float64(ref.Iteraciones) || det["paro"] != ref.Paro ||
		len(rondas) != ref.Iteraciones || len(det["rondas"].([]any)) != ref.Iteraciones || finWS["iteraciones"] != det["iteraciones"] {
		t.Fatalf("con 1 nodo: %v; %d rondas por el WebSocket; la referencia tiene %d iteraciones", det, len(rondas), ref.Iteraciones)
	}
	for i, r := range rondas {
		if r["inercia"] != ref.Inercias[i] {
			t.Fatalf("ronda %d: inercia %v, kmeans.json dice %v", i+1, r["inercia"], ref.Inercias[i])
		}
	}
	activo := pedir(t, "GET", base+"/api/modelos/activo", admin, "")
	if activo["id"] != "entrenado-"+det["id"].(string) || activo["inercia"] != ref.Inercia || activo["iteraciones"] != float64(ref.Iteraciones) {
		t.Fatalf("modelo activo = %v; kmeans.json: inercia %v en %d iteraciones", activo, ref.Inercia, ref.Iteraciones)
	}

	// Con tres: el mismo modelo salvo redondeo, y los nodos volvieron solos.
	esperarLibres(3)
	det3, rondas3, _ := entrenar(3)
	if det3["estado"] != "terminado" || det3["iteraciones"] != float64(ref.Iteraciones) || len(rondas3) != ref.Iteraciones ||
		fmt.Sprint(det3["nodos"]) != "[nodo-1 nodo-2 nodo-3]" || det3["dif_secuencial"].(float64) > 1e-9 {
		t.Fatalf("con 3 nodos: %v", det3)
	}
	if len(rondas3[0]["ms_por_nodo"].(map[string]any)) != 3 {
		t.Fatalf("ms_por_nodo = %v", rondas3[0]["ms_por_nodo"])
	}
	if m := pedir(t, "GET", base+"/api/metricas", admin, ""); m["ultimo_entrenamiento"] != det3["id"] || m["nodos_en_pool"] == nil {
		t.Fatalf("métricas = %v", m)
	}
	// Los «logs» del clúster: las altas de los tres nodos y, por cada
	// entrenamiento, la baja de los que se llevó; el más reciente primero.
	esperarLibres(3) // los tres volvieron: sus altas ya pasaron
	var eventos []struct{ TS, Tipo, Nodo, Detalle string }
	altas, bajas := map[string]int{}, 0
	limite := time.Now().Add(5 * time.Second)
	for {
		r, _ := http.NewRequest("GET", base+"/api/cluster/eventos?limite=200", nil)
		r.Header.Set("Authorization", "Bearer "+admin)
		resp, err := http.DefaultClient.Do(r)
		if err != nil || resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /api/cluster/eventos: %v, %v", resp, err)
		}
		eventos = nil
		json.NewDecoder(resp.Body).Decode(&eventos)
		resp.Body.Close()
		altas, bajas = map[string]int{}, 0
		for _, e := range eventos {
			switch {
			case e.Tipo == "alta":
				altas[e.Nodo]++
			case e.Tipo == "baja" && strings.Contains(e.Detalle, "terminado"):
				bajas++
			}
		}
		// Los eventos se guardan en segundo plano: se espera a que estén los
		// del segundo entrenamiento (1 + 3 bajas y, de vuelta, 3 + 1 + 3 altas).
		if (bajas == 4 && len(altas) == 3 && altas["nodo-1"]+altas["nodo-2"]+altas["nodo-3"] == 7) || time.Now().After(limite) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if bajas != 4 || altas["nodo-1"]+altas["nodo-2"]+altas["nodo-3"] != 7 {
		t.Fatalf("eventos del clúster: %d bajas por entrenamiento terminado y altas %v: %+v", bajas, altas, eventos)
	}
	for i := 1; i < len(eventos); i++ {
		antes, err1 := time.Parse(time.RFC3339, eventos[i-1].TS)
		este, err2 := time.Parse(time.RFC3339, eventos[i].TS)
		if err1 != nil || err2 != nil || este.After(antes) || !strings.HasSuffix(eventos[i].TS, "-05:00") {
			t.Fatalf("los eventos no vienen del más reciente al más antiguo, en hora de Lima: %s antes que %s", eventos[i-1].TS, eventos[i].TS)
		}
	}
	// CORS quedó configurado desde la variable de entorno.
	pre, _ := http.NewRequest("OPTIONS", base+"/api/entrenamientos", nil)
	pre.Header.Set("Origin", "http://localhost:5173")
	pre.Header.Set("Access-Control-Request-Method", "POST")
	if resp, err := http.DefaultClient.Do(pre); err != nil || resp.StatusCode != http.StatusNoContent || resp.Header.Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Fatalf("preflight: %v, %v", resp, err)
	}

	// Apagado: el cliente del WebSocket recibe el cierre 1001 y correr vuelve.
	apagar()
	if op, carga := ws.leer(t); op != 0x8 || len(carga) < 2 || binary.BigEndian.Uint16(carga) != 1001 {
		t.Fatalf("al apagar se esperaba un cierre 1001: %#x %q", op, carga)
	}
	ws.conn.Close()
	select {
	case err := <-fin:
		if err != nil {
			t.Fatalf("correr devolvió %v al apagarse", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("la API no se apagó")
	}
	for _, linea := range []string{"clúster: los nodos llaman aquí", "gold para entrenar", "entrenamiento terminado", "pool: alta nodo-1"} {
		if !strings.Contains(salida.String(), linea) {
			t.Errorf("falta en el log: %q", linea)
		}
	}
	if strings.Contains(salida.String(), admin) {
		t.Error("el token del WebSocket quedó en el log")
	}
}

// Sin el gold la API arranca igual y lo dice; con un gold que no es el de la
// estandarización, no arranca.
func TestArrancaSinGoldYNoArrancaConOtroGold(t *testing.T) {
	op, err := leerOpciones(append([]string{}, argsDelRepo...))
	if err != nil {
		t.Fatal(err)
	}
	var salida bytes.Buffer
	registro := slogDe(&salida)
	entorno := entornoCon(map[string]string{"JWT_SECRETO": secreto, "API_USUARIOS": usuariosPrueba})
	op.gold = "no-existe/oefa_features.csv"
	if _, _, err := armar(op, entorno, registro, nil); err != nil || !strings.Contains(salida.String(), "no puede entrenar") {
		t.Fatalf("sin el gold tiene que arrancar con un aviso: %v\n%s", err, salida.String())
	}
	_, gold, _ := repoSintetico(t)
	op.gold, op.receta = gold, "../../configs/kmeans.json"
	if _, _, err := armar(op, entorno, registro, nil); err == nil || !strings.Contains(err.Error(), "estandarización") {
		t.Fatalf("un gold que no es el de reports/estandarizacion.json: %v", err)
	}
	for _, args := range [][]string{{"-plazo-ronda", "0s"}, {"-latido", "-1s"}, {"-max-filas", "0"}} {
		if _, err := leerOpciones(args); err == nil {
			t.Errorf("leerOpciones aceptó %v", args)
		}
	}
	if _, _, err := armar(op, entornoCon(map[string]string{"JWT_SECRETO": secreto, "API_USUARIOS": usuariosPrueba, "CORS_ORIGENES": "localhost"}), registro, nil); err == nil ||
		!strings.Contains(err.Error(), "CORS") {
		t.Fatalf("un CORS_ORIGENES mal escrito: %v", err)
	}
}
