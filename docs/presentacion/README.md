# Exposición

La presentación del sistema. El deck creció desde la PC3 hasta la **sustentación del TB2**, en la
clase de la semana 15, donde «cada grupo es responsable de presentar el funcionamiento del sistema
y responder a las preguntas del docente». El video del TB2 (6 minutos como máximo) vive aparte, en
[`docs/video/`](../video/).

- [`slides/`](slides/README.md): el deck en Quarto, reveal.js y D3. `bash slides/render.sh` deja un
  HTML que funciona sin red.
- **La exposición de la PC4** es otro deck, [`slides/pc4.qmd`](slides/pc4.qmd)
  (`bash slides/render.sh pc4` → `slides/_site/pc4.html`): 21 láminas y 5 de respaldo, en el orden
  de la rúbrica de la PC4 (Distribución, API, Bases de datos, Evidencias, Documentación, GitHub y
  Kanban). Toma del deck del TB2 las láminas del clúster, la API y las bases, y agrega una lámina
  con las cinco observaciones del docente y dónde se responde cada una, más Docker, los registros
  en MongoDB y Redis, una demo corta en el Compose de la laptop y el Kanban con la tarjeta #76
  marcada (`slides/recorte_kanban.py`). Dura unos 10:45 más 2:20 de demo; cada lámina dice quién
  habla y sus notas traen el tiempo.
- [`reparto.md`](reparto.md): quién dice cada lámina y cuánto dura, la versión corta, el guion de
  la demo en vivo, la preparación y el plan B si falla la red.
- [`preguntas.md`](preguntas.md): las 29 preguntas más probables del docente, con una respuesta
  corta y el archivo y la línea del código que la respaldan; las observaciones de la PC4 y cómo se
  responden ahora; lo que no se dice.
- [`auditoria/`](auditoria/auditoria.md): la revisión con la skill `revisar-entrega` (modo A) contra
  la rúbrica, el vocabulario de los labs y el informe del TB2, con un corrector en frío: veredicto,
  matriz, afirmaciones corregidas y riesgos. La cierra una lectura humana.
- [`en-sencillo.md`](en-sencillo.md): la historia de la PC3 en palabras simples, escrita antes que
  las láminas.
- **Publicado en <https://uni.rosewt.dev/concurrente/pc3/>** (la PC3), con su PDF y una portada
  del curso en <https://uni.rosewt.dev/concurrente/>. `bash publicar.sh tb2` renderiza el deck y lo
  publica en `/tb2/` del sitio de Netlify `concurrente-rosewt`, que el catálogo
  [uni.rosewt.dev](https://github.com/R0SEWT/uni) monta en `/concurrente/`; `sitio/index.html` es la
  portada del curso: tiene la tarjeta de la PC4 y la del TB2 comentada, que se descomenta al
  publicarlo (hasta entonces `/tb2/` da 404). Netlify publica `sitio/` entero: antes de publicar
  el TB2, `sitio/pc3/` y `sitio/pc4/` tienen que seguir en la carpeta (si no están, se rehacen
  desde el tag `pc3.1` y con `bash publicar.sh pc4`). El TB2 todavía no está publicado.
- **La PC4 está publicada en <https://uni.rosewt.dev/concurrente/pc4/>**, con su PDF
  (`bash publicar.sh pc4`).

Mismo método que la charla de SIMBig 2026: primero la historia, después el deck, y ninguna cifra
escrita a mano (salen de `reports/` y de las cifras del informe).

Base hecha con IA (Claude) a partir del código, de `reports/` y del informe. Antes de exponer, cada
integrante revisa sus láminas, sus notas y sus preguntas.
