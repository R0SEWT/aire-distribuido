"""Monta el video de la PC3 a partir de guion.md: diapositivas, narración y ffmpeg.

Lo llama armar.sh (no se corre a mano):

    uv run --with pillow python docs/video/montar.py

Lee docs/video/guion.md, y por cada ``## Escena`` genera en docs/video/build/:
  - la diapositiva (PNG 1920×1080) con Pillow,
  - la narración (edge-tts, o la voz propia de docs/video/voz/escena-NN.* si existe),
  - el clip de la escena (H.264 + AAC, 30 fps),
y concatena todo en docs/video/build/video-pc3.mp4. Si una grabación de terminal
(docs/video/casts/*.cast) es más corta que su narración, se reproduce más lenta (hasta 0,3×) y
después se congela el último fotograma.

Modo cursed (CURSED=1, `armar.sh --cursed`): lee guion-cursed.md y escribe en build/cursed/
video-pc3-cursed.mp4. Mismas diapositivas y grabaciones, pero con voz tipo Loquendo, deep-fry,
punch-ins, sacudidas, rótulos con borde negro, ojos rojos y efectos de sonido sintetizados con
ffmpeg (ver la cabecera de guion-cursed.md). La versión seria no cambia: el modo solo agrega.

Variables de entorno: FFMPEG, FFPROBE, AGG, VOZ (voz de edge-tts), RITMO (su velocidad, p. ej.
"+10%"), TONO (su tono, solo cursed), EQUIPO (ruta al equipo.yml), CURSED.
"""

from __future__ import annotations

import hashlib
import io
import json
import math
import os
import random
import re
import subprocess
import sys
from dataclasses import dataclass, field
from pathlib import Path

from PIL import Image, ImageDraw, ImageEnhance, ImageFilter, ImageFont

RAIZ = Path(__file__).resolve().parents[2]
VIDEO = RAIZ / "docs" / "video"
CURSED = os.environ.get("CURSED") == "1"
BUILD = VIDEO / "build" / "cursed" if CURSED else VIDEO / "build"
CASTS = VIDEO / "casts"
VOZ_PROPIA = VIDEO / "voz"
GUION = VIDEO / ("guion-cursed.md" if CURSED else "guion.md")
SALIDA = BUILD / ("video-pc3-cursed.mp4" if CURSED else "video-pc3.mp4")

FFMPEG = os.environ.get("FFMPEG", "ffmpeg")
FFPROBE = os.environ.get("FFPROBE", "ffprobe")
AGG = os.environ.get("AGG", "agg")
VOZ = os.environ.get("VOZ", "es-ES-AlvaroNeural" if CURSED else "es-PE-AlexNeural")
RITMO = os.environ.get("RITMO", "+30%" if CURSED else "+15%")  # velocidad de edge-tts; "+0%" es la natural
TONO = os.environ.get("TONO", "-10Hz")  # tono de edge-tts; solo lo usa el modo cursed
EQUIPO = os.environ.get("EQUIPO", str(RAIZ / "docs" / "informe" / "equipo.local.yml"))

ANCHO, ALTO = 1920, 1080
FPS = 30
SILENCIO_INICIO = 0.6  # s antes de que empiece la voz
SILENCIO_FIN = 0.9  # s después de que termina
FUNDIDO = 0.35  # s de fundido a negro entre escenas

NAVY = (31, 58, 95)
CREMA = (247, 245, 240)
CREMA_OSCURA = (232, 228, 220)
GRIS = (90, 90, 90)
BLANCO = (255, 255, 255)
TERMINAL_FONDO = (13, 17, 23)
ACENTO = (214, 93, 51)

TITULO_TRABAJO = "Regímenes de calidad del aire con K-Means paralelo en Go"
CASO = "Caso 10 · Monitoreo de Calidad del Aire y Sensores Ambientales"
CURSO = "1ACC0065 Programación Concurrente y Distribuida · UPC · 2026-20"
ENTREGA = "Trabajo Final · Entregable 1 (PC3)"
REPO = "github.com/R0SEWT/aire-distribuido"


def fuente(negrita: bool = False, mono: bool = False, tamano: int = 32) -> ImageFont.FreeTypeFont:
    nombre = "DejaVuSansMono" if mono else "DejaVuSans"
    if negrita:
        nombre += "-Bold"
    for d in ("/usr/share/fonts/truetype/dejavu", str(Path.home() / "miniconda3/fonts")):
        p = Path(d) / f"{nombre}.ttf"
        if p.exists():
            return ImageFont.truetype(str(p), tamano)
    return ImageFont.load_default(size=tamano)


def ruta_fuente_impact() -> str:
    """Impact si está instalada; si no, la negrita más parecida que haya."""
    candidatas = [
        "/usr/share/fonts/truetype/msttcorefonts/Impact.ttf",
        "/usr/share/fonts/truetype/dejavu/DejaVuSansCondensed-Bold.ttf",
        "/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf",
        "/usr/share/fonts/truetype/liberation/LiberationSans-Bold.ttf",
    ]
    for c in candidatas:
        if Path(c).exists():
            return c
    sys.exit("no hay una fuente negrita para los rótulos")


# ----------------------------------------------------------------------------- guion


@dataclass
class Escena:
    numero: int
    titulo: str
    pantalla: str
    narracion: str
    pie: str = ""
    extras: dict = field(default_factory=dict)
    efectos: str = ""
    textos: list = field(default_factory=list)  # [(texto, a, b)]


def leer_guion(ruta: Path) -> list[Escena]:
    texto = ruta.read_text(encoding="utf-8")
    partes = re.split(r"^## Escena (\d+) · (.+)$", texto, flags=re.M)
    escenas = []
    for i in range(1, len(partes), 3):
        numero, titulo, cuerpo = int(partes[i]), partes[i + 1].strip(), partes[i + 2]
        m = re.search(r"^- Pantalla: `([^`]+)`", cuerpo, flags=re.M)
        if not m:
            sys.exit(f"escena {numero}: falta la línea `- Pantalla:`")
        pantalla = m.group(1).strip()
        n = re.search(r"```narración\n(.*?)```", cuerpo, flags=re.S)
        if not n:
            sys.exit(f"escena {numero}: falta el bloque ```narración")
        narracion = " ".join(l.strip() for l in n.group(1).strip().splitlines())
        esc = Escena(numero, titulo, pantalla, narracion)
        e = re.search(r"^- Efectos: `([^`]*)`", cuerpo, flags=re.M)
        esc.efectos = e.group(1).strip() if e else ""
        for t in re.finditer(r"^- Texto: `(.+?) @ ([\d.]+)-([\d.]+)`", cuerpo, flags=re.M):
            esc.textos.append((t.group(1).strip(), float(t.group(2)), float(t.group(3))))
        escenas.append(esc)
    if not escenas:
        sys.exit("el guion no tiene escenas")
    return escenas


def leer_equipo(ruta: str) -> list[str]:
    p = Path(ruta)
    if not p.exists():
        return []
    nombres = []
    for linea in p.read_text(encoding="utf-8").splitlines():
        m = re.match(r"\s*-\s*nombre:\s*(.+?)\s*$", linea)
        if m:
            nombres.append(m.group(1).strip().strip('"'))
    return nombres


# ----------------------------------------------------------------------------- dibujo


def envolver(draw: ImageDraw.ImageDraw, texto: str, f: ImageFont.FreeTypeFont, ancho: int) -> list[str]:
    lineas, actual = [], ""
    for palabra in texto.split():
        prueba = (actual + " " + palabra).strip()
        if draw.textlength(prueba, font=f) <= ancho:
            actual = prueba
        else:
            if actual:
                lineas.append(actual)
            actual = palabra
    if actual:
        lineas.append(actual)
    return lineas


def marco(titulo: str, numero: int, total: int, fondo=CREMA) -> tuple[Image.Image, ImageDraw.ImageDraw]:
    im = Image.new("RGB", (ANCHO, ALTO), fondo)
    d = ImageDraw.Draw(im)
    d.rectangle([0, 0, ANCHO, 96], fill=NAVY)
    d.rectangle([0, 96, ANCHO, 102], fill=ACENTO)
    d.text((64, 24), titulo, font=fuente(True, tamano=40), fill=BLANCO)
    etiqueta = "Caso 10 · PC3"
    f = fuente(tamano=26)
    d.text((ANCHO - 64 - d.textlength(etiqueta, font=f), 34), etiqueta, font=f, fill=(200, 210, 225))
    d.rectangle([0, ALTO - 44, ANCHO, ALTO], fill=CREMA_OSCURA if fondo == CREMA else (22, 27, 34))
    f = fuente(tamano=22)
    color = GRIS if fondo == CREMA else (150, 160, 170)
    d.text((64, ALTO - 34), f"aire-distribuido · {REPO}", font=f, fill=color)
    s = f"{numero} / {total}"
    d.text((ANCHO - 64 - d.textlength(s, font=f), ALTO - 34), s, font=f, fill=color)
    return im, d


def pegar_ajustada(im: Image.Image, fig: Image.Image, caja: tuple[int, int, int, int]) -> None:
    x0, y0, x1, y1 = caja
    escala = min((x1 - x0) / fig.width, (y1 - y0) / fig.height, 1.25)
    nuevo = (max(1, round(fig.width * escala)), max(1, round(fig.height * escala)))
    fig = fig.convert("RGB").resize(nuevo, Image.LANCZOS)
    im.paste(fig, (x0 + ((x1 - x0) - nuevo[0]) // 2, y0 + ((y1 - y0) - nuevo[1]) // 2))


def pie_de_figura(d: ImageDraw.ImageDraw, pie: str, y: int) -> None:
    f = fuente(tamano=24)
    for i, linea in enumerate(envolver(d, pie, f, ANCHO - 2 * 80)[:2]):
        d.text(((ANCHO - d.textlength(linea, font=f)) / 2, y + i * 30), linea, font=f, fill=GRIS)


def diapositiva_caratula(equipo: list[str]) -> Image.Image:
    im = Image.new("RGB", (ANCHO, ALTO), NAVY)
    d = ImageDraw.Draw(im)
    d.rectangle([0, 0, 14, ALTO], fill=ACENTO)
    y = 120
    f = fuente(tamano=30)
    d.text((110, y), CURSO, font=f, fill=(200, 210, 225))
    y += 48
    d.text((110, y), ENTREGA, font=f, fill=(200, 210, 225))
    y += 110
    ft = fuente(True, tamano=66)
    for linea in envolver(d, TITULO_TRABAJO, ft, ANCHO - 220):
        d.text((110, y), linea, font=ft, fill=BLANCO)
        y += 84
    y += 20
    fs = fuente(tamano=36)
    for linea in envolver(d, CASO, fs, ANCHO - 220):
        d.text((110, y), linea, font=fs, fill=(230, 235, 245))
        y += 48
    y += 70
    d.text((110, y), "Integrantes" if equipo else "Equipo", font=fuente(tamano=26), fill=(180, 195, 215))
    y += 44
    fn = fuente(True, tamano=34)
    for nombre in equipo or ["Equipo del caso 10"]:
        d.text((110, y), nombre, font=fn, fill=BLANCO)
        y += 46
    f = fuente(tamano=28)
    d.text((110, ALTO - 100), REPO, font=f, fill=(200, 210, 225))
    return im


def diapositiva_figura(esc: Escena, total: int, rutas: list[str], pie: str, crop: tuple | None) -> Image.Image:
    im, d = marco(esc.titulo, esc.numero, total)
    margen_y0, margen_y1 = 124, ALTO - 44 - (74 if pie else 16)
    figs = []
    for r in rutas:
        fig = Image.open(RAIZ / r)
        if crop and len(rutas) == 1:
            fig = fig.crop((crop[0], crop[1], crop[0] + crop[2], crop[1] + crop[3]))
        figs.append(fig)
    if len(figs) == 1:
        pegar_ajustada(im, figs[0], (64, margen_y0, ANCHO - 64, margen_y1))
    else:
        # Lado a lado, repartiendo el ancho según la proporción de cada figura.
        altura = margen_y1 - margen_y0
        anchos = [f.width * altura / f.height for f in figs]
        disponible = ANCHO - 64 * 2 - 40 * (len(figs) - 1)
        escala = min(1.0, disponible / sum(anchos))
        x = 64 + (disponible - sum(anchos) * escala) / 2
        for f, a in zip(figs, anchos):
            pegar_ajustada(im, f, (round(x), margen_y0, round(x + a * escala), margen_y1))
            x += a * escala + 40
    if pie:
        pie_de_figura(d, pie, margen_y1 + 14)
    return im


def diapositiva_texto(esc: Escena, total: int, titulo: str, lineas: list[str]) -> Image.Image:
    im, d = marco(esc.titulo, esc.numero, total)
    y = 150
    ft = fuente(True, tamano=46)
    d.text((110, y), titulo, font=ft, fill=NAVY)
    y += 90
    f = fuente(tamano=34)
    for linea in lineas:
        d.ellipse([110, y + 14, 126, y + 30], fill=ACENTO)
        for j, l in enumerate(envolver(d, linea, f, ANCHO - 110 - 150 - 64)):
            d.text((150, y), l, font=f, fill=(40, 40, 40))
            y += 46
        y += 22
    return im


def fondo_terminal(esc: Escena, total: int, pie: str) -> Image.Image:
    im, d = marco(esc.titulo, esc.numero, total, fondo=TERMINAL_FONDO)
    if pie:
        f = fuente(tamano=24)
        for i, linea in enumerate(envolver(d, pie, f, ANCHO - 160)[:1]):
            d.text(((ANCHO - d.textlength(linea, font=f)) / 2, ALTO - 44 - 34), linea, font=f, fill=(170, 180, 190))
    return im


# ----------------------------------------------------------------------------- herramientas


def correr(cmd: list[str], silencio: bool = True) -> None:
    r = subprocess.run(cmd, capture_output=silencio, text=True)
    if r.returncode != 0:
        sys.exit(f"falló: {' '.join(cmd)}\n{r.stderr if silencio else ''}")


def duracion(ruta: Path) -> float:
    r = subprocess.run(
        [FFPROBE, "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", str(ruta)],
        capture_output=True, text=True, check=True,
    )
    for linea in r.stdout.splitlines():
        try:
            return float(linea.strip())
        except ValueError:
            continue
    sys.exit(f"ffprobe no dio la duración de {ruta}: {r.stdout!r}")


def clip_valido(ruta: Path) -> bool:
    r = subprocess.run([FFPROBE, "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", str(ruta)],
                       capture_output=True, text=True)
    return r.returncode == 0 and any(l.strip() for l in r.stdout.splitlines())


def huella(*partes: str) -> str:
    return hashlib.sha256("\n".join(partes).encode("utf-8")).hexdigest()[:16]


def huella_insumos(rutas: list[str]) -> str:
    """Hash del contenido de los archivos fuente de una escena (figuras, grabaciones, videos), para
    que la clave de la caché cambie cuando cambia el insumo y no solo su ruta."""
    h = hashlib.sha256()
    for r in rutas:
        for candidato in (RAIZ / r, CASTS / f"{r}.cast"):
            if candidato.is_file():
                h.update(candidato.name.encode("utf-8"))
                h.update(candidato.read_bytes())
                break
    return h.hexdigest()[:16]


def narracion(esc: Escena) -> Path:
    """Devuelve la pista de voz de la escena: la propia si existe, si no edge-tts (cacheada)."""
    for ext in ("wav", "mp3", "m4a", "ogg", "flac"):
        propia = VOZ_PROPIA / f"escena-{esc.numero:02d}.{ext}"
        if propia.exists():
            return propia
    texto = esc.narracion
    extra = []
    if CURSED:
        texto = re.sub(r"\s*\[[^\]]*\]", "", texto)  # las acotaciones [así] no se leen
        extra = [TONO]
    salida = BUILD / f"voz-{esc.numero:02d}-{huella(VOZ, RITMO, *extra, texto)}.mp3"
    if not salida.exists():
        for vieja in BUILD.glob(f"voz-{esc.numero:02d}-*.mp3"):
            vieja.unlink()
        cmd = ["uvx", "--from", "edge-tts", "edge-tts", "--voice", VOZ, f"--rate={RITMO}"]
        if CURSED:
            cmd.append(f"--pitch={TONO}")
        correr(cmd + ["--text", texto, "--write-media", str(salida)])
    return salida


def duracion_cast(cast: Path) -> float:
    ultimo = 0.0
    with cast.open(encoding="utf-8") as f:
        next(f)  # cabecera
        for linea in f:
            if linea.strip():
                ultimo = json.loads(linea)[0]
    return ultimo


def render_terminal(nombre: str, objetivo: float, velocidad: float | None = None) -> tuple[Path, float]:
    """Renderiza el .cast a GIF con agg, a una velocidad que lo acerque a `objetivo` segundos."""
    cast = CASTS / f"{nombre}.cast"
    if not cast.exists():
        sys.exit(f"falta la grabación {cast}: correr docs/video/grabar.sh {nombre}")
    real = duracion_cast(cast)
    if velocidad is None:
        velocidad = min(2.0, max(0.3, real / max(objetivo, 1.0)))
    gif = BUILD / f"term-{nombre}-{huella(str(real), f'{velocidad:.3f}', huella_insumos([nombre]))}.gif"
    if not gif.exists():
        correr([
            AGG, "--cols", "120", "--rows", "30", "--font-family", "DejaVu Sans Mono",
            "--font-size", "22", "--line-height", "1.35", "--theme", "github-dark",
            "--speed", f"{velocidad:.3f}", "--fps-cap", "30", "--idle-time-limit", "2",
            "--no-loop", str(cast), str(gif),
        ])
    return gif, velocidad


# ----------------------------------------------------------------------------- escenas


def interpretar_pantalla(esc: Escena) -> tuple[str, list[str], str, dict]:
    tipo, _, resto = esc.pantalla.partition(" ")
    campos = [c.strip() for c in resto.split("|")]
    extras = {}
    for c in campos[2:]:
        k, _, v = c.partition("=")
        extras[k.strip()] = v.strip()
    rutas = [r.strip() for r in campos[0].split("+")] if campos and campos[0] else []
    pie = campos[1] if len(campos) > 1 else ""
    return tipo, rutas, pie, extras


def codec_salida(salida: Path) -> list[str]:
    # El modo cursed comprime más (el deep-fry pesa) y codifica más rápido.
    calidad = ["-preset", "faster", "-crf", "25"] if CURSED else ["-preset", "medium", "-crf", "20"]
    return [
        "-c:v", "libx264", *calidad, "-r", str(FPS),
        "-c:a", "aac", "-b:a", "128k", "-ar", "48000", str(salida),
    ]


def clip_estatico(png: Path, voz: Path, salida: Path) -> float:
    total = SILENCIO_INICIO + duracion(voz) + SILENCIO_FIN
    correr([
        FFMPEG, "-y", "-loglevel", "error", "-loop", "1", "-framerate", str(FPS), "-i", str(png),
        "-i", str(voz),
        "-filter_complex",
        f"[0:v]format=yuv420p,fade=t=in:st=0:d={FUNDIDO},fade=t=out:st={total - FUNDIDO:.3f}:d={FUNDIDO}[v];"
        f"[1:a]adelay={int(SILENCIO_INICIO * 1000)}|{int(SILENCIO_INICIO * 1000)},apad,"
        f"aresample=48000,aformat=channel_layouts=stereo[a]",
        "-map", "[v]", "-map", "[a]", "-t", f"{total:.3f}",
    ] + codec_salida(salida))
    return total


def caja_terminal(gif: Path) -> tuple[int, int, int, int]:
    """Dónde cabe la terminal en la diapositiva: (w, h, x, y)."""
    g = Image.open(gif)
    caja_w, caja_h = ANCHO - 2 * 40, ALTO - 124 - 44 - 40
    escala = min(1.0, caja_w / g.width, caja_h / g.height)
    w, h = (round(g.width * escala) // 2) * 2, (round(g.height * escala) // 2) * 2
    return w, h, (ANCHO - w) // 2, 124 + (caja_h - h) // 2


def clip_terminal(fondo: Path, gif: Path, voz: Path, salida: Path) -> float:
    total = SILENCIO_INICIO + duracion(voz) + SILENCIO_FIN
    w, h, x, y = caja_terminal(gif)
    correr([
        FFMPEG, "-y", "-loglevel", "error", "-loop", "1", "-framerate", str(FPS), "-i", str(fondo),
        "-i", str(gif), "-i", str(voz),
        "-filter_complex",
        f"[1:v]scale={w}:{h}:flags=lanczos,tpad=stop_mode=clone:stop_duration=600[t];"
        f"[0:v][t]overlay={x}:{y}:shortest=1,format=yuv420p,"
        f"fade=t=in:st=0:d={FUNDIDO},fade=t=out:st={total - FUNDIDO:.3f}:d={FUNDIDO}[v];"
        f"[2:a]adelay={int(SILENCIO_INICIO * 1000)}|{int(SILENCIO_INICIO * 1000)},apad,"
        f"aresample=48000,aformat=channel_layouts=stereo[a]",
        "-map", "[v]", "-map", "[a]", "-t", f"{total:.3f}",
    ] + codec_salida(salida))
    return total


def clip_video(video: Path, voz: Path, salida: Path) -> float:
    """Una escena animada ya renderizada (Remotion) con la narración encima: si la voz dura más que
    el clip, se congela su último cuadro; si dura menos, el clip se corta al final. Nunca se acelera."""
    total = SILENCIO_INICIO + duracion(voz) + SILENCIO_FIN
    correr([
        FFMPEG, "-y", "-loglevel", "error", "-i", str(video), "-i", str(voz),
        "-filter_complex",
        f"[0:v]scale={ANCHO}:{ALTO}:flags=lanczos,fps={FPS},tpad=stop_mode=clone:stop_duration=600,"
        f"format=yuv420p,fade=t=in:st=0:d={FUNDIDO},fade=t=out:st={total - FUNDIDO:.3f}:d={FUNDIDO}[v];"
        f"[1:a]adelay={int(SILENCIO_INICIO * 1000)}|{int(SILENCIO_INICIO * 1000)},apad,"
        f"aresample=48000,aformat=channel_layouts=stereo[a]",
        "-map", "[v]", "-map", "[a]", "-t", f"{total:.3f}",
    ] + codec_salida(salida))
    return total


# ----------------------------------------------------------------------------- modo cursed


def freir(im: Image.Image) -> Image.Image:
    """Deep-fry: saturación, contraste, nitidez exagerada, JPEG a calidad ínfima, ruido y tinte."""
    im = im.convert("RGB")
    im = ImageEnhance.Color(im).enhance(2.6)
    im = ImageEnhance.Contrast(im).enhance(1.8)
    im = im.filter(ImageFilter.UnsharpMask(radius=3, percent=220, threshold=2))
    for calidad in (7, 5):
        buf = io.BytesIO()
        im.save(buf, "JPEG", quality=calidad)
        buf.seek(0)
        im = Image.open(buf).convert("RGB")
    ruido = Image.effect_noise(im.size, 28).convert("RGB")
    im = Image.blend(im, ruido, 0.12)
    tinte = Image.new("RGB", im.size, (255, 120, 0))
    return Image.blend(im, tinte, 0.06)


def ojos_png(puntos: list[tuple[int, int]], ruta: Path) -> Path:
    """Ojos rojos que brillan: halos rojos con pupila clara, sobre fondo transparente."""
    im = Image.new("RGBA", (ANCHO, ALTO), (0, 0, 0, 0))
    d = ImageDraw.Draw(im)
    for x, y in puntos:
        for r in range(80, 0, -2):
            a = int(255 * (1 - r / 80) ** 1.6)
            d.ellipse([x - r, y - r * 0.7, x + r, y + r * 0.7], fill=(255, 30, 10, a))
        d.ellipse([x - 12, y - 8, x + 12, y + 8], fill=(255, 235, 210, 255))
    im.save(ruta)
    return ruta


def _contorno(d: ImageDraw.ImageDraw, puntos, color, grosor: int) -> None:
    d.line(puntos, fill=(20, 20, 20, 255), width=grosor + 8, joint="curve")
    d.line(puntos, fill=color, width=grosor, joint="curve")


def gato_png(ruta: Path, color: tuple, derecha: bool) -> Path:
    """Un gato de caricatura con metralleta, dibujado con primitivas (nada con derechos). 640×520."""
    if ruta.exists():
        return ruta
    im = Image.new("RGBA", (640, 520), (0, 0, 0, 0))
    d = ImageDraw.Draw(im)
    o = (20, 20, 20, 255)
    oscuro = tuple(max(0, c - 70) for c in color[:3]) + (255,)
    _contorno(d, [(150, 380), (70, 310), (95, 215)], color, 26)  # cola
    d.ellipse([130, 250, 430, 470], fill=color, outline=o, width=6)  # cuerpo
    for x in (185, 265, 330, 395):  # patas
        d.ellipse([x - 32, 418, x + 32, 482], fill=color, outline=o, width=6)
    for x0, x1 in ((190, 240), (300, 350)):  # rayas del cuerpo
        d.arc([x0, 250, x1 + 40, 330], 200, 340, fill=oscuro, width=10)
    d.polygon([(320, 150), (338, 55), (402, 132)], fill=color, outline=o, width=6)  # orejas
    d.polygon([(500, 150), (482, 55), (418, 132)], fill=color, outline=o, width=6)
    d.polygon([(338, 135), (346, 85), (384, 128)], fill=(255, 150, 170, 255))
    d.polygon([(482, 135), (474, 85), (436, 128)], fill=(255, 150, 170, 255))
    d.ellipse([300, 110, 520, 320], fill=color, outline=o, width=6)  # cabeza
    for x0 in (360, 400, 440):  # rayas de la frente
        d.line([(x0, 118), (x0 + 10, 160)], fill=oscuro, width=8)
    for cx in (375, 455):  # ojos
        d.ellipse([cx - 27, 178, cx + 27, 232], fill=(255, 255, 255, 255), outline=o, width=4)
        d.ellipse([cx - 7, 184, cx + 7, 228], fill=(30, 30, 30, 255))
        d.ellipse([cx - 4, 190, cx + 2, 198], fill=(255, 255, 255, 255))
    d.polygon([(403, 246), (427, 246), (415, 260)], fill=(255, 130, 150, 255))  # nariz
    d.line([(415, 260), (415, 272)], fill=o, width=4)
    d.arc([393, 258, 417, 286], 0, 180, fill=o, width=4)
    d.arc([413, 258, 437, 286], 0, 180, fill=o, width=4)
    for dy in (-12, 0, 12):  # bigotes
        d.line([(340, 262 + dy), (250, 250 + dy * 2)], fill=o, width=4)
        d.line([(490, 262 + dy), (580, 250 + dy * 2)], fill=o, width=4)
    # metralleta: culata, cuerpo, cañón, cargador, empuñadura y mira
    gris, negro = (70, 70, 78, 255), (35, 35, 40, 255)
    d.rectangle([180, 334, 250, 366], fill=(110, 70, 40, 255), outline=o, width=4)
    d.rectangle([245, 326, 520, 374], fill=gris, outline=o, width=5)
    d.rectangle([515, 338, 640, 360], fill=negro, outline=o, width=4)
    d.rectangle([330, 372, 372, 436], fill=negro, outline=o, width=4)
    d.rectangle([425, 372, 455, 412], fill=negro, outline=o, width=4)
    d.rectangle([360, 306, 420, 330], fill=negro, outline=o, width=4)
    for x in (300, 470):  # patas delanteras sobre el arma
        d.ellipse([x - 30, 336, x + 30, 384], fill=color, outline=o, width=5)
    if not derecha:
        im = im.transpose(Image.FLIP_LEFT_RIGHT)
    im.save(ruta)
    return ruta


def fogonazo_png(ruta: Path, derecha: bool) -> Path:
    """Fogonazo y trazadoras que salen del cañón. 300×200; el cañón está en el borde izquierdo."""
    if ruta.exists():
        return ruta
    im = Image.new("RGBA", (300, 200), (0, 0, 0, 0))
    d = ImageDraw.Draw(im)
    cx, cy = 20, 100
    puntas = []
    for i in range(16):
        ang = i * math.pi * 2 / 16
        r = 70 if i % 2 == 0 else 30
        puntas.append((cx + r * 1.3 * math.cos(ang), cy + r * math.sin(ang)))
    d.polygon(puntas, fill=(255, 190, 40, 230))
    d.ellipse([cx - 28, cy - 22, cx + 28, cy + 22], fill=(255, 245, 160, 255))
    for k in range(3):  # trazadoras
        x0 = 95 + k * 70
        d.line([(x0, cy - 4 + k * 4), (x0 + 48, cy - 4 + k * 4)], fill=(255, 220, 90, 240), width=7)
        d.line([(x0 + 44, cy - 4 + k * 4), (x0 + 52, cy - 4 + k * 4)], fill=(255, 255, 255, 255), width=7)
    if not derecha:
        im = im.transpose(Image.FLIP_LEFT_RIGHT)
    im.save(ruta)
    return ruta


def explosion_pngs(carpeta: Path, n: int = 7) -> list[Path]:
    """Fotogramas de una explosión de caricatura (900×900), de chispa a humo."""
    rutas = []
    for i in range(n):
        ruta = carpeta / f"explosion-{i}.png"
        rutas.append(ruta)
        if ruta.exists():
            continue
        im = Image.new("RGBA", (900, 900), (0, 0, 0, 0))
        d = ImageDraw.Draw(im)
        f = (i + 1) / n
        r = int(60 + 380 * f)
        alfa = int(255 * (1 - f * 0.75))
        d.ellipse([450 - r, 450 - r, 450 + r, 450 + r], fill=(120, 120, 120, int(alfa * 0.6)))  # humo
        d.ellipse([450 - r * 0.85, 450 - r * 0.85, 450 + r * 0.85, 450 + r * 0.85], fill=(230, 60, 20, alfa))
        d.ellipse([450 - r * 0.6, 450 - r * 0.6, 450 + r * 0.6, 450 + r * 0.6], fill=(255, 150, 30, alfa))
        d.ellipse([450 - r * 0.3, 450 - r * 0.3, 450 + r * 0.3, 450 + r * 0.3], fill=(255, 245, 170, alfa))
        azar = random.Random(i)
        for _ in range(14):  # chispas
            ang = azar.uniform(0, 6.283)
            rr = r * azar.uniform(0.9, 1.35)
            x, y = 450 + rr * math.cos(ang), 450 + rr * math.sin(ang)
            d.ellipse([x - 9, y - 9, x + 9, y + 9], fill=(255, 210, 60, alfa))
        im.save(ruta)
    return rutas


def laser_png(puntos: list[tuple[int, int]], ruta: Path) -> Path:
    """Rayos láser rojos que salen de los ojos hacia abajo a la derecha."""
    im = Image.new("RGBA", (ANCHO, ALTO), (0, 0, 0, 0))
    d = ImageDraw.Draw(im)
    for x, y in puntos:
        fin = (x + 1400, y + 1000)
        d.line([(x, y), fin], fill=(255, 40, 20, 110), width=34)
        d.line([(x, y), fin], fill=(255, 90, 60, 200), width=16)
        d.line([(x, y), fin], fill=(255, 230, 220, 255), width=5)
    im.save(ruta)
    return ruta


def sfx(nombre: str) -> Path:
    """Efectos de sonido sintetizados con ffmpeg (nada con derechos): boom, airhorn, metralleta, bruh."""
    salida = BUILD / f"sfx-{nombre}.wav"
    if salida.exists():
        return salida
    recetas = {
        # vine boom: seno grave que baja de tono y decae
        "boom": ("aevalsrc='sin(2*PI*(50+35*exp(-t*9))*t)*exp(-t*2.6)*min(1,t*300)':d=1.5:s=48000",
                 "bass=g=4:f=70,volume=1.6,alimiter=limit=0.9"),
        # airhorn: dientes de sierra desafinados con glissando y vibrato
        "airhorn": ("aevalsrc='(sin(2*PI*(392+30*min(1,t*4))*t)+0.5*sin(2*PI*(784+60*min(1,t*4))*t)"
                    "+0.3*sin(2*PI*(1176+90*min(1,t*4))*t)+0.6*sin(2*PI*(466+36*min(1,t*4))*t))*0.3':d=1.0:s=48000",
                    "lowpass=f=2600,afade=t=in:d=0.03,afade=t=out:st=0.7:d=0.3,volume=0.9,alimiter=limit=0.9"),
        # metralleta: ráfagas de ruido blanco de 50 ms a 12 Hz
        "metralleta": ("aevalsrc='(random(0)*2-1)*lt(mod(t,0.0833),0.04)*exp(-mod(t,0.0833)*60)':d=1.1:s=48000",
                       "lowpass=f=4000,highpass=f=200,volume=0.9,alimiter=limit=0.9"),
        # «bruh»: tono grave distorsionado que cae
        "bruh": ("aevalsrc='(sin(2*PI*(110-40*t)*t)+0.4*sin(2*PI*(220-80*t)*t))*exp(-t*3)*min(1,t*200)':d=0.8:s=48000",
                 "volume=2.2,alimiter=limit=0.85,lowpass=f=1800"),
    }
    if nombre not in recetas:
        sys.exit(f"efecto de sonido desconocido: {nombre}")
    fuente_, filtro = recetas[nombre]
    correr([FFMPEG, "-y", "-loglevel", "error", "-f", "lavfi", "-i", fuente_, "-af", filtro,
            "-ac", "2", "-ar", "48000", str(salida)])
    return salida


@dataclass
class Efectos:
    normal: bool = False
    velocidad: float = 6.0
    sonidos: list = field(default_factory=list)  # [(nombre, t)]
    punches: list = field(default_factory=list)  # [(t, z)]
    sacudidas: list = field(default_factory=list)  # [(a, b)]
    ojos: list = field(default_factory=list)  # [([(x, y), ...], a, b)]
    gatos: list = field(default_factory=list)  # [(t, desde_la_izquierda)]
    lluvia: list = field(default_factory=list)  # [(a, b)]


def leer_efectos(texto: str) -> Efectos:
    ef = Efectos()
    for parte in [p.strip() for p in texto.split(";") if p.strip()]:
        if parte == "normal":
            ef.normal = True
        elif parte.startswith("velocidad="):
            ef.velocidad = float(parte.split("=", 1)[1])
        elif parte.startswith("sfx="):
            for s in parte[4:].split(","):
                nombre, _, t = s.strip().partition("@")
                ef.sonidos.append((nombre, float(t)))
        elif parte.startswith("punch@"):
            t, _, z = parte[6:].partition(":")
            ef.punches.append((float(t), float(z) if z else 2.0))
        elif parte.startswith("shake@"):
            a, _, b = parte[6:].partition("-")
            ef.sacudidas.append((float(a), float(b)))
        elif parte.startswith("gato@"):
            t, _, lado = parte[5:].partition(":")
            ef.gatos.append((float(t), lado != "der"))
        elif parte.startswith("lluvia_gatos@"):
            a, _, b = parte[13:].partition("-")
            ef.lluvia.append((float(a), float(b)))
        elif parte.startswith("ojos="):
            coords, _, rango = parte[5:].partition("@")
            v = [int(c) for c in coords.split(",")]
            a, _, b = rango.partition("-")
            ef.ojos.append((list(zip(v[0::2], v[1::2])), float(a), float(b)))
        else:
            sys.exit(f"efecto desconocido: «{parte}»")
    # Un boom o un airhorn traen un punch-in; una metralleta, una sacudida.
    for nombre, t in ef.sonidos:
        if nombre in ("boom", "airhorn") and not any(abs(t - p[0]) < 0.3 for p in ef.punches):
            ef.punches.append((t, 2.0 if nombre == "boom" else 2.3))
        if nombre == "metralleta":
            ef.sacudidas.append((t, t + 1.3))
            if not any(abs(t - g[0]) < 0.3 for g in ef.gatos):
                ef.gatos.append((t, len(ef.gatos) % 2 == 0))  # un gato por ráfaga, alternando el lado
    return ef


def expresiones_zoompan(ef: Efectos, semilla: int) -> str:
    """zoompan con punch-ins instantáneos (0,7 s) y sacudidas, todo en función de in_time."""
    azar = random.Random(semilla)
    z, x, y = ["1"], [], []
    for t, zoom in ef.punches:
        a, b = t, t + 0.7
        cx, cy = azar.uniform(0.2, 0.8), azar.uniform(0.2, 0.8)
        z.append(f"+{zoom - 1:.2f}*between(in_time,{a:.2f},{b:.2f})")
        x.append(f"{cx:.2f}*between(in_time,{a:.2f},{b:.2f})")
        y.append(f"{cy:.2f}*between(in_time,{a:.2f},{b:.2f})")
    for a, b in ef.sacudidas:
        f1, f2 = azar.uniform(50, 70), azar.uniform(45, 65)
        z.append(f"+0.14*between(in_time,{a:.2f},{b:.2f})")
        x.append(f"(0.5+0.5*sin({f1:.1f}*in_time))*between(in_time,{a:.2f},{b:.2f})")
        y.append(f"(0.5+0.5*cos({f2:.1f}*in_time))*between(in_time,{a:.2f},{b:.2f})")
    zx = "(iw-iw/zoom)*(" + ("+".join(x) or "0") + ")"
    zy = "(ih-ih/zoom)*(" + ("+".join(y) or "0") + ")"
    return f"zoompan=z='{''.join(z)}':x='{zx}':y='{zy}':d=1:s={ANCHO}x{ALTO}:fps={FPS}"


def rotulo_png(texto: str, ruta: Path, abajo: bool) -> Path:
    """Un rótulo tipo meme: mayúsculas blancas con borde negro grueso y sombra, centrado."""
    im = Image.new("RGBA", (ANCHO, ALTO), (0, 0, 0, 0))
    d = ImageDraw.Draw(im)
    tamano = max(64, min(150, int(1760 / (0.62 * max(len(texto), 1)))))
    f = ImageFont.truetype(ruta_fuente_impact(), tamano)
    lineas = envolver(d, texto, f, ANCHO - 120)
    alto_linea = int(tamano * 1.15)
    y = (ALTO * 0.62 if abajo else ALTO / 2) - alto_linea * len(lineas) / 2
    for linea in lineas:
        x = (ANCHO - d.textlength(linea, font=f)) / 2
        d.text((x + 7, y + 7), linea, font=f, fill=(0, 0, 0, 150), stroke_width=9, stroke_fill=(0, 0, 0, 150))
        d.text((x, y), linea, font=f, fill=(255, 255, 255, 255), stroke_width=9, stroke_fill=(0, 0, 0, 255))
        y += alto_linea
    im.save(ruta)
    return ruta


def clip_cursed(esc: Escena, png: Path, gif: Path | None, voz: Path, salida: Path, ef: Efectos) -> float:
    dur_voz = duracion(voz)
    total = SILENCIO_INICIO + dur_voz + SILENCIO_FIN
    for _, t in ef.sonidos:
        total = max(total, t + 1.9)
    for _, _, b in esc.textos:
        total = max(total, b + 0.4)
    for _, _, b in ef.ojos:
        total = max(total, b + 0.4)
    for t, _ in ef.gatos:
        total = max(total, t + 2.3)
    for _, b in ef.lluvia:
        total = max(total, b + 0.3)
    prefijo = f"escena-{esc.numero:02d}"

    entradas = [FFMPEG, "-y", "-loglevel", "error", "-loop", "1", "-framerate", str(FPS), "-i", str(png)]
    cadena = []
    if gif is not None:
        w, h, x, y = caja_terminal(gif)
        entradas += ["-i", str(gif)]
        cadena.append(f"[1:v]scale={w}:{h}:flags=lanczos,tpad=stop_mode=clone:stop_duration=600[t]")
        base = f"[0:v][t]overlay={x}:{y}:shortest=1"
        idx = 2
    else:
        base = "[0:v]null"
        idx = 1
    entradas += ["-i", str(voz)]
    i_voz = idx
    idx += 1
    i_ojos = None
    if ef.ojos:
        puntos = [p for pts, _, _ in ef.ojos for p in pts]
        entradas += ["-loop", "1", "-framerate", str(FPS), "-i", str(ojos_png(puntos, BUILD / f"{prefijo}-ojos.png"))]
        i_ojos = idx
        idx += 1
    i_rotulos = []
    for k, (texto, a, b) in enumerate(esc.textos):
        png_rotulo = rotulo_png(texto, BUILD / f"{prefijo}-rotulo-{k}.png", abajo=(k % 2 == 1))
        entradas += ["-loop", "1", "-framerate", str(FPS), "-i", str(png_rotulo)]
        i_rotulos.append((idx, a, b))
        idx += 1
    i_sfx = []
    for nombre, t in ef.sonidos:
        entradas += ["-i", str(sfx(nombre))]
        i_sfx.append((idx, t))
        idx += 1
    # gatos con metralleta: entran desde un lado, disparan con fogonazo y retroceso, y se van
    colores = [(150, 150, 160, 255), (235, 150, 60, 255), (60, 60, 65, 255), (240, 240, 235, 255)]
    i_gatos = []
    for k, (t, desde_izq) in enumerate(ef.gatos):
        c = (esc.numero + k) % len(colores)
        gato = gato_png(BUILD / f"gato-{c}-{'der' if desde_izq else 'izq'}.png", colores[c], desde_izq)
        flash = fogonazo_png(BUILD / f"fogonazo-{'der' if desde_izq else 'izq'}.png", desde_izq)
        entradas += ["-loop", "1", "-framerate", str(FPS), "-i", str(gato),
                     "-loop", "1", "-framerate", str(FPS), "-i", str(flash)]
        i_gatos.append((idx, idx + 1, t, desde_izq))
        idx += 2
    # explosiones de caricatura en cada boom
    i_expl = []
    if not ef.normal and any(n == "boom" for n, _ in ef.sonidos):
        for ruta in explosion_pngs(BUILD):
            entradas += ["-loop", "1", "-framerate", str(FPS), "-i", str(ruta)]
            i_expl.append(idx)
            idx += 1
    # rayos láser con los ojos rojos
    i_laser = None
    if ef.ojos:
        puntos = [p for pts, _, _ in ef.ojos for p in pts]
        entradas += ["-loop", "1", "-framerate", str(FPS), "-i", str(laser_png(puntos, BUILD / f"{prefijo}-laser.png"))]
        i_laser = idx
        idx += 1
    # lluvia de gatos
    i_lluvia = []
    for a, b in ef.lluvia:
        azar = random.Random(esc.numero * 7 + int(a))
        for k in range(6):
            c = k % len(colores)
            gato = gato_png(BUILD / f"gato-{c}-{'der' if k % 2 == 0 else 'izq'}.png", colores[c], k % 2 == 0)
            entradas += ["-loop", "1", "-framerate", str(FPS), "-i", str(gato)]
            i_lluvia.append((idx, a + k * 0.35, b, azar.randint(0, ANCHO - 640)))
            idx += 1

    # --- video
    v = base
    if not ef.normal and gif is not None:  # la diapositiva ya salió frita de Pillow; la terminal se fríe aquí
        v += ",eq=saturation=2.3:contrast=1.5:gamma=0.92,unsharp=5:5:1.8,noise=alls=10:allf=t"
    if ef.punches or ef.sacudidas:
        v += "," + expresiones_zoompan(ef, esc.numero)
    v += ",format=yuv420p"
    etapa = 0
    for i, a, b in i_rotulos:
        cadena.append(v + f"[v{etapa}]")
        v = f"[v{etapa}][{i}:v]overlay=0:0:shortest=1:enable='between(t,{a:.2f},{b:.2f})'"
        etapa += 1
    booms = [t for n, t in ef.sonidos if n == "boom"]
    if i_expl and booms:
        azar = random.Random(esc.numero * 3)
        for tb in booms:
            cx, cy = azar.randint(200, ANCHO - 200) - 450, azar.randint(150, ALTO - 150) - 450
            for j, i in enumerate(i_expl):
                a, b = tb + j * 0.07, tb + (j + 1) * 0.07
                cadena.append(v + f"[v{etapa}]")
                v = f"[v{etapa}][{i}:v]overlay={cx}:{cy}:shortest=1:enable='between(t,{a:.2f},{b:.2f})'"
                etapa += 1
    if i_laser is not None:
        cadena.append(v + f"[v{etapa}]")
        activo = "+".join(f"between(t,{a:.2f},{b:.2f})" for _, a, b in ef.ojos)
        v = f"[v{etapa}][{i_laser}:v]overlay=0:0:shortest=1:enable='({activo})*lt(mod(t,0.34),0.26)'"
        etapa += 1
    if i_ojos is not None:
        cadena.append(v + f"[v{etapa}]")
        activo = "+".join(f"between(t,{a:.2f},{b:.2f})" for _, a, b in ef.ojos)
        v = f"[v{etapa}][{i_ojos}:v]overlay=0:0:shortest=1:enable='({activo})*lt(mod(t,0.34),0.22)'"
        etapa += 1
    for ig, iflash, t, desde_izq in i_gatos:
        a, b = t - 0.35, t + 2.1
        y = ALTO - 44 - 520
        retroceso = f"9*sin(95*t)*between(t,{t:.2f},{t + 1.1:.2f})"
        if desde_izq:
            x = f"-640+760*min(1,(t-{a:.2f})*4)+{retroceso}"
            xf, fy = f"({x})+630", y + 349 - 100  # el cañón apunta a la derecha
        else:
            x = f"{ANCHO}-760*min(1,(t-{a:.2f})*4)-{retroceso}"
            xf, fy = f"({x})-290", y + 349 - 100
        cadena.append(v + f"[v{etapa}]")
        v = f"[v{etapa}][{ig}:v]overlay=x='{x}':y={y}:eval=frame:shortest=1:enable='between(t,{a:.2f},{b:.2f})'"
        etapa += 1
        cadena.append(v + f"[v{etapa}]")
        v = (f"[v{etapa}][{iflash}:v]overlay=x='{xf}':y={fy}:eval=frame:shortest=1:"
             f"enable='between(t,{t:.2f},{t + 1.1:.2f})*lt(mod(t,0.1),0.055)'")
        etapa += 1
    for i, a, b, x in i_lluvia:
        cadena.append(v + f"[v{etapa}]")
        v = (f"[v{etapa}][{i}:v]overlay=x={x}:y='-520+(t-{a:.2f})*700':eval=frame:shortest=1:"
             f"enable='between(t,{a:.2f},{b:.2f})'")
        etapa += 1
    v += f",fade=t=in:st=0:d={FUNDIDO},fade=t=out:st={total - FUNDIDO:.3f}:d={FUNDIDO}[v]"
    cadena.append(v)

    # --- audio: narración tipo Loquendo (banda estrecha y un poco de bitcrush), ducking con los efectos
    ms = int(SILENCIO_INICIO * 1000)
    narr = (f"[{i_voz}:a]adelay={ms}|{ms},highpass=f=160,lowpass=f=5200,"
            f"acrusher=bits=10:mode=log:aa=1:mix=0.35,volume=1.1,apad,aresample=48000,aformat=channel_layouts=stereo")
    if i_sfx:
        for k, (i, t) in enumerate(i_sfx):
            d = int(t * 1000)
            cadena.append(f"[{i}:a]adelay={d}|{d},aresample=48000,aformat=channel_layouts=stereo[s{k}]")
        cadena.append("".join(f"[s{k}]" for k in range(len(i_sfx))) +
                      f"amix=inputs={len(i_sfx)}:normalize=0:dropout_transition=0,bass=g=2:f=90,"
                      f"volume=0.45,apad,asplit[sfxa][sfxb]")
        cadena.append(narr + "[narr]")
        cadena.append("[narr][sfxa]sidechaincompress=threshold=0.06:ratio=4:attack=10:release=250[nd]")
        cadena.append("[nd][sfxb]amix=inputs=2:normalize=0:dropout_transition=0,alimiter=limit=0.95[a]")
    else:
        cadena.append(narr + "[a]")

    correr(entradas + ["-filter_complex", ";".join(cadena), "-map", "[v]", "-map", "[a]",
                       "-t", f"{total:.3f}"] + codec_salida(salida))
    return total


# ----------------------------------------------------------------------------- montaje


def montar() -> None:
    BUILD.mkdir(parents=True, exist_ok=True)
    escenas = leer_guion(GUION)
    equipo = leer_equipo(EQUIPO)
    total = len(escenas)
    clips, resumen = [], []
    for esc in escenas:
        tipo, rutas, pie, extras = interpretar_pantalla(esc)
        ef = leer_efectos(esc.efectos) if CURSED else Efectos()
        voz = narracion(esc)
        dur_voz = duracion(voz)
        base = f"escena-{esc.numero:02d}"
        claves = [esc.titulo, esc.pantalla, str(voz), f"{dur_voz:.3f}", str(total), " ".join(equipo),
                  huella_insumos(rutas)]
        if CURSED:
            claves += [esc.efectos, json.dumps(esc.textos, ensure_ascii=False), "cursed-v4"]
        clip = BUILD / f"{base}-{huella(*claves)}.mp4"
        png = BUILD / f"{base}.png"
        if tipo == "caratula":
            img = diapositiva_caratula(equipo)
        elif tipo in ("figura", "figuras"):
            crop = tuple(int(v) for v in extras["crop"].split(",")) if "crop" in extras else None
            img = diapositiva_figura(esc, total, rutas, pie, crop)
        elif tipo == "texto":
            titulo = rutas[0] if rutas else esc.titulo
            lineas = [l.strip() for l in pie.split(";") if l.strip()]
            img = diapositiva_texto(esc, total, titulo, lineas)
        elif tipo == "terminal":
            img = fondo_terminal(esc, total, pie)
        elif tipo == "video":
            video = RAIZ / rutas[0]
            if not video.exists():
                sys.exit(f"escena {esc.numero}: falta {video} (renderizarlo con Remotion, ver docs/video/remotion/README.md)")
            if CURSED:
                sys.exit(f"escena {esc.numero}: la pantalla «video» no tiene modo cursed")
            img = None
        else:
            sys.exit(f"escena {esc.numero}: pantalla desconocida «{tipo}»")
        if img is not None:
            if CURSED and not ef.normal and tipo != "terminal":
                img = freir(img)
            img.save(png)
        if clip.exists() and not clip_valido(clip):  # un montaje interrumpido deja un mp4 a medias
            clip.unlink()
        if not clip.exists():
            for viejo in BUILD.glob(f"{base}-*.mp4"):
                viejo.unlink()
            if CURSED:
                gif = None
                if tipo == "terminal":
                    gif, _ = render_terminal(rutas[0], objetivo=dur_voz, velocidad=ef.velocidad)
                dur = clip_cursed(esc, png, gif, voz, clip, ef)
                print(f"  escena {esc.numero:2d}  {dur:6.1f} s  cursed {tipo}  «{esc.titulo}»")
            elif tipo == "terminal":
                gif, vel = render_terminal(rutas[0], objetivo=dur_voz * 0.75)
                dur = clip_terminal(png, gif, voz, clip)
                print(f"  escena {esc.numero:2d}  {dur:6.1f} s  terminal {rutas[0]} a {vel:.2f}×  «{esc.titulo}»")
            elif tipo == "video":
                dur = clip_video(RAIZ / rutas[0], voz, clip)
                print(f"  escena {esc.numero:2d}  {dur:6.1f} s  video {rutas[0]} ({duracion(RAIZ / rutas[0]):.0f} s)  «{esc.titulo}»")
            else:
                dur = clip_estatico(png, voz, clip)
                print(f"  escena {esc.numero:2d}  {dur:6.1f} s  {tipo}  «{esc.titulo}»")
        else:
            dur = duracion(clip)
            print(f"  escena {esc.numero:2d}  {dur:6.1f} s  (ya estaba)  «{esc.titulo}»")
        clips.append(clip)
        resumen.append({"escena": esc.numero, "titulo": esc.titulo, "segundos": round(dur, 2),
                        "voz": "propia" if voz.parent == VOZ_PROPIA else VOZ})

    lista = BUILD / "lista.txt"
    lista.write_text("".join(f"file '{c}'\n" for c in clips), encoding="utf-8")
    correr([
        FFMPEG, "-y", "-loglevel", "error", "-f", "concat", "-safe", "0", "-i", str(lista),
        "-c", "copy", "-movflags", "+faststart", str(SALIDA),
    ])
    (BUILD / "escenas.json").write_text(json.dumps(resumen, ensure_ascii=False, indent=2), encoding="utf-8")
    total_s = sum(r["segundos"] for r in resumen)
    print(f"{SALIDA}  {int(total_s // 60)} min {total_s % 60:.0f} s  ({SALIDA.stat().st_size / 1e6:.1f} MB)")


if __name__ == "__main__":
    montar()
