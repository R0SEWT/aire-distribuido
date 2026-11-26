# Exposición

La presentación del sistema: hoy cuenta la PC3 y crece hacia la sustentación de la semana 15 y el
video del TB2, que el enunciado pide de 6 minutos como máximo y con la participación de todos.

- [`en-sencillo.md`](en-sencillo.md): la historia en palabras simples, el reparto entre los tres,
  lo que no se dice y las preguntas probables del docente con su respuesta corta. Se escribió
  antes que las láminas.
- [`slides/`](slides/README.md): el deck en Quarto, reveal.js y D3. `bash slides/render.sh` deja un
  HTML que funciona sin red.
- **Publicado en <https://uni.rosewt.dev/concurrente/pc3/>**, con su PDF y una portada del curso
  en <https://uni.rosewt.dev/concurrente/>. `bash publicar.sh` renderiza el deck y lo vuelve a
  publicar en el sitio de Netlify `concurrente-rosewt`, que el catálogo
  [uni.rosewt.dev](https://github.com/R0SEWT/uni) monta en `/concurrente/`; `sitio/index.html` es
  la portada del curso. Al publicar la PC4 o el
  TB2, `bash publicar.sh pc4` lo deja en `/pc4/` sin tocar `/pc3/`, siempre que `sitio/pc3/` siga
  en la carpeta: Netlify publica `sitio/` entero.

Mismo método que la charla de SIMBig 2026: primero la historia, después el deck, y ninguna cifra
escrita a mano (salen de `reports/` y de las cifras del informe).

Base hecha con IA (Claude) a partir del informe de la PC3. Antes de exponer, cada integrante
revisa sus láminas y sus notas.
