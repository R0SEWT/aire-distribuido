import { describe, expect, it } from 'vitest'
import { validar } from './formulario'

describe('la validación del formulario de entrenamiento', () => {
  it('con tres enteros válidos arma el pedido con números, no con textos', () => {
    expect(validar({ nodos: ' 3 ', replicas: '300', semilla: '-7' })).toEqual({
      pedido: { nodos: 3, replicas: 300, semilla: -7 },
      errores: null,
    })
  })

  it.each([
    ['', 'Falta el número de nodos.'],
    ['dos', 'Tiene que ser un número entero, sin decimales.'],
    ['2.5', 'Tiene que ser un número entero, sin decimales.'],
    ['1e3', 'Tiene que ser un número entero, sin decimales.'],
    ['0', 'Tiene que ser 1 o más.'],
    ['-1', 'Tiene que ser 1 o más.'],
  ])('nodos «%s»: %s', (nodos, mensaje) => {
    const r = validar({ nodos, replicas: '1', semilla: '42' })

    expect(r.pedido).toBeNull()
    expect(r.errores).toEqual({ nodos: mensaje })
  })

  it('las réplicas siguen la misma regla que los nodos', () => {
    expect(validar({ nodos: '1', replicas: '0', semilla: '42' }).errores).toEqual({ replicas: 'Tiene que ser 1 o más.' })
    expect(validar({ nodos: '1', replicas: '', semilla: '42' }).errores).toEqual({ replicas: 'Falta el número de réplicas.' })
  })

  it('la semilla puede ser cero o negativa, pero tiene que ser un entero que el navegador represente', () => {
    expect(validar({ nodos: '1', replicas: '1', semilla: '0' }).pedido).toEqual({ nodos: 1, replicas: 1, semilla: 0 })
    expect(validar({ nodos: '1', replicas: '1', semilla: '' }).errores).toEqual({ semilla: 'Falta la semilla.' })
    expect(validar({ nodos: '1', replicas: '1', semilla: 'abc' }).errores?.semilla).toMatch(/número entero/)
    expect(validar({ nodos: '1', replicas: '1', semilla: '9223372036854775807' }).errores?.semilla).toMatch(/demasiado grande/)
  })

  it('informa todos los campos con error a la vez', () => {
    expect(Object.keys(validar({ nodos: '', replicas: 'x', semilla: '' }).errores ?? {})).toEqual(['nodos', 'replicas', 'semilla'])
  })
})
