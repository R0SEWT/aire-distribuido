// Cómo se escriben las cifras y los nombres en Impacto.

import { numero } from '../../componentes/formato'

/** Con menos días con datos que estos, una estación es una campaña de pocas semanas: se marca en su fila. */
export const DIAS_DE_CAMPANA_CORTA = 60

/** «1.8 %»: una parte de un total, con un decimal. */
export function porciento(parte: number, total: number, decimales = 1): string {
  if (total <= 0) return '—'
  const valor = (100 * parte) / total
  // Pocas horas no son cero: 20 de 64,748 no se escribe «0.0 %».
  const minimo = 10 ** -decimales
  if (parte > 0 && valor < minimo) return `menos de ${numero(minimo, decimales)} %`
  return `${numero(valor, decimales)} %`
}

/** «35.0 %» para una proporción entre 0 y 1. */
export function proporcion(valor: number, decimales = 1): string {
  return `${numero(valor * 100, decimales)} %`
}

/** «San Marcos» para «SAN MARCOS»: la ficha trae los lugares en mayúsculas. */
export function lugar(nombre: string | null | undefined): string {
  if (!nombre) return ''
  return nombre.toLowerCase().replace(/(^|[\s-])\p{L}/gu, (l) => l.toUpperCase())
}

/** Una mediana: entera si lo es, con un decimal si no (0.5). */
export function horas(valor: number | null | undefined): string {
  if (valor === null || valor === undefined) return '—'
  return numero(valor, Number.isInteger(valor) ? 0 : 1)
}

/** «1 hora», «20 horas». */
export function enHoras(valor: number): string {
  return `${horas(valor)} ${valor === 1 ? 'hora' : 'horas'}`
}

/** «una», «dos», «tres»… para contar pocas cosas dentro de una frase. */
export function enLetras(n: number): string {
  return ['cero', 'una', 'dos', 'tres', 'cuatro', 'cinco', 'seis', 'siete', 'ocho', 'nueve'][n] ?? String(n)
}
