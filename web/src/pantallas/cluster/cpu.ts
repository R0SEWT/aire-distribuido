// El uso de CPU del proceso de la API (el coordinador), calculado como dice el
// contrato: /api/metricas publica el tiempo de CPU ACUMULADO desde que arrancó
// el proceso, no un porcentaje. El uso entre dos consultas es
//
//   Δ(cpu_usuario_s + cpu_sistema_s) / Δen_marcha_s
//
// donde 1 es un núcleo ocupado todo ese tiempo y el tope es `nucleos`.
// No es el CPU de los nodos: el protocolo del clúster no lo transporta.

import type { components } from '../../api/contrato'

export type Proceso = components['schemas']['ProcesoAPI']

export interface UsoDeCpu {
  /** Núcleos ocupados en el intervalo: de 0 a `nucleos`. 1 es un núcleo entero. */
  nucleos: number
  /** Lo mismo como porcentaje de UN núcleo: 100 es un núcleo entero; el tope es 100 × nucleos. */
  porcentaje: number
  /** Los segundos de reloj entre las dos muestras. */
  intervaloS: number
}

function cpuTotal(p: Proceso): number {
  return p.cpu_usuario_s + p.cpu_sistema_s
}

function acotar(nucleos: number, tope: number): number {
  return Math.min(Math.max(nucleos, 0), Math.max(tope, 1))
}

/**
 * El uso de CPU entre dos muestras de `proceso_api`. Devuelve null si no se
 * puede calcular: son la misma muestra, o la API se reinició entre las dos (el
 * reloj `en_marcha_s` o el CPU acumulado retrocedieron).
 */
export function usoDeCpu(anterior: Proceso, actual: Proceso): UsoDeCpu | null {
  const intervaloS = actual.en_marcha_s - anterior.en_marcha_s
  const cpu = cpuTotal(actual) - cpuTotal(anterior)
  if (!(intervaloS > 0) || !(cpu >= 0)) return null
  const nucleos = acotar(cpu / intervaloS, actual.nucleos)
  return { nucleos, porcentaje: nucleos * 100, intervaloS }
}

/** Con una sola muestra: el promedio desde que arrancó el proceso. */
export function usoPromedio(p: Proceso): UsoDeCpu | null {
  if (!(p.en_marcha_s > 0)) return null
  const nucleos = acotar(cpuTotal(p) / p.en_marcha_s, p.nucleos)
  return { nucleos, porcentaje: nucleos * 100, intervaloS: p.en_marcha_s }
}

/** Si esta plataforma no mide el tiempo de CPU (Windows): los campos llegan en 0 y `no_disponible` lo dice. */
export function sinMedicionDeCpu(p: Proceso): boolean {
  return p.no_disponible?.some((c) => c === 'cpu_usuario_s' || c === 'cpu_sistema_s') ?? false
}

/** Cuántas muestras se guardan para la curva: con una cada 2 s, los últimos 3 minutos. */
export const MUESTRAS_MAXIMAS = 90

/**
 * Suma una muestra a las guardadas. Una muestra repetida (el mismo instante) no
 * se suma, y si la API se reinició (su reloj retrocedió) se empieza de nuevo:
 * el CPU acumulado de antes ya no se puede comparar con el de ahora.
 */
export function agregarMuestra(muestras: readonly Proceso[], nueva: Proceso, maximo = MUESTRAS_MAXIMAS): Proceso[] {
  const ultima = muestras[muestras.length - 1]
  if (ultima && nueva.en_marcha_s === ultima.en_marcha_s) return muestras as Proceso[]
  if (ultima && nueva.en_marcha_s < ultima.en_marcha_s) return [nueva]
  return [...muestras, nueva].slice(-maximo)
}

export interface PuntoDeUso {
  /** El instante de la muestra, en segundos desde que arrancó la API. */
  t: number
  /** El uso en el intervalo que termina en `t`, como porcentaje de un núcleo. */
  porcentaje: number
}

/** La serie del uso de CPU: un punto por cada par de muestras seguidas. */
export function serieDeUso(muestras: readonly Proceso[]): PuntoDeUso[] {
  const serie: PuntoDeUso[] = []
  for (let i = 1; i < muestras.length; i++) {
    const uso = usoDeCpu(muestras[i - 1]!, muestras[i]!)
    if (uso) serie.push({ t: muestras[i]!.en_marcha_s, porcentaje: uso.porcentaje })
  }
  return serie
}
