"""Copia los textos y las duraciones de guion-tb2.md al mensaje del grupo y a la guía de grabación.

    python3 docs/video/sincronizar_textos.py

El guion manda: los mensajes 2 y 3 de mensaje-grupo-tb2.md (los textos de Dayana y de Julio), las
duraciones del mensaje 1, la tabla de grabar-tu-parte.md y los totales por persona se reescriben desde él,
para que lo que lee cada integrante sea siempre lo que el montaje espera.
"""

from __future__ import annotations

import re
from pathlib import Path

VIDEO = Path(__file__).resolve().parent
NOMBRES = {"rody": "Rody", "dayana": "Dayana", "julio": "Julio"}


def segundos(d: str) -> int:
    m, s = d.split(":")
    return int(m) * 60 + int(s)


def mmss(s: int) -> str:
    return f"{s // 60}:{s % 60:02d}"


def leer_guion() -> list[tuple[int, str, str, str, str]]:
    g = (VIDEO / "guion-tb2.md").read_text(encoding="utf-8")
    partes = re.split(r"^## Bloque (\d+) · (.+)$", g, flags=re.M)
    bloques = []
    for i in range(1, len(partes), 3):
        n, titulo, cuerpo = int(partes[i]), partes[i + 1].strip(), partes[i + 2]
        quien = re.search(r"^- Quién: (\w+)", cuerpo, re.M).group(1).lower()
        dur = re.search(r"^- Duración: (\S+)", cuerpo, re.M).group(1)
        texto = re.search(r"```dice\n(.*?)```", cuerpo, re.S).group(1).strip()
        bloques.append((n, titulo, quien, dur, texto))
    return bloques


def main() -> None:
    bloques = leer_guion()
    por = {q: sum(segundos(b[3]) for b in bloques if b[2] == q) for q in NOMBRES}
    total = sum(por.values())

    # Guion: la tabla de reparto y la duración objetivo.
    p = VIDEO / "guion-tb2.md"
    t = p.read_text(encoding="utf-8")
    t = re.sub(r"\*\*Duración objetivo: [0-9:]+\*\*", f"**Duración objetivo: {mmss(total)}**", t)
    for q, nombre in NOMBRES.items():
        t = re.sub(rf"(\| \*\*{nombre}\*\* \| [^|]+\| )[0-9:]+( \|)", rf"\g<1>{mmss(por[q])}\2", t)
    p.write_text(t, encoding="utf-8")

    # Mensaje: textos de Dayana y de Julio, y las duraciones del mensaje 1.
    p = VIDEO / "mensaje-grupo-tb2.md"
    t = p.read_text(encoding="utf-8")
    for quien, encabezado in (("dayana", "## Mensaje 2 (Dayana)"), ("julio", "## Mensaje 3 (Julio)")):
        suyos = [b for b in bloques if b[2] == quien]
        lineas = [f"> *{NOMBRES[quien]}: tus {len(suyos)} textos*", ">"]
        for n, titulo, q, dur, texto in suyos:
            lineas.append(f"> *{n:02d}-{q} · {titulo} ({dur})*")
            lineas += ["> " + l for l in texto.splitlines()]
            lineas.append(">")
        i = t.index(encabezado) + len(encabezado)
        j = t.find("\n## ", i)
        resto = t[j:] if j != -1 else "\n"
        t = t[:i] + "\n\n" + "\n".join(lineas[:-1]) + "\n" + resto
    t = re.sub(r"El video total dura [0-9:]+", f"El video total dura {mmss(total)}", t)
    for n, _, q, dur, _ in bloques:
        t = re.sub(rf"`{n:02d}-{q}` \([0-9:]+", f"`{n:02d}-{q}` ({dur}", t)
    p.write_text(t, encoding="utf-8")

    # Guía de grabación: la tabla y los totales por persona.
    p = VIDEO / "grabar-tu-parte.md"
    t = p.read_text(encoding="utf-8")
    for n, _, q, dur, _ in bloques:
        t = re.sub(rf"(\| `{n:02d}-{q}\.mp4` \| [^|]+\| [^|]+\| )[0-9:]+( \|)", rf"\g<1>{dur}\2", t)
    t = re.sub(r"Por persona: .*?\.\n",
               f"Por persona: **Dayana**, 3 clips ({mmss(por['dayana'])} en total); **Julio**, 4 clips "
               f"({mmss(por['julio'])}); **Rody**, 5 clips ({mmss(por['rody'])}).\n", t, count=1)
    p.write_text(t, encoding="utf-8")

    # README: el total.
    p = VIDEO / "README.md"
    t = p.read_text(encoding="utf-8")
    t = re.sub(r"Doce bloques que suman \*\*[0-9:]+\*\*: Rody [0-9:]+, Dayana [0-9:]+ y\nJulio [0-9:]+\.",
               f"Doce bloques que suman **{mmss(total)}**: Rody {mmss(por['rody'])}, Dayana {mmss(por['dayana'])} y\n"
               f"Julio {mmss(por['julio'])}.", t)
    p.write_text(t, encoding="utf-8")
    print(f"total {mmss(total)}: " + ", ".join(f"{NOMBRES[q]} {mmss(s)}" for q, s in por.items()))


if __name__ == "__main__":
    main()
