// Fechas que componentes/formato.ts no cubre: un día sin hora («2021-12-14»,
// que no es un instante: no se pasa por la zona de Lima) y las horas de un tramo.

const LOCAL = 'es-PE'
const ZONA = 'America/Lima'
const HORA_MS = 3_600_000

function comoFecha(dia: string): Date | null {
  const fecha = new Date(`${dia}T00:00:00Z`)
  return Number.isNaN(fecha.getTime()) ? null : fecha
}

/** «14 dic. 2021» para un día escrito como 2021-12-14, con el mismo formato que fechaHora(). */
export function fechaDia(dia: string | null | undefined): string {
  const fecha = dia ? comoFecha(dia) : null
  if (!fecha) return '—'
  return new Intl.DateTimeFormat(LOCAL, { timeZone: 'UTC', day: 'numeric', month: 'short', year: 'numeric' }).format(fecha)
}

/** «14 y 15 de diciembre de 2021» si los días son del mismo mes; si no, cada uno con su mes. */
export function diasEnLlano(dias: readonly string[]): string {
  const fechas = dias.map(comoFecha).filter((f): f is Date => f !== null)
  if (fechas.length === 0) return '—'
  const largo = new Intl.DateTimeFormat(LOCAL, { timeZone: 'UTC', day: 'numeric', month: 'long', year: 'numeric' })
  const primero = fechas[0]!
  const mismoMes = fechas.every(
    (f) => f.getUTCMonth() === primero.getUTCMonth() && f.getUTCFullYear() === primero.getUTCFullYear(),
  )
  if (!mismoMes || fechas.length === 1) return enumerar(fechas.map((f) => largo.format(f)))
  const mesYAnio = new Intl.DateTimeFormat(LOCAL, { timeZone: 'UTC', month: 'long', year: 'numeric' }).format(primero)
  return `${enumerar(fechas.map((f) => String(f.getUTCDate())))} de ${mesYAnio}`
}

/** «a, b y c». */
export function enumerar(partes: readonly string[]): string {
  if (partes.length <= 1) return partes.join('')
  return `${partes.slice(0, -1).join(', ')} y ${partes[partes.length - 1]}`
}

/** «14 dic.», en hora de Lima: el día de un instante, para el eje de un gráfico. */
export function diaCorto(iso: string | number): string {
  return new Intl.DateTimeFormat(LOCAL, { timeZone: ZONA, day: 'numeric', month: 'short' }).format(new Date(iso))
}

/** «14 dic. 2021», en hora de Lima: el día de un instante. */
export function diaDeInstante(iso: string | number): string {
  return new Intl.DateTimeFormat(LOCAL, { timeZone: ZONA, day: 'numeric', month: 'short', year: 'numeric' }).format(new Date(iso))
}

/** La hora (0 a 23) de un instante en Lima. */
export function horaDeLima(iso: string | number): number {
  // Lima no tiene horario de verano: siempre UTC−5.
  return new Date(new Date(iso).getTime() - 5 * HORA_MS).getUTCHours()
}

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
