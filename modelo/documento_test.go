package modelo

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/R0SEWT/aire-distribuido/almacen"
)

// idaYVuelta pasa el modelo por su documento y por JSON, como cuando la API lo
// guarda en Redis y lo recupera al arrancar.
func idaYVuelta(t *testing.T, m *Modelo) *Modelo {
	t.Helper()
	b, err := json.Marshal(m.Documento())
	if err != nil {
		t.Fatal(err)
	}
	var doc almacen.Modelo
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	otro, err := DesdeDocumento(doc)
	if err != nil {
		t.Fatal(err)
	}
	return otro
}

// El modelo recuperado de su documento predice lo mismo que el original en los
// casos fijos del gold, y conserva de dónde salió.
func TestElModeloVuelveDeSuDocumento(t *testing.T) {
	m := sembrarDelRepo(t)
	otro := idaYVuelta(t, m)
	if otro.ID != IDSembrado || otro.Origen != almacen.OrigenSembrado || otro.GoldSHA256 != m.GoldSHA256 ||
		!otro.Creado.Equal(m.Creado) || otro.Iteraciones != m.Iteraciones || otro.Inercia != m.Inercia {
		t.Fatalf("se perdió la procedencia: %+v", otro)
	}
	if m.GoldSHA256 == "" || m.Creado.IsZero() {
		t.Fatalf("el modelo sembrado no trae el gold (%q) o la fecha (%v) de reports/kmeans.json", m.GoldSHA256, m.Creado)
	}
	if !reflect.DeepEqual(otro.Regimenes, m.Regimenes) || !reflect.DeepEqual(otro.CentroidesEstandarizados(), m.CentroidesEstandarizados()) ||
		!reflect.DeepEqual(otro.CentroidesOriginales, m.CentroidesOriginales) || !reflect.DeepEqual(otro.Receta, m.Receta) {
		t.Fatal("el modelo recuperado no es el original")
	}
	b, err := os.ReadFile(rutaCasos)
	if err != nil {
		t.Fatal(err)
	}
	var a archivoCasos
	if err := json.Unmarshal(b, &a); err != nil {
		t.Fatal(err)
	}
	if len(a.Casos) == 0 {
		t.Fatal("no hay casos fijos")
	}
	for _, c := range a.Casos {
		l := Lectura{Estacion: c.Estacion, PM25: c.PM25, PM10: c.PM10}
		r1, err1 := m.Predecir(l)
		r2, err2 := otro.Predecir(l)
		if err1 != nil || err2 != nil || r1 != r2 || r2.Regimen != c.Regimen {
			t.Fatalf("%+v: el original da %+v (%v) y el recuperado %+v (%v)", c, r1, err1, r2, err2)
		}
	}
}

// El documento es el de docs/datos/ejemplos/modelos.json: mismos valores que
// el modelo que se siembra desde reports/.
func TestElDocumentoDelModeloSembradoEsElDelEjemplo(t *testing.T) {
	var ejemplo almacen.Modelo
	b, err := os.ReadFile("../docs/datos/ejemplos/modelos.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &ejemplo); err != nil {
		t.Fatal(err)
	}
	doc := sembrarDelRepo(t).Documento()
	if doc.Activo {
		t.Error("Documento no decide si el modelo está activo")
	}
	doc.Activo = ejemplo.Activo
	if !doc.Creado.Equal(ejemplo.Creado) {
		t.Errorf("creado = %v, en el ejemplo %v", doc.Creado, ejemplo.Creado)
	}
	doc.Creado = ejemplo.Creado
	if !reflect.DeepEqual(doc, ejemplo) {
		t.Errorf("documento:\n%+v\nejemplo:\n%+v", doc, ejemplo)
	}
}

// Con nivel = estacion la estandarización es por estación y el modelo no tiene
// centroides en unidades originales: también eso sobrevive.
func TestElModeloPorEstacionVuelveDeSuDocumento(t *testing.T) {
	m, err := Sembrar(reportsSinteticos(t, sintetico{
		features: `["pm2_5", "pm10"]`, log1p: `[]`, nivel: "estacion",
		porEstacion: `{"CA-A": {"n": 1, "media": [10, 20], "desv": [2, 4]}}`,
		centroides:  `[[0, 0], [3, 3]]`,
	}))
	if err != nil {
		t.Fatal(err)
	}
	otro := idaYVuelta(t, m)
	if r, err := otro.Predecir(Lectura{Estacion: "CA-A", PM25: 16, PM10: 32}); err != nil || r.Regimen != 1 {
		t.Fatalf("Predecir = %+v, %v; se esperaba el régimen 1", r, err)
	}
	if otro.CentroidesOriginales != nil {
		t.Fatal("con nivel = estacion los centroides no tienen unidades originales")
	}
}

// Un documento viene de fuera del proceso: lo que no cierra se rechaza con un
// error que nombra el modelo, en vez de predecir mal o entrar en pánico.
func TestDesdeDocumentoRechazaDocumentosQueNoCierran(t *testing.T) {
	base := func() almacen.Modelo {
		doc := sembrarDelRepo(t).Documento()
		doc.Creado = time.Time{}
		// Copias propias, para que un caso no le cambie los datos a otro.
		b, _ := json.Marshal(doc)
		var copia almacen.Modelo
		json.Unmarshal(b, &copia)
		return copia
	}
	if _, err := DesdeDocumento(base()); err != nil {
		t.Fatalf("el documento base tiene que valer: %v", err)
	}
	for nombre, cambiar := range map[string]func(*almacen.Modelo){
		"sin id":                       func(d *almacen.Modelo) { d.ID = "" },
		"documento vacío":              func(d *almacen.Modelo) { *d = almacen.Modelo{ID: "m-1"} },
		"un centroide de menos":        func(d *almacen.Modelo) { d.CentroidesStd = d.CentroidesStd[:2] },
		"un centroide corto":           func(d *almacen.Modelo) { d.CentroidesStd[1] = d.CentroidesStd[1][:1] },
		"un régimen de menos":          func(d *almacen.Modelo) { d.Regimenes = d.Regimenes[:2] },
		"regímenes en desorden":        func(d *almacen.Modelo) { d.Regimenes[0], d.Regimenes[1] = d.Regimenes[1], d.Regimenes[0] },
		"feature que no se deriva":     func(d *almacen.Modelo) { d.Receta.Features = []string{"pm2_5", "temperatura"} },
		"estandarización de otra cosa": func(d *almacen.Modelo) { d.Estandarizacion.Features = []string{"pm10", "pm2_5"} },
		"estandarización sin media":    func(d *almacen.Modelo) { d.Estandarizacion.Media = nil },
		"k de la receta distinto":      func(d *almacen.Modelo) { d.Receta.K = 4 },
	} {
		doc := base()
		cambiar(&doc)
		_, err := DesdeDocumento(doc)
		if err == nil {
			t.Errorf("%s: DesdeDocumento lo aceptó", nombre)
		} else if doc.ID != "" && !strings.Contains(err.Error(), doc.ID) {
			t.Errorf("%s: el error no nombra el modelo: %v", nombre, err)
		}
	}
}
