package datos

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/R0SEWT/aire-distribuido/kmeans"
)

// sha256Muestra es el de data/sample/oefa_muestra.csv, que TestMuestraVersionadaAlDia
// mantiene fijo. Si la muestra se regenera a propósito, este valor cambia con ella.
const sha256Muestra = "38fdecc55cb180f57665614326543b74db6ece281682728e6d146c68901c5b89"

func TestVersionDeArchivo(t *testing.T) {
	v, err := VersionDeArchivo(rutaMuestra)
	if err != nil {
		t.Fatal(err)
	}
	if v != (VersionArchivo{Ruta: rutaMuestra, Sha256: sha256Muestra, Existe: true}) {
		t.Errorf("versión de la muestra: %+v", v)
	}
	falta := filepath.Join(t.TempDir(), "kmeans.json")
	v, err = VersionDeArchivo(falta)
	if err != nil {
		t.Fatalf("un archivo que no existe no es un error (receta provisional): %v", err)
	}
	if v.Existe || v.Sha256 != "" || v.Ruta != falta {
		t.Errorf("archivo inexistente: %+v", v)
	}
	dir := t.TempDir()
	if _, err := VersionDeArchivo(dir); err == nil {
		t.Error("un directorio no es un archivo de config: tiene que fallar")
	}
}

func TestAnotarProcedencia(t *testing.T) {
	m := generarMuestra()
	res := cargarBytes(t, m.csv, ConfigPorDefecto(), RecetaProvisional(), opMuestra)
	p := Procedencia{
		Metadatos:     kmeans.RecolectarMetadatos(),
		Sha256Entrada: sha256Muestra,
		Configs: map[string]VersionArchivo{
			"limpieza": {Ruta: "configs/limpieza.json", Sha256: "abc", Existe: true},
			"kmeans":   {Ruta: "configs/kmeans.json"},
		},
	}
	res.Anotar(p)
	for nombre, meta := range map[string]*kmeans.Metadatos{
		"limpieza": res.Limpieza.Metadatos, "estandarizacion": res.Estandarizacion.Metadatos,
	} {
		if meta == nil || meta.VersionGo == "" || meta.CPUsLogicas < 1 || meta.Momento.IsZero() || meta.Maquina == "" {
			t.Errorf("%s: metadatos incompletos: %+v", nombre, meta)
		}
	}
	if res.Limpieza.Sha256 != sha256Muestra {
		t.Errorf("sha256 de la entrada: %q", res.Limpieza.Sha256)
	}
	if res.Limpieza.Configs["limpieza"].Sha256 != "abc" || res.Estandarizacion.Configs["kmeans"].Existe {
		t.Errorf("configs: %+v / %+v", res.Limpieza.Configs, res.Estandarizacion.Configs)
	}
}

func TestSpeedupConICSobreRepeticionesSinteticas(t *testing.T) {
	// Base ~2.0 s y W ~0.5 s con ruido chico: el speedup ronda 4.
	base := []float64{2.00, 2.03, 1.98, 2.05, 1.97, 2.01, 2.02, 1.99, 2.04, 1.96}
	conc := []float64{0.50, 0.52, 0.49, 0.51, 0.48, 0.50, 0.53, 0.49, 0.50, 0.51}
	ic, recortada, err := speedupConIC(base, conc, BootstrapPorDefecto)
	if err != nil {
		t.Fatal(err)
	}
	if ic.Nivel != 0.95 || !(ic.Inferior <= ic.Punto && ic.Punto <= ic.Superior) {
		t.Fatalf("intervalo mal formado: %+v", ic)
	}
	if ic.Inferior < 3.7 || ic.Superior > 4.3 || ic.Superior-ic.Inferior <= 0 {
		t.Errorf("con ruido chico el IC tiene que ser angosto y alrededor de 4: %+v", ic)
	}
	quiero, _ := kmeans.MediaRecortada(conc, BootstrapPorDefecto.Recorte)
	if recortada != quiero {
		t.Errorf("media recortada %v, quiero %v", recortada, quiero)
	}
	otra, _, _ := speedupConIC(base, conc, BootstrapPorDefecto)
	if *otra != *ic {
		t.Error("con la semilla fija el IC tiene que ser reproducible")
	}

	// Una sola repetición: el intervalo degenera en el punto.
	uno, _, err := speedupConIC([]float64{2}, []float64{0.5}, BootstrapPorDefecto)
	if err != nil || uno.Punto != 4 || uno.Inferior != 4 || uno.Superior != 4 {
		t.Errorf("una repetición: %+v, %v", uno, err)
	}
	// Ruido grande: el intervalo se ensancha.
	ruidosa := []float64{0.3, 0.9, 0.4, 1.2, 0.5, 0.35, 1.0, 0.45, 0.6, 0.8}
	ancho, _, _ := speedupConIC(base, ruidosa, BootstrapPorDefecto)
	if ancho.Superior-ancho.Inferior <= ic.Superior-ic.Inferior {
		t.Errorf("más ruido tiene que dar un IC más ancho: %+v vs %+v", ancho, ic)
	}
	if _, _, err := speedupConIC(nil, conc, BootstrapPorDefecto); err == nil {
		t.Error("sin tiempos de la base tiene que fallar")
	}
	if math.IsNaN(ic.Punto) {
		t.Error("punto NaN")
	}
}

func TestBenchmarkTraeICYProtocolo(t *testing.T) {
	ruta := escribirMuestra(t)
	bm, _, err := MedirCarga(ruta, ConfigPorDefecto(), RecetaProvisional(), []int{1, 2}, 3, Opciones{Bloque: 8 << 10, Chunk: 97})
	if err != nil {
		t.Fatal(err)
	}
	if bm.Protocolo.ReplicasBootstrap != BootstrapPorDefecto.Replicas || bm.Protocolo.RecortePorExtremo != BootstrapPorDefecto.Recorte ||
		bm.Protocolo.Speedup == "" || bm.Protocolo.SpeedupIC == "" {
		t.Errorf("protocolo: %+v", bm.Protocolo)
	}
	for _, m := range bm.Mediciones {
		ic := m.SpeedupIC
		if ic == nil || !(ic.Inferior <= ic.Punto && ic.Punto <= ic.Superior) || m.MediaRecortadaS <= 0 {
			t.Fatalf("W=%d: IC %+v, media recortada %v", m.Workers, ic, m.MediaRecortadaS)
		}
		// El cociente de medianas sigue siendo el speedup (tablas.py lo verifica).
		if m.Speedup != bm.Mediciones[0].MedianaS/m.MedianaS {
			t.Errorf("W=%d: speedup %v no es el cociente de medianas", m.Workers, m.Speedup)
		}
	}
	if bm.Mediciones[0].SpeedupIC.Punto != 1 {
		t.Errorf("la base contra sí misma tiene punto 1: %+v", bm.Mediciones[0].SpeedupIC)
	}
	if _, err := os.Stat(ruta); err != nil {
		t.Fatal(err)
	}
}
