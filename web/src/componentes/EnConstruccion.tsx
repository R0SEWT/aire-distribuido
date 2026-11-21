import type { ReactNode } from 'react'

interface Props {
  /** El nombre de la pantalla. */
  pantalla: string
  /** La sección de la spec que la describe: «§5.2». */
  seccion: string
  /** El archivo donde se construye, para quien la tome. */
  archivo: string
  /** Qué va a mostrar, en una o dos frases. */
  children: ReactNode
}

/**
 * El marcador de una pantalla que todavía no se construyó. La ruta, la guarda
 * y el menú ya están: quien la tome reemplaza este marcador en su archivo de
 * src/pantallas/ sin tocar el armazón.
 */
export function EnConstruccion({ pantalla, seccion, archivo, children }: Props) {
  return (
    <div data-en-construccion={pantalla} className="mx-auto max-w-3xl">
      <h1 className="text-texto text-2xl font-semibold">{pantalla}</h1>
      <div className="border-borde bg-superficie border-l-aviso mt-4 rounded-xl border border-l-4 p-5">
        <p className="rotulo text-aviso">En construcción</p>
        <div className="text-texto mt-2 space-y-2 leading-relaxed">{children}</div>
        <p className="text-tenue mt-4 text-sm">
          Spec del frontend, {seccion} · <code className="font-mono">{archivo}</code>
        </p>
      </div>
    </div>
  )
}
