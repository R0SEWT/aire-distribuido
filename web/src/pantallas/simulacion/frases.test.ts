// Las frases de la marcha de una simulación: que digan lo que pasó y no otra cosa.

import { describe, expect, it } from 'vitest'
import type { ReglasDeSeries } from '../../api/datos'
import type { LecturaVista } from '../../api/vistas'
import { causaDeLaAlerta, fraseDelDia, fraseDelEscenario } from './frases'
import type { Corrida, Seguimiento } from './motor'
import type { Prediccion, Tramo } from './series'

const REGLAS = { ventana_horas: 24, minimo_lecturas: 18, eca_pm2_5: 50, eca_pm10: 100 } as ReglasDeSeries

function seguimiento(estacion: string, o: { pm2_5?: number | null; pm10?: number | null; alerta?: boolean; primeraAlerta?: string | null } = {}): Seguimiento {
  const pm2_5 = o.pm2_5 === undefined ? 10 : o.pm2_5
  const pm10 = o.pm10 === undefined ? 20 : o.pm10
  const hayMedia = pm2_5 !== null && pm10 !== null
  return {
    estacion,
    puntos: [],
    ultima: { estacion, media_24h: { pm2_5, pm10, lecturas: hayMedia ? 24 : 5 }, alerta: o.alerta ?? false, inca: hayMedia ? 'buena' : null } as Prediccion,
    primeraMedia: null,
    primeraAlerta: o.primeraAlerta ?? null,
  }
}

const corrida = (...seguimientos: Seguimiento[]) => ({ porEstacion: Object.fromEntries(seguimientos.map((s) => [s.estacion, s])) }) as unknown as Corrida
const tramoDe = (n: number) => ({ estaciones: Array.from({ length: n }, (_, i) => `CA-${i}`) }) as unknown as Tramo

describe('la frase del escenario', () => {
  it('sin ninguna en alerta no dice que «su media supera el ECA»', () => {
    const frase = fraseDelEscenario(tramoDe(3), corrida(seguimiento('CA-A'), seguimiento('CA-B')), REGLAS)

    expect(frase).toEqual({
      tono: 'buena',
      texto: '2 de 3 estaciones tienen media de 24 horas y ninguna está en alerta: ninguna media supera el ECA.',
    })
    expect(frase.texto).not.toContain('su media supera el ECA')
  })

  it('con una sola con media, en singular: es lo que se ve a mitad de la hora en que aparecen las medias', () => {
    const frase = fraseDelEscenario(tramoDe(14), corrida(seguimiento('CA-A'), seguimiento('CA-B', { pm2_5: null, pm10: null })), REGLAS)

    expect(frase.texto).toBe('1 de 14 estaciones tiene media de 24 horas y ninguna está en alerta: ninguna media supera el ECA.')
  })

  it('con alguna en alerta dice cuáles y por qué', () => {
    const una = fraseDelEscenario(tramoDe(3), corrida(seguimiento('CA-A', { pm10: 120, alerta: true }), seguimiento('CA-B')), REGLAS)
    expect(una).toEqual({ tono: 'alerta', texto: '2 de 3 estaciones tienen media de 24 horas y está en alerta CA-A: su media supera el ECA.' })

    const dos = fraseDelEscenario(
      tramoDe(2),
      corrida(seguimiento('CA-A', { pm10: 120, alerta: true }), seguimiento('CA-B', { pm2_5: 60, alerta: true })),
      REGLAS,
    )
    expect(dos.texto).toBe('2 de 2 estaciones tienen media de 24 horas y están en alerta CA-A y CA-B: su media supera el ECA.')
  })

  it('mientras ninguna tiene media, dice por qué todavía no la hay', () => {
    const frase = fraseDelEscenario(tramoDe(2), corrida(seguimiento('CA-A', { pm2_5: null, pm10: null })), REGLAS)

    expect(frase.tono).toBe('acento')
    expect(frase.texto).toBe('Todavía ninguna estación tiene media de 24 horas: la API la da recién cuando la ventana junta 18 lecturas.')
  })
})

describe('la frase de un día', () => {
  it('sin alerta dice que no supera el ECA; con alerta, cuál supera y desde cuándo', () => {
    expect(fraseDelDia(seguimiento('CA-A'), REGLAS).texto).toBe(
      'La media de las últimas 24 horas es 10.0 µg/m³ de PM2.5 y 20.0 µg/m³ de PM10: no supera el ECA (50 y 100).',
    )
    const conAlerta = fraseDelDia(seguimiento('CA-A', { pm10: 131.9, alerta: true, primeraAlerta: '2021-12-21T07:00:00-05:00' }), REGLAS)
    expect(conAlerta.tono).toBe('alerta')
    expect(conAlerta.texto).toMatch(/supera el ECA, el de PM10 \(100\)\. Hay alerta desde el 21 dic\.? 2021,? 07:00\.$/)
  })
})

describe('por cuál contaminante se encendió la alerta', () => {
  const punto = (hora: number, media_pm2_5: number | null, media_pm10: number | null) =>
    ({ fecha_hora: `2021-12-21T0${hora}:00:00-05:00`, media_pm2_5, media_pm10 }) as LecturaVista
  const PUNTOS = [punto(5, 20, 90), punto(6, 22, 99), punto(7, 23, 101), punto(8, 60, 120)]

  it('son los que superaban su ECA en la lectura en que se encendió, no los de después', () => {
    expect(causaDeLaAlerta(PUNTOS, '2021-12-21T07:00:00-05:00', REGLAS)).toEqual(['PM10'])
    expect(causaDeLaAlerta(PUNTOS, '2021-12-21T08:00:00-05:00', REGLAS)).toEqual(['PM2.5', 'PM10'])
    expect(causaDeLaAlerta([punto(7, 51, 40)], '2021-12-21T07:00:00-05:00', REGLAS)).toEqual(['PM2.5'])
  })

  it('la misma hora escrita en otra zona es la misma lectura', () => {
    expect(causaDeLaAlerta(PUNTOS, '2021-12-21T12:00:00Z', REGLAS)).toEqual(['PM10'])
  })

  it('sin alerta, o si esa lectura no se vio, no afirma nada', () => {
    expect(causaDeLaAlerta(PUNTOS, null, REGLAS)).toEqual([])
    expect(causaDeLaAlerta(PUNTOS, '2021-12-22T07:00:00-05:00', REGLAS)).toEqual([])
    expect(causaDeLaAlerta([punto(7, null, null)], '2021-12-21T07:00:00-05:00', REGLAS)).toEqual([])
  })
})
