// Piezas chicas que comparten las secciones de Impacto.

import type { ReactNode } from 'react'
import { ENLACES } from '../../armazon/enlaces'
import { Desplegable, fechaDia, numero } from '../../componentes/indice'
import type { Impacto } from '../../api/datos'

interface PropsDeDondeSale {
  impacto: Impacto
  /** Las claves de metadatos.fuentes de las que sale la cifra. */
  fuentes: readonly string[]
  children: ReactNode
}

const NOMBRES_DE_FUENTE: Record<string, string> = {
  gold: 'Lecturas horarias usadas en el análisis',
  silver: 'Concentraciones antes de estandarizar',
  asignaciones: 'Régimen asignado a cada lectura',
  regimenes: 'Perfil de los regímenes del modelo',
  alerta_24h: 'Comprobación de la regla de alerta en Go',
  ficha: 'Ubicación y actividad vigilada por cada estación',
  poblacion: 'Población censada por distrito y su fuente',
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
              <strong className="text-texto">{NOMBRES_DE_FUENTE[clave] ?? 'Archivo de origen'}</strong>{': '}
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
      <p>
        El contexto del análisis está en el{' '}
        <a href={ENLACES.informe} target="_blank" rel="noopener noreferrer">informe de PC3 y PC4</a>. Las cifras nuevas
        de esta pantalla proceden del archivo de impacto indicado arriba.
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
