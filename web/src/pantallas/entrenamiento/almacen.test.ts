// El entrenamiento a la vista fuera de React: qué se anota con la pantalla
// abierta y con la pantalla cerrada, y qué queda al terminar la sesión.

import { describe, expect, it, vi } from 'vitest'
import { AlmacenDeEntrenamiento } from './almacen'
import type { Detalle, EventoFinal, EventoRonda } from './seguimiento'

const ronda = (id: string, iter: number): EventoRonda => ({ id, iter, inercia: 70_000 - iter, ms_ronda: 0.4, ms_fusion: 0.004, ms_por_nodo: { 'nodo-a': 0.3 } })
const fin = (id: string): EventoFinal => ({ id, iteraciones: 2, paro: 'tolerancia', dif_secuencial: 0, ms_total: 50 })
const PEDIDO = { nodos: 1, replicas: 1, semilla: 42 }
const detalle = (id: string): Detalle => ({ id, estado: 'terminado', nodos: ['nodo-a'], replicas: 1, iteraciones: 1, ms_total: 50, rondas: [ronda(id, 1)] }) as Detalle

/** Un almacén con la pantalla «abierta»: alguien lo está mirando. */
function conPantalla() {
  const almacen = new AlmacenDeEntrenamiento()
  const cerrar = almacen.suscribir(() => {})
  return { almacen, cerrar }
}

describe('el entrenamiento a la vista', () => {
  it('con la pantalla abierta sigue lo que lanza y lo que llega', () => {
    const { almacen } = conPantalla()
    almacen.despachar({ tipo: 'lanzar', pedido: PEDIDO })
    almacen.despachar({ tipo: 'aceptado', id: 'e-1' })
    almacen.recibir({ tipo: 'ronda', evento: ronda('e-1', 1) })

    expect(almacen.leer().seguimiento.corrida).toMatchObject({ id: 'e-1', origen: 'propia', rondas: [{ iter: 1 }], fin: null })
  })

  it('con la pantalla cerrada sigue anotando las rondas y el fin del que quedó a la vista', () => {
    const { almacen, cerrar } = conPantalla()
    almacen.despachar({ tipo: 'lanzar', pedido: PEDIDO })
    almacen.despachar({ tipo: 'aceptado', id: 'e-1' })
    almacen.recibir({ tipo: 'ronda', evento: ronda('e-1', 1) })

    cerrar()
    almacen.recibir({ tipo: 'ronda', evento: ronda('e-1', 2) })
    almacen.recibir({ tipo: 'fin', evento: fin('e-1'), ahora: 1234 })

    expect(almacen.leer().seguimiento.corrida).toMatchObject({ id: 'e-1', rondas: [{ iter: 1 }, { iter: 2 }], fin: { abortado: false }, finEn: 1234 })
  })

  it('con la pantalla cerrada, lo que llega mientras el pedido viaja no se pierde', () => {
    const { almacen, cerrar } = conPantalla()
    almacen.despachar({ tipo: 'lanzar', pedido: PEDIDO })

    cerrar()
    almacen.recibir({ tipo: 'ronda', evento: ronda('e-1', 1) })
    almacen.recibir({ tipo: 'fin', evento: fin('e-1'), ahora: 1 })
    almacen.despachar({ tipo: 'aceptado', id: 'e-1' })

    expect(almacen.leer().seguimiento.corrida).toMatchObject({ id: 'e-1', origen: 'propia', rondas: [{ iter: 1 }], fin: { abortado: false } })
  })

  it('con la pantalla cerrada, uno de otra sesión no ocupa la vista; con la pantalla abierta, sí', () => {
    const almacen = new AlmacenDeEntrenamiento()
    almacen.recibir({ tipo: 'ronda', evento: ronda('e-ajeno', 1) })
    expect(almacen.leer().seguimiento.corrida).toBeNull()

    almacen.suscribir(() => {})
    almacen.recibir({ tipo: 'ronda', evento: ronda('e-ajeno', 2) })
    expect(almacen.leer().seguimiento.corrida).toMatchObject({ id: 'e-ajeno', origen: 'ajena' })
  })

  it('avisa a quien mira solo cuando algo cambia', () => {
    const almacen = new AlmacenDeEntrenamiento()
    const oyente = vi.fn()
    almacen.suscribir(oyente)

    almacen.recibir({ tipo: 'ronda', evento: ronda('e-1', 1) })
    almacen.recibir({ tipo: 'ronda', evento: ronda('e-1', 1) }) // repetida: no cambia nada
    almacen.cambiarVelocidad('normal') // la que ya estaba
    almacen.cambiarVelocidad('lenta')

    expect(oyente).toHaveBeenCalledTimes(2)
    expect(almacen.leer().velocidad).toBe('lenta')
  })

  it('recuerda por dónde iba la reproducción del que está a la vista, y la olvida al pasar a otro', () => {
    const { almacen } = conPantalla()
    almacen.despachar({ tipo: 'ver', detalle: detalle('e-1') })
    almacen.memoria.guardar('e-1', 12)
    expect(almacen.memoria.leer('e-1')).toBe(12)
    expect(almacen.memoria.leer('e-2')).toBeUndefined()

    almacen.despachar({ tipo: 'ver', detalle: detalle('e-2') })
    expect(almacen.memoria.leer('e-1')).toBeUndefined()
  })

  it('al reiniciar (la sesión terminó) no queda nada: ni el entrenamiento, ni la velocidad, ni la reproducción', () => {
    const { almacen } = conPantalla()
    almacen.despachar({ tipo: 'ver', detalle: detalle('e-1') })
    almacen.cambiarVelocidad('rapida')
    almacen.memoria.guardar('e-1', 1)

    almacen.reiniciar()

    expect(almacen.leer()).toEqual({ seguimiento: { lanzando: null, corrida: null, buzon: {}, perdido: null }, velocidad: 'normal' })
    expect(almacen.memoria.leer('e-1')).toBeUndefined()
  })
})
