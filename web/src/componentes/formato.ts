// Cómo se escriben los números y las fechas en toda la app. Las fechas van en
// la hora de Lima, que es la de los datos de OEFA, esté donde esté el navegador.

const LOCAL = 'es-PE'
const ZONA = 'America/Lima'

/** Un número con sus decimales fijos: 121.4, 1,536. null o NaN dan «—». */
export function numero(valor: number | null | undefined, decimales = 0): string {
  if (valor === null || valor === undefined || !Number.isFinite(valor)) return '—'
  return new Intl.NumberFormat(LOCAL, { minimumFractionDigits: decimales, maximumFractionDigits: decimales }).format(
    valor,
  )
}

/** «14 nov 2026, 15:00»: la fecha y la hora de un estado. */
export function fechaHora(iso: string | number | Date): string {
  const fecha = new Date(iso)
  if (Number.isNaN(fecha.getTime())) return '—'
  return new Intl.DateTimeFormat(LOCAL, {
    timeZone: ZONA,
    day: 'numeric',
    month: 'short',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(fecha)
}

/** «15:30»: solo la hora. */
export function hora(iso: string | number | Date): string {
  const fecha = new Date(iso)
  if (Number.isNaN(fecha.getTime())) return '—'
  return new Intl.DateTimeFormat(LOCAL, { timeZone: ZONA, hour: '2-digit', minute: '2-digit', hour12: false }).format(
    fecha,
  )
}
