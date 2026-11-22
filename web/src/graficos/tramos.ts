// Dónde se corta la línea de una serie horaria. Una media de 24 horas no es
// continua a través de un hueco grande: entre el último punto de un tramo y el
// primero del siguiente pudo pasar cualquier cosa, y unirlos con una recta la
// inventaría.

const HORA_MS = 3_600_000

/**
 * Con más horas que estas entre dos puntos seguidos, la línea no los une. Son
 * varias y no una porque de una ráfaga de lecturas no llega cada estado: la API
 * manda, a lo sumo, un evento por estación cada 250 ms, y a ritmo rápido eso
 * son unas 3 horas simuladas entre un estado recibido y el siguiente.
 */
export const HUECO_QUE_CORTA_H = 6

interface Punto {
  /** El instante, en milisegundos. */
  t: number
  /** null donde no hay valor (la ventana no tenía lecturas suficientes). */
  valor: number | null
}

/**
 * Parte una serie, ya ordenada por tiempo, en los tramos que una línea puede
 * unir: se corta donde un punto no tiene valor y donde entre dos seguidos hay
 * más de `huecoMaximoH` horas. Los puntos sin valor no entran en ningún tramo.
 */
export function partirPorHuecos<P extends Punto>(puntos: readonly P[], huecoMaximoH = HUECO_QUE_CORTA_H): (P & { valor: number })[][] {
  const tramos: (P & { valor: number })[][] = []
  let actual: (P & { valor: number })[] = []
  let anterior: P | undefined
  for (const punto of puntos) {
    const hayHueco = anterior !== undefined && punto.t - anterior.t > huecoMaximoH * HORA_MS
    if ((punto.valor === null || hayHueco) && actual.length > 0) {
      tramos.push(actual)
      actual = []
    }
    if (punto.valor !== null) actual.push(punto as P & { valor: number })
    anterior = punto
  }
  if (actual.length > 0) tramos.push(actual)
  return tramos
}

/** Cuántos huecos de más de `huecoMaximoH` horas hay entre puntos seguidos de la serie, tengan valor o no. */
export function contarHuecos(puntos: readonly { t: number }[], huecoMaximoH = HUECO_QUE_CORTA_H): number {
  let huecos = 0
  for (let i = 1; i < puntos.length; i++) if (puntos[i]!.t - puntos[i - 1]!.t > huecoMaximoH * HORA_MS) huecos++
  return huecos
}
