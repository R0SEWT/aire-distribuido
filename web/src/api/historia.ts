// Lo que esta pestaña recibió de cada estación durante la sesión del navegador
// (spec §5.3): la API no tiene una serie histórica, así que la curva de la
// Estación se arma con los estados que fueron llegando, por un GET o por el
// evento `estacion.regimen`.
//
// No se suscribe al WebSocket: mira la caché de TanStack Query, que es adonde
// el armazón lleva cada evento (api/enVivo.ts). Cada estado que entra en
// claves.regimenes o en claves.regimen(id) queda anotado aquí, con su hora.
//
// Se guarda en sessionStorage, como la sesión (agrupando las escrituras: a lo
// sumo una cada medio segundo, y al ocultarse la página): un F5 no borra la curva, y al
// cerrar la pestaña se va. Al terminar la sesión («Salir», vencimiento o 401)
// la borra App.tsx con olvidarHistoria(), junto con todo lo demás de la sesión.
//
// La API manda, a lo sumo, un evento por estación cada 250 ms (api/avisos.go):
// de una ráfaga de lecturas llegan solo algunos estados, y la curva tiene esos.
// El evento tampoco trae la lectura horaria: las que esta sesión envió están en
// api/vistas.ts.

import { useQueryClient, type QueryClient } from '@tanstack/react-query'
import { useEffect, useMemo, useSyncExternalStore } from 'react'
import type { BandaINCA } from '../componentes/bandas'
import { claves } from './cliente'
import type { EstadoEstacion } from './enVivo'
import type { RegimenEstacion } from './estaciones'

/** Un estado recibido de una estación: un punto de la curva. */
export interface PuntoSesion {
  /** La hora de la lectura a la que corresponde el estado. */
  fecha_hora: string
  /** La misma hora, en milisegundos. */
  t: number
  /** La media de 24 horas; null con menos de 18 lecturas en la ventana. */
  pm2_5: number | null
  pm10: number | null
  lecturas: number
  alerta: boolean
  inca: BandaINCA
}

const CLAVE = 'aire.curvas'
/** Tope por estación: la API lleva 72 lecturas por estación; con esto sobra para varios días. */
const TOPE = 240
const NINGUNO: readonly PuntoSesion[] = []

function puntoDe(estado: RegimenEstacion): PuntoSesion | null {
  const t = Date.parse(estado.fecha_hora)
  if (!Number.isFinite(t)) return null
  const { pm2_5, pm10, lecturas } = estado.media_24h
  return { fecha_hora: estado.fecha_hora, t, pm2_5, pm10, lecturas, alerta: estado.alerta, inca: estado.inca }
}

function iguales(a: PuntoSesion, b: PuntoSesion): boolean {
  return (
    a.t === b.t &&
    a.pm2_5 === b.pm2_5 &&
    a.pm10 === b.pm10 &&
    a.lecturas === b.lecturas &&
    a.alerta === b.alerta &&
    a.inca === b.inca
  )
}

/**
 * Cada cuánto, como mucho, se escribe en sessionStorage. A ritmo rápido llegan
 * decenas de estados por segundo y lo guardado llega a pesar decenas de KB:
 * escribirlo entero con cada uno eran cientos de escrituras por corrida.
 */
export const GUARDAR_CADA_MS = 500

export class Historia {
  private readonly porEstacion = new Map<string, readonly PuntoSesion[]>()
  private readonly oyentes = new Set<() => void>()
  /** El reloj de la escritura pendiente, si hay cambios sin guardar. */
  private porGuardar: ReturnType<typeof setTimeout> | undefined

  constructor() {
    this.recuperar()
  }

  readonly suscribir = (oyente: () => void) => {
    this.oyentes.add(oyente)
    return () => this.oyentes.delete(oyente)
  }

  /** Los estados recibidos de una estación, del más antiguo al más nuevo. La lista no se modifica: se reemplaza. */
  leer(estacion: string): readonly PuntoSesion[] {
    return this.porEstacion.get(estacion) ?? NINGUNO
  }

  /** Anota un estado recibido. Devuelve si cambió algo. */
  anotar(estacion: string, estado: RegimenEstacion): boolean {
    const punto = puntoDe(estado)
    if (!punto) return false
    const antes = this.leer(estacion)
    const ultimo = antes[antes.length - 1]
    let despues: readonly PuntoSesion[]
    if (!ultimo || punto.t > ultimo.t) {
      despues = [...antes, punto].slice(-TOPE)
    } else if (punto.t === ultimo.t) {
      // Una corrección de la misma hora: reemplaza al punto, no suma otro.
      if (iguales(punto, ultimo)) return false
      despues = [...antes.slice(0, -1), punto]
    } else {
      // El último estado de una estación no retrocede (spec §5.4): si llega uno
      // anterior, el sistema se reinició y lo que se había visto ya no vale.
      despues = [punto]
    }
    this.porEstacion.set(estacion, despues)
    return true
  }

  /** La estación ya no tiene estado en la API (el sistema se reinició): su curva se borra. */
  olvidar(estacion: string): boolean {
    return this.porEstacion.delete(estacion)
  }

  estaciones(): string[] {
    return [...this.porEstacion.keys()]
  }

  /** La sesión terminó: se olvida todo, también lo guardado, y se avisa a quien esté mirando. */
  vaciar() {
    clearTimeout(this.porGuardar)
    this.porGuardar = undefined
    this.porEstacion.clear()
    borrarLoGuardado()
    for (const oyente of [...this.oyentes]) oyente()
  }

  /**
   * Avisa ya a quien esté mirando y deja programada la escritura. Se llama una
   * vez por tanda de cambios; las escrituras se agrupan: una cada GUARDAR_CADA_MS.
   */
  publicar() {
    this.porGuardar ??= setTimeout(() => this.guardar(), GUARDAR_CADA_MS)
    for (const oyente of [...this.oyentes]) oyente()
  }

  /**
   * Escribe ya lo que estuviera pendiente; sin cambios pendientes no hace nada.
   * Lo llama el reloj de publicar() y la página cuando se oculta o se va, para
   * que un F5 a mitad de una ráfaga no pierda lo último.
   */
  guardar() {
    if (this.porGuardar === undefined) return
    clearTimeout(this.porGuardar)
    this.porGuardar = undefined
    try {
      sessionStorage.setItem(CLAVE, JSON.stringify(Object.fromEntries(this.porEstacion)))
    } catch {
      // Sin sessionStorage (o lleno), la curva dura hasta recargar la página.
    }
  }

  private recuperar() {
    try {
      const guardado: unknown = JSON.parse(sessionStorage.getItem(CLAVE) ?? '{}')
      if (typeof guardado !== 'object' || guardado === null) return
      for (const [estacion, puntos] of Object.entries(guardado)) {
        if (!Array.isArray(puntos)) continue
        const validos = (puntos as PuntoSesion[]).filter((p) => typeof p?.t === 'number' && typeof p.fecha_hora === 'string')
        if (validos.length > 0) this.porEstacion.set(estacion, validos)
      }
    } catch {
      // Lo guardado no se puede leer: se empieza de cero.
    }
  }
}

function borrarLoGuardado() {
  try {
    sessionStorage.removeItem(CLAVE)
  } catch {
    // Sin sessionStorage no había nada guardado.
  }
}

const historias = new WeakMap<QueryClient, Historia>()

/**
 * Olvida la historia de una sesión que terminó: quien entre después en la misma
 * pestaña no hereda sus curvas. Borra lo guardado aunque nadie la haya
 * encendido todavía (una sesión que venció antes de recargar la página).
 */
export function olvidarHistoria(consultas: QueryClient) {
  const historia = historias.get(consultas)
  if (historia) historia.vaciar()
  else borrarLoGuardado()
}

/**
 * La historia de una caché. La primera vez que se pide queda mirando la caché
 * (y anota lo que ya tenía); dura lo que dure la caché.
 */
export function historiaDe(consultas: QueryClient): Historia {
  const historia = historias.get(consultas)
  if (historia) return historia
  const nueva = new Historia()
  historias.set(consultas, nueva)

  const anotarLista = (lista: readonly EstadoEstacion[]) => {
    let cambio = false
    const conEstado = new Set(lista.map((e) => e.estacion))
    for (const estacion of nueva.estaciones()) if (!conEstado.has(estacion)) cambio = nueva.olvidar(estacion) || cambio
    for (const estado of lista) cambio = nueva.anotar(estado.estacion, estado) || cambio
    if (cambio) nueva.publicar()
  }
  const anotarUna = (estacion: string, estado: RegimenEstacion | null) => {
    if (estado ? nueva.anotar(estacion, estado) : nueva.olvidar(estacion)) nueva.publicar()
  }
  const mirar = (clave: readonly unknown[], datos: unknown) => {
    if (clave[0] !== claves.regimenes[0] || datos === undefined) return
    if (clave.length === 1 && Array.isArray(datos)) anotarLista(datos as EstadoEstacion[])
    else if (clave.length === 2 && typeof clave[1] === 'string') anotarUna(clave[1], datos as RegimenEstacion | null)
  }

  const cache = consultas.getQueryCache()
  for (const consulta of cache.getAll()) mirar(consulta.queryKey, consulta.state.data)
  cache.subscribe((evento) => {
    if (evento.type === 'updated' && evento.action.type === 'success') mirar(evento.query.queryKey, evento.query.state.data)
  })
  // Lo pendiente se escribe antes de que la página se oculte o se vaya (un F5, cerrar la pestaña).
  if (typeof window !== 'undefined') {
    window.addEventListener('pagehide', () => nueva.guardar())
    document.addEventListener('visibilitychange', () => {
      if (document.visibilityState === 'hidden') nueva.guardar()
    })
  }
  return nueva
}

/**
 * Deja anotando la historia de la sesión. Lo llama el armazón, una sola vez:
 * cada estado que llegue queda para la curva, se mire la pantalla que se mire,
 * también después de recargar la página en cualquiera de ellas.
 */
export function useRegistroDeSesion() {
  const consultas = useQueryClient()
  useEffect(() => {
    historiaDe(consultas)
  }, [consultas])
}

/** Los estados recibidos de una estación en esta sesión, al día. */
export function useHistoria(estacion: string): readonly PuntoSesion[] {
  const consultas = useQueryClient()
  const historia = useMemo(() => historiaDe(consultas), [consultas])
  return useSyncExternalStore(historia.suscribir, () => historia.leer(estacion))
}
