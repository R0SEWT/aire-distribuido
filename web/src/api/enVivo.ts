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
// Con eso quedan cubiertos los tres huecos entre un GET y el WebSocket:
//
//   1. Lo que cambió mientras el WebSocket estaba caído: al volver a
//      «conectada» se vuelven a pedir todas las consultas marcadas EN_VIVO.
//   2. El orden al entrar: no hace falta abrir el WebSocket antes del GET.
//      La primera conexión también refresca lo marcado EN_VIVO.
//   3. Un evento que llega mientras un GET viaja: la respuesta del GET es de
//      antes del evento y lo pisaría. pedirRegimenes y pedirRegimen guardan los
//      eventos que llegan durante el pedido y los superponen a la respuesta.

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

/** Vuelve a pedir todas las consultas marcadas EN_VIVO. El armazón lo llama al (re)conectar. */
export function refrescarEnVivo(consultas: QueryClient): Promise<void> {
  return consultas.invalidateQueries({ predicate: (c) => c.meta?.enVivo === true })
}

/**
 * Si `nuevo` es tan reciente o más que `previo`: por la hora de la lectura y,
 * a igual hora (se repitió la lectura), por cuándo se calculó. Es la regla de
 * la API: el último estado de una estación no retrocede.
 */
export function esMasReciente(nuevo: RegimenEstacion, previo: RegimenEstacion): boolean {
  const hora = Date.parse(nuevo.fecha_hora) - Date.parse(previo.fecha_hora)
  if (hora !== 0) return hora > 0
  return Date.parse(nuevo.ts) >= Date.parse(previo.ts)
}

/**
 * La lista con `estados` encima: de cada estación queda su estado más
 * reciente, y una estación que no estaba entra. Ordenada por código, como la
 * manda la API. No modifica la lista que recibe.
 */
export function superponer(lista: readonly EstadoEstacion[], estados: Iterable<EstadoEstacion>): EstadoEstacion[] {
  const porEstacion = new Map(lista.map((e) => [e.estacion, e]))
  for (const estado of estados) {
    const previo = porEstacion.get(estado.estacion)
    if (!previo || esMasReciente(estado, previo)) porEstacion.set(estado.estacion, estado)
  }
  return [...porEstacion.values()].sort((a, b) => a.estacion.localeCompare(b.estacion))
}

// Los eventos que llegaron mientras un GET viajaba, por caché y por pedido.
const enViaje = new WeakMap<QueryClient, Set<Map<string, EstadoEstacion>>>()

function anotar(consultas: QueryClient): { eventos: Map<string, EstadoEstacion>; dejar: () => void } {
  let pedidos = enViaje.get(consultas)
  if (!pedidos) {
    pedidos = new Set()
    enViaje.set(consultas, pedidos)
  }
  const eventos = new Map<string, EstadoEstacion>()
  pedidos.add(eventos)
  return { eventos, dejar: () => pedidos.delete(eventos) }
}

/**
 * Lleva un evento `estacion.regimen` a la caché: actualiza la lista (si ya se
 * pidió) y la entrada de la estación. Un estado más viejo que el guardado no
 * lo reemplaza. Lo llama el armazón; una pantalla solo lee la caché.
 */
export function aplicarEstadoEstacion(consultas: QueryClient, estado: EstadoEstacion) {
  for (const eventos of enViaje.get(consultas) ?? []) {
    const previo = eventos.get(estado.estacion)
    if (!previo || esMasReciente(estado, previo)) eventos.set(estado.estacion, estado)
  }
  // Sin lista pedida no se inventa una con una sola estación: se deja sin datos.
  consultas.setQueryData<EstadoEstacion[]>(claves.regimenes, (lista) => lista && superponer(lista, [estado]))
  consultas.setQueryData<RegimenEstacion>(claves.regimen(estado.estacion), (previo) =>
    previo && !esMasReciente(estado, previo) ? previo : estado,
  )
}

/**
 * Pide la lista de estados (GET /api/regimenes) para una consulta EN_VIVO: a
 * la respuesta le superpone los eventos que llegaron mientras el pedido viajaba.
 */
export async function pedirRegimenes(
  consultas: QueryClient,
  pedirLista: () => Promise<EstadoEstacion[]>,
): Promise<EstadoEstacion[]> {
  const { eventos, dejar } = anotar(consultas)
  try {
    return superponer(await pedirLista(), eventos.values())
  } finally {
    dejar()
  }
}

/**
 * Pide el estado de una estación (GET /api/estaciones/{id}/regimen) para una
 * consulta EN_VIVO. Si mientras tanto llegó un evento de esa estación más
 * reciente que la respuesta, gana el evento; también si la API respondió 404
 * porque la estación todavía no tenía estado.
 */
export async function pedirRegimen(
  consultas: QueryClient,
  estacion: string,
  pedirUno: () => Promise<RegimenEstacion>,
): Promise<RegimenEstacion> {
  const { eventos, dejar } = anotar(consultas)
  try {
    const respuesta = await pedirUno()
    const evento = eventos.get(estacion)
    return evento && esMasReciente(evento, respuesta) ? evento : respuesta
  } catch (error) {
    const evento = eventos.get(estacion)
    if (evento && error instanceof ErrorApi && error.tipo === 'no_encontrado') return evento
    throw error
  } finally {
    dejar()
  }
}
