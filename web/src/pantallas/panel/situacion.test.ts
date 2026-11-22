import { describe, expect, it } from 'vitest'
import { ESTACIONES, estadoDe, EN_ALERTA } from './dobles'
import { detalleSinBanda, filaDe, filasDe, frasePanel, resumir, tonoDe } from './situacion'

const SIN_PM = new Set(['CA-01'])
const [ca01, ayas] = ESTACIONES as [(typeof ESTACIONES)[number], (typeof ESTACIONES)[number]]

describe('los tres estados de una estación', () => {
  it('con media de 24 horas tiene estado, con su banda y su alerta', () => {
    const fila = filaDe(ayas, estadoDe('CA-AYAS-01', EN_ALERTA), true)

    expect(fila).toMatchObject({ situacion: 'con_estado', banda: 'mala', alerta: true })
  })

  it('con menos de 18 lecturas no tiene datos suficientes, y se dice cuántas lleva', () => {
    const fila = filaDe(ayas, estadoDe('CA-AYAS-01', { pm2_5: null, pm10: null, inca: null, lecturas: 5 }), true)

    expect(fila).toMatchObject({ situacion: 'sin_datos', banda: 'sin_datos', alerta: false })
    expect(detalleSinBanda(fila)).toBe('5 de 18 lecturas en la ventana')
  })

  it('si mide PM por hora y no recibió nada, tampoco tiene datos suficientes', () => {
    const fila = filaDe(ayas, undefined, true)

    expect(fila).toMatchObject({ situacion: 'sin_datos', estado: null })
    expect(detalleSinBanda(fila)).toBe('todavía no recibió lecturas')
  })

  it('si no mide PM por hora, no mide: nunca va a tener estado', () => {
    const fila = filaDe(ca01, undefined, false)

    expect(fila).toMatchObject({ situacion: 'no_mide', banda: 'no_mide', alerta: false })
    expect(detalleSinBanda(fila)).toBe('no mide PM2.5 y PM10 por hora')
  })

  it('si la API tiene un estado de una estación que la ficha da por apagada, manda la API', () => {
    expect(filaDe(ca01, estadoDe('CA-01'), false)).toMatchObject({ situacion: 'con_estado', banda: 'buena' })
  })
})

describe('el orden por gravedad', () => {
  it('la alerta va primero, después de la banda peor a la mejor, después sin datos y al final las que no miden', () => {
    const filas = filasDe(
      ESTACIONES,
      [
        estadoDe('CA-AYAS-01', { inca: 'moderada' }),
        estadoDe('CA-CG-02', { inca: 'umbral_de_cuidado' }),
        // En alerta con una banda mejor que la de CA-CG-02: la alerta manda.
        estadoDe('CA-HU-04', EN_ALERTA),
        estadoDe('CA-MAC-01', { pm2_5: null, pm10: null, inca: null, lecturas: 3 }),
      ],
      SIN_PM,
    )

    expect(filas.map((f) => f.id)).toEqual(['CA-HU-04', 'CA-CG-02', 'CA-AYAS-01', 'CA-MAC-01', 'CA-HUAR-01', 'CA-01'])
  })

  it('a igual gravedad, por código', () => {
    const filas = filasDe(ESTACIONES, [estadoDe('CA-MAC-01'), estadoDe('CA-AYAS-01'), estadoDe('CA-HU-04')], SIN_PM)

    expect(filas.slice(0, 3).map((f) => f.id)).toEqual(['CA-AYAS-01', 'CA-HU-04', 'CA-MAC-01'])
  })
})

describe('las cifras y la frase del panel', () => {
  const resumenDe = (...estados: ReturnType<typeof estadoDe>[]) => resumir(filasDe(ESTACIONES, estados, SIN_PM))

  it('cuenta las que están en alerta, las que tienen estado y las que miden', () => {
    const r = resumenDe(
      estadoDe('CA-HU-04', EN_ALERTA),
      estadoDe('CA-AYAS-01'),
      estadoDe('CA-MAC-01', { pm2_5: null, pm10: null, inca: null, lecturas: 3 }),
    )

    expect(r).toMatchObject({ enAlerta: 1, conEstado: 2, sinDatos: 3, noMiden: 1, miden: 5 })
  })

  it('el último estado recibido es el que la API guardó más tarde, no el de fecha más nueva', () => {
    const viejoPeroRecienLlegado = { ...estadoDe('CA-AYAS-01', { hora: 3 }), ts: '2026-11-21T18:00:00-05:00' }
    const r = resumenDe(estadoDe('CA-HU-04', { hora: 20 }), viejoPeroRecienLlegado)

    expect(r.ultimo?.id).toBe('CA-AYAS-01')
  })

  it('sin ningún estado no hay último', () => {
    expect(resumenDe().ultimo).toBeNull()
  })

  it('la frase sigue una plantilla fija y nunca dice «ahora»', () => {
    const dos = frasePanel(resumenDe(estadoDe('CA-HU-04', EN_ALERTA), estadoDe('CA-AYAS-01', EN_ALERTA)))
    const una = frasePanel(resumenDe(estadoDe('CA-HU-04', EN_ALERTA), estadoDe('CA-AYAS-01')))
    const ninguna = frasePanel(resumenDe(estadoDe('CA-AYAS-01')))
    const sinMedia = frasePanel(resumenDe(estadoDe('CA-AYAS-01', { pm2_5: null, pm10: null, inca: null, lecturas: 2 })))

    expect(dos).toEqual({
      tono: 'alerta',
      texto: 'En el último estado recibido hay 2 estaciones en alerta: su media de 24 horas supera el ECA.',
    })
    expect(una.texto).toBe('En el último estado recibido hay 1 estación en alerta: su media de 24 horas supera el ECA.')
    expect(ninguna).toEqual({
      tono: 'buena',
      texto: 'En el último estado recibido ninguna estación está en alerta: ninguna media de 24 horas supera el ECA.',
    })
    expect(sinMedia.texto).toMatch(/hacen falta 18 lecturas/)
    for (const f of [dos, una, ninguna, sinMedia]) expect(f.texto).not.toMatch(/ahora/i)
  })
})

describe('el tono', () => {
  it('el tono es el de la alerta o el de la banda', () => {
    expect(tonoDe({ alerta: true, banda: 'moderada' })).toBe('alerta')
    expect(tonoDe({ alerta: false, banda: 'umbral_de_cuidado' })).toBe('alerta')
    expect(tonoDe({ alerta: false, banda: 'moderada' })).toBe('moderada')
    expect(tonoDe({ alerta: false, banda: 'sin_datos' })).toBe('neutro')
  })
})
