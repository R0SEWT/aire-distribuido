"""Monta el video del TB2 a partir de guion-tb2.md y de los clips de voz y cámara de cada integrante.

Lo llama armar-tb2.sh (no se corre a mano):

    uv run --with pillow python docs/video/montar_tb2.py [--final] [--tiempos] [--hojas]

Por cada ``## Bloque NN`` del guion:
  1. Busca el clip de quien habla en docs/video/clips/NN-nombre.* (video o solo audio). Recorta el
     silencio del inicio y del final, limpia y nivela la voz (EBU R128, -16 LUFS). Si el clip no está,
     usa una voz sintética (edge-tts) y marca el bloque como BORRADOR.
  2. Arma lo que se ve detrás, de la duración exacta de esa voz: escenas de Remotion renderizadas a esa
     duración, grabaciones de la SPA (pantalla/grabar.mjs) aceleradas o frenadas hasta un 40 %,
     grabaciones de terminal (casts/, con agg) y diapositivas de Pillow.
  3. Pone la cámara encima: en un círculo (`burbuja`, abajo a la derecha, `izq` o `arriba`) o a
     pantalla completa (`completa`), con el nombre y el rol.
y concatena todo en docs/video/build/tb2/video-tb2.mp4 (video-tb2-borrador.mp4 si falta algún clip).

--final se niega a montar si falta un clip o si el video pasa de 6 minutos. --tiempos solo estima la
duración de cada bloque (por palabras y con la voz sintética) sin montar. --hojas escribe, por persona,
un PDF con sus textos en letra grande para leer frente a la cámara (build/tb2/guion-NOMBRE.pdf).

En exodia (lo que hace armar-tb2.sh): --preparar, en la Lenovo, deja en build/tb2/exodia/repo todo lo
que el montaje necesita sin codificar nada con ffmpeg (voces sintéticas, escenas de Remotion, GIF de la
terminal, clips, grabaciones y figuras); allá, con INSUMOS_PREVIOS=1, se monta igual que en local.

Variables de entorno: FFMPEG, FFPROBE, AGG, CHROME (para Remotion), CLIPS (carpeta de los clips),
RITMO_TB2 (velocidad de la voz sintética; por omisión -5 %, cerca del ritmo de una persona leyendo),
CODEC_VIDEO («h264_nvenc» para codificar en una GPU NVIDIA con un ffmpeg que lo traiga; por omisión
libx264, que es el que se probó).
"""

from __future__ import annotations

import json
import os
import re
import subprocess
import sys
import unicodedata
from dataclasses import dataclass, field
from pathlib import Path

from PIL import Image, ImageDraw, ImageFont

import montar as pc3  # mismas herramientas que el video de la PC3: ffmpeg, agg, caché por huella

RAIZ = pc3.RAIZ
VIDEO = RAIZ / "docs" / "video"
BUILD = VIDEO / "build" / "tb2"
CLIPS = Path(os.environ.get("CLIPS", VIDEO / "clips"))
GUION = VIDEO / "guion-tb2.md"
PANTALLAS = BUILD / "pantalla"
REMOTION = VIDEO / "remotion"
CHROME = os.environ.get("CHROME", "/usr/bin/google-chrome")
RITMO = os.environ.get("RITMO_TB2", "-5%")
# Montaje en exodia (armar-tb2.sh): en la Lenovo no se codifica con ffmpeg. Allá se preparan las voces
# sintéticas, las escenas de Remotion (traen su propio ffmpeg) y los GIF de la terminal (agg), en
# build/tb2/previos/; con INSUMOS_PREVIOS=1, el montaje los toma de ahí en vez de generarlos.
PREVIOS = BUILD / "previos"
EN_EXODIA = os.environ.get("INSUMOS_PREVIOS") == "1"
PAQUETE = BUILD / "exodia" / "repo"  # lo que se sube a exodia, con la misma estructura del repo

FFMPEG, FFPROBE = pc3.FFMPEG, pc3.FFPROBE
CODEC_VIDEO = os.environ.get("CODEC_VIDEO", "libx264")
ANCHO, ALTO, FPS = 1920, 1080, 30
TOPE_INTERNO = 345  # 5:45
TOPE = 360  # 6:00, lo que pide el enunciado
DIAMETRO = 300  # la burbuja de la cámara
MARGEN = 40
FUNDIDO = 0.15
ANTES, DESPUES = 0.30, 0.45  # silencio alrededor de una voz sintética
EXT_VIDEO = {".mp4", ".mov", ".webm", ".mkv", ".m4v", ".avi"}
EXT_AUDIO = {".m4a", ".mp3", ".wav", ".ogg", ".opus", ".flac", ".aac"}

# La paleta de la SPA y de las escenas de Remotion (src/comun.tsx).
FONDO = (15, 17, 23)
PANEL = (23, 26, 35)
BORDE = (42, 47, 61)
TEXTO = (229, 231, 235)
MUDO = (156, 163, 175)
ACENTO = (42, 120, 214)
ROJO = (208, 59, 59)

TITULO_TRABAJO = "aire-distribuido"
SUBTITULO = "Regímenes de calidad del aire con K-Means distribuido en Go"
CASO = "Caso 10 · Monitoreo de Calidad del Aire y Sensores Ambientales (OEFA)"
CURSO = "1ACC0065 Programación Concurrente y Distribuida · UPC · 2026-20 · Trabajo Final (TB2)"
REPO = "github.com/R0SEWT/aire-distribuido"
SITIO = "aire.rosewt.dev"

PERSONAS = {
    "rody": {"nombre": "Rody Vilchez", "rol": "coordinación · clúster y API", "color": (96, 165, 250),
             "voz": "es-PE-AlexNeural"},
    "dayana": {"nombre": "Dayana Gómez", "rol": "datos · bases e impacto", "color": (74, 222, 128),
               "voz": "es-PE-CamilaNeural"},
    "julio": {"nombre": "Julio Meza", "rol": "API · demo y pruebas", "color": (167, 139, 250),
              "voz": "es-MX-JorgeNeural"},
}


def slug(texto: str) -> str:
    s = unicodedata.normalize("NFD", texto.lower())
    return re.sub(r"[^a-z0-9]+", "-", "".join(c for c in s if unicodedata.category(c) != "Mn")).strip("-")


# ----------------------------------------------------------------------------- fuentes


_FUENTES: dict = {}


def fuente(tamano: int, negrita: bool = False, mono: bool = False) -> ImageFont.FreeTypeFont:
    """Inter (la de la SPA) si está instalada; si no, DejaVu."""
    clave = (tamano, negrita, mono)
    if clave in _FUENTES:
        return _FUENTES[clave]
    f = None
    if not mono:
        extra = [Path(os.environ["FUENTES"]) / "Inter.ttc"] if os.environ.get("FUENTES") else []
        for ttc in (*extra, Path.home() / ".local/share/fonts/Inter/Inter.ttc", Path("/usr/share/fonts/truetype/inter/Inter.ttc")):
            if not ttc.exists():
                continue
            for i in range(40):
                try:
                    cand = ImageFont.truetype(str(ttc), tamano, index=i)
                except OSError:
                    break
                familia, estilo = cand.getname()
                if familia == "Inter" and estilo == ("Bold" if negrita else "Regular"):
                    f = cand
                    break
            if f:
                break
    if f is None:
        f = pc3.fuente(negrita=negrita, mono=mono, tamano=tamano)
    _FUENTES[clave] = f
    return f


# ----------------------------------------------------------------------------- guion


@dataclass
class Pantalla:
    tipo: str
    rutas: list
    pie: str
    extras: dict


@dataclass
class Bloque:
    numero: int
    titulo: str
    quien: str
    objetivo: float
    camara: str
    lado: str
    pantallas: list
    texto: str
    clip: Path | None = None
    voz: Path | None = None
    sintetica: bool = False
    tiene_video: bool = False
    ini: float = 0.0
    fin: float = 0.0
    giro: dict = field(default_factory=dict)


def interpretar(linea: str) -> Pantalla:
    tipo, _, resto = linea.partition(" ")
    campos = [c.strip() for c in resto.split("|")]
    extras = {}
    for c in campos[2:]:
        k, _, v = c.partition("=")
        extras[k.strip()] = v.strip()
    rutas = [r.strip() for r in campos[0].split("+")] if campos and campos[0] else []
    return Pantalla(tipo, rutas, campos[1] if len(campos) > 1 else "", extras)


def leer_guion() -> list[Bloque]:
    texto = GUION.read_text(encoding="utf-8")
    partes = re.split(r"^## Bloque (\d+) · (.+)$", texto, flags=re.M)
    bloques = []
    for i in range(1, len(partes), 3):
        numero, titulo, cuerpo = int(partes[i]), partes[i + 1].strip(), partes[i + 2]

        def campo(nombre: str) -> str:
            m = re.search(rf"^- {nombre}: (.+)$", cuerpo, flags=re.M)
            if not m:
                sys.exit(f"bloque {numero}: falta la línea `- {nombre}:`")
            return m.group(1).strip()

        quien = slug(campo("Quién"))
        if quien not in PERSONAS:
            sys.exit(f"bloque {numero}: «{quien}» no está en PERSONAS de montar_tb2.py")
        mm, ss = campo("Duración").split(":")
        cam = campo("Cámara").split()
        pantallas = [interpretar(m.group(1)) for m in re.finditer(r"^- Pantalla: `([^`]+)`", cuerpo, flags=re.M)]
        if not pantallas:
            sys.exit(f"bloque {numero}: falta `- Pantalla:`")
        d = re.search(r"```dice\n(.*?)```", cuerpo, flags=re.S)
        if not d:
            sys.exit(f"bloque {numero}: falta el bloque ```dice")
        bloques.append(Bloque(
            numero, titulo, quien, int(mm) * 60 + int(ss), cam[0], cam[1] if len(cam) > 1 else "der",
            pantallas, " ".join(l.strip() for l in d.group(1).strip().splitlines()),
        ))
    if not bloques:
        sys.exit("el guion no tiene bloques")
    return bloques


# ----------------------------------------------------------------------------- ffmpeg


def codificar(final: bool) -> list[str]:
    """Los parámetros del codificador de video: los tramos intermedios, rápidos y casi sin pérdida; el
    bloque final, con la calidad del entregable. Con CODEC_VIDEO=h264_nvenc, los ajustes de montar.py
    (probados en una RTX 4060 con el video de la PC3): NVENC comprime menos que x264 a la misma
    «calidad», así que cq va más alto para quedar en un tamaño parecido."""
    if CODEC_VIDEO == "h264_nvenc":
        return ["-c:v", "h264_nvenc", "-preset", "p6", "-rc", "vbr", "-cq", "27" if final else "19", "-b:v", "0",
                "-maxrate", "8M" if final else "20M", "-bufsize", "12M" if final else "30M"]
    if CODEC_VIDEO != "libx264":
        sys.exit(f"CODEC_VIDEO={CODEC_VIDEO}: solo libx264 (por omisión) o h264_nvenc")
    return ["-c:v", "libx264", "-preset", "medium" if final else "veryfast", "-crf", "20" if final else "18"]


def correr(cmd: list[str]) -> None:
    r = subprocess.run([str(c) for c in cmd], capture_output=True, text=True)
    if r.returncode != 0:
        sys.exit(f"falló: {' '.join(str(c) for c in cmd)[:600]}\n{r.stderr[-2500:]}")


def duracion(ruta: Path) -> float:
    return pc3.duracion(ruta)


def sondear(ruta: Path) -> dict:
    r = subprocess.run([FFPROBE, "-v", "error", "-show_streams", "-show_format", "-of", "json", str(ruta)],
                       capture_output=True, text=True)
    if r.returncode != 0:
        sys.exit(f"no se pudo leer {ruta}: {r.stderr}")
    datos = json.loads(r.stdout)
    video = next((s for s in datos["streams"] if s["codec_type"] == "video"
                  and not s.get("disposition", {}).get("attached_pic")), None)
    audio = next((s for s in datos["streams"] if s["codec_type"] == "audio"), None)
    info = {"duracion": float(datos["format"].get("duration", 0)), "audio": audio is not None, "video": video is not None}
    if video:
        w, h = int(video["width"]), int(video["height"])
        giro = 0
        for sd in video.get("side_data_list", []):
            if "rotation" in sd:
                giro = int(float(sd["rotation"]))
        giro = int(video.get("tags", {}).get("rotate", giro))
        if abs(giro) % 180 == 90:  # ffmpeg rota al decodificar: lo que importa es el cuadro ya rotado
            w, h = h, w
        info.update(ancho=w, alto=h)
    return info


def tramo_con_voz(ruta: Path, total: float) -> tuple[float, float]:
    """Dónde empieza y termina la voz: lo que no es silencio (silencedetect), con un margen."""
    r = subprocess.run([FFMPEG, "-hide_banner", "-i", str(ruta), "-vn", "-af",
                        "highpass=f=120,silencedetect=noise=-38dB:d=0.35", "-f", "null", "-"],
                       capture_output=True, text=True)
    inicios = [float(x) for x in re.findall(r"silence_start: (-?[\d.]+)", r.stderr)]
    finales = [float(x) for x in re.findall(r"silence_end: ([\d.]+)", r.stderr)]
    ini, fin = 0.0, total
    if inicios and inicios[0] <= 0.05 and finales:
        ini = finales[0]
    if inicios and (len(finales) < len(inicios) or finales[-1] >= total - 0.05):
        fin = inicios[-1]
    if fin - ini < 1.0:  # algo raro (todo silencio, o ruido constante): no se recorta
        return 0.0, total
    return max(0.0, ini - 0.25), min(total, fin + 0.35)


# ----------------------------------------------------------------------------- la voz


def buscar_clip(b: Bloque) -> Path | None:
    if not CLIPS.is_dir():
        return None
    base = f"{b.numero:02d}-{b.quien}"
    candidatos = [p for p in CLIPS.iterdir() if p.is_file() and p.suffix.lower() in EXT_VIDEO | EXT_AUDIO
                  and slug(p.stem).startswith(base)]
    if not candidatos:
        return None
    exactos = [p for p in candidatos if slug(p.stem) == base]
    # Varias tomas: la exacta si está; si no, la más reciente.
    return (exactos or sorted(candidatos, key=lambda p: p.stat().st_mtime))[-1]


def voz_sintetica(b: Bloque) -> Path:
    """El mp3 de edge-tts del bloque (cacheado por voz, ritmo y texto). No usa ffmpeg."""
    p = PERSONAS[b.quien]
    mp3 = BUILD / f"tts-{b.numero:02d}-{pc3.huella(p['voz'], RITMO, b.texto)}.mp3"
    if not mp3.exists():
        if EN_EXODIA:
            sys.exit(f"falta {mp3.name}: la voz sintética se prepara en la Lenovo (montar_tb2.py --preparar)")
        for viejo in BUILD.glob(f"tts-{b.numero:02d}-*.mp3"):
            viejo.unlink()
        correr(["uvx", "--from", "edge-tts", "edge-tts", "--voice", p["voz"], f"--rate={RITMO}",
                "--text", b.texto, "--write-media", mp3])
    return mp3


def preparar_voz(b: Bloque) -> None:
    """Deja en b.voz un WAV limpio y nivelado, y en b.ini/b.fin el tramo del clip que se usa."""
    b.clip = buscar_clip(b)
    if b.clip is None:
        mp3 = voz_sintetica(b)
        b.sintetica = True
        wav = BUILD / f"voz-{b.numero:02d}-tts-{pc3.huella(str(mp3))}.wav"
        if not wav.exists():
            ms = int(ANTES * 1000)
            correr([FFMPEG, "-y", "-loglevel", "error", "-i", mp3, "-af",
                    f"adelay={ms}|{ms},apad=pad_dur={DESPUES},aresample=48000,aformat=channel_layouts=stereo",
                    "-c:a", "pcm_s16le", wav])
        b.voz = wav
        return
    info = sondear(b.clip)
    if not info["audio"]:
        sys.exit(f"{b.clip} no tiene audio")
    b.tiene_video = info["video"]
    b.giro = info
    b.ini, b.fin = tramo_con_voz(b.clip, info["duracion"])
    huella = pc3.huella(str(b.clip), str(b.clip.stat().st_mtime), str(b.clip.stat().st_size), f"{b.ini:.3f}", f"{b.fin:.3f}")
    wav = BUILD / f"voz-{b.numero:02d}-{huella}.wav"
    if not wav.exists():
        for viejo in BUILD.glob(f"voz-{b.numero:02d}-*.wav"):
            viejo.unlink()
        # Limpieza suave: graves de ventilador y mesa, ruido de fondo constante, y nivel EBU R128.
        correr([FFMPEG, "-y", "-loglevel", "error", "-ss", f"{b.ini:.3f}", "-to", f"{b.fin:.3f}", "-i", b.clip,
                "-vn", "-af", "highpass=f=80,afftdn=nf=-25,loudnorm=I=-16:TP=-1.5:LRA=11,"
                "aresample=48000,aformat=channel_layouts=stereo", "-c:a", "pcm_s16le", wav])
    b.voz = wav


# ----------------------------------------------------------------------------- diapositivas


def lienzo() -> tuple[Image.Image, ImageDraw.ImageDraw]:
    im = Image.new("RGB", (ANCHO, ALTO), FONDO)
    return im, ImageDraw.Draw(im)


def pie_de_pagina(d: ImageDraw.ImageDraw) -> None:
    f = fuente(22)
    d.text((MARGEN + 30, ALTO - 50), f"{TITULO_TRABAJO} · {REPO}", font=f, fill=MUDO)


def encabezado(d: ImageDraw.ImageDraw, titulo: str, bajada: str = "") -> int:
    d.text((90, 56), titulo, font=fuente(58, True), fill=TEXTO)
    y = 56 + 76
    if bajada:
        for linea in pc3.envolver(d, bajada, fuente(30), ANCHO - 180)[:2]:
            d.text((90, y), linea, font=fuente(30), fill=MUDO)
            y += 40
    return y + 20


def diapositiva_caratula() -> Image.Image:
    im, d = lienzo()
    d.rectangle([0, 0, 12, ALTO], fill=ACENTO)
    d.text((110, 120), CURSO, font=fuente(30), fill=MUDO)
    d.text((110, 250), TITULO_TRABAJO, font=fuente(110, True), fill=TEXTO)
    y = 400
    for linea in pc3.envolver(d, SUBTITULO, fuente(46), ANCHO - 220):
        d.text((110, y), linea, font=fuente(46), fill=TEXTO)
        y += 60
    d.text((110, y + 20), CASO, font=fuente(34), fill=MUDO)
    y += 140
    for p in PERSONAS.values():
        d.ellipse([110, y + 10, 130, y + 30], fill=p["color"])
        d.text((150, y), p["nombre"], font=fuente(38, True), fill=TEXTO)
        d.text((150 + d.textlength(p["nombre"], font=fuente(38, True)) + 24, y + 8), p["rol"], font=fuente(28), fill=MUDO)
        y += 58
    d.text((110, ALTO - 110), f"{REPO}  ·  {SITIO}", font=fuente(30), fill=MUDO)
    return im


def diapositiva_final() -> Image.Image:
    im, d = lienzo()
    d.text((110, 200), TITULO_TRABAJO, font=fuente(110, True), fill=TEXTO)
    d.text((110, 350), SUBTITULO, font=fuente(44), fill=TEXTO)
    y = 470
    for p in PERSONAS.values():
        d.ellipse([110, y + 10, 130, y + 30], fill=p["color"])
        d.text((150, y), p["nombre"], font=fuente(40, True), fill=TEXTO)
        y += 62
    d.text((110, y + 40), REPO, font=fuente(36), fill=(96, 165, 250))
    d.text((110, y + 90), SITIO, font=fuente(36), fill=(96, 165, 250))
    d.text((110, ALTO - 110), "Datos: OEFA, vigilancia de la calidad del aire (2018–2022), licencia ODC-By.",
           font=fuente(26), fill=MUDO)
    return im


def diapositiva_figura(b: Bloque, rutas: list[str], pie: str, crop: tuple | None = None) -> Image.Image:
    im, d = lienzo()
    y0 = encabezado(d, b.titulo)
    y1 = ALTO - (130 if pie else 80)
    figs = [Image.open(RAIZ / r).convert("RGB") for r in rutas]
    if crop and len(figs) == 1:  # crop=x,y,ancho,alto: el área de la captura que responde lo que se narra
        figs = [figs[0].crop((crop[0], crop[1], crop[0] + crop[2], crop[1] + crop[3]))]
    caja_w = (ANCHO - 180 - 40 * (len(figs) - 1)) // len(figs)
    x = 90
    for fig in figs:
        escala = min(caja_w / fig.width, (y1 - y0) / fig.height, 1.6)
        nueva = fig.resize((round(fig.width * escala), round(fig.height * escala)), Image.LANCZOS)
        d.rounded_rectangle([x - 6, y0 - 6, x + caja_w + 6, y1 + 6], radius=18, fill=PANEL, outline=BORDE, width=2)
        im.paste(nueva, (x + (caja_w - nueva.width) // 2, y0 + (y1 - y0 - nueva.height) // 2))
        x += caja_w + 40
    if pie:
        # El pie no llega a la esquina de la cámara.
        for i, linea in enumerate(pc3.envolver(d, pie, fuente(26), ANCHO - 180 - 620)[:2]):
            d.text((90, y1 + 24 + i * 34), linea, font=fuente(26), fill=MUDO)
    return im


def diapositiva_texto(b: Bloque, titulo: str, lineas: list[str]) -> Image.Image:
    # El texto empieza más abajo y termina antes de la columna de la cámara (300 px más el nombre).
    im, d = lienzo()
    y = max(encabezado(d, titulo or b.titulo) + 60, 300)
    f = fuente(44)
    for linea in lineas:
        d.ellipse([96, y + 18, 114, y + 36], fill=ACENTO)
        for l in pc3.envolver(d, linea, f, ANCHO - 140 - 520):
            d.text((140, y), l, font=f, fill=TEXTO)
            y += 58
        y += 30
    return im


def fondo_terminal(b: Bloque, pie: str) -> Image.Image:
    im, d = lienzo()
    d.text((90, 40), b.titulo, font=fuente(40, True), fill=TEXTO)
    if pie:
        linea = pc3.envolver(d, pie, fuente(24), ANCHO - 180 - 620)[0]  # no llega al nombre de abajo a la derecha
        d.text((90, ALTO - 58), linea, font=fuente(24), fill=MUDO)
    return im


def caja_terminal(gif: Path) -> tuple[int, int, int, int]:
    g = Image.open(gif)
    caja_w, caja_h = ANCHO - 2 * MARGEN, ALTO - 110 - 80
    escala = min(1.0, caja_w / g.width, caja_h / g.height)
    w, h = (round(g.width * escala) // 2) * 2, (round(g.height * escala) // 2) * 2
    return w, h, (ANCHO - w) // 2, 110 + (caja_h - h) // 2


# ----------------------------------------------------------------------------- lo que se ve


def segmento_estatico(png: Path, d: float, salida: Path) -> None:
    correr([FFMPEG, "-y", "-loglevel", "error", "-loop", "1", "-framerate", FPS, "-t", f"{d:.3f}", "-i", png,
            "-vf", "format=yuv420p", *codificar(False), "-r", FPS, salida])


def ajustar(entrada: Path, d: float, salida: Path, velocidad: float = 1.0, extra: str = "") -> None:
    """Escala a 1920×1080, cambia la velocidad, congela el último cuadro si falta y corta a d segundos."""
    correr([FFMPEG, "-y", "-loglevel", "error", "-i", entrada, "-an", "-vf",
            f"setpts=PTS/{velocidad:.4f},scale={ANCHO}:{ALTO}:force_original_aspect_ratio=decrease:flags=lanczos,"
            f"pad={ANCHO}:{ALTO}:(ow-iw)/2:(oh-ih)/2:color=0x0f1117,fps={FPS},{extra}"
            f"tpad=stop_mode=clone:stop_duration=600,format=yuv420p",
            "-t", f"{d:.3f}", *codificar(False), salida])


def render_remotion(id_: str, d: float) -> Path:
    segundos = round(d * 2) / 2  # a medio segundo: la caché sirve aunque la voz cambie unas décimas
    fuentes = sorted((REMOTION / "src").glob("*")) + [REMOTION / "datos.py"]
    huella = pc3.huella(id_, str(segundos), *[p.read_text(encoding="utf-8") for p in fuentes if p.is_file()])
    salida = BUILD / "remotion" / f"{id_}-{segundos:g}s-{huella}.mp4"
    if salida.exists():
        return salida
    salida.parent.mkdir(parents=True, exist_ok=True)
    for viejo in salida.parent.glob(f"{id_}-{segundos:g}s-*.mp4"):
        viejo.unlink()
    if not (REMOTION / "node_modules").is_dir():
        correr(["npm", "ci", "--prefix", REMOTION, "--no-audit", "--no-fund"])
    cmd = ["npx", "remotion", "render", id_, str(salida), f"--props={json.dumps({'segundos': segundos})}", "--log=error"]
    if Path(CHROME).exists():
        cmd.append(f"--browser-executable={CHROME}")
    print(f"    remotion {id_} a {segundos:g} s…", flush=True)
    r = subprocess.run(cmd, cwd=REMOTION, capture_output=True, text=True)
    if r.returncode != 0:
        sys.exit(f"falló el render de {id_}:\n{r.stderr[-2000:]}")
    return salida


def segmento(b: Bloque, k: int, p: Pantalla, d: float) -> tuple[Path, str]:
    """Un tramo de lo que se ve en el bloque, de d segundos. Devuelve el mp4 y una nota para el resumen."""
    salida = BUILD / f"seg-{b.numero:02d}-{k}.mp4"
    png = BUILD / f"seg-{b.numero:02d}-{k}.png"
    if p.tipo in ("caratula", "final", "figura", "figuras", "texto"):
        if p.tipo == "caratula":
            im = diapositiva_caratula()
        elif p.tipo == "final":
            im = diapositiva_final()
        elif p.tipo == "texto":
            im = diapositiva_texto(b, p.rutas[0] if p.rutas else "", [l.strip() for l in p.pie.split(";") if l.strip()])
        else:
            crop = tuple(int(v) for v in p.extras["crop"].split(",")) if "crop" in p.extras else None
            im = diapositiva_figura(b, p.rutas, p.pie, crop)
        im.save(png)
        segmento_estatico(png, d, salida)
        return salida, p.tipo
    if p.tipo == "remotion":
        if EN_EXODIA:  # renderizada en la Lenovo a la duración estimada: aquí se ajusta a la real
            previo = PREVIOS / "remotion" / f"{p.rutas[0]}.mp4"
            vel = duracion(previo) / d
            ajustar(previo, d, salida, vel)
            return salida, f"remotion {p.rutas[0]} a {vel:.2f}×"
        ajustar(render_remotion(p.rutas[0], d), d, salida)
        return salida, f"remotion {p.rutas[0]}"
    if p.tipo == "video":
        ajustar(RAIZ / p.rutas[0], d, salida)
        return salida, f"video {p.rutas[0]}"
    if p.tipo == "pantalla":
        rec = PANTALLAS / f"{p.rutas[0]}.mp4"
        if not rec.exists():
            sys.exit(f"bloque {b.numero}: falta {rec.relative_to(RAIZ)} (bash docs/video/grabar-pantalla.sh)")
        vel = min(1.6, max(0.7, duracion(rec) / d))
        ajustar(rec, d, salida, vel)
        return salida, f"pantalla {p.rutas[0]} a {vel:.2f}×"
    if p.tipo == "terminal":
        vel = float(p.extras["vel"]) if "vel" in p.extras else None
        if EN_EXODIA:  # el GIF lo hizo agg en la Lenovo
            gif = PREVIOS / "gif" / f"{p.rutas[0]}.gif"
            vel = json.loads((PREVIOS / "manifiesto.json").read_text(encoding="utf-8"))["gif"][p.rutas[0]]
        else:
            gif, vel = pc3.render_terminal(p.rutas[0], objetivo=d * 0.85, velocidad=vel)
        fondo_terminal(b, p.pie).save(png)
        w, h, x, y = caja_terminal(gif)
        correr([FFMPEG, "-y", "-loglevel", "error", "-loop", "1", "-framerate", FPS, "-i", png, "-i", gif,
                "-filter_complex",
                f"[1:v]scale={w}:{h}:flags=lanczos,tpad=stop_mode=clone:stop_duration=600[t];"
                f"[0:v][t]overlay={x}:{y}:shortest=1,format=yuv420p[v]",
                "-map", "[v]", "-t", f"{d:.3f}", "-r", FPS, *codificar(False), salida])
        return salida, f"terminal {p.rutas[0]} a {vel:.2f}×"
    sys.exit(f"bloque {b.numero}: pantalla desconocida «{p.tipo}»")


def partes(b: Bloque) -> list[float]:
    """Qué fracción del bloque ocupa cada pantalla: las que dicen parte=, eso; las otras, el resto."""
    fijas = [float(p.extras["parte"]) for p in b.pantallas if "parte" in p.extras]
    libres = [p for p in b.pantallas if "parte" not in p.extras]
    resto = max(0.0, 1.0 - sum(fijas))
    return [float(p.extras["parte"]) if "parte" in p.extras else resto / max(1, len(libres)) for p in b.pantallas]


def fondo_del_bloque(b: Bloque, total: float) -> tuple[Path, list[str]]:
    duraciones = [total * f for f in partes(b)]
    duraciones[-1] = total - sum(duraciones[:-1])
    segs, notas = [], []
    for k, (p, d) in enumerate(zip(b.pantallas, duraciones)):
        s, nota = segmento(b, k, p, d)
        segs.append(s)
        notas.append(nota)
    if len(segs) == 1:
        return segs[0], notas
    salida = BUILD / f"fondo-{b.numero:02d}.mp4"
    entradas = []
    for s in segs:
        entradas += ["-i", s]
    correr([FFMPEG, "-y", "-loglevel", "error", *entradas, "-filter_complex",
            "".join(f"[{i}:v]" for i in range(len(segs))) + f"concat=n={len(segs)}:v=1:a=0,format=yuv420p[v]",
            "-map", "[v]", *codificar(False), salida])
    return salida, notas


# ----------------------------------------------------------------------------- la cámara


# Modo «columna»: lo que se ve se reduce a la izquierda (sin que nada quede tapado) y la cámara y el
# nombre van en una columna a la derecha. Para las grabaciones de la SPA, que llenan la pantalla.
COLUMNA_W = 1580
COLUMNA_H = round(COLUMNA_W * 9 / 16 / 2) * 2


def posicion(lado: str) -> tuple[int, int]:
    if lado == "columna":
        return COLUMNA_W + (ANCHO - COLUMNA_W - DIAMETRO) // 2, ALTO - MARGEN - DIAMETRO
    if lado == "izq":
        return MARGEN, ALTO - MARGEN - DIAMETRO
    if lado == "arriba":
        return ANCHO - MARGEN - DIAMETRO, MARGEN
    return ANCHO - MARGEN - DIAMETRO, ALTO - MARGEN - DIAMETRO


def mascara() -> Path:
    ruta = BUILD / f"mascara-{DIAMETRO}.png"
    if not ruta.exists():
        escala = 4  # dibujada en grande y reducida: el borde queda suave
        im = Image.new("L", (DIAMETRO * escala, DIAMETRO * escala), 0)
        ImageDraw.Draw(im).ellipse([0, 0, DIAMETRO * escala - 1, DIAMETRO * escala - 1], fill=255)
        im.resize((DIAMETRO, DIAMETRO), Image.LANCZOS).save(ruta)
    return ruta


def capa_burbuja(b: Bloque, con_avatar: bool) -> Path:
    """Lo que va encima del video en modo burbuja: el anillo, el nombre y el rol, y (sin cámara) un
    círculo con las iniciales. Con voz sintética, además, la marca BORRADOR."""
    p = PERSONAS[b.quien]
    x, y = posicion(b.lado)
    im = Image.new("RGBA", (ANCHO, ALTO), (0, 0, 0, 0))
    d = ImageDraw.Draw(im)
    if con_avatar:
        d.ellipse([x, y, x + DIAMETRO, y + DIAMETRO], fill=PANEL + (255,))
        iniciales = "".join(w[0] for w in p["nombre"].split()[:2]).upper()
        f = fuente(110, True)
        d.text((x + (DIAMETRO - d.textlength(iniciales, font=f)) / 2, y + DIAMETRO / 2 - 66), iniciales, font=f, fill=p["color"] + (255,))
    d.ellipse([x - 5, y - 5, x + DIAMETRO + 5, y + DIAMETRO + 5], outline=p["color"] + (255,), width=7)
    # El nombre, al lado de la burbuja.
    fn, fr = fuente(34, True), fuente(24)
    w = int(max(d.textlength(p["nombre"], font=fn), d.textlength(p["rol"], font=fr))) + 48
    h = 96
    if b.lado == "arriba":  # debajo del círculo: a su izquierda está el título de la escena
        cx, cy = x + DIAMETRO - w, y + DIAMETRO + 18
    elif b.lado == "columna":  # encima del círculo, centrado en la columna
        cx, cy = COLUMNA_W + (ANCHO - COLUMNA_W - w) // 2, y - h - 22
    else:
        cy = y + DIAMETRO - h - 10
        cx = x + DIAMETRO + 24 if b.lado == "izq" else x - 24 - w
    d.rounded_rectangle([cx, cy, cx + w, cy + h], radius=16, fill=(15, 17, 23, 225), outline=p["color"] + (255,), width=2)
    d.text((cx + 24, cy + 12), p["nombre"], font=fn, fill=TEXTO + (255,))
    d.text((cx + 24, cy + 56), p["rol"], font=fr, fill=MUDO + (255,))
    if b.sintetica:
        marca_borrador(d, b)
    ruta = BUILD / f"capa-{b.numero:02d}.png"
    im.save(ruta)
    return ruta


def marca_borrador(d: ImageDraw.ImageDraw, b: Bloque) -> None:
    texto = f"BORRADOR · voz sintética · falta {b.numero:02d}-{b.quien}"
    f = fuente(28, True)
    w = d.textlength(texto, font=f) + 40
    x = (ANCHO - w) / 2
    d.rounded_rectangle([x, 14, x + w, 62], radius=12, fill=ROJO + (235,))
    d.text((x + 20, 22), texto, font=f, fill=(255, 255, 255, 255))


def capa_completa(b: Bloque) -> Path:
    """Para la cámara a pantalla completa: un degradado abajo con el nombre y el rol; en el bloque de la
    carátula, también el título del trabajo y el curso arriba."""
    p = PERSONAS[b.quien]
    im = Image.new("RGBA", (ANCHO, ALTO), (0, 0, 0, 0))
    d = ImageDraw.Draw(im)
    for i in range(320):
        a = int(210 * (i / 320) ** 1.6)
        d.line([(0, ALTO - 320 + i), (ANCHO, ALTO - 320 + i)], fill=(10, 12, 16, a))
    d.rectangle([90, ALTO - 170, 98, ALTO - 70], fill=p["color"] + (255,))
    d.text((122, ALTO - 176), p["nombre"], font=fuente(52, True), fill=(255, 255, 255, 255))
    d.text((124, ALTO - 110), p["rol"], font=fuente(32), fill=(220, 225, 232, 255))
    if any(x.tipo == "caratula" for x in b.pantallas):
        for i in range(330):  # velo: el título se lee sobre cualquier fondo
            a = int(235 * (1 - i / 330) ** 1.3)
            d.line([(0, i), (ANCHO, i)], fill=(10, 12, 16, a))
        borde = {"stroke_width": 3, "stroke_fill": (10, 12, 16, 255)}
        d.text((90, 50), TITULO_TRABAJO, font=fuente(64, True), fill=(255, 255, 255, 255), **borde)
        d.text((92, 132), SUBTITULO, font=fuente(32), fill=(235, 238, 242, 255), **borde)
        d.text((92, 178), CURSO, font=fuente(24), fill=(220, 225, 232, 255), **borde)
    if b.sintetica:
        marca_borrador(d, b)
    ruta = BUILD / f"capa-{b.numero:02d}.png"
    im.save(ruta)
    return ruta


def filtro_camara(b: Bloque, modo: str) -> str:
    """El filtro de ffmpeg que deja la cámara del clip lista: recortada al tramo con voz, y en círculo
    o a pantalla completa."""
    w, h = b.giro.get("ancho", 1920), b.giro.get("alto", 1080)
    base = f"[1:v]trim=start={b.ini:.3f}:end={b.fin:.3f},setpts=PTS-STARTPTS,fps={FPS}"
    if modo == "completa":
        if w >= h:
            return (base + f",scale={ANCHO}:{ALTO}:force_original_aspect_ratio=increase,crop={ANCHO}:{ALTO},"
                    "format=yuv420p[cam]")
        # Celular vertical: la imagen al centro sobre una copia desenfocada.
        return (base + ",split[c1][c2];"
                f"[c1]scale={ANCHO}:{ALTO}:force_original_aspect_ratio=increase,crop={ANCHO}:{ALTO},boxblur=30[bg];"
                f"[c2]scale=-2:{ALTO}[fg];[bg][fg]overlay=(W-w)/2:0,format=yuv420p[cam]")
    # Círculo: el lado corto a DIAMETRO y un recorte cuadrado; en vertical, un poco más arriba (la cara).
    escala = f"scale=-2:{DIAMETRO}" if w >= h else f"scale={DIAMETRO}:-2"
    return (base + f",{escala},crop={DIAMETRO}:{DIAMETRO}:(iw-{DIAMETRO})/2:(ih-{DIAMETRO})*0.3,format=yuva420p[c];"
            f"[3:v]format=gray,scale={DIAMETRO}:{DIAMETRO}[m];[c][m]alphamerge[cam]")


def montar_bloque(b: Bloque, salida: Path) -> tuple[float, list[str]]:
    total = duracion(b.voz)
    completa = b.camara == "completa" and b.tiene_video
    notas = []
    fondo, notas = (None, ["cámara completa"]) if completa else fondo_del_bloque(b, total)
    capa = capa_completa(b) if completa else capa_burbuja(b, con_avatar=not b.tiene_video)
    fin_fundido = total - FUNDIDO
    audio = [f"[2:a]aresample=48000,aformat=channel_layouts=stereo,apad[a]"]
    if completa:
        # 0: (no se usa) · 1: el clip · 2: la voz · 3: la capa · 4: la diapositiva final, si la hay
        entradas = ["-f", "lavfi", "-i", f"color=c=0x0f1117:s={ANCHO}x{ALTO}:r={FPS}:d={total:.3f}",
                    "-i", b.clip, "-i", b.voz, "-loop", "1", "-framerate", FPS, "-t", f"{total:.3f}", "-i", capa]
        cadena = [filtro_camara(b, "completa"), "[cam][3:v]overlay=0:0:shortest=1[v0]"]
        ultimo = "[v0]"
        if any(p.tipo == "final" for p in b.pantallas):
            png = BUILD / f"seg-{b.numero:02d}-final.png"
            diapositiva_final().save(png)
            entradas += ["-loop", "1", "-framerate", FPS, "-t", f"{total:.3f}", "-i", png]
            desde = max(0.0, total - 3.5)
            cadena.append(f"[4:v]format=yuva420p,fade=t=in:st={desde:.3f}:d=0.6:alpha=1[fin];"
                          f"{ultimo}[fin]overlay=0:0:shortest=1[v1]")
            ultimo = "[v1]"
            notas.append("con la tarjeta final")
    else:
        x, y = posicion(b.lado)
        entradas = ["-i", fondo]
        base = "[0:v]"
        if b.lado == "columna":
            base = "[f]"
            reducir = (f"[0:v]scale={COLUMNA_W}:{COLUMNA_H}:flags=lanczos,"
                       f"pad={ANCHO}:{ALTO}:0:{(ALTO - COLUMNA_H) // 2}:color=0x0f1117[f]")
        if b.tiene_video:
            entradas += ["-i", b.clip, "-i", b.voz, "-loop", "1", "-framerate", FPS, "-t", f"{total:.3f}", "-i", mascara(),
                         "-loop", "1", "-framerate", FPS, "-t", f"{total:.3f}", "-i", capa]
            cadena = [filtro_camara(b, "burbuja"),
                      f"{base}[cam]overlay={x}:{y}:shortest=1[v0]",
                      "[v0][4:v]overlay=0:0:shortest=1[v1]"]
        else:
            # Sin cámara: un relleno en la entrada 1 para que la voz siga siendo la 2.
            entradas += ["-f", "lavfi", "-i", "anullsrc=r=48000:cl=stereo", "-i", b.voz,
                         "-loop", "1", "-framerate", FPS, "-t", f"{total:.3f}", "-i", capa]
            cadena = [f"{base}[3:v]overlay=0:0:shortest=1[v1]"]
        if b.lado == "columna":
            cadena.insert(0, reducir)
        ultimo = "[v1]"
    cadena.append(f"{ultimo}fps={FPS},format=yuv420p,fade=t=in:st=0:d={FUNDIDO},"
                  f"fade=t=out:st={fin_fundido:.3f}:d={FUNDIDO}[v]")
    correr([FFMPEG, "-y", "-loglevel", "error", *entradas, "-filter_complex", ";".join(cadena + audio),
            "-map", "[v]", "-map", "[a]", "-t", f"{total:.3f}", "-r", FPS,
            *codificar(True), "-pix_fmt", "yuv420p",
            "-c:a", "aac", "-b:a", "160k", "-ar", "48000", salida])
    return total, notas


# ----------------------------------------------------------------------------- hojas para leer


def hojas(bloques: list[Bloque]) -> None:
    """Un PDF por persona con sus bloques en letra grande, uno por página, para leer frente a la cámara."""
    for quien, p in PERSONAS.items():
        paginas = []
        for b in [b for b in bloques if b.quien == quien]:
            im = Image.new("RGB", (1754, 1240), (255, 255, 255))  # A4 apaisada a 150 ppp
            d = ImageDraw.Draw(im)
            d.text((90, 70), f"{b.numero:02d}-{quien}  ·  {b.titulo}", font=fuente(40, True), fill=(20, 20, 20))
            d.text((90, 130), f"Duración: unos {int(b.objetivo)} s  ·  archivo: {b.numero:02d}-{quien}.mp4  ·  "
                   f"cámara {'a pantalla completa' if b.camara == 'completa' else 'en círculo'}",
                   font=fuente(28), fill=(90, 90, 90))
            y = 220
            for linea in pc3.envolver(d, b.texto, fuente(50), 1754 - 180):
                d.text((90, y), linea, font=fuente(50), fill=(10, 10, 10))
                y += 72
            paginas.append(im)
        if paginas:
            ruta = BUILD / f"guion-{quien}.pdf"
            paginas[0].save(ruta, save_all=True, append_images=paginas[1:], resolution=150)
            print(f"  {ruta.relative_to(RAIZ)} ({len(paginas)} páginas)")


# ----------------------------------------------------------------------------- preparar para exodia


def enlazar(origen: Path, destino: Path) -> None:
    """Pone el archivo en el paquete sin copiarlo si se puede (enlace duro, mismo disco)."""
    import shutil
    destino.parent.mkdir(parents=True, exist_ok=True)
    if destino.exists():
        destino.unlink()
    try:
        os.link(origen, destino)
    except OSError:
        shutil.copy2(origen, destino)


def preparar_para_exodia(bloques: list[Bloque], final: bool) -> None:
    """En la Lenovo, sin codificar con ffmpeg: las voces sintéticas que falten (edge-tts), las escenas de
    Remotion y los GIF de la terminal a la duración estimada de cada bloque, y el paquete que se sube a
    exodia (build/tb2/exodia/repo, con la estructura del repo). La duración real se conoce allá, después
    de recortar los silencios; las escenas y los GIF se ajustan a ella con una diferencia de pocos
    segundos. ffprobe solo lee metadatos."""
    import shutil
    faltan = []
    archivos: set[Path] = {VIDEO / "montar.py", VIDEO / "montar_tb2.py", GUION}
    manifiesto: dict = {"remotion": {}, "gif": {}}
    for d in ("remotion", "gif"):
        shutil.rmtree(PREVIOS / d, ignore_errors=True)
        (PREVIOS / d).mkdir(parents=True, exist_ok=True)
    for b in bloques:
        b.clip = buscar_clip(b)
        if b.clip is None:
            faltan.append(f"{b.numero:02d}-{b.quien}")
            mp3 = voz_sintetica(b)
            archivos.add(mp3)
            estimada = duracion(mp3) + ANTES + DESPUES
            origen = "voz sintética"
        else:
            archivos.add(b.clip)
            estimada = max(3.0, sondear(b.clip)["duracion"] - 1.2)  # los silencios de los bordes se recortan allá
            origen = b.clip.name
        print(f"  bloque {b.numero:2d}  ~{estimada:5.1f} s  {origen}", flush=True)
        for p, f in zip(b.pantallas, partes(b)):
            d = estimada * f
            if p.tipo == "remotion":
                enlazar(render_remotion(p.rutas[0], d), PREVIOS / "remotion" / f"{p.rutas[0]}.mp4")
                manifiesto["remotion"][p.rutas[0]] = round(d, 2)
            elif p.tipo == "terminal":
                vel = float(p.extras["vel"]) if "vel" in p.extras else None
                gif, vel = pc3.render_terminal(p.rutas[0], objetivo=d * 0.85, velocidad=vel)
                enlazar(gif, PREVIOS / "gif" / f"{p.rutas[0]}.gif")
                manifiesto["gif"][p.rutas[0]] = vel
                archivos.add(pc3.CASTS / f"{p.rutas[0]}.cast")
            elif p.tipo == "pantalla":
                rec = PANTALLAS / f"{p.rutas[0]}.mp4"
                if not rec.exists():
                    sys.exit(f"bloque {b.numero}: falta {rec.relative_to(RAIZ)} (bash docs/video/grabar-pantalla.sh)")
                archivos.add(rec)
            elif p.tipo in ("figura", "figuras", "video"):
                archivos.update(RAIZ / r for r in p.rutas)
    if final and faltan:
        sys.exit(f"--final: faltan los clips {', '.join(faltan)} en {CLIPS}")
    (PREVIOS / "manifiesto.json").write_text(json.dumps(manifiesto, ensure_ascii=False, indent=2), encoding="utf-8")
    archivos.update((PREVIOS / "remotion").glob("*.mp4"))
    archivos.update((PREVIOS / "gif").glob("*.gif"))
    archivos.add(PREVIOS / "manifiesto.json")

    shutil.rmtree(PAQUETE, ignore_errors=True)
    for a in sorted(archivos):
        # Los clips van a docs/video/clips del paquete aunque aquí estén en otra carpeta (CLIPS).
        rel = Path("docs/video/clips") / a.name if a.parent == CLIPS else a.relative_to(RAIZ)
        enlazar(a, PAQUETE / rel)
    for ttc in (Path.home() / ".local/share/fonts/Inter/Inter.ttc", Path("/usr/share/fonts/truetype/inter/Inter.ttc")):
        if ttc.exists():
            enlazar(ttc, PAQUETE / "fuentes" / "Inter.ttc")
            break
    peso = sum(f.stat().st_size for f in PAQUETE.rglob("*") if f.is_file())
    print(f"paquete: {PAQUETE.relative_to(RAIZ)} ({len(archivos)} archivos, {peso / 1e6:.0f} MB)"
          + (f"; {len(faltan)} bloques con voz sintética" if faltan else ""))


# ----------------------------------------------------------------------------- montaje


def montar() -> None:
    final = "--final" in sys.argv
    solo_tiempos = "--tiempos" in sys.argv
    BUILD.mkdir(parents=True, exist_ok=True)
    bloques = leer_guion()
    if "--hojas" in sys.argv:
        hojas(bloques)
        return
    if "--preparar" in sys.argv:
        preparar_para_exodia(bloques, final)
        return
    for b in bloques:
        preparar_voz(b)
    faltan = [f"{b.numero:02d}-{b.quien}" for b in bloques if b.sintetica]
    if final and faltan:
        sys.exit(f"--final: faltan los clips {', '.join(faltan)} en {CLIPS}")

    print(f"{'bloque':>6}  {'quién':7} {'objetivo':>8} {'voz':>6}  origen")
    for b in bloques:
        palabras = len(b.texto.split())
        origen = "SINTÉTICA" if b.sintetica else f"{b.clip.name} [{b.ini:.1f}–{b.fin:.1f} s]" + ("" if b.tiene_video else " (solo audio)")
        print(f"{b.numero:>6}  {b.quien:7} {b.objetivo:>7.0f}s {duracion(b.voz):>5.1f}s  {origen}  ({palabras} palabras)")
    total_voz = sum(duracion(b.voz) for b in bloques)
    print(f"total: {int(total_voz // 60)}:{total_voz % 60:04.1f}")
    if solo_tiempos:
        return

    clips, resumen = [], []
    for b in bloques:
        claves = [b.titulo, b.camara, b.lado, json.dumps([p.__dict__ for p in b.pantallas], ensure_ascii=False),
                  str(b.voz), str(b.ini), str(b.fin), "tb2-v4", " ".join(codificar(True))]
        for p in b.pantallas:
            if p.tipo == "pantalla":
                claves.append(pc3.huella_insumos([str((PANTALLAS / f"{p.rutas[0]}.mp4").relative_to(RAIZ))]))
            elif p.tipo == "terminal":
                claves.append(pc3.huella_insumos([p.rutas[0]]))
            elif p.tipo == "remotion":
                claves.append(pc3.huella(*[f.read_text(encoding="utf-8") for f in sorted((REMOTION / "src").glob("*"))]))
            elif p.tipo in ("figura", "figuras", "video"):
                claves.append(pc3.huella_insumos(p.rutas))
        clip = BUILD / f"bloque-{b.numero:02d}-{pc3.huella(*claves)}.mp4"
        if clip.exists() and not pc3.clip_valido(clip):
            clip.unlink()
        if clip.exists():
            dur, notas = duracion(clip), ["(ya estaba)"]
        else:
            for viejo in BUILD.glob(f"bloque-{b.numero:02d}-*.mp4"):
                viejo.unlink()
            dur, notas = montar_bloque(b, clip)
        print(f"  bloque {b.numero:2d}  {dur:5.1f} s  {b.quien:7} {'; '.join(notas)}  «{b.titulo}»", flush=True)
        clips.append(clip)
        resumen.append({"bloque": b.numero, "titulo": b.titulo, "quien": b.quien, "segundos": round(dur, 2),
                        "objetivo": b.objetivo, "clip": b.clip.name if b.clip else None,
                        "voz": "sintética" if b.sintetica else ("audio" if not b.tiene_video else "cámara")})

    salida = BUILD / ("video-tb2-borrador.mp4" if faltan else "video-tb2.mp4")
    lista = BUILD / "lista.txt"
    lista.write_text("".join(f"file '{c}'\n" for c in clips), encoding="utf-8")
    correr([FFMPEG, "-y", "-loglevel", "error", "-f", "concat", "-safe", "0", "-i", lista, "-c", "copy",
            "-movflags", "+faststart", salida])
    total = duracion(salida)
    (BUILD / "bloques.json").write_text(json.dumps(resumen, ensure_ascii=False, indent=2), encoding="utf-8")
    print(f"\n{salida.relative_to(RAIZ)}  {int(total // 60)}:{total % 60:04.1f}  ({salida.stat().st_size / 1e6:.1f} MB)")
    por = {}
    for r in resumen:
        por[r["quien"]] = por.get(r["quien"], 0) + r["segundos"]
    print("  por persona: " + ", ".join(f"{PERSONAS[q]['nombre']} {int(s // 60)}:{s % 60:04.1f}" for q, s in por.items()))
    if faltan:
        print(f"  BORRADOR: faltan {len(faltan)} clips ({', '.join(faltan)}); van con voz sintética y la marca roja")
    if total > TOPE:
        msg = f"  dura {total:.0f} s: pasa de los 6 minutos del enunciado"
        if final:
            sys.exit(msg)
        print(msg)
    elif total > TOPE_INTERNO:
        print(f"  dura {total:.0f} s: pasa del tope interno de 5:45 (el enunciado permite hasta 6:00)")
    (BUILD / "salida.txt").write_text(str(salida), encoding="utf-8")


if __name__ == "__main__":
    montar()
