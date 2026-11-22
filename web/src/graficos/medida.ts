// El tamaño en pantalla de un gráfico. Los gráficos de D3 se dibujan uno a uno
// con los píxeles de su caja (el viewBox mide lo mismo que la caja): así una
// marca o una letra no se achican cuando la caja es chica, y en el modo
// proyector, que agranda el rem, crecen con el resto de la página.

import { useLayoutEffect, useRef, useState, type RefObject } from 'react'

export interface Medida {
  ancho: number
  alto: number
  /** Cuánto más grande que 16 px es el rem: 1 en el modo normal, 1.1875 en el proyector. */
  escala: number
}

function escalaDeLaPagina(): number {
  const rem = Number.parseFloat(getComputedStyle(document.documentElement).fontSize)
  return Number.isFinite(rem) && rem > 0 ? rem / 16 : 1
}

/**
 * Mide la caja de `ref` y la vuelve a medir cada vez que cambia. Mientras no
 * se pueda medir (antes del primer dibujo, o en las pruebas, donde no hay
 * ResizeObserver) vale `porOmision`.
 */
export function useMedida<E extends Element>(porOmision: { ancho: number; alto: number }): [RefObject<E | null>, Medida] {
  const ref = useRef<E>(null)
  const [medida, setMedida] = useState<Medida>({ ...porOmision, escala: 1 })

  useLayoutEffect(() => {
    const caja = ref.current
    if (!caja || typeof ResizeObserver === 'undefined') return
    const medir = () => {
      const { width, height } = caja.getBoundingClientRect()
      const ancho = Math.round(width)
      const alto = Math.round(height)
      if (ancho === 0 || alto === 0) return
      const escala = escalaDeLaPagina()
      setMedida((antes) =>
        antes.ancho === ancho && antes.alto === alto && antes.escala === escala ? antes : { ancho, alto, escala },
      )
    }
    medir()
    const observador = new ResizeObserver(medir)
    observador.observe(caja)
    return () => observador.disconnect()
  }, [])

  return [ref, medida]
}
