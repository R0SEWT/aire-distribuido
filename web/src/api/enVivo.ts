// El último estado de las estaciones, en vivo: un solo camino del evento
// `estacion.regimen` a la caché de TanStack Query (spec §9.4).
//
// El armazón ya está suscrito al evento y llama a aplicarEstadoEstacion: una
// pantalla NO se suscribe a `estacion.regimen` para mantener la caché. Lee:
//
//   - claves.regimenes       la lista, de GET /api/regimenes
//   - claves.regimen(id)     una estación, de GET /api/estaciones/{id}/regimen
//
// y marca su consulta como «en vivo» pidiéndola con el ayudante:
//
//   useQuery({
//     queryKey: claves.regimenes,
//     meta: EN_VIVO,
//     queryFn: () => pedirRegimenes(consultas, () => pedir(api.GET('/api/regimenes'))),
//   })
//
// LA REGLA DE FUSIÓN entre un GET y los eventos. El GET y el WebSocket viajan
// por conexiones distintas, así que la respuesta de un GET puede llegar DESPUÉS
// de un evento más nuevo de la misma estación. Por eso:
//
//   De una estación de la que ya llegó un evento desde que se abrió (o se
//   reabrió) el WebSocket, la entrada del GET se ignora: vale el evento.
//
// Si el GET traía algo más nuevo que ese evento, su propio evento viene detrás
// por el WebSocket, que entrega en orden. No se compara por `ts` (va al
// segundo) ni por `fecha_hora` (una corrección de la misma hora la conserva):
// entre dos eventos manda el que llegó último, y entre un evento y un GET, el
// evento.
//
// Esa memoria vale mientras la conexión está abierta. En cuanto deja de
// estarlo se vacía: sin WebSocket se pierden eventos, lo que se sabía por
// eventos ya no es de fiar y manda lo que responda un GET. Y al abrirse o
// reabrirse se vuelve a pedir todo lo que el WebSocket mantiene al día. Con eso
// tampoco importa el orden al entrar: no hace falta abrir el WebSocket antes
// del primer GET.

import type { QueryClient } from '@tanstack/react-query'
import { claves } from './cliente'
import type { components } from './contrato'
import { ErrorApi } from './errores'
import type { EventosWS } from './eventos'

declare module '@tanstack/react-query' {
  interface Register {
    queryMeta: {
      /** La consulta se mantiene al día por el WebSocket: se vuelve a pedir al reconectar. */
      enVivo?: boolean
    }
  }
}

/** El `meta` de una consulta que el WebSocket mantiene al día. */
export const EN_VIVO = { enVivo: true } as const

/** El estado de una estación con su código: lo que trae el evento y cada entrada de la lista. */
export type EstadoEstacion = EventosWS['estacion.regimen']
type RegimenEstacion = components['schemas']['RegimenEstacion']

// Por caché: el último evento de cada estación desde que se abrió el WebSocket.
const conEvento = new WeakMap<QueryClient, Map<string, EstadoEstacion>>()

function eventosDe(consultas: QueryClient): Map<string, EstadoEstacion> {
  let eventos = conEvento.get(consultas)
  if (!eventos) {
    eventos = new Map()
    conEvento.set(consultas, eventos)
  }
  return eventos
}

const porCodigo = (a: EstadoEstacion, b: EstadoEstacion) => a.estacion.localeCompare(b.estacion)

/**
 * La respuesta de GET /api/regimenes con la regla de fusión aplicada: las
 * estaciones con evento desde la última conexión conservan su evento (y entran
 * si el GET no las traía); las demás son las del GET. Ordenada por código,
 * como la manda la API. No modifica lo que recibe.
 */
export function fusionar(delGet: readonly EstadoEstacion[], eventos: ReadonlyMap<string, EstadoEstacion>): EstadoEstacion[] {
  return [...delGet.filter((e) => !eventos.has(e.estacion)), ...eventos.values()].sort(porCodigo)
}

/**
 * Lleva un evento `estacion.regimen` a la caché: actualiza la lista (si ya se
 * pidió) y la entrada de la estación, y recuerda que esa estación ya tiene un
 * evento en esta conexión. Lo llama el armazón; una pantalla solo lee la caché.
 */
export function aplicarEstadoEstacion(consultas: QueryClient, estado: EstadoEstacion) {
  eventosDe(consultas).set(estado.estacion, estado)
  // Sin lista pedida no se inventa una con una sola estación: se deja sin datos.
  consultas.setQueryData<EstadoEstacion[]>(
    claves.regimenes,
    (lista) => lista && [...lista.filter((e) => e.estacion !== estado.estacion), estado].sort(porCodigo),
  )
  consultas.setQueryData<RegimenEstacion>(claves.regimen(estado.estacion), estado)
}

/**
 * El WebSocket se abrió o se reabrió: se olvida de qué estaciones había
 * eventos (se pudieron perder otros más nuevos mientras estuvo caído) y se
 * vuelven a pedir todas las consultas marcadas EN_VIVO. El armazón lo llama en
 * el mismo instante en que la conexión queda abierta, antes de que pueda
 * llegar ningún evento por ella.
 */
export function alConectarEnVivo(consultas: QueryClient): Promise<void> {
  eventosDe(consultas).clear()
  // Además de lo marcado EN_VIVO, todo el estado de las estaciones: las entradas
  // por estación que creó un evento (aplicarEstadoEstacion) no llevan `meta`
  // hasta que una pantalla las pide, y quedarían con lo de antes del corte.
  return consultas.invalidateQueries({ predicate: (c) => c.meta?.enVivo === true || c.queryKey[0] === claves.regimenes[0] })
}

/**
 * La conexión dejó de estar abierta (se cortó, está reconectando o el servidor
 * la cerró): desde ahora se pierden eventos. Se olvida de qué estaciones había
 * eventos, para que un evento viejo no le gane a un GET más nuevo, y lo que la
 * caché sabe de las estaciones queda marcado como viejo: la pantalla que se
 * abra lo vuelve a pedir. No se pide nada ahora: eso es al reconectar.
 */
export function alDesconectarEnVivo(consultas: QueryClient): Promise<void> {
  eventosDe(consultas).clear()
  return consultas.invalidateQueries({ queryKey: claves.regimenes, refetchType: 'none' })
}

/**
 * La sesión terminó: se olvida de qué estaciones había eventos. Si no, hasta
 * que el WebSocket de la sesión siguiente quede abierto, un evento de la
 * anterior seguiría valiendo más que la respuesta de la API.
 */
export function olvidarEventosEnVivo(consultas: QueryClient) {
  eventosDe(consultas).clear()
}

/**
 * Pide la lista de estados (GET /api/regimenes) para una consulta EN_VIVO y le
 * aplica la regla de fusión con los eventos que hayan llegado hasta el momento
 * de la respuesta, también los que llegaron mientras el pedido viajaba.
 */
export async function pedirRegimenes(
  consultas: QueryClient,
  pedirLista: () => Promise<EstadoEstacion[]>,
): Promise<EstadoEstacion[]> {
  return fusionar(await pedirLista(), eventosDe(consultas))
}

/**
 * Pide el estado de una estación (GET /api/estaciones/{id}/regimen) para una
 * consulta EN_VIVO, con la misma regla: si de esa estación ya llegó un evento
 * en esta conexión, vale el evento, también si la API respondió 404 porque al
 * atender el pedido la estación todavía no tenía estado.
 */
export async function pedirRegimen(
  consultas: QueryClient,
  estacion: string,
  pedirUno: () => Promise<RegimenEstacion>,
): Promise<RegimenEstacion> {
  const eventos = eventosDe(consultas)
  try {
    const respuesta = await pedirUno()
    return eventos.get(estacion) ?? respuesta
  } catch (error) {
    const evento = eventos.get(estacion)
    if (evento && error instanceof ErrorApi && error.tipo === 'no_encontrado') return evento
    throw error
  }
}
