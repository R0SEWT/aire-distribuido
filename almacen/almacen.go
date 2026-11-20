// Package almacen define lo que la API guarda y consulta, detrás de interfaces
// pequeñas (spec de la PC4, §7.3): la API solo conoce estas interfaces. El
// doble en memoria (almacen/memoria) sirve para las pruebas, el CI y el
// esqueleto de la API; las implementaciones con MongoDB y Redis llegan después
// en almacen/mongo y almacen/redis.
//
// Qué guarda cada una (§7.1 y §7.2):
//
//   - Estaciones: la colección estaciones de MongoDB, sembrada desde
//     reports/estaciones_ficha.csv (LeerFichaEstaciones).
//   - Predicciones: la colección predicciones de MongoDB, un documento por
//     lectura predicha.
//   - Cache: en Redis, la clave estacion:{id}:regimen con el último régimen de
//     cada estación. Es la única fuente de ese dato.
//
// Modelos, Entrenamientos y Eventos (las otras tres interfaces de §7.3) llegan
// con el entrenamiento por la API.
package almacen

import (
	"context"
	"errors"
	"time"
)

// ErrNoEncontrado es lo que devuelve una consulta cuando el documento o la
// clave no existe. Se compara con errors.Is.
var ErrNoEncontrado = errors.New("no encontrado")

// Estacion es un documento de la colección estaciones: el código OEFA como id y
// la ficha de §7.1. Un texto que falta en la ficha es nil (null en JSON).
type Estacion struct {
	ID                string  `json:"id"`
	ObjetoFiscalizado *string `json:"objeto_fiscalizado"`
	TipoActividad     *string `json:"tipo_actividad"`
	Departamento      *string `json:"departamento"`
	Provincia         *string `json:"provincia"`
	Distrito          *string `json:"distrito"`
	Lat               float64 `json:"lat"`
	Lon               float64 `json:"lon"`
}

// Media24h es la media de las lecturas de una estación en las 24 h que
// terminan en la fecha_hora de una lectura (esquema Media24h del contrato).
// Con menos de 18 lecturas en la ventana, PM25 y PM10 son nil.
type Media24h struct {
	PM25     *float64 `json:"pm2_5"`
	PM10     *float64 `json:"pm10"`
	Lecturas int      `json:"lecturas"`
}

// Prediccion es un documento de la colección predicciones: la lectura que
// llegó, el régimen que se le asignó y con qué modelo, y la media de 24 h de
// la estación con su alerta y su banda INCA. TS es el momento de la
// predicción y FechaHora, el de la lectura.
type Prediccion struct {
	TS        time.Time
	Estacion  string
	FechaHora time.Time
	PM25      float64
	PM10      float64
	Regimen   int
	ExcedeECA bool
	Media24h  Media24h
	Alerta    bool
	INCA      *string
	ModeloID  string
	Ms        float64
}

// RegimenEstacion es el valor de la clave estacion:{id}:regimen: el estado de
// la estación en su lectura más reciente por fecha_hora (el régimen, la media
// de 24 h, la alerta y la banda INCA). FechaHora es la hora de esa lectura y
// TS, cuándo se predijo.
type RegimenEstacion struct {
	FechaHora time.Time `json:"fecha_hora"`
	Regimen   int       `json:"regimen"`
	ExcedeECA bool      `json:"excede_eca"`
	Media24h  Media24h  `json:"media_24h"`
	Alerta    bool      `json:"alerta"`
	INCA      *string   `json:"inca"`
	TS        time.Time `json:"ts"`
}

// Estaciones es la colección estaciones.
type Estaciones interface {
	// Listar devuelve todas las estaciones, ordenadas por id. Una lista vacía
	// es válida.
	Listar(ctx context.Context) ([]Estacion, error)
	// Obtener devuelve una estación o ErrNoEncontrado.
	Obtener(ctx context.Context, id string) (Estacion, error)
}

// Predicciones es la colección predicciones.
type Predicciones interface {
	Guardar(ctx context.Context, p Prediccion) error
}

// Cache es la caché de Redis.
type Cache interface {
	// GuardarRegimen guarda el régimen de una estación, salvo que el guardado
	// sea de una fecha_hora posterior a la de r: una lectura más antigua no lo
	// reemplaza (una de la misma hora, sí). La comparación y la escritura son
	// una sola operación.
	GuardarRegimen(ctx context.Context, estacion string, r RegimenEstacion) error
	// Regimen devuelve el último régimen de una estación o ErrNoEncontrado.
	Regimen(ctx context.Context, estacion string) (RegimenEstacion, error)
}

// Pinger dice si un almacén responde. Es lo que GET /api/salud informa como
// mongo y redis.
type Pinger interface {
	Ping(ctx context.Context) error
}
