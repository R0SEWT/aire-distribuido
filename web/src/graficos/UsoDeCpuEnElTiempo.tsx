// El uso de CPU del proceso de la API en los últimos minutos (spec §5.6): un
// punto por cada par de consultas seguidas a /api/metricas. Es el coordinador,
// no los nodos.

import { area, line, scaleLinear } from 'd3'
import { useRef } from 'react'
import { numero } from '../componentes/formato'
import { useMedidas } from './medirAncho'

export interface PuntoDeCpu {
  /** Segundos desde que arrancó la API. */
  t: number
  /** Porcentaje de un núcleo: 100 es un núcleo entero. */
  porcentaje: number
}

interface Props {
  serie: readonly PuntoDeCpu[]
  /** Cuántos segundos abarca el eje. */
  ventanaS: number
}

const ALTO_REM = 6.5
/** El eje no baja de este tope: con la API quieta, un 0.3 % no tiene que parecer un pico. */
const TOPE_MINIMO = 5

function topeRedondo(maximo: number): number {
  const pasos = [5, 10, 25, 50, 100, 200, 400, 800, 1600, 3200, 6400]
  return pasos.find((p) => p >= maximo) ?? Math.ceil(maximo / 1000) * 1000
}

export function UsoDeCpuEnElTiempo({ serie, ventanaS }: Props) {
  const caja = useRef<HTMLDivElement>(null)
  const { ancho, rem } = useMedidas(caja)
  const alto = ALTO_REM * rem
  const arriba = 1.25 * rem
  const abajo = alto - 1.25 * rem
  const izquierda = 0.25 * rem
  const derecha = ancho - 0.5 * rem

  const ultimo = serie[serie.length - 1]
  const hasta = ultimo?.t ?? ventanaS
  const x = scaleLinear()
    .domain([hasta - ventanaS, hasta])
    .range([izquierda, derecha])
  const tope = topeRedondo(Math.max(TOPE_MINIMO, ...serie.map((p) => p.porcentaje)))
  const y = scaleLinear().domain([0, tope]).range([abajo, arriba])

  const visibles = serie.filter((p) => p.t >= hasta - ventanaS)
  const trazo = line<PuntoDeCpu>()
    .x((p) => x(p.t))
    .y((p) => y(p.porcentaje))(visibles)
  const relleno = area<PuntoDeCpu>()
    .x((p) => x(p.t))
    .y0(abajo)
    .y1((p) => y(p.porcentaje))(visibles)

  const minutos = Math.round(ventanaS / 60)
  const resumen = ultimo
    ? `Uso de CPU del proceso de la API en los últimos ${minutos} minutos; ahora, ${numero(ultimo.porcentaje, 1)} % de un núcleo.`
    : 'Uso de CPU del proceso de la API: todavía no hay dos muestras para calcularlo.'

  return (
    <div ref={caja} className="w-full">
      <svg width={ancho} height={alto} role="img" aria-label={resumen} data-grafico="uso-de-cpu" className="block">
        <line x1={izquierda} x2={derecha} y1={arriba} y2={arriba} className="stroke-borde" strokeWidth={1} strokeDasharray="2 4" />
        <text x={izquierda} y={arriba - 0.35 * rem} className="fill-tenue font-mono text-xs">
          {numero(tope)} % de un núcleo
        </text>
        <line x1={izquierda} x2={derecha} y1={abajo} y2={abajo} className="stroke-tenue" strokeWidth={1} />
        <text x={izquierda} y={abajo + 1 * rem} className="fill-tenue text-xs">
          hace {minutos} min
        </text>
        <text x={derecha} y={abajo + 1 * rem} textAnchor="end" className="fill-tenue text-xs">
          ahora
        </text>

        {relleno && <path d={relleno} className="fill-acento" fillOpacity={0.25} />}
        {trazo && <path d={trazo} fill="none" className="stroke-acento-claro" strokeWidth={2} strokeLinejoin="round" />}
        {ultimo && (
          <circle cx={x(ultimo.t)} cy={y(ultimo.porcentaje)} r={4.5} className="fill-acento-claro stroke-superficie" strokeWidth={2} />
        )}
        {visibles.map((p) => (
          <circle key={p.t} cx={x(p.t)} cy={y(p.porcentaje)} r={6} fill="transparent">
            <title>{`${numero(p.porcentaje, 1)} % de un núcleo`}</title>
          </circle>
        ))}
      </svg>
    </div>
  )
}
