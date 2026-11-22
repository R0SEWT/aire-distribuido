import { describe, expect, it } from 'vitest'
import { estadoDe } from '../panel/dobles'
import { contaminanteInicial, fraseDelRegimen, fraseEstacion } from './norma'

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

describe('una media apenas sobre el umbral', () => {
  it('no se escribe «50.0 µg/m³: supera el ECA (50)»: lleva los decimales que hagan falta', () => {
    const frase = fraseEstacion(estadoDe('CA-HU-09', { pm2_5: 50.03, pm10: 73.5, alerta: true, inca: 'mala' }))

    expect(frase).toBe(
      'La media de las últimas 24 horas de PM2.5 es 50.03 µg/m³: supera el ECA (50). La de PM10 es 73.5 µg/m³: no lo supera (100).',
    )
  })

  it('lo mismo con PM10 y con las dos', () => {
    expect(fraseEstacion(estadoDe('CA-HU-04', { pm2_5: 20, pm10: 100.04, alerta: true, inca: 'moderada' }))).toContain(
      'PM10 es 100.04 µg/m³: supera el ECA (100)',
    )
    expect(fraseEstacion(estadoDe('CA-HU-04', { pm2_5: 50.03, pm10: 100.04, alerta: true, inca: 'mala' }))).toContain(
      'PM2.5 es 50.03 µg/m³ y la de PM10 es 100.04 µg/m³: las dos superan el ECA',
    )
  })

  it('y apenas debajo: no dice «50.0: no supera el ECA (50)»', () => {
    expect(fraseEstacion(estadoDe('CA-HU-04', { pm2_5: 49.96, pm10: 99.97 }))).toContain('PM2.5 es 49.96 µg/m³ y la de PM10 es 99.97 µg/m³: ninguna supera')
  })
})

describe('fraseDelRegimen', () => {
  it('dice que el «excede el ECA» es del grupo de lecturas y no de la estación', () => {
    expect(fraseDelRegimen(false)).toBe(
      'La media de ese grupo de lecturas no excede el ECA. Describe al grupo, no a esta estación: su alerta sale de su propia media de 24 h.',
    )
    expect(fraseDelRegimen(true)).toMatch(/^La media de ese grupo de lecturas excede el ECA\. Describe al grupo, no a esta estación/)
    // «su media», a secas, se leía como la de la estación.
    for (const excede of [true, false]) expect(fraseDelRegimen(excede)).not.toMatch(/^su media|; su media/)
  })
})
