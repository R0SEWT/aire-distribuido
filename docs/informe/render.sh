#!/usr/bin/env bash
# Renderiza el informe de la PC3 a Word: un .docx por integrante, con su código en la carátula, y
# el reporte de participación. Todo queda en salida/ (no se versiona).
#
#   bash render.sh                 # tablas + historial + los tres informes + participación
#   bash render.sh U000000002      # solo el informe de ese código (más tablas e historial)
#   FETCH=1 bash render.sh         # además hace git fetch antes de armar el historial
#
# Pasos:
#   1. tablas.py: generado/*.json → generado/tablas/*.md y generado/cifras.yml
#   2. generado/local/ (no versionado): equipo.yml (de equipo.local.yml o, si no existe, de
#      equipo.ejemplo.yml), _participacion.md (de participacion.local.md o de la plantilla
#      _participacion.md) e historial.md (generar-historial.sh o, si falla, el de ejemplo)
#   3. quarto render informe.qmd -M codigo:<código> por cada integrante de `equipo`
#   4. quarto render participacion.qmd
#   5. w:updateFields en cada .docx: Word pregunta al abrir si actualiza los campos (el índice)
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

echo "→ generado/local/"
mkdir -p generado/local
if [ -f equipo.local.yml ]; then
  cp equipo.local.yml generado/local/equipo.yml
else
  echo "aviso: no hay equipo.local.yml; la carátula sale con los códigos de ejemplo de equipo.ejemplo.yml" >&2
  cp equipo.ejemplo.yml generado/local/equipo.yml
fi
if [ -f participacion.local.md ]; then
  cp participacion.local.md generado/local/_participacion.md
else
  cp _participacion.md generado/local/_participacion.md
fi
if ! ./generar-historial.sh; then
  echo "aviso: no se pudo generar el historial; se usa generado/historial.ejemplo.md" >&2
  cp generado/historial.ejemplo.md generado/local/historial.md
fi

# Los códigos salen de `equipo` (líneas indentadas), la misma fuente de la carátula.
# Bash 3.2 (macOS) no incluye mapfile.
equipo=()
while IFS= read -r codigo; do
  equipo+=("$codigo")
done < <(sed -n 's/^[[:space:]]\{1,\}codigo:[[:space:]]*\(U[0-9]\{9\}\)[[:space:]]*\(#.*\)\{0,1\}$/\1/p' generado/local/equipo.yml)
[ "${#equipo[@]}" -gt 0 ] || { echo "error: no hay códigos en generado/local/equipo.yml" >&2; exit 1; }
if [ "$#" -gt 0 ]; then codigos=("$@"); else codigos=("${equipo[@]}"); fi
for c in "${codigos[@]}"; do
  [[ " ${equipo[*]} " == *" $c "* ]] || { echo "error: $c no está en equipo (generado/local/equipo.yml)" >&2; exit 1; }
done

mkdir -p salida
# Quarto deja el archivo en output-dir (salida/); si una versión lo deja junto al .qmd, se mueve.
mover() { [ -f "$1" ] && mv -f "$1" "salida/$1"; [ -f "salida/$1" ]; }

# Word no llena el índice (un campo TOC) hasta que actualiza los campos: con w:updateFields en
# settings.xml lo pregunta al abrir el archivo.
actualizar_campos() {
  python3 - "$1" <<'PY'
import os, re, sys, tempfile, zipfile
ruta = sys.argv[1]
with zipfile.ZipFile(ruta) as z:
    partes = [(i, z.read(i.filename)) for i in z.infolist()]
salida = []
for info, datos in partes:
    if info.filename == "word/settings.xml":
        xml = datos.decode("utf-8")
        if "w:updateFields" not in xml:
            # Orden del esquema (CT_Settings): updateFields va antes de estos elementos.
            despues = re.search(r"<w:(hdrShapeDefaults|footnotePr|endnotePr|compat|docVars|rsids)\b|<m:mathPr\b|"
                                r"<w:(attachedSchema|themeFontLang|clrSchemeMapping|doNotIncludeSubdocsInStats|"
                                r"doNotAutoCompressPictures|forceUpgrade|captions|readModeInkLockDown|smartTagType|"
                                r"shapeDefaults|doNotEmbedSmartTags|decimalSymbol|listSeparator)\b|</w:settings>", xml)
            xml = xml[:despues.start()] + '<w:updateFields w:val="true"/>' + xml[despues.start():]
        datos = xml.encode("utf-8")
    salida.append((info, datos))
fd, tmp = tempfile.mkstemp(dir=os.path.dirname(ruta) or ".", suffix=".docx")
os.close(fd)
with zipfile.ZipFile(tmp, "w", zipfile.ZIP_DEFLATED) as z:
    for info, datos in salida:
        z.writestr(info, datos)
os.replace(tmp, ruta)
PY
}

for c in "${codigos[@]}"; do
  nombre="1ACC0065_PC3_202610_Informe_${c}.docx"
  echo "→ $nombre"
  quarto render informe.qmd -M "codigo:$c" --output "$nombre" --quiet
  mover "$nombre" || { echo "error: quarto no produjo $nombre" >&2; exit 1; }
  actualizar_campos "salida/$nombre"
done

if [ "$#" -eq 0 ]; then
  participacion="1ACC0065-202620_Participación_PC3.docx"
  echo "→ $participacion"
  # Con la plantilla del docente y los datos locales, se llena el .docx del curso tal cual (logo,
  # tablas y estilos del formato); si falta alguno, se renderiza la versión de Quarto como respaldo.
  if [ -f "${PLANTILLA_PARTICIPACION:-recursos/plantilla-participacion.docx}" ] && [ -f participacion.local.yml ] && command -v uv >/dev/null 2>&1; then
    uv run --quiet --with python-docx --with pyyaml llenar-participacion.py "salida/$participacion"
  else
    echo "aviso: sin recursos/plantilla-participacion.docx o participacion.local.yml: participación desde Quarto (no es el formato del curso)" >&2
    quarto render participacion.qmd --output "$participacion" --quiet
    mover "$participacion" || { echo "error: quarto no produjo $participacion" >&2; exit 1; }
  fi
fi

n_pendiente="$(cat informe.qmd secciones/*.qmd _participacion.md | grep -c 'title="PENDIENTE' || true)"
n_placeholder="$( (grep -l '"placeholder": *true' generado/*.json 2>/dev/null || true) | wc -l)"
echo
echo "listo: salida/"
ls -1 salida/*.docx
echo "avisos PENDIENTE en las fuentes: $n_pendiente · JSON PLACEHOLDER en generado/: $n_placeholder"
echo "(la entrega final no lleva ninguno, ni datos sintéticos: ./empaquetar-pc3.sh lo comprueba)"
