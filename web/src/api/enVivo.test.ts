import { QueryClient } from '@tanstack/react-query'
import { describe, expect, it } from 'vitest'
import { claves } from './cliente'
import {
  alConectarEnVivo,
  alDesconectarEnVivo,
  aplicarEstadoEstacion,
  EN_VIVO,
  fusionar,
  olvidarEventosEnVivo,
  pedirRegimen,
  pedirRegimenes,
  type EstadoEstacion,
} from './enVivo'
import { errorDeRespuesta } from './errores'

/** Un estado de la estación a una hora del 13/12/2021. */
function estado(estacion: string, hora: number, extra: Partial<EstadoEstacion> = {}): EstadoEstacion {
  const hh = String(hora).padStart(2, '0')
  return {
    estacion,
    fecha_hora: `2021-12-13T${hh}:00:00-05:00`,
    regimen: 0,
    excede_eca: false,
    media_24h: { pm2_5: 10, pm10: 20, lecturas: 24 },
    alerta: false,
    inca: 'buena',
    ts: `2021-12-13T${hh}:00:02-05:00`,
    ...extra,
  }
}

/** Una promesa que se resuelve a mano, para entrelazar un evento con un GET que viaja. */
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

/** La lista como la pide una pantalla: marcada en vivo y con el ayudante. */
function pedirLista(consultas: QueryClient, get: () => Promise<EstadoEstacion[]>) {
  return consultas.fetchQuery({
    queryKey: claves.regimenes,
    meta: EN_VIVO,
    queryFn: () => pedirRegimenes(consultas, get),
  })
}

describe('fusionar', () => {
  it('sin eventos, la lista es la del GET, ordenada por código', () => {
    expect(fusionar([estado('CA-B', 14), estado('CA-A', 14)], new Map()).map((e) => e.estacion)).toEqual(['CA-A', 'CA-B'])
  })

  it('de una estación con evento se ignora la entrada del GET; las demás son las del GET', () => {
    const delGet = [estado('CA-A', 14), estado('CA-B', 14)]
    const eventos = new Map([['CA-B', estado('CA-B', 15, { alerta: true })]])

    expect(fusionar(delGet, eventos)).toEqual([estado('CA-A', 14), estado('CA-B', 15, { alerta: true })])
    expect(delGet).toEqual([estado('CA-A', 14), estado('CA-B', 14)]) // no modifica lo que recibe
  })

  it('una estación con evento que el GET no traía entra a la lista', () => {
    const eventos = new Map([['CA-B', estado('CA-B', 15)]])

    expect(fusionar([estado('CA-A', 14), estado('CA-C', 14)], eventos).map((e) => e.estacion)).toEqual(['CA-A', 'CA-B', 'CA-C'])
  })

  it('no compara fechas: vale el evento aunque la entrada del GET diga una hora posterior', () => {
    // Si el GET de verdad traía algo más nuevo, su evento llega detrás por el WebSocket.
    const eventos = new Map([['CA-B', estado('CA-B', 15)]])

    expect(fusionar([estado('CA-B', 16)], eventos)).toEqual([estado('CA-B', 15)])
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

  it('una estación que no tenía estado entra a la lista ya pedida, en su lugar por código', () => {
    const consultas = nuevaCache()
    consultas.setQueryData(claves.regimenes, [estado('CA-A', 14), estado('CA-C', 14)])

    aplicarEstadoEstacion(consultas, estado('CA-B', 15))

    expect(consultas.getQueryData<EstadoEstacion[]>(claves.regimenes)!.map((e) => e.estacion)).toEqual(['CA-A', 'CA-B', 'CA-C'])
  })

  it('entre dos eventos manda el que llegó último, sin mirar fechas: una corrección de la misma hora se ve', () => {
    const consultas = nuevaCache()
    consultas.setQueryData(claves.regimenes, [])
    const primera = estado('CA-B', 15, { media_24h: { pm2_5: 10, pm10: 90, lecturas: 24 } })
    // La misma hora, corregida: misma fecha_hora y, porque ts va al segundo, mismo ts.
    const corregida = estado('CA-B', 15, { media_24h: { pm2_5: 10, pm10: 121.4, lecturas: 24 }, alerta: true })

    aplicarEstadoEstacion(consultas, primera)
    aplicarEstadoEstacion(consultas, corregida)

    expect(consultas.getQueryData(claves.regimenes)).toEqual([corregida])
    expect(consultas.getQueryData(claves.regimen('CA-B'))).toEqual(corregida)
  })
})

describe('la fusión del GET con los eventos', () => {
  it('llega el evento S2 y después la respuesta del GET con S1: queda S2', async () => {
    const consultas = nuevaCache()
    void alConectarEnVivo(consultas) // se abrió el WebSocket
    const S1 = estado('CA-B', 14)
    const S2 = estado('CA-B', 15, { alerta: true })
    const get = pendiente<EstadoEstacion[]>()
    const pedido = pedirLista(consultas, () => get.promesa) // el GET sale…

    aplicarEstadoEstacion(consultas, S2) // …llega el evento, más nuevo, por la otra conexión…
    get.resolver([estado('CA-A', 14), S1]) // …y recién entonces la respuesta, que es de antes

    expect(await pedido).toEqual([estado('CA-A', 14), S2])
    expect(consultas.getQueryData(claves.regimenes)).toEqual([estado('CA-A', 14), S2])
  })

  it('lo mismo si el evento llegó antes de que saliera el GET, en esta conexión', async () => {
    const consultas = nuevaCache()
    void alConectarEnVivo(consultas)
    const S2 = estado('CA-B', 15, { alerta: true })
    aplicarEstadoEstacion(consultas, S2)

    const lista = await pedirLista(consultas, async () => [estado('CA-A', 14), estado('CA-B', 14)])

    expect(lista).toEqual([estado('CA-A', 14), S2])
  })

  it('y aunque el GET traiga el mismo ts o una fecha_hora posterior: del evento no se duda', async () => {
    const consultas = nuevaCache()
    void alConectarEnVivo(consultas)
    const evento = estado('CA-B', 15, { alerta: true })
    aplicarEstadoEstacion(consultas, evento)

    expect(await pedirLista(consultas, async () => [estado('CA-B', 15)])).toEqual([evento]) // mismo ts y misma hora
    await consultas.invalidateQueries({ queryKey: claves.regimenes })
    expect(await pedirLista(consultas, async () => [estado('CA-B', 16)])).toEqual([evento]) // hora posterior
  })

  it('una estación sin evento en esta conexión toma lo que diga el GET, aunque en la caché hubiera otra cosa', async () => {
    const consultas = nuevaCache()
    void alConectarEnVivo(consultas)
    consultas.setQueryData(claves.regimenes, [estado('CA-A', 9)])

    expect(await pedirLista(consultas, async () => [estado('CA-A', 14)])).toEqual([estado('CA-A', 14)])
  })

  it('al reabrirse el WebSocket se olvidan los eventos: el GET vuelve a mandar', async () => {
    const consultas = nuevaCache()
    void alConectarEnVivo(consultas)
    aplicarEstadoEstacion(consultas, estado('CA-B', 15))

    // Corte y reconexión. Mientras tanto la estación avanzó y ese evento se perdió.
    void alConectarEnVivo(consultas)

    expect(await pedirLista(consultas, async () => [estado('CA-B', 17, { alerta: true })])).toEqual([
      estado('CA-B', 17, { alerta: true }),
    ])
  })

  it('con el WebSocket caído, un evento viejo no le gana a un GET más nuevo', async () => {
    const consultas = nuevaCache()
    void alConectarEnVivo(consultas)
    aplicarEstadoEstacion(consultas, estado('CA-B', 10))

    // Se cae el WebSocket y otra sesión manda la lectura de las 11: ese evento no llega.
    void alDesconectarEnVivo(consultas)

    expect(await pedirLista(consultas, async () => [estado('CA-B', 11, { alerta: true })])).toEqual([estado('CA-B', 11, { alerta: true })])
    expect(await pedirRegimen(consultas, 'CA-B', async () => estado('CA-B', 11, { alerta: true }))).toMatchObject({ alerta: true })
  })

  it('al caerse, lo que la caché sabe de las estaciones queda por volver a pedir, sin pedirlo todavía', async () => {
    const consultas = nuevaCache()
    const pedidos = { lista: 0 }
    await consultas.prefetchQuery({ queryKey: claves.regimenes, meta: EN_VIVO, queryFn: async () => (pedidos.lista++, [estado('CA-A', 10)]) })
    aplicarEstadoEstacion(consultas, estado('CA-B', 10))
    expect(consultas.getQueryState(claves.regimen('CA-B'))?.isInvalidated).toBe(false)

    await alDesconectarEnVivo(consultas)

    expect(consultas.getQueryState(claves.regimenes)?.isInvalidated).toBe(true)
    expect(consultas.getQueryState(claves.regimen('CA-B'))?.isInvalidated).toBe(true)
    expect(pedidos.lista).toBe(1)
  })

  it('al reconectar también se marcan las entradas por estación que creó un evento, que no llevan meta', async () => {
    const consultas = nuevaCache()
    void alConectarEnVivo(consultas)
    aplicarEstadoEstacion(consultas, estado('CA-B', 10))
    expect(consultas.getQueryCache().find({ queryKey: claves.regimen('CA-B') })?.meta).toBeUndefined()

    await alConectarEnVivo(consultas)

    expect(consultas.getQueryState(claves.regimen('CA-B'))?.isInvalidated).toBe(true)
  })

  it('al terminar la sesión se olvidan los eventos: en la siguiente manda la API, también su 404', async () => {
    const consultas = new QueryClient()
    aplicarEstadoEstacion(consultas, estado('CA-A', 15, { alerta: true }))

    olvidarEventosEnVivo(consultas)

    expect(await pedirRegimenes(consultas, async () => [])).toEqual([])
    await expect(
      pedirRegimen(consultas, 'CA-A', async () => {
        throw errorDeRespuesta(404, { error: 'la estación CA-A todavía no tiene un régimen registrado' }, true)
      }),
    ).rejects.toMatchObject({ tipo: 'no_encontrado' })
  })

  it('al reabrirse tras un reinicio limpio del sistema, la lista vacía del GET queda vacía', async () => {
    const consultas = nuevaCache()
    void alConectarEnVivo(consultas)
    aplicarEstadoEstacion(consultas, estado('CA-B', 15))
    expect(await pedirLista(consultas, async () => [])).toEqual([estado('CA-B', 15)])

    void alConectarEnVivo(consultas) // docker compose down -v y arriba otra vez, con la pestaña abierta
    await consultas.invalidateQueries({ queryKey: claves.regimenes })

    expect(await pedirLista(consultas, async () => [])).toEqual([])
  })

  it('un evento posterior a la reconexión vuelve a mandar sobre el GET que la reconexión disparó', async () => {
    const consultas = nuevaCache()
    void alConectarEnVivo(consultas)
    aplicarEstadoEstacion(consultas, estado('CA-B', 15))
    void alConectarEnVivo(consultas)
    const get = pendiente<EstadoEstacion[]>()
    const pedido = pedirLista(consultas, () => get.promesa)

    aplicarEstadoEstacion(consultas, estado('CA-B', 18, { alerta: true }))
    get.resolver([estado('CA-B', 17)])

    expect(await pedido).toEqual([estado('CA-B', 18, { alerta: true })])
  })

  it('cada caché lleva su cuenta: los eventos de una no se mezclan en otra', async () => {
    const una = nuevaCache()
    const otra = nuevaCache()
    aplicarEstadoEstacion(una, estado('CA-B', 15))

    expect(await pedirLista(otra, async () => [estado('CA-B', 14)])).toEqual([estado('CA-B', 14)])
  })
})

describe('pedirRegimen, una sola estación', () => {
  it('llega el evento S2 y después la respuesta del GET con S1: queda S2', async () => {
    const consultas = nuevaCache()
    void alConectarEnVivo(consultas)
    const get = pendiente<EstadoEstacion>()
    const pedido = pedirRegimen(consultas, 'CA-B', () => get.promesa)

    aplicarEstadoEstacion(consultas, estado('CA-A', 18)) // de otra estación: no cuenta
    aplicarEstadoEstacion(consultas, estado('CA-B', 15, { alerta: true }))
    get.resolver(estado('CA-B', 14))

    expect(await pedido).toEqual(estado('CA-B', 15, { alerta: true }))
  })

  it('sin evento de esa estación, vale la respuesta', async () => {
    const consultas = nuevaCache()
    void alConectarEnVivo(consultas)
    aplicarEstadoEstacion(consultas, estado('CA-A', 18))

    expect(await pedirRegimen(consultas, 'CA-B', async () => estado('CA-B', 14))).toEqual(estado('CA-B', 14))
  })

  it('si la API respondió 404 porque aún no tenía estado y ya llegó su primer evento, vale el evento', async () => {
    const consultas = nuevaCache()
    void alConectarEnVivo(consultas)
    const get = pendiente<EstadoEstacion>()
    const pedido = pedirRegimen(consultas, 'CA-B', () => get.promesa)

    aplicarEstadoEstacion(consultas, estado('CA-B', 15))
    get.rechazar(errorDeRespuesta(404, { error: 'la estación CA-B todavía no tiene un régimen registrado' }, true))

    expect(await pedido).toEqual(estado('CA-B', 15))
  })

  it('sin evento, el 404 es un error; y un 503 no se tapa con un evento', async () => {
    const consultas = nuevaCache()
    void alConectarEnVivo(consultas)
    const noEsta = errorDeRespuesta(404, { error: 'sin régimen' }, true)
    const caida = errorDeRespuesta(503, { error: 'Redis no disponible' }, true)

    await expect(pedirRegimen(consultas, 'CA-B', () => Promise.reject(noEsta))).rejects.toBe(noEsta)

    aplicarEstadoEstacion(consultas, estado('CA-B', 15))
    await expect(pedirRegimen(consultas, 'CA-B', () => Promise.reject(caida))).rejects.toBe(caida)
  })
})

describe('alConectarEnVivo', () => {
  it('marca para volver a pedir las consultas EN_VIVO y deja en paz a las demás', async () => {
    const consultas = nuevaCache()
    await consultas.fetchQuery({ queryKey: claves.regimenes, meta: EN_VIVO, queryFn: async () => [] })
    await consultas.fetchQuery({ queryKey: claves.regimen('CA-B'), meta: EN_VIVO, queryFn: async () => estado('CA-B', 14) })
    await consultas.fetchQuery({ queryKey: claves.estaciones, queryFn: async () => [] })
    const invalidada = (clave: readonly unknown[]) => consultas.getQueryCache().find({ queryKey: clave })!.state.isInvalidated

    await alConectarEnVivo(consultas)

    // Que una consulta con alguien mirándola se vuelve a pedir de verdad lo prueba App.test.tsx.
    expect(invalidada(claves.regimenes)).toBe(true)
    expect(invalidada(claves.regimen('CA-B'))).toBe(true)
    expect(invalidada(claves.estaciones)).toBe(false)
  })
})
