# Auditoría de la sustentación del TB2 · Programación Concurrente y Distribuida · 28/11/2026

Revisión con la skill `revisar-entrega` (modo A) de lo que el grupo presenta en la clase de la
semana 15: el deck renderizado (`docs/presentacion/slides/_site/index.html` y su PDF, 41
páginas), lo que se dice en cada lámina (las notas del orador de `slides/index.qmd`), el reparto
(`reparto.md`) y las respuestas preparadas (`preguntas.md`). La sustentación cuenta para el TB2:
cada criterio de la rúbrica dice «cumple con la exposición de su trabajo demostrando conocimiento
total del tema». No es la DD (esa es un informe individual sobre ABET 4, aparte).

Fuentes: rúbrica del enunciado ✓ · rúbrica del Aula (`Rubrica_Logro_1ACC0065_202620.xlsx`) ✓ ·
enunciado ✓ · observaciones previas del docente (PC4) ✓ · material del docente, semanas 10 a 14 ✓
· informe del TB2 (rama local `claude/tb2-informe-065625`, `e0eb784`) ✓.

Auditoría hecha con IA (Claude); la cierra una persona (sección final).

## Veredicto

**Listo con riesgos**: cubre los tres criterios que se juegan en la exposición con las palabras del
docente (sockets, puertos, canales, Docker, la API y su camino al clúster), y las cifras salen de
los JSON; los riesgos que quedan son de ejecución (la demo en vivo y el tiempo), no de contenido.

{{CORRECTOR}}

## Puntos en riesgo (mayor a menor)

| # | Criterio (pts) | Qué buscará el docente | Qué hay hoy | Arreglo | Costo |
|:--|:---|:---|:---|:---|:---|
| 1 | Implementación (8) e Interfaz (5) | Que el sistema funcione en vivo («funciona correctamente», «muestra los resultados en tiempo real») | Demo en aire.rosewt.dev con plan B en tres niveles; el nivel 2 (sin Docker ni red) probado el 27/11 | Ensayar la demo completa el día anterior con la lista de `reparto.md`; subir `JWT_DURACION=4h`; comprobar que CA-ESP-01 no tenga estado posterior al 21/12/2021 | 30 min |
| 2 | Todos («conocimiento total») | Que cada integrante responda sus preguntas sin leer | 29 preguntas con respuesta y su `archivo:línea`; reparto por quién responde | Que cada uno lea sus preguntas y abra una vez el código que cita | 1 h por persona |
| 3 | Todos | Que la exposición entre en el tiempo que dé el docente | 14:30 más 4:00 de demo; versión corta de unos 11 + 3:20 | Ensayo con reloj; decidir antes qué versión se usa | 30 min |
| 4 | Implementación (8) | Docker «utiliza docker»: el Dockerfile y el compose a la vista | Lámina 8 (servicios, puertos y el Dockerfile nombrado) y respaldo con el Dockerfile en dos etapas | Si pregunta, abrir el respaldo «Docker Compose» | — |

## Matriz de trazabilidad

`matriz.json` tiene el vocabulario por criterio; `buscar_evidencia.py` se corrió sobre el PDF
proyectado, sobre lo que se dice y sobre `preguntas.md`. Los números de lámina son los que muestra
el deck (la portada es la 1).

| Criterio | Pts | Vocabulario del docente (fuente) | Dónde está | ¿Con sus palabras? | Estado |
|:---|---:|:---|:---|:---:|:---|
| Planteamiento: problema, dataset, pregunta de impacto social | 3 | OEFA, >1M, pregunta, ECA, para quién [Rúbrica, Enunciado §4] | Láminas 2 y 3; pregunta 1 | sí | cubierto |
| Planteamiento: diagrama de arquitectura | 3 | «diagrama de arquitectura», puertos, Docker Compose [Rúbrica] | Lámina 8 (título «Arquitectura»), 9 (el camino de un pedido); pregunta 2 | sí (corregido) | cubierto |
| Implementación: carga concurrente de >1M | 8 | goroutines, canales, workers, `WaitGroup` [Rúbrica, Enunciado parte A, Clase sem. 10] | Láminas 6 y 11; pregunta 6 | sí | cubierto |
| Implementación: entrenamiento paralelo sin condiciones de carrera | 8 | worker pool, barrera, `Mutex`, «condiciones de carrera» [Rúbrica, Enunciado parte B] | Láminas 12 y 13 (corregidas: «sin condiciones de carrera», `go test -race`); preguntas 7 y 8 | sí (corregido) | cubierto |
| Implementación: distribución por sockets y puertos | 8 | `net.Listen`, `Accept`, `net.Dial`, puerto, `bufio`, JSON, goroutine por conexión [Enunciado: «Sockets y patrones»; Clase sem. 10–14] | Láminas 15 (código de `Listen`, `Accept`, `DialContext`; `bufio` agregado) y 16 (la ronda con su `WaitGroup`); preguntas 10 a 16 | sí | cubierto |
| Implementación: Docker | 8 | Dockerfile, `docker compose`, imagen, healthcheck [Rúbrica: «utiliza docker»; Clase sem. 11] | Lámina 8 (corregida: el Dockerfile nombrado), respaldo «Docker Compose» (Dockerfile en dos etapas, servicios, `healthcheck`); preguntas 22 y 23 | sí (corregido) | cubierto |
| Implementación: API REST, tiempo real, métricas, < 100 ms | 8 | `net/http`, JWT, `POST /api/predicciones`, p99, WebSocket [Rúbrica, Enunciado parte C] | Láminas 18 (el handler) y 19 (latencia); preguntas 17, 19 y 20 | sí | cubierto |
| Flujo: la API coordina el clúster; la predicción no pasa por él | 8 | El camino del pedido hasta los nodos [Enunciado §6; observación de la PC4] | Lámina 9; pregunta 18 | sí | cubierto |
| Implícito: MongoDB (persistente) y Redis (caché) | — | «Persistente: MongoDB», «Cache: Redis» [Enunciado §6] | Lámina 20 (corregida: «Redis es la caché»); pregunta 21 | sí (corregido) | cubierto |
| Interfaz: framework, menú, parámetros, tiempo real, impacto | 5 | React, «menú de opciones», configura desde la UI, tiempo real, impacto [Rúbrica] | Lámina 22 (corregida: «menú de opciones por rol»), 26 y 27; demo; pregunta 24 | sí (corregido) | cubierto |
| Interfaz: módulos del enunciado | 5 | ingreso, dashboard, entrenamiento y simulación, panel admin (CPU, latencia, nodos, logs) [Enunciado §6] | Lámina 22, respaldo «Las pantallas de admin», demo | sí | cubierto |
| Entregable 3: seguridad de la API | 5 | CORS, JWT, políticas [Enunciado, Entregable 3] | Lámina 23; preguntas 25 y 26 | sí | cubierto |
| Entregable 3: evaluación experimental | 5 | pruebas, datos de prueba, responde la pregunta de impacto [Enunciado, Entregable 3] | Láminas 25 a 27; preguntas 19, 27 y 28 | sí | cubierto |
| Implícito: Git Flow y Kanban | 2 | Git Flow, Kanban [Enunciado; Rúbrica: Informe] | No están en el deck (son del informe); `preguntas.md`, «Proceso» (Kanban agregado) | sí | débil, a propósito |
| Ética (ABET 4) | — | lo que el dato no puede decir | Lámina 28; láminas 26 y 27 | sí | cubierto |

## Afirmaciones

`afirmaciones.py` sobre el PDF, las notas, `preguntas.md` y `reparto.md`:

- **ALTA (analogía)**: solo aparece la frase que el docente marcó como incorrecta en la PC4,
  citada a propósito en «Lo que no se dice» y en la pregunta 13, con su reemplazo literal. En el
  deck no hay ninguna.
- **MEDIA (absolutos)**, todas con su respaldo:
  - «bit a bit para todo W» (láminas 13 y 25): `benchmark_kmeans.json` (`identico`) y la inercia
    idéntica con 1 a 12 workers.
  - «con 1 nodo, bit a bit» (láminas 16 y 25): `cifras.distribuido.bit_a_bit_con` y las pruebas
    `TestLloydConUnNodoEsElConcurrenteBitABit` y `TestUnNodoDaElModeloDeEntrenarBitABit`.
  - «el coordinador nunca llama» (lámina 15): en el código, `net.Dial` solo está en `cmd/nodo`.
  - «nunca "ahora"» (lámina 28) y «fusionar siempre en el mismo orden» (lámina 30): ciertas al pie
    de la letra.

## Coherencia con el informe del TB2

- Las 107 claves de `cifras.yml` que usa el deck son idénticas en la rama del informe.
- Las cifras de impacto del deck (`_deck.yml`, desde `reports/impacto.json`) coinciden con
  `cifras.impacto` del informe; dos cuartiles iban con otro formato («22» y «20» frente a «22,0» y
  «20,0»): **corregido** en `datos.py`.
- La pregunta de impacto social tenía otra redacción en el deck: **corregida** a la del informe
  («¿en qué estaciones de OEFA y a qué horas el aire entra en el nivel alto de PM2,5/PM10, y con
  cuánta anticipación eso avisa de un episodio que supera el ECA?»).
- «Nivel alto» en la anticipación: **corregido** a «régimen alto», como el informe y la pantalla de
  impacto.
- Pruebas del frontend y WebSocket: el texto del informe dice 708 pruebas en 36 archivos y
  p50 2,25 ms / p99 9,98 ms, igual que el deck. *Para el informe*: su tabla de pruebas del TB2
  (`cifras.pruebas_tb2.web`) cuenta 587, que son los `it`/`test` declarados (las parametrizadas
  cuentan una vez); conviene decirlo en esa tabla para que no parezca otra cifra.

## Correcciones de esta auditoría

| Dónde | Antes | Ahora |
|:---|:---|:---|
| Lámina 8 | «Las cuatro partes, en Docker Compose»; el Dockerfile no se nombraba | «Arquitectura: …» y un chip «Dockerfile: una imagen para la API y los nodos»; en las notas, el Dockerfile en dos etapas |
| Lámina 3 y pregunta 1 | Una redacción propia de la pregunta de impacto | La del informe |
| Lámina 13 y notas de la 12 | No decía «condiciones de carrera» | «Sin condiciones de carrera al actualizar los centroides: `go test -race` en cada push» |
| Lámina 15 | «`encoding/json` + `\n`» | «`bufio` + `encoding/json` + `\n`», como en los labs |
| Lámina 20 | «Redis responde rápido» | «Redis es la caché», como el enunciado |
| Lámina 22 | No decía «menú» | «Menú de opciones por rol» |
| Lámina 27 | «nivel alto del K-Means» | «régimen alto del K-Means» |
| Respaldo Docker Compose | Sin el Dockerfile; no decía `healthcheck` | El Dockerfile en dos etapas, como texto; el orden de arranque por `healthcheck` |
| `reparto.md` y `preguntas.md` | Los números de lámina iban uno por detrás de los del deck (no contaban la portada) | Renumerados; la portada es la 1 y tiene su fila |
| `preguntas.md` | 28 preguntas | 29: entra «Expliquen la arquitectura» (2); la 12 compara la estrella con el anillo y el P2P de los labs; la 14 nombra `bufio`; la 19 suma Postman; la 20 explica por qué no gorilla (lab 14) y por qué el hub no necesita su `Lock()`; la 21 dice «caché»; «Proceso» suma el Kanban |

{{CORRECCIONES_FRIO}}

## Lectura humana (la cierra una persona, no la IA)

- [ ] Leí el deck renderizado (`bash slides/render.sh`, pantalla completa) contra esta matriz.
  Quién: ______ Fecha: ______
- [ ] Cada integrante leyó sus láminas, sus notas y sus preguntas.
- [ ] Ensayo con reloj y la demo completa, con la lista de `reparto.md`.
- [ ] El anexo de uso de IA del informe declara también esta revisión.
