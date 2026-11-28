#!/usr/bin/env bash
# Renderiza un deck: datos.py (las series de D3 y la portada, desde reports/) y quarto render.
# Deja un solo archivo HTML que se abre sin servidor y sin red.
#
#   bash render.sh              # el deck del TB2 (index.qmd) → _site/index.html
#   bash render.sh pc4          # el de la PC4 (pc4.qmd)      → _site/pc4.html
#   bash render.sh [pc4] --pdf  # además, el PDF (_site/deck.pdf o _site/pc4.pdf): respaldo para
#                               # imprimir o si falla el navegador
set -euo pipefail
cd "$(dirname "$0")"

deck=index
pdf=no
for arg in "$@"; do
  case "$arg" in
    --pdf) pdf=si ;;
    pc4) deck=pc4 ;;
    *) echo "error: argumento desconocido '$arg' (se aceptan pc4 y --pdf)" >&2; exit 1 ;;
  esac
done

command -v quarto >/dev/null 2>&1 || { echo "error: falta quarto (https://quarto.org)" >&2; exit 1; }

python3 datos.py
quarto render "$deck.qmd"

if [ "$pdf" = si ]; then
  chrome="$(command -v google-chrome || command -v chromium || command -v chromium-browser || true)"
  [ -n "$chrome" ] || { echo "error: --pdf necesita Chrome o Chromium" >&2; exit 1; }
  salida="$PWD/_site/deck.pdf"
  [ "$deck" = index ] || salida="$PWD/_site/$deck.pdf"
  # ?print-pdf: reveal pone una lámina por página y los gráficos dibujan su estado final
  "$chrome" --headless=new --disable-gpu --no-sandbox --hide-scrollbars --window-size=1280,720 \
    --virtual-time-budget=15000 --run-all-compositor-stages-before-draw --no-pdf-header-footer \
    --print-to-pdf="$salida" "file://$PWD/_site/$deck.html?print-pdf" 2>/dev/null
  echo "PDF en $salida"
fi
echo "deck en $PWD/_site/$deck.html"
