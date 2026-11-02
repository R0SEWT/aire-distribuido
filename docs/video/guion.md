# Guion del video de la PC3 (Entregable 1)

Caso 10, *Monitoreo de Calidad del Aire y Sensores Ambientales*. Duración objetivo: entre 6 y 8
minutos. Sigue el orden del informe (`docs/informe/informe.qmd`): caso y pregunta, datos y
limpieza, arquitectura del cargador, K-Means y regímenes, benchmark y determinismo, diseño
distribuido, pruebas, CI, Docker y Kanban.

**Cómo se usa este archivo.** `armar.sh` lo lee tal cual: cada `## Escena` es un clip. La línea
`Pantalla:` dice qué se ve (una carátula, una figura del informe, una grabación real de la
terminal o un texto) y el bloque `narración` es el texto exacto que la voz lee. Si el docente
exige voz propia, Rody graba cada bloque con su micrófono, guarda el audio como
`docs/video/voz/escena-NN.wav` (o `.mp3`) y vuelve a correr `armar.sh`: el montaje usa esa pista
en vez de la sintética. Nada de música.

**Cifras.** Todas salen de `docs/informe/generado/cifras.yml` y de `reports/` (la clave está
entre corchetes la primera vez que se usa). Ninguna está inventada ni redondeada más allá de lo
que se dice de viva voz.

**Convenciones de pantalla**

- `caratula`: tarjeta de título con curso, caso y nombres del equipo.
- `figura RUTA | pie [| crop=x,y,ancho,alto]`: una figura del informe o un diagrama, con un pie;
  `crop` recorta un área del PNG original para que el texto se lea a 1080p.
- `figuras RUTA1 + RUTA2 | pie`: dos figuras lado a lado.
- `terminal NOMBRE | pie`: la grabación real `docs/video/casts/NOMBRE.cast` (hecha con
  `grabar.sh`), reproducida a una velocidad que cabe en la narración.
- `texto Título | línea ; línea ; …`: una tarjeta de texto.

---

## Escena 1 · Carátula

- Pantalla: `caratula`
- Qué se ve: título del trabajo, curso, caso, integrantes y el enlace del repositorio.

```narración
Este es el video del Entregable 1 del Trabajo Final de Programación Concurrente y Distribuida.
Caso diez: monitoreo de la calidad del aire con los datos de OEFA. Un cargador concurrente y un
K-Means paralelo, escritos en Go solo con la biblioteca estándar.
```

## Escena 2 · El caso y la pregunta de impacto social

- Pantalla: `figura docs/informe/generado/figuras/mapa.png | Estaciones de OEFA y horas válidas por estación (notebook 04 sobre el silver y el gold de Go)`
- Qué se ve: el mapa de las estaciones de OEFA con sus horas válidas.
- Cifras: 1,3 GB [limpieza.bytes], 2 883 877 filas [limpieza.filas_largas_leidas], 33 estaciones
  [limpieza.n_estaciones], 10 departamentos [limpieza.n_departamentos], 2018–2022 [limpieza.anios].

```narración
OEFA, el Organismo de Evaluación y Fiscalización Ambiental, publica las lecturas horarias de sus
estaciones de calidad del aire: un CSV de 1,3 gigabytes con 2 millones 883 mil filas, de 33
estaciones en 10 departamentos, entre 2018 y 2022. Están junto a las actividades que OEFA fiscaliza,
no repartidas por el país. La pregunta de impacto social, elegida con una
criba sobre el dato: ¿en qué estaciones y a qué horas el aire entra en el nivel
alto de PM2,5 y PM10, y con cuánta anticipación eso avisa de un episodio que supera el ECA? Y una
advertencia: una zona sin estación no es una zona con aire limpio.
```

## Escena 3 · El dato de OEFA y la limpieza

- Pantalla: `figura docs/informe/generado/figuras/faltantes.png | Destino de cada fila del archivo (primera regla que la descarta) y la regla de las 18 horas de OEFA`
- Qué se ve: a la izquierda, el destino de las filas largas por estación; a la derecha, cómo
  OEFA publica el promedio de 24 horas en cero cuando hay menos de 18 horas medidas.
- Cifras: 93 % sin hora [limpieza.pct_hora_inferida], control 100 % [eda.control_fisico],
  18 horas [eda.regla_horas_validas], 50,3 % descartado [limpieza.pct_descartadas], días en cero
  26,4 % [limpieza.pct_bloque_en_cero], filas vacías 9,1 % [limpieza.pct_sin_valor], no horarios
  7,6 % [limpieza.pct_descartadas_periodo], 236 001 anchas [limpieza.filas_anchas], 65 873
  completas [limpieza.filas_completas], 17 estaciones [limpieza.n_estaciones_gold].

```narración
El archivo viene en formato largo: cada fila trae un solo parámetro, y hay que pivotarlo. El 93
por ciento de las lecturas no trae la hora: se reconstruye por su posición dentro de cada día de 24
filas, validada donde la hora sí existe. El cero de OEFA significa
«sin dato»: si un día tiene menos de 18 horas medidas, su promedio de 24 horas se publica en cero. Se descarta la mitad del archivo: días en cero, 26 por
ciento; filas vacías de 2022, 9 por ciento; promedios no horarios, 8 por ciento. Quedan 236 mil
horas-estación y 65 873 completas con PM2,5 y PM10, de 17 estaciones.
```

## Escena 4 · El cargador en la terminal

- Pantalla: `terminal cargar | go run ./cmd/cargar sobre el CSV completo de OEFA (grabación real)`
- Qué se ve: la ejecución real de `cmd/cargar` sobre el CSV de 1,3 GB: conteos por regla, tabla
  por estación y archivos escritos.
- Cifras: 12 workers [limpieza.workers], 1,33 s [limpieza.duracion_s].

```narración
Así corre el cargador sobre el CSV completo: 2 millones 883 mil filas con 12 workers en poco más
de un segundo. Reporta cuánto descarta cada regla, en cascada y sin doble conteo: las filas leídas
son las procesadas más las descartadas. Y escribe el silver, el gold y los reportes de los que
salen las cifras del informe.
```

## Escena 5 · Arquitectura concurrente del cargador

- Pantalla: `figura docs/diagramas/arquitectura.png | Parte A del diagrama de arquitectura: lector → workers → recolector → z-score (docs/diagramas/arquitectura.mmd) | crop=120,350,2180,760`
- Qué se ve: el tramo del diagrama de arquitectura con el CSV, el lector por bloques, los
  workers, el recolector y la estandarización.
- Cifras: bloque de 64 MiB [benchmark_cargador.bloque].

```narración
El cargador es un pipeline de goroutines. Un lector parte el archivo en bloques de 64 megabytes y
manda sus índices por un canal. Cada worker lee
su bloque con ReadAt, aplica las reglas por fila y acumula en su propia memoria, sin candados. El
recolector fusiona los parciales en orden de bloque, no de llegada: por eso el resultado no depende
del número de workers. Luego una fase serial reconstruye las horas, el pivot corre en paralelo por
estación, y la estandarización usa Welford por chunk con la fusión de Chan.
```

## Escena 6 · K-Means: la receta y su calibración

- Pantalla: `figura docs/informe/generado/figuras/codo.png | Método del codo sobre reports/codo.json (Go); la receta se calibró en Colab y Go la lee de configs/kmeans.json`
- Qué se ve: la curva del codo, inercia por número de clusters de 2 a 10.
- Cifras: 1 080 corridas [calibracion.n_corridas], 24 variantes [calibracion.n_variantes],
  k = 3 [kmeans.k], ARI 0,99 [calibracion.ari_semillas], log1p: recall 0 % [calibracion.log_recall].

```narración
El modelo es el K-Means de Lloyd, portado del Trabajo Parcial. Go no decide la receta: se calibró en Colab con 1 080 corridas sobre 24 variantes y quedó
congelada en un JSON. Ganó la más simple: PM2,5 y PM10 en sus unidades, sin logaritmo, con k igual
a 3. Es estable entre semillas, no aprende la geografía de las estaciones, y el logaritmo borra justamente
los episodios. El codo confirma que en k igual a 3 la inercia
deja de caer rápido.
```

## Escena 7 · Tres regímenes frente al ECA

- Pantalla: `figura docs/informe/generado/figuras/niveles.png | PM2,5 frente a PM10 por régimen, con los valores del ECA de 24 horas (DS 003-2017-MINAM)`
- Qué se ve: la nube de horas del gold coloreada por régimen, los centroides y las líneas del ECA.
- Cifras: bajo 73,1 % (9,0 / 17,6) [bajo.*], medio 24,6 % (24,4 / 58,2) [medio.*], alto 2,3 %
  (65,1 / 186,7, excede) [alto.*], recall 44 % [calibracion.recall], fracción fina 0,35
  [alto.cociente].

```narración
Los tres clusters son niveles de aire. El bajo reúne el 73 por ciento de las horas, con medias de
9 microgramos de PM2,5 y 18 de PM10. El medio, el 25 por ciento, con 24 y 58. El alto, el 2,3 por
ciento, con 65 y 187: supera los dos valores del ECA de 24 horas, 50 y 100. Ese régimen recoge el
44 por ciento de las horas en que el promedio diario excede la norma, y su fracción fina, 0,35,
apunta a polvo, no a combustión.
```

## Escena 8 · Dónde y cuándo aparece el nivel alto

- Pantalla: `figura docs/informe/generado/figuras/regimen-hora.png | Hora del día en que aparece cada régimen (reports/regimenes.json); la distribución por estación se ve en la escena 9`
- Qué se ve: el mapa de calor por hora del día de cada régimen.
- Cifras: 45 % de mañana [alto.pct_franja], pico 08:00 [alto.hora_pico], CA-CP-01 35 %
  [alto.pct_estacion], Cusco, 3 600 m [alto.altitud], 30 % de Lima [alto.pct_lima].

```narración
El nivel alto tiene hora: el 45 por ciento de sus horas es de la mañana, con pico a las ocho. Y
tiene lugar: la estación CA-CP-01, en Cusco, pone el 35 por ciento. Un límite va a la vista: el 30
por ciento del nivel alto sale de tres estaciones de Lima que midieron solo unas semanas de
diciembre de 2021. Eso no es la exposición de un año.
```

## Escena 9 · El entrenamiento en la terminal

- Pantalla: `terminal entrenar | go run ./cmd/entrenar -codo 2:10 sobre el gold real (grabación real)`
- Qué se ve: la ejecución real de `cmd/entrenar`: el codo, las diez semillas, el perfil de cada
  régimen con su etiqueta ECA, y las horas y estaciones donde aparece.
- Cifras: 10 semillas [kmeans.semillas], 44 iteraciones [kmeans.iteraciones], 10 cerca del
  mínimo [kmeans.semillas_cerca_del_minimo].

```narración
El comando entrenar lee el gold y la receta, corre diez semillas de k-means++ en paralelo y se
queda con la de menor inercia; las diez llegan al mismo mínimo. Imprime el perfil de cada régimen,
su etiqueta frente al ECA, y las horas y estaciones donde aparece.
```

## Escena 10 · Paralelismo, speedup y determinismo

- Pantalla: `figura docs/informe/generado/figuras/speedup.png | Speedup del cargador y del K-Means frente al número de workers (reports/benchmark_*.json)`
- Qué se ve: las dos curvas de speedup con sus intervalos bootstrap.
- Cifras: 4,94 [benchmark_cargador.speedup], IC [4,66; 5,21] [benchmark_cargador.speedup_ic],
  3,86 [benchmark_kmeans.speedup], Karp-Flatt 13 % y 19 % [*.karp_flatt].

```narración
El K-Means paralelo usa un worker pool persistente: en cada iteración los centroides son de solo
lectura, cada worker escribe el parcial de su chunk, y la única sincronización es una barrera con
WaitGroup. La reducción suma en orden de chunk, así que el resultado es idéntico bit a bit para
cualquier número de workers. El cargador logra un speedup de 4,94 con 12 workers; el K-Means, 3,86. Karp y Flatt dan una fracción serial del 13 y del 19
por ciento: lo que no escala es el orden, y ese es el precio del determinismo.
```

## Escena 11 · El benchmark en la terminal

- Pantalla: `terminal benchmark | go run ./cmd/benchmark con 1, 2, 4, 8 y 12 workers y 3 repeticiones (grabación real)`
- Qué se ve: la tabla del benchmark del K-Means y la verificación de identidad bit a bit.

```narración
El benchmark mide el K-Means secuencial y concurrente con 1, 2, 4, 8 y 12 workers y dos tamaños
de chunk, con medias recortadas e intervalos bootstrap. Y al final verifica lo central: la inercia
es idéntica bit a bit con cualquier número de workers.
```

## Escena 12 · Diseño distribuido para la PC4

- Pantalla: `figura docs/diagramas/secuencia-iteracion.png | Una iteración del K-Means distribuido: ITERAR, PARCIAL, la barrera y el plazo (docs/diagramas/secuencia-iteracion.mmd) | crop=60,640,2000,1190`
- Qué se ve: el tramo del diagrama de secuencia con el bucle de iteraciones, la barrera, el plazo
  vencido y el parcial rezagado.

```narración
La versión distribuida de la PC4 ejecuta el mismo código: solo cambia quién llama a Parciales, un
worker sobre un chunk o un nodo sobre su partición. El nodo se conecta al coordinador, nunca al
revés. Cada nodo carga su rango una sola vez; por la red solo
viajan centroides y parciales, en JSON por línea sobre TCP. Cada iteración es una barrera a través
de la red: el coordinador manda ITERAR, espera los K parciales, fusiona en orden de nodo y
actualiza. Si un nodo no responde, vence el plazo con un error claro, y un parcial rezagado se
descarta por su número de ronda. El protocolo ya está escrito y probado.
```

## Escena 13 · Pruebas con el detector de carreras

- Pantalla: `terminal tests | go test -race -count=1 ./... sobre todo el módulo (grabación real)`
- Qué se ve: la ejecución real de las pruebas de todos los paquetes con `-race`.

```narración
Toda prueba corre con el detector de carreras. Las pruebas fijan tres equivalencias: el mismo
dataset con 1 y con 8 workers, los mismos centroides para cualquier número de workers, y la
conservación de filas. Y en el protocolo, la fusión en orden de nodo y el plazo que vence.
```

## Escena 14 · Integración continua y Git Flow

- Pantalla: `figura docs/informe/capturas/ci.png | GitHub Actions: go test -race, go vet y gofmt por push y por pull request`
- Qué se ve: la captura de las corridas de CI en verde y las pull requests fusionadas.

```narración
La integración continua en GitHub Actions corre go test con race, go vet y gofmt en cada push y
en cada pull request. El trabajo sigue Git Flow: ramas feature, pull requests hacia develop y una
rama release por entrega.
```

## Escena 15 · Docker

- Pantalla: `terminal docker | docker build y docker run de la imagen con la muestra versionada (grabación real)`
- Qué se ve: la construcción real de la imagen y una corrida de `cargar` dentro del contenedor.

```narración
El Dockerfile compila los tres comandos como binarios estáticos sobre una imagen distroless, sin
shell, de unos pocos megabytes. Los datos se montan como volumen. Aquí la imagen carga la muestra
versionada con dos workers. El Docker Compose completo llega con la PC4.
```

## Escena 16 · Tablero Kanban

- Pantalla: `figura docs/informe/capturas/kanban.png | Tablero Kanban de la PC3 en GitHub Projects`
- Qué se ve: la captura del tablero con las tarjetas de la PC3.

```narración
Las tareas se siguieron en un tablero Kanban de GitHub Projects, con una tarjeta por tarea del
plan y por pull request, movidas durante la semana.
```

## Escena 17 · Cierre

- Pantalla: `texto Lo que deja la PC3 | Carga concurrente: 2 883 877 filas, cada descarte contado, speedup 4,94 con 12 workers ; K-Means paralelo: idéntico bit a bit para cualquier número de workers, speedup 3,86 ; Auditoría en pandas y scikit-learn: 40 de 40 chequeos iguales ; Tres niveles de aire: el alto (2,3 %) excede el ECA, de mañana y en pocas estaciones ; Pendiente para el TB2: con cuánta anticipación avisa cada régimen ; El modelo ve 17 estaciones: donde no hay estación, no hay dato`
- Cifras: 40 de 40 [auditoria.ok, auditoria.chequeos].

```narración
La carga concurrente cumple el millón de registros y cuenta cada fila que descarta; el
K-Means paralelo es reproducible bit a bit, y una auditoría en pandas y scikit-learn reproduce 40
de 40 chequeos; y el aire se agrupa en tres niveles, donde el alto dice dónde y cuándo. Falta la
segunda mitad de la pregunta, con cuánta anticipación avisa cada régimen, que se mide en el TB2. Y
la advertencia de siempre: donde no hay estación, no hay dato.
```
