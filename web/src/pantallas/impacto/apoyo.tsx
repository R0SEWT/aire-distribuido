// Piezas chicas que comparten las secciones de Impacto.

import type { ReactNode } from 'react'
import { Desplegable, fechaDia, numero } from '../../componentes/indice'
import type { Impacto } from '../../api/datos'

interface PropsDeDondeSale {
  impacto: Impacto
  /** Las claves de metadatos.fuentes de las que sale la cifra. */
  fuentes: readonly string[]
  children: ReactNode
}

/** «¿De dónde sale este número?»: la regla, los archivos (con su huella) y quién lo calcula. */
export function DeDondeSale({ impacto, fuentes, children }: PropsDeDondeSale) {
  const { metadatos } = impacto
  return (
    <Desplegable titulo="¿De dónde sale este número?">
      {children}
      <ul className="list-disc space-y-0.5 pl-5">
        {fuentes.map((clave) => {
          const f = metadatos.fuentes[clave]
          if (!f) return null
          return (
            <li key={clave}>
              <code className="text-texto font-mono">{f.ruta}</code>
              {f.filas !== undefined && <> · {numero(f.filas)} filas</>}
              {' · '}SHA-256 <code className="font-mono">{f.sha256.slice(0, 12)}…</code>
              {f.que_es && <> · {f.que_es}</>}
            </li>
          )
        })}
      </ul>
      <p>
        Lo calcula <code className="text-texto font-mono">{metadatos.generador}</code> y lo escribe en{' '}
        <code className="font-mono">/datos/impacto.json</code> (generado el {fechaDia(metadatos.generado)}). Ninguna cifra
        de esta página se escribe a mano.
      </p>
    </Desplegable>
  )
}

/** El encabezado de una sección de la página: su pregunta, en grande. */
export function Seccion({ id, pregunta, titulo, children }: { id: string; pregunta: string; titulo: string; children: ReactNode }) {
  return (
    <section aria-labelledby={id} className="border-borde bg-superficie rounded-xl border p-5">
      <header className="mb-4">
        <p className="rotulo">{pregunta}</p>
        <h2 id={id} className="text-texto text-lg font-semibold">
          {titulo}
        </h2>
      </header>
      <div className="space-y-4">{children}</div>
    </section>
  )
}
