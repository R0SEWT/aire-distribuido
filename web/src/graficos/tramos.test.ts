import { describe, expect, it } from 'vitest'
import { contarHuecos, HUECO_QUE_CORTA_H, partirPorHuecos } from './tramos'

const HORA_MS = 3_600_000
const p = (hora: number, valor: number | null = hora) => ({ t: hora * HORA_MS, valor })
const horas = (tramos: { t: number }[][]) => tramos.map((tramo) => tramo.map((x) => x.t / HORA_MS))

describe('partirPorHuecos', () => {
  it('una serie hora por hora es un solo tramo', () => {
    expect(horas(partirPorHuecos([p(0), p(1), p(2), p(3)]))).toEqual([[0, 1, 2, 3]])
    expect(partirPorHuecos([])).toEqual([])
  })

  it('corta donde faltan más horas que el máximo: dos tramos separados por días no quedan unidos', () => {
    const serie = [p(0), p(1), p(2), p(100), p(101)]

    expect(horas(partirPorHuecos(serie))).toEqual([
      [0, 1, 2],
      [100, 101],
    ])
  })

  it('unas pocas horas sin estado no cortan: de una ráfaga no llega cada estado', () => {
    // Estados recibidos cada 3 horas (a ritmo rápido) y uno justo en el límite.
    expect(horas(partirPorHuecos([p(0), p(3), p(6), p(6 + HUECO_QUE_CORTA_H)]))).toEqual([[0, 3, 6, 12]])
    expect(horas(partirPorHuecos([p(0), p(HUECO_QUE_CORTA_H + 1)]))).toEqual([[0], [HUECO_QUE_CORTA_H + 1]])
  })

  it('un punto sin valor corta la línea y no entra en ningún tramo', () => {
    expect(horas(partirPorHuecos([p(0, null), p(1, null), p(2), p(3), p(4, null), p(5)]))).toEqual([[2, 3], [5]])
    expect(partirPorHuecos([p(0, null)])).toEqual([])
  })

  it('el hueco se mide entre puntos seguidos, tengan valor o no', () => {
    // De la hora 0 a la 10 hay puntos cada 5 horas, uno de ellos sin media: no hay hueco de más de 6.
    expect(horas(partirPorHuecos([p(0), p(5, null), p(10)]))).toEqual([[0], [10]])
    expect(contarHuecos([p(0), p(5, null), p(10)])).toBe(0)
  })

  it('el máximo se puede elegir', () => {
    expect(horas(partirPorHuecos([p(0), p(2), p(3)], 1))).toEqual([[0], [2, 3]])
  })

  it('conserva los puntos tal como vinieron', () => {
    const serie = [{ t: 0, valor: 5, nota: 'a' }]
    expect(partirPorHuecos(serie)[0]![0]).toBe(serie[0])
  })
})

describe('contarHuecos', () => {
  it('cuenta los huecos entre puntos seguidos', () => {
    expect(contarHuecos([])).toBe(0)
    expect(contarHuecos([p(0), p(1), p(2)])).toBe(0)
    expect(contarHuecos([p(0), p(1), p(50), p(51), p(200)])).toBe(2)
  })
})
