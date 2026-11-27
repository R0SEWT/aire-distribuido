// Dobles para las pruebas: un fetch que responde lo que se le diga y un
// WebSocket falso que se abre, recibe y se cierra a mano.

import { vi } from 'vitest'
import type { Enchufe } from '../api/ws'

type Respuesta = { estado: number; cuerpo?: unknown } | 'sin_respuesta'

/** Arma una Response JSON como las de la API. */
export function respuesta(estado: number, cuerpo?: unknown): Response {
  return new Response(cuerpo === undefined ? null : JSON.stringify(cuerpo), {
    status: estado,
    headers: cuerpo === undefined ? {} : { 'Content-Type': 'application/json' },
  })
}

/**
 * Reemplaza fetch. `rutas` dice qué responde cada «MÉTODO /ruta»; lo que no
 * esté responde 404. Devuelve el espía, para mirar qué se pidió.
 */
export function simularApi(rutas: Record<string, Respuesta | (() => Respuesta)>) {
  const espia = vi.fn(async (pedido: Request) => {
    const clave = `${pedido.method} ${new URL(pedido.url).pathname}`
    const r = rutas[clave]
    const dada = typeof r === 'function' ? r() : r
    if (dada === 'sin_respuesta') throw new TypeError('Failed to fetch')
    if (!dada) return respuesta(404, { error: `no hay doble para ${clave}` })
    return respuesta(dada.estado, dada.cuerpo)
  })
  vi.stubGlobal('fetch', espia)
  return espia
}

export const SALUD_OK = {
  estado: 200,
  cuerpo: { api: 'ok', mongo: 'ok', redis: 'ok', nodos_en_pool: 2, modelo_activo: 'sembrado-kmeans-json' },
} as const

/** Un token como el de POST /api/auth/token, que vence dentro de `enMs`. */
export function tokenDe(rol: 'usuario' | 'admin', enMs = 3_600_000) {
  return { token: `token-de-${rol}`, rol, expira: new Date(Date.now() + enMs).toISOString() }
}

export class EnchufeFalso implements Enchufe {
  onopen: ((e: Event) => void) | null = null
  onmessage: ((e: MessageEvent) => void) | null = null
  onclose: ((e: CloseEvent) => void) | null = null
  onerror: ((e: Event) => void) | null = null
  cerradoCon: { codigo?: number; motivo?: string } | null = null

  constructor(
    readonly url: string,
    readonly protocolos: string[],
  ) {}

  close(codigo?: number, motivo?: string) {
    this.cerradoCon = { codigo, motivo }
  }

  /** El servidor aceptó el apretón de manos. */
  abrir() {
    this.onopen?.(new Event('open'))
  }

  /** El servidor mandó una trama de texto. */
  recibir(trama: unknown) {
    this.onmessage?.({ data: typeof trama === 'string' ? trama : JSON.stringify(trama) } as MessageEvent)
  }

  /** La conexión se cerró: 1006 es una caída, 1008 un cierre por política. */
  cerrarDesdeElServidor(codigo: number, motivo = '') {
    this.onclose?.({ code: codigo, reason: motivo } as CloseEvent)
  }
}

/** Una fábrica de enchufes falsos que recuerda todos los que creó. */
export function fabricaDeEnchufes() {
  const creados: EnchufeFalso[] = []
  const crearEnchufe = (url: string, protocolos: string[]) => {
    const enchufe = new EnchufeFalso(url, protocolos)
    creados.push(enchufe)
    return enchufe
  }
  return { creados, crearEnchufe, ultimo: () => creados[creados.length - 1]! }
}
