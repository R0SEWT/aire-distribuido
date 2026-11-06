package datos

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"time"

	"github.com/R0SEWT/aire-distribuido/kmeans"
)

// ReporteLimpieza es reports/limpieza.json: la evidencia de limpieza del
// informe. Todas las reglas aparecen, aun en cero.
type ReporteLimpieza struct {
	Entrada     string   `json:"entrada"`
	Bytes       int64    `json:"bytes"`
	Sha256      string   `json:"sha256,omitempty"` // de la entrada; lo pone Anotar
	Cabecera    []string `json:"cabecera"`
	BOM         bool     `json:"bom"`
	Delimitador string   `json:"delimitador"`
	// ConvencionHora dice qué instante del intervalo horario es fecha_hora:
	// "fin" (OEFA: la lectura de 00:00 a 01:00 se fecha a las 01:00) o "inicio".
	ConvencionHora      string                      `json:"convencion_hora"`
	FilasLargas         ConteoLargas                `json:"filas_largas"`
	FilasAnchas         ConteoAnchas                `json:"filas_anchas"`
	ParametrosPresentes []string                    `json:"parametros_presentes"`
	Features            []string                    `json:"features"`
	Nivel               string                      `json:"nivel"`
	Estaciones          map[string]*ReporteEstacion `json:"estaciones"`
	Vistos              Vistos                      `json:"vistos"`
	Avisos              []string                    `json:"avisos,omitempty"`
	Workers             int                         `json:"workers"`
	BloqueBytes         int64                       `json:"bloque_bytes"`
	Bloques             int                         `json:"bloques"`
	Chunk               int                         `json:"chunk"`
	DuracionS           float64                     `json:"duracion_s"`
	FasesS              map[string]float64          `json:"fases_s"`
	// Procedencia (Anotar): máquina, Go y commit, y versión de cada config.
	Metadatos *kmeans.Metadatos         `json:"metadatos,omitempty"`
	Configs   map[string]VersionArchivo `json:"configs,omitempty"`
}

// ConteoLargas cuenta filas del CSV largo. Se cumple
// Leidas = Procesadas + Descartadas, Descartadas = Σ DescartadasPorRegla y
// Procesadas = HoraExplicita + HoraInferida. Procesadas son las lecturas que
// llegaron al silver (una por celda).
type ConteoLargas struct {
	Leidas              int64            `json:"leidas"`
	Procesadas          int64            `json:"procesadas"`
	Descartadas         int64            `json:"descartadas"`
	DescartadasPorRegla map[string]int64 `json:"descartadas_por_regla"`
	// MarcadasPorRegla cuenta lo que se conserva pero se señala, en celdas:
	// duplicado_valor_distinto (el valor conservado tuvo un duplicado con otro
	// valor), unidad_convertida (el valor viene de ppb o mmHg), hora_inferida
	// (la hora salió de la posición en el día; igual a HoraInferida) y
	// hora_no_en_punto (truncada a la hora, si la política es truncar). Y en
	// filas largas leídas: ubicacion_corregida (la estación tiene una entrada en
	// correcciones_ubicacion que cambió algún campo de esa fila).
	MarcadasPorRegla             map[string]int64 `json:"marcadas_por_regla"`
	FueraDeRangoPorParametro     map[string]int64 `json:"fuera_de_rango_por_parametro"`
	BloqueEnCeroPorParametro     map[string]int64 `json:"bloque_en_cero_por_parametro"`
	CeroComoFaltantePorParametro map[string]int64 `json:"cero_como_faltante_por_parametro"`
	HoraExplicita                int64            `json:"hora_explicita"`
	HoraInferida                 int64            `json:"hora_inferida"`
	// HoraPorBloqueFisico son filas largas cuya hora salió de su posición en el
	// bloque físico porque el día del parámetro estaba incompleto por filas
	// vacías (plantilla de 2022). Se cuentan al recibir la hora: después aún
	// pueden caer en bloque_en_cero, cero_como_faltante o un duplicado.
	HoraPorBloqueFisico          int64            `json:"hora_por_bloque_fisico"`
	UnidadConvertidaPorParametro map[string]int64 `json:"unidad_convertida_por_parametro"`
}

// ConteoAnchas cuenta filas de la tabla ancha (estación × fecha-hora).
type ConteoAnchas struct {
	Total         int `json:"total"`
	CompletasGold int `json:"completas_gold"`
	// Pm25MayorPm10 son filas con las dos lecturas y PM2.5 > PM10. Se
	// conservan en el silver; en el gold la fracción fina se recorta a 1.
	Pm25MayorPm10 int `json:"pm2_5_mayor_pm10"`
	// HoraInferida son filas con alguna celda de hora reconstruida por posición
	// (columna hora_inferida del silver).
	HoraInferida int `json:"hora_inferida"`
}

// ReporteEstacion resume una estación. Una estación con filas leídas y ninguna
// procesada aparece igual, con ceros: que desaparezca en silencio es peor.
// Ubicacion es la combinación de columnas de ubicación más frecuente, ya con
// correcciones_ubicacion aplicadas, más lat y lon en WGS84 (grados decimales,
// 6 decimales) calculadas desde este, norte y zona; UbicacionOriginal guarda lo
// que traía el CSV en los campos corregidos. Si hubo más de una combinación,
// UbicacionesDistintas las lista todas con sus filas (dos puntos con el mismo
// nombre en lugares distintos se funden: la clave es el nombre).
type ReporteEstacion struct {
	LargasLeidas         int64             `json:"largas_leidas"`
	LargasProcesadas     int64             `json:"largas_procesadas"`
	Anchas               int               `json:"anchas"`
	CompletasGold        int               `json:"completas_gold"`
	Desde                string            `json:"desde,omitempty"`
	Hasta                string            `json:"hasta,omitempty"`
	Ubicacion            map[string]string `json:"ubicacion,omitempty"`
	UbicacionOriginal    map[string]string `json:"ubicacion_original,omitempty"`
	UbicacionesDistintas []ubicacionVista  `json:"ubicaciones_distintas,omitempty"`
}

// Vistos son los valores de texto tal como vienen en el archivo, con su
// frecuencia. Sirven para cerrar la config con el CSV real. Con una columna
// por parámetro no hay parámetros sin mapeo: parametros_no_mapeados queda vacío.
type Vistos struct {
	Parametros           map[string]int64            `json:"parametros"`
	ParametrosNoMapeados map[string]int64            `json:"parametros_no_mapeados"`
	Periodos             map[string]int64            `json:"periodos"`
	Unidades             map[string]map[string]int64 `json:"unidades"`
	ValoresNoNumericos   map[string]int64            `json:"valores_no_numericos"`
}

// armarReporte arma todo lo que sale de la carga y de la tabla ancha; las
// filas completas del gold se agregan después con contarGold, para poder
// soltar el acumulador antes de armar el gold.
func armarReporte(a *acumulador, t *TablaAncha, st estadoTabla, rec Receta, reg *reglas) (*ReporteLimpieza, []string) {
	rep := &ReporteLimpieza{
		FilasLargas: ConteoLargas{
			Leidas:              a.leidas,
			Procesadas:          st.procesadas,
			DescartadasPorRegla: map[string]int64{},
			MarcadasPorRegla: map[string]int64{
				"duplicado_valor_distinto": st.celdasEnConflicto,
				"unidad_convertida":        0,
				"hora_inferida":            st.inferidas,
				"hora_no_en_punto":         a.horaTruncada,
				"ubicacion_corregida":      0,
			},
			FueraDeRangoPorParametro:     map[string]int64{},
			BloqueEnCeroPorParametro:     map[string]int64{},
			CeroComoFaltantePorParametro: map[string]int64{},
			HoraInferida:                 st.inferidas,
			HoraExplicita:                st.procesadas - st.inferidas,
			HoraPorBloqueFisico:          a.horaFisica,
			UnidadConvertidaPorParametro: map[string]int64{},
		},
		FilasAnchas: ConteoAnchas{
			Total:         t.N,
			Pm25MayorPm10: st.pm25MayorPm10,
			HoraInferida:  st.filasInferidas,
		},
		ParametrosPresentes: append([]string{}, t.Parametros...),
		Features:            append([]string{}, rec.Features...),
		Nivel:               rec.Nivel,
		Estaciones:          map[string]*ReporteEstacion{},
		Vistos: Vistos{
			Parametros:           a.parametros,
			ParametrosNoMapeados: map[string]int64{},
			Periodos:             a.periodos,
			Unidades:             map[string]map[string]int64{},
			ValoresNoNumericos:   a.noNumericos,
		},
	}
	for r, n := range a.descartadas {
		rep.FilasLargas.DescartadasPorRegla[nombresReglas[r]] = n
		rep.FilasLargas.Descartadas += n
	}
	for q, n := range a.fueraDeRango {
		if n > 0 {
			rep.FilasLargas.FueraDeRangoPorParametro[ParametrosCanonicos[q]] = n
		}
	}
	for q, n := range a.enCero {
		if n > 0 {
			rep.FilasLargas.BloqueEnCeroPorParametro[ParametrosCanonicos[q]] = n
		}
	}
	for q, n := range a.ceroFaltante {
		if n > 0 {
			rep.FilasLargas.CeroComoFaltantePorParametro[ParametrosCanonicos[q]] = n
		}
	}
	for q, n := range st.convertidas {
		if n > 0 {
			rep.FilasLargas.UnidadConvertidaPorParametro[ParametrosCanonicos[q]] = n
			rep.FilasLargas.MarcadasPorRegla["unidad_convertida"] += n
		}
	}
	for q, us := range a.unidades {
		if us != nil {
			rep.Vistos.Unidades[ParametrosCanonicos[q]] = us
		}
	}

	var avisos []string
	for _, e := range a.estaciones {
		re := &ReporteEstacion{LargasLeidas: e.leidas, LargasProcesadas: st.procesadasPorEstacion[e.nombre]}
		mas := -1
		for i, u := range e.ubicaciones {
			if mas < 0 || u.Filas > e.ubicaciones[mas].Filas {
				mas = i
			}
			if len(u.Original) > 0 {
				rep.FilasLargas.MarcadasPorRegla["ubicacion_corregida"] += u.Filas
			}
		}
		if mas >= 0 && len(e.ubicaciones[mas].Campos) > 0 {
			u := e.ubicaciones[mas]
			re.Ubicacion = maps.Clone(u.Campos) // el mapa es también el de ubicaciones_distintas
			if lat, lon, ok := latLonDeUbicacion(u.Campos); ok {
				re.Ubicacion["lat"] = strconv.FormatFloat(lat, 'f', 6, 64)
				re.Ubicacion["lon"] = strconv.FormatFloat(lon, 'f', 6, 64)
			}
			re.UbicacionOriginal = u.Original
		}
		if len(e.ubicaciones) > 1 {
			re.UbicacionesDistintas = e.ubicaciones
			avisos = append(avisos, fmt.Sprintf("la estación %s aparece con %d ubicaciones distintas (ver ubicaciones_distintas): "+
				"se cargan como una sola estación", e.nombre, len(e.ubicaciones)))
		}
		rep.Estaciones[e.nombre] = re
	}
	sinEstacion := []string{}
	for nombre := range reg.correcciones {
		if _, ok := rep.Estaciones[nombre]; !ok {
			sinEstacion = append(sinEstacion, nombre)
		}
	}
	slices.Sort(sinEstacion)
	for _, nombre := range sinEstacion {
		avisos = append(avisos, fmt.Sprintf("correcciones_ubicacion: la estación %s no está en el archivo; su corrección no se aplica", nombre))
	}
	for i := 0; i < t.N; i++ {
		e := rep.Estaciones[t.Estaciones[t.Estacion[i]]]
		hora := time.Unix(t.Hora[i], 0).In(t.Zona).Format(time.RFC3339)
		if e.Anchas == 0 {
			e.Desde = hora // las filas están ordenadas por fecha dentro de la estación
		}
		e.Hasta = hora
		e.Anchas++
	}
	return rep, avisos
}

// contarGold agrega al reporte las filas completas del gold.
func contarGold(rep *ReporteLimpieza, ds *Dataset) {
	rep.FilasAnchas.CompletasGold = ds.N
	for _, i := range ds.Estacion {
		rep.Estaciones[ds.Estaciones[i]].CompletasGold++
	}
}
