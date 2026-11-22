// Lo que la API respondió contra lo «esperado» de series.json (spec §7.3 y §12).
//
// El esperado lo calcula scripts/impacto.py con la regla de ventana/ventana.go,
// sin preguntarle a la API: si los dos coinciden, la API y el script llegaron
// al mismo número por caminos distintos.
//
// El régimen es aparte: el esperado vale para el modelo sembrado. Con otro
// modelo activo los índices pueden cambiar, y eso no es un error de la media,
// de la alerta ni del INCA, que no dependen del modelo.

import type { Seguimiento } from './motor'
import type { Esperado } from '../../api/datos'
import type { Tramo } from './series'

export type Campo =
  | 'lecturas_enviadas'
  | 'media_pm2_5'
  | 'media_pm10'
  | 'lecturas_en_ventana'
  | 'alerta'
  | 'inca'
  | 'regimen'
  | 'excede_eca'
  | 'primera_media'
  | 'primera_alerta'

export type Veredicto =
  /** Todavía no se mandaron todas las lecturas de la estación. */
  | 'incompleta'
  | 'coincide'
  /** Solo difiere el régimen (o su excede_eca): depende del modelo activo. */
  | 'solo_regimen'
  | 'difiere'

export interface Comparacion {
  estacion: string
  esperado: Esperado
  seguimiento: Seguimiento | null
  enviadas: number
  veredicto: Veredicto
  /** Los campos en que la respuesta no es la esperada. Vacío mientras esté incompleta. */
  diferencias: Campo[]
}

const DEL_MODELO: readonly Campo[] = ['regimen', 'excede_eca']

/** La API y Python promedian los mismos números en doble precisión: la diferencia, si la hay, es de redondeo. */
const TOLERANCIA = 1e-6

function mismoNumero(a: number | null, b: number | null): boolean {
  if (a === null || b === null) return a === b
  return Math.abs(a - b) <= TOLERANCIA * Math.max(1, Math.abs(a), Math.abs(b))
}

function mismoInstante(a: string | null, b: string | null): boolean {
  if (a === null || b === null) return a === b
  return Date.parse(a) === Date.parse(b)
}

export function compararEstacion(estacion: string, esperado: Esperado, seguimiento: Seguimiento | undefined): Comparacion {
  const enviadas = seguimiento?.puntos.length ?? 0
  const base = { estacion, esperado, seguimiento: seguimiento ?? null, enviadas }
  const r = seguimiento?.ultima
  if (!seguimiento || !r || enviadas < esperado.lecturas_enviadas) {
    return { ...base, veredicto: 'incompleta', diferencias: [] }
  }
  const pruebas: [Campo, boolean][] = [
    ['lecturas_enviadas', enviadas === esperado.lecturas_enviadas],
    ['media_pm2_5', mismoNumero(r.media_24h.pm2_5, esperado.media_24h.pm2_5)],
    ['media_pm10', mismoNumero(r.media_24h.pm10, esperado.media_24h.pm10)],
    ['lecturas_en_ventana', r.media_24h.lecturas === esperado.media_24h.lecturas],
    ['alerta', r.alerta === esperado.alerta],
    ['inca', r.inca === esperado.inca],
    ['regimen', r.regimen === esperado.regimen],
    ['excede_eca', r.excede_eca === esperado.excede_eca],
    ['primera_media', mismoInstante(seguimiento.primeraMedia, esperado.primera_media)],
    ['primera_alerta', mismoInstante(seguimiento.primeraAlerta, esperado.primera_alerta)],
  ]
  const diferencias = pruebas.filter(([, igual]) => !igual).map(([campo]) => campo)
  const veredicto: Veredicto =
    diferencias.length === 0 ? 'coincide' : diferencias.every((c) => DEL_MODELO.includes(c)) ? 'solo_regimen' : 'difiere'
  return { ...base, veredicto, diferencias }
}

/** La comparación de cada estación del tramo, por código. */
export function compararTramo(tramo: Tramo, porEstacion: Readonly<Record<string, Seguimiento>>): Comparacion[] {
  return tramo.estaciones.flatMap((estacion) => {
    const esperado = tramo.esperado[estacion]
    return esperado ? [compararEstacion(estacion, esperado, porEstacion[estacion])] : []
  })
}

export interface Resumen {
  estaciones: number
  completas: number
  coinciden: number
  soloRegimen: number
  difieren: number
  enAlerta: string[]
}

export function resumir(comparaciones: readonly Comparacion[]): Resumen {
  const cuantas = (v: Veredicto) => comparaciones.filter((c) => c.veredicto === v).length
  return {
    estaciones: comparaciones.length,
    completas: comparaciones.length - cuantas('incompleta'),
    coinciden: cuantas('coincide'),
    soloRegimen: cuantas('solo_regimen'),
    difieren: cuantas('difiere'),
    enAlerta: comparaciones.filter((c) => c.seguimiento?.ultima?.alerta).map((c) => c.estacion),
  }
}

/** Cómo se nombra cada campo cuando difiere. */
export const NOMBRE_DE_CAMPO: Record<Campo, string> = {
  lecturas_enviadas: 'lecturas enviadas',
  media_pm2_5: 'media de PM2.5',
  media_pm10: 'media de PM10',
  lecturas_en_ventana: 'lecturas en la ventana',
  alerta: 'alerta',
  inca: 'banda INCA',
  regimen: 'régimen',
  excede_eca: 'si el régimen excede el ECA',
  primera_media: 'hora de la primera media',
  primera_alerta: 'hora de la primera alerta',
}
