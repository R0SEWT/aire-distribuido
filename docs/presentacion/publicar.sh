#!/usr/bin/env bash
# Publica las presentaciones en https://uni.rosewt.dev/concurrente/ (sitio de Netlify
# «concurrente-rosewt», cuenta r0sewt). Este sitio es el origen: no tiene dominio propio y el catálogo
# uni.rosewt.dev (repo R0SEWT/uni) lo monta en /concurrente/ con un proxy. La dirección vieja,
# concurrente.rosewt.dev, redirige ahí.
#
#   bash publicar.sh          # renderiza el deck y lo publica como /pc3/
#   bash publicar.sh pc4      # lo mismo, como /pc4/
#
# Cómo funciona: sitio/ es la raíz del sitio. index.html (la portada) se versiona; cada entrega
# (sitio/pc3/, sitio/pc4/, …) es el deck renderizado y su PDF, que no se versionan (el HTML pesa
# unos 15 MB). Netlify publica sitio/ entero, así que las entregas anteriores tienen que seguir en
# sitio/ al publicar la siguiente: si se pierden, se rehacen desde el commit de su entrega.
#
# Requiere quarto, Chrome (para el PDF) y la CLI de Netlify con sesión iniciada (netlify login), o
# npx, que la descarga.
set -euo pipefail
cd "$(dirname "$0")"

entrega="${1:-pc3}"
[[ "$entrega" =~ ^(pc3|pc4|tb2)$ ]] || { echo "error: la entrega es pc3, pc4 o tb2, no '$entrega'" >&2; exit 1; }

bash slides/render.sh --pdf
mkdir -p "sitio/$entrega"
cp slides/_site/index.html "sitio/$entrega/index.html"
cp slides/_site/deck.pdf "sitio/$entrega/aire-distribuido-$entrega.pdf"

if command -v netlify >/dev/null 2>&1; then netlify=(netlify); else netlify=(npx -y netlify-cli); fi
commit="$(git rev-parse --short HEAD)"
# El id del sitio y no su nombre: esta CLI no encuentra el sitio por nombre al desplegar. --no-build:
# no hay nada que construir en Netlify, se sube sitio/ tal cual.
sitio_id="b375213d-41e4-493d-ad99-04da09b1236f"   # concurrente-rosewt
"${netlify[@]}" deploy --prod --no-build --dir sitio --site "$sitio_id" \
  --message "$entrega desde $commit ($(git rev-parse --abbrev-ref HEAD))"

echo "publicado: https://uni.rosewt.dev/concurrente/$entrega/"
