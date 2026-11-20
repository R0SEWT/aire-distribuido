package almacen_test

import (
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/R0SEWT/aire-distribuido/almacen"
)

// camposJSON devuelve las rutas de los campos de un valor JSON (a.b.c), en
// orden. De un arreglo de objetos toma los del primero. No entra en los campos
// de sinEntrar, cuyas claves son datos (nombres de nodos) y no campos.
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

func leerEjemplo(t *testing.T, archivo string) []byte {
	t.Helper()
	b, err := os.ReadFile("../docs/datos/ejemplos/" + archivo)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Los valores de Redis y los documentos que se guardan tal cual tienen
// exactamente los campos de su ejemplo en docs/datos/ejemplos/: ni uno más ni
// uno menos. Cada ejemplo se lee en su tipo y se vuelve a escribir; si un
// campo del ejemplo no existiera en el tipo, se perdería en el camino.
func TestLosDocumentosTienenLosCamposDeSusEjemplos(t *testing.T) {
	sinEntrar := []string{"rondas.ms_por_nodo", "estandarizacion.por_estacion"}
	for archivo, destino := range map[string]any{
		"redis-estacion-regimen.json":    &almacen.RegimenEstacion{},
		"redis-modelo-activo.json":       &almacen.Modelo{},
		"modelos.json":                   &almacen.Modelo{},
		"redis-entrenamiento-punto.json": &almacen.PuntoControl{},
		"entrenamientos.json":            &almacen.Entrenamiento{},
		"eventos_cluster.json":           &almacen.EventoCluster{},
	} {
		t.Run(archivo, func(t *testing.T) {
			ejemplo := leerEjemplo(t, archivo)
			if err := json.Unmarshal(ejemplo, destino); err != nil {
				t.Fatal(err)
			}
			escrito, err := json.Marshal(destino)
			if err != nil {
				t.Fatal(err)
			}
			var a, b any
			json.Unmarshal(ejemplo, &a)
			json.Unmarshal(escrito, &b)
			esperados := camposJSON(a, "", sinEntrar)
			if archivo == "eventos_cluster.json" {
				// El _id de un evento es el ObjectId que genera MongoDB.
				esperados = slices.DeleteFunc(esperados, func(c string) bool { return c == "_id" })
			}
			if got := camposJSON(b, "", sinEntrar); !slices.Equal(got, esperados) {
				t.Errorf("campos del tipo:\n  %v\ncampos del ejemplo:\n  %v", got, esperados)
			}
		})
	}
}

// El ejemplo del modelo, leído y vuelto a escribir, conserva sus valores: es
// lo que pasa cuando la API guarda el modelo en Redis y lo recupera al arrancar.
func TestElModeloDeEjemploSobreviveLaIdaYVuelta(t *testing.T) {
	var m, otra almacen.Modelo
	if err := json.Unmarshal(leerEjemplo(t, "redis-modelo-activo.json"), &m); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &otra); err != nil {
		t.Fatal(err)
	}
	if !m.Creado.Equal(otra.Creado) {
		t.Fatalf("creado cambió: %v → %v", m.Creado, otra.Creado)
	}
	m.Creado, otra.Creado = time.Time{}, time.Time{}
	if !reflect.DeepEqual(m, otra) {
		t.Fatalf("el modelo cambió:\n%+v\n%+v", m, otra)
	}
	if m.ID != "sembrado-kmeans-json" || m.Origen != almacen.OrigenSembrado || len(m.CentroidesStd) != 3 ||
		m.Receta.MaxIter != 100 || m.Estandarizacion.N != 65873 || m.Regimenes[1].Tamano != 1536 || !m.Activo {
		t.Fatalf("el ejemplo no se leyó completo: %+v", m)
	}
}
