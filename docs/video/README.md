# Videos de la PC3 y del TB2

Los dos videos se arman solos con scripts: nada se graba a mano salvo, en el del TB2, la voz y la cara
de cada integrante.

## Video del TB2 (Entregable 3)

Seis minutos como máximo, con la presentación de la solución, su construcción y su funcionamiento de
punta a punta, y con los tres integrantes. Doce bloques que suman **5:30**: Rody 2:05, Dayana 1:40 y
Julio 1:45.

| Archivo | Qué es |
|---|---|
| [`guion-tb2.md`](guion-tb2.md) | Los bloques cronometrados: quién habla, qué se ve y el texto exacto. Es la fuente del montaje. |
| [`grabar-tu-parte.md`](grabar-tu-parte.md) | Para los integrantes: qué clip graba cada uno, cómo, en qué formato y con qué nombre. |
| [`mensaje-grupo-tb2.md`](mensaje-grupo-tb2.md) | El mensaje para el grupo de WhatsApp (borrador; lo envía Rody). |
| `armar-tb2.sh` | Monta el mp4: verifica herramientas, renderiza lo que falta y comprueba duración y formato. |
| `montar_tb2.py` | El montador que llama `armar-tb2.sh` (Pillow + ffmpeg + edge-tts para los borradores). |
| `grabar-pantalla.sh`, `pantalla/` | Graba la SPA con Playwright y el Chrome del sistema contra un Compose propio y limpio. |
| `grabar.sh carga bases pruebas compose` | Graba la terminal del TB2 con asciinema contra ese mismo Compose. |
| `remotion/` | Las escenas animadas: `Arquitectura` (bloque 02), `PipelineCargador` (03) y `RondaDistribuida` (04). |

### Cómo se arma

```bash
# 1. Lo que se ve (una vez; hace falta el .env del video, ver abajo, y el gold en GOLD_DIR)
bash docs/video/grabar-pantalla.sh                      # Compose limpio y la SPA → build/tb2/pantalla/*.mp4
bash docs/video/grabar.sh carga bases pruebas compose   # la terminal → casts/*.cast

# 2. Los clips de cada uno, con el nombre de la tabla de grabar-tu-parte.md
rclone copy gdrive:aire-distribuido/video/tb2/clips docs/video/clips/

# 3. El montaje
bash docs/video/armar-tb2.sh --tiempos   # cuánto dura cada bloque con los clips que hay
bash docs/video/armar-tb2.sh             # borrador: lo que falta va con voz sintética y la marca BORRADOR
bash docs/video/armar-tb2.sh --final     # exige los 12 clips y 6:00 o menos → build/tb2/video-tb2.mp4
bash docs/video/armar-tb2.sh --hojas     # build/tb2/guion-NOMBRE.pdf: los textos de cada uno en letra grande
```

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
