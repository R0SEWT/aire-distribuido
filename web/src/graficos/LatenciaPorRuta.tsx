// La latencia de una ruta de la API como una marca sobre un eje común (spec
// §5.6): el tramo de p50 a p99, con p95 en el medio, y la línea de los 100 ms.
//
// La escala es logarítmica: una ruta responde en 0.1 ms y otra en 50 ms, y en
// una escala lineal la primera no se vería. Cada fila de la tabla dibuja su
// marca con la misma escala, y el encabezado, el eje: por eso el tamaño es fijo
// (en rem, vía viewBox) y no depende del ancho de la tarjeta.

import { scaleLog } from 'd3'
import { milisegundos } from '../componentes/formato'

/** El objetivo de la PC4: la predicción responde en menos de 100 ms. */
export const OBJETIVO_MS = 100

// En unidades del viewBox: 16 por rem. La clase le da el tamaño real, en rem.
const ANCHO = 288
const ALTO = 24
const MARGEN = 10
const CLASE = 'block h-6 w-72 normal-case tracking-normal'

const DESDE_MS = 0.01
const HASTA_MS = 10_000
const DECADAS = [0.01, 0.1, 1, 10, 100, 1000, 10_000]

const x = scaleLog()
  .domain([DESDE_MS, HASTA_MS])
  .range([MARGEN, ANCHO - MARGEN])
  .clamp(true)

function posicion(ms: number): number {
  return x(Math.max(ms, DESDE_MS))
}

function Fondo() {
  return (
    <>
      {DECADAS.map((d) => (
        <line key={d} x1={x(d)} x2={x(d)} y1={0} y2={ALTO} className="stroke-borde" strokeWidth={1} />
      ))}
      <line
        x1={x(OBJETIVO_MS)}
        x2={x(OBJETIVO_MS)}
        y1={0}
        y2={ALTO}
        className="stroke-aviso"
        strokeWidth={1.5}
        strokeDasharray="3 2"
      />
    </>
  )
}

/** El eje, para el encabezado de la columna: las décadas con su rótulo y la línea de los 100 ms. */
export function EjeDeLatencia() {
  return (
    <svg viewBox={`0 0 ${ANCHO} ${ALTO}`} className={CLASE} role="img" aria-label="Escala logarítmica de 0.01 ms a 10 s; la línea punteada marca los 100 ms">
      {[0.1, 1, 10, 1000].map((d) => (
        <text key={d} x={x(d)} y={15} textAnchor="middle" fontSize={11} className="fill-tenue font-mono">
          {d >= 1000 ? '1 s' : d}
        </text>
      ))}
      <text x={x(OBJETIVO_MS)} y={15} textAnchor="middle" fontSize={11} className="fill-aviso font-mono font-semibold">
        100 ms
      </text>
      {DECADAS.map((d) => (
        <line key={d} x1={x(d)} x2={x(d)} y1={19} y2={ALTO} className={d === OBJETIVO_MS ? 'stroke-aviso' : 'stroke-borde'} strokeWidth={1} />
      ))}
    </svg>
  )
}

interface Props {
  p50: number
  p95: number
  p99: number
}

/** La marca de una ruta: un tramo de p50 a p99, p50 hueco, p95 lleno y p99 en rombo. */
export function MarcaDeLatencia({ p50, p95, p99 }: Props) {
  const medio = ALTO / 2
  const sobre = p99 >= OBJETIVO_MS
  const x99 = posicion(p99)
  return (
    <svg
      viewBox={`0 0 ${ANCHO} ${ALTO}`}
      className={CLASE}
      role="img"
      data-sobre-objetivo={sobre}
      aria-label={`p50 ${milisegundos(p50)} ms, p95 ${milisegundos(p95)} ms, p99 ${milisegundos(p99)} ms${sobre ? ': p99 sobre los 100 ms' : ''}`}
    >
      <Fondo />
      <line x1={posicion(p50)} x2={x99} y1={medio} y2={medio} className="stroke-acento" strokeWidth={3} strokeLinecap="round" />
      <circle cx={posicion(p50)} cy={medio} r={4} className="fill-superficie stroke-acento-claro" strokeWidth={2}>
        <title>{`p50: ${milisegundos(p50)} ms`}</title>
      </circle>
      <circle cx={posicion(p95)} cy={medio} r={4} className="fill-acento-claro stroke-superficie" strokeWidth={1.5}>
        <title>{`p95: ${milisegundos(p95)} ms`}</title>
      </circle>
      <path
        d={`M ${x99 - 5.5} ${medio} L ${x99} ${medio - 5.5} L ${x99 + 5.5} ${medio} L ${x99} ${medio + 5.5} Z`}
        className={sobre ? 'fill-falla stroke-superficie' : 'fill-texto stroke-superficie'}
        strokeWidth={1.5}
      >
        <title>{`p99: ${milisegundos(p99)} ms`}</title>
      </path>
    </svg>
  )
}

/** La leyenda de las tres marcas, para poner bajo la tabla. */
export function LeyendaDeLatencia() {
  return (
    <span className="text-suave inline-flex flex-wrap items-center gap-x-4 gap-y-1 text-xs">
      <span className="inline-flex items-center gap-1.5">
        <svg viewBox="0 0 12 12" className="size-3" aria-hidden="true">
          <circle cx={6} cy={6} r={4} className="fill-superficie stroke-acento-claro" strokeWidth={2} />
        </svg>
        p50 (la mitad de los pedidos)
      </span>
      <span className="inline-flex items-center gap-1.5">
        <svg viewBox="0 0 12 12" className="size-3" aria-hidden="true">
          <circle cx={6} cy={6} r={4.5} className="fill-acento-claro" />
        </svg>
        p95
      </span>
      <span className="inline-flex items-center gap-1.5">
        <svg viewBox="0 0 12 12" className="size-3" aria-hidden="true">
          <path d="M 0.5 6 L 6 0.5 L 11.5 6 L 6 11.5 Z" className="fill-texto" />
        </svg>
        p99
      </span>
      <span className="inline-flex items-center gap-1.5">
        <svg viewBox="0 0 12 12" className="size-3" aria-hidden="true">
          <line x1={6} x2={6} y1={0} y2={12} className="stroke-aviso" strokeWidth={1.5} strokeDasharray="3 2" />
        </svg>
        100 ms, el objetivo de la predicción
      </span>
    </span>
  )
}
