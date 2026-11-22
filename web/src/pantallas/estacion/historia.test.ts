import { QueryClient } from '@tanstack/react-query'
import { describe, expect, it } from 'vitest'
import { claves } from '../../api/cliente'
import { estadoDe } from '../panel/dobles'
import { Historia, historiaDe } from './historia'

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

    expect(horas(new Historia(), 'CA-HU-04')).toEqual(['10'])
    expect(localStorage.length).toBe(0)
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

    it('no mira lo que no es el estado de las estaciones', () => {
      const consultas = new QueryClient()
      const h = historiaDe(consultas)

      consultas.setQueryData(claves.estaciones, [{ id: 'CA-HU-04' }])
      consultas.setQueryData(claves.entrenamiento('e-1'), estadoDe('CA-HU-04'))

      expect(h.estaciones()).toEqual([])
    })
  })
})
