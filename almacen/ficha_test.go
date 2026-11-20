package almacen

import (
	"os"
	"strings"
	"testing"
)

func TestLeerFichaEstacionesConLaFichaDelRepo(t *testing.T) {
	f, err := os.Open("../reports/estaciones_ficha.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	es, err := LeerFichaEstaciones(f)
	if err != nil {
		t.Fatalf("LeerFichaEstaciones: %v", err)
	}
	if len(es) != 33 {
		t.Fatalf("%d estaciones, se esperaban las 33 de la ficha", len(es))
	}
	var ayas *Estacion
	for i := range es {
		if es[i].ID == "CA-AYAS-01" {
			ayas = &es[i]
		}
	}
	if ayas == nil {
		t.Fatal("falta CA-AYAS-01")
	}
	// El ejemplo del contrato (docs/api/openapi.yaml, GET /api/estaciones).
	want := Estacion{
		ID: "CA-AYAS-01", ObjetoFiscalizado: texto("Unidad minera Antamina"), TipoActividad: texto("minería"),
		Departamento: texto("ANCASH"), Provincia: texto("HUARI"), Distrito: texto("SAN MARCOS"),
		Lat: -9.52384, Lon: -77.02043,
	}
	if !igualEstacion(*ayas, want) {
		t.Fatalf("CA-AYAS-01 = %+v, se esperaba %+v", *ayas, want)
	}
}

func TestLeerFichaEstacionesNulosYComillas(t *testing.T) {
	// Las mismas reglas que docs/datos/seed.js: vacío, «-» y «NA» son null.
	csv := "estacion,objeto_fiscalizado,tipo_actividad,departamento,provincia,distrito,lat,lon,otra\n" +
		"CA-1,\"Mina \"\"La Buena\"\", tajo norte\",minería,NA,-, ,-10.5,-76.25,x\n"
	es, err := LeerFichaEstaciones(strings.NewReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	want := Estacion{ID: "CA-1", ObjetoFiscalizado: texto(`Mina "La Buena", tajo norte`), TipoActividad: texto("minería"), Lat: -10.5, Lon: -76.25}
	if len(es) != 1 || !igualEstacion(es[0], want) {
		t.Fatalf("estaciones = %+v, se esperaba %+v", es, want)
	}
}

func TestLeerFichaEstacionesRechaza(t *testing.T) {
	cab := "estacion,objeto_fiscalizado,tipo_actividad,departamento,provincia,distrito,lat,lon\n"
	casos := map[string]string{
		"sin columna lon":      "estacion,objeto_fiscalizado,tipo_actividad,departamento,provincia,distrito,lat\nCA-1,a,b,c,d,e,1\n",
		"estación vacía":       cab + " ,a,b,c,d,e,1,2\n",
		"estación repetida":    cab + "CA-1,a,b,c,d,e,1,2\nCA-1,a,b,c,d,e,1,2\n",
		"lat no numérica":      cab + "CA-1,a,b,c,d,e,norte,2\n",
		"lon ausente":          cab + "CA-1,a,b,c,d,e,1,NA\n",
		"lat fuera de rango":   cab + "CA-1,a,b,c,d,e,91,2\n",
		"ficha sin estaciones": cab,
	}
	for nombre, csv := range casos {
		t.Run(nombre, func(t *testing.T) {
			if es, err := LeerFichaEstaciones(strings.NewReader(csv)); err == nil {
				t.Fatalf("no falló: %+v", es)
			}
		})
	}
}

func texto(s string) *string { return &s }

func igualEstacion(a, b Estacion) bool {
	igual := func(x, y *string) bool { return (x == nil && y == nil) || (x != nil && y != nil && *x == *y) }
	return a.ID == b.ID && igual(a.ObjetoFiscalizado, b.ObjetoFiscalizado) && igual(a.TipoActividad, b.TipoActividad) &&
		igual(a.Departamento, b.Departamento) && igual(a.Provincia, b.Provincia) && igual(a.Distrito, b.Distrito) &&
		a.Lat == b.Lat && a.Lon == b.Lon
}
