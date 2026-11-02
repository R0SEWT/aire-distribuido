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

Variables de entorno: FFMPEG, FFPROBE, AGG, VOZ (voz de edge-tts), RITMO (su velocidad, p. ej.
"+10%"), EQUIPO (ruta al equipo.yml).
"""

from __future__ import annotations

import hashlib
import json
import os
import re
import subprocess
import sys
from dataclasses import dataclass, field
from pathlib import Path

from PIL import Image, ImageDraw, ImageFont

RAIZ = Path(__file__).resolve().parents[2]
VIDEO = RAIZ / "docs" / "video"
BUILD = VIDEO / "build"
CASTS = VIDEO / "casts"
VOZ_PROPIA = VIDEO / "voz"

FFMPEG = os.environ.get("FFMPEG", "ffmpeg")
FFPROBE = os.environ.get("FFPROBE", "ffprobe")
AGG = os.environ.get("AGG", "agg")
VOZ = os.environ.get("VOZ", "es-PE-AlexNeural")
RITMO = os.environ.get("RITMO", "+15%")  # velocidad de edge-tts; "+0%" es la natural
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


# ----------------------------------------------------------------------------- guion


@dataclass
class Escena:
    numero: int
    titulo: str
    pantalla: str
    narracion: str
    pie: str = ""
    extras: dict = field(default_factory=dict)


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
        escenas.append(Escena(numero, titulo, pantalla, narracion))
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


def huella(*partes: str) -> str:
    return hashlib.sha256("\n".join(partes).encode("utf-8")).hexdigest()[:16]


def narracion(esc: Escena) -> Path:
    """Devuelve la pista de voz de la escena: la propia si existe, si no edge-tts (cacheada)."""
    for ext in ("wav", "mp3", "m4a", "ogg", "flac"):
        propia = VOZ_PROPIA / f"escena-{esc.numero:02d}.{ext}"
        if propia.exists():
            return propia
    salida = BUILD / f"voz-{esc.numero:02d}-{huella(VOZ, RITMO, esc.narracion)}.mp3"
    if not salida.exists():
        for vieja in BUILD.glob(f"voz-{esc.numero:02d}-*.mp3"):
            vieja.unlink()
        correr([
            "uvx", "--from", "edge-tts", "edge-tts", "--voice", VOZ, f"--rate={RITMO}",
            "--text", esc.narracion, "--write-media", str(salida),
        ])
    return salida


def duracion_cast(cast: Path) -> float:
    ultimo = 0.0
    with cast.open(encoding="utf-8") as f:
        next(f)  # cabecera
        for linea in f:
            if linea.strip():
                ultimo = json.loads(linea)[0]
    return ultimo


def render_terminal(nombre: str, objetivo: float) -> tuple[Path, float]:
    """Renderiza el .cast a GIF con agg, a una velocidad que lo acerque a `objetivo` segundos."""
    cast = CASTS / f"{nombre}.cast"
    if not cast.exists():
        sys.exit(f"falta la grabación {cast}: correr docs/video/grabar.sh {nombre}")
    real = duracion_cast(cast)
    velocidad = min(2.0, max(0.3, real / max(objetivo, 1.0)))
    gif = BUILD / f"term-{nombre}-{huella(str(real), f'{velocidad:.3f}')}.gif"
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
        "-c:v", "libx264", "-preset", "medium", "-crf", "20", "-r", str(FPS),
        "-c:a", "aac", "-b:a", "128k", "-ar", "48000", str(salida),
    ])
    return total


def clip_terminal(fondo: Path, gif: Path, voz: Path, salida: Path) -> float:
    total = SILENCIO_INICIO + duracion(voz) + SILENCIO_FIN
    g = Image.open(gif)
    # La terminal cabe en el área entre la cabecera y el pie, centrada.
    caja_w, caja_h = ANCHO - 2 * 40, ALTO - 124 - 44 - 40
    escala = min(1.0, caja_w / g.width, caja_h / g.height)
    w, h = (round(g.width * escala) // 2) * 2, (round(g.height * escala) // 2) * 2
    x, y = (ANCHO - w) // 2, 124 + (caja_h - h) // 2
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
        "-c:v", "libx264", "-preset", "medium", "-crf", "20", "-r", str(FPS),
        "-c:a", "aac", "-b:a", "128k", "-ar", "48000", str(salida),
    ])
    return total


def montar() -> None:
    BUILD.mkdir(parents=True, exist_ok=True)
    escenas = leer_guion(VIDEO / "guion.md")
    equipo = leer_equipo(EQUIPO)
    total = len(escenas)
    clips, resumen = [], []
    for esc in escenas:
        tipo, rutas, pie, extras = interpretar_pantalla(esc)
        voz = narracion(esc)
        dur_voz = duracion(voz)
        base = f"escena-{esc.numero:02d}"
        clave = huella(esc.titulo, esc.pantalla, str(voz), f"{dur_voz:.3f}", str(total), " ".join(equipo))
        clip = BUILD / f"{base}-{clave}.mp4"
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
        else:
            sys.exit(f"escena {esc.numero}: pantalla desconocida «{tipo}»")
        img.save(png)
        if not clip.exists():
            for viejo in BUILD.glob(f"{base}-*.mp4"):
                viejo.unlink()
            if tipo == "terminal":
                gif, vel = render_terminal(rutas[0], objetivo=dur_voz * 0.75)
                dur = clip_terminal(png, gif, voz, clip)
                print(f"  escena {esc.numero:2d}  {dur:6.1f} s  terminal {rutas[0]} a {vel:.2f}×  «{esc.titulo}»")
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
    final = BUILD / "video-pc3.mp4"
    correr([
        FFMPEG, "-y", "-loglevel", "error", "-f", "concat", "-safe", "0", "-i", str(lista),
        "-c", "copy", "-movflags", "+faststart", str(final),
    ])
    (BUILD / "escenas.json").write_text(json.dumps(resumen, ensure_ascii=False, indent=2), encoding="utf-8")
    total_s = sum(r["segundos"] for r in resumen)
    print(f"{final}  {int(total_s // 60)} min {total_s % 60:.0f} s  ({final.stat().st_size / 1e6:.1f} MB)")


if __name__ == "__main__":
    montar()
