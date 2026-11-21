// Las guardas de ruta (spec §4 y §10). Esconden lo que el rol no puede usar;
// quien decide de verdad es la API, que responde 403 aunque se fuerce la URL.

import type { ReactNode } from 'react'
import { Navigate, Outlet, useLocation } from 'react-router'
import { Estado } from '../componentes/Estado'
import type { Rol } from './almacen'
import { puede } from './roles'
import { useSesion } from './useSesion'

/** De dónde venía quien fue mandado al ingreso, para volver ahí después. */
export interface DesdeDonde {
  desde?: string
}

/** Sin sesión, al ingreso. Con sesión, las rutas de adentro. */
export function RequiereSesion() {
  const { sesion, motivo } = useSesion()
  const ubicacion = useLocation()
  if (!sesion) {
    // Se recuerda dónde estaba solo si la sesión faltaba o venció: quien salió
    // a propósito no deja su pantalla a quien entre después.
    const estado: DesdeDonde = motivo === 'salir' ? {} : { desde: ubicacion.pathname + ubicacion.search }
    return <Navigate to="/ingreso" replace state={estado} />
  }
  return <Outlet />
}

/** Muestra lo de adentro solo si el rol de la sesión está en `roles`; si no, «sin permiso». */
export function RequiereRol({ roles, children }: { roles: readonly Rol[]; children: ReactNode }) {
  const { sesion } = useSesion()
  if (!puede(sesion?.rol, roles)) {
    return (
      <Estado tipo="sin-permiso" titulo="Esta pantalla es solo para administradores">
        Tu usuario tiene el rol «{sesion?.rol ?? 'ninguno'}». Si necesitas entrar, ingresa con una cuenta de
        administrador.
      </Estado>
    )
  }
  return <>{children}</>
}
