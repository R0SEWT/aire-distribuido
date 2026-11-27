import { describe, expect, it } from 'vitest'
import { agregarMuestra, serieDeUso, sinMedicionDeCpu, usoDeCpu, usoPromedio, type Proceso } from './cpu'

function muestra(extra: Partial<Proceso>): Proceso {
  return {
    cpu_usuario_s: 0,
    cpu_sistema_s: 0,
    en_marcha_s: 0,
    nucleos: 8,
    heap_mb: 20,
    sistema_mb: 40,
    max_rss_mb: 60,
    goroutines: 20,
    ...extra,
  }
}

describe('el uso de CPU del proceso de la API', () => {
  it('entre dos muestras es Δ(usuario + sistema) / Δen_marcha', () => {
    const antes = muestra({ cpu_usuario_s: 12.0, cpu_sistema_s: 3.0, en_marcha_s: 100 })
    const ahora = muestra({ cpu_usuario_s: 12.4, cpu_sistema_s: 3.1, en_marcha_s: 102 })

    const uso = usoDeCpu(antes, ahora)

    // 0.5 s de CPU en 2 s de reloj: un cuarto de núcleo.
    expect(uso?.nucleos).toBeCloseTo(0.25)
    expect(uso?.porcentaje).toBeCloseTo(25)
    expect(uso?.intervaloS).toBeCloseTo(2)
  })

  it('con varios núcleos ocupados pasa de 100 %: 100 es UN núcleo', () => {
    const antes = muestra({ cpu_usuario_s: 10, en_marcha_s: 100 })
    const ahora = muestra({ cpu_usuario_s: 16, en_marcha_s: 102 })

    expect(usoDeCpu(antes, ahora)?.porcentaje).toBeCloseTo(300)
  })

  it('el tope es el número de núcleos', () => {
    const antes = muestra({ cpu_usuario_s: 0, en_marcha_s: 100, nucleos: 2 })
    const ahora = muestra({ cpu_usuario_s: 9, en_marcha_s: 101, nucleos: 2 })

    expect(usoDeCpu(antes, ahora)?.nucleos).toBe(2)
  })

  it('sin CPU consumido es cero, no un error', () => {
    const antes = muestra({ cpu_usuario_s: 1, en_marcha_s: 100 })
    const ahora = muestra({ cpu_usuario_s: 1, en_marcha_s: 102 })

    expect(usoDeCpu(antes, ahora)?.porcentaje).toBe(0)
  })

  it('no se calcula con la misma muestra dos veces', () => {
    const m = muestra({ cpu_usuario_s: 1, en_marcha_s: 100 })

    expect(usoDeCpu(m, m)).toBeNull()
  })

  it('no se calcula si la API se reinició entre las dos (el reloj o el CPU retroceden)', () => {
    const antes = muestra({ cpu_usuario_s: 50, en_marcha_s: 3600 })

    expect(usoDeCpu(antes, muestra({ cpu_usuario_s: 0.1, en_marcha_s: 4 }))).toBeNull()
    expect(usoDeCpu(antes, muestra({ cpu_usuario_s: 0.1, en_marcha_s: 3602 }))).toBeNull()
  })

  it('con una sola muestra da el promedio desde el arranque', () => {
    const uso = usoPromedio(muestra({ cpu_usuario_s: 12.48, cpu_sistema_s: 3.105, en_marcha_s: 1824.6 }))

    expect(uso?.porcentaje).toBeCloseTo((15.585 / 1824.6) * 100)
    expect(usoPromedio(muestra({ en_marcha_s: 0 }))).toBeNull()
  })

  it('sabe cuándo la plataforma no mide el CPU', () => {
    expect(sinMedicionDeCpu(muestra({}))).toBe(false)
    expect(sinMedicionDeCpu(muestra({ no_disponible: ['max_rss_mb'] }))).toBe(false)
    expect(sinMedicionDeCpu(muestra({ no_disponible: ['cpu_usuario_s', 'cpu_sistema_s', 'max_rss_mb'] }))).toBe(true)
  })
})

describe('las muestras guardadas', () => {
  it('se suman en orden y se quedan con las últimas', () => {
    let muestras: Proceso[] = []
    for (let t = 1; t <= 5; t++) muestras = agregarMuestra(muestras, muestra({ en_marcha_s: t }), 3)

    expect(muestras.map((m) => m.en_marcha_s)).toEqual([3, 4, 5])
  })

  it('una muestra repetida no se suma', () => {
    const una = [muestra({ en_marcha_s: 10 })]

    expect(agregarMuestra(una, muestra({ en_marcha_s: 10 }))).toBe(una)
  })

  it('si la API se reinició, se empieza de nuevo', () => {
    const viejas = [muestra({ en_marcha_s: 3600 }), muestra({ en_marcha_s: 3602 })]

    expect(agregarMuestra(viejas, muestra({ en_marcha_s: 2 })).map((m) => m.en_marcha_s)).toEqual([2])
  })

  it('la serie tiene un punto por cada par de muestras seguidas', () => {
    const serie = serieDeUso([
      muestra({ cpu_usuario_s: 0, en_marcha_s: 10 }),
      muestra({ cpu_usuario_s: 1, en_marcha_s: 12 }),
      muestra({ cpu_usuario_s: 1, en_marcha_s: 14 }),
    ])

    expect(serie).toEqual([
      { t: 12, porcentaje: 50 },
      { t: 14, porcentaje: 0 },
    ])
  })
})
