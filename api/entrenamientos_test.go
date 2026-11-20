package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/R0SEWT/aire-distribuido/almacen"
	"github.com/R0SEWT/aire-distribuido/cluster"
	"github.com/R0SEWT/aire-distribuido/kmeans"
	"github.com/R0SEWT/aire-distribuido/protocolo"
)

const chunkDePrueba = 64

// cluster de prueba: la API con su pool sobre loopback, un gold sintético y
// nodos de verdad (cluster.Nodo) que la llaman.
type entornoCluster struct {
	*entorno
	ent      *Entrenador
	goldRuta string
	dirNodos string
	admin    string // token de admin
}

// entrenadorSintetico escribe un gold sintético y arma el Entrenador con su
// estandarización y una receta de k = 3.
func entrenadorSintetico(t *testing.T) (*Entrenador, string) {
	t.Helper()
	s, err := kmeans.GoldSintetico(kmeans.OpcionesSintetico{Features: []string{"pm2_5", "pm10"}, Nivel: kmeans.NivelAbsoluto,
		Estaciones: 3, Horas: 400, Ruido: 0.25, Semilla: 5})
	if err != nil {
		t.Fatal(err)
	}
	ruta := filepath.Join(t.TempDir(), "oefa_features.csv")
	var buf bytes.Buffer
	if err := kmeans.EscribirGold(&buf, s.Gold); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ruta, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	sha, err := kmeans.Sha256Archivo(ruta)
	if err != nil {
		t.Fatal(err)
	}
	receta, err := kmeans.DecodificarReceta(strings.NewReader(fmt.Sprintf(`{"features": ["pm2_5", "pm10"], "log1p": [], "nivel": "absoluto",
		"k": 3, "semilla": 42, "semillas": 1, "tolerancia": 1e-6, "max_iter": 100, "workers": 0, "chunk": %d}`, chunkDePrueba)))
	if err != nil {
		t.Fatal(err)
	}
	// El gold se relee del archivo, como hace la API: es lo que tienen los nodos.
	g, err := kmeans.LeerGold(ruta, receta.Features)
	if err != nil {
		t.Fatal(err)
	}
	return &Entrenador{Gold: g, Sha256: sha, Receta: receta, Estandarizacion: s.Est,
		PlazoCarga: 5 * time.Second, PlazoRonda: 5 * time.Second}, ruta
}

func nuevoEntornoCluster(t *testing.T, cambiar ...func(*Config)) *entornoCluster {
	t.Helper()
	ent, ruta := entrenadorSintetico(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	base := func(c *Config) {
		c.Nodos = ln
		c.Pool = cluster.OpcionesPool{PlazoHola: time.Second, CadaLatido: 20 * time.Millisecond, PlazoLatido: 500 * time.Millisecond}
		c.Entrenador = ent
	}
	ec := &entornoCluster{ent: ent, goldRuta: ruta, dirNodos: ln.Addr().String()}
	ec.entorno = nuevoEntorno(t, append([]func(*Config){base}, cambiar...)...)
	ec.admin = ec.token(t, "admin", "clave-admin")
	return ec
}

var logNodos = log.New(io.Discard, "", 0)

// lanzarNodos levanta n nodos de verdad que llaman a la API, atienden una
// corrida y vuelven a llamar, y espera a que estén en el pool.
func (ec *entornoCluster) lanzarNodos(t *testing.T, n int) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	antes := ec.srv.nodosEnPool()
	for i := range n {
		nd := &cluster.Nodo{Nombre: fmt.Sprintf("nodo-%d", antes+i+1), Workers: 1 + i%2, Chunk: chunkDePrueba, Gold: ec.goldRuta, Log: logNodos}
		go nd.Correr(ctx, func(ctx context.Context) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "tcp", ec.dirNodos)
		}, 5*time.Millisecond, false)
	}
	ec.esperarLibres(t, antes+n)
}

func (ec *entornoCluster) esperarLibres(t *testing.T, n int) {
	t.Helper()
	esperarHasta(t, fmt.Sprintf("%d nodos libres en el pool", n), func() bool {
		libres := 0
		for _, nd := range ec.srv.pool.Nodos() {
			if nd.Libre {
				libres++
			}
		}
		return libres == n
	})
}

// nodoMudo se presenta, responde los PING y carga su rango como un nodo sano;
// después no responde ningún ITERAR. Devuelve cómo apagarlo.
func (ec *entornoCluster) nodoMudo(t *testing.T, nombre string) {
	t.Helper()
	conn, err := net.Dial("tcp", ec.dirNodos)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	go func() {
		cod, dec := protocolo.NuevoCodificador(conn), protocolo.NuevoDecodificador(conn)
		cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Hola, Version: protocolo.Version, Nodo: nombre, Workers: 1})
		for {
			m, err := dec.Leer()
			if err != nil {
				return
			}
			switch m.Tipo {
			case protocolo.Ping:
				cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Pong})
			case protocolo.Cargar:
				cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Listo, N: m.Rango.Len(), Huella: m.Huella})
			}
		}
	}()
}

// entrenar lanza un entrenamiento y devuelve su id.
func (ec *entornoCluster) entrenar(t *testing.T, cuerpo string) string {
	t.Helper()
	obj := objeto(t, ec.pedir(t, "POST", "/api/entrenamientos", ec.admin, cuerpo), http.StatusAccepted, "id", "estado")
	if obj["estado"] != "en_curso" || len(obj) != 2 {
		t.Fatalf("202 = %v; se esperaba solo id y estado en_curso", obj)
	}
	return obj["id"].(string)
}

// esperarFin espera a que el entrenamiento deje de estar en curso y devuelve su detalle.
func (ec *entornoCluster) esperarFin(t *testing.T, id string) map[string]any {
	t.Helper()
	var det map[string]any
	esperarHasta(t, "el fin del entrenamiento "+id, func() bool {
		det = objeto(t, ec.pedir(t, "GET", "/api/entrenamientos/"+id, ec.admin, ""), http.StatusOK, "id", "estado", "nodos", "replicas", "rondas")
		return det["estado"] != "en_curso"
	})
	return det
}

// referencia es el Lloyd concurrente de una sola máquina desde los mismos
// centroides iniciales que usa la API para esa semilla.
func (ec *entornoCluster) referencia(t *testing.T, semilla int64) *kmeans.Resultado {
	t.Helper()
	in, _, err := ec.ent.inicio(semilla)
	if err != nil {
		t.Fatal(err)
	}
	op := ec.ent.Receta.OpcionesConc()
	op.K, op.Workers = in.KEfectivo, 2
	r, err := kmeans.Concurrente(ec.ent.Gold.Datos, in.Centroides, op)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func mismosBits(a, b []float64) bool {
	return slices.EqualFunc(a, b, func(x, y float64) bool { return math.Float64bits(x) == math.Float64bits(y) })
}

func TestNodosDelCluster(t *testing.T) {
	// Sin clúster: una lista vacía, no un error.
	sin := nuevoEntorno(t)
	admin := sin.token(t, "admin", "clave-admin")
	if w := sin.pedir(t, "GET", "/api/cluster/nodos", admin, ""); w.Code != 200 || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("sin clúster: %d %s", w.Code, w.Body)
	}

	ec := nuevoEntornoCluster(t)
	mensajeDeError(t, ec.pedir(t, "GET", "/api/cluster/nodos", "", ""), http.StatusUnauthorized, msgNoAutenticado)
	mensajeDeError(t, ec.pedir(t, "GET", "/api/cluster/nodos", ec.token(t, "demo", "demo"), ""), http.StatusForbidden, "se requiere el rol admin")
	ec.lanzarNodos(t, 2)

	var nodos []map[string]any
	w := ec.pedir(t, "GET", "/api/cluster/nodos", ec.admin, "")
	if err := json.Unmarshal(w.Body.Bytes(), &nodos); err != nil || w.Code != 200 || len(nodos) != 2 {
		t.Fatalf("nodos: %d %s", w.Code, w.Body)
	}
	for i, n := range nodos {
		if len(n) != 5 || n["nombre"] != fmt.Sprintf("nodo-%d", i+1) || n["libre"] != true || n["workers"].(float64) < 1 ||
			n["latencia_ms"].(float64) <= 0 || !strings.HasPrefix(n["direccion"].(string), "127.0.0.1:") {
			t.Errorf("nodo %d = %v; se esperaban los cinco campos del esquema Nodo", i, n)
		}
	}
	if salud := objeto(t, ec.pedir(t, "GET", "/api/salud", "", ""), 200); salud["nodos_en_pool"] != 2.0 {
		t.Fatalf("salud = %v", salud)
	}
	if m := objeto(t, ec.pedir(t, "GET", "/api/metricas", ec.admin, ""), 200); m["nodos_en_pool"] != 2.0 || m["ultimo_entrenamiento"] != nil {
		t.Fatalf("métricas = %v", m)
	}
	// Las altas quedaron en el almacén de eventos.
	esperarHasta(t, "las dos altas en eventos_cluster", func() bool {
		altas := 0
		for _, e := range ec.ev.Todos() {
			if e.Tipo == cluster.EventoAlta && strings.HasPrefix(e.Nodo, "nodo-") && !e.TS.IsZero() && strings.Contains(e.Detalle, "127.0.0.1:") {
				altas++
			}
		}
		return altas == 2
	})
}

func TestEntrenamientoPedidosQueNoSeAceptan(t *testing.T) {
	ec := nuevoEntornoCluster(t)
	const ruta = "/api/entrenamientos"
	mensajeDeError(t, ec.pedir(t, "POST", ruta, "", `{"nodos": 1, "replicas": 1, "semilla": 42}`), http.StatusUnauthorized, msgNoAutenticado)
	usuario := ec.token(t, "demo", "demo")
	for _, r := range [][2]string{{"POST", ruta}, {"GET", ruta}, {"GET", ruta + "/e-1"}} {
		mensajeDeError(t, ec.pedir(t, r[0], r[1], usuario, `{"nodos": 1, "replicas": 1, "semilla": 42}`), http.StatusForbidden, "se requiere el rol admin")
	}

	for cuerpo, want := range map[string]string{
		`no es json`: msgCuerpoEntrenamiento,
		`[1, 1, 42]`: msgCuerpoEntrenamiento,
		`{"nodos": 1, "replicas": 1, "semilla": 42} x`:                 msgCuerpoEntrenamiento,
		`{"replicas": 1, "semilla": 42}`:                               msgNodos,
		`{"nodos": 0, "replicas": 1, "semilla": 42}`:                   msgNodos,
		`{"nodos": 1.5, "replicas": 1, "semilla": 42}`:                 msgNodos,
		`{"nodos": "2", "replicas": 1, "semilla": 42}`:                 msgNodos,
		`{"nodos": 1, "semilla": 42}`:                                  msgReplicas,
		`{"nodos": 1, "replicas": -3, "semilla": 42}`:                  msgReplicas,
		`{"nodos": 1, "replicas": 1}`:                                  msgSemilla,
		`{"nodos": 1, "replicas": 1, "semilla": 1.5}`:                  msgSemilla,
		`{"nodos": 1, "replicas": 1, "semilla": 99999999999999999999}`: msgSemilla,
		`{"nodos": 5000, "replicas": 1, "semilla": 42}`:                "nodos no puede superar las 1200 filas del gold",
	} {
		mensajeDeError(t, ec.pedir(t, "POST", ruta, ec.admin, cuerpo), http.StatusBadRequest, want)
	}
	// Réplicas que no entran en los nodos pedidos: se rechaza antes de tocar el pool.
	w := ec.pedir(t, "POST", ruta, ec.admin, `{"nodos": 2, "replicas": 100000, "semilla": 42}`)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "el máximo es 41666") {
		t.Fatalf("réplicas de más: %d %s", w.Code, w.Body)
	}

	// 422: no hay nodos libres suficientes, con el mensaje del contrato.
	mensajeDeError(t, ec.pedir(t, "POST", ruta, ec.admin, `{"nodos": 2, "replicas": 1, "semilla": 42}`),
		http.StatusUnprocessableEntity, "hay 0 nodos libres en el pool y se pidieron 2")
	ec.lanzarNodos(t, 2)
	mensajeDeError(t, ec.pedir(t, "POST", ruta, ec.admin, `{"nodos": 4, "replicas": 1, "semilla": 42}`),
		http.StatusUnprocessableEntity, "hay 2 nodos libres en el pool y se pidieron 4")
	// Un pedido rechazado no deja nada tomado: ni el semáforo ni los nodos.
	ec.esperarLibres(t, 2)

	// Nada de lo anterior quedó en el historial.
	if w := ec.pedir(t, "GET", ruta, ec.admin, ""); w.Code != 200 || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("historial: %d %s", w.Code, w.Body)
	}
	mensajeDeError(t, ec.pedir(t, "GET", ruta+"/e-20261119-9999", ec.admin, ""), http.StatusNotFound, "no existe el entrenamiento e-20261119-9999")

	// Sin clúster: 422 con cero nodos. Sin gold: 503.
	sinCluster := nuevoEntorno(t, func(c *Config) { c.Entrenador = ec.ent })
	mensajeDeError(t, sinCluster.pedir(t, "POST", ruta, sinCluster.token(t, "admin", "clave-admin"), `{"nodos": 1, "replicas": 1, "semilla": 42}`),
		http.StatusUnprocessableEntity, "hay 0 nodos libres en el pool y se pidieron 1")
	sinGold := nuevoEntorno(t)
	w = sinGold.pedir(t, "POST", ruta, sinGold.token(t, "admin", "clave-admin"), `{"nodos": 1, "replicas": 1, "semilla": 42}`)
	if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "gold") {
		t.Fatalf("sin gold: %d %s", w.Code, w.Body)
	}
}

func TestEntrenamientoConTresNodos(t *testing.T) {
	ec := nuevoEntornoCluster(t)
	ec.lanzarNodos(t, 3)
	id := ec.entrenar(t, `{"nodos": 3, "replicas": 1, "semilla": 42}`)
	if !strings.HasPrefix(id, "e-") || len(id) != len("e-20261119-1530") {
		t.Fatalf("id = %q; se esperaba e-AAAAMMDD-HHMM", id)
	}
	det := ec.esperarFin(t, id)
	ref := ec.referencia(t, 42)

	if det["estado"] != "terminado" || det["replicas"] != 1.0 || det["paro"] != ref.Paro || det["iteraciones"] != float64(ref.Iteraciones) {
		t.Fatalf("detalle = %v; la referencia paró por %s en %d iteraciones", det, ref.Paro, ref.Iteraciones)
	}
	if _, hay := det["error"]; hay {
		t.Fatalf("un entrenamiento terminado no trae error: %v", det)
	}
	if _, hay := det["aviso"]; hay || det["modelo_id"] != "entrenado-"+id {
		t.Fatalf("activó su modelo: trae modelo_id y ningún aviso: %v", det)
	}
	if got := fmt.Sprint(det["nodos"]); got != "[nodo-1 nodo-2 nodo-3]" {
		t.Fatalf("nodos = %s", got)
	}
	if dif := det["dif_secuencial"].(float64); dif < 0 || dif > 1e-9 {
		t.Fatalf("dif_secuencial = %g; con tres nodos tiene que ser redondeo", dif)
	}
	if det["ms_total"].(float64) <= 0 {
		t.Fatalf("ms_total = %v", det["ms_total"])
	}
	rondas := det["rondas"].([]any)
	if len(rondas) != ref.Iteraciones {
		t.Fatalf("%d rondas para %d iteraciones", len(rondas), ref.Iteraciones)
	}
	for i, x := range rondas {
		r := x.(map[string]any)
		porNodo, _ := r["ms_por_nodo"].(map[string]any)
		if len(r) != 5 || r["iter"] != float64(i+1) || r["ms_ronda"].(float64) <= 0 || r["ms_fusion"].(float64) < 0 || len(porNodo) != 3 {
			t.Fatalf("ronda %d = %v; se esperaban los cinco campos del esquema Ronda", i+1, r)
		}
		if rel := math.Abs(r["inercia"].(float64)-ref.Inercias[i]) / ref.Inercias[i]; rel > 1e-9 {
			t.Fatalf("ronda %d: inercia %v, la referencia %v", i+1, r["inercia"], ref.Inercias[i])
		}
		for _, nombre := range []string{"nodo-1", "nodo-2", "nodo-3"} {
			if v, _ := porNodo[nombre].(float64); v <= 0 || v > r["ms_ronda"].(float64) {
				t.Fatalf("ronda %d: %s tardó %v ms y la ronda %v", i+1, nombre, porNodo[nombre], r["ms_ronda"])
			}
		}
	}

	// replicas = 1 y terminó bien: el modelo nuevo es el activo, y es el de una
	// sola máquina salvo redondeo.
	m := ec.srv.modelo.Load()
	if m.ID != "entrenado-"+id || m.Iteraciones != ref.Iteraciones {
		t.Fatalf("modelo activo = %s (%d iteraciones)", m.ID, m.Iteraciones)
	}
	for i, c := range m.CentroidesEstandarizados() {
		if math.Abs(c-ref.Centroides[i]) > 1e-9 {
			t.Fatalf("centroide %d: %v, la referencia %v", i, c, ref.Centroides[i])
		}
	}
	activo := objeto(t, ec.pedir(t, "GET", "/api/modelos/activo", ec.admin, ""), 200, "id", "k", "regimenes")
	if activo["id"] != "entrenado-"+id || activo["k"] != 3.0 || len(activo["regimenes"].([]any)) != 3 {
		t.Fatalf("modelo activo por la API = %v", activo)
	}
	total := 0.0
	for _, r := range activo["regimenes"].([]any) {
		total += r.(map[string]any)["tamano"].(float64)
	}
	if total != 1200 {
		t.Fatalf("los regímenes del modelo nuevo suman %v filas y el gold tiene 1200", total)
	}
	// Y se predice con él.
	pred := objeto(t, ec.pedir(t, "POST", "/api/predicciones", ec.admin, lecturaEjemplo), 200, camposPrediccion...)
	if pred["modelo_id"] != "entrenado-"+id {
		t.Fatalf("la predicción usó el modelo %v", pred["modelo_id"])
	}

	// El historial lo trae sin rondas; las métricas lo nombran.
	var lista []map[string]any
	w := ec.pedir(t, "GET", "/api/entrenamientos", ec.admin, "")
	if err := json.Unmarshal(w.Body.Bytes(), &lista); err != nil || len(lista) != 1 || lista[0]["id"] != id || lista[0]["estado"] != "terminado" {
		t.Fatalf("historial: %s", w.Body)
	}
	if _, hay := lista[0]["rondas"]; hay || lista[0]["iteraciones"] != float64(ref.Iteraciones) {
		t.Fatalf("el historial va sin rondas y con el resumen: %v", lista[0])
	}
	if mt := objeto(t, ec.pedir(t, "GET", "/api/metricas", ec.admin, ""), 200); mt["ultimo_entrenamiento"] != id {
		t.Fatalf("métricas = %v", mt)
	}

	// En el almacén quedó el documento de §7.1, con el rango de cada nodo.
	doc, err := ec.entorno.ent.Obtener(context.Background(), id)
	if err != nil || doc.Parametros != (almacen.ParametrosEntrenamiento{Nodos: 3, Replicas: 1, Semilla: 42}) || doc.ModeloID != m.ID ||
		doc.Fin.Before(doc.Inicio) || len(doc.Nodos) != 3 || doc.Nodos[0].Desde != 0 || doc.Nodos[2].Hasta != 1200 || doc.Nodos[1].Desde != doc.Nodos[0].Hasta {
		t.Fatalf("documento = %+v, %v", doc, err)
	}

	// Los nodos volvieron solos al pool, con su baja y su alta en los eventos.
	ec.esperarLibres(t, 3)
	bajas := 0
	for _, e := range ec.ev.Todos() {
		if e.Tipo == cluster.EventoBaja && strings.Contains(e.Detalle, "entrenamiento "+id+" terminado") {
			bajas++
		}
	}
	if bajas != 3 {
		t.Fatalf("se esperaban tres bajas por el fin del entrenamiento: %+v", ec.ev.Todos())
	}

	// Otro entrenamiento en el mismo minuto tiene otro id.
	if id2 := ec.entrenar(t, `{"nodos": 2, "replicas": 1, "semilla": 7}`); id2 == id {
		t.Fatalf("dos entrenamientos con el id %s", id)
	} else if det := ec.esperarFin(t, id2); det["estado"] != "terminado" || fmt.Sprint(det["nodos"]) != "[nodo-1 nodo-2]" {
		t.Fatalf("segundo entrenamiento = %v", det)
	}
}

// Con un solo nodo y el chunk de la receta, el modelo que queda activo es el
// del Lloyd concurrente de una sola máquina BIT A BIT.
func TestEntrenamientoConUnNodoEsBitABit(t *testing.T) {
	ec := nuevoEntornoCluster(t)
	ec.lanzarNodos(t, 1)
	det := ec.esperarFin(t, ec.entrenar(t, `{"nodos": 1, "replicas": 1, "semilla": 42}`))
	if det["estado"] != "terminado" {
		t.Fatalf("detalle = %v", det)
	}
	ref := ec.referencia(t, 42)
	m := ec.srv.modelo.Load()
	if !mismosBits(m.CentroidesEstandarizados(), ref.Centroides) || math.Float64bits(m.Inercia) != math.Float64bits(ref.Inercia) || m.Iteraciones != ref.Iteraciones {
		t.Fatalf("con un nodo el modelo no es el concurrente bit a bit:\n%v (inercia %v, %d iteraciones)\n%v (inercia %v, %d iteraciones)",
			m.CentroidesEstandarizados(), m.Inercia, m.Iteraciones, ref.Centroides, ref.Inercia, ref.Iteraciones)
	}
	rondas := det["rondas"].([]any)
	for i, x := range rondas {
		if got := x.(map[string]any)["inercia"].(float64); math.Float64bits(got) != math.Float64bits(ref.Inercias[i]) {
			t.Fatalf("ronda %d: inercia %v, la referencia %v", i+1, got, ref.Inercias[i])
		}
	}
	if dif := det["dif_secuencial"].(float64); dif > 1e-9 {
		t.Fatalf("dif_secuencial = %g", dif)
	}
}

// Con réplicas la corrida es una medición: termina, pero no activa nada.
func TestEntrenamientoConReplicasNoActivaElModelo(t *testing.T) {
	ec := nuevoEntornoCluster(t)
	ec.lanzarNodos(t, 2)
	antes := ec.srv.modelo.Load()
	id := ec.entrenar(t, `{"nodos": 2, "replicas": 3, "semilla": 42}`)
	det := ec.esperarFin(t, id)
	if det["estado"] != "terminado" || det["replicas"] != 3.0 {
		t.Fatalf("detalle = %v", det)
	}
	if dif := det["dif_secuencial"].(float64); dif > 1e-9 {
		t.Fatalf("dif_secuencial = %g: los centroides de la tabla replicada son los del gold", dif)
	}
	if ec.srv.modelo.Load() != antes {
		t.Fatalf("con replicas = 3 se activó el modelo %s", ec.srv.modelo.Load().ID)
	}
	// Es lo pedido, no un problema: ni modelo_id ni aviso.
	if _, hay := det["modelo_id"]; hay || det["aviso"] != nil {
		t.Fatalf("una medición no trae modelo_id ni aviso: %v", det)
	}
	doc, _ := ec.entorno.ent.Obtener(context.Background(), id)
	if doc.ModeloID != "" || doc.Nodos[1].Hasta != 3600 {
		t.Fatalf("documento = %+v; la tabla repartida tiene 3 × 1200 filas", doc)
	}
	// La inercia de la tabla replicada es 3 veces la del gold.
	ref := ec.referencia(t, 42)
	primera := det["rondas"].([]any)[0].(map[string]any)["inercia"].(float64)
	if rel := math.Abs(primera/3-ref.Inercias[0]) / ref.Inercias[0]; rel > 1e-9 {
		t.Fatalf("inercia de la ronda 1 = %v; 3 × %v", primera, ref.Inercias[0])
	}
}

// Un nodo que deja de responder: mientras vence el plazo el entrenamiento está
// en curso y un segundo pedido recibe 409; después queda abortado con el nodo
// en el error, sin modelo nuevo, y el semáforo y los demás nodos quedan libres.
func TestUnEntrenamientoALaVezYElNodoQueFallaLoAborta(t *testing.T) {
	ec := nuevoEntornoCluster(t, func(c *Config) { c.Entrenador.PlazoRonda = 400 * time.Millisecond })
	t.Cleanup(func() { ec.ent.PlazoRonda = 5 * time.Second })
	ec.lanzarNodos(t, 2)
	ec.nodoMudo(t, "nodo-3-mudo")
	ec.esperarLibres(t, 3)
	antes := ec.srv.modelo.Load()

	const pedido = `{"nodos": 3, "replicas": 1, "semilla": 42}`
	id := ec.entrenar(t, pedido)
	mensajeDeError(t, ec.pedir(t, "POST", "/api/entrenamientos", ec.admin, pedido), http.StatusConflict, "ya hay un entrenamiento en curso: "+id)
	enCurso := objeto(t, ec.pedir(t, "GET", "/api/entrenamientos/"+id, ec.admin, ""), 200, "id", "estado", "nodos", "replicas", "rondas")
	if enCurso["estado"] != "en_curso" || len(enCurso) != 5 || len(enCurso["rondas"].([]any)) != 0 {
		t.Fatalf("en curso = %v; solo identificación y rondas vacías", enCurso)
	}
	// Durante la corrida los tres están en el pool, ocupados.
	for _, n := range ec.srv.pool.Nodos() {
		if n.Libre {
			t.Fatalf("%s figura libre durante la corrida", n.Nombre)
		}
	}

	det := ec.esperarFin(t, id)
	if det["estado"] != "abortado" || len(det["rondas"].([]any)) != 0 {
		t.Fatalf("detalle = %v", det)
	}
	errTexto, _ := det["error"].(string)
	if !strings.Contains(errTexto, `"nodo-3-mudo"`) || strings.Contains(errTexto, `"nodo-1"`) || !strings.Contains(errTexto, "plazo") {
		t.Fatalf("el error tiene que nombrar al nodo que no respondió, y solo a él: %q", errTexto)
	}
	for _, campo := range []string{"iteraciones", "paro", "dif_secuencial", "ms_total"} {
		if _, hay := det[campo]; hay {
			t.Errorf("un entrenamiento abortado no trae %s: %v", campo, det)
		}
	}
	if ec.srv.modelo.Load() != antes {
		t.Fatal("un entrenamiento abortado no activa ningún modelo")
	}
	esperarHasta(t, "el evento de error del nodo", func() bool {
		for _, e := range ec.ev.Todos() {
			if e.Tipo == cluster.EventoError && e.Nodo == "nodo-3-mudo" && strings.Contains(e.Detalle, id) {
				return true
			}
		}
		return false
	})

	// El semáforo quedó libre y los nodos sanos volvieron: se puede entrenar otra vez.
	ec.esperarLibres(t, 2)
	if det := ec.esperarFin(t, ec.entrenar(t, `{"nodos": 2, "replicas": 1, "semilla": 42}`)); det["estado"] != "terminado" {
		t.Fatalf("después del aborto: %v", det)
	}
}

// Un nodo que se cae a mitad de la corrida (cierra la conexión): abortado con
// las rondas que sí se hicieron.
func TestNodoQueSeCaeAMitadDeLaCorridaConservaLasRondasHechas(t *testing.T) {
	ec := nuevoEntornoCluster(t)
	ec.lanzarNodos(t, 1)
	// Un nodo que responde dos rondas y se apaga.
	conn, err := net.Dial("tcp", ec.dirNodos)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	go func() {
		cod, dec := protocolo.NuevoCodificador(conn), protocolo.NuevoDecodificador(conn)
		cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Hola, Version: protocolo.Version, Nodo: "nodo-2-fragil", Workers: 1})
		var datos *kmeans.Datos
		rondas := 0
		for {
			m, err := dec.Leer()
			if err != nil {
				return
			}
			switch m.Tipo {
			case protocolo.Ping:
				cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Pong})
			case protocolo.Cargar:
				datos = cluster.FilasVirtuales(ec.ent.Gold.Datos, *m.Rango)
				cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Listo, N: datos.N, Huella: m.Huella})
			case protocolo.Iterar:
				if rondas++; rondas > 2 {
					conn.Close()
					return
				}
				p := kmeans.ParcialesPorChunks(datos, m.Centroides, 1, chunkDePrueba)
				cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Parcial, Iter: m.Iter, Sumas: p.Sumas, Conteos: p.Conteos, Inercia: p.Inercia})
			}
		}
	}()
	ec.esperarLibres(t, 2)

	det := ec.esperarFin(t, ec.entrenar(t, `{"nodos": 2, "replicas": 1, "semilla": 42}`))
	errTexto, _ := det["error"].(string)
	if det["estado"] != "abortado" || !strings.Contains(errTexto, `"nodo-2-fragil"`) || !strings.Contains(errTexto, "ronda 3") {
		t.Fatalf("detalle = %v", det)
	}
	if rondas := det["rondas"].([]any); len(rondas) != 2 || rondas[1].(map[string]any)["iter"] != 2.0 {
		t.Fatalf("tienen que quedar las dos rondas hechas: %v", det["rondas"])
	}
}

// --- WebSocket por HTTP y de punta a punta dentro del proceso ---

// abrirWS hace el apretón de manos contra un servidor HTTP de verdad y
// devuelve el cliente, o la respuesta si no fue un 101.
func abrirWS(t *testing.T, base, ruta string, cabeceras map[string]string) (*clienteWSPrueba, *http.Response) {
	t.Helper()
	conn, err := net.Dial("tcp", strings.TrimPrefix(base, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	pedido := "GET " + ruta + " HTTP/1.1\r\nHost: " + strings.TrimPrefix(base, "http://") + "\r\n"
	enc := map[string]string{"Connection": "Upgrade", "Upgrade": "websocket", "Sec-WebSocket-Version": "13",
		"Sec-WebSocket-Key": "dGhlIHNhbXBsZSBub25jZQ=="}
	for k, v := range cabeceras {
		enc[k] = v
	}
	for k, v := range enc {
		if v != "" {
			pedido += k + ": " + v + "\r\n"
		}
	}
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := io.WriteString(conn, pedido+"\r\n"); err != nil {
		t.Fatal(err)
	}
	r := bufio.NewReader(conn)
	resp, err := http.ReadResponse(r, nil)
	if err != nil {
		t.Fatal(err)
	}
	conn.SetDeadline(time.Time{})
	if resp.StatusCode != http.StatusSwitchingProtocols {
		cuerpo, _ := io.ReadAll(resp.Body)
		resp.Body = io.NopCloser(bytes.NewReader(cuerpo))
		return nil, resp
	}
	if resp.Header.Get("Sec-WebSocket-Accept") != "s3pPLMBiTxaQ9kYGzzhZRbK+xOo=" || !strings.EqualFold(resp.Header.Get("Upgrade"), "websocket") ||
		!strings.EqualFold(resp.Header.Get("Connection"), "upgrade") {
		t.Fatalf("101 mal formado: %v", resp.Header)
	}
	return &clienteWSPrueba{conn: conn, r: r}, resp
}

func TestApretonDeManosDelWebSocket(t *testing.T) {
	e := nuevoEntorno(t, func(c *Config) { c.OrigenesCORS = []string{"http://localhost:5173"} })
	ts := httptest.NewServer(e.srv)
	defer ts.Close()
	admin, usuario := e.token(t, "admin", "clave-admin"), e.token(t, "demo", "demo")

	rechazo := func(nombre, ruta string, cab map[string]string, codigo int, mensaje string) *http.Response {
		t.Helper()
		c, resp := abrirWS(t, ts.URL, ruta, cab)
		if c != nil {
			t.Fatalf("%s: se abrió el WebSocket", nombre)
		}
		var cuerpo struct{ Error string }
		json.NewDecoder(resp.Body).Decode(&cuerpo)
		if resp.StatusCode != codigo || !strings.Contains(cuerpo.Error, mensaje) || !strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") {
			t.Fatalf("%s: %d %q; se esperaba %d con %q", nombre, resp.StatusCode, cuerpo.Error, codigo, mensaje)
		}
		return resp
	}
	rechazo("sin token", "/ws", nil, 401, msgNoAutenticado)
	rechazo("token inválido", "/ws?token=abc.def.ghi", nil, 401, msgNoAutenticado)
	rechazo("subprotocolo sin token", "/ws", map[string]string{"Sec-WebSocket-Protocol": "aire.jwt"}, 401, msgNoAutenticado)
	rechazo("token inválido en el subprotocolo", "/ws", map[string]string{"Sec-WebSocket-Protocol": "aire.jwt, abc.def.ghi"}, 401, msgNoAutenticado)
	rechazo("token en otro subprotocolo", "/ws", map[string]string{"Sec-WebSocket-Protocol": "chat, " + admin}, 401, msgNoAutenticado)
	rechazo("otro origen", "/ws?token="+admin, map[string]string{"Origin": "http://otro.example"}, 403, "origen")
	if resp := rechazo("otra versión", "/ws?token="+admin, map[string]string{"Sec-WebSocket-Version": "8"}, 400, "13"); resp.Header.Get("Sec-WebSocket-Version") != "13" {
		t.Fatal("el 400 por versión tiene que decir cuál habla el servidor")
	}
	rechazo("sin Upgrade", "/ws?token="+admin, map[string]string{"Upgrade": "", "Connection": "keep-alive"}, 400, "WebSocket")
	rechazo("clave mala", "/ws?token="+admin, map[string]string{"Sec-WebSocket-Key": "YWJj"}, 400, "Sec-WebSocket-Key")
	// Un GET común a /ws es un 400 en JSON, no un cuelgue; y POST, un 405.
	if w := e.pedir(t, "GET", "/ws", admin, ""); w.Code != 400 {
		t.Fatalf("GET /ws sin apretón: %d", w.Code)
	}
	if w := e.pedir(t, "POST", "/ws", admin, ""); w.Code != 405 {
		t.Fatalf("POST /ws: %d", w.Code)
	}

	// Se abre con el token en la consulta (el navegador), en Authorization (un
	// cliente que puede poner encabezados), desde un origen permitido y desde
	// el mismo host.
	clientes := []*clienteWSPrueba{}
	for nombre, caso := range map[string]struct {
		ruta string
		cab  map[string]string
	}{
		"token en la consulta": {"/ws?token=" + admin, nil},
		"Authorization":        {"/ws", map[string]string{"Authorization": "Bearer " + admin}},
		"origen permitido":     {"/ws?token=" + admin, map[string]string{"Origin": "http://localhost:5173"}},
		"mismo host":           {"/ws?token=" + admin, map[string]string{"Origin": ts.URL}},
	} {
		c, resp := abrirWS(t, ts.URL, caso.ruta, caso.cab)
		if c == nil {
			t.Fatalf("%s: %d", nombre, resp.StatusCode)
		}
		if resp.Header.Get("Sec-WebSocket-Protocol") != "" {
			t.Fatalf("%s: el servidor confirmó un subprotocolo que nadie ofreció: %v", nombre, resp.Header)
		}
		clientes = append(clientes, c)
	}
	// El navegador: el token va en Sec-WebSocket-Protocol, detrás de aire.jwt.
	// El 101 confirma aire.jwt (un navegador corta si no) y nunca el token.
	for _, ofrecidos := range []string{"aire.jwt, " + admin, "chat,aire.jwt," + admin} {
		c, resp := abrirWS(t, ts.URL, "/ws", map[string]string{"Sec-WebSocket-Protocol": ofrecidos, "Origin": "http://localhost:5173"})
		if c == nil || resp.Header.Get("Sec-WebSocket-Protocol") != "aire.jwt" {
			t.Fatalf("token en el subprotocolo (%q): %v", ofrecidos[:12], resp)
		}
		clientes = append(clientes, c)
	}
	// Quien ofrece aire.jwt pero trae el token en la consulta también recibe la confirmación.
	if c, resp := abrirWS(t, ts.URL, "/ws?token="+admin, map[string]string{"Sec-WebSocket-Protocol": "aire.jwt"}); c == nil ||
		resp.Header.Get("Sec-WebSocket-Protocol") != "aire.jwt" {
		t.Fatalf("aire.jwt ofrecido con el token en la consulta: %v", resp)
	} else {
		clientes = append(clientes, c)
	}
	// Un usuario común también entra (spec §6.1), por cualquiera de los tres caminos.
	comun, resp := abrirWS(t, ts.URL, "/ws", map[string]string{"Sec-WebSocket-Protocol": "aire.jwt, " + usuario})
	if comun == nil {
		t.Fatalf("un usuario común tiene que poder abrir /ws: %d", resp.StatusCode)
	}
	// Quedaron anotados antes del 101: lo que se difunde ahora les llega a
	// todos los admin. Los eventos de un entrenamiento son solo para ellos: el
	// usuario común no los recibe, y sí lo que es para todos.
	if err := e.srv.hub.Difundir(eventoRonda, map[string]int{"iter": 1}, rolesEventosEntrenamiento...); err != nil {
		t.Fatal(err)
	}
	if err := e.srv.hub.Difundir("para.todos", 1); err != nil {
		t.Fatal(err)
	}
	for _, c := range clientes {
		if got := c.leerTexto(t); got != `{"evento":"entrenamiento.ronda","datos":{"iter":1}}` {
			t.Fatalf("trama = %q", got)
		}
	}
	if got := comun.leerTexto(t); got != `{"evento":"para.todos","datos":1}` {
		t.Fatalf("lo primero que recibe el usuario común es %q: los eventos de entrenamiento son de admin", got)
	}
	// El apretón queda medido como una ruta más, sin la duración de la conexión.
	if mt := objeto(t, e.pedir(t, "GET", "/api/metricas", admin, ""), 200); mt["endpoints"].(map[string]any)["GET /ws"] == nil {
		t.Fatalf("métricas sin GET /ws: %v", mt["endpoints"])
	}
}

// ReadTimeout y WriteTimeout del servidor no cortan un WebSocket abierto.
func TestElWebSocketSobreviveALosPlazosDelServidor(t *testing.T) {
	e := nuevoEntorno(t)
	ts := httptest.NewUnstartedServer(e.srv)
	ts.Config.ReadTimeout, ts.Config.WriteTimeout, ts.Config.IdleTimeout = 80*time.Millisecond, 80*time.Millisecond, 80*time.Millisecond
	ts.Start()
	defer ts.Close()
	c, resp := abrirWS(t, ts.URL, "/ws?token="+e.token(t, "admin", "clave-admin"), nil)
	if c == nil {
		t.Fatalf("no abrió: %d", resp.StatusCode)
	}
	for i := range 3 {
		time.Sleep(150 * time.Millisecond) // más que cualquiera de los plazos
		if err := e.srv.hub.Difundir("x", i); err != nil {
			t.Fatal(err)
		}
		if got := c.leerTexto(t); got != fmt.Sprintf(`{"evento":"x","datos":%d}`, i) {
			t.Fatalf("trama %d = %q", i, got)
		}
	}
	c.mandar(opCierre, []byte{0x03, 0xE8})
	if codigo, _ := c.leerCierre(t); codigo != cierreNormal {
		t.Fatalf("cierre = %d", codigo)
	}
}

// De punta a punta dentro del proceso: la API con su pool, nodos de verdad por
// loopback, un entrenamiento lanzado por HTTP, las rondas recibidas por un
// cliente WebSocket y el detalle final por GET. Las rondas llegan a los DOS
// consumidores: lo que vio el WebSocket es lo que quedó en el almacén.
func TestDePuntaAPuntaLasRondasLleganAlWebSocketYAlAlmacen(t *testing.T) {
	for _, nodos := range []int{1, 3} {
		t.Run(fmt.Sprintf("%d nodos", nodos), func(t *testing.T) {
			ec := nuevoEntornoCluster(t)
			ts := httptest.NewServer(ec.srv)
			defer ts.Close()
			ec.lanzarNodos(t, nodos)
			ws, _ := abrirWS(t, ts.URL, "/ws?token="+ec.admin, nil)
			otro, _ := abrirWS(t, ts.URL, "/ws", map[string]string{"Sec-WebSocket-Protocol": "aire.jwt, " + ec.admin}) // dos clientes: los dos reciben todo
			comun, _ := abrirWS(t, ts.URL, "/ws?token="+ec.token(t, "demo", "demo"), nil)
			if ws == nil || otro == nil || comun == nil {
				t.Fatal("no abrió el WebSocket")
			}

			resp, err := http.Post(ts.URL+"/api/entrenamientos", "application/json",
				strings.NewReader(fmt.Sprintf(`{"nodos": %d, "replicas": 1, "semilla": 42}`, nodos)))
			if err != nil || resp.StatusCode != 401 {
				t.Fatalf("sin token: %v, %v", resp, err)
			}
			pedido, _ := http.NewRequest("POST", ts.URL+"/api/entrenamientos",
				strings.NewReader(fmt.Sprintf(`{"nodos": %d, "replicas": 1, "semilla": 42}`, nodos)))
			pedido.Header.Set("Authorization", "Bearer "+ec.admin)
			resp, err = http.DefaultClient.Do(pedido)
			if err != nil || resp.StatusCode != http.StatusAccepted {
				t.Fatalf("POST: %v, %v", resp, err)
			}
			var aceptado struct{ ID, Estado string }
			json.NewDecoder(resp.Body).Decode(&aceptado)
			resp.Body.Close()

			type sobre struct {
				Evento string         `json:"evento"`
				Datos  map[string]any `json:"datos"`
			}
			leerHastaElFin := func(c *clienteWSPrueba) (rondas []map[string]any, fin map[string]any) {
				for {
					var s sobre
					texto := c.leerTexto(t)
					if err := json.Unmarshal([]byte(texto), &s); err != nil || s.Datos["id"] != aceptado.ID {
						t.Fatalf("trama %q: %v", texto, err)
					}
					switch s.Evento {
					case eventoRonda:
						rondas = append(rondas, s.Datos)
					case eventoFin:
						return rondas, s.Datos
					default:
						t.Fatalf("evento desconocido: %q", texto)
					}
				}
			}
			rondas, fin := leerHastaElFin(ws)
			rondasOtro, finOtro := leerHastaElFin(otro)

			// El detalle final, pedido apenas llega el fin: ya está cerrado y completo.
			detalle, _ := http.NewRequest("GET", ts.URL+"/api/entrenamientos/"+aceptado.ID, nil)
			detalle.Header.Set("Authorization", "Bearer "+ec.admin)
			resp, err = http.DefaultClient.Do(detalle)
			if err != nil || resp.StatusCode != 200 {
				t.Fatalf("GET: %v, %v", resp, err)
			}
			var det map[string]any
			json.NewDecoder(resp.Body).Decode(&det)
			resp.Body.Close()

			ref := ec.referencia(t, 42)
			if det["estado"] != "terminado" || det["iteraciones"] != float64(ref.Iteraciones) {
				t.Fatalf("detalle = %v", det)
			}
			if len(fin) != 6 || fin["iteraciones"] != det["iteraciones"] || fin["paro"] != det["paro"] ||
				fin["dif_secuencial"] != det["dif_secuencial"] || fin["ms_total"] != det["ms_total"] ||
				fin["modelo_id"] != "entrenado-"+aceptado.ID || det["modelo_id"] != fin["modelo_id"] {
				t.Fatalf("el evento de fin %v no es el resumen del detalle %v", fin, det)
			}
			enAlmacen := det["rondas"].([]any)
			if len(rondas) != ref.Iteraciones || len(enAlmacen) != len(rondas) || len(rondasOtro) != len(rondas) || fmt.Sprint(finOtro) != fmt.Sprint(fin) {
				t.Fatalf("%d rondas por el WebSocket, %d por el otro cliente, %d en el almacén y %d iteraciones",
					len(rondas), len(rondasOtro), len(enAlmacen), ref.Iteraciones)
			}
			for i, r := range rondas {
				if len(r) != 6 || r["iter"] != float64(i+1) || len(r["ms_por_nodo"].(map[string]any)) != nodos {
					t.Fatalf("evento de la ronda %d = %v; se esperaba EventoRonda (la ronda con su id)", i+1, r)
				}
				delete(r, "id")
				if fmt.Sprint(r) != fmt.Sprint(enAlmacen[i]) || fmt.Sprint(rondasOtro[i]["inercia"]) != fmt.Sprint(r["inercia"]) {
					t.Fatalf("ronda %d: por el WebSocket %v y en el almacén %v", i+1, r, enAlmacen[i])
				}
			}
			// El usuario común estuvo conectado todo el entrenamiento y no vio nada de él.
			ec.srv.hub.Difundir("para.todos", 1)
			if got := comun.leerTexto(t); got != `{"evento":"para.todos","datos":1}` {
				t.Fatalf("el usuario común recibió %q de un entrenamiento, que es de admin", got)
			}
			m := ec.srv.modelo.Load()
			if m.ID != "entrenado-"+aceptado.ID {
				t.Fatalf("modelo activo = %s", m.ID)
			}
			if nodos == 1 && !mismosBits(m.CentroidesEstandarizados(), ref.Centroides) {
				t.Fatal("con un nodo el modelo tiene que ser el concurrente bit a bit")
			}
		})
	}
}

// Apagar con un entrenamiento en curso: queda abortado en el almacén, su fin
// sale por el WebSocket y después el cliente recibe el cierre 1001.
func TestApagarAbortaElEntrenamientoEnCursoYDespideAlWebSocket(t *testing.T) {
	ent, ruta := entrenadorSintetico(t)
	ent.PlazoRonda = time.Minute
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	base := nuevoEntorno(t) // solo para reusar su configuración armada
	cfg := Config{
		Modelo: base.srv.modelo.Load(), Firmador: base.srv.firmador, Usuarios: base.srv.usuarios,
		Catalogo: []almacen.Estacion{{ID: "CA-AYAS-01"}}, Estaciones: base.srv.estaciones, Predicciones: base.pred, Cache: base.cache,
		Mongo: base.cache, Redis: base.cache, OpenAPI: []byte("x"), TamCola: 4, Log: logMudo,
		Entrenamientos: base.ent, Eventos: base.ev, Nodos: ln, Entrenador: ent,
		Pool: cluster.OpcionesPool{PlazoHola: time.Second, CadaLatido: 20 * time.Millisecond, PlazoLatido: 500 * time.Millisecond},
	}
	srv, err := Nuevo(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx, apagar := context.WithCancel(context.Background())
	coordinado := make(chan struct{})
	go func() { srv.Coordinar(ctx); close(coordinado) }()
	defer func() { apagar(); <-coordinado }()
	ec := &entornoCluster{entorno: &entorno{srv: srv, pred: base.pred, cache: base.cache, ent: base.ent, ev: base.ev},
		ent: ent, goldRuta: ruta, dirNodos: ln.Addr().String()}
	ec.admin = ec.token(t, "admin", "clave-admin")
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ec.lanzarNodos(t, 1)
	ec.nodoMudo(t, "nodo-2-mudo")
	ec.esperarLibres(t, 2)
	ws, _ := abrirWS(t, ts.URL, "/ws?token="+ec.admin, nil)
	id := ec.entrenar(t, `{"nodos": 2, "replicas": 1, "semilla": 42}`)

	inicio := time.Now()
	apagar()
	select {
	case <-coordinado:
	case <-time.After(10 * time.Second):
		t.Fatal("Coordinar no volvió: el entrenamiento en curso no se cortó")
	}
	if d := time.Since(inicio); d > 5*time.Second {
		t.Fatalf("apagar tardó %v con un plazo de ronda de un minuto", d)
	}
	doc, err := base.ent.Obtener(context.Background(), id)
	if err != nil || doc.Estado != almacen.EstadoAbortado || !strings.Contains(doc.Error, "apagando") || strings.Contains(doc.Error, "mudo") {
		t.Fatalf("documento = %+v, %v", doc, err)
	}
	if got := ws.leerTexto(t); !strings.Contains(got, `"evento":"entrenamiento.fin"`) || !strings.Contains(got, "apagando") || !strings.Contains(got, id) {
		t.Fatalf("antes de despedirse tiene que llegar el fin del entrenamiento: %q", got)
	}
	if codigo, _ := ws.leerCierre(t); codigo != cierreSeVa {
		t.Fatalf("cierre = %d", codigo)
	}
	// Ya apagado: no se lanza otro ni se abre otro WebSocket.
	mensajeDeError(t, ec.pedir(t, "POST", "/api/entrenamientos", ec.admin, `{"nodos": 1, "replicas": 1, "semilla": 42}`),
		http.StatusServiceUnavailable, errApagando.Error())
	if c, resp := abrirWS(t, ts.URL, "/ws?token="+ec.admin, nil); c != nil || resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("un WebSocket después de apagar: %v", resp)
	}
}

// Con el gold real y un nodo, el modelo es BIT A BIT el de reports/kmeans.json
// (cmd/entrenar). El gold no se versiona: sin él la prueba se salta.
func TestConElGoldRealYUnNodoElModeloEsElDeKmeansJSONBitABit(t *testing.T) {
	const gold = "../data/gold/oefa_features.csv"
	if _, err := os.Stat(gold); err != nil {
		t.Skip("sin el gold real (data/gold/oefa_features.csv no se versiona)")
	}
	if testing.Short() {
		t.Skip("-short: no se carga el gold real")
	}
	ent, err := LeerEntrenador("../configs/kmeans.json", "../reports/estandarizacion.json", gold, "../reports/centroides_iniciales.json")
	if err != nil {
		t.Fatal(err)
	}
	var ref struct {
		Gold struct {
			Sha256 string `json:"sha256"`
		} `json:"gold"`
		Semilla     int64       `json:"semilla"`
		Iteraciones int         `json:"iteraciones"`
		Inercia     float64     `json:"inercia"`
		Inercias    []float64   `json:"inercias_por_iteracion"`
		Centroides  [][]float64 `json:"centroides_estandarizados"`
	}
	b, err := os.ReadFile("../reports/kmeans.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &ref); err != nil {
		t.Fatal(err)
	}
	if ref.Gold.Sha256 != ent.Sha256 {
		t.Skipf("el gold local (%.12s…) no es el de reports/kmeans.json (%.12s…)", ent.Sha256, ref.Gold.Sha256)
	}
	if ent.Inicios == nil {
		t.Fatal("no se leyeron los centroides iniciales guardados")
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	ec := &entornoCluster{ent: ent, goldRuta: gold, dirNodos: ln.Addr().String()}
	ec.entorno = nuevoEntorno(t, func(c *Config) {
		c.Nodos, c.Entrenador = ln, ent
		c.Pool = cluster.OpcionesPool{PlazoHola: time.Second, CadaLatido: 50 * time.Millisecond, PlazoLatido: time.Second}
	})
	ec.admin = ec.token(t, "admin", "clave-admin")
	// El nodo, como cmd/nodo por omisión: el chunk de la receta.
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	nd := &cluster.Nodo{Nombre: "nodo-1", Workers: 4, Gold: gold, Log: logNodos}
	go nd.Correr(ctx, func(ctx context.Context) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, "tcp", ec.dirNodos)
	}, 5*time.Millisecond, false)
	ec.esperarLibres(t, 1)

	id := ec.entrenar(t, fmt.Sprintf(`{"nodos": 1, "replicas": 1, "semilla": %d}`, ref.Semilla))
	var det map[string]any
	limite := time.Now().Add(60 * time.Second)
	for {
		det = objeto(t, ec.pedir(t, "GET", "/api/entrenamientos/"+id, ec.admin, ""), 200)
		if det["estado"] != "en_curso" {
			break
		}
		if time.Now().After(limite) {
			t.Fatal("el entrenamiento sobre el gold real no terminó en un minuto")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if det["estado"] != "terminado" || det["iteraciones"] != float64(ref.Iteraciones) {
		t.Fatalf("detalle = %v; reports/kmeans.json tiene %d iteraciones", det["estado"], ref.Iteraciones)
	}
	m := ec.srv.modelo.Load()
	if !mismosBits(m.CentroidesEstandarizados(), slices.Concat(ref.Centroides...)) {
		t.Fatalf("centroides %v; reports/kmeans.json tiene %v", m.CentroidesEstandarizados(), ref.Centroides)
	}
	if math.Float64bits(m.Inercia) != math.Float64bits(ref.Inercia) {
		t.Fatalf("inercia %v; reports/kmeans.json tiene %v", m.Inercia, ref.Inercia)
	}
	for i, x := range det["rondas"].([]any) {
		if got := x.(map[string]any)["inercia"].(float64); math.Float64bits(got) != math.Float64bits(ref.Inercias[i]) {
			t.Fatalf("ronda %d: inercia %v; reports/kmeans.json tiene %v", i+1, got, ref.Inercias[i])
		}
	}
	t.Logf("gold real, 1 nodo: %d iteraciones, inercia %v, dif_secuencial %v, ms_total %v: bit a bit con reports/kmeans.json",
		ref.Iteraciones, m.Inercia, det["dif_secuencial"], det["ms_total"])
}

// nodoFalso es un nodo escrito a mano para las pruebas: se presenta, responde
// los PING y, según cómo se arme, carga su rango y calcula sus PARCIAL. Con
// cargar en false nunca confirma el CARGAR; con mentira distinta de 1 calcula
// bien y multiplica sus sumas, que siguen bien formadas y con los conteos de su
// rango: nada del protocolo lo delata.
func (ec *entornoCluster) nodoFalso(t *testing.T, nombre string, cargar bool, mentira float64) {
	t.Helper()
	conn, err := net.Dial("tcp", ec.dirNodos)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	go func() {
		cod, dec := protocolo.NuevoCodificador(conn), protocolo.NuevoDecodificador(conn)
		cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Hola, Version: protocolo.Version, Nodo: nombre, Workers: 1})
		var datos *kmeans.Datos
		for {
			m, err := dec.Leer()
			if err != nil {
				return
			}
			switch {
			case m.Tipo == protocolo.Ping:
				cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Pong})
			case m.Tipo == protocolo.Cargar && cargar:
				datos = cluster.FilasVirtuales(ec.ent.Gold.Datos, *m.Rango)
				cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Listo, N: datos.N, Huella: m.Huella})
			case m.Tipo == protocolo.Iterar:
				p := kmeans.ParcialesPorChunks(datos, m.Centroides, 1, chunkDePrueba)
				for i := range p.Sumas {
					p.Sumas[i] *= mentira
				}
				cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Parcial, Iter: m.Iter, Sumas: p.Sumas, Conteos: p.Conteos, Inercia: p.Inercia})
			}
		}
	}()
}

// Un nodo que devuelve PARCIAL bien formados pero falsos lleva la corrida a
// otros centroides sin que nada falle. El entrenamiento termina, con su
// dif_secuencial a la vista, pero el modelo NO se activa: sigue el anterior.
func TestUnNodoQueMienteNoActivaElModelo(t *testing.T) {
	ec := nuevoEntornoCluster(t)
	ec.lanzarNodos(t, 1)
	ec.nodoFalso(t, "nodo-2-mentiroso", true, 1.5)
	ec.esperarLibres(t, 2)
	antes := ec.srv.modelo.Load()
	ts := httptest.NewServer(ec.srv)
	defer ts.Close()
	ws, _ := abrirWS(t, ts.URL, "/ws?token="+ec.admin, nil)

	id := ec.entrenar(t, `{"nodos": 2, "replicas": 1, "semilla": 42}`)
	det := ec.esperarFin(t, id)
	if det["estado"] != "terminado" {
		t.Fatalf("nada del protocolo falla: el entrenamiento termina. Detalle: %v", det)
	}
	dif, _ := det["dif_secuencial"].(float64)
	aviso, _ := det["aviso"].(string)
	if dif <= cluster.Tolerancia {
		t.Fatalf("dif_secuencial = %g: la mentira tenía que notarse", dif)
	}
	if _, hay := det["modelo_id"]; hay || !strings.Contains(aviso, "no se activó") || !strings.Contains(aviso, "dif_secuencial") {
		t.Fatalf("no tiene que activar el modelo y tiene que decir por qué: %v", det)
	}
	if _, hay := det["error"]; hay {
		t.Fatalf("no es un aborto: no trae error: %v", det)
	}
	if ec.srv.modelo.Load() != antes {
		t.Fatalf("se activó el modelo %s con dif_secuencial %g", ec.srv.modelo.Load().ID, dif)
	}
	// El fin por el WebSocket y el historial dicen lo mismo.
	var fin map[string]any
	for fin == nil {
		var sobre struct {
			Evento string         `json:"evento"`
			Datos  map[string]any `json:"datos"`
		}
		json.Unmarshal([]byte(ws.leerTexto(t)), &sobre)
		if sobre.Evento == eventoFin {
			fin = sobre.Datos
		}
	}
	if fin["aviso"] != aviso || fin["modelo_id"] != nil || fin["dif_secuencial"] != dif {
		t.Fatalf("evento de fin = %v", fin)
	}
	doc, _ := ec.entorno.ent.Obtener(context.Background(), id)
	if doc.ModeloID != "" || doc.Aviso != aviso || doc.Estado != almacen.EstadoTerminado {
		t.Fatalf("documento = %+v", doc)
	}
	// Y se sigue prediciendo con el modelo de antes.
	if pred := objeto(t, ec.pedir(t, "POST", "/api/predicciones", ec.admin, lecturaEjemplo), 200); pred["modelo_id"] != antes.ID {
		t.Fatalf("la predicción usó %v", pred["modelo_id"])
	}
}

// Apagar mientras los nodos cargan su rango: no se espera al plazo de la carga
// (30 s por omisión). El entrenamiento queda abortado por la cancelación, no
// por un plazo vencido, y durante el apagado un pedido nuevo recibe 503, no 409.
func TestApagarDuranteLaCargaNoEsperaElPlazo(t *testing.T) {
	ent, ruta := entrenadorSintetico(t)
	ent.PlazoCarga = time.Minute
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	base := nuevoEntorno(t) // solo para reusar su configuración armada
	srv, err := Nuevo(Config{
		Modelo: base.srv.modelo.Load(), Firmador: base.srv.firmador, Usuarios: base.srv.usuarios,
		Catalogo: []almacen.Estacion{{ID: "CA-AYAS-01"}}, Estaciones: base.srv.estaciones, Predicciones: base.pred, Cache: base.cache,
		Mongo: base.cache, Redis: base.cache, OpenAPI: []byte("x"), TamCola: 4, Log: logMudo,
		Entrenamientos: base.ent, Eventos: base.ev, Nodos: ln, Entrenador: ent,
		Pool: cluster.OpcionesPool{PlazoHola: time.Second, CadaLatido: 20 * time.Millisecond, PlazoLatido: 500 * time.Millisecond},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, apagar := context.WithCancel(context.Background())
	coordinado := make(chan struct{})
	go func() { srv.Coordinar(ctx); close(coordinado) }()
	defer func() { apagar(); <-coordinado }()
	ec := &entornoCluster{entorno: &entorno{srv: srv, pred: base.pred, cache: base.cache, ent: base.ent, ev: base.ev},
		ent: ent, goldRuta: ruta, dirNodos: ln.Addr().String()}
	ec.admin = ec.token(t, "admin", "clave-admin")
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ec.nodoFalso(t, "nodo-1-no-carga", false, 1)
	ec.esperarLibres(t, 1)
	const pedido = `{"nodos": 1, "replicas": 1, "semilla": 42}`
	id := ec.entrenar(t, pedido)
	if det := objeto(t, ec.pedir(t, "GET", "/api/entrenamientos/"+id, ec.admin, ""), 200); det["estado"] != "en_curso" {
		t.Fatalf("mientras el nodo no confirma la carga está en curso: %v", det)
	}

	// Empezó el apagado y el entrenamiento todavía no se cortó: 503, no 409, y
	// no se abren WebSockets.
	srv.apagando.Store(true)
	mensajeDeError(t, ec.pedir(t, "POST", "/api/entrenamientos", ec.admin, pedido), http.StatusServiceUnavailable, errApagando.Error())
	if c, resp := abrirWS(t, ts.URL, "/ws?token="+ec.admin, nil); c != nil || resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("un WebSocket durante el apagado: %v", resp)
	}

	inicio := time.Now()
	apagar()
	select {
	case <-coordinado:
	case <-time.After(10 * time.Second):
		t.Fatal("Coordinar no volvió: el apagado esperó a la carga")
	}
	if d := time.Since(inicio); d > 3*time.Second {
		t.Fatalf("apagar durante la carga tardó %v con un plazo de carga de un minuto", d)
	}
	doc, err := base.ent.Obtener(context.Background(), id)
	if err != nil || doc.Estado != almacen.EstadoAbortado || !strings.Contains(doc.Error, "cancelada") || !strings.Contains(doc.Error, "apagando") ||
		strings.Contains(doc.Error, "plazo vencido") || strings.Contains(doc.Error, "nodo-1-no-carga") {
		t.Fatalf("tiene que quedar abortado por la cancelación, sin culpar al nodo ni a un plazo: %+v, %v", doc, err)
	}
}
