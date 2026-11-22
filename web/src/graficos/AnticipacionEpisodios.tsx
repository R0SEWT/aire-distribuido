// La anticipación, episodio por episodio (spec §7.4). Cada punto es una vez que
// la media de 24 h de una estación cruzó el ECA; su lugar dice cuántas horas
// antes ya estaba encendida la señal. Van las dos medidas juntas, una debajo de
// la otra, porque dicen cosas muy distintas:
//
//  - desde la primera vez que la señal se enciende en las 24 horas previas, y
//  - cuántas horas seguidas llevaba encendida justo antes del cruce.
//
// La mediana y los cuartiles de cada fila vienen del archivo: no se recalculan.

import { scaleLinear, select } from 'd3'
import { numero } from '../componentes/formato'
import type { Cuartiles } from '../api/datos'
import { useDibujo } from './dibujo'
import { useMedida } from './medida'

export interface FilaDeAnticipacion {
  /** La señal: «Lectura horaria», «Régimen alto». */
  senal: string
  /** Las horas de cada episodio. */
  horas: readonly number[]
  cuartiles: Cuartiles | null
}

export interface MedidaDeAnticipacion {
  titulo: string
  filas: readonly FilaDeAnticipacion[]
  /** Si los episodios en el tope se dibujan huecos: ahí la medida no puede ser mayor. */
  marcaElTope: boolean
}

interface Props {
  medidas: readonly MedidaDeAnticipacion[]
  /** El máximo que la medida puede dar por construcción (23 horas). */
  tope: number
}

interface FilaPuesta extends FilaDeAnticipacion {
  clave: string
  /** Dónde va la línea de base de la fila y cuánto mide de alto. */
  base: number
  altura: number
  marcaElTope: boolean
}

interface Punto {
  clave: string
  hora: number
  nivel: number
  enElTope: boolean
}

function pilas(horas: readonly number[]): Map<number, number> {
  const cuenta = new Map<number, number>()
  for (const h of horas) cuenta.set(h, (cuenta.get(h) ?? 0) + 1)
  return cuenta
}

// Donde no hay con qué medir (las pruebas, en jsdom) se dibuja a este ancho.
const POR_OMISION = { ancho: 760 }

export function AnticipacionEpisodios({ medidas, tope }: Props) {
  const [contenedor, { ancho, rem }] = useMedida<HTMLDivElement>(POR_OMISION)
  const radio = 0.22 * rem
  const paso = 2 * radio + 1.5
  const altoTitulo = 1.7 * rem
  const relleno = 0.9 * rem
  const abajo = 2.4 * rem

  // El alto de cada fila sale de su pila más alta.
  const puestas: { titulo: string; y: number; filas: FilaPuesta[] }[] = []
  let y = 0
  for (const medida of medidas) {
    const grupo = { titulo: medida.titulo, y, filas: [] as FilaPuesta[] }
    y += altoTitulo
    for (const fila of medida.filas) {
      const masAlta = Math.max(1, ...pilas(fila.horas).values())
      const altura = Math.max(2.2 * rem, masAlta * paso + relleno)
      y += altura
      grupo.filas.push({ ...fila, clave: `${medida.titulo}|${fila.senal}`, base: y, altura, marcaElTope: medida.marcaElTope })
    }
    y += 0.9 * rem
    puestas.push(grupo)
  }
  const alto = y + abajo

  const svg = useDibujo<SVGSVGElement>(
    (nodo) => {
      const raiz = select(nodo)
      const izquierda = Math.min(9.5 * rem, ancho * 0.24)
      const derecha = 9 * rem
      const x = scaleLinear()
        .domain([0, tope])
        .range([izquierda + radio + 2, Math.max(izquierda + 20, ancho - derecha)])
      const filas = puestas.flatMap((g) => g.filas)
      const fin = alto - abajo

      // La escala de horas, común a las dos medidas.
      const marcas = [...new Set([0, 4, 8, 12, 16, 20, tope].filter((h) => h <= tope))]
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
          g.select('line').attr('x1', x(d)).attr('x2', x(d)).attr('y1', altoTitulo).attr('y2', fin).attr('class', 'stroke-borde/70')
          g.select('text')
            .attr('x', x(d))
            .attr('y', fin + 1.05 * rem)
            .attr('text-anchor', 'middle')
            .attr('class', 'fill-tenue font-mono text-xs')
            .text(`${d} h`)
        })
      raiz
        .select('.escala-nombre')
        .attr('x', x(tope / 2))
        .attr('y', fin + 2.15 * rem)
        .text('horas antes del cruce del ECA')

      raiz
        .select('.titulos')
        .selectAll<SVGTextElement, { titulo: string; y: number }>('text')
        .data(puestas, (g) => g.titulo)
        .join('text')
        .attr('x', 0)
        .attr('y', (g) => g.y + 1.05 * rem)
        .attr('class', 'fill-texto stroke-superficie text-sm font-semibold [paint-order:stroke] stroke-[0.35rem]')
        .text((g) => g.titulo)

      raiz
        .select('.filas')
        .selectAll<SVGGElement, FilaPuesta>('g.fila')
        .data(filas, (f) => f.clave)
        .join((entra) => {
          const g = entra.append('g').attr('class', 'fila')
          g.append('line').attr('class', 'base')
          g.append('text').attr('class', 'senal')
          g.append('rect').attr('class', 'entre-cuartiles')
          g.append('g').attr('class', 'puntos')
          g.append('line').attr('class', 'mediana')
          g.append('text').attr('class', 'cifra')
          return g
        })
        .each(function (f) {
          const g = select(this)
          const centro = f.base - f.altura / 2
          g.select('line.base').attr('x1', izquierda).attr('x2', x(tope) + radio).attr('y1', f.base).attr('y2', f.base).attr('class', 'base stroke-borde')
          g.select('text.senal')
            .attr('x', 0.2 * rem)
            .attr('y', centro)
            .attr('dominant-baseline', 'central')
            .attr('class', 'senal fill-suave text-sm')
            .text(f.senal)

          const q = f.cuartiles
          g.select('rect.entre-cuartiles')
            .attr('x', q ? x(q.q1) - radio : 0)
            .attr('y', f.base - f.altura + 3)
            .attr('width', q ? Math.max(2 * radio, x(q.q3) - x(q.q1) + 2 * radio) : 0)
            .attr('height', f.altura - 3)
            .attr('rx', 3)
            .attr('class', 'entre-cuartiles fill-acento/20')

          const niveles = new Map<number, number>()
          const puntos: Punto[] = f.horas.map((hora, i) => {
            const nivel = niveles.get(hora) ?? 0
            niveles.set(hora, nivel + 1)
            return { clave: `${i}`, hora, nivel, enElTope: f.marcaElTope && hora >= tope }
          })
          g.select('g.puntos')
            .selectAll<SVGCircleElement, Punto>('circle')
            .data(puntos, (p) => p.clave)
            .join('circle')
            .attr('cx', (p) => x(p.hora))
            .attr('cy', (p) => f.base - radio - 2 - p.nivel * paso)
            .attr('r', radio)
            .attr('class', (p) => (p.enElTope ? 'fill-superficie stroke-acento-claro stroke-[1.5px]' : 'fill-acento-claro'))

          g.select('line.mediana')
            .attr('x1', q ? x(q.mediana) : 0)
            .attr('x2', q ? x(q.mediana) : 0)
            .attr('y1', f.base - f.altura + 1)
            .attr('y2', f.base + 3)
            .attr('class', q ? 'mediana stroke-texto stroke-[2.5px]' : 'mediana stroke-transparent')
          g.select('text.cifra')
            .attr('x', x(tope) + radio + 0.7 * rem)
            .attr('y', centro)
            .attr('dominant-baseline', 'central')
            .attr('class', 'cifra fill-texto font-mono text-sm font-semibold')
            .text(q ? `mediana ${numero(q.mediana, Number.isInteger(q.mediana) ? 0 : 1)} h` : 'sin episodios')
        })
    },
    // `puestas` se arma en cada dibujo con lo mismo de lo que dependen medidas, tope y las medidas del lienzo.
    [medidas, tope, ancho, rem, alto],
  )

  const descripcion = medidas
    .map((m) => `${m.titulo}: ${m.filas.map((f) => `${f.senal}, mediana ${f.cuartiles ? numero(f.cuartiles.mediana, 1) : 'sin dato'} horas`).join('; ')}`)
    .join('. ')
  return (
    <div ref={contenedor} className="w-full">
      <svg ref={svg} width={ancho} height={alto} role="img" aria-label={`Anticipación de cada señal, episodio por episodio. ${descripcion}.`} className="block">
        <g className="escala" />
        <text className="escala-nombre fill-tenue text-xs" textAnchor="middle" />
        <g className="titulos" />
        <g className="filas" />
      </svg>
    </div>
  )
}
