// El menú y las rutas de las pantallas, en un solo lugar: la barra dibuja las
// píldoras desde aquí y App.tsx arma las rutas con sus guardas.

import type { Rol } from '../sesion/almacen'
import { SOLO_ADMIN, TODOS } from '../sesion/roles'

export interface EntradaDeMenu {
  ruta: string
  etiqueta: string
  /** Quién la ve en el menú y quién pasa la guarda (spec §4). */
  roles: readonly Rol[]
}

export const RUTAS = {
  ingreso: '/ingreso',
  panel: '/',
  estacion: '/estacion',
  simulacion: '/simulacion',
  impacto: '/impacto',
  entrenamiento: '/entrenamiento',
  cluster: '/cluster',
  /** Fuera del menú: el muestrario de los componentes base. */
  muestrario: '/muestrario',
} as const

export const MENU: readonly EntradaDeMenu[] = [
  { ruta: RUTAS.panel, etiqueta: 'Panel', roles: TODOS },
  { ruta: RUTAS.estacion, etiqueta: 'Estación', roles: TODOS },
  { ruta: RUTAS.simulacion, etiqueta: 'Simulación', roles: TODOS },
  { ruta: RUTAS.impacto, etiqueta: 'Impacto', roles: TODOS },
  { ruta: RUTAS.entrenamiento, etiqueta: 'Entrenamiento', roles: SOLO_ADMIN },
  { ruta: RUTAS.cluster, etiqueta: 'Clúster', roles: SOLO_ADMIN },
]

/** Las entradas del menú que ve un rol. */
export function menuDe(rol: Rol): readonly EntradaDeMenu[] {
  return MENU.filter((e) => e.roles.includes(rol))
}

/** La entrada del menú a la que pertenece una ruta: /estacion/CA-AYAS-01 es de «Estación». */
export function entradaDe(ruta: string): EntradaDeMenu | undefined {
  const camino = ruta.split(/[?#]/)[0] ?? ruta
  return MENU.find((e) => camino === e.ruta || (e.ruta !== RUTAS.panel && camino.startsWith(`${e.ruta}/`)))
}

/** Si un rol pasa la guarda de una ruta. Lo que no está en el menú no tiene guarda de rol. */
export function puedeEntrar(ruta: string, rol: Rol): boolean {
  return entradaDe(ruta)?.roles.includes(rol) ?? true
}

export const NOMBRE = 'Aire distribuido'

/** El título del documento para una ruta: «Clúster · Aire distribuido». */
export function tituloDe(ruta: string): string {
  const camino = ruta.split(/[?#]/)[0] ?? ruta
  const pantalla =
    entradaDe(camino)?.etiqueta ??
    (camino === RUTAS.ingreso ? 'Ingreso' : camino === RUTAS.muestrario ? 'Muestrario de componentes' : 'No existe')
  return `${pantalla} · ${NOMBRE}`
}
