#!/usr/bin/env bash
# Publica las presentaciones en https://uni.rosewt.dev/concurrente/ (sitio de Netlify
# «concurrente-rosewt», cuenta r0sewt). Este sitio es el origen: no tiene dominio propio y el catálogo
# uni.rosewt.dev (repo R0SEWT/uni) lo monta en /concurrente/ con un proxy. La dirección vieja,
# concurrente.rosewt.dev, redirige ahí.
#
#   bash publicar.sh pc4      # renderiza slides/pc4.qmd y lo publica como /pc4/
#   bash publicar.sh tb2      # renderiza slides/index.qmd y lo publica como /tb2/
#
# La PC3 ya no se publica desde aquí: su deck era index.qmd, que hoy es el del TB2. Para rehacerla,
# el deck de la PC3 está en el commit f023c24 (ver el error de abajo).
#
# Cómo funciona: sitio/ es la raíz del sitio. index.html (la portada) se versiona; cada entrega
# (sitio/pc3/, sitio/pc4/, …) es el deck renderizado y su PDF, que no se versionan (el HTML pesa
# unos 15 MB). Netlify publica sitio/ entero, así que las entregas que ya están en línea tienen que
# seguir en sitio/ al publicar otra: si falta alguna, el script se detiene antes de renderizar.
#
# Requiere quarto, Chrome (para el PDF) y la CLI de Netlify con sesión iniciada (netlify login), o
# npx, que la descarga.
set -euo pipefail
cd "$(dirname "$0")"

# Las entregas que ya están en línea. Al publicar una nueva por primera vez, agregarla aquí.
publicadas=(pc3 pc4)

entrega="${1:-}"
case "$entrega" in
  pc4|tb2) ;;
  pc3)
    cat >&2 <<'EOF'
error: la PC3 no se publica desde este checkout: su deck era slides/index.qmd, que hoy es el del TB2,
y se publicaría el TB2 en /pc3/. Para rehacer sitio/pc3/:

  git worktree add /tmp/aire-pc3 f023c24
  bash /tmp/aire-pc3/docs/presentacion/slides/render.sh --pdf
  mkdir -p sitio/pc3
  cp /tmp/aire-pc3/docs/presentacion/slides/_site/index.html sitio/pc3/index.html
  cp /tmp/aire-pc3/docs/presentacion/slides/_site/deck.pdf sitio/pc3/aire-distribuido-pc3.pdf
  git worktree remove /tmp/aire-pc3
EOF
    exit 1 ;;
  *) echo "error: uso: bash publicar.sh pc4|tb2 (se recibió '$entrega')" >&2; exit 1 ;;
esac

# Netlify publica sitio/ entero: una entrega que está en línea y falta en la carpeta desaparecería del
# sitio. Se comprueba antes de renderizar, para no gastar el render.
for e in "${publicadas[@]}"; do
  [ "$e" = "$entrega" ] && continue
  for f in "sitio/$e/index.html" "sitio/$e/aire-distribuido-$e.pdf"; do
    [ -s "$f" ] || {
      echo "error: falta $f: publicar ahora borraría /$e/ del sitio. Rehacerla antes" \
           "(bash publicar.sh $e, o el error de pc3 dice cómo)." >&2
      exit 1
    }
  done
done

# La PC4 tiene su propio deck (slides/pc4.qmd); el TB2 es index.qmd.
if [ "$entrega" = pc4 ]; then
  bash slides/render.sh pc4 --pdf
  html=slides/_site/pc4.html; pdf=slides/_site/pc4.pdf
else
  bash slides/render.sh --pdf
  html=slides/_site/index.html; pdf=slides/_site/deck.pdf
fi
mkdir -p "sitio/$entrega"
cp "$html" "sitio/$entrega/index.html"
cp "$pdf" "sitio/$entrega/aire-distribuido-$entrega.pdf"

if command -v netlify >/dev/null 2>&1; then netlify=(netlify); else netlify=(npx -y netlify-cli); fi
commit="$(git rev-parse --short HEAD)"
# El id del sitio y no su nombre: esta CLI no encuentra el sitio por nombre al desplegar. --no-build:
# no hay nada que construir en Netlify, se sube sitio/ tal cual.
sitio_id="b375213d-41e4-493d-ad99-04da09b1236f"   # concurrente-rosewt
"${netlify[@]}" deploy --prod --no-build --dir sitio --site "$sitio_id" \
  --message "$entrega desde $commit ($(git rev-parse --abbrev-ref HEAD))"

echo "publicado: https://uni.rosewt.dev/concurrente/$entrega/"
