#!/usr/bin/env bash
# Arma la carpeta del nodo para los Windows del laboratorio: compila nodo.exe
# para Windows, copia el gold junto a él (el nodo lo busca ahí), copia
# EJECUTAR-NODO.bat y escribe coordinador.txt.
#
#   lab/armar-usb.sh DESTINO [IP:PUERTO]
#   GOLD=/ruta/oefa_features.csv lab/armar-usb.sh DESTINO [IP:PUERTO]
#
# DESTINO suele ser una carpeta del USB (se crea si no existe). Sin IP:PUERTO
# usa la IP con la que esta máquina sale a la red y el puerto 7070, que es el
# que usan cmd/smoke y cmd/distribuido por defecto. GOLD es el gold que se
# copia; por omisión, data/gold/oefa_features.csv del repo. Tiene que ser el
# mismo archivo que usa el coordinador: el nodo compara su sha256 con la huella
# del CARGAR. GOLD=ninguno arma un USB sin gold, que solo sirve para el smoke test.
set -euo pipefail

if [[ $# -lt 1 || $# -gt 2 ]]; then
	echo "uso: [GOLD=ruta] $0 DESTINO [IP:PUERTO]" >&2
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
gold=${GOLD:-$raiz/data/gold/oefa_features.csv}
if [[ $gold != ninguno && ! -f $gold ]]; then
	echo "no encuentro el gold en $gold: córrelo con cmd/cargar, pásalo con GOLD=/ruta/oefa_features.csv" >&2
	echo "o arma un USB solo para el smoke test con GOLD=ninguno" >&2
	exit 1
fi

mkdir -p "$destino"
(cd "$raiz" && GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
	go build -trimpath -ldflags "-s -w" -o "$destino/nodo.exe" ./cmd/nodo)
# Windows espera CRLF en los .bat; coordinador.txt lo lee `set /p`.
sed 's/\r$//; s/$/\r/' "$raiz/lab/EJECUTAR-NODO.bat" > "$destino/EJECUTAR-NODO.bat"
printf '%s\r\n' "$coord" > "$destino/coordinador.txt"
archivos=(nodo.exe EJECUTAR-NODO.bat coordinador.txt)
if [[ $gold == ninguno ]]; then
	if [[ -f $destino/oefa_features.csv ]]; then
		echo "aviso: $destino ya tiene un oefa_features.csv de antes; EJECUTAR-NODO.bat lo va a copiar" >&2
	fi
else
	cp "$gold" "$destino/oefa_features.csv"
	archivos+=(oefa_features.csv)
fi
sync

echo "listo en $destino (coordinador $coord):"
(cd "$destino" && sha256sum "${archivos[@]}")
if [[ $gold != ninguno ]]; then
	echo "la huella del gold (sha256 de oefa_features.csv) tiene que ser la que imprime el coordinador"
fi
