// Cómo se reparten unas horas entre las cuatro bandas INCA: una barra apilada.
// Sirve en grande (el total, con su leyenda en palabras al lado) y en chico
// (una fila de una tabla, junto a la banda más frecuente escrita).

import { scaleLinear, select } from 'd3'
import { BANDAS } from '../componentes/bandas'
import { numero, porciento } from '../componentes/formato'
import type { HorasPorBanda, NombreDeBanda } from '../api/datos'
import { useDibujo } from './dibujo'
import { useMedida } from './medida'

const ORDEN: readonly NombreDeBanda[] = ['buena', 'moderada', 'mala', 'umbral_de_cuidado']

interface Props {
  horas: HorasPorBanda
  /** De qué son las horas: «CA-HU-04», «todas las estaciones». */
  de: string
  /** El alto de la barra, en rem. */
  alto?: number
}

interface Tramo {
  banda: NombreDeBanda
  horas: number
  desde: number
  ancho: number
}

// Donde no hay con qué medir (las pruebas, en jsdom) se dibuja a este ancho.
const POR_OMISION = { ancho: 760 }

export function BarraBandas({ horas, de, alto = 0.75 }: Props) {
  const [contenedor, { ancho, rem }] = useMedida<HTMLDivElement>(POR_OMISION)
  const total = ORDEN.reduce((suma, b) => suma + horas[b], 0)
  const altoPx = alto * rem

  const svg = useDibujo<SVGSVGElement>(
    (nodo) => {
      // Una banda con pocas horas no desaparece: tiene un ancho mínimo, que se les quita a las demás.
      const minimo = Math.min(4, ancho / 8)
      const presentes = ORDEN.filter((b) => horas[b] > 0)
      const x = scaleLinear()
        .domain([0, Math.max(1, total)])
        .range([0, Math.max(0, ancho - (presentes.length - 1))])
      const anchos = presentes.map((b) => Math.max(minimo, x(horas[b])))
      const sobra = anchos.reduce((s, a) => s + a, 0) + (presentes.length - 1) - ancho
      const mayor = anchos.indexOf(Math.max(...anchos))
      if (sobra > 0 && mayor >= 0) anchos[mayor] = Math.max(minimo, anchos[mayor]! - sobra)
      let desde = 0
      const tramos: Tramo[] = presentes.map((banda, i) => {
        const t = { banda, horas: horas[banda], desde, ancho: anchos[i]! }
        desde += anchos[i]! + 1
        return t
      })
      select(nodo)
        .selectAll<SVGRectElement, Tramo>('rect')
        .data(tramos, (t) => t.banda)
        .join((entra) => {
          const r = entra.append('rect')
          r.append('title')
          return r
        })
        .attr('x', (t) => t.desde)
        .attr('y', 0)
        .attr('width', (t) => t.ancho)
        .attr('height', altoPx)
        .attr('rx', Math.min(3, altoPx / 3))
        .style('fill', (t) => BANDAS[t.banda].color)
        .select('title')
        .text((t) => `${BANDAS[t.banda].palabra}: ${numero(t.horas)} horas (${porciento(t.horas, total)})`)
    },
    [horas, total, ancho, altoPx],
  )

  const descripcion = ORDEN.map((b) => `${BANDAS[b].palabra.toLowerCase()} ${porciento(horas[b], total)}`).join(', ')
  return (
    <div ref={contenedor} className="w-full">
      <svg ref={svg} width={ancho} height={altoPx} role="img" aria-label={`Bandas INCA de ${de}: ${descripcion}.`} className="block" />
    </div>
  )
}
