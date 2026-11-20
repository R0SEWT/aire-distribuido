package mongo

import (
	"bytes"
	"encoding/json"
	"os"
	"slices"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/R0SEWT/aire-distribuido/almacen"
)

// codificar escribe un documento como lo hace el cliente de Conectar, con
// opcionesBSON, sin necesitar un MongoDB.
func codificar(t *testing.T, v any) bson.Raw {
	t.Helper()
	var buf bytes.Buffer
	enc := bson.NewEncoder(bson.NewDocumentWriter(&buf))
	if op := opcionesBSON(); op.UseJSONStructTags {
		enc.UseJSONStructTags()
	}
	if op := opcionesBSON(); op.NilSliceAsEmpty {
		enc.NilSliceAsEmpty()
	}
	if err := enc.Encode(v); err != nil {
		t.Fatal(err)
	}
	return bson.Raw(buf.Bytes())
}

// camposBSON devuelve las rutas de los campos de un documento (a.b.c), en
// orden. De un arreglo de documentos toma los del primero.
func camposBSON(t *testing.T, doc bson.Raw, prefijo string, sinEntrar []string) []string {
	t.Helper()
	elems, err := doc.Elements()
	if err != nil {
		t.Fatal(err)
	}
	var campos []string
	for _, e := range elems {
		ruta := prefijo + e.Key()
		campos = append(campos, ruta)
		if slices.Contains(sinEntrar, ruta) {
			continue
		}
		switch v := e.Value(); v.Type {
		case bson.TypeEmbeddedDocument:
			campos = append(campos, camposBSON(t, v.Document(), ruta+".", sinEntrar)...)
		case bson.TypeArray:
			if vals, _ := v.Array().Values(); len(vals) > 0 && vals[0].Type == bson.TypeEmbeddedDocument {
				campos = append(campos, camposBSON(t, vals[0].Document(), ruta+".", sinEntrar)...)
			}
		}
	}
	slices.Sort(campos)
	return slices.Compact(campos)
}

func camposJSON(v any, prefijo string, sinEntrar []string) []string {
	var campos []string
	switch x := v.(type) {
	case map[string]any:
		for k, hijo := range x {
			ruta := prefijo + k
			campos = append(campos, ruta)
			if !slices.Contains(sinEntrar, ruta) {
				campos = append(campos, camposJSON(hijo, ruta+".", sinEntrar)...)
			}
		}
	case []any:
		if len(x) > 0 {
			campos = append(campos, camposJSON(x[0], prefijo, sinEntrar)...)
		}
	}
	slices.Sort(campos)
	return slices.Compact(campos)
}

func leerEjemplo(t *testing.T, archivo string, destino any) []string {
	t.Helper()
	b, err := os.ReadFile("../../docs/datos/ejemplos/" + archivo)
	if err != nil {
		t.Fatal(err)
	}
	if destino != nil {
		if err := json.Unmarshal(b, destino); err != nil {
			t.Fatal(err)
		}
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	return camposJSON(v, "", sinEntrar)
}

// Campos cuyas claves son datos (nombres de nodos o de estaciones).
var sinEntrar = []string{"rondas.ms_por_nodo", "estandarizacion.por_estacion"}

// Cada documento que se escribe en MongoDB tiene exactamente los campos de su
// ejemplo en docs/datos/ejemplos/, con esos nombres (snake_case y _id): es el
// diseño de entidades de la spec §7.1. Corre sin MongoDB.
func TestLosDocumentosTienenLosCamposDeSusEjemplos(t *testing.T) {
	sinID := func(campos []string) []string {
		return slices.DeleteFunc(campos, func(c string) bool { return c == "_id" })
	}
	inca := "buena"
	fin := time.Date(2026, 11, 16, 22, 0, 0, 31_200_000, time.UTC)
	paro, dif, ms := "tolerancia", 0.0, 31.2
	var modelo almacen.Modelo
	camposModelo := leerEjemplo(t, "modelos.json", &modelo)
	casos := map[string]struct {
		doc       any
		esperados []string
	}{
		"estaciones": {aEstacionDoc(almacen.Estacion{ID: "CA-AYAS-01", Lat: -9.5, Lon: -77}), leerEjemplo(t, "estaciones.json", nil)},
		// El _id de una predicción y el de un evento los genera MongoDB.
		"predicciones": {prediccionDoc(almacen.Prediccion{Estacion: "CA-AYAS-01", INCA: &inca}), sinID(leerEjemplo(t, "predicciones.json", nil))},
		"modelos":      {modelo, camposModelo},
		"entrenamientos": {almacen.Entrenamiento{
			ID: "e-1", Fin: &fin, Estado: almacen.EstadoTerminado, Paro: &paro, DifSecuencial: &dif, MsTotal: &ms,
			Nodos:  []almacen.NodoEntrenamiento{{Nombre: "n1"}},
			Rondas: []almacen.Ronda{{Iter: 1, MsPorNodo: map[string]float64{"n1": 0.1}}},
		}, leerEjemplo(t, "entrenamientos.json", nil)},
		"eventos_cluster": {almacen.EventoCluster{Tipo: almacen.EventoAlta}, sinID(leerEjemplo(t, "eventos_cluster.json", nil))},
	}
	for nombre, c := range casos {
		t.Run(nombre, func(t *testing.T) {
			if got := camposBSON(t, codificar(t, c.doc), "", sinEntrar); !slices.Equal(got, c.esperados) {
				t.Errorf("campos del documento:\n  %v\ncampos del ejemplo:\n  %v", got, c.esperados)
			}
		})
	}
}

// Las fechas van como BSON Date y un slice vacío, como arreglo: un campo tiene
// siempre el mismo tipo.
func TestTiposDeLosCampos(t *testing.T) {
	lima := time.FixedZone("-05", -5*3600)
	ts := time.Date(2026, 11, 19, 22, 5, 0, 123_456_789, lima)
	pred := codificar(t, prediccionDoc(almacen.Prediccion{TS: ts, FechaHora: ts.Truncate(time.Hour), Estacion: "CA-A"}))
	for _, campo := range []string{"ts", "fecha_hora"} {
		if tipo := pred.Lookup(campo).Type; tipo != bson.TypeDateTime {
			t.Errorf("predicciones.%s es %v, se esperaba una fecha", campo, tipo)
		}
	}
	// Se pierden el desfase y lo que hay bajo el milisegundo; el instante no.
	if got := pred.Lookup("ts").Time(); !got.Equal(ts.Truncate(time.Millisecond)) {
		t.Errorf("ts = %v, se esperaba el mismo instante al milisegundo (%v)", got, ts.Truncate(time.Millisecond))
	}
	if tipo := pred.Lookup("media_24h", "pm2_5").Type; tipo != bson.TypeNull {
		t.Errorf("media_24h.pm2_5 sin media es %v, se esperaba null", tipo)
	}
	en := codificar(t, almacen.Entrenamiento{ID: "e-1", Inicio: ts, Estado: almacen.EstadoEnCurso})
	for campo, tipo := range map[string]bson.Type{
		"_id": bson.TypeString, "inicio": bson.TypeDateTime, "fin": bson.TypeNull, "rondas": bson.TypeArray,
		"nodos": bson.TypeArray, "ms_total": bson.TypeNull, "error": bson.TypeNull,
	} {
		if got := en.Lookup(campo).Type; got != tipo {
			t.Errorf("entrenamientos.%s es %v, se esperaba %v", campo, got, tipo)
		}
	}
	if tipo := codificar(t, almacen.EventoCluster{TS: ts}).Lookup("ts").Type; tipo != bson.TypeDateTime {
		t.Errorf("eventos_cluster.ts es %v", tipo)
	}
	if tipo := codificar(t, almacen.Modelo{Creado: ts}).Lookup("creado").Type; tipo != bson.TypeDateTime {
		t.Errorf("modelos.creado es %v", tipo)
	}
}

func TestEstacionSinCoordenadas(t *testing.T) {
	if _, err := (estacionDoc{ID: "CA-X"}).estacion(); err == nil {
		t.Fatal("aceptó una estación sin lat ni lon")
	}
}

func TestConectarRechazaUnaURIMalaOSinBase(t *testing.T) {
	if _, err := Conectar("no-es-una-uri", "aire"); err == nil {
		t.Error("aceptó una URI que no es de MongoDB")
	}
	if _, err := Conectar("mongodb://localhost:27017", ""); err == nil {
		t.Error("aceptó una base de datos sin nombre")
	}
}
