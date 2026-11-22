// Las lecturas que la UI mandó en esta sesión, con lo que la API respondió a
// cada una. No hay una serie histórica desde la base (spec §6): la curva de una
// estación (§5.3) se arma con lo que la UI vio, y esto es lo que vio.
//
// Es un almacén fuera de React, como la sesión: el simulador escribe aunque su
// pantalla no esté a la vista. Se vacía al terminar la sesión.

import { useSyncExternalStore } from 'react'
import { sesionActual, suscribir as suscribirASesion } from '../../sesion/almacen'
import type { BandaINCA } from './series'

/** Una lectura enviada y la respuesta de la API a esa lectura. */
export interface Punto {
  fecha_hora: string
  pm2_5: number
  pm10: number
  /** La media de 24 h que respondió la API; null con menos de 18 lecturas en la ventana. */
  media_pm2_5: number | null
  media_pm10: number | null
  /** Cuántas lecturas tenía la ventana de 24 h. */
  lecturas: number
  alerta: boolean
  inca: BandaINCA
  regimen: number
  excede_eca: boolean
}

const VACIO: readonly Punto[] = []
const porEstacion = new Map<string, readonly Punto[]>()
const oyentes = new Set<() => void>()

function avisar() {
  for (const oyente of [...oyentes]) oyente()
}

/** Anota una lectura vista. Una de la misma hora reemplaza a la anterior, como en la API. */
export function anotarVista(estacion: string, punto: Punto) {
  const t = Date.parse(punto.fecha_hora)
  const otras = (porEstacion.get(estacion) ?? VACIO).filter((p) => Date.parse(p.fecha_hora) !== t)
  porEstacion.set(
    estacion,
    [...otras, punto].sort((a, b) => Date.parse(a.fecha_hora) - Date.parse(b.fecha_hora)),
  )
  avisar()
}

/** Lo visto de una estación en esta sesión, de la lectura más antigua a la más nueva. */
export function lecturasVistas(estacion: string): readonly Punto[] {
  return porEstacion.get(estacion) ?? VACIO
}

export function olvidarVistas() {
  if (porEstacion.size === 0) return
  porEstacion.clear()
  avisar()
}

function suscribir(oyente: () => void) {
  oyentes.add(oyente)
  return () => oyentes.delete(oyente)
}

/** Lo visto de una estación, dentro de React: se vuelve a dibujar con cada lectura nueva. */
export function useLecturasVistas(estacion: string): readonly Punto[] {
  return useSyncExternalStore(suscribir, () => lecturasVistas(estacion))
}

// Lo que vio una sesión no lo ve la siguiente en la misma pestaña.
suscribirASesion(() => {
  if (!sesionActual()) olvidarVistas()
})
