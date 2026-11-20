// Package ventana lleva, por estación y en memoria, las lecturas horarias
// recientes, y con la media de 24 h decide la alerta frente al ECA y la banda
// del INCA. Las dos normas son de 24 horas; el régimen del K-Means describe el
// tipo de aire de una lectura y no entra aquí.
//
// La regla (decisión del equipo; contrato docs/api/openapi.yaml, esquemas
// Lectura, Media24h, Alerta y BandaINCA):
//
//   - fecha_hora se trunca a la hora: hay una lectura por estación y hora, y
//     repetirla la reemplaza.
//   - La ventana de una lectura son las de su estación con fecha_hora en
//     (t − 24 h, t], donde t es la hora de esa lectura, no la del servidor ni la
//     más alta vista. Sirven los datos históricos y las lecturas desordenadas.
//   - Por estación se guardan las 72 lecturas horarias más cercanas a la última
//     recibida: al pasar de 72 se descarta la más lejana a esa (a igual
//     distancia, la más vieja). Una fecha lejana, pasada o futura, desplaza a lo
//     sumo una lectura y no borra las demás. La memoria queda acotada.
//   - Con menos de 18 lecturas en la ventana no hay media (pm2_5 y pm10 null),
//     la alerta es false y el INCA es null.
//
// En el gold, el régimen alto solo acierta el 35 % de las horas cuya media de
// 24 h supera el ECA, y la lectura horaria sobre el umbral, el 22 %: por eso
// la alerta sale de la media y no del régimen.
package ventana

import (
	"cmp"
	"slices"
	"sync"
	"time"

	"github.com/R0SEWT/aire-distribuido/almacen"
	"github.com/R0SEWT/aire-distribuido/kmeans"
)

const (
	// Duracion es el largo de la ventana: el periodo del ECA y del INCA de PM.
	Duracion = 24 * time.Hour
	// MinimoLecturas es lo que hace falta en la ventana para calcular la
	// media: 18 de 24 horas, el 75 %.
	MinimoLecturas = 18
	// Guardadas es cuántas lecturas horarias se conservan por estación: tres
	// ventanas.
	Guardadas = 72
)

// Bandas del INCA (RM 181-2016-MINAM, tabla 3), de la mejor a la peor.
const (
	INCABuena           = "buena"
	INCAModerada        = "moderada"
	INCAMala            = "mala"
	INCAUmbralDeCuidado = "umbral_de_cuidado"
)

var (
	bandas = [...]string{INCABuena, INCAModerada, INCAMala, INCAUmbralDeCuidado}
	// Límites superiores, en µg/m³ de 24 h, de buena, moderada y mala; por
	// encima del último es el umbral de cuidado. Son los de la tabla tbl-inca
	// de docs/informe/secciones/_06-modelo.qmd.
	cortesPM10 = [...]float64{75, 150, 250}
	cortesPM25 = [...]float64{12.5, 25, 125}
)

// Alerta dice si la media de 24 h supera el ECA de 24 h (DS 003-2017-MINAM):
// PM2.5 > 50 µg/m³ o PM10 > 100 µg/m³. Sin media (menos de 18 lecturas) es
// false.
func Alerta(m almacen.Media24h) bool {
	return m.PM25 != nil && m.PM10 != nil && (*m.PM25 > kmeans.ECAPM25 || *m.PM10 > kmeans.ECAPM10)
}

// INCA devuelve la banda del INCA de la media de 24 h: la peor entre la de
// PM10 y la de PM2.5. Sin media es nil (null en JSON).
func INCA(m almacen.Media24h) *string {
	if m.PM25 == nil || m.PM10 == nil {
		return nil
	}
	b := bandas[max(banda(*m.PM10, cortesPM10), banda(*m.PM25, cortesPM25))]
	return &b
}

func banda(x float64, cortes [3]float64) int {
	for i, c := range cortes {
		if x <= c {
			return i
		}
	}
	return len(cortes)
}

// Hora trunca una fecha_hora a la hora en punto: la clave de una lectura.
func Hora(t time.Time) time.Time { return t.Truncate(time.Hour) }

// Ventanas son las ventanas de todas las estaciones. Es seguro usarlo desde
// varias goroutines: cada estación tiene su propio mutex, así que dos
// estaciones no se esperan entre sí. Quien llama decide qué estaciones entran
// (la API solo deja pasar las del catálogo).
type Ventanas struct {
	mu          sync.RWMutex
	porEstacion map[string]*ventana
}

func Nuevas() *Ventanas { return &Ventanas{porEstacion: map[string]*ventana{}} }

// ventana son las lecturas guardadas de una estación, ordenadas por hora, a
// lo sumo Guardadas.
type ventana struct {
	mu     sync.Mutex
	puntos []punto
}

type punto struct {
	h          int64 // horas desde la época Unix: la misma hora en otra zona es la misma lectura
	pm25, pm10 float64
}

const horasVentana = int64(Duracion / time.Hour)

// Agregar guarda una lectura (o reemplaza la de la misma hora) y devuelve la
// media de su ventana (t − 24 h, t].
func (v *Ventanas) Agregar(estacion string, fechaHora time.Time, pm25, pm10 float64) almacen.Media24h {
	w := v.de(estacion)
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.agregar(punto{h: Hora(fechaHora).Unix() / 3600, pm25: pm25, pm10: pm10})
}

func (v *Ventanas) de(estacion string) *ventana {
	v.mu.RLock()
	w, ok := v.porEstacion[estacion]
	v.mu.RUnlock()
	if ok {
		return w
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if w, ok = v.porEstacion[estacion]; !ok {
		w = &ventana{}
		v.porEstacion[estacion] = w
	}
	return w
}

func (w *ventana) agregar(p punto) almacen.Media24h {
	i, existe := slices.BinarySearchFunc(w.puntos, p.h, compararH)
	if existe {
		w.puntos[i] = p
	} else {
		w.puntos = slices.Insert(w.puntos, i, p)
		if len(w.puntos) > Guardadas {
			// Sale la más lejana a la que acaba de llegar, que está en un extremo
			// porque están ordenadas; a igual distancia, la más vieja.
			primera, ultima := w.puntos[0].h, w.puntos[len(w.puntos)-1].h
			if p.h-primera >= ultima-p.h {
				w.puntos = slices.Delete(w.puntos, 0, 1)
			} else {
				w.puntos = w.puntos[:len(w.puntos)-1]
			}
		}
	}
	desde, _ := slices.BinarySearchFunc(w.puntos, p.h-horasVentana+1, compararH)
	hasta, _ := slices.BinarySearchFunc(w.puntos, p.h+1, compararH)
	return calcular(w.puntos[desde:hasta])
}

func compararH(q punto, h int64) int { return cmp.Compare(q.h, h) }

// calcular es la media aritmética de las lecturas, en orden de hora, así que
// no depende del orden en que llegaron.
func calcular(ps []punto) almacen.Media24h {
	m := almacen.Media24h{Lecturas: len(ps)}
	if len(ps) < MinimoLecturas {
		return m
	}
	var s25, s10 float64
	for _, p := range ps {
		s25 += p.pm25
		s10 += p.pm10
	}
	pm25, pm10 := s25/float64(len(ps)), s10/float64(len(ps))
	m.PM25, m.PM10 = &pm25, &pm10
	return m
}

// lecturasGuardadas y extremos son para las pruebas.
func (v *Ventanas) lecturasGuardadas(estacion string) int {
	w := v.de(estacion)
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.puntos)
}

func (v *Ventanas) extremos(estacion string) (time.Time, time.Time) {
	w := v.de(estacion)
	w.mu.Lock()
	defer w.mu.Unlock()
	return time.Unix(w.puntos[0].h*3600, 0), time.Unix(w.puntos[len(w.puntos)-1].h*3600, 0)
}
