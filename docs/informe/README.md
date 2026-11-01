# Informe de la PC3 (Quarto → Word)

Informe del Entregable 1 del Trabajo Final (PC3), en Quarto, renderizado a `.docx` como pide el
enunciado: uno por integrante, con el nombre `1ACC0065_PC3_202610_Informe_<código>` (se respeta el
`202610` del enunciado aunque el ciclo sea 2026-20) y su código en la carátula.

```bash
bash render.sh                # tablas + historial + los tres .docx + participación → salida/
bash render.sh U202311495     # solo el de un código
./empaquetar-pc3.sh           # entrega completa en salida/entrega/ (se niega si queda algo pendiente)
```

Requisitos: Quarto ≥ 1.4 (probado con 1.10.19), Python 3 con pandas (o `uv`, que lo instala al
vuelo) y git. No hace falta Jupyter ni LaTeX. Para ver el índice, abrir el `.docx` en Word y aceptar
«actualizar campos» (o clic derecho sobre el índice → Actualizar campo).

## Estructura

| Archivo | Qué es |
|---|---|
| `_quarto.yml` | Proyecto: formato docx, plantilla, APA, numeración, equipo y el parámetro `codigo` |
| `informe.qmd` | Esqueleto en el orden del enunciado, a) a j); incluye las secciones |
| `secciones/_NN-*.qmd` | Una sección por archivo, para escribir en paralelo sin pisarse |
| `_participacion.md` | Reporte de participación (Anexo D y archivo suelto `participacion.qmd`) |
| `filtros/informe.lua` | Carátula por código, índice, figuras faltantes, avisos condicionales, tablas |
| `tablas.py` | `generado/*.json` → `generado/tablas/*.md` y `generado/cifras.yml` |
| `generado/` | JSON que escriben los comandos de Go, tablas y cifras derivadas, historial, figuras |
| `generar-historial.sh` | `generado/historial.md`: commits por integrante, PR fusionadas, commits de la entrega |
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
   mientras quede uno, ni una cifra PLACEHOLDER, ni una figura faltante.
3. **Figuras**: se referencian por su ruta final (`generado/figuras/x.png`, `capturas/x.png`,
   `../diagramas/arquitectura.png`). Si el archivo no existe, el filtro pone
   `recursos/figura-pendiente.png` y avisa por stderr; el render no falla. El aviso PENDIENTE de cada
   figura va dentro de un div `:::: {si-falta="ruta"}`, que desaparece solo cuando llega el PNG.
4. **Capturas**: de la terminal real, con el comando a la vista (regla del docente), en
   `capturas/`: `cargar.png`, `go-test-race.png`, `ci.png`, `benchmark.png`, `entrenar.png`,
   `docker.png`, `kanban.png`.
5. **Citas**: `[@clave]`, con la clave en `referencias.bib`. Solo referencias verificadas (DOI o
   documento oficial): el encabezado del `.bib` dice cómo se verificó cada una.

## Decisiones

- **Motor `markdown`, no Jupyter.** Hay Jupyter en la laptop, pero el `.qmd` no tiene celdas
  ejecutables: `tablas.py` corre antes y el render solo incluye. Así renderizar no depende de un
  kernel, los tres renders son rápidos y las cifras salen siempre de los JSON de Go.
- **El parámetro `codigo` es metadato** (`-M codigo:U…`), porque el motor markdown no tiene
  `params`. El filtro lo busca en `equipo` y falla si no está: no sale una carátula equivocada.
- **El índice lo pone el filtro, no `toc: true`.** Pandoc pone su índice antes de todo el cuerpo,
  es decir, antes de la carátula; el enunciado ordena carátula, resumen, índice. El filtro inserta
  el mismo campo TOC de Word donde está `::: indice`.
- **Tablas de Word**: Quarto dibuja figuras, tablas y avisos como tablas de Word; el filtro las
  separa con un párrafo mínimo (si no, Word las funde) y alinea las tablas con id a la izquierda
  (Quarto las centra, también el texto de las celdas).
- **Reporte de participación**: mismo contenido en el Anexo D y en un `.docx` suelto,
  `1ACC0065-202620_Participación_PC3.docx` (nombre de la plantilla oficial del curso con `_PC3`).
  La plantilla oficial es del docente y no se versiona.

## JSON de `generado/`: los escribe Go

Los esquemas son los de los comandos de Go, que son la fuente de verdad; `tablas.py` se adapta a
ellos, nunca al revés. Se copian desde `reports/` (o `python3 tablas.py --entrada ../../reports`).

| Archivo | Lo escribe | Esquema en | Qué toma el informe |
|---|---|---|---|
| `limpieza.json` | `cmd/cargar` | `datos/reporte.go` | filas largas (leídas, procesadas, descartadas y marcadas por regla, fuera de rango por parámetro), filas anchas (total, completas del gold, PM2.5 > PM10), por estación (leídas, procesadas, anchas, gold, desde, hasta, departamento), parámetros, periodos y unidades vistos, workers, bloques, duración |
| `estandarizacion.json` | `cmd/cargar` | `datos/estandarizar.go` | features, log1p, nivel, n, media, desv, ddof, chunk |
| `benchmark_cargador.json` | `cmd/cargar -bench` | `datos/medicion.go` | por W: tiempos, mediana, speedup (cociente de medianas), sha256 del silver y del gold; `identicos` |
| `benchmark_kmeans.json` | `cmd/benchmark` | `cmd/benchmark/main.go` | `resumen` por experimento (iteraciones fijas y hasta converger), modo, W y chunk: mediana, media recortada, ms por iteración, speedup con IC bootstrap, eficiencia; `determinismo` |
| `kmeans.json` | `cmd/entrenar` | `cmd/entrenar/main.go` | k, k efectivo, semilla, iteraciones, paro, inercia, tamaños, centroides estandarizados y des-estandarizados |
| `regimenes.json` | `cmd/entrenar` | `kmeans/regimenes.go` | por régimen: perfil (media aritmética), PM2.5 medio (derivado si no es feature), PM10 medio, etiqueta ECA, conteos por hora y por estación |
| `codo.json` | `cmd/entrenar -codo` | `kmeans/codo.go` | inercia, iteraciones y paro por k; el k elegido es el de la receta |

Hoy los siete son **PLACEHOLDER**: salidas reales de Go sobre la muestra sintética
`data/sample/oefa_muestra.csv`, con `"placeholder": true` y una `_nota` agregadas a mano. Al
correr los comandos sobre el CSV de OEFA se reemplaza cada archivo entero (sin `placeholder`).

`tablas.py` comprueba los invariantes de Go (leídas = procesadas + descartadas = procesadas + Σ por
regla; las sumas por estación, por cluster y por hora; el speedup del cargador como cociente de
medianas; el determinismo de los dos benchmarks) y los avisa en el informe. Con `--estricto` falla
si falta un JSON o un invariante no cuadra; con `--final`, además, si queda un PLACEHOLDER (es el que
usa `empaquetar-pc3.sh`).

**Lo que el informe querría y Go todavía no escribe** (está como PENDIENTE en el informe, no
inventado): commit, fecha y máquina en `limpieza.json` y `estandarizacion.json`; sha256 del CSV de
entrada; intervalo de confianza del speedup del cargador; nombre interpretativo y banda INCA de cada
régimen; la tabla de decisión de la calibración (sale de los notebooks).

## Antes de entregar

- `grep -rn 'title="PENDIENTE' secciones/` vacío, `python3 tablas.py --final` sin errores.
- Push de todas las ramas y `./generar-historial.sh` con `main` ya en el tag `pc3`.
- Tablero Kanban y video (si la PC3 lo lleva) publicados.
- `./empaquetar-pc3.sh pc3`: cada integrante sube su `.docx` y su `.zip`; el coordinador, además,
  la participación.
