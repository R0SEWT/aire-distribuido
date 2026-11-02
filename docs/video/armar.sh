#!/usr/bin/env bash
# Arma el video de la PC3 de principio a fin, sin grabar pantalla a mano:
#
#   bash docs/video/armar.sh              # monta docs/video/build/video-pc3.mp4
#   bash docs/video/armar.sh --grabar     # además vuelve a grabar las escenas de terminal
#   bash docs/video/armar.sh --verificar  # solo comprueba el mp4 ya montado y saca fotogramas
#
# Qué usa y de dónde sale:
#   - guion.md: el texto exacto de cada escena y qué se ve (ver su cabecera).
#   - casts/*.cast: grabaciones reales de la terminal (grabar.sh, asciinema).
#   - edge-tts (vía uvx): narración sintética en español; VOZ=es-PE-CamilaNeural la cambia y
#     RITMO=+0% la deja a velocidad natural (por defecto +15%).
#     Si existe docs/video/voz/escena-NN.wav|mp3, se usa esa pista (voz propia) en su lugar.
#   - agg: .cast → GIF;  ffmpeg/ffprobe: montaje;  Pillow (vía uv run): diapositivas.
#   - docs/informe/equipo.local.yml (no versionado): nombres de la carátula; si falta, «Equipo
#     del caso 10».
#
# Instala lo que falte en ~/.local/bin con uv (asciinema, agg, static-ffmpeg para ffprobe).
set -euo pipefail
cd "$(dirname "$0")/../.."

GRABAR=0; SOLO_VERIFICAR=0
for a in "$@"; do
  case "$a" in
    --grabar) GRABAR=1 ;;
    --verificar) SOLO_VERIFICAR=1 ;;
    *) echo "uso: $0 [--grabar] [--verificar]" >&2; exit 2 ;;
  esac
done

falta() { ! command -v "$1" >/dev/null 2>&1; }
if falta uv; then echo "falta uv: curl -LsSf https://astral.sh/uv/install.sh | sh" >&2; exit 1; fi
if falta asciinema; then uv tool install asciinema; fi
if falta agg; then uv tool install agg 2>/dev/null || cargo install --git https://github.com/asciinema/agg; fi
if falta ffmpeg || falta ffprobe; then uv tool install static-ffmpeg; fi
export FFMPEG=$(command -v ffmpeg || command -v static_ffmpeg)
export FFPROBE=$(command -v ffprobe || command -v static_ffprobe)
export AGG=$(command -v agg)
export VOZ=${VOZ:-es-PE-AlexNeural}
export EQUIPO=${EQUIPO:-docs/informe/equipo.local.yml}

SALIDA=docs/video/build/video-pc3.mp4

if [ "$SOLO_VERIFICAR" = 0 ]; then
  if [ "$GRABAR" = 1 ]; then
    bash docs/video/grabar.sh
    git checkout -- reports/   # las corridas reescriben los reportes versionados
  fi
  echo "Montando $SALIDA"
  uv run --quiet --with pillow python docs/video/montar.py
fi

# Verificación: duración, audio en cada escena, y fotogramas para revisar a ojo.
echo
echo "Verificación"
dur=$("$FFPROBE" -v error -show_entries format=duration -of csv=p=0 "$SALIDA")
printf '  duración: %s s (%s min)\n' "${dur%.*}" "$(awk "BEGIN{printf \"%.1f\", $dur/60}")"
awk "BEGIN{exit !($dur >= 360 && $dur <= 480)}" && echo "  entre 6 y 8 min: sí" || echo "  entre 6 y 8 min: NO"
printf '  tamaño: %s\n' "$(du -h "$SALIDA" | cut -f1)"
"$FFPROBE" -v error -select_streams v:0 -show_entries stream=codec_name,width,height,r_frame_rate -of csv=p=0 "$SALIDA" | sed 's/^/  video: /'
"$FFPROBE" -v error -select_streams a:0 -show_entries stream=codec_name,sample_rate,channels -of csv=p=0 "$SALIDA" | sed 's/^/  audio: /'
sin_audio=0
for clip in docs/video/build/escena-*.mp4; do
  if [ "$("$FFPROBE" -v error -select_streams a -show_entries stream=codec_name -of csv=p=0 "$clip")" != "aac" ]; then
    echo "  SIN AUDIO: $clip"; sin_audio=1
  fi
done
[ "$sin_audio" = 0 ] && echo "  audio AAC en todas las escenas: sí"
mkdir -p docs/video/build/fotogramas
rm -f docs/video/build/fotogramas/*.png
"$FFMPEG" -y -loglevel error -i "$SALIDA" -vf "fps=1/60,scale=1920:1080" docs/video/build/fotogramas/min-%02d.png
echo "  fotogramas (uno por minuto): docs/video/build/fotogramas/"
