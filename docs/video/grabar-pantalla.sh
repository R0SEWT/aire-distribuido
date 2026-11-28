#!/usr/bin/env bash
# Graba la SPA de punta a punta para el video del TB2 con Playwright (docs/video/pantalla/grabar.mjs),
# en docs/video/build/tb2/pantalla/*.mp4, contra un Docker Compose propio y limpio.
#
#   bash docs/video/grabar-pantalla.sh                 # Compose limpio y las seis escenas
#   bash docs/video/grabar-pantalla.sh impacto swagger # solo esas (no reinicia el Compose)
#   AIRE_URL=https://aire.rosewt.dev bash docs/video/grabar-pantalla.sh impacto   # sitio público, solo lectura
#
# El Compose del video es el proyecto `aire-tb2-video` en otros puertos (UI 8091, API 8090, Mongo 27027,
# Redis 6389) para no chocar con otro levantado en los puertos por omisión. Lee el .env del repo
# (JWT_SECRETO, API_USUARIOS con un admin, y WEB_PORT=8091 etc.; ver .env.example) y el gold de GOLD_DIR.
# El escenario se graba sobre un sistema sin estados: por eso, sin escenas pedidas, el Compose se baja
# con `down -v` (borra sus volúmenes, no los de otro proyecto) y se vuelve a levantar con tres nodos.
#
# Requiere Node 20+, Docker, ffmpeg y Google Chrome (/usr/bin/google-chrome, o CHROME=ruta).
set -euo pipefail
cd "$(dirname "$0")/../.."

PROYECTO=${PROYECTO:-aire-tb2-video}
export AIRE_URL=${AIRE_URL:-http://localhost:${WEB_PORT:-8091}}
export FFMPEG=${FFMPEG:-$(command -v ffmpeg || command -v static_ffmpeg)}

if [ ! -d docs/video/pantalla/node_modules ]; then
  (cd docs/video/pantalla && npm install --no-audit --no-fund)
fi

local_=0
case "$AIRE_URL" in http://localhost:*|http://127.0.0.1:*) local_=1 ;; esac

if [ "$local_" = 1 ] && [ $# -eq 0 ]; then
  if [ ! -f .env ] || ! grep -q '^WEB_PORT=8091' .env; then
    echo "falta el .env del video: cp .env.example .env, completar JWT_SECRETO y API_USUARIOS (con un admin)" >&2
    echo "y poner WEB_PORT=8091 API_PORT=8090 MONGO_PORT=27027 REDIS_PORT=6389 GOLD_DIR=<carpeta del gold>" >&2
    exit 1
  fi
  echo "Compose limpio: $PROYECTO (down -v y up con 3 nodos)"
  docker compose -p "$PROYECTO" down -v --remove-orphans
  docker compose -p "$PROYECTO" up -d --build --scale nodo=3
  for _ in $(seq 60); do
    salud=$(curl -fsS "$AIRE_URL/api/salud" 2>/dev/null || true)
    case "$salud" in *'"nodos_en_pool":3'*) break ;; esac
    sleep 2
  done
  echo "  salud: ${salud:-sin respuesta}"
fi

node docs/video/pantalla/grabar.mjs "$@"
ls -la docs/video/build/tb2/pantalla/*.mp4
