// El punto de estado de la API (spec §5): verde, ámbar o rojo desde /api/salud,
// siempre con su palabra al lado.

import { useQuery } from '@tanstack/react-query'
import { api, claves, pedir } from '../api/cliente'
import type { components } from '../api/contrato'

type Salud = components['schemas']['Salud']

export type NivelDeSalud = 'consultando' | 'ok' | 'degradada' | 'caida'

export interface SaludPresentada {
  nivel: NivelDeSalud
  palabra: string
  /** Qué anda mal, o qué está bien, en una frase. */
  detalle: string
}

/** Traduce la respuesta de /api/salud (o su falta) a un nivel, una palabra y un detalle. */
export function presentarSalud(salud: Salud | undefined, fallo: boolean): SaludPresentada {
  if (fallo) return { nivel: 'caida', palabra: 'API sin respuesta', detalle: 'La API no responde a /api/salud.' }
  if (!salud) return { nivel: 'consultando', palabra: 'Consultando la API…', detalle: 'Esperando a /api/salud.' }
  const problemas: string[] = []
  if (salud.mongo !== 'ok') problemas.push('MongoDB no responde')
  if (salud.redis !== 'ok') problemas.push('Redis no responde')
  if (!salud.modelo_activo) problemas.push('no hay modelo activo')
  if (problemas.length > 0) {
    return { nivel: 'degradada', palabra: 'API con problemas', detalle: `La API responde, pero ${problemas.join(', ')}.` }
  }
  const nodos = salud.nodos_en_pool === 1 ? '1 nodo' : `${salud.nodos_en_pool} nodos`
  return {
    nivel: 'ok',
    palabra: 'API en línea',
    detalle: `MongoDB y Redis responden, ${nodos} en el pool, modelo ${salud.modelo_activo}.`,
  }
}

const CADA_MS = 10_000

/** La salud de la API, consultada cada 10 segundos. Es pública: funciona también sin sesión. */
export function useSalud(): SaludPresentada {
  const consulta = useQuery({
    queryKey: claves.salud,
    queryFn: ({ signal }) => pedir(api.GET('/api/salud', { signal })),
    refetchInterval: CADA_MS,
    // El punto tiene que ponerse rojo enseguida: sin reintentos silenciosos.
    retry: false,
  })
  return presentarSalud(consulta.isError ? undefined : consulta.data, consulta.isError)
}
