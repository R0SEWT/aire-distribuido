// El entrenamiento que está a la vista, fuera de React: como el simulador, vive
// lo que dure la sesión y no lo que dure la pantalla. Se lanza un entrenamiento,
// se va al Clúster a ver los nodos y, al volver, sigue ahí: el mismo, por donde
// iba su reproducción y, si todavía corría, con las rondas que llegaron mientras.
//
// Qué se sigue y qué llegó lo decide el reductor de seguimiento.ts; aquí se
// guarda su estado, la velocidad elegida y por dónde va la reproducción.
//
// Los eventos del WebSocket entran por recibir(), que llama App.tsx desde arriba
// de las rutas: llegan también cuando la pantalla no está montada.

import { useSyncExternalStore } from 'react'
import { reducir, SIN_SEGUIMIENTO, type Accion, type Seguimiento } from './seguimiento'
import type { MemoriaDeReproduccion, Velocidad } from './useReproduccion'

export interface Vista {
  seguimiento: Seguimiento
  /** La velocidad de la reproducción que eligió la persona. */
  velocidad: Velocidad
}

type DelWebSocket = Extract<Accion, { tipo: 'ronda' | 'fin' }>

const INICIAL: Vista = { seguimiento: SIN_SEGUIMIENTO, velocidad: 'normal' }

export class AlmacenDeEntrenamiento {
  private vista: Vista = INICIAL
  private readonly oyentes = new Set<() => void>()
  /** Por dónde va la reproducción del entrenamiento a la vista. No vuelve a dibujar a nadie: es memoria. */
  private avance: { clave: string; mostradas: number } | null = null

  readonly suscribir = (oyente: () => void) => {
    this.oyentes.add(oyente)
    return () => this.oyentes.delete(oyente)
  }

  readonly leer = () => this.vista

  /** Lo que hace la pantalla: lanzar, la respuesta de la API, abrir uno del historial, su detalle. */
  readonly despachar = (accion: Accion) => {
    const antes = this.vista.seguimiento
    const despues = reducir(antes, accion)
    if (despues === antes) return
    // Otro entrenamiento a la vista: su reproducción empieza de nuevo.
    if (despues.corrida?.id !== antes.corrida?.id) this.avance = null
    this.publicar({ ...this.vista, seguimiento: despues })
  }

  /**
   * Un evento del WebSocket. Con la pantalla abierta se sigue como siempre (el
   * reductor decide). Con la pantalla cerrada solo se anota lo del
   * entrenamiento que quedó a la vista o del pedido que está en viaje: uno que
   * lanzó otra sesión no ocupa la pantalla a espaldas de quien la dejó.
   */
  recibir(accion: DelWebSocket) {
    const { corrida, lanzando } = this.vista.seguimiento
    const aLaVista = this.oyentes.size > 0
    if (!aLaVista && corrida?.id !== accion.evento.id && lanzando === null) return
    this.despachar(accion)
  }

  cambiarVelocidad(velocidad: Velocidad) {
    if (velocidad !== this.vista.velocidad) this.publicar({ ...this.vista, velocidad })
  }

  /** Para useReproduccion: dónde iba la reproducción de un entrenamiento, y dónde va ahora. */
  readonly memoria: MemoriaDeReproduccion = {
    leer: (clave) => (this.avance?.clave === clave ? this.avance.mostradas : undefined),
    guardar: (clave, mostradas) => {
      this.avance = { clave, mostradas }
    },
  }

  /** La sesión terminó: quien entre después no hereda el entrenamiento a la vista. */
  reiniciar() {
    this.avance = null
    if (this.vista !== INICIAL) this.publicar(INICIAL)
  }

  private publicar(vista: Vista) {
    this.vista = vista
    for (const oyente of [...this.oyentes]) oyente()
  }
}

/** El de la app: uno solo, para que el entrenamiento a la vista siga ahí al cambiar de pantalla. */
export const entrenamientoALaVista = new AlmacenDeEntrenamiento()

/** El entrenamiento a la vista, dentro de React. */
export function useEntrenamientoALaVista(almacen: AlmacenDeEntrenamiento = entrenamientoALaVista): Vista {
  return useSyncExternalStore(almacen.suscribir, almacen.leer)
}
