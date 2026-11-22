// El simulador: manda las lecturas de un tramo a POST /api/predicciones, una por
// una y en el orden del archivo, a un ritmo que se pueda seguir (spec §5.4).
//
// Vive fuera de React, como la sesión: una corrida sigue aunque se cambie de
// pantalla (se carga el escenario y se va al panel a verlo llenarse). La
// pantalla lo lee con useCorrida() y lo maneja con iniciar, pausar, continuar y
// detener.
//
// Reglas:
//  - El orden no se altera: cada lectura sale cuando la anterior ya tuvo
//    respuesta. El ritmo se mide en horas simuladas por segundo, y la espera va
//    entre una hora y la siguiente, no entre lecturas de la misma hora.
//  - Pausar deja terminar el pedido que está en viaje (esa lectura ya salió) y
//    no manda la siguiente. Detener cancela el pedido y termina la corrida.
//  - Si un pedido falla, la corrida se queda en esa lectura: no la salta.
//    continuar() la vuelve a mandar.
//  - La corrida vive en la pestaña: recargarla o cerrarla la corta y deja el
//    tramo a medias en el sistema. Mientras está en curso, el navegador pregunta
//    antes de irse (`beforeunload`); en pausa, detenida o terminada, no.

import { useSyncExternalStore } from 'react'
import { api, pedir } from '../../api/cliente'
import { anotarVista, type LecturaVista } from '../../api/vistas'
import type { Lectura, Prediccion, Tramo } from './series'

export type Fase =
  /** No hay corrida. */
  | 'lista'
  | 'en_curso'
  | 'en_pausa'
  /** Un pedido falló: la corrida espera en esa lectura. */
  | 'con_error'
  /** Se detuvo antes de mandar todo. */
  | 'detenida'
  | 'terminada'

/** Lo que se sabe de una estación del tramo: lo enviado y lo que la API respondió. */
export interface Seguimiento {
  estacion: string
  puntos: readonly LecturaVista[]
  /** La respuesta a su lectura más reciente. */
  ultima: Prediccion | null
  /** La fecha_hora de la primera lectura a la que la API le devolvió media, y de la primera con alerta. */
  primeraMedia: string | null
  primeraAlerta: string | null
}

export interface Corrida {
  fase: Fase
  tramo: Tramo | null
  horasPorSegundo: number
  /** Cuántas lecturas ya tienen respuesta. */
  enviadas: number
  /** La fecha_hora de la última lectura con respuesta. */
  horaActual: string | null
  porEstacion: Readonly<Record<string, Seguimiento>>
  /** El error del pedido que falló, con la lectura que no pasó. */
  error: unknown
  lecturaConError: Lectura | null
  /** El modelo que respondió (modelo_id de la última respuesta). */
  modeloId: string | null
  /** Cuánto lleva o cuánto tardó, sin contar las pausas, en milisegundos. */
  duracionMs: number
}

/** De la ventana del navegador, lo que hace falta para avisar antes de que se cierre o se recargue. */
export interface VentanaQueAvisa {
  addEventListener: (tipo: 'beforeunload', oyente: (evento: BeforeUnloadEvent) => void) => void
  removeEventListener: (tipo: 'beforeunload', oyente: (evento: BeforeUnloadEvent) => void) => void
}

export interface Dependencias {
  enviar: (lectura: Lectura, signal: AbortSignal) => Promise<Prediccion>
  /** Espera `ms` o hasta que se corte; nunca rechaza. */
  esperar: (ms: number, signal: AbortSignal) => Promise<void>
  ahora: () => number
  /** La ventana a la que se le pide avisar con una corrida en curso; null si no hay (fuera del navegador). */
  ventana: VentanaQueAvisa | null
}

const VACIA: Corrida = {
  fase: 'lista',
  tramo: null,
  horasPorSegundo: 4,
  enviadas: 0,
  horaActual: null,
  porEstacion: {},
  error: null,
  lecturaConError: null,
  modeloId: null,
  duracionMs: 0,
}

function esperarDeVerdad(ms: number, signal: AbortSignal): Promise<void> {
  return new Promise((resolver) => {
    if (ms <= 0 || signal.aborted) return resolver()
    const reloj = setTimeout(listo, ms)
    function listo() {
      clearTimeout(reloj)
      signal.removeEventListener('abort', listo)
      resolver()
    }
    signal.addEventListener('abort', listo)
  })
}

const REALES: Dependencias = {
  enviar: (lectura, signal) => pedir(api.POST('/api/predicciones', { body: lectura, signal })),
  esperar: esperarDeVerdad,
  ahora: () => Date.now(),
  ventana: typeof window === 'undefined' ? null : window,
}

/**
 * Lo que oye `beforeunload` con una corrida en curso: le pide al navegador que
 * pregunte antes de irse. El texto de la pregunta lo pone el navegador.
 */
function preguntarAntesDeSalir(evento: BeforeUnloadEvent) {
  evento.preventDefault()
  // Algunos navegadores solo preguntan si además se pone returnValue.
  evento.returnValue = ''
}

export class Simulador {
  private foto: Corrida = VACIA
  private readonly oyentes = new Set<() => void>()
  private readonly deps: Dependencias

  // Lo que cambia con cada lectura se lleva aparte y se copia a la foto al
  // publicar: una hora del escenario son hasta 14 respuestas y un solo dibujo.
  private seguimientos = new Map<string, { puntos: LecturaVista[]; ultima: Prediccion; primeraMedia: string | null; primeraAlerta: string | null }>()
  private tocadas = new Set<string>()
  private siguiente = 0
  private modeloId: string | null = null
  private horaActual: string | null = null

  // Cada corrida (y cada detención) es un turno: un bucle de un turno anterior no toca nada.
  private turno = 0
  private activo = false
  private corte = new AbortController()
  private acumulado = 0
  private desde: number | null = null
  /** Si el aviso de `beforeunload` está puesto. */
  private avisaAlSalir = false

  constructor(deps: Partial<Dependencias> = {}) {
    this.deps = { ...REALES, ...deps }
  }

  readonly suscribir = (oyente: () => void) => {
    this.oyentes.add(oyente)
    return () => this.oyentes.delete(oyente)
  }

  readonly leer = () => this.foto

  /** Si hay una corrida sin terminar: en curso, en pausa o esperando tras un error. */
  get ocupado(): boolean {
    return enMarcha(this.foto.fase)
  }

  /** Empieza a mandar un tramo desde su primera lectura. Si hay otra corrida sin terminar, no hace nada. */
  iniciar(tramo: Tramo, horasPorSegundo: number) {
    if (this.ocupado) return
    this.turno++
    this.corte = new AbortController()
    this.seguimientos = new Map()
    this.tocadas = new Set()
    this.siguiente = 0
    this.modeloId = null
    this.horaActual = null
    this.acumulado = 0
    this.desde = this.deps.ahora()
    this.foto = { ...VACIA, fase: tramo.lecturas.length === 0 ? 'terminada' : 'en_curso', tramo, horasPorSegundo }
    this.avisar()
    if (this.foto.fase === 'en_curso') void this.correr(this.turno)
  }

  /** No manda la lectura siguiente. La que está en viaje termina y se anota. */
  pausar() {
    if (this.foto.fase !== 'en_curso') return
    this.detenerReloj()
    this.publicar({ fase: 'en_pausa' })
  }

  /** Sigue desde la lectura siguiente; después de un error, vuelve a mandar la que falló. */
  continuar() {
    if (this.foto.fase !== 'en_pausa' && this.foto.fase !== 'con_error') return
    this.desde = this.deps.ahora()
    this.publicar({ fase: 'en_curso', error: null, lecturaConError: null })
    if (!this.activo) void this.correr(this.turno)
  }

  /** Termina la corrida donde está. Lo ya enviado queda en el sistema. */
  detener() {
    if (!this.ocupado) return
    this.cortar()
    this.publicar({ fase: 'detenida', error: null, lecturaConError: null })
  }

  /** Olvida la corrida (la que haya, terminada o no) y vuelve a «lista». */
  reiniciar() {
    this.cortar()
    this.seguimientos = new Map()
    this.tocadas = new Set()
    this.siguiente = 0
    this.foto = { ...VACIA, horasPorSegundo: this.foto.horasPorSegundo }
    this.avisar()
  }

  /** Cambia el ritmo; vale desde la hora siguiente. */
  cambiarRitmo(horasPorSegundo: number) {
    if (horasPorSegundo === this.foto.horasPorSegundo) return
    this.publicar({ horasPorSegundo })
  }

  private cortar() {
    this.turno++
    this.activo = false
    this.corte.abort()
    this.corte = new AbortController()
    this.detenerReloj()
  }

  private detenerReloj() {
    if (this.desde === null) return
    this.acumulado += this.deps.ahora() - this.desde
    this.desde = null
  }

  private duracion(): number {
    return this.acumulado + (this.desde === null ? 0 : this.deps.ahora() - this.desde)
  }

  private async correr(turno: number) {
    const tramo = this.foto.tramo
    if (!tramo) return
    const { signal } = this.corte
    this.activo = true
    let inicioDeHora = this.deps.ahora()
    try {
      while (turno === this.turno && this.foto.fase === 'en_curso' && this.siguiente < tramo.lecturas.length) {
        const lectura = tramo.lecturas[this.siguiente]!
        let respuesta: Prediccion
        try {
          respuesta = await this.deps.enviar(lectura, signal)
        } catch (error) {
          // Detenida o reiniciada mientras el pedido viajaba: ya no es de esta corrida.
          if (turno !== this.turno) return
          this.detenerReloj()
          this.publicar({ fase: 'con_error', error, lecturaConError: lectura })
          return
        }
        if (turno !== this.turno) return
        this.anotar(lectura, respuesta)
        this.siguiente++

        const proxima = tramo.lecturas[this.siguiente]
        if (proxima === undefined) break
        if (proxima.fecha_hora !== lectura.fecha_hora) {
          this.publicar({})
          if (this.foto.fase !== 'en_curso') break
          const periodo = 1000 / this.foto.horasPorSegundo
          await this.deps.esperar(periodo - (this.deps.ahora() - inicioDeHora), signal)
          inicioDeHora = this.deps.ahora()
        }
      }
      if (turno !== this.turno) return
      if (this.siguiente >= tramo.lecturas.length) {
        this.detenerReloj()
        this.publicar({ fase: 'terminada' })
      } else {
        this.publicar({})
      }
    } finally {
      if (turno === this.turno) this.activo = false
    }
  }

  private anotar(lectura: Lectura, r: Prediccion) {
    const punto: LecturaVista = {
      fecha_hora: lectura.fecha_hora,
      pm2_5: lectura.pm2_5,
      pm10: lectura.pm10,
      media_pm2_5: r.media_24h.pm2_5,
      media_pm10: r.media_24h.pm10,
      lecturas: r.media_24h.lecturas,
      alerta: r.alerta,
      inca: r.inca,
      regimen: r.regimen,
      excede_eca: r.excede_eca,
    }
    const hayMedia = r.media_24h.pm2_5 !== null || r.media_24h.pm10 !== null
    const s = this.seguimientos.get(lectura.estacion)
    if (s) {
      s.puntos.push(punto)
      s.ultima = r
      if (hayMedia) s.primeraMedia ??= lectura.fecha_hora
      if (r.alerta) s.primeraAlerta ??= lectura.fecha_hora
    } else {
      this.seguimientos.set(lectura.estacion, {
        puntos: [punto],
        ultima: r,
        primeraMedia: hayMedia ? lectura.fecha_hora : null,
        primeraAlerta: r.alerta ? lectura.fecha_hora : null,
      })
    }
    this.tocadas.add(lectura.estacion)
    this.modeloId = r.modelo_id
    this.horaActual = lectura.fecha_hora
    anotarVista(lectura.estacion, punto)
  }

  /** Arma una foto nueva con lo anotado desde la anterior y avisa. */
  private publicar(cambios: Partial<Corrida>) {
    let porEstacion = this.foto.porEstacion
    if (this.tocadas.size > 0) {
      const copia: Record<string, Seguimiento> = { ...porEstacion }
      for (const estacion of this.tocadas) {
        const s = this.seguimientos.get(estacion)!
        copia[estacion] = { estacion, ...s, puntos: s.puntos.slice() }
      }
      porEstacion = copia
      this.tocadas = new Set()
    }
    this.foto = {
      ...this.foto,
      ...cambios,
      porEstacion,
      enviadas: this.siguiente,
      horaActual: this.horaActual,
      modeloId: this.modeloId,
      duracionMs: this.duracion(),
    }
    this.avisar()
  }

  private avisar() {
    this.vigilarLaSalida()
    for (const oyente of [...this.oyentes]) oyente()
  }

  /** Pone el aviso de `beforeunload` mientras la corrida está en curso y lo quita en cuanto deja de estarlo. */
  private vigilarLaSalida() {
    const enCurso = this.foto.fase === 'en_curso'
    if (enCurso === this.avisaAlSalir) return
    this.avisaAlSalir = enCurso
    if (enCurso) this.deps.ventana?.addEventListener('beforeunload', preguntarAntesDeSalir)
    else this.deps.ventana?.removeEventListener('beforeunload', preguntarAntesDeSalir)
  }
}

/** Si la fase es de una corrida sin terminar. */
export function enMarcha(fase: Fase): boolean {
  return fase === 'en_curso' || fase === 'en_pausa' || fase === 'con_error'
}

/**
 * El simulador de la app: uno solo, para que la corrida siga al cambiar de
 * pantalla. No sigue en la sesión siguiente: al terminar la sesión, App.tsx lo
 * reinicia junto con todo lo demás que la sesión dejó en la pestaña.
 */
export const simulador = new Simulador()

/** La corrida de un simulador, dentro de React. */
export function useCorrida(de: Simulador = simulador): Corrida {
  return useSyncExternalStore(de.suscribir, de.leer)
}
