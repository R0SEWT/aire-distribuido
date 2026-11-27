// Las marcas de un eje de tiempo, en la hora de Lima.
//
// d3 elige marcas «redondas» (las 00:00, las 06:00…) en la zona del navegador
// (scaleTime) o en UTC (scaleUtc). Los datos de OEFA y toda la app van en hora
// de Lima: en un navegador de otra zona, las marcas de scaleTime caerían en
// horas que en Lima no son redondas (desde la India, a las y media).

import { scaleUtc } from 'd3'

/** Lima no tiene horario de verano: siempre UTC−5. */
const LIMA_MS = -5 * 3_600_000

/**
 * Unas `cuantas` marcas entre dos instantes (en milisegundos), en horas
 * redondas de Lima sea cual sea la zona del navegador. Se corre el intervalo a
 * la hora de Lima, se le piden las marcas a d3 en UTC y se las corre de vuelta.
 */
export function marcasEnHoraDeLima(desde: number, hasta: number, cuantas: number): number[] {
  return scaleUtc()
    .domain([desde + LIMA_MS, hasta + LIMA_MS])
    .ticks(cuantas)
    .map((marca) => marca.getTime() - LIMA_MS)
}
