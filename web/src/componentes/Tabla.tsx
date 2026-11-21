import type { ReactNode } from 'react'

export interface Columna<F> {
  clave: string
  titulo: ReactNode
  /** Los números van a la derecha y en monoespaciada, para que las cifras se alineen. */
  numerica?: boolean
  celda: (fila: F) => ReactNode
}

interface Props<F> {
  /** Qué lista es; se lee como título de la tabla. */
  titulo: string
  /** Si el título se ve o solo lo leen los lectores de pantalla. */
  tituloVisible?: boolean
  columnas: readonly Columna<F>[]
  filas: readonly F[]
  claveDe: (fila: F) => string
  /** Lo que se dice cuando no hay filas. */
  vacio?: ReactNode
}

/** Una tabla densa: encabezados chicos, filas separadas por una línea fina y números alineados. */
export function Tabla<F>({ titulo, tituloVisible = false, columnas, filas, claveDe, vacio = 'No hay filas.' }: Props<F>) {
  return (
    <div className="overflow-x-auto">
      <table className="w-full border-collapse text-sm">
        <caption className={tituloVisible ? 'text-texto mb-2 text-left text-base font-semibold' : 'sr-only'}>
          {titulo}
        </caption>
        <thead>
          <tr className="border-borde border-b">
            {columnas.map((c) => (
              <th key={c.clave} scope="col" className={`rotulo px-3 py-2 ${c.numerica ? 'text-right' : 'text-left'}`}>
                {c.titulo}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {filas.length === 0 ? (
            <tr>
              <td colSpan={columnas.length} className="text-suave px-3 py-6 text-center">
                {vacio}
              </td>
            </tr>
          ) : (
            filas.map((fila) => (
              <tr key={claveDe(fila)} className="border-borde/60 border-b last:border-b-0">
                {columnas.map((c) => (
                  <td
                    key={c.clave}
                    className={`px-3 py-2 align-middle ${c.numerica ? 'text-right font-mono tabular-nums' : 'text-left'}`}
                  >
                    {c.celda(fila)}
                  </td>
                ))}
              </tr>
            ))
          )}
        </tbody>
      </table>
    </div>
  )
}
