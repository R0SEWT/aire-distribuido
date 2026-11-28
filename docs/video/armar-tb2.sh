#!/usr/bin/env bash
# Arma el video del TB2 (Entregable 3) desde docs/video/guion-tb2.md y los clips de voz y cámara de cada
# integrante en docs/video/clips/ (NN-nombre.mp4; ver grabar-tu-parte.md):
#
#   bash docs/video/armar-tb2.sh             # monta con lo que haya: un clip que falta va con voz sintética
#                                            # y la marca BORRADOR → build/tb2/video-tb2-borrador.mp4
#   bash docs/video/armar-tb2.sh --final     # exige los 12 clips y 6 minutos o menos → build/tb2/video-tb2.mp4
#   bash docs/video/armar-tb2.sh --tiempos   # solo cuánto dura cada bloque, sin montar
#   bash docs/video/armar-tb2.sh --hojas     # un PDF por persona con sus textos en letra grande
#   bash docs/video/armar-tb2.sh --verificar # solo revisa el mp4 ya montado y saca fotogramas
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
ARGS=()
for a in "$@"; do
  case "$a" in
    --verificar) VERIFICAR=1 ;;
    --final|--tiempos|--hojas) ARGS+=("$a") ;;
    *) echo "uso: $0 [--final|--tiempos|--hojas|--verificar]" >&2; exit 2 ;;
  esac
done

falta() { ! command -v "$1" >/dev/null 2>&1; }
if falta uv; then echo "falta uv: curl -LsSf https://astral.sh/uv/install.sh | sh" >&2; exit 1; fi
if falta agg; then uv tool install agg 2>/dev/null || cargo install --git https://github.com/asciinema/agg; fi
if falta ffmpeg || { falta ffprobe && falta static_ffprobe; }; then uv tool install static-ffmpeg; fi
export FFMPEG=$(command -v ffmpeg || command -v static_ffmpeg)
export FFPROBE=$(command -v ffprobe || command -v static_ffprobe)
export AGG=$(command -v agg)
if falta npx; then echo "falta Node 22 (npx) para las escenas de Remotion" >&2; exit 1; fi

BUILD=docs/video/build/tb2
mkdir -p "$BUILD"
if [ "$VERIFICAR" = 0 ]; then
  python3 docs/video/remotion/datos.py >/dev/null   # las cifras de las escenas, desde reports/
  uv run --quiet --with pillow python docs/video/montar_tb2.py "${ARGS[@]}"
  for a in "${ARGS[@]}"; do case "$a" in --tiempos|--hojas) exit 0 ;; esac; done
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
mkdir -p "$BUILD/fotogramas"
rm -f "$BUILD/fotogramas"/*.png
"$FFMPEG" -y -loglevel error -i "$SALIDA" -vf "fps=1/20,scale=1920:1080" "$BUILD/fotogramas/s-%03d.png"
echo "  fotogramas (uno cada 20 s): $BUILD/fotogramas/"
case "$SALIDA" in *borrador*) echo "  ES UN BORRADOR: faltan clips (ver la tabla de arriba o build/tb2/bloques.json)" ;; esac
