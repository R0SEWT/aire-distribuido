// Antes de simular: ¿el sistema está limpio para este tramo? (spec §5.4)
//
// El último estado de una estación no retrocede: una lectura más antigua que la
// guardada se predice y se responde igual, pero no cambia el estado ni sale por
// el WebSocket (una de la misma hora, sí la reemplaza). Si una estación del
// tramo ya tiene un estado de una hora igual o posterior a su primera lectura,
// el panel no va a moverse con esas lecturas, y eso se dice antes de empezar.

import type { EstadoEstacion } from '../../api/enVivo'
import type { DiaDeSeries, Tramo } from './series'

/**
 * - dentro: el estado guardado cae en medio del tramo. Las lecturas hasta esa
 *   hora no cambian el estado; las siguientes, sí.
 * - misma: el estado guardado es de la última hora del tramo (ya se simuló):
 *   solo la última lectura lo reemplaza.
 * - posterior: el estado guardado es posterior a todo el tramo. Ninguna lectura
 *   lo cambia.
 */
export type TipoDeChoque = 'dentro' | 'misma' | 'posterior'

export interface Choque {
  estacion: string
  /** La fecha_hora del estado que la estación ya tiene. */
  guardada: string
  tipo: TipoDeChoque
}

export interface Revision {
  limpio: boolean
  choques: Choque[]
}

/** Compara el estado guardado de cada estación del tramo con las lecturas que se van a mandar. */
export function revisarSistema(tramo: Tramo, estados: readonly EstadoEstacion[]): Revision {
  const choques: Choque[] = []
  for (const estado of estados) {
    const primera = tramo.primera[estado.estacion]
    const ultima = tramo.ultima[estado.estacion]
    if (primera === undefined || ultima === undefined) continue
    const guardada = Date.parse(estado.fecha_hora)
    if (!Number.isFinite(guardada) || guardada < Date.parse(primera)) continue
    const fin = Date.parse(ultima)
    choques.push({
      estacion: estado.estacion,
      guardada: estado.fecha_hora,
      tipo: guardada > fin ? 'posterior' : guardada === fin ? 'misma' : 'dentro',
    })
  }
  choques.sort((a, b) => a.estacion.localeCompare(b.estacion))
  return { limpio: choques.length === 0, choques }
}

/** Si simular ese día todavía movería el estado de su estación: no tiene uno igual o posterior. */
export function diaDisponible(dia: DiaDeSeries, estados: readonly EstadoEstacion[]): boolean {
  const estado = estados.find((e) => e.estacion === dia.estacion)
  const primera = dia.lecturas[0]?.fecha_hora
  if (!estado || primera === undefined) return true
  return Date.parse(estado.fecha_hora) < Date.parse(primera)
}
