// La curva de una estación durante una simulación: las lecturas horarias que se
// le mandaron a la API (barras) y la media de 24 h que la API respondió a cada
// una (línea), contra el ECA y con las bandas INCA de fondo. Se ve cómo la media
// aparece recién con 18 lecturas en la ventana y en qué hora se enciende la alerta.

import { line, max, scaleLinear, select } from 'd3'
import { BANDAS, type EstadoBanda } from '../componentes/bandas'
import { diaCorto, hora as horaEnLima, horaDeLima, numero } from '../componentes/formato'
import type { Lectura } from '../pantallas/simulacion/series'
import type { Punto } from '../pantallas/simulacion/vistas'
import { useDibujo } from './dibujo'
import { useMedida } from './medida'

const HORA_MS = 3_600_000
const ORDEN: readonly EstadoBanda[] = ['buena', 'moderada', 'mala', 'umbral_de_cuidado']

// Un rótulo que va encima de las barras lleva un borde del color del fondo, para que se lea.
const HALO = 'stroke-fondo stroke-[0.3rem] [paint-order:stroke] [stroke-linejoin:round]'

export type Contaminante = 'pm2_5' | 'pm10'

interface Props {
  estacion: string
  /** Los instantes de cada hora del tramo. */
  horas: readonly number[]
  /** Todas las lecturas de la estación en el tramo: las que faltan mandar se dibujan apagadas. */
  programadas: readonly Lectura[]
  /** Las ya enviadas, con lo que respondió la API. */
  puntos: readonly Punto[]
  contaminante: Contaminante
  /** El ECA de 24 h del contaminante y los cortes de sus bandas INCA, en µg/m³. */
  eca: number
  cortes: readonly number[]
  /** La hora de la primera lectura con media y de la primera con alerta, según lo que respondió la API. */
  primeraMedia: string | null
  primeraAlerta: string | null
  /** El alto del gráfico, en rem. */
  alto?: number
}

interface Barra {
  t: number
  valor: number
  enviada: boolean
}

interface Marca {
  t: number
  texto: string
  alerta: boolean
}

// Donde no hay con qué medir (las pruebas, en jsdom) se dibuja a este ancho.
const POR_OMISION = { ancho: 760 }

export function CurvaSimulacion({ estacion, horas, programadas, puntos, contaminante, eca, cortes, primeraMedia, primeraAlerta, alto: altoEnRem = 15.5 }: Props) {
  const [contenedor, { ancho, rem }] = useMedida<HTMLDivElement>(POR_OMISION)
  const alto = altoEnRem * rem
  const nombre = contaminante === 'pm2_5' ? 'PM2.5' : 'PM10'

  const svg = useDibujo<SVGSVGElement>(
    (nodo) => {
      const raiz = select(nodo)
      const margen = { arriba: 1.6 * rem, derecha: 8.5 * rem, abajo: 2.6 * rem, izquierda: 3 * rem }
      const x0 = margen.izquierda
      const x1 = Math.max(x0 + 10, ancho - margen.derecha)
      const y0 = alto - margen.abajo
      const y1 = margen.arriba

      const enviadas = new Set(puntos.map((p) => Date.parse(p.fecha_hora)))
      const barras: Barra[] = programadas.map((l) => {
        const t = Date.parse(l.fecha_hora)
        return { t, valor: l[contaminante], enviada: enviadas.has(t) }
      })
      // La media de cada lectura enviada; null mientras la ventana no junta las lecturas que hacen falta.
      const serie = puntos.map((p) => ({ t: Date.parse(p.fecha_hora), valor: contaminante === 'pm2_5' ? p.media_pm2_5 : p.media_pm10 }))
      const medias = serie.filter((m): m is { t: number; valor: number } => m.valor !== null)

      const primera = horas[0] ?? 0
      const paso = (x1 - x0) / Math.max(1, horas.length)
      // Cada hora ocupa una columna; su lectura va al centro.
      const x = (t: number) => x0 + ((t - primera) / HORA_MS + 0.5) * paso
      const tope = Math.max(eca * 1.3, max(barras, (b) => b.valor) ?? 0, max(medias, (m) => m.valor) ?? 0)
      const y = scaleLinear().domain([0, tope]).nice(5).range([y0, y1])
      const [, techo] = y.domain() as [number, number]

      // Las bandas INCA de fondo, cada una con su palabra a la derecha.
      const limites = [0, ...cortes, Infinity]
      const bandas = ORDEN.map((banda, i) => ({ banda, desde: limites[i]!, hasta: Math.min(limites[i + 1]!, techo) })).filter(
        (b) => b.desde < techo,
      )
      raiz
        .select('.bandas')
        .selectAll<SVGRectElement, (typeof bandas)[number]>('rect')
        .data(bandas, (b) => b.banda)
        .join('rect')
        .attr('x', x0)
        .attr('width', x1 - x0)
        .attr('y', (b) => y(b.hasta))
        .attr('height', (b) => Math.max(0, y(b.desde) - y(b.hasta)))
        .attr('class', 'opacity-[0.13]')
        .style('fill', (b) => BANDAS[b.banda].color)
      const conLugar = bandas.filter((b) => y(b.desde) - y(b.hasta) >= 0.95 * rem)
      raiz
        .select('.nombres-de-banda')
        .selectAll<SVGGElement, (typeof bandas)[number]>('g')
        .data(conLugar, (b) => b.banda)
        .join((entra) => {
          const g = entra.append('g')
          g.append('rect')
          g.append('text')
          return g
        })
        .each(function (b) {
          const g = select(this)
          const medio = (y(b.desde) + y(b.hasta)) / 2
          g.select('rect')
            .attr('x', x1 + 0.6 * rem)
            .attr('y', medio - 0.3 * rem)
            .attr('width', 0.6 * rem)
            .attr('height', 0.6 * rem)
            .attr('rx', 2)
            .style('fill', BANDAS[b.banda].color)
          g.select('text')
            .attr('x', x1 + 1.5 * rem)
            .attr('y', medio)
            .attr('dominant-baseline', 'central')
            .attr('class', 'fill-suave text-xs')
            .text(BANDAS[b.banda].palabra)
        })

      // El eje vertical: líneas finas y su número.
      const marcasY = y.ticks(5)
      raiz
        .select('.eje-y')
        .selectAll<SVGGElement, number>('g')
        .data(marcasY, (d) => d)
        .join((entra) => {
          const g = entra.append('g')
          g.append('line')
          g.append('text')
          return g
        })
        .each(function (d) {
          const g = select(this)
          g.select('line').attr('x1', x0).attr('x2', x1).attr('y1', y(d)).attr('y2', y(d)).attr('class', 'stroke-borde/70')
          g.select('text')
            .attr('x', x0 - 0.5 * rem)
            .attr('y', y(d))
            .attr('text-anchor', 'end')
            .attr('dominant-baseline', 'central')
            .attr('class', 'fill-tenue font-mono text-xs')
            .text(numero(d))
        })

      // El eje horizontal: una marca cada 6 horas y el día debajo de la primera de cada día.
      const marcasX = horas.filter((t) => horaDeLima(t) % 6 === 0)
      raiz
        .select('.eje-x')
        .selectAll<SVGTextElement, number>('text')
        .data(marcasX, (d) => d)
        .join('text')
        .attr('x', (t) => x(t))
        .attr('y', y0 + 1.05 * rem)
        .attr('text-anchor', 'middle')
        .attr('class', 'fill-tenue font-mono text-xs')
        .text((t) => horaEnLima(t))
      const dias = new Map<string, number>()
      for (const t of horas) {
        const dia = diaCorto(t - HORA_MS)
        if (!dias.has(dia)) dias.set(dia, t)
      }
      raiz
        .select('.dias')
        .selectAll<SVGTextElement, [string, number]>('text')
        .data([...dias], (d) => d[0])
        .join('text')
        .attr('x', (d) => x(d[1]) - paso / 2)
        .attr('y', y0 + 2.2 * rem)
        .attr('class', 'fill-suave text-xs font-semibold')
        .text((d) => d[0])
      raiz
        .select('.cambio-de-dia')
        .selectAll<SVGLineElement, number>('line')
        .data([...dias.values()].slice(1))
        .join('line')
        .attr('x1', (t) => x(t) - paso / 2)
        .attr('x2', (t) => x(t) - paso / 2)
        .attr('y1', y1)
        .attr('y2', y0 + 2.4 * rem)
        .attr('class', 'stroke-borde')

      // Las lecturas horarias: llenas las enviadas, en contorno las que faltan.
      const anchoBarra = Math.max(2, paso * 0.62)
      raiz
        .select('.lecturas')
        .selectAll<SVGRectElement, Barra>('rect')
        .data(barras, (b) => b.t)
        .join('rect')
        .attr('x', (b) => x(b.t) - anchoBarra / 2)
        .attr('width', anchoBarra)
        .attr('y', (b) => y(b.valor))
        .attr('height', (b) => Math.max(1, y0 - y(b.valor)))
        .attr('rx', 1.5)
        .attr('class', (b) => (b.enviada ? 'fill-acento/85' : 'fill-transparent stroke-borde-control/70'))

      // El ECA de 24 h.
      raiz.select('.eca line').attr('x1', x0).attr('x2', x1).attr('y1', y(eca)).attr('y2', y(eca))
      raiz
        .select('.eca text')
        .attr('x', x0 + 0.4 * rem)
        .attr('y', y(eca) - 0.35 * rem)
        .text(`ECA de 24 h: ${numero(eca)} µg/m³`)

      // La media de 24 h que respondió la API.
      // Donde la ventana se queda sin lecturas suficientes, la línea se corta: no se inventa una media.
      const trazo = line<{ t: number; valor: number | null }>()
        .defined((m) => m.valor !== null)
        .x((m) => x(m.t))
        .y((m) => y(m.valor ?? 0))
      raiz.select('.media path').attr('d', trazo(serie) ?? '')
      // El último valor de la media, marcado: es el que la pantalla dice en cifras.
      raiz
        .select('.media-puntos')
        .selectAll<SVGCircleElement, { t: number; valor: number }>('circle')
        .data(medias.slice(-1), () => 'ultimo')
        .join('circle')
        .attr('cx', (m) => x(m.t))
        .attr('cy', (m) => y(m.valor))
        .attr('r', 0.28 * rem)
        .attr('class', (m) => (m.valor > eca ? 'fill-falla stroke-fondo stroke-2' : 'fill-texto stroke-fondo stroke-2'))

      // Cuándo apareció la media y cuándo se encendió la alerta.
      const marcas: Marca[] = []
      const tMedia = primeraMedia === null ? null : Date.parse(primeraMedia)
      const tAlerta = primeraAlerta === null ? null : Date.parse(primeraAlerta)
      if (tMedia !== null && tMedia === tAlerta) {
        marcas.push({ t: tMedia, texto: `media y alerta desde las ${horaEnLima(tMedia)}`, alerta: true })
      } else {
        if (tMedia !== null) marcas.push({ t: tMedia, texto: `media desde las ${horaEnLima(tMedia)}`, alerta: false })
        if (tAlerta !== null) marcas.push({ t: tAlerta, texto: `alerta desde las ${horaEnLima(tAlerta)}`, alerta: true })
      }
      raiz
        .select('.marcas')
        .selectAll<SVGGElement, Marca>('g')
        .data(marcas, (m) => m.texto)
        .join((entra) => {
          const g = entra.append('g')
          g.append('line')
          g.append('text')
          return g
        })
        .each(function (m, i) {
          const g = select(this)
          const alFinal = x(m.t) > (x0 + x1) / 2
          g.select('line')
            .attr('x1', x(m.t))
            .attr('x2', x(m.t))
            .attr('y1', y1 - 0.2 * rem)
            .attr('y2', y0)
            .attr('stroke-dasharray', '4 3')
            .attr('class', m.alerta ? 'stroke-falla' : 'stroke-suave')
          g.select('text')
            .attr('x', x(m.t) + (alFinal ? -0.4 : 0.4) * rem)
            // Dos marcas no se pisan: la segunda va un renglón más abajo.
            .attr('y', y1 + (0.35 + i * 1.05) * rem)
            .attr('text-anchor', alFinal ? 'end' : 'start')
            .attr('class', `text-xs font-semibold ${HALO} ${m.alerta ? 'fill-falla' : 'fill-texto'}`)
            .text(m.texto)
        })
    },
    [horas, programadas, puntos, contaminante, eca, cortes, primeraMedia, primeraAlerta, ancho, rem, alto],
  )

  const conMedia = puntos.filter((p) => (contaminante === 'pm2_5' ? p.media_pm2_5 : p.media_pm10) !== null).length
  return (
    <div ref={contenedor} className="w-full">
      <svg
        ref={svg}
        width={ancho}
        height={alto}
        role="img"
        aria-label={`Curva de ${nombre} de ${estacion}: ${puntos.length} de ${programadas.length} lecturas enviadas, ${conMedia} con media de 24 horas. ECA de 24 horas: ${eca} µg/m³.`}
        className="block"
      >
        <g className="bandas" />
        <g className="nombres-de-banda" />
        <g className="eje-y" />
        <text x={0} y={0.8 * rem} className="fill-tenue text-xs">
          µg/m³
        </text>
        <g className="cambio-de-dia" />
        <g className="lecturas" />
        <g className="eca">
          <line className="stroke-falla" strokeDasharray="7 4" strokeWidth={1.5} />
          <text className={`fill-falla text-xs font-semibold ${HALO}`} />
        </g>
        <g className="media">
          <path className="stroke-texto fill-none" strokeWidth={2.25} strokeLinejoin="round" strokeLinecap="round" />
        </g>
        <g className="media-puntos" />
        <g className="marcas" />
        <g className="eje-x" />
        <g className="dias" />
      </svg>
      <ul className="text-suave mt-2 flex flex-wrap gap-x-5 gap-y-1 text-xs">
        <li className="flex items-center gap-2">
          <span aria-hidden="true" className="bg-acento inline-block h-3 w-2 rounded-sm" />
          Lectura horaria de {nombre} enviada
        </li>
        <li className="flex items-center gap-2">
          <span aria-hidden="true" className="border-borde-control inline-block h-3 w-2 rounded-sm border" />
          Por enviar
        </li>
        <li className="flex items-center gap-2">
          <span aria-hidden="true" className="bg-texto inline-block h-0.5 w-5" />
          Media de 24 h que respondió la API
        </li>
        <li className="flex items-center gap-2">
          <span aria-hidden="true" className="border-falla inline-block w-5 border-t-2 border-dashed" />
          ECA de 24 h ({numero(eca)} µg/m³)
        </li>
      </ul>
    </div>
  )
}
