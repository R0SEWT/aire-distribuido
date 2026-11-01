# /// script
# requires-python = ">=3.10"
# dependencies = ["python-docx>=1.1", "pillow>=10"]
# ///
"""Genera los recursos binarios del informe: la plantilla de Word y el marcador de figura.

    uv run recursos/crear_recursos.py        # desde docs/informe/

- recursos/plantilla.docx: el `reference-doc` de Quarto. Parte del reference.docx por defecto de
  Pandoc y le cambia estilos (Arial 11, interlineado 1,5, títulos en negro, A4 con márgenes de
  2,5 cm, número de página al pie salvo en la carátula, estilos de la carátula, leyendas en cursiva,
  bibliografía con sangría francesa de APA, tabla con filete arriba y abajo de la cabecera).
- recursos/figura-pendiente.png: lo que el filtro filtros/informe.lua pone en lugar de una figura
  que todavía no existe.

Los dos archivos se versionan; este script solo hace falta para cambiar el estilo.
"""

from __future__ import annotations

import subprocess
import tempfile
from pathlib import Path

from docx import Document
from docx.enum.section import WD_SECTION
from docx.enum.style import WD_STYLE_TYPE
from docx.enum.text import WD_ALIGN_PARAGRAPH, WD_LINE_SPACING
from docx.oxml import OxmlElement
from docx.oxml.ns import qn
from docx.shared import Cm, Pt, RGBColor

AQUI = Path(__file__).resolve().parent
FUENTE = "Arial"
MONO = "Consolas"
NEGRO = RGBColor(0, 0, 0)


def referencia_pandoc(destino: Path) -> None:
    subprocess.run(
        ["quarto", "pandoc", "-o", str(destino), "--print-default-data-file", "reference.docx"],
        check=True,
    )


def fuente(estilo, nombre=FUENTE, tam=None, negrita=None, cursiva=None, color=None):
    f = estilo.font
    f.name = nombre
    # name solo fija ascii/hAnsi; los títulos de Pandoc traen fuentes del tema en rFonts.
    rpr = estilo.element.get_or_add_rPr()
    rfonts = rpr.find(qn("w:rFonts"))
    if rfonts is None:
        rfonts = OxmlElement("w:rFonts")
        rpr.insert(0, rfonts)
    for atributo in ("w:asciiTheme", "w:hAnsiTheme", "w:eastAsiaTheme", "w:cstheme"):
        rfonts.attrib.pop(qn(atributo), None)
    for atributo in ("w:ascii", "w:hAnsi", "w:cs"):
        rfonts.set(qn(atributo), nombre)
    if tam is not None:
        f.size = Pt(tam)
    if negrita is not None:
        f.bold = negrita
    if cursiva is not None:
        f.italic = cursiva
    if color is not None:
        f.color.rgb = color


def parrafo(estilo, antes=None, despues=None, interlineado=None, alineacion=None,
            con_siguiente=None, salto_antes=None):
    p = estilo.paragraph_format
    if antes is not None:
        p.space_before = Pt(antes)
    if despues is not None:
        p.space_after = Pt(despues)
    if interlineado is not None:
        p.line_spacing_rule = WD_LINE_SPACING.MULTIPLE
        p.line_spacing = interlineado
    if alineacion is not None:
        p.alignment = alineacion
    if con_siguiente is not None:
        p.keep_with_next = con_siguiente
    if salto_antes is not None:
        p.page_break_before = salto_antes


def buscar(doc, nombre):
    """Estilo por nombre, sin distinguir mayúsculas (Word guarda «caption» en minúscula)."""
    for e in doc.styles:
        if e.name and e.name.lower() == nombre.lower():
            return e
    raise KeyError(nombre)


def estilo(doc, nombre, base="Normal"):
    try:
        return buscar(doc, nombre)
    except KeyError:
        nuevo = doc.styles.add_style(nombre, WD_STYLE_TYPE.PARAGRAPH)
        nuevo.base_style = doc.styles[base]
        nuevo.quick_style = True
        return nuevo


def idioma_por_defecto(doc, lang="es-PE"):
    rpr = doc.styles.element.find(qn("w:docDefaults")).find(qn("w:rPrDefault")).find(qn("w:rPr"))
    nodo = rpr.find(qn("w:lang"))
    if nodo is None:
        nodo = OxmlElement("w:lang")
        rpr.append(nodo)
    nodo.set(qn("w:val"), lang)


def filetes_de_cabecera(doc):
    """Tabla al estilo APA: filete arriba y abajo de la fila de cabecera.

    Solo la cabecera: Quarto envuelve figuras, tablas y avisos en tablas de maquetación con el
    mismo estilo «Table» pero sin cabecera (firstRow=0), y esas no deben llevar bordes.
    """
    tabla = doc.styles["Table"].element
    for pr in tabla.findall(qn("w:tblStylePr")):
        if pr.get(qn("w:type")) == "firstRow":
            tabla.remove(pr)
    pr = OxmlElement("w:tblStylePr")
    pr.set(qn("w:type"), "firstRow")
    rpr = OxmlElement("w:rPr")
    rpr.append(OxmlElement("w:b"))
    pr.append(rpr)
    tcpr = OxmlElement("w:tcPr")
    bordes = OxmlElement("w:tcBorders")
    for lado in ("top", "bottom"):
        b = OxmlElement(f"w:{lado}")
        b.set(qn("w:val"), "single")
        b.set(qn("w:sz"), "8")
        b.set(qn("w:space"), "0")
        b.set(qn("w:color"), "000000")
        bordes.append(b)
    tcpr.append(bordes)
    valign = OxmlElement("w:vAlign")
    valign.set(qn("w:val"), "bottom")
    tcpr.append(valign)
    pr.append(tcpr)
    tabla.append(pr)


def numero_de_pagina(parrafo_pie):
    parrafo_pie.alignment = WD_ALIGN_PARAGRAPH.CENTER
    run = parrafo_pie.add_run()
    for tipo, texto in (("begin", None), (None, " PAGE "), ("separate", None), (None, "1"), ("end", None)):
        if tipo:
            fld = OxmlElement("w:fldChar")
            fld.set(qn("w:fldCharType"), tipo)
            run._r.append(fld)
        elif texto.strip() == "PAGE":
            instr = OxmlElement("w:instrText")
            instr.set(qn("xml:space"), "preserve")
            instr.text = texto
            run._r.append(instr)
        else:
            t = OxmlElement("w:t")
            t.text = texto
            run._r.append(t)
    run.font.name = FUENTE
    run.font.size = Pt(9)


def plantilla(destino: Path) -> None:
    with tempfile.TemporaryDirectory() as tmp:
        base = Path(tmp) / "reference.docx"
        referencia_pandoc(base)
        doc = Document(str(base))

    idioma_por_defecto(doc)

    normal = doc.styles["Normal"]
    fuente(normal, tam=11)
    for nombre in ("Body Text", "First Paragraph"):
        e = doc.styles[nombre]
        fuente(e, tam=11)
        parrafo(e, antes=0, despues=6, interlineado=1.5, alineacion=WD_ALIGN_PARAGRAPH.LEFT)

    compacto = doc.styles["Compact"]  # texto de las celdas de tabla
    fuente(compacto, tam=9)
    parrafo(compacto, antes=1, despues=1, interlineado=1.0)

    for nivel, (tam, antes) in enumerate(((14, 18), (12, 14), (11, 12), (11, 10)), start=1):
        h = doc.styles[f"Heading {nivel}"]
        fuente(h, tam=tam, negrita=True, cursiva=(nivel == 4), color=NEGRO)
        parrafo(h, antes=antes, despues=6, interlineado=1.15, con_siguiente=True)
        try:
            fuente(doc.styles[f"Heading {nivel} Char"], tam=tam, negrita=True, color=NEGRO)
        except KeyError:
            pass

    toc = doc.styles["TOC Heading"]
    fuente(toc, tam=14, negrita=True, color=NEGRO)
    parrafo(toc, antes=0, despues=12)

    for nombre in ("Caption", "Table Caption", "Image Caption"):
        e = buscar(doc, nombre)
        fuente(e, tam=10, cursiva=True, color=NEGRO)
        parrafo(e, antes=6, despues=6, interlineado=1.0, con_siguiente=True)

    biblio = doc.styles["Bibliography"]
    fuente(biblio, tam=11)
    parrafo(biblio, antes=0, despues=6, interlineado=1.15)
    biblio.paragraph_format.left_indent = Cm(1.27)
    biblio.paragraph_format.first_line_indent = Cm(-1.27)

    bloque = doc.styles["Block Text"]
    fuente(bloque, tam=11, cursiva=False)
    parrafo(bloque, antes=6, despues=6, interlineado=1.15)
    bloque.paragraph_format.left_indent = Cm(1)
    bloque.paragraph_format.right_indent = Cm(1)

    fuente(doc.styles["Footnote Text"], tam=9)

    codigo = estilo(doc, "Source Code")
    fuente(codigo, nombre=MONO, tam=9)
    parrafo(codigo, antes=0, despues=0, interlineado=1.0)
    try:
        fuente(doc.styles["Verbatim Char"], nombre=MONO, tam=9.5)
    except KeyError:
        pass

    try:
        doc.styles["Hyperlink"].font.color.rgb = RGBColor(0x05, 0x63, 0xC1)
    except KeyError:
        pass

    # Carátula: se usan con ::: {custom-style="Caratula ..."} en informe.qmd.
    centro = WD_ALIGN_PARAGRAPH.CENTER
    caratula = {
        "Caratula": dict(tam=12, antes=0, despues=2),
        "Caratula Logo": dict(tam=12, antes=0, despues=18),
        "Caratula Institucion": dict(tam=15, negrita=True, antes=0, despues=4),
        "Caratula Curso": dict(tam=13, negrita=True, antes=18, despues=2),
        "Caratula Entrega": dict(tam=12, antes=6, despues=2),
        "Caratula Titulo": dict(tam=20, negrita=True, antes=36, despues=12),
        "Caratula Subtitulo": dict(tam=13, cursiva=True, antes=0, despues=30),
        "Caratula Etiqueta": dict(tam=11, negrita=True, antes=14, despues=2),
        "Caratula Pie": dict(tam=11, antes=36, despues=0),
    }
    for nombre, c in caratula.items():
        e = estilo(doc, nombre)
        fuente(e, tam=c["tam"], negrita=c.get("negrita", False), cursiva=c.get("cursiva", False),
               color=NEGRO)
        parrafo(e, antes=c["antes"], despues=c["despues"], interlineado=1.15, alineacion=centro)

    filetes_de_cabecera(doc)

    seccion = doc.sections[0]
    seccion.start_type = WD_SECTION.NEW_PAGE
    seccion.page_width, seccion.page_height = Cm(21), Cm(29.7)
    for lado in ("left_margin", "right_margin", "top_margin", "bottom_margin"):
        setattr(seccion, lado, Cm(2.5))
    seccion.header_distance = seccion.footer_distance = Cm(1.25)
    # Sin número en la carátula: la primera página tiene su propio pie, vacío.
    seccion.different_first_page_header_footer = True
    numero_de_pagina(seccion.footer.paragraphs[0])

    # El cuerpo de la referencia (ejemplos de estilos) no pasa al documento: Pandoc solo toma
    # estilos, numeración, encabezados, pies y propiedades de sección.
    doc.core_properties.title = "Informe PC3"
    doc.core_properties.author = "aire-distribuido"
    doc.save(str(destino))


def marcador(destino: Path) -> None:
    from PIL import Image, ImageDraw, ImageFont

    ancho, alto = 1600, 640
    img = Image.new("RGB", (ancho, alto), (245, 245, 245))
    d = ImageDraw.Draw(img)
    for x in range(0, ancho, 28):  # borde discontinuo
        d.line([(x, 6), (x + 14, 6)], fill=(170, 170, 170), width=6)
        d.line([(x, alto - 7), (x + 14, alto - 7)], fill=(170, 170, 170), width=6)
    for y in range(0, alto, 28):
        d.line([(6, y), (6, y + 14)], fill=(170, 170, 170), width=6)
        d.line([(ancho - 7, y), (ancho - 7, y + 14)], fill=(170, 170, 170), width=6)

    def letra(tam, negrita=False):
        for nombre in (("DejaVuSans-Bold.ttf" if negrita else "DejaVuSans.ttf"),
                       ("Arial Bold.ttf" if negrita else "Arial.ttf")):
            try:
                return ImageFont.truetype(nombre, tam)
            except OSError:
                continue
        return ImageFont.load_default(size=tam)

    lineas = [
        ("FIGURA PENDIENTE", letra(84, True), (150, 40, 20)),
        ("El archivo de esta figura todavía no existe.", letra(40), (60, 60, 60)),
        ("Ver el aviso PENDIENTE junto a la figura: dice de dónde sale.", letra(40), (60, 60, 60)),
    ]
    y = 170
    for texto, f, color in lineas:
        caja = d.textbbox((0, 0), texto, font=f)
        d.text(((ancho - (caja[2] - caja[0])) / 2, y), texto, font=f, fill=color)
        y += (caja[3] - caja[1]) + 50
    img.save(destino, optimize=True)


if __name__ == "__main__":
    plantilla(AQUI / "plantilla.docx")
    marcador(AQUI / "figura-pendiente.png")
    print("escritos recursos/plantilla.docx y recursos/figura-pendiente.png")
