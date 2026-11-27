import { useQueryClient } from '@tanstack/react-query'
import { useContext, useEffect, useState, useSyncExternalStore, type ReactNode } from 'react'
import { useSesion } from '../sesion/useSesion'
import { alConectarEnVivo, alDesconectarEnVivo, aplicarEstadoEstacion } from './enVivo'
import { ContextoTiempoReal, EnlaceTiempoReal, useEvento } from './tiempoReal'
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
 * queda abierta (la primera vez y después de un corte) se olvida de qué
 * estaciones había eventos y se vuelven a pedir las consultas EN_VIVO. Cuando
 * deja de estar abierta también se olvidan: mientras no haya WebSocket, manda
 * lo que responda la API.
 */
function CacheEnVivo() {
  const consultas = useQueryClient()
  const { conexion } = useContext(ContextoTiempoReal)

  useEvento('estacion.regimen', (evento) => aplicarEstadoEstacion(consultas, evento))

  useEffect(() => {
    if (!conexion) return
    // Escuchando a la conexión misma, no al estado que ve React: el aviso llega
    // en el instante en que se abre, antes que cualquier evento. Si la memoria
    // se vaciara un render después, se llevaría un evento recién llegado.
    const alConectar = () => void alConectarEnVivo(consultas)
    if (conexion.estado === 'conectada') alConectar()
    return conexion.alCambiar((estado) => {
      if (estado === 'conectada') alConectar()
      // Sin conexión se pierden eventos: lo que se sabía por eventos deja de mandar sobre un GET.
      else void alDesconectarEnVivo(consultas)
    })
  }, [conexion, consultas])

  return null
}
