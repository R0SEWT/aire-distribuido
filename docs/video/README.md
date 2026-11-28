# Videos de la PC3 y del TB2

Los dos videos se arman solos con scripts: nada se graba a mano salvo, en el del TB2, la voz y la cara
de cada integrante.

## Video del TB2 (Entregable 3)

Seis minutos como máximo, con la presentación de la solución, su construcción y su funcionamiento de
punta a punta, y con los tres integrantes. Doce bloques que suman **5:37**: Rody 2:08, Dayana 1:48 y
Julio 1:41.

| Archivo | Qué es |
|---|---|
| [`guion-tb2.md`](guion-tb2.md) | Los bloques cronometrados: quién habla, qué se ve y el texto exacto. Es la fuente del montaje. |
| [`grabar-tu-parte.md`](grabar-tu-parte.md) | Para los integrantes: qué clip graba cada uno, cómo, en qué formato y con qué nombre. |
| [`mensaje-grupo-tb2.md`](mensaje-grupo-tb2.md) | El mensaje para el grupo de WhatsApp (borrador; lo envía Rody). |
| `armar-tb2.sh` | Monta el mp4: verifica herramientas, renderiza lo que falta y comprueba duración y formato. |
| `montar_tb2.py` | El montador que llama `armar-tb2.sh` (Pillow + ffmpeg + edge-tts para los borradores). |
| `grabar-pantalla.sh`, `pantalla/` | Graba la SPA con Playwright y el Chrome del sistema contra un Compose propio y limpio. |
| `grabar.sh carga bases pruebas compose cargar-tb2` | Graba la terminal del TB2 con asciinema contra ese mismo Compose. |
| `remotion/` | Las escenas animadas: `Arquitectura` (bloque 02), `PipelineCargador` (03) y `RondaDistribuida` (04). |

### Cómo se arma

```bash
# 1. Lo que se ve (una vez; hace falta el .env del video, ver abajo, y el gold en GOLD_DIR)
bash docs/video/grabar-pantalla.sh                      # Compose limpio y la SPA → build/tb2/pantalla/*.mp4
bash docs/video/grabar.sh carga bases pruebas compose cargar-tb2   # la terminal → casts/*.cast

# 2. Los clips de cada uno, con el nombre de la tabla de grabar-tu-parte.md
rclone copy gdrive:aire-distribuido/video/tb2/clips docs/video/clips/

# 3. El montaje (con los 12 clips en docs/video/clips/, es este solo comando)
bash docs/video/armar-tb2.sh --final     # exige los 12 clips y 6:00 o menos → build/tb2/video-tb2.mp4

bash docs/video/armar-tb2.sh             # borrador: lo que falta va con voz sintética y la marca BORRADOR
bash docs/video/armar-tb2.sh --tiempos   # cuánto dura cada bloque con los clips que hay
bash docs/video/armar-tb2.sh --hojas     # build/tb2/guion-NOMBRE.pdf: los textos de cada uno en letra grande
```

**Si la grabación es un solo video continuo** (por ejemplo, la de la clase): no hace falta el montaje,
ese video es la entrega; basta con revisarlo contra el guion (`revisar-clips.sh` necesita los clips por
bloque, así que para un video entero se usa la transcripción: dejarlo en `build/tb2/`, apuntar
`build/tb2/salida.txt` a él y correr `revisar-clips.sh --solo-video` con un `bloques.json` de un solo bloque,
o simplemente verlo). Si se quiere sumarle las escenas, las grabaciones de la SPA y la terminal, se corta en
los doce bloques del guion con un comando por bloque en exodia, sin recodificar:

```bash
exodia-ff run -w -- ffmpeg -ss 0:00:12 -to 0:00:37 -i ~/Videos/clase.mp4 -c copy docs/video/clips/01-rody.mp4
```

(`-ss` y `-to` son el inicio y el fin de cada bloque en la grabación; el montaje recorta el silencio que
sobre). Después, `bash docs/video/armar-tb2.sh --final` como con los clips.

**Dos grabaciones pendientes, con la laptop libre** (con otra carga, las cifras salen infladas y
contradicen lo que se dice): `casts/carga.cast` es de la API en el 8090 (el p99 de 5,93 ms calza con «no
pasó de 6 milisegundos»; para que el puerto calce con el :8080 del diagrama, levantar el Compose con
`API_PORT=8080 WEB_PORT=8081 MONGO_PORT=27017 REDIS_PORT=6379` y correr
`API_VIDEO=http://localhost:8080 bash docs/video/grabar.sh carga`, y conservarla solo si el p99 de cada
nivel queda bajo 6 ms). Y el bloque 03 muestra la captura de la PC3 (14 workers, 1,319 s); para una corrida
actual con 12 workers, `bash docs/video/grabar.sh cargar-tb2` y, si la duración queda bajo 1,5 s, cambiar la
pantalla del bloque 03 por `terminal cargar-tb2 | … | parte=0.24`.

**La ronda de revisión** cuando llegan clips (la coordinadora los copia a `docs/video/clips/`):

```bash
bash docs/video/revisar-clips.sh              # cada clip contra el guion → build/tb2/revision/revision-clips.md
bash docs/video/armar-tb2.sh --final          # o sin --final mientras falten: lo que falta va con voz sintética
bash docs/video/revisar-clips.sh --solo-video # transcripción del mp4 montado, bloque por bloque
```

`revisar-clips.sh` corre en exodia (`revisar_clips.py`, con ffprobe, ffmpeg y faster-whisper «small» en un
entorno efímero de uv; la caché queda en `exodia:~/Code/exprimental/temp/cache-whisper/`). Por clip mide
la voz sin los silencios de los bordes contra la duración del guion (observación si se pasa por más de
5 s), la imagen (horizontal y 720p o más, o si es solo audio), el nivel (saturado desde -0,5 dBTP, muy
bajo bajo -35 LUFS) y los silencios, y compara la transcripción con el texto: **una cifra distinta o que
no se oye es una observación**, con el segundo y la frase; las palabras dichas de otra forma son
informativas. La transcripción automática se equivoca: antes de pedir que se regrabe, escuchar el
segundo. Lo que depende de quien grabó (una cifra, un clip vertical, audio inservible) no se arregla en
el montaje: se reporta y lo decide Rody. Después del montaje, la auditoría (`revisar-entrega`, modo A) y
el corrector en frío lee `transcripcion-docente.md` (solo lo que se oye, sin el guion) y los fotogramas
de `build/tb2/fotogramas/`; `transcripcion-video.md` pone al lado el guion y las cifras que no calzan.

**Dónde se codifica.** En la laptop de Rody no se corre ffmpeg que codifique (skill `ffmpeg-exodia`):
si está `exodia-ff`, `armar-tb2.sh` prepara aquí lo que no necesita ffmpeg (las voces sintéticas que
falten, las escenas de Remotion, que traen su propio ffmpeg, y los GIF de la terminal con agg), arma el
paquete `build/tb2/exodia/repo` con la estructura del repo y lo sube a
`exodia:~/Code/exprimental/temp/<JOB>/`. Allá `montar_tb2.py` corre completo en tmux con
`INSUMOS_PREVIOS=1` (ffmpeg 7 estático y el ffprobe del sistema, Python con Pillow y las fuentes del
paquete) y el mp4, `bloques.json` y un fotograma cada 20 s se bajan solos a `build/tb2/`.
`exodia-ff ls` muestra el trabajo y `ssh -t exodia tmux attach -t ff-<JOB>` lo deja ver en vivo. Las
escenas de Remotion se renderizan aquí a la duración estimada de cada voz y allá se ajustan a la real
(unos segundos de diferencia, sin que se note). `--local` hace todo en la máquina, como antes. Lo mismo
vale para `grabar-pantalla.sh`: los mp4 de la SPA se arman en exodia (`FFMPEG_LOCAL=1` los arma aquí).

Qué hace el montaje con cada clip: recorta el silencio del inicio y del final (`silencedetect`), limpia
la voz (pasa altos, `afftdn`) y la nivela a -16 LUFS (`loudnorm`). La duración de cada bloque es la de su
voz: las escenas de Remotion se renderizan a esa duración (sus tiempos internos se estiran en proporción,
`remotion/src/tiempo.ts`), las grabaciones de la SPA se aceleran o se frenan hasta un 40 % y después se
cortan o se congelan, y la terminal se reproduce a la velocidad que calce. La cámara va en un círculo de
300 px con el nombre y el rol al lado, o a pantalla completa (bloques 01 y 12); un video vertical de
celular se centra sobre una copia desenfocada. Un clip solo de audio lleva un círculo con las iniciales.

**El Compose del video** es el proyecto `aire-tb2-video`, en otros puertos para no chocar con uno
levantado en los de siempre. El `.env` del repo (no versionado) lleva, además de `JWT_SECRETO` y
`API_USUARIOS` con un admin y un usuario: `API_INVITADO`, `JWT_DURACION=4h`, `WEB_PORT=8091`,
`API_PORT=8090`, `MONGO_PORT=27027`, `REDIS_PORT=6389` y `GOLD_DIR` con la carpeta del gold.
`grabar-pantalla.sh` lo baja con `down -v` antes de grabar, porque el escenario se graba sobre un
sistema sin estados; `docker compose -p aire-tb2-video down -v` lo apaga al terminar. Contra el sitio
público (`AIRE_URL=https://aire.rosewt.dev`) solo se graban las escenas de lectura (impacto y swagger).

Las grabaciones de la SPA se hacen en el **modo proyector** de la UI (letra más grande), con un cursor
dibujado en la página para que se vea dónde se hace clic. `node docs/video/pantalla/explorar.mjs /ruta`
deja la captura y el árbol de accesibilidad de una pantalla para escribir los pasos de una escena nueva.

Requiere: uv, ffmpeg (o `static-ffmpeg`), agg, asciinema, Node 22, Docker, Go y Google Chrome.
En una máquina con GPU NVIDIA y un ffmpeg con `h264_nvenc`, `CODEC_VIDEO=h264_nvenc bash docs/video/armar-tb2.sh`
codifica en la GPU (lo mismo vale para `armar.sh`). Por omisión es libx264; cambiar de códec regenera los
bloques.

## Video de la PC3 (Entregable 1)

| Archivo | Qué es |
|---|---|
| `guion.md` | Las escenas: qué se ve y el texto exacto que se narra. Es la fuente del montaje. |
| `grabar.sh` | Graba con asciinema las ejecuciones reales (`cargar`, `entrenar`, `benchmark`, `go test -race`, Docker) en `casts/`. |
| `armar.sh` | Monta el mp4: diapositivas con las figuras del informe, grabaciones de terminal, narración y concatenación con ffmpeg. |
| `montar.py` | El montador que `armar.sh` ejecuta (Pillow + edge-tts + ffmpeg). `montar_tb2.py` reusa sus herramientas. |
| `casts/*.cast` | Las grabaciones de terminal (texto, pesan KB; se versionan). |
| `remotion/` | La escena animada del pipeline del cargador (`PipelineCargador`, 24 s por omisión). |
| `guion-cursed.md` | La variante shitpost (no es la entrega): mismas cifras, tono cursed, efectos y voz tipo Loquendo. |

```bash
bash docs/video/armar.sh            # → docs/video/build/video-pc3.mp4 (1080p, H.264 + AAC)
bash docs/video/armar.sh --grabar   # vuelve a grabar la terminal antes (necesita el CSV en data/bronze/)
```

`bash docs/video/armar.sh --cursed` arma la variante cursed en `docs/video/build/cursed/video-pc3-cursed.mp4`
(3 a 5 min) sin tocar la seria: voz `es-ES-AlvaroNeural` a +30 % y −10 Hz, deep-fry con Pillow,
punch-ins y sacudidas con `zoompan`, rótulos con borde negro, ojos rojos y efectos de sonido
sintetizados con ffmpeg (`aevalsrc`: boom, bocina, metralleta y «bruh»; nada con derechos, sin música).

La escena 5 del video serio es la animación de Remotion: `armar.sh` usa `remotion/out/pipeline-cargador.mp4`
si existe y, si no, la renderiza con `npx remotion render` (necesita Node 22) o avisa que falta Node.

`build/` y `clips/` no se versionan (mp4, audios, GIF, fotogramas, clips de los integrantes). El mp4
entregado de la PC3 está en Drive: `gdrive:aire-distribuido/video/video-pc3.mp4`.

`armar.sh` instala con `uv tool install` lo que falte en `~/.local/bin`: `asciinema`, `agg`
(asciicast → GIF) y `static-ffmpeg` (si no hay `ffmpeg`/`ffprobe`). La narración usa `edge-tts`
vía `uvx` (voz `es-PE-AlexNeural` a +15 % de velocidad; `VOZ=es-PE-CamilaNeural RITMO=+0% bash docs/video/armar.sh` cambia voz y ritmo) y
las diapositivas, Pillow vía `uv run --with pillow`. Si el docente exige la voz del alumno: grabar cada
bloque `narración` de `guion.md` como `docs/video/voz/escena-NN.wav` (o `.mp3`). Los nombres de la
carátula salen de `docs/informe/equipo.local.yml` (no versionado; formato de
`docs/informe/equipo.ejemplo.yml`).

Los guiones y los scripts se hicieron con ayuda de IA (Claude); el equipo revisa sus textos antes de
grabar.
