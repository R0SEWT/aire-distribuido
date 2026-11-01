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

## Contrato de los JSON (`generado/`)

Los escriben los comandos de Go (o se copian desde `reports/`: `python3 tablas.py --entrada
../../reports`). Hoy son **PLACEHOLDER**: `"placeholder": true` y cifras inventadas con este
esquema. Al llegar el real se reemplaza el archivo entero (sin `placeholder`). Si el esquema real
difiere, se ajusta `tablas.py`, no el informe. Campos comunes: `generado_por` (comando), `momento`
(ISO 8601), `commit`, `maquina`.

| Archivo | Lo escribe | Campos |
|---|---|---|
| `limpieza.json` | `cmd/cargar` | `fuente` {`archivo`, `bytes`, `sha256`}; `parametros` {`workers`, `bloque_bytes`, `config`}; `duracion_s`; `filas_largas` {`leidas`, `procesadas`, `descartadas`}; `reglas` [{`regla`, `nombre`, `descripcion`, `descartadas`, `marcadas`}] (reglas 1 a 6, en filas largas); `filas_anchas` {`total`, `completas`}; `violaciones_pm2_5_mayor_pm10`; `estaciones` [{`estacion`, `nombre`, `departamento`, `desde`, `hasta`, `filas_anchas`, `completas`}] |
| `estandarizacion.json` | `cmd/cargar` | `receta`; `nivel` (`absoluto` o `por_estacion`); `n`; `features` [{`feature`, `log1p`, `media`, `desv`}], o con `por_estacion` [{`estacion`, `n`, `media`, `desv`}] si el nivel es por estación |
| `benchmark_cargador.json`, `benchmark_kmeans.json` | `cmd/benchmark` | `metadatos` {`maquina`, `version_go`, `gomaxprocs`, `cpus_logicas`, `commit`, `arbol_sucio`}; `protocolo` {`repeticiones`, `calentamiento`, `estadistico`, `referencia` (`workers=1` o `secuencial`), `unidad_chunk` (`bytes` o `filas`), `replicas_bootstrap`, `nivel_ic`, `semilla`, y en el K-Means `iteraciones_fijas`, `k`}; `datos`; `resultados` [{`modo`, `workers`, `chunk`, `repeticiones`, `tiempos_s` [], `mediana_s`, `speedup` {`punto`, `ic_inferior`, `ic_superior`, `nivel`}, `eficiencia`, y en el K-Means `ms_por_iteracion`}]. La primera fila de `resultados` es la referencia |
| `kmeans.json` | `cmd/entrenar` | `receta` {`features`, `log1p`, `nivel`, `k`, `semilla`, `tolerancia`, `max_iter`, `config`}; `n`; `d`; `iteraciones`; `convergio`; `inercia`; `duracion_s`; `clusters` [{`cluster`, `tamano`, `centroide_z` [d], `centroide` {feature: valor en su unidad}}] |
| `regimenes.json` | `cmd/entrenar` | `eca` {`pm2_5_24h`, `pm10_24h`, `unidad`, `norma`, `regla`}; `unidades`; `regimenes` [{`cluster`, `nombre`, `tamano`, `proporcion`, `perfil` {parámetro: media}, `etiqueta_eca` (`no excede`, `excede PM2.5`, `excede PM10`, `excede ambos`), `banda_inca`, `por_hora` [24 proporciones que suman 1], `por_estacion` [{`estacion`, `filas`, `proporcion`}]}] |
| `codo.json` | `cmd/entrenar -codo` | `submuestra` {`n`, `metodo`}; `semilla`; `k_elegido`; `puntos` [{`k`, `inercia`}] |

`tablas.py` comprueba los invariantes (leídas = procesadas + descartadas; suma por regla, por
estación y por cluster; `por_hora` suma 1) y los avisa en el informe; con `--estricto` falla.

## Antes de entregar

- `grep -rn 'title="PENDIENTE' secciones/` vacío, `python3 tablas.py --estricto` sin errores.
- Push de todas las ramas y `./generar-historial.sh` con `main` ya en el tag `pc3`.
- Tablero Kanban y video (si la PC3 lo lleva) publicados.
- `./empaquetar-pc3.sh pc3`: cada integrante sube su `.docx` y su `.zip`; el coordinador, además,
  la participación.
