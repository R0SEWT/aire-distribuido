// Lo común de los gráficos de Simulación e Impacto: cuánto mide el lugar donde
// se dibuja y un efecto que le entrega a D3 su nodo.
//
// Los gráficos se dibujan en píxeles reales, no con un viewBox que escale: así
// las letras miden en rem (clases de Tailwind sobre <text>) y el modo proyector
// las agranda igual que al resto de la app. Por eso hace falta saber el ancho
// del contenedor y cuánto vale 1 rem en este momento.

import { useEffect, useLayoutEffect, useRef, useState, type DependencyList, type RefObject } from 'react'
import { useModoProyector } from '../armazon/modo'

export interface Medidas {
  /** El ancho del contenedor, en píxeles. */
  ancho: number
  /** Cuánto vale 1 rem, en píxeles: 16 en modo normal y 19 en modo proyector. */
  rem: number
}

// Sin maquetado (las pruebas en jsdom) se dibuja con estas medidas.
const POR_OMISION: Medidas = { ancho: 760, rem: 16 }

function medir(elemento: Element): Medidas {
  const ancho = elemento.clientWidth || POR_OMISION.ancho
  const rem = Number.parseFloat(getComputedStyle(document.documentElement).fontSize) || POR_OMISION.rem
  return { ancho, rem }
}

/** El ancho del elemento y el valor de 1 rem, al día cuando cambia el tamaño o el modo proyector. */
export function useMedidas<E extends Element>(): [RefObject<E | null>, Medidas] {
  const ref = useRef<E>(null)
  const [medidas, setMedidas] = useState(POR_OMISION)
  const [proyector] = useModoProyector()

  useLayoutEffect(() => {
    const elemento = ref.current
    if (!elemento) return
    const actualizar = () => {
      const m = medir(elemento)
      setMedidas((antes) => (antes.ancho === m.ancho && antes.rem === m.rem ? antes : m))
    }
    if (typeof ResizeObserver === 'undefined') return
    // El observador avisa una vez al empezar a mirar y después con cada cambio de tamaño.
    const observador = new ResizeObserver(actualizar)
    observador.observe(elemento)
    return () => observador.disconnect()
    // El modo proyector cambia el rem aunque el contenedor conserve su ancho.
  }, [proyector])

  return [ref, medidas]
}

/**
 * Le entrega a D3 el nodo donde dibuja, cada vez que cambian las dependencias.
 * Quien dibuja usa .join(): no borra y vuelve a crear, actualiza lo que hay.
 */
export function useDibujo<E extends Element>(dibujar: (nodo: E) => void, dependencias: DependencyList): RefObject<E | null> {
  const ref = useRef<E>(null)
  useEffect(() => {
    if (ref.current) dibujar(ref.current)
    // Las dependencias las da quien dibuja: son los datos y las medidas del gráfico.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, dependencias)
  return ref
}
