#!/usr/bin/env bash
# Graba las ejecuciones reales de la terminal para los videos de la PC3 y del TB2 con asciinema,
# sin tocar la pantalla de nadie: cada escena corre en un pseudoterminal propio.
#
#   bash docs/video/grabar.sh            # graba las cinco escenas de la PC3 en docs/video/casts/
#   bash docs/video/grabar.sh cargar     # solo una (cargar, entrenar, benchmark, tests, docker)
#   bash docs/video/grabar.sh carga bases pruebas compose # las del TB2 (ver abajo)
#
# Las del TB2 (carga, bases, compose) corren contra el Compose del video, el proyecto
# aire-tb2-video que levanta grabar-pantalla.sh (API en el 8090): se graban después de la SPA,
# porque `carga` manda lecturas. Las claves salen del .env del repo y no aparecen en la grabación.
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

# escena NOMBRE COMANDO...: por cada comando muestra un prompt, lo "tipea" y lo ejecuta, todo grabado.
escena() {
  local nombre=$1; shift
  local guion
  guion=$(mktemp)
  : > "$guion"
  for cmd in "$@"; do
    cat >> "$guion" <<EOS
printf '\033[1;32m~/aire-distribuido\033[0m \033[1;34m\$\033[0m '
cmd=\$(cat <<'EOC'
$cmd
EOC
)
for ((i=0; i<\${#cmd}; i++)); do printf '%s' "\${cmd:i:1}"; sleep 0.012; done
printf '\n'
sleep 0.4
bash -c "\$cmd"
sleep 0.8
EOS
  done
  cat >> "$guion" <<'EOS'
printf '\n\033[1;32m~/aire-distribuido\033[0m \033[1;34m$\033[0m '
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
# --- TB2: contra el Compose del video (grabar-pantalla.sh), sin mostrar claves.
export COMPOSE_PROJECT_NAME=${COMPOSE_PROJECT_NAME:-aire-tb2-video}
API_VIDEO=${API_VIDEO:-http://localhost:8090}
clave_de() {  # la clave de un usuario de API_USUARIOS en el .env, sin imprimirla
  sed -n 's/^API_USUARIOS=//p' .env 2>/dev/null | tr ',' '\n' | awk -F: -v u="$1" '$1 == u {print $2}'
}
if quiere carga "${pedidas[@]}" && [ $# -gt 0 ]; then
  ( export CARGA_CLAVE CARGA_CLAVE_ADMIN; CARGA_CLAVE=$(clave_de demo); CARGA_CLAVE_ADMIN=$(clave_de admin)
    escena carga "go run ./cmd/carga -url $API_VIDEO -pedidos 20000 -espera-escrituras 5s -salida docs/video/build/tb2/api_latencia.json" )
fi
if quiere bases "${pedidas[@]}" && [ $# -gt 0 ]; then
  escena bases \
    "docker compose exec -T mongo mongosh --quiet aire_distribuido --eval 'for (const c of [\"estaciones\",\"predicciones\",\"modelos\",\"entrenamientos\",\"eventos_cluster\"]) print(c.padEnd(16), db.getCollection(c).countDocuments())'" \
    "docker compose exec -T redis redis-cli --scan --pattern 'estacion:*' | sort | head -4" \
    "docker compose exec -T redis redis-cli GET estacion:CA-HU-04:regimen | jq -c ."
fi
if quiere pruebas "${pedidas[@]}" && [ $# -gt 0 ]; then
  escena pruebas "go test -race -count=1 ./..."   # la de la PC3 (tests) queda como se entregó
fi
if quiere compose "${pedidas[@]}" && [ $# -gt 0 ]; then
  escena compose \
    "docker compose up -d --scale nodo=3" \
    "docker compose ps --format 'table {{.Service}}\t{{.Status}}\t{{.Ports}}'" \
    "curl -s $API_VIDEO/api/salud | jq ."
fi

if quiere docker "${pedidas[@]}"; then
  escena docker "docker build --no-cache -t aire-distribuido . && docker run --rm --user \"\$(id -u):\$(id -g)\" -v \"\$PWD/data:/datos\" aire-distribuido cargar -entrada /datos/sample/oefa_real_muestra.csv -silver /datos/silver/muestra_ancho.csv -gold /datos/gold/muestra_features.csv -reports /datos/reports -workers 2"
fi
