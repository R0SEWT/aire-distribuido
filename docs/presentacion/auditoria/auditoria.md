# Auditoría de la sustentación del TB2 · Programación Concurrente y Distribuida · 28/11/2026

Revisión con la skill `revisar-entrega` (modo A) de lo que el grupo presenta en la clase de la
semana 15: el deck renderizado (`slides/_site/index.html` y su PDF, 42 láminas), lo que se dice
en cada una (las notas del orador de `slides/index.qmd`), el reparto (`reparto.md`) y las
respuestas preparadas (`preguntas.md`). La sustentación cuenta para el TB2: cada criterio de la
rúbrica dice «cumple con la exposición de su trabajo demostrando conocimiento total del tema». No
es la DD, que es un informe individual sobre ABET 4 y se entrega aparte.

Fuentes: rúbrica del enunciado ✓ · rúbrica del Aula (`Rubrica_Logro_1ACC0065_202620.xlsx`: mismos
criterios y puntajes, Planteamiento 3, Implementación 8, Interfaz 5, Informe 2, Video 2; agrega que
no tener video afecta los demás) ✓ · enunciado ✓ · observaciones del docente a la PC4 ✓ · labs de
las semanas 10 a 14 ✓ · informe del TB2 (rama local `claude/tb2-informe-065625`, `e0eb784`) ✓.

Archivos de la auditoría, en esta carpeta: `auditoria.md` (este reporte) y `matriz.json` (el
vocabulario por criterio, para `buscar_evidencia.py`). Los números de lámina son los que muestra
el deck, con la portada como 1.

Auditoría hecha con IA (Claude): una sesión revisó y otra, en frío, hizo de docente sin ver el
repositorio. La cierra una persona (sección final).

## Veredicto

**Listo con riesgos.** Los tres criterios que se juegan en la exposición (Planteamiento,
Implementación e Interfaz) están a la vista y con las palabras del docente: `net.Listen`, `Accept`,
`Dial`, puertos, canales, `WaitGroup`, Docker, el camino de un pedido hasta el clúster. Las cifras
salen de los JSON y coinciden con el informe. Los riesgos que quedan son la demo en vivo, el tiempo
y una pregunta de fondo que el docente probablemente hará: qué aporta el K-Means si la alerta es la
media de 24 horas, y dónde está la predicción de concentraciones del caso 10.

## Puntos en riesgo (mayor a menor)

| # | Criterio (pts) | Qué buscará el docente | Qué hay hoy | Arreglo | Costo |
|:--|:---|:---|:---|:---|:---|
| 1 | Implementación (8) e Interfaz (5) | Que funcione en vivo y en tiempo real | Demo en la lámina 24, en el medio para que no se corte; plan B en tres niveles, el 2 probado sin red el 27/11 | Ensayar la demo completa el día anterior con la lista de `reparto.md`; `JWT_DURACION=4h`; que CA-ESP-01 no tenga estado posterior al 21/12/2021 | 30 min |
| 2 | Planteamiento (3) | «¿Qué aporta el K-Means? ¿Dónde está la predicción de PM2,5/PM10 del caso 10?» | Lámina 14 (los tres regímenes, nueva) y 29 (la señal temprana y su precisión); respuesta 5 | Que Rody la ensaye: el régimen es contexto y señal medida; la alerta es la norma; el pronóstico quedó fuera, dicho sin rodeos | 15 min |
| 3 | Interfaz (5) | «Panel administrador: métricas del cluster (uso CPU…)» | El CPU que se ve es el del proceso de la API (el coordinador); el de los nodos no viaja por el protocolo | Decirlo así (lámina 23, paso 4 de la demo, respuesta 24). Mostrarlo de verdad pide cambiar el protocolo: no para este plazo | — |
| 4 | Todos | Que la exposición entre en el tiempo | 14:55 más 3:50 de demo; versión corta de unos 12 + 3:15 | Ensayo con reloj y decidir antes qué versión se usa | 30 min |
| 5 | Planteamiento (3) | Leer el diagrama de arquitectura | Lámina 8: proyectado, el texto de las cajas no se lee | Alt + clic (Ctrl + clic en Linux) amplía; las píldoras dicen los puertos y la lámina 9 cuenta los flujos | — |
| 6 | Todos («conocimiento total») | Que cada uno responda sin leer | 29 preguntas con su `archivo:línea` y quién responde, más las de reserva | Que cada integrante lea las suyas y abra una vez el código que citan | 1 h por persona |

## Matriz de trazabilidad

| Criterio | Pts | Vocabulario del docente (fuente) | Dónde está | ¿Con sus palabras? | Estado |
|:---|---:|:---|:---|:---:|:---|
| Planteamiento: problema, dataset y pregunta de impacto | 3 | OEFA, >1M, pregunta, ECA, para quién [Rúbrica; Enunciado §4] | Láminas 2 y 3 (con los valores del ECA); pregunta 1 | sí | cubierto |
| Planteamiento: diagrama de arquitectura | 3 | «diagrama de arquitectura», puertos, Docker Compose [Rúbrica] | Láminas 8 y 9; pregunta 2 | sí | cubierto (diagrama denso: riesgo 5) |
| Implementación: carga concurrente de >1M | 8 | goroutines, canales, workers, `WaitGroup` [Rúbrica; Enunciado A; sem. 10] | Láminas 6 y 11; pregunta 6 | sí | cubierto |
| Implementación: entrenamiento paralelo sin condiciones de carrera | 8 | worker pool, barrera, `Mutex`, «condiciones de carrera» [Rúbrica; Enunciado B] | Láminas 12 a 14; preguntas 7 y 8 | sí | cubierto |
| Implementación: distribución por sockets y puertos | 8 | `net.Listen`, `Accept`, `net.Dial`, puerto, `bufio`, JSON, goroutine por conexión [Enunciado: «Sockets y patrones»; sem. 10–14] | Láminas 16 y 17; preguntas 10 a 16 | sí | cubierto |
| Implementación: Docker | 8 | Dockerfile, `docker compose`, imagen, `healthcheck` [Rúbrica: «utiliza docker»; sem. 11] | Lámina 8 (el Dockerfile nombrado), respaldo 33 (el Dockerfile en dos etapas, servicios, puertos, `healthcheck`), paso 6 de la demo; preguntas 22 y 23 | sí | cubierto |
| Implementación: API REST, tiempo real, métricas, < 100 ms | 8 | `net/http`, JWT, `POST /api/predicciones`, p99, WebSocket [Rúbrica; Enunciado C] | Láminas 19 y 20; preguntas 17, 19 y 20 | sí | cubierto |
| Flujo: la API coordina el clúster; la predicción no pasa por él | 8 | El camino del pedido hasta los nodos [Enunciado §6; observación de la PC4] | Lámina 9; pregunta 18 | sí | cubierto |
| Implícito: MongoDB persistente y Redis caché | — | «Persistente: MongoDB», «Cache: Redis» [Enunciado §6] | Lámina 21; pregunta 21 | sí | cubierto |
| Interfaz: framework, menú, parámetros, tiempo real, impacto | 5 | React, «menú de opciones», configura desde la UI, tiempo real [Rúbrica] | Láminas 23 y 24 (demo), 28 y 29; pregunta 24 | sí | cubierto |
| Interfaz: módulos del enunciado | 5 | ingreso, dashboard, entrenamiento y simulación, panel admin (CPU, latencia, nodos, logs) [Enunciado §6] | Lámina 23, demo (paso 1, el ingreso), respaldo 41 | sí | cubierto, salvo el CPU de los nodos (riesgo 3) |
| Entregable 3: seguridad | 5 | CORS, JWT, políticas [Enunciado] | Lámina 25, paso 8 de la demo; preguntas 25 y 26 | sí | cubierto |
| Entregable 3: evaluación experimental | 5 | pruebas, datos de prueba, responde la pregunta de impacto [Enunciado] | Láminas 27 a 29; preguntas 19, 27 y 28 | sí | cubierto |
| Implícito: Git Flow y Kanban | 2 | Git Flow, Kanban [Enunciado; Rúbrica: Informe] | No están en el deck (son del informe); `preguntas.md`, «Proceso» | sí | débil, a propósito |
| Ética (ABET 4) | — | lo que el dato no puede decir | Láminas 28 a 30 | sí | cubierto |

## Afirmaciones

`afirmaciones.py` sobre el PDF, las notas, `preguntas.md` y `reparto.md`: la única ALTA es la
analogía que el docente marcó en la PC4, citada a propósito en «Lo que no se dice» con su reemplazo.
Las MEDIA y las que marcó el corrector en frío quedaron así:

| Frase de antes (dónde) | Problema | Ahora |
|:---|:---|:---|
| «2 883 877 lecturas horarias» (lámina 2) | Son filas: hay promedios diarios y filas vacías | «filas de lecturas de 33 estaciones» |
| «Es la pregunta que el enunciado pone de ejemplo para el caso 10» (3) | El ejemplo es general | «El enunciado pone una pregunta así entre sus ejemplos de impacto social» |
| «la memoria no crece con el archivo» (11) | El pivot sí necesita las filas válidas | «a lo sumo 2W bloques leídos sin fusionar» |
| «no comparten nada» (notas de 11) | Comparten canales y el archivo | «no comparten estado mutable» |
| «la fusión en orden y las fases seriales (8,3 %) no se reparten» (11) | Con 8,3 %, Amdahl daría más de 6× | Karp-Flatt (13 %) y los pocos bloques de 64 MiB para 12 workers; en Windows, bloques más chicos tardan menos |
| «darían números distintos» (notas de 12) | Absoluto | «podrían dar» |
| «bit a bit para todo W» (13) | Vale con el mismo chunk | «… y el mismo chunk» |
| «Una iteración dura 2,03 ms» frente a 0,551 ms (13 y respaldo 39) | Parecía una contradicción | «2,03 ms con 1 worker y 0,551 ms con 12» |
| «Sin condiciones de carrera: 549 funciones» (13 y 27) | El detector no prueba la ausencia | «`go test -race` no detecta carreras»; «ninguna detectada en 549 funciones» |
| «sin permisos de administrador: solo conexiones de salida» (16) | Escuchar no pide admin; lo que bloquea es el firewall | «Colab está detrás de NAT y el firewall de las PC del laboratorio bloquea las entrantes» |
| «la barrera de una ronda mide 0,515 ms» (16, 27, 39) | El smoke es de `PING`, sin cálculo | «una ronda de `PING` a todos a la vez, sin cálculo: lo que la barrera paga en red» |
| «la misma suma que en una máquina» (17) | Es la misma función; agrupar por nodo cambia el orden | «la misma función»; «en el orden de 10⁻¹⁵ … porque agrupar por nodo cambia el orden de la suma» |
| «a lo sumo 1,3 × 10⁻¹⁵» frente a 7,7 × 10⁻¹⁵ (17, 27, respaldo 41) | Dos corridas distintas sin decirlo | «a lo sumo 1,3 × 10⁻¹⁵ en `cmd/distribuido`»; la captura, «7,7 × 10⁻¹⁵ … dentro de 10⁻⁹» |
| «Distribuido = secuencial» (27) y el cierre (31) | Entre nodos difiere a 10⁻¹⁵ | «frente a»; «en una máquina, bit a bit; entre nodos, a 10⁻¹⁵» |
| «El 7070 no se publica» y los nodos del laboratorio (notas de 8, 16) | ¿Cómo llegaron? | En el laboratorio el coordinador corrió fuera de Docker; en Compose, el 7070 no está en `ports` |
| «con `expose` queda dentro de la red» (pregunta 11) | `expose` es documentación | «no está en `ports`» |
| «Puertos solo en 127.0.0.1» (25) | En la máquina pública están el 80 y el 443 | «En el Compose…; en la máquina pública, solo 80 y 443» |
| «MongoDB y Redis guardaron cada predicción» (notas de 20) | La persistencia es de mejor esfuerzo; 80 000 frente a 80 800 | «en esa corrida, ninguna descartada»; 80 800 = 80 000 pedidos y 800 de calentamiento |
| «el punto de control de cada ronda» (21) | Nunca se lee | «los centroides de la última ronda; todavía no se reanuda desde ahí» |
| «sigue cada ronda en vivo» (23, demo) | La UI las reproduce, y su pantalla lo dice | «las rondas llegan por el WebSocket; la UI las reproduce» |
| «clúster (nodos, CPU…)» (23) | Es el CPU de la API | «CPU del proceso de la API» (y el de los nodos no viaja) |
| «no hay modelo: uno con menos nodos sería otro modelo» (respaldo 34) | Reasignando el rango sería el mismo | «no reasignamos el rango del nodo caído» |
| «Auditoría independiente» (27) | La hizo el mismo equipo | «Otra implementación, en pandas y scikit-learn» |
| «Todo en Go y en contenedores: … y la SPA» (7) | La SPA es TypeScript; Mongo y Redis no son Go | «En contenedores: el sistema en Go (…), MongoDB, Redis y la SPA en React» |

## Coherencia con el informe del TB2

- Las 107 claves de `cifras.yml` que usa el deck son idénticas en la rama del informe; las de
  impacto de `_deck.yml` coinciden con `cifras.impacto` (los cuartiles van ahora con una decimal,
  como allá).
- La pregunta de impacto social tiene la redacción del informe, y la anticipación dice «régimen
  alto», como el informe y la pantalla de impacto.
- Pruebas del frontend (708 en 36 archivos) y WebSocket (p50 2,25 ms, p99 9,98 ms): iguales.
- **Para el informe** (no es de esta carpeta): su tabla de pruebas del TB2 cuenta 587 pruebas de
  la interfaz (`cifras.pruebas_tb2.web`, los `it`/`test` declarados) y su texto dice 708 (las que
  corre Vitest). Y el diagrama `docs/diagramas/arquitectura.png` dice que MongoDB guarda «corridas,
  centroides, regímenes» y Redis «predicciones, centroides por ronda», distinto de las 5 colecciones
  y 3 claves reales.

## Corrector en frío

Un subagente nuevo hizo de docente con la rúbrica, el enunciado, los labs, el PDF, lo que se dice y
`preguntas.md`, sin ver el repositorio. Leyó la versión anterior a las correcciones de la primera
pasada (por eso marcó la numeración corrida y el Dockerfile ausente, ya resueltos). Su puntaje
simulado: Planteamiento 2,5/3, Implementación 6,5/8 y Interfaz 4/5 (13/16), condicionado a la demo.
Lo que aportó y no estaba en la matriz, con su destino:

| Observación | Qué se hizo |
|:---|:---|
| El resultado del modelo no se ve en la exposición | Lámina 14 nueva: los tres regímenes con las líneas del ECA |
| La demo, donde se gana la Interfaz, iba al final y se podía cortar | Movida a la lámina 24, después de la SPA; suma el paso de ingreso |
| Tres contradicciones (7070, 1,3 frente a 7,7 × 10⁻¹⁵, 2,03 frente a 0,551 ms) | Corregidas (tabla de afirmaciones) |
| El ECA nunca se proyecta con sus valores; INCA y régimen 0/1/2 sin leyenda | Nota del ECA en la lámina 3 y sus líneas en la 14; leyenda de INCA y régimen en la 23 |
| ¿Qué aporta el K-Means? ¿Y la predicción del caso 10? | Respuesta 5 reescrita; queda como riesgo 2 |
| Preguntas sin respuesta preparada: el impacto en Python, `go func` antes de Go 1.22, el orden de los candados, cómo ven los workers los centroides nuevos, el cluster vacío, el apretón del WebSocket | Agregadas a las respuestas 7, 8 y 13, a «Proceso» y a «De reserva» |
| No ofrecer por iniciativa propia el experimento de 20 GPU en Python | Quitado de «Proceso» |
| «Error de hecho» sobre el Kanban de la PC4, dicho en la sala | Reescrito: si sale, se muestra la tarjeta #76, sin discutir |
| La UI usa formato numérico inglés («1,184», «7.7 e-15») | Es del frontend: anotado para esa sesión |
| Replicar el gold no es ML masivo; el clúster no acelera con la tabla real | Se mantiene, dicho con honestidad en el respaldo 39 y la respuesta 29 |
| «20 h» como titular de la anticipación | Se mantiene, con la cota baja (1 h), el tope de 23 h y «no es un pronóstico» en la misma lámina |

## Lectura humana (la cierra una persona, no la IA)

- [ ] Leí el deck renderizado (`bash slides/render.sh`, pantalla completa) contra esta matriz.
  Quién: ______ Fecha: ______
- [ ] Cada integrante leyó sus láminas, sus notas y sus preguntas.
- [ ] Ensayo con reloj y la demo completa, con la lista de `reparto.md`.
- [ ] El anexo de uso de IA del informe declara también esta revisión.
