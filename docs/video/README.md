# Video de la PC3

El video del Entregable 1 se arma solo, sin grabar la pantalla a mano, a partir de:

| Archivo | Qué es |
|---|---|
| `guion.md` | Las escenas: qué se ve y el texto exacto que se narra. Es la fuente del montaje. |
| `grabar.sh` | Graba con asciinema las ejecuciones reales (`cargar`, `entrenar`, `benchmark`, `go test -race`, Docker) en `casts/`. |
| `armar.sh` | Monta el mp4: diapositivas con las figuras del informe, grabaciones de terminal, narración y concatenación con ffmpeg. |
| `montar.py` | El montador que `armar.sh` ejecuta (Pillow + edge-tts + ffmpeg). |
| `casts/*.cast` | Las grabaciones de terminal (texto, pesan KB; se versionan). |
| `remotion/` | Proyecto Remotion de la escena animada del pipeline del cargador (`PipelineCargador`, 24 s). Su render `remotion/out/pipeline-cargador.mp4` no se versiona. |
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
si existe y, si no, la renderiza con `npx remotion render` (necesita Node 22; el primer render descarga el
Chrome de Remotion) o avisa que falta Node.

`build/` no se versiona (mp4, audios, GIF, fotogramas). El mp4 entregado está en Drive:
`gdrive:aire-distribuido/video/video-pc3.mp4` (`rclone copy docs/video/build/video-pc3.mp4 gdrive:aire-distribuido/video/`).

## Qué instala

`armar.sh` instala con `uv tool install` lo que falte en `~/.local/bin`: `asciinema`, `agg`
(asciicast → GIF) y `static-ffmpeg` (si no hay `ffmpeg`/`ffprobe`). La narración usa `edge-tts`
vía `uvx` (voz `es-PE-AlexNeural` a +15 % de velocidad; `VOZ=es-PE-CamilaNeural RITMO=+0% bash docs/video/armar.sh` cambia voz y ritmo) y
las diapositivas, Pillow vía `uv run --with pillow`. Hace falta Go y Docker solo para `--grabar`.

## Voz propia

Si el docente exige la voz del alumno: grabar cada bloque `narración` de `guion.md` y guardarlo
como `docs/video/voz/escena-NN.wav` (o `.mp3`). `armar.sh` usa esa pista en lugar de la
sintética para esa escena y ajusta la duración del clip. Sin música en ningún caso.

## Carátula

Los nombres salen de `docs/informe/equipo.local.yml` (no versionado; formato de
`docs/informe/equipo.ejemplo.yml`). Si no existe, la carátula dice «Equipo del caso 10».
