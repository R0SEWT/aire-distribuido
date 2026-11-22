// El tamaño en pantalla de un gráfico: el único lugar que mide.
//
// Los gráficos de D3 se dibujan uno a uno con los píxeles de su caja, sin un
// viewBox que escale: así una marca o una letra no se achican cuando la caja es
// chica. Pero la app mide en rem, y el modo proyector agranda la raíz: por eso,
// además del ancho y el alto de la caja, hace falta saber cuánto vale 1 rem
// ahora. El gráfico calcula con eso sus márgenes; la letra sale de clases
// (text-xs…), que ya están en rem.

import { useLayoutEffect, useRef, useState, type RefObject } from 'react'

export interface Medida {
  /** El ancho de la caja, en píxeles enteros: lo dibujado nunca la desborda. */
  ancho: number
  /** El alto de la caja, en píxeles. Solo importa a los gráficos que llenan una caja de alto fijo. */
  alto: number
  /** Cuántos píxeles es 1 rem ahora: 16 en el modo normal y 19 en el proyector. */
  rem: number
  /** rem / 16: por cuánto se multiplica una medida pensada en píxeles del modo normal. */
  escala: number
}

const REM_POR_OMISION = 16

function remDeLaPagina(): number {
  const rem = Number.parseFloat(getComputedStyle(document.documentElement).fontSize)
  return Number.isFinite(rem) && rem > 0 ? rem : REM_POR_OMISION
}

/**
 * Mide la caja de `ref` y la vuelve a medir cuando cambia de tamaño o cuando
 * cambia el modo proyector (que mueve el rem aunque la caja conserve su ancho).
 * Mientras no se pueda medir (antes del primer dibujo, o en las pruebas, donde
 * no hay ResizeObserver) vale `porOmision`.
 *
 *   const [caja, { ancho, rem }] = useMedida<HTMLDivElement>({ ancho: 640 })
 *   return <div ref={caja} className="w-full"><svg width={ancho} …/></div>
 */
export function useMedida<E extends Element>(porOmision: { ancho: number; alto?: number }): [RefObject<E | null>, Medida] {
  const ref = useRef<E>(null)
  const [medida, setMedida] = useState<Medida>({
    ancho: porOmision.ancho,
    alto: porOmision.alto ?? 0,
    rem: REM_POR_OMISION,
    escala: 1,
  })

  useLayoutEffect(() => {
    const caja = ref.current
    if (!caja || typeof ResizeObserver === 'undefined') return
    const medir = () => {
      const { width, height } = caja.getBoundingClientRect()
      const ancho = Math.floor(width)
      // Una caja sin ancho no está a la vista (una pestaña oculta): se conserva lo último medido.
      if (ancho === 0) return
      const rem = remDeLaPagina()
      setMedida((antes) => {
        const alto = Math.floor(height) || antes.alto
        return antes.ancho === ancho && antes.alto === alto && antes.rem === rem
          ? antes
          : { ancho, alto, rem, escala: rem / REM_POR_OMISION }
      })
    }
    medir()
    const tamano = new ResizeObserver(medir)
    tamano.observe(caja)
    const modo = new MutationObserver(medir)
    modo.observe(document.documentElement, { attributes: true, attributeFilter: ['data-modo'] })
    return () => {
      tamano.disconnect()
      modo.disconnect()
    }
  }, [])

  return [ref, medida]
}
