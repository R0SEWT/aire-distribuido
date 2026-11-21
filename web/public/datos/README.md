# Datos estáticos del frontend

La SPA sirve estos archivos tal cual, desde `/datos/`. Ninguno se edita a mano: cada uno lo escribe un
script de `scripts/`, y `scripts/test_impacto.py` comprueba que sigan cuadrando entre sí y con `reports/`.

| Archivo | De dónde sale |
|---|---|
| `impacto.json` | Copia idéntica de `reports/impacto.json`. Lo escribe `scripts/impacto.py` con las lecturas del gold (PM crudo del silver de OEFA), sus regímenes (`data/gold/asignaciones.csv`), `reports/estaciones_ficha.csv` y `reports/poblacion_distritos.csv` (población censada, INEI, Censos Nacionales 2017). Sus totales son los de `reports/alerta_24h.json`. |
| `series.json` | Lo escribe `scripts/impacto.py`: lecturas reales de OEFA (PM2.5 y PM10 crudos de `data/silver/oefa_ancho.csv`) para el escenario y los días sueltos de la simulación, con lo que la API tiene que responder al final, calculado con la regla de `ventana/ventana.go`. Comprobado contra la API real con `scripts/verificar_series_api.py`. |
| `peru.geojson` | Lo escribe `scripts/mapa_peru.py` a partir de geoBoundaries (ver abajo). |
| `README.md` | Este archivo, escrito a mano. |

Para regenerarlos, desde la raíz del repositorio y con el silver y el gold en `data/` (no se versionan):

```bash
python3 scripts/impacto.py                 # impacto.json (y reports/impacto.json) y series.json
python3 scripts/impacto.py --verificar     # recalcula y compara, sin escribir
python3 -m unittest discover -s scripts -p 'test_impacto.py'
```

Los datos de OEFA son los de *Vigilancia y Seguimiento ambiental en la calidad del aire*, del Portal de
Datos Abiertos del Perú (`data/README.md`). El SHA-256 del gold y del silver con que se generó cada
archivo está en su `metadatos.fuentes`.

## `peru.geojson`

- **Qué es**: los límites de primer nivel del Perú, 26 polígonos: los 24 departamentos, el Callao y Lima
  Metropolitana, que la fuente trae aparte del departamento de Lima.
- **Fuente**: geoBoundaries, colección gbOpen, `PER` `ADM1`, `boundaryID` `PER-ADM1-1875124`
  (<https://www.geoboundaries.org/api/current/gbOpen/PER/ADM1/>). El archivo de partida es la geometría
  simplificada que publica la propia fuente, `geoBoundaries-PER-ADM1_simplified.geojson`, del commit
  `90a1d52` de <https://github.com/wmgeolab/geoBoundaries>
  (SHA-256 `df5bbc9e4eabae9dcf9fe163180be85c1ef297bb1633e250b6f0c5bab5deebdf`).
  geoBoundaries declara como origen del trazo Wikimedia Commons, con 2008 como año representado y
  compilación del 21 de febrero de 2024.
- **Licencia**: geoBoundaries marca este límite como *Public Domain* (`boundaryLicense`) y distribuye su
  base bajo CC BY 4.0. Cita: Runfola, D. et al. (2020), *geoBoundaries: A global database of political
  administrative boundaries*, PLoS ONE 15(4): e0231866.
- **Fecha de descarga**: 21 de noviembre de 2026.
- **Qué se le hizo** (`scripts/mapa_peru.py`): coordenadas a 4 decimales (unos 11 m) y tres propiedades
  por polígono. Pesa unos 140 KB.
  - `departamento`: la clave con que `reports/estaciones_ficha.csv` nombra al departamento, en mayúsculas
    y sin tildes (`ANCASH`, `LA LIBERTAD`). Los dos polígonos de Lima llevan `LIMA`; el del Callao, `CALLAO`.
  - `nombre`: el de la fuente, tal cual (`Áncash` aparece como `Ancash`; Lima Metropolitana, como
    `Municipalidad Metropolitana de Lima`; el Callao, como `El Callao`).
  - `iso`: el código ISO 3166-2 (`PE-LMA` para Lima Metropolitana).
- **Sentido de los anillos**: el exterior va en sentido horario, que es lo que espera `d3-geo`. Es al
  revés de lo que pide el RFC 7946; Leaflet, QGIS y geojson.io aceptan los dos.
- **Comprobación**: las 33 estaciones de la ficha caen dentro de un polígono del departamento que dice su
  fila. `CA-HUAR-01`, que no trae departamento en la fuente, cae en Pasco. `CA-RZC-02` (Jicamarca) cae en
  el polígono del departamento de Lima, a medio kilómetro del de Lima Metropolitana: los dos son `LIMA`.
  El trazo es grueso; sirve para ubicar una estación en el país, no para decidir de qué distrito es.

```bash
python3 -I scripts/mapa_peru.py RUTA/geoBoundaries-PER-ADM1_simplified.geojson   # lo vuelve a escribir
python3 scripts/mapa_peru.py --verificar                                         # solo comprueba
```

El archivo descargado no se versiona.
