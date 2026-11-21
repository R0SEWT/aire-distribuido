import { QueryClient } from '@tanstack/react-query'
import { describe, expect, it } from 'vitest'
import { claves } from './cliente'
import {
  aplicarEstadoEstacion,
  EN_VIVO,
  esMasReciente,
  pedirRegimen,
  pedirRegimenes,
  refrescarEnVivo,
  superponer,
  type EstadoEstacion,
} from './enVivo'
import { errorDeRespuesta } from './errores'

/** Un estado de la estación a una hora del 13/12/2021, calculado `ts` segundos después. */
function estado(estacion: string, hora: number, extra: Partial<EstadoEstacion> = {}, ts = 2): EstadoEstacion {
  const hh = String(hora).padStart(2, '0')
  return {
    estacion,
    fecha_hora: `2021-12-13T${hh}:00:00-05:00`,
    regimen: 0,
    excede_eca: false,
    media_24h: { pm2_5: 10, pm10: 20, lecturas: 24 },
    alerta: false,
    inca: 'buena',
    ts: `2021-12-13T${hh}:00:${String(ts).padStart(2, '0')}-05:00`,
    ...extra,
  }
}

/** Una promesa que se resuelve a mano, para hacer llegar un evento mientras un GET viaja. */
function pendiente<T>() {
  let resolver!: (valor: T) => void
  let rechazar!: (error: unknown) => void
  const promesa = new Promise<T>((res, rej) => {
    resolver = res
    rechazar = rej
  })
  return { promesa, resolver, rechazar }
}

const nuevaCache = () => new QueryClient({ defaultOptions: { queries: { retry: false } } })

describe('esMasReciente', () => {
  it('gana la hora de lectura más tardía', () => {
    expect(esMasReciente(estado('A', 15), estado('A', 14))).toBe(true)
    expect(esMasReciente(estado('A', 14), estado('A', 15))).toBe(false)
  })

  it('a igual hora de lectura, gana el que se calculó después (la lectura se repitió)', () => {
    expect(esMasReciente(estado('A', 15, {}, 9), estado('A', 15, {}, 2))).toBe(true)
    expect(esMasReciente(estado('A', 15, {}, 2), estado('A', 15, {}, 9))).toBe(false)
    expect(esMasReciente(estado('A', 15), estado('A', 15))).toBe(true)
  })

  it('compara instantes, no textos: la misma hora en otra zona es la misma', () => {
    const enUtc = { ...estado('A', 15), fecha_hora: '2021-12-13T20:00:00Z', ts: '2021-12-13T20:00:02Z' }

    expect(esMasReciente(enUtc, estado('A', 15))).toBe(true)
    expect(esMasReciente(estado('A', 15), enUtc)).toBe(true)
  })
})

describe('superponer', () => {
  it('reemplaza el estado de una estación por uno más reciente y deja las demás', () => {
    const lista = [estado('CA-A', 14), estado('CA-B', 14)]

    const nueva = superponer(lista, [estado('CA-B', 15, { alerta: true })])

    expect(nueva).toEqual([estado('CA-A', 14), estado('CA-B', 15, { alerta: true })])
    expect(lista[1]).toEqual(estado('CA-B', 14)) // la lista original no se toca
  })

  it('una estación que no estaba entra, en su lugar por código', () => {
    const nueva = superponer([estado('CA-A', 14), estado('CA-C', 14)], [estado('CA-B', 14)])

    expect(nueva.map((e) => e.estacion)).toEqual(['CA-A', 'CA-B', 'CA-C'])
  })

  it('un estado más viejo que el que hay no lo reemplaza: el último estado no retrocede', () => {
    const lista = [estado('CA-A', 15, { alerta: true })]

    expect(superponer(lista, [estado('CA-A', 9)])).toEqual(lista)
  })
})

describe('aplicarEstadoEstacion', () => {
  it('actualiza la lista y la entrada de la estación con un solo evento', () => {
    const consultas = nuevaCache()
    consultas.setQueryData(claves.regimenes, [estado('CA-A', 14), estado('CA-B', 14)])
    consultas.setQueryData(claves.regimen('CA-B'), estado('CA-B', 14))

    aplicarEstadoEstacion(consultas, estado('CA-B', 15, { alerta: true, inca: 'mala' }))

    expect(consultas.getQueryData(claves.regimenes)).toEqual([
      estado('CA-A', 14),
      estado('CA-B', 15, { alerta: true, inca: 'mala' }),
    ])
    expect(consultas.getQueryData(claves.regimen('CA-B'))).toMatchObject({ alerta: true, inca: 'mala' })
    expect(consultas.getQueryData(claves.regimen('CA-A'))).toBeUndefined()
  })

  it('si la lista todavía no se pidió, no inventa una con una sola estación', () => {
    const consultas = nuevaCache()

    aplicarEstadoEstacion(consultas, estado('CA-B', 15))

    expect(consultas.getQueryData(claves.regimenes)).toBeUndefined()
    expect(consultas.getQueryData(claves.regimen('CA-B'))).toEqual(estado('CA-B', 15))
  })

  it('una estación que no tenía estado entra a la lista ya pedida', () => {
    const consultas = nuevaCache()
    consultas.setQueryData(claves.regimenes, [])

    aplicarEstadoEstacion(consultas, estado('CA-B', 15))

    expect(consultas.getQueryData(claves.regimenes)).toEqual([estado('CA-B', 15)])
  })

  it('un evento más viejo que lo guardado no pisa nada', () => {
    const consultas = nuevaCache()
    consultas.setQueryData(claves.regimenes, [estado('CA-B', 15)])
    consultas.setQueryData(claves.regimen('CA-B'), estado('CA-B', 15))

    aplicarEstadoEstacion(consultas, estado('CA-B', 9, { alerta: true }))

    expect(consultas.getQueryData(claves.regimenes)).toEqual([estado('CA-B', 15)])
    expect(consultas.getQueryData(claves.regimen('CA-B'))).toEqual(estado('CA-B', 15))
  })
})

describe('un evento que llega mientras un GET viaja', () => {
  it('la lista: la respuesta, que es de antes del evento, no lo pisa', async () => {
    const consultas = nuevaCache()
    const get = pendiente<EstadoEstacion[]>()
    const pedido = consultas.fetchQuery({
      queryKey: claves.regimenes,
      meta: EN_VIVO,
      queryFn: () => pedirRegimenes(consultas, () => get.promesa),
    })

    aplicarEstadoEstacion(consultas, estado('CA-B', 15, { alerta: true })) // llega con el GET en viaje
    get.resolver([estado('CA-A', 14), estado('CA-B', 14)]) // la API respondió con lo de antes

    expect(await pedido).toEqual([estado('CA-A', 14), estado('CA-B', 15, { alerta: true })])
    expect(consultas.getQueryData(claves.regimenes)).toEqual([estado('CA-A', 14), estado('CA-B', 15, { alerta: true })])
  })

  it('la lista: si la respuesta ya trae algo más reciente que el evento, gana la respuesta', async () => {
    const consultas = nuevaCache()
    const get = pendiente<EstadoEstacion[]>()
    const pedido = pedirRegimenes(consultas, () => get.promesa)

    aplicarEstadoEstacion(consultas, estado('CA-B', 15))
    get.resolver([estado('CA-B', 16)])

    expect(await pedido).toEqual([estado('CA-B', 16)])
  })

  it('la lista: un evento de antes o de después del pedido no se mezcla con su respuesta', async () => {
    const consultas = nuevaCache()
    aplicarEstadoEstacion(consultas, estado('CA-X', 15)) // antes: el sistema se reinició después y ya no existe

    const lista = await pedirRegimenes(consultas, async () => [])
    aplicarEstadoEstacion(consultas, estado('CA-Y', 15)) // después

    expect(lista).toEqual([]) // una API recién levantada responde vacío, y vacío queda
    expect(await pedirRegimenes(consultas, async () => [])).toEqual([])
  })

  it('una estación: gana el evento si es más reciente que la respuesta', async () => {
    const consultas = nuevaCache()
    const get = pendiente<EstadoEstacion>()
    const pedido = pedirRegimen(consultas, 'CA-B', () => get.promesa)

    aplicarEstadoEstacion(consultas, estado('CA-A', 18)) // de otra estación: no cuenta
    aplicarEstadoEstacion(consultas, estado('CA-B', 15, { alerta: true }))
    get.resolver(estado('CA-B', 14))

    expect(await pedido).toEqual(estado('CA-B', 15, { alerta: true }))
  })

  it('una estación: si la API respondió 404 porque aún no tenía estado y llegó su primer evento, vale el evento', async () => {
    const consultas = nuevaCache()
    const get = pendiente<EstadoEstacion>()
    const pedido = pedirRegimen(consultas, 'CA-B', () => get.promesa)

    aplicarEstadoEstacion(consultas, estado('CA-B', 15))
    get.rechazar(errorDeRespuesta(404, { error: 'la estación CA-B todavía no tiene un régimen registrado' }, true))

    expect(await pedido).toEqual(estado('CA-B', 15))
  })

  it('una estación: sin evento, el 404 y cualquier otro error siguen siendo errores', async () => {
    const consultas = nuevaCache()
    const noEsta = errorDeRespuesta(404, { error: 'sin régimen' }, true)
    const caida = errorDeRespuesta(503, { error: 'Redis no disponible' }, true)

    await expect(pedirRegimen(consultas, 'CA-B', () => Promise.reject(noEsta))).rejects.toBe(noEsta)

    const get = pendiente<EstadoEstacion>()
    const pedido = pedirRegimen(consultas, 'CA-B', () => get.promesa)
    aplicarEstadoEstacion(consultas, estado('CA-B', 15))
    get.rechazar(caida)
    await expect(pedido).rejects.toBe(caida) // un 503 no se tapa con el evento
  })
})

describe('refrescarEnVivo', () => {
  it('marca para volver a pedir las consultas EN_VIVO y deja en paz a las demás', async () => {
    const consultas = nuevaCache()
    await consultas.fetchQuery({ queryKey: claves.regimenes, meta: EN_VIVO, queryFn: async () => [] })
    await consultas.fetchQuery({ queryKey: claves.regimen('CA-B'), meta: EN_VIVO, queryFn: async () => estado('CA-B', 14) })
    await consultas.fetchQuery({ queryKey: claves.estaciones, queryFn: async () => [] })
    const invalidada = (clave: readonly unknown[]) => consultas.getQueryCache().find({ queryKey: clave })!.state.isInvalidated

    await refrescarEnVivo(consultas)

    // Que una consulta con alguien mirándola se vuelve a pedir de verdad lo prueba App.test.tsx.
    expect(invalidada(claves.regimenes)).toBe(true)
    expect(invalidada(claves.regimen('CA-B'))).toBe(true)
    expect(invalidada(claves.estaciones)).toBe(false)
  })
})
