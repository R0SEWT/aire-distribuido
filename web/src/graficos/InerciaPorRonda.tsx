// La inercia de un entrenamiento, ronda por ronda (spec §5.5): una curva que
// baja y se aplana cuando el K-Means converge. Dibuja solo las rondas ya
// mostradas: la pantalla las va sumando al ritmo de la reproducción.
//
// React arma el SVG y D3 pone las escalas y la forma de la línea. Los colores
// salen de los tokens por clases (stroke-acento-claro…): nada en línea.

import { line, scaleLinear } from 'd3'
import { compacto, milisegundos, numero } from '../componentes/formato'
import { ejeDeRondas } from './ejeDeRondas'
import { useMedida } from './medida'

export interface PuntoDeInercia {
  iter: number
  inercia: number
  ms_ronda: number
}

interface Props {
  /** Todas las rondas recibidas, en orden. */
  rondas: readonly PuntoDeInercia[]
  /** Cuántas se muestran ya. */
  mostradas: number
  /** Las iteraciones del entrenamiento, si ya se saben: fija el largo del eje. */
  total?: number
}

const ALTO_REM = 11.5

// Donde no hay con qué medir (las pruebas, en jsdom) se dibuja a este ancho.
const POR_OMISION = { ancho: 640 }

export function InerciaPorRonda({ rondas, mostradas, total }: Props) {
  const [caja, { ancho, rem }] = useMedida<HTMLDivElement>(POR_OMISION)
  const alto = ALTO_REM * rem
  const arriba = 1.5 * rem
  const abajo = alto - 2 * rem
  const eje = ejeDeRondas(Math.max(total ?? 0, rondas.length), ancho, rem)

  // La escala vertical sale de todas las rondas recibidas, no solo de las
  // mostradas: así no salta mientras la reproducción avanza. No empieza en
  // cero: la inercia baja un 30 % y después cambia en el sexto decimal.
  const valores = rondas.map((r) => r.inercia)
  const minimo = valores.length > 0 ? Math.min(...valores) : 0
  const maximo = valores.length > 0 ? Math.max(...valores) : 1
  const holgura = (maximo - minimo || Math.abs(maximo) || 1) * 0.08
  const y = scaleLinear()
    .domain([minimo - holgura, maximo + holgura])
    .range([abajo, arriba])

  const visibles = rondas.slice(0, mostradas)
  const ultima = visibles[visibles.length - 1]
  const trazo = line<PuntoDeInercia>()
    .x((r) => eje.x(r.iter))
    .y((r) => y(r.inercia))(visibles)

  // Con millones (una medición con réplicas) los decimales no dicen nada y no entran.
  const etiqueta = ultima ? numero(ultima.inercia, ultima.inercia >= 1e6 ? 0 : 1) : ''
  // Cada letra de la monoespaciada chica mide unos 0.55 rem.
  const entraALaDerecha = ultima !== undefined && eje.x(ultima.iter) + (0.6 + etiqueta.length * 0.55) * rem <= ancho

  const resumen =
    visibles.length === 0
      ? 'Inercia por ronda: todavía no hay rondas.'
      : `Inercia por ronda: de ${numero(visibles[0]!.inercia, 1)} en la ronda ${visibles[0]!.iter} a ${numero(ultima!.inercia, 1)} en la ronda ${ultima!.iter}.`

  return (
    <div ref={caja} className="w-full">
      <svg width={ancho} height={alto} role="img" aria-label={resumen} data-grafico="inercia" className="block">
        {/* La cuadrícula y el eje vertical: recesivos. */}
        {y.ticks(4).map((v) => (
          <g key={v}>
            <line x1={eje.izquierda} x2={eje.derecha} y1={y(v)} y2={y(v)} className="stroke-borde" strokeWidth={1} />
            <text x={eje.izquierda - 0.5 * rem} y={y(v)} dy="0.32em" textAnchor="end" className="fill-tenue font-mono text-xs">
              {compacto(v)}
            </text>
          </g>
        ))}
        <text x={0} y={0.8 * rem} className="fill-tenue text-xs font-semibold tracking-wider uppercase">
          Inercia
        </text>

        {/* El eje de las rondas. */}
        <line x1={eje.izquierda} x2={eje.derecha} y1={abajo} y2={abajo} className="stroke-tenue" strokeWidth={1} />
        {eje.marcas.map((r) => (
          <text key={r} x={eje.x(r)} y={abajo + 1.1 * rem} textAnchor="middle" className="fill-tenue font-mono text-xs">
            {r}
          </text>
        ))}
        <text x={eje.derecha + 0.5 * rem} y={abajo + 1.1 * rem} className="fill-tenue text-xs">
          ronda
        </text>

        {trazo && <path d={trazo} fill="none" className="stroke-acento-claro" strokeWidth={2} strokeLinejoin="round" />}
        {visibles.map((r) => (
          <circle key={r.iter} cx={eje.x(r.iter)} cy={y(r.inercia)} r={Math.min(2.5, eje.paso / 3)} className="fill-acento-claro" />
        ))}

        {ultima && (
          <g data-ronda-actual={ultima.iter}>
            <line
              x1={eje.x(ultima.iter)}
              x2={eje.x(ultima.iter)}
              y1={y(ultima.inercia)}
              y2={abajo}
              className="stroke-acento"
              strokeWidth={1}
              strokeDasharray="3 3"
            />
            <circle cx={eje.x(ultima.iter)} cy={y(ultima.inercia)} r={5} className="fill-acento-claro stroke-superficie" strokeWidth={2} />
            {/* El valor de la última ronda, a la derecha del punto; si no entra, encima y hacia la izquierda. */}
            <text
              x={eje.x(ultima.iter) + (entraALaDerecha ? 0.6 * rem : 0)}
              y={y(ultima.inercia)}
              dy={entraALaDerecha ? '-0.5em' : '-0.9em'}
              textAnchor={entraALaDerecha ? 'start' : 'end'}
              className="fill-texto font-mono text-sm font-semibold"
            >
              {etiqueta}
            </text>
          </g>
        )}

        {/* Al pasar el cursor por la columna de una ronda, sus números. */}
        {visibles.map((r) => (
          <rect key={r.iter} x={eje.x(r.iter) - eje.paso / 2} y={arriba} width={eje.paso} height={abajo - arriba} fill="transparent">
            <title>{`Ronda ${r.iter}: inercia ${numero(r.inercia, 2)} · tardó ${milisegundos(r.ms_ronda)} ms`}</title>
          </rect>
        ))}
      </svg>
    </div>
  )
}
