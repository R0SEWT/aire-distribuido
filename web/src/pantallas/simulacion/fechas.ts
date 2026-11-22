// Las cuentas con horas que hace la simulación. Cómo se escribe una fecha está
// en componentes/formato.ts.

const HORA_MS = 3_600_000

/** Los instantes de cada hora entre dos fechas, las dos incluidas. */
export function horasEntre(desde: string, hasta: string): number[] {
  const a = Date.parse(desde)
  const b = Date.parse(hasta)
  if (!Number.isFinite(a) || !Number.isFinite(b) || b < a) return []
  const horas: number[] = []
  for (let t = a; t <= b; t += HORA_MS) horas.push(t)
  return horas
}

/** La hora siguiente a un instante, en RFC 3339 con la zona de Lima (−05:00). */
export function horaSiguiente(iso: string): string {
  const t = new Date(Date.parse(iso) + HORA_MS - 5 * HORA_MS)
  const dos = (n: number) => String(n).padStart(2, '0')
  return `${t.getUTCFullYear()}-${dos(t.getUTCMonth() + 1)}-${dos(t.getUTCDate())}T${dos(t.getUTCHours())}:00:00-05:00`
}
