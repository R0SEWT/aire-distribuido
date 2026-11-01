# Informe de la PC3 (Quarto → Word)

Informe del Entregable 1 del Trabajo Final (PC3), en Quarto, renderizado a `.docx` como pide el
enunciado: uno por integrante, con el nombre `1ACC0065_PC3_202610_Informe_<código>` (se respeta el
`202610` del enunciado aunque el ciclo sea 2026-20) y su código en la carátula.

```bash
cp equipo.ejemplo.yml equipo.local.yml   # una vez: poner los códigos de alumno reales (no se versiona)
bash render.sh                           # tablas + historial + los tres .docx + participación → salida/
bash render.sh U000000002                # solo el de un código
FETCH=1 bash render.sh                   # además hace git fetch antes del historial
./empaquetar-pc3.sh                      # entrega en salida/entrega/ (se niega si queda algo pendiente)
```

Requisitos: Quarto ≥ 1.4 (probado con 1.10.19), Python 3 con pandas (o `uv`, que lo instala al
vuelo) y git. Para renderizar no hace falta Jupyter ni LaTeX; para **regenerar las figuras** en
local sí: los notebooks se corren con `jupyter nbconvert` (ver `notebooks/README.md`). El índice es un
campo de Word: `render.sh` marca el `.docx` para que Word pregunte al abrirlo si actualiza los
campos; hay que aceptar (o clic derecho sobre el índice → Actualizar campo).

Renderizar se hace con `render.sh`, no con `quarto render` a secas: el script arma antes
`generado/local/` (no versionado), que el proyecto necesita.

## Estructura

| Archivo | Qué es |
|---|---|
| `_quarto.yml` | Proyecto: formato docx, plantilla, APA, numeración, filtro y metadatos |
| `informe.qmd` | Esqueleto en el orden del enunciado, a) a j); incluye las secciones |
| `secciones/_NN-*.qmd` | Una sección por archivo, para escribir en paralelo sin pisarse |
| `equipo.ejemplo.yml` | Nombres y códigos **de ejemplo**; los reales van en `equipo.local.yml`, no versionado |
| `_participacion.md` | Plantilla del reporte de participación, sin niveles de cumplimiento; la llenada va en `participacion.local.md`, no versionado |
| `filtros/informe.lua` | Carátula por código, índice, figuras faltantes, avisos condicionales, tablas |
| `tablas.py` | `generado/*.json` → `generado/tablas/*.md` y `generado/cifras.yml` |
| `generado/` | JSON de Go, tablas y cifras derivadas, figuras, `historial.ejemplo.md` y `local/` (no versionado: equipo, participación e historial del momento) |
| `generar-historial.sh` | `generado/local/historial.md`: commits por integrante, PR fusionadas, commits de la entrega |
| `render.sh` | Todo lo anterior y los tres `.docx` |
| `empaquetar-pc3.sh` | Los `.docx`, el zip del código por integrante, la participación, `SHA256SUMS` |
| `referencias.bib`, `apa.csl` | Bibliografía (BibLaTeX) y estilo APA 7 oficial del repositorio de CSL |
| `recursos/` | `plantilla.docx` (estilos de Word), marcador de figura, logo; `crear_recursos.py` los regenera |
| `capturas/` | Capturas reales de la terminal (PNG) |

## Reglas

1. **Ninguna cifra a mano.** Las tablas se incluyen con
   `{{< include generado/tablas/x.md >}}` y las cifras de la prosa con
   `{{< meta cifras.grupo.clave >}}` (las claves están en `generado/cifras.yml`). Si hace falta
   una cifra nueva, se agrega a `tablas.py`, no al texto.
2. **Cada hueco es un aviso visible**: `::: {.callout-warning title="PENDIENTE"}` con qué va y de
   qué archivo sale. Se borra cuando el hueco se llena. `empaquetar-pc3.sh` no arma la entrega
   mientras quede uno, ni una cifra PLACEHOLDER, ni un dato o una figura sintéticos, ni una figura
   faltante.
3. **Figuras**: se referencian por su ruta final (`generado/figuras/x.png`, `capturas/x.png`,
   `../diagramas/arquitectura.png`). Si el archivo no existe, el filtro pone
   `recursos/figura-pendiente.png` y avisa por stderr; el render no falla. El aviso PENDIENTE de cada
   figura va dentro de un div `:::: {si-falta="ruta"}`, que desaparece solo cuando llega el PNG.
4. **Capturas**: de la terminal real, con el comando a la vista (regla del docente), en
   `capturas/`: `cargar.png`, `go-test-race.png`, `ci.png`, `benchmark.png`, `entrenar.png`,
   `docker.png`, `kanban.png`.
5. **Citas**: `[@clave]`, con la clave en `referencias.bib`. Solo referencias verificadas (DOI o
   documento oficial): el encabezado del `.bib` dice cómo se verificó cada una.
6. **Datos personales**: el repositorio es público. Los códigos de alumno y la participación
   llenada viven solo en `equipo.local.yml` y `participacion.local.md`; `render.sh` los copia a
   `generado/local/`, también ignorado.

## Figuras desde Colab

El notebook 03 dibuja las figuras del informe. Si corre en Colab, las deja en Drive, en
`MyDrive/aire-distribuido/reports/figuras/`, junto con `origen.json`, que dice de qué datos sale
cada una y si es sintética. Se copian **todas, con `origen.json`**, a `generado/figuras/`:

```bash
rclone copy <remoto>:aire-distribuido/reports/figuras/ docs/informe/generado/figuras/
# o descargarlas de Drive a mano y copiarlas a docs/informe/generado/figuras/
```

Los nombres son los que esperan las secciones (`completitud.png`, `distribuciones.png`, `codo.png`,
`perfiles.png`, `regimen-hora.png`, `regimen-estacion.png`, `episodio.png`, `speedup.png`,
`cobertura.png`). Una figura que `origen.json` marca con `"sintetico": true` lleva la marca de
agua «DATOS SINTÉTICOS» y hace fallar `tablas.py --final`.

## Decisiones

- **Motor `markdown`, no Jupyter.** El `.qmd` no tiene celdas ejecutables: `tablas.py` corre antes
  y el render solo incluye. Así renderizar no depende de un kernel, los tres renders son rápidos y
  las cifras salen siempre de los JSON de Go.
- **El parámetro `codigo` es metadato** (`-M codigo:U…`), porque el motor markdown no tiene
  `params`. El filtro lo busca en `equipo` y detiene el render si no está: no sale una carátula
  equivocada.
- **El índice lo pone el filtro, no `toc: true`.** Pandoc pone su índice antes de todo el cuerpo,
  es decir, antes de la carátula; el enunciado ordena carátula, resumen, índice. El filtro inserta
  el mismo campo TOC de Word donde está `::: indice`, y `render.sh` agrega `w:updateFields` al
  `settings.xml` del `.docx` para que Word lo llene al abrir.
- **Tablas de Word**: Quarto dibuja figuras, tablas y avisos como tablas de Word; el filtro las
  separa con un párrafo mínimo (si no, Word las funde) y alinea las tablas con id a la izquierda
  (Quarto las centra, también el texto de las celdas).
- **El speedup del resumen es el de un W fijo**, el máximo medido, con su IC: no el mejor de la
  tabla, que sesgaría hacia arriba.
- **Reporte de participación**: mismo contenido en el Anexo D y en un `.docx` suelto,
  `1ACC0065-202620_Participación_PC3.docx` (nombre de la plantilla oficial del curso con `_PC3`).
  La plantilla oficial es del docente y no se versiona.

## JSON de `generado/`: los escribe Go

Los esquemas son los de los comandos de Go, que son la fuente de verdad; `tablas.py` se adapta a
ellos, nunca al revés. Se copian desde `reports/` (o `python3 tablas.py --entrada ../../reports`).

| Archivo | Lo escribe | Esquema en | Qué toma el informe |
|---|---|---|---|
| `limpieza.json` | `cmd/cargar` | `datos/reporte.go`, reglas en `datos/doc.go` | `metadatos`, `sha256` del CSV y de las `configs`, `convencion_hora`; filas largas (leídas, procesadas, descartadas por regla en el orden de la cascada, marcadas: hora inferida, unidad convertida; hora explícita e inferida; días en cero y fuera de rango por parámetro; conversiones por parámetro); filas anchas (total, completas del gold, PM2.5 > PM10, con hora inferida); por estación (leídas, procesadas, anchas, gold, desde, hasta, ubicación y ubicaciones distintas); parámetros, periodos y unidades vistos; avisos; workers, bloques, duración y `fases_s` (para Amdahl) |
| `estandarizacion.json` | `cmd/cargar` | `datos/estandarizar.go` | `metadatos`, features, log1p, nivel, n, media, desv, ddof, chunk |
| `benchmark_cargador.json` | `cmd/cargar -bench` | `datos/medicion.go` | `metadatos`, `protocolo` (con `nota_recorte` si las repeticiones no alcanzan para el recorte); por W: tiempos, mediana, media recortada, speedup con IC bootstrap (`speedup_ic`, cociente de medias recortadas) y cociente de medianas (`speedup`), sha256 del silver y del gold; `identicos` |
| `benchmark_kmeans.json` | `cmd/benchmark` | `cmd/benchmark/main.go` | `resumen` por experimento (iteraciones fijas y hasta converger), modo, W y chunk: mediana, media recortada, ms por iteración, speedup con IC bootstrap, eficiencia; `determinismo` |
| `kmeans.json` | `cmd/entrenar` | `cmd/entrenar/main.go`, `kmeans/semillas.go`, `kmeans/doc.go` | k, semilla elegida, `arranques` (una corrida por semilla), `arranque_elegido`, corridas a la vez, `centroides_iniciales` (archivo, origen, sha256) y `nota_tiempos_arranques`, iteraciones, paro, inercia, tamaños, centroides estandarizados |
| `regimenes.json` | `cmd/entrenar` | `kmeans/regimenes.go` | por régimen: perfil (media aritmética de las lecturas), PM2.5 medio (derivado si no es feature), PM10 medio, etiqueta ECA, conteos por hora y por estación |
| `codo.json` | `cmd/entrenar -codo` | `kmeans/codo.go` | inercia, semilla, iteraciones y paro por k; el k elegido es el de la receta |
| `eda.json` | transcrito de `reports/eda_criba.md` (notebook 01) | — | lo que solo mide el EDA: el control de la hora por posición (días completos con hora explícita), los registros multilínea y los episodios de 24 h sobre el ECA. Si el notebook 01 vuelve a correr, se actualiza desde el `.md` |

Los siete de Go son copias de `reports/` de la **corrida sobre el CSV completo de OEFA** (01/11).
Para actualizarlos, se vuelven a copiar desde `reports/` después de correr los comandos. Si alguna
vez se prueba con la muestra sintética, se marca `"placeholder": true` y las guardas de `--final`
la detectan igual.

`tablas.py` comprueba los invariantes de Go (leídas = procesadas + descartadas = procesadas + Σ por
regla; las sumas por estación, por cluster y por hora; los dos speedups del cargador; el arranque
elegido es el de menor inercia; el determinismo de los dos benchmarks) y los avisa en el informe.
Con `--estricto` falla si falta un JSON o un invariante no cuadra. Con `--final`, el que usa
`empaquetar-pc3.sh`, además falla con cualquier señal de datos de prueba: un `placeholder`, una
`entrada` bajo `data/sample/`, una estación `SINT-` o `SIN-`, un gold sintético o una figura
sintética en `generado/figuras/origen.json`. Go no marca sus salidas como sintéticas: estas guardas
son las que impiden entregar la muestra como si fueran los datos.

**Lo que el informe querría y no sale de Go** (no se inventa): la banda INCA de cada régimen
(`regimenes.json` trae el perfil y la etiqueta ECA; los nombres de los regímenes son una lectura de
sus perfiles, escrita en la prosa); la tabla de decisión de la calibración (notebook 02); y lo que
solo mide el EDA (`generado/eda.json`, transcrito).

## Antes de entregar

- `equipo.local.yml` con los códigos reales y `participacion.local.md` con los niveles marcados.
- `grep -rn 'title="PENDIENTE' secciones/` vacío, `python3 tablas.py --final` sin errores.
- Push de todas las ramas y `FETCH=1 bash render.sh` con `main` ya en el tag `pc3`.
- Tablero Kanban y video (si la PC3 lo lleva) publicados.
- `./empaquetar-pc3.sh pc3`: cada integrante sube su `.docx` y su `.zip`; el coordinador, además,
  la participación.
