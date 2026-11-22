// Lo que tardó cada nodo en responder cada ronda (spec §5.5): una fila por
// nodo y, en cada fila, una barra por ronda. Todas las filas comparten la
// escala, para que un nodo más lento se vea más alto.
//
// Es el tiempo medido desde el coordinador (ms_por_nodo), que incluye la red:
// no es el CPU del nodo, que el protocolo no transporta.

import { scaleLinear } from 'd3'
import { milisegundos } from '../componentes/formato'
import { ejeDeRondas } from './ejeDeRondas'
import { useMedida } from './medida'

export interface RondaConTiempos {
  iter: number
  ms_por_nodo: Record<string, number>
}

interface Props {
  /** Los nodos del entrenamiento, en el orden en que se muestran. */
  nodos: readonly string[]
  /** Todas las rondas recibidas, en orden. */
  rondas: readonly RondaConTiempos[]
  /** Cuántas se muestran ya. */
  mostradas: number
  /** Las iteraciones del entrenamiento, si ya se saben: fija el largo del eje. */
  total?: number
}

const FILA_REM = 2.75
const ENTRE_FILAS_REM = 0.75
const LETRAS_DEL_NOMBRE = 13

function recortar(nombre: string): string {
  return nombre.length <= LETRAS_DEL_NOMBRE ? nombre : `${nombre.slice(0, LETRAS_DEL_NOMBRE - 1)}…`
}

// Donde no hay con qué medir (las pruebas, en jsdom) se dibuja a este ancho.
const POR_OMISION = { ancho: 640 }

export function TiemposPorNodo({ nodos, rondas, mostradas, total }: Props) {
  const [caja, { ancho, rem }] = useMedida<HTMLDivElement>(POR_OMISION)
  const eje = ejeDeRondas(Math.max(total ?? 0, rondas.length), ancho, rem)

  const fila = FILA_REM * rem
  const arriba = 0.5 * rem
  const abajo = arriba + nodos.length * fila + Math.max(0, nodos.length - 1) * ENTRE_FILAS_REM * rem
  const alto = abajo + 2 * rem

  // El tope de la escala es el mayor tiempo de cualquier nodo en cualquier
  // ronda recibida: el mismo para todas las filas, y no salta al reproducir.
  const tope = Math.max(0, ...rondas.flatMap((r) => nodos.map((n) => r.ms_por_nodo[n] ?? 0)))
  const y = scaleLinear()
    .domain([0, tope || 1])
    .range([0, fila - 0.25 * rem])

  const visibles = rondas.slice(0, mostradas)
  const ultima = visibles[visibles.length - 1]
  const barra = Math.max(1, Math.min(eje.paso - 2, eje.paso * 0.72))

  const resumen =
    ultima === undefined
      ? 'Tiempo de respuesta de cada nodo por ronda: todavía no hay rondas.'
      : `Tiempo de respuesta de cada nodo en la ronda ${ultima.iter}: ` +
        nodos.map((n) => `${n}, ${milisegundos(ultima.ms_por_nodo[n] ?? 0)} ms`).join('; ') +
        '.'

  return (
    <div ref={caja} className="w-full">
      <svg width={ancho} height={alto} role="img" aria-label={resumen} data-grafico="tiempos-por-nodo" className="block">
        {nodos.map((nodo, i) => {
          const base = arriba + i * (fila + ENTRE_FILAS_REM * rem) + fila
          const tiempos = visibles.map((r) => r.ms_por_nodo[nodo]).filter((v): v is number => v !== undefined)
          const media = tiempos.length > 0 ? tiempos.reduce((a, b) => a + b, 0) / tiempos.length : null
          const ahora = ultima?.ms_por_nodo[nodo]
          return (
            <g key={nodo} data-nodo={nodo}>
              <text x={0} y={base - fila / 2} dy="0.32em" className="fill-texto font-mono text-sm font-semibold">
                {recortar(nodo)}
                <title>{nodo}</title>
              </text>
              <line x1={eje.izquierda} x2={eje.derecha} y1={base} y2={base} className="stroke-borde" strokeWidth={1} />
              {visibles.map((r) => {
                const ms = r.ms_por_nodo[nodo]
                if (ms === undefined) return null
                const altura = Math.max(1, y(ms))
                return (
                  <rect
                    key={r.iter}
                    x={eje.x(r.iter) - barra / 2}
                    y={base - altura}
                    width={barra}
                    height={altura}
                    rx={Math.min(2, barra / 2)}
                    className={r === ultima ? 'fill-acento-claro' : 'fill-acento'}
                  >
                    <title>{`${nodo} · ronda ${r.iter}: ${milisegundos(ms)} ms`}</title>
                  </rect>
                )
              })}
              {/* A la derecha, lo que tardó en la última ronda mostrada y su media hasta ahí. */}
              <text x={ancho} y={base - fila / 2} dy="-0.15em" textAnchor="end" className="fill-texto font-mono text-sm font-semibold">
                {ahora === undefined ? '—' : `${milisegundos(ahora)} ms`}
              </text>
              <text x={ancho} y={base - fila / 2} dy="1.05em" textAnchor="end" className="fill-tenue font-mono text-xs">
                {media === null ? '' : `media ${milisegundos(media)}`}
              </text>
            </g>
          )
        })}

        {eje.marcas.map((r) => (
          <text key={r} x={eje.x(r)} y={abajo + 1.1 * rem} textAnchor="middle" className="fill-tenue font-mono text-xs">
            {r}
          </text>
        ))}
        <text x={eje.derecha + 0.5 * rem} y={abajo + 1.1 * rem} className="fill-tenue text-xs">
          ronda
        </text>
      </svg>
    </div>
  )
}
