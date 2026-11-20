"""Sirve el experimento de GPU a los PCs del laboratorio y junta sus resultados.

    python3 lab/gpu/servidor.py --dir ~/.cache/gpu-lab/servir --bind 10.11.37.73 --puerto 8090

GET  /                         página con cómo lanzarlo y la tabla de resultados
GET  /<archivo>                gpu.bat, kmeans_gpu.py, el gold, python-torch.zip
PUT  /resultados/<nombre>.json lo sube cada PC al terminar (gpu.bat lo hace solo)

Solo biblioteca estándar. Escucha en la IP que se le da (la del cable del lab), y
solo acepta escrituras de JSON chicos en resultados/.
"""

from __future__ import annotations

import argparse
import html
import json
import re
from functools import partial
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

NOMBRE_VALIDO = re.compile(r"^[A-Za-z0-9_.-]{1,80}\.json$")
MAX_BYTES = 1 << 20


class Manejador(SimpleHTTPRequestHandler):
    servidor_url = ""

    def do_GET(self):
        if self.path in ("/", "/index.html"):
            cuerpo = self.pagina().encode("utf-8")
            self.send_response(200)
            self.send_header("Content-Type", "text/html; charset=utf-8")
            self.send_header("Content-Length", str(len(cuerpo)))
            self.end_headers()
            self.wfile.write(cuerpo)
            return
        super().do_GET()

    def do_PUT(self):
        partes = self.path.split("/")
        if len(partes) != 3 or partes[1] != "resultados" or not NOMBRE_VALIDO.match(partes[2]):
            self.send_error(403, "solo se aceptan /resultados/<nombre>.json")
            return
        largo = int(self.headers.get("Content-Length", 0))
        if not 0 < largo <= MAX_BYTES:
            self.send_error(413, "resultado vacío o demasiado grande")
            return
        datos = self.rfile.read(largo)
        try:
            json.loads(datos)
        except ValueError:
            self.send_error(400, "no es JSON")
            return
        destino = Path(self.directory) / "resultados" / partes[2]
        destino.parent.mkdir(exist_ok=True)
        destino.write_bytes(datos)
        self.send_response(201)
        self.send_header("Content-Length", "0")
        self.end_headers()

    def pagina(self) -> str:
        filas = []
        for f in sorted((Path(self.directory) / "resultados").glob("*.json")):
            try:
                filas.append(self.fila(json.loads(f.read_text(encoding="utf-8"))))
            except (ValueError, KeyError, IndexError, TypeError):
                continue
        linea = f"cmd /k curl.exe -fsSo %TEMP%\\gpu.bat {self.servidor_url}/gpu.bat &amp;&amp; %TEMP%\\gpu.bat"
        return f"""<!doctype html><html lang="es"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1"><meta http-equiv="refresh" content="15">
<title>K-Means en GPU</title>
<style>body{{font:16px system-ui,sans-serif;max-width:1000px;margin:24px auto;padding:0 16px}}
code{{background:#eee;padding:2px 6px;border-radius:4px;word-break:break-all}}
table{{border-collapse:collapse;width:100%}}td,th{{border-bottom:1px solid #ddd;padding:6px;text-align:left}}</style></head>
<body><h1>aire-distribuido: K-Means del gold en la GPU</h1>
<p><b>Para lanzarlo en este PC:</b> Win+R, pega esta línea y Enter:</p>
<p><code>{linea}</code></p>
<p>O baja <a href="gpu.bat">gpu.bat</a> y ábrelo (Windows puede pedir confirmación por venir de la red).</p>
<h2>Resultados ({len(filas)})</h2>
<table><tr><th>Equipo</th><th>Dispositivo</th><th>Iteraciones (Go: 44)</th><th>fp64 vs Go</th>
<th>Suma atómica fp64 repetible</th><th>ms/iter gold fp64</th><th>Mayor tamaño</th><th>Mfilas/s fp32</th></tr>{"".join(filas)}</table>
<p><small>La página se actualiza sola cada 15 s.</small></p></body></html>"""

    @staticmethod
    def fila(r: dict) -> str:
        conv = {(c["precision"], c["suma"]): c for c in r["convergencia"]}
        f64 = conv[("float64", "matmul")]
        atom = conv[("float64", "atomica")]
        vel = [v for v in r["velocidad"] if "error" not in v]
        gold = next(v for v in vel if v["replicas"] == 1 and v["precision"] == "float64" and v["suma"] == "matmul")
        mayor = max((v for v in vel if v["precision"] == "float32"), key=lambda v: (v["filas"], v["millones_filas_por_s"]))
        celdas = [
            r["equipo"], r["dispositivo"], str(f64["iteraciones"]), f"{f64['dif_max_centroides_go']:.1e}",
            "sí" if atom["repetible_bit_a_bit"] else "NO", f"{gold['ms_por_iteracion']:.3f}",
            f"{mayor['filas']:,} filas", f"{mayor['millones_filas_por_s']:,.0f}",
        ]
        return "<tr>" + "".join(f"<td>{html.escape(c)}</td>" for c in celdas) + "</tr>"


def main() -> None:
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("--dir", type=Path, required=True)
    p.add_argument("--bind", required=True)
    p.add_argument("--puerto", type=int, default=8090)
    a = p.parse_args()
    (a.dir / "resultados").mkdir(parents=True, exist_ok=True)
    Manejador.servidor_url = f"http://{a.bind}:{a.puerto}"
    servidor = ThreadingHTTPServer((a.bind, a.puerto), partial(Manejador, directory=str(a.dir)))
    print(f"sirviendo {a.dir} en {Manejador.servidor_url}/", flush=True)
    servidor.serve_forever()


if __name__ == "__main__":
    main()
