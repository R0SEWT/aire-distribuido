# Video del TB2: qué graba cada uno

El video dura 5 minutos y medio y lo arma un script (`armar-tb2.sh`): las animaciones, la interfaz y la
terminal ya están grabadas. **Lo único que falta son nuestras voces y caras**: cada uno graba un clip
corto por bloque, leyendo el texto del guion a la cámara. El montaje pone la cámara en un círculo sobre
lo que se ve, recorta los silencios del inicio y del final y nivela el volumen.

El texto exacto de cada bloque está en [`guion-tb2.md`](guion-tb2.md) (el bloque ```` ```dice ````).
Se puede decir con palabras propias, pero **las cifras van tal cual**.

**Plazo: sábado 28/11 a las 12:00.** Con eso Rody monta en la tarde y todos revisan el video antes de
entregarlo el domingo.

## La lista

| Archivo | Quién | Bloque | Duración | Cámara |
|---|---|---|---|---|
| `01-rody.mp4` | Rody | Quiénes somos y qué pregunta respondemos | 0:23 | completa |
| `02-rody.mp4` | Rody | Arquitectura | 0:20 | círculo |
| `03-dayana.mp4` | Dayana | El dato de OEFA y el cargador concurrente | 0:40 | círculo |
| `04-rody.mp4` | Rody | El clúster: K-Means distribuido por sockets TCP | 0:40 | círculo |
| `05-julio.mp4` | Julio | La API coordinadora | 0:35 | círculo |
| `06-dayana.mp4` | Dayana | MongoDB y Redis | 0:25 | círculo |
| `07-julio.mp4` | Julio | Demo: ingreso, escenario y tiempo real | 0:27 | círculo |
| `08-julio.mp4` | Julio | Demo: simular un día | 0:20 | círculo |
| `09-rody.mp4` | Rody | Demo: entrenamiento y clúster desde la UI | 0:25 | círculo |
| `10-dayana.mp4` | Dayana | Impacto social | 0:40 | círculo |
| `11-julio.mp4` | Julio | Pruebas y despliegue | 0:25 | círculo |
| `12-rody.mp4` | Rody | Cierre | 0:15 | completa |

Por persona: **Dayana**, 3 clips (1:45 en total); **Julio**, 4 clips (1:47); **Rody**, 5 clips (2:03).
Cada clip se graba completo de una vez: son cortos, y si alguien se equivoca, repite el bloque entero.

## Cómo grabar

1. **Cámara horizontal**, a la altura de los ojos, con la cara y los hombros en el cuadro. La laptop
   sirve (Windows: app *Cámara*; Mac: *Photo Booth* o *QuickTime*; Linux: *Cheese* u OBS), y el celular
   también (modo video, horizontal, 1080p a 30 fps).
2. **Luz de frente**: la ventana o la lámpara delante de ti, nunca detrás.
3. **Silencio**: sin música, televisor ni ventilador cerca. El micrófono cerca de la boca: los audífonos
   con micrófono del celular suenan mejor que el de la laptop.
4. **El texto al lado de la cámara**: ábrelo en una ventana pegada a la webcam, o en otro celular apoyado
   debajo. Mira a la cámara lo más que puedas; no hace falta memorizar.
5. **Graba así**: dale a grabar, espera un segundo en silencio, lee el bloque a ritmo de conversación,
   espera un segundo y corta. No edites nada: el montaje recorta solo.
6. **Duración**: más o menos la de la tabla. Si te pasas por más de 5 segundos, vuelve a grabarlo un poco
   más suelto (el video completo no puede pasar de 6 minutos).

Cómo se dicen algunas cosas:

| Se escribe | Se dice |
|---|---|
| PM2,5 y PM10 | pe eme dos coma cinco, pe eme diez |
| ECA | e-ca (el estándar de calidad ambiental) |
| net.Listen, net.Dial | net punto listen, net punto dial |
| JSON, JWT | yeison, jota doble ve te |
| `-race` | guion race |
| aire.rosewt.dev | aire punto rosewt punto dev |
| 1,3 gigabytes; 2 883 877 | uno coma tres gigabytes; 2 millones 883 mil |
| 64 748 | sesenta y cuatro mil setecientos cuarenta y ocho |

## Formato y nombre

- **Video**: mp4, mov, webm o mkv; 1080p o 720p; 30 fps. La cámara es la que demuestra que participamos
  todos, así que va con cara.
- **Si de verdad no se puede video**: solo audio (m4a, mp3, ogg u opus; una nota de voz de WhatsApp
  sirve). El montaje pone un círculo con tus iniciales, pero es el último recurso.
- **Nombre**: el de la tabla, `NN-nombre` y la extensión que salga (`03-dayana.mp4`, `07-julio.mov`).
  Si grabas dos tomas, manda la buena; si mandas las dos, `03-dayana-toma2.mp4`.

## Cómo mandarlo

- **Por la carpeta de Drive del video** que comparte Rody (`aire-distribuido/video/tb2/clips/`), o
- **por WhatsApp como *Documento***, no como video: como video, WhatsApp lo comprime y se ve borroso.

## Qué hace Rody con los clips

```bash
rclone copy gdrive:aire-distribuido/video/tb2/clips docs/video/clips/   # o los copia a mano
bash docs/video/armar-tb2.sh                 # monta con lo que haya; lo que falte sale con voz sintética
bash docs/video/armar-tb2.sh --final         # exige los 12 clips y que el video dure 6 min o menos
```

El borrador lleva la marca BORRADOR en los bloques con voz sintética: sirve para ver el ritmo y el
encuadre antes de que lleguen todos los clips. El video final (`docs/video/build/tb2/video-tb2.mp4`) se
comparte en el grupo para que todos lo revisen antes de subirlo.
