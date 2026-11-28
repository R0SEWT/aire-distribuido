# Guion del video del TB2 (Entregable 3)

Caso 10, *Monitoreo de Calidad del Aire y Sensores Ambientales*. El enunciado pide un video de **6 minutos
como máximo** con «la presentación de la solución, puntos importantes de su construcción y su
funcionamiento (end to end)», donde **participen todos los integrantes y demuestren conocimiento del
tema**. Sin video se pierden los 2 puntos de la rúbrica y se afecta el resto.

**Duración objetivo: 5:37** (12 bloques). El tope interno es **5:45**: deja 15 segundos de margen sobre
los 6 minutos por si alguien habla más lento. `armar-tb2.sh` lo comprueba y avisa.

| Quién | Bloques | Tiempo | Qué explica |
|---|---|---|---|
| **Rody** | 01, 02, 04, 09, 12 | 2:08 | Apertura, arquitectura, el clúster por sockets TCP, el entrenamiento desde la UI y el cierre |
| **Dayana** | 03, 06, 10 | 1:48 | El dato y el cargador concurrente (y su reproducción en macOS), MongoDB y Redis, el impacto social |
| **Julio** | 05, 07, 08, 11 | 1:41 | La API coordinadora, la demo de punta a punta, las pruebas (y su corrida en Windows) y el despliegue |

Cada quien explica lo técnico de la parte que construyó o documentó (reporte de participación de la PC3
y de la PC4): Dayana el dato y las bases, Julio la API (contrato, Postman, latencia) y Rody el clúster.

**Cómo se usa este archivo.** `montar_tb2.py` lo lee tal cual. Cada `## Bloque NN` es un clip:

- `Quién`: la persona que habla. Su clip de voz y cámara va en `docs/video/clips/NN-nombre.*`
  (`03-dayana.mp4`). Mientras no llegue, el montaje usa una voz sintética y marca el bloque como
  BORRADOR; `armar-tb2.sh --final` se niega a montar con un bloque sin clip.
- `Duración`: el tiempo objetivo. Sale de leer el texto a ritmo natural (unas 2,3 palabras por segundo).
  El montaje ajusta lo que se ve a lo que dure el clip de verdad.
- `Cámara`: `burbuja` (un círculo con la cámara sobre lo que se ve, abajo a la derecha; `izq` lo pasa a
  la izquierda y `arriba`, arriba a la derecha) o `completa` (la cámara llena la pantalla con el nombre
  abajo). Sin video en el clip (solo audio), va un círculo con las iniciales.
- `Pantalla`: qué se ve detrás. Puede haber varias líneas `Pantalla`; `parte=0.4` dice qué fracción
  del bloque ocupa cada una (las que no lo dicen se reparten el resto). Los tipos:
  - `remotion ID | pie`: una escena animada de `docs/video/remotion/`, renderizada a la duración del
    bloque (sus tiempos internos se estiran en proporción).
  - `pantalla NOMBRE | pie`: una grabación real de la SPA hecha por `pantalla/grabar.mjs` (Playwright
    contra el Compose local). Se acelera o se frena hasta un 40 % para calzar; si aún sobra, se corta,
    y si falta, se congela el último cuadro.
  - `terminal NOMBRE | pie`: la grabación real `casts/NOMBRE.cast` (`grabar.sh`, asciinema).
    `vel=N` fija la velocidad.
  - `figura RUTA | pie`, `figuras RUTA1 + RUTA2 | pie`, `texto Título | línea ; línea`, `caratula`,
    `final`: diapositivas de Pillow.
- El bloque ```` ```dice ```` es **el texto exacto que la persona lee**. Se puede decir con las palabras de
  cada uno, pero las cifras van tal cual.

**Cifras.** Todas salen de `reports/` y de `docs/informe/generado/cifras.yml` (la clave va entre
corchetes). Ninguna es inventada.

**Lo que no se dice** (ver también `docs/presentacion/en-sencillo.md`):

- «Predecimos episodios» o «pronosticamos»: el sistema clasifica y avisa sobre lo medido; la anticipación
  es una medida descriptiva sobre los mismos datos.
- «Donde no hay estación, el aire está bien»: OEFA mide junto a lo que fiscaliza.
- «Escala lineal» o «el clúster es más rápido»: con 65 873 filas, la red y la carga pesan tanto como el
  cálculo. Lo que se demuestra es que la ronda reparte el trabajo y da el mismo modelo.
- Comparaciones con otras cosas («es como…»): se dice qué hace el código, con los nombres del curso.
- «El WaitGroup hace de barrera» ni nada que «pasa a ser» otra cosa: en la PC4 el docente marcó como
  «Incorrecto» una frase así. Se dice el mecanismo: una goroutine por nodo y un `WaitGroup` que espera a todas.
- Que la predicción pasa por el clúster: no pasa. La resuelve la API con el modelo activo en memoria; el
  clúster entrena. El bloque 05 lo dice, porque en la PC4 su ausencia se marcó.
- «Todo se prueba con -race»: las pruebas de Go, sí; las de la interfaz son de Vitest.

---

## Bloque 01 · Quiénes somos y qué pregunta respondemos

- Quién: Rody
- Duración: 0:24
- Cámara: completa
- Pantalla: `caratula`
- Qué se ve: Rody a cámara completa, con el título del trabajo, el curso y los tres nombres sobre la
  imagen. Si Rody manda solo audio, la carátula.
- Cifras: caso 10; ECA de 24 h, 50 µg/m³ de PM2,5 y 100 de PM10 [alerta.eca_pm2_5, alerta.eca_pm10].

```dice
Hola, somos Rody Vilchez, Dayana Gómez y Julio Meza. Este es aire-distribuido, para el caso diez: las
lecturas horarias de calidad del aire de OEFA. La pregunta: en qué estaciones y a qué horas el aire
supera el estándar de calidad ambiental, el ECA, y con cuánta anticipación se puede avisar.
```

## Bloque 02 · Arquitectura

- Quién: Rody
- Duración: 0:27
- Cámara: burbuja
- Pantalla: `remotion Arquitectura | Las cuatro partes del enunciado sobre Docker Compose (docs/diagramas/arquitectura.drawio)`
- Qué se ve: el diagrama animado: el CSV y el cargador, los nodos, la API coordinadora con MongoDB y
  Redis, y la SPA detrás de Caddy. Cada caja se enciende cuando se nombra.

```dice
Son cuatro partes, en Go salvo la interfaz. Un cargador concurrente limpia el CSV. Un clúster de nodos
entrena un K-Means que agrupa cada hora, por su PM2,5 y PM10, en tres regímenes: bajo, medio y alto. Una
API coordinadora habla con los nodos por TCP, guarda en MongoDB y Redis y atiende a una SPA en React.
```

## Bloque 03 · El dato de OEFA y el cargador concurrente

- Quién: Dayana
- Duración: 0:41
- Cámara: burbuja arriba
- Pantalla: `remotion PipelineCargador | El pipeline del cargador (cmd/cargar): bloques por bytes, un pool de workers y la fusión en orden de bloque | parte=0.58`
- Pantalla: `figura docs/informe/capturas/cargar.png | go run ./cmd/cargar sobre el CSV completo de OEFA: leídas, procesadas y lo que descarta cada regla (captura real de la PC3) | parte=0.24 | crop=0,314,1240,578`
- Pantalla: `texto El mismo SHA-256 en tres sistemas | Linux, el benchmark del informe: silver 63b707f4…, gold 2fdace5a… ; Windows (#42, Julio): los mismos, con bloques de 16, 64 y 128 MiB y de 1 a 12 workers ; macOS (#45, Dayana): los mismos, y la auditoría 40 de 40`
- Qué se ve: la escena animada del pipeline (el CSV, los bloques, los workers, la fusión y el gold); la captura
  real de `cmd/cargar` sobre el CSV completo, con lo que descarta cada regla; y la tarjeta con las
  huellas de silver y gold en Linux, Windows y macOS.
- Cifras: 1,3 GB [limpieza.bytes], 2 883 877 filas [limpieza.filas_largas_leidas], bloques de 64 MiB
  [benchmark_cargador.bloque], 1,33 s con 12 workers [limpieza.workers, limpieza.duracion_s] y 1,32 a 1,41 s
  con 14 en las capturas: por eso se dice «menos de un segundo y medio», 4,94×
  [benchmark_cargador.speedup]; las huellas de silver y gold en `reports/benchmark_cargador.json`,
  `reports/benchmark_cargador_windows.json` y `reports/revision_dayana/ejecucion.json`
  [revision.mismos_hashes, revision.ok, revision.chequeos].

```dice
Soy Dayana. El dato de OEFA es un CSV de 1,3 gigabytes con 2 millones 883 mil filas. El cargador lo
parte en bloques de 64 megabytes y manda los índices por un canal a un pool de workers. Cada worker
limpia su bloque en su propia memoria, sin candados, y un recolector fusiona los parciales en orden de
bloque. La mitad de las filas se descarta, sobre todo ceros que en OEFA significan «sin dato». El
resultado es el mismo con cualquier número de workers, y el archivo entero se carga en menos de un
segundo y medio. Y lo reproduje en una Mac: el mismo SHA-256 que en Linux.
```

## Bloque 04 · El clúster: K-Means distribuido por sockets TCP

- Quién: Rody
- Duración: 0:38
- Cámara: burbuja
- Pantalla: `remotion RondaDistribuida | Una iteración del K-Means distribuido (protocolo/, cluster/; docs/diseno.md §4)`
- Qué se ve: el coordinador y tres nodos. Los nodos marcan al puerto 7070; CARGAR y LISTO; las rondas
  ITERAR → PARCIAL con la barrera, la fusión en orden de nodo y la inercia que baja; al final, la tabla
  de 1 a 4 nodos con la misma inercia y la diferencia con el secuencial (se ve, no se narra).
- Cifras: puerto 7070 [pc4.puerto_nodos], 44 iteraciones [distribuido.iteraciones], 1 a 4 nodos
  [distribuido.nodos_lista], diferencia máxima 1,3 × 10⁻¹⁵ [distribuido.dif_max]. Las corridas de 1 a 4
  nodos son procesos locales en una misma máquina (`reports/kmeans_distribuido_local_N.json`): el video no
  dice que fueran máquinas distintas.
- Código: `net.Listen` y `Accept` en `cmd/api/main.go` y `cluster/reunir.go`; `net.Dialer.DialContext` en
  `cmd/nodo/main.go`; `json.Marshal`, `bufio.Writer` y `bufio.Reader` en `protocolo/codec.go`; la goroutine
  por nodo y el `WaitGroup` en `protocolo.Ronda` (`protocolo/coordinador.go`).

```dice
La API escucha a los nodos en el puerto 7070 con net.Listen y Accept, y cada nodo se conecta con un
net.Dialer. Hablan en JSON por línea, con json.Marshal y un bufio.Reader. Con CARGAR, cada nodo lee su
rango de su propia copia del gold: por la red solo viajan centroides y parciales. En cada iteración, el
coordinador lanza una goroutine por nodo, que manda ITERAR y espera el PARCIAL con un plazo; un
WaitGroup espera a todas y los parciales se fusionan en orden de nodo.
```

## Bloque 05 · La API coordinadora

- Quién: Julio
- Duración: 0:34
- Cámara: burbuja
- Pantalla: `pantalla swagger | Swagger UI en /docs: el contrato docs/api/openapi.yaml, servido por la propia API | parte=0.55`
- Pantalla: `terminal carga | go run ./cmd/carga: 20 000 pedidos por nivel con 1, 4, 16 y 64 clientes (grabación real contra el Compose)`
- Qué se ve: Swagger con las rutas y el `POST /api/predicciones` abierto; después la medición de latencia
  en la terminal, con el p99 de cada nivel y «cumple».
- Cifras: 80 000 pedidos [api_latencia.pedidos_total], 64 clientes [api_latencia.clientes_max], p99 de
  3,14 ms en el informe [api_latencia.p99_peor] y de 5,9 ms en la corrida grabada (la máquina estaba grabando a
  la vez: por eso se dice «no pasó de 6»), 0 errores [api_latencia.errores], límite de 100 ms.

```dice
Soy Julio. La API está hecha con net/http, sin frameworks, con su contrato en OpenAPI y un token JWT por
rol. Una lectura entra por POST y no pasa por el clúster: la API usa el modelo activo en memoria, asigna
el régimen, actualiza la media de 24 horas, decide la alerta y manda el nuevo estado por WebSocket. Con
80 mil pedidos y hasta 64 clientes a la vez, el percentil 99 no pasó de 6 milisegundos.
```

## Bloque 06 · MongoDB y Redis

- Quién: Dayana
- Duración: 0:27
- Cámara: burbuja
- Pantalla: `terminal bases | Las cinco colecciones de MongoDB y las claves de Redis en el Compose (grabación real)`
- Qué se ve: `mongosh` contando las cinco colecciones y `redis-cli` con el último estado de una
  estación.
- Cifras: 5 colecciones [datos.colecciones]; 80 800 predicciones aceptadas, contando las 800 de
  calentamiento, escritas todas en MongoDB y en Redis, 0 descartadas y 0 fallidas
  [api_latencia.escrituras_aceptadas, api_latencia.escritas_mongo, api_latencia.escritas_redis,
  api_latencia.escrituras_descartadas]; cola de 32 768 [pc4.cola_compose].

```dice
Guardamos en dos niveles. MongoDB, el persistente, tiene predicciones, entrenamientos, modelos,
estaciones y eventos del clúster; Redis, el último estado de cada estación y el modelo activo. La
predicción no espera a las bases: va a un canal con búfer y una goroutine por base escribe en lotes. En
la prueba de carga llegaron las 80 800 predicciones, contando el calentamiento, sin descartar ninguna.
```

## Bloque 07 · Demo: ingreso, escenario y tiempo real

- Quién: Julio
- Duración: 0:26
- Cámara: burbuja
- Pantalla: `pantalla escenario | La SPA contra el Compose local: ingreso, panel vacío, escenario y el mapa que se llena por WebSocket`
- Qué se ve: el ingreso; el panel vacío; «cargar el escenario»; el mapa del Perú y la lista que se
  llenan solas, con las estaciones en alerta arriba.
- Cifras: 48 horas de diciembre de 2021 (spec del frontend, §5.4).

```dice
Ahora la interfaz, de punta a punta. Entro con usuario y clave y la API me da el token. Cargo el
escenario: 48 horas reales de diciembre de 2021, cada lectura un POST. En la API, un hub recibe por
canales las altas, las bajas y los eventos, y los reparte a cada cliente del WebSocket: por eso el mapa
se pinta solo.
```

## Bloque 08 · Demo: simular un día

- Quién: Julio
- Duración: 0:17
- Cámara: burbuja
- Pantalla: `pantalla simulacion | Simulación: un día real de una estación, lectura por lectura`
- Qué se ve: la pantalla de simulación con la estación, el día y la velocidad; la ventana que se llena,
  la media que aparece y la alerta que se enciende.
- Cifras: 18 lecturas como mínimo [pc4.minimo_lecturas], ventana de 24 h [pc4.ventana_horas].

```dice
En Simulación elijo una estación y un día. Las lecturas se mandan una por una: se ve cómo se
llena la ventana de 24 horas, cuándo aparece la media, con 18 lecturas, y en qué hora se enciende la
alerta.
```

## Bloque 09 · Demo: entrenamiento y clúster desde la UI

- Quién: Rody
- Duración: 0:24
- Cámara: burbuja
- Pantalla: `pantalla entrenamiento | Entrenamiento con los tres nodos del Compose: las rondas llegan por WebSocket y se reproducen | parte=0.6`
- Pantalla: `pantalla cluster | Clúster: nodos y su latencia, CPU del coordinador, latencia de la API por ruta y eventos`
- Qué se ve: el formulario con 3 nodos, la curva de inercia que baja ronda a ronda y el resultado;
  después la pantalla del clúster.
- Cifras: 44 iteraciones [distribuido.iteraciones].

```dice
Como administrador lanzo un entrenamiento con los tres nodos. Dura milisegundos, así que la pantalla
recibe las rondas por el WebSocket y las reproduce: la inercia baja en cada iteración. En Clúster se ven
los nodos con su latencia, el CPU del coordinador, la latencia de cada ruta y los eventos.
```

## Bloque 10 · Impacto social

- Quién: Dayana
- Duración: 0:40
- Cámara: burbuja
- Pantalla: `pantalla impacto | Impacto: con datos reales del gold (reports/impacto.json)`
- Qué se ve: la página de impacto de arriba abajo, en el orden en que se narra, con los textos de la #115:
  las horas-estación sobre el ECA, «Dónde podría servir esta información» (la población de cada distrito),
  la anticipación (las dos medidas) y lo que el dato no puede decir.
- Cifras: 64 748 horas calculables y 1 184 sobre el ECA [alerta.horas_calculables,
  alerta.horas_sobre_eca]; 30 episodios, mediana de 20 horas desde el primer encendido y de 1 hora
  encendida sin interrupción [impacto.anticipacion.total.lectura_horaria]; 17 de 33 estaciones con PM
  por hora, en 10 distritos [impacto.alcance].

```dice
¿Y para qué sirve? De 64 748 horas-estación con media de 24 horas, 1 184 superaron el ECA; la misma hora
en dos estaciones cuenta dos veces. Cada estación muestra la población de su distrito como contexto, sin
sumarla. En los 30 episodios sobre el ECA, la lectura horaria ya había pasado el umbral una mediana de
20 horas antes; pero se prende y se apaga, y seguida llevaba apenas una hora. Es una descripción, no un
pronóstico. Y solo 17 de las 33 estaciones miden PM por hora: donde no se mide, no se puede afirmar que
el aire esté limpio.
```

## Bloque 11 · Pruebas y despliegue

- Quién: Julio
- Duración: 0:24
- Cámara: burbuja
- Pantalla: `terminal pruebas | go test -race -count=1 ./... (grabación real) | parte=0.45 | vel=6`
- Pantalla: `terminal compose | docker compose up --scale nodo=3, docker compose ps y la salud de la API (grabación real)`
- Qué se ve: las pruebas de todos los paquetes de Go con el detector de carreras; el Compose con la UI, la API,
  tres nodos, MongoDB y Redis sanos.

```dice
Las pruebas de Go corren con el detector de carreras, guion race, también en la integración continua. Y
en Windows comprobé que el cargador da el mismo SHA-256 que en Linux. Docker Compose levanta la
interfaz, la API, las bases y los nodos, que se escalan con scale. Y está desplegado en aire punto
rosewt punto dev.
```

## Bloque 12 · Cierre

- Quién: Rody
- Duración: 0:15
- Cámara: completa
- Pantalla: `final`
- Qué se ve: Rody a cámara completa; en los últimos segundos, la tarjeta final con los tres nombres, el
  repositorio y la URL.

```dice
Eso es aire-distribuido: datos reales de OEFA procesados en paralelo, un modelo entrenado en un clúster
y una alerta que cualquiera puede leer. Gracias por vernos.
```
