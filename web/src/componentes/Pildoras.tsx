import type { ReactNode } from 'react'

export interface Opcion<V extends string> {
  valor: V
  etiqueta: ReactNode
  deshabilitada?: boolean
}

interface Props<V extends string> {
  /** Qué se está eligiendo; lo leen los lectores de pantalla. */
  etiqueta: string
  opciones: readonly Opcion<V>[]
  valor: V
  alCambiar: (valor: V) => void
  className?: string
}

/** Pestañas en píldora para elegir una opción entre pocas: PM2.5 o PM10, la velocidad… (spec §8). */
export function Pildoras<V extends string>({ etiqueta, opciones, valor, alCambiar, className = '' }: Props<V>) {
  return (
    <div role="group" aria-label={etiqueta} className={`flex flex-wrap gap-2 ${className}`}>
      {opciones.map((o) => (
        <button
          key={o.valor}
          type="button"
          className="pildora"
          aria-pressed={o.valor === valor}
          disabled={o.deshabilitada}
          onClick={() => alCambiar(o.valor)}
        >
          {o.etiqueta}
        </button>
      ))}
    </div>
  )
}
