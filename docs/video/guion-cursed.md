# Guion de la variante cursed (no es la entrega)

Versión para reírse y compartir. **No reemplaza al video serio** (`guion.md`): mismas cifras reales
del informe (`docs/informe/generado/cifras.yml`, `reports/`), nada inventado sobre el proyecto, pero
narrado en tono shitpost, con voz tipo Loquendo, cortes bruscos, zoom a las cifras, imágenes
deep-fried, textos en mayúsculas con borde negro y efectos de sonido sintetizados (sin música ni
clips con copyright). Se arma con `bash docs/video/armar.sh --cursed` →
`docs/video/build/cursed/video-pc3-cursed.mp4`. Duración objetivo: 3 a 5 minutos.

Mismo formato que `guion.md` (`## Escena N · Título`, `- Pantalla:` y el bloque `narración`), más:

- `- Efectos:` una lista separada por `;`:
  - `normal`: sin deep-fry ni zooms (para el contraste cómico).
  - `sfx=boom@1.2,metralleta@4.0,airhorn@7.5,bruh@9.0`: efectos en segundos desde el inicio de la
    escena. Un `boom` o `airhorn` trae un punch-in; una `metralleta`, una sacudida de cámara.
  - `punch@t:z` y `shake@a-b`: zooms y sacudidas extra.
  - `ojos=x,y,x2,y2@a-b`: ojos rojos brillando, con rayos láser (coordenadas en la diapositiva de 1920×1080).
  - `gato@t:izq|der`: un gato con metralleta entra por ese lado, dispara y se va (cada `metralleta`
    trae uno solo). `lluvia_gatos@a-b`: seis gatos cayendo del cielo. Cada `boom` trae una explosión.
  - `velocidad=6`: velocidad de la grabación de terminal.
- `- Texto:` un rótulo `TEXTO @ a-b` (varias líneas permitidas).

Las acotaciones entre corchetes en la narración no se leen: las quita el montador.

---

## Escena 1 · Carátula

- Pantalla: `caratula`
- Efectos: `normal; sfx=boom@6.0; punch@6.0:1.9`
- Texto: `VERSIÓN CURSED @ 6.0-9.5`

```narración
Hola. Este es el video de la PC3 del caso diez. La versión seria está en el otro archivo. Esta no
es esa. [pausa] Esta es la otra.
```

## Escena 2 · OEFA y sus 33 estaciones

- Pantalla: `figura docs/informe/generado/figuras/mapa.png | 33 estaciones. Para todo el Perú.`
- Efectos: `sfx=boom@2.2,metralleta@12.5; punch@5.0:2.0; ojos=1096,610,1222,612@2.2-5.5; gato@6.5:izq`
- Texto: `33 ESTACIONES @ 2.2-4.5`
- Texto: `PARA TODO EL PERÚ @ 4.6-6.5`
- Texto: `UNA ZONA SIN ESTACIÓN NO ES UNA ZONA CON AIRE LIMPIO @ 9.0-12.5`

```narración
OEFA tiene 33 estaciones. ¿Para todo el Perú? Para todo el Perú. Treinta y tres. Están donde hay
una mina, una refinería o alguien a quien fiscalizar. Así que ojo: una zona sin estación no es una
zona con aire limpio. Es una zona sin estación. [sonido de metralleta]
```

## Escena 3 · La mitad eran cero

- Pantalla: `figura docs/informe/generado/figuras/faltantes.png | 2 883 877 filas. La mitad, cero.`
- Efectos: `sfx=boom@1.8,bruh@6.5,boom@14.5,airhorn@21.0; punch@9.5:2.2; ojos=1540,330,1700,330@14.5-17.5`
- Texto: `2 883 877 FILAS @ 0.8-2.6`
- Texto: `LA MITAD ERAN CERO @ 2.7-4.8`
- Texto: `18 HORAS O NADA @ 10.0-13.0`
- Texto: `93 % SIN HORA @ 15.0-17.5`
- Texto: `65 873 HORAS · 17 ESTACIONES @ 21.0-24.5`

```narración
Dos millones 883 mil filas. La mitad eran cero. No, en serio: el 26 por ciento del archivo son
días enteros en cero, porque para OEFA el cero significa sin dato. Si un día tiene menos de 18 horas
medidas, el promedio de 24 horas se publica en cero. Cero. Y el 93 por ciento de las lecturas viene
sin hora: la hora se deduce por posición. Validado al cien por ciento, tranquilos. Quedan 65 873
horas con PM2,5 y PM10. De 17 estaciones. [bocina]
```

## Escena 4 · El cargador, un segundo

- Pantalla: `terminal cargar | go run ./cmd/cargar a 6× (grabación real)`
- Efectos: `velocidad=6; sfx=metralleta@1.0,boom@5.5,airhorn@10.5; punch@6.0:2.0; punch@11.0:2.4`
- Texto: `1,3 GB @ 4.5-6.0`
- Texto: `1,33 SEGUNDOS @ 6.1-8.5`
- Texto: `LEÍDAS = PROCESADAS + DESCARTADAS @ 11.0-14.5`

```narración
El cargador. [metralleta] 12 goroutines. 1,3 gigabytes. Un segundo con treinta y tres. Sí, un
segundo. Cada regla cuenta lo que tira, en cascada, sin doble conteo: leídas igual a procesadas
más descartadas. Matemática.
```

## Escena 5 · Sin candados

- Pantalla: `figura docs/diagramas/arquitectura.png | lector → workers → recolector → z-score | crop=120,350,2180,760`
- Efectos: `sfx=boom@2.0,metralleta@8.0,boom@13.5; punch@2.0:1.8; punch@13.5:2.1`
- Texto: `SIN MUTEX @ 4.5-6.5`
- Texto: `ORDEN DE BLOQUE, NO DE LLEGADA @ 8.0-11.0`
- Texto: `WELFORD + CHAN @ 13.5-16.0`

```narración
Bloques de 64 megas por un canal. Workers sin candados, cada uno con su memoria. Un recolector que
fusiona en orden de bloque, no de llegada: determinismo. El pivot en paralelo por estación. Y
Welford con Chan para el z-score, porque sumar flotantes en cualquier orden es pecado.
```

## Escena 6 · k igual a tres

- Pantalla: `figura docs/informe/generado/figuras/codo.png | 1 080 corridas de calibración para elegir k = 3`
- Efectos: `sfx=boom@1.2,bruh@7.5,airhorn@12.5; punch@1.2:2.0; punch@9.0:2.3; shake@12.5-14.0`
- Texto: `1 080 CORRIDAS @ 1.2-3.5`
- Texto: `k = 3 @ 7.5-9.5`
- Texto: `EL LOGARITMO BORRABA LOS EPISODIOS @ 12.5-15.5`

```narración
El K-Means. Mil ochenta corridas de calibración en Colab para elegir la receta. Ganó la más simple:
PM2,5 y PM10, sin logaritmo, k igual a tres, porque el equipo dijo que no se complicaran. ¿Y el
logaritmo? Borraba los episodios. Literalmente los borraba. [bocina]
```

## Escena 7 · El nivel alto

- Pantalla: `figura docs/informe/generado/figuras/niveles.png | 2,3 % de las horas. Excede el ECA. A las ocho.`
- Efectos: `sfx=boom@3.0,boom@7.5,metralleta@13.0,bruh@18.0; ojos=1462,345,1510,345@7.5-11.5; punch@7.5:2.4`
- Texto: `73 · 25 · 2,3 @ 1.0-3.0`
- Texto: `EXCEDE EL ECA @ 7.5-9.5`
- Texto: `08:00 · CUSCO · 3 600 m @ 13.0-16.0`
- Texto: `ES POLVO @ 18.0-20.5`

```narración
Tres niveles. Bajo: 73 por ciento. Medio: 25. Alto: 2,3 por ciento, con 65 de PM2,5 y 187 de PM10.
Eso excede el ECA. Los dos valores. Aparece a las ocho de la mañana, sobre todo en Cusco, a 3 600
metros. Y es polvo, no combustión: fracción fina 0,35. [pausa dramática]
```

## Escena 8 · Diez de diez

- Pantalla: `terminal entrenar | go run ./cmd/entrenar a 6× (grabación real)`
- Efectos: `velocidad=6; sfx=metralleta@1.0,boom@6.0,airhorn@10.0; punch@6.0:2.2`
- Texto: `10 SEMILLAS, LA MISMA INERCIA @ 6.0-9.0`
- Texto: `SEMILLA 42, OBVIAMENTE @ 10.0-12.5`

```narración
Diez semillas de k-means++ en paralelo. Las diez llegan a la misma inercia. Diez. De diez. 44
iteraciones. Semilla 42, obviamente.
```

## Escena 9 · Bit a bit

- Pantalla: `figura docs/informe/generado/figuras/speedup.png | 4,94× el cargador, 3,86× el K-Means`
- Efectos: `sfx=boom@1.0,boom@4.5,metralleta@10.0,airhorn@15.5; punch@1.0:2.3; punch@4.5:2.3; ojos=760,300,1160,300@15.5-19.5; gato@15.5:der`
- Texto: `4,94× @ 1.0-3.5`
- Texto: `3,86× @ 4.5-6.5`
- Texto: `EL ORDEN ES EL PRECIO DEL DETERMINISMO @ 10.0-14.0`
- Texto: `IDÉNTICO BIT A BIT @ 15.5-19.5`

```narración
Speedup del cargador: 4,94 con 12 workers. K-Means: 3,86. Karp y Flatt dicen que el 13 y el 19 por
ciento es serial. Lo que no escala es el orden, y el orden es el precio del determinismo. [metralleta]
Idéntico bit a bit para cualquier número de workers. Bit. A. Bit. [bocina]
```

## Escena 10 · El benchmark lo comprueba

- Pantalla: `terminal benchmark | go run ./cmd/benchmark a 8× (grabación real)`
- Efectos: `velocidad=8; sfx=boom@1.5,metralleta@6.5; punch@2.0:2.4; shake@6.5-8.0`
- Texto: `88 CORRIDAS @ 1.5-3.5`
- Texto: `HASTA EL ÚLTIMO DECIMAL @ 6.5-9.5`

```narración
El benchmark lo comprueba: 88 corridas, cuatro configuraciones, y la misma inercia hasta el último
decimal. [metralleta]
```

## Escena 11 · Lo mismo pero por TCP

- Pantalla: `figura docs/diagramas/secuencia-iteracion.png | ITERAR, PARCIAL, barrera y plazo | crop=60,640,2000,1190`
- Efectos: `sfx=boom@2.5,metralleta@9.0,boom@14.0,bruh@17.5; punch@2.5:2.0; punch@9.0:2.2; shake@14.0-15.5`
- Texto: `EL NODO LLAMA AL COORDINADOR @ 2.5-5.5`
- Texto: `BARRERA POR ITERACIÓN @ 9.0-11.5`
- Texto: `SIN PIEDAD @ 17.5-19.5`

```narración
Para la PC4, lo mismo pero por TCP. El nodo se conecta al coordinador, nunca al revés. ITERAR,
PARCIAL, barrera, fusión en orden de nodo. Si un nodo no responde, vence el plazo y hay un error
claro. Si un parcial llega tarde, se descarta. Sin piedad.
```

## Escena 12 · Todo verde

- Pantalla: `terminal tests | go test -race a 4× (grabación real)`
- Efectos: `velocidad=4; sfx=boom@1.0,airhorn@6.0; punch@6.0:2.0`
- Texto: `TODO VERDE @ 1.0-3.0`
- Texto: `SIEMPRE CON -race @ 6.0-9.0`

```narración
go test con race. Todo verde. Siempre con race, porque sin el detector un test que pasa no dice
nada. [bocina]
```

## Escena 13 · Distroless

- Pantalla: `terminal docker | docker build y docker run a 8× (grabación real)`
- Efectos: `velocidad=8; sfx=metralleta@1.0,boom@5.0,bruh@9.0; punch@5.0:2.2; ojos=300,170,520,170@5.0-9.0`
- Texto: `SIN SHELL @ 5.0-7.0`
- Texto: `NI UN ls @ 9.0-11.0`

```narración
Docker: binarios estáticos sobre distroless, sin shell, unos pocos megas. Dentro de la imagen no
hay ni un ls. Nada. [boom]
```

## Escena 14 · Fin

- Pantalla: `texto Resumen para los que llegaron hasta aquí | Carga concurrente: 2 883 877 filas, cada descarte contado ; K-Means paralelo: idéntico bit a bit ; Tres niveles de aire, el alto excede el ECA ; Auditoría: 40 de 40 chequeos ; Lo que falta: con cuánta anticipación avisa (TB2) ; Donde no hay estación, no hay dato`
- Efectos: `sfx=boom@1.0,metralleta@8.0,airhorn@14.5,boom@18.5; punch@1.0:1.8; punch@14.5:2.4; ojos=620,178,760,178@18.5-21.5; lluvia_gatos@14.5-23.0`
- Texto: `40 DE 40 @ 8.0-10.5`
- Texto: `FIN @ 18.5-21.5`

```narración
Resumen: carga concurrente, K-Means paralelo bit a bit, tres niveles de aire y 40 de 40 chequeos
auditados en pandas y scikit-learn. Falta la anticipación, que va en el TB2. Y donde no hay
estación, no hay dato. Fin. [boom]
```
