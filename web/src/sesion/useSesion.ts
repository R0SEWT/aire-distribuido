// La sesión dentro de React: quién está, con qué rol, y cómo entrar y salir.

import { useCallback, useSyncExternalStore } from 'react'
import { api, pedir } from '../api/cliente'
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
    abrirSesion({ usuario, token: token.token, rol: token.rol, expira: Date.parse(token.expira) })
  }, [])

  const salir = useCallback(() => cerrarSesion('salir'), [])

  return { ...estado, ingresar, salir }
}
