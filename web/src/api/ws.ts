// La conexión de tiempo real (spec §9.4): un WebSocket por sesión contra /ws.
//
// - El token va en Sec-WebSocket-Protocol (subprotocolo aire.jwt), no en la
//   URL: new WebSocket(url, ['aire.jwt', token]). La API confirma aire.jwt.
// - Si la conexión se cae, reconecta sola con una espera que crece (1 s, 2 s,
//   4 s… hasta 30 s) y algo de azar, para que varias pestañas no golpeen juntas.
// - NO reintenta si el servidor cerró con 1008 (el token venció, o echó a un
//   cliente lento) ni pasada la hora de `expira`: con un token vencido cada
//   intento sería un 401 más.
// - Si el apretón de manos falla varias veces seguidas avisa (alFallarApreton),
//   para que se compruebe la sesión por HTTP: un token que la API ya no acepta
//   (cambió su secreto) no tiene que quedar reintentando para siempre.
//
// Esta clase no sabe de React ni de la sesión: avisa su estado y reparte los
// eventos por nombre. api/TiempoReal.tsx la une con la sesión y da los hooks.

import type { EventosWS, NombreEvento } from './eventos'

export const SUBPROTOCOLO = 'aire.jwt'
/** Cierre por política (RFC 6455): la API lo usa para el token vencido y el cliente lento. */
export const CIERRE_POLITICA = 1008

export type EstadoWS =
  /** Todavía no se pidió conectar, o se cerró a propósito. */
  | 'inactiva'
  | 'conectando'
  | 'conectada'
  /** Se cayó y hay un reintento en espera o en curso. */
  | 'reconectando'
  /** El servidor cerró con 1008 sin que el token haya vencido: no se reintenta sola. */
  | 'cortada'
  /** El token venció: no hay nada que reintentar, toca volver al ingreso. */
  | 'vencida'

/** Lo que esta clase usa de un WebSocket; las pruebas pasan uno falso. */
export interface Enchufe {
  onopen: ((e: Event) => void) | null
  onmessage: ((e: MessageEvent) => void) | null
  onclose: ((e: CloseEvent) => void) | null
  onerror: ((e: Event) => void) | null
  close(codigo?: number, motivo?: string): void
}

export interface OpcionesWS {
  url: string
  token: string
  /** Cuándo vence el token, en ms desde la época. Pasada esa hora no se conecta. */
  expira: number
  crearEnchufe?: (url: string, protocolos: string[]) => Enchufe
  ahora?: () => number
  azar?: () => number
  esperaBaseMs?: number
  esperaMaximaMs?: number
  /**
   * Se llama cada `fallosParaAvisar` apretones de manos fallidos seguidos (la
   * conexión se cerró sin llegar a abrirse). El navegador no dice por qué
   * falló un apretón: un 401 por un token que la API ya no acepta y una API
   * apagada se ven igual. Quien usa la conexión aprovecha el aviso para
   * comprobar la sesión por HTTP. Los reintentos siguen mientras tanto.
   */
  alFallarApreton?: (fallosSeguidos: number) => void
  fallosParaAvisar?: number
}

type Manejador<N extends NombreEvento> = (datos: EventosWS[N]) => void
type ManejadorSuelto = (datos: never) => void

// Un 1008 a menos de este margen del vencimiento se toma como token vencido,
// aunque el reloj de esta máquina vaya algo atrasado respecto del servidor.
const MARGEN_VENCIMIENTO_MS = 60_000

export class ConexionWS {
  private readonly op: Required<OpcionesWS>
  private enchufe: Enchufe | null = null
  private espera: ReturnType<typeof setTimeout> | undefined
  private intentos = 0
  private apretonesFallidos = 0
  private estadoActual: EstadoWS = 'inactiva'
  private readonly oyentes = new Set<(estado: EstadoWS) => void>()
  private readonly manejadores = new Map<string, Set<ManejadorSuelto>>()

  constructor(opciones: OpcionesWS) {
    this.op = {
      crearEnchufe: (url, protocolos) => new WebSocket(url, protocolos),
      ahora: Date.now,
      azar: Math.random,
      esperaBaseMs: 1_000,
      esperaMaximaMs: 30_000,
      alFallarApreton: () => {},
      fallosParaAvisar: 3,
      ...opciones,
    }
  }

  get estado(): EstadoWS {
    return this.estadoActual
  }

  /** Avisa cada cambio de estado. Devuelve cómo dejar de escuchar. */
  alCambiar(oyente: (estado: EstadoWS) => void): () => void {
    this.oyentes.add(oyente)
    return () => this.oyentes.delete(oyente)
  }

  /**
   * Suscribe un manejador a un evento por su nombre ('estacion.regimen',
   * 'entrenamiento.ronda'…). Se puede suscribir antes de conectar. Devuelve
   * cómo desuscribirse.
   */
  suscribir<N extends NombreEvento>(evento: N, manejador: Manejador<N>): () => void {
    let grupo = this.manejadores.get(evento)
    if (!grupo) {
      grupo = new Set()
      this.manejadores.set(evento, grupo)
    }
    grupo.add(manejador as ManejadorSuelto)
    return () => grupo.delete(manejador as ManejadorSuelto)
  }

  /** Abre la conexión, si no hay una abierta o en camino. */
  conectar() {
    if (this.enchufe || this.espera !== undefined) return
    this.abrir()
  }

  /**
   * Vuelve a conectar a pedido de la persona después de un corte del servidor
   * ('cortada'). No es un reintento automático.
   */
  reconectar() {
    if (this.estadoActual !== 'cortada') return
    this.intentos = 0
    this.apretonesFallidos = 0
    this.abrir()
  }

  /** Cierra a propósito (al salir de la sesión). No reconecta. */
  cerrar() {
    clearTimeout(this.espera)
    this.espera = undefined
    const enchufe = this.soltar()
    enchufe?.close(1000, 'la sesión terminó')
    this.cambiar('inactiva')
  }

  private vencio(): boolean {
    return this.op.ahora() >= this.op.expira
  }

  private abrir() {
    this.espera = undefined
    if (this.vencio()) {
      this.cambiar('vencida')
      return
    }
    this.cambiar(this.intentos === 0 ? 'conectando' : 'reconectando')
    let enchufe: Enchufe
    try {
      enchufe = this.op.crearEnchufe(this.op.url, [SUBPROTOCOLO, this.op.token])
    } catch {
      this.apretonFallido()
      this.reintentar()
      return
    }
    this.enchufe = enchufe
    let abrio = false
    enchufe.onopen = () => {
      abrio = true
      this.intentos = 0
      this.apretonesFallidos = 0
      this.cambiar('conectada')
    }
    enchufe.onmessage = (e) => this.repartir(e.data)
    // Un error siempre viene seguido de un cierre: todo se decide ahí.
    enchufe.onerror = null
    enchufe.onclose = (e) => {
      if (this.enchufe !== enchufe) return
      this.soltar()
      if (!abrio) this.apretonFallido()
      this.alCerrarse(e)
    }
  }

  private apretonFallido() {
    this.apretonesFallidos += 1
    if (this.apretonesFallidos % this.op.fallosParaAvisar !== 0) return
    try {
      this.op.alFallarApreton(this.apretonesFallidos)
    } catch (error) {
      console.error('alFallarApreton falló', error)
    }
  }

  private alCerrarse(e: CloseEvent) {
    if (e.code === CIERRE_POLITICA) {
      const porToken = /token/i.test(e.reason) || this.op.expira - this.op.ahora() <= MARGEN_VENCIMIENTO_MS
      this.cambiar(porToken ? 'vencida' : 'cortada')
      return
    }
    if (this.vencio()) {
      this.cambiar('vencida')
      return
    }
    this.reintentar()
  }

  private reintentar() {
    const tope = Math.min(this.op.esperaMaximaMs, this.op.esperaBaseMs * 2 ** this.intentos)
    // Entre la mitad y el total de la espera, para repartir los reintentos.
    const espera = tope * (0.5 + 0.5 * this.op.azar())
    this.intentos += 1
    this.cambiar('reconectando')
    this.espera = setTimeout(() => this.abrir(), espera)
  }

  private soltar(): Enchufe | null {
    const enchufe = this.enchufe
    if (enchufe) enchufe.onopen = enchufe.onmessage = enchufe.onclose = enchufe.onerror = null
    this.enchufe = null
    return enchufe
  }

  private repartir(trama: unknown) {
    if (typeof trama !== 'string') return
    let evento: unknown
    let datos: unknown
    try {
      ;({ evento, datos } = JSON.parse(trama) as { evento?: unknown; datos?: unknown })
    } catch {
      return // una trama que no es JSON no es de este protocolo
    }
    if (typeof evento !== 'string') return
    for (const manejador of [...(this.manejadores.get(evento) ?? [])]) {
      try {
        manejador(datos as never)
      } catch (error) {
        // Un manejador que falla no deja sin evento a los demás.
        console.error(`el manejador de ${evento} falló`, error)
      }
    }
  }

  private cambiar(estado: EstadoWS) {
    if (estado === this.estadoActual) return
    this.estadoActual = estado
    for (const oyente of [...this.oyentes]) oyente(estado)
  }
}

/** La URL de /ws en el mismo origen que la página: ws:// o wss:// según cómo se sirva. */
export function urlWS(ubicacion: Pick<Location, 'protocol' | 'host'> = globalThis.location): string {
  return `${ubicacion.protocol === 'https:' ? 'wss' : 'ws'}://${ubicacion.host}/ws`
}
