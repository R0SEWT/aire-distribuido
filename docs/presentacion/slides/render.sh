#!/usr/bin/env bash
# Renderiza el deck: datos.py (las series de D3 y la portada, desde reports/) y quarto render.
# Deja _site/index.html, un solo archivo que se abre sin servidor y sin red.
#
#   bash render.sh          # el deck
#   bash render.sh --pdf    # además, _site/deck.pdf (respaldo para imprimir o si falla el navegador)
set -euo pipefail
cd "$(dirname "$0")"

command -v quarto >/dev/null 2>&1 || { echo "error: falta quarto (https://quarto.org)" >&2; exit 1; }

python3 datos.py
quarto render index.qmd

if [ "${1:-}" = "--pdf" ]; then
  chrome="$(command -v google-chrome || command -v chromium || command -v chromium-browser || true)"
  [ -n "$chrome" ] || { echo "error: --pdf necesita Chrome o Chromium" >&2; exit 1; }
  # ?print-pdf: reveal pone una lámina por página y los gráficos dibujan su estado final
  "$chrome" --headless=new --disable-gpu --no-sandbox --hide-scrollbars --window-size=1280,720 \
    --virtual-time-budget=15000 --run-all-compositor-stages-before-draw --no-pdf-header-footer \
    --print-to-pdf="$PWD/_site/deck.pdf" "file://$PWD/_site/index.html?print-pdf" 2>/dev/null
  echo "PDF en $PWD/_site/deck.pdf"
fi
echo "deck en $PWD/_site/index.html"
