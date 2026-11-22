// Formatos propios de la pantalla del clúster. Los eventos se guardan al
// milisegundo y varios caen en el mismo segundo: aquí la hora lleva segundos
// (componentes/formato.ts llega al minuto).

const LOCAL = 'es-PE'
const ZONA = 'America/Lima'

/** «21 nov, 17:17:15», en la hora de Lima. */
export function fechaConSegundos(iso: string | number | Date): string {
  const fecha = new Date(iso)
  if (Number.isNaN(fecha.getTime())) return '—'
  return new Intl.DateTimeFormat(LOCAL, {
    timeZone: ZONA,
    day: 'numeric',
    month: 'short',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  }).format(fecha)
}

/** «17:17:15», en la hora de Lima. */
export function horaConSegundos(iso: string | number | Date): string {
  const fecha = new Date(iso)
  if (Number.isNaN(fecha.getTime())) return '—'
  return new Intl.DateTimeFormat(LOCAL, {
    timeZone: ZONA,
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  }).format(fecha)
}

/** «2 h 14 min», «3 min 20 s», «45 s»: cuánto lleva en marcha la API. */
export function lapso(segundos: number): string {
  const s = Math.max(0, Math.floor(segundos))
  const h = Math.floor(s / 3600)
  const min = Math.floor((s % 3600) / 60)
  if (h > 0) return `${h} h ${min} min`
  if (min > 0) return `${min} min ${s % 60} s`
  return `${s} s`
}
