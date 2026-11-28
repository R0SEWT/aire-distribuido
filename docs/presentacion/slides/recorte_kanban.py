"""Recorta la captura del Kanban de la PC4 y marca la tarjeta #76 para el deck de la PC4.

En la PC4 el docente no ubicó en el tablero la tarea «Diagrama de entidades de MongoDB y Redis»: estaba
en la fila 12 de la captura del informe, sin marcar. Este recorte muestra las filas 9 a 15 de la misma
captura (docs/informe/capturas/pc4/kanban.png, sin editar nada más) con un recuadro en esa fila.

    python3 recorte_kanban.py      # deja img/kanban-pc4-76.png
"""

from pathlib import Path

from PIL import Image, ImageDraw

AQUI = Path(__file__).resolve().parent
ORIGEN = AQUI / "../../informe/capturas/pc4/kanban.png"
DESTINO = AQUI / "img/kanban-pc4-76.png"

# Coordenadas en la captura original (1500 × 1000): cada fila mide 37 px y la 12 va de 649 a 686.
CAJA = (60, 538, 1110, 760)        # filas 9 a 15; columnas Title, Assignees y Status
FILA_76 = (64, 649, 1106, 686)
NARANJA = (235, 104, 52)           # $alto en custom.scss

im = Image.open(ORIGEN).convert("RGB")
d = ImageDraw.Draw(im)
d.rounded_rectangle(FILA_76, radius=6, outline=NARANJA, width=4)
im.crop(CAJA).save(DESTINO, optimize=True)
print(f"recorte en {DESTINO.relative_to(AQUI)}")
