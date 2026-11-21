// La conexión de tiempo real dentro de React: una ConexionWS por sesión.
//
//   const estado = useEstadoWS()                       // 'conectada', 'reconectando'…
//   useEvento('estacion.regimen', (e) => { … })        // suscribirse por nombre
//
// Lo usual en una pantalla es que el manejador actualice la caché de TanStack
// Query (queryClient.setQueryData) y que lo que se dibuja salga de la caché.

import { createContext, useContext, useEffect, useMemo, useRef } from 'react'
import { cerrarSesion } from '../sesion/almacen'
import { validarSesion } from './cliente'
import type { EventosWS, NombreEvento } from './eventos'
import { ConexionWS, urlWS, type EstadoWS, type OpcionesWS } from './ws'

export interface TiempoReal {
  estado: EstadoWS
  conexion: ConexionWS | null
}

const INACTIVO: TiempoReal = { estado: 'inactiva', conexion: null }

/**
 * Une la sesión con su conexión: abre una al empezar la sesión, la cierra al
 * terminar y le cuenta a React cada cambio de estado (useSyncExternalStore).
 */
export class EnlaceTiempoReal {
  private foto: TiempoReal = INACTIVO
  private readonly oyentes = new Set<() => void>()

  constructor(private readonly opciones: Partial<OpcionesWS> = {}) {}

  readonly suscribir = (oyente: () => void) => {
    this.oyentes.add(oyente)
    return () => this.oyentes.delete(oyente)
  }

  readonly leer = () => this.foto

  /** Abre la conexión de una sesión. Devuelve cómo cerrarla. */
  usar(token: string, expira: number): () => void {
    const conexion = new ConexionWS({
      url: urlWS(),
      // Tres apretones fallidos seguidos: ¿la API está apagada, o ya no acepta
      // este token? Si responde 401, validarSesion cierra la sesión.
      alFallarApreton: () => void validarSesion(),
      ...this.opciones,
      token,
      expira,
    })
    const dejar = conexion.alCambiar((estado) => {
      this.poner({ estado, conexion })
      // El servidor dio el token por vencido: la sesión se cierra y la UI vuelve al ingreso.
      if (estado === 'vencida') cerrarSesion('vencida')
    })
    this.poner({ estado: conexion.estado, conexion })
    conexion.conectar()
    return () => {
      dejar()
      conexion.cerrar()
      this.poner(INACTIVO)
    }
  }

  private poner(foto: TiempoReal) {
    this.foto = foto
    for (const oyente of [...this.oyentes]) oyente()
  }
}

export const ContextoTiempoReal = createContext<TiempoReal>(INACTIVO)

/** El estado de la conexión de tiempo real, para mostrarlo. */
export function useEstadoWS(): EstadoWS {
  return useContext(ContextoTiempoReal).estado
}

/** Para el botón «Reconectar» cuando el servidor cortó la conexión. */
export function useReconectarWS(): () => void {
  const { conexion } = useContext(ContextoTiempoReal)
  return useMemo(() => () => conexion?.reconectar(), [conexion])
}

/**
 * Suscribe el componente a un evento del WebSocket por su nombre mientras esté
 * montado. El manejador puede cambiar entre renders sin volver a suscribirse.
 */
export function useEvento<N extends NombreEvento>(evento: N, manejador: (datos: EventosWS[N]) => void) {
  const { conexion } = useContext(ContextoTiempoReal)
  const ultimo = useRef(manejador)
  useEffect(() => {
    ultimo.current = manejador
  })
  useEffect(() => {
    if (!conexion) return
    return conexion.suscribir(evento, (datos) => ultimo.current(datos))
  }, [conexion, evento])
}
