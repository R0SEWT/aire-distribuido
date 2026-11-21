import { useEffect, useState, useSyncExternalStore, type ReactNode } from 'react'
import { useSesion } from '../sesion/useSesion'
import { ContextoTiempoReal, EnlaceTiempoReal } from './tiempoReal'
import type { OpcionesWS } from './ws'

/**
 * Mantiene un solo WebSocket mientras haya sesión (spec §9.4). Va una vez,
 * arriba de las rutas. `opciones` es para las pruebas.
 */
export function TiempoRealProveedor({ children, opciones }: { children: ReactNode; opciones?: Partial<OpcionesWS> }) {
  const { sesion } = useSesion()
  const token = sesion?.token
  const expira = sesion?.expira
  const [enlace] = useState(() => new EnlaceTiempoReal(opciones))
  const valor = useSyncExternalStore(enlace.suscribir, enlace.leer)

  useEffect(() => {
    if (token === undefined || expira === undefined) return
    return enlace.usar(token, expira)
  }, [enlace, token, expira])

  return <ContextoTiempoReal.Provider value={valor}>{children}</ContextoTiempoReal.Provider>
}
