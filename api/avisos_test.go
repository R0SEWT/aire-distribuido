package api

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/R0SEWT/aire-distribuido/almacen"
	"github.com/R0SEWT/aire-distribuido/almacen/memoria"
	"github.com/R0SEWT/aire-distribuido/kmeans"
)

// avisosDePrueba son unos avisos corriendo que guardan lo que difunden.
type avisosDePrueba struct {
	*avisosRegimen
	mu       sync.Mutex
	salieron []estadoEstacion
	cuando   []time.Time
	rechazar int    // cuántos de los próximos avisos no se aceptan
	parar    func() // cancela y espera a que correr vuelva
}

// recibir es el difundir de la prueba: guarda el aviso y lo acepta, salvo que
// queden rechazos por hacer.
func (p *avisosDePrueba) recibir(e estadoEstacion) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.rechazar > 0 {
		p.rechazar--
		return false
	}
	p.salieron = append(p.salieron, e)
	p.cuando = append(p.cuando, time.Now())
	return true
}

func levantarAvisos(t *testing.T, cada time.Duration) *avisosDePrueba {
	t.Helper()
	p := &avisosDePrueba{}
	p.avisosRegimen = nuevosAvisosRegimen(cada, p.recibir)
	ctx, cancel := context.WithCancel(context.Background())
	listo := make(chan struct{})
	go func() { p.correr(ctx); close(listo) }()
	p.parar = sync.OnceFunc(func() { cancel(); <-listo })
	t.Cleanup(p.parar)
	return p
}

func (p *avisosDePrueba) vistos() []estadoEstacion {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]estadoEstacion{}, p.salieron...)
}

// horaDePrueba es la hora h de un día cualquiera, en Lima.
func horaDePrueba(h int) time.Time {
	return time.Date(2021, 12, 13, 0, 0, 0, 0, kmeans.ZonaPeru).Add(time.Duration(h) * time.Hour)
}

func regimenDe(h, regimen int) almacen.RegimenEstacion {
	return almacen.RegimenEstacion{FechaHora: horaDePrueba(h), Regimen: regimen, TS: horaDePrueba(h)}
}

// relojDePrueba es un reloj que solo avanza cuando la prueba lo dice.
type relojDePrueba struct{ t time.Time }

func (r *relojDePrueba) ahora() time.Time        { return r.t }
func (r *relojDePrueba) avanzar(d time.Duration) { r.t = r.t.Add(d) }

// La regla del agrupado, pasada por pasada y con un reloj que la prueba
// controla (sin goroutine ni esperas): el primer cambio de una estación sale
// enseguida; los que llegan antes de que pase el intervalo se agrupan y, al
// cumplirse, sale uno solo con el último estado; en calma, el siguiente vuelve
// a salir enseguida.
func TestAvisosLaReglaDelAgrupadoPasoAPaso(t *testing.T) {
	const cada = 250 * time.Millisecond
	p := &avisosDePrueba{}
	p.avisosRegimen = nuevosAvisosRegimen(cada, p.recibir)
	reloj := &relojDePrueba{t: horaDePrueba(0)}
	p.ahora = reloj.ahora
	ultimo := map[string]time.Time{}
	horas := func() (hs []int) {
		for _, e := range p.vistos() {
			hs = append(hs, int(e.FechaHora.Sub(horaDePrueba(0))/time.Hour))
		}
		return hs
	}
	comprobar := func(que string, proximo, quiere time.Time, salidos ...int) {
		t.Helper()
		if !proximo.Equal(quiere) || !slices.Equal(horas(), salidos) {
			t.Fatalf("%s: hay que volver a mirar en %v (se esperaba %v) y salieron las horas %v (se esperaban %v)",
				que, proximo, quiere, horas(), salidos)
		}
	}
	nunca := time.Time{}

	// Sin nada pendiente no hay nada que hacer ni cuándo volver.
	comprobar("vacío", p.pasada(ultimo, false), nunca)
	// El primero sale enseguida.
	p.anotar(map[string]almacen.RegimenEstacion{"CA-A": regimenDe(1, 0)})
	comprobar("el primero", p.pasada(ultimo, false), nunca, 1)
	salio := reloj.t
	// Una ráfaga dentro del intervalo: nada sale, y hay que volver al cumplirse.
	for h := 2; h <= 40; h++ {
		reloj.avanzar(time.Millisecond)
		p.anotar(map[string]almacen.RegimenEstacion{"CA-A": regimenDe(h, h%3)})
		comprobar("la ráfaga", p.pasada(ultimo, false), salio.Add(cada), 1)
	}
	// Un nanosegundo antes de cumplirse, todavía no.
	reloj.t = salio.Add(cada - time.Nanosecond)
	comprobar("justo antes", p.pasada(ultimo, false), salio.Add(cada), 1)
	if p.alDia() {
		t.Fatal("con un estado pendiente los avisos no están al día")
	}
	// Al cumplirse sale uno solo, el último; los 38 intermedios no salen.
	reloj.t = salio.Add(cada)
	comprobar("al cumplirse", p.pasada(ultimo, false), nunca, 1, 40)
	if r, e, a := p.recibidos.Load(), p.emitidos.Load(), p.agrupados.Load(); r != 40 || e != 2 || a != 38 || !p.alDia() {
		t.Fatalf("recibidos %d, emitidos %d, agrupados %d; se esperaban 40, 2 y 38, y al día", r, e, a)
	}
	// El intervalo corre desde que el aviso terminó de salir: si difundir
	// tarda, el siguiente espera un intervalo entero desde ahí, no desde que
	// se decidió mandarlo.
	p.difundir = func(e estadoEstacion) bool { reloj.avanzar(100 * time.Millisecond); return p.recibir(e) }
	reloj.avanzar(time.Hour) // en calma: el siguiente sale enseguida
	decidido := reloj.t
	p.anotar(map[string]almacen.RegimenEstacion{"CA-A": regimenDe(41, 1)})
	comprobar("en calma", p.pasada(ultimo, false), nunca, 1, 40, 41)
	p.anotar(map[string]almacen.RegimenEstacion{"CA-A": regimenDe(42, 1)})
	reloj.t = decidido.Add(cada) // un intervalo desde la decisión, pero no desde la salida
	comprobar("un intervalo desde la decisión", p.pasada(ultimo, false), decidido.Add(100*time.Millisecond+cada), 1, 40, 41)
	reloj.t = decidido.Add(100*time.Millisecond + cada)
	comprobar("un intervalo desde la salida", p.pasada(ultimo, false), nunca, 1, 40, 41, 42)
}

// Con la goroutine y el reloj de verdad, lo que el agrupador garantiza pase lo
// que pase con el planificador: el primer cambio sale, entre dos avisos de la
// misma estación pasa al menos el intervalo, los estados nunca retroceden,
// todo lo recibido salió o lo pisó uno posterior, y el último aviso es el
// último estado. No se supone cuánto tarda la prueba en mandar la ráfaga: si
// la máquina está cargada salen más avisos, y todos cumplen lo mismo.
func TestAvisosConLaGoroutineRespetanElIntervalo(t *testing.T) {
	const cada = 100 * time.Millisecond
	p := levantarAvisos(t, cada)
	inicio := time.Now()
	p.anotar(map[string]almacen.RegimenEstacion{"CA-A": regimenDe(1, 0)})
	esperarHasta(t, "el primer aviso", func() bool { return len(p.vistos()) >= 1 })
	for h := 2; h <= 40; h++ {
		p.anotar(map[string]almacen.RegimenEstacion{"CA-A": regimenDe(h, h%3)})
	}
	esperarHasta(t, "el aviso agrupado", p.alDia)
	duracion := time.Since(inicio)
	v := p.vistos()
	p.mu.Lock()
	cuando := slices.Clone(p.cuando)
	p.mu.Unlock()
	if len(v) < 2 || !v[0].FechaHora.Equal(horaDePrueba(1)) || !v[len(v)-1].FechaHora.Equal(horaDePrueba(40)) || v[len(v)-1].Regimen != 40%3 {
		t.Fatalf("avisos = %+v; el primero tiene que ser la hora 1 y el último, la 40", v)
	}
	for i := 1; i < len(v); i++ {
		if entre := cuando[i].Sub(cuando[i-1]); entre < cada {
			t.Fatalf("entre los avisos %d y %d de la misma estación pasaron %v; el intervalo es %v", i-1, i, entre, cada)
		}
		if !v[i].FechaHora.After(v[i-1].FechaHora) {
			t.Fatalf("el aviso %d (%v) no es posterior al %d (%v)", i, v[i].FechaHora, i-1, v[i-1].FechaHora)
		}
	}
	if tope := int(duracion/cada) + 1; len(v) > tope {
		t.Fatalf("en %v salieron %d avisos; con un intervalo de %v el tope es %d", duracion, len(v), cada, tope)
	}
	if r, e, a := p.recibidos.Load(), p.emitidos.Load(), p.agrupados.Load(); r != 40 || e != uint64(len(v)) || e+a != 40 {
		t.Fatalf("recibidos %d, emitidos %d, agrupados %d con %d avisos", r, e, a, len(v))
	}
}

// Si el hub no acepta un aviso (su canal de difusión está lleno), el estado no
// se da por emitido: queda pendiente y se reintenta un intervalo después. Si
// entre tanto la estación cambia otra vez, sale el estado nuevo, no el viejo.
func TestAvisosReintentanLoQueElHubNoAcepto(t *testing.T) {
	const cada = 250 * time.Millisecond
	p := &avisosDePrueba{}
	p.avisosRegimen = nuevosAvisosRegimen(cada, p.recibir)
	reloj := &relojDePrueba{t: horaDePrueba(0)}
	p.ahora = reloj.ahora
	ultimo := map[string]time.Time{}

	// Dos estaciones; el hub rechaza el primer intento (CA-A, por orden).
	p.rechazar = 1
	p.anotar(map[string]almacen.RegimenEstacion{"CA-A": regimenDe(1, 0), "CA-B": regimenDe(1, 1)})
	proximo := p.pasada(ultimo, false)
	if v := p.vistos(); len(v) != 1 || v[0].Estacion != "CA-B" || !proximo.Equal(reloj.t.Add(cada)) || p.alDia() {
		t.Fatalf("tras el rechazo salieron %+v y hay que volver en %v; CA-A tenía que quedar pendiente para un intervalo después", v, proximo)
	}
	if e, r := p.emitidos.Load(), p.rechazados.Load(); e != 1 || r != 1 {
		t.Fatalf("emitidos %d, rechazados %d; el rechazado no cuenta como emitido", e, r)
	}
	// Antes de tiempo no se reintenta; al cumplirse, sí, y con el mismo estado.
	reloj.avanzar(cada - time.Millisecond)
	if p.pasada(ultimo, false); len(p.vistos()) != 1 {
		t.Fatal("reintentó antes de que pasara el intervalo")
	}
	reloj.avanzar(time.Millisecond)
	if proximo = p.pasada(ultimo, false); len(p.vistos()) != 2 || p.vistos()[1].Estacion != "CA-A" || !p.vistos()[1].FechaHora.Equal(horaDePrueba(1)) || !proximo.IsZero() || !p.alDia() {
		t.Fatalf("el reintento no sacó el estado rechazado: %+v", p.vistos())
	}

	// Rechazado otra vez, y mientras tanto llega un estado más nuevo: el que
	// sale en el reintento es el nuevo.
	reloj.avanzar(time.Hour)
	p.rechazar = 1
	p.anotar(map[string]almacen.RegimenEstacion{"CA-A": regimenDe(2, 0)})
	p.pasada(ultimo, false)
	p.anotar(map[string]almacen.RegimenEstacion{"CA-A": regimenDe(3, 2)})
	reloj.avanzar(cada)
	p.pasada(ultimo, false)
	if v := p.vistos(); len(v) != 3 || !v[2].FechaHora.Equal(horaDePrueba(3)) || !p.alDia() {
		t.Fatalf("tras el rechazo y un estado más nuevo salieron %+v; se esperaba la hora 3", v)
	}
	// Las cuentas cierran: todo lo recibido salió o lo pisó uno posterior.
	if r, e, a := p.recibidos.Load(), p.emitidos.Load(), p.agrupados.Load(); r != 4 || e != 3 || a != 1 {
		t.Fatalf("recibidos %d, emitidos %d, agrupados %d; se esperaban 4, 3 y 1", r, e, a)
	}

	// Un estado más nuevo que llega justo mientras el hub rechaza el anterior
	// no se pisa con el rechazado al devolverlo.
	reloj.avanzar(time.Hour)
	p.anotar(map[string]almacen.RegimenEstacion{"CA-A": regimenDe(4, 0)})
	p.difundir = func(estadoEstacion) bool {
		p.anotar(map[string]almacen.RegimenEstacion{"CA-A": regimenDe(5, 1)})
		return false
	}
	p.pasada(ultimo, false)
	p.difundir = p.recibir
	reloj.avanzar(cada)
	p.pasada(ultimo, false)
	if v := p.vistos(); len(v) != 4 || !v[3].FechaHora.Equal(horaDePrueba(5)) || !p.alDia() {
		t.Fatalf("salieron %+v; el estado de la hora 5 llegó durante el rechazo y es el que tenía que salir", v)
	}
}

// Lo mismo con la goroutine: el reintento lo arma ella sola, sin que nadie
// vuelva a anotar.
func TestAvisosElReintentoNoNecesitaOtroCambio(t *testing.T) {
	p := &avisosDePrueba{rechazar: 3}
	p.avisosRegimen = nuevosAvisosRegimen(20*time.Millisecond, p.recibir)
	ctx, cancel := context.WithCancel(context.Background())
	listo := make(chan struct{})
	go func() { p.correr(ctx); close(listo) }()
	t.Cleanup(func() { cancel(); <-listo })
	p.anotar(map[string]almacen.RegimenEstacion{"CA-A": regimenDe(7, 2)})
	esperarHasta(t, "que el aviso rechazado tres veces salga al cuarto intento", p.alDia)
	if v := p.vistos(); len(v) != 1 || !v[0].FechaHora.Equal(horaDePrueba(7)) || p.rechazados.Load() != 3 {
		t.Fatalf("avisos = %+v, rechazados = %d", v, p.rechazados.Load())
	}
}

// difundirRegimen le dice a los avisos si el hub aceptó: con el canal de
// difusión lleno, no (se reintenta); con el hub apagado, sí (ya no hay a quién
// mandarle nada).
func TestDifundirRegimenDiceSiElHubAcepto(t *testing.T) {
	e := nuevoEntorno(t)
	e.parar() // nada corre: se puede cambiar el hub sin carreras
	estado := nuevoEstadoEstacion("CA-A", regimenDe(1, 0))
	if !e.srv.difundirRegimen(estado) {
		t.Fatal("con el hub apagado no hay nada que reintentar")
	}
	op := opcionesHubDePrueba()
	op.buferDifusion = 1
	e.srv.hub = nuevoHub(op, logMudo) // un hub que no corre: su canal se llena con un evento
	if !e.srv.difundirRegimen(estado) {
		t.Fatal("el primer evento cabe en el canal de difusión")
	}
	if e.srv.difundirRegimen(estado) {
		t.Fatal("con el canal de difusión lleno, el evento no quedó aceptado")
	}
	if d := e.srv.hub.descartados.Load(); d != 1 {
		t.Fatalf("descartados = %d", d)
	}
}

// El intervalo es de cada estación: una ráfaga en una no retiene a las demás.
// Lo que sale junto, sale en orden de estación.
func TestAvisosCadaEstacionTieneSuIntervalo(t *testing.T) {
	p := levantarAvisos(t, time.Hour)
	p.anotar(map[string]almacen.RegimenEstacion{"CA-B": regimenDe(1, 1), "CA-A": regimenDe(1, 0), "CA-C": regimenDe(1, 2)})
	esperarHasta(t, "los tres primeros avisos", func() bool { return len(p.vistos()) == 3 })
	if v := p.vistos(); v[0].Estacion != "CA-A" || v[1].Estacion != "CA-B" || v[2].Estacion != "CA-C" {
		t.Fatalf("orden de los avisos: %s, %s, %s", v[0].Estacion, v[1].Estacion, v[2].Estacion)
	}
	// CA-A queda retenida (su intervalo es de una hora); CA-D, que no había
	// avisado, sale igual.
	p.anotar(map[string]almacen.RegimenEstacion{"CA-A": regimenDe(2, 1)})
	p.anotar(map[string]almacen.RegimenEstacion{"CA-D": regimenDe(1, 1)})
	esperarHasta(t, "el aviso de CA-D", func() bool { return len(p.vistos()) == 4 })
	if v := p.vistos(); v[3].Estacion != "CA-D" {
		t.Fatalf("el cuarto aviso es de %s", v[3].Estacion)
	}
}

// Al parar sale lo pendiente, sin esperar su intervalo; y anotar después no
// bloquea ni acumula más de un estado por estación.
func TestAvisosAlPararSaleLoPendienteYAnotarNoBloquea(t *testing.T) {
	p := levantarAvisos(t, time.Hour)
	p.anotar(map[string]almacen.RegimenEstacion{"CA-A": regimenDe(1, 0)})
	esperarHasta(t, "el primer aviso", func() bool { return len(p.vistos()) == 1 })
	p.anotar(map[string]almacen.RegimenEstacion{"CA-A": regimenDe(2, 1)})
	p.anotar(map[string]almacen.RegimenEstacion{"CA-A": regimenDe(3, 2)})
	p.parar()
	if v := p.vistos(); len(v) != 2 || !v[1].FechaHora.Equal(horaDePrueba(3)) {
		t.Fatalf("al parar tenía que salir el estado pendiente (hora 3): %+v", v)
	}
	hecho := make(chan struct{})
	go func() {
		for h := 4; h < 5000; h++ {
			p.anotar(map[string]almacen.RegimenEstacion{"CA-A": regimenDe(h, 0), "CA-B": regimenDe(h, 0)})
		}
		close(hecho)
	}()
	select {
	case <-hecho:
	case <-time.After(5 * time.Second):
		t.Fatal("anotar bloqueó con los avisos parados")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.avisosRegimen.mu.Lock()
	defer p.avisosRegimen.mu.Unlock()
	if len(p.pendientes) != 2 || len(p.salieron) != 2 {
		t.Fatalf("pendientes = %d (se esperaba una entrada por estación), avisos = %d", len(p.pendientes), len(p.salieron))
	}
}

// conectarWS anota en el hub del servidor un extremo de net.Pipe y devuelve el
// otro como cliente. net.Pipe no tiene búfer: lo que el cliente no lee se
// queda en su búfer del hub, que es lo que hace falta para probar a un cliente
// lento.
func (e *entorno) conectarWS(t *testing.T, rol string) *clienteWSPrueba {
	t.Helper()
	servidor, cliente := net.Pipe()
	t.Cleanup(func() { cliente.Close() })
	if !e.srv.hub.Atender(servidor, bufio.NewReader(servidor), rol, time.Time{}, nil) {
		t.Fatal("el hub no admitió al cliente")
	}
	return &clienteWSPrueba{conn: cliente, r: bufio.NewReader(cliente)}
}

// leerEvento lee la siguiente trama de texto y la separa en el nombre del
// evento y sus datos, tal como llegaron.
func (c *clienteWSPrueba) leerEvento(t *testing.T) (evento string, datos json.RawMessage) {
	t.Helper()
	var sobre struct {
		Evento string          `json:"evento"`
		Datos  json.RawMessage `json:"datos"`
	}
	texto := c.leerTexto(t)
	if err := json.Unmarshal([]byte(texto), &sobre); err != nil || sobre.Evento == "" {
		t.Fatalf("la trama no es un EventoWS (%v): %s", err, texto)
	}
	return sobre.Evento, sobre.Datos
}

// esperarAvisos espera a que el escritor haya escrito n predicciones y a que
// todo cambio de estado que eso produjo ya haya salido hacia el hub.
func (e *entorno) esperarAvisos(t *testing.T, n uint64) {
	t.Helper()
	e.esperarEscritas(t, n)
	esperarHasta(t, "que salgan los avisos de régimen pendientes", e.srv.avisos.alDia)
}

// finDePrueba difunde un evento que no es de la API. Lo que un cliente lee
// antes de él es todo lo que la API le mandó hasta ese momento.
const finDePrueba = "prueba.fin"

func (e *entorno) marcarFin(t *testing.T) {
	t.Helper()
	if err := e.srv.hub.Difundir(finDePrueba, nil); err != nil {
		t.Fatal(err)
	}
}

// estadoDelGET es la entrada de una estación en GET /api/regimenes, como JSON
// canónico (las claves ordenadas), o "" si la estación no está.
func (e *entorno) estadoDelGET(t *testing.T, tok, estacion string) string {
	t.Helper()
	for _, estado := range lista(t, e.pedir(t, "GET", "/api/regimenes", tok, ""), 200) {
		if estado["estacion"] == estacion {
			b, _ := json.Marshal(estado)
			return string(b)
		}
	}
	return ""
}

// canonico reordena las claves de un objeto JSON para compararlo con otro.
func canonico(t *testing.T, crudo json.RawMessage) string {
	t.Helper()
	var obj map[string]any
	if err := json.Unmarshal(crudo, &obj); err != nil {
		t.Fatalf("no es un objeto JSON: %s", crudo)
	}
	b, _ := json.Marshal(obj)
	return string(b)
}

// lecturaEn es el cuerpo de una predicción para una hora de horaDePrueba.
func lecturaEn(estacion string, h int, pm25, pm10 float64) string {
	b, _ := json.Marshal(map[string]any{"estacion": estacion, "fecha_hora": horaDePrueba(h).Format(time.RFC3339), "pm2_5": pm25, "pm10": pm10})
	return string(b)
}

// Una lectura que pasa a ser el estado de su estación sale por el WebSocket
// como estacion.regimen, para usuario y para admin, con el mismo objeto que
// GET /api/regimenes tiene para esa estación. Un usuario sigue sin recibir los
// eventos de entrenamiento.
func TestElEventoDeRegimenLlegaAUsuarioYAdminYEsElEstadoDelGET(t *testing.T) {
	e := nuevoEntorno(t)
	tok := e.token(t, "demo", "demo")
	usuario, admin := e.conectarWS(t, RolUsuario), e.conectarWS(t, RolAdmin)

	objeto(t, e.pedir(t, "POST", "/api/predicciones", tok, lecturaEn("CA-AYAS-01", 10, 70.2, 190.5)), 200)
	e.esperarAvisos(t, 1)
	delGET := e.estadoDelGET(t, tok, "CA-AYAS-01")
	if delGET == "" {
		t.Fatal("GET /api/regimenes no trae CA-AYAS-01")
	}
	for rol, c := range map[string]*clienteWSPrueba{RolUsuario: usuario, RolAdmin: admin} {
		evento, datos := c.leerEvento(t)
		if evento != "estacion.regimen" {
			t.Fatalf("%s recibió el evento %q", rol, evento)
		}
		var campos map[string]any
		json.Unmarshal(datos, &campos)
		for _, campo := range camposEstadoEstacion {
			if _, ok := campos[campo]; !ok {
				t.Fatalf("al evento le falta %q: %s", campo, datos)
			}
		}
		if len(campos) != len(camposEstadoEstacion) || campos["estacion"] != "CA-AYAS-01" || campos["fecha_hora"] != "2021-12-13T10:00:00-05:00" {
			t.Fatalf("datos del evento: %s", datos)
		}
		if got := canonico(t, datos); got != delGET {
			t.Fatalf("%s: el evento y el GET no dicen lo mismo:\n evento %s\n GET    %s", rol, got, delGET)
		}
	}

	// Un evento de entrenamiento y el fin de la prueba: el usuario solo ve lo segundo.
	if err := e.srv.hub.Difundir("entrenamiento.ronda", map[string]any{"id": "e-1", "iter": 1}, rolesEventosEntrenamiento...); err != nil {
		t.Fatal(err)
	}
	e.marcarFin(t)
	if evento, _ := admin.leerEvento(t); evento != "entrenamiento.ronda" {
		t.Fatalf("el admin tenía que recibir la ronda y recibió %q", evento)
	}
	if evento, _ := admin.leerEvento(t); evento != finDePrueba {
		t.Fatalf("el admin recibió %q de más", evento)
	}
	if evento, datos := usuario.leerEvento(t); evento != finDePrueba {
		t.Fatalf("un usuario no recibe eventos de entrenamiento; recibió %q %s", evento, datos)
	}
}

// Una lectura más antigua que la guardada se predice y se responde, pero no es
// el estado de la estación: no cambia GET /api/regimenes ni emite evento. Si
// lo emitiera, el panel mostraría un estado que el GET no confirma.
func TestUnaLecturaMasAntiguaNoEmiteEventoNiCambiaElGET(t *testing.T) {
	e := nuevoEntorno(t)
	tok := e.token(t, "demo", "demo")
	c := e.conectarWS(t, RolUsuario)

	objeto(t, e.pedir(t, "POST", "/api/predicciones", tok, lecturaEn("CA-MAC-01", 10, 80, 150)), 200)
	e.esperarAvisos(t, 1)
	antes := e.estadoDelGET(t, tok, "CA-MAC-01")
	if evento, datos := c.leerEvento(t); evento != "estacion.regimen" || canonico(t, datos) != antes {
		t.Fatalf("evento de la hora 10: %q %s", evento, datos)
	}
	// La hora 4, que llega después: una respuesta normal y nada más.
	objeto(t, e.pedir(t, "POST", "/api/predicciones", tok, lecturaEn("CA-MAC-01", 4, 5, 5)), 200, camposPrediccion...)
	e.esperarAvisos(t, 2)
	e.marcarFin(t)
	if evento, datos := c.leerEvento(t); evento != finDePrueba {
		t.Fatalf("la lectura más antigua emitió %q: %s", evento, datos)
	}
	if despues := e.estadoDelGET(t, tok, "CA-MAC-01"); despues != antes {
		t.Fatalf("la lectura más antigua cambió el GET:\n antes   %s\n después %s", antes, despues)
	}
	if r := e.srv.avisos.recibidos.Load(); r != 1 {
		t.Fatalf("a los avisos llegaron %d estados; solo el de la hora 10 quedó guardado", r)
	}
	// La misma hora 10, corregida, sí es el estado nuevo: evento y GET cambian juntos.
	objeto(t, e.pedir(t, "POST", "/api/predicciones", tok, lecturaEn("CA-MAC-01", 10, 5, 5)), 200)
	e.esperarAvisos(t, 3)
	corregido := e.estadoDelGET(t, tok, "CA-MAC-01")
	if evento, datos := c.leerEvento(t); evento != "estacion.regimen" || canonico(t, datos) != corregido || corregido == antes {
		t.Fatalf("evento de la hora 10 corregida: %q %s (GET %s)", evento, datos, corregido)
	}
}

// El caso entero: Redis guarda el estado pero su respuesta no llega. La API
// cuenta el lote como fallido y, sin la relectura, el GET mostraría un estado
// que el WebSocket nunca anunció hasta el siguiente cambio de la estación. Con
// ella, el evento sale en cuanto Redis vuelve a responder, y es el del GET.
func TestElEventoSaleAunqueLaRespuestaDeRedisSePierda(t *testing.T) {
	cache := &cacheSinRespuesta{Cache: memoria.NuevaCache()}
	e := nuevoEntorno(t, func(c *Config) { c.Cache = cache })
	// Antes de la primera predicción: la goroutine de Redis lo lee recién
	// después de recibir una por su cola.
	e.srv.escritor.cadaRelectura = 20 * time.Millisecond
	tok := e.token(t, "demo", "demo")
	c := e.conectarWS(t, RolUsuario)

	cache.perder.Store(1)
	objeto(t, e.pedir(t, "POST", "/api/predicciones", tok, lecturaEn("CA-MAC-01", 10, 80, 150)), 200, camposPrediccion...)
	esperarHasta(t, "la relectura y su aviso", func() bool {
		return e.srv.escritor.relecturas.Load() == 1 && e.srv.avisos.recibidos.Load() == 1 && e.srv.avisos.alDia()
	})
	delGET := e.estadoDelGET(t, tok, "CA-MAC-01")
	if delGET == "" {
		t.Fatal("el estado tenía que estar en el GET: Redis lo guardó")
	}
	if evento, datos := c.leerEvento(t); evento != "estacion.regimen" || canonico(t, datos) != delGET {
		t.Fatalf("el evento no es el estado del GET:\n evento %q %s\n GET    %s", evento, datos, delGET)
	}
	// El lote sigue contando como fallido en las métricas: la escritura no se confirmó.
	if est := e.srv.escritor.estado()["redis"]; est.Fallidas != 1 || est.Escritas != 0 {
		t.Fatalf("escritura en redis = %+v", est)
	}
}

// Una ráfaga sobre la misma estación (lo que hace cmd/carga) no llena el búfer
// de un cliente que no lee al ritmo: a lo sumo sale un evento por intervalo.
// Sin agrupar serían mil eventos contra un búfer de 256 y el hub lo daría de
// baja por lento. Cuando el cliente por fin lee, el último evento es el estado
// que GET /api/regimenes tiene.
func TestUnaRafagaNoDesconectaAlClienteLentoYSuUltimoEventoEsElDelGET(t *testing.T) {
	const (
		rafaga = 1000
		cada   = 100 * time.Millisecond
	)
	e := nuevoEntorno(t, func(c *Config) { c.CadaAvisoRegimen, c.TamCola = cada, 2*rafaga })
	tok := e.token(t, "demo", "demo")
	lento := e.conectarWS(t, RolUsuario) // no lee nada hasta que termina la ráfaga
	if b := e.srv.hub.op.buferCliente; rafaga <= b {
		t.Fatalf("la ráfaga (%d) tiene que superar el búfer de un cliente (%d)", rafaga, b)
	}

	inicio := time.Now()
	for h := 1; h <= rafaga; h++ {
		if w := e.pedir(t, "POST", "/api/predicciones", tok, lecturaEn("CA-MAC-01", h, float64(h%90), 150)); w.Code != 200 {
			t.Fatalf("predicción %d: %d %s", h, w.Code, w.Body)
		}
	}
	e.esperarAvisos(t, rafaga)
	duracion := time.Since(inicio)
	if l, c := e.srv.hub.lentos.Load(), e.srv.hub.conectados.Load(); l != 0 || c != 1 {
		t.Fatalf("lentos = %d, conectados = %d: la ráfaga desconectó al cliente", l, c)
	}
	if r := e.srv.avisos.recibidos.Load(); r == 0 || r > rafaga {
		t.Fatalf("a los avisos llegaron %d estados de %d predicciones", r, rafaga)
	}

	delGET := e.estadoDelGET(t, tok, "CA-MAC-01")
	e.marcarFin(t)
	var eventos []string
	for {
		evento, datos := lento.leerEvento(t)
		if evento == finDePrueba {
			break
		}
		if evento != "estacion.regimen" {
			t.Fatalf("evento inesperado %q", evento)
		}
		eventos = append(eventos, canonico(t, datos))
	}
	// A lo sumo uno por intervalo, más el primero y el que cierra.
	if tope := int(duracion/cada) + 2; len(eventos) == 0 || len(eventos) > tope {
		t.Fatalf("de %d predicciones en %v salieron %d eventos; con un intervalo de %v el tope es %d",
			rafaga, duracion, len(eventos), cada, tope)
	}
	if ultimo := eventos[len(eventos)-1]; ultimo != delGET || !strings.Contains(ultimo, fmt.Sprintf("%q", horaDePrueba(rafaga).Format(time.RFC3339))) {
		t.Fatalf("el último evento no es el estado del GET:\n evento %s\n GET    %s", ultimo, delGET)
	}
	t.Logf("%d predicciones en %v: %d eventos estacion.regimen", rafaga, duracion.Round(time.Millisecond), len(eventos))
}

// Apagado limpio con un cambio todavía retenido por su intervalo: el estado
// pendiente sale antes del cierre 1001, Escribir y Coordinar vuelven, y no
// queda ninguna goroutine de la API, del hub ni de los avisos.
func TestApagadoLimpioConUnAvisoPendiente(t *testing.T) {
	antes := runtime.NumGoroutine()
	e := nuevoEntorno(t, func(c *Config) { c.CadaAvisoRegimen = time.Hour })
	tok := e.token(t, "demo", "demo")
	c := e.conectarWS(t, RolUsuario)

	objeto(t, e.pedir(t, "POST", "/api/predicciones", tok, lecturaEn("CA-MAC-01", 1, 10, 20)), 200)
	e.esperarEscritas(t, 1)
	if evento, _ := c.leerEvento(t); evento != "estacion.regimen" {
		t.Fatalf("primer evento: %q", evento)
	}
	// El segundo cambio queda pendiente: el intervalo es de una hora.
	objeto(t, e.pedir(t, "POST", "/api/predicciones", tok, lecturaEn("CA-MAC-01", 2, 10, 20)), 200)
	e.esperarEscritas(t, 2)
	delGET := e.estadoDelGET(t, tok, "CA-MAC-01")
	if e.srv.avisos.alDia() {
		t.Fatal("el segundo estado tenía que estar pendiente")
	}

	apagado := make(chan struct{})
	go func() { e.parar(); close(apagado) }()
	if evento, datos := c.leerEvento(t); evento != "estacion.regimen" || canonico(t, datos) != delGET {
		t.Fatalf("al apagar tenía que salir el estado pendiente: %q %s", evento, datos)
	}
	if codigo, motivo := c.leerCierre(t); codigo != cierreSeVa || !strings.Contains(motivo, "apagando") {
		t.Fatalf("cierre = %d %q", codigo, motivo)
	}
	c.mandar(opCierre, []byte{0x03, 0xE9})
	if !c.seCorto() {
		t.Fatal("la conexión tenía que cortarse")
	}
	select {
	case <-apagado:
	case <-time.After(10 * time.Second):
		t.Fatal("Escribir y Coordinar no volvieron")
	}
	// Con todo apagado, un estado que llegue tarde no bloquea a nadie.
	e.srv.avisos.anotar(map[string]almacen.RegimenEstacion{"CA-MAC-01": regimenDe(3, 0)})
	esperarHasta(t, fmt.Sprintf("que terminen las goroutines (había %d, hay %d)", antes, runtime.NumGoroutine()),
		func() bool { return runtime.NumGoroutine() <= antes })
}
