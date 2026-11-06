# Deck (Quarto + reveal.js + D3)

`index.qmd` es la exposición de la PC3: portada, 19 láminas y 7 de respaldo en 16:9 (1280×720).
Dura unos 9 minutos y se reparte entre los tres integrantes; cada lámina dice quién habla, arriba a
la derecha, y sus notas traen el tiempo y qué decir. El arco y las preguntas probables del docente
están en [`../en-sencillo.md`](../en-sencillo.md).

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
cantidad de workers y la animación vuelve a empezar. Se detiene sola al salir de la lámina.

Respaldo en PDF: `bash render.sh --pdf`, o abrir `_site/index.html?print-pdf` en Chrome e imprimir.
En el PDF los gráficos salen en su estado final.

## De dónde salen los números

- **El texto** los toma de `docs/informe/generado/cifras.yml`, los mismos del informe, con el
  shortcode `{{< meta cifras.… >}}`. Lo que solo cita el deck (las funciones de prueba) lo escribe
  `datos.py` en `_deck.yml`.
- **Los gráficos de D3** leen `datos.js`, que `datos.py` arma desde `reports/*.json` sin redondear
  nada. `datos.js` se versiona: el deck se renderiza sin correr Go.
- **Las figuras y diagramas** son los del informe (`docs/diagramas/`,
  `docs/informe/generado/figuras/`, `docs/informe/capturas/`), referenciados, no copiados.

Si cambian los reports, se corre `tablas.py` del informe (para `cifras.yml`) y después
`render.sh`; el deck no puede contradecir al informe.

## Dónde se diseña

- `custom.scss`: los tokens (`$tinta`, `$bajo`, `$medio`, `$alto`, `$critico`, …), las fuentes
  (Inter, Inter Tight, IBM Plex Mono) y todas las piezas: `.cifra`, `.pregunta`, `.tres`, `.aviso`,
  `.chips`, `.evidencias` y las clases de los SVG. Los colores de los tres niveles son los de las
  figuras del informe.
- `js/graficos.js`: los gráficos de D3 (embudo de la limpieza, speedup con su intervalo, el worker
  pool animado, los tres niveles frente al ECA, el nivel alto por hora y por estación). Cada uno es
  un `<div data-grafico="…">` en `index.qmd` y se anima al entrar a la lámina; los fragmentos
  `.paso` con `data-paso` le dicen en qué paso está.
- `img/estaciones-portada.svg`: las estaciones en su posición real, para la portada (lo genera
  `datos.py`). `img/qr-repo.svg`: el QR del repositorio (segno).

## Pendiente

- Que cada integrante revise sus láminas y sus notas antes de exponer.
- Para el video del TB2 (6 minutos como máximo): reemplazar «Hacia la PC4» y «Qué sigue» por las
  láminas del clúster, la API y el frontend, y recortar.
