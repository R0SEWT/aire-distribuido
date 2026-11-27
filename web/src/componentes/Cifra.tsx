import type { ReactNode } from 'react'
import { numero } from './formato'

const TAMANOS = {
  grande: { valor: 'text-6xl', unidad: 'text-xl' },
  mediana: { valor: 'text-3xl', unidad: 'text-base' },
  chica: { valor: 'text-lg', unidad: 'text-sm' },
} as const

interface Props {
  /** El número. null es «todavía no hay dato»: se muestra una raya y `sinDato`. */
  valor: number | null | undefined
  unidad?: string
  /** Qué es el número: «PM10 · media de 24 h». */
  rotulo?: ReactNode
  decimales?: number
  tamano?: keyof typeof TAMANOS
  /** Lo que se dice en lugar de la unidad cuando no hay valor. */
  sinDato?: string
  className?: string
}

/** Un número grande en monoespaciada con su unidad chica al lado (spec §8). */
export function Cifra({
  valor,
  unidad,
  rotulo,
  decimales = 0,
  tamano = 'grande',
  sinDato = 'sin dato',
  className = '',
}: Props) {
  const hay = valor !== null && valor !== undefined && Number.isFinite(valor)
  const t = TAMANOS[tamano]
  return (
    <div className={className}>
      {rotulo && <p className="rotulo mb-1">{rotulo}</p>}
      <p className="flex items-baseline gap-2">
        <span className={`text-texto font-mono leading-none font-semibold tabular-nums ${t.valor}`}>
          {hay ? numero(valor, decimales) : <span aria-hidden="true">—</span>}
        </span>
        <span className={`text-suave font-medium ${t.unidad}`}>{hay ? unidad : sinDato}</span>
      </p>
    </div>
  )
}
