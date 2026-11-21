import type { ReactNode } from 'react'
import { BORDE_IZQUIERDO, type Tono } from './tonos'

/**
 * La frase en llano que acompaña a una cifra: qué significa y comparado con qué
 * (spec §8). «La media de las últimas 24 horas de PM10 es 121 µg/m³: supera el ECA (100)».
 */
export function Frase({ tono = 'acento', children }: { tono?: Tono; children: ReactNode }) {
  return (
    <p
      className={`bg-relieve text-texto rounded-lg border-l-4 px-4 py-3 text-[0.95rem] leading-snug font-semibold ${BORDE_IZQUIERDO[tono]}`}
    >
      {children}
    </p>
  )
}
