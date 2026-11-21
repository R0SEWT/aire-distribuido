// Los eventos del WebSocket (/ws). El contrato los describe en una extensión
// (x-websocket-events), que openapi-typescript no convierte: se escriben a mano
// contra los mismos esquemas (spec §9.3).
//
// Cada trama es {"evento": "<nombre>", "datos": {...}}. Para sumar un evento,
// se agrega aquí su nombre con el tipo de sus datos.

import type { components } from './contrato'

type Esquemas = components['schemas']

export interface EventosWS {
  /** Solo admin. Una ronda de un entrenamiento. */
  'entrenamiento.ronda': Esquemas['EventoRonda']
  /** Solo admin. El entrenamiento terminó (EventoFin) o abortó (EventoError, trae `error`). */
  'entrenamiento.fin': Esquemas['EventoFin'] | Esquemas['EventoError']
  /**
   * Para todos. El último estado de una estación cambió (spec §6). La API lo
   * suma al contrato como EstadoEstacion junto con GET /api/regimenes; cuando
   * esté en develop, `pnpm tipos` y esto pasa a ser Esquemas['EstadoEstacion'].
   */
  'estacion.regimen': Esquemas['RegimenEstacion'] & { estacion: string }
}

export type NombreEvento = keyof EventosWS
