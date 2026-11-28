"""Revisa los clips de voz y cámara del video del TB2 contra guion-tb2.md y transcribe el mp4 montado.

Corre en exodia (lo lanza revisar-clips.sh con exodia-ff): usa ffprobe, ffmpeg y faster-whisper.

    python revisar_clips.py --guion guion-tb2.md --clips clips --salida out
    python revisar_clips.py --guion guion-tb2.md --video video-tb2.mp4 --bloques bloques.json --salida out

Por cada clip (NN-nombre.*, como los busca el montaje):
  - la duración de la voz (sin los silencios de los bordes) contra la del guion: avisa si se pasa por
    más de 5 s;
  - si el video es horizontal y de 720p o más (o si es solo audio);
  - el nivel: saturado (pico verdadero de -0,5 dBTP o más) o muy bajo (menos de -35 LUFS), y los
    silencios del inicio y del final;
  - la transcripción contra el texto del guion: las CIFRAS (una cifra que no se oye o que se oye otra es
    una observación) y las palabras (se puede decir con palabras propias: es informativo).
Con --video transcribe el mp4 montado, bloque por bloque según bloques.json, para el corrector en frío.

Escribe revision-clips.md y revision-clips.json (o transcripcion-video.md y .json, y transcripcion-docente.md,
solo lo que se oye, para el corrector en frío) en --salida.
Modelo de faster-whisper: WHISPER_MODELO (por omisión «small»), en CPU con int8.
"""

from __future__ import annotations

import argparse
import difflib
import json
import os
import re
import subprocess
import sys
import unicodedata
from dataclasses import dataclass
from pathlib import Path

EXT = {".mp4", ".mov", ".webm", ".mkv", ".m4v", ".avi", ".m4a", ".mp3", ".wav", ".ogg", ".opus", ".flac", ".aac"}
TOLERANCIA_S = 5.0
FFMPEG = os.environ.get("FFMPEG", "ffmpeg")
FFPROBE = os.environ.get("FFPROBE", "ffprobe")
MODELO = os.environ.get("WHISPER_MODELO", "small")
# Términos técnicos para orientar a whisper. Sin cifras, para que no «oiga» la cifra esperada.
VOCABULARIO = ("aire-distribuido, OEFA, ECA, PM2,5, PM10, K-Means, goroutines, workers, WaitGroup, net.Listen, "
               "Accept, net.Dialer, json.Marshal, bufio.Reader, CARGAR, ITERAR, PARCIAL, MongoDB, Redis, "
               "WebSocket, net/http, OpenAPI, JWT, Docker Compose, SHA-256, Remotion, SPA, React, horas-estación.")


# ----------------------------------------------------------------------------- guion


@dataclass
class Bloque:
    numero: int
    titulo: str
    quien: str
    objetivo: float
    texto: str


def slug(texto: str) -> str:
    s = unicodedata.normalize("NFD", texto.lower())
    return re.sub(r"[^a-z0-9]+", "-", "".join(c for c in s if unicodedata.category(c) != "Mn")).strip("-")


def leer_guion(ruta: Path) -> list[Bloque]:
    partes = re.split(r"^## Bloque (\d+) · (.+)$", ruta.read_text(encoding="utf-8"), flags=re.M)
    bloques = []
    for i in range(1, len(partes), 3):
        cuerpo = partes[i + 2]
        quien = slug(re.search(r"^- Quién: (.+)$", cuerpo, re.M).group(1))
        mm, ss = re.search(r"^- Duración: (\d+):(\d+)", cuerpo, re.M).groups()
        texto = " ".join(re.search(r"```dice\n(.*?)```", cuerpo, re.S).group(1).split())
        bloques.append(Bloque(int(partes[i]), partes[i + 1].strip(), quien, int(mm) * 60 + int(ss), texto))
    return bloques


# ----------------------------------------------------------------------------- cifras en español

UNIDADES = {"cero": 0, "un": 1, "uno": 1, "una": 1, "dos": 2, "tres": 3, "cuatro": 4, "cinco": 5, "seis": 6,
            "siete": 7, "ocho": 8, "nueve": 9, "diez": 10, "once": 11, "doce": 12, "trece": 13, "catorce": 14,
            "quince": 15, "dieciseis": 16, "diecisiete": 17, "dieciocho": 18, "diecinueve": 19, "veinte": 20,
            "veintiun": 21, "veintiuno": 21, "veintiuna": 21, "veintidos": 22, "veintitres": 23, "veinticuatro": 24,
            "veinticinco": 25, "veintiseis": 26, "veintisiete": 27, "veintiocho": 28, "veintinueve": 29}
DECENAS = {"treinta": 30, "cuarenta": 40, "cincuenta": 50, "sesenta": 60, "setenta": 70, "ochenta": 80, "noventa": 90}
CENTENAS = {"cien": 100, "ciento": 100, "doscientos": 200, "doscientas": 200, "trescientos": 300, "trescientas": 300,
            "cuatrocientos": 400, "cuatrocientas": 400, "quinientos": 500, "quinientas": 500, "seiscientos": 600,
            "seiscientas": 600, "setecientos": 700, "setecientas": 700, "ochocientos": 800, "ochocientas": 800,
            "novecientos": 900, "novecientas": 900}


def normalizar(texto: str) -> str:
    """Minúsculas, sin tildes, con los nombres que no son cifras (PM2,5, PM10, SHA-256, K-Means…) como
    palabras, y los números escritos con separadores de miles unidos."""
    t = unicodedata.normalize("NFD", texto.lower())
    t = "".join(c for c in t if unicodedata.category(c) != "Mn")
    t = re.sub(r"\bp\s*\.?\s*m\s*\.?\s*2\s*[.,]\s*5\b|\bpe\s+eme\s+dos\s+coma\s+cinco\b", " pmdoscinco ", t)
    t = re.sub(r"\bp\s*\.?\s*m\s*\.?\s*10\b|\bpe\s+eme\s+diez\b", " pmdiez ", t)
    t = re.sub(r"\b(?:sha|sa|sai|cha|shah)\s*-?\s*256\b|\bsha\s+doscientos\s+cincuenta\s+y\s+seis\b", " shadoscincuentayseis ", t)
    t = re.sub(r"\bk\s*-?\s*means\b", " kmeans ", t)
    t = re.sub(r"\bmp4\b", " mpcuatro ", t)
    # Un grupo de exactamente tres cifras después de espacio, punto o coma es de miles (el guion no tiene
    # decimales de tres cifras): 64 748, 64.748 y 64,748 son 64748; 1,3 es un decimal.
    t = re.sub(r"(?<![\d.,])(\d{1,3})((?:[ .,]\d{3})+)(?![\d.,]?\d)", lambda m: m.group(1) + re.sub(r"[ .,]", "", m.group(2)), t)
    t = re.sub(r"(\d+),(\d+)", r"\1.\2", t)
    t = re.sub(r"(\d)\s*%", r"\1 por ciento", t)
    t = re.sub(r"[^\w.\s]", " ", t)
    t = re.sub(r"(?<!\d)\.|\.(?!\d)", " ", t)
    return " ".join(t.split())


def pulir(toks: list[str], tiempos: list[float] | None = None) -> tuple[list[str], list[float]]:
    """Une lo que la transcripción parte en varias palabras: «2 883 000» (miles), «sha 256», «pm 2 5»,
    «pm 10», «k means». Se aplica igual al guion y a lo oído."""
    ts = list(tiempos) if tiempos is not None else [0.0] * len(toks)
    out, out_t = [], []
    i = 0
    while i < len(toks):
        t, ti = toks[i], ts[i]
        sig = toks[i + 1] if i + 1 < len(toks) else ""
        sig2 = toks[i + 2] if i + 2 < len(toks) else ""
        if t in ("sha", "sa", "sai", "shah", "cha", "chat") and sig == "256":
            out.append("shadoscincuentayseis"); out_t.append(ti); i += 2; continue
        if t in ("pm", "pe") and sig == "2" and sig2 == "5":
            out.append("pmdoscinco"); out_t.append(ti); i += 3; continue
        if t in ("pm2", "pm") and sig in ("5", "25") or t == "pm25":
            out.append("pmdoscinco"); out_t.append(ti); i += 1 if t == "pm25" else 2; continue
        if t in ("pm", "pe") and sig == "10" or t == "pm10":
            out.append("pmdiez"); out_t.append(ti); i += 1 if t == "pm10" else 2; continue
        if t == "k" and sig == "means":
            out.append("kmeans"); out_t.append(ti); i += 2; continue
        if (out and re.fullmatch(r"\d{3}", t) and re.fullmatch(r"\d{1,3}(\d{3})*", out[-1])):
            out[-1] += t; i += 1; continue  # 2 883 000 → 2883000
        if t in UNIDADES and UNIDADES[t] < 10 and re.fullmatch(r"\d{3}", sig):
            out.append(f"{UNIDADES[t]}{sig}"); out_t.append(ti); i += 2; continue  # «uno 284» → 1284
        out.append(t); out_t.append(ti); i += 1
    return out, out_t


def fichas(texto: str) -> list[str]:
    return pulir(normalizar(texto).split())[0]


ARTICULOS = {"un", "una", "uno"}  # solos son artículos («un pool», «una SPA»); dentro de un número, cifras


def cifras(tokens: list[str]) -> list[tuple[float, int, int]]:
    """Los números de una lista de fichas: (valor, primera ficha, última ficha). Une «2 millones 883 mil»,
    «sesenta y cuatro mil setecientos cuarenta y ocho» y «uno coma tres»; «setenta setenta» (o «70 70»)
    es 7070, como se dicen los puertos."""
    crudas = []
    i = 0
    while i < len(tokens):
        if not es_numero(tokens[i]):
            i += 1
            continue
        inicio = i
        total, grupo, decimal = 0.0, 0.0, None
        visto = False
        while i < len(tokens):
            t = tokens[i]
            if visto and grupo % 100 >= 10 and (t in DECENAS or t in UNIDADES and UNIDADES[t] >= 10
                                                or re.fullmatch(r"\d+", t)) and tokens[i - 1] != "y":
                break  # «setenta setenta»: dos números seguidos, se juntan abajo
            if re.fullmatch(r"\d+(\.\d+)?", t):
                if visto and grupo:
                    break
                grupo += float(t)
            elif t in UNIDADES:
                grupo += UNIDADES[t]
            elif t in DECENAS:
                grupo += DECENAS[t]
            elif t in CENTENAS:
                grupo += CENTENAS[t]
            elif t == "y" and visto and i + 1 < len(tokens) and tokens[i + 1] in UNIDADES:
                pass
            elif t == "mil" and visto:
                total += (grupo or 1) * 1000
                grupo = 0
            elif t in ("millon", "millones") and visto:
                total += (grupo or 1) * 1_000_000
                grupo = 0
            elif t == "coma" and visto and i + 1 < len(tokens) and es_numero(tokens[i + 1]):
                decimal = tokens[i + 1]
                i += 2
                break
            else:
                break
            visto = True
            i += 1
        if i == inicio:
            i += 1
            continue
        valor = total + grupo
        if decimal is not None:
            d = str(int(float(decimal))) if re.fullmatch(r"\d+(\.0)?", decimal) else str(UNIDADES.get(decimal, decimal))
            valor = float(f"{int(valor)}.{d}")
        if decimal is None and i - inicio == 1 and tokens[inicio] in ARTICULOS:
            continue
        crudas.append((valor, inicio, i - 1))
    salida: list[tuple[float, int, int]] = []
    for v, a, b in crudas:  # dos números de dos cifras seguidos: 70 70 → 7070
        if salida and salida[-1][2] == a - 1 and 10 <= salida[-1][0] <= 99 and 10 <= v <= 99 and v == int(v):
            pv, pa, _ = salida[-1]
            salida[-1] = (float(f"{int(pv)}{int(v)}"), pa, b)
        else:
            salida.append((v, a, b))
    return salida


def es_numero(t: str) -> bool:
    return bool(re.fullmatch(r"\d+(\.\d+)?", t)) or t in UNIDADES or t in DECENAS or t in CENTENAS


def fmt(v: float) -> str:
    return f"{v:,.0f}".replace(",", " ") if v == int(v) else f"{v:g}".replace(".", ",")


# ----------------------------------------------------------------------------- ffmpeg


def correr(cmd: list) -> subprocess.CompletedProcess:
    return subprocess.run([str(c) for c in cmd], capture_output=True, text=True)


def sondear(ruta: Path) -> dict:
    r = correr([FFPROBE, "-v", "error", "-show_streams", "-show_format", "-of", "json", ruta])
    d = json.loads(r.stdout or "{}")
    video = next((s for s in d.get("streams", []) if s["codec_type"] == "video"
                  and not s.get("disposition", {}).get("attached_pic")), None)
    audio = next((s for s in d.get("streams", []) if s["codec_type"] == "audio"), None)
    info = {"duracion": float(d.get("format", {}).get("duration", 0)), "audio": audio is not None, "video": video is not None}
    if video:
        w, h = int(video["width"]), int(video["height"])
        giro = int(video.get("tags", {}).get("rotate", 0))
        for sd in video.get("side_data_list", []):
            if "rotation" in sd:
                giro = int(float(sd["rotation"]))
        if abs(giro) % 180 == 90:
            w, h = h, w
        info.update(ancho=w, alto=h, fps=video.get("avg_frame_rate", ""))
    return info


def voz_y_nivel(ruta: Path, total: float) -> dict:
    """Silencios de los bordes (como los recorta el montaje) y el nivel: EBU R128 y pico verdadero."""
    r = correr([FFMPEG, "-hide_banner", "-nostdin", "-i", ruta, "-vn", "-af",
                "highpass=f=120,silencedetect=noise=-38dB:d=0.35", "-f", "null", "-"])
    inicios = [float(x) for x in re.findall(r"silence_start: (-?[\d.]+)", r.stderr)]
    finales = [float(x) for x in re.findall(r"silence_end: ([\d.]+)", r.stderr)]
    ini, fin = 0.0, total
    if inicios and inicios[0] <= 0.05 and finales:
        ini = finales[0]
    if inicios and (len(finales) < len(inicios) or finales[-1] >= total - 0.05):
        fin = inicios[-1]
    n = correr([FFMPEG, "-hide_banner", "-nostdin", "-i", ruta, "-vn", "-af", "ebur128=peak=true", "-f", "null", "-"])
    lufs = re.findall(r"I:\s+(-?[\d.]+) LUFS", n.stderr)
    pico = re.findall(r"Peak:\s+(-?[\d.]+|-inf) dBFS", n.stderr)
    return {"voz_desde": round(ini, 2), "voz_hasta": round(fin, 2), "silencio_inicio": round(ini, 2),
            "silencio_final": round(total - fin, 2), "voz_s": round(max(0.0, fin - ini), 2),
            "lufs": float(lufs[-1]) if lufs else None,
            "pico_dbtp": float(pico[-1]) if pico and pico[-1] != "-inf" else None}


# ----------------------------------------------------------------------------- transcripción

_modelo = None


def transcribir(ruta: Path) -> list[dict]:
    """Las palabras con su tiempo: [{"palabra", "inicio", "fin"}]."""
    global _modelo
    if _modelo is None:
        from faster_whisper import WhisperModel
        _modelo = WhisperModel(MODELO, device="cpu", compute_type="int8", cpu_threads=os.cpu_count() or 4)
    # El audio lo decodifica ffmpeg (16 kHz, mono, float32), no PyAV: así no depende de su versión.
    import numpy as np
    crudo = subprocess.run([FFMPEG, "-nostdin", "-v", "error", "-i", str(ruta), "-vn", "-ac", "1", "-ar", "16000",
                            "-f", "f32le", "-"], capture_output=True, check=True).stdout
    audio = np.frombuffer(crudo, dtype=np.float32)
    segmentos, _ = _modelo.transcribe(audio, language="es", word_timestamps=True, vad_filter=True,
                                      beam_size=5, initial_prompt=VOCABULARIO)
    palabras = []
    for s in segmentos:
        for w in s.words or []:
            palabras.append({"palabra": w.word.strip(), "inicio": round(w.start, 2), "fin": round(w.end, 2)})
    return palabras


def fichas_con_tiempo(palabras: list[dict]) -> tuple[list[str], list[float]]:
    # Whisper parte «1,3» en «1» y «.3» (o «1,» y «3») y «2.883.000» en «2», «.883» y «.000»: se juntan
    # antes de normalizar, que distingue los miles del decimal.
    unidas: list[dict] = []
    for p in palabras:
        w = p["palabra"].strip()
        if unidas and ((re.search(r"\d[,.]$", unidas[-1]["palabra"]) and re.match(r"\d", w))
                       or (re.search(r"\d$", unidas[-1]["palabra"]) and re.match(r"[,.]\d", w))):
            unidas[-1] = {**unidas[-1], "palabra": unidas[-1]["palabra"] + w}
        else:
            unidas.append({**p, "palabra": w})
    toks, tiempos = [], []
    for p in unidas:
        for t in normalizar(p["palabra"]).split():
            toks.append(t)
            tiempos.append(p["inicio"])
    return pulir(toks, tiempos)


def comparar(texto: str, palabras: list[dict]) -> dict:
    """Las cifras del guion contra las que se oyen, y las palabras que difieren (con su segundo)."""
    g = fichas(texto)
    o, tiempos = fichas_con_tiempo(palabras)
    sm = difflib.SequenceMatcher(a=g, b=o, autojunk=False)
    # A qué ficha de lo oído corresponde cada ficha del guion (la más cercana alineada).
    mapa = {}
    for a, b, n in sm.get_matching_blocks():
        for k in range(n):
            mapa[a + k] = b + k

    def oido_cerca(i: int) -> int | None:
        for d in range(0, 12):
            for j in (i - d, i + d):
                if j in mapa:
                    return mapa[j]
        return None

    esperadas = cifras(g)
    oidas = cifras(o)
    usadas = set()
    observaciones = []
    for valor, a, b in esperadas:
        hallada = next((k for k, (v, _, _) in enumerate(oidas) if k not in usadas and abs(v - valor) < 1e-6), None)
        if hallada is not None:
            usadas.add(hallada)
            continue
        j = oido_cerca(a)
        cerca = [(v, x) for k, (v, x, _) in enumerate(oidas) if k not in usadas and j is not None and abs(x - j) <= 6]
        seg = tiempos[j] if j is not None and j < len(tiempos) else None
        frase_g = " ".join(g[max(0, a - 3): b + 4])
        frase_o = " ".join(o[max(0, (j or 0) - 3): (j or 0) + 5]) if j is not None else ""
        if cerca:
            v, x = min(cerca, key=lambda c: abs(c[1] - j))
            usadas.add(next(k for k, (vv, xx, _) in enumerate(oidas) if vv == v and xx == x))
            observaciones.append({"tipo": "cifra distinta", "esperada": fmt(valor), "oida": fmt(v),
                                  "segundo": seg, "guion": frase_g, "se_oye": frase_o})
        else:
            observaciones.append({"tipo": "cifra que no se oye", "esperada": fmt(valor), "oida": None,
                                  "segundo": seg, "guion": frase_g, "se_oye": frase_o})
    for k, (v, x, _) in enumerate(oidas):
        if k not in usadas:
            observaciones.append({"tipo": "cifra de más", "esperada": None, "oida": fmt(v),
                                  "segundo": tiempos[x] if x < len(tiempos) else None,
                                  "guion": "", "se_oye": " ".join(o[max(0, x - 3): x + 4])})
    difs = []
    for op, a1, a2, b1, b2 in sm.get_opcodes():
        if op == "equal":
            continue
        difs.append({"guion": " ".join(g[a1:a2]), "se_oye": " ".join(o[b1:b2]),
                     "segundo": tiempos[b1] if b1 < len(tiempos) else (tiempos[-1] if tiempos else None)})
    return {"parecido": round(sm.ratio(), 3), "cifras": observaciones, "palabras": difs,
            "transcripcion": " ".join(p["palabra"] for p in palabras)}


# ----------------------------------------------------------------------------- clips


def buscar_clips(carpeta: Path, bloques: list[Bloque]) -> tuple[dict, list[Path]]:
    archivos = [p for p in sorted(carpeta.iterdir()) if p.is_file() and p.suffix.lower() in EXT] if carpeta.is_dir() else []
    elegidos, sobran = {}, []
    for b in bloques:
        base = f"{b.numero:02d}-{b.quien}"
        cand = [p for p in archivos if slug(p.stem).startswith(base)]
        exactos = [p for p in cand if slug(p.stem) == base]
        if cand:
            elegidos[b.numero] = (exactos or sorted(cand, key=lambda p: p.stat().st_mtime))[-1]
    usados = set(elegidos.values())
    sobran = [p for p in archivos if p not in usados]
    return elegidos, sobran


def revisar_clip(b: Bloque, ruta: Path) -> dict:
    info = sondear(ruta)
    r = {"bloque": b.numero, "quien": b.quien, "titulo": b.titulo, "archivo": ruta.name, "objetivo_s": b.objetivo,
         "duracion_archivo_s": round(info["duracion"], 2), "observaciones": [], "avisos": []}
    obs, avisos = r["observaciones"], r["avisos"]
    if not info["audio"]:
        obs.append("no tiene audio")
        return r
    if not info["video"]:
        obs.append("solo audio: no hay cámara (el montaje pone un círculo con las iniciales)")
    else:
        r.update(ancho=info["ancho"], alto=info["alto"])
        if info["alto"] > info["ancho"]:
            obs.append(f"vertical ({info['ancho']}×{info['alto']}): se pidió horizontal")
        if min(info["ancho"], info["alto"]) < 720:
            obs.append(f"resolución {info['ancho']}×{info['alto']}: menos de 720p")
    r.update(voz_y_nivel(ruta, info["duracion"]))
    exceso = r["voz_s"] - b.objetivo
    if exceso > TOLERANCIA_S:
        obs.append(f"la voz dura {r['voz_s']:.1f} s y el guion pide {b.objetivo:.0f}: se pasa por {exceso:.1f} s")
    elif exceso < -12:
        avisos.append(f"la voz dura {r['voz_s']:.1f} s, {abs(exceso):.0f} s menos que el guion: ¿falta una parte?")
    if r["pico_dbtp"] is not None and r["pico_dbtp"] >= -0.5:
        obs.append(f"audio saturado: pico de {r['pico_dbtp']:.1f} dBTP")
    if r["lufs"] is not None and r["lufs"] < -35:
        obs.append(f"audio muy bajo: {r['lufs']:.1f} LUFS (el montaje lo sube, pero el ruido sube con él)")
    elif r["lufs"] is not None and r["lufs"] < -28:
        avisos.append(f"audio bajo: {r['lufs']:.1f} LUFS")
    if r["silencio_inicio"] > 4 or r["silencio_final"] > 4:
        avisos.append(f"silencios largos: {r['silencio_inicio']:.1f} s al inicio y {r['silencio_final']:.1f} s al final "
                      "(el montaje los recorta)")
    c = comparar(b.texto, transcribir(ruta))
    r["comparacion"] = c
    for x in c["cifras"]:
        s = f" (s {x['segundo']:.1f})" if x["segundo"] is not None else ""
        if x["tipo"] == "cifra distinta":
            obs.append(f"cifra: el guion dice {x['esperada']} y se oye {x['oida']}{s}: «{x['se_oye']}»")
        elif x["tipo"] == "cifra que no se oye":
            obs.append(f"cifra: no se oye {x['esperada']}{s}; guion: «{x['guion']}»; se oye: «{x['se_oye']}»")
        else:
            avisos.append(f"se oye una cifra que el guion no tiene: {x['oida']}{s}: «{x['se_oye']}»")
    if c["parecido"] < 0.75:
        avisos.append(f"el texto se aparta del guion (parecido {c['parecido']:.0%}); ver las diferencias")
    return r


def informe_clips(bloques: list[Bloque], resultados: list[dict], faltan: list[Bloque], sobran: list[Path]) -> str:
    l = ["# Revisión de los clips del video del TB2", "",
         f"Modelo de transcripción: faster-whisper «{MODELO}» (CPU, int8). Las cifras se comparan con las del guion; "
         "las palabras pueden decirse distinto (es informativo). Una transcripción automática se equivoca: "
         "antes de pedir que se regrabe algo, escuchar el segundo indicado.", ""]
    l += ["| Bloque | Archivo | Voz | Guion | Imagen | Nivel | Cifras | Estado |", "|:--|:--|--:|--:|:--|:--|:--|:--|"]
    for r in resultados:
        img = f"{r.get('ancho', '—')}×{r.get('alto', '—')}" if "ancho" in r else "solo audio"
        nivel = f"{r.get('lufs', '—')} LUFS, {r.get('pico_dbtp', '—')} dBTP" if r.get("lufs") is not None else "—"
        n_cifras = sum(1 for x in r.get("comparacion", {}).get("cifras", []) if x["tipo"] != "cifra de más")
        estado = "✗ ver observaciones" if r["observaciones"] else ("⚠ avisos" if r["avisos"] else "✓")
        l.append(f"| {r['bloque']:02d}-{r['quien']} | `{r['archivo']}` | {r.get('voz_s', 0):.1f} s | {r['objetivo_s']:.0f} s "
                 f"| {img} | {nivel} | {'✓' if n_cifras == 0 else f'{n_cifras} distintas'} | {estado} |")
    if faltan:
        l += ["", "**Faltan**: " + ", ".join(f"`{b.numero:02d}-{b.quien}`" for b in faltan) + "."]
    if sobran:
        l += ["", "**Archivos que no corresponden a ningún bloque** (el montaje no los usa): "
              + ", ".join(f"`{p.name}`" for p in sobran) + "."]
    for r in resultados:
        l += ["", f"## {r['bloque']:02d}-{r['quien']} · {r['titulo']}", ""]
        for o in r["observaciones"]:
            l.append(f"- **Observación:** {o}")
        for a in r["avisos"]:
            l.append(f"- Aviso: {a}")
        if not r["observaciones"] and not r["avisos"]:
            l.append("- Sin observaciones.")
        c = r.get("comparacion")
        if c:
            l += ["", f"Parecido con el guion: {c['parecido']:.0%}. Diferencias de palabras (guion → se oye):", ""]
            for d in c["palabras"][:25]:
                s = f"s {d['segundo']:.1f}" if d["segundo"] is not None else ""
                l.append(f"- {s}: «{d['guion'] or '∅'}» → «{d['se_oye'] or '∅'}»")
            if len(c["palabras"]) > 25:
                l.append(f"- … y {len(c['palabras']) - 25} más (en revision-clips.json)")
            l += ["", f"Transcripción: {c['transcripcion']}"]
    return "\n".join(l) + "\n"


# ----------------------------------------------------------------------------- video montado


def transcribir_video(video: Path, bloques_json: Path, bloques: list[Bloque]) -> tuple[str, dict]:
    resumen = json.loads(bloques_json.read_text(encoding="utf-8"))
    palabras = transcribir(video)
    cortes, t = [], 0.0
    for r in resumen:
        cortes.append((r["bloque"], t, t + r["segundos"]))
        t += r["segundos"]
    por_bloque = {}
    l = ["# Transcripción del video montado", "",
         f"`{video.name}`, {t:.1f} s. Transcripción automática (faster-whisper «{MODELO}»), cortada por bloque "
         "con los tiempos de bloques.json. Al lado, lo que dice el guion.", ""]
    guion = {b.numero: b for b in bloques}
    for num, a, b in cortes:
        dentro = [p for p in palabras if a <= p["inicio"] < b]
        texto = " ".join(p["palabra"] for p in dentro)
        c = comparar(guion[num].texto, dentro) if num in guion else None
        por_bloque[num] = {"desde": round(a, 2), "hasta": round(b, 2), "texto": texto, "comparacion": c}
        m = lambda s: f"{int(s // 60)}:{s % 60:04.1f}"
        l += [f"## Bloque {num:02d} · {guion[num].titulo if num in guion else ''} ({m(a)}–{m(b)}, {guion[num].quien if num in guion else ''})",
              "", f"**Se oye:** {texto}", "", f"**Guion:** {guion[num].texto if num in guion else ''}", ""]
        if c:
            malas = [x for x in c["cifras"] if x["tipo"] != "cifra de más"]
            l.append(f"Parecido {c['parecido']:.0%}; cifras distintas o que no se oyen: "
                     + ("ninguna" if not malas else "; ".join(f"{x['esperada']} → {x['oida'] or 'no se oye'}" for x in malas)))
            l.append("")
    return "\n".join(l) + "\n", {"video": video.name, "segundos": round(t, 2), "bloques": por_bloque}


def vista_docente(js: dict) -> str:
    """Lo que oye el docente, sin el guion ni las comparaciones: para el corrector en frío, junto con los
    fotogramas de build/tb2/fotogramas/ (uno cada 20 s)."""
    m = lambda s: f"{int(s // 60)}:{s % 60:04.1f}"
    l = [f"# Lo que se oye en el video (transcripción automática del mp4, {m(js['segundos'])})", "",
         "Un fotograma cada 20 s en fotogramas/ (s-001 = 0:00, s-002 = 0:20, …).", ""]
    for _, v in sorted(js["bloques"].items(), key=lambda x: int(x[0])):
        l += [f"## {m(v['desde'])}–{m(v['hasta'])}", "", v["texto"], ""]
    return "\n".join(l)


# ----------------------------------------------------------------------------- principal


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("--guion", type=Path, required=True)
    ap.add_argument("--clips", type=Path)
    ap.add_argument("--video", type=Path)
    ap.add_argument("--bloques", type=Path)
    ap.add_argument("--salida", type=Path, required=True)
    a = ap.parse_args()
    a.salida.mkdir(parents=True, exist_ok=True)
    bloques = leer_guion(a.guion)
    if a.clips:
        elegidos, sobran = buscar_clips(a.clips, bloques)
        resultados = []
        for b in bloques:
            if b.numero in elegidos:
                print(f"revisando {elegidos[b.numero].name}…", flush=True)
                resultados.append(revisar_clip(b, elegidos[b.numero]))
        faltan = [b for b in bloques if b.numero not in elegidos]
        (a.salida / "revision-clips.md").write_text(informe_clips(bloques, resultados, faltan, sobran), encoding="utf-8")
        (a.salida / "revision-clips.json").write_text(json.dumps(
            {"modelo": MODELO, "clips": resultados, "faltan": [f"{b.numero:02d}-{b.quien}" for b in faltan],
             "sobran": [p.name for p in sobran]}, ensure_ascii=False, indent=2), encoding="utf-8")
        abiertas = sum(len(r["observaciones"]) for r in resultados)
        print(f"{len(resultados)} clips revisados, {len(faltan)} faltan, {abiertas} observaciones")
    if a.video:
        if not a.bloques:
            sys.exit("--video necesita --bloques (bloques.json del montaje)")
        md, js = transcribir_video(a.video, a.bloques, bloques)
        (a.salida / "transcripcion-video.md").write_text(md, encoding="utf-8")
        (a.salida / "transcripcion-docente.md").write_text(vista_docente(js), encoding="utf-8")
        (a.salida / "transcripcion-video.json").write_text(json.dumps(js, ensure_ascii=False, indent=2), encoding="utf-8")
        print(f"transcripción del video: {js['segundos']} s")


if __name__ == "__main__":
    main()
