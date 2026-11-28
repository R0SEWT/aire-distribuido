# Preguntas probables del docente en la sustentación del TB2

Las 29 preguntas que con más probabilidad hará el docente, con una respuesta corta y el archivo
y la línea que la respaldan, para abrir el código si pide verlo. Cada una dice quién la responde
según el [reparto](reparto.md), aunque los tres tienen que poder contestar cualquiera.

El docente corrige con el vocabulario de sus laboratorios: sockets, `Listen` y `Dial`, puertos,
canales, `WaitGroup`, `Mutex`, patrones y Docker. Por eso las respuestas usan esas palabras y
nombran la función de Go, no una abstracción nuestra. Las cifras son las del deck, que salen de
`reports/` y de `docs/informe/generado/cifras.yml`; si cambian, manda el deck.

Las referencias `archivo:línea` son de la rama `develop` del 27/11/2026 (commit `785c2d0`; el
código Go no cambió desde `6d91f4f`).
Respuestas verificadas contra el código con ayuda de IA (Claude); cada integrante revisa las
suyas antes de la sustentación.

**Índice.** [Observaciones de la PC4](#lo-que-el-docente-observó-en-la-pc4) ·
[Planteamiento y datos](#planteamiento-y-datos) ·
[Concurrencia en una máquina](#concurrencia-en-una-máquina) ·
[Distribución](#distribución-sockets-puertos-y-protocolo) · [La API](#la-api) ·
[MongoDB y Redis](#mongodb-y-redis) · [Docker](#docker) ·
[Frontend y seguridad](#frontend-y-seguridad) · [Evaluación e impacto](#evaluación-e-impacto) ·
[Proceso](#proceso) · [Lo que no se dice](#lo-que-no-se-dice)

## Lo que el docente observó en la PC4

Cuatro observaciones con base. La quinta, la del Kanban, pedía una tarjeta que sí estaba (la #76):
si sale, se muestra la tarjeta, sin discutir. Las cuatro tienen ahora una lámina y una respuesta
que no repite el error.

| Observación de la PC4 | Qué faltaba | Dónde se responde ahora |
|:--|:--|:--|
| No explica el código Go distribuido: listen, dial, puertos, patrones | Los listados mostraban `Ronda`, `Lloyd` y `Pool`, un nivel por encima de los sockets | Lámina 16 (el código de `net.Listen`, `Accept` y `DialContext`), lámina 17 (la ronda con su `WaitGroup`), respaldo de patrones; preguntas 10 a 16 |
| «Incorrecto: los chunks pasan a ser nodos, el WaitGroup pasa a ser una barrera a través de la red y el canal pasa a ser una conexión TCP» | Una analogía falsa al pie de la letra | Lámina 17: «entre procesos, sockets TCP; dentro de cada proceso, goroutines, canales y `WaitGroup`»; pregunta 13 |
| No explica el código Go de la API ni cómo pasa la petición al clúster | «El servidor en Go» era prosa; no se dijo que las predicciones no pasan por el clúster | Lámina 9 (el camino de un pedido) y lámina 19 (el handler); preguntas 17 a 19 |
| No incluye el Dockerfile ni el docker-compose; no explica el despliegue | Los archivos iban solo en el ZIP | Lámina 8 (servicios y puertos), respaldo de Docker Compose; preguntas 22 y 23 |

## Planteamiento y datos

### 1. ¿Cuál es la pregunta de impacto social y para quién es?

**Respuesta.** ¿En qué estaciones de OEFA y a qué horas el aire entra en el nivel alto de
PM2,5/PM10, y con cuánta anticipación eso avisa de un episodio que supera el ECA? Es para las personas con enfermedades respiratorias que viven junto
a una actividad que OEFA fiscaliza. El enunciado pone una casi igual entre sus ejemplos de impacto
social («¿cuándo se disparará la contaminación del aire para alertar a personas con enfermedades
respiratorias?»), elegida con una criba sobre el dato entre nueve candidatas.

*Dónde:* `docs/specs/2026-10-31-preguntas-candidatas.md` · *Quién:* Rody · *Lámina:* 3

### 2. Expliquen la arquitectura de la solución.

**Respuesta.** Las cuatro partes del enunciado, en Docker Compose. A, el cargador concurrente
(`cmd/cargar`): lee el CSV de 1,3 GB por bloques con goroutines y deja el silver y el gold. B, los
nodos (`cmd/nodo`): K contenedores que llaman a la API por TCP en el puerto 7070 y calculan los
parciales del K-Means sobre su rango del gold. C, la API coordinadora (`cmd/api`), en el 8080: REST
con JWT, el WebSocket y el coordinador del clúster; guarda en MongoDB (27017) y en Redis (6379).
D, la SPA en React, servida por un Caddy (servicio `web`) en el mismo origen que la API. Y dos
caminos: una predicción se resuelve en la API con el modelo en memoria; un entrenamiento pasa por
los nodos.

*Dónde:* `docs/diagramas/arquitectura.png`, `docker-compose.yml` · *Quién:* Rody · *Lámina:* 8 y 9

### 3. ¿Dónde está el millón de registros?

**Respuesta.** El CSV de OEFA tiene 2 883 877 filas en formato largo (una por estación, parámetro y
hora), 1,3 GB. El cargador las lee todas en paralelo; 1 434 498 son lecturas válidas, que se pivotan
a 236 001 horas-estación, y 65 873 tienen PM2,5 y PM10 a la vez: esas entran al K-Means.

*Dónde:* `reports/limpieza.json` (`filas_largas`) · *Quién:* Dayana · *Lámina:* 2 y 6

### 4. ¿Por qué descartan la mitad del archivo?

**Respuesta.** Porque OEFA marca con 0 lo que no midió: el 26,4 % de las filas son días enteros en
0, y si se tomaran como aire limpio diluirían los episodios. Además hay filas sin valor (una
plantilla de 2022), promedios diarios o móviles que no son horarios y gases repetidos en dos
unidades. Cada descarte queda contado por regla en el reporte, y una auditoría independiente en
pandas rehízo la limpieza: 40 de 40 chequeos sin diferencias.

*Dónde:* `datos/doc.go` (la cascada de reglas), `configs/limpieza.json` · *Quién:* Dayana ·
*Lámina:* 5, 6 y el respaldo de reglas

### 5. ¿Por qué K-Means y k = 3? Si la alerta es la media de 24 h, ¿qué aporta el modelo?

**Respuesta.** El enunciado recomienda K-Means para el caso 10, y lo usamos para describir
regímenes de aire con PM2,5 y PM10 en µg/m³ (`configs/kmeans.json`). k = 3 salió de 1 080 corridas
de calibración: la silueta más alta de la grilla con k ≥ 3 (0,54) y estable entre semillas (ARI
0,99). Los tres regímenes: bajo (73,1 % de las horas), medio (24,6 %) y alto (2,3 %, el único sobre
el ECA). Qué aporta: el régimen de cada lectura, que el panel muestra junto a cada estación, y una
señal temprana medida: en los 30 episodios, el régimen alto apareció antes (18 h en la mediana).
Pero acierta el 35 % de las veces que suena, y el ECA ya es una etiqueta legal sobre la media de
24 h: calcularla es exacto, aprenderla sería peor. Por eso la alerta es la norma y el régimen es
contexto. La predicción de concentraciones del caso 10 quedó fuera: no la hicimos, y no decimos
que el sistema pronostique.

*Dónde:* `configs/kmeans.json`, `reports/calibracion_decision.md` · *Quién:* Rody · *Lámina:*
14 y el respaldo «Por qué k = 3»

## Concurrencia en una máquina

### 6. ¿Dónde están los canales del cargador y para qué sirve cada uno?

**Respuesta.** Es un pipeline productor–consumidor. `trabajos` lleva los índices de bloque del
productor a los W workers; `resultados` lleva el parcial de cada bloque al recolector; `enVuelo`
es un canal con búfer de 2W que hace de semáforo, para que no haya más de 2W bloques leídos sin
fusionar (la lectura no acumula más de 2W bloques en memoria). Un `WaitGroup` cierra `resultados` cuando terminan
los workers.

*Dónde:* `datos/pipeline.go:29-31` · *Quién:* Julio · *Lámina:* 11

### 7. ¿Cómo garantizan que no hay condiciones de carrera al actualizar los centroides?

**Respuesta.** Por diseño, no hay escritura compartida: cada worker escribe solo el parcial de su
chunk, en su casilla `P[b]` de un arreglo preasignado, y los centroides son de solo lectura durante
la iteración. Después de la barrera, `Fusionar` y `Actualizar` corren en una sola goroutine. Todas
las pruebas corren con `go test -race` en cada push y PR, y el detector no encuentra ninguna en lo
que ejercitan las pruebas (mira los caminos que corren; no prueba la ausencia en general). Y los
workers ven los centroides nuevos sin carrera porque los reciben después por el canal: en Go, lo que
una goroutine escribe antes de enviar por un canal lo ve la que recibe.

*Dónde:* `kmeans/concurrente.go:34-35` (el contrato), `:58-59` (canal y barrera),
`kmeans/parciales.go:78` (`Fusionar`), `.github/workflows/go.yml` · *Quién:* Rody · *Lámina:* 12

### 8. ¿Usan Mutex? ¿Dónde y por qué?

**Respuesta.** En el bucle caliente del K-Means no hace falta, por lo de la pregunta anterior. Sí
donde hay estado compartido de verdad: la ventana de 24 h de cada estación en la API
(`sync.RWMutex` para el mapa y un `sync.Mutex` por estación, porque llegan predicciones de varias
estaciones a la vez), el pool de nodos (dos candados: `uso` para hablar con los nodos libres y `mu`
para los mapas, que se toman siempre en ese orden, primero `uso` y después `mu`, para que no haya
interbloqueo) y el hub del WebSocket. Y para publicar el modelo activo no se usa lock, sino un
`atomic.Pointer`.

*Dónde:* `ventana/ventana.go:98` y `:107`, `cluster/pool.go:86-88`, `api/servidor.go:104`
(`atomic.Pointer`) · *Quién:* Rody o Julio · *Lámina:* respaldo de patrones

### 9. ¿Por qué el speedup no llega a 12 con 12 workers?

**Respuesta.** El cargador llega a 4,94× (de 6,68 s a 1,35 s). Las fases seriales son el 8,3 % del
tiempo, y con eso Amdahl dejaría llegar a más de 6×; Karp-Flatt da una fracción serial efectiva del
13 %: el resto es coordinar y repartir bloques de 64 MiB, que en un CSV de 1,3 GB son pocos para 12
workers. En Windows se ve: con 4 workers, bloques de 16 MiB tardan un 8,5 % menos que los de 64 MiB.
El K-Means llega a 3,86×: una iteración dura 2,03 ms con 1 worker y 0,55 ms con 12, y en algo tan
corto la barrera de cada iteración pesa; con chunks de
10 000 filas solo hay 7 chunks para 12 workers y cae a 2,25×. En Windows, con 32 CPU, el cargador da
4,81× con 12 workers (intervalo 3,81 a 5,99). Lo importante: con el mismo chunk, el resultado es el mismo bit a
bit para cualquier W, y se reprodujo en otra máquina: Dayana rehízo la PC3 en macOS (darwin/arm64) y obtuvo los
mismos SHA-256 del silver y el gold, con 40 de 40 chequeos de la auditoría.

*Dónde:* `reports/benchmark_cargador.json`, `reports/benchmark_kmeans.json`,
`reports/benchmark_cargador_windows.json`, `reports/revision_dayana/` · *Quién:* Julio (cargador y
Windows), Rody (K-Means), Dayana (macOS) · *Lámina:* 11, 13, 27 y el respaldo de speedup

## Distribución: sockets, puertos y protocolo

### 10. ¿Dónde están los sockets? ¿Quién hace `Listen` y quién hace `Dial`?

**Respuesta.** La API, que es el coordinador, abre dos sockets de escucha con `net.Listen("tcp",
…)`: el 7070 para los nodos y el 8080 para HTTP. Una goroutine hace `Accept` en un bucle y cada
conexión nueva se atiende en su propia goroutine, que espera el `HOLA` con un plazo de 5 s. El nodo
es el que llama: `net.Dialer{Timeout: 3 s}` y `DialContext(ctx, "tcp", "api:7070")`.

*Dónde:* `cmd/api/main.go:318` y `:329` (`Listen`), `cluster/reunir.go:155` y `:171` (`Accept` y
la goroutine por conexión), `cmd/nodo/main.go:72-74` (`Dial`) · *Quién:* Julio · *Lámina:* 16

### 11. ¿Qué puertos usa el sistema?

**Respuesta.** 8080, la API por HTTP (REST, Swagger y WebSocket); 7070, la API por TCP para los
nodos; 27017 MongoDB; 6379 Redis; y el Caddy del servicio `web` en el 80 (443 con HTTPS en la
máquina pública). En Compose, los que se publican van solo a `127.0.0.1`, y el 7070 no se publica:
no está en `ports`, así que solo se alcanza dentro de la red de Compose, que es donde están los
nodos. `expose` no restringe nada: es documentación, como el `EXPOSE` del Dockerfile. En el
laboratorio el coordinador corrió fuera de Docker y escuchó en la IP de la PC.

*Dónde:* `cmd/api/main.go` (flags `-addr ":8080"` y `-nodos-addr ":7070"`),
`docker-compose.yml:32` (`expose: "7070"`), `:49`, `:74`, `:99`, `:132` · *Quién:* Julio ·
*Lámina:* 8

### 12. ¿Por qué el nodo llama al coordinador y no al revés? ¿Por qué estrella?

**Respuesta.** Porque así el nodo funciona detrás de cualquier red: Colab está detrás de NAT, y el
firewall de Windows de las PC del laboratorio bloquea las conexiones entrantes (abrir el puerto pide
administrador), mientras que las de salida pasan. Si pierde la conexión, vuelve a llamar solo. Estrella porque la fusión es una
suma de parciales que solo necesita el coordinador: los nodos no tienen nada que decirse. En clase
vimos otras topologías, el anillo de la papa caliente y el P2P de Agrawala, donde los procesos se
pasan un token o compiten por un recurso; aquí no hay token que circule ni recurso que excluir: cada
ronda es una reducción (una suma) y la estrella la hace en un solo salto. Y la medimos: con 4 PC del
laboratorio, una ronda de `PING` a todos a la vez, sin cálculo (`cmd/smoke`), tarda 0,515 ms de
mediana: es lo que la barrera paga en red en cada iteración.

*Dónde:* `cmd/nodo/main.go:1-22`, `protocolo/doc.go` (sección Sesión), `reports/smoke_lab_4.json`
· *Quién:* Julio · *Lámina:* 16

### 13. ¿Qué cambia entre el K-Means en una máquina y en el clúster?

**Respuesta.** Hay dos niveles, cada uno con su herramienta. **Entre procesos**, sockets TCP: el
coordinador manda `ITERAR` a cada nodo por su conexión y lee su `PARCIAL`. **Dentro de cada
proceso**, goroutines, canales y `WaitGroup`: en el coordinador, una goroutine por conexión y un
`WaitGroup` que espera a todas; en cada nodo, el mismo worker pool de la parte B sobre su rango.
Lo que no cambia es el algoritmo: `Fusionar` y el cierre de la iteración son las mismas funciones.
(El `go func()` de la ronda usa `i` y `e` del bucle sin pasarlas como parámetro: desde Go 1.22 cada
vuelta tiene su propia variable, y el módulo usa Go 1.26.)

*Dónde:* `protocolo/coordinador.go:194-203` (la ronda), `kmeans/nodo.go:38-53` (el pool del
nodo), `cluster/lloyd.go:288` · *Quién:* Julio · *Lámina:* 17

> No decir: «los chunks pasan a ser nodos, el `WaitGroup` pasa a ser una barrera por la red y el
> canal pasa a ser una conexión TCP». El docente la marcó como incorrecta en la PC4, y lo es: el
> `WaitGroup` sigue existiendo (es la barrera del coordinador, sobre goroutines bloqueadas leyendo
> cada una su socket), los canales y los chunks siguen dentro de cada nodo, y un canal comunica
> goroutines de un mismo proceso mientras que una conexión TCP comunica procesos.

### 14. ¿Cómo es el protocolo? ¿Por qué JSON y no binario?

**Respuesta.** Cada mensaje es un objeto JSON en una línea terminada en `\n`, con un campo `tipo`:
`HOLA`, `CARGAR`/`LISTO`, `ITERAR`, `PARCIAL`, `FIN`, `PING`/`PONG` y `ERROR`. Como en los labs,
cada conexión se lee con un `bufio.Reader` y se escribe con un `bufio.Writer`. Texto porque una ronda
son unos kilobytes (el costo es la latencia, no el ancho de banda), se puede leer con `nc` o
`tcpdump`, y `encoding/json` escribe cada `float64` con la representación más corta que se relee al
mismo valor: la ida y vuelta no cambia ningún bit. Los plazos son los del socket:
`SetReadDeadline` y `SetWriteDeadline`.

*Dónde:* `protocolo/doc.go`, `protocolo/codec.go:42` y `:123` (`bufio`), `:101` (el `\n`), `:72` (`SetWriteDeadline`) ·
*Quién:* Julio · *Lámina:* 17

### 15. ¿Cómo sabe que la fusión distribuida da el mismo modelo? ¿Por qué en orden de nodo?

**Respuesta.** La suma en coma flotante no es asociativa: sumar los parciales en el orden de
llegada podría dar centroides distintos en cada corrida. Por eso, después de la barrera, `Fusionar`
suma en orden de nodo, que es el orden de las filas. Con un nodo, el distribuido es el de una
máquina bit a bit; con varios, es la misma función pero agrupar por nodo cambia el orden de la suma,
y difiere en el orden de 10⁻¹⁵: a lo sumo 1,3 × 10⁻¹⁵ en las corridas de `cmd/distribuido` con
hasta cuatro nodos, y 7,7 × 10⁻¹⁵ en la corrida de la captura, con dos nodos por la API. La API
además compara cada entrenamiento con el K-Means secuencial y solo activa el modelo si la
diferencia es ≤ 10⁻⁹.

*Dónde:* `cluster/lloyd.go:142-144` y `:288`, `cluster/lloyd.go:25` (`Tolerancia`),
`api/entrenamientos.go:517` · *Quién:* Julio · *Lámina:* 17

### 16. ¿Qué pasa si un nodo se cae a mitad del entrenamiento? ¿Y si se cae la API?

**Respuesta.** Si un nodo no manda su `PARCIAL` dentro del plazo (10 s por ronda), la ronda se
aborta con un error que lo nombra y no hay modelo: no reasignamos el rango del nodo caído (se
podría, y el modelo sería el mismo salvo redondeo, pero no está hecho). Los nodos vuelven a llamar
solos y se relanza con los que queden. Un
`PARCIAL` rezagado se reconoce por su número de ronda y se descarta. Si se cae la API, el
entrenamiento en curso se pierde (los centroides de la última ronda quedan en Redis, pero todavía
no se reanuda desde ahí), los
nodos reintentan cada 2 s y la API vuelve con el último modelo activo, que está en MongoDB y Redis.

*Dónde:* `protocolo/coordinador.go:221-224`, `api/entrenamientos.go:425`,
`cmd/nodo/main.go` (`-espera 2s`), `protocolo/coordinador_test.go:551`
(`TestParcialRezagadoRealistaSobreTCP`, sobre TCP real) · *Quién:* Julio · *Lámina:* respaldo de
plazos

## La API

### 17. Explíquenos el código Go de la API: ¿cómo atiende una predicción?

**Respuesta.** Es `net/http` de la biblioteca estándar. Cada ruta se registra en el mux con los roles
que admite, y la función `ruta` la envuelve con la verificación del JWT y la medición de latencia.
El handler `predecir` lee el modelo de un `atomic.Pointer` (sin lock), clasifica la lectura con el
centroide más cercano, actualiza la media de 24 h de la estación en memoria, encola la predicción
para MongoDB y Redis y responde. Nada de ese camino espera a una base de datos.

*Dónde:* `api/servidor.go:203-220` (rutas), `api/servidor.go:395` y `:405` (`ruta` y `exigir`),
`api/rutas.go:238-290` (`predecir`) · *Quién:* Julio · *Lámina:* 19

### 18. ¿La predicción pasa por el clúster?

**Respuesta.** No. El clúster entrena; la API predice con el modelo ya entrenado, que vive en su
memoria. Un pedido de predicción se resuelve en la API en menos de un milisegundo. El que pasa por
el clúster es el entrenamiento: `POST /api/entrenamientos` (solo admin) toma K nodos del pool,
corre las rondas por TCP y, si el resultado coincide con el secuencial, reemplaza el modelo activo
con un `Store` atómico. Las predicciones en curso terminan con el anterior. Si se piden más nodos que
los libres, responde 422; si ya hay un entrenamiento en curso, 409.

*Dónde:* `api/rutas.go:240`, `api/entrenamientos.go:267-284` (semáforo), `:517` · *Quién:* Rody ·
*Lámina:* 9

### 19. ¿Cómo demuestran que responde en menos de 100 ms?

**Respuesta.** Con `cmd/carga`: 80 000 pedidos a `POST /api/predicciones` con JWT, con 1, 4, 16 y 64
clientes a la vez, medidos del lado del cliente. Con 64 clientes, p99 de 3,14 ms y máximo de
4,84 ms; ningún pedido sobre el límite y ningún error. Además la API mide la latencia de cada ruta
y la publica en `GET /api/metricas`, que la pantalla Clúster dibuja con la línea de los 100 ms. Y
la colección de Postman, que newman corre con 78 lecturas de prueba (5 casos de 14 estaciones),
recorre la API de punta a punta. Mientras tanto MongoDB y Redis guardaron 80 800 predicciones cada
uno, los 80 000 pedidos y los 800 de calentamiento, sin descartar ninguna; si una cola se llenara,
la API descartaría y lo contaría, pero el pedido no espera.

*Dónde:* `reports/api_latencia.json`, `cmd/carga/`, `docs/postman/` · *Quién:* Julio · *Lámina:* 20 y 27

### 20. ¿Cómo funciona el WebSocket? ¿Usaron una librería?

**Respuesta.** No: está escrito sobre `net/http` con la parte de RFC 6455 que hace falta. El
handler valida el apretón de manos, el origen y el token antes de responder `101`, y le quita la
conexión a `net/http` con `Hijack`. Un hub con canales de alta, baja y difusión reparte los eventos:
`estacion.regimen` cuando cambia una estación y `entrenamiento.ronda` en cada ronda. Cada cliente
tiene un búfer de 256 mensajes; si no lee a ese ritmo, se cierra con el código 1008. Medido entre
dos sesiones distintas (lo que manda el admin, lo que recibe un invitado que solo escucha), un
evento llega en 2,25 ms de mediana y 9,98 ms en el p99, sobre 132 lecturas. ¿Por qué no gorilla,
como en el laboratorio 14? Porque el sistema usa solo la biblioteca estándar (las únicas
dependencias son los drivers de MongoDB y Redis): el `Upgrade()` de gorilla lo reemplaza un apretón
escrito con `Hijack`. El reparto sí es el patrón del lab, un canal de difusión; pero el conjunto de
clientes lo maneja una sola goroutine, el hub, que lo cambia por los canales de alta y baja dentro
de un `select`, así que no necesita el `Lock()` del lab.

*Dónde:* `api/ws.go:17-30`, `api/hub.go:58-61` (canales), `:139-153` (el `select` del hub), `:471-500` (`seguirEventos`),
`docs/informe/capturas/tb2/pruebas/ws_latencia.json` · *Quién:* Julio · *Lámina:* 19 y 27

## MongoDB y Redis

### 21. ¿Qué guardan en MongoDB y qué en Redis? ¿Por qué las dos?

**Respuesta.** El enunciado pide los dos niveles. MongoDB es lo que tiene que durar, en 5
colecciones: `estaciones`, `predicciones`, `modelos`, `entrenamientos` (con sus rondas) y
`eventos_cluster`, con 5 índices. Redis es la caché, lo que se lee rápido, en 3 claves: el último régimen de
cada estación (lo que pinta el mapa), el modelo activo (para arrancar en caliente) y los centroides
de la última ronda de un entrenamiento (todavía no se reanuda desde ahí). La API escribe con una cola con búfer y una goroutine por base, por lotes
(`InsertMany` y un pipeline de Redis): bajo carga, 80 800 escrituras en cada una (80 000 pedidos y
800 de calentamiento) y 0 descartadas.

*Dónde:* `almacen/mongo/mongo.go:58-62` y `:240`, `almacen/redis/redis.go:40-43` y `:146`,
`api/escritor.go:232-240`, `docs/diagramas/entidades.png` · *Quién:* Dayana · *Lámina:* 21

## Docker

### 22. ¿Cómo se despliega? ¿Dónde está el Dockerfile y el docker-compose?

**Respuesta.** `docker compose up --build --scale nodo=K` levanta seis servicios: `web` (Caddy con
la UI), `api`, `nodo` × K, `mongo`, `redis` y `seed`. El `Dockerfile` de la raíz construye en dos
etapas: compila todos los comandos con `CGO_ENABLED=0` y los copia a una imagen distroless de 59 MB,
la misma para la API y los nodos. El de `web/` compila la UI con Node y la sirve con Caddy. El orden
de arranque lo dan los healthchecks: mongo y redis sanos, el seed termina, la API sana
(`api -salud`, porque distroless no tiene curl), y recién entonces la web y los nodos.

*Dónde:* `Dockerfile:30` y `:56`, `web/Dockerfile`, `docker-compose.yml` · *Quién:* Rody ·
*Lámina:* 8 y el respaldo de Docker Compose

### 23. ¿Cómo escala el número de nodos? ¿Cómo saben los nodos dónde está la API?

**Respuesta.** Con `--scale nodo=K`: todos los contenedores `nodo` corren
`nodo -coordinador api:7070`, y Docker resuelve `api` por su nombre en la red de Compose. Como el
nodo es el que llama, no hay que configurar sus direcciones en ningún lado: cada uno se presenta
con un nombre único (equipo y PID) y la API lo anota en su pool. Fuera de Docker, el mismo binario
compilado para Windows corrió en las PC del laboratorio.

*Dónde:* `docker-compose.yml` (servicio `nodo`), `cmd/nodo/main.go:113`, `lab/armar-usb.sh`
· *Quién:* Julio · *Lámina:* 8

## Frontend y seguridad

### 24. ¿Qué framework usa el frontend y qué módulos tiene?

**Respuesta.** React con Vite y TypeScript; los gráficos y el mapa en D3, sin teselas de terceros.
Las pantallas del enunciado: ingreso, panel (el mapa con el último estado de cada estación),
estación, simulación (un día de lecturas reales enviadas una por una), impacto y, para el admin,
entrenamiento (nodos, réplicas y semilla; las rondas llegan por el WebSocket y, como el
entrenamiento real dura milisegundos, la UI las reproduce más despacio) y clúster (nodos con su
latido, el CPU del proceso de la API, que es el coordinador, la latencia por ruta y los eventos; el
CPU de cada nodo no viaja por el protocolo). El tiempo real llega por el WebSocket.

*Dónde:* `web/src/App.tsx:40-67`, `web/src/armazon/menu.ts:14-24`, `web/src/api/ws.ts` · *Quién:*
Dayana · *Lámina:* 23

### 25. ¿Qué políticas de seguridad configuraron en la API?

**Respuesta.** JWT HS256 firmado con `crypto/hmac`, con una clave de al menos 32 bytes que llega por
variable de entorno; dura una hora. Cada ruta exige un rol: un usuario recibe `403` en una de admin
aunque fuerce la URL. Las claves se comparan en tiempo constante, el cuerpo de un pedido se limita a
64 KiB, el servidor tiene plazos de lectura y escritura, y el WebSocket valida origen y token antes
del `101` (el token va en `Sec-WebSocket-Protocol`, no en la URL). CORS solo para los orígenes de
`CORS_ORIGENES`. En el despliegue, el Caddy sirve UI y API en el mismo origen, con una
`Content-Security-Policy` estricta, HSTS, `nosniff` y `Referrer-Policy`; el token vive en
`sessionStorage`.

*Dónde:* `api/jwt.go:23`, `api/servidor.go:481`, `:492` y `:518`, `api/hub.go:444` y `:478`,
`api/cors.go`, `cmd/api/main.go:347`, `web/Caddyfile:37`, `web/src/sesion/almacen.ts:3` ·
*Quién:* Dayana · *Lámina:* 25

### 26. Si la UI esconde las pantallas de admin, ¿qué impide que un usuario entre igual?

**Respuesta.** La API. La UI solo esconde; la autorización la decide `exigir`, que verifica el JWT y
su rol en cada pedido. Con el token del invitado, `GET /api/metricas` responde 403. Lo mostramos en
la demo desde Swagger.

*Dónde:* `api/servidor.go:405` (`exigir`), `api/servidor.go:212-217` (rutas de admin) ·
*Quién:* Dayana o Julio · *Lámina:* 25 y la demo

## Evaluación e impacto

### 27. ¿Qué responde el sistema sobre el aire? ¿A cuántas personas beneficia?

**Respuesta.** De 64 748 horas con media de 24 h calculable, 1 184 superaron el ECA (1,8 %), en 30
episodios de 8 estaciones. Depende de la actividad: junto a la refinería de zinc de Cajamarquilla y
las ladrilleras de Huachipa, en Lurigancho, 585 de 879 horas; junto a las minas y la agroindustria,
menos del 1 %. Lurigancho tenía 240 814 habitantes en 2017 (INEI), pero no decimos que esa sea la
gente expuesta: vivir en el distrito no es haber respirado ese aire, y por eso la población no se
suma entre distritos.

*Dónde:* `reports/impacto.json` (`totales`, `tipos_actividad`, `distritos`, `metadatos.notas`) ·
*Quién:* Dayana · *Lámina:* 28

### 28. ¿Con cuánta anticipación avisa? ¿Es un pronóstico?

**Respuesta.** No es un pronóstico: es una medida descriptiva sobre las mismas lecturas del
entrenamiento, sin partición temporal. En los 30 episodios, la primera lectura horaria sobre el
umbral apareció 20 h antes del cruce (mediana; cuartiles 16,2 a 22 h), y la primera hora en el nivel
alto del K-Means, 18 h antes. Pero justo antes del cruce la señal estuvo encendida solo 1 h
seguida: las señales van y vienen. Y el nivel alto acierta el 35 % de las veces que suena. Por eso
la alerta del sistema es la norma misma, la media de 24 h.

*Dónde:* `reports/impacto.json` (`anticipacion`, `senales`), `ventana/ventana.go:67` (`Alerta`) ·
*Quién:* Dayana · *Lámina:* 29

### 29. ¿Qué limitaciones tiene?

**Respuesta.** 33 estaciones (el portal anuncia 34; en el CSV hay 33) en 10 de las 25 regiones,
puestas donde OEFA fiscaliza, no donde vive la gente: una zona sin estación no es una zona con aire
limpio. Solo 17 miden PM2,5 y PM10 por hora; las otras 16 no entran en ninguna cifra. Muchas son
campañas de semanas y los datos son de 2018 a 2022: la UI dice «último estado recibido», nunca
«ahora». Y con 65 873 filas repartir no acelera: la ronda por la red cuesta lo mismo que una
iteración en una máquina; el clúster se justifica con más datos.

*Dónde:* `reports/impacto.json` (`alcance`), `reports/eda_criba.md` · *Quién:* Rody · *Lámina:* 30
y el respaldo «Cuándo conviene repartir»

## Proceso

Por si preguntan fuera del sistema; no son las más probables.

- **Git Flow.** `develop` es la rama por defecto; cada cambio va en `feature/*` con una PR a
  `develop`; cada entrega sale de `release/pc3`, `release/pc4` y `release/tb2` con una PR a `main`
  y un tag (`pc3`, `pc3.1`, `pc4`). El CI corre `go test -race`, `go vet`, `gofmt` y, para la UI,
  lint, pruebas y build.
- **¿Por qué hay Python? ¿Por qué el impacto lo calcula Python?** El sistema (cargador, K-Means,
  nodos y API) es Go, y también todo lo que se calcula en línea: la media de 24 h, la alerta y la
  banda INCA de cada lectura (`ventana/ventana.go`). Python solo está en el laboratorio: los
  notebooks de Colab (criba, calibración, auditoría y figuras) y `scripts/impacto.py`, que resume el
  gold histórico para la página de impacto y el informe con las mismas reglas que `ventana/ventana.go`
  (lo dice `reports/impacto.json`, en `metadatos.reglas.implementacion`).
- **Kanban.** El proyecto de GitHub «aire-distribuido · Trabajo Final CC65 (PC3, PC4 y TB2)»
  (<https://github.com/users/R0SEWT/projects/8>), con las tareas de cada entrega.
- **IA.** Se declara en el anexo del informe: se usó Claude como asistente de código y de
  redacción, con revisión del equipo.

## De reserva

Preguntas menos probables, con la respuesta en una línea.

- **¿Qué pasa con un cluster vacío?** `Actualizar` lo deja donde estaba: solo divide si el conteo
  es mayor que cero (`kmeans/parciales.go:152`). En el modelo entregado no hubo ninguno.
- **¿Cómo es el apretón de manos del WebSocket sin librería?** El cliente manda
  `Sec-WebSocket-Key`; la API responde `101 Switching Protocols` con `Sec-WebSocket-Accept`, que es
  el SHA-1 de la clave más una constante del RFC 6455, en base64, y le quita la conexión a
  `net/http` con `Hijack` (`api/ws.go`).
- **¿Por qué no reasignan el rango de un nodo caído?** Se podría y el modelo sería el mismo salvo
  redondeo; no está hecho: la corrida se aborta y se relanza.
- **¿Las rondas que se ven son en tiempo real?** Llegan por el WebSocket en cuanto el coordinador
  cierra cada una; como el entrenamiento real dura milisegundos, la UI las reproduce más despacio.
- **¿El CPU del panel es el de los nodos?** No: es el del proceso de la API, que coordina. De cada
  nodo se ve su latido y lo que tardó en cada ronda.

## Lo que no se dice

| No decir | Por qué | Decir |
|:--|:--|:--|
| «Los chunks pasan a ser nodos, el `WaitGroup` pasa a ser una barrera por la red y el canal pasa a ser una conexión TCP» | El docente la marcó como incorrecta en la PC4 (pregunta 13) | «Entre procesos, sockets TCP; dentro de cada proceso, goroutines, canales y `WaitGroup`» |
| «La predicción la calcula el clúster» | El clúster entrena; la API predice con el modelo en memoria | «El clúster entrena; la API predice» |
| «Las rondas en vivo» | La UI las reproduce más despacio, y su pantalla lo dice | «Las rondas llegan por el WebSocket; la UI las reproduce» |
| «El 7070 no se publica por el `expose`» | `expose` es documentación | «No está en `ports`» |
| «Distribuido = secuencial» | Con varios nodos difiere a 10⁻¹⁵ | «Bit a bit con un nodo; con varios, a 10⁻¹⁵, dentro de la tolerancia» |
| «Predecimos episodios» o «avisamos con 20 horas» | La anticipación es descriptiva, sin partición temporal; la cota baja es 1 h | «En los datos, la primera señal apareció 20 h antes, en la mediana» |
| «34 estaciones» | El portal lo dice; en el CSV hay 33 | «33 estaciones, 17 con PM por hora» |
| «Una hora sobre 50 µg/m³ incumple el ECA» | El ECA es una media de 24 horas | «La media de 24 horas superó el ECA» |
| «No usamos Mutex» | La API sí los usa (pregunta 8) | «El bucle del K-Means no lo necesita» |
| «Escala lineal» o «el clúster es más rápido» | 4,94× con 12 workers; con esta tabla, repartir no acelera | «El resultado no depende de cómo se reparte» |
| «Afecta a 240 mil personas» | Es la población del distrito, no gente expuesta | «Su distrito tenía 240 814 habitantes en 2017» |
| «El aire de ahora» | Los datos son de 2018 a 2022 | «El último estado recibido» |
