// Las consultas que comparten las pantallas sobre las estaciones: el catálogo
// (GET /api/estaciones) y el último estado recibido de cada una, que la caché
// en vivo mantiene al día (web/README.md, «El último estado de las estaciones»).

import { useQuery, useQueryClient } from '@tanstack/react-query'
import { api, claves, pedir } from './cliente'
import type { components } from './contrato'
import { EN_VIVO, pedirRegimen, pedirRegimenes } from './enVivo'
import { ErrorApi } from './errores'

export type Estacion = components['schemas']['Estacion']
export type RegimenEstacion = components['schemas']['RegimenEstacion']

/** El catálogo: las 33 estaciones con su ficha y sus coordenadas. Cambia solo si se vuelve a sembrar. */
export function useEstaciones() {
  return useQuery({
    queryKey: claves.estaciones,
    queryFn: ({ signal }) => pedir(api.GET('/api/estaciones', { signal })),
    staleTime: 5 * 60_000,
  })
}

/** El último estado de todas las estaciones que tienen uno, al día por el WebSocket. */
export function useEstados() {
  const consultas = useQueryClient()
  return useQuery({
    queryKey: claves.regimenes,
    meta: EN_VIVO,
    queryFn: ({ signal }) => pedirRegimenes(consultas, () => pedir(api.GET('/api/regimenes', { signal }))),
  })
}

/**
 * El último estado de una estación, al día por el WebSocket. null si todavía
 * no tiene ninguno: la API responde 404, y para la pantalla no es un error.
 */
export function useEstado(id: string) {
  const consultas = useQueryClient()
  return useQuery({
    queryKey: claves.regimen(id),
    meta: EN_VIVO,
    queryFn: async ({ signal }): Promise<RegimenEstacion | null> => {
      try {
        return await pedirRegimen(consultas, id, () =>
          pedir(api.GET('/api/estaciones/{id}/regimen', { params: { path: { id } }, signal })),
        )
      } catch (e) {
        if (e instanceof ErrorApi && e.tipo === 'no_encontrado') return null
        throw e
      }
    },
  })
}
