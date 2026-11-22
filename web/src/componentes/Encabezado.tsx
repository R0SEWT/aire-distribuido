import type { ReactNode } from 'react'

interface Props {
  /** El nombre de la pantalla: el mismo del menú. */
  titulo: string
  /** Lo que acompaña al título en su renglón: el código de la estación que se mira. */
  junto?: ReactNode
  /** La bajada: qué muestra la pantalla, en una frase. */
  bajada?: ReactNode
  /** Lo que va a la derecha del título: un selector, unas píldoras, cuándo se actualizó. */
  children?: ReactNode
}

/**
 * El encabezado de una pantalla: el título, lo que va a su derecha y, debajo y
 * a todo el ancho, la bajada. Todas las pantallas empiezan igual.
 */
export function Encabezado({ titulo, junto, bajada, children }: Props) {
  return (
    <header data-encabezado>
      <div className="flex flex-wrap items-center justify-between gap-x-6 gap-y-2">
        <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
          <h1 className="text-texto text-2xl font-semibold">{titulo}</h1>
          {junto}
        </div>
        {children && <div className="flex flex-wrap items-center gap-3">{children}</div>}
      </div>
      {bajada && <p className="text-suave mt-1">{bajada}</p>}
    </header>
  )
}
