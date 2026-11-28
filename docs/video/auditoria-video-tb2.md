# Auditoría del video del TB2 · 1ACC0065 · 2026-11-28

Revisado antes de grabar, con la skill `revisar-entrega` (modo A): `guion-tb2.md`, los textos de
`mensaje-grupo-tb2.md`, las escenas de Remotion (`Arquitectura`, `PipelineCargador`, `RondaDistribuida`) y
las grabaciones automáticas (`build/tb2/pantalla/*.mp4`, `casts/{carga,bases,pruebas,compose}.cast`). El
video todavía no existe: falta grabar las voces.

Fuentes: rúbrica del Aula (`Rubrica_Logro_1ACC0065_202620.xlsx`) ✓ · enunciado del TB2 ✓ · observaciones de
la PC4 (`references/caso-pc4.md`) ✓ · labs de las semanas 10 a 14 ✓ · informe del TB2 (rama
`claude/tb2-informe-065625`, e0eb784) ✓ · corrector en frío (subagente nuevo, sin el repo) ✓.

## Veredicto

**Listo con riesgos** para mandar el mensaje y grabar: cada frase que dicen los tres es verdadera al pie de
la letra contra el código y los JSON, y dice lo mismo que el informe. Los riesgos que quedan son de
alcance, no de exactitud (abajo).

## Afirmaciones corregidas

| Bloque | Antes | Problema | Ahora |
|:--|:--|:--|:--|
| 04 Rody | «Un WaitGroup hace de barrera, con un plazo» | Es la forma de la analogía que el docente marcó «Incorrecto» en la PC4, y un `WaitGroup` no tiene plazo | «el coordinador lanza una goroutine por nodo, que manda ITERAR y espera el PARCIAL con un plazo; un WaitGroup espera a todas» (`protocolo.Ronda`: `esperar(Parcial, iter, limite)`) |
| 04 Rody | «cada nodo se conecta con net.Dial» | El nodo usa `net.Dialer.DialContext` | «cada nodo se conecta con un net.Dialer»; la escena dice `net.Dialer → api:7070` |
| 04 Rody | «lee su rango del gold una sola vez» | Es una vez por entrenamiento, y el corrector preguntó si entonces los datos viajan | «lee su rango de su propia copia del gold: por la red solo viajan centroides y parciales» |
| 04 Rody | «sale el mismo modelo que el secuencial» | No es idéntico bit a bit con más de un nodo (1,3 × 10⁻¹⁵) | No se narra: la tabla lo muestra con la cifra |
| 04 escena | Tabla con «sus rondas (ms)» | De 3 a 4 nodos empeora (13,5 → 13,9) y el video no tiene tiempo de explicarlo | Sin esa columna; el subtítulo dice «nodos como procesos locales en una misma máquina» |
| 05 Julio | (no decía qué pasa con la predicción y el clúster) | En la PC4 el docente marcó esa ausencia | «Una lectura entra por POST y no pasa por el clúster: la API usa el modelo activo en memoria» |
| 06 Dayana | «entraron 80 800 escrituras sin perder ninguna» | Son 80 800 predicciones (80 000 medidas + 800 de calentamiento) escritas en cada base | «llegaron las 80 800 predicciones, contando el calentamiento, sin descartar ninguna» (igual que el informe, §13) |
| 03 Dayana | «un pool de 12 workers … en 1,3 segundos» | La captura en pantalla dice 14 workers y 1,319 s | «un pool de workers … en menos de un segundo y medio» |
| 03 Dayana | (no explicaba por qué la pantalla dice «descartadas 1 449 379») | El corrector lo marcó | «La mitad de las filas se descarta, sobre todo ceros que en OEFA significan “sin dato”», sobre la captura real de `cmd/cargar` |
| 02 Rody | «Docker Compose levanta todo» | El cargador no está en el Compose | Se quitó; la escena saca al cargador del marco del Compose |
| 02 Rody | (no decía qué es un régimen) | Planteamiento: nadie decía qué agrupa el K-Means | «un K-Means que agrupa cada hora, por su PM2,5 y PM10, en tres regímenes: bajo, medio y alto» |
| 07 Julio | Solo narraba clics | El corrector: «narra clics», sin conocimiento | «un hub recibe por canales las altas, las bajas y los eventos, y los reparte a cada cliente del WebSocket» (`api/hub.go`) |
| 10 Dayana | «pero seguida, apenas una hora» | No se entiende con una sola escucha | «ya había pasado el umbral una mediana de 20 horas antes; pero se prende y se apaga, y seguida llevaba apenas una hora» |
| 10 Dayana | «17 de las 33 estaciones miden por hora» | Las otras 16 pueden medir otras cosas por hora | «miden PM por hora» |
| 11 Julio | «Todo se prueba con go test y -race» | La interfaz se prueba con Vitest | «Las pruebas de Go corren con el detector de carreras» |
| 11 Julio | «4,8 veces más rápido» sobre la captura de Windows | La captura muestra 5,36 en medianas y es la salida de un script de Python | Sin la cifra ni la captura: «en Windows comprobé que el cargador da el mismo SHA-256 que en Linux» |

Cifras que quedan y de dónde salen: 2 883 877 filas y 1,3 GB (`limpieza.json`); 64 MiB por bloque
(`cmd/cargar`, `-bloque 64MB`); SHA-256 iguales en Linux, Windows y macOS (`benchmark_cargador.json`,
`benchmark_cargador_windows.json`, `revision_dayana/ejecucion.json`); puerto 7070 (`cmd/api`, `-nodos-addr`);
80 000 pedidos y p99 de 3,14 ms en el informe y 5,93 ms en el cast, «no pasó de 6» (`api_latencia.json` y
`casts/carga.cast`); 80 800 predicciones escritas (`api_latencia.json`); 48 horas del 14 y 15 de diciembre
de 2021 (`series.json`); 18 lecturas, ventana de 24 h; 64 748 y 1 184 horas, 30 episodios, medianas de 20 h
y de 1 h, 17 de 33 estaciones (`impacto.json`, igual que `cifras.impacto` del informe).

**Después de la #115** (textos de Impacto, 28/11): el bloque 10 usa las palabras de la pantalla corregida:
«horas-estación» (la misma hora en dos estaciones cuenta dos veces), la población «como contexto», «los 30
episodios sobre el ECA» (un episodio agrupa superaciones; no es cada cruce) y «donde no se mide, no se puede
afirmar que el aire esté limpio». El grabador busca el título nuevo, «Dónde podría servir esta información».

## Matriz: lo que el enunciado pide del video

| Pedido (fuente) | Dónde está | Estado |
|:--|:--|:--|
| 6 minutos como máximo (enunciado, rúbrica) | 5:37 con voz sintética; `armar-tb2.sh --final` falla si pasa de 6:00 | cubierto, con riesgo (abajo, 4) |
| Participan todos (enunciado) | Rody 2:08, Dayana 1:45, Julio 1:41, cada uno a cámara | cubierto |
| Presentación de la solución | 01 (pregunta) y 02 (arquitectura con los regímenes) | cubierto |
| Puntos importantes de la construcción, con el vocabulario de los labs | 03 canal, pool de workers, sin candados; 04 `net.Listen`, `Accept`, `net.Dialer`, `json.Marshal`, `bufio.Reader`, goroutine por nodo, `WaitGroup`; 06 canal con búfer y goroutine por base; 07 hub con canales | cubierto |
| Funcionamiento de punta a punta | 07 ingreso y escenario por WebSocket, 08 simulación, 09 entrenamiento y clúster, 10 impacto, 11 Compose | cubierto |
| API coordinadora del clúster; qué pasa con una predicción (enunciado; PC4) | 04 (entrenamiento) y 05 («no pasa por el clúster») | cubierto |
| Panel del administrador: CPU, latencia, nodos, logs (enunciado) | 09: la pantalla de Clúster | cubierto, breve |
| Parámetros desde la UI (rúbrica Interfaz) | 08 estación y día; 09 nodos del entrenamiento | cubierto |
| Impacto social (rúbrica Interfaz, enunciado) | 10 | cubierto |
| Docker Compose (instrucciones generales) | 02 escena y 11 | cubierto |
| Seguridad de la API: JWT, roles, CSP (Entregable 3) | 05 «token JWT por rol»; no se ve un 401/403 ni la CSP | débil |

## Riesgos que quedan (mayor a menor)

1. **El clúster se ve como animación y como repetición en la UI, no corriendo en varias máquinas.**
   Los nodos son procesos o contenedores en una misma máquina (lo dice la tabla del bloque 04). Mostrar
   el clúster en varias máquinas exigiría el laboratorio, que no se hizo (`cifras.distribuido.lab: no`).
   El video no afirma lo contrario.
2. **Con 65 873 filas, distribuir el K-Means no acelera mucho** (el corrector lo preguntaría). La
   respuesta corta está en el informe; el video no la da por tiempo.
3. **No se ve una política de seguridad en acción** (un 403 de un usuario en Clúster, la CSP). Las capturas
   están en `docs/informe/capturas/tb2/seguridad/`. Si sobra tiempo, 5 segundos en el bloque 09.
4. **La duración depende del ritmo de cada uno.** Con la voz sintética son 5:37; leyendo más lento se puede
   acercar a 6:00. El montaje avisa sobre 5:45 y `--final` no deja pasar 6:00. Si pasa, se recorta el bloque
   02 o el 10.
5. **En pantalla aparecen cosas que no se narran**: «INCA moderada» con «En alerta» en Simulación, `"excede_eca":false`
   con `"alerta":true` en Redis. Son correctas (la alerta es la media de 24 h, no el régimen), pero el
   docente puede preguntar en la sustentación.
6. **El orden de la demo** predice (07–08) antes de entrenar (09): no se ve que un modelo nuevo cambie una
   predicción.

## Corrector en frío

Un subagente nuevo, sin el repo, leyó la rúbrica, el enunciado, los labs y la transcripción del guion
(antes de estas correcciones). Puntuaría Video 1,5/2, Planteamiento 2/3, Implementación 6/8 e Interfaz
3,5/5. De sus observaciones se atendieron las de la tabla de arriba. Quedan como riesgo las de los puntos 1,
2, 3, 5 y 6, más dos de diseño que no son del video: «Simular escribe estado compartido» en un sitio
publicado, y que el formulario de entrenamiento muestra «Réplicas» y «Medición larga» sin explicarlas.

La auditoría la hizo una IA (Claude); la firma una persona: alguien del equipo mira el borrador
(`build/tb2/video-tb2-borrador.mp4`) contra esta lista antes de entregar.
