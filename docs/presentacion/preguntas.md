# Preguntas probables del docente en la sustentación del TB2

Las 28 preguntas que con más probabilidad hará el docente, con una respuesta corta y el archivo
y la línea que la respaldan, para abrir el código si pide verlo. Cada una dice quién la responde
según el [reparto](reparto.md), aunque los tres tienen que poder contestar cualquiera.

El docente corrige con el vocabulario de sus laboratorios: sockets, `Listen` y `Dial`, puertos,
canales, `WaitGroup`, `Mutex`, patrones y Docker. Por eso las respuestas usan esas palabras y
nombran la función de Go, no una abstracción nuestra. Las cifras son las del deck, que salen de
`reports/` y de `docs/informe/generado/cifras.yml`; si cambian, manda el deck.

Las referencias `archivo:línea` son de la rama `develop` del 27/11/2026 (commit `6d91f4f`).
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

Cuatro observaciones con base (la quinta, la del Kanban, era un error de hecho). Las cuatro tienen
ahora una lámina y una respuesta que no repite el error.

| Observación de la PC4 | Qué faltaba | Dónde se responde ahora |
|:--|:--|:--|
| No explica el código Go distribuido: listen, dial, puertos, patrones | Los listados mostraban `Ronda`, `Lloyd` y `Pool`, un nivel por encima de los sockets | Lámina 14 (el código de `net.Listen`, `Accept` y `DialContext`), lámina 15 (la ronda con su `WaitGroup`), respaldo de patrones; preguntas 9 a 15 |
| «Incorrecto: los chunks pasan a ser nodos, el WaitGroup pasa a ser una barrera a través de la red y el canal pasa a ser una conexión TCP» | Una analogía falsa al pie de la letra | Lámina 15: «entre procesos, sockets TCP; dentro de cada proceso, goroutines, canales y `WaitGroup`»; pregunta 12 |
| No explica el código Go de la API ni cómo pasa la petición al clúster | «El servidor en Go» era prosa; no se dijo que las predicciones no pasan por el clúster | Lámina 8 (el camino de un pedido) y lámina 17 (el handler); preguntas 16 a 18 |
| No incluye el Dockerfile ni el docker-compose; no explica el despliegue | Los archivos iban solo en el ZIP | Lámina 7 (servicios y puertos), respaldo de Docker Compose; preguntas 21 y 22 |

## Planteamiento y datos

### 1. ¿Cuál es la pregunta de impacto social y para quién es?

**Respuesta.** ¿En qué estaciones y a qué horas el aire supera el ECA de PM2,5 y PM10, y con cuánta
anticipación se puede avisar? Es para las personas con enfermedades respiratorias que viven junto
a una actividad que OEFA fiscaliza. Es, casi literal, el ejemplo del enunciado para el caso 10
(«¿cuándo se disparará la contaminación del aire para alertar a personas con enfermedades
respiratorias?»), elegida con una criba sobre el dato entre nueve candidatas.

*Dónde:* `docs/specs/2026-10-31-preguntas-candidatas.md` · *Quién:* Rody · *Lámina:* 2

### 2. ¿Dónde está el millón de registros?

**Respuesta.** El CSV de OEFA tiene 2 883 877 filas en formato largo (una por estación, parámetro y
hora), 1,3 GB. El cargador las lee todas en paralelo; 1 434 498 son lecturas válidas, que se pivotan
a 236 001 horas-estación, y 65 873 tienen PM2,5 y PM10 a la vez: esas entran al K-Means.

*Dónde:* `reports/limpieza.json` (`filas_largas`) · *Quién:* Dayana · *Lámina:* 1 y 5

### 3. ¿Por qué descartan la mitad del archivo?

**Respuesta.** Porque OEFA marca con 0 lo que no midió: el 26,4 % de las filas son días enteros en
0, y si se tomaran como aire limpio diluirían los episodios. Además hay filas sin valor (una
plantilla de 2022), promedios diarios o móviles que no son horarios y gases repetidos en dos
unidades. Cada descarte queda contado por regla en el reporte, y una auditoría independiente en
pandas rehízo la limpieza: 40 de 40 chequeos sin diferencias.

*Dónde:* `datos/doc.go` (la cascada de reglas), `configs/limpieza.json` · *Quién:* Dayana ·
*Lámina:* 4, 5 y el respaldo de reglas

### 4. ¿Por qué K-Means? ¿Por qué k = 3?

**Respuesta.** El enunciado recomienda K-Means o redes neuronales para el caso 10, y lo que se busca
son niveles de aire, sin etiquetas: es clustering. Las features son PM2,5 y PM10 en µg/m³,
estandarizadas (`configs/kmeans.json`). k = 3 salió de 1 080 corridas de calibración: la silueta
más alta de la grilla con k ≥ 3 (0,54) y estable entre semillas (ARI 0,99). Los tres niveles: bajo
(73,1 % de las horas), medio (24,6 %) y alto (2,3 %, el único sobre el ECA).

*Dónde:* `configs/kmeans.json`, `reports/calibracion_decision.md` · *Quién:* Rody · *Lámina:*
respaldo «Por qué k = 3»

## Concurrencia en una máquina

### 5. ¿Dónde están los canales del cargador y para qué sirve cada uno?

**Respuesta.** Es un pipeline productor–consumidor. `trabajos` lleva los índices de bloque del
productor a los W workers; `resultados` lleva el parcial de cada bloque al recolector; `enVuelo`
es un canal con búfer de 2W que hace de semáforo, para que no haya más de 2W bloques leídos sin
fusionar (la memoria no crece con el archivo). Un `WaitGroup` cierra `resultados` cuando terminan
los workers.

*Dónde:* `datos/pipeline.go:29-31` · *Quién:* Julio · *Lámina:* 10

### 6. ¿Cómo garantizan que no hay condiciones de carrera al actualizar los centroides?

**Respuesta.** Por diseño, no hay escritura compartida: cada worker escribe solo el parcial de su
chunk, en su casilla `P[b]` de un arreglo preasignado, y los centroides son de solo lectura durante
la iteración. Después de la barrera, `Fusionar` y `Actualizar` corren en una sola goroutine. Todas
las pruebas corren con `go test -race` en cada push y PR, y el detector de carreras no encuentra
ninguna.

*Dónde:* `kmeans/concurrente.go:34-35` (el contrato), `:58-59` (canal y barrera),
`kmeans/parciales.go:78` (`Fusionar`), `.github/workflows/go.yml` · *Quién:* Rody · *Lámina:* 11

### 7. ¿Usan Mutex? ¿Dónde y por qué?

**Respuesta.** En el bucle caliente del K-Means no hace falta, por lo de la pregunta anterior. Sí
donde hay estado compartido de verdad: la ventana de 24 h de cada estación en la API
(`sync.RWMutex` para el mapa y un `sync.Mutex` por estación, porque llegan predicciones de varias
estaciones a la vez), el pool de nodos (dos candados: `uso` para hablar con los nodos libres y `mu`
para los mapas) y el hub del WebSocket. Y para publicar el modelo activo no se usa lock, sino un
`atomic.Pointer`.

*Dónde:* `ventana/ventana.go:98` y `:107`, `cluster/pool.go:86-88`, `api/servidor.go:104`
(`atomic.Pointer`) · *Quién:* Rody o Julio · *Lámina:* respaldo de patrones

### 8. ¿Por qué el speedup no llega a 12 con 12 workers?

**Respuesta.** El cargador llega a 4,94× (de 6,68 s a 1,35 s): la fusión en orden y las fases
seriales, el 8,3 % del tiempo, no se reparten; Karp-Flatt da una fracción serial efectiva del 13 %.
El K-Means llega a 3,86×: una iteración dura 2 ms y la barrera de cada iteración pesa; con chunks de
10 000 filas solo hay 7 chunks para 12 workers y cae a 2,25×. Lo importante: el resultado es el mismo
bit a bit para cualquier W.

*Dónde:* `reports/benchmark_cargador.json`, `reports/benchmark_kmeans.json` · *Quién:* Julio
(cargador), Rody (K-Means) · *Lámina:* 10, 12 y el respaldo de speedup

## Distribución: sockets, puertos y protocolo

### 9. ¿Dónde están los sockets? ¿Quién hace `Listen` y quién hace `Dial`?

**Respuesta.** La API, que es el coordinador, abre dos sockets de escucha con `net.Listen("tcp",
…)`: el 7070 para los nodos y el 8080 para HTTP. Una goroutine hace `Accept` en un bucle y cada
conexión nueva se atiende en su propia goroutine, que espera el `HOLA` con un plazo de 5 s. El nodo
es el que llama: `net.Dialer{Timeout: 3 s}` y `DialContext(ctx, "tcp", "api:7070")`.

*Dónde:* `cmd/api/main.go:318` y `:329` (`Listen`), `cluster/reunir.go:155` y `:171` (`Accept` y
la goroutine por conexión), `cmd/nodo/main.go:72-74` (`Dial`) · *Quién:* Julio · *Lámina:* 14

### 10. ¿Qué puertos usa el sistema?

**Respuesta.** 8080, la API por HTTP (REST, Swagger y WebSocket); 7070, la API por TCP para los
nodos; 27017 MongoDB; 6379 Redis; y el Caddy del servicio `web` en el 80 (443 con HTTPS en la
máquina pública). En Compose, todos se publican solo en `127.0.0.1` y el 7070 ni se publica: con
`expose` queda dentro de la red de Compose, que es donde están los nodos.

*Dónde:* `cmd/api/main.go` (flags `-addr ":8080"` y `-nodos-addr ":7070"`),
`docker-compose.yml:32` (`expose: "7070"`), `:49`, `:74`, `:99`, `:132` · *Quién:* Julio ·
*Lámina:* 7

### 11. ¿Por qué el nodo llama al coordinador y no al revés? ¿Por qué estrella?

**Respuesta.** Porque así el nodo funciona detrás de cualquier red: Colab no acepta conexiones
entrantes, y en las PC del laboratorio el nodo corre sin permisos de administrador, solo con
conexiones de salida. Si pierde la conexión, vuelve a llamar solo. Estrella porque la fusión es una
suma de parciales que solo necesita el coordinador: los nodos no tienen nada que decirse. Y la
medimos: con 4 PC del laboratorio, una ronda completa por la red tarda 0,515 ms de mediana.

*Dónde:* `cmd/nodo/main.go:1-22`, `protocolo/doc.go` (sección Sesión), `reports/smoke_lab_4.json`
· *Quién:* Julio · *Lámina:* 14

### 12. ¿Qué cambia entre el K-Means en una máquina y en el clúster?

**Respuesta.** Hay dos niveles, cada uno con su herramienta. **Entre procesos**, sockets TCP: el
coordinador manda `ITERAR` a cada nodo por su conexión y lee su `PARCIAL`. **Dentro de cada
proceso**, goroutines, canales y `WaitGroup`: en el coordinador, una goroutine por conexión y un
`WaitGroup` que espera a todas; en cada nodo, el mismo worker pool de la parte B sobre su rango.
Lo que no cambia es el algoritmo: `Fusionar` y el cierre de la iteración son las mismas funciones.

*Dónde:* `protocolo/coordinador.go:194-203` (la ronda), `kmeans/nodo.go:38-53` (el pool del
nodo), `cluster/lloyd.go:288` · *Quién:* Julio · *Lámina:* 15

> No decir: «los chunks pasan a ser nodos, el `WaitGroup` pasa a ser una barrera por la red y el
> canal pasa a ser una conexión TCP». El docente la marcó como incorrecta en la PC4, y lo es: el
> `WaitGroup` sigue existiendo (es la barrera del coordinador, sobre goroutines bloqueadas leyendo
> cada una su socket), los canales y los chunks siguen dentro de cada nodo, y un canal comunica
> goroutines de un mismo proceso mientras que una conexión TCP comunica procesos.

### 13. ¿Cómo es el protocolo? ¿Por qué JSON y no binario?

**Respuesta.** Cada mensaje es un objeto JSON en una línea terminada en `\n`, con un campo `tipo`:
`HOLA`, `CARGAR`/`LISTO`, `ITERAR`, `PARCIAL`, `FIN`, `PING`/`PONG` y `ERROR`. Texto porque una ronda
son unos kilobytes (el costo es la latencia, no el ancho de banda), se puede leer con `nc` o
`tcpdump`, y `encoding/json` escribe cada `float64` con la representación más corta que se relee al
mismo valor: la ida y vuelta no cambia ningún bit. Los plazos son los del socket:
`SetReadDeadline` y `SetWriteDeadline`.

*Dónde:* `protocolo/doc.go`, `protocolo/codec.go:101` (el `\n`), `:72` (`SetWriteDeadline`) ·
*Quién:* Julio · *Lámina:* 15

### 14. ¿Cómo sabe que la fusión distribuida da el mismo modelo? ¿Por qué en orden de nodo?

**Respuesta.** La suma en coma flotante no es asociativa: sumar los parciales en el orden de
llegada daría centroides distintos en cada corrida. Por eso, después de la barrera, `Fusionar`
suma en orden de nodo, que es el orden de las filas. Con un nodo, el distribuido es el de una
máquina bit a bit; con hasta cuatro, difiere a lo sumo en 1,3 × 10⁻¹⁵, que es redondeo. La API
además compara cada entrenamiento con el K-Means secuencial y solo activa el modelo si la
diferencia es ≤ 10⁻⁹.

*Dónde:* `cluster/lloyd.go:142-144` y `:288`, `cluster/lloyd.go:25` (`Tolerancia`),
`api/entrenamientos.go:517` · *Quién:* Julio · *Lámina:* 15

### 15. ¿Qué pasa si un nodo se cae a mitad del entrenamiento? ¿Y si se cae la API?

**Respuesta.** Si un nodo no manda su `PARCIAL` dentro del plazo (10 s por ronda), la ronda se
aborta con un error que lo nombra, la corrida se corta con todos y no hay modelo: uno con menos
nodos sería otro modelo. Los nodos vuelven a llamar solos y quedan libres para relanzar. Un
`PARCIAL` rezagado se reconoce por su número de ronda y se descarta. Si se cae la API, el
entrenamiento en curso se pierde (el punto de control queda en Redis, pero no se reanuda), los
nodos reintentan cada 2 s y la API vuelve con el último modelo activo, que está en MongoDB y Redis.

*Dónde:* `protocolo/coordinador.go:221-224`, `api/entrenamientos.go:425`,
`cmd/nodo/main.go` (`-espera 2s`), `protocolo/coordinador_test.go:551`
(`TestParcialRezagadoRealistaSobreTCP`, sobre TCP real) · *Quién:* Julio · *Lámina:* respaldo de
plazos

## La API

### 16. Explíquenos el código Go de la API: ¿cómo atiende una predicción?

**Respuesta.** Es `net/http` de la biblioteca estándar. Cada ruta se registra en el mux con los roles
que admite, y la función `ruta` la envuelve con la verificación del JWT y la medición de latencia.
El handler `predecir` lee el modelo de un `atomic.Pointer` (sin lock), clasifica la lectura con el
centroide más cercano, actualiza la media de 24 h de la estación en memoria, encola la predicción
para MongoDB y Redis y responde. Nada de ese camino espera a una base de datos.

*Dónde:* `api/servidor.go:203-220` (rutas), `api/servidor.go:395` y `:405` (`ruta` y `exigir`),
`api/rutas.go:238-290` (`predecir`) · *Quién:* Julio · *Lámina:* 17

### 17. ¿La predicción pasa por el clúster?

**Respuesta.** No. El clúster entrena; la API predice con el modelo ya entrenado, que vive en su
memoria. Un pedido de predicción se resuelve en la API en menos de un milisegundo. El que pasa por
el clúster es el entrenamiento: `POST /api/entrenamientos` (solo admin) toma K nodos del pool,
corre las rondas por TCP y, si el resultado coincide con el secuencial, reemplaza el modelo activo
con un `Store` atómico. Las predicciones en curso terminan con el anterior.

*Dónde:* `api/rutas.go:240`, `api/entrenamientos.go:267-284` (semáforo), `:517` · *Quién:* Rody ·
*Lámina:* 8

### 18. ¿Cómo demuestran que responde en menos de 100 ms?

**Respuesta.** Con `cmd/carga`: 80 000 pedidos a `POST /api/predicciones` con JWT, con 1, 4, 16 y 64
clientes a la vez, medidos del lado del cliente. Con 64 clientes, p99 de 3,14 ms y máximo de
4,84 ms; ningún pedido sobre el límite y ningún error. Además la API mide la latencia de cada ruta
y la publica en `GET /api/metricas`, que la pantalla Clúster dibuja con la línea de los 100 ms.

*Dónde:* `reports/api_latencia.json`, `cmd/carga/` · *Quién:* Julio · *Lámina:* 18

### 19. ¿Cómo funciona el WebSocket? ¿Usaron una librería?

**Respuesta.** No: está escrito sobre `net/http` con la parte de RFC 6455 que hace falta. El
handler valida el apretón de manos, el origen y el token antes de responder `101`, y le quita la
conexión a `net/http` con `Hijack`. Un hub con canales de alta, baja y difusión reparte los eventos:
`estacion.regimen` cuando cambia una estación y `entrenamiento.ronda` en cada ronda. Cada cliente
tiene un búfer de 256 mensajes; si no lee a ese ritmo, se cierra con el código 1008.

*Dónde:* `api/ws.go:17-30`, `api/hub.go:58-61` (canales), `:471-500` (`seguirEventos`) · *Quién:*
Julio · *Lámina:* 17

## MongoDB y Redis

### 20. ¿Qué guardan en MongoDB y qué en Redis? ¿Por qué las dos?

**Respuesta.** El enunciado pide los dos niveles. MongoDB es lo que tiene que durar, en 5
colecciones: `estaciones`, `predicciones`, `modelos`, `entrenamientos` (con sus rondas) y
`eventos_cluster`, con 5 índices. Redis es lo que se lee rápido, en 3 claves: el último régimen de
cada estación (lo que pinta el mapa), el modelo activo (para arrancar en caliente) y el punto de
control de cada ronda. La API escribe con una cola con búfer y una goroutine por base, por lotes
(`InsertMany` y un pipeline de Redis): bajo carga, 80 800 escrituras en cada una y 0 descartadas.

*Dónde:* `almacen/mongo/mongo.go:58-62` y `:240`, `almacen/redis/redis.go:40-43` y `:146`,
`api/escritor.go:232-240`, `docs/diagramas/entidades.png` · *Quién:* Dayana · *Lámina:* 19

## Docker

### 21. ¿Cómo se despliega? ¿Dónde está el Dockerfile y el docker-compose?

**Respuesta.** `docker compose up --build --scale nodo=K` levanta seis servicios: `web` (Caddy con
la UI), `api`, `nodo` × K, `mongo`, `redis` y `seed`. El `Dockerfile` de la raíz construye en dos
etapas: compila todos los comandos con `CGO_ENABLED=0` y los copia a una imagen distroless de 59 MB,
la misma para la API y los nodos. El de `web/` compila la UI con Node y la sirve con Caddy. El orden
de arranque lo dan los healthchecks: mongo y redis sanos, el seed termina, la API sana
(`api -salud`, porque distroless no tiene curl), y recién entonces la web y los nodos.

*Dónde:* `Dockerfile:30` y `:56`, `web/Dockerfile`, `docker-compose.yml` · *Quién:* Rody ·
*Lámina:* 7 y el respaldo de Docker Compose

### 22. ¿Cómo escala el número de nodos? ¿Cómo saben los nodos dónde está la API?

**Respuesta.** Con `--scale nodo=K`: todos los contenedores `nodo` corren
`nodo -coordinador api:7070`, y Docker resuelve `api` por su nombre en la red de Compose. Como el
nodo es el que llama, no hay que configurar sus direcciones en ningún lado: cada uno se presenta
con un nombre único (equipo y PID) y la API lo anota en su pool. Fuera de Docker, el mismo binario
compilado para Windows corrió en las PC del laboratorio.

*Dónde:* `docker-compose.yml` (servicio `nodo`), `cmd/nodo/main.go:113`, `lab/armar-usb.sh`
· *Quién:* Julio · *Lámina:* 7

## Frontend y seguridad

### 23. ¿Qué framework usa el frontend y qué módulos tiene?

**Respuesta.** React con Vite y TypeScript; los gráficos y el mapa en D3, sin teselas de terceros.
Las pantallas del enunciado: ingreso, panel (el mapa con el último estado de cada estación),
estación, simulación (un día de lecturas reales enviadas una por una), impacto y, para el admin,
entrenamiento (nodos, réplicas y semilla; las rondas en vivo) y clúster (nodos, CPU, latencia por
ruta y eventos). El tiempo real llega por el WebSocket.

*Dónde:* `web/src/App.tsx:40-67`, `web/src/armazon/menu.ts:14-24`, `web/src/api/ws.ts` · *Quién:*
Dayana · *Lámina:* 21

### 24. ¿Qué políticas de seguridad configuraron en la API?

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
*Quién:* Dayana · *Lámina:* 22

### 25. Si la UI esconde las pantallas de admin, ¿qué impide que un usuario entre igual?

**Respuesta.** La API. La UI solo esconde; la autorización la decide `exigir`, que verifica el JWT y
su rol en cada pedido. Con el token del invitado, `GET /api/metricas` responde 403. Lo mostramos en
la demo desde Swagger.

*Dónde:* `api/servidor.go:405` (`exigir`), `api/servidor.go:212-217` (rutas de admin) ·
*Quién:* Dayana o Julio · *Lámina:* 22 y la demo

## Evaluación e impacto

### 26. ¿Qué responde el sistema sobre el aire? ¿A cuántas personas beneficia?

**Respuesta.** De 64 748 horas con media de 24 h calculable, 1 184 superaron el ECA (1,8 %), en 30
episodios de 8 estaciones. Depende de la actividad: junto a la refinería de zinc de Cajamarquilla y
las ladrilleras de Huachipa, en Lurigancho, 585 de 879 horas; junto a las minas y la agroindustria,
menos del 1 %. Lurigancho tenía 240 814 habitantes en 2017 (INEI), pero no decimos que esa sea la
gente expuesta: vivir en el distrito no es haber respirado ese aire, y por eso la población no se
suma entre distritos.

*Dónde:* `reports/impacto.json` (`totales`, `tipos_actividad`, `distritos`, `metadatos.notas`) ·
*Quién:* Dayana · *Lámina:* 25

### 27. ¿Con cuánta anticipación avisa? ¿Es un pronóstico?

**Respuesta.** No es un pronóstico: es una medida descriptiva sobre las mismas lecturas del
entrenamiento, sin partición temporal. En los 30 episodios, la primera lectura horaria sobre el
umbral apareció 20 h antes del cruce (mediana; cuartiles 16,2 a 22 h), y la primera hora en el nivel
alto del K-Means, 18 h antes. Pero justo antes del cruce la señal estuvo encendida solo 1 h
seguida: las señales van y vienen. Y el nivel alto acierta el 35 % de las veces que suena. Por eso
la alerta del sistema es la norma misma, la media de 24 h.

*Dónde:* `reports/impacto.json` (`anticipacion`, `senales`), `ventana/ventana.go:67` (`Alerta`) ·
*Quién:* Dayana · *Lámina:* 26

### 28. ¿Qué limitaciones tiene?

**Respuesta.** 33 estaciones (el portal anuncia 34; en el CSV hay 33) en 10 de las 25 regiones,
puestas donde OEFA fiscaliza, no donde vive la gente: una zona sin estación no es una zona con aire
limpio. Solo 17 miden PM2,5 y PM10 por hora; las otras 16 no entran en ninguna cifra. Muchas son
campañas de semanas y los datos son de 2018 a 2022: la UI dice «último estado recibido», nunca
«ahora». Y con 65 873 filas repartir no acelera: la ronda por la red cuesta lo mismo que una
iteración en una máquina; el clúster se justifica con más datos.

*Dónde:* `reports/impacto.json` (`alcance`), `reports/eda_criba.md` · *Quién:* Rody · *Lámina:* 27
y el respaldo «Cuándo conviene repartir»

## Proceso

Por si preguntan fuera del sistema; no son las más probables.

- **Git Flow.** `develop` es la rama por defecto; cada cambio va en `feature/*` con una PR a
  `develop`; cada entrega sale de `release/pc3`, `release/pc4` y `release/tb2` con una PR a `main`
  y un tag (`pc3`, `pc3.1`, `pc4`). El CI corre `go test -race`, `go vet`, `gofmt` y, para la UI,
  lint, pruebas y build.
- **¿Hay Python?** Solo en el laboratorio: los notebooks de Colab (criba, calibración, auditoría y
  figuras) y `scripts/impacto.py`. El sistema (cargador, K-Means, nodos y API) es Go. El experimento
  de 20 GPU del laboratorio D56 usó nodos Python con PyTorch y datos sintéticos, con el coordinador
  en Go; está fuera del sistema y su código no está en la rama entregada.
- **IA.** Se declara en el anexo del informe: se usó Claude como asistente de código y de
  redacción, con revisión del equipo.

## Lo que no se dice

| No decir | Por qué | Decir |
|:--|:--|:--|
| «Los chunks pasan a ser nodos, el `WaitGroup` pasa a ser una barrera por la red y el canal pasa a ser una conexión TCP» | El docente la marcó como incorrecta en la PC4 (pregunta 12) | «Entre procesos, sockets TCP; dentro de cada proceso, goroutines, canales y `WaitGroup`» |
| «La predicción la calcula el clúster» | El clúster entrena; la API predice con el modelo en memoria | «El clúster entrena; la API predice» |
| «Predecimos episodios» o «avisamos con 20 horas» | La anticipación es descriptiva, sin partición temporal; la cota baja es 1 h | «En los datos, la primera señal apareció 20 h antes, en la mediana» |
| «34 estaciones» | El portal lo dice; en el CSV hay 33 | «33 estaciones, 17 con PM por hora» |
| «Una hora sobre 50 µg/m³ incumple el ECA» | El ECA es una media de 24 horas | «La media de 24 horas superó el ECA» |
| «No usamos Mutex» | La API sí los usa (pregunta 7) | «El bucle del K-Means no lo necesita» |
| «Escala lineal» o «el clúster es más rápido» | 4,94× con 12 workers; con esta tabla, repartir no acelera | «El resultado no depende de cómo se reparte» |
| «Afecta a 240 mil personas» | Es la población del distrito, no gente expuesta | «Su distrito tenía 240 814 habitantes en 2017» |
| «El aire de ahora» | Los datos son de 2018 a 2022 | «El último estado recibido» |
