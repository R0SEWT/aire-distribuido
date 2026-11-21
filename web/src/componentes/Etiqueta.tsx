import type { ReactNode } from 'react'
import { BORDE, TEXTO, type Tono } from './tonos'

/** Una etiqueta chica con borde de color y texto: «En alerta», «Modelo activo». */
export function Etiqueta({ tono = 'neutro', children }: { tono?: Tono; children: ReactNode }) {
  return (
    <span
      className={`inline-flex items-center rounded-full border px-2 py-0.5 text-xs font-semibold whitespace-nowrap ${BORDE[tono]} ${TEXTO[tono]}`}
    >
      {children}
    </span>
  )
}
