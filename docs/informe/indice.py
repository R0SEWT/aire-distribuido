"""Llena el índice de los .docx con sus entradas y sus páginas, para que no salga vacío fuera de Word.

    python3 indice.py salida/A.docx [salida/B.docx ...]

El índice es un campo TOC de Word (lo pone filtros/informe.lua). Word lo llena al abrir el archivo, porque
render.sh marca w:updateFields, pero LibreOffice, el visor del Aula Virtual o un PDF exportado sin actualizar
muestran solo el texto de reemplazo: un índice vacío. Este script escribe en el resultado del campo una
entrada por título (Heading 1 a 3), con su número y su página. El campo sigue siendo un TOC marcado para
actualizar, así que Word lo vuelve a calcular con su propia paginación; fuera de Word se ve lo que escribe
este script.

Las páginas se miden convirtiendo el primer .docx a PDF con LibreOffice, en dos pasadas: primero se escriben
las entradas con un número de relleno (el índice ya ocupa sus páginas) y se mide; después se ponen los
números. Los demás .docx tienen la misma estructura (solo cambia el nombre de la carátula) y reciben las
mismas páginas. Sin soffice o sin pdftotext avisa y deja los .docx como estaban.
"""

from __future__ import annotations

import os
import re
import shutil
import subprocess
import sys
import tempfile
import zipfile

NIVELES = {"Heading1": 1, "Heading2": 2, "Heading3": 3}
PARRAFO = re.compile(r"<w:p\b[^>]*>(?:(?!</w:p>).)*?</w:p>", re.S)


def leer(ruta: str) -> tuple[list[zipfile.ZipInfo], dict[str, bytes]]:
    with zipfile.ZipFile(ruta) as z:
        infos = z.infolist()
        return infos, {i.filename: z.read(i.filename) for i in infos}


def escribir(ruta: str, infos: list[zipfile.ZipInfo], partes: dict[str, bytes]) -> None:
    fd, tmp = tempfile.mkstemp(dir=os.path.dirname(ruta) or ".", suffix=".docx")
    os.close(fd)
    with zipfile.ZipFile(tmp, "w", zipfile.ZIP_DEFLATED) as z:
        for i in infos:
            z.writestr(i, partes[i.filename])
    os.replace(tmp, ruta)


def titulos(xml: str) -> list[tuple[int, str, str | None]]:
    """(nivel, texto ya escapado como XML, marcador) de cada título, en orden."""
    salida = []
    for p in PARRAFO.findall(xml):
        m = re.search(r'<w:pStyle w:val="(Heading[123])"', p)
        if not m:
            continue
        texto = "".join(re.findall(r"<w:t(?: [^>]*)?>([^<]*)</w:t>", p)).strip()
        if not texto:
            continue
        marca = re.search(r'<w:bookmarkStart [^>]*w:name="([^"]+)"', p)
        salida.append((NIVELES[m.group(1)], texto, marca.group(1) if marca else None))
    return salida


def ancho_texto(xml: str) -> int:
    """Ancho útil de la página en twips, para el tabulador de las páginas."""
    pg = re.search(r'<w:pgSz [^>]*w:w="(\d+)"', xml)
    mar = re.search(r'<w:pgMar [^>]*>', xml)
    if not pg or not mar:
        return 9071  # 16 cm
    izq = re.search(r'w:left="(\d+)"', mar.group(0))
    der = re.search(r'w:right="(\d+)"', mar.group(0))
    return int(pg.group(1)) - int(izq.group(1) if izq else 1440) - int(der.group(1) if der else 1440)


def entradas(lista, paginas, pos: int) -> list[str]:
    ps = []
    for (nivel, texto, marca), pagina in zip(lista, paginas):
        corridas = (f'<w:r><w:t xml:space="preserve">{texto}</w:t></w:r><w:r><w:tab/></w:r>'
                    f'<w:r><w:t>{pagina}</w:t></w:r>')
        if marca:
            corridas = f'<w:hyperlink w:anchor="{marca}" w:history="1">{corridas}</w:hyperlink>'
        ps.append(f'<w:pPr><w:pStyle w:val="TOC{nivel}"/><w:tabs><w:tab w:val="right" w:leader="dot" w:pos="{pos}"/></w:tabs>'
                  f'<w:spacing w:before="0" w:after="40"/><w:ind w:left="{(nivel - 1) * 284}"/></w:pPr>{corridas}')
    return ps


def con_indice(xml: str, lista, paginas) -> str:
    """El document.xml con el resultado del campo TOC reemplazado por las entradas."""
    campo = next((p for p in PARRAFO.findall(xml) if 'w:fldCharType="begin"' in p and "TOC \\o" in p), None)
    if campo is None:
        raise ValueError("no hay un campo TOC en el documento")
    instr = re.search(r"<w:instrText[^>]*>[^<]*</w:instrText>", campo).group(0)
    ps = entradas(lista, paginas, ancho_texto(xml))
    inicio = ('<w:r><w:fldChar w:fldCharType="begin" w:dirty="true"/></w:r>'
              f"<w:r>{instr}</w:r><w:r><w:fldChar w:fldCharType=\"separate\"/></w:r>")
    fin = '<w:r><w:fldChar w:fldCharType="end"/></w:r>'
    nuevos = []
    for i, p in enumerate(ps):
        ppr, corridas = p.split("</w:pPr>", 1)
        nuevos.append(f"<w:p>{ppr}</w:pPr>{inicio if i == 0 else ''}{corridas}{fin if i == len(ps) - 1 else ''}</w:p>")
    return xml.replace(campo, "".join(nuevos), 1)


def normal(s: str) -> str:
    s = s.replace("&amp;", "&").replace("&lt;", "<").replace("&gt;", ">").replace("&quot;", '"').replace("&apos;", "'")
    return re.sub(r"\s+", " ", s.replace("­", "")).strip()


def medir(docx: str, lista) -> list[int]:
    """Las páginas de cada título en el PDF que da LibreOffice."""
    with tempfile.TemporaryDirectory() as tmp:
        subprocess.run(["soffice", "--headless", "--convert-to", "pdf", "--outdir", tmp, docx],
                       check=True, capture_output=True, timeout=600)
        pdf = os.path.join(tmp, os.path.splitext(os.path.basename(docx))[0] + ".pdf")
        texto = subprocess.run(["pdftotext", "-layout", pdf, "-"], check=True, capture_output=True, text=True).stdout
    paginas = [normal(p) for p in texto.split("\f")]
    # El índice repite los títulos: los que van después de él se buscan desde la página que sigue a su última
    # entrada; los de antes (el resumen), antes de la página del índice.
    ultimo = normal(lista[-1][1])[:50]
    primero_despues = next(i for i, (_, t, _) in enumerate(lista) if t.startswith("1"))
    pag_indice = next(i for i, p in enumerate(paginas) if "Índice" in p[:200])
    fin_indice = next(i for i in range(pag_indice, len(paginas)) if ultimo in paginas[i])
    resultado, desde = [], 0
    for j, (_, t, _) in enumerate(lista):
        clave = normal(t)[:50]
        if j == primero_despues:
            desde = fin_indice + 1
        tope = pag_indice if j < primero_despues else len(paginas)
        pagina = next((i for i in range(desde, tope) if clave in paginas[i]), None)
        if pagina is None:
            print(f"aviso: indice.py no encontró «{clave}» en el PDF; se usa la página anterior", file=sys.stderr)
            pagina = desde
        resultado.append(pagina + 1)
        desde = pagina
    return resultado


def main(rutas: list[str]) -> int:
    if not rutas:
        print(__doc__.split("\n\n")[1], file=sys.stderr)
        return 2
    if not shutil.which("soffice") or not shutil.which("pdftotext"):
        print("aviso: indice.py necesita soffice (LibreOffice) y pdftotext: el índice queda para Word", file=sys.stderr)
        return 0
    infos, partes = leer(rutas[0])
    xml = partes["word/document.xml"].decode("utf-8")
    lista = titulos(xml)
    # Primera pasada: entradas con un número de relleno del mismo ancho que las páginas reales.
    relleno = ["000"] * len(lista)
    with tempfile.TemporaryDirectory() as tmp:
        prueba = os.path.join(tmp, os.path.basename(rutas[0]))
        partes_prueba = dict(partes)
        partes_prueba["word/document.xml"] = con_indice(xml, lista, relleno).encode("utf-8")
        escribir(prueba, infos, partes_prueba)
        paginas = medir(prueba, lista)
    for ruta in rutas:
        infos_r, partes_r = leer(ruta)
        xml_r = partes_r["word/document.xml"].decode("utf-8")
        lista_r = titulos(xml_r)
        if [t for _, t, _ in lista_r] != [t for _, t, _ in lista]:
            print(f"aviso: {ruta} no tiene los mismos títulos que {rutas[0]}: su índice queda para Word", file=sys.stderr)
            continue
        partes_r["word/document.xml"] = con_indice(xml_r, lista_r, paginas).encode("utf-8")
        escribir(ruta, infos_r, partes_r)
    print(f"índice: {len(lista)} entradas, páginas medidas con LibreOffice ({paginas[-1]} la última)")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
