package api

import (
	"context"
	"errors"
	"net"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/R0SEWT/aire-distribuido/almacen"
	"github.com/R0SEWT/aire-distribuido/almacen/memoria"
	"github.com/R0SEWT/aire-distribuido/cluster"
	"github.com/R0SEWT/aire-distribuido/kmeans"
	"github.com/R0SEWT/aire-distribuido/protocolo"
)

// puntosAnotados es un almacén de puntos de control que anota cada escritura y
// puede trabarla o hacerla fallar.
type puntosAnotados struct {
	*memoria.Cache
	mu      sync.Mutex
	iters   []int
	falla   bool
	trabado chan struct{} // si no es nil, la primera escritura espera a que se cierre
	entro   chan struct{} // avisa que la primera escritura llegó
	una     sync.Once
}

func (p *puntosAnotados) GuardarPunto(ctx context.Context, id string, pc almacen.PuntoControl) error {
	if p.trabado != nil {
		p.una.Do(func() { close(p.entro) })
		<-p.trabado
	}
	p.mu.Lock()
	p.iters = append(p.iters, pc.Iter)
	p.mu.Unlock()
	if p.falla {
		return errors.New("Redis no responde")
	}
	return p.Cache.GuardarPunto(ctx, id, pc)
}

func (p *puntosAnotados) escrituras() []int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.iters)
}

// estadoPuntosDe lee de /api/metricas lo que el coordinador hizo con los puntos de control.
func (ec *entornoCluster) estadoPuntosDe(t *testing.T) (escritos, fallidos, omitidos float64) {
	t.Helper()
	mt := objeto(t, ec.pedir(t, "GET", "/api/metricas", ec.admin, ""), 200, "puntos_de_control")
	pc := mt["puntos_de_control"].(map[string]any)
	return pc["escritos"].(float64), pc["fallidos"].(float64), pc["omitidos"].(float64)
}

// El punto de control se escribe en cada ronda (spec §5.2 y §7.2), no solo al
// final: la iteración guardada avanza durante la corrida y termina en la
// última, con los centroides del modelo.
func TestElPuntoDeControlAvanzaConCadaRonda(t *testing.T) {
	puntos := &puntosAnotados{Cache: memoria.NuevaCache()}
	ec := nuevoEntornoCluster(t, func(c *Config) { c.Puntos = puntos })
	ec.lanzarNodos(t, 3)
	id := ec.entrenar(t, `{"nodos": 3, "replicas": 1, "semilla": 42}`)
	det := ec.esperarFin(t, id)
	iteraciones := int(det["iteraciones"].(float64))
	if det["estado"] != "terminado" || iteraciones < 4 {
		t.Fatalf("detalle = %v; hace falta una corrida de varias rondas", det)
	}
	iters := puntos.escrituras()
	if len(iters) < 2 || iters[len(iters)-1] != iteraciones {
		t.Fatalf("se escribieron los puntos de las rondas %v; se esperaba más de uno y que el último fuera el de la ronda %d", iters, iteraciones)
	}
	for i := 1; i < len(iters); i++ {
		if iters[i] <= iters[i-1] {
			t.Fatalf("el punto de control no avanza: rondas %v", iters)
		}
	}
	// Con el almacén al día no se saltea ninguna ronda: lo escrito más lo
	// salteado son todas.
	escritos, fallidos, omitidos := ec.estadoPuntosDe(t)
	if int(escritos) != len(iters) || fallidos != 0 || int(escritos+omitidos) != iteraciones {
		t.Fatalf("puntos_de_control = %v escritos, %v fallidos, %v omitidos; hubo %d rondas y %d escrituras", escritos, fallidos, omitidos, iteraciones, len(iters))
	}
	punto, err := puntos.Punto(context.Background(), id)
	if err != nil || punto.Iter != iteraciones ||
		!slices.Equal(slices.Concat(punto.Centroides...), ec.srv.modelo.Load().CentroidesEstandarizados()) {
		t.Fatalf("punto final = %+v, %v; se esperaba el de la ronda %d con los centroides del modelo", punto, err, iteraciones)
	}
}

// nodoFragil responde el CARGAR y las primeras rondas y después se apaga.
func (ec *entornoCluster) nodoFragil(t *testing.T, nombre string, rondasQueResponde int) {
	t.Helper()
	conn, err := net.Dial("tcp", ec.dirNodos)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	go func() {
		cod, dec := protocolo.NuevoCodificador(conn), protocolo.NuevoDecodificador(conn)
		cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Hola, Version: protocolo.Version, Nodo: nombre, Workers: 1})
		var datos *kmeans.Datos
		rondas := 0
		for {
			m, err := dec.Leer()
			if err != nil {
				return
			}
			switch m.Tipo {
			case protocolo.Ping:
				cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Pong})
			case protocolo.Cargar:
				datos = cluster.FilasVirtuales(ec.ent.Gold.Datos, *m.Rango)
				cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Listo, N: datos.N, Huella: m.Huella})
			case protocolo.Iterar:
				if rondas++; rondas > rondasQueResponde {
					conn.Close()
					return
				}
				p := kmeans.ParcialesPorChunks(datos, m.Centroides, 1, chunkDePrueba)
				cod.Escribir(protocolo.Mensaje{Tipo: protocolo.Parcial, Iter: m.Iter, Sumas: p.Sumas, Conteos: p.Conteos, Inercia: p.Inercia})
			}
		}
	}()
}

// Si un nodo cae a mitad de la corrida, el punto de control que queda es el de
// la última ronda completa: es lo que la corrida alcanzó a hacer.
func TestSiUnNodoCaeQuedaElPuntoDeLaUltimaRondaCompleta(t *testing.T) {
	puntos := &puntosAnotados{Cache: memoria.NuevaCache()}
	ec := nuevoEntornoCluster(t, func(c *Config) { c.Puntos = puntos })
	ec.lanzarNodos(t, 1)
	ec.nodoFragil(t, "nodo-2-fragil", 3) // responde tres rondas y se apaga
	ec.esperarLibres(t, 2)
	id := ec.entrenar(t, `{"nodos": 2, "replicas": 1, "semilla": 42}`)
	det := ec.esperarFin(t, id)
	if det["estado"] != "abortado" || len(det["rondas"].([]any)) != 3 {
		t.Fatalf("detalle = %v; se esperaba abortado con tres rondas hechas", det)
	}
	punto, err := puntos.Punto(context.Background(), id)
	if err != nil || punto.Iter != 3 || len(punto.Centroides) != 3 || len(punto.Centroides[0]) != 2 {
		t.Fatalf("punto = %+v, %v; se esperaba el de la ronda 3, con k = 3 centroides de 2 valores", punto, err)
	}
	if iters := puntos.escrituras(); iters[len(iters)-1] != 3 {
		t.Fatalf("la última escritura fue la de la ronda %d: %v", iters[len(iters)-1], iters)
	}
}

// La escritura del punto no frena la corrida. Con Redis trabado en la primera
// escritura, todas las rondas se hacen y llegan al almacén de entrenamientos;
// cuando Redis vuelve, se escribe el punto de la última ronda y las del medio,
// que ya no hacen falta, quedan contadas como omitidas.
func TestUnRedisTrabadoNoFrenaLasRondas(t *testing.T) {
	puntos := &puntosAnotados{Cache: memoria.NuevaCache(), trabado: make(chan struct{}), entro: make(chan struct{})}
	ec := nuevoEntornoCluster(t, func(c *Config) { c.Puntos = puntos })
	ec.lanzarNodos(t, 3)
	ref := ec.referencia(t, 42)
	id := ec.entrenar(t, `{"nodos": 3, "replicas": 1, "semilla": 42}`)
	<-puntos.entro // Redis ya tiene la primera escritura y no la suelta

	// Las rondas siguen: llegan todas al almacén mientras Redis no responde.
	esperarHasta(t, "todas las rondas en el almacén con Redis trabado", func() bool {
		doc, err := ec.entorno.ent.Obtener(context.Background(), id)
		return err == nil && len(doc.Rondas) == ref.Iteraciones
	})
	if n := len(puntos.escrituras()); n != 0 {
		t.Fatalf("con Redis trabado se anotaron %d escrituras", n)
	}
	close(puntos.trabado)
	if det := ec.esperarFin(t, id); det["estado"] != "terminado" || det["modelo_id"] != "entrenado-"+id {
		t.Fatalf("detalle = %v", det)
	}
	iters := puntos.escrituras()
	if len(iters) != 2 || iters[0] != 1 || iters[1] != ref.Iteraciones {
		t.Fatalf("escrituras de las rondas %v; se esperaban la 1 (la que estaba trabada) y la %d (la última)", iters, ref.Iteraciones)
	}
	if escritos, fallidos, omitidos := ec.estadoPuntosDe(t); escritos != 2 || fallidos != 0 || int(omitidos) != ref.Iteraciones-2 {
		t.Fatalf("puntos_de_control = %v escritos, %v fallidos, %v omitidos; se esperaban 2, 0 y %d", escritos, fallidos, omitidos, ref.Iteraciones-2)
	}
}

// Con Redis caído el entrenamiento termina igual, activa su modelo y los
// fallos quedan contados.
func TestConRedisCaidoElEntrenamientoTerminaYLosFallosSeCuentan(t *testing.T) {
	puntos := &puntosAnotados{Cache: memoria.NuevaCache(), falla: true}
	ec := nuevoEntornoCluster(t, func(c *Config) { c.Puntos = puntos })
	ec.lanzarNodos(t, 2)
	inicio := time.Now()
	id := ec.entrenar(t, `{"nodos": 2, "replicas": 1, "semilla": 42}`)
	det := ec.esperarFin(t, id)
	if det["estado"] != "terminado" || det["modelo_id"] != "entrenado-"+id {
		t.Fatalf("detalle = %v", det)
	}
	if d := time.Since(inicio); d > 3*time.Second {
		t.Fatalf("con Redis caído el entrenamiento tardó %v", d)
	}
	escritos, fallidos, omitidos := ec.estadoPuntosDe(t)
	if escritos != 0 || fallidos == 0 || int(fallidos) != len(puntos.escrituras()) || int(fallidos+omitidos) != int(det["iteraciones"].(float64)) {
		t.Fatalf("puntos_de_control = %v escritos, %v fallidos, %v omitidos con %d intentos y %v rondas",
			escritos, fallidos, omitidos, len(puntos.escrituras()), det["iteraciones"])
	}
	if _, err := puntos.Punto(context.Background(), id); !errors.Is(err, almacen.ErrNoEncontrado) {
		t.Fatalf("punto con Redis caído: %v", err)
	}
}
