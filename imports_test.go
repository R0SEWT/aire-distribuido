package airedistribuido

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// El repositorio es biblioteca estándar de Go salvo por los drivers de MongoDB
// y Redis, que solo pueden importarse en almacen/mongo y almacen/redis (spec de
// la PC4, §3 y §13.1). Esta prueba recorre todos los .go, incluidas las
// pruebas, y falla si un import de fuera aparece en otro paquete: el núcleo
// concurrente (datos, kmeans, protocolo, cluster) y la API no dependen de nada
// externo.
func TestLosDriversSoloSeImportanEnElAlmacen(t *testing.T) {
	const modulo = "github.com/R0SEWT/aire-distribuido"
	permitidos := []string{filepath.Join("almacen", "mongo"), filepath.Join("almacen", "redis")}
	externosVistos := 0
	err := filepath.WalkDir(".", func(ruta string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if nombre := d.Name(); ruta != "." && (strings.HasPrefix(nombre, ".") || nombre == "data" || nombre == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(ruta, ".go") {
			return nil
		}
		archivo, err := parser.ParseFile(token.NewFileSet(), ruta, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range archivo.Imports {
			camino, _ := strconv.Unquote(imp.Path.Value)
			// Un paquete de la biblioteca estándar no tiene punto en su primer elemento.
			primero, _, _ := strings.Cut(camino, "/")
			if !strings.Contains(primero, ".") || camino == modulo || strings.HasPrefix(camino, modulo+"/") {
				continue
			}
			externosVistos++
			if !slices.Contains(permitidos, filepath.Dir(ruta)) {
				t.Errorf("%s importa %s: las dependencias externas solo van en %v", ruta, camino, permitidos)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if externosVistos == 0 {
		t.Fatal("no se vio ningún import externo: la prueba no está mirando almacen/mongo ni almacen/redis")
	}
}
