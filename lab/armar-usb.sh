#!/usr/bin/env bash
# Arma la carpeta del nodo para los Windows del laboratorio: compila nodo.exe
# para Windows, copia EJECUTAR-NODO.bat y escribe coordinador.txt.
#
#   lab/armar-usb.sh DESTINO [IP:PUERTO]
#
# DESTINO suele ser una carpeta del USB (se crea si no existe). Sin IP:PUERTO
# usa la IP con la que esta máquina sale a la red y el puerto 7070, que es el
# que usa cmd/smoke por defecto.
set -euo pipefail

if [[ $# -lt 1 || $# -gt 2 ]]; then
	echo "uso: $0 DESTINO [IP:PUERTO]" >&2
	exit 2
fi
destino=$1
coord=${2:-}
if [[ -z $coord ]]; then
	ip=$(ip -4 route get 1.1.1.1 | awk '{for (i = 1; i < NF; i++) if ($i == "src") print $(i + 1)}')
	[[ -n $ip ]] || { echo "no pude averiguar la IP local; pásala como IP:PUERTO" >&2; exit 1; }
	coord="$ip:7070"
fi

raiz=$(cd "$(dirname "$0")/.." && pwd)
mkdir -p "$destino"
(cd "$raiz" && GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
	go build -trimpath -ldflags "-s -w" -o "$destino/nodo.exe" ./cmd/nodo)
# Windows espera CRLF en los .bat; coordinador.txt lo lee `set /p`.
sed 's/\r$//; s/$/\r/' "$raiz/lab/EJECUTAR-NODO.bat" > "$destino/EJECUTAR-NODO.bat"
printf '%s\r\n' "$coord" > "$destino/coordinador.txt"
sync

echo "listo en $destino (coordinador $coord):"
(cd "$destino" && sha256sum nodo.exe EJECUTAR-NODO.bat coordinador.txt)
