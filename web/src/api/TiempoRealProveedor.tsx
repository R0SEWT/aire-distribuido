import { useQueryClient } from '@tanstack/react-query'
import { useEffect, useState, useSyncExternalStore, type ReactNode } from 'react'
import { useSesion } from '../sesion/useSesion'
import { aplicarEstadoEstacion, refrescarEnVivo } from './enVivo'
import { ContextoTiempoReal, EnlaceTiempoReal, useEstadoWS, useEvento } from './tiempoReal'
import type { OpcionesWS } from './ws'

/**
 * Mantiene un solo WebSocket mientras haya sesión (spec §9.4) y, con él, la
 * caché al día. Va una vez, arriba de las rutas. `opciones` es para las pruebas.
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

  return (
    <ContextoTiempoReal.Provider value={valor}>
      <CacheEnVivo />
      {children}
    </ContextoTiempoReal.Provider>
  )
}

/**
 * El camino del WebSocket a la caché (api/enVivo.ts): cada `estacion.regimen`
 * actualiza la lista y la entrada de su estación, y cada vez que la conexión
 * queda «conectada» (la primera vez y después de un corte) se vuelven a pedir
 * las consultas marcadas EN_VIVO, que pueden haberse perdido eventos.
 */
function CacheEnVivo() {
  const consultas = useQueryClient()
  const estado = useEstadoWS()

  useEvento('estacion.regimen', (evento) => aplicarEstadoEstacion(consultas, evento))

  useEffect(() => {
    if (estado === 'conectada') void refrescarEnVivo(consultas)
  }, [estado, consultas])

  return null
}
