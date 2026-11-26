// Las marcas del eje de tiempo caen en horas redondas de Lima en cualquier zona.
// La prueba corre con el reloj en la India (UTC+5:30): con marcas elegidas en la
// zona del navegador, todas caerían a las y media de Lima.

import { render } from '@testing-library/react'
import { afterAll, beforeAll, describe, expect, it, vi } from 'vitest'
import { CurvaSesion } from './CurvaSesion'
import { marcasEnHoraDeLima } from './ejeDeHoras'

const HORA_MS = 3_600_000
const limaDe = (t: number) => new Date(t - 5 * HORA_MS) // sus campos UTC son la hora de Lima
const t = (iso: string) => Date.parse(iso)

describe('con el navegador en otra zona horaria', () => {
  beforeAll(() => {
    vi.stubEnv('TZ', 'Asia/Kolkata')
  })
  afterAll(() => {
    vi.unstubAllEnvs()
  })

  it('el reloj de la prueba está de verdad en la India', () => {
    expect(new Date(t('2021-12-15T00:00:00Z')).getTimezoneOffset()).toBe(-330)
  })

  it('las marcas son horas redondas de Lima: en punto, y en múltiplos del paso', () => {
    const marcas = marcasEnHoraDeLima(t('2021-12-14T01:00:00-05:00'), t('2021-12-16T00:00:00-05:00'), 6)

    expect(marcas.length).toBeGreaterThanOrEqual(3)
    for (const marca of marcas) {
      expect(limaDe(marca).getUTCMinutes()).toBe(0)
      expect(limaDe(marca).getUTCHours() % 6).toBe(0)
    }
    // Y una de ellas es la medianoche de Lima del 15, donde cambia el día.
    expect(marcas).toContain(t('2021-12-15T00:00:00-05:00'))
  })

  it('con un tramo de pocas horas también caen en punto', () => {
    const marcas = marcasEnHoraDeLima(t('2021-12-15T08:20:00-05:00'), t('2021-12-15T13:40:00-05:00'), 5)

    expect(marcas.map((m) => limaDe(m).getUTCHours())).toEqual([9, 10, 11, 12, 13])
    for (const marca of marcas) expect(limaDe(marca).getUTCMinutes()).toBe(0)
  })

  it('la curva de la Estación rotula su eje con horas en punto de Lima', () => {
    const puntos = Array.from({ length: 24 }, (_, h) => ({ t: t('2021-12-15T00:00:00-05:00') + h * HORA_MS, valor: 80 + h }))

    const { container } = render(<CurvaSesion puntos={puntos} eca={100} cortes={[75, 150, 250]} titulo="curva de prueba" />)

    const horas = [...container.querySelectorAll('[data-capa="ejes"] text')].map((e) => e.textContent ?? '').filter((txt) => /^\d\d:\d\d$/.test(txt))
    expect(horas.length).toBeGreaterThanOrEqual(3)
    for (const hora of horas) expect(hora).toMatch(/:00$/)
    expect(horas).toContain('00:00')
  })
})
