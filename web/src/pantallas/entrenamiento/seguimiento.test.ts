import { describe, expect, it } from 'vitest'
import { reducir, SIN_SEGUIMIENTO, type Accion, type Detalle, type EventoRonda, type Seguimiento } from './seguimiento'

const PEDIDO = { nodos: 2, replicas: 1, semilla: 42 }

function ronda(id: string, iter: number): EventoRonda {
  return { id, iter, inercia: 100 - iter, ms_ronda: 0.5, ms_fusion: 0.01, ms_por_nodo: { 'nodo-a': 0.3, 'nodo-b': 0.4 } }
}

function fin(id: string) {
  return { id, iteraciones: 3, paro: 'tolerancia' as const, dif_secuencial: 1e-15, ms_total: 50, modelo_id: `entrenado-${id}` }
}

function aplicar(acciones: Accion[], desde: Seguimiento = SIN_SEGUIMIENTO): Seguimiento {
  return acciones.reduce(reducir, desde)
}

describe('el seguimiento de un entrenamiento', () => {
  it('al aceptarse el pedido, la corrida es propia y empieza sin rondas', () => {
    const s = aplicar([
      { tipo: 'lanzar', pedido: PEDIDO },
      { tipo: 'aceptado', id: 'e-1' },
    ])

    expect(s.lanzando).toBeNull()
    expect(s.corrida).toMatchObject({ id: 'e-1', origen: 'propia', pedido: PEDIDO, rondas: [], fin: null })
  })

  it('las rondas y el fin que llegan ANTES que el 202 no se pierden', () => {
    const s = aplicar([
      { tipo: 'lanzar', pedido: PEDIDO },
      { tipo: 'ronda', evento: ronda('e-1', 1) },
      { tipo: 'ronda', evento: ronda('e-1', 2) },
      { tipo: 'fin', evento: fin('e-1'), ahora: 1000 },
      { tipo: 'aceptado', id: 'e-1' },
    ])

    expect(s.corrida?.rondas.map((r) => r.iter)).toEqual([1, 2])
    expect(s.corrida?.fin).toMatchObject({ abortado: false, iteraciones: 3, modelo_id: 'entrenado-e-1' })
    expect(s.corrida?.finEn).toBe(1000)
    expect(s.buzon).toEqual({})
  })

  it('una ronda repetida no se cuenta dos veces y las desordenadas quedan en orden', () => {
    const s = aplicar([
      { tipo: 'lanzar', pedido: PEDIDO },
      { tipo: 'aceptado', id: 'e-1' },
      { tipo: 'ronda', evento: ronda('e-1', 2) },
      { tipo: 'ronda', evento: ronda('e-1', 1) },
      { tipo: 'ronda', evento: ronda('e-1', 2) },
    ])

    expect(s.corrida?.rondas.map((r) => r.iter)).toEqual([1, 2])
  })

  it('un fin con `error` deja la corrida abortada con las rondas que llegaron', () => {
    const s = aplicar([
      { tipo: 'lanzar', pedido: PEDIDO },
      { tipo: 'aceptado', id: 'e-1' },
      { tipo: 'ronda', evento: ronda('e-1', 1) },
      { tipo: 'fin', evento: { id: 'e-1', error: 'ronda 2 abortada: nodo "nodo-b": no respondió' }, ahora: 5 },
    ])

    expect(s.corrida?.fin).toEqual({ abortado: true, error: 'ronda 2 abortada: nodo "nodo-b": no respondió' })
    expect(s.corrida?.rondas).toHaveLength(1)
  })

  it('si el pedido se rechaza, la corrida anterior sigue a la vista', () => {
    const anterior = aplicar([
      { tipo: 'lanzar', pedido: PEDIDO },
      { tipo: 'aceptado', id: 'e-1' },
      { tipo: 'fin', evento: fin('e-1'), ahora: 1 },
    ])

    const s = aplicar([{ tipo: 'lanzar', pedido: PEDIDO }, { tipo: 'rechazado' }], anterior)

    expect(s.lanzando).toBeNull()
    expect(s.corrida?.id).toBe('e-1')
  })

  it('un 409 con las rondas del entrenamiento que lo causó pasa a seguir ese', () => {
    const s = aplicar([
      { tipo: 'lanzar', pedido: PEDIDO },
      { tipo: 'ronda', evento: ronda('e-de-otro', 7) },
      { tipo: 'rechazado' },
    ])

    expect(s.corrida).toMatchObject({ id: 'e-de-otro', origen: 'ajena', pedido: null })
    expect(s.corrida?.rondas.map((r) => r.iter)).toEqual([7])
  })

  it('sigue un entrenamiento lanzado desde otra sesión si no hay otro a medias', () => {
    const s = aplicar([{ tipo: 'ronda', evento: ronda('e-de-otro', 1) }])

    expect(s.corrida).toMatchObject({ id: 'e-de-otro', origen: 'ajena', fin: null })
  })

  it('no cambia de entrenamiento mientras el que sigue no terminó', () => {
    const s = aplicar([
      { tipo: 'lanzar', pedido: PEDIDO },
      { tipo: 'aceptado', id: 'e-1' },
      { tipo: 'ronda', evento: ronda('e-de-otro', 1) },
      { tipo: 'fin', evento: fin('e-de-otro'), ahora: 1 },
    ])

    expect(s.corrida).toMatchObject({ id: 'e-1', fin: null, rondas: [] })
  })

  describe('con el detalle de la API', () => {
    const detalle: Detalle = {
      id: 'e-1',
      estado: 'terminado',
      nodos: ['nodo-a', 'nodo-b'],
      replicas: 1,
      iteraciones: 3,
      paro: 'tolerancia',
      dif_secuencial: 1e-15,
      ms_total: 50,
      rondas: [ronda('e-1', 1), ronda('e-1', 2), ronda('e-1', 3)],
    }

    it('abrir uno del historial lo muestra entero, con su fin', () => {
      const s = aplicar([{ tipo: 'ver', detalle }])

      expect(s.corrida).toMatchObject({ id: 'e-1', origen: 'historial', finEn: null })
      expect(s.corrida?.rondas).toHaveLength(3)
      expect(s.corrida?.fin).toMatchObject({ abortado: false, iteraciones: 3 })
    })

    it('completa las rondas que el WebSocket no trajo y, si falta, el fin', () => {
      const s = aplicar([
        { tipo: 'lanzar', pedido: PEDIDO },
        { tipo: 'aceptado', id: 'e-1' },
        { tipo: 'ronda', evento: ronda('e-1', 1) },
        { tipo: 'detalle', detalle, ahora: 77 },
      ])

      expect(s.corrida?.rondas.map((r) => r.iter)).toEqual([1, 2, 3])
      expect(s.corrida?.fin).toMatchObject({ abortado: false })
      expect(s.corrida?.finEn).toBe(77)
    })

    it('no pisa el fin que ya llegó por el WebSocket ni toca otra corrida', () => {
      const conFin = aplicar([
        { tipo: 'lanzar', pedido: PEDIDO },
        { tipo: 'aceptado', id: 'e-1' },
        { tipo: 'fin', evento: fin('e-1'), ahora: 10 },
      ])

      expect(reducir(conFin, { tipo: 'detalle', detalle: { ...detalle, rondas: [] }, ahora: 99 })).toBe(conFin)
      expect(reducir(conFin, { tipo: 'detalle', detalle: { ...detalle, id: 'e-2' }, ahora: 99 })).toBe(conFin)
    })

    it('un detalle abortado trae el error', () => {
      const s = aplicar([{ tipo: 'ver', detalle: { id: 'e-9', estado: 'abortado', nodos: ['nodo-a'], replicas: 1, rondas: [], error: 'se cayó' } }])

      expect(s.corrida?.fin).toEqual({ abortado: true, error: 'se cayó' })
    })
  })

  describe('cuando la API ya no conoce el entrenamiento seguido', () => {
    it('se suelta y queda dicho cuál era', () => {
      const s = aplicar([
        { tipo: 'lanzar', pedido: PEDIDO },
        { tipo: 'aceptado', id: 'e-1' },
        { tipo: 'ronda', evento: ronda('e-1', 1) },
        { tipo: 'perdido', id: 'e-1' },
      ])

      expect(s.corrida).toBeNull()
      expect(s.perdido).toBe('e-1')
    })

    it('si la respuesta es de otro entrenamiento (llegó tarde), no toca nada', () => {
      const antes = aplicar([
        { tipo: 'lanzar', pedido: PEDIDO },
        { tipo: 'aceptado', id: 'e-2' },
      ])

      expect(reducir(antes, { tipo: 'perdido', id: 'e-1' })).toBe(antes)
    })

    it('el aviso se retira al lanzar otro, al abrir uno del historial o al seguir uno que llega', () => {
      const perdido = aplicar([
        { tipo: 'lanzar', pedido: PEDIDO },
        { tipo: 'aceptado', id: 'e-1' },
        { tipo: 'perdido', id: 'e-1' },
      ])

      expect(reducir(perdido, { tipo: 'lanzar', pedido: PEDIDO }).perdido).toBeNull()
      expect(reducir(perdido, { tipo: 'ver', detalle: { id: 'e-9', estado: 'terminado', nodos: [], replicas: 1, rondas: [] } }).perdido).toBeNull()
      expect(reducir(perdido, { tipo: 'ronda', evento: ronda('e-3', 1) }).perdido).toBeNull()
    })
  })
})
