import type { ReactNode } from 'react'

/** «¿De dónde sale este número?»: la explicación que se abre debajo de una cifra (spec §8). */
export function Desplegable({ titulo, children }: { titulo: ReactNode; children: ReactNode }) {
  return (
    <details className="text-sm">
      <summary className="text-texto font-semibold">{titulo}</summary>
      <div className="text-suave mt-2 space-y-2 leading-relaxed">{children}</div>
    </details>
  )
}
