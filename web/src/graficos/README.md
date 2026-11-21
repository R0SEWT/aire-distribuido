# graficos/

Los gráficos de D3 (spec §9.2): `Mapa`, `Curva`, `Inercia` y `TiemposPorNodo`. Los escribe cada pantalla.

Dos reglas, por la CSP de la app (`web/Caddyfile`):

- D3 con `.style()` y clases, **no** con `style` como atributo (`.attr('style', …)`): la política de contenido no
  admite estilos en línea.
- Los colores salen de los tokens (`var(--color-inca-mala)`, o `BANDAS` en `componentes/bandas.ts`), para que el
  modo proyector los cambie solo. El color nunca va solo: cada estado lleva su palabra.
