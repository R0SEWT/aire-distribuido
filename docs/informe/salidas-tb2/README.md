# Salidas reales del TB2 para el informe

Salidas de terminal en texto, copiadas tal como salieron (con los tokens tapados). `tablas.py` las lee
(`SALIDAS_TB2`, `series_prueba` y `latencia_tb2`) y las pone en el informe como listados de texto, con sus
cifras. Se tomaron el 27/11/2026 desde la laptop de desarrollo, sobre el commit `6d91f4f` de `develop`.

| Archivo | Qué es | Cómo se repite |
|---|---|---|
| `verificar-series.txt` | Las 1 086 lecturas de `web/public/datos/series.json` mandadas a la API real y comparadas con lo esperado, calculado sin la API | `JWT_SECRETO=$(openssl rand -hex 32) go run ./cmd/api -demo -addr 127.0.0.1:18093 -nodos-addr ""` y `python3 scripts/verificar_series_api.py --api http://127.0.0.1:18093` |
| `metricas-verificacion.txt` | `GET /api/metricas` de esa misma API al terminar: la latencia de la predicción medida dentro del servidor (las bases en memoria) | `curl` con un token de administrador de `-demo` |
| `seguridad.txt` | Las cabeceras de Caddy y las políticas de la API (CORS, sin token, invitado y su rol) probadas con `curl` contra el despliegue público, <https://aire.rosewt.dev> | Los `curl` que encabezan cada bloque; el token del invitado va tapado |
| `latencia-remota.txt` | Dos rutas de lectura del despliegue público medidas desde la laptop, sobre una conexión TLS ya abierta: el tiempo es casi todo de red | `python3 latencia_remota.py aire.rosewt.dev 50` (`latencia_remota.py` está en esta carpeta) |

Las salidas de la sesión del frontend (latencia con `cmd/carga` directo y por Caddy, del pedido al evento
del WebSocket, las pruebas de la interfaz, `docker compose ps` y las políticas contra el Compose local)
están en `../capturas/tb2/`, con su índice en `../capturas/tb2/README.md`.
