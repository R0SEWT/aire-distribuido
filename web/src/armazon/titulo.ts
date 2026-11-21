import { useEffect } from 'react'
import { NOMBRE } from './menu'

/**
 * Pone el título del documento mientras el componente esté a la vista:
 * useTitulo('CA-AYAS-01 · Estación') da «CA-AYAS-01 · Estación · Aire distribuido».
 *
 * El armazón ya pone el de cada ruta («Estación · Aire distribuido»). Una
 * pantalla lo usa solo si quiere uno más preciso.
 */
export function useTitulo(titulo: string) {
  useEffect(() => {
    document.title = `${titulo} · ${NOMBRE}`
  }, [titulo])
}
