#!/usr/bin/env bash
# Revisa en exodia los clips de los integrantes contra guion-tb2.md (duración, imagen, nivel, silencios y
# las cifras de la transcripción) y, con --video, transcribe el mp4 montado bloque por bloque.
#
#   bash docs/video/revisar-clips.sh               # clips de docs/video/clips/ → build/tb2/revision/revision-clips.md
#   bash docs/video/revisar-clips.sh --video       # además transcribe el mp4 de build/tb2/salida.txt
#   bash docs/video/revisar-clips.sh --solo-video  # solo la transcripción del mp4 → transcripcion-video.md
#   CLIPS=otra/carpeta WHISPER_MODELO=medium bash docs/video/revisar-clips.sh
#
# Todo corre en exodia con exodia-ff (skill ffmpeg-exodia): ffprobe y ffmpeg para medir, y faster-whisper
# para transcribir, en un entorno efímero de uv. En esta máquina no se instala ni se codifica nada. La caché
# de uv y del modelo queda en exodia:~/Code/exprimental/temp/cache-whisper/, para no bajar el modelo en cada
# ronda (se puede borrar cuando ya no haga falta).
set -euo pipefail
cd "$(dirname "$0")/../.."

CLIPS=${CLIPS:-docs/video/clips}
BUILD=docs/video/build/tb2
CON_CLIPS=1
CON_VIDEO=0
for a in "$@"; do
  case "$a" in
    --video) CON_VIDEO=1 ;;
    --solo-video) CON_VIDEO=1; CON_CLIPS=0 ;;
    *) echo "uso: $0 [--video|--solo-video]" >&2; exit 2 ;;
  esac
done
command -v exodia-ff >/dev/null 2>&1 || { echo "falta exodia-ff (skill ffmpeg-exodia)" >&2; exit 1; }

P=$BUILD/revision/paquete
rm -rf "$P"
mkdir -p "$P/clips"
poner() { ln -f "$1" "$2" 2>/dev/null || cp -p "$1" "$2"; }   # enlace duro si se puede: no copia los videos
poner docs/video/revisar_clips.py "$P/revisar_clips.py"
poner docs/video/guion-tb2.md "$P/guion-tb2.md"
args="--guion guion-tb2.md --salida ../../out"
if [ "$CON_CLIPS" = 1 ]; then
  n=0
  for f in "$CLIPS"/*; do
    [ -f "$f" ] || continue
    poner "$f" "$P/clips/$(basename "$f")"
    n=$((n + 1))
  done
  [ "$n" -gt 0 ] || { echo "no hay clips en $CLIPS" >&2; exit 1; }
  echo "$n archivos de $CLIPS"
  args+=" --clips clips"
fi
if [ "$CON_VIDEO" = 1 ]; then
  v=$(cat "$BUILD/salida.txt" 2>/dev/null || true)
  [ -f "$v" ] && [ -f "$BUILD/bloques.json" ] || { echo "no hay un mp4 montado (bash docs/video/armar-tb2.sh)" >&2; exit 1; }
  poner "$v" "$P/$(basename "$v")"
  poner "$BUILD/bloques.json" "$P/bloques.json"
  args+=" --video $(basename "$v") --bloques bloques.json"
fi

mkdir -p "$BUILD/revision"
exodia-ff run -w -n revisar-clips --in "$P" -f "$BUILD/revision" --sh "cd in/paquete && \
export UV_CACHE_DIR=\$HOME/Code/exprimental/temp/cache-whisper/uv HF_HOME=\$HOME/Code/exprimental/temp/cache-whisper/hf \
WHISPER_MODELO=${WHISPER_MODELO:-small} && \
uv run --quiet --no-project --python /usr/bin/python3 --with faster-whisper python revisar_clips.py $args"
for f in revision-clips.md transcripcion-video.md; do
  [ -f "$BUILD/revision/$f" ] && echo "→ $BUILD/revision/$f"
done
