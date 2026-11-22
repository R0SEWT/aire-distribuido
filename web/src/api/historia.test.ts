import { QueryClient } from '@tanstack/react-query'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { estadoDe } from '../pantallas/panel/dobles'
import { claves } from './cliente'
import { GUARDAR_CADA_MS, Historia, historiaDe, olvidarHistoria } from './historia'

const horas = (h: Historia, estacion: string) => h.leer(estacion).map((p) => p.fecha_hora.slice(11, 13))

describe('la historia de la sesión', () => {
  it('anota cada estado recibido de una estación, en orden', () => {
    const h = new Historia()

    h.anotar('CA-HU-04', estadoDe('CA-HU-04', { hora: 10, pm10: 90 }))
    h.anotar('CA-HU-04', estadoDe('CA-HU-04', { hora: 11, pm10: 101, alerta: true }))
    h.anotar('CA-AYAS-01', estadoDe('CA-AYAS-01', { hora: 11 }))

    expect(h.leer('CA-HU-04')).toMatchObject([
      { pm10: 90, alerta: false, lecturas: 24 },
      { pm10: 101, alerta: true },
    ])
    expect(horas(h, 'CA-HU-04')).toEqual(['10', '11'])
    expect(horas(h, 'CA-AYAS-01')).toEqual(['11'])
    expect(h.leer('CA-XX-99')).toEqual([])
  })

  it('el mismo estado dos veces no suma un punto, y una corrección de la misma hora lo reemplaza', () => {
    const h = new Historia()
    h.anotar('CA-HU-04', estadoDe('CA-HU-04', { hora: 10, pm10: 90 }))
    const antes = h.leer('CA-HU-04')

    expect(h.anotar('CA-HU-04', estadoDe('CA-HU-04', { hora: 10, pm10: 90 }))).toBe(false)
    expect(h.leer('CA-HU-04')).toBe(antes) // la misma lista: quien la mira no se vuelve a dibujar

    expect(h.anotar('CA-HU-04', estadoDe('CA-HU-04', { hora: 10, pm10: 95 }))).toBe(true)
    expect(h.leer('CA-HU-04')).toMatchObject([{ pm10: 95 }])
  })

  it('guarda también los estados sin media: la curva los muestra aparte', () => {
    const h = new Historia()

    h.anotar('CA-HU-04', estadoDe('CA-HU-04', { pm2_5: null, pm10: null, inca: null, lecturas: 4 }))

    expect(h.leer('CA-HU-04')).toMatchObject([{ pm2_5: null, pm10: null, lecturas: 4, inca: null }])
  })

  it('si llega un estado anterior al último, el sistema se reinició: la curva empieza de nuevo', () => {
    const h = new Historia()
    h.anotar('CA-HU-04', estadoDe('CA-HU-04', { hora: 10 }))
    h.anotar('CA-HU-04', estadoDe('CA-HU-04', { hora: 11 }))

    h.anotar('CA-HU-04', estadoDe('CA-HU-04', { hora: 3 }))

    expect(horas(h, 'CA-HU-04')).toEqual(['03'])
  })

  it('sobrevive a recargar la página: se guarda en sessionStorage, no en localStorage', () => {
    const h = new Historia()
    h.anotar('CA-HU-04', estadoDe('CA-HU-04', { hora: 10, pm10: 90 }))
    h.publicar()
    h.guardar() // lo que hace la página al irse: no espera al reloj

    expect(horas(new Historia(), 'CA-HU-04')).toEqual(['10'])
    expect(localStorage.length).toBe(0)
  })

  it('al vaciarse (la sesión terminó) olvida todo, también lo guardado, y avisa a quien mira', () => {
    const h = new Historia()
    h.anotar('CA-HU-04', estadoDe('CA-HU-04', { hora: 10 }))
    h.anotar('CA-AYAS-01', estadoDe('CA-AYAS-01', { hora: 10 }))
    h.publicar()
    h.guardar()
    expect(sessionStorage.getItem('aire.curvas')).toContain('CA-HU-04')
    let avisos = 0
    h.suscribir(() => avisos++)

    h.vaciar()

    expect(h.estaciones()).toEqual([])
    expect(h.leer('CA-HU-04')).toEqual([])
    expect(sessionStorage.getItem('aire.curvas')).toBeNull()
    expect(new Historia().leer('CA-HU-04')).toEqual([])
    expect(avisos).toBe(1)
  })

  describe('las escrituras en sessionStorage se agrupan', () => {
    beforeEach(() => {
      vi.useFakeTimers()
    })
    afterEach(() => {
      vi.useRealTimers()
    })

    it('una ráfaga de estados avisa cada vez a quien mira, pero escribe una sola vez', () => {
      const escribir = vi.spyOn(Storage.prototype, 'setItem')
      const h = new Historia()
      let avisos = 0
      h.suscribir(() => avisos++)

      // Una corrida a ritmo rápido: decenas de estados en menos de medio segundo.
      for (let hora = 0; hora < 20; hora++) {
        h.anotar('CA-HU-04', estadoDe('CA-HU-04', { hora }))
        h.publicar()
        vi.advanceTimersByTime(10)
      }

      expect(avisos).toBe(20)
      expect(horas(h, 'CA-HU-04')).toHaveLength(20) // en memoria está todo, al instante
      expect(escribir).not.toHaveBeenCalled()

      vi.advanceTimersByTime(GUARDAR_CADA_MS)

      expect(escribir).toHaveBeenCalledTimes(1)
      expect(horas(new Historia(), 'CA-HU-04')).toHaveLength(20)
    })

    it('una corrida larga escribe a lo sumo una vez por intervalo, no una por estado', () => {
      const escribir = vi.spyOn(Storage.prototype, 'setItem')
      const h = new Historia()

      // 600 estados en 6 segundos, como el escenario a ritmo rápido con 14 estaciones.
      for (let i = 0; i < 600; i++) {
        h.anotar(`CA-${i % 14}`, estadoDe(`CA-${i % 14}`, { hora: Math.floor(i / 14) % 24 }))
        h.publicar()
        vi.advanceTimersByTime(10)
      }
      vi.advanceTimersByTime(GUARDAR_CADA_MS)

      expect(escribir.mock.calls.length).toBeLessThanOrEqual(6000 / GUARDAR_CADA_MS + 1)
      expect(escribir.mock.calls.length).toBeGreaterThan(0)
    })

    it('guardar() escribe ya lo pendiente y no deja otra escritura programada; sin nada pendiente no escribe', () => {
      const escribir = vi.spyOn(Storage.prototype, 'setItem')
      const h = new Historia()
      h.guardar()
      expect(escribir).not.toHaveBeenCalled()

      h.anotar('CA-HU-04', estadoDe('CA-HU-04', { hora: 10 }))
      h.publicar()
      h.guardar()
      expect(escribir).toHaveBeenCalledTimes(1)

      vi.advanceTimersByTime(GUARDAR_CADA_MS * 3)
      expect(escribir).toHaveBeenCalledTimes(1)
    })

    it('al vaciarse con una escritura pendiente, esa escritura ya no ocurre', () => {
      const h = new Historia()
      h.anotar('CA-HU-04', estadoDe('CA-HU-04', { hora: 10 }))
      h.publicar()

      h.vaciar()
      vi.advanceTimersByTime(GUARDAR_CADA_MS * 3)

      expect(sessionStorage.getItem('aire.curvas')).toBeNull()
    })

    it('al ocultarse o irse la página se escribe lo pendiente: un F5 a mitad de una ráfaga no pierde lo último', () => {
      const consultas = new QueryClient()
      historiaDe(consultas)

      consultas.setQueryData(claves.regimen('CA-HU-04'), estadoDe('CA-HU-04', { hora: 10 }))
      expect(sessionStorage.getItem('aire.curvas')).toBeNull()
      window.dispatchEvent(new Event('pagehide'))
      expect(sessionStorage.getItem('aire.curvas')).toContain('CA-HU-04')

      consultas.setQueryData(claves.regimen('CA-AYAS-01'), estadoDe('CA-AYAS-01', { hora: 10 }))
      Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => 'hidden' })
      document.dispatchEvent(new Event('visibilitychange'))
      expect(sessionStorage.getItem('aire.curvas')).toContain('CA-AYAS-01')
      Reflect.deleteProperty(document, 'visibilityState')
      olvidarHistoria(consultas)
    })
  })

  it('si lo guardado no se puede leer, empieza de cero', () => {
    sessionStorage.setItem('aire.curvas', '{"CA-HU-04": "no es una lista"')

    expect(new Historia().leer('CA-HU-04')).toEqual([])
  })

  describe('mirando la caché', () => {
    it('anota lo que ya estaba y lo que va entrando, por la lista o por la estación', () => {
      const consultas = new QueryClient()
      consultas.setQueryData(claves.regimenes, [estadoDe('CA-HU-04', { hora: 10 }), estadoDe('CA-AYAS-01', { hora: 10 })])

      const h = historiaDe(consultas)
      expect(horas(h, 'CA-HU-04')).toEqual(['10'])

      // Lo que hace el armazón con un evento: la lista y la entrada de la estación.
      const evento = estadoDe('CA-HU-04', { hora: 11 })
      consultas.setQueryData(claves.regimenes, [evento, estadoDe('CA-AYAS-01', { hora: 10 })])
      consultas.setQueryData(claves.regimen('CA-HU-04'), evento)

      expect(horas(h, 'CA-HU-04')).toEqual(['10', '11'])
      expect(horas(h, 'CA-AYAS-01')).toEqual(['10'])
      expect(historiaDe(consultas)).toBe(h)
    })

    it('avisa a quien mira cuando cambia, y no cuando llega lo mismo', () => {
      const consultas = new QueryClient()
      const h = historiaDe(consultas)
      let avisos = 0
      h.suscribir(() => avisos++)

      consultas.setQueryData(claves.regimen('CA-HU-04'), estadoDe('CA-HU-04', { hora: 10 }))
      consultas.setQueryData(claves.regimen('CA-HU-04'), estadoDe('CA-HU-04', { hora: 10 }))

      expect(avisos).toBe(1)
    })

    it('si una estación deja de tener estado en la API, su curva se borra', () => {
      const consultas = new QueryClient()
      const h = historiaDe(consultas)
      consultas.setQueryData(claves.regimenes, [estadoDe('CA-HU-04'), estadoDe('CA-AYAS-01')])

      consultas.setQueryData(claves.regimenes, [estadoDe('CA-AYAS-01')])
      expect(h.leer('CA-HU-04')).toEqual([])

      consultas.setQueryData(claves.regimen('CA-AYAS-01'), null)
      expect(h.leer('CA-AYAS-01')).toEqual([])
    })

    it('olvidarHistoria vacía la de esa caché, que sigue anotando lo que llegue después', () => {
      const consultas = new QueryClient()
      const h = historiaDe(consultas)
      consultas.setQueryData(claves.regimen('CA-HU-04'), estadoDe('CA-HU-04', { hora: 10 }))
      h.guardar()
      expect(sessionStorage.getItem('aire.curvas')).toContain('CA-HU-04')

      olvidarHistoria(consultas)
      expect(h.leer('CA-HU-04')).toEqual([])
      expect(sessionStorage.getItem('aire.curvas')).toBeNull()

      consultas.setQueryData(claves.regimen('CA-AYAS-01'), estadoDe('CA-AYAS-01', { hora: 11 }))
      expect(horas(h, 'CA-AYAS-01')).toEqual(['11'])
    })

    it('olvidarHistoria borra lo guardado aunque nadie la haya encendido en esa caché', () => {
      sessionStorage.setItem('aire.curvas', '{"CA-HU-04":[]}')

      olvidarHistoria(new QueryClient())

      expect(sessionStorage.getItem('aire.curvas')).toBeNull()
    })

    it('no mira lo que no es el estado de las estaciones', () => {
      const consultas = new QueryClient()
      const h = historiaDe(consultas)

      consultas.setQueryData(claves.estaciones, [{ id: 'CA-HU-04' }])
      consultas.setQueryData(claves.entrenamiento('e-1'), estadoDe('CA-HU-04'))

      expect(h.estaciones()).toEqual([])
    })
  })
})
