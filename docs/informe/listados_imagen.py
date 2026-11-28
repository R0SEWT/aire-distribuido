#!/usr/bin/env python3
"""Pasa a imagen los listados del informe.

tablas.py escribe cada fragmento de código (generado/tablas/frag_*.md) y cada salida de terminal
(salida_*.md) como un bloque de texto. En Word una línea larga se parte y el listado se lee mal, así
que este paso, que render.sh corre después de tablas.py, dibuja cada bloque como se ve en un editor o
en una terminal (generado/imagenes/*.png) y deja en el .md la imagen en lugar del bloque. El texto
sigue estando en el repositorio: el código en su archivo y la salida en capturas/pc4/*.txt.

Necesita pygments y Chrome (google-chrome) para fotografiar la página.
"""
import html
import math
import re
import subprocess
import sys
import tempfile
from pathlib import Path

from pygments import highlight
from pygments.formatters import HtmlFormatter
from pygments.lexers import get_lexer_by_name

AQUI = Path(__file__).resolve().parent
TABLAS = AQUI / "generado" / "tablas"
IMAGENES = AQUI / "generado" / "imagenes"
FONDO, BARRA, TEXTO, TENUE, VERDE = "#0d1117", "#161b22", "#e6edf3", "#9da7b3", "#7ee787"
MONO = "'DejaVu Sans Mono','Liberation Mono',monospace"
ANCHO_CARACTER, ALTO_LINEA = 8.43, 21  # DejaVu Sans Mono a 14 px
COLUMNAS_TERMINAL = 104


def foto(pagina: str, destino: Path, ancho: int, alto: int) -> None:
    with tempfile.NamedTemporaryFile("w", suffix=".html", delete=False, encoding="utf-8") as f:
        f.write(pagina)
    subprocess.run(["google-chrome", "--headless=new", "--disable-gpu", "--hide-scrollbars", "--no-sandbox",
                    "--force-device-scale-factor=2", f"--screenshot={destino}", f"--window-size={ancho},{alto}",
                    f"file://{f.name}"], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    Path(f.name).unlink()


def pagina(titulo: str, cuerpo: str, extra: str = "") -> str:
    return f"""<!doctype html><meta charset="utf-8"><style>
body{{margin:0;background:{FONDO};color:{TEXTO};font:14px/{ALTO_LINEA}px {MONO}}}
header{{background:{BARRA};border-bottom:1px solid #30363d;padding:8px 16px;font:600 13px/20px 'DejaVu Sans',sans-serif;color:{TENUE}}}
pre{{margin:0;padding:12px 16px;font:inherit}} .o{{color:{VERDE};font-weight:600}}
td.linenos pre,.linenos{{color:#6e7681}} table{{border-spacing:0}} td{{padding:0}} td.linenos pre{{padding-right:4px}}
{extra}</style><header>{html.escape(titulo)}</header>{cuerpo}"""


def codigo(nombre: str, lenguaje: str, texto: str, ruta: str, pie: str) -> float:
    lineas = texto.expandtabs(4).split("\n")
    # Con un solo tramo seguido se numeran las líneas como en el archivo.
    m = re.search(r"líneas? (\d+)(?:–(\d+))?\)", pie)
    numerar = bool(m) and "y " not in pie[m.start() - 40:m.start()] and int(m.group(2) or m.group(1)) - int(m.group(1)) + 1 == len(lineas)
    lexer = get_lexer_by_name(lenguaje if lenguaje not in ("text", "") else "text")
    formato = HtmlFormatter(noclasses=True, style="github-dark", nobackground=True,
                            linenos="inline" if numerar else False, linenostart=int(m.group(1)) if numerar else 1)
    cuerpo = highlight("\n".join(lineas), lexer, formato)
    columnas = max(len(l) for l in lineas) + (6 if numerar else 0)
    ancho = max(640, math.ceil(columnas * ANCHO_CARACTER) + 40)
    alto = 37 + 24 + len(lineas) * ALTO_LINEA + 8
    foto(pagina(ruta, cuerpo, "pre{line-height:%dpx}" % ALTO_LINEA), IMAGENES / f"{nombre}.png", ancho, alto)
    return ancho


def terminal(nombre: str, texto: str, origen: str) -> float:
    # tablas.py parte las líneas largas con « \»: aquí se vuelven a unir y la imagen las dobla sola.
    unidas: list[str] = []
    for l in texto.split("\n"):
        if unidas and unidas[-1].endswith(" \\") and l.startswith("    "):
            anterior = unidas[-1][:-2]
            unidas[-1] = anterior + ("" if anterior.endswith("_") else " ") + l.strip()
        else:
            unidas.append(l)
    filas, partes = 0, []
    for l in unidas:
        filas += max(1, math.ceil(len(l) / COLUMNAS_TERMINAL))
        e = html.escape(l)
        partes.append(f'<span class="o">{e}</span>' if l.startswith("$ ") else e)
    ancho = math.ceil(COLUMNAS_TERMINAL * ANCHO_CARACTER) + 40
    alto = 37 + 24 + filas * ALTO_LINEA + 8
    cuerpo = "<pre>" + "\n".join(partes) + "</pre>"
    foto(pagina("terminal · " + origen, cuerpo, "pre{white-space:pre-wrap;word-break:break-all}"),
         IMAGENES / f"{nombre}.png", ancho, alto)
    return ancho


def de_la_pc4() -> set[str]:
    """Los listados que incluye el informe de la PC4: solo esos pasan a imagen. Los del TB2 van como texto
    (el docente observó en la PC4 que en una imagen no se puede buscar el código)."""
    aqui = TABLAS.parent.parent
    fuentes = [aqui / "informe-pc4.qmd", *sorted((aqui / "secciones").glob("*.qmd")), *sorted((aqui / "secciones" / "pc4").glob("*.qmd"))]
    texto = "\n".join(f.read_text(encoding="utf-8") for f in fuentes if f.exists())
    return set(re.findall(r"generado/tablas/((?:frag|salida)_\w+)\.md", texto))


def listados() -> int:
    hechos = 0
    incluidos = de_la_pc4()
    for md in sorted(list(TABLAS.glob("frag_*.md")) + list(TABLAS.glob("salida_*.md"))):
        if md.stem not in incluidos:
            continue
        s = md.read_text(encoding="utf-8")
        m = re.search(r"```(\w*)\n(.*?)\n```\n", s, re.S)
        if not m or "PENDIENTE" in s:
            continue
        origen = (re.search(r"desde (\S+?)\. No editar", s) or [None, md.stem])[1]
        pie = s[m.end():]
        if md.name.startswith("frag_"):
            ancho = codigo(md.stem, m.group(1), m.group(2), origen, pie)
        else:
            ancho = terminal(md.stem, m.group(2), origen)
        cm = min(16.0, round(16.0 * ancho / 920, 1))
        s = s[:m.start()] + f"![](generado/imagenes/{md.stem}.png){{width={cm}cm}}\n" + s[m.end():]
        md.write_text(s, encoding="utf-8")
        hechos += 1
    return hechos


if __name__ == "__main__":
    IMAGENES.mkdir(parents=True, exist_ok=True)
    print(f"listados_imagen.py: {listados()} listados pasados a imagen en generado/imagenes/")
