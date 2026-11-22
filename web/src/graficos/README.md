# graficos/

Los gráficos de D3 (spec §9.2). Cada uno recibe sus datos ya armados por la pantalla y no pide nada.

| Archivo | Qué dibuja | Quién lo usa |
|---|---|---|
| `MapaPeru.tsx` | El mapa del Perú con las 33 estaciones (también exporta `GlifoEstacion` y `TramaNoMide`, para la leyenda) | Panel |
| `CurvaSesion.tsx` | La media de 24 h de lo que la sesión vio de una estación y, si las envió, sus lecturas horarias | Estación |
| `CurvaSimulacion.tsx` | Las lecturas horarias de un tramo (barras) y la media que respondió la API | Simulación |
| `RejillaEscenario.tsx` | Una fila por estación y una celda por hora del escenario | Simulación |
| `BarraBandas.tsx` | Cómo se reparten unas horas entre las bandas INCA | Impacto |
| `SenalesAlerta.tsx` | Precisión y cobertura de cada señal | Impacto |
| `AnticipacionEpisodios.tsx` | Con cuánta anticipación se enciende cada señal, episodio por episodio | Impacto |
| `InerciaPorRonda.tsx` | La inercia de un entrenamiento, ronda por ronda | Entrenamiento |
| `TiemposPorNodo.tsx` | Lo que tardó cada nodo en responder cada ronda | Entrenamiento |
| `UsoDeCpuEnElTiempo.tsx` | El uso de CPU del proceso de la API en los últimos minutos | Clúster |
| `LatenciaPorRuta.tsx` | El eje, la marca y la leyenda de la latencia de una ruta, en escala logarítmica | Clúster |

Lo que comparten:

- **`medida.ts`**: `useMedida()`, el único lugar que mide. Da el ancho y el alto de la caja del gráfico, cuántos
  píxeles vale 1 rem ahora y la escala (`rem / 16`), y vuelve a medir cuando cambia el tamaño o el modo
  proyector. Los gráficos se dibujan con los píxeles de su caja, sin un `viewBox` que escale: los márgenes salen de
  `rem` o de `escala`, y la letra, de clases (`text-xs`…), que ya están en rem.

  ```tsx
  const [caja, { ancho, rem }] = useMedida<HTMLDivElement>({ ancho: 640 }) // lo de por omisión vale en las pruebas
  return <div ref={caja} className="w-full"><svg width={ancho} height={8 * rem} /></div>
  ```

- **`dibujo.ts`**: `useDibujo()`, para los gráficos en que D3 dibuja por su cuenta con `.join()` sobre un nodo
  (los de Simulación e Impacto). En los demás, React arma el SVG y D3 pone las escalas y las formas.
- **`ejeDeRondas.ts`**: el eje horizontal que comparten `InerciaPorRonda` y `TiemposPorNodo`, para que una ronda
  quede en la misma vertical en los dos.

Los números y las fechas de un gráfico se escriben con `componentes/formato.ts`, como en el resto de la app; las
bandas INCA, con `BANDAS` de `componentes/bandas.ts`. Un gráfico no importa nada de `pantallas/`: lo que necesita
de los datos está en `api/` (`api/datos.ts`, `api/vistas.ts`).

Dos reglas, por la CSP de la app (`web/Caddyfile`):

- D3 con `.style()` y clases, **no** con `style` como atributo (`.attr('style', …)`): la política de contenido no
  admite estilos en línea.
- Los colores salen de los tokens (`var(--color-inca-mala)`, o `BANDAS` en `componentes/bandas.ts`), para que el
  modo proyector los cambie solo. El color nunca va solo: cada estado lleva su palabra o su forma.
