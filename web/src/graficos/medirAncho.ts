// Cuánto mide el contenedor de un gráfico, para dibujarlo a su ancho.
//
// Los gráficos de D3 se dibujan en píxeles, pero la app mide en rem: en modo
// proyector la raíz crece y con ella las tarjetas. Este hook da el ancho del
// contenedor y cuánto vale un rem ahora, y avisa cuando cambian; el gráfico
// calcula con eso sus márgenes y su alto, y la letra sale de clases (text-xs…),
// que ya están en rem.

import { useLayoutEffect, useState, type RefObject } from 'react'

export interface Medidas {
  /** El ancho del contenedor, en píxeles. */
  ancho: number
  /** Cuántos píxeles es 1 rem ahora (16 en modo normal, 19 en modo proyector). */
  rem: number
}

// Donde no hay con qué medir (las pruebas, en jsdom) el gráfico se dibuja igual, a un tamaño fijo.
const SIN_MEDIR: Medidas = { ancho: 640, rem: 16 }

function medir(elemento: Element): Medidas {
  const ancho = elemento.getBoundingClientRect().width
  const rem = Number.parseFloat(getComputedStyle(document.documentElement).fontSize)
  return {
    ancho: ancho > 0 ? ancho : SIN_MEDIR.ancho,
    rem: Number.isFinite(rem) && rem > 0 ? rem : SIN_MEDIR.rem,
  }
}

/** Las medidas del elemento de `ref`, al día mientras esté montado. */
export function useMedidas(ref: RefObject<Element | null>): Medidas {
  const [medidas, setMedidas] = useState(SIN_MEDIR)

  useLayoutEffect(() => {
    const elemento = ref.current
    if (!elemento) return
    const actualizar = () => {
      const nuevas = medir(elemento)
      setMedidas((antes) => (antes.ancho === nuevas.ancho && antes.rem === nuevas.rem ? antes : nuevas))
    }
    actualizar()
    if (typeof ResizeObserver === 'undefined') return
    const observador = new ResizeObserver(actualizar)
    observador.observe(elemento)
    // El modo proyector cambia el rem (data-modo en la raíz) aunque el ancho quede igual.
    const modo = new MutationObserver(actualizar)
    modo.observe(document.documentElement, { attributes: true, attributeFilter: ['data-modo'] })
    return () => {
      observador.disconnect()
      modo.disconnect()
    }
  }, [ref])

  return medidas
}
