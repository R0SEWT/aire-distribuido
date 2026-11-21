import { describe, expect, it } from 'vitest'
import {
  convieneReintentar,
  ErrorApi,
  errorDeExcepcion,
  errorDeFetch,
  errorDeRespuesta,
  esFalloDeCarga,
  presentarError,
  textoDeError,
} from './errores'

describe('la traducción de los errores de la API', () => {
  it.each([
    [400, 'pedido_invalido'],
    [401, 'sesion_vencida'],
    [403, 'sin_permiso'],
    [404, 'no_encontrado'],
    [409, 'conflicto'],
    [422, 'pedido_invalido'],
    [500, 'inesperado'],
    [503, 'no_disponible'],
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

  it.each([
    [502, ''],
    [502, '<html>502 Bad Gateway</html>'],
    [504, ''],
    [503, ''],
  ])('un %i del proxy, sin las palabras de la API (%j), es «la API no responde», no un problema de dependencias', (estado, cuerpo) => {
    const error = errorDeRespuesta(estado, cuerpo, true)
    const p = presentarError(error)

    expect(error.tipo).toBe('sin_respuesta')
    expect(error.estado).toBe(estado)
    expect(p.titulo).toBe('La API no responde')
    expect(p.detalle).toMatch(/apagada o reiniciándose/)
    expect(p.detalle).not.toMatch(/está en marcha/)
    expect(p.reintentable).toBe(true)
  })

  it('un 401 con sesión se muestra como sesión vencida', () => {
    expect(presentarError(errorDeRespuesta(401, { error: 'token ausente, inválido o expirado' }, true))).toMatchObject({
      estado: 'sesion-vencida',
      titulo: 'La sesión venció',
      reintentable: false,
    })
  })

  it('que fetch no llegue a la API es «la API no responde»', () => {
    const error = errorDeFetch(new TypeError('Failed to fetch'))

    expect(error).toBeInstanceOf(ErrorApi)
    expect(error.tipo).toBe('sin_respuesta')
    expect(error.estado).toBeNull()
    expect(presentarError(error)).toMatchObject({ titulo: 'La API no responde', reintentable: true })
  })

  it('un TypeError que no viene de fetch es un error del programa: inesperado y sin reintento', () => {
    const p = presentarError(new TypeError("Cannot read properties of undefined (reading 'pm10')"))

    expect(p).toEqual({
      estado: 'error',
      titulo: 'Algo falló',
      detalle: "Cannot read properties of undefined (reading 'pm10')",
      reintentable: false,
    })
    expect(convieneReintentar(new TypeError('Failed to fetch'))).toBe(false)
  })

  it('un 500 de la API es inesperado pero se puede reintentar', () => {
    expect(presentarError(errorDeRespuesta(500, { error: 'error interno' }, true))).toMatchObject({
      titulo: 'Algo falló',
      detalle: 'error interno',
      reintentable: true,
    })
  })

  it('cualquier otra excepción es un error inesperado, y un ErrorApi pasa tal cual', () => {
    expect(errorDeExcepcion(new Error('se rompió')).tipo).toBe('inesperado')
    expect(presentarError('un texto suelto')).toMatchObject({ estado: 'error', reintentable: false })
    const original = errorDeRespuesta(404, { error: 'no existe' }, true)
    expect(errorDeExcepcion(original)).toBe(original)
    expect(errorDeFetch(original)).toBe(original)
  })

  it('solo se repite solo lo que podría salir bien a la segunda', () => {
    expect(convieneReintentar(errorDeRespuesta(503, { error: 'MongoDB no disponible' }, true))).toBe(true)
    expect(convieneReintentar(errorDeRespuesta(502, '', true))).toBe(true)
    expect(convieneReintentar(errorDeFetch(new TypeError('Failed to fetch')))).toBe(true)
    expect(convieneReintentar(errorDeRespuesta(403, null, true))).toBe(false)
    expect(convieneReintentar(errorDeRespuesta(404, null, true))).toBe(false)
    expect(convieneReintentar(errorDeRespuesta(401, null, true))).toBe(false)
    expect(convieneReintentar(new Error('se rompió'))).toBe(false)
  })

  it.each([
    'Failed to fetch dynamically imported module: http://localhost:8081/assets/Panel-abc123.js',
    'error loading dynamically imported module: http://localhost:8081/assets/Panel-abc123.js',
    'Importing a module script failed.',
    'Unable to preload CSS for /assets/Panel-abc123.css',
  ])('reconoce un trozo de la app que no se pudo descargar: %s', (mensaje) => {
    expect(esFalloDeCarga(new TypeError(mensaje))).toBe(true)
  })

  it('un error cualquiera no es un fallo de carga', () => {
    expect(esFalloDeCarga(new Error('se rompió'))).toBe(false)
    expect(esFalloDeCarga(new TypeError('Failed to fetch'))).toBe(false)
    expect(esFalloDeCarga('texto')).toBe(false)
  })
})
