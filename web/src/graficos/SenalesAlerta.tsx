// Qué acertaría cada señal si se usara como alerta: precisión (de las horas en
// que suena, en cuántas la media de 24 h supera el ECA) y cobertura (de las
// horas sobre el ECA, cuántas detecta). Barras de 0 a 100 %, con su cifra escrita.

import { scaleLinear, select } from 'd3'
import { numero } from '../componentes/formato'
import { useDibujo } from './dibujo'
import { useMedida } from './medida'

export interface SenalDibujada {
  nombre: string
  /** Entre 0 y 1. */
  precision: number
  cobertura: number
  /** Los conteos de los que salen: «524 de 1,497 horas». */
  detallePrecision: string
  detalleCobertura: string
}

interface Barra {
  clave: string
  senal: string
  medida: 'precisión' | 'cobertura'
  valor: number
  detalle: string
  fila: number
  primera: boolean
}

// Donde no hay con qué medir (las pruebas, en jsdom) se dibuja a este ancho.
const POR_OMISION = { ancho: 760 }

export function SenalesAlerta({ senales }: { senales: readonly SenalDibujada[] }) {
  const [contenedor, { ancho, rem }] = useMedida<HTMLDivElement>(POR_OMISION)
  const altoFila = 1.7 * rem
  const altoNombre = 1.6 * rem
  const entreGrupos = 0.9 * rem
  const arriba = 1.5 * rem
  const altoGrupo = altoNombre + 2 * altoFila + entreGrupos
  const alto = arriba + senales.length * altoGrupo

  const svg = useDibujo<SVGSVGElement>(
    (nodo) => {
      const raiz = select(nodo)
      const izquierda = 5.5 * rem
      const derecha = 4.5 * rem
      const x = scaleLinear()
        .domain([0, 1])
        .range([izquierda, Math.max(izquierda + 10, ancho - derecha)])
      const yDeGrupo = (grupo: number) => arriba + grupo * altoGrupo
      const yDe = (fila: number) => yDeGrupo(Math.floor(fila / 2)) + altoNombre + (fila % 2) * altoFila

      const barras: Barra[] = senales.flatMap((s, i) => [
        { clave: `${s.nombre}|p`, senal: s.nombre, medida: 'precisión' as const, valor: s.precision, detalle: s.detallePrecision, fila: 2 * i, primera: true },
        { clave: `${s.nombre}|c`, senal: s.nombre, medida: 'cobertura' as const, valor: s.cobertura, detalle: s.detalleCobertura, fila: 2 * i + 1, primera: false },
      ])

      // La escala: una línea cada 25 %.
      const marcas = [0, 0.25, 0.5, 0.75, 1]
      raiz
        .select('.escala')
        .selectAll<SVGGElement, number>('g')
        .data(marcas, (d) => d)
        .join((entra) => {
          const g = entra.append('g')
          g.append('line')
          g.append('text')
          return g
        })
        .each(function (d) {
          const g = select(this)
          g.select('line').attr('x1', x(d)).attr('x2', x(d)).attr('y1', arriba - 0.3 * rem).attr('y2', alto - entreGrupos).attr('class', 'stroke-borde')
          g.select('text')
            .attr('x', x(d))
            .attr('y', 0.9 * rem)
            .attr('text-anchor', 'middle')
            .attr('class', 'fill-tenue font-mono text-xs')
            .text(`${numero(d * 100)} %`)
        })

      raiz
        .select('.barras')
        .selectAll<SVGGElement, Barra>('g')
        .data(barras, (b) => b.clave)
        .join((entra) => {
          const g = entra.append('g')
          g.append('text').attr('class', 'nombre')
          g.append('text').attr('class', 'medida')
          g.append('rect').attr('class', 'fondo')
          g.append('rect').attr('class', 'valor')
          g.append('text').attr('class', 'cifra')
          g.append('title')
          return g
        })
        .each(function (b) {
          const g = select(this)
          const y = yDe(b.fila)
          const medio = y + altoFila / 2
          const grosor = altoFila * 0.62
          g.select('text.nombre')
            .attr('x', 0)
            .attr('y', y - altoNombre / 2)
            .attr('dominant-baseline', 'central')
            .attr('class', 'nombre fill-texto stroke-superficie text-sm font-semibold [paint-order:stroke] stroke-[0.35rem]')
            .text(b.primera ? b.senal : '')
          g.select('text.medida')
            .attr('x', izquierda - 0.6 * rem)
            .attr('y', medio)
            .attr('text-anchor', 'end')
            .attr('dominant-baseline', 'central')
            .attr('class', 'medida fill-suave text-xs')
            .text(b.medida)
          g.select('rect.fondo')
            .attr('x', x(0))
            .attr('y', medio - grosor / 2)
            .attr('width', x(1) - x(0))
            .attr('height', grosor)
            .attr('rx', 3)
            .attr('class', 'fondo fill-relieve')
          g.select('rect.valor')
            .attr('x', x(0))
            .attr('y', medio - grosor / 2)
            .attr('width', Math.max(1, x(b.valor) - x(0)))
            .attr('height', grosor)
            .attr('rx', 3)
            .attr('class', b.medida === 'precisión' ? 'valor fill-acento' : 'valor fill-acento-claro')
          g.select('text.cifra')
            .attr('x', x(1) + 0.5 * rem)
            .attr('y', medio)
            .attr('dominant-baseline', 'central')
            .attr('class', 'cifra fill-texto font-mono text-sm font-semibold')
            .text(`${numero(b.valor * 100, 1)} %`)
          g.select('title').text(`${b.senal}, ${b.medida}: ${numero(b.valor * 100, 1)} % (${b.detalle})`)
        })
    },
    [senales, ancho, rem, alto],
  )

  const descripcion = senales
    .map((s) => `${s.nombre}: precisión ${numero(s.precision * 100, 1)} % y cobertura ${numero(s.cobertura * 100, 1)} %`)
    .join('; ')
  return (
    <div ref={contenedor} className="w-full">
      <svg ref={svg} width={ancho} height={alto} role="img" aria-label={`Precisión y cobertura de cada señal. ${descripcion}.`} className="block">
        <g className="escala" />
        <g className="barras" />
      </svg>
    </div>
  )
}
