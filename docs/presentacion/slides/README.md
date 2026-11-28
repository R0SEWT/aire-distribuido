# Deck (Quarto + reveal.js + D3)

`index.qmd` es la sustentación del TB2: portada, 30 láminas y 10 de respaldo (los números de lámina de `reparto.md` y `preguntas.md` son los que muestra el deck, con la portada como 1) en 16:9 (1280×720).
Dura unos 15 minutos más 3:50 de demo (en el medio, después de la SPA), y se reparte entre los tres integrantes; cada lámina dice
quién habla, arriba a la derecha, y sus notas traen el tiempo y qué decir. El reparto, la versión
corta y el guion de la demo están en [`../reparto.md`](../reparto.md); las preguntas probables del
docente, en [`../preguntas.md`](../preguntas.md). La versión de la PC3 está en el tag `pc3.1`.

Sigue el método del deck de SIMBig 2026: la historia primero, ninguna cifra escrita a mano, el
diseño en tokens y las piezas gráficas con clases que toman el color del tema.

## Ver y presentar

```bash
bash render.sh          # deja _site/index.html
bash render.sh --pdf    # y además _site/deck.pdf
```

`_site/index.html` es un solo archivo (`embed-resources: true`): D3, los datos, las imágenes y las
fuentes van adentro, así que se abre con doble clic y funciona sin red. Por eso D3 está
vendorizado en `js/d3.v7.min.js` y no se usa el OJS de Quarto, que lo baja de un CDN al abrir.

| Tecla | Qué hace |
|---|---|
| `S` | Vista del orador: las notas, con el tiempo y quién habla |
| `M` | Menú con todas las láminas |
| `F` | Pantalla completa |

La lámina del worker pool anima una iteración del K-Means: los botones `W = 1, 2, 4, 8` cambian la
cantidad de workers y la animación vuelve a empezar. Se detiene sola al salir de la lámina. Las de
la latencia de la API y del impacto por actividad se dibujan al entrar.

Respaldo en PDF: `bash render.sh --pdf`, o abrir `_site/index.html?print-pdf` en Chrome e imprimir.
En el PDF los gráficos salen en su estado final.

En línea, la de la PC3: <https://uni.rosewt.dev/concurrente/pc3/>. La del TB2 se publica con
`bash ../publicar.sh tb2`, con el OK de Rody.

## De dónde salen los números

- **El texto** los toma de `docs/informe/generado/cifras.yml`, los mismos del informe, con el
  shortcode `{{< meta cifras.… >}}`. Lo que solo cita el deck lo escribe `datos.py` en `_deck.yml`
  (`{{< meta deck.… >}}`): las funciones de prueba de Go, las pruebas del frontend, la latencia del
  WebSocket y las
  cifras de impacto de `reports/impacto.json` (horas sobre el ECA, episodios, anticipación, el
  distrito con más horas sobre el ECA), con el mismo formato que `tablas.py`.
- **Los gráficos de D3** leen `datos.js`, que `datos.py` arma desde `reports/*.json` sin redondear
  nada: el embudo, los speedups, los niveles, la latencia de `reports/api_latencia.json` y el
  impacto por actividad. `datos.js` y `_deck.yml` se versionan: el deck se renderiza sin correr Go.
- **El código Go** de las láminas de sockets, de la ronda y de la API es texto, no imagen, con el
  archivo encima: son extractos del código real (las referencias con su línea están en
  `../preguntas.md`). Si el código cambia, hay que revisarlos a mano.
- **Las capturas de la app** son las de `docs/informe/capturas/tb2/`, de la corrida de punta a
  punta del 27/11/2026 sobre el Compose del repo (su README dice cómo se tomaron): referenciadas,
  no copiadas, como las figuras del informe.
- **Las cifras de otras plataformas** (el cargador en Windows y la reproducción en macOS) son
  `cifras.benchmark_cargador_windows` y `cifras.revision`, del informe. Las pruebas del frontend
  y la latencia del WebSocket entre dos sesiones las lee `datos.py` de
  `docs/informe/capturas/tb2/pruebas/` (`web-pruebas.txt` y `ws_latencia.json`).
- **Las figuras y diagramas** son los del informe (`docs/diagramas/`,
  `docs/informe/generado/figuras/`), referenciados, no copiados.
- **Las capturas de terminal** son la pantalla entera y en una lámina no se leen: `recortes.py`
  recorta de las del informe (`docs/informe/capturas/`) solo lo que importa, sin reescalar, y lo
  deja en `img/captura-*.png`. En la lámina de evidencias se muestran grandes, una por clic.
  `render.sh` los rehace si hay `uv`.

Si cambian los reports, se corre `tablas.py` del informe (para `cifras.yml`) y después
`render.sh`; el deck no puede contradecir al informe.

## Dónde se diseña

- `custom.scss`: los tokens (`$tinta`, `$bajo`, `$medio`, `$alto`, `$critico`, …), las fuentes
  (Inter, Inter Tight, IBM Plex Mono) y todas las piezas: `.cifra`, `.pregunta`, `.tres`, `.aviso`,
  `.chips`, `.evid-fila`, `.evid-stack` y las clases de los SVG. Los colores de los tres niveles son los de las
  figuras del informe.
- `js/graficos.js`: los gráficos de D3 (embudo de la limpieza, speedup con su intervalo, el worker
  pool animado, los tres niveles frente al ECA, el nivel alto por hora y por estación, la latencia
  de la API frente a los 100 ms en escala logarítmica y las horas sobre el ECA por actividad). Cada uno es
  un `<div data-grafico="…">` en `index.qmd` y se anima al entrar a la lámina; los fragmentos
  `.paso` con `data-paso` le dicen en qué paso está.
- `img/estaciones-portada.svg`: las estaciones en su posición real, para la portada (lo genera
  `datos.py`). `img/qr-repo.svg`: el QR del repositorio (segno).

## Pendiente

- Que cada integrante revise sus láminas, sus notas y sus preguntas antes de exponer.
- Ensayar con reloj: el reparto suma 14:55 más 3:50 de demo; si el docente da menos tiempo, la
  versión corta está en `../reparto.md`.
- Si cambian los reports o las cifras del informe del TB2, correr `render.sh` otra vez.
