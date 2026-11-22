import { useEffect, useState } from 'react'

/**
 * Si todavía se está dentro de los `duracionMs` que siguen al instante `desde`
 * (en ms de reloj; null si no hay tal instante). Al cumplirse, el componente se
 * vuelve a dibujar solo.
 */
export function useVentana(desde: number | null, duracionMs: number): boolean {
  // El instante cuya ventana ya se cumplió.
  const [cumplida, setCumplida] = useState<number | null>(null)

  useEffect(() => {
    if (desde === null) return
    const reloj = setTimeout(() => setCumplida(desde), Math.max(0, desde + duracionMs - Date.now()))
    return () => clearTimeout(reloj)
  }, [desde, duracionMs])

  return desde !== null && cumplida !== desde
}
