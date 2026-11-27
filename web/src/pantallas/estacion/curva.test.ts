// Qué dibuja la curva de la Estación y con qué palabras lo dice, en sus dos
// casos: solo estados recibidos, o también lecturas horarias enviadas.

import { describe, expect, it } from 'vitest'
import type { PuntoSesion } from '../../api/historia'
import type { LecturaVista } from '../../api/vistas'
import { armarCurva, textosDeCurva } from './curva'

const iso = (hora: number) => `2021-12-15T${String(hora).padStart(2, '0')}:00:00-05:00`

function estado(hora: number, pm10: number | null, pm2_5: number | null = pm10 === null ? null : pm10 / 2): PuntoSesion {
  return { fecha_hora: iso(hora), t: Date.parse(iso(hora)), pm2_5, pm10, lecturas: pm10 === null ? 5 : 24, alerta: (pm10 ?? 0) > 100, inca: null }
}

function vista(hora: number, pm10: number, media: number | null): LecturaVista {
  return {
    fecha_hora: iso(hora),
    pm2_5: pm10 / 2,
    pm10,
    media_pm2_5: media === null ? null : media / 2,
    media_pm10: media,
    lecturas: media === null ? 5 : 24,
    alerta: (media ?? 0) > 100,
    inca: null,
    regimen: 0,
    excede_eca: false,
  }
}

describe('armarCurva', () => {
  it('sin nada recibido ni enviado no hay curva', () => {
    expect(armarCurva([], [], 'pm10')).toEqual({
      medias: [],
      lecturas: [],
      estados: 0,
      estadosAjenos: 0,
      sinMedia: 0,
      huecos: 0,
      ultimoEstado: null,
      ultimaLectura: null,
      vacia: true,
    })
  })

  it('con solo estados recibidos, la media de cada uno y ninguna lectura', () => {
    const curva = armarCurva([estado(10, 90), estado(11, 120.7), estado(12, null)], [], 'pm10')

    expect(curva.medias.map((m) => m.valor)).toEqual([90, 120.7, null])
    expect(curva.lecturas).toEqual([])
    expect(curva).toMatchObject({ estados: 3, estadosAjenos: 3, sinMedia: 1, ultimoEstado: Date.parse(iso(12)), ultimaLectura: null, vacia: false })
  })

  it('con lecturas enviadas, cada una con su valor y con la media que respondió la API', () => {
    const curva = armarCurva([], [vista(10, 150, null), vista(11, 180, 95), vista(12, 60, 101)], 'pm10')

    expect(curva.lecturas.map((l) => l.valor)).toEqual([150, 180, 60])
    expect(curva.medias.map((m) => m.valor)).toEqual([null, 95, 101])
    expect(curva).toMatchObject({ estados: 0, estadosAjenos: 0, sinMedia: 1, ultimoEstado: null, ultimaLectura: Date.parse(iso(12)), vacia: false })
  })

  it('elige el contaminante en las lecturas y en las medias', () => {
    const curva = armarCurva([estado(13, 80, 33)], [vista(12, 60, 100)], 'pm2_5')

    expect(curva.lecturas.map((l) => l.valor)).toEqual([30])
    expect(curva.medias.map((m) => m.valor)).toEqual([50, 33])
  })

  it('una hora que está en las dos fuentes es un solo punto de la media, y manda el estado recibido', () => {
    // La lectura de las 11 se envió y su estado llegó por el WebSocket; el de las 13 es de otra sesión.
    const curva = armarCurva([estado(11, 96), estado(13, 99)], [vista(10, 150, 90), vista(11, 180, 95)], 'pm10')

    expect(curva.medias.map((m) => [new Date(m.t).getUTCHours() - 5, m.valor])).toEqual([
      [10, 90],
      [11, 96],
      [13, 99],
    ])
    expect(curva.lecturas).toHaveLength(2)
    expect(curva).toMatchObject({ estados: 2, estadosAjenos: 1 })
  })

  it('ordena por hora aunque las lecturas se hayan anotado en otro orden', () => {
    const curva = armarCurva([], [vista(12, 3, 3), vista(10, 1, 1), vista(11, 2, 2)], 'pm10')

    expect(curva.lecturas.map((l) => l.valor)).toEqual([1, 2, 3])
    expect(curva.medias.map((m) => m.valor)).toEqual([1, 2, 3])
  })
})

describe('los huecos', () => {
  const otroDia = (hora: number, pm10: number): PuntoSesion => {
    const fecha_hora = `2021-12-20T${String(hora).padStart(2, '0')}:00:00-05:00`
    return { fecha_hora, t: Date.parse(fecha_hora), pm2_5: pm10 / 2, pm10, lecturas: 24, alerta: false, inca: null }
  }

  it('dos tramos de la misma estación separados por días cuentan un hueco', () => {
    const curva = armarCurva([estado(10, 90), estado(11, 95), otroDia(8, 40), otroDia(9, 42)], [], 'pm10')

    expect(curva.huecos).toBe(1)
    expect(curva.medias).toHaveLength(4)
  })

  it('unas horas sin estado no son un hueco', () => {
    expect(armarCurva([estado(1, 90), estado(4, 95), estado(10, 99)], [], 'pm10').huecos).toBe(0)
    expect(armarCurva([estado(1, 90), estado(8, 95)], [], 'pm10').huecos).toBe(1)
  })

  it('el hueco puede estar entre lo enviado y lo recibido', () => {
    const curva = armarCurva([otroDia(8, 40)], [vista(10, 150, 90), vista(11, 180, 95)], 'pm10')

    expect(curva.huecos).toBe(1)
  })

  it('el pie dice que la línea se corta y por qué, en los dos casos', () => {
    const soloEstados = textosDeCurva('CA-HU-04', armarCurva([estado(10, 90), otroDia(8, 40)], [], 'pm10'), 'pm10')
    expect(soloEstados.pie).toContain(
      'La línea se corta en 1 lugar donde faltan más de 6 horas: son tramos distintos, y la media de 24 horas no es continua de uno al otro.',
    )

    const conLecturas = textosDeCurva('CA-HU-04', armarCurva([otroDia(8, 40), otroDia(20, 41)], [vista(10, 150, 90)], 'pm10'), 'pm10')
    expect(conLecturas.pie).toContain('La línea se corta en 2 lugares donde faltan más de 6 horas')
  })

  it('sin huecos no dice nada de cortes', () => {
    expect(textosDeCurva('CA-HU-04', armarCurva([estado(10, 90), estado(11, 95)], [], 'pm10'), 'pm10').pie).not.toContain('se corta')
  })
})

describe('textosDeCurva', () => {
  it('con solo estados dice que cada punto es la media de un estado y que las lecturas no se ven', () => {
    const t = textosDeCurva('CA-HU-04', armarCurva([estado(10, 90), estado(11, 120.7)], [], 'pm10'), 'pm10')

    expect(t.rotulo).toBe('Lo recibido en esta sesión')
    expect(t.titulo).toBe('Media de 24 horas de PM10')
    expect(t.cuenta).toBe('2 estados recibidos')
    expect(t.pie).toMatch(/^ por esta pestaña, el último del 15 dic\.? 2021,? 11:00\. Cada punto de la línea es la media de 24 horas de un estado\./)
    expect(t.pie).toContain('Las lecturas horarias no se ven: el estado no las trae, y esta sesión no envió ninguna de CA-HU-04 desde Simulación.')
    expect(t.pie).toContain('No es una serie histórica')
    expect(t.pie).not.toContain('rombo')
    expect(t.descripcion).toMatch(/^Media de 24 horas de PM10 de CA-HU-04: 2 estados recibidos en esta sesión, el último del .*\. ECA: 100 µg\/m³\.$/)
  })

  it('con un solo estado, en singular', () => {
    const t = textosDeCurva('CA-HU-04', armarCurva([estado(10, 90)], [], 'pm2_5'), 'pm2_5')

    expect(t.cuenta).toBe('1 estado recibido')
    expect(t.titulo).toBe('Media de 24 horas de PM2.5')
    expect(t.descripcion).toContain('ECA: 50 µg/m³.')
  })

  it('dice cuántas horas quedaron sin media y por qué', () => {
    const t = textosDeCurva('CA-CG-02', armarCurva([estado(10, null), estado(11, null), estado(12, 40)], [], 'pm10'), 'pm10')

    expect(t.pie).toContain('2 horas quedaron sin media, porque la ventana tenía menos de 18 lecturas (círculo hueco, abajo).')
  })

  it('con lecturas enviadas dice qué es cada marca y que el ECA se compara con la media', () => {
    const curva = armarCurva([estado(11, 95)], [vista(10, 150, null), vista(11, 180, 95)], 'pm10')
    const t = textosDeCurva('CA-HU-04', curva, 'pm10')

    expect(t.rotulo).toBe('Lo enviado y lo recibido en esta sesión')
    expect(t.titulo).toBe('Lecturas horarias y media de 24 horas de PM10')
    expect(t.cuenta).toBe('2 lecturas horarias')
    expect(t.pie).toMatch(/^ de PM10 que esta sesión envió desde Simulación, la última del 15 dic\.? 2021,? 11:00\./)
    expect(t.pie).toContain('Cada rombo es una de esas lecturas; la línea, la media de 24 horas que la API respondió a cada una y la de los estados recibidos.')
    expect(t.pie).toContain('1 hora quedó sin media')
    expect(t.pie).toContain('El ECA es de 24 horas: se compara con la media, no con cada lectura.')
    expect(t.pie).toContain('Las lecturas enviadas se olvidan al recargar la página')
    expect(t.pie).not.toContain('cuya lectura no envió esta sesión')
    expect(t.descripcion).toMatch(/^Lecturas horarias y media de 24 horas de PM10 de CA-HU-04: 2 lecturas horarias enviadas en esta sesión, la última del .*, y la media de 24 horas de 1 hora\. ECA: 100 µg\/m³\.$/)
  })

  it('si hay estados de lecturas que esta sesión no envió, dice que de esas horas solo se ve la media', () => {
    const curva = armarCurva([estado(13, 99), estado(14, 98)], [vista(11, 180, 95)], 'pm10')
    const t = textosDeCurva('CA-HU-04', curva, 'pm10')

    expect(t.cuenta).toBe('1 lectura horaria')
    expect(t.pie).toContain('2 puntos de la línea son de estados recibidos cuya lectura no envió esta sesión: de esas horas se ve la media, no la lectura.')
    // «La última» es la última lectura enviada, no el último estado recibido.
    expect(t.pie).toMatch(/la última del 15 dic\.? 2021,? 11:00/)
  })
})
