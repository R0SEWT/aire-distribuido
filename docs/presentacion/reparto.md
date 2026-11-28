# Sustentación del TB2: reparto y guion de la demo

Para la clase de la semana 15 (30/11–06/12; el día exacto está por confirmar). El enunciado pide que
«cada grupo [sea] responsable de presentar el funcionamiento del sistema y responder a las
preguntas del docente». La rúbrica lo repite en cada criterio: *cumple con la exposición de su
trabajo demostrando conocimiento total del tema*. Por eso cada uno expone lo que mejor conoce y
cada uno prepara las preguntas de sus láminas ([`preguntas.md`](preguntas.md)).

El deck es [`slides/index.qmd`](slides/index.qmd); en la vista del orador (tecla `S`) cada lámina
trae el tiempo y lo que hay que decir. El reparto es una propuesta: se ajusta en el ensayo.

Borrador hecho con IA (Claude) a partir del código, de `reports/` y del informe. Cada integrante
revisa su parte antes de exponer.

## Reparto de las láminas

| # | Lámina | Quién | Tiempo |
|--:|:--|:--|--:|
| 1 | Portada: 2 883 877 lecturas, ¿qué aire se respira junto a una mina? | Rody | 0:45 |
| 2 | La pregunta de impacto social | Rody | 0:30 |
| 3 | Separador: el dato | Dayana | 0:05 |
| 4 | Lo que el CSV de OEFA esconde (hora, ceros, unidades) | Dayana | 0:30 |
| 5 | El embudo de la limpieza | Dayana | 0:30 |
| 6 | Separador: el sistema | Rody | 0:05 |
| 7 | Las cuatro partes en Docker Compose, con los puertos | Rody | 0:40 |
| 8 | El camino de un pedido: predecir no pasa por el clúster; entrenar, sí | Rody | 0:40 |
| 9 | Separador: la concurrencia | Julio | 0:05 |
| 10 | Parte A: el cargador (canales, semáforo, `WaitGroup`) y su speedup | Julio | 0:45 |
| 11 | Parte B: el worker pool del K-Means (animación) | Rody | 0:50 |
| 12 | Bit a bit para todo W, y el speedup | Rody | 0:25 |
| 13 | Separador: la distribución | Julio | 0:05 |
| 14 | Los sockets: `net.Listen`, `Accept`, `DialContext`, la estrella | Julio | 0:50 |
| 15 | Una ronda: una goroutine por nodo, `WaitGroup`, fusión en orden de nodo | Julio | 0:50 |
| 16 | Separador: el servicio | Julio | 0:05 |
| 17 | La API en Go: el handler de una predicción | Julio | 0:50 |
| 18 | Menos de 100 ms con 64 clientes | Julio | 0:30 |
| 19 | MongoDB y Redis | Dayana | 0:40 |
| 20 | Separador: la interfaz | Dayana | 0:05 |
| 21 | La SPA (capturas reales: el panel en alerta y la simulación que cruza el ECA) | Dayana | 0:45 |
| 22 | Seguridad: la UI esconde, la API decide | Dayana | 0:40 |
| 23 | Separador: lo que responde | Julio | 0:05 |
| 24 | Evaluación experimental | Julio | 0:45 |
| 25 | Lo que responde: horas sobre el ECA por actividad | Dayana | 0:45 |
| 26 | ¿Con cuánta anticipación? | Dayana | 0:40 |
| 27 | Lo que este sistema no puede decir | Rody | 0:35 |
| 28 | Demo en vivo (Rody conduce; cada uno narra su parte) | los tres | 4:00 |
| 29 | Cierre y preguntas | Rody | 0:30 |

Exposición: **14:30** (Rody 5:00, Dayana 4:40, Julio 4:50) más **4:00** de demo. Después vienen
las láminas de respaldo, que solo se abren si el docente pregunta (tecla `M` para el menú):
Docker Compose, plazos del clúster y caída de un nodo, patrones de Go, reglas de limpieza, k = 3,
speedup con Karp-Flatt, cuándo conviene repartir, rutas de la API, las pantallas de admin
(Clúster y Entrenamiento) y la de impacto.

**Por qué este reparto.** Julio cuenta todo lo que el docente observó en la PC4 (sockets, puertos,
el código de la API): tiene que poder abrir el código y señalar la línea. Además midió el cargador
en Windows (#42). Dayana cuenta lo que ve el usuario (dato, interfaz, seguridad e impacto) y
rehízo la PC3 en macOS con los mismos SHA-256 (#45): si el docente pregunta por la
reproducibilidad, responde ella. Rody abre, cuenta la arquitectura y el K-Means, cierra y conduce
la demo, porque tiene la sesión de admin.

### Si dan menos tiempo

Versión de unos 11 minutos más 3:20 de demo, en este orden:

1. Saltar los seis separadores (−0:30).
2. Saltar el embudo (lámina 5) y decir solo «de 2,9 millones de filas a 65 873 horas» (−0:30).
3. Saltar «bit a bit para todo W» (12) y decirlo en la del worker pool (−0:25).
4. Saltar la latencia (18) y decir la cifra en la de la API: «p99 de 3,1 ms con 64 clientes» (−0:30).
5. Evaluación en 0:20, anticipación en 0:20 y límites en 0:20 (−0:60).
6. En la demo, saltar Impacto y Swagger (−0:40).

Lo que **no** se salta nunca: el camino de un pedido (8), los sockets (14), la ronda (15) y el
código de la API (17). Son exactamente lo que faltó en la PC4.

### Transiciones

Cada uno cierra su bloque con una frase que pasa al siguiente, para no perder tiempo:

- Rody (8) → Julio: «Julio, ¿cómo se reparte eso en una máquina?»
- Julio (10) → Rody: «Y el K-Means usa el mismo patrón; Rody.»
- Rody (12) → Julio: «Eso en una máquina. Entre máquinas, Julio.»
- Julio (18) → Dayana: «Todo eso se guarda en dos bases; Dayana.»
- Dayana (22) → Julio: «¿Y cómo sabemos que funciona?»
- Julio (24) → Dayana: «Y lo que dice sobre el aire.»
- Dayana (26) → Rody: «Rody, lo que no puede decir.»

## Guion de la demo

Rody maneja la laptop; cada uno narra su paso. Cuatro minutos. Todo sobre
<https://aire.rosewt.dev>, con dos ventanas del navegador lado a lado: a la izquierda el
**Panel**, a la derecha la pantalla que toque. Así se ve el tiempo real: lo que pasa a la derecha
cambia el panel de la izquierda sin recargar.

| # | Quién narra | Qué se hace | Qué se dice (idea, no texto) |
|--:|:--|:--|:--|
| 1 | Dayana · 0:30 | Panel: pasar el cursor por dos o tres estaciones del mapa; abrir una en la tabla | El último estado recibido de cada estación, su banda INCA y si está en alerta. «Último estado», no «ahora»: son datos de 2018 a 2022 |
| 2 | Dayana · 0:45 | Simulación → «Un día de una estación» → **CA-ESP-01, 21/12/2021** («cruza el ECA») → Simular, ritmo Rápida | La UI manda las lecturas reales una por una a `POST /api/predicciones`. La ventana de 24 h se llena; en la lectura 18 aparece la media; a las 07:00 la media de PM10 supera el ECA y se enciende la alerta, y el panel de la izquierda cambia solo: es el evento `estacion.regimen` por WebSocket |
| 3 | Julio · 0:30 | Clúster: la tabla de nodos, la latencia por ruta con la línea de 100 ms y los eventos | Cada nodo con su **IP:puerto**: es la dirección remota de su conexión TCP, como la ve el coordinador. Las altas y bajas de los nodos son los eventos |
| 4 | Rody · 0:50 | Entrenamiento: K = todos los nodos libres, réplicas 1, semilla 42 → Lanzar entrenamiento | La API toma K nodos del pool, les manda `CARGAR` y una ronda por iteración. El entrenamiento real dura unos 50 ms; la UI reproduce sus 44 rondas más despacio, con la inercia y lo que tardó cada nodo. Al final, la diferencia con el K-Means secuencial (con 2 nodos fue 7,7 × 10⁻¹⁵) y «el modelo nuevo quedó activo» porque está dentro de 10⁻⁹. Si se piden más nodos que los libres, la API responde 422 y la UI lo dice |
| 5 | Julio · 0:30 | Terminal: `docker compose ps` y `docker compose logs --tail 15 nodo` | Los servicios con sus puertos. En el log de un nodo: `conectado desde <IP>:<puerto> a <IP>:7070`, el `CARGAR` y la primera ronda: el `Dial` del nodo, en vivo |
| 6 | Dayana · 0:20 | Impacto: bajar hasta «Cada estación vigila una actividad» | La unidad honesta es la actividad: junto a la refinería y las ladrilleras de Lurigancho, la mayoría de las horas superó el ECA |
| 7 | Julio · 0:20 | `/docs` (Swagger): `POST /api/auth/invitado` → copiar el token → Authorize → `GET /api/metricas` | 403: el token es de rol usuario y la ruta es de admin. La UI esconde, la API decide |

Si en el paso 4 un nodo se cae o no responde, **es el comportamiento diseñado** y se cuenta así:
«la ronda se abortó con un error que nombra al nodo; no hay modelo, porque uno con menos nodos
sería otro modelo; los nodos vuelven a llamar solos». Se relanza con los que queden.

### Preparación

**El día anterior** (Rody):

- [ ] aire.rosewt.dev en línea: «API en línea» y «Tiempo real: conectado» arriba; en Clúster,
  los nodos en el pool (el despliegue de `despliegue/azure/` levanta dos, `--scale nodo=2`; con
  tres o cuatro la ronda se ve mejor); un modelo activo.
- [ ] En el `.env` del servidor, `JWT_DURACION=4h` y reiniciar la API: el token por omisión dura
  una hora y no se renueva.
- [ ] **El estado no retrocede.** Si una estación ya tiene un estado posterior al día que se
  simula, el panel no se mueve (la pantalla dice «Este tramo ya está cargado»). Comprobar en el
  Panel que CA-ESP-01 no tiene estado posterior al 21/12/2021; si lo tiene, elegir otro día que
  «cruza el ECA» de una estación sin estado y anotarlo aquí: ________. En el plan B el sistema
  arranca limpio y no hace falta.
- [ ] Opcional, si sobra un minuto: «Cargar el escenario» (admin, 48 h reales de 14 estaciones,
  667 lecturas) llena el panel del invitado en la otra ventana; a ritmo lento tarda demasiado
  para los cuatro minutos.
- [ ] Probar un entrenamiento completo de punta a punta.
- [ ] Plan B en la laptop (abajo): imágenes construidas con red, gold en `data/gold/`, `.env`
  completo. `docker compose build` necesita internet (módulos de Go, pnpm, imágenes base): no se
  construye en el aula.
- [ ] El deck sin red: `bash docs/presentacion/slides/render.sh --pdf` y copiar
  `slides/_site/index.html` y `slides/_site/deck.pdf` a la laptop y a un USB. Dayana o Julio
  lleva otra copia.
- [ ] El video del TB2 descargado.

**Media hora antes** (los tres):

- [ ] Rody: sesión de admin abierta en aire.rosewt.dev; las dos ventanas lado a lado; el zoom del
  navegador al 125 % para el proyector (o el botón «Modo proyector» de la app).
- [ ] Julio: una terminal abierta en el servidor (o en la laptop con el plan B). En el servidor,
  el Compose lleva el override de Azure; para no tipearlo en vivo:
  `alias dc='sudo docker compose -f docker-compose.yml -f despliegue/azure/compose.azure.yml'`, y
  en la demo `dc ps` y `dc logs --tail 15 nodo`. `/docs` abierto en otra pestaña.
- [ ] Probar el proyector con el deck en pantalla completa (`F`) y la vista del orador (`S`).
- [ ] Datos móviles listos para compartir internet si la red del aula falla.

### Plan B

**Nivel 1 · falla la red del aula.** Primero, internet desde un celular. Si tampoco hay, el mismo
sistema en la laptop, levantado **antes** de la clase:

```bash
cp .env.example .env   # JWT_SECRETO, API_USUARIOS, API_INVITADO=demo y JWT_DURACION=4h
```

```bash
docker compose up -d --scale nodo=3
```

La UI queda en <http://localhost:8081>, con la misma demo. El seed carga el catálogo y la API
arranca con el modelo de `reports/kmeans.json`; la simulación llena el estado de las estaciones.

**Nivel 2 · falla Docker.** La API y los nodos son binarios de Go y corren sin Docker, sin red y
sin MongoDB ni Redis (usan los dobles en memoria). Es la mejor forma de *mostrar los sockets*:

```bash
export JWT_SECRETO=$(openssl rand -hex 32)
```

```bash
go run ./cmd/api -demo
```

En otra terminal:

```bash
go run ./cmd/nodo -coordinador localhost:7070 -nombre nodo-1
```

Se ve en el log del nodo `conectado desde 127.0.0.1:<puerto> a 127.0.0.1:7070`, y en el de la API
`pool: alta nodo-1 (127.0.0.1:<puerto>)`. Con `-demo` los usuarios son `admin`/`admin` y
`demo`/`demo` (solo para la laptop). Sin el gold en `data/gold/` la API predice y recibe nodos,
pero no entrena. Comprobado el 27/11 en la laptop de Rody.

Y la prueba que corre un coordinador y un nodo sobre TCP real, en dos segundos:

```bash
go test -race -run SobreTCP -v ./protocolo/
```

**Nivel 3 · falla la laptop.** El deck en PDF desde el USB en otra máquina: tiene las capturas
reales de la app (la lámina de la SPA y dos respaldos: las pantallas de admin, Clúster y
Entrenamiento, y la de Impacto), y el video del TB2 muestra el sistema de punta a punta. Las
capturas completas de cada pantalla, con su lista de punta a punta, están en
`docs/informe/capturas/tb2/`.

## Lo que no se dice

Las frases que el docente ya corrigió, o que el dato no sostiene. Están explicadas en
[`preguntas.md`](preguntas.md#lo-que-no-se-dice).

- ~~«Los chunks pasan a ser nodos, el WaitGroup pasa a ser una barrera por la red y el canal pasa a
  ser una conexión TCP».~~ El docente la marcó como incorrecta en la PC4. Se dice: «entre procesos,
  sockets TCP; dentro de cada proceso, goroutines, canales y `WaitGroup`».
- ~~«La predicción la calcula el clúster».~~ El clúster entrena; la API predice con el modelo en
  memoria.
- ~~«Predecimos episodios».~~ La anticipación es descriptiva, sin partición temporal.
- ~~«34 estaciones».~~ El portal lo anuncia, pero en el CSV hay 33.
- ~~«Una hora sobre 50 µg/m³ incumple el ECA».~~ El ECA es una media de 24 horas.
- ~~«No usamos Mutex».~~ El bucle del K-Means no lo necesita; la API sí los usa (ventana de 24 h,
  pool de nodos, hub).
