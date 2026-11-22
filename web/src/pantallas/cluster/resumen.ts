// Lo que las métricas dicen en una frase (spec §5.6), sin React.

import type { components } from '../../api/contrato'
import { milisegundos, numero, type Tono } from '../../componentes/indice'
import { OBJETIVO_MS } from '../../graficos/LatenciaPorRuta'

type Metricas = components['schemas']['Metricas']
type Entrenamiento = components['schemas']['Entrenamiento']

export interface LatenciaDeRuta {
  ruta: string
  pedidos: number
  p50_ms: number
  p95_ms: number
  p99_ms: number
}

/** La ruta cuyo objetivo es responder en menos de 100 ms (spec de la PC4, §6.2). */
export const RUTA_DE_PREDICCION = 'POST /api/predicciones'

/** Las rutas con sus percentiles, de la más lenta a la más rápida en p99. */
export function rutasPorLatencia(endpoints: Metricas['endpoints']): LatenciaDeRuta[] {
  return Object.entries(endpoints)
    .map(([ruta, m]) => ({ ruta, ...m }))
    .sort((a, b) => b.p99_ms - a.p99_ms || a.ruta.localeCompare(b.ruta))
}

export interface Dicho {
  tono: Tono
  texto: string
}

export function decirLatencia(rutas: readonly LatenciaDeRuta[]): Dicho | null {
  const peor = rutas[0]
  if (!peor) return null
  const lentas = rutas.filter((r) => r.p99_ms >= OBJETIVO_MS)
  const prediccion = rutas.find((r) => r.ruta === RUTA_DE_PREDICCION)
  if (prediccion) {
    const cumple = prediccion.p99_ms < OBJETIVO_MS
    return {
      tono: cumple ? 'buena' : 'alerta',
      texto: `El 99 % de las ${numero(prediccion.pedidos)} predicciones respondió en ${milisegundos(prediccion.p99_ms)} ms o menos: ${cumple ? 'dentro' : 'fuera'} del objetivo de ${OBJETIVO_MS} ms.`,
    }
  }
  if (lentas.length > 0) {
    return {
      tono: 'alerta',
      texto: `${lentas.length === 1 ? 'Una ruta tiene' : `${lentas.length} rutas tienen`} el p99 en ${OBJETIVO_MS} ms o más; la más lenta es ${peor.ruta}, con ${milisegundos(peor.p99_ms)} ms.`,
    }
  }
  return {
    tono: 'buena',
    texto: `Ninguna ruta llega a los ${OBJETIVO_MS} ms: la más lenta en p99 es ${peor.ruta}, con ${milisegundos(peor.p99_ms)} ms. Todavía no hubo predicciones, que son las que tienen ese objetivo.`,
  }
}

export function decirEscrituras(m: Pick<Metricas, 'escritura' | 'escrituras_descartadas' | 'escrituras_fallidas'>): Dicho {
  const escritas = m.escritura.mongo.escritas + m.escritura.redis.escritas
  const perdidas = m.escrituras_descartadas + m.escrituras_fallidas
  if (perdidas > 0) {
    return {
      tono: 'alerta',
      texto: `Se perdieron ${numero(perdidas)} escrituras: ${numero(m.escrituras_descartadas)} descartadas por una cola llena y ${numero(m.escrituras_fallidas)} que el almacén rechazó.`,
    }
  }
  if (escritas === 0) {
    return {
      tono: 'acento',
      texto: 'Todavía no se escribió ninguna predicción: se escriben al predecir, desde la simulación o al cargar el escenario.',
    }
  }
  return { tono: 'buena', texto: 'No se perdió ninguna escritura: todo lo que la API encoló llegó a su almacén.' }
}

export interface RespuestaDeNodo {
  /** La media de lo que tardó en responder cada ronda, en ms. */
  media: number
  maximo: number
}

export interface RespuestaPorRonda {
  /** El entrenamiento del que salen los tiempos. */
  id: string
  replicas: number
  rondas: number
  porNodo: Record<string, RespuestaDeNodo>
}

/**
 * Lo que tardó cada nodo en responder las rondas de un entrenamiento
 * (ms_por_nodo, medido desde la API y con la red incluida). Es lo que se sabe
 * de la carga de un nodo: su CPU no viaja por el protocolo. null si el
 * entrenamiento no completó ninguna ronda.
 */
export function respuestaPorRonda(e: Entrenamiento): RespuestaPorRonda | null {
  if (e.rondas.length === 0) return null
  const tiempos: Record<string, number[]> = {}
  for (const ronda of e.rondas) {
    for (const [nodo, ms] of Object.entries(ronda.ms_por_nodo)) (tiempos[nodo] ??= []).push(ms)
  }
  const porNodo: Record<string, RespuestaDeNodo> = {}
  for (const [nodo, ms] of Object.entries(tiempos)) {
    porNodo[nodo] = { media: ms.reduce((a, b) => a + b, 0) / ms.length, maximo: Math.max(...ms) }
  }
  return { id: e.id, replicas: e.replicas, rondas: e.rondas.length, porNodo }
}
