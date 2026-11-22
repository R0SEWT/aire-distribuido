// El eje horizontal que comparten los dos gráficos de un entrenamiento
// (InerciaPorRonda y TiemposPorNodo): una posición por ronda, con los mismos
// márgenes, para que la ronda 12 quede en la misma vertical en los dos.

import { scaleLinear, type ScaleLinear } from 'd3'

/** Los márgenes laterales, en rem: a la izquierda van los rótulos y a la derecha el último valor. */
export const MARGEN_IZQUIERDO_REM = 7.5
export const MARGEN_DERECHO_REM = 6.5

/** Con menos rondas que estas, el eje igual reserva lugar: la primera barra no ocupa todo el ancho. */
const RONDAS_MINIMAS = 10

export interface EjeDeRondas {
  /** De la ronda (1, 2, 3…) al centro de su columna, en píxeles. */
  x: ScaleLinear<number, number>
  /** El ancho de la columna de una ronda, en píxeles. */
  paso: number
  /** La última ronda que entra en el eje. */
  hasta: number
  /** Las rondas que llevan número en el eje. */
  marcas: number[]
  izquierda: number
  derecha: number
}

/**
 * Arma el eje para `rondas` rondas (las que se conocen: las recibidas o, si ya
 * terminó, las iteraciones del entrenamiento) en un gráfico de `ancho` píxeles.
 */
export function ejeDeRondas(rondas: number, ancho: number, rem: number): EjeDeRondas {
  const hasta = Math.max(rondas, RONDAS_MINIMAS)
  const izquierda = MARGEN_IZQUIERDO_REM * rem
  const derecha = Math.max(izquierda + 1, ancho - MARGEN_DERECHO_REM * rem)
  const paso = (derecha - izquierda) / hasta
  const x = scaleLinear()
    .domain([1, hasta])
    .range([izquierda + paso / 2, derecha - paso / 2])
  // Un número cada 5, 10, 20… rondas, según cuántos entren sin pisarse (unos 2.5 rem cada uno).
  const lugar = Math.max(1, Math.floor((derecha - izquierda) / (2.5 * rem)))
  const cada = [1, 2, 5, 10, 20, 50, 100, 200, 500].find((c) => hasta / c <= lugar) ?? 1000
  const marcas = [1]
  for (let r = cada; r <= hasta; r += cada) if (r !== 1) marcas.push(r)
  return { x, paso, hasta, marcas, izquierda, derecha }
}
