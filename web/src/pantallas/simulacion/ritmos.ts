// El ritmo de una simulación, en horas simuladas por segundo. La espera va
// entre una hora y la siguiente: una hora del escenario son hasta 14 lecturas.

export const RITMOS = [
  { valor: 'lenta', etiqueta: 'Lenta', horasPorSegundo: 1 },
  { valor: 'normal', etiqueta: 'Normal', horasPorSegundo: 4 },
  { valor: 'rapida', etiqueta: 'Rápida', horasPorSegundo: 12 },
] as const

export type Ritmo = (typeof RITMOS)[number]['valor']

export function horasPorSegundoDe(ritmo: Ritmo): number {
  return RITMOS.find((r) => r.valor === ritmo)!.horasPorSegundo
}

export function ritmoDe(horasPorSegundo: number): Ritmo {
  return RITMOS.find((r) => r.horasPorSegundo === horasPorSegundo)?.valor ?? 'normal'
}
