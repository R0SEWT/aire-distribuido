#!/usr/bin/env python3
"""Llena la plantilla de participación del docente con los datos de participacion.local.yml.

    uv run --with python-docx --with pyyaml llenar-participacion.py [salida.docx]

La plantilla (recursos/plantilla-participacion.docx, o $PLANTILLA_PARTICIPACION) se copia tal cual:
logo, estilos y tablas; solo se escribe dentro de las celdas. La tabla de participación tiene cuatro
filas por integrante (una por responsabilidad); el número y el nombre van fusionados verticalmente.
"""
import os
import sys
from pathlib import Path

import yaml
from docx import Document

AQUI = Path(__file__).resolve().parent
plantilla = Path(os.environ.get("PLANTILLA_PARTICIPACION", AQUI / "recursos" / "plantilla-participacion.docx"))
datos_ruta = AQUI / "participacion.local.yml"
salida = Path(sys.argv[1]) if len(sys.argv) > 1 else AQUI / "salida" / "1ACC0065-202620_Participación_PC3.docx"
if not plantilla.exists():
    sys.exit(f"falta la plantilla {plantilla} (copiarla del Aula Virtual: 1ACC0065-202620_Participación.docx)")
if not datos_ruta.exists():
    sys.exit(f"falta {datos_ruta} (copiar participacion.ejemplo.yml y llenarlo)")
d = yaml.safe_load(datos_ruta.read_text(encoding="utf-8"))


def escribir(celda, texto, negrita=None, centrar=False):
    """Escribe en la celda conservando el formato del primer run de la plantilla."""
    p = celda.paragraphs[0]
    runs = p.runs
    if runs:
        runs[0].text = texto
        for r in runs[1:]:
            r.text = ""
    else:
        r = p.add_run(texto)
        if negrita is not None:
            r.bold = negrita
    if centrar:
        from docx.enum.text import WD_ALIGN_PARAGRAPH
        p.alignment = WD_ALIGN_PARAGRAPH.CENTER
    for extra in celda.paragraphs[1:]:
        for r in extra.runs:
            r.text = ""


doc = Document(str(plantilla))
t_datos, t_comentario, t_part, _ = doc.tables
escribir(t_datos.rows[0].cells[1], d["entrega"])
escribir(t_datos.rows[1].cells[1], d["grupo"])
escribir(t_datos.rows[2].cells[1], d["solucion"])
escribir(t_datos.rows[3].cells[1], d["lider"])
escribir(t_comentario.rows[1].cells[0], d["comentario"])
COL = {"cumplio": 3, "parcial": 4, "no_cumplio": 5}
for i, integ in enumerate(d["integrantes"][:3]):
    base = 2 + 4 * i
    filas = t_part.rows[base:base + 4]
    escribir(filas[0].cells[0], str(i + 1))
    escribir(filas[0].cells[1], integ["nombre"])
    resp = list(integ.get("responsabilidades") or [])[:4]
    for j, fila in enumerate(filas):
        escribir(fila.cells[2], resp[j] if j < len(resp) else "")
        for c in (3, 4, 5):
            escribir(fila.cells[c], "")
    col = COL.get(integ.get("cumplimiento", "cumplio"), 3)
    escribir(filas[0].cells[col], "X", centrar=True)
salida.parent.mkdir(parents=True, exist_ok=True)
doc.save(str(salida))
print(f"escrito {salida}")
