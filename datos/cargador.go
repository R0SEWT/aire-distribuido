package datos

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"time"
)

// BloquePorDefecto es B: 64 MB. Con 1.3 GB salen unos 21 bloques, que se
// reparten bien entre 1 y 11 workers.
const BloquePorDefecto = 64 << 20

// Opciones del paralelismo de una carga. Workers y Bloque no cambian el
// resultado: con el mismo Chunk, cualquier combinación da el mismo silver, el
// mismo gold y los mismos conteos, bit a bit. El Chunk sí puede mover el último
// bit de la media y la desviación (otro árbol de suma), nunca el silver.
type Opciones struct {
	Workers int   // goroutines; 0 = runtime.NumCPU()
	Bloque  int64 // bytes por bloque; 0 = BloquePorDefecto
	Chunk   int   // filas por chunk de la estandarización; 0 = receta.Chunk o ChunkPorDefecto

	secuencial bool // tests: procesar y fusionar sin goroutines ni canales (oráculo)
}

func (op Opciones) completar(rec Receta) Opciones {
	if op.Workers <= 0 {
		op.Workers = runtime.NumCPU()
	}
	if op.Bloque <= 0 {
		op.Bloque = BloquePorDefecto
	}
	if op.Chunk <= 0 {
		op.Chunk = rec.Chunk
		if op.Chunk <= 0 {
			op.Chunk = ChunkPorDefecto
		}
	}
	return op
}

// Resultado es todo lo que produce una carga.
type Resultado struct {
	Ancha           *TablaAncha
	Dataset         *Dataset
	Limpieza        *ReporteLimpieza
	Estandarizacion *ReporteEstandarizacion
}

// Cargar abre el CSV largo de OEFA y lo carga con CargarDesde.
func Cargar(ruta string, cfg ConfigLimpieza, rec Receta, op Opciones) (*Resultado, error) {
	f, err := os.Open(ruta)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	return CargarDesde(f, info.Size(), ruta, cfg, rec, op)
}

// CargarDesde lee el CSV largo por bloques con W workers, lo limpia (reglas 1
// a 6), lo pivota a la tabla ancha (reglas 5 y 7), arma el gold con las
// features de la receta (regla 8) y lo estandariza. Ver doc.go.
func CargarDesde(r io.ReaderAt, tam int64, nombre string, cfg ConfigLimpieza, rec Receta, op Opciones) (*Resultado, error) {
	inicio := time.Now()
	if err := rec.Validar(); err != nil {
		return nil, err
	}
	reg, err := cfg.compilar()
	if err != nil {
		return nil, err
	}
	op = op.completar(rec)

	cab, err := leerCabecera(r, tam, reg)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", nombre, err)
	}
	cols, avisos, err := resolverColumnas(cab.nombres, reg)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", nombre, err)
	}
	rangos, err := particionar(r, tam, cab.fin, op.Bloque)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", nombre, err)
	}
	var acum *acumulador
	if op.secuencial {
		acum, err = leerBloquesSecuencial(r, rangos, reg, cols, cab.delimitador)
	} else {
		acum, err = leerBloques(r, rangos, reg, cols, cab.delimitador, op.Workers)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", nombre, err)
	}
	tLectura := time.Now()
	acum.resolverHoras(reg)
	tHoras := time.Now()
	pivotes := acum.pivotar(op.Workers)
	tPivot := time.Now()
	ancha, st, err := acum.tabla(pivotes, reg.zona)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", nombre, err)
	}
	rep, avisosEst := armarReporte(acum, ancha, st, rec, reg)
	tFusion, bloques := acum.tFusion, acum.bloques
	acum, pivotes = nil, nil // los registros ya no hacen falta: se sueltan antes del gold
	tTabla := time.Now()

	ds, est, err := construirGold(ancha, rec, op.Workers, op.Chunk)
	if err != nil {
		return nil, err
	}
	contarGold(rep, ds)
	fin := time.Now()

	avisos = append(avisos, avisosEst...)
	avisos = append(avisos, avisosFeatures(ancha, rec)...)
	rep.Entrada = nombre
	rep.Bytes = tam
	rep.Cabecera = cab.nombres
	rep.BOM = cab.bom
	rep.Delimitador = string(cab.delimitador)
	rep.ConvencionHora = HoraInicio
	if reg.convFin {
		rep.ConvencionHora = HoraFin
	}
	rep.Avisos = avisos
	rep.Workers = op.Workers
	rep.BloqueBytes = op.Bloque
	rep.Bloques = bloques
	rep.Chunk = op.Chunk
	rep.DuracionS = fin.Sub(inicio).Seconds()
	// fases_s permite el análisis de Amdahl: lectura_parseo es paralela (W
	// workers) y contiene a fusion, que es serial pero se solapa con ella;
	// horas y tabla son seriales; pivot es paralelo por estación;
	// gold_estandarizacion es paralelo por chunks. Las tres claves del primer
	// esquema se conservan como sumas.
	rep.FasesS = map[string]float64{
		"lectura_parseo":         tLectura.Sub(inicio).Seconds(),
		"fusion":                 tFusion.Seconds(),
		"horas":                  tHoras.Sub(tLectura).Seconds(),
		"pivot":                  tPivot.Sub(tHoras).Seconds(),
		"tabla":                  tTabla.Sub(tPivot).Seconds(),
		"gold_estandarizacion":   fin.Sub(tTabla).Seconds(),
		"lectura_limpieza_pivot": tPivot.Sub(inicio).Seconds(),
		"orden_tabla_ancha":      tTabla.Sub(tPivot).Seconds(),
	}
	return &Resultado{Ancha: ancha, Dataset: ds, Limpieza: rep, Estandarizacion: est}, nil
}

// avisosFeatures dice qué features de la receta no pueden salir del silver.
func avisosFeatures(t *TablaAncha, rec Receta) []string {
	presente := map[string]bool{}
	for _, p := range t.Parametros {
		presente[p] = true
	}
	var avisos []string
	for _, f := range rec.Features {
		switch {
		case f == FeatureFraccionFina && !(presente["pm10"] && presente["pm2_5"]):
			avisos = append(avisos, "la feature fraccion_fina necesita pm10 y pm2_5 en el silver: el gold queda vacío")
		case f != FeatureFraccionFina && !presente[f]:
			avisos = append(avisos, fmt.Sprintf("la feature %s no está en el silver: el gold queda vacío", f))
		}
	}
	return avisos
}
