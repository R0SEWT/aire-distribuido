// Cómo se escriben los números, las fechas y los nombres en toda la app: una
// sola implementación de cada formato. Las fechas van en la hora de Lima, que
// es la de los datos de OEFA, esté donde esté el navegador.

const LOCAL = 'es-PE'
const ZONA = 'America/Lima'
const HORA_MS = 3_600_000

type Instante = string | number | Date

function formatoDeFecha(opciones: Intl.DateTimeFormatOptions, zona = ZONA) {
  const formato = new Intl.DateTimeFormat(LOCAL, { timeZone: zona, hour12: false, ...opciones })
  return (instante: Instante): string => {
    const fecha = new Date(instante)
    return Number.isNaN(fecha.getTime()) ? '—' : formato.format(fecha)
  }
}

// ── Números ────────────────────────────────────────────────────────────────

/** Un número con sus decimales fijos: 121.4, 1,536. null o NaN dan «—». */
export function numero(valor: number | null | undefined, decimales = 0): string {
  if (valor === null || valor === undefined || !Number.isFinite(valor)) return '—'
  return new Intl.NumberFormat(LOCAL, { minimumFractionDigits: decimales, maximumFractionDigits: decimales }).format(
    valor,
  )
}

/** Un número grande en pocas letras, para un eje: 73 mil, 21.9 M. */
export function compacto(valor: number): string {
  return new Intl.NumberFormat(LOCAL, { notation: 'compact', maximumFractionDigits: 1 }).format(valor)
}

/** Milisegundos con las cifras que hacen falta, sin la unidad: 0.27, 3.4, 38, 1,250. */
export function milisegundos(ms: number): string {
  return numero(ms, ms < 1 ? 2 : ms < 10 ? 1 : 0)
}

/** «1.8 %»: una parte de un total, con un decimal. Sin total, «—». */
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

// ── Instantes, en la hora de Lima ──────────────────────────────────────────

/** «14 nov 2026, 15:00»: la fecha y la hora de un estado. */
export const fechaHora = formatoDeFecha({ day: 'numeric', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit' })

/** «15:30»: solo la hora. */
export const hora = formatoDeFecha({ hour: '2-digit', minute: '2-digit' })

/** «17:17:15»: la hora con segundos, para lo que pasa varias veces por minuto. */
export const horaConSegundos = formatoDeFecha({ hour: '2-digit', minute: '2-digit', second: '2-digit' })

/** «21 nov, 17:17:15»: el día y la hora con segundos, sin el año. */
export const fechaConSegundos = formatoDeFecha({
  day: 'numeric',
  month: 'short',
  hour: '2-digit',
  minute: '2-digit',
  second: '2-digit',
})

/** «14 dic»: el día de un instante, para el eje de un gráfico. */
export const diaCorto = formatoDeFecha({ day: 'numeric', month: 'short' })

/** «14 dic 2021»: el día de un instante. */
export const diaDeInstante = formatoDeFecha({ day: 'numeric', month: 'short', year: 'numeric' })

/** La hora (0 a 23) de un instante en Lima. */
export function horaDeLima(instante: Instante): number {
  // Lima no tiene horario de verano: siempre UTC−5.
  return new Date(new Date(instante).getTime() - 5 * HORA_MS).getUTCHours()
}

// ── Días sin hora ──────────────────────────────────────────────────────────
// «2021-12-14» no es un instante: no se pasa por la zona de Lima.

function comoDia(dia: string | null | undefined): Date | null {
  if (!dia) return null
  const fecha = new Date(`${dia}T00:00:00Z`)
  return Number.isNaN(fecha.getTime()) ? null : fecha
}

const DIA_SIN_HORA = formatoDeFecha({ day: 'numeric', month: 'short', year: 'numeric' }, 'UTC')

/** «14 dic 2021» para un día escrito como 2021-12-14, con el mismo formato que fechaHora(). */
export function fechaDia(dia: string | null | undefined): string {
  const fecha = comoDia(dia)
  return fecha ? DIA_SIN_HORA(fecha) : '—'
}

/** «14 y 15 de diciembre de 2021» si los días son del mismo mes; si no, cada uno con su mes. */
export function diasEnLlano(dias: readonly string[]): string {
  const fechas = dias.map(comoDia).filter((f): f is Date => f !== null)
  if (fechas.length === 0) return '—'
  const largo = formatoDeFecha({ day: 'numeric', month: 'long', year: 'numeric' }, 'UTC')
  const primero = fechas[0]!
  const mismoMes = fechas.every(
    (f) => f.getUTCMonth() === primero.getUTCMonth() && f.getUTCFullYear() === primero.getUTCFullYear(),
  )
  if (!mismoMes || fechas.length === 1) return enumerar(fechas.map(largo))
  const mesYAnio = formatoDeFecha({ month: 'long', year: 'numeric' }, 'UTC')(primero)
  return `${enumerar(fechas.map((f) => String(f.getUTCDate())))} de ${mesYAnio}`
}

// ── Lapsos ─────────────────────────────────────────────────────────────────

/** «2 h 14 min», «3 min 20 s», «45 s»: cuánto hace de algo, a partir de sus segundos. */
export function lapso(segundos: number): string {
  const s = Math.max(0, Math.floor(segundos))
  const h = Math.floor(s / 3600)
  const min = Math.floor((s % 3600) / 60)
  if (h > 0) return `${h} h ${min} min`
  if (min > 0) return `${min} min ${s % 60} s`
  return `${s} s`
}

/** Una duración en la unidad que se lee mejor: «56 ms», «1.9 s», «12 s». */
export function duracion(ms: number): string {
  if (ms < 1000) return `${milisegundos(ms)} ms`
  return `${numero(ms / 1000, ms < 10_000 ? 1 : 0)} s`
}

// ── Palabras ───────────────────────────────────────────────────────────────

/** «a, b y c». */
export function enumerar(partes: readonly string[]): string {
  if (partes.length <= 1) return partes.join('')
  return `${partes.slice(0, -1).join(', ')} y ${partes[partes.length - 1]}`
}

const PALABRAS_MENORES = new Set(['de', 'del', 'la', 'las', 'los', 'y'])

/**
 * «Madre de Dios» a partir de «MADRE DE DIOS»: la ficha de OEFA trae los
 * lugares en mayúsculas. Las preposiciones y los artículos quedan en minúscula,
 * salvo al empezar («La Libertad»). null si no hay nombre.
 */
export function nombrePropio(nombre: string | null | undefined): string | null {
  if (!nombre?.trim()) return null
  return nombre
    .trim()
    .toLocaleLowerCase('es')
    .split(/\s+/)
    .map((palabra, i) =>
      i > 0 && PALABRAS_MENORES.has(palabra)
        ? palabra
        : // Cada parte de un nombre con guion lleva su mayúscula.
          palabra.replace(/(^|-)\p{L}/gu, (inicial) => inicial.toLocaleUpperCase('es')),
    )
    .join(' ')
}
