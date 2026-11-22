// La rejilla del escenario: una fila por estación y una columna por hora. Cada
// celda es una lectura, y se va llenando a medida que la API responde: primero
// «enviada, todavía sin media» (un punto) y, desde la lectura 18 de la ventana,
// el color de la banda INCA de la media de 24 h, con un anillo si hay alerta.
//
// El color no va solo: a la derecha de cada fila va, en palabras, el estado que
// respondió la API a su última lectura, y cada celda lleva su descripción.

import { select } from 'd3'
import { BANDAS, estadoDeBanda } from '../componentes/bandas'
import { fechaHora, numero } from '../componentes/formato'
import { diaCorto, horaDeLima } from '../pantallas/simulacion/fechas'
import type { Punto } from '../pantallas/simulacion/vistas'
import { useDibujo } from './dibujo'
import { useMedida } from './medida'

const HORA_MS = 3_600_000

export interface FilaDeRejilla {
  estacion: string
  /** Las horas (instantes) en que el archivo trae una lectura de la estación: las demás son huecos. */
  programadas: ReadonlySet<number>
  /** Las lecturas enviadas, con lo que la API respondió. */
  puntos: readonly Punto[]
}

interface Props {
  /** Los instantes de cada columna, de la primera hora a la última. */
  horas: readonly number[]
  filas: readonly FilaDeRejilla[]
  /** Cuántas lecturas hacen falta en la ventana para que haya media. */
  minimo: number
  /** La hora de la última lectura enviada: la columna que se está llenando. */
  cursor: number | null
  elegida?: string
  alElegir?: (estacion: string) => void
}

interface Celda {
  clave: string
  fila: number
  columna: number
  estacion: string
  hora: number
  programada: boolean
  punto: Punto | undefined
}

function palabras(punto: Punto | undefined, minimo: number): string {
  if (!punto) return 'por enviar'
  if (punto.inca === null) return `${punto.lecturas} de ${minimo} lecturas`
  return punto.alerta ? `${BANDAS[punto.inca].palabra} · en alerta` : BANDAS[punto.inca].palabra
}

function descripcion(c: Celda, minimo: number): string {
  const cuando = fechaHora(c.hora)
  if (!c.programada) return `${c.estacion} · ${cuando}: sin lectura en la fuente`
  if (!c.punto) return `${c.estacion} · ${cuando}: por enviar`
  const p = c.punto
  const medida = `PM2.5 ${numero(p.pm2_5, 1)} y PM10 ${numero(p.pm10, 1)} µg/m³`
  if (p.inca === null) return `${c.estacion} · ${cuando}: ${medida}. Ventana con ${p.lecturas} lecturas: sin media (hacen falta ${minimo})`
  return `${c.estacion} · ${cuando}: ${medida}. Media de 24 h: PM2.5 ${numero(p.media_pm2_5, 1)} y PM10 ${numero(p.media_pm10, 1)}. ${palabras(p, minimo)}`
}

// Donde no hay con qué medir (las pruebas, en jsdom) se dibuja a este ancho.
const POR_OMISION = { ancho: 760 }

export function RejillaEscenario({ horas, filas, minimo, cursor, elegida, alElegir }: Props) {
  const [contenedor, { ancho, rem }] = useMedida<HTMLDivElement>(POR_OMISION)

  const izquierda = 6.25 * rem
  const derecha = 9.25 * rem
  const arriba = 2.3 * rem
  const altoFila = 1.1 * rem
  const paso = Math.max(4, (ancho - izquierda - derecha) / Math.max(1, horas.length))
  const lado = Math.max(3, Math.min(paso - 2, altoFila - 4))
  const alto = arriba + filas.length * altoFila + 0.4 * rem

  const svg = useDibujo<SVGSVGElement>(
    (nodo) => {
      const raiz = select(nodo)
      const x = (columna: number) => izquierda + columna * paso
      const y = (fila: number) => arriba + fila * altoFila

      const celdas: Celda[] = filas.flatMap((f, fila) => {
        const porHora = new Map(f.puntos.map((p) => [Date.parse(p.fecha_hora), p]))
        return horas.map((hora, columna) => ({
          clave: `${f.estacion}|${hora}`,
          fila,
          columna,
          estacion: f.estacion,
          hora,
          programada: f.programadas.has(hora),
          punto: porHora.get(hora),
        }))
      })

      // El fondo de la fila elegida y de cada fila al pasar por encima.
      raiz
        .select('.filas')
        .selectAll<SVGRectElement, FilaDeRejilla>('rect')
        .data(filas, (f) => f.estacion)
        .join('rect')
        .attr('x', 0)
        .attr('y', (_, i) => y(i))
        .attr('width', ancho)
        .attr('height', altoFila)
        .attr('rx', 3)
        .attr('class', (f) => (f.estacion === elegida ? 'fill-relieve' : 'fill-transparent'))

      // El eje: el día arriba y las horas debajo. Una lectura lleva la hora en
      // que termina su promedio, así que el día de una columna es el de la hora anterior.
      const dias = new Map<string, { desde: number; hasta: number }>()
      horas.forEach((hora, columna) => {
        const dia = diaCorto(hora - HORA_MS)
        const d = dias.get(dia)
        if (d) d.hasta = columna
        else dias.set(dia, { desde: columna, hasta: columna })
      })
      raiz
        .select('.dias')
        .selectAll<SVGTextElement, [string, { desde: number; hasta: number }]>('text')
        .data([...dias], (d) => d[0])
        .join('text')
        .attr('x', (d) => x(d[1].desde))
        .attr('y', 0.8 * rem)
        .attr('class', 'fill-suave text-xs font-semibold')
        .text((d) => d[0])
      raiz
        .select('.separadores')
        .selectAll<SVGLineElement, { desde: number }>('line')
        .data([...dias.values()].slice(1))
        .join('line')
        .attr('x1', (d) => x(d.desde) - 1)
        .attr('x2', (d) => x(d.desde) - 1)
        .attr('y1', 0.2 * rem)
        .attr('y2', alto)
        .attr('class', 'stroke-borde')

      const marcas = horas.map((hora, columna) => ({ hora, columna })).filter((m) => horaDeLima(m.hora) % 6 === 0)
      raiz
        .select('.horas')
        .selectAll<SVGTextElement, { hora: number; columna: number }>('text')
        .data(marcas, (m) => m.hora)
        .join('text')
        .attr('x', (m) => x(m.columna) + paso / 2)
        .attr('y', 1.9 * rem)
        .attr('text-anchor', 'middle')
        .attr('class', 'fill-tenue font-mono text-xs')
        .text((m) => `${String(horaDeLima(m.hora)).padStart(2, '0')}h`)

      // La columna que se está llenando.
      const columnaCursor = cursor === null ? -1 : horas.indexOf(cursor)
      raiz
        .select('.cursor')
        .selectAll<SVGRectElement, number>('rect')
        .data(columnaCursor >= 0 ? [columnaCursor] : [])
        .join('rect')
        .attr('x', (c) => x(c))
        .attr('y', arriba - 2)
        .attr('width', paso)
        .attr('height', filas.length * altoFila + 4)
        .attr('rx', 2)
        .attr('class', 'fill-acento/15 stroke-acento-claro')

      // Las celdas.
      const centro = (c: Celda) => ({ cx: x(c.columna) + paso / 2, cy: y(c.fila) + altoFila / 2 })
      const conMedia = (c: Celda) => c.punto !== undefined && c.punto.inca !== null
      raiz
        .select('.celdas')
        .selectAll<SVGRectElement, Celda>('rect')
        .data(celdas, (c) => c.clave)
        .join((entra) => {
          const r = entra.append('rect')
          r.append('title')
          return r
        })
        .each(function (c) {
          const rect = select(this)
          // Sin lectura en la fuente: una raya. Enviada sin media: un punto. Con media: la celda entera.
          const tamano = !c.programada ? { w: lado * 0.5, h: 1.5 } : c.punto && !conMedia(c) ? { w: lado * 0.42, h: lado * 0.42 } : { w: lado, h: lado }
          const { cx, cy } = centro(c)
          rect
            .attr('x', cx - tamano.w / 2)
            .attr('y', cy - tamano.h / 2)
            .attr('width', tamano.w)
            .attr('height', tamano.h)
            .attr('rx', c.programada && c.punto && !conMedia(c) ? tamano.w / 2 : 1.5)
            .attr(
              'class',
              !c.programada
                ? 'fill-borde-control'
                : !c.punto
                  ? 'fill-transparent stroke-borde-control/60'
                  : !conMedia(c)
                    ? 'fill-sin-datos'
                    : c.punto.alerta
                      ? 'stroke-texto stroke-2'
                      : '',
            )
            .style('fill', () => (conMedia(c) ? BANDAS[estadoDeBanda(c.punto!.inca)].color : null))
          rect.select('title').text(descripcion(c, minimo))
        })

      // El código de la estación a la izquierda y, a la derecha, su último estado en palabras.
      raiz
        .select('.codigos')
        .selectAll<SVGTextElement, FilaDeRejilla>('text')
        .data(filas, (f) => f.estacion)
        .join('text')
        .attr('x', 0.4 * rem)
        .attr('y', (_, i) => y(i) + altoFila / 2)
        .attr('dominant-baseline', 'central')
        .attr('class', (f) => `font-mono text-xs ${f.estacion === elegida ? 'fill-texto font-semibold' : 'fill-suave'}`)
        .text((f) => f.estacion)
      raiz
        .select('.estados')
        .selectAll<SVGTextElement, FilaDeRejilla>('text')
        .data(filas, (f) => f.estacion)
        .join('text')
        .attr('x', izquierda + horas.length * paso + 0.6 * rem)
        .attr('y', (_, i) => y(i) + altoFila / 2)
        .attr('dominant-baseline', 'central')
        .attr('class', (f) => {
          const ultimo = f.puntos[f.puntos.length - 1]
          return `text-xs ${ultimo?.alerta ? 'fill-falla font-semibold' : ultimo?.inca ? 'fill-texto' : 'fill-tenue'}`
        })
        .text((f) => palabras(f.puntos[f.puntos.length - 1], minimo))

      // Encima de todo, lo que recibe el clic de cada fila.
      raiz
        .select('.toques')
        .selectAll<SVGRectElement, FilaDeRejilla>('rect')
        .data(alElegir ? filas : [], (f) => f.estacion)
        .join('rect')
        .attr('x', 0)
        .attr('y', (_, i) => y(i))
        .attr('width', izquierda)
        .attr('height', altoFila)
        .attr('class', 'cursor-pointer fill-transparent')
        .on('click', (_, f) => alElegir?.(f.estacion))
    },
    [horas, filas, minimo, cursor, elegida, alElegir, ancho, rem],
  )

  const conMedia = filas.filter((f) => f.puntos[f.puntos.length - 1]?.inca).length
  const enAlerta = filas.filter((f) => f.puntos[f.puntos.length - 1]?.alerta).length
  return (
    <div ref={contenedor} className="w-full">
      <svg
        ref={svg}
        width={ancho}
        height={alto}
        role="img"
        aria-label={`Rejilla del escenario, estación por estación y hora por hora (hora de Lima): ${filas.length} estaciones por ${horas.length} horas. ${conMedia} con media de 24 horas y ${enAlerta} en alerta.`}
        className="block"
      >
        <g className="filas" />
        <g className="separadores" />
        <g className="dias" />
        <g className="horas" />
        <g className="cursor" />
        <g className="celdas" />
        <g className="codigos" />
        <g className="estados" />
        <g className="toques" />
      </svg>
    </div>
  )
}
