// El cliente de la API: fetch con los tipos generados del contrato
// (src/api/contrato.d.ts, que sale de docs/api/openapi.yaml con `pnpm tipos`).
//
//   const estaciones = await pedir(api.GET('/api/estaciones'))
//   const regimen = await pedir(api.GET('/api/estaciones/{id}/regimen', { params: { path: { id } } }))
//
// api.GET y api.POST conocen cada ruta, sus parámetros y lo que responde.
// pedir() devuelve los datos o lanza un ErrorApi ya traducido (api/errores.ts):
// es lo que espera TanStack Query en queryFn y mutationFn.

import { QueryClient } from '@tanstack/react-query'
import createClient, { type Middleware } from 'openapi-fetch'
import { cerrarSesion, sesionActual } from '../sesion/almacen'
import type { paths } from './contrato'
import { convieneReintentar, errorDeExcepcion, errorDeRespuesta, esCancelacion } from './errores'

const ENCABEZADO_TOKEN = 'Authorization'

const autenticacion: Middleware = {
  onRequest({ request }) {
    const sesion = sesionActual()
    if (sesion) request.headers.set(ENCABEZADO_TOKEN, `Bearer ${sesion.token}`)
    return request
  },
  onResponse({ request, response }) {
    // Un 401 a un pedido que llevaba el token de la sesión actual la cierra
    // (spec §9.5): la UI vuelve al ingreso. Si el token era de una sesión
    // anterior (la respuesta llegó tarde), la de ahora no se toca.
    const enviado = request.headers.get(ENCABEZADO_TOKEN)
    const sesion = sesionActual()
    if (response.status === 401 && sesion && enviado === `Bearer ${sesion.token}`) cerrarSesion('vencida')
    return response
  },
}

// La UI y la API comparten origen: en Compose lo arma Caddy y en desarrollo el
// proxy de Vite. Por eso no hay una URL de la API que configurar ni CORS.
export const api = createClient<paths>({
  baseUrl: globalThis.location?.origin ?? 'http://localhost',
  // Se busca fetch en cada pedido, no al crear el cliente: las pruebas lo reemplazan.
  fetch: (pedido) => globalThis.fetch(pedido),
})
api.use(autenticacion)

interface Resultado<T> {
  data?: T
  error?: unknown
  response: Response
}

/**
 * Espera un pedido de `api` y devuelve sus datos; si falla, lanza un ErrorApi.
 * `ingreso` marca el pedido del token: ahí un 401 es «usuario o clave
 * incorrectos», no una sesión vencida.
 */
export async function pedir<T>(pedido: Promise<Resultado<T>>, opciones: { ingreso?: boolean } = {}): Promise<T> {
  let resultado: Resultado<T>
  try {
    resultado = await pedido
  } catch (e) {
    if (esCancelacion(e)) throw e
    throw errorDeExcepcion(e)
  }
  if (resultado.response.ok) return resultado.data as T
  throw errorDeRespuesta(resultado.response.status, resultado.error, !opciones.ingreso)
}

/** La caché de TanStack Query de toda la app. Los eventos del WebSocket la actualizan. */
export function crearClienteDeConsultas(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: 5_000,
        // Solo se repite solo lo que podría salir bien a la segunda: un 403 o un 404 no.
        retry: (intentos, error) => intentos < 2 && convieneReintentar(error),
      },
      mutations: { retry: false },
    },
  })
}

/** Las claves de la caché, en un solo lugar para que una pantalla invalide lo de otra. */
export const claves = {
  salud: ['salud'] as const,
  estaciones: ['estaciones'] as const,
  regimenes: ['regimenes'] as const,
  regimen: (estacion: string) => ['regimenes', estacion] as const,
  modeloActivo: ['modelos', 'activo'] as const,
  nodos: ['cluster', 'nodos'] as const,
  entrenamientos: ['entrenamientos'] as const,
  entrenamiento: (id: string) => ['entrenamientos', id] as const,
  metricas: ['metricas'] as const,
}
