import { describe, expect, it } from 'vitest'
import type { Corrida, Ronda } from './seguimiento'
import { compuerta, describirAvance, diferencia, nodosNombrados } from './textos'

function rondas(n: number, ms = 0.4): Ronda[] {
  return Array.from({ length: n }, (_, i) => ({ iter: i + 1, inercia: 100 - i, ms_ronda: ms, ms_fusion: 0.01, ms_por_nodo: { a: ms } }))
}

function corrida(extra: Partial<Corrida>): Corrida {
  return { id: 'e-1', origen: 'propia', pedido: null, rondas: [], fin: null, finEn: null, ...extra }
}

describe('lo que la pantalla dice del avance', () => {
  it('aceptado y sin rondas: los nodos están cargando', () => {
    const a = describirAvance(corrida({}), 0, 120)

    expect(a.fase).toBe('cargando')
    expect(a.frase).toMatch(/todavía no llegó ninguna ronda/)
  })

  it('con rondas por mostrar de un entrenamiento que ya terminó, dice que es una reproducción y cuánto más lenta', () => {
    const c = corrida({ rondas: rondas(44), fin: { abortado: false, iteraciones: 44, ms_total: 56 } })

    const a = describirAvance(c, 12, 120)

    expect(a.fase).toBe('reproduciendo')
    expect(a.etiqueta).toBe('Reproduciendo · ronda 12 de 44')
    expect(a.frase).toContain('El entrenamiento real duró 56 ms')
    expect(a.frase).toContain('Lo que ves es una reproducción: cada ronda se muestra 120 ms, unas 300 veces más de lo que tardó.')
  })

  it('al terminar la reproducción lo sigue diciendo, en pasado', () => {
    const c = corrida({ rondas: rondas(44), fin: { abortado: false, iteraciones: 44, ms_total: 56 } })

    const a = describirAvance(c, 44, 120)

    expect(a.fase).toBe('terminado')
    expect(a.etiqueta).toBe('Terminado · 44 rondas')
    expect(a.frase).toMatch(/Lo que viste fue una reproducción/)
  })

  it('si las rondas tardan más que el paso (una medición larga), no dice que las frena', () => {
    const c = corrida({ rondas: rondas(5, 140) })

    const a = describirAvance(c, 5, 120)

    expect(a.fase).toBe('en_vivo')
    expect(a.etiqueta).toBe('En vivo · ronda 5')
    expect(a.frase).toMatch(/Cada ronda está tardando 140 ms de verdad/)
  })

  it('en vivo con una sola ronda, en singular', () => {
    expect(describirAvance(corrida({ rondas: rondas(1, 245) }), 1, 120).frase).toMatch(/ya se mostró la ronda que llegó/)
  })

  it('abortado: dice en qué ronda y cuántas se completaron', () => {
    const c = corrida({ rondas: rondas(24), fin: { abortado: true, error: 'x' } })

    const a = describirAvance(c, 24, 120)

    expect(a.fase).toBe('abortado')
    expect(a.tono).toBe('alerta')
    expect(a.etiqueta).toBe('Abortado en la ronda 25')
    expect(a.frase).toMatch(/se muestran las 24 rondas que llegaron a completarse/)
  })

  it('abortado sin ninguna ronda', () => {
    const a = describirAvance(corrida({ fin: { abortado: true, error: 'x' } }), 0, 120)

    expect(a.etiqueta).toBe('Abortado antes de la primera ronda')
  })
})

describe('la compuerta del modelo', () => {
  it('con modelo_id, quedó activo y dice por qué pasó', () => {
    const c = compuerta({ replicas: 1, modelo_id: 'entrenado-e-1', dif_secuencial: 7.5e-15 }, 'entrenado-e-1')

    expect(c).toMatchObject({ tono: 'buena', titulo: 'El modelo nuevo quedó activo' })
    expect(c?.detalle).toContain('7.5 e-15')
    expect(c?.detalle).toContain('dentro de la tolerancia')
  })

  it('si después se activó otro, lo dice', () => {
    const c = compuerta({ replicas: 1, modelo_id: 'entrenado-e-1' }, 'entrenado-e-2')

    expect(c?.titulo).toBe('Activó su modelo (después se activó otro)')
    expect(c?.detalle).toContain('entrenado-e-2')
  })

  it('con aviso, no se activó, y van las palabras de la API', () => {
    const aviso = 'el modelo no se activó y sigue el anterior: dif_secuencial 0.5 supera la tolerancia 1e-09'

    expect(compuerta({ replicas: 1, aviso }, 'sembrado')).toEqual({
      tono: 'alerta',
      titulo: 'El modelo no se activó',
      detalle: `La API dice: «${aviso}».`,
    })
  })

  it('con réplicas es una medición y no cambia el modelo', () => {
    expect(compuerta({ replicas: 300 }, 'sembrado')?.titulo).toBe('Fue una medición: el modelo activo no cambió')
  })

  it('sin saber las réplicas ni el modelo, no afirma nada', () => {
    expect(compuerta({}, 'sembrado')).toBeNull()
  })
})

describe('los formatos', () => {
  it('la diferencia con el secuencial va en notación científica', () => {
    expect(diferencia(7.549e-15)).toBe('7.5 e-15')
    expect(diferencia(0)).toBe('0')
    expect(diferencia(undefined)).toBe('—')
  })

  it('encuentra en el error el nodo que falló, por su nombre entero', () => {
    const error = 'ronda 25 abortada: nodo "nodo-1": esperando PARCIAL: leer: connection reset by peer'

    expect(nodosNombrados(error, ['nodo-1', 'nodo-10', 'nodo-2'])).toEqual(['nodo-1'])
    expect(nodosNombrados('ronda 1: el nodo H45-023 no respondió dentro del plazo', ['H45-022', 'H45-023'])).toEqual(['H45-023'])
    expect(nodosNombrados('la API se está apagando', ['nodo-1'])).toEqual([])
  })
})
