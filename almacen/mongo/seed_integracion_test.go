//go:build integracion

package mongo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// correrSeed ejecuta docs/datos/seed.js con el servicio seed del Compose, sobre
// la base de a y con la ficha que se le pasa. Devuelve la salida y el error de
// salida de mongosh.
func correrSeed(t *testing.T, a *Almacen, ficha string) (string, error) {
	t.Helper()
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("hace falta docker para correr el seed")
	}
	ruta := filepath.Join(t.TempDir(), "ficha.csv")
	if err := os.WriteFile(ruta, []byte(ficha), 0o644); err != nil {
		t.Fatal(err)
	}
	// El directorio temporal es 0700: se abre para el usuario del contenedor.
	if err := os.Chmod(filepath.Dir(ruta), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("docker", "compose", "run", "--rm", "-T",
		"-e", "MONGO_INITDB_DATABASE="+a.db.Name(),
		"-e", "ESTACIONES_CSV=/seed/prueba.csv",
		"-v", ruta+":/seed/prueba.csv:ro",
		"seed")
	cmd.Dir = "../.." // la raíz del repo, donde está docker-compose.yml
	salida, err := cmd.CombinedOutput()
	return string(salida), err
}

func fichaDelRepo(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("../../reports/estaciones_ficha.csv")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// cambiarCampo devuelve la ficha con un campo de una estación reemplazado.
func cambiarCampo(t *testing.T, ficha, estacion, columna, valor string) string {
	t.Helper()
	lineas := strings.Split(ficha, "\n")
	col := -1
	for i, c := range strings.Split(strings.TrimRight(lineas[0], "\r"), ",") {
		if c == columna {
			col = i
		}
	}
	for i, l := range lineas {
		if campos := strings.Split(l, ","); col >= 0 && campos[0] == estacion {
			campos[col] = valor
			lineas[i] = strings.Join(campos, ",")
			return strings.Join(lineas, "\n")
		}
	}
	t.Fatalf("la ficha no tiene la estación %s o la columna %s", estacion, columna)
	return ""
}

func contarEstaciones(t *testing.T, a *Almacen) int64 {
	t.Helper()
	n, err := a.db.Collection(colEstaciones).CountDocuments(ctx, bson.D{})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// El seed valida la ficha con las mismas reglas que almacen.LeerFichaEstaciones
// antes de escribir: una ficha que la API no podría leer no entra a MongoDB,
// el seed sale con error y la base queda como estaba.
func TestElSeedRechazaLasFichasQueLaAPINoPuedeLeer(t *testing.T) {
	buena := fichaDelRepo(t)
	// CA-AYAS-01 no tiene comas dentro de sus campos: se puede editar por columnas.
	malas := map[string]struct{ ficha, mensaje string }{
		"lat vacía":            {cambiarCampo(t, buena, "CA-AYAS-01", "lat", ""), "CA-AYAS-01: lat ausente"},
		"lat fuera de rango":   {cambiarCampo(t, buena, "CA-AYAS-01", "lat", "999"), "CA-AYAS-01: lat fuera de"},
		"lon fuera de rango":   {cambiarCampo(t, buena, "CA-AYAS-01", "lon", "-181"), "CA-AYAS-01: lon fuera de"},
		"lon que no es número": {cambiarCampo(t, buena, "CA-AYAS-01", "lon", "0x10"), "CA-AYAS-01: lon no numérica"},
	}

	t.Run("sobre una base vacía no crea nada", func(t *testing.T) {
		a := temporal(t)
		for nombre, c := range malas {
			salida, err := correrSeed(t, a, c.ficha)
			if err == nil || !strings.Contains(salida, c.mensaje) {
				t.Errorf("%s: err = %v, se esperaba un error con %q:\n%s", nombre, err, c.mensaje, salida)
			}
		}
		colecciones, err := a.db.ListCollectionNames(ctx, bson.D{})
		if err != nil || len(colecciones) != 0 {
			t.Fatalf("un seed rechazado creó las colecciones %v (%v)", colecciones, err)
		}
	})

	t.Run("sobre una base sembrada no cambia nada", func(t *testing.T) {
		a := temporal(t)
		if salida, err := correrSeed(t, a, buena); err != nil {
			t.Fatalf("la ficha del repo tiene que sembrar: %v\n%s", err, salida)
		}
		if n := contarEstaciones(t, a); n != 33 {
			t.Fatalf("%d estaciones sembradas, se esperaban 33", n)
		}
		// Una ficha mala que además ya no trae una estación: no se retira.
		lineas := strings.Split(strings.TrimRight(buena, "\n"), "\n")
		sinUltima := strings.Join(lineas[:len(lineas)-1], "\n") + "\n"
		if salida, err := correrSeed(t, a, cambiarCampo(t, sinUltima, "CA-AYAS-01", "lat", "")); err == nil {
			t.Fatalf("aceptó una ficha sin lat:\n%s", salida)
		}
		if n := contarEstaciones(t, a); n != 33 {
			t.Fatalf("un seed rechazado dejó %d estaciones; tenían que seguir las 33", n)
		}
		if e, err := a.Estaciones().Obtener(ctx, "CA-AYAS-01"); err != nil || e.Lat != -9.52384 {
			t.Fatalf("CA-AYAS-01 después del seed rechazado = %+v, %v", e, err)
		}
		// Lo que el seed deja, la API lo lee entero.
		if lista, err := a.Estaciones().Listar(ctx); err != nil || len(lista) != 33 {
			t.Fatalf("Listar = %d estaciones, %v", len(lista), err)
		}
	})
}

// Una ficha guardada con marca de orden de bytes (como la deja Excel) se lee
// igual que en Go, y una estación que sale de la ficha sale de MongoDB.
func TestElSeedLeeUnaFichaConBOMYRetiraEstaciones(t *testing.T) {
	a := temporal(t)
	buena := fichaDelRepo(t)
	if salida, err := correrSeed(t, a, "\ufeff"+buena); err != nil {
		t.Fatalf("con BOM: %v\n%s", err, salida)
	}
	if n := contarEstaciones(t, a); n != 33 {
		t.Fatalf("%d estaciones con BOM, se esperaban 33", n)
	}
	lineas := strings.Split(strings.TrimRight(buena, "\n"), "\n")
	retirada := strings.Split(lineas[len(lineas)-1], ",")[0]
	salida, err := correrSeed(t, a, strings.Join(lineas[:len(lineas)-1], "\n")+"\n")
	if err != nil || !strings.Contains(salida, "estaciones_retiradas: 1") {
		t.Fatalf("sin la última estación: %v\n%s", err, salida)
	}
	if n := contarEstaciones(t, a); n != 32 {
		t.Fatalf("quedaron %d estaciones, se esperaban 32", n)
	}
	if _, err := a.Estaciones().Obtener(ctx, retirada); err == nil {
		t.Fatalf("la estación %s sigue en MongoDB", retirada)
	}
}
