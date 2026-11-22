// Los formatos de toda la app: cada uno con una sola implementación, probada aquí.

import { describe, expect, it } from 'vitest'
import {
  compacto,
  diaCorto,
  diaDeInstante,
  diasEnLlano,
  duracion,
  enumerar,
  fechaConSegundos,
  fechaDia,
  fechaHora,
  hora,
  horaConSegundos,
  horaDeLima,
  lapso,
  milisegundos,
  nombrePropio,
  numero,
  porciento,
  proporcion,
} from './formato'

describe('los números', () => {
  it('numero: con sus decimales fijos, y una raya cuando no hay', () => {
    expect(numero(58.349, 1)).toBe('58.3')
    expect(numero(1536)).toBe('1,536')
    expect(numero(120, 1)).toBe('120.0')
    expect(numero(null)).toBe('—')
    expect(numero(undefined)).toBe('—')
    expect(numero(Number.NaN)).toBe('—')
    expect(numero(Number.POSITIVE_INFINITY)).toBe('—')
  })

  it('compacto: un número grande en pocas letras', () => {
    expect(compacto(950)).toBe('950')
    expect(compacto(73_000)).toMatch(/^73\s?(mil|k)$/i)
    expect(compacto(21_900_000)).toMatch(/^21\.9\s?M$/)
  })

  it('milisegundos: las cifras que hacen falta, sin la unidad', () => {
    expect(milisegundos(0.27)).toBe('0.27')
    expect(milisegundos(3.44)).toBe('3.4')
    expect(milisegundos(38.2)).toBe('38')
    expect(milisegundos(1250)).toBe('1,250')
  })

  it('porciento: una parte de un total; lo poco no se escribe como cero', () => {
    expect(porciento(18, 1000)).toBe('1.8 %')
    expect(porciento(1, 3, 0)).toBe('33 %')
    expect(porciento(0, 1000)).toBe('0.0 %')
    expect(porciento(20, 64_748)).toBe('menos de 0.1 %')
    expect(porciento(1, 1000, 0)).toBe('menos de 1 %')
    expect(porciento(5, 0)).toBe('—')
  })

  it('proporcion: de un valor entre 0 y 1', () => {
    expect(proporcion(0.35)).toBe('35.0 %')
    expect(proporcion(0.4375, 0)).toBe('44 %')
  })
})

describe('los instantes, en la hora de Lima esté donde esté el navegador', () => {
  // 13 de diciembre de 2021, 20:00:07 UTC: las 15:00:07 en Lima.
  const INSTANTE = '2021-12-13T20:00:07Z'

  it('fechaHora y hora', () => {
    expect(fechaHora(INSTANTE)).toMatch(/^13 dic\.? 2021,? 15:00$/)
    expect(hora(INSTANTE)).toBe('15:00')
    expect(hora(Date.parse(INSTANTE))).toBe('15:00')
    expect(hora(new Date(INSTANTE))).toBe('15:00')
  })

  it('con segundos, para lo que pasa varias veces por minuto', () => {
    expect(horaConSegundos(INSTANTE)).toBe('15:00:07')
    expect(fechaConSegundos(INSTANTE)).toMatch(/^13 dic\.?,? 15:00:07$/)
  })

  it('el día de un instante es el de Lima, no el de UTC', () => {
    // Las 03:00 UTC del 14 son las 22:00 del 13 en Lima.
    expect(diaCorto('2021-12-14T03:00:00Z')).toMatch(/^13 dic\.?$/)
    expect(diaDeInstante('2021-12-14T03:00:00Z')).toMatch(/^13 dic\.? 2021$/)
    expect(horaDeLima('2021-12-14T03:00:00Z')).toBe(22)
    expect(horaDeLima('2021-12-14T00:00:00-05:00')).toBe(0)
  })

  it('lo que no es una fecha da una raya', () => {
    for (const formato of [fechaHora, hora, horaConSegundos, fechaConSegundos, diaCorto, diaDeInstante]) {
      expect(formato('no es una fecha')).toBe('—')
    }
  })
})

describe('los días sin hora', () => {
  it('fechaDia: no se corre de día por la zona horaria', () => {
    expect(fechaDia('2021-12-10')).toMatch(/^10 dic\.? 2021$/)
    expect(fechaDia('2022-01-01')).toMatch(/^1 ene\.? 2022$/)
    expect(fechaDia('no es una fecha')).toBe('—')
    expect(fechaDia(null)).toBe('—')
    expect(fechaDia(undefined)).toBe('—')
  })

  it('fechaDia y fechaHora escriben el día igual', () => {
    const dia = fechaDia('2021-12-13')
    expect(fechaHora('2021-12-13T20:00:00Z').startsWith(dia)).toBe(true)
  })

  it('diasEnLlano: los días de un mismo mes comparten el mes', () => {
    expect(diasEnLlano(['2021-12-14', '2021-12-15'])).toBe('14 y 15 de diciembre de 2021')
    expect(diasEnLlano(['2021-12-14', '2021-12-15', '2021-12-16'])).toBe('14, 15 y 16 de diciembre de 2021')
    expect(diasEnLlano(['2021-12-14'])).toBe('14 de diciembre de 2021')
    expect(diasEnLlano(['2021-12-31', '2022-01-01'])).toBe('31 de diciembre de 2021 y 1 de enero de 2022')
    expect(diasEnLlano([])).toBe('—')
    expect(diasEnLlano(['no es una fecha'])).toBe('—')
  })
})

describe('los lapsos', () => {
  it('lapso: cuánto hace de algo, a partir de sus segundos', () => {
    expect(lapso(45)).toBe('45 s')
    expect(lapso(200)).toBe('3 min 20 s')
    expect(lapso(8040)).toBe('2 h 14 min')
    expect(lapso(-3)).toBe('0 s')
    expect(lapso(59.9)).toBe('59 s')
  })

  it('duracion: elige la unidad', () => {
    expect(duracion(0.27)).toBe('0.27 ms')
    expect(duracion(56.4)).toBe('56 ms')
    expect(duracion(1887)).toBe('1.9 s')
    expect(duracion(12_400)).toBe('12 s')
  })
})

describe('las palabras', () => {
  it('enumerar: «a, b y c»', () => {
    expect(enumerar([])).toBe('')
    expect(enumerar(['CA-HU-04'])).toBe('CA-HU-04')
    expect(enumerar(['CA-HU-04', 'CA-CG-02'])).toBe('CA-HU-04 y CA-CG-02')
    expect(enumerar(['a', 'b', 'c'])).toBe('a, b y c')
  })

  it('nombrePropio: los nombres de la ficha, que vienen en mayúsculas, se escriben como nombres', () => {
    expect(nombrePropio('LA LIBERTAD')).toBe('La Libertad')
    expect(nombrePropio('MADRE DE DIOS')).toBe('Madre de Dios')
    expect(nombrePropio('SAN JUAN DE LURIGANCHO')).toBe('San Juan de Lurigancho')
    expect(nombrePropio('PARIÑAS')).toBe('Pariñas')
    expect(nombrePropio('ÁNCASH')).toBe('Áncash')
    expect(nombrePropio('CASTILLA-LA NUEVA')).toBe('Castilla-La Nueva')
    expect(nombrePropio('  SAN   MARCOS ')).toBe('San Marcos')
  })

  it('nombrePropio: sin nombre, null (quien lo usa decide qué decir)', () => {
    expect(nombrePropio(null)).toBeNull()
    expect(nombrePropio(undefined)).toBeNull()
    expect(nombrePropio('')).toBeNull()
    expect(nombrePropio('   ')).toBeNull()
  })
})
