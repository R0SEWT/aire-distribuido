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
