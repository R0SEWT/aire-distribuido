#!/usr/bin/env bash
# Graba las ejecuciones reales de la terminal para el video de la PC3 con asciinema,
# sin tocar la pantalla de nadie: cada escena corre en un pseudoterminal propio.
#
#   bash docs/video/grabar.sh            # graba las cinco escenas en docs/video/casts/
#   bash docs/video/grabar.sh cargar     # solo una (cargar, entrenar, benchmark, tests, docker)
#
# Requiere: asciinema (uv tool install asciinema), go, docker, y el CSV de OEFA en
# data/bronze/ (o un enlace simbólico a él). Las corridas escriben data/silver, data/gold y
# reports/: al terminar, `git checkout -- reports/` deja los reportes versionados como estaban.
set -euo pipefail
cd "$(dirname "$0")/../.."
CASTS=docs/video/casts
mkdir -p "$CASTS" data/silver data/gold data/reports
COLS=${COLS:-120}
ROWS=${ROWS:-30}
CSV="data/bronze/1a_Vigilancia y Seguimiento ambiental en la calidad del aire.csv"

# escena NOMBRE COMANDO...: muestra un prompt, "tipea" el comando y lo ejecuta, todo grabado.
escena() {
  local nombre=$1; shift
  local cmd="$*"
  local guion
  guion=$(mktemp)
  cat > "$guion" <<EOS
printf '\033[1;32m~/aire-distribuido\033[0m \033[1;34m\$\033[0m '
cmd=\$(cat <<'EOC'
$cmd
EOC
)
for ((i=0; i<\${#cmd}; i++)); do printf '%s' "\${cmd:i:1}"; sleep 0.012; done
printf '\n'
sleep 0.4
bash -c "\$cmd"
printf '\n\033[1;32m~/aire-distribuido\033[0m \033[1;34m\$\033[0m '
sleep 1.5
EOS
  rm -f "$CASTS/$nombre.cast"
  asciinema rec --quiet --overwrite --cols "$COLS" --rows "$ROWS" -i 2 \
    -t "aire-distribuido · $nombre" -c "bash $guion" "$CASTS/$nombre.cast"
  rm -f "$guion"
  echo "grabada: $CASTS/$nombre.cast"
}

# Compilar antes para que `go run` no se demore compilando dentro de la grabación.
go build ./... >/dev/null

quiere() { local q=$1; shift; [ $# -eq 0 ] || [[ " $* " == *" $q "* ]]; }
pedidas=("$@")

if quiere cargar "${pedidas[@]}"; then
  escena cargar "go run ./cmd/cargar -entrada \"$CSV\""
fi
if quiere entrenar "${pedidas[@]}"; then
  escena entrenar "go run ./cmd/entrenar -codo 2:10"
fi
if quiere benchmark "${pedidas[@]}"; then
  escena benchmark "go run ./cmd/benchmark -workers 1,2,4,8,12 -repes 3"
fi
if quiere tests "${pedidas[@]}"; then
  escena tests "go test -race -count=1 ./..."
fi
if quiere docker "${pedidas[@]}"; then
  escena docker "docker build --no-cache -t aire-distribuido . && docker run --rm --user \"\$(id -u):\$(id -g)\" -v \"\$PWD/data:/datos\" aire-distribuido cargar -entrada /datos/sample/oefa_real_muestra.csv -silver /datos/silver/muestra_ancho.csv -gold /datos/gold/muestra_features.csv -reports /datos/reports -workers 2"
fi
