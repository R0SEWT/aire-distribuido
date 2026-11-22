// El mapa del Perú con las estaciones (spec §5.2). D3 hace las cuentas (la
// proyección, el trazo de los departamentos y el reparto de las marcas que se
// pisan) y React dibuja el SVG: sin estilos en línea, los colores salen de los
// tokens por clases, y el modo proyector los cambia solo.
//
// El color nunca va solo. Cada uno de los tres estados tiene su forma: un
// círculo lleno (con estado, del color de su banda INCA), un círculo hueco (sin
// datos suficientes) y un rombo con trama (no mide PM por hora). La alerta es
// un anillo y, además, el código de la estación escrito al lado. La palabra de
// cada marca está en la leyenda, en su nombre accesible y en la lista.

import {
  forceCollide,
  forceSimulation,
  forceX,
  forceY,
  geoMercator,
  geoPath,
  type ExtendedFeatureCollection,
  type SimulationNodeDatum,
} from 'd3'
import { useId, useLayoutEffect, useMemo, useState, type ReactNode } from 'react'
import { Link } from 'react-router'
import type { EstadoBanda } from '../componentes/indice'
import { useMedida, type Medida } from './medida'

/** El tamaño con que se dibuja mientras la caja no se pudo medir. */
const POR_OMISION = { ancho: 440, alto: 560 }
const MARGEN = 16

/** Dónde está una estación. No cambia con los estados: de aquí sale el reparto de las marcas. */
export interface LugarEnMapa {
  id: string
  lon: number
  lat: number
  /** No mide PM por hora: su marca es chica y nunca lleva anillo. */
  apagada: boolean
}

/** Cómo se dibuja una estación con su último estado recibido. */
export interface MarcaEnMapa {
  banda: EstadoBanda
  alerta: boolean
  /** Lo que se lee de la marca: «CA-HU-04, Lurigancho: INCA mala, en alerta». */
  nombre: string
}

interface Props {
  geo: ExtendedFeatureCollection
  lugares: readonly LugarEnMapa[]
  /**
   * La marca de cada estación. Su orden (el de inserción) es el orden en que se
   * recorren con el teclado: quien arma el mapa las pone de la más grave a la
   * menos, como en la lista que lo acompaña.
   */
  marcas: ReadonlyMap<string, MarcaEnMapa>
  /** La dirección a la que lleva la marca de una estación. */
  enlaceDe: (id: string) => string
  /** Qué estación tiene el cursor o el foco, para mostrar su detalle fuera del mapa. */
  alSenalar?: (id: string | null) => void
  senalada?: string | null
  /** Qué es el mapa, para los lectores de pantalla. */
  titulo: string
  /**
   * Contra qué borde de la caja va el país cuando sobra ancho. A la derecha
   * deja libre el mar del suroeste, que es donde cabe una leyenda encima.
   */
  alinear?: 'centro' | 'derecha'
  /** Las clases de la caja: de ellas sale el tamaño del mapa. */
  className?: string
  /**
   * Lo que va encima del mapa (la leyenda), ubicado por quien lo pone. Lo que
   * lleve `data-sobre-mapa` no se tapa con el código de una estación en alerta.
   */
  children?: ReactNode
}

const RELLENO: Record<EstadoBanda, string> = {
  buena: 'fill-inca-buena',
  moderada: 'fill-inca-moderada',
  mala: 'fill-inca-mala',
  umbral_de_cuidado: 'fill-inca-cuidado',
  sin_datos: 'fill-fondo',
  no_mide: 'fill-no-mide',
}

const RADIO = { banda: 6.5, hueco: 5, rombo: 5.5, anillo: 10.5 } as const

/**
 * La marca de una estación, centrada en el origen. La usan el mapa y su
 * leyenda, para que digan lo mismo. `trama` es el id del patrón de «no mide».
 */
export function GlifoEstacion({ banda, alerta, trama }: { banda: EstadoBanda; alerta: boolean; trama: string }) {
  if (banda === 'no_mide') {
    const r = RADIO.rombo
    return (
      <g data-glifo="rombo">
        <path d={`M0 ${-r}L${r} 0L0 ${r}L${-r} 0Z`} className="fill-fondo stroke-no-mide" strokeWidth={1.5} />
        <path d={`M0 ${-r}L${r} 0L0 ${r}L${-r} 0Z`} fill={`url(#${trama})`} />
      </g>
    )
  }
  if (banda === 'sin_datos') {
    return <circle data-glifo="hueco" r={RADIO.hueco} className="fill-fondo stroke-sin-datos" strokeWidth={2} />
  }
  return (
    <g data-glifo="lleno">
      {alerta && <circle data-anillo r={RADIO.anillo} className="stroke-falla fill-none" strokeWidth={2.25} />}
      <circle r={RADIO.banda} className={`stroke-fondo ${RELLENO[banda]}`} strokeWidth={1.5} />
    </g>
  )
}

/** El patrón de rayas de «no mide PM por hora»: va una vez dentro del <svg> que use el glifo. */
export function TramaNoMide({ id }: { id: string }) {
  return (
    <pattern id={id} width={3} height={3} patternUnits="userSpaceOnUse">
      <rect width={1.5} height={3} className="fill-no-mide" />
    </pattern>
  )
}

interface Nodo extends SimulationNodeDatum {
  id: string
  x: number
  y: number
  /** El lugar verdadero de la estación en el mapa. */
  x0: number
  y0: number
  /** El sitio que ocupa su marca, con el anillo si puede llevarlo. */
  r: number
}

/**
 * Proyecta el país para que entre en la caja y reparte las marcas: varias
 * estaciones vigilan la misma actividad y a esta escala caen una sobre otra
 * (doce entre Lima y el Callao). Cada marca se aparta lo justo para no pisar a
 * las vecinas y queda unida a su lugar por una línea.
 */
function disponer(geo: ExtendedFeatureCollection, lugares: readonly LugarEnMapa[], medida: Medida, alinear: 'centro' | 'derecha') {
  const { ancho, alto, escala: k } = medida
  const margen = MARGEN * k
  const proyeccion = geoMercator().fitExtent(
    [
      [margen, margen],
      [ancho - margen, alto - margen],
    ],
    geo,
  )
  if (alinear === 'derecha') {
    const [, [derecha]] = geoPath(proyeccion).bounds(geo)
    const [tx, ty] = proyeccion.translate()
    proyeccion.translate([tx + (ancho - margen - derecha), ty])
  }
  const trazo = geoPath(proyeccion).digits(1)
  const departamentos = geo.features.map((f, i) => ({
    clave: `${String(f.properties?.iso ?? f.properties?.departamento ?? '')}-${i}`,
    d: trazo(f) ?? '',
  }))

  const nodos: Nodo[] = []
  for (const l of lugares) {
    const p = proyeccion([l.lon, l.lat])
    if (!p) continue
    nodos.push({ id: l.id, x: p[0], y: p[1], x0: p[0], y0: p[1], r: (l.apagada ? RADIO.rombo + 1.5 : RADIO.anillo + 1.5) * k })
  }
  // Se corre hasta que se asienta, sin animar: el resultado es siempre el mismo
  // para el mismo catálogo y la misma caja, y no cambia cuando llega un estado.
  forceSimulation(nodos)
    .force('x', forceX<Nodo>((n) => n.x0).strength(0.12))
    .force('y', forceY<Nodo>((n) => n.y0).strength(0.12))
    .force('choque', forceCollide<Nodo>((n) => n.r).iterations(6))
    .stop()
    .tick(400)
  for (const n of nodos) {
    n.x = Math.min(ancho - n.r, Math.max(n.r, n.x))
    n.y = Math.min(alto - n.r, Math.max(n.r, n.y))
  }
  return { departamentos, nodos }
}

interface Caja {
  x0: number
  y0: number
  x1: number
  y1: number
}

interface Rotulo {
  id: string
  x: number
  y: number
  ancla: 'start' | 'end' | 'middle'
  /** Si quedó lejos de su marca, la línea que lo une a ella. */
  guia: { x: number; y: number } | null
}

// El rótulo va en monoespaciada de 0.75 rem: cada letra mide 0.6 de eso.
const ALTO_ROTULO = 14
const ANCHO_LETRA = 7.2

/**
 * Dónde escribir el código de cada estación en alerta: al lado de su marca, en
 * el primer sitio que no tape otra marca ni otro rótulo ni se salga del lienzo.
 */
function rotular(nodos: readonly Nodo[], enAlerta: readonly string[], medida: Medida, ocupado: readonly Caja[]): Rotulo[] {
  const { ancho: anchoCaja, alto: altoCaja, escala: k } = medida
  const puestas: Caja[] = [...ocupado]
  const choca = (c: Caja, propio: Nodo) => {
    if (c.x0 < 2 || c.y0 < 2 || c.x1 > anchoCaja - 2 || c.y1 > altoCaja - 2) return true
    if (puestas.some((p) => c.x0 < p.x1 && c.x1 > p.x0 && c.y0 < p.y1 && c.y1 > p.y0)) return true
    return nodos.some((n) => {
      if (n === propio) return false
      const dx = Math.max(c.x0 - n.x, 0, n.x - c.x1)
      const dy = Math.max(c.y0 - n.y, 0, n.y - c.y1)
      return dx * dx + dy * dy < (n.r - 1) * (n.r - 1)
    })
  }
  const rotulos: Rotulo[] = []
  for (const id of enAlerta) {
    const n = nodos.find((x) => x.id === id)
    if (!n) continue
    const ancho = id.length * ANCHO_LETRA * k
    const altoRotulo = ALTO_ROTULO * k
    const d = (RADIO.anillo + 4) * k
    const candidatos: { x: number; y: number; ancla: Rotulo['ancla']; lejos: number }[] = []
    // Se prueba cada vez más lejos: a la izquierda (hacia el mar para la costa), a la derecha, arriba y abajo.
    for (const lejos of [0, 10 * k, 20 * k, 32 * k]) {
      candidatos.push(
        { x: n.x - d - lejos, y: n.y, ancla: 'end', lejos },
        { x: n.x + d + lejos, y: n.y, ancla: 'start', lejos },
        { x: n.x, y: n.y - d - 4 - lejos, ancla: 'middle', lejos },
        { x: n.x, y: n.y + d + 4 + lejos, ancla: 'middle', lejos },
        { x: n.x - d - lejos, y: n.y - d, ancla: 'end', lejos },
        { x: n.x - d - lejos, y: n.y + d, ancla: 'end', lejos },
        { x: n.x + d + lejos, y: n.y - d, ancla: 'start', lejos },
        { x: n.x + d + lejos, y: n.y + d, ancla: 'start', lejos },
      )
    }
    const cajaDe = (c: (typeof candidatos)[number]): Caja => {
      const x0 = c.ancla === 'end' ? c.x - ancho : c.ancla === 'start' ? c.x : c.x - ancho / 2
      return { x0, y0: c.y - altoRotulo / 2, x1: x0 + ancho, y1: c.y + altoRotulo / 2 }
    }
    const elegido = candidatos.find((c) => !choca(cajaDe(c), n)) ?? candidatos[0]!
    puestas.push(cajaDe(elegido))
    rotulos.push({ id, x: elegido.x, y: elegido.y, ancla: elegido.ancla, guia: elegido.lejos > 0 ? { x: n.x, y: n.y } : null })
  }
  return rotulos
}

// Qué tan arriba se dibuja cada marca: lo más grave va encima.
const CAPA: Record<EstadoBanda, number> = { no_mide: 0, sin_datos: 1, buena: 2, moderada: 3, mala: 4, umbral_de_cuidado: 5 }

/** El mapa del Perú con una marca por estación; cada marca lleva a su página. */
export function MapaPeru({ geo, lugares, marcas, enlaceDe, alSenalar, senalada, titulo, alinear = 'centro', className = '', children }: Props) {
  const trama = useId()
  const [caja, medida] = useMedida<HTMLDivElement>(POR_OMISION)
  const k = medida.escala
  const { departamentos, nodos } = useMemo(() => disponer(geo, lugares, medida, alinear), [geo, lugares, medida, alinear])

  // Dónde quedó lo que va encima del mapa, para no escribirle un rótulo encima.
  // Se mide al montar y cuando cambia de tamaño el mapa o lo que lleva encima,
  // no en cada dibujo: a ritmo rápido el mapa se vuelve a dibujar varias veces
  // por segundo, y medir obliga al navegador a recalcular el maquetado cada vez.
  const [ocupado, setOcupado] = useState<readonly Caja[]>([])
  useLayoutEffect(() => {
    const marco = caja.current
    if (!marco) return
    const sobreElMapa = [...marco.querySelectorAll('[data-sobre-mapa]')]
    const medir = () => {
      const origen = marco.getBoundingClientRect()
      const cajas = sobreElMapa.map((e) => {
        const r = e.getBoundingClientRect()
        return { x0: r.left - origen.left, y0: r.top - origen.top, x1: r.right - origen.left, y1: r.bottom - origen.top }
      })
      setOcupado((antes) => (JSON.stringify(antes) === JSON.stringify(cajas) ? antes : cajas))
    }
    medir()
    if (typeof ResizeObserver === 'undefined' || sobreElMapa.length === 0) return
    const tamano = new ResizeObserver(medir)
    for (const e of sobreElMapa) tamano.observe(e)
    return () => tamano.disconnect()
  }, [caja, medida])

  // Los enlaces van en el orden de `marcas`, que es el del teclado: de la más
  // grave a la menos. Lo que no tenga marca no se dibuja.
  const dibujados = useMemo(() => {
    const lugar = new Map([...marcas.keys()].map((id, i) => [id, i]))
    return nodos.filter((n) => lugar.has(n.id)).sort((a, b) => lugar.get(a.id)! - lugar.get(b.id)!)
  }, [nodos, marcas])
  // En el SVG lo último que se dibuja queda encima, así que las más graves
  // quedarían debajo de las demás donde se toquen: se repiten arriba de todo, sin enlace.
  const encima = useMemo(() => {
    const capaDe = (id: string) => {
      const m = marcas.get(id)!
      return CAPA[m.banda] + (m.alerta ? 10 : 0)
    }
    return dibujados.filter((n) => marcas.get(n.id)!.alerta || CAPA[marcas.get(n.id)!.banda] >= CAPA.mala).sort((a, b) => capaDe(a.id) - capaDe(b.id))
  }, [dibujados, marcas])

  const rotulos = useMemo(
    () =>
      rotular(
        nodos,
        nodos.filter((n) => marcas.get(n.id)?.alerta).map((n) => n.id),
        medida,
        ocupado,
      ),
    [nodos, marcas, medida, ocupado],
  )

  return (
    <div ref={caja} className={`relative overflow-hidden ${className}`}>
      <svg
        width={medida.ancho}
        height={medida.alto}
        viewBox={`0 0 ${medida.ancho} ${medida.alto}`}
        role="group"
        aria-label={titulo}
        className="absolute inset-0 block"
      >
        <defs>
          <TramaNoMide id={trama} />
        </defs>
        <g aria-hidden="true" data-capa="departamentos">
          {departamentos.map((dep) => (
            <path key={dep.clave} d={dep.d} className="fill-relieve stroke-borde-control/50" strokeWidth={0.75} strokeLinejoin="round" />
          ))}
        </g>
        <g aria-hidden="true" data-capa="guias">
          {nodos.map((n) =>
            Math.hypot(n.x - n.x0, n.y - n.y0) > 3 * k ? (
              <g key={n.id}>
                <line x1={n.x0} y1={n.y0} x2={n.x} y2={n.y} className="stroke-tenue" strokeWidth={0.75} />
                <circle cx={n.x0} cy={n.y0} r={1.25 * k} className="fill-tenue" />
              </g>
            ) : null,
          )}
        </g>
        <g aria-hidden="true" data-capa="guias-de-rotulos">
          {rotulos.map((r) => r.guia && <line key={r.id} x1={r.guia.x} y1={r.guia.y} x2={r.x} y2={r.y} className="stroke-texto" strokeWidth={0.75} />)}
        </g>
        <g data-capa="estaciones">
          {dibujados.map((n) => {
            const marca = marcas.get(n.id)
            if (!marca) return null
            return (
              <Link
                key={n.id}
                to={enlaceDe(n.id)}
                aria-label={marca.nombre}
                data-estacion={n.id}
                data-banda={marca.banda}
                data-alerta={marca.alerta || undefined}
                className="cursor-pointer"
                onMouseEnter={() => alSenalar?.(n.id)}
                onMouseLeave={() => alSenalar?.(null)}
                onFocus={() => alSenalar?.(n.id)}
                onBlur={() => alSenalar?.(null)}
              >
                <g transform={`translate(${n.x.toFixed(1)} ${n.y.toFixed(1)})`}>
                  <title>{marca.nombre}</title>
                  {/* El sitio entero de la marca responde al cursor, no solo el dibujo. */}
                  <circle r={n.r} className="fill-transparent" />
                  {senalada === n.id && <circle r={n.r + 1.5} className="stroke-texto fill-none" strokeWidth={1.25} />}
                  <g transform={`scale(${k})`}>
                    <GlifoEstacion banda={marca.banda} alerta={marca.alerta} trama={trama} />
                  </g>
                </g>
              </Link>
            )
          })}
        </g>
        <g aria-hidden="true" data-capa="encima" className="pointer-events-none">
          {encima.map((n) => {
            const marca = marcas.get(n.id)!
            return (
              <g key={n.id} transform={`translate(${n.x.toFixed(1)} ${n.y.toFixed(1)}) scale(${k})`}>
                <GlifoEstacion banda={marca.banda} alerta={marca.alerta} trama={trama} />
              </g>
            )
          })}
        </g>
        <g aria-hidden="true" data-capa="rotulos">
          {rotulos.map((r) => (
            <text
              key={r.id}
              x={r.x.toFixed(1)}
              y={r.y.toFixed(1)}
              textAnchor={r.ancla}
              dominantBaseline="central"
              className="fill-texto stroke-fondo font-mono text-xs font-semibold [paint-order:stroke]"
              strokeWidth={3 * k}
              strokeLinejoin="round"
            >
              {r.id}
            </text>
          ))}
        </g>
      </svg>
      {children}
    </div>
  )
}
