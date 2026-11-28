#!/usr/bin/env bash
# Arma el video del TB2 (Entregable 3) desde docs/video/guion-tb2.md y los clips de voz y cámara de cada
# integrante en docs/video/clips/ (NN-nombre.mp4; ver grabar-tu-parte.md):
#
#   bash docs/video/armar-tb2.sh             # monta con lo que haya: un clip que falta va con voz sintética
#                                            # y la marca BORRADOR → build/tb2/video-tb2-borrador.mp4
#   bash docs/video/armar-tb2.sh --final     # exige los 12 clips y 6 minutos o menos → build/tb2/video-tb2.mp4
#   bash docs/video/armar-tb2.sh --tiempos   # solo cuánto dura cada bloque, sin montar
#   bash docs/video/armar-tb2.sh --hojas     # un PDF por persona con sus textos en letra grande
#   bash docs/video/armar-tb2.sh --verificar # solo revisa el mp4 ya montado
#   bash docs/video/armar-tb2.sh --local     # todo en esta máquina (sin exodia); se combina con lo de arriba
#
# Dónde corre. Si está `exodia-ff` (skill ffmpeg-exodia), nada se codifica con ffmpeg en esta máquina:
#   1. Aquí, `montar_tb2.py --preparar` genera las voces sintéticas que falten (edge-tts), las escenas de
#      Remotion (traen su propio ffmpeg) y los GIF de la terminal (agg), y arma el paquete
#      build/tb2/exodia/repo con la estructura del repo: código, guion, clips, grabaciones y figuras.
#   2. `exodia-ff run` lo sube a exodia:~/Code/exprimental/temp/<JOB>/in/, corre ahí el montaje completo
#      en tmux con su ffmpeg (INSUMOS_PREVIOS=1) y baja el mp4, bloques.json y los fotogramas a build/tb2/.
#   Sin exodia-ff, o con --local, todo corre aquí como antes.
#
# Lo que se ve sale de grabaciones automáticas que se hacen antes, una vez:
#   - docs/video/build/tb2/pantalla/*.mp4: la SPA de punta a punta (bash docs/video/grabar-pantalla.sh).
#   - docs/video/casts/*.cast: la terminal (bash docs/video/grabar.sh carga bases pruebas compose).
#   - docs/video/remotion/: las escenas animadas; el montaje las renderiza a la duración de cada voz.
# Los clips de los integrantes se bajan de Drive con:
#   rclone copy gdrive:aire-distribuido/video/tb2/clips docs/video/clips/
set -euo pipefail
cd "$(dirname "$0")/../.."

VERIFICAR=0
LOCAL=0
ARGS=()
for a in "$@"; do
  case "$a" in
    --verificar) VERIFICAR=1 ;;
    --local) LOCAL=1 ;;
    --final|--tiempos|--hojas) ARGS+=("$a") ;;
    *) echo "uso: $0 [--final|--tiempos|--hojas|--verificar] [--local]" >&2; exit 2 ;;
  esac
done
tiene() { for a in "${ARGS[@]}"; do [ "$a" = "$1" ] && return 0; done; return 1; }
falta() { ! command -v "$1" >/dev/null 2>&1; }
command -v exodia-ff >/dev/null 2>&1 || LOCAL=1

if falta uv; then echo "falta uv: curl -LsSf https://astral.sh/uv/install.sh | sh" >&2; exit 1; fi
if falta agg; then uv tool install agg 2>/dev/null || cargo install --git https://github.com/asciinema/agg; fi
if { falta ffprobe && falta static_ffprobe; } || { [ "$LOCAL" = 1 ] && falta ffmpeg && falta static_ffmpeg; }; then
  uv tool install static-ffmpeg
fi
export FFMPEG=$(command -v ffmpeg || command -v static_ffmpeg || echo ffmpeg)
export FFPROBE=$(command -v ffprobe || command -v static_ffprobe)   # aquí solo lee metadatos
export AGG=$(command -v agg)
if falta npx; then echo "falta Node 22 (npx) para las escenas de Remotion" >&2; exit 1; fi

BUILD=docs/video/build/tb2
mkdir -p "$BUILD"
if tiene --hojas; then
  uv run --quiet --with pillow python docs/video/montar_tb2.py --hojas   # solo Pillow
  exit 0
fi

if [ "$VERIFICAR" = 0 ]; then
  python3 docs/video/remotion/datos.py >/dev/null   # las cifras de las escenas, desde reports/
  if [ "$LOCAL" = 1 ]; then
    uv run --quiet --with pillow python docs/video/montar_tb2.py "${ARGS[@]}"
    tiene --tiempos && exit 0
  else
    echo "Preparando en esta máquina (voces, Remotion, GIF y el paquete para exodia)"
    preparar=(--preparar)
    tiene --final && preparar+=(--final)
    uv run --quiet --with pillow python docs/video/montar_tb2.py "${preparar[@]}"
    remoto="INSUMOS_PREVIOS=1 FUENTES=fuentes FFMPEG=ffmpeg FFPROBE=ffprobe python3 docs/video/montar_tb2.py ${ARGS[*]}"
    if tiene --tiempos; then
      exodia-ff run -w -n video-tb2-tiempos --in "$BUILD/exodia/repo" --sh "cd in/repo && $remoto"
      exit 0
    fi
    # El mp4, el resumen y un fotograma cada 20 s se bajan a build/tb2/ (los fotogramas también se
    # sacan en exodia: aquí no se escribe nada con ffmpeg).
    rm -f "$BUILD"/video-tb2*.mp4 "$BUILD/fotogramas"/*.png
    exodia-ff run -w -n video-tb2 --in "$BUILD/exodia/repo" -f "$BUILD" --sh "cd in/repo && $remoto && \
s=\$(cat docs/video/build/tb2/salida.txt) && cp \"\$s\" docs/video/build/tb2/bloques.json ../../out/ && \
mkdir -p ../../out/fotogramas && ffmpeg -nostdin -y -loglevel error -i \"\$s\" -vf fps=1/20 ../../out/fotogramas/s-%03d.png && \
basename \"\$s\" > ../../out/salida.txt"
    echo "$BUILD/$(cat "$BUILD/salida.txt")" > "$BUILD/salida.txt"
  fi
fi

SALIDA=$(cat "$BUILD/salida.txt" 2>/dev/null || echo "$BUILD/video-tb2.mp4")
[ -f "$SALIDA" ] || { echo "no hay video montado todavía ($SALIDA)" >&2; exit 1; }
echo
echo "Verificación de $SALIDA"
dur=$("$FFPROBE" -v error -show_entries format=duration -of csv=p=0 "$SALIDA")
printf '  duración: %s s (%s min)\n' "${dur%.*}" "$(awk "BEGIN{printf \"%.2f\", $dur/60}")"
awk "BEGIN{exit !($dur <= 360)}" && echo "  6 minutos o menos: sí" || echo "  6 minutos o menos: NO"
mb=$(( $(stat -c %s "$SALIDA") / 1000000 ))
echo "  tamaño: $mb MB"
"$FFPROBE" -v error -select_streams v:0 -show_entries stream=codec_name,width,height,r_frame_rate -of csv=p=0 "$SALIDA" | sed 's/^/  video: /'
"$FFPROBE" -v error -select_streams a:0 -show_entries stream=codec_name,sample_rate,channels -of csv=p=0 "$SALIDA" | sed 's/^/  audio: /'
if [ "$LOCAL" = 1 ] && [ "$VERIFICAR" = 0 ]; then
  mkdir -p "$BUILD/fotogramas"
  rm -f "$BUILD/fotogramas"/*.png
  "$FFMPEG" -y -loglevel error -i "$SALIDA" -vf "fps=1/20,scale=1920:1080" "$BUILD/fotogramas/s-%03d.png"
fi
echo "  fotogramas (uno cada 20 s): $BUILD/fotogramas/"
case "$SALIDA" in *borrador*) echo "  ES UN BORRADOR: faltan clips (ver build/tb2/bloques.json)" ;; esac
