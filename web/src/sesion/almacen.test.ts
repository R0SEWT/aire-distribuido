import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { abrirSesion, cerrarSesion, estadoSesion, recargarSesion, sesionActual, suscribir, type Sesion } from './almacen'

const CLAVE = 'aire.sesion'

function sesionQueVenceEn(ms: number): Sesion {
  return { usuario: 'demo', token: 'un-token', rol: 'usuario', expira: Date.now() + ms }
}

describe('la sesión', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    recargarSesion()
  })
  afterEach(() => {
    cerrarSesion('salir')
    vi.useRealTimers()
  })

  it('guarda el token en sessionStorage y no en localStorage', () => {
    abrirSesion(sesionQueVenceEn(60_000))

    expect(sesionActual()?.token).toBe('un-token')
    expect(JSON.parse(sessionStorage.getItem(CLAVE)!)).toMatchObject({ token: 'un-token', rol: 'usuario' })
    expect(localStorage.length).toBe(0)
  })

  it('sobrevive a recargar la página', () => {
    abrirSesion(sesionQueVenceEn(60_000))

    recargarSesion() // lo que pasa al cargar el módulo después de un F5

    expect(sesionActual()).toMatchObject({ usuario: 'demo', rol: 'usuario' })
  })

  it('se cierra sola a la hora en que vence el token, con el motivo «vencida»', () => {
    const oyente = vi.fn()
    const dejar = suscribir(oyente)
    abrirSesion(sesionQueVenceEn(60_000))

    vi.advanceTimersByTime(59_999)
    expect(sesionActual()).not.toBeNull()

    vi.advanceTimersByTime(1)
    expect(estadoSesion()).toEqual({ sesion: null, motivo: 'vencida' })
    expect(sessionStorage.getItem(CLAVE)).toBeNull()
    expect(oyente).toHaveBeenCalledTimes(2) // al abrir y al vencer
    dejar()
  })

  it('no levanta una sesión guardada que ya venció', () => {
    sessionStorage.setItem(CLAVE, JSON.stringify(sesionQueVenceEn(-1)))

    recargarSesion()

    expect(sesionActual()).toBeNull()
    expect(sessionStorage.getItem(CLAVE)).toBeNull()
  })

  it.each([
    ['algo que no es JSON', '{no es json'],
    ['un rol que no existe', JSON.stringify({ usuario: 'x', token: 't', rol: 'root', expira: Date.now() + 60_000 })],
    ['una sesión sin token', JSON.stringify({ usuario: 'x', token: '', rol: 'admin', expira: Date.now() + 60_000 })],
  ])('descarta de sessionStorage %s', (_, guardado) => {
    sessionStorage.setItem(CLAVE, guardado)

    recargarSesion()

    expect(sesionActual()).toBeNull()
    expect(sessionStorage.getItem(CLAVE)).toBeNull()
  })

  it('al salir borra el token y recuerda que fue a propósito', () => {
    abrirSesion(sesionQueVenceEn(60_000))

    cerrarSesion('salir')

    expect(estadoSesion()).toEqual({ sesion: null, motivo: 'salir' })
    expect(sessionStorage.getItem(CLAVE)).toBeNull()
    // Y el reloj del vencimiento ya no dispara nada.
    vi.advanceTimersByTime(120_000)
    expect(estadoSesion().motivo).toBe('salir')
  })
})
