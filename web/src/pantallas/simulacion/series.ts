// La forma de /datos/series.json (lo escribe scripts/impacto.py; spec §7.3) y lo
// que la simulación saca de él: un «tramo» es lo que se manda de una vez, sea el
// escenario (48 horas de varias estaciones) o un día suelto de una estación.

import type { components } from '../../api/contrato'

export type Lectura = components['schemas']['Lectura']
export type Prediccion = components['schemas']['Prediccion']
export type BandaINCA = components['schemas']['BandaINCA']

/** Lo que la API tiene que responder en la última lectura de una estación del tramo. */
export interface Esperado {
  fecha_hora: string
  lecturas_enviadas: number
  regimen: number
  excede_eca: boolean
  media_24h: { pm2_5: number | null; pm10: number | null; lecturas: number }
  alerta: boolean
  inca: BandaINCA
  /** La fecha_hora de la primera lectura que devuelve media; null si ninguna. */
  primera_media: string | null
  /** La fecha_hora de la primera lectura que devuelve alerta = true; null si ninguna. */
  primera_alerta: string | null
}

export interface ReglasDeSeries {
  ventana_horas: number
  minimo_lecturas: number
  eca_pm2_5: number
  eca_pm10: number
  alerta: string
  inca: { bandas: string[]; cortes_pm10: number[]; cortes_pm2_5: number[]; regla: string }
  dia: string
}

export interface EscenarioDeSeries {
  id: string
  desde: string
  hasta: string
  dias: string[]
  horas: number
  criterio: string
  estaciones: { estacion: string; lecturas: number; con_media_al_final: boolean }[]
  estaciones_sin_lecturas: string[]
  lecturas: Lectura[]
  esperado: Record<string, Esperado>
}

export interface DiaDeSeries {
  id: string
  estacion: string
  dia: string
  caso: string
  descripcion: string
  desde: string
  hasta: string
  horas: number
  huecos: number
  lecturas: Lectura[]
  esperado: Esperado
}

export interface Series {
  metadatos: {
    generado: string
    generador: string
    reglas: ReglasDeSeries
    como_se_usa: { envio: string; esperado: string; regimen: string; orden: string }
  }
  escenario: EscenarioDeSeries
  dias: DiaDeSeries[]
}

/** Lo que se manda de una vez a POST /api/predicciones, en el orden del archivo. */
export interface Tramo {
  id: string
  tipo: 'escenario' | 'dia'
  titulo: string
  desde: string
  hasta: string
  lecturas: readonly Lectura[]
  /** Las estaciones que tienen lecturas en el tramo, por código. */
  estaciones: readonly string[]
  esperado: Readonly<Record<string, Esperado>>
  /** La fecha_hora de la primera y de la última lectura de cada estación. */
  primera: Readonly<Record<string, string>>
  ultima: Readonly<Record<string, string>>
}

function armarTramo(base: Pick<Tramo, 'id' | 'tipo' | 'titulo' | 'desde' | 'hasta' | 'lecturas' | 'esperado'>): Tramo {
  const primera: Record<string, string> = {}
  const ultima: Record<string, string> = {}
  for (const l of base.lecturas) {
    primera[l.estacion] ??= l.fecha_hora
    ultima[l.estacion] = l.fecha_hora
  }
  return { ...base, estaciones: Object.keys(primera).sort(), primera, ultima }
}

export function tramoDelEscenario(escenario: EscenarioDeSeries): Tramo {
  return armarTramo({
    id: escenario.id,
    tipo: 'escenario',
    titulo: 'El escenario',
    desde: escenario.desde,
    hasta: escenario.hasta,
    lecturas: escenario.lecturas,
    esperado: escenario.esperado,
  })
}

export function tramoDelDia(dia: DiaDeSeries): Tramo {
  return armarTramo({
    id: dia.id,
    tipo: 'dia',
    titulo: `${dia.estacion} · ${dia.dia}`,
    desde: dia.desde,
    hasta: dia.hasta,
    lecturas: dia.lecturas,
    esperado: { [dia.estacion]: dia.esperado },
  })
}

/** Las estaciones con PM por hora (las 17 del gold): las del escenario y las que no midieron en él. */
export function estacionesConPM(series: Series): string[] {
  return [...series.escenario.estaciones.map((e) => e.estacion), ...series.escenario.estaciones_sin_lecturas].sort()
}

/** Cómo se nombra en la pantalla cada caso de un día suelto. */
export const CASOS: Record<string, string> = {
  limpio: 'Día limpio',
  mala_sin_alerta: 'INCA mala sin alerta',
  cruza_eca: 'Cruza el ECA',
  en_alerta: 'En alerta todo el tramo',
  umbral_de_cuidado: 'Umbral de cuidado',
  huecos_con_media: 'Con huecos y con media',
  huecos_sin_media: 'Con huecos y sin media',
}

export function nombreDelCaso(caso: string): string {
  return CASOS[caso] ?? caso.replaceAll('_', ' ')
}

/** Si lo leído tiene la forma mínima de series.json: lo demás se confía al script que lo escribe. */
export function esSeries(x: unknown): x is Series {
  if (typeof x !== 'object' || x === null) return false
  const s = x as Partial<Series>
  return Array.isArray(s.dias) && Array.isArray(s.escenario?.lecturas) && typeof s.metadatos?.reglas === 'object'
}
