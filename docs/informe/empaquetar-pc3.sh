#!/usr/bin/env bash
# Arma la entrega de la PC3 con los nombres que pide el enunciado:
#
#   1ACC0065_PC3_202610_Informe_<código>.docx   uno por integrante; cada uno sube el suyo
#   1ACC0065_PC3_<código>.zip                   el código fuente (git archive de la ref), sin datos
#   1ACC0065-202620_Participación_PC3.docx      lo sube el coordinador junto con su informe
#   SHA256SUMS y ENTREGA.md                     qué commit produjo cada archivo
#
#   ./empaquetar-pc3.sh                         # desde HEAD, en salida/entrega/
#   ./empaquetar-pc3.sh --preliminar            # igual, aunque queden PENDIENTE o PLACEHOLDER
#   ./empaquetar-pc3.sh pc3.1 ~/Descargas/PC3   # desde un tag de la PC3, en otra carpeta
#
# La versión final sale de un tag de la PC3 (pc3 o pc3.N), después del release a main por
# release/pc3 o release/pc3.N (nunca con develop como rama de origen). El zip lleva solo lo versionado: el CSV de OEFA (1,3 GB) nunca lo está, y
# de data/ entran el README y la muestra de las pruebas. No entran la base de tareas ni la
# configuración de los agentes (.beads/, .claude/, .codex/, .agents/, CLAUDE.md, AGENTS.md) ni el
# montaje del video (docs/video/, que es del TB2): no son la solución.
#
# Los códigos de alumno salen de equipo.local.yml (no versionado). Sin él solo se arma una entrega
# --preliminar, con los códigos de ejemplo.
set -euo pipefail
cd "$(dirname "$0")"

preliminar=false
if [ "${1:-}" = "--preliminar" ]; then preliminar=true; shift; fi
ref="${1:-HEAD}"
destino="${2:-$PWD/salida/entrega}"

git rev-parse -q --verify "$ref^{commit}" >/dev/null || { echo "error: no existe la ref '$ref'" >&2; exit 1; }
commit="$(git rev-parse "$ref^{commit}")"
tag="$(git describe --tags --exact-match "$commit" 2>/dev/null || true)"
raiz="$(git rev-parse --show-toplevel)"
problemas=()
equipo=equipo.local.yml
if [ ! -f "$equipo" ]; then
  problemas+=("no hay equipo.local.yml con los códigos reales (ver equipo.ejemplo.yml)")
  equipo=equipo.ejemplo.yml
fi
mapfile -t codigos < <(sed -n 's/^[[:space:]]\{1,\}codigo:[[:space:]]*\(U[0-9]\{9\}\)[[:space:]]*\(#.*\)\{0,1\}$/\1/p' "$equipo")
[ "${#codigos[@]}" -eq 3 ] || echo "aviso: $equipo tiene ${#codigos[@]} códigos" >&2

# 1. Ninguna cifra de prueba: tablas.py --final falla si queda un PLACEHOLDER, una entrada de
#    data/sample/, una estación SINT-/SIN-, una figura sintética, falta un JSON o un invariante no
#    cuadra. Go no marca los datos sintéticos: estas guardas son las que lo impiden.
if python3 -c 'import pandas' 2>/dev/null; then tablas=(python3 tablas.py --final); else tablas=(uv run --quiet tablas.py --final); fi
"${tablas[@]}" >/dev/null 2>&1 || problemas+=("quedan cifras PLACEHOLDER o invariantes sin cuadrar (python3 tablas.py --final)")
# 1b. El informe (tablas, cifras y fragmentos de código) sale del árbol de trabajo y el zip, de la ref: tienen
#     que ser el mismo commit, o el informe mostraría código que no está en el zip.
[ "$(git rev-parse HEAD)" = "$commit" ] || problemas+=("HEAD no es $ref: el informe saldría del árbol de trabajo y el zip de $ref (git checkout $ref)")
# 2. Ningún aviso PENDIENTE en las fuentes, sin contar los condicionales «si-falta» cuyo archivo existe
#    (no se renderizan) ni la plantilla de la participación cuando hay una versión local llenada.
n="$(python3 - <<'PY'
import glob, os, re
archivos = ["informe.qmd"] + sorted(glob.glob("secciones/*.qmd"))
if not os.path.exists("participacion.local.md"):
    archivos.append("_participacion.md")
n = 0
for f in archivos:
    s = open(f, encoding="utf-8").read()
    s = "\n".join(l for l in s.split("\n") if not l.startswith("#"))  # la cabecera comentada de informe.qmd nombra el aviso
    for m in re.finditer(r':::: \{si-falta="([^"]+)"\}.*?\n::::', s, re.S):
        if os.path.exists(m.group(1)):
            s = s.replace(m.group(0), "")
    n += s.count('title="PENDIENTE')
print(n)
PY
)"
[ "$n" -eq 0 ] || problemas+=("quedan $n avisos PENDIENTE en las fuentes (grep -n 'title=\"PENDIENTE' secciones/*.qmd)")
# 2b. Ningún borrador de IA sin revisar: el aviso se borra cuando Rody aprueba la sección.
n="$(cat informe.qmd secciones/*.qmd | grep -c 'title="Borrador generado con IA' || true)"
[ "$n" -eq 0 ] || problemas+=("quedan $n secciones marcadas como borrador de IA sin revisar (grep -n 'Borrador generado con IA' secciones/*.qmd)")
# 3. Ninguna figura faltante: las rutas de los avisos si-falta tienen que existir.
while IFS= read -r ruta; do
  [ -e "$ruta" ] || problemas+=("falta la figura $ruta")
done < <(grep -ho 'si-falta="[^"]*"' secciones/*.qmd | sed 's/si-falta="\(.*\)"/\1/' | sort -u)

echo "→ render"
bash render.sh >/dev/null

# 4. El .docx final no puede decir PENDIENTE ni PLACEHOLDER en ninguna parte.
for c in "${codigos[@]}"; do
  f="salida/1ACC0065_PC3_202610_Informe_${c}.docx"
  [ -f "$f" ] || { echo "error: falta $f" >&2; exit 1; }
  if python3 -c 'import sys, zipfile; x = zipfile.ZipFile(sys.argv[1]).read("word/document.xml").decode(); sys.exit(0 if ("PENDIENTE" in x or "PLACEHOLDER" in x) else 1)' "$f"; then
    problemas+=("$f todavía contiene PENDIENTE o PLACEHOLDER")
  fi
done

if [ "${#problemas[@]}" -gt 0 ]; then
  printf 'problema: %s\n' "${problemas[@]}" >&2
  if ! $preliminar; then
    echo "no se arma la entrega final; para una prueba: ./empaquetar-pc3.sh --preliminar" >&2
    exit 1
  fi
fi

mkdir -p "$destino"
rm -f "$destino"/1ACC0065_PC3_* "$destino"/1ACC0065-202620_Participación_PC3.docx
for c in "${codigos[@]}"; do
  cp "salida/1ACC0065_PC3_202610_Informe_${c}.docx" "$destino/"
  git -C "$raiz" archive --format=zip --prefix=aire-distribuido/ -o "$destino/1ACC0065_PC3_${c}.zip" "$commit" \
    -- . ':(exclude).beads' ':(exclude).claude' ':(exclude).codex' ':(exclude).agents' \
    ':(exclude)CLAUDE.md' ':(exclude)AGENTS.md' ':(exclude)docs/video'
done
cp "salida/1ACC0065-202620_Participación_PC3.docx" "$destino/"

# Ningún archivo grande en el zip: los datos no se versionan, pero se comprueba.
python3 - "$destino/1ACC0065_PC3_${codigos[0]}.zip" <<'PY'
import sys, zipfile
grandes = [(i.filename, i.file_size) for i in zipfile.ZipFile(sys.argv[1]).infolist() if i.file_size > 20 * 2**20]
if grandes:
    sys.exit("error: el zip tiene archivos de más de 20 MiB: " + ", ".join(f for f, _ in grandes))
PY

( cd "$destino" && sha256sum 1ACC0065_PC3_* 1ACC0065-202620_Participación_PC3.docx > SHA256SUMS )

estado="final"
[ "${#problemas[@]}" -eq 0 ] || estado="PRELIMINAR (${#problemas[@]} problemas: ver la salida del script)"
[[ "$tag" =~ ^pc3(\.[0-9]+)?$ ]] || estado="$estado; la ref no es un tag de la PC3 (pc3, pc3.1…)"
[ -z "$(git status --porcelain -- .)" ] || estado="$estado; hay cambios sin commitear en docs/informe"

{
  echo "# Entrega PC3 · 1ACC0065 · 2026-20"
  echo
  echo "- Estado: **$estado**"
  echo "- Commit: \`$commit\`${tag:+ (tag \`$tag\`)}"
  echo "- Generado: $(date '+%Y-%m-%d %H:%M %Z')"
  echo
  echo "| Archivo | Lo sube |"
  echo "|---|---|"
  for c in "${codigos[@]}"; do
    echo "| \`1ACC0065_PC3_202610_Informe_${c}.docx\` y \`1ACC0065_PC3_${c}.zip\` | $c |"
  done
  echo "| \`1ACC0065-202620_Participación_PC3.docx\` | el coordinador, junto con su informe |"
  echo
  echo "Cada integrante sube sus archivos al Aula Virtual antes del sáb 07/11 23:30: el enunciado no"
  echo "admite otro medio. El tag \`${tag:-pc3}\` tiene que estar en \`main\` antes de esa hora. Los sha256 están"
  echo "en \`SHA256SUMS\`."
} > "$destino/ENTREGA.md"

echo
echo "entrega en $destino ($estado)"
ls -1 "$destino"
