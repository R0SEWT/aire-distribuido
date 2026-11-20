package almacen

import (
	"context"
	"time"

	"github.com/R0SEWT/aire-distribuido/kmeans"
)

// OrigenSembrado es el origen de un modelo que la API leyó de reports/ al
// arrancar. El de un modelo entrenado por la API es el id de su entrenamiento.
const OrigenSembrado = "sembrado"

// Modelo es un documento de la colección modelos y también el valor de la
// clave modelo:activo de Redis (spec §7.1 y §7.2): todo lo que hace falta para
// volver a armar el modelo y predecir con él, y de dónde salió.
type Modelo struct {
	ID     string    `json:"_id"`
	Creado time.Time `json:"creado"`
	// Origen es OrigenSembrado o el id del entrenamiento que lo produjo.
	Origen          string                 `json:"origen"`
	Receta          kmeans.Receta          `json:"receta"`
	GoldSHA256      string                 `json:"gold_sha256"`
	Estandarizacion kmeans.Estandarizacion `json:"estandarizacion"`
	// CentroidesStd son los k centroides estandarizados, con los que se
	// predice. Centroides son los mismos en unidades originales, o nil con
	// nivel = estacion (un centroide mezcla estaciones y no tiene unidades).
	CentroidesStd [][]float64     `json:"centroides_std"`
	Centroides    [][]float64     `json:"centroides"`
	Regimenes     []RegimenModelo `json:"regimenes"`
	Inercia       float64         `json:"inercia"`
	Iteraciones   int             `json:"iteraciones"`
	Activo        bool            `json:"activo"`
}

// RegimenModelo es el perfil de un régimen dentro del documento del modelo:
// los mismos campos, en el mismo orden, que modelo.Regimen.
type RegimenModelo struct {
	Regimen   int     `json:"regimen"`
	PM25Medio float64 `json:"pm2_5_medio"`
	PM10Medio float64 `json:"pm10_medio"`
	ExcedeECA bool    `json:"excede_eca"`
	Tamano    int     `json:"tamano"`
}

// Modelos es la colección modelos.
type Modelos interface {
	// Guardar crea el modelo o lo reemplaza si ya existe uno con su id. Si
	// m.Activo es true, los demás dejan de estar activos: a lo sumo uno lo
	// está.
	Guardar(ctx context.Context, m Modelo) error
	// Activo devuelve el modelo activo o ErrNoEncontrado.
	Activo(ctx context.Context) (Modelo, error)
}

// CacheModelo es la clave modelo:activo de Redis: el modelo con el que la API
// arranca en caliente.
type CacheModelo interface {
	GuardarModeloActivo(ctx context.Context, m Modelo) error
	// ModeloActivo devuelve el modelo guardado o ErrNoEncontrado.
	ModeloActivo(ctx context.Context) (Modelo, error)
}
