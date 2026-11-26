// La reproducción de las rondas a ritmo humano (spec §5.5).
//
// Un entrenamiento real dura unos 50 ms: sus 44 rondas llegan por el WebSocket
// casi juntas. La pantalla las recibe todas y las va mostrando de a una, cada
// `pasoMs`. Si las rondas llegan más despacio que eso (una medición larga), se
// muestran a medida que llegan: nunca se muestra una ronda que no llegó.

import { useCallback, useEffect, useState } from 'react'

export type Velocidad = 'lenta' | 'normal' | 'rapida'

/** Cuánto dura cada ronda en pantalla, en milisegundos. */
export const PASO_MS: Record<Velocidad, number> = { lenta: 300, normal: 120, rapida: 40 }

export interface Reproduccion {
  /** Cuántas rondas se muestran ya, de 0 a `recibidas`. */
  mostradas: number
  /** Muestra de una vez todas las recibidas. */
  irAlFinal: () => void
  /** Vuelve a empezar desde la primera ronda. */
  repetir: () => void
}

/**
 * Dónde recordar por qué ronda va la reproducción de cada entrenamiento, para
 * retomarla ahí si la pantalla se desmonta y se vuelve a montar.
 */
export interface MemoriaDeReproduccion {
  leer: (clave: string) => number | undefined
  guardar: (clave: string, mostradas: number) => void
}

interface Estado {
  clave: string | null
  mostradas: number
}

/**
 * @param clave      qué entrenamiento se reproduce; al cambiar, la reproducción empieza de nuevo
 * @param recibidas  cuántas rondas hay para mostrar hasta ahora
 * @param pasoMs     cuánto dura cada ronda en pantalla
 * @param completa   si empieza ya con todas a la vista (uno abierto desde el historial)
 * @param memoria    si se da, la reproducción de un entrenamiento ya visto se retoma donde iba
 */
export function useReproduccion(
  clave: string | null,
  recibidas: number,
  pasoMs: number,
  completa = false,
  memoria?: MemoriaDeReproduccion,
): Reproduccion {
  const alEmpezar = (): number => (clave === null ? undefined : memoria?.leer(clave)) ?? (completa ? recibidas : 0)
  const [estado, setEstado] = useState<Estado>(() => ({ clave, mostradas: alEmpezar() }))
  // Otro entrenamiento: se ajusta el estado en el mismo render, sin esperar a un efecto.
  if (estado.clave !== clave) setEstado({ clave, mostradas: alEmpezar() })
  const mostradas = Math.min(estado.clave === clave ? estado.mostradas : alEmpezar(), recibidas)

  useEffect(() => {
    if (clave !== null) memoria?.guardar(clave, mostradas)
  }, [clave, mostradas, memoria])

  // El reloj depende de si queda algo por mostrar, no de cuántas rondas hay: si
  // dependiera de `recibidas`, cada ronda que llega lo reiniciaría, y con rondas
  // que llegan más seguido que el paso (una medición larga) no avanzaría nunca.
  const quedan = mostradas < recibidas
  useEffect(() => {
    if (!quedan) return
    const reloj = setTimeout(() => {
      setEstado((e) => (e.clave === clave ? { clave, mostradas: e.mostradas + 1 } : e))
    }, pasoMs)
    return () => clearTimeout(reloj)
  }, [clave, mostradas, quedan, pasoMs])

  const irAlFinal = useCallback(() => setEstado({ clave, mostradas: recibidas }), [clave, recibidas])
  const repetir = useCallback(() => setEstado({ clave, mostradas: 0 }), [clave])

  return { mostradas, irAlFinal, repetir }
}
