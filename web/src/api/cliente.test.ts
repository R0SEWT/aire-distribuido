import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { simularApi } from '../pruebas/dobles'
import { abrirSesion, cerrarSesion, estadoSesion, recargarSesion, sesionActual } from '../sesion/almacen'
import { api, pedir } from './cliente'
import { ErrorApi } from './errores'

function conSesion(token = 'token-vigente') {
  abrirSesion({ usuario: 'demo', token, rol: 'usuario', expira: Date.now() + 60_000 })
}

async function errorDe(promesa: Promise<unknown>): Promise<ErrorApi> {
  try {
    await promesa
  } catch (e) {
    expect(e).toBeInstanceOf(ErrorApi)
    return e as ErrorApi
  }
  throw new Error('se esperaba un error')
}

describe('el cliente de la API', () => {
  beforeEach(() => recargarSesion())
  afterEach(() => cerrarSesion('salir'))

  it('manda el token de la sesión en Authorization y devuelve los datos', async () => {
    conSesion()
    const fetch = simularApi({ 'GET /api/estaciones': { estado: 200, cuerpo: [] } })

    const estaciones = await pedir(api.GET('/api/estaciones'))

    expect(estaciones).toEqual([])
    expect(fetch.mock.calls[0]![0].headers.get('Authorization')).toBe('Bearer token-vigente')
  })

  it('sin sesión no manda Authorization', async () => {
    const fetch = simularApi({
      'GET /api/salud': {
        estado: 200,
        cuerpo: { api: 'ok', mongo: 'ok', redis: 'ok', nodos_en_pool: 0, modelo_activo: null },
      },
    })

    await pedir(api.GET('/api/salud'))

    expect(fetch.mock.calls[0]![0].headers.has('Authorization')).toBe(false)
  })

  it('un 401 con sesión la cierra: la UI vuelve al ingreso', async () => {
    conSesion()
    simularApi({ 'GET /api/estaciones': { estado: 401, cuerpo: { error: 'token ausente, inválido o expirado' } } })

    const error = await errorDe(pedir(api.GET('/api/estaciones')))

    expect(error.tipo).toBe('sesion_vencida')
    expect(estadoSesion()).toEqual({ sesion: null, motivo: 'vencida' })
  })

  it('un 401 al ingresar no es una sesión vencida', async () => {
    simularApi({ 'POST /api/auth/token': { estado: 401, cuerpo: { error: 'usuario o clave incorrectos' } } })

    const error = await errorDe(
      pedir(api.POST('/api/auth/token', { body: { usuario: 'demo', clave: 'mala' } }), { ingreso: true }),
    )

    expect(error.tipo).toBe('credenciales')
    expect(estadoSesion().motivo).not.toBe('vencida')
  })

  it('un 401 que llega tarde, de un token anterior, no cierra la sesión nueva', async () => {
    conSesion('token-viejo')
    simularApi({
      'GET /api/estaciones': () => {
        conSesion('token-nuevo') // la persona volvió a ingresar mientras el pedido viajaba
        return { estado: 401, cuerpo: { error: 'token ausente, inválido o expirado' } }
      },
    })

    await errorDe(pedir(api.GET('/api/estaciones')))

    expect(sesionActual()?.token).toBe('token-nuevo')
  })

  it('un 403 no cierra la sesión', async () => {
    conSesion()
    simularApi({ 'GET /api/metricas': { estado: 403, cuerpo: { error: 'se requiere el rol admin' } } })

    const error = await errorDe(pedir(api.GET('/api/metricas')))

    expect(error.tipo).toBe('sin_permiso')
    expect(error.detalle).toBe('se requiere el rol admin')
    expect(sesionActual()).not.toBeNull()
  })

  it('si la API no responde, lanza «sin respuesta»', async () => {
    simularApi({ 'GET /api/salud': 'sin_respuesta' })

    expect((await errorDe(pedir(api.GET('/api/salud')))).tipo).toBe('sin_respuesta')
  })

  it('un 502 del proxy sin cuerpo JSON es «no disponible»', async () => {
    conSesion()
    simularApi({ 'GET /api/estaciones': { estado: 502 } })

    const error = await errorDe(pedir(api.GET('/api/estaciones')))

    expect(error.tipo).toBe('no_disponible')
    expect(error.detalle).toBeNull()
    expect(sesionActual()).not.toBeNull()
  })
})
