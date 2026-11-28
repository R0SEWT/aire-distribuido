"""Datos de los gráficos del deck: lee reports/*.json y escribe datos.js y _deck.yml.

    python3 datos.py        # desde docs/presentacion/slides/ (solo biblioteca estándar)

Las cifras del texto salen, sobre todo, de docs/informe/generado/cifras.yml con el shortcode meta,
igual que el informe. Lo que el informe todavía no cita y el deck del TB2 sí (el impacto de
reports/impacto.json, las pruebas del frontend) lo escribe este script en _deck.yml, con el mismo
formato que tablas.py (espacio duro en los millares, coma decimal). Además prepara las series que
dibuja D3 (js/graficos.js), sin redondear ni escribir ningún número a mano, para que el deck no
pueda contradecir al informe. datos.js y _deck.yml se versionan: el deck se renderiza sin correr Go.
"""

from __future__ import annotations

import json
import re
from pathlib import Path

AQUI = Path(__file__).resolve().parent
RAIZ = AQUI.parents[2]  # docs/presentacion/slides → raíz del repo
REPORTS = RAIZ / "reports"

# Motivos de descarte del cargador, en palabras de la exposición. Los que no están aquí van a «otras».
MOTIVOS = {
    "bloque_en_cero": "día entero en 0 (no medido)",
    "sin_valor": "fila sin valor",
    "periodo": "promedio diario o móvil",
    "duplicado_unidad": "gas repetido en otra unidad",
    "fuera_de_rango": "fuera del rango físico",
}


def leer(nombre: str) -> dict:
    return json.loads((REPORTS / nombre).read_text(encoding="utf-8"))


# Formato de las cifras del texto, el mismo de docs/informe/tablas.py.
def entero(x) -> str:
    return f"{int(round(x)):,}".replace(",", "\u00a0")


def decimal(x, d=1) -> str:
    return f"{x:,.{d}f}".replace(",", "\u00a0").replace(".", ",")


def pct(x, d=0) -> str:
    return f"{decimal(100 * x, d)}\u00a0%"


def horas(x) -> str:
    """Una mediana o un cuartil de horas: entero si lo es, con una decimal si no."""
    return entero(x) if float(x).is_integer() else decimal(x, 1)


def embudo(limpieza: dict) -> dict:
    fl = limpieza["filas_largas"]
    por_regla = fl["descartadas_por_regla"]
    motivos = [{"motivo": MOTIVOS[k], "filas": por_regla.get(k, 0)} for k in MOTIVOS]
    otras = sum(v for k, v in por_regla.items() if k not in MOTIVOS)
    motivos.sort(key=lambda m: -m["filas"])
    motivos.append({"motivo": "otras reglas", "filas": otras})
    assert sum(m["filas"] for m in motivos) == fl["descartadas"], "los motivos no suman las descartadas"
    assert fl["procesadas"] + fl["descartadas"] == fl["leidas"], "procesadas + descartadas ≠ leídas"
    estaciones = limpieza["estaciones"]
    return {
        "leidas": fl["leidas"],
        "procesadas": fl["procesadas"],
        "descartadas": fl["descartadas"],
        "motivos": motivos,
        "hora_inferida": fl["hora_inferida"],
        "anchas": limpieza["filas_anchas"]["total"] if isinstance(limpieza["filas_anchas"], dict)
        else limpieza["filas_anchas"],
        "completas": sum(e.get("completas_gold", 0) for e in estaciones.values()),
        "estaciones": len(estaciones),
        "estaciones_gold": sum(1 for e in estaciones.values() if e.get("completas_gold", 0) > 0),
    }


def speedup_cargador(bc: dict) -> list[dict]:
    return [{"w": m["workers"], "punto": m["speedup_ic"]["punto"], "inf": m["speedup_ic"]["inferior"],
             "sup": m["speedup_ic"]["superior"]} for m in sorted(bc["mediciones"], key=lambda m: m["workers"])]


def speedup_kmeans(bk: dict, chunk: int, experimento: str = "convergencia") -> list[dict]:
    filas = [r for r in bk["resumen"] if r.get("experimento") == experimento and r.get("modo") == "conc"
             and r.get("chunk") == chunk]
    return [{"w": r["workers"], "punto": r["speedup"]["punto"], "inf": r["speedup"]["inferior"],
             "sup": r["speedup"]["superior"]} for r in sorted(filas, key=lambda r: r["workers"])]


def niveles(reg: dict, limpieza: dict) -> dict:
    # El nombre del nivel sale del PM2,5 medio de cada régimen, como en el informe (tablas.py).
    orden = sorted(reg["regimenes"], key=lambda r: r["pm2_5_medio"])
    nombres = dict(zip((r["regimen"] for r in orden), ("bajo", "medio", "alto")))
    estaciones = limpieza["estaciones"]

    def ubicacion(e: str) -> dict:
        u = estaciones.get(e, {}).get("ubicacion", {})
        return {"departamento": (u.get("departamento") or "").title(), "altitud": u.get("altitud")}

    filas = []
    for r in orden:
        total = r["tamano"]
        top = sorted(r["por_estacion"].items(), key=lambda t: -t[1])[:5]
        filas.append({
            "nivel": nombres[r["regimen"]],
            "regimen": r["regimen"],
            "horas": total,
            "fraccion": r["fraccion"],
            "pm2_5": r["pm2_5_medio"],
            "pm10": r["pm10_medio"],
            "eca": r["eca"],
            "por_hora": r["por_hora"],
            "estaciones": [{"estacion": e, "horas": n, "fraccion": n / total, **ubicacion(e)} for e, n in top],
        })
    return {"niveles": filas, "eca": {"pm2_5": reg["eca"]["pm2_5_ugm3"], "pm10": reg["eca"]["pm10_ugm3"]},
            "convencion_hora": reg.get("convencion_hora")}


def impacto(imp: dict) -> tuple[dict, dict]:
    """Lo que el deck cuenta del impacto: las cifras del texto (para _deck.yml) y las series de D3.

    La población es la censada del distrito de cada estación y se muestra como contexto de su fila,
    nunca sumada entre distritos (reports/impacto.json, metadatos.notas)."""
    al, tot, an = imp["alcance"], imp["totales"], imp["anticipacion"]
    lh, rg = an["total"]["lectura_horaria"], an["total"]["regimen"]
    con_episodios = [e for e in an["por_estacion"] if e["episodios"] > 0]
    assert sum(e["episodios"] for e in con_episodios) == an["total"]["episodios"], "los episodios no suman"
    # Por actividad fiscalizada: la unidad honesta, porque cada estación vigila una actividad.
    tipos = []
    for t in imp["tipos_actividad"]:
        distritos = t["distritos"]
        tipos.append({
            "tipo": t["tipo_actividad"],
            "estaciones": len(t["estaciones"]),
            "calculables": t["horas_calculables"],
            "sobre_eca": t["horas_sobre_eca"],
            "fraccion": t["horas_sobre_eca"] / t["horas_calculables"],
            "distritos": [f'{d["distrito"].title()} ({d["departamento"].title()})' for d in distritos],
            # Solo cuando el grupo está en un único distrito: sumar distritos no es una cifra honesta.
            "poblacion": distritos[0]["poblacion"] if len(distritos) == 1 else None,
        })
    tipos.sort(key=lambda t: -t["fraccion"])
    assert sum(t["sobre_eca"] for t in tipos) == tot["horas_sobre_eca"], "los tipos no suman las horas sobre el ECA"
    # El distrito con más horas sobre el ECA (Lurigancho, en Lima: ladrilleras y una refinería).
    peor = max(imp["distritos"], key=lambda d: d["horas_sobre_eca"])
    cuidado = sum(e["inca"]["umbral_de_cuidado"] for e in imp["estaciones"])
    texto = {
        "estaciones_ficha": entero(al["estaciones_en_la_ficha"]),
        "estaciones_pm": entero(al["estaciones_con_pm_horario"]),
        "estaciones_sin_pm": entero(len(al["estaciones_sin_pm_horario"])),
        "distritos_pm": entero(al["distritos_con_pm_horario"]),
        "lecturas": entero(tot["lecturas"]),
        "calculables": entero(tot["horas_calculables"]),
        "sobre_eca": entero(tot["horas_sobre_eca"]),
        "pct_sobre_eca": pct(tot["horas_sobre_eca"] / tot["horas_calculables"], 1),
        "episodios": entero(an["total"]["episodios"]),
        "estaciones_episodios": entero(len(con_episodios)),
        "con_media_previa": entero(an["total"]["con_media_previa"]),
        "sin_senal_previa": entero(an["total"]["sin_senal_previa"]),
        "tope": entero(an["definicion"]["tope_horas"]),
        "lh_mediana": horas(lh["horas_antes"]["mediana"]),
        # Los cuartiles con una decimal, como en el informe (tablas.py del TB2).
        "lh_q1": decimal(lh["horas_antes"]["q1"], 1),
        "lh_q3": decimal(lh["horas_antes"]["q3"], 1),
        "lh_seguidas": horas(lh["horas_seguidas"]["mediana"]),
        "lh_en_el_tope": entero(lh["en_el_tope"]),
        "rg_mediana": horas(rg["horas_antes"]["mediana"]),
        "rg_q1": decimal(rg["horas_antes"]["q1"], 1),
        "rg_q3": decimal(rg["horas_antes"]["q3"], 1),
        "rg_seguidas": horas(rg["horas_seguidas"]["mediana"]),
        "peor_distrito": peor["distrito"].title(),
        "peor_departamento": peor["departamento"].title(),
        "peor_poblacion": entero(peor["poblacion"]),
        "peor_anio": str(peor["anio"]),
        "peor_estaciones": entero(len(peor["estaciones"])),
        "peor_sobre_eca": entero(peor["horas_sobre_eca"]),
        "peor_calculables": entero(peor["horas_calculables"]),
        "peor_pct": pct(peor["horas_sobre_eca"] / tot["horas_sobre_eca"]),
        "peor_desde": peor["primera_fecha"],
        "peor_hasta": peor["ultima_fecha"],
        "umbral_cuidado": entero(cuidado),
    }
    return texto, {"tipos": tipos}


def latencia(lat: dict) -> dict:
    """Los percentiles por nivel de concurrencia, del lado del cliente (reports/api_latencia.json)."""
    return {
        "limite_ms": lat["parametros"]["limite_ms"],
        "niveles": [{"clientes": n["clientes"], "p50": n["p50_ms"], "p99": n["p99_ms"], "max": n["max_ms"],
                     "cumple": n["cumple"]} for n in lat["niveles"]],
    }


PRUEBAS_TB2 = RAIZ / "docs" / "informe" / "capturas" / "tb2" / "pruebas"


def pruebas_web() -> tuple[int, int]:
    """Pruebas y archivos de prueba del frontend, de la corrida de Vitest que guardó la sesión de
    capturas del TB2 (pnpm test en web-pruebas.txt). Tienen que haber pasado todas."""
    texto = (PRUEBAS_TB2 / "web-pruebas.txt").read_text(encoding="utf-8")
    cuenta = {}
    for clave, patron in (("archivos", r"Test Files\s+(\d+) passed \((\d+)\)"), ("pruebas", r"Tests\s+(\d+) passed \((\d+)\)")):
        m = re.search(patron, texto)
        assert m and m.group(1) == m.group(2), f"web-pruebas.txt: {clave} que no pasaron o no están"
        cuenta[clave] = int(m.group(1))
    return cuenta["pruebas"], cuenta["archivos"]


def tiempo_real() -> dict:
    """De lo que manda una sesión a lo que ve otra por el WebSocket (pruebas/ws_latencia.json)."""
    ws = json.loads((PRUEBAS_TB2 / "ws_latencia.json").read_text(encoding="utf-8"))
    assert ws["cumple"] and not ws["metadatos"]["mismo_token"], "ws_latencia.json no cumple o es una sola sesión"
    ev = ws["evento_ms"]
    return {"lecturas": entero(ws["lecturas"]), "p50": decimal(ev["p50"], 2), "p99": decimal(ev["p99"], 2),
            "sin_evento": entero(ws["sin_evento"])}


def pruebas() -> int:
    """Funciones Test* de los archivos versionados: lo que corre go test -race ./... (sin las copias
    de las carpetas ocultas, como los worktrees de .claude/)."""
    patron = re.compile(r"^func Test\w*\(t \*testing\.T\)", re.M)
    return sum(len(patron.findall(p.read_text(encoding="utf-8"))) for p in RAIZ.rglob("*_test.go")
               if not any(parte.startswith(".") for parte in p.relative_to(RAIZ).parts))


def portada(limpieza: dict) -> str:
    """Fondo de la portada: las estaciones en su posición real (lon escalada por cos(lat)), con
    colores fijos porque una imagen de fondo no hereda el CSS. Llenas, las que llegan al gold."""
    import math

    puntos = []
    for e, x in limpieza["estaciones"].items():
        u = x.get("ubicacion") or {}
        try:
            lat, lon = float(u["lat"]), float(u["lon"])
        except (KeyError, TypeError, ValueError):
            continue
        puntos.append((lat, lon, x.get("anchas", 0), x.get("completas_gold", 0) > 0))
    # Marco fijo del Perú, para que la forma se reconozca aunque falten estaciones en el oriente
    lat_n, lat_s, lon_o, lon_e = 0.2, -18.6, -81.5, -68.5
    k = math.cos(math.radians((lat_n + lat_s) / 2))
    ancho, alto = 420.0, 600.0
    esc = min(ancho / ((lon_e - lon_o) * k), alto / (lat_n - lat_s))
    max_anchas = max(p[2] for p in puntos) or 1
    circulos = []
    for lat, lon, anchas, gold in sorted(puntos, key=lambda p: -p[2]):
        cx, cy = (lon - lon_o) * k * esc, (lat_n - lat) * esc
        r = 5 + 22 * math.sqrt(anchas / max_anchas)
        estilo = ('fill="#86B6EF" fill-opacity="0.85" stroke="#FFFFFF" stroke-opacity="0.5" stroke-width="1.5"'
                  if gold else 'fill="none" stroke="#7F93AA" stroke-width="2" stroke-dasharray="4 3"')
        circulos.append(f'<circle cx="{cx:.1f}" cy="{cy:.1f}" r="{r:.1f}" {estilo}/>')
    return (f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="-30 -30 {ancho + 60:.0f} {alto + 60:.0f}">'
            + "".join(circulos) + "</svg>\n")


def main() -> None:
    limpieza = leer("limpieza.json")
    bk = leer("benchmark_kmeans.json")
    datos = {
        "embudo": embudo(limpieza),
        "speedup": {
            "cargador": speedup_cargador(leer("benchmark_cargador.json")),
            "kmeans_1000": speedup_kmeans(bk, 1000),
            "kmeans_10000": speedup_kmeans(bk, 10000),
            "chunks_1000": -(-bk["datos"]["n"] // 1000),
            "chunks_10000": -(-bk["datos"]["n"] // 10000),
        },
        **niveles(leer("regimenes.json"), limpieza),
        "pruebas": pruebas(),
        "latencia": latencia(leer("api_latencia.json")),
    }
    texto_impacto, series_impacto = impacto(leer("impacto.json"))
    datos["impacto"] = series_impacto
    web, archivos_web = pruebas_web()
    ws = tiempo_real()
    js = ("// Generado por datos.py desde reports/*.json. No editar a mano.\n"
          "window.DATOS = " + json.dumps(datos, ensure_ascii=False, indent=1) + ";\n")
    (AQUI / "datos.js").write_text(js, encoding="utf-8")
    # Lo que el texto del deck cita y no está en cifras.yml: se lee con {{< meta deck.x >}}
    lineas = [f'  pruebas: "{entero(datos["pruebas"])}"', f'  pruebas_web: "{entero(web)}"',
              f'  archivos_web: "{entero(archivos_web)}"', "  ws:"]
    lineas += [f'    {k}: "{v}"' for k, v in ws.items()]
    lineas.append("  impacto:")
    lineas += [f'    {k}: "{v}"' for k, v in texto_impacto.items()]
    (AQUI / "_deck.yml").write_text(
        "# Generado por datos.py desde reports/. No editar a mano.\n"
        "# Uso en el .qmd: shortcode meta con la clave deck.… (deck.impacto.episodios)\n"
        "deck:\n" + "\n".join(lineas) + "\n", encoding="utf-8")
    (AQUI / "img" / "estaciones-portada.svg").write_text(portada(limpieza), encoding="utf-8")
    print(f"datos.js, _deck.yml e img/estaciones-portada.svg: {len(datos['niveles'])} niveles, "
          f"{datos['pruebas']} pruebas de Go, {web} del frontend, WebSocket p50 {ws['p50']} ms, "
          f"{len(series_impacto['tipos'])} tipos de actividad")


if __name__ == "__main__":
    main()
