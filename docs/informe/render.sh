#!/usr/bin/env bash
# Renderiza el informe de un entregable a Word: un .docx por integrante, con su código en la
# carátula, y el reporte de participación. Todo queda en salida/ (no se versiona).
#
#   bash render.sh                 # el del TB2 (informe final): tablas + historial + los tres informes + participación
#   ENTREGA=pc4 bash render.sh     # el de la PC4 (informe-pc4.qmd), como se entregó
#   ENTREGA=pc3 bash render.sh     # el de la PC3 (informe.qmd), como se entregó
#   bash render.sh U000000002      # solo el informe de ese código (más tablas e historial)
#   FETCH=1 bash render.sh         # además hace git fetch antes de armar el historial
#
# El informe de la PC4 (informe-pc4.qmd) contiene el de la PC3: las secciones de secciones/ más las
# de secciones/pc4/. El del TB2 (informe-tb2.qmd) contiene los dos, con las secciones observadas en la
# PC4 corregidas en secciones/tb2/, y sus listados de código van como texto (no corre listados_imagen.py).
#
# Pasos:
#   1. tablas.py: generado/*.json → generado/tablas/*.md y generado/cifras.yml
#   2. verificar_limpieza.py: valida y regenera reports/revision_limpieza.md
#   3. generado/local/ (no versionado): equipo.yml (de equipo.local.yml o, si no existe, de
#      equipo.ejemplo.yml), _participacion.md (de participacion.local.md o de la plantilla
#      _participacion.md), entidades-girado.png (el diagrama de entidades girado, para la PC4) e
#      historial.md (generar-historial.sh o, si falla, el de ejemplo)
#   4. quarto render informe.qmd (o informe-pc4.qmd, informe-tb2.qmd) -M codigo:<código> por cada integrante de `equipo`
#   5. quarto render participacion.qmd (o participacion-pc4.qmd, participacion-tb2.qmd)
#   6. w:updateFields en cada .docx: Word pregunta al abrir si actualiza los campos (el índice)
#   7. indice.py: llena el índice con sus entradas y páginas (medidas con LibreOffice), para que no
#      salga vacío fuera de Word
set -euo pipefail
cd "$(dirname "$0")"

command -v quarto >/dev/null 2>&1 || { echo "error: falta quarto (https://quarto.org)" >&2; exit 1; }

ENTREGA="${ENTREGA:-tb2}"
case "$ENTREGA" in
  pc3) fuente=informe.qmd;     sufijo="";     etiqueta=PC3 ;;
  pc4) fuente=informe-pc4.qmd; sufijo="-pc4"; etiqueta=PC4 ;;
  tb2) fuente=informe-tb2.qmd; sufijo="-tb2"; etiqueta=TB2 ;;
  *) echo "error: ENTREGA tiene que ser pc3, pc4 o tb2, es '$ENTREGA'" >&2; exit 1 ;;
esac
echo "→ informe de la $etiqueta ($fuente)"

echo "→ tablas y cifras"
if python3 -c 'import pandas' 2>/dev/null; then
  python3 tablas.py --entrega "$ENTREGA"
elif command -v uv >/dev/null 2>&1; then
  uv run --quiet tablas.py --entrega "$ENTREGA"
else
  echo "error: tablas.py necesita pandas (pip install pandas, o instalar uv)" >&2
  exit 1
fi

echo "→ verificación de limpieza"
python3 ../../scripts/verificar_limpieza.py

# Los listados de código y de terminal pasan a imagen (una línea larga se parte mal en Word).
# Solo en el informe de la PC4, como se entregó. En el del TB2 van como texto: el docente observó en la
# PC4 que en una imagen no se puede buscar el código (tablas.py los deja como texto).
# Necesita Chrome y pygments; si falta alguno, se avisa y los listados quedan como texto, que es como
# los escribe tablas.py.
if [ "$ENTREGA" = pc4 ]; then
  if ! command -v google-chrome >/dev/null 2>&1; then
    echo "aviso: no hay google-chrome: los listados quedan como texto (listados_imagen.py no corre)" >&2
  elif python3 -c 'import pygments' 2>/dev/null; then
    python3 listados_imagen.py
  elif command -v uv >/dev/null 2>&1; then
    uv run --quiet --with pygments listados_imagen.py
  else
    echo "aviso: falta pygments (pip install pygments, o instalar uv): los listados quedan como texto" >&2
  fi
fi

echo "→ generado/local/"
mkdir -p generado/local
if [ -f equipo.local.yml ]; then
  cp equipo.local.yml generado/local/equipo.yml
else
  echo "aviso: no hay equipo.local.yml; la carátula sale con los códigos de ejemplo de equipo.ejemplo.yml" >&2
  cp equipo.ejemplo.yml generado/local/equipo.yml
fi
# La participación de cada entregable: la llenada (no versionada) o, si no existe, su plantilla.
for p in participacion participacion-pc4 participacion-tb2; do
  if [ -f "$p.local.md" ]; then
    cp "$p.local.md" "generado/local/_$p.md"
  else
    cp "_$p.md" "generado/local/_$p.md"
  fi
done
# El diagrama de entidades es apaisado (3500 × 2200): a lo ancho de la página sus campos no se leen. Se
# pone girado, a página completa; la copia girada se arma aquí desde el PNG de docs/diagramas/, que no se
# toca. Sin ImageMagick va sin girar, con un aviso.
entidades=../diagramas/entidades.png
if [ -f "$entidades" ]; then
  if command -v magick >/dev/null 2>&1; then
    magick "$entidades" -rotate -90 generado/local/entidades-girado.png
  elif command -v convert >/dev/null 2>&1; then
    convert "$entidades" -rotate -90 generado/local/entidades-girado.png
  else
    echo "aviso: sin ImageMagick (magick o convert): el diagrama de entidades va sin girar y se lee peor" >&2
    cp "$entidades" generado/local/entidades-girado.png
  fi
fi
# La arquitectura del TB2 también es apaisada (3474 × 1422): va girada a página completa, como la de
# entidades. La copia girada se arma desde ../diagramas/arquitectura-tb2.png, que no se toca.
arquitectura=../diagramas/arquitectura-tb2.png
if [ -f "$arquitectura" ]; then
  if command -v magick >/dev/null 2>&1; then
    magick "$arquitectura" -rotate -90 generado/local/arquitectura-girada.png
  elif command -v convert >/dev/null 2>&1; then
    convert "$arquitectura" -rotate -90 generado/local/arquitectura-girada.png
  else
    echo "aviso: sin ImageMagick (magick o convert): la arquitectura va sin girar y se lee peor" >&2
    cp "$arquitectura" generado/local/arquitectura-girada.png
  fi
fi
if ! ./generar-historial.sh; then
  echo "aviso: no se pudo generar el historial; se usa generado/historial.ejemplo.md" >&2
  cp generado/historial.ejemplo.md generado/local/historial.md
fi

# Los códigos salen de `equipo` (líneas indentadas), la misma fuente de la carátula.
# macOS todavía incluye Bash 3.2, que no tiene mapfile; el bucle funciona tanto
# allí como en versiones nuevas de Bash.
equipo=()
while IFS= read -r codigo_equipo || [[ -n "$codigo_equipo" ]]; do
  equipo+=("$codigo_equipo")
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

informes=()
for c in "${codigos[@]}"; do
  nombre="1ACC0065_${etiqueta}_202610_Informe_${c}.docx"
  echo "→ $nombre"
  quarto render "$fuente" -M "codigo:$c" --output "$nombre" --quiet
  mover "$nombre" || { echo "error: quarto no produjo $nombre" >&2; exit 1; }
  actualizar_campos "salida/$nombre"
  informes+=("salida/$nombre")
done
# El índice con sus entradas y páginas, para que no salga vacío fuera de Word (el visor del Aula, LibreOffice,
# un PDF): indice.py mide las páginas con LibreOffice. Word lo vuelve a calcular al abrir. Sin soffice, avisa.
echo "→ índice"
python3 indice.py "${informes[@]}" || echo "aviso: indice.py falló; el índice de los .docx queda para Word" >&2

if [ "$#" -eq 0 ]; then
  participacion="1ACC0065-202620_Participación_${etiqueta}.docx"
  echo "→ $participacion"
  # Con la plantilla del docente y los datos locales, se llena el .docx del curso tal cual (logo,
  # tablas y estilos del formato); si falta alguno, se renderiza la versión de Quarto como respaldo.
  if [ -f "${PLANTILLA_PARTICIPACION:-recursos/plantilla-participacion.docx}" ] && [ -f "participacion$sufijo.local.yml" ] && command -v uv >/dev/null 2>&1; then
    DATOS_PARTICIPACION="participacion$sufijo.local.yml" uv run --quiet --with python-docx --with pyyaml llenar-participacion.py "salida/$participacion"
  else
    echo "aviso: sin recursos/plantilla-participacion.docx o participacion$sufijo.local.yml: participación desde Quarto (no es el formato del curso)" >&2
    quarto render "participacion$sufijo.qmd" --output "$participacion" --quiet
    mover "$participacion" || { echo "error: quarto no produjo $participacion" >&2; exit 1; }
  fi
fi

fuentes=(informe.qmd secciones/*.qmd _participacion.md)
[ "$ENTREGA" = pc4 ] && fuentes=(informe-pc4.qmd secciones/*.qmd secciones/pc4/*.qmd _participacion-pc4.md)
[ "$ENTREGA" = tb2 ] && fuentes=(informe-tb2.qmd secciones/*.qmd secciones/pc4/*.qmd secciones/tb2/*.qmd _participacion-tb2.md)
n_pendiente="$(cat "${fuentes[@]}" | grep -c 'title="PENDIENTE' || true)"
n_placeholder="$( (grep -l '"placeholder": *true' generado/*.json 2>/dev/null || true) | wc -l)"
echo
echo "listo: salida/"
ls -1 salida/*.docx
echo "avisos PENDIENTE en las fuentes: $n_pendiente · JSON PLACEHOLDER en generado/: $n_placeholder"
echo "(la entrega final no lleva ninguno, ni datos sintéticos: ./empaquetar-$ENTREGA.sh lo comprueba)"
