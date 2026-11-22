import { act, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useReproduccion } from './useReproduccion'

interface Props {
  clave: string | null
  recibidas: number
  paso: number
  completa?: boolean
}

function montar(inicial: Props) {
  return renderHook((p: Props) => useReproduccion(p.clave, p.recibidas, p.paso, p.completa), { initialProps: inicial })
}

// De a un milisegundo o de a diez: cada ronda mostrada programa la siguiente
// recién al redibujarse, así que el reloj no se puede adelantar de un salto.
function avanzar(ms: number) {
  const tramo = ms <= 400 ? 1 : 10
  for (let t = 0; t < ms; t += tramo) {
    act(() => {
      vi.advanceTimersByTime(tramo)
    })
  }
}

describe('la reproducción de las rondas', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  it('muestra las rondas de a una, cada `paso` milisegundos, aunque hayan llegado todas juntas', () => {
    const { result } = montar({ clave: 'e-1', recibidas: 44, paso: 120 })

    expect(result.current.mostradas).toBe(0)
    avanzar(119)
    expect(result.current.mostradas).toBe(0)
    avanzar(1)
    expect(result.current.mostradas).toBe(1)
    avanzar(120 * 10)
    expect(result.current.mostradas).toBe(11)
    avanzar(120 * 100)
    expect(result.current.mostradas).toBe(44)
  })

  it('nunca muestra una ronda que no llegó: espera a la siguiente', () => {
    const { result, rerender } = montar({ clave: 'e-1', recibidas: 2, paso: 120 })

    avanzar(1000)
    expect(result.current.mostradas).toBe(2)

    rerender({ clave: 'e-1', recibidas: 3, paso: 120 })
    expect(result.current.mostradas).toBe(2)
    avanzar(120)
    expect(result.current.mostradas).toBe(3)
  })

  it('sigue avanzando aunque lleguen rondas más seguido que el paso (una medición larga)', () => {
    const { result, rerender } = montar({ clave: 'e-1', recibidas: 1, paso: 120 })

    // Una ronda nueva cada 40 ms, durante 1.2 s.
    for (let recibidas = 2; recibidas <= 31; recibidas++) {
      avanzar(40)
      rerender({ clave: 'e-1', recibidas, paso: 120 })
    }

    expect(result.current.mostradas).toBe(10)
  })

  it('«ir al final» muestra todas las recibidas y «repetir» vuelve a empezar', () => {
    const { result } = montar({ clave: 'e-1', recibidas: 44, paso: 120 })

    act(() => result.current.irAlFinal())
    expect(result.current.mostradas).toBe(44)

    act(() => result.current.repetir())
    expect(result.current.mostradas).toBe(0)
    avanzar(240)
    expect(result.current.mostradas).toBe(2)
  })

  it('otro entrenamiento empieza de cero', () => {
    const { result, rerender } = montar({ clave: 'e-1', recibidas: 5, paso: 120 })
    avanzar(1000)
    expect(result.current.mostradas).toBe(5)

    rerender({ clave: 'e-2', recibidas: 3, paso: 120 })

    expect(result.current.mostradas).toBe(0)
    avanzar(120)
    expect(result.current.mostradas).toBe(1)
  })

  it('uno abierto desde el historial empieza completo', () => {
    const { result } = montar({ clave: 'e-1', recibidas: 44, paso: 120, completa: true })

    expect(result.current.mostradas).toBe(44)
  })

  it('con otra velocidad cambia el paso', () => {
    const { result, rerender } = montar({ clave: 'e-1', recibidas: 44, paso: 300 })
    avanzar(300)
    expect(result.current.mostradas).toBe(1)

    rerender({ clave: 'e-1', recibidas: 44, paso: 40 })
    avanzar(400)

    expect(result.current.mostradas).toBe(11)
  })
})
