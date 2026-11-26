//go:build integracion

// La API entera contra MongoDB y Redis reales, los de docker-compose.yml:
//
//	docker compose up -d mongo redis seed
//	go test -race -tags integracion ./...
//
// MONGO_URI (mongodb://localhost:27017) y REDIS_ADDR (localhost:6379) dicen
// dónde están. Escribe en una base de MongoDB propia, que borra al terminar, y
// en la base 14 de Redis, que vacía; las de la API (aire_distribuido y la 0)
// no se tocan.
package main

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/R0SEWT/aire-distribuido/almacen"
	"github.com/R0SEWT/aire-distribuido/almacen/mongo"
	"github.com/R0SEWT/aire-distribuido/almacen/redis"
)

const dbRedisDePruebas = 14

// levantar corre la API con las bases reales y devuelve su URL, su log y cómo
// apagarla (lo que devuelve correr al terminar).
func levantar(t *testing.T, vars map[string]string) (base string, log *bytes.Buffer, apagar func() error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	log = &bytes.Buffer{}
	dir, fin := make(chan string, 1), make(chan error, 1)
	go func() {
		fin <- correr(ctx, append([]string{"-addr", "127.0.0.1:0", "-demo"}, argsDelRepo...),
			entornoCon(vars), &bufferSeguro{b: log}, func(d, _ string) { dir <- d })
	}()
	apagado := false
	apagar = func() error {
		if apagado {
			return nil
		}
		apagado = true
		cancel()
		select {
		case err := <-fin:
			return err
		case <-time.After(10 * time.Second):
			return fmt.Errorf("la API no se apagó en 10 s")
		}
	}
	t.Cleanup(func() { apagar() })
	select {
	case d := <-dir:
		return "http://" + d, log, apagar
	case err := <-fin:
		t.Fatalf("correr terminó antes de escuchar: %v\n%s", err, log)
	case <-time.After(15 * time.Second):
		t.Fatal("la API no empezó a escuchar")
	}
	return "", nil, nil
}

func TestLaAPIGuardaEnMongoDBYRedisReales(t *testing.T) {
	ctx := context.Background()
	uri, addr := cmp.Or(os.Getenv("MONGO_URI"), "mongodb://localhost:27017"), cmp.Or(os.Getenv("REDIS_ADDR"), "localhost:6379")
	baseMongo := fmt.Sprintf("aire_integracion_api_%d", time.Now().UnixNano())
	vars := map[string]string{
		"JWT_SECRETO": secreto, "MONGO_URI": uri, "MONGO_DB": baseMongo,
		"REDIS_ADDR": addr, "REDIS_DB": fmt.Sprint(dbRedisDePruebas),
	}
	// Las mismas bases, vistas desde la prueba.
	m, err := mongo.Conectar(uri, baseMongo)
	if err != nil {
		t.Fatal(err)
	}
	r, err := redis.Conectar(redis.Opciones{Addr: addr, DB: dbRedisDePruebas})
	if err != nil {
		t.Fatal(err)
	}
	if err := cmp.Or(m.Ping(ctx), r.Ping(ctx)); err != nil {
		t.Fatalf("las bases no responden (docker compose up -d mongo redis seed): %v", err)
	}
	if err := r.Vaciar(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := cmp.Or(m.BorrarBase(ctx), r.Vaciar(ctx)); err != nil {
			t.Errorf("no se limpiaron las bases de la prueba: %v", err)
		}
		m.Cerrar(ctx)
		r.Cerrar()
	})

	// Primer arranque: no hay nada guardado, el modelo sale de reports/.
	base, log, apagar := levantar(t, vars)
	salud := pedir(t, "GET", base+"/api/salud", "", "")
	if salud["mongo"] != "ok" || salud["redis"] != "ok" || salud["modelo_activo"] != "sembrado-kmeans-json" {
		t.Fatalf("salud = %v", salud)
	}
	tok := pedir(t, "POST", base+"/api/auth/token", "", `{"usuario": "demo", "clave": "demo"}`)["token"].(string)
	// 299 horas seguidas de una estación y, al final, una lectura atrasada de
	// la hora 10.
	const total = 300
	lima := time.FixedZone("-05", -5*3600)
	hora := func(h int) time.Time { return time.Date(2026, 11, 1, h, 0, 0, 0, lima) }
	predecir := func(h int) {
		pedir(t, "POST", base+"/api/predicciones", tok,
			fmt.Sprintf(`{"estacion": "CA-AYAS-01", "fecha_hora": %q, "pm2_5": 70.2, "pm10": 190.5}`, hora(h).Format(time.RFC3339)))
	}
	for h := range total - 1 {
		predecir(h)
	}
	predecir(10)
	// Apagar vacía las colas: después, todo lo predicho está en las bases.
	if err := apagar(); err != nil {
		t.Fatalf("correr devolvió %v al apagarse\n%s", err, log)
	}
	if strings.Contains(log.String(), "level=ERROR") {
		t.Errorf("hubo errores de escritura:\n%s", log)
	}
	guardadas, err := m.Predicciones().DeEstacion(ctx, "CA-AYAS-01", total+1)
	if err != nil || len(guardadas) != total {
		t.Fatalf("%d predicciones en MongoDB (%v), se esperaban %d", len(guardadas), err, total)
	}
	// La última que se predijo es la atrasada; la anterior, la de la hora 298,
	// con su media de 24 h completa.
	if p := guardadas[0]; !p.FechaHora.Equal(hora(10)) || p.Regimen != 1 || !p.ExcedeECA || p.PM25 != 70.2 || p.ModeloID != "sembrado-kmeans-json" {
		t.Errorf("la última predicción guardada = %+v", p)
	}
	if p := guardadas[1]; !p.FechaHora.Equal(hora(298)) || !p.Alerta || p.Media24h.Lecturas != 24 || p.INCA == nil {
		t.Errorf("la predicción de la hora 298 = %+v", p)
	}
	// En Redis queda la hora más reciente, no la última en llegar.
	reg, err := r.Regimen(ctx, "CA-AYAS-01")
	if err != nil || reg.Regimen != 1 || !reg.Alerta || reg.Media24h.Lecturas != 24 || !reg.FechaHora.Equal(hora(298)) {
		t.Fatalf("estacion:CA-AYAS-01:regimen = %+v, %v; se esperaba la lectura de %v", reg, err, hora(298))
	}
	// El modelo con que arrancó quedó guardado como activo en las dos bases.
	enRedis, err1 := r.ModeloActivo(ctx)
	enMongo, err2 := m.Modelos().Activo(ctx)
	if err1 != nil || err2 != nil || enRedis.ID != "sembrado-kmeans-json" || enMongo.ID != enRedis.ID || !enMongo.Activo || enMongo.GoldSHA256 == "" {
		t.Fatalf("modelo:activo = %q (%v), modelos activo = %q (%v)", enRedis.ID, err1, enMongo.ID, err2)
	}

	// La API anterior «murió» con un entrenamiento en curso: queda en MongoDB.
	huerfano := almacen.Entrenamiento{ID: "e-20261120-0520", Inicio: time.Date(2026, 11, 20, 10, 20, 0, 0, time.UTC),
		Estado: almacen.EstadoEnCurso, Parametros: almacen.ParametrosEntrenamiento{Nodos: 3, Replicas: 1, Semilla: 42}}
	if err := m.Entrenamientos().Crear(ctx, huerfano); err != nil {
		t.Fatal(err)
	}
	if err := m.Entrenamientos().AgregarRonda(ctx, huerfano.ID, almacen.Ronda{Iter: 1, Inercia: 73075.5}); err != nil {
		t.Fatal(err)
	}

	// Segundo arranque, con un modelo entrenado en Redis: arranca con ese.
	enRedis.ID, enRedis.Origen = "m-e-20261119-1530", "e-20261119-1530"
	if err := r.GuardarModeloActivo(ctx, enRedis); err != nil {
		t.Fatal(err)
	}
	base, log, apagar = levantar(t, vars)
	if salud := pedir(t, "GET", base+"/api/salud", "", ""); salud["modelo_activo"] != "m-e-20261119-1530" {
		t.Fatalf("después de reiniciar, salud = %v; se esperaba el modelo que estaba en Redis\n%s", salud, log)
	}
	tok = pedir(t, "POST", base+"/api/auth/token", "", `{"usuario": "demo", "clave": "demo"}`)["token"].(string)
	// El régimen de la estación sobrevivió al reinicio: está en Redis, no en la memoria de la API.
	if reg := pedir(t, "GET", base+"/api/estaciones/CA-AYAS-01/regimen", tok, ""); reg["regimen"] != 1.0 || reg["fecha_hora"] != "2026-11-13T10:00:00-05:00" {
		t.Fatalf("régimen después de reiniciar = %v", reg)
	}
	// Al arrancar recuperó de MongoDB el último entrenamiento y cerró el que
	// había quedado en curso, sin tocar su ronda.
	admin := pedir(t, "POST", base+"/api/auth/token", "", `{"usuario": "admin", "clave": "admin"}`)["token"].(string)
	if mt := pedir(t, "GET", base+"/api/metricas", admin, ""); mt["ultimo_entrenamiento"] != huerfano.ID {
		t.Fatalf("ultimo_entrenamiento después de reiniciar = %v, se esperaba %s", mt["ultimo_entrenamiento"], huerfano.ID)
	}
	det := pedir(t, "GET", base+"/api/entrenamientos/"+huerfano.ID, admin, "")
	if errTexto, _ := det["error"].(string); det["estado"] != "abortado" || !strings.Contains(errTexto, "se reinició") || len(det["rondas"].([]any)) != 1 {
		t.Fatalf("el entrenamiento que quedó en curso = %v", det)
	}
	if doc, err := m.Entrenamientos().Obtener(ctx, huerfano.ID); err != nil || doc.Estado != almacen.EstadoAbortado || doc.Fin.IsZero() {
		t.Fatalf("documento del huérfano = %+v, %v", doc, err)
	}
	if err := apagar(); err != nil {
		t.Fatal(err)
	}
	if activo, err := m.Modelos().Activo(ctx); err != nil || activo.ID != "m-e-20261119-1530" {
		t.Fatalf("modelos activo = %q, %v; se esperaba el entrenado", activo.ID, err)
	}
}

// pedirLista es pedir para una ruta que responde una lista: devuelve el cuerpo
// tal cual y sus objetos.
func pedirLista(t *testing.T, url, token string) (string, []map[string]any) {
	t.Helper()
	r, _ := http.NewRequest("GET", url, nil)
	r.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	cuerpo, _ := io.ReadAll(resp.Body)
	var l []map[string]any
	if err := json.Unmarshal(cuerpo, &l); err != nil || resp.StatusCode != 200 || l == nil {
		t.Fatalf("GET %s: %d, %v: %s", url, resp.StatusCode, err, cuerpo)
	}
	return strings.TrimSpace(string(cuerpo)), l
}

// Lo que el frontend del TB2 le pide a la API, con MongoDB y Redis reales: el
// estado de las estaciones en un pedido, el evento estacion.regimen para un
// usuario (solo de lo que Redis de verdad guardó, y agrupado bajo una ráfaga),
// los eventos del clúster y el CPU del proceso.
func TestElEstadoDeLasEstacionesConLasBasesReales(t *testing.T) {
	ctx := context.Background()
	uri, addr := cmp.Or(os.Getenv("MONGO_URI"), "mongodb://localhost:27017"), cmp.Or(os.Getenv("REDIS_ADDR"), "localhost:6379")
	baseMongo := fmt.Sprintf("aire_integracion_tb2_%d", time.Now().UnixNano())
	vars := map[string]string{
		"JWT_SECRETO": secreto, "MONGO_URI": uri, "MONGO_DB": baseMongo,
		"REDIS_ADDR": addr, "REDIS_DB": fmt.Sprint(dbRedisDePruebas),
	}
	m, err := mongo.Conectar(uri, baseMongo)
	if err != nil {
		t.Fatal(err)
	}
	r, err := redis.Conectar(redis.Opciones{Addr: addr, DB: dbRedisDePruebas})
	if err != nil {
		t.Fatal(err)
	}
	if err := cmp.Or(m.Ping(ctx), r.Ping(ctx)); err != nil {
		t.Fatalf("las bases no responden (docker compose up -d mongo redis seed): %v", err)
	}
	if err := r.Vaciar(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := cmp.Or(m.BorrarBase(ctx), r.Vaciar(ctx)); err != nil {
			t.Errorf("no se limpiaron las bases de la prueba: %v", err)
		}
		m.Cerrar(ctx)
		r.Cerrar()
	})

	base, log, apagar := levantar(t, vars)
	tok := pedir(t, "POST", base+"/api/auth/token", "", `{"usuario": "demo", "clave": "demo"}`)["token"].(string)
	admin := pedir(t, "POST", base+"/api/auth/token", "", `{"usuario": "admin", "clave": "admin"}`)["token"].(string)

	// Recién levantado: ninguna estación tiene estado, y es un 200.
	if cuerpo, _ := pedirLista(t, base+"/api/regimenes", tok); cuerpo != "[]" {
		t.Fatalf("GET /api/regimenes sin predicciones = %s", cuerpo)
	}

	// El panel: primero el WebSocket, como usuario; después las lecturas.
	ws := abrirWS(t, strings.TrimPrefix(base, "http://"), "/ws?token="+tok)
	const rafaga = 200
	lima := time.FixedZone("-05", -5*3600)
	hora := func(h int) string { return time.Date(2021, 12, 13, h, 0, 0, 0, lima).Format(time.RFC3339) }
	predecir := func(estacion string, h int) {
		pedir(t, "POST", base+"/api/predicciones", tok,
			fmt.Sprintf(`{"estacion": %q, "fecha_hora": %q, "pm2_5": 70.2, "pm10": 190.5}`, estacion, hora(h)))
	}
	inicio := time.Now()
	for h := range rafaga {
		predecir("CA-AYAS-01", h)
	}
	predecir("CA-AYAS-01", 10) // más antigua que la guardada: no es el estado
	predecir("CA-MAC-01", 0)   // otra estación, después: cuando llegue su evento, la anterior ya se procesó

	// Se leen los eventos hasta tener el último estado de CA-AYAS-01 y el de
	// CA-MAC-01, y un rato más por si llegara alguno que no tiene que llegar.
	type estado struct {
		Evento string          `json:"evento"`
		Datos  json.RawMessage `json:"datos"`
	}
	var deAyas []map[string]any
	var ultimoAyas string
	vioMac := false
	leer := func(plazo time.Duration) bool {
		ws.conn.SetReadDeadline(time.Now().Add(plazo))
		var cab [2]byte
		if _, err := io.ReadFull(ws.r, cab[:]); err != nil {
			return false // se cumplió el plazo sin tramas
		}
		n := int(cab[1] & 0x7F)
		if n == 126 {
			var ext [2]byte
			io.ReadFull(ws.r, ext[:])
			n = int(ext[0])<<8 | int(ext[1])
		}
		carga := make([]byte, n)
		io.ReadFull(ws.r, carga)
		if cab[0]&0x0F != 0x1 {
			return true // un ping
		}
		var e estado
		var datos map[string]any
		if err := json.Unmarshal(carga, &e); err != nil || e.Evento != "estacion.regimen" || json.Unmarshal(e.Datos, &datos) != nil {
			t.Fatalf("un usuario solo recibe estacion.regimen: %s", carga)
		}
		switch datos["estacion"] {
		case "CA-AYAS-01":
			deAyas = append(deAyas, datos)
			ultimoAyas = string(e.Datos)
		case "CA-MAC-01":
			vioMac = true
		default:
			t.Fatalf("evento de una estación que nadie midió: %s", carga)
		}
		return true
	}
	limite := time.Now().Add(15 * time.Second)
	for !(vioMac && len(deAyas) > 0 && deAyas[len(deAyas)-1]["fecha_hora"] == hora(rafaga-1)) {
		if time.Now().After(limite) {
			t.Fatalf("no llegaron los eventos: %d de CA-AYAS-01 (último %s), CA-MAC-01 %v\n%s", len(deAyas), ultimoAyas, vioMac, log)
		}
		leer(time.Second)
	}
	duracion := time.Since(inicio)
	for leer(700 * time.Millisecond) { // casi tres intervalos sin nada nuevo
	}
	// Agrupado: de 200 lecturas salen a lo sumo una por intervalo de 250 ms.
	if tope := int(duracion/(250*time.Millisecond)) + 2; len(deAyas) > tope {
		t.Fatalf("de %d lecturas en %v salieron %d eventos de CA-AYAS-01; el tope es %d", rafaga, duracion, len(deAyas), tope)
	}
	// Ningún evento es de la lectura atrasada, y el último es el estado.
	for i, e := range deAyas {
		if i > 0 && e["fecha_hora"].(string) < deAyas[i-1]["fecha_hora"].(string) {
			t.Fatalf("el estado retrocedió en los eventos: %v después de %v", e["fecha_hora"], deAyas[i-1]["fecha_hora"])
		}
	}
	if ultima := deAyas[len(deAyas)-1]; ultima["fecha_hora"] != hora(rafaga-1) || ultima["alerta"] != true {
		t.Fatalf("el último evento de CA-AYAS-01 = %v", ultima)
	}

	// El GET dice lo mismo que el último evento, byte a byte, y en una lista
	// ordenada por estación.
	cuerpo, estados := pedirLista(t, base+"/api/regimenes", tok)
	if len(estados) != 2 || estados[0]["estacion"] != "CA-AYAS-01" || estados[1]["estacion"] != "CA-MAC-01" {
		t.Fatalf("GET /api/regimenes = %s", cuerpo)
	}
	if !strings.Contains(cuerpo, ultimoAyas) {
		t.Fatalf("el GET y el último evento no dicen lo mismo:\n evento %s\n GET    %s", ultimoAyas, cuerpo)
	}
	// Y es lo que Redis tiene: la hora más reciente, no la última en llegar.
	if reg, err := r.Regimen(ctx, "CA-AYAS-01"); err != nil || reg.FechaHora.In(lima).Format(time.RFC3339) != hora(rafaga-1) {
		t.Fatalf("estacion:CA-AYAS-01:regimen = %+v, %v", reg, err)
	}
	t.Logf("%d lecturas en %v: %d eventos estacion.regimen de CA-AYAS-01; el último: %s", rafaga, duracion.Round(time.Millisecond), len(deAyas), ultimoAyas)

	// Los eventos del clúster salen de MongoDB: sin nodos, ninguno; con tres
	// guardados, los dos más recientes primero.
	if cuerpo, _ := pedirLista(t, base+"/api/cluster/eventos", admin); cuerpo != "[]" {
		t.Fatalf("GET /api/cluster/eventos sin eventos = %s", cuerpo)
	}
	t0 := time.Date(2026, 11, 19, 15, 30, 0, 0, lima)
	for i, tipo := range []string{"alta", "baja", "alta"} {
		if err := m.Eventos().Guardar(ctx, almacen.EventoCluster{TS: t0.Add(time.Duration(i) * time.Second), Tipo: tipo, Nodo: "H45-022", Detalle: fmt.Sprint("evento ", i)}); err != nil {
			t.Fatal(err)
		}
	}
	if cuerpo, evs := pedirLista(t, base+"/api/cluster/eventos?limite=2", admin); len(evs) != 2 || evs[0]["detalle"] != "evento 2" ||
		evs[1]["detalle"] != "evento 1" || evs[1]["tipo"] != "baja" || evs[0]["ts"] != "2026-11-19T15:30:02-05:00" {
		t.Fatalf("GET /api/cluster/eventos?limite=2 = %s", cuerpo)
	}
	// El CPU y la memoria del proceso de la API, en las métricas.
	proceso, _ := pedir(t, "GET", base+"/api/metricas", admin, "")["proceso_api"].(map[string]any)
	if cpu, _ := proceso["cpu_usuario_s"].(float64); cpu <= 0 || proceso["en_marcha_s"].(float64) <= 0 || proceso["heap_mb"].(float64) <= 0 {
		t.Fatalf("proceso_api = %v", proceso)
	}

	if err := apagar(); err != nil {
		t.Fatalf("correr devolvió %v al apagarse\n%s", err, log)
	}
	if strings.Contains(log.String(), "level=ERROR") {
		t.Errorf("hubo errores:\n%s", log)
	}
}
