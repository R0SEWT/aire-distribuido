// Los archivos de /datos/: cómo se piden, qué pasa si faltan o no tienen la
// forma esperada, y que dos pantallas que usan el mismo archivo lo pidan una vez.

import { QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { describe, expect, it, vi } from 'vitest'
import crudoDeImpacto from '../../public/datos/impacto.json?raw'
import crudoDeSeries from '../../public/datos/series.json?raw'
import { respuesta, simularApi } from '../pruebas/dobles'
import { claves, crearClienteDeConsultas } from './cliente'
import { esImpacto, esSeries, pedirDatos, useImpacto, useSeries } from './datos'
import { ErrorApi } from './errores'

const IMPACTO: unknown = JSON.parse(crudoDeImpacto)
const SERIES: unknown = JSON.parse(crudoDeSeries)

function conCache() {
  const consultas = crearClienteDeConsultas()
  consultas.setDefaultOptions({ queries: { retry: false } })
  const wrapper = ({ children }: { children: ReactNode }) => <QueryClientProvider client={consultas}>{children}</QueryClientProvider>
  return { consultas, wrapper }
}

describe('pedirDatos', () => {
  it('devuelve el archivo ya leído', async () => {
    const fetch = simularApi({ 'GET /datos/series.json': { estado: 200, cuerpo: { dias: [] } } })
    await expect(pedirDatos('series.json')).resolves.toEqual({ dias: [] })
    expect(fetch).toHaveBeenCalledTimes(1)
  })

  it('si el archivo no está, lanza un ErrorApi que dice cuál', async () => {
    simularApi({})
    const error = await pedirDatos('impacto.json').catch((e: unknown) => e)
    expect(error).toBeInstanceOf(ErrorApi)
    expect((error as ErrorApi).tipo).toBe('no_encontrado')
    expect((error as ErrorApi).detalle).toMatch(/no se pudo leer \/datos\/impacto\.json \(respondió 404\)/)
  })

  it('si lo que llega no es JSON (Vite responde la página), lo dice', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response('<!doctype html>', { status: 200 })))
    const error = await pedirDatos('peru.geojson').catch((e: unknown) => e)
    expect(error).toBeInstanceOf(ErrorApi)
    expect((error as ErrorApi).detalle).toBe('/datos/peru.geojson no es un JSON válido')
  })

  it('si la API no responde, es «sin respuesta»', async () => {
    simularApi({ 'GET /datos/series.json': 'sin_respuesta' })
    const error = await pedirDatos('series.json').catch((e: unknown) => e)
    expect((error as ErrorApi).tipo).toBe('sin_respuesta')
  })
})

describe('la forma de los archivos', () => {
  it('los archivos de public/datos/ tienen la forma que las pantallas esperan', () => {
    expect(esImpacto(IMPACTO)).toBe(true)
    expect(esSeries(SERIES)).toBe(true)
  })

  it('un archivo al que le falta una parte no pasa', () => {
    const { poblacion_distritos: _, ...sinPoblacion } = IMPACTO as Record<string, unknown>
    expect(esImpacto(sinPoblacion)).toBe(false)
    expect(esImpacto(null)).toBe(false)
    expect(esSeries({ dias: [] })).toBe(false)
  })
})

describe('los hooks de /datos/', () => {
  it('dos pantallas que usan impacto.json lo piden una sola vez y comparten la caché', async () => {
    const fetch = simularApi({ 'GET /datos/impacto.json': { estado: 200, cuerpo: IMPACTO } })
    const { consultas, wrapper } = conCache()
    const una = renderHook(() => useImpacto(), { wrapper })
    const otra = renderHook(() => useImpacto(), { wrapper })
    await waitFor(() => expect(una.result.current.isSuccess).toBe(true))
    await waitFor(() => expect(otra.result.current.isSuccess).toBe(true))
    expect(fetch).toHaveBeenCalledTimes(1)
    expect(otra.result.current.data).toBe(una.result.current.data)
    expect(consultas.getQueryData(claves.datosImpacto)).toBe(una.result.current.data)

    // Una tercera, montada después, tampoco lo vuelve a pedir.
    const tercera = renderHook(() => useImpacto(), { wrapper })
    expect(tercera.result.current.data).toBe(una.result.current.data)
    expect(fetch).toHaveBeenCalledTimes(1)
  })

  it('un archivo sin la forma esperada es un error, no una pantalla a medias', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => respuesta(200, { dias: [] })))
    const { wrapper } = conCache()
    const series = renderHook(() => useSeries(), { wrapper })
    await waitFor(() => expect(series.result.current.isError).toBe(true))
    expect((series.result.current.error as ErrorApi).detalle).toBe('/datos/series.json no tiene la forma esperada')
  })
})
