import { describe, expect, it } from 'vitest'
import { convieneReintentar, ErrorApi, errorDeExcepcion, errorDeRespuesta, presentarError, textoDeError } from './errores'

describe('la traducción de los errores de la API', () => {
  it.each([
    [400, 'pedido_invalido'],
    [401, 'sesion_vencida'],
    [403, 'sin_permiso'],
    [404, 'no_encontrado'],
    [409, 'conflicto'],
    [422, 'pedido_invalido'],
    [500, 'inesperado'],
    [502, 'no_disponible'],
    [503, 'no_disponible'],
    [504, 'no_disponible'],
  ] as const)('un %i con sesión es «%s»', (estado, tipo) => {
    expect(errorDeRespuesta(estado, { error: 'x' }, true).tipo).toBe(tipo)
  })

  it('un 401 al ingresar es «usuario o clave incorrectos», no una sesión vencida', () => {
    const error = errorDeRespuesta(401, { error: 'usuario o clave incorrectos' }, false)

    expect(error.tipo).toBe('credenciales')
    expect(presentarError(error)).toMatchObject({ estado: 'error', titulo: 'Usuario o clave incorrectos' })
  })

  it('guarda las palabras de la API de {"error": "…"}', () => {
    const error = errorDeRespuesta(422, { error: 'hay 2 nodos libres en el pool y se pidieron 4' }, true)

    expect(error.detalle).toBe('hay 2 nodos libres en el pool y se pidieron 4')
    expect(presentarError(error).detalle).toBe('hay 2 nodos libres en el pool y se pidieron 4')
  })

  it.each([[''], ['<html>502 Bad Gateway</html>'], [null], [{ otro: 'campo' }], [{ error: '   ' }], [{ error: 42 }]])(
    'un cuerpo que no es {"error": "…"} (%j) no da detalle',
    (cuerpo) => {
      expect(textoDeError(cuerpo)).toBeNull()
    },
  )

  it('un 403 se muestra como «sin permiso» y no ofrece reintentar', () => {
    const p = presentarError(errorDeRespuesta(403, { error: 'se requiere el rol admin' }, true))

    expect(p).toEqual({
      estado: 'sin-permiso',
      titulo: 'No tienes permiso para ver esto',
      detalle: 'se requiere el rol admin',
      reintentable: false,
    })
  })

  it('un 503 es «el servicio no está disponible», con lo que dijo la API, y se puede reintentar', () => {
    const p = presentarError(errorDeRespuesta(503, { error: 'MongoDB no disponible' }, true))

    expect(p).toEqual({
      estado: 'error',
      titulo: 'El servicio no está disponible',
      detalle: 'MongoDB no disponible',
      reintentable: true,
    })
  })

  it('un 502 del proxy, sin cuerpo, también es «no disponible» y explica por qué', () => {
    const p = presentarError(errorDeRespuesta(502, '', true))

    expect(p.titulo).toBe('El servicio no está disponible')
    expect(p.detalle).toMatch(/no responde/)
  })

  it('un 401 con sesión se muestra como sesión vencida', () => {
    expect(presentarError(errorDeRespuesta(401, { error: 'token ausente, inválido o expirado' }, true))).toMatchObject({
      estado: 'sesion-vencida',
      titulo: 'La sesión venció',
      reintentable: false,
    })
  })

  it('que fetch no llegue a la API es «la API no responde»', () => {
    const error = errorDeExcepcion(new TypeError('Failed to fetch'))

    expect(error).toBeInstanceOf(ErrorApi)
    expect(error.tipo).toBe('sin_respuesta')
    expect(error.estado).toBeNull()
    expect(presentarError(error)).toMatchObject({ titulo: 'La API no responde', reintentable: true })
  })

  it('cualquier otra excepción es un error inesperado, y un ErrorApi pasa tal cual', () => {
    expect(errorDeExcepcion(new Error('se rompió')).tipo).toBe('inesperado')
    expect(presentarError('un texto suelto').estado).toBe('error')
    const original = errorDeRespuesta(404, { error: 'no existe' }, true)
    expect(errorDeExcepcion(original)).toBe(original)
  })

  it('solo se repite solo lo que podría salir bien a la segunda', () => {
    expect(convieneReintentar(errorDeRespuesta(503, null, true))).toBe(true)
    expect(convieneReintentar(new TypeError('Failed to fetch'))).toBe(true)
    expect(convieneReintentar(errorDeRespuesta(403, null, true))).toBe(false)
    expect(convieneReintentar(errorDeRespuesta(404, null, true))).toBe(false)
    expect(convieneReintentar(errorDeRespuesta(401, null, true))).toBe(false)
  })
})
