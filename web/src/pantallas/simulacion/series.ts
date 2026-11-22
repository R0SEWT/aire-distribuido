// Lo que la simulación saca de /datos/series.json (su forma está en
// api/datos.ts): un «tramo» es lo que se manda de una vez, sea el escenario
// (48 horas de varias estaciones) o un día suelto de una estación.

import type { components } from '../../api/contrato'
import type { DiaDeSeries, EscenarioDeSeries, Esperado, Series } from '../../api/datos'

export type Lectura = components['schemas']['Lectura']
export type Prediccion = components['schemas']['Prediccion']

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
