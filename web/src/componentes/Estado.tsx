import type { ReactNode } from 'react'
import { presentarError } from '../api/errores'

/** Los estados que toda pantalla tiene (spec §5.8). La sesión vencida no es uno: vuelve al ingreso. */
export type TipoEstado = 'cargando' | 'vacio' | 'error' | 'sin-permiso'

interface Accion {
  etiqueta: string
  alHacer: () => void
}

interface Props {
  tipo: TipoEstado
  titulo?: ReactNode
  /** El detalle: por qué está vacío, qué dijo la API, qué se puede hacer. */
  children?: ReactNode
  accion?: Accion
  className?: string
}

const TITULOS: Record<TipoEstado, string> = {
  cargando: 'Cargando…',
  vacio: 'Todavía no hay nada que mostrar',
  error: 'Algo falló',
  'sin-permiso': 'No tienes permiso para ver esto',
}

const COLORES: Record<TipoEstado, string> = {
  cargando: 'text-acento-claro',
  vacio: 'text-tenue',
  error: 'text-falla',
  'sin-permiso': 'text-aviso',
}

// Cada estado tiene su dibujo además de su color y su título: no depende del color.
function Icono({ tipo }: { tipo: TipoEstado }) {
  const comun = {
    viewBox: '0 0 24 24',
    fill: 'none',
    stroke: 'currentColor',
    strokeWidth: 2,
    strokeLinecap: 'round' as const,
    strokeLinejoin: 'round' as const,
    'aria-hidden': true,
    className: `size-8 ${COLORES[tipo]} ${tipo === 'cargando' ? 'animate-spin' : ''}`,
  }
  switch (tipo) {
    case 'cargando':
      return (
        <svg {...comun}>
          <path d="M21 12a9 9 0 1 1-6.2-8.56" />
        </svg>
      )
    case 'vacio':
      return (
        <svg {...comun}>
          <path d="M3 13l3-8h12l3 8v6H3z" />
          <path d="M3 13h5l1 3h6l1-3h5" />
        </svg>
      )
    case 'error':
      return (
        <svg {...comun}>
          <path d="M12 3l10 18H2z" />
          <path d="M12 10v5M12 18h.01" />
        </svg>
      )
    case 'sin-permiso':
      return (
        <svg {...comun}>
          <rect x="5" y="11" width="14" height="10" rx="2" />
          <path d="M8 11V7a4 4 0 0 1 8 0v4" />
        </svg>
      )
  }
}

/** Cargando, vacío, error y sin permiso, con el mismo aspecto en toda la app. */
export function Estado({ tipo, titulo, children, accion, className = '' }: Props) {
  return (
    <div
      // Un error interrumpe; lo demás se anuncia cuando el lector de pantalla pueda.
      role={tipo === 'error' ? 'alert' : 'status'}
      data-estado={tipo}
      className={`border-borde bg-superficie flex flex-col items-center gap-3 rounded-xl border border-dashed px-6 py-10 text-center ${className}`}
    >
      <Icono tipo={tipo} />
      <p className="text-texto text-base font-semibold">{titulo ?? TITULOS[tipo]}</p>
      {children && <div className="text-suave max-w-prose text-sm">{children}</div>}
      {accion && (
        <button type="button" className="pildora mt-1" onClick={accion.alHacer}>
          {accion.etiqueta}
        </button>
      )}
    </div>
  )
}

/**
 * El estado que le corresponde a un error de la API (o a cualquier excepción):
 * «sin permiso» para un 403 y «error» con reintento para lo demás. Qué se dice
 * de cada error lo decide api/errores.ts.
 */
export function EstadoDeError({ error, alReintentar }: { error: unknown; alReintentar?: () => void }) {
  const p = presentarError(error)
  return (
    <Estado
      tipo={p.estado === 'sin-permiso' ? 'sin-permiso' : 'error'}
      titulo={p.titulo}
      accion={p.reintentable && alReintentar ? { etiqueta: 'Reintentar', alHacer: alReintentar } : undefined}
    >
      {p.detalle}
    </Estado>
  )
}
