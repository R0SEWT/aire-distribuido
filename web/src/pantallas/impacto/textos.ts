// Las palabras propias de Impacto. Los formatos generales (porcentajes, nombres
// de lugares) están en componentes/formato.ts.

import { numero } from '../../componentes/formato'

/** Con menos días con datos que estos, una estación es una campaña de pocas semanas: se marca en su fila. */
export const DIAS_DE_CAMPANA_CORTA = 60

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
