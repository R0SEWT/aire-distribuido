import type { Rol } from './almacen'

export const TODOS: readonly Rol[] = ['usuario', 'admin']
export const SOLO_ADMIN: readonly Rol[] = ['admin']

/** Si el rol tiene acceso a algo pensado para `roles`. */
export function puede(rol: Rol | undefined, roles: readonly Rol[]): boolean {
  return rol !== undefined && roles.includes(rol)
}
