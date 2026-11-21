import type { ReactNode } from 'react'
import { BORDE_IZQUIERDO, type Tono } from './tonos'

interface Props {
  /** La etiqueta chica en mayúsculas de arriba: «CA-AYAS-01 · MEDIA DE 24 H». */
  rotulo?: ReactNode
  titulo?: ReactNode
  /** Lo que va a la derecha del título: una fecha, un control. */
  aLaDerecha?: ReactNode
  /** Con acento, la tarjeta lleva un borde izquierdo de color: es la que manda en la pantalla. */
  acento?: Tono
  className?: string
  children?: ReactNode
}

/** El contenedor de casi todo: fondo apenas más claro que la página y borde fino (spec §8). */
export function Tarjeta({ rotulo, titulo, aLaDerecha, acento, className = '', children }: Props) {
  const borde = acento ? `border-l-4 ${BORDE_IZQUIERDO[acento]}` : ''
  return (
    <section className={`border-borde bg-superficie rounded-xl border p-5 ${borde} ${className}`}>
      {(rotulo || titulo || aLaDerecha) && (
        <header className="mb-3 flex items-start justify-between gap-4">
          <div>
            {rotulo && <p className="rotulo">{rotulo}</p>}
            {titulo && <h2 className="text-texto text-base font-semibold">{titulo}</h2>}
          </div>
          {aLaDerecha && <div className="text-suave shrink-0 text-sm">{aLaDerecha}</div>}
        </header>
      )}
      {children}
    </section>
  )
}
