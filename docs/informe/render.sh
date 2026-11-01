#!/usr/bin/env bash
# Renderiza el informe de la PC3 a Word: un .docx por integrante, con su código en la carátula, y
# el reporte de participación. Todo queda en salida/ (no se versiona).
#
#   bash render.sh                 # tablas + historial + los tres informes + participación
#   bash render.sh U202311495      # solo el informe de ese código (más tablas e historial)
#
# Pasos:
#   1. tablas.py: generado/*.json → generado/tablas/*.md y generado/cifras.yml
#   2. generar-historial.sh: generado/historial.md (si no hay red, se conserva el anterior)
#   3. quarto render informe.qmd -M codigo:<código> por cada integrante de `equipo` (_quarto.yml)
#   4. quarto render participacion.qmd
set -euo pipefail
cd "$(dirname "$0")"

command -v quarto >/dev/null 2>&1 || { echo "error: falta quarto (https://quarto.org)" >&2; exit 1; }

echo "→ tablas y cifras"
if python3 -c 'import pandas' 2>/dev/null; then
  python3 tablas.py
elif command -v uv >/dev/null 2>&1; then
  uv run --quiet tablas.py
else
  echo "error: tablas.py necesita pandas (pip install pandas, o instalar uv)" >&2
  exit 1
fi

echo "→ historial de git"
if ! ./generar-historial.sh; then
  if [ -s generado/historial.md ]; then
    echo "aviso: no se pudo actualizar el historial; se conserva generado/historial.md" >&2
  else
    echo "error: no hay historial anterior para continuar" >&2
    exit 1
  fi
fi

# Los códigos salen de `equipo` en _quarto.yml (líneas indentadas), la misma fuente de la carátula.
mapfile -t equipo < <(sed -n 's/^[[:space:]]\{1,\}codigo:[[:space:]]*\(U[0-9]\{9\}\)[[:space:]]*$/\1/p' _quarto.yml)
[ "${#equipo[@]}" -gt 0 ] || { echo "error: no hay códigos en equipo (_quarto.yml)" >&2; exit 1; }
if [ "$#" -gt 0 ]; then codigos=("$@"); else codigos=("${equipo[@]}"); fi
for c in "${codigos[@]}"; do
  [[ " ${equipo[*]} " == *" $c "* ]] || { echo "error: $c no está en equipo (_quarto.yml)" >&2; exit 1; }
done

mkdir -p salida
# Quarto deja el archivo en output-dir (salida/); si una versión lo deja junto al .qmd, se mueve.
mover() { [ -f "$1" ] && mv -f "$1" "salida/$1"; [ -f "salida/$1" ]; }

for c in "${codigos[@]}"; do
  nombre="1ACC0065_PC3_202610_Informe_${c}.docx"
  echo "→ $nombre"
  quarto render informe.qmd -M "codigo:$c" --output "$nombre" --quiet
  mover "$nombre" || { echo "error: quarto no produjo $nombre" >&2; exit 1; }
done

if [ "$#" -eq 0 ]; then
  participacion="1ACC0065-202620_Participación_PC3.docx"
  echo "→ $participacion"
  quarto render participacion.qmd --output "$participacion" --quiet
  mover "$participacion" || { echo "error: quarto no produjo $participacion" >&2; exit 1; }
fi

n_pendiente="$(cat informe.qmd secciones/*.qmd _participacion.md | grep -c 'title="PENDIENTE' || true)"
n_placeholder="$(grep -l '"placeholder": *true' generado/*.json 2>/dev/null | wc -l)"
echo
echo "listo: salida/"
ls -1 salida/*.docx
echo "avisos PENDIENTE en las fuentes: $n_pendiente · JSON PLACEHOLDER en generado/: $n_placeholder"
echo "(la entrega final no lleva ninguno: ./empaquetar-pc3.sh lo comprueba)"
