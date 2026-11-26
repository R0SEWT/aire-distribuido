// Para los gráficos que D3 dibuja por su cuenta (con .join() sobre un nodo), en
// vez de que React arme el SVG.

import { useEffect, useRef, type DependencyList, type RefObject } from 'react'

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
