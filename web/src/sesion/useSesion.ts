// La sesión dentro de React: quién está, con qué rol, y cómo entrar y salir.

import { useCallback, useSyncExternalStore } from 'react'
import { api, pedir } from '../api/cliente'
import { ErrorApi } from '../api/errores'
import { abrirSesion, cerrarSesion, estadoSesion, suscribir, type EstadoSesion } from './almacen'

export interface UsoDeSesion extends EstadoSesion {
  /** Pide un token a la API y abre la sesión. Si falla, lanza un ErrorApi. */
  ingresar: (usuario: string, clave: string) => Promise<void>
  salir: () => void
}

export function useSesion(): UsoDeSesion {
  const estado = useSyncExternalStore(suscribir, estadoSesion)

  const ingresar = useCallback(async (usuario: string, clave: string) => {
    const token = await pedir(api.POST('/api/auth/token', { body: { usuario, clave } }), { ingreso: true })
    const expira = Date.parse(token.expira)
    // Sin una hora de vencimiento no se puede avisar ni cerrar a tiempo: no es una sesión.
    if (!Number.isFinite(expira)) throw new ErrorApi('inesperado', 200, 'la API respondió un token sin una fecha de vencimiento válida')
    abrirSesion({ usuario, token: token.token, rol: token.rol, expira })
  }, [])

  const salir = useCallback(() => cerrarSesion('salir'), [])

  return { ...estado, ingresar, salir }
}
