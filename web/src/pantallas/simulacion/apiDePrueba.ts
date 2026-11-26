// Una API falsa para las pruebas de Simulación: responde /datos/series.json con
// el archivo real y POST /api/predicciones con la regla de ventana/ventana.go
// (ventana (t − 24 h, t], media desde 18 lecturas, alerta e INCA), escrita aquí
// otra vez. Así «lo que respondió la API contra lo esperado» se prueba de
// verdad: el esperado del archivo lo calculó scripts/impacto.py por su lado.
//
// También lleva el último estado de cada estación como la API: no retrocede.

import { vi } from 'vitest'
import crudoDeSeries from '../../../public/datos/series.json?raw'
import type { Series } from '../../api/datos'
import type { EstadoEstacion } from '../../api/enVivo'
import type { BandaINCA } from '../../componentes/bandas'
import { respuesta } from '../../pruebas/dobles'
import type { Lectura, Prediccion } from './series'

export const SERIES = JSON.parse(crudoDeSeries) as Series

const HORA_MS = 3_600_000
type Ventana = Map<number, { pm2_5: number; pm10: number }>

function banda(valor: number, cortes: readonly number[]): number {
  const i = cortes.findIndex((c) => valor <= c)
  return i === -1 ? cortes.length : i
}

interface Opciones {
  /** El último estado con que arranca el sistema: vacío es un sistema limpio. */
  estados?: EstadoEstacion[]
  /** Lo que responde /datos/series.json; por omisión, el archivo real. */
  series?: unknown
  /**
   * Antes de responder una predicción (n cuenta desde 1): puede esperar, o
   * devolver otra respuesta en lugar de la calculada.
   */
  alPredecir?: (lectura: Lectura, n: number, calculada: Prediccion) => Promise<Response | void> | Response | void
}

export function apiDePrueba({ estados = [], series = SERIES, alPredecir }: Opciones = {}) {
  const reglas = SERIES.metadatos.reglas
  const ventanas = new Map<string, Ventana>()
  const estado = new Map(estados.map((e) => [e.estacion, e]))
  /** Los cuerpos de los POST /api/predicciones, en el orden en que llegaron. */
  const lecturas: Lectura[] = []
  /** «MÉTODO /ruta» de cada pedido, en orden. */
  const pedidos: string[] = []

  // El régimen lo decide el modelo, que aquí no está: se responde el que el archivo espera para ese tramo.
  const tramos = [
    ...Object.entries(SERIES.escenario.esperado).map(([estacion, esperado]) => ({ estacion, desde: SERIES.escenario.desde, esperado })),
    ...SERIES.dias.map((d) => ({ estacion: d.estacion, desde: d.desde, esperado: d.esperado })),
  ]
  function regimenDe(l: Lectura) {
    const t = Date.parse(l.fecha_hora)
    const tramo = tramos.find((x) => x.estacion === l.estacion && Date.parse(x.desde) <= t && t <= Date.parse(x.esperado.fecha_hora))
    return { regimen: tramo?.esperado.regimen ?? 0, excede_eca: tramo?.esperado.excede_eca ?? false }
  }

  function predecir(l: Lectura): Prediccion {
    const t = Math.floor(Date.parse(l.fecha_hora) / HORA_MS) * HORA_MS
    const ventana = ventanas.get(l.estacion) ?? new Map()
    ventanas.set(l.estacion, ventana)
    ventana.set(t, { pm2_5: l.pm2_5, pm10: l.pm10 })
    const dentro = [...ventana].filter(([h]) => h > t - reglas.ventana_horas * HORA_MS && h <= t).map(([, v]) => v)
    const hayMedia = dentro.length >= reglas.minimo_lecturas
    const media = (campo: 'pm2_5' | 'pm10') => dentro.reduce((s, v) => s + v[campo], 0) / dentro.length
    const pm2_5 = hayMedia ? media('pm2_5') : null
    const pm10 = hayMedia ? media('pm10') : null
    const inca =
      pm2_5 === null || pm10 === null
        ? null
        : (reglas.inca.bandas[Math.max(banda(pm10, reglas.inca.cortes_pm10), banda(pm2_5, reglas.inca.cortes_pm2_5))] as BandaINCA)
    const prediccion: Prediccion = {
      estacion: l.estacion,
      ...regimenDe(l),
      media_24h: { pm2_5, pm10, lecturas: dentro.length },
      alerta: pm2_5 !== null && pm10 !== null && (pm2_5 > reglas.eca_pm2_5 || pm10 > reglas.eca_pm10),
      inca,
      pm2_5_medio_regimen: 9.04,
      pm10_medio_regimen: 17.64,
      modelo_id: 'sembrado-kmeans-json',
      ms: 0.04,
    }
    // El estado no retrocede: una lectura más antigua que la guardada no lo cambia; una de la misma hora, sí.
    const guardado = estado.get(l.estacion)
    if (!guardado || Date.parse(guardado.fecha_hora) <= t) {
      estado.set(l.estacion, {
        estacion: l.estacion,
        fecha_hora: l.fecha_hora,
        regimen: prediccion.regimen,
        excede_eca: prediccion.excede_eca,
        media_24h: prediccion.media_24h,
        alerta: prediccion.alerta,
        inca: prediccion.inca,
        ts: '2026-11-21T12:00:00-05:00',
      })
    }
    return prediccion
  }

  const fetch = vi.fn(async (pedido: Request) => {
    const ruta = new URL(pedido.url).pathname
    pedidos.push(`${pedido.method} ${ruta}`)
    switch (`${pedido.method} ${ruta}`) {
      case 'GET /datos/series.json':
        return series === null ? respuesta(404) : respuesta(200, series)
      case 'GET /api/regimenes':
        return respuesta(200, [...estado.values()].sort((a, b) => a.estacion.localeCompare(b.estacion)))
      case 'GET /api/estaciones':
        return respuesta(200, [])
      case 'POST /api/predicciones': {
        const lectura = (await pedido.json()) as Lectura
        lecturas.push(lectura)
        // Repetir la lectura de una hora la reemplaza: reintentar una que falló no cambia la ventana.
        const calculada = predecir(lectura)
        const otra = await alPredecir?.(lectura, lecturas.length, calculada)
        return otra ?? respuesta(200, calculada)
      }
      default:
        return respuesta(404, { error: `no hay doble para ${pedido.method} ${ruta}` })
    }
  })
  vi.stubGlobal('fetch', fetch)
  return { fetch, lecturas, pedidos, estado }
}
