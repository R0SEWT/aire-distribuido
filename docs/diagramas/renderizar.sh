#!/usr/bin/env bash
# Renderiza cada .mmd de esta carpeta a un PNG con el mismo nombre.
#
# arquitectura.png y secuencia-iteracion.png NO salen de aquí: su fuente es el .drawio del mismo
# nombre (draw.io, editable; el PNG lleva el XML embebido). Se exportan con el draw.io de escritorio:
#   drawio -x -f png -e -b 12 -s 2 -o arquitectura.png arquitectura.drawio
#   drawio -x -f png -e -b 12 -s 3 -o secuencia-iteracion.png secuencia-iteracion.drawio
#
# Usa el Mermaid que trae Quarto (el mismo que dibuja los diagramas del informe)
# en una página HTML mínima, la captura con Chrome sin cabeza y recorta el
# blanco con ImageMagick. No se usa la salida docx de Quarto porque recorta los
# diagramas anchos por la derecha y por abajo.
#
# Uso: docs/diagramas/renderizar.sh
# Requiere: quarto (por su mermaid.min.js), google-chrome o chromium, magick.
set -euo pipefail

dir="$(cd "$(dirname "$0")" && pwd)"
mermaid="$(quarto --paths | sed -n 2p)/formats/html/mermaid/mermaid.min.js"
chrome="$(command -v google-chrome || command -v chromium || command -v chromium-browser)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

for mmd in "$dir"/*.mmd; do
    nombre="$(basename "$mmd" .mmd)"
    fuente="$(python3 -c 'import json, sys; print(json.dumps(open(sys.argv[1], encoding="utf-8").read()))' "$mmd")"
    cat > "$tmp/$nombre.html" <<HTML
<!doctype html>
<html><head><meta charset="utf-8">
<style>body { margin: 40px; background: #fff; } svg { max-width: none !important; }
/* halo blanco detrás de los rótulos para que las líneas de vida no los atraviesen */
svg .messageText, svg .loopText, svg .noteText, svg text.labelText { paint-order: stroke; stroke: #ffffff; stroke-width: 5px; stroke-linejoin: round; }</style>
<script src="file://$mermaid"></script>
</head><body><div id="d"></div>
<script>
  mermaid.initialize({ startOnLoad: false, securityLevel: "loose" });
  mermaid.render("g", $fuente).then(({ svg }) => {
    document.getElementById("d").innerHTML = svg;
    // Tamaño natural (el del viewBox), no el ancho de la ventana.
    const s = document.querySelector("#d svg"), vb = s.viewBox.baseVal;
    // Las líneas de vida van al fondo del SVG, para que no pasen por encima de los rótulos.
    s.querySelectorAll("line.actor-line, .actor-line").forEach((l) => s.insertBefore(l, s.firstChild));
    s.setAttribute("width", vb.width);
    s.setAttribute("height", vb.height);
  }).catch((e) => { document.body.textContent = "ERROR DE MERMAID: " + e; });
</script>
</body></html>
HTML
    "$chrome" --headless=new --disable-gpu --hide-scrollbars --allow-file-access-from-files \
        --force-device-scale-factor=2 --window-size=3000,4000 --virtual-time-budget=10000 \
        --screenshot="$tmp/$nombre-crudo.png" "file://$tmp/$nombre.html" 2>/dev/null
    # Si el dibujo llega al borde de la ventana, quedó cortado: mejor fallar.
    read -r ancho alto <<<"$(magick identify -format '%w %h' "$tmp/$nombre-crudo.png")"
    caja="$(magick "$tmp/$nombre-crudo.png" -format '%@' info:)" # AxB+X+Y del contenido
    IFS='x+' read -r cw ch cx cy <<<"$caja"
    if (( cx + cw >= ancho - 2 || cy + ch >= alto - 2 )); then
        echo "$nombre: el diagrama no entra en la ventana de captura; agranda --window-size" >&2
        exit 1
    fi
    magick "$tmp/$nombre-crudo.png" -trim +repage -bordercolor white -border 30 -strip "$dir/$nombre.png"
    echo "$dir/$nombre.png $(magick identify -format '%wx%h' "$dir/$nombre.png")"
done
