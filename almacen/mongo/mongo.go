// Package mongo implementa con MongoDB las interfaces de almacen que son
// colecciones (spec de la PC4, §7.1): estaciones, predicciones, modelos,
// entrenamientos y eventos_cluster. Con almacen/redis, es uno de los dos
// únicos paquetes del repositorio que importan un driver.
//
// # Documentos
//
// Cada documento tiene los campos de §7.1 con los nombres de
// docs/datos/ejemplos/ (snake_case y _id). almacen.Modelo ya es un documento y
// se guarda tal cual: el driver usa su etiqueta json como nombre del campo. Los
// demás tipos de almacen tienen aquí su tipo de documento con etiquetas bson
// (estacionDoc, prediccionDoc, entrenamientoDoc, eventoDoc), porque en la API
// se llaman distinto (id en vez de _id) o no llevan etiquetas.
//
// Un campo que todavía no tiene valor se guarda como null, no como cero ni
// como texto vacío: el fin, el paro, dif_secuencial y ms_total de un
// entrenamiento en curso, o el error de uno que terminó bien.
//
// # Fechas
//
// Todas las fechas (ts, fecha_hora, creado, inicio, fin) se guardan como BSON
// Date, nunca como texto: así un campo tiene un solo tipo, el índice
// {estacion, ts} ordena por tiempo y mongosh las muestra como ISODate. Un BSON
// Date es un instante en UTC con precisión de milisegundos, así que al guardar
// se pierden dos cosas:
//
//   - el desfase -05:00 con que la API recibe y escribe las fechas. El instante
//     es el mismo; al leer vuelve en UTC y la API lo expresa otra vez en la
//     hora de Lima;
//   - lo que haya por debajo del milisegundo (el creado de un modelo viene con
//     nanosegundos). Por eso la duración de un entrenamiento se guarda en
//     ms_total y no se calcula restando inicio y fin.
//
// # Índices
//
// Este paquete no crea índices: los crea docs/datos/seed.js, con nombre propio
// (activo, creado, inicio, estacion_ts, ts). Crearlos también desde aquí con
// otro nombre fallaría con IndexOptionsConflict. La prueba de integración
// comprueba que existan.
package mongo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	driver "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"

	"github.com/R0SEWT/aire-distribuido/almacen"
)

// Nombres de las colecciones (§7.1), los mismos que crea docs/datos/seed.js.
const (
	colEstaciones     = "estaciones"
	colPredicciones   = "predicciones"
	colModelos        = "modelos"
	colEntrenamientos = "entrenamientos"
	colEventos        = "eventos_cluster"
)

// Plazos del cliente. Una operación cuyo contexto no trae plazo (un handler de
// la API) no espera más que plazoOperacion; elegir un servidor al que mandarla
// no espera más que plazoSeleccion, así que con MongoDB caído una consulta
// falla en dos segundos en vez de colgarse los 30 que trae el driver.
const (
	plazoOperacion = 5 * time.Second
	plazoSeleccion = 2 * time.Second
)

// opcionesBSON es cómo se codifican los documentos: el nombre de un campo sale
// de la etiqueta bson y, si no hay, de la etiqueta json; un slice nil se guarda
// como un arreglo vacío y no como null, para que un campo que es un arreglo lo
// sea siempre ($push sobre rondas lo necesita).
func opcionesBSON() *options.BSONOptions {
	return &options.BSONOptions{UseJSONStructTags: true, NilSliceAsEmpty: true}
}

// Almacen es la base de datos de la API en MongoDB.
type Almacen struct {
	cliente *driver.Client
	db      *driver.Database
}

var _ almacen.Pinger = (*Almacen)(nil)

// Conectar prepara el cliente para la base de datos base en uri (por ejemplo
// mongodb://mongo:27017). Solo falla si uri no es válida: no espera a que
// MongoDB responda, y el cliente se conecta y se vuelve a conectar solo. Para
// saber si responde está Ping.
func Conectar(uri, base string) (*Almacen, error) {
	if base == "" {
		return nil, errors.New("mongo: falta el nombre de la base de datos")
	}
	cliente, err := driver.Connect(options.Client().
		ApplyURI(uri).
		SetAppName("aire-distribuido-api").
		SetBSONOptions(opcionesBSON()).
		SetServerSelectionTimeout(plazoSeleccion).
		SetTimeout(plazoOperacion))
	if err != nil {
		return nil, fmt.Errorf("mongo: %w", err)
	}
	return &Almacen{cliente: cliente, db: cliente.Database(base)}, nil
}

// Ping dice si MongoDB responde.
func (a *Almacen) Ping(ctx context.Context) error {
	return a.cliente.Ping(ctx, readpref.Primary())
}

// Cerrar cierra las conexiones.
func (a *Almacen) Cerrar(ctx context.Context) error { return a.cliente.Disconnect(ctx) }

// Estaciones es la colección estaciones.
func (a *Almacen) Estaciones() *Estaciones { return &Estaciones{a.db.Collection(colEstaciones)} }

// Predicciones es la colección predicciones.
func (a *Almacen) Predicciones() *Predicciones {
	return &Predicciones{a.db.Collection(colPredicciones)}
}

// Modelos es la colección modelos.
func (a *Almacen) Modelos() *Modelos { return &Modelos{a.db.Collection(colModelos)} }

// Entrenamientos es la colección entrenamientos.
func (a *Almacen) Entrenamientos() *Entrenamientos {
	return &Entrenamientos{a.db.Collection(colEntrenamientos)}
}

// Eventos es la colección eventos_cluster.
func (a *Almacen) Eventos() *Eventos { return &Eventos{a.db.Collection(colEventos)} }

// noEncontrado traduce el «no hay documentos» del driver al error de almacen.
func noEncontrado(err error) error {
	if errors.Is(err, driver.ErrNoDocuments) {
		return almacen.ErrNoEncontrado
	}
	return err
}

// estacionDoc es un documento de estaciones. lat y lon son punteros porque el
// seed guarda null cuando la ficha no trae la coordenada.
type estacionDoc struct {
	ID                string   `bson:"_id"`
	ObjetoFiscalizado *string  `bson:"objeto_fiscalizado"`
	TipoActividad     *string  `bson:"tipo_actividad"`
	Departamento      *string  `bson:"departamento"`
	Provincia         *string  `bson:"provincia"`
	Distrito          *string  `bson:"distrito"`
	Lat               *float64 `bson:"lat"`
	Lon               *float64 `bson:"lon"`
}

func aEstacionDoc(e almacen.Estacion) estacionDoc {
	return estacionDoc{e.ID, e.ObjetoFiscalizado, e.TipoActividad, e.Departamento, e.Provincia, e.Distrito, &e.Lat, &e.Lon}
}

// estacion falla si falta una coordenada: el contrato (Estacion en
// docs/api/openapi.yaml) no las deja ser null.
func (d estacionDoc) estacion() (almacen.Estacion, error) {
	if d.Lat == nil || d.Lon == nil {
		return almacen.Estacion{}, fmt.Errorf("mongo: la estación %s no tiene lat o lon", d.ID)
	}
	return almacen.Estacion{
		ID: d.ID, ObjetoFiscalizado: d.ObjetoFiscalizado, TipoActividad: d.TipoActividad,
		Departamento: d.Departamento, Provincia: d.Provincia, Distrito: d.Distrito, Lat: *d.Lat, Lon: *d.Lon,
	}, nil
}

// Estaciones implementa almacen.Estaciones. Solo lee: la colección la siembra
// docs/datos/seed.js desde la ficha.
type Estaciones struct{ col *driver.Collection }

var _ almacen.Estaciones = (*Estaciones)(nil)

func (e *Estaciones) Listar(ctx context.Context) ([]almacen.Estacion, error) {
	cur, err := e.col.Find(ctx, bson.D{}, options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}))
	if err != nil {
		return nil, err
	}
	var docs []estacionDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	es := make([]almacen.Estacion, len(docs))
	for i, d := range docs {
		if es[i], err = d.estacion(); err != nil {
			return nil, err
		}
	}
	return es, nil
}

func (e *Estaciones) Obtener(ctx context.Context, id string) (almacen.Estacion, error) {
	var d estacionDoc
	if err := e.col.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&d); err != nil {
		return almacen.Estacion{}, noEncontrado(err)
	}
	return d.estacion()
}

// prediccionDoc es un documento de predicciones: los campos de
// almacen.Prediccion, en el mismo orden, con el nombre con que se guardan. El
// _id es el ObjectId que genera el driver: una estación puede tener varias
// predicciones de la misma hora.
type prediccionDoc struct {
	TS        time.Time        `bson:"ts"`
	Estacion  string           `bson:"estacion"`
	FechaHora time.Time        `bson:"fecha_hora"`
	PM25      float64          `bson:"pm2_5"`
	PM10      float64          `bson:"pm10"`
	Regimen   int              `bson:"regimen"`
	ExcedeECA bool             `bson:"excede_eca"`
	Media24h  almacen.Media24h `bson:"media_24h"`
	Alerta    bool             `bson:"alerta"`
	INCA      *string          `bson:"inca"`
	ModeloID  string           `bson:"modelo_id"`
	Ms        float64          `bson:"ms"`
}

// Predicciones implementa almacen.Predicciones.
type Predicciones struct{ col *driver.Collection }

var _ almacen.Predicciones = (*Predicciones)(nil)

// GuardarLote inserta el lote con un solo insertMany sin orden: un documento
// que falla no impide insertar los demás.
func (p *Predicciones) GuardarLote(ctx context.Context, ps []almacen.Prediccion) error {
	if len(ps) == 0 {
		return nil
	}
	docs := make([]prediccionDoc, len(ps))
	for i, pr := range ps {
		docs[i] = prediccionDoc(pr)
	}
	_, err := p.col.InsertMany(ctx, docs, options.InsertMany().SetOrdered(false))
	return err
}

// DeEstacion devuelve a lo sumo n predicciones de una estación, de la más
// reciente a la más antigua por ts (cuándo se predijo). Es la consulta para la
// que existe el índice {estacion, ts}.
func (p *Predicciones) DeEstacion(ctx context.Context, estacion string, n int) ([]almacen.Prediccion, error) {
	ps := []almacen.Prediccion{}
	if n <= 0 {
		return ps, nil
	}
	cur, err := p.col.Find(ctx, bson.D{{Key: "estacion", Value: estacion}},
		options.Find().SetSort(bson.D{{Key: "ts", Value: -1}}).SetLimit(int64(n)))
	if err != nil {
		return nil, err
	}
	var docs []prediccionDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	for _, d := range docs {
		ps = append(ps, almacen.Prediccion(d))
	}
	return ps, nil
}

// Modelos implementa almacen.Modelos.
type Modelos struct{ col *driver.Collection }

var _ almacen.Modelos = (*Modelos)(nil)

// Guardar reemplaza el modelo y, si queda activo, desactiva los demás. Son dos
// escrituras y MongoDB sin réplicas no tiene transacciones: entre una y otra
// puede haber dos modelos activos, y Activo elige el más nuevo.
func (m *Modelos) Guardar(ctx context.Context, mod almacen.Modelo) error {
	if mod.ID == "" {
		return errors.New("mongo: el modelo no tiene id")
	}
	if _, err := m.col.ReplaceOne(ctx, bson.D{{Key: "_id", Value: mod.ID}}, mod, options.Replace().SetUpsert(true)); err != nil {
		return err
	}
	if !mod.Activo {
		return nil
	}
	_, err := m.col.UpdateMany(ctx,
		bson.D{{Key: "activo", Value: true}, {Key: "_id", Value: bson.D{{Key: "$ne", Value: mod.ID}}}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "activo", Value: false}}}})
	return err
}

func (m *Modelos) Activo(ctx context.Context) (almacen.Modelo, error) {
	var mod almacen.Modelo
	err := m.col.FindOne(ctx, bson.D{{Key: "activo", Value: true}},
		options.FindOne().SetSort(bson.D{{Key: "creado", Value: -1}})).Decode(&mod)
	return mod, noEncontrado(err)
}

// entrenamientoDoc es un documento de entrenamientos. Los campos del resultado
// son punteros: null mientras no hay resultado.
type entrenamientoDoc struct {
	ID            string          `bson:"_id"`
	Inicio        time.Time       `bson:"inicio"`
	Fin           *time.Time      `bson:"fin"`
	Estado        string          `bson:"estado"`
	Parametros    parametrosDoc   `bson:"parametros"`
	Nodos         []nodoDoc       `bson:"nodos"`
	Rondas        []almacen.Ronda `bson:"rondas"`
	Iteraciones   int             `bson:"iteraciones"`
	Paro          *string         `bson:"paro"`
	DifSecuencial *float64        `bson:"dif_secuencial"`
	MsTotal       *float64        `bson:"ms_total"`
	ModeloID      *string         `bson:"modelo_id"`
	Aviso         *string         `bson:"aviso"`
	Error         *string         `bson:"error"`
}

type parametrosDoc struct {
	Nodos    int   `bson:"nodos"`
	Replicas int   `bson:"replicas"`
	Semilla  int64 `bson:"semilla"`
}

type nodoDoc struct {
	Nombre    string   `bson:"nombre"`
	Direccion string   `bson:"direccion"`
	Workers   int      `bson:"workers"`
	Rango     rangoDoc `bson:"rango"`
}

// rangoDoc es el rango de filas del gold de un nodo, [desde, hasta).
type rangoDoc struct {
	Desde int `bson:"desde"`
	Hasta int `bson:"hasta"`
}

// textoONulo guarda un texto vacío como null.
func textoONulo(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func deref[T any](p *T) T {
	var cero T
	if p == nil {
		return cero
	}
	return *p
}

// resultado son los campos que escribe el cierre. dif_secuencial y ms_total
// solo tienen valor si la corrida terminó: en una abortada no se midieron, y
// un 0 ahí se leería como «idéntico al secuencial».
type resultado struct {
	fin                           *time.Time
	paro, modeloID, aviso, motivo *string
	dif, ms                       *float64
}

func resultadoDe(estado string, fin time.Time, paro string, dif, ms float64, modeloID, aviso, motivo string) resultado {
	r := resultado{paro: textoONulo(paro), modeloID: textoONulo(modeloID), aviso: textoONulo(aviso), motivo: textoONulo(motivo)}
	if !fin.IsZero() {
		r.fin = &fin
	}
	if estado == almacen.EstadoTerminado {
		r.dif, r.ms = &dif, &ms
	}
	return r
}

func aEntrenamientoDoc(e almacen.Entrenamiento) entrenamientoDoc {
	r := resultadoDe(e.Estado, e.Fin, e.Paro, e.DifSecuencial, e.MsTotal, e.ModeloID, e.Aviso, e.Error)
	d := entrenamientoDoc{
		ID: e.ID, Inicio: e.Inicio, Fin: r.fin, Estado: e.Estado, Parametros: parametrosDoc(e.Parametros),
		Nodos: make([]nodoDoc, len(e.Nodos)), Rondas: e.Rondas, Iteraciones: e.Iteraciones,
		Paro: r.paro, DifSecuencial: r.dif, MsTotal: r.ms, ModeloID: r.modeloID, Aviso: r.aviso, Error: r.motivo,
	}
	for i, n := range e.Nodos {
		d.Nodos[i] = nodoDoc{n.Nombre, n.Direccion, n.Workers, rangoDoc{n.Desde, n.Hasta}}
	}
	return d
}

func (d entrenamientoDoc) entrenamiento() almacen.Entrenamiento {
	e := almacen.Entrenamiento{
		ID: d.ID, Inicio: d.Inicio, Fin: deref(d.Fin), Estado: d.Estado, Parametros: almacen.ParametrosEntrenamiento(d.Parametros),
		Nodos: make([]almacen.NodoEntrenamiento, len(d.Nodos)), Rondas: d.Rondas, Iteraciones: d.Iteraciones,
		Paro: deref(d.Paro), DifSecuencial: deref(d.DifSecuencial), MsTotal: deref(d.MsTotal),
		ModeloID: deref(d.ModeloID), Aviso: deref(d.Aviso), Error: deref(d.Error),
	}
	for i, n := range d.Nodos {
		e.Nodos[i] = almacen.NodoEntrenamiento{Nombre: n.Nombre, Direccion: n.Direccion, Workers: n.Workers, Desde: n.Rango.Desde, Hasta: n.Rango.Hasta}
	}
	return e
}

// Entrenamientos implementa almacen.Entrenamientos.
type Entrenamientos struct{ col *driver.Collection }

var _ almacen.Entrenamientos = (*Entrenamientos)(nil)

func (e *Entrenamientos) Crear(ctx context.Context, en almacen.Entrenamiento) error {
	if en.ID == "" {
		return errors.New("mongo: el entrenamiento no tiene id")
	}
	_, err := e.col.InsertOne(ctx, aEntrenamientoDoc(en))
	if driver.IsDuplicateKeyError(err) {
		return fmt.Errorf("mongo: entrenamiento %s: %w", en.ID, almacen.ErrYaExiste)
	}
	return err
}

// AgregarRonda es un $push: no lee ni reescribe las rondas anteriores.
func (e *Entrenamientos) AgregarRonda(ctx context.Context, id string, r almacen.Ronda) error {
	return e.actualizar(ctx, id, bson.D{{Key: "$push", Value: bson.D{{Key: "rondas", Value: r}}}})
}

// Cerrar es un $set de los campos del resultado: no toca las rondas.
func (e *Entrenamientos) Cerrar(ctx context.Context, id string, c almacen.CierreEntrenamiento) error {
	r := resultadoDe(c.Estado, c.Fin, c.Paro, c.DifSecuencial, c.MsTotal, c.ModeloID, c.Aviso, c.Error)
	return e.actualizar(ctx, id, bson.D{{Key: "$set", Value: bson.D{
		{Key: "fin", Value: r.fin},
		{Key: "estado", Value: c.Estado},
		{Key: "iteraciones", Value: c.Iteraciones},
		{Key: "paro", Value: r.paro},
		{Key: "dif_secuencial", Value: r.dif},
		{Key: "ms_total", Value: r.ms},
		{Key: "modelo_id", Value: r.modeloID},
		{Key: "aviso", Value: r.aviso},
		{Key: "error", Value: r.motivo},
	}}})
}

func (e *Entrenamientos) actualizar(ctx context.Context, id string, cambio bson.D) error {
	res, err := e.col.UpdateByID(ctx, id, cambio)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return almacen.ErrNoEncontrado
	}
	return nil
}

func (e *Entrenamientos) Obtener(ctx context.Context, id string) (almacen.Entrenamiento, error) {
	var d entrenamientoDoc
	if err := e.col.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&d); err != nil {
		return almacen.Entrenamiento{}, noEncontrado(err)
	}
	return d.entrenamiento(), nil
}

// Listar usa el índice inicio y no trae las rondas.
func (e *Entrenamientos) Listar(ctx context.Context) ([]almacen.Entrenamiento, error) {
	cur, err := e.col.Find(ctx, bson.D{}, options.Find().
		SetSort(bson.D{{Key: "inicio", Value: -1}, {Key: "_id", Value: -1}}).
		SetProjection(bson.D{{Key: "rondas", Value: 0}}))
	if err != nil {
		return nil, err
	}
	var docs []entrenamientoDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	lista := make([]almacen.Entrenamiento, len(docs))
	for i, d := range docs {
		lista[i] = d.entrenamiento()
	}
	return lista, nil
}

// eventoDoc es un documento de eventos_cluster. El _id es el ObjectId que
// genera el driver.
type eventoDoc struct {
	TS      time.Time `bson:"ts"`
	Tipo    string    `bson:"tipo"`
	Nodo    string    `bson:"nodo"`
	Detalle string    `bson:"detalle"`
}

// Eventos implementa almacen.Eventos.
type Eventos struct{ col *driver.Collection }

var _ almacen.Eventos = (*Eventos)(nil)

func (e *Eventos) Guardar(ctx context.Context, ev almacen.EventoCluster) error {
	_, err := e.col.InsertOne(ctx, eventoDoc(ev))
	return err
}

// Recientes devuelve a lo sumo n eventos, del más reciente al más antiguo.
// Usa el índice ts. Entre dos eventos del mismo milisegundo, primero el que se
// insertó último (el ObjectId crece con el tiempo).
func (e *Eventos) Recientes(ctx context.Context, n int) ([]almacen.EventoCluster, error) {
	lista := []almacen.EventoCluster{}
	if n <= 0 {
		return lista, nil
	}
	cur, err := e.col.Find(ctx, bson.D{}, options.Find().
		SetSort(bson.D{{Key: "ts", Value: -1}, {Key: "_id", Value: -1}}).SetLimit(int64(n)))
	if err != nil {
		return nil, err
	}
	var docs []eventoDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	for _, d := range docs {
		lista = append(lista, almacen.EventoCluster(d))
	}
	return lista, nil
}
