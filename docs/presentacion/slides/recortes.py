"""Recortes de las capturas de terminal para la lámina de evidencias.

    uv run --with pillow python recortes.py      # desde docs/presentacion/slides/

Las capturas del informe (docs/informe/capturas/) son la pantalla entera: en una lámina, a ese
tamaño, no se lee nada. Este script recorta la parte que importa de cada una, sin reescalar, y la
deja en img/: el comando de go test con sus paquetes en ok, las corridas del CI en verde, y el
final de la construcción de la imagen con la salida del cargador dentro del contenedor. Las
coordenadas son de las capturas actuales (1920×1200); si se vuelven a tomar, hay que revisarlas.
"""

from __future__ import annotations

from pathlib import Path

from PIL import Image, ImageDraw, ImageFont

AQUI = Path(__file__).resolve().parent
CAPTURAS = AQUI.parents[1] / "informe" / "capturas"
SALIDA = AQUI / "img"
FONDO = (18, 18, 18)  # el negro de la terminal, medido en las capturas


def recortar(nombre: str, caja: tuple[int, int, int, int]) -> Image.Image:
    return Image.open(CAPTURAS / nombre).convert("RGB").crop(caja)


def con_margen(img: Image.Image, m: int = 14) -> Image.Image:
    lienzo = Image.new("RGB", (img.width + 2 * m, img.height + 2 * m), FONDO)
    lienzo.paste(img, (m, m))
    return lienzo


def fuente(tam: int) -> ImageFont.ImageFont:
    for nombre in ("DejaVuSansMono.ttf", "LiberationMono-Regular.ttf"):
        try:
            return ImageFont.truetype(nombre, tam)
        except OSError:
            continue
    return ImageFont.load_default()


def main() -> None:
    # go test -race: el comando y los siete paquetes
    con_margen(recortar("go-test-race.png", (0, 330, 745, 508))).save(SALIDA / "captura-test.png", optimize=True)

    # CI: la cabecera y las diez corridas (estado, título, workflow, rama y evento)
    con_margen(recortar("ci.png", (0, 368, 1355, 592))).save(SALIDA / "captura-ci.png", optimize=True)

    # Docker: la línea de la construcción terminada y la salida del cargador en el contenedor,
    # con los 31 pasos de la construcción resumidos en medio
    arriba = recortar("docker.png", (0, 368, 905, 390))
    abajo = recortar("docker.png", (0, 1030, 905, 1190))
    hueco = 34
    img = Image.new("RGB", (905, arriba.height + hueco + abajo.height), FONDO)
    img.paste(arriba, (0, 0))
    ImageDraw.Draw(img).text((8, arriba.height + 6), "...  (los pasos de la construcción)", fill=(120, 120, 120),
                             font=fuente(17))
    img.paste(abajo, (0, arriba.height + hueco))
    con_margen(img).save(SALIDA / "captura-docker.png", optimize=True)

    print("img/captura-test.png, img/captura-ci.png e img/captura-docker.png")


if __name__ == "__main__":
    main()
