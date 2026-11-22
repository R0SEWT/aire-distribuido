// La curva de una estación (spec §5.3): lo que la UI vio de ella en esta sesión,
// con la línea del ECA y las bandas INCA de fondo. Dibuja dos cosas:
//
//  - la media de 24 horas (una línea con un punto por hora que la tiene): la de
//    cada estado recibido y la que la API respondió a cada lectura enviada;
//  - las lecturas horarias (rombos sueltos, sin línea), cuando esta sesión las
//    envió. Un estado recibido no trae su lectura: de esas horas solo hay media.
//
// No es una serie histórica: la API no tiene una. Si entre dos puntos de la
// media hubo otros que no llegaron, la línea los une igual; pero si entre los
// dos faltan más de unas horas (tramos.ts), se corta: son tramos distintos, y la
// media de 24 horas no es continua de uno al otro.
//
// D3 hace las escalas y el trazo; React dibuja el SVG, con clases y sin estilos
// en línea. El color no va solo: cada franja lleva su palabra, el ECA su
// rótulo, y un punto sobre el ECA está, además, por encima de su línea.

import { line, scaleLinear, scaleTime } from 'd3'
import { useMemo } from 'react'
import { diaCorto, hora, numero } from '../componentes/formato'
import { useMedida } from './medida'
import { partirPorHuecos } from './tramos'

export interface PuntoCurva {
  /** La hora de la lectura a la que corresponde el estado, en milisegundos. */
  t: number
  /** La media de 24 horas; null si la ventana tenía menos de 18 lecturas. */
  valor: number | null
}

/** Una lectura horaria: lo que midió la estación en esa hora, en µg/m³. */
export interface LecturaCurva {
  t: number
  valor: number
}

interface Props {
  /** La media de 24 horas, de la hora más antigua a la más nueva. */
  puntos: readonly PuntoCurva[]
  /** Las lecturas horarias que se conocen, en el mismo orden. Sin ellas se dibuja solo la media. */
  lecturas?: readonly LecturaCurva[]
  /** ECA de 24 horas del contaminante, en µg/m³. */
  eca: number
  /** Hasta dónde llegan las bandas buena, moderada y mala del INCA. */
  cortes: readonly [number, number, number]
  /** Qué muestra, para los lectores de pantalla. */
  titulo: string
  className?: string
}

const POR_OMISION = { ancho: 720, alto: 300 }
const HORA = 3_600_000

const FRANJAS = [
  { banda: 'buena', palabra: 'Buena', relleno: 'fill-inca-buena/15', marca: 'fill-inca-buena' },
  { banda: 'moderada', palabra: 'Moderada', relleno: 'fill-inca-moderada/15', marca: 'fill-inca-moderada' },
  { banda: 'mala', palabra: 'Mala', relleno: 'fill-inca-mala/15', marca: 'fill-inca-mala' },
  { banda: 'umbral_de_cuidado', palabra: 'Umbral de cuidado', relleno: 'fill-inca-cuidado/20', marca: 'fill-inca-cuidado' },
] as const

const SIN_LECTURAS: readonly LecturaCurva[] = []

export function CurvaSesion({ puntos, lecturas = SIN_LECTURAS, eca, cortes, titulo, className = '' }: Props) {
  const [caja, medida] = useMedida<HTMLDivElement>(POR_OMISION)
  const { ancho, alto, escala: k } = medida

  const dibujo = useMemo(() => {
    // A la derecha va el nombre de cada banda: «Umbral de cuidado» es el más largo.
    const margen = { arriba: 30 * k, derecha: 140 * k, abajo: 40 * k, izquierda: 44 * k }
    const conMedia = puntos.filter((p): p is { t: number; valor: number } => p.valor !== null)
    const instantes = [...puntos, ...lecturas].map((p) => p.t)
    const hayAlgo = instantes.length > 0

    // El eje llega, como mínimo, un poco más arriba del ECA: la línea siempre se ve.
    // Una lectura horaria puede ser mucho más alta que la media: también tiene que entrar.
    const tope = Math.max(eca * 1.25, ...conMedia.map((p) => p.valor * 1.15), ...lecturas.map((l) => l.valor * 1.05))
    const y = scaleLinear().domain([0, tope]).nice(5).range([alto - margen.abajo, margen.arriba])
    const [, yMax = tope] = y.domain()

    // Con pocas horas recibidas se muestra un día entero, que es lo que promedia la media.
    const hasta = hayAlgo ? Math.max(...instantes) : 0
    const desde = Math.min(hayAlgo ? Math.min(...instantes) : 0, hasta - 23 * HORA)
    const x = scaleTime()
      .domain([desde, hasta])
      .range([margen.izquierda + 6 * k, ancho - margen.derecha - 10 * k])

    const limites = [0, ...cortes, Infinity]
    const franjas = FRANJAS.map((f, i) => {
      const abajo = limites[i]!
      const arriba = Math.min(limites[i + 1]!, yMax)
      return abajo < yMax ? { ...f, y0: y(arriba), y1: y(abajo) } : null
    }).filter((f) => f !== null)

    const marcasX = x.ticks(Math.max(2, Math.floor((ancho - margen.izquierda - margen.derecha) / (92 * k)))).map((fecha, i, todas) => {
      const dia = diaCorto(fecha)
      const anterior = todas[i - 1]
      return { t: fecha.getTime(), x: x(fecha), hora: hora(fecha), dia: !anterior || diaCorto(anterior) !== dia ? dia : null }
    })

    // La línea une solo lo que es continuo: se corta donde no hubo media y donde
    // faltan horas (tramos.ts). Un tramo de un solo punto queda como su punto.
    const trazo = line<{ t: number; valor: number }>()
      .x((p) => x(p.t))
      .y((p) => y(p.valor))

    return {
      margen,
      franjas,
      marcasY: y.ticks(5).map((v) => ({ v, y: y(v) })),
      marcasX,
      yEca: y(eca),
      caminos: partirPorHuecos(puntos).map((tramo) => ({ desde: tramo[0]!.t, d: trazo(tramo) ?? '' })),
      puntos: puntos.map((p) => ({ ...p, x: x(p.t), y: y(p.valor ?? 0), sobre: p.valor !== null && p.valor > eca })),
      lecturas: lecturas.map((l) => ({ ...l, x: x(l.t), y: y(l.valor) })),
      base: y(0),
    }
  }, [puntos, lecturas, eca, cortes, ancho, alto, k])

  const { margen } = dibujo
  const derecha = ancho - margen.derecha
  const final = dibujo.puntos[dibujo.puntos.length - 1]
  // Con muchos estados los puntos se achican para no taparse.
  const radio = (dibujo.puntos.length > 60 ? 1.75 : 2.75) * k
  // Una lectura es un rombo suelto: se distingue de la media por la forma y porque no la une ninguna línea.
  const lado = (dibujo.lecturas.length > 60 ? 2.5 : 3.25) * k
  const hayLecturas = dibujo.lecturas.length > 0

  return (
    <div ref={caja} className={`relative overflow-hidden ${className}`}>
      <svg width={ancho} height={alto} viewBox={`0 0 ${ancho} ${alto}`} role="img" aria-label={titulo} className="absolute inset-0 block">
        <g data-capa="bandas">
          {dibujo.franjas.map((f) => (
            <g key={f.banda} data-franja={f.banda}>
              <rect x={margen.izquierda} y={f.y0} width={derecha - margen.izquierda} height={Math.max(0, f.y1 - f.y0)} className={f.relleno} />
              {f.y1 - f.y0 >= 15 * k && (
                <>
                  <rect x={derecha + 8 * k} y={(f.y0 + f.y1) / 2 - 4 * k} width={8 * k} height={8 * k} rx={2 * k} className={f.marca} />
                  <text x={derecha + 22 * k} y={(f.y0 + f.y1) / 2} dominantBaseline="central" className="fill-suave text-xs">
                    {f.palabra}
                  </text>
                </>
              )}
            </g>
          ))}
        </g>

        <g data-capa="ejes" className="text-xs">
          {dibujo.marcasY.map((m) => (
            <g key={m.v}>
              <line x1={margen.izquierda} x2={derecha} y1={m.y} y2={m.y} className="stroke-borde" strokeWidth={m.v === 0 ? 1.25 : 0.75} />
              <text x={margen.izquierda - 8 * k} y={m.y} textAnchor="end" dominantBaseline="central" className="fill-tenue font-mono tabular-nums">
                {numero(m.v)}
              </text>
            </g>
          ))}
          <text x={margen.izquierda - 8 * k} y={9 * k} textAnchor="end" dominantBaseline="central" className="fill-tenue">
            µg/m³
          </text>
          {dibujo.marcasX.map((m) => (
            <g key={m.t}>
              <line x1={m.x} x2={m.x} y1={dibujo.base} y2={dibujo.base + 4 * k} className="stroke-tenue" strokeWidth={1} />
              <text x={m.x} y={dibujo.base + 14 * k} textAnchor="middle" dominantBaseline="central" className="fill-suave font-mono tabular-nums">
                {m.hora}
              </text>
              {m.dia && (
                <text x={m.x} y={dibujo.base + 29 * k} textAnchor="middle" dominantBaseline="central" className="fill-tenue">
                  {m.dia}
                </text>
              )}
            </g>
          ))}
        </g>

        <g data-capa="eca">
          <line
            x1={margen.izquierda}
            x2={derecha}
            y1={dibujo.yEca}
            y2={dibujo.yEca}
            className="stroke-falla"
            strokeWidth={1.5}
            strokeDasharray={`${7 * k} ${4 * k}`}
          />
          <text
            x={margen.izquierda + 8 * k}
            y={dibujo.yEca - 9 * k}
            dominantBaseline="central"
            className="fill-falla stroke-superficie text-xs font-semibold [paint-order:stroke]"
            strokeWidth={3 * k}
            strokeLinejoin="round"
          >
            ECA de 24 h: {numero(eca)} µg/m³
          </text>
        </g>

        {hayLecturas && (
          <g data-capa="lecturas">
            {dibujo.lecturas.map((l) => (
              <path
                key={l.t}
                data-lectura
                d={`M${l.x} ${l.y - lado}L${l.x + lado} ${l.y}L${l.x} ${l.y + lado}L${l.x - lado} ${l.y}Z`}
                className="fill-acento stroke-acento-claro"
                strokeWidth={0.75}
              >
                <title>{`${diaCorto(l.t)}, ${hora(l.t)}: lectura de ${numero(l.valor, 1)} µg/m³`}</title>
              </path>
            ))}
          </g>
        )}

        {hayLecturas && (
          <g data-capa="leyenda" className="text-xs">
            <path
              d={`M${margen.izquierda + 12 * k} ${5 * k}l${4 * k} ${4 * k}l${-4 * k} ${4 * k}l${-4 * k} ${-4 * k}Z`}
              className="fill-acento stroke-acento-claro"
              strokeWidth={0.75}
            />
            <text x={margen.izquierda + 22 * k} y={9 * k} dominantBaseline="central" className="fill-suave">
              Lectura horaria
            </text>
            <line x1={margen.izquierda + 122 * k} x2={margen.izquierda + 142 * k} y1={9 * k} y2={9 * k} className="stroke-texto" strokeWidth={2 * k} strokeLinecap="round" />
            <circle cx={margen.izquierda + 132 * k} cy={9 * k} r={2.75 * k} className="fill-texto" />
            <text x={margen.izquierda + 149 * k} y={9 * k} dominantBaseline="central" className="fill-suave">
              Media de 24 h
            </text>
          </g>
        )}

        <g data-capa="curva">
          {dibujo.caminos.map((c) => (
            <path key={c.desde} data-tramo d={c.d} className="stroke-texto fill-none" strokeWidth={2 * k} strokeLinejoin="round" strokeLinecap="round" />
          ))}
          {dibujo.puntos.map((p) =>
            p.valor === null ? (
              <circle key={p.t} data-punto="sin-media" cx={p.x} cy={dibujo.base} r={radio} className="fill-superficie stroke-sin-datos" strokeWidth={1.25} />
            ) : (
              <circle key={p.t} data-punto={p.sobre ? 'sobre-eca' : 'media'} cx={p.x} cy={p.y} r={radio} className={p.sobre ? 'fill-falla' : 'fill-texto'} />
            ),
          )}
          {final && final.valor !== null && (
            <g data-capa="ultimo">
              <circle cx={final.x} cy={final.y} r={5 * k} className={`stroke-superficie ${final.sobre ? 'fill-falla' : 'fill-texto'}`} strokeWidth={2 * k} />
              <text
                x={final.x + 5 * k}
                y={final.y - 15 * k}
                textAnchor="end"
                dominantBaseline="central"
                className="fill-texto stroke-superficie font-mono text-sm font-semibold tabular-nums [paint-order:stroke]"
                strokeWidth={4 * k}
                strokeLinejoin="round"
              >
                {numero(final.valor, 1)}
              </text>
            </g>
          )}
        </g>
      </svg>
    </div>
  )
}
