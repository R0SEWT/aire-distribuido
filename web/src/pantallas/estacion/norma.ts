// Los umbrales de la norma, para dibujarlos y para decirlos (spec §5.3). Son
// los del contrato (docs/api/openapi.yaml, esquemas Alerta y BandaINCA). La UI
// no decide con ellos si hay alerta ni en qué banda está una estación: eso lo
// manda la API. Sirven para la línea del ECA, las franjas del INCA y la frase.

import { numero } from '../../componentes/indice'
import type { RegimenEstacion } from '../panel/datos'
import { MINIMO_LECTURAS } from '../panel/situacion'

export type Contaminante = 'pm2_5' | 'pm10'

export interface Norma {
  nombre: string
  /** ECA de 24 horas, en µg/m³ (DS 003-2017-MINAM). */
  eca: number
  /** Hasta dónde llegan las bandas buena, moderada y mala del INCA (RM 181-2016-MINAM); más arriba, umbral de cuidado. */
  cortes: readonly [number, number, number]
}

export const NORMA: Record<Contaminante, Norma> = {
  pm2_5: { nombre: 'PM2.5', eca: 50, cortes: [12.5, 25, 125] },
  pm10: { nombre: 'PM10', eca: 100, cortes: [75, 150, 250] },
}

const supera = (valor: number | null, c: Contaminante) => valor !== null && valor > NORMA[c].eca

/** El contaminante con que abre la curva: el que supera el ECA, si alguno lo hace. */
export function contaminanteInicial(estado: RegimenEstacion | null): Contaminante {
  if (!estado) return 'pm10'
  if (supera(estado.media_24h.pm10, 'pm10')) return 'pm10'
  if (supera(estado.media_24h.pm2_5, 'pm2_5')) return 'pm2_5'
  return 'pm10'
}

const ug = (valor: number) => `${numero(valor, 1)} µg/m³`

/**
 * La frase en llano de la estación (spec §5.3): «La media de las últimas 24
 * horas de PM10 es 121.4 µg/m³: supera el ECA (100)». Va con un decimal: con
 * 50.3 redondeado a 50, «supera el ECA (50)» no se entendería.
 */
export function fraseEstacion(estado: RegimenEstacion): string {
  const { pm2_5, pm10, lecturas } = estado.media_24h
  if (pm2_5 === null || pm10 === null) {
    return `Todavía no hay media de 24 horas: la ventana tiene ${lecturas} ${lecturas === 1 ? 'lectura' : 'lecturas'} y hacen falta ${MINIMO_LECTURAS}.`
  }
  const s25 = supera(pm2_5, 'pm2_5')
  const s10 = supera(pm10, 'pm10')
  // Si la comparación de la UI y la alerta de la API no dijeran lo mismo, manda la API y no se dan razones.
  if ((s25 || s10) !== estado.alerta) {
    return estado.alerta
      ? 'La estación está en alerta: su media de las últimas 24 horas supera el ECA.'
      : 'La estación no está en alerta: su media de las últimas 24 horas no supera el ECA.'
  }
  if (s25 && s10) {
    return `La media de las últimas 24 horas de PM2.5 es ${ug(pm2_5)} y la de PM10 es ${ug(pm10)}: las dos superan el ECA (${NORMA.pm2_5.eca} y ${NORMA.pm10.eca}).`
  }
  if (s10) {
    return `La media de las últimas 24 horas de PM10 es ${ug(pm10)}: supera el ECA (${NORMA.pm10.eca}). La de PM2.5 es ${ug(pm2_5)}: no lo supera (${NORMA.pm2_5.eca}).`
  }
  if (s25) {
    return `La media de las últimas 24 horas de PM2.5 es ${ug(pm2_5)}: supera el ECA (${NORMA.pm2_5.eca}). La de PM10 es ${ug(pm10)}: no lo supera (${NORMA.pm10.eca}).`
  }
  return `La media de las últimas 24 horas de PM2.5 es ${ug(pm2_5)} y la de PM10 es ${ug(pm10)}: ninguna supera el ECA (${NORMA.pm2_5.eca} y ${NORMA.pm10.eca}).`
}

/** «10 dic 2021» a partir de «2021-12-10»: un día sin hora no se pasa por ninguna zona horaria. */
export function fechaDia(dia: string): string {
  const fecha = new Date(`${dia}T00:00:00Z`)
  if (Number.isNaN(fecha.getTime())) return '—'
  return new Intl.DateTimeFormat('es-PE', { timeZone: 'UTC', day: 'numeric', month: 'short', year: 'numeric' }).format(fecha)
}
