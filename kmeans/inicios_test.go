package kmeans

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func archivoDePrueba(t *testing.T) (*Sintetico, *Multiarranque, ArchivoInicios) {
	t.Helper()
	s := casoOptimoLocal(t)
	m, err := MejorDeSemillas(s.Gold.Datos, 5, 42, 4, opcionesSemillas(4, 250), 0)
	if err != nil {
		t.Fatal(err)
	}
	r := recetaPrueba(NivelAbsoluto)
	return s, m, NuevoArchivoInicios("sha256:gold", r, s.Gold.Datos.D, m.Inicios)
}

func TestLosIniciosGuardadosDanElMismoResultadoBitABit(t *testing.T) {
	s, m, a := archivoDePrueba(t)
	ruta := filepath.Join(t.TempDir(), "centroides_iniciales.json")
	if err := GuardarInicios(ruta, a); err != nil {
		t.Fatal(err)
	}
	leido, err := LeerInicios(ruta)
	if err != nil {
		t.Fatalf("LeerInicios: %v", err)
	}
	inicios, err := leido.Para("sha256:gold", recetaPrueba(NivelAbsoluto), s.Gold.Datos.D, 42, 4)
	if err != nil {
		t.Fatalf("Para: %v", err)
	}
	for i := range inicios {
		if HashCentroides(inicios[i].Centroides) != HashCentroides(m.Inicios[i].Centroides) {
			t.Fatalf("semilla %d: los centroides cambiaron en la ida y vuelta por el archivo", inicios[i].Semilla)
		}
	}
	desdeArchivo, err := MejorDeInicios(s.Gold.Datos, inicios, opcionesSemillas(4, 250), 0)
	if err != nil {
		t.Fatal(err)
	}
	if desdeArchivo.Elegido != m.Elegido {
		t.Fatalf("elegido %d, sin archivo %d", desdeArchivo.Elegido, m.Elegido)
	}
	mismoResultado(t, "desde el archivo contra k-means++", desdeArchivo.Mejor, m.Mejor)
}

func TestParaEligeLasSemillasPedidasDelArchivo(t *testing.T) {
	s, m, a := archivoDePrueba(t)
	d := s.Gold.Datos.D
	una, err := a.Para("sha256:gold", recetaPrueba(NivelAbsoluto), d, 43, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(una) != 2 || una[0].Semilla != 43 || una[1].Semilla != 44 ||
		HashCentroides(una[0].Centroides) != HashCentroides(m.Inicios[1].Centroides) {
		t.Errorf("Para(43, 2) = %+v", una)
	}
}

func TestParaRechazaUnArchivoDeOtraCorrida(t *testing.T) {
	s, _, a := archivoDePrueba(t)
	d := s.Gold.Datos.D
	r := recetaPrueba(NivelAbsoluto)
	otraReceta := func(cambiar func(*Receta)) Receta { c := r; cambiar(&c); return c }
	casos := map[string]func() error{
		"otro gold": func() error { _, err := a.Para("sha256:otro", r, d, 42, 4); return err },
		"otro k": func() error {
			_, err := a.Para("sha256:gold", otraReceta(func(c *Receta) { c.K = 4 }), d, 42, 4)
			return err
		},
		"otras features": func() error {
			_, err := a.Para("sha256:gold", otraReceta(func(c *Receta) { c.Features = []string{"pm10", "humedad"} }), d, 42, 4)
			return err
		},
		"otro log1p": func() error {
			_, err := a.Para("sha256:gold", otraReceta(func(c *Receta) { c.Log1p = nil }), d, 42, 4)
			return err
		},
		"otro nivel": func() error {
			_, err := a.Para("sha256:gold", otraReceta(func(c *Receta) { c.Nivel = NivelEstacion }), d, 42, 4)
			return err
		},
		"faltan semillas": func() error { _, err := a.Para("sha256:gold", r, d, 42, 6); return err },
	}
	for nombre, f := range casos {
		t.Run(nombre, func(t *testing.T) {
			err := f()
			if err == nil {
				t.Fatal("se esperaba error")
			}
			if !strings.Contains(err.Error(), "bórralo") {
				t.Errorf("el error debería decir cómo salir del paso: %v", err)
			}
		})
	}
}

func TestLeerIniciosRechazaUnArchivoCorrupto(t *testing.T) {
	_, _, a := archivoDePrueba(t)
	escribirArchivo := func(t *testing.T, v any) string {
		ruta := filepath.Join(t.TempDir(), "c.json")
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(ruta, b, 0o600); err != nil {
			t.Fatal(err)
		}
		return ruta
	}
	tocado := a
	tocado.Inicios = append([]InicioGuardado(nil), a.Inicios...)
	copia := make([][]float64, len(a.Inicios[0].Centroides))
	for j, c := range a.Inicios[0].Centroides {
		copia[j] = append([]float64(nil), c...)
	}
	copia[2][1] += 1e-9 // una edición a mano: misma forma, otros números
	tocado.Inicios[0].Centroides = copia
	if _, err := LeerInicios(escribirArchivo(t, tocado)); err == nil || !strings.Contains(err.Error(), "sha256") {
		t.Errorf("centroides que no son los de su sha256: %v", err)
	}
	cortado := a
	cortado.Inicios = append([]InicioGuardado(nil), a.Inicios...)
	cortado.Inicios[1].Centroides = cortado.Inicios[1].Centroides[:2]
	if _, err := LeerInicios(escribirArchivo(t, cortado)); err == nil {
		t.Error("menos centroides que k_efectivo: se esperaba error")
	}
	if _, err := LeerInicios(filepath.Join(t.TempDir(), "no.json")); err == nil {
		t.Error("archivo inexistente: se esperaba error")
	}
}
