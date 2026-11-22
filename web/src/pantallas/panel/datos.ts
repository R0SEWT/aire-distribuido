// Los datos que comparten el Panel y la Estación: el catálogo (GET
// /api/estaciones), los archivos estáticos de /datos/ (public/datos/README.md)
// y el último estado de las estaciones, que la caché en vivo mantiene al día.

import { useQuery, useQueryClient } from '@tanstack/react-query'
import type { ExtendedFeatureCollection } from 'd3'
import { api, claves, pedir } from '../../api/cliente'
import type { components } from '../../api/contrato'
import { EN_VIVO, pedirRegimen, pedirRegimenes } from '../../api/enVivo'
import { ErrorApi, errorDeFetch, errorDeRespuesta, esCancelacion } from '../../api/errores'

export type Estacion = components['schemas']['Estacion']
export type RegimenEstacion = components['schemas']['RegimenEstacion']

/** Las claves de caché de los archivos de /datos/. No están en `claves` (api/cliente.ts). */
export const clavesDatos = {
  impacto: ['datos', 'impacto'] as const,
  mapa: ['datos', 'peru'] as const,
}

/** Lo que estas dos pantallas leen de impacto.json (lo escribe scripts/impacto.py). */
export interface Impacto {
  alcance: {
    estaciones_con_pm_horario: number
    /** Las que no están en el gold: de muestreo manual o sin PM2.5 y PM10 horarios. */
    estaciones_sin_pm_horario: string[]
  }
  estaciones: {
    estacion: string
    primera_fecha: string
    ultima_fecha: string
    dias_con_datos: number
  }[]
  poblacion_distritos: {
    ubigeo: string
    distrito: string
    poblacion: number
    anio: number
    fuente: string
    estaciones: string[]
  }[]
}

/** Los 26 polígonos de peru.geojson, con el departamento de cada uno. */
export type MapaDelPeru = ExtendedFeatureCollection

/**
 * Pide un archivo de /datos/. Lanza un ErrorApi como pedir(), para que la
 * pantalla lo muestre con <EstadoDeError>.
 */
export async function pedirDatos<T>(archivo: string, signal?: AbortSignal): Promise<T> {
  let respuesta: Response
  try {
    respuesta = await globalThis.fetch(new Request(new URL(`/datos/${archivo}`, globalThis.location.origin), { signal }))
  } catch (e) {
    if (esCancelacion(e)) throw e
    throw errorDeFetch(e)
  }
  if (!respuesta.ok) throw errorDeRespuesta(respuesta.status, null, true)
  try {
    return (await respuesta.json()) as T
  } catch {
    // En desarrollo, Vite responde la página a un archivo que no existe.
    throw new ErrorApi('inesperado', respuesta.status, `/datos/${archivo} no es un JSON`)
  }
}

/** El catálogo: las 33 estaciones con su ficha y sus coordenadas. Cambia solo si se vuelve a sembrar. */
export function useEstaciones() {
  return useQuery({
    queryKey: claves.estaciones,
    queryFn: ({ signal }) => pedir(api.GET('/api/estaciones', { signal })),
    staleTime: 5 * 60_000,
  })
}

/** impacto.json: qué estaciones no miden PM por hora, cuándo midió cada una y la población de su distrito. */
export function useImpacto() {
  return useQuery({
    queryKey: clavesDatos.impacto,
    queryFn: ({ signal }) => pedirDatos<Impacto>('impacto.json', signal),
    staleTime: Infinity,
  })
}

export function useMapaDelPeru() {
  return useQuery({
    queryKey: clavesDatos.mapa,
    queryFn: ({ signal }) => pedirDatos<MapaDelPeru>('peru.geojson', signal),
    staleTime: Infinity,
  })
}

/** El último estado de todas las estaciones que tienen uno, al día por el WebSocket (web/README.md). */
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
