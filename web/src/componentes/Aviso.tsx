import type { ReactNode } from 'react'
import { BORDE_IZQUIERDO, TEXTO, type Tono } from './tonos'

interface Accion {
  etiqueta: string
  alHacer: () => void
  deshabilitada?: boolean
}

interface Props {
  /** `alerta` es un rechazo o un error: interrumpe (role="alert"). Lo demás se anuncia cuando se pueda. */
  tono?: Extract<Tono, 'aviso' | 'alerta' | 'acento'>
  /** Lo que pasó, en una línea. */
  titulo?: ReactNode
  /** El detalle: las palabras de la API, qué se puede hacer. */
  children?: ReactNode
  accion?: Accion
  className?: string
}

/**
 * Un aviso dentro de un formulario o de una tarjeta: el rechazo de un pedido,
 * un error que no reemplaza a la pantalla, algo que conviene saber antes de
 * seguir. Para lo que ocupa el lugar de la pantalla entera está <Estado>.
 */
export function Aviso({ tono = 'aviso', titulo, children, accion, className = '' }: Props) {
  return (
    <div
      role={tono === 'alerta' ? 'alert' : 'status'}
      data-aviso={tono}
      className={`bg-relieve text-texto rounded-lg border-l-4 px-4 py-3 text-sm ${BORDE_IZQUIERDO[tono]} ${className}`}
    >
      {titulo && <p className="font-semibold">{titulo}</p>}
      {children && <div className={`text-suave space-y-1 ${titulo ? 'mt-1' : ''}`}>{children}</div>}
      {accion && (
        <button type="button" className="pildora mt-3" onClick={accion.alHacer} disabled={accion.deshabilitada}>
          {accion.etiqueta}
        </button>
      )}
    </div>
  )
}

/**
 * Una advertencia de una línea junto a un control: «Hay 1 nodo libre y pides 2».
 * No interrumpe: acompaña a lo que se está por hacer.
 */
export function Advertencia({ chica = false, children, ...resto }: { chica?: boolean; children: ReactNode } & Record<`data-${string}`, string>) {
  return (
    <p className={`font-semibold ${TEXTO.aviso} ${chica ? 'text-xs' : 'text-sm'}`} {...resto}>
      {children}
    </p>
  )
}
