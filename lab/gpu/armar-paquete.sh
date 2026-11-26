#!/usr/bin/env bash
# Arma, desde Linux, el Python portable con PyTorch CUDA para los Windows del
# laboratorio y deja lista la carpeta que sirve lab/gpu/servidor.py.
#
#   lab/gpu/armar-paquete.sh [DIR]        # por defecto ~/.cache/gpu-lab
#
# Resultado en DIR/servir: python-torch.zip (Python 3.12 + torch 2.14.1+cu126 +
# numpy + el runtime de Visual C++), gpu.bat, kmeans_gpu.py, el gold y los JSON de
# Go con los que se compara. Necesita uv, curl, bsdtar (libarchive) y python3. La
# primera vez baja unos 2.6 GB (la rueda de torch); uv la deja en DIR/uv-cache.
#
# Por qué cu126: corre con cualquier driver de NVIDIA desde el 528 y trae código
# para sm_75 (la T1000). Por qué el runtime de Visual C++ adentro: torch carga
# msvcp140, msvcp140_atomic_wait y vcruntime140_threads (14.42 o más nuevo), que
# Python no trae; un PC sin el redistribuible instalado fallaría en `import torch`.
# Se sacan del instalador oficial de Microsoft, no de un paquete de terceros.
set -euo pipefail

DIR=${1:-$HOME/.cache/gpu-lab}
PY_URL="https://github.com/astral-sh/python-build-standalone/releases/download/20261110/cpython-3.12.16+20261110-x86_64-pc-windows-msvc-install_only.tar.gz"
VC_URL="https://aka.ms/vs/17/release/vc_redist.x64.exe"
raiz=$(cd "$(dirname "$0")/../.." && pwd)
paquete="$DIR/paquete"
servir="$DIR/servir"
mkdir -p "$DIR/descargas" "$paquete" "$servir/resultados"

echo "1/5 Python portable para Windows"
[[ -f $DIR/descargas/python.tar.gz ]] || curl -fsSL -o "$DIR/descargas/python.tar.gz" "$PY_URL"
rm -rf "$paquete/python"
tar -xzf "$DIR/descargas/python.tar.gz" -C "$paquete"

echo "2/5 torch 2.14.1+cu126 y numpy, cruzados para win_amd64"
UV_CACHE_DIR="$DIR/uv-cache" uv pip install -q \
	--python-platform x86_64-pc-windows-msvc --python-version 3.12 \
	--target "$paquete/python/Lib/site-packages" \
	--index-url https://pypi.org/simple --extra-index-url https://download.pytorch.org/whl/cu126 \
	--index-strategy unsafe-best-match \
	"torch==2.14.1+cu126" numpy
rm -rf "$paquete/python/Lib/site-packages/bin" # lanzadores de Linux con rutas de esta máquina

echo "3/5 runtime de Visual C++ del instalador oficial"
[[ -f $DIR/descargas/vc_redist.x64.exe ]] || curl -fsSL -o "$DIR/descargas/vc_redist.x64.exe" "$VC_URL"
python3 -I - "$DIR/descargas/vc_redist.x64.exe" "$paquete/python" <<'EOF'
# El instalador es un ejecutable con un CAB pegado; adentro hay más CAB, uno con las
# DLL de x64 llamadas "<nombre>.dll_amd64". Se recortan por su firma MSCF y se
# extraen con bsdtar.
import shutil, struct, subprocess, sys, tempfile
from pathlib import Path

QUIERO = ["msvcp140.dll", "msvcp140_1.dll", "msvcp140_2.dll", "msvcp140_atomic_wait.dll",
          "msvcp140_codecvt_ids.dll", "vcruntime140.dll", "vcruntime140_1.dll",
          "vcruntime140_threads.dll", "concrt140.dll", "vcomp140.dll"]

def cabs(datos):
    i = 0
    while (i := datos.find(b"MSCF\0\0\0\0", i)) >= 0:
        tam = struct.unpack_from("<I", datos, i + 8)[0]
        if 0 < tam <= len(datos) - i:
            yield datos[i:i + tam]
        i += 4

def version(p):
    d = p.read_bytes()
    i = d.find(struct.pack("<I", 0xFEEF04BD))
    ms, ls = struct.unpack_from("<II", d, i + 8)
    return (ms >> 16, ms & 0xFFFF, ls >> 16)

destino = Path(sys.argv[2])
with tempfile.TemporaryDirectory() as tmp:
    tmp = Path(tmp)
    pendientes = [Path(sys.argv[1]).read_bytes()]
    while pendientes:
        for n, cab in enumerate(cabs(pendientes.pop())):
            d = tmp / f"c{len(list(tmp.iterdir()))}"
            d.mkdir()
            (d / "x.cab").write_bytes(cab)
            subprocess.run(["bsdtar", "-xf", "x.cab"], cwd=d, check=True)
            for f in d.rglob("*"):
                if f.is_file() and f.name != "x.cab" and f.read_bytes()[:4] == b"MSCF":
                    pendientes.append(f.read_bytes())
    for nombre in QUIERO:
        origen = next(tmp.rglob(nombre + "_amd64"), None)
        if origen is None:
            sys.exit(f"no encontré {nombre} en el instalador")
        if version(origen) < (14, 42, 0):
            sys.exit(f"{nombre} es {version(origen)}: torch pide 14.42 o más nuevo")
        shutil.copyfile(origen, destino / nombre)
    print(f"    {len(QUIERO)} DLL, versión {'.'.join(map(str, version(destino / 'msvcp140.dll')))}")
EOF

echo "4/5 zip (deflate rápido; las DLL que no comprimen van sin comprimir)"
python3 -I - "$paquete" "$servir/python-torch.zip" <<'EOF'
import os, sys, zipfile, zlib
from pathlib import Path
src, out = Path(sys.argv[1]), Path(sys.argv[2])
tmp = out.with_suffix(".zip.part")
with zipfile.ZipFile(tmp, "w", allowZip64=True) as zf:
    for dp, dn, fn in os.walk(src / "python"):
        dn.sort(); fn.sort()
        for f in fn:
            p = Path(dp) / f
            muestra = p.open("rb").read(4 << 20)
            comprime = len(muestra) < 65536 or len(zlib.compress(muestra, 1)) < 0.9 * len(muestra)
            zf.write(p, p.relative_to(src).as_posix(),
                     compress_type=zipfile.ZIP_DEFLATED if comprime else zipfile.ZIP_STORED, compresslevel=1)
os.replace(tmp, out)
print(f"    {out.stat().st_size / 2**30:.2f} GiB")
EOF
(cd "$servir" && sha256sum python-torch.zip > python-torch.sha256)

echo "5/5 archivos que se sirven"
sed 's/\r$//; s/$/\r/' "$raiz/lab/gpu/gpu.bat" > "$servir/gpu.bat"
cp "$raiz/lab/gpu/kmeans_gpu.py" "$raiz/data/gold/oefa_features.csv" \
	"$raiz/reports/kmeans.json" "$raiz/reports/centroides_iniciales.json" "$servir/"

echo "listo. Para servirlo en la red del laboratorio:"
echo "  python3 lab/gpu/servidor.py --dir $servir --bind <IP del cable> --puerto 8090"
