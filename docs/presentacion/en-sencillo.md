# La PC3 en sencillo

Marco de la exposición, antes de pensar en láminas. Sigue el método de la charla de SIMBig 2026:
primero la historia en palabras simples, después el deck, y ninguna cifra escrita a mano.

Borrador hecho con IA (Claude) a partir del informe de la PC3 y de `reports/`. Antes de exponer,
el equipo lo revisa y cada uno valida la parte que le toca decir.

## Para qué es

El enunciado pide dos cosas que se apoyan en este deck:

- **La sustentación de la semana 15.** Cada grupo presenta el funcionamiento del sistema y
  responde las preguntas del docente. La rúbrica premia «la exposición de su trabajo demostrando
  conocimiento total del tema», el dataset de más de un millón de registros, el diagrama de
  arquitectura y la programación concurrente y distribuida.
- **El video del TB2.** Seis minutos como máximo, con la participación de todos los integrantes.

Por eso el deck dura unos 9 minutos, se reparte entre los tres y trae láminas de respaldo para
las preguntas. Hoy cuenta la PC3. Cuando lleguen la PC4 (clúster, API) y el TB2 (frontend), se
reemplazan las láminas de «qué sigue» por las de esas partes y se recorta a 6 minutos para el
video.

## En una frase

OEFA publica 2,9 millones de lecturas horarias del aire junto a minas, fundiciones y otras
actividades que fiscaliza. Las cargamos en paralelo en Go, limpiamos lo que el archivo esconde
(la hora que falta, el 0 que significa «no medido») y agrupamos las horas con un K-Means
paralelo: **el aire cae en tres niveles, y el alto aparece de mañana y en pocas estaciones**.

## El arco

Primero el problema que cualquiera entiende, después el dato tal como es y recién ahí la
concurrencia, que es lo que evalúa el curso. Cada parte la dice quien la hizo, según el reporte de
participación; el reparto es una propuesta.

| Parte | Quién | Qué se dice | Tiempo |
|---|---|---|---|
| Portada y apertura | Rody | El aire que se mide junto a una mina. 2,9 millones de lecturas, 33 estaciones | 0:45 |
| La pregunta | Rody | En qué estaciones y a qué horas el aire entra en un nivel alto, y con cuánta anticipación avisa. La PC3 responde el dónde y el cuándo | 0:30 |
| Dónde mide OEFA | Rody | El mapa. Una zona sin estación no es una zona con aire limpio | 0:25 |
| El CSV de verdad | Dayana | El 93 % de las lecturas no trae hora y se reconstruye por posición. Un 0 es «no medido». El embudo: de 2,9 millones de filas a 65 873 horas completas | 1:15 |
| Arquitectura | Rody | Las cuatro partes y qué entra en la PC3 | 0:35 |
| Parte A: el cargador | Julio | Productor, W workers, recolector en orden. 4,94× con 12 workers y el mismo silver para todo W | 1:15 |
| Parte B: el K-Means | Rody | La animación del worker pool: chunks por un canal, la barrera, la fusión en orden. Bit a bit para todo W y 3,86× | 1:30 |
| Hacia la PC4 | Rody | El mismo código en un nodo; la ronda por la red ya está probada | 0:30 |
| Lo que encontramos | Dayana | Tres niveles de aire. El alto: 2,3 % de las horas, de mañana, encabezado por Cusco. No aísla los episodios: la alerta necesita el nivel y el ECA juntos | 1:20 |
| Cómo sabemos que está bien | Julio | `go test -race`, CI, auditoría en scikit-learn 40 de 40, Docker | 0:25 |
| Cierre | Rody | Qué aprendimos y qué sigue | 0:30 |

Son 9 minutos según los tiempos de las notas: Rody 4:45, Dayana 2:35 y Julio 1:40. Si hay
que recortar, la primera en salir es «Hacia la PC4».

## Lo que no se dice

- **«Predecimos episodios».** No: el K-Means describe niveles. El nivel alto recoge el 44 % de las
  horas en que el promedio de 24 horas supera el ECA. La anticipación se mide en el TB2.
- **«Donde no hay estación el aire está bien».** OEFA mide donde fiscaliza, no donde vive la gente.
- **«Una hora sobre 50 µg/m³ incumple el ECA».** El ECA es un promedio de 24 horas.
- **«Escala lineal»**. 4,94× con 12 workers es un 41 % de eficiencia, y se explica por las fases
  seriales y por lo poco que dura cada iteración del K-Means.
- **«Ya usamos puertos TCP»**. Las pruebas del protocolo corren sobre `net.Pipe`, una conexión en
  memoria. El listener TCP y el clúster son la PC4.

## Preguntas probables del docente

| Pregunta | Respuesta corta | Lámina de respaldo |
|---|---|---|
| ¿Por qué no usan `Mutex`? | Cada worker escribe solo el parcial de su chunk; los centroides son de solo lectura durante la iteración. La única sincronización es la barrera | Parte B |
| ¿Por qué el speedup no llega a 12? | Fases seriales (8,3 % del tiempo del cargador), fusión en orden y una iteración de 2 ms en el K-Means: el costo de coordinar pesa | Speedup y Karp-Flatt |
| ¿Cómo saben que el resultado no depende de W? | La reducción suma en orden de bloque o de chunk, nunca en el de llegada; la suma en coma flotante no es asociativa. Las pruebas comparan bit a bit | Parte B |
| ¿Dónde están los canales? ¿Y los puertos? | Canales: `trabajos`, `resultados`, el semáforo `enVuelo`. Puertos: el protocolo JSON sobre TCP está diseñado y probado sobre `net.Pipe`; el listener es la PC4 | Protocolo |
| ¿Más de un millón de registros? | 2 883 877 filas leídas, 1 434 498 lecturas válidas | El CSV de verdad |
| ¿Por qué un 0 es faltante? | OEFA marca con 0 lo que no midió: días enteros en 0 y la regla de las 18 horas de su promedio diario | Reglas de limpieza |
| ¿Por qué k = 3? | 1 080 corridas de calibración: la silueta más alta de la grilla con k ≥ 3 (0,54) y estable entre semillas (ARI 0,99) | Calibración |
| ¿Cómo validan el K-Means? | Un notebook rehace la limpieza y el K-Means en pandas y scikit-learn: 40 de 40 chequeos sin diferencias | Evidencias |
| ¿Qué pasa si un nodo se cae? | Cada ronda tiene un plazo; vence con un error que nombra al nodo y el operador relanza. Reasignar el rango queda como extensión | Protocolo |

## Cifras y de dónde salen

Ninguna está escrita a mano en el deck. El texto las toma de `docs/informe/generado/cifras.yml`
(las mismas del informe) y los gráficos de D3, de `reports/*.json` a través de
`slides/datos.py`. Este documento las repite para leerlo de corrido; si una cambia, manda el deck.
