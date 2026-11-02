#!/usr/bin/env bash
# Genera el historial de git del Trabajo Final por integrante, para el Anexo C del informe (la
# rúbrica pide el historial de versiones y evidencia de Git Flow). Portado de
# ../concurrente/tp/informe/tp/generar-historial.sh (LaTeX → Markdown).
#
#   ./generar-historial.sh                 # escribe generado/local/historial.md (no versionado)
#   ./generar-historial.sh <archivo>       # en otro lugar (p. ej. para actualizar el de ejemplo)
#   FETCH=1 ./generar-historial.sh         # hace git fetch origin antes (por defecto no toca la red)
#
# Qué escribe:
#   1. Commits por integrante y por entregable. Los cortes son los tags pc3, pc4 y tb2 (los que
#      existan); lo posterior al último tag es el entregable en curso. Sin tags, todo es la PC3.
#   2. Las pull requests fusionadas (rama de origen y destino): la evidencia del Git Flow.
#   3. Los commits del entregable en curso, con una marca de si ya están en origin/main.
#
# Usa los nombres, nunca los correos: %aN pasa por el .mailmap de la raíz del repo, y los alias que
# el .mailmap no puede unificar sin escribir un correo se unifican abajo, por nombre.
# Se corre desde origin (ramas publicadas): hacer push antes de generar la versión que se entrega.
set -euo pipefail
cd "$(dirname "$0")"

if [ "${FETCH:-0}" = "1" ] && ! git fetch --quiet origin 2>/dev/null; then
  echo "aviso: git fetch falló; se usan las refs de origin del último fetch" >&2
fi
if [ -z "$(git for-each-ref --count=1 --format='%(refname)' refs/remotes/origin/)" ]; then
  echo "error: no hay refs de origin; haz git fetch origin antes de generar el historial" >&2
  exit 1
fi

out="${1:-generado/local/historial.md}"
mkdir -p "$(dirname "$out")"
tmp="$(mktemp "${out}.XXXXXX")"
trap 'rm -f "$tmp"' EXIT

remotos=(--exclude='origin/__dolt*' --exclude='origin/HEAD' --remotes=origin)

# Alias de autor por nombre. Formato: nombre en el commit|nombre real (una línea por alias).
alias_nombres='R0SEWT-CGIAR|Rody Sebastian Vilchez Marin'
unificar() { awk -F'|' -v alias="$alias_nombres" '
  BEGIN { n = split(alias, a, "\n"); for (i = 1; i <= n; i++) { split(a[i], par, "|"); m[par[1]] = par[2] } }
  { print (($0 in m) ? m[$0] : $0) }'; }

# Escapa lo que Markdown/Pandoc interpretaría dentro de una celda (énfasis, citas, tablas...).
escapar() { sed -e 's/[][\\`*_{}<>|$@~^#]/\\&/g'; }

# Conteo por autor de los commits (sin merges) de un rango de git log: "autor|n".
conteo() {
  git log "$@" --no-merges --format='%aN' | unificar | sort | uniq -c \
    | awk '{n=$1; $1=""; sub(/^ /, ""); print $0 "|" n}' | sort -t '|' -k1,1
}
buscar() { printf '%s\n' "$2" | awk -F'|' -v a="$1" '$1==a {print $2; f=1} END {if (!f) print 0}'; }

# Cortes por entregable.
nombres=(PC3 PC4 TB2)
tags=(pc3 pc4 tb2)
columnas=()
rangos=()
anterior=""
for i in "${!tags[@]}"; do
  t="${tags[$i]}"
  git rev-parse -q --verify "refs/tags/$t" >/dev/null || break
  columnas+=("${nombres[$i]}")
  if [ -z "$anterior" ]; then rangos+=("$t"); else rangos+=("$t --not $anterior"); fi
  anterior="$t"
done
n_cortes=${#columnas[@]}
if [ "$n_cortes" -lt "${#nombres[@]}" ]; then
  en_curso="${nombres[$n_cortes]}"
  [ "$n_cortes" -gt 0 ] && en_curso="$en_curso (en curso)"
else
  en_curso="Después de TB2"
fi
columnas+=("$en_curso")
rango_curso=("${remotos[@]}")
[ -n "$anterior" ] && rango_curso+=(--not "$anterior")

conteos=()
for r in "${rangos[@]}"; do
  # shellcheck disable=SC2086  # el rango se parte en palabras a propósito ("pc4 --not pc3")
  conteos+=("$(conteo $r)")
done
conteos+=("$(conteo "${rango_curso[@]}")")
merges="$(git log "${remotos[@]}" --merges --grep='^Merge pull request' --format='%aN' | unificar \
  | sort | uniq -c | awk '{n=$1; $1=""; sub(/^ /, ""); print $0 "|" n}' | sort -t '|' -k1,1)"

autores="$( (printf '%s\n' "${conteos[@]}" "$merges" | cut -d'|' -f1) | grep -v '^$' | sort -u)"
commits="$(git log "${rango_curso[@]}" --no-merges --format='%H|%ad|%aN|%h|%s' --date=short)"
prs="$(git log "${remotos[@]}" --merges --grep='^Merge pull request' --format='%ad|%aN|%s' --date=short)"
if [ -z "$commits" ] && [ "$n_cortes" -eq 0 ]; then
  echo "error: git log no devolvió commits de origin; no se sobrescribe $out" >&2
  exit 1
fi

{
  echo "<!-- Generado por generar-historial.sh el $(date +%F) desde origin. No editar a mano. -->"
  echo
  echo '::: {#tbl-commits-autor}'
  printf '| Integrante |'; for c in "${columnas[@]}"; do printf ' %s |' "$c"; done; echo ' Merges de PR |'
  printf '|:---|'; for _ in "${columnas[@]}"; do printf -- '---:|'; done; printf -- '---:|\n'
  while IFS= read -r a; do
    [ -n "$a" ] || continue
    printf '| %s |' "$(printf '%s' "$a" | unificar | escapar)"
    for c in "${conteos[@]}"; do printf ' %s |' "$(buscar "$a" "$c")"; done
    printf ' %s |\n' "$(buscar "$a" "$merges")"
  done <<< "$autores"
  echo
  echo 'Commits por integrante y entregable'
  echo ':::'
  echo
  echo "*Nota.* Commits sin merges en las ramas publicadas en \`origin\`. Cada entregable cuenta desde el tag del anterior hasta el suyo (\`pc3\`, \`pc4\`, \`tb2\`); el que no tiene tag todavía cuenta todo lo publicado después del último. Los merges de PR se cuentan aparte porque no tocan archivos por sí mismos."
  echo
  echo '::: {#tbl-prs tbl-colwidths="[15,8,50,27]"}'
  echo '| Fecha | PR | Rama de origen | Fusionada por |'
  echo '|:---|:---|:---|:---|'
  if [ -n "$prs" ]; then
    while IFS='|' read -r fecha autor msg; do
      num="$(printf '%s' "$msg" | sed -n 's/^Merge pull request #\([0-9]*\).*/\1/p')"
      rama="$(printf '%s' "$msg" | sed -n 's/^Merge pull request #[0-9]* from [^/]*\/\(.*\)$/\1/p')"
      printf '| %s | #%s | %s | %s |\n' "$fecha" "$num" "$(printf '%s' "$rama" | escapar)" \
        "$(printf '%s' "$autor" | unificar | escapar)"
    done <<< "$prs"
  else
    echo '| — | — | — | — |'
  fi
  echo
  echo 'Pull requests fusionadas (Git Flow: feature/\* → develop, release/\* → main)'
  echo ':::'
  echo
  echo '::: {#tbl-commits tbl-colwidths="[12,22,10,48,8]"}'
  echo '| Fecha | Autor | Commit | Mensaje | En `main` |'
  echo '|:---|:---|:---|:---|:---|'
  if [ -n "$commits" ]; then
    while IFS='|' read -r sha fecha autor corto msg; do
      if git merge-base --is-ancestor "$sha" origin/main 2>/dev/null; then en='sí'; else en='no'; fi
      printf '| %s | %s | `%s` | %s | %s |\n' "$fecha" "$(printf '%s' "$autor" | unificar | escapar)" \
        "$corto" "$(printf '%s' "$msg" | escapar)" "$en"
    done <<< "$commits"
  else
    echo '| — | — | — | — | — |'
  fi
  echo
  echo "Commits de ${en_curso}, del más reciente al más antiguo"
  echo ':::'
  echo
  echo '*Nota.* «En `main`» indica si el commit ya forma parte de `origin/main` al generar esta tabla; los que dicen «no» están en una rama publicada pendiente de fusión.'
} > "$tmp"

mv "$tmp" "$out"
trap - EXIT
echo "escrito $out"
