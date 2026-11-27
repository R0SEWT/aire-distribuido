// Qué dibuja la curva de la Estación (spec §5.3) y con qué palabras lo dice.
//
// Hay dos fuentes, las dos de esta sesión del navegador:
//  - los estados recibidos (api/historia.ts): la media de 24 horas de cada
//    estado que llegó por un GET o por el evento `estacion.regimen`. El estado
//    no trae la lectura horaria.
//  - las lecturas enviadas (api/vistas.ts): las que esta sesión mandó desde
//    Simulación, con la media que la API respondió a cada una.
//
// La curva dibuja la media de las dos (una línea) y, cuando las hay, las
// lecturas horarias (rombos sueltos). Aquí no hay React: se prueba frase por frase.

import type { PuntoSesion } from '../../api/historia'
import type { LecturaVista } from '../../api/vistas'
import { fechaHora } from '../../componentes/formato'
import type { PuntoCurva } from '../../graficos/CurvaSesion'
import { contarHuecos, HUECO_QUE_CORTA_H } from '../../graficos/tramos'
import { MINIMO_LECTURAS } from '../panel/situacion'
import { NORMA, type Contaminante } from './norma'

const HORA_MS = 3_600_000
/** Las lecturas son horarias: dos instantes de la misma hora son la misma lectura. */
const horaDe = (t: number) => Math.floor(t / HORA_MS)

export interface Curva {
  /** La media de 24 horas, hora por hora, de la más antigua a la más nueva; null donde no la hubo. */
  medias: PuntoCurva[]
  /** Las lecturas horarias que esta sesión envió. Vacío si no envió ninguna. */
  lecturas: { t: number; valor: number }[]
  /** Cuántos estados recibió la pestaña. */
  estados: number
  /** Cuántas de las medias son de estados cuya lectura no envió esta sesión. */
  estadosAjenos: number
  /** Cuántos puntos de la media no tienen valor: la ventana tenía menos de 18 lecturas. */
  sinMedia: number
  /** En cuántos lugares la línea de la media se corta porque faltan horas entre dos puntos seguidos. */
  huecos: number
  /** El instante del último estado recibido y el de la última lectura enviada; null si no hay. */
  ultimoEstado: number | null
  ultimaLectura: number | null
  /** Si no hay nada que dibujar: ni estados ni lecturas. */
  vacia: boolean
}

/**
 * Junta lo recibido y lo enviado de una estación. En una misma hora manda el
 * estado recibido: es el que muestra la tarjeta de arriba.
 */
export function armarCurva(historia: readonly PuntoSesion[], vistas: readonly LecturaVista[], contaminante: Contaminante): Curva {
  const medias = new Map<number, PuntoCurva>()
  const lecturas: Curva['lecturas'] = []
  const enviadas = new Set<number>()
  for (const v of vistas) {
    const t = Date.parse(v.fecha_hora)
    if (!Number.isFinite(t)) continue
    enviadas.add(horaDe(t))
    lecturas.push({ t, valor: v[contaminante] })
    medias.set(horaDe(t), { t, valor: contaminante === 'pm2_5' ? v.media_pm2_5 : v.media_pm10 })
  }
  let estadosAjenos = 0
  for (const p of historia) {
    if (!enviadas.has(horaDe(p.t))) estadosAjenos++
    medias.set(horaDe(p.t), { t: p.t, valor: p[contaminante] })
  }
  const enOrden = [...medias.values()].sort((a, b) => a.t - b.t)
  lecturas.sort((a, b) => a.t - b.t)
  return {
    medias: enOrden,
    lecturas,
    estados: historia.length,
    estadosAjenos,
    sinMedia: enOrden.filter((m) => m.valor === null).length,
    huecos: contarHuecos(enOrden),
    ultimoEstado: historia[historia.length - 1]?.t ?? null,
    ultimaLectura: lecturas[lecturas.length - 1]?.t ?? null,
    vacia: enOrden.length === 0 && lecturas.length === 0,
  }
}

const cuantos = (n: number, uno: string, varios: string) => `${n} ${n === 1 ? uno : varios}`

export interface TextosDeCurva {
  /** El rótulo y el título de la tarjeta. */
  rotulo: string
  titulo: string
  /** Qué se ve, para quien no ve el gráfico (aria-label). */
  descripcion: string
  /** Lo que va en negrita al empezar el pie: cuánto hay. */
  cuenta: string
  /** El resto del pie: qué es cada marca y qué no se ve. */
  pie: string
}

/** Lo que la tarjeta de la curva dice, según haya o no lecturas horarias que mostrar. */
export function textosDeCurva(id: string, curva: Curva, contaminante: Contaminante): TextosDeCurva {
  const { nombre, eca } = NORMA[contaminante]
  const ultimo =
    curva.ultimaLectura !== null
      ? `, la última del ${fechaHora(curva.ultimaLectura)}`
      : curva.ultimoEstado !== null
        ? `, el último del ${fechaHora(curva.ultimoEstado)}`
        : ''
  const marco = `La línea cortada es el ECA y las franjas, las bandas INCA de ${nombre}; las horas son las de Lima.`
  const sinMedia =
    curva.sinMedia > 0
      ? ` ${cuantos(curva.sinMedia, 'hora quedó', 'horas quedaron')} sin media, porque la ventana tenía menos de ${MINIMO_LECTURAS} lecturas (círculo hueco, abajo).`
      : ''

  const huecos =
    curva.huecos > 0
      ? ` La línea se corta en ${cuantos(curva.huecos, 'lugar', 'lugares')} donde faltan más de ${HUECO_QUE_CORTA_H} horas: son tramos distintos, y la media de 24 horas no es continua de uno al otro.`
      : ''

  if (curva.lecturas.length === 0) {
    const cuenta = cuantos(curva.estados, 'estado recibido', 'estados recibidos')
    return {
      rotulo: 'Lo recibido en esta sesión',
      titulo: `Media de 24 horas de ${nombre}`,
      descripcion: `Media de 24 horas de ${nombre} de ${id}: ${cuenta} en esta sesión${ultimo}. ECA: ${eca} µg/m³.`,
      cuenta,
      pie:
        ` por esta pestaña${ultimo}. Cada punto de la línea es la media de 24 horas de un estado.${sinMedia}${huecos}` +
        ` Las lecturas horarias no se ven: el estado no las trae, y esta sesión no envió ninguna de ${id} desde Simulación.` +
        ` ${marco} No es una serie histórica: la API no guarda una, y lo anterior a esta sesión no se ve.`,
    }
  }

  const cuenta = cuantos(curva.lecturas.length, 'lectura horaria', 'lecturas horarias')
  const ajenos =
    curva.estadosAjenos > 0
      ? ` ${cuantos(curva.estadosAjenos, 'punto de la línea es', 'puntos de la línea son')} de estados recibidos cuya lectura no envió esta sesión: de esas horas se ve la media, no la lectura.`
      : ''
  return {
    rotulo: 'Lo enviado y lo recibido en esta sesión',
    titulo: `Lecturas horarias y media de 24 horas de ${nombre}`,
    descripcion:
      `Lecturas horarias y media de 24 horas de ${nombre} de ${id}: ${cuenta} enviadas en esta sesión${ultimo}, ` +
      `y la media de 24 horas de ${cuantos(curva.medias.length - curva.sinMedia, 'hora', 'horas')}. ECA: ${eca} µg/m³.`,
    cuenta,
    pie:
      ` de ${nombre} que esta sesión envió desde Simulación${ultimo}. Cada rombo es una de esas lecturas; la línea, la media de 24 horas` +
      ` que la API respondió a cada una y la de los estados recibidos.${ajenos}${sinMedia}${huecos}` +
      ` El ECA es de 24 horas: se compara con la media, no con cada lectura. ${marco}` +
      ' No es una serie histórica: la API no guarda una. Las lecturas enviadas se olvidan al recargar la página; los estados recibidos, al cerrar la pestaña.',
  }
}
