import type { ReactNode } from 'react'

const TAMANOS = {
  // Del alto de una <Cifra> mediana, para ir a su lado en una fila de tarjetas.
  mediana: 'leading-8',
  chica: 'leading-tight',
} as const

interface Props {
  /** Qué es: «Motivo de paro», «Último estado recibido». */
  rotulo: ReactNode
  /** El dato, escrito: una fecha, una palabra, una duración con su unidad. */
  children: ReactNode
  /** Una línea chica debajo: de dónde sale, cuándo llegó. */
  detalle?: ReactNode
  tamano?: keyof typeof TAMANOS
  /** Una aclaración que se ve al pasar el cursor. */
  titulo?: string
  className?: string
}

/**
 * Un dato que se escribe con palabras y no con un número: lo que <Cifra> no
 * cubre. El mismo rótulo de arriba y la misma monoespaciada, sin unidad al lado.
 */
export function Dato({ rotulo, children, detalle, tamano = 'chica', titulo, className = '' }: Props) {
  return (
    <div title={titulo} className={className}>
      <p className="rotulo mb-1">{rotulo}</p>
      <p className={`text-texto font-mono text-lg font-semibold tabular-nums ${TAMANOS[tamano]}`}>{children}</p>
      {detalle && <p className="text-suave mt-1 text-xs">{detalle}</p>}
    </div>
  )
}
