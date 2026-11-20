//go:build integracion

// Pruebas contra un MongoDB real: el de docker-compose.yml, ya sembrado.
//
//	docker compose up -d mongo redis seed
//	go test -race -tags integracion ./almacen/...
//
// MONGO_URI (mongodb://localhost:27017) y MONGO_DB (aire_distribuido) dicen
// dónde está. Lo que escriben va a una base aparte que se borra al terminar; de
// la base sembrada solo leen.
package mongo

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/R0SEWT/aire-distribuido/almacen"
)

var ctx = context.Background()

func entorno(variable, porOmision string) string {
	return cmp.Or(os.Getenv(variable), porOmision)
}

func conectar(t *testing.T, base string) *Almacen {
	t.Helper()
	a, err := Conectar(entorno("MONGO_URI", "mongodb://localhost:27017"), base)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Cerrar(ctx) })
	if err := a.Ping(ctx); err != nil {
		t.Fatalf("MongoDB no responde (docker compose up -d mongo redis seed): %v", err)
	}
	return a
}

// sembrada es la base que cargó docs/datos/seed.js.
func sembrada(t *testing.T) *Almacen {
	return conectar(t, entorno("MONGO_DB", "aire_distribuido"))
}

// temporal es una base vacía, propia de la prueba, que se borra al terminar.
func temporal(t *testing.T) *Almacen {
	t.Helper()
	a := conectar(t, fmt.Sprintf("aire_integracion_%d_%d", os.Getpid(), time.Now().UnixNano()))
	t.Cleanup(func() {
		if err := a.BorrarBase(ctx); err != nil {
			t.Errorf("no se borró la base temporal %s: %v", a.db.Name(), err)
		}
	})
	return a
}

// Lo que la API lee de MongoDB es, estación por estación, lo que lee de la
// ficha: el seed y LeerFichaEstaciones aplican las mismas reglas.
func TestEstacionesDelSeedSonLasDeLaFicha(t *testing.T) {
	f, err := os.Open("../../reports/estaciones_ficha.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	ficha, err := almacen.LeerFichaEstaciones(f)
	if err != nil {
		t.Fatal(err)
	}
	slices.SortFunc(ficha, func(a, b almacen.Estacion) int { return strings.Compare(a.ID, b.ID) })

	es := sembrada(t).Estaciones()
	lista, err := es.Listar(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(lista, ficha) {
		t.Fatalf("MongoDB tiene %d estaciones y la ficha, %d; o alguna difiere", len(lista), len(ficha))
	}
	una, err := es.Obtener(ctx, "CA-AYAS-01")
	if err != nil || una.Departamento == nil || *una.Departamento != "ANCASH" || una.Lat != -9.52384 {
		t.Fatalf("Obtener(CA-AYAS-01) = %+v, %v", una, err)
	}
	if _, err := es.Obtener(ctx, "CA-NO-EXISTE"); !errors.Is(err, almacen.ErrNoEncontrado) {
		t.Fatalf("Obtener de una estación que no existe = %v, se esperaba ErrNoEncontrado", err)
	}
}

// Este paquete no crea índices: usa los del seed, que tienen nombre propio.
func TestLosIndicesSonLosDelSeed(t *testing.T) {
	a := sembrada(t)
	for coleccion, esperados := range map[string][]string{
		colModelos:        {"_id_", "activo", "creado"},
		colEntrenamientos: {"_id_", "inicio"},
		colPredicciones:   {"_id_", "estacion_ts"},
		colEventos:        {"_id_", "ts"},
		colEstaciones:     {"_id_"},
	} {
		cur, err := a.db.Collection(coleccion).Indexes().List(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var indices []struct{ Name string }
		if err := cur.All(ctx, &indices); err != nil {
			t.Fatal(err)
		}
		var nombres []string
		for _, i := range indices {
			nombres = append(nombres, i.Name)
		}
		slices.Sort(nombres)
		if !slices.Equal(nombres, esperados) {
			t.Errorf("%s tiene los índices %v, se esperaban %v", coleccion, nombres, esperados)
		}
	}
}

func TestPrediccionesGuardaUnLote(t *testing.T) {
	a := temporal(t)
	lima := time.FixedZone("-05", -5*3600)
	ts := time.Date(2026, 11, 19, 22, 5, 0, 123_000_000, lima)
	pm25, pm10, inca := 58.3, 121.4, "mala"
	lote := []almacen.Prediccion{
		{TS: ts, Estacion: "CA-AYAS-01", FechaHora: time.Date(2026, 11, 19, 22, 0, 0, 0, lima), PM25: 70.2, PM10: 190.5,
			Regimen: 1, ExcedeECA: true, Media24h: almacen.Media24h{PM25: &pm25, PM10: &pm10, Lecturas: 24},
			Alerta: true, INCA: &inca, ModeloID: "sembrado-kmeans-json", Ms: 0.04},
		{TS: ts.Add(time.Millisecond), Estacion: "CA-AYAS-01", FechaHora: time.Date(2026, 11, 19, 22, 0, 0, 0, lima), Media24h: almacen.Media24h{Lecturas: 1}},
		{TS: ts.Add(2 * time.Millisecond), Estacion: "CA-HUAR-01", FechaHora: time.Date(2021, 12, 10, 1, 0, 0, 0, lima)},
	}
	p := a.Predicciones()
	if err := p.GuardarLote(ctx, lote); err != nil {
		t.Fatal(err)
	}
	if err := p.GuardarLote(ctx, nil); err != nil {
		t.Fatalf("un lote vacío no es un error: %v", err)
	}
	if n, err := p.col.CountDocuments(ctx, bson.D{}); err != nil || n != 3 {
		t.Fatalf("%d predicciones guardadas (%v), se esperaban 3", n, err)
	}
	// Dos predicciones de la misma estación y hora son dos documentos.
	if n, _ := p.col.CountDocuments(ctx, bson.D{{Key: "estacion", Value: "CA-AYAS-01"}}); n != 2 {
		t.Fatalf("%d predicciones de CA-AYAS-01, se esperaban 2", n)
	}
	// DeEstacion: las de una estación, la más reciente primero.
	deAyas, err := p.DeEstacion(ctx, "CA-AYAS-01", 10)
	if err != nil || len(deAyas) != 2 || !deAyas[0].TS.Equal(lote[1].TS) || !deAyas[1].TS.Equal(lote[0].TS) {
		t.Fatalf("DeEstacion(CA-AYAS-01) = %+v, %v", deAyas, err)
	}
	if una, _ := p.DeEstacion(ctx, "CA-AYAS-01", 1); len(una) != 1 || !una[0].TS.Equal(lote[1].TS) {
		t.Fatalf("DeEstacion con n = 1 = %+v", una)
	}
	if ninguna, err := p.DeEstacion(ctx, "CA-NO-EXISTE", 10); err != nil || len(ninguna) != 0 {
		t.Fatalf("DeEstacion de una estación sin predicciones = %+v, %v", ninguna, err)
	}
	var doc prediccionDoc
	uno := p.col.FindOne(ctx, bson.D{{Key: "regimen", Value: 1}})
	raw, err := uno.Raw()
	if err != nil {
		t.Fatal(err)
	}
	if err := uno.Decode(&doc); err != nil {
		t.Fatal(err)
	}
	// La media va con los nombres del contrato, no con los del struct de Go.
	if v := raw.Lookup("media_24h", "pm2_5"); v.Type != bson.TypeDouble || v.Double() != pm25 {
		t.Errorf("media_24h.pm2_5 = %v", v)
	}
	// El instante se conserva; vuelve en UTC (el desfase -05:00 no se guarda).
	if !doc.TS.Equal(lote[0].TS) || !doc.FechaHora.Equal(lote[0].FechaHora) || doc.TS.Location() != time.UTC {
		t.Errorf("ts = %v, fecha_hora = %v; se esperaban los mismos instantes, en UTC", doc.TS, doc.FechaHora)
	}
	doc.TS, doc.FechaHora = lote[0].TS, lote[0].FechaHora
	if !reflect.DeepEqual(almacen.Prediccion(doc), lote[0]) {
		t.Errorf("se leyó %+v, se guardó %+v", doc, lote[0])
	}
	for campo, tipo := range map[string]bson.Type{"_id": bson.TypeObjectID, "ts": bson.TypeDateTime, "fecha_hora": bson.TypeDateTime} {
		if got := raw.Lookup(campo).Type; got != tipo {
			t.Errorf("%s es %v, se esperaba %v", campo, got, tipo)
		}
	}
}

func modeloDeEjemplo(t *testing.T) almacen.Modelo {
	t.Helper()
	var m almacen.Modelo
	leerEjemplo(t, "modelos.json", &m)
	return m
}

func TestModelosGuardaYDejaUnSoloActivo(t *testing.T) {
	m := temporal(t).Modelos()
	if _, err := m.Activo(ctx); !errors.Is(err, almacen.ErrNoEncontrado) {
		t.Fatalf("Activo sin modelos = %v, se esperaba ErrNoEncontrado", err)
	}
	sembrado := modeloDeEjemplo(t)
	for range 2 { // guardar dos veces el mismo no lo duplica
		if err := m.Guardar(ctx, sembrado); err != nil {
			t.Fatal(err)
		}
	}
	leido, err := m.Activo(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// creado pierde los nanosegundos y el desfase; todo lo demás vuelve igual.
	if !leido.Creado.Equal(sembrado.Creado.Truncate(time.Millisecond)) {
		t.Errorf("creado = %v, se esperaba %v al milisegundo", leido.Creado, sembrado.Creado)
	}
	leido.Creado = sembrado.Creado
	if !reflect.DeepEqual(leido, sembrado) {
		t.Fatalf("se leyó\n%+v\nse guardó\n%+v", leido, sembrado)
	}
	// El documento guarda los nombres de la receta tal como en reports/.
	raw, err := m.col.FindOne(ctx, bson.D{{Key: "_id", Value: sembrado.ID}}).Raw()
	if err != nil {
		t.Fatal(err)
	}
	if v := raw.Lookup("receta", "max_iter"); v.AsInt64() != 100 {
		t.Errorf("receta.max_iter = %v", v)
	}
	if v := raw.Lookup("estandarizacion", "por_estacion"); v.Type != 0 {
		t.Errorf("estandarizacion.por_estacion vacío no tendría que guardarse, es %v", v.Type)
	}

	entrenado := sembrado
	entrenado.ID, entrenado.Origen, entrenado.Creado = "m-e-20261119-1530", "e-20261119-1530", sembrado.Creado.Add(time.Hour)
	medicion := sembrado
	medicion.ID, medicion.Activo = "m-e-20261119-1600", false
	for _, mod := range []almacen.Modelo{entrenado, medicion} {
		if err := m.Guardar(ctx, mod); err != nil {
			t.Fatal(err)
		}
	}
	if a, err := m.Activo(ctx); err != nil || a.ID != entrenado.ID {
		t.Fatalf("Activo = %s, %v; se esperaba %s", a.ID, err, entrenado.ID)
	}
	if n, _ := m.col.CountDocuments(ctx, bson.D{{Key: "activo", Value: true}}); n != 1 {
		t.Fatalf("%d modelos activos, se esperaba 1", n)
	}
	if n, _ := m.col.CountDocuments(ctx, bson.D{}); n != 3 {
		t.Fatalf("%d modelos, se esperaban 3", n)
	}
}

func TestEntrenamientosCicloCompleto(t *testing.T) {
	e := temporal(t).Entrenamientos()
	t0 := time.Date(2026, 11, 19, 20, 30, 0, 0, time.UTC)
	nuevo := almacen.Entrenamiento{
		ID: "e-20261119-1530", Inicio: t0, Estado: almacen.EstadoEnCurso,
		Parametros: almacen.ParametrosEntrenamiento{Nodos: 2, Replicas: 1, Semilla: 42},
		Nodos: []almacen.NodoEntrenamiento{
			{Nombre: "H45-022", Direccion: "10.11.38.69:56455", Workers: 24, Rango: almacen.Rango{Desde: 0, Hasta: 32937}},
			{Nombre: "pc.lab.upc", Direccion: "10.11.38.70:51000", Workers: 24, Rango: almacen.Rango{Desde: 32937, Hasta: 65873}},
		},
	}
	if err := e.Crear(ctx, nuevo); err != nil {
		t.Fatal(err)
	}
	if err := e.Crear(ctx, nuevo); err == nil || !strings.Contains(err.Error(), "ya existe") {
		t.Fatalf("Crear con un id repetido = %v", err)
	}
	if err := e.Crear(ctx, almacen.Entrenamiento{ID: "e-20261119-1600", Inicio: t0.Add(30 * time.Minute), Estado: almacen.EstadoEnCurso}); err != nil {
		t.Fatal(err)
	}
	// En curso: sin fin ni resultado, y con las rondas como arreglo vacío.
	en, err := e.Obtener(ctx, nuevo.ID)
	if err != nil || en.Estado != almacen.EstadoEnCurso || en.Fin != nil || en.MsTotal != nil || len(en.Rondas) != 0 || len(en.Nodos) != 2 {
		t.Fatalf("Obtener en curso = %+v, %v", en, err)
	}
	for i := 1; i <= 3; i++ {
		// Un nombre de nodo con puntos es una clave válida de ms_por_nodo.
		r := almacen.Ronda{Iter: i, Inercia: 73075.5 / float64(i), MsRonda: 0.61, MsFusion: 0.003,
			MsPorNodo: map[string]float64{"H45-022": 0.11, "pc.lab.upc": 0.12}}
		if err := e.AgregarRonda(ctx, nuevo.ID, r); err != nil {
			t.Fatal(err)
		}
	}
	paro, dif, ms := "tolerancia", 8.9e-16, 31.2
	if err := e.Terminar(ctx, nuevo.ID, almacen.Cierre{Fin: t0.Add(31200 * time.Microsecond), Estado: almacen.EstadoTerminado,
		Iteraciones: 3, Paro: &paro, DifSecuencial: &dif, MsTotal: &ms}); err != nil {
		t.Fatal(err)
	}
	en, err = e.Obtener(ctx, nuevo.ID)
	if err != nil {
		t.Fatal(err)
	}
	if en.Estado != almacen.EstadoTerminado || en.Iteraciones != 3 || en.Paro == nil || *en.Paro != paro ||
		en.DifSecuencial == nil || *en.DifSecuencial != dif || en.Error != nil {
		t.Errorf("cierre = %+v", en)
	}
	// ms_total es un campo: con fin - inicio al milisegundo saldrían 31 ms.
	if en.MsTotal == nil || *en.MsTotal != 31.2 || en.Fin == nil || en.Fin.Sub(en.Inicio) != 31*time.Millisecond {
		t.Errorf("ms_total = %v, fin - inicio = %v", en.MsTotal, en.Fin.Sub(en.Inicio))
	}
	if len(en.Rondas) != 3 || en.Rondas[2].Iter != 3 || en.Rondas[0].MsPorNodo["pc.lab.upc"] != 0.12 || !reflect.DeepEqual(en.Nodos, nuevo.Nodos) {
		t.Errorf("rondas = %+v, nodos = %+v", en.Rondas, en.Nodos)
	}
	lista, err := e.Listar(ctx)
	if err != nil || len(lista) != 2 || lista[0].ID != "e-20261119-1600" || lista[1].ID != nuevo.ID ||
		lista[1].Rondas != nil || lista[1].Iteraciones != 3 {
		t.Fatalf("Listar = %+v, %v; se esperaban dos, el más reciente primero y sin rondas", lista, err)
	}
	motivo := "el nodo H45-022 dejó de responder en la ronda 7"
	if err := e.Terminar(ctx, "e-20261119-1600", almacen.Cierre{Fin: t0.Add(time.Hour), Estado: almacen.EstadoAbortado, Iteraciones: 6, Error: &motivo}); err != nil {
		t.Fatal(err)
	}
	if ab, err := e.Obtener(ctx, "e-20261119-1600"); err != nil || ab.Estado != almacen.EstadoAbortado || ab.Error == nil || *ab.Error != motivo || ab.MsTotal != nil {
		t.Errorf("abortado = %+v, %v", ab, err)
	}
	for nombre, err := range map[string]error{
		"AgregarRonda": e.AgregarRonda(ctx, "e-x", almacen.Ronda{}),
		"Terminar":     e.Terminar(ctx, "e-x", almacen.Cierre{}),
	} {
		if !errors.Is(err, almacen.ErrNoEncontrado) {
			t.Errorf("%s de un entrenamiento que no existe = %v, se esperaba ErrNoEncontrado", nombre, err)
		}
	}
	if _, err := e.Obtener(ctx, "e-x"); !errors.Is(err, almacen.ErrNoEncontrado) {
		t.Errorf("Obtener(e-x) = %v", err)
	}
	if vacia, err := temporal(t).Entrenamientos().Listar(ctx); err != nil || vacia == nil || len(vacia) != 0 {
		t.Errorf("Listar sin entrenamientos = %#v, %v; se esperaba una lista vacía", vacia, err)
	}
}

func TestEventosRecientes(t *testing.T) {
	e := temporal(t).Eventos()
	t0 := time.Date(2026, 11, 19, 20, 0, 0, 0, time.UTC)
	for i, ev := range []almacen.EventoCluster{
		{TS: t0, Tipo: almacen.EventoAlta, Nodo: "H45-022", Detalle: "24 workers desde 10.11.38.69:56455"},
		{TS: t0.Add(time.Second), Tipo: almacen.EventoError, Nodo: "H45-022", Detalle: "no respondió la ronda 7"},
		{TS: t0.Add(time.Second), Tipo: almacen.EventoBaja, Nodo: "H45-022"},
	} {
		if err := e.Registrar(ctx, ev); err != nil {
			t.Fatalf("evento %d: %v", i, err)
		}
	}
	rec, err := e.Recientes(ctx, 2)
	if err != nil || len(rec) != 2 || rec[0].Tipo != almacen.EventoBaja || rec[1].Tipo != almacen.EventoError || !rec[1].TS.Equal(t0.Add(time.Second)) {
		t.Fatalf("Recientes(2) = %+v, %v", rec, err)
	}
	if todos, _ := e.Recientes(ctx, 10); len(todos) != 3 || todos[2].Detalle != "24 workers desde 10.11.38.69:56455" {
		t.Fatalf("Recientes(10) = %+v", todos)
	}
	if ninguno, err := e.Recientes(ctx, 0); err != nil || len(ninguno) != 0 {
		t.Fatalf("Recientes(0) = %+v, %v", ninguno, err)
	}
}

// Con MongoDB caído nada se cuelga: Ping y una escritura fallan dentro del
// plazo de selección de servidor, no a los 30 s que trae el driver.
func TestConMongoCaidoFallaPronto(t *testing.T) {
	a, err := Conectar("mongodb://127.0.0.1:1", "aire")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Cerrar(ctx)
	inicio := time.Now()
	if err := a.Ping(ctx); err == nil {
		t.Fatal("Ping no falló sin MongoDB")
	}
	if err := a.Predicciones().GuardarLote(ctx, []almacen.Prediccion{{Estacion: "CA-A"}}); err == nil {
		t.Fatal("GuardarLote no falló sin MongoDB")
	}
	if _, err := a.Estaciones().Listar(ctx); err == nil {
		t.Fatal("Listar no falló sin MongoDB")
	}
	if d := time.Since(inicio); d > 3*(plazoSeleccion+time.Second) {
		t.Fatalf("tres operaciones sin MongoDB tardaron %v", d)
	}
}
