"""Baja la población de WorldPop y los límites de geoBoundaries del Perú (spec §3.2).

    uv run python descargar_estaticos.py            # baja lo que falte a data/mapa/crudo/estaticos/
    uv run python descargar_estaticos.py --forzar   # vuelve a bajar todo

- WorldPop 2020, 1 km (30″), conteos sin restringir y ajustados al total de la ONU para el Perú
  (``per_ppp_2020_1km_Aggregated_UNadj.tif``), elegido por su API ``hub.worldpop.org/rest/data/pop``.
- geoBoundaries PER ADM0 y ADM3 por su API ``geoboundaries.org/api/current/<release>/PER/<nivel>/``.
  Se prueba primero gbOpen; si no tiene el nivel (el ADM3 del Perú no está en gbOpen) se usa
  gbHumanitarian.

Cada fuente queda en ``crudo/estaticos/fuentes_estaticos.json`` con la licencia que declara su API,
la cita, la URL, el sha256 y el tamaño. ``preparar.py`` lo copia al manifiesto.
"""

import argparse
import json
import sys
import urllib.error
import urllib.request
from pathlib import Path

from comun import DIR_MAPA, escribir_json, sha256_archivo

AGENTE = {"User-Agent": "mapa-datos/0.1"}
API_WORLDPOP = "https://hub.worldpop.org/rest/data/pop/wpicuadj1km?iso3=PER"
API_GEOBOUNDARIES = "https://www.geoboundaries.org/api/current/{release}/PER/{nivel}/"
RELEASES = ("gbOpen", "gbHumanitarian")

ARCHIVO_WORLDPOP = "per_ppp_2020_1km_Aggregated_UNadj.tif"
ARCHIVO_ADM = "geoBoundaries-PER-{nivel}.geojson"
ARCHIVO_FUENTES = "fuentes_estaticos.json"


def obtener_json(url: str):
    """El JSON de una URL, o None si responde 404."""
    try:
        with urllib.request.urlopen(urllib.request.Request(url, headers=AGENTE), timeout=60) as r:
            return json.load(r)
    except urllib.error.HTTPError as e:
        if e.code == 404:
            return None
        raise


def bajar(url: str, destino: Path) -> None:
    destino.parent.mkdir(parents=True, exist_ok=True)
    parcial = destino.with_name(destino.name + ".parcial")
    with urllib.request.urlopen(urllib.request.Request(url, headers=AGENTE), timeout=600) as r, open(parcial, "wb") as f:
        while bloque := r.read(1 << 20):
            f.write(bloque)
    parcial.replace(destino)


def metadatos_worldpop() -> dict:
    datos = obtener_json(API_WORLDPOP)
    entrada = next(e for e in datos["data"] if e.get("popyear") == "2020")
    tif = next(f for f in entrada["files"] if f.endswith(".tif"))
    with urllib.request.urlopen(urllib.request.Request(entrada["license"], headers=AGENTE), timeout=60) as r:
        texto = r.read().decode("utf-8", "replace")
    licencia = "CC-BY-4.0" if "Creative Commons Attribution 4.0" in texto else "ver licencia_url"
    return {
        "fuente": "WorldPop",
        "titulo": entrada["title"],
        "categoria": entrada["category"],
        "anio": 2020,
        "doi": entrada["doi"],
        "cita": entrada["citation"],
        "url": tif,
        "api": API_WORLDPOP,
        "licencia": licencia,
        "licencia_url": entrada["license"],
    }


def metadatos_geoboundaries(nivel: str) -> dict:
    for release in RELEASES:
        api = API_GEOBOUNDARIES.format(release=release, nivel=nivel)
        datos = obtener_json(api)
        if datos is not None:
            return {
                "fuente": f"geoBoundaries {release}",
                "nivel": nivel,
                "boundaryID": datos["boundaryID"],
                "anio_representado": datos["boundaryYearRepresented"],
                "unidades": int(datos["admUnitCount"]),
                "origen": datos["boundarySource"],
                "licencia": datos["boundaryLicense"],
                "licencia_fuente": datos["licenseSource"],
                "url": datos["gjDownloadURL"],
                "api": api,
                "cita": "Runfola, D. et al. (2020) geoBoundaries: A global database of political administrative "
                "boundaries. PLoS ONE 15(4): e0231866. https://doi.org/10.1371/journal.pone.0231866",
            }
    raise RuntimeError(f"geoBoundaries no tiene PER {nivel} en {RELEASES}")


def main(argv=None) -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--destino", type=Path, default=DIR_MAPA / "crudo" / "estaticos")
    ap.add_argument("--forzar", action="store_true", help="vuelve a bajar aunque el archivo ya esté")
    args = ap.parse_args(argv)

    fuentes = {
        ARCHIVO_WORLDPOP: metadatos_worldpop(),
        ARCHIVO_ADM.format(nivel="ADM0"): metadatos_geoboundaries("ADM0"),
        ARCHIVO_ADM.format(nivel="ADM3"): metadatos_geoboundaries("ADM3"),
    }
    for relativa, meta in fuentes.items():
        ruta = args.destino / relativa
        if args.forzar or not ruta.exists() or ruta.stat().st_size == 0:
            print(f"bajando {meta['url']}", flush=True)
            bajar(meta["url"], ruta)
        meta["archivo"] = relativa
        meta["sha256"] = sha256_archivo(ruta)
        meta["bytes"] = ruta.stat().st_size
        print(f"{relativa}: {meta['bytes'] / 1e6:.1f} MB, licencia {meta['licencia']} ({meta['fuente']})")
    escribir_json(args.destino / ARCHIVO_FUENTES, list(fuentes.values()))
    print(f"fuentes y licencias en {args.destino / ARCHIVO_FUENTES}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
