// El simulador sin pantalla: el orden de envío, el ritmo, la pausa, la
// detención y qué pasa cuando un pedido falla a mitad de camino.

import { describe, expect, it, vi } from 'vitest'
import { ErrorApi } from '../../api/errores'
import { Simulador, type Dependencias } from './motor'
import type { Lectura, Prediccion, Tramo } from './series'
import { tramoDelDia } from './series'

const HORA_MS = 3_600_000
const INICIO = Date.parse('2021-12-14T01:00:00-05:00')

function lectura(estacion: string, hora: number, pm = 10): Lectura {
  const t = new Date(INICIO + hora * HORA_MS - 5 * HORA_MS)
  const dos = (n: number) => String(n).padStart(2, '0')
  const fecha = `${t.getUTCFullYear()}-${dos(t.getUTCMonth() + 1)}-${dos(t.getUTCDate())}T${dos(t.getUTCHours())}:00:00-05:00`
  return { estacion, fecha_hora: fecha, pm2_5: pm, pm10: pm * 2 }
}

/** Un tramo de `horas` horas con una lectura por estación y hora, en el orden del archivo. */
function tramoDe(estaciones: string[], horas: number): Tramo {
  const lecturas = Array.from({ length: horas }, (_, h) => estaciones.map((e) => lectura(e, h))).flat()
  return {
    id: 'prueba',
    tipo: 'escenario',
    titulo: 'prueba',
    desde: lecturas[0]!.fecha_hora,
    hasta: lecturas[lecturas.length - 1]!.fecha_hora,
    lecturas,
    estaciones,
    esperado: {},
    primera: {},
    ultima: {},
  }
}

function prediccion(l: Lectura, n: number): Prediccion {
  const hayMedia = n >= 18
  return {
    estacion: l.estacion,
    regimen: 0,
    excede_eca: false,
    media_24h: { pm2_5: hayMedia ? l.pm2_5 : null, pm10: hayMedia ? l.pm10 : null, lecturas: Math.min(n, 24) },
    alerta: hayMedia && l.pm10 > 100,
    inca: hayMedia ? 'buena' : null,
    pm2_5_medio_regimen: 9,
    pm10_medio_regimen: 17,
    modelo_id: 'de-prueba',
    ms: 0.01,
  }
}

/** Un pedido que se resuelve a mano: para pausar o detener con una lectura en viaje. */
function aplazado<T>() {
  let resolver!: (v: T) => void
  let rechazar!: (e: unknown) => void
  const promesa = new Promise<T>((a, b) => {
    resolver = a
    rechazar = b
  })
  return { promesa, resolver, rechazar }
}

function armar(parcial: Partial<Dependencias> = {}) {
  const enviadas: Lectura[] = []
  const porEstacion = new Map<string, number>()
  const esperas: number[] = []
  const deps: Dependencias = {
    enviar: async (l) => {
      enviadas.push(l)
      const n = (porEstacion.get(l.estacion) ?? 0) + 1
      porEstacion.set(l.estacion, n)
      return prediccion(l, n)
    },
    esperar: async (ms) => {
      esperas.push(ms)
    },
    ahora: () => 0,
    ventana: null,
    ...parcial,
  }
  return { simulador: new Simulador(deps), enviadas, esperas }
}

async function hasta(simulador: Simulador, condicion: () => boolean) {
  await vi.waitFor(() => {
    if (!condicion()) throw new Error(`todavía no: fase ${simulador.leer().fase}, enviadas ${simulador.leer().enviadas}`)
  })
}

describe('el simulador', () => {
  it('manda las lecturas una por una, en el orden del archivo, y termina', async () => {
    const tramo = tramoDe(['CA-A', 'CA-B', 'CA-C'], 4)
    const { simulador, enviadas } = armar()

    simulador.iniciar(tramo, 4)
    await hasta(simulador, () => simulador.leer().fase === 'terminada')

    expect(enviadas).toEqual(tramo.lecturas)
    const corrida = simulador.leer()
    expect(corrida.enviadas).toBe(12)
    expect(corrida.horaActual).toBe(tramo.hasta)
    expect(corrida.modeloId).toBe('de-prueba')
    expect(Object.keys(corrida.porEstacion).sort()).toEqual(['CA-A', 'CA-B', 'CA-C'])
    expect(corrida.porEstacion['CA-B']!.puntos.map((p) => p.fecha_hora)).toEqual(tramo.lecturas.filter((l) => l.estacion === 'CA-B').map((l) => l.fecha_hora))
  })

  it('no manda una lectura hasta que la anterior tuvo respuesta', async () => {
    const tramo = tramoDe(['CA-A', 'CA-B'], 2)
    const enViaje: ReturnType<typeof aplazado<Prediccion>>[] = []
    const pedidas: Lectura[] = []
    const { simulador } = armar({
      enviar: (l) => {
        pedidas.push(l)
        const a = aplazado<Prediccion>()
        enViaje.push(a)
        return a.promesa
      },
    })

    simulador.iniciar(tramo, 4)
    await vi.waitFor(() => expect(pedidas).toHaveLength(1))
    await Promise.resolve()
    expect(pedidas).toHaveLength(1)

    enViaje[0]!.resolver(prediccion(pedidas[0]!, 1))
    await vi.waitFor(() => expect(pedidas).toHaveLength(2))
    expect(pedidas[1]).toEqual(tramo.lecturas[1])
    simulador.detener()
  })

  it('espera entre una hora y la siguiente, no entre lecturas de la misma hora, al ritmo pedido', async () => {
    const tramo = tramoDe(['CA-A', 'CA-B', 'CA-C'], 4)
    const { simulador, esperas } = armar()

    simulador.iniciar(tramo, 4)
    await hasta(simulador, () => simulador.leer().fase === 'terminada')

    // 4 horas: 3 esperas, de un cuarto de segundo (4 horas por segundo).
    expect(esperas).toEqual([250, 250, 250])
  })

  it('la primera media y la primera alerta son las de la primera respuesta que las trae', async () => {
    const tramo = tramoDelDia({
      id: 'CA-A-dia',
      estacion: 'CA-A',
      dia: '2021-12-15',
      caso: 'cruza_eca',
      descripcion: '',
      desde: lectura('CA-A', 0).fecha_hora,
      hasta: lectura('CA-A', 29).fecha_hora,
      horas: 30,
      huecos: 0,
      // Desde la hora 22, el PM10 pasa de 100.
      lecturas: Array.from({ length: 30 }, (_, h) => lectura('CA-A', h, h >= 22 ? 80 : 10)),
      esperado: {
        fecha_hora: lectura('CA-A', 29).fecha_hora,
        lecturas_enviadas: 30,
        regimen: 0,
        excede_eca: false,
        media_24h: { pm2_5: 80, pm10: 160, lecturas: 24 },
        alerta: true,
        inca: 'buena',
        primera_media: lectura('CA-A', 17).fecha_hora,
        primera_alerta: lectura('CA-A', 22).fecha_hora,
      },
    })
    const { simulador } = armar()

    simulador.iniciar(tramo, 12)
    await hasta(simulador, () => simulador.leer().fase === 'terminada')

    const s = simulador.leer().porEstacion['CA-A']!
    expect(s.primeraMedia).toBe(lectura('CA-A', 17).fecha_hora)
    expect(s.primeraAlerta).toBe(lectura('CA-A', 22).fecha_hora)
    expect(s.puntos[16]).toMatchObject({ media_pm10: null, lecturas: 17, inca: null })
    expect(s.puntos[17]).toMatchObject({ media_pm10: 20, lecturas: 18, inca: 'buena', alerta: false })
    expect(s.ultima?.alerta).toBe(true)
  })

  describe('en pausa', () => {
    it('deja terminar la lectura que está en viaje, la anota y no manda la siguiente', async () => {
      const tramo = tramoDe(['CA-A'], 6)
      const pedidas: Lectura[] = []
      let puerta: ReturnType<typeof aplazado<void>> | null = null
      const { simulador } = armar({
        enviar: async (l) => {
          pedidas.push(l)
          if (pedidas.length === 3) {
            puerta = aplazado<void>()
            await puerta.promesa
          }
          return prediccion(l, pedidas.length)
        },
      })

      simulador.iniciar(tramo, 4)
      await vi.waitFor(() => expect(puerta).not.toBeNull())
      simulador.pausar()
      expect(simulador.leer().fase).toBe('en_pausa')
      expect(simulador.leer().enviadas).toBe(2)

      puerta!.resolver()
      await hasta(simulador, () => simulador.leer().enviadas === 3)
      await new Promise((r) => setTimeout(r, 10))

      expect(pedidas).toHaveLength(3)
      expect(simulador.leer().fase).toBe('en_pausa')
      expect(simulador.leer().porEstacion['CA-A']!.puntos).toHaveLength(3)
    })

    it('al continuar sigue desde la lectura siguiente, sin repetir ni saltar ninguna', async () => {
      const tramo = tramoDe(['CA-A', 'CA-B'], 5)
      const { simulador, enviadas } = armar({
        esperar: async () => {
          // Se pausa en la primera espera: entre la hora 0 y la hora 1.
          if (simulador.leer().enviadas === 2) simulador.pausar()
        },
      })

      simulador.iniciar(tramo, 4)
      await hasta(simulador, () => simulador.leer().fase === 'en_pausa')
      await new Promise((r) => setTimeout(r, 10))
      expect(enviadas).toHaveLength(2)

      simulador.continuar()
      await hasta(simulador, () => simulador.leer().fase === 'terminada')

      expect(enviadas).toEqual(tramo.lecturas)
    })
  })

  describe('si un pedido falla a mitad de camino', () => {
    it('se queda en esa lectura, con el error, y no manda las que siguen', async () => {
      const tramo = tramoDe(['CA-A'], 6)
      const pedidas: Lectura[] = []
      const error = new ErrorApi('no_disponible', 503, 'no hay modelo activo')
      const { simulador } = armar({
        enviar: async (l) => {
          pedidas.push(l)
          if (pedidas.length === 4) throw error
          return prediccion(l, pedidas.length)
        },
      })

      simulador.iniciar(tramo, 4)
      await hasta(simulador, () => simulador.leer().fase === 'con_error')
      await new Promise((r) => setTimeout(r, 10))

      const corrida = simulador.leer()
      expect(pedidas).toHaveLength(4)
      expect(corrida.enviadas).toBe(3)
      expect(corrida.error).toBe(error)
      expect(corrida.lecturaConError).toEqual(tramo.lecturas[3])
    })

    it('al continuar vuelve a mandar la que falló y sigue hasta el final', async () => {
      const tramo = tramoDe(['CA-A'], 6)
      const pedidas: Lectura[] = []
      let falla = true
      const { simulador } = armar({
        enviar: async (l) => {
          pedidas.push(l)
          if (pedidas.length === 4 && falla) throw new ErrorApi('sin_respuesta', null, null)
          return prediccion(l, pedidas.length)
        },
      })
      simulador.iniciar(tramo, 4)
      await hasta(simulador, () => simulador.leer().fase === 'con_error')

      falla = false
      simulador.continuar()
      await hasta(simulador, () => simulador.leer().fase === 'terminada')

      expect(pedidas.map((l) => l.fecha_hora)).toEqual([
        ...tramo.lecturas.slice(0, 4).map((l) => l.fecha_hora),
        ...tramo.lecturas.slice(3).map((l) => l.fecha_hora),
      ])
      expect(simulador.leer().enviadas).toBe(6)
      expect(simulador.leer().error).toBeNull()
    })
  })

  describe('al detener', () => {
    it('cancela el pedido en viaje, no manda más y conserva lo que ya se respondió', async () => {
      const tramo = tramoDe(['CA-A'], 6)
      const pedidas: Lectura[] = []
      let cortada = false
      const { simulador } = armar({
        enviar: (l, signal) => {
          pedidas.push(l)
          if (pedidas.length < 3) return Promise.resolve(prediccion(l, pedidas.length))
          return new Promise((_, rechazar) => {
            signal.addEventListener('abort', () => {
              cortada = true
              rechazar(new DOMException('cancelado', 'AbortError'))
            })
          })
        },
      })

      simulador.iniciar(tramo, 4)
      await vi.waitFor(() => expect(pedidas).toHaveLength(3))
      simulador.detener()
      await new Promise((r) => setTimeout(r, 10))

      expect(cortada).toBe(true)
      expect(pedidas).toHaveLength(3)
      expect(simulador.leer().fase).toBe('detenida')
      expect(simulador.leer().enviadas).toBe(2)
      expect(simulador.leer().error).toBeNull()
    })

    it('se puede empezar otra corrida, que no arrastra nada de la anterior', async () => {
      const { simulador, enviadas } = armar({
        esperar: async () => {
          if (simulador.leer().tramo?.id === 'prueba' && simulador.leer().enviadas === 1 && enviadas.length === 1) simulador.detener()
        },
      })
      simulador.iniciar(tramoDe(['CA-A'], 5), 4)
      await hasta(simulador, () => simulador.leer().fase === 'detenida')

      const otro = { ...tramoDe(['CA-B'], 3), id: 'otro' }
      simulador.iniciar(otro, 4)
      await hasta(simulador, () => simulador.leer().fase === 'terminada')

      expect(simulador.leer().tramo?.id).toBe('otro')
      expect(Object.keys(simulador.leer().porEstacion)).toEqual(['CA-B'])
      expect(simulador.leer().enviadas).toBe(3)
    })
  })

  it('con una corrida sin terminar, iniciar otra no hace nada', async () => {
    const puerta = aplazado<Prediccion>()
    const { simulador } = armar({ enviar: () => puerta.promesa })
    const primero = tramoDe(['CA-A'], 3)
    simulador.iniciar(primero, 4)

    simulador.iniciar({ ...tramoDe(['CA-B'], 3), id: 'otro' }, 4)

    expect(simulador.leer().tramo).toBe(primero)
    simulador.detener()
  })

  it('avisa a quien está suscrito y entrega una foto nueva cada vez', async () => {
    const { simulador } = armar()
    const fotos = new Set<unknown>()
    const dejar = simulador.suscribir(() => fotos.add(simulador.leer()))

    simulador.iniciar(tramoDe(['CA-A'], 3), 4)
    await hasta(simulador, () => simulador.leer().fase === 'terminada')
    dejar()

    // Al empezar, después de cada hora y al terminar.
    expect(fotos.size).toBeGreaterThanOrEqual(3)
    expect([...fotos].at(-1)).toMatchObject({ fase: 'terminada', enviadas: 3 })
  })

  describe('el aviso antes de cerrar o recargar la pestaña', () => {
    /** Una ventana de mentira que recuerda quién oye `beforeunload`. */
    function ventanaFalsa() {
      const oyentes = new Set<(evento: BeforeUnloadEvent) => void>()
      return {
        oyentes,
        addEventListener: vi.fn((_tipo: 'beforeunload', oyente: (evento: BeforeUnloadEvent) => void) => void oyentes.add(oyente)),
        removeEventListener: vi.fn((_tipo: 'beforeunload', oyente: (evento: BeforeUnloadEvent) => void) => void oyentes.delete(oyente)),
      }
    }

    /** Un simulador cuya primera lectura queda en viaje hasta que se la suelte. */
    function conLecturaEnViaje(ventana: ReturnType<typeof ventanaFalsa>) {
      const enViaje: ReturnType<typeof aplazado<Prediccion>>[] = []
      const { simulador } = armar({
        ventana,
        enviar: () => {
          const a = aplazado<Prediccion>()
          enViaje.push(a)
          return a.promesa
        },
      })
      return { simulador, enViaje }
    }

    it('sin corrida no hay aviso; al empezar se registra, una sola vez', async () => {
      const ventana = ventanaFalsa()
      const { simulador, enViaje } = conLecturaEnViaje(ventana)
      expect(ventana.addEventListener).not.toHaveBeenCalled()

      simulador.iniciar(tramoDe(['CA-A'], 3), 4)
      await vi.waitFor(() => expect(enViaje).toHaveLength(1))

      expect(ventana.addEventListener).toHaveBeenCalledTimes(1)
      expect(ventana.addEventListener).toHaveBeenCalledWith('beforeunload', expect.any(Function))
      expect(ventana.oyentes.size).toBe(1)
      simulador.detener()
    })

    it('lo que se registra le pide al navegador que pregunte antes de salir', async () => {
      const ventana = ventanaFalsa()
      const { simulador, enViaje } = conLecturaEnViaje(ventana)
      simulador.iniciar(tramoDe(['CA-A'], 3), 4)
      await vi.waitFor(() => expect(enViaje).toHaveLength(1))
      const evento = { preventDefault: vi.fn(), returnValue: undefined as unknown } as unknown as BeforeUnloadEvent

      for (const oyente of ventana.oyentes) oyente(evento)

      expect(evento.preventDefault).toHaveBeenCalledOnce()
      expect(evento.returnValue).toBe('')
      simulador.detener()
    })

    it('en pausa se retira, y al continuar vuelve', async () => {
      const ventana = ventanaFalsa()
      const { simulador, enViaje } = conLecturaEnViaje(ventana)
      simulador.iniciar(tramoDe(['CA-A'], 3), 4)
      await vi.waitFor(() => expect(enViaje).toHaveLength(1))

      simulador.pausar()
      expect(ventana.oyentes.size).toBe(0)
      expect(ventana.removeEventListener).toHaveBeenCalledWith('beforeunload', ventana.addEventListener.mock.calls[0]![1])

      simulador.continuar()
      expect(ventana.oyentes.size).toBe(1)
      simulador.detener()
    })

    it('al detener la corrida se retira', async () => {
      const ventana = ventanaFalsa()
      const { simulador, enViaje } = conLecturaEnViaje(ventana)
      simulador.iniciar(tramoDe(['CA-A'], 3), 4)
      await vi.waitFor(() => expect(enViaje).toHaveLength(1))

      simulador.detener()

      expect(simulador.leer().fase).toBe('detenida')
      expect(ventana.oyentes.size).toBe(0)
    })

    it('al terminar la corrida se retira', async () => {
      const ventana = ventanaFalsa()
      const { simulador } = armar({ ventana })

      simulador.iniciar(tramoDe(['CA-A', 'CA-B'], 3), 4)
      expect(ventana.oyentes.size).toBe(1)
      await hasta(simulador, () => simulador.leer().fase === 'terminada')

      expect(ventana.oyentes.size).toBe(0)
      expect(ventana.addEventListener).toHaveBeenCalledTimes(1)
      expect(ventana.removeEventListener).toHaveBeenCalledTimes(1)
    })

    it('si un pedido falla, la corrida espera y el aviso se retira; al reintentar vuelve', async () => {
      const ventana = ventanaFalsa()
      let fallar = true
      const { simulador } = armar({
        ventana,
        enviar: async (l) => {
          if (fallar) throw new ErrorApi('sin_respuesta', null, null)
          return prediccion(l, 1)
        },
      })

      simulador.iniciar(tramoDe(['CA-A'], 2), 4)
      await hasta(simulador, () => simulador.leer().fase === 'con_error')
      expect(ventana.oyentes.size).toBe(0)

      fallar = false
      simulador.continuar()
      expect(ventana.oyentes.size).toBe(1)
      await hasta(simulador, () => simulador.leer().fase === 'terminada')
      expect(ventana.oyentes.size).toBe(0)
    })

    it('al reiniciar (la sesión terminó) con una corrida en curso también se retira', async () => {
      const ventana = ventanaFalsa()
      const { simulador, enViaje } = conLecturaEnViaje(ventana)
      simulador.iniciar(tramoDe(['CA-A'], 3), 4)
      await vi.waitFor(() => expect(enViaje).toHaveLength(1))

      simulador.reiniciar()

      expect(simulador.leer().fase).toBe('lista')
      expect(ventana.oyentes.size).toBe(0)
    })

    it('un tramo sin lecturas termina sin haber puesto ningún aviso', () => {
      const ventana = ventanaFalsa()
      const { simulador } = armar({ ventana })

      simulador.iniciar({ ...tramoDe(['CA-A'], 1), lecturas: [] }, 4)

      expect(simulador.leer().fase).toBe('terminada')
      expect(ventana.addEventListener).not.toHaveBeenCalled()
    })
  })
})
