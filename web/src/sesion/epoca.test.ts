import { describe, expect, it } from 'vitest'
import { avanzarEpoca, marcarSesion } from './epoca'

describe('la época de la sesión', () => {
  it('una marca sigue valiendo mientras la sesión no termine', () => {
    const sigue = marcarSesion()

    expect(sigue()).toBe(true)
    expect(sigue()).toBe(true)
  })

  it('al terminar la sesión, las marcas de antes dejan de valer y las de después valen', () => {
    const deAntes = marcarSesion()

    avanzarEpoca()
    const deDespues = marcarSesion()

    expect(deAntes()).toBe(false)
    expect(deDespues()).toBe(true)
    avanzarEpoca()
    expect(deDespues()).toBe(false)
  })
})
