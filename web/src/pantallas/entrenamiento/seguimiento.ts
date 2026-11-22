// El entrenamiento que la pantalla está siguiendo, como un reductor sin React:
// qué rondas llegaron, si ya terminó y de dónde salió.
//
// Un entrenamiento real dura unos 50 ms, así que los eventos llegan casi juntos
// y, a veces, ANTES que la respuesta 202 de POST /api/entrenamientos, que es la
// que trae el id. Por eso, mientras el pedido viaja, lo que llega se guarda en
// un buzón por id y se reclama al conocer el propio.

import type { components } from '../../api/contrato'
import type { EventosWS } from '../../api/eventos'

type Esquemas = components['schemas']

export type Ronda = Esquemas['Ronda']
export type Pedido = Esquemas['PedidoEntrenamiento']
export type Resumen = Esquemas['EntrenamientoResumen']
export type Detalle = Esquemas['Entrenamiento']
export type EventoRonda = EventosWS['entrenamiento.ronda']
export type EventoFinal = EventosWS['entrenamiento.fin']

/** Cómo terminó: bien, con los números del evento, o abortado, con el error (que nombra al nodo). */
export type Fin =
  | {
      abortado: false
      iteraciones: number
      ms_total: number
      paro?: Esquemas['Paro']
      dif_secuencial?: number
      /** El modelo que quedó activo, si activó uno. */
      modelo_id?: string
      /** Por qué no activó su modelo, si tenía que hacerlo. */
      aviso?: string
    }
  | { abortado: true; error: string }

export type Origen =
  /** Lo lanzó esta pantalla. */
  | 'propia'
  /** Lo lanzó otra sesión (otro admin, Postman): sus eventos llegaron igual. */
  | 'ajena'
  /** Se abrió desde el historial: sus rondas vienen de GET /api/entrenamientos/{id}. */
  | 'historial'

export interface Corrida {
  id: string
  origen: Origen
  /** Lo que se pidió; solo se conoce de una corrida propia. */
  pedido: Pedido | null
  /** Las rondas recibidas, en orden de iteración. */
  rondas: Ronda[]
  fin: Fin | null
  /** Cuándo llegó el fin por el WebSocket (ms de reloj); null si no se vio terminar. */
  finEn: number | null
}

interface EnElBuzon {
  rondas: Ronda[]
  fin: Fin | null
  finEn: number | null
}

export interface Seguimiento {
  /** El pedido que está viajando a la API, si hay uno. */
  lanzando: Pedido | null
  corrida: Corrida | null
  /** Lo que llegó de entrenamientos todavía sin dueño, por id, mientras el pedido viaja. */
  buzon: Record<string, EnElBuzon>
  /**
   * El id del entrenamiento que se seguía y la API ya no conoce (se reinició
   * sin almacén): se dejó de seguir, y se dice. null si no pasó.
   */
  perdido: string | null
}

export const SIN_SEGUIMIENTO: Seguimiento = { lanzando: null, corrida: null, buzon: {}, perdido: null }

export type Accion =
  | { tipo: 'lanzar'; pedido: Pedido }
  | { tipo: 'aceptado'; id: string }
  | { tipo: 'rechazado' }
  | { tipo: 'ronda'; evento: EventoRonda }
  | { tipo: 'fin'; evento: EventoFinal; ahora: number }
  | { tipo: 'ver'; detalle: Detalle }
  | { tipo: 'detalle'; detalle: Detalle; ahora: number }
  /** La API respondió 404 al detalle del entrenamiento seguido: ya no existe. */
  | { tipo: 'perdido'; id: string }
  | { tipo: 'cerrar' }

/** El fin de un entrenamiento como lo trae el evento `entrenamiento.fin`. */
export function finDeEvento(evento: EventoFinal): Fin {
  if ('error' in evento) return { abortado: true, error: evento.error }
  return {
    abortado: false,
    iteraciones: evento.iteraciones,
    ms_total: evento.ms_total,
    paro: evento.paro,
    dif_secuencial: evento.dif_secuencial,
    modelo_id: evento.modelo_id,
    aviso: evento.aviso,
  }
}

/** El fin de un entrenamiento como lo trae su detalle; null si sigue en curso. */
export function finDeDetalle(detalle: Resumen): Fin | null {
  if (detalle.estado === 'abortado') return { abortado: true, error: detalle.error ?? 'el entrenamiento abortó' }
  if (detalle.estado === 'terminado') {
    return {
      abortado: false,
      iteraciones: detalle.iteraciones ?? 0,
      ms_total: detalle.ms_total ?? 0,
      paro: detalle.paro,
      dif_secuencial: detalle.dif_secuencial,
      modelo_id: detalle.modelo_id,
      aviso: detalle.aviso,
    }
  }
  return null
}

/** Suma una ronda sin repetir iteraciones y dejándolas en orden (un evento puede llegar dos veces). */
function conRonda(rondas: Ronda[], evento: EventoRonda): Ronda[] {
  if (rondas.some((r) => r.iter === evento.iter)) return rondas
  const ronda: Ronda = {
    iter: evento.iter,
    inercia: evento.inercia,
    ms_ronda: evento.ms_ronda,
    ms_fusion: evento.ms_fusion,
    ms_por_nodo: evento.ms_por_nodo,
  }
  const nuevas = [...rondas, ronda]
  // Casi siempre llega al final; si no, se ordena.
  if (rondas.length > 0 && rondas[rondas.length - 1]!.iter > ronda.iter) nuevas.sort((a, b) => a.iter - b.iter)
  return nuevas
}

/** Si la pantalla puede pasar a mostrar otro entrenamiento: no hay ninguno, o el que hay ya terminó. */
function estaLibre(s: Seguimiento): boolean {
  return s.corrida === null || s.corrida.fin !== null
}

export function reducir(s: Seguimiento, a: Accion): Seguimiento {
  switch (a.tipo) {
    case 'lanzar':
      // La corrida anterior sigue a la vista hasta que la API acepte el pedido: si lo rechaza, no se pierde.
      return { ...s, lanzando: a.pedido, buzon: {}, perdido: null }

    case 'aceptado': {
      // Sin un pedido en viaje no hay nada que aceptar: es la respuesta tardía de una sesión que ya terminó.
      if (s.lanzando === null) return s
      const llegado = s.buzon[a.id]
      return {
        lanzando: null,
        buzon: {},
        perdido: null,
        corrida: {
          id: a.id,
          origen: 'propia',
          pedido: s.lanzando,
          rondas: llegado?.rondas ?? [],
          fin: llegado?.fin ?? null,
          finEn: llegado?.finEn ?? null,
        },
      }
    }

    case 'rechazado': {
      if (s.lanzando === null) return s
      // Si mientras tanto llegaron rondas, son de otro entrenamiento (el que
      // causó el 409, por ejemplo): se muestra ese, si la pantalla está libre.
      const ids = Object.keys(s.buzon)
      const id = ids[ids.length - 1]
      if (id === undefined || !estaLibre(s)) return { ...s, lanzando: null, buzon: {} }
      return { lanzando: null, buzon: {}, perdido: null, corrida: { id, origen: 'ajena', pedido: null, ...s.buzon[id]! } }
    }

    case 'ronda': {
      const { id } = a.evento
      if (s.corrida?.id === id) {
        const rondas = conRonda(s.corrida.rondas, a.evento)
        return rondas === s.corrida.rondas ? s : { ...s, corrida: { ...s.corrida, rondas } }
      }
      if (s.lanzando) {
        const antes = s.buzon[id] ?? { rondas: [], fin: null, finEn: null }
        return { ...s, buzon: { ...s.buzon, [id]: { ...antes, rondas: conRonda(antes.rondas, a.evento) } } }
      }
      // Un entrenamiento que no lanzó esta pantalla: se sigue si no hay otro a medias.
      if (!estaLibre(s)) return s
      return { ...s, perdido: null, corrida: { id, origen: 'ajena', pedido: null, rondas: conRonda([], a.evento), fin: null, finEn: null } }
    }

    case 'fin': {
      const { id } = a.evento
      const fin = finDeEvento(a.evento)
      if (s.corrida?.id === id) {
        if (s.corrida.fin) return s
        return { ...s, corrida: { ...s.corrida, fin, finEn: a.ahora } }
      }
      if (s.lanzando) {
        const antes = s.buzon[id] ?? { rondas: [], fin: null, finEn: null }
        return { ...s, buzon: { ...s.buzon, [id]: { ...antes, fin, finEn: a.ahora } } }
      }
      // El fin de un entrenamiento ajeno del que no se vio ninguna ronda: queda para el historial.
      return s
    }

    case 'ver': {
      const fin = finDeDetalle(a.detalle)
      return {
        ...s,
        perdido: null,
        corrida: {
          id: a.detalle.id,
          // Uno en curso abierto desde el historial se sigue como ajeno: sus rondas siguen llegando.
          origen: fin ? 'historial' : 'ajena',
          pedido: null,
          rondas: [...a.detalle.rondas].sort((x, y) => x.iter - y.iter),
          fin,
          finEn: null,
        },
      }
    }

    case 'detalle': {
      // El detalle de GET /api/entrenamientos/{id}, pedido al terminar o, con el
      // WebSocket caído, mientras corre. Trae todas las rondas guardadas: si por
      // el WebSocket llegaron menos (un corte a mitad), se completan. Y si ya
      // tiene el estado final y el evento de fin no llegó, vale el del detalle.
      const c = s.corrida
      if (c?.id !== a.detalle.id) return s
      const masRondas = a.detalle.rondas.length > c.rondas.length
      const fin = c.fin ?? finDeDetalle(a.detalle)
      if (!masRondas && fin === c.fin) return s
      return {
        ...s,
        corrida: {
          ...c,
          rondas: masRondas ? [...a.detalle.rondas].sort((x, y) => x.iter - y.iter) : c.rondas,
          fin,
          finEn: c.fin ? c.finEn : fin ? a.ahora : null,
        },
      }
    }

    case 'perdido':
      // Solo si todavía es el que se sigue: la respuesta pudo llegar tarde.
      if (s.corrida?.id !== a.id) return s
      return { ...s, corrida: null, perdido: a.id }

    case 'cerrar':
      return { ...s, corrida: null }
  }
}
