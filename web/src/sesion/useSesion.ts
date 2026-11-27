// La sesión dentro de React: quién está, con qué rol, y cómo entrar y salir.

import { useCallback, useSyncExternalStore } from 'react'
import { api, pedir } from '../api/cliente'
import type { components } from '../api/contrato'
import { ErrorApi } from '../api/errores'
import { abrirSesion, cerrarSesion, estadoSesion, suscribir, type EstadoSesion } from './almacen'

type Token = components['schemas']['Token']

/**
 * Con qué nombre se muestra a quien entró como invitado. La API responde el
 * token y el rol, no el usuario de la cuenta: la UI no lo conoce ni lo necesita.
 */
export const USUARIO_INVITADO = 'invitado'

export interface UsoDeSesion extends EstadoSesion {
  /** Pide un token a la API y abre la sesión. Si falla, lanza un ErrorApi. */
  ingresar: (usuario: string, clave: string) => Promise<void>
  /**
   * Pide el token de la cuenta de invitado (POST /api/auth/invitado, sin
   * credenciales) y abre la sesión. Si la API no tiene invitado responde 404:
   * lanza un ErrorApi de tipo `no_encontrado`. Con `senal` abortada no abre
   * nada, aunque la respuesta ya haya llegado.
   */
  entrarComoInvitado: (senal?: AbortSignal) => Promise<void>
  salir: () => void
}

/** Abre la sesión con el token que respondió la API: el camino de los dos ingresos. */
function abrirConToken(usuario: string, token: Token) {
  const expira = Date.parse(token.expira)
  // Sin una hora de vencimiento no se puede avisar ni cerrar a tiempo: no es una sesión.
  if (!Number.isFinite(expira)) throw new ErrorApi('inesperado', 200, 'la API respondió un token sin una fecha de vencimiento válida')
  abrirSesion({ usuario, token: token.token, rol: token.rol, expira })
}

export function useSesion(): UsoDeSesion {
  const estado = useSyncExternalStore(suscribir, estadoSesion)

  const ingresar = useCallback(async (usuario: string, clave: string) => {
    const token = await pedir(api.POST('/api/auth/token', { body: { usuario, clave } }), { ingreso: true })
    abrirConToken(usuario, token)
  }, [])

  const entrarComoInvitado = useCallback(async (senal?: AbortSignal) => {
    const token = await pedir(api.POST('/api/auth/invitado', { signal: senal }), { ingreso: true })
    // Quien lo pidió ya no lo quiere (cerró el diálogo): una respuesta que llegó tarde no abre la sesión.
    senal?.throwIfAborted()
    abrirConToken(USUARIO_INVITADO, token)
  }, [])

  const salir = useCallback(() => cerrarSesion('salir'), [])

  return { ...estado, ingresar, entrarComoInvitado, salir }
}
