// Lo que la pantalla de entrenamiento dice con palabras: en qué va la
// reproducción, cuánto duró de verdad y qué pasó con el modelo. Sin React,
// para probar las frases una por una.

import { duracion, numero, type Tono } from '../../componentes/indice'
import type { Corrida, Fin, Ronda } from './seguimiento'

/** «1 ronda», «44 rondas». */
export function cuantasRondas(n: number): string {
  return n === 1 ? '1 ronda' : `${n} rondas`
}

export type Fase =
  /** La API aceptó el pedido y todavía no llegó ninguna ronda: los nodos cargan su parte del gold. */
  | 'cargando'
  /** Hay rondas recibidas que todavía no se mostraron. */
  | 'reproduciendo'
  /** Se mostró todo lo recibido y el entrenamiento sigue: la próxima ronda se ve cuando llegue. */
  | 'en_vivo'
  | 'terminado'
  | 'abortado'

export function faseDe(corrida: Corrida, mostradas: number): Fase {
  if (mostradas < corrida.rondas.length) return 'reproduciendo'
  if (corrida.fin) return corrida.fin.abortado ? 'abortado' : 'terminado'
  return corrida.rondas.length === 0 ? 'cargando' : 'en_vivo'
}

export interface Avance {
  fase: Fase
  /** La etiqueta corta del estado: «Reproduciendo · ronda 12 de 44». */
  etiqueta: string
  tono: Tono
  /** La frase que dice, sin disimular, qué se está viendo y cuánto tardó de verdad. */
  frase: string
}

function mediaDeRonda(rondas: readonly Ronda[]): number {
  if (rondas.length === 0) return 0
  return rondas.reduce((suma, r) => suma + r.ms_ronda, 0) / rondas.length
}

/** Cuántas veces más lenta es la reproducción que la ronda real: «unas 400 veces», «el doble». */
function vecesMasLenta(pasoMs: number, msReal: number): string | null {
  if (msReal <= 0) return null
  const veces = pasoMs / msReal
  if (veces < 1.5) return null
  if (veces < 10) return `unas ${numero(veces, 0)} veces más de lo que tardó`
  // Con dos cifras significativas alcanza: es un orden de magnitud, no una medida.
  const redondo = Number(veces.toPrecision(veces < 100 ? 1 : 2))
  return `unas ${numero(redondo, 0)} veces más de lo que tardó`
}

/** En qué va la reproducción de una corrida, con sus palabras. */
export function describirAvance(corrida: Corrida, mostradas: number, pasoMs: number): Avance {
  const fase = faseDe(corrida, mostradas)
  const recibidas = corrida.rondas.length
  const media = mediaDeRonda(corrida.rondas)
  const paso = duracion(pasoMs)
  const lenta = vecesMasLenta(pasoMs, media)
  const reproduccion = lenta
    ? `Lo que ves es una reproducción: cada ronda se muestra ${paso}, ${lenta}.`
    : 'Las rondas tardan más que el paso de la reproducción: se ven casi a su ritmo real.'

  switch (fase) {
    case 'cargando':
      return {
        fase,
        etiqueta: 'Cargando datos en los nodos',
        tono: 'acento',
        frase: 'La API aceptó el pedido. Cada nodo está cargando su parte del gold; todavía no llegó ninguna ronda.',
      }
    case 'en_vivo':
      return {
        fase,
        etiqueta: `En vivo · ronda ${mostradas}`,
        tono: 'acento',
        frase: `En vivo: ya se ${recibidas === 1 ? 'mostró la ronda que llegó' : `mostraron las ${recibidas} rondas que llegaron`} y la siguiente se ve en cuanto llegue. Cada ronda está tardando ${duracion(media)} de verdad.`,
      }
    case 'reproduciendo': {
      const fin = corrida.fin
      const etiqueta = fin ? `Reproduciendo · ronda ${mostradas} de ${recibidas}` : `Reproduciendo · ronda ${mostradas}`
      if (fin && !fin.abortado) {
        return { fase, etiqueta, tono: 'acento', frase: `${duroDeVerdad(fin, corrida.rondas)} ${reproduccion}` }
      }
      if (fin?.abortado) {
        return {
          fase,
          etiqueta,
          tono: 'acento',
          frase: `El entrenamiento ya abortó, después de ${cuantasRondas(recibidas)} de ${duracion(media)} en promedio. ${reproduccion}`,
        }
      }
      return {
        fase,
        etiqueta,
        tono: 'acento',
        frase: `El entrenamiento sigue en curso: llegaron ${cuantasRondas(recibidas)}, de ${duracion(media)} en promedio. ${reproduccion}`,
      }
    }
    case 'terminado': {
      const fin = corrida.fin as Extract<Fin, { abortado: false }>
      return {
        fase,
        etiqueta: `Terminado · ${cuantasRondas(recibidas)}`,
        tono: 'buena',
        frase: `${duroDeVerdad(fin, corrida.rondas)} ${lenta ? `Lo que viste fue una reproducción: cada ronda se mostró ${paso}, ${lenta}.` : 'Se vio casi a su ritmo real.'}`,
      }
    }
    case 'abortado':
      return {
        fase,
        etiqueta: recibidas === 0 ? 'Abortado antes de la primera ronda' : `Abortado en la ronda ${recibidas + 1}`,
        tono: 'alerta',
        frase:
          recibidas === 0
            ? 'El entrenamiento abortó antes de completar una ronda: no hay nada que reproducir.'
            : `El entrenamiento abortó en la ronda ${recibidas + 1}: se muestran las ${cuantasRondas(recibidas)} que llegaron a completarse.`,
      }
  }
}

function duroDeVerdad(fin: Extract<Fin, { abortado: false }>, rondas: readonly Ronda[]): string {
  const enRondas = rondas.reduce((suma, r) => suma + r.ms_ronda, 0)
  return `El entrenamiento real duró ${duracion(fin.ms_total)} (${cuantasRondas(rondas.length)} que suman ${duracion(enRondas)}; el resto es cargar los datos en los nodos).`
}

/** La tolerancia de la compuerta del modelo (cluster.Tolerancia en la API). */
export const TOLERANCIA = 1e-9

/** La diferencia con el K-Means secuencial, que va de 0 a 1e-15 o a decenas: en notación científica. */
export function diferencia(dif: number | undefined): string {
  if (dif === undefined) return '—'
  if (dif === 0) return '0'
  return dif.toExponential(1).replace('e', ' e')
}

export function motivoDeParo(paro: 'tolerancia' | 'max_iter' | undefined): string {
  if (paro === 'tolerancia') return 'Convergió'
  if (paro === 'max_iter') return 'Tope de iteraciones'
  return '—'
}

export interface Compuerta {
  tono: Tono
  titulo: string
  detalle: string
}

interface ParaLaCompuerta {
  replicas?: number
  modelo_id?: string
  aviso?: string
  dif_secuencial?: number
}

/**
 * Qué pasó con el modelo de un entrenamiento terminado y por qué (la compuerta
 * del contrato): se activa solo con réplicas = 1 y si su diferencia con el
 * K-Means secuencial no supera la tolerancia. `activo` es el modelo activo ahora.
 */
export function compuerta(e: ParaLaCompuerta, activo: string | undefined): Compuerta | null {
  if (e.modelo_id) {
    const sigue = activo === undefined || activo === e.modelo_id
    return {
      tono: 'buena',
      titulo: sigue ? 'El modelo nuevo quedó activo' : 'Activó su modelo (después se activó otro)',
      detalle: `${e.modelo_id} pasó la compuerta: su mayor diferencia con el K-Means secuencial es ${diferencia(e.dif_secuencial)}, dentro de la tolerancia (1 e-9). ${sigue ? 'Las predicciones ya usan este modelo.' : `El modelo activo ahora es ${activo}.`}`,
    }
  }
  if (e.aviso) {
    return { tono: 'alerta', titulo: 'El modelo no se activó', detalle: `La API dice: «${e.aviso}».` }
  }
  if (e.replicas !== undefined && e.replicas > 1) {
    return {
      tono: 'acento',
      titulo: 'Fue una medición: el modelo activo no cambió',
      detalle: `Con ${numero(e.replicas)} réplicas los nodos procesan ${numero(e.replicas)} copias del gold. Sirve para medir tiempos; solo un entrenamiento con 1 réplica puede activar su modelo.`,
    }
  }
  return null
}

/** Los nodos del entrenamiento que el texto del error nombra: el que falló llega dentro de `error`. */
export function nodosNombrados(error: string, nodos: readonly string[]): string[] {
  // El nombre entero: «nodo-1» no es «nodo-10».
  const parteDeNombre = /[\w-]/
  return nodos.filter((n) => {
    if (n === '') return false
    for (let i = error.indexOf(n); i !== -1; i = error.indexOf(n, i + 1)) {
      const antes = error[i - 1]
      const despues = error[i + n.length]
      if (!(antes && parteDeNombre.test(antes)) && !(despues && parteDeNombre.test(despues))) return true
    }
    return false
  })
}
