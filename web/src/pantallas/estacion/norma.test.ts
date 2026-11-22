import { describe, expect, it } from 'vitest'
import { estadoDe } from '../panel/dobles'
import { contaminanteInicial, fechaDia, fraseEstacion } from './norma'

describe('la frase de la estación', () => {
  it('dice la media y contra qué se compara, con un decimal', () => {
    expect(fraseEstacion(estadoDe('CA-AYAS-01', { pm2_5: 58.3, pm10: 121.44, alerta: true, inca: 'mala' }))).toBe(
      'La media de las últimas 24 horas de PM2.5 es 58.3 µg/m³ y la de PM10 es 121.4 µg/m³: las dos superan el ECA (50 y 100).',
    )
    expect(fraseEstacion(estadoDe('CA-AYAS-01', { pm2_5: 18.2, pm10: 121.4, alerta: true, inca: 'moderada' }))).toBe(
      'La media de las últimas 24 horas de PM10 es 121.4 µg/m³: supera el ECA (100). La de PM2.5 es 18.2 µg/m³: no lo supera (50).',
    )
    // 50.3 redondeado a 50 «superaría el ECA (50)»: por eso va el decimal.
    expect(fraseEstacion(estadoDe('CA-HU-09', { pm2_5: 50.33, pm10: 73.52, alerta: true, inca: 'mala' }))).toBe(
      'La media de las últimas 24 horas de PM2.5 es 50.3 µg/m³: supera el ECA (50). La de PM10 es 73.5 µg/m³: no lo supera (100).',
    )
    expect(fraseEstacion(estadoDe('CA-AYAS-01', { pm2_5: 9, pm10: 17.6 }))).toBe(
      'La media de las últimas 24 horas de PM2.5 es 9.0 µg/m³ y la de PM10 es 17.6 µg/m³: ninguna supera el ECA (50 y 100).',
    )
  })

  it('sin media dice cuántas lecturas hay y cuántas hacen falta', () => {
    expect(fraseEstacion(estadoDe('CA-MAC-01', { pm2_5: null, pm10: null, inca: null, lecturas: 3 }))).toBe(
      'Todavía no hay media de 24 horas: la ventana tiene 3 lecturas y hacen falta 18.',
    )
    expect(fraseEstacion(estadoDe('CA-MAC-01', { pm2_5: null, pm10: null, inca: null, lecturas: 1 }))).toMatch(/tiene 1 lectura y/)
  })

  it('si la comparación de la UI y la alerta de la API no coinciden, manda la API', () => {
    expect(fraseEstacion(estadoDe('CA-AYAS-01', { pm2_5: 9, pm10: 17.6, alerta: true }))).toBe(
      'La estación está en alerta: su media de las últimas 24 horas supera el ECA.',
    )
    expect(fraseEstacion(estadoDe('CA-AYAS-01', { pm2_5: 9, pm10: 120, alerta: false }))).toBe(
      'La estación no está en alerta: su media de las últimas 24 horas no supera el ECA.',
    )
  })
})

describe('el contaminante con que abre la curva', () => {
  it('es el que supera el ECA; si ninguno, PM10', () => {
    expect(contaminanteInicial(estadoDe('CA-HU-09', { pm2_5: 50.3, pm10: 73.5 }))).toBe('pm2_5')
    expect(contaminanteInicial(estadoDe('CA-HU-04', { pm2_5: 88.8, pm10: 120.7 }))).toBe('pm10')
    expect(contaminanteInicial(estadoDe('CA-AYAS-01'))).toBe('pm10')
    expect(contaminanteInicial(null)).toBe('pm10')
  })
})

describe('fechaDia', () => {
  it('escribe un día sin hora sin correrlo de día por la zona horaria', () => {
    expect(fechaDia('2021-12-10')).toMatch(/^10 dic\.? 2021$/)
    expect(fechaDia('2022-01-01')).toMatch(/^1 ene\.? 2022$/)
    expect(fechaDia('no es una fecha')).toBe('—')
  })
})
